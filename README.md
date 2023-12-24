前言 :

这段时间我做了一个实验 : 用基于 `mmap` 和 `unmmap` 实现的 `buffer` (你要叫它 `arena` 也行), 改写整个 ` parse ` 阶段的内存分配方式以降低 `GC` 的开销, 这个方案听上去让人感觉不安——它不仅要要求你理清楚成员的生命周期 (这个倒无可厚非), 还需要搞清楚成员的内存是来自于 `go` 的还是 `map` (或者说 `c`)的, 如果没拎清楚的话那可真是打开潘多拉魔盒了.
因此实验只能是实验了, 但是期间还是踩了一些有点意思的坑, 写下来权当储备, 给以后的技术选型提供思考的素材. 如果设计之初就考虑到会用到基于 `c` 的 `buffer` 来管理内存的话, 其实应该是一件可以克服的事情.

本文中出现的绝大部分代码用例都可以在 [arena_experiment](https://github.com/jensenojs/arena_experiment) 中获得, 为了使用某些特性, golang 版本要求不低于 `1.21`.

阅读本文不需要对 golang 的 GC 有深刻认知, 但了解过基于 C 的内存分配的逻辑会对阅读本文很有帮助. 预计阅读时间 : 五分钟

# 背景 : 我们的问题是什么

目前 (1.0/1.1) GC 的压力对于 mo 来说有很大的影响, 在一些像 tpcc 的场景 GC 的开销能干到百分之四五十的 CPU. 旭哥建了一个 [MOCissue384](https://github.com/matrixorigin/MO-Cloud/issues/384) 专门跟踪, 并不是某一块区域集中地造成了 GC 的问题, 而是星星点点地遍布在 mo 的每个角落. 

mo 苦 GC 久矣, 是时候对它亮剑了!

##  GC 开销分析与对策

GC 的 big idea 是很简单的, 内存中所有的对象都可以被视为图上的节点, 那些不可达的节点 —— 换言之就是再也访问不到的那些内存——就是可以被 GC 干掉的. go 使用的 GC 就是这种类型, 大体上分成标记阶段和清除阶段 —— 为了追踪扫描进度，`go` 会给遇到的活动内存打上标记。扫描一旦完成，GC 就会遍历堆上的所有内存并把没有标记的内存设置成可用内存。这一过程称之为清除。

一般而言, 扫描的阶段代价都比较大, 这从算法的角度听上去也挺直观, 因此很自然地, 降低扫描代价的要求就是减少堆上对象, 而做到这个事情, 大体上有几种解决的思路:

- 釜底抽薪:使用堆外内存
	- 使用 `cgo`, `mmap` 的内存, 由于这块内存不归 `go` 管, 因此能够完全绕过 GC
		- 这似乎是完全绕开 GC 的唯一方案, 回到手动管理内存的刀耕火种时期
 - 临深履薄:结构体成员中尽量避免使用指针对象
	 - 使用值对象可以减少内存分配的次数和堆内存碎片
		 - 听旭哥说我们的 pb 里面生成对象中的成员都是指针, 可能后面要改

使用内存池重用对象意味着在单位时间内有更少的新对象被放置到堆上，这减缓了堆空间的增长速度，从而减少了 GC 的频率和工作量。
>而且也许针对 `sync.Pool` 会有些什么特别的优化可以做?

内存池是最常见的方案之一, 减少使用指针对象平常是不起眼的细节, 而使用堆外内存就很是有点奇巧淫技的意思了

关于 golang GC 的介绍材料已经非常丰富了, 这里附上官方文档 : [A Guide to the Go Garbage Collector](https://tip.golang.org/doc/gc-guide), 上面讨论的关于 GC 成本模型就来源于此. 
> 这里还有一个[中译版本](https://taoshu.in/go/gc-guide.html)的 (超小声

## 可能的技术方案之一 : arena

这一小节盘点一下, 如果我们要手动管理内存了, 有什么方案是可以被考虑的, 讨论的重点会围绕 `arena` 展开.
但其实现在 (最起码)对于 mo 来说更有价值的是讨论为什么选用 xxx 的方案,  我们这里的 `arena` 方案讨论就图个乐.

## 可能的技术方案之二 : sync. Pool

现在用这个! 迟点仔细瞅瞅

### What it is and why?
 
golang 在 1.20 阶段引入这个实验特性, 在 google 团队内部也有运用, 相关的 [issue 链接](https://github.com/golang/go/issues/51317), 这一部分的主要内容都是提炼自这里. 但是这个提案被无期限的搁置了, 不然直接用它的就完了...

`go` 的工程师主要是这样考虑的:
- 大型 Go 应用程序花费了大量的 CPU 时间进行垃圾回收。此外，平均堆大小通常比必要的要大得多
	- 这是为了减少需要运行垃圾收集器的频率
- 非垃圾回收语言的内存分配和回收开销也很大。由于分配的对象大小和生命周期差异很大，非垃圾回收语言的堆分配器必须是通用的
	- 考虑到分配对象的不同大小和生命周期，这样的分配器必须有相当复杂的代码来为新对象寻找内存和处理内存碎片

为了减少非垃圾回收语言的分配和回收的开销, 一种可能的方案就是**从一个连续的内存区域分配一组内存对象**, 这就是 arena 的大想法.
它很适合处理这样一种场景 —— 某个阶段开始分配大量对象、一段时间内操纵这些对象完成某些计算，完成后就不再需要这些对象的场景, 一个例子是 parser 的阶段, 要生成 ast, 但是在 plan 消费完这个 amt 之后, 理论上 ast 就可以整个都释放掉了.

在类似于 parser/plan/compile-tree 的场景下, 使用 arena 可以高效地分配和批量释放所有的内存.

下面是 go 提案中对于 API 的设计, 这里不展开, 毕竟要分析也是分析我们实现的 arena...
```go
package arena

type Arena struct {
	// contains filtered or unexported fields
}

// New allocates a new arena.
func New() *Arena

// Free frees the arena (and all objects allocated from the arena) so that
// memory backing the arena can be reused fairly quickly without garbage
// collection overhead.  Applications must not call any method on this
// arena after it has been freed.
func (a *Arena) Free()

// New allocates an object from arena a.  If the concrete type of objPtr is
// a pointer to a pointer to type T (**T), New allocates an object of type
// T and stores a pointer to the object in *objPtr.  The object must not
// be accessed after arena a is freed.
func (a *Arena) New(objPtr interface{})

// NewSlice allocates a slice from arena a.  If the concrete type of slicePtr
// is *[]T, NewSlice creates a slice of element type T with the specified
// capacity whose backing store is from the arena a and stores it in
// *slicePtr. The length of the slice is set to the capacity.  The slice must
// not be accessed after arena a is freed.
func (a *Arena) NewSlice(slicePtr interface{}, cap int)

// Clone makes a shallow copy of the input value that is no longer bound to any
// arena it may have been allocated from, returning the copy. If it was not
// allocated from an arena, it is returned untouched. This function is useful
// to more easily let an arena-allocated value out-live its arena.
// T must be a pointer, a slice, or a string, otherwise this function will panic.
func Clone[T any](s T) T {
  	return runtime_arena_heapify(s).(T)
}
```

噢, 对于 `Clone` 方法需要多提一嘴, 当有些 `arena` 中的对象我们需要它拥有更长的生命周期时, 这个方法可以把它挪到堆上. 也有人建议把这个方法的名称改为 `ToHeap`, 我们的实现目前没有这个方法, 想要实现它可能, emmm 也不是一件简单的事情, 后面会进一步讨论, 这个功能的缺失会在某些场景下比较麻烦...

它听上去很完美, 但是正如前面所说, 它被无期限延后了, 我翻到了对应的讨论 : [some worry about arena](https://github.com/golang/go/issues/51317#issuecomment-1056637872), 毕竟 `arena` 成为内置功能之后, 我们使用的所有第三方库都可能使用它. 那我们可能就在不知道什么地方会访问到一块已经被 arena 给 free 掉的内存... 

简单来说, arena 本身可能不是问题, 但是 arena 的使用如果放开了, 那可能会有点绷不住.

https://mp.weixin.qq.com/s/nygLC4o0cmxjo84xXMif2A 

### mo 主库中的 buffer

golang 的工程师很保守, 相比之下我们就也许可以激进一点... ~~好吧, 实际上不行~~

莫尘实现的 buffer c 本质上是基于 mmap/munmap 的 arena, 它的好处和坏处分别如下
- 好处
	- 首先它是个内存池
	- 其次它是个能完全避免 GC 的内存池
- 坏处
	- 手动管理内存的复杂度它都有
	- 如果写代码时不区分内存是来自于 `go` 还是 `arena` , 会有些有趣的问题
		- 这里的“有趣”显然不是什么好东西...

但不管怎么样, 我们先看看已经在 upstream/main 中的 buffer 长什么样, 它位于 `pkg/common/buffer` 目录下,  相关链接🔗 [在这](https://github.com/matrixorigin/matrixone/blob/main/pkg/common/buffer/buffer.go#L23)

#### 提供的接口

它提供类 C 的接口, 你可以直接 `Alloc/Free` 一个对象, `arena` 的一个要求是批量释放, buffer 也提供了支持.
```go
func New() *Buffer
func (b *Buffer) Free()

func Alloc[T any](b *Buffer) *T // call buffer.alloc
func Free[T any](b *Buffer, v *T) // call buffer.free

func MakeSlice[T any](b *Buffer, len, cap int) []T
func FreeSlice[T any](b *Buffer, vs []T)
```

但是主库中没有 `AppendSlice` 的接口, 实现还不完整.

#### arena 特性
下面简要地讲一下我们的 buffer 如何能具有 arena 的特性, 它的结构体成员很简单, 数据就塞在`chunks` 的数组中, 用一把锁管起来.
```go
var ChunkSize int

type chunk struct {
	sync.Mutex
	...
	data     []byte
}

type Buffer struct { // Buffer is our arena
	sync.Mutex
	chunks []*chunk
}
```

##### 批量释放
一个 Buffer 有一系列的 chunk, 因此需要 free 掉整个 arena 的时候, 遍历所有的 chunk 将其 munmap 即可, 这样就支持了批量地释放.
```go
func (b *Buffer) Free() {
	b.Lock()
	defer b.Unlock()
	for i := range b.chunks {
		unix.Munmap(b.chunks[i].data)
	}
	b.chunks = nil
}
```

##### 在连续的内存上分配对象
释放只是好说明, 下面我们来看看它分配一个对象的过程, `Alloc` 函数首先调用 `alloc` 函数申请到一块足够大的内存, 然后转换为对应的类型后返回出去.
```go
func Alloc[T any](b *Buffer) *T {
	var v T

	data := b.alloc(int(unsafe.Sizeof(v)))
	return (*T)(unsafe.Pointer(unsafe.SliceData(data)))
}
```

细节就在调用的 `buffer.alloc` 函数中, 我们已经知道 `buffer` 由一系列的 `chunk` 组成, 首先 `buffer` 会尝试获取到一个 `chunk`, 先不考虑 `chunk` 等于 `nil` 的情况的话, 也就是直接调用 [`chunk.alloc`](https://github.com/matrixorigin/matrixone/blob/main/pkg/common/buffer/chunk.go#L29), 它会返回满足大小的 `[]byte` 给 `Alloc` 用去做类型转换.
```go
func (b *Buffer) alloc(sz int) []byte {
	c := b.pop()
	if c == nil {
		c = b.newChunk()
	}
	data := c.alloc(sz) // not really alloc AT ALL !
	if data == nil {
		c = b.newChunk()
		data = c.alloc(sz)
	}
	b.push(c)
	return data
}
```

稍微扫两眼 [`chunk.alloc`](https://github.com/matrixorigin/matrixone/blob/main/pkg/common/buffer/chunk.go#L29) 之后, 你可能仍然看不懂它具体做了什么事情, 但是你敏锐地意识到这个函数里面并没有内存分配
实际上它做的事情就是清点一下目前取出 `chunk` 的剩余空间够不够这次 `Alloc[T]` 的大小要求, 如果满足的话, 那么就把它“切下来”拿去用而已.
```go
// only do some calculate, not mmap
func (c *chunk) alloc(sz int) []byte {
	c.Lock()
	defer c.Unlock()
	if int(c.off)+sz+int(PointerSize) >= len(c.data) {
		c.flag |= FULL
		return nil
	}
	data := c.data[c.off : int(c.off)+sz+int(PointerSize)]
	*((*unsafe.Pointer)(unsafe.Pointer(unsafe.SliceData(data)))) = unsafe.Pointer(c)
	c.off += uint32(sz + int(PointerSize))
	c.numAlloc++
	return data[PointerSize:]
}
```

那真正的内存分配在哪? 就是在 `chunk == nil` 时会触发的 [`buffer.newChunk`](https://github.com/matrixorigin/matrixone/blob/main/pkg/common/buffer/buffer.go#L78) 中, 它会通过 `mmap` 拿到 `1MB` 的内存
```go
func (b *Buffer) newChunk() *chunk {
	data, err := unix.Mmap(-1, 0, DefaultChunkBufferSize, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_ANON|unix.MAP_PRIVATE)
	if err != nil {
		panic(err)
	}
	c := (*chunk)(unsafe.Pointer(unsafe.SliceData(data)))
	c.data = data
	c.off = uint32(ChunkSize)
	return c
}
```

#### 小节 
到这里, 莫尘的 `buffer` 是如何支持 `arena` 的分配特性也分析完成了. 主库 `buffer` 包中也有如何使用它的单元测试用例, 这里不展开.

它实际上不太是连续的内存, 但提供了基本完整的接口, buffer 其实也没有做到完整的内存复用, 在原本的方案中这个 buffer 是 session 级别的. 如果能提供类似 `Reset` 的方法重置 `buffer` 的状态而不真的将 `chunks` 给 `free` 掉, 那使用 buffer 的话不仅完全避免掉了 `GC`, 在 `session` 存在期间, 在运行一段时间后也几乎不会有真正的内存分配了.

此外, `DefaultChunkBufferSize` 的上限是 `1MB`, 当你使用它申请超过这个上限的对象时, 目前会失败.

### 友商的选择

只是觉得这里该有一个章节, 但我可能会咕咕咕. 比如说 `tidb` 实现了自己的 [arena](https://github.com/pingcap/tidb/blob/master/pkg/util/arena/arena.go),但是它们并不支持泛型, 所以与之对标的应该是其他的东西. 这个部分其实都可以单开一个文章, 先按下不提.

# 利用 buffer 修改 parser 阶段的内存分配

铺垫完成! 接下来让我们用 buffer 来干点有意思的事情,  我们首先准备用 buffer 替换掉整个 parser 阶段的内存分配, 看看它对降低 GC 开销有多大的帮助. 我会挑选一些我还记得同时又不会太无趣的事情来说说.
parser 阶段绝大多数对象的构造都是写死在 yacc 中的, 有构造函数的对象百分之十都不到, 因此首先要给所有的对象都写好对应的构造函数, 构造函数内部通过 `buffer` 来申请内存.
当然也可以在 yacc 中直接写死申请包装的方式, 之前好像莫尘还是远宁哥用 arena 改过一版, 但我还是选择用构造函数的方式, 事后看这一批构造函数可能是唯一留得下来的遗产.

anyway, 在这个方案的初期, 我们的构想就是构建一个 session 级别的 buffer, 把要内存申请的东西统统都通过这个 buffer 走, 在 Query 执行完之后统一释放

## 如何保证正确性

说构造函数的事情其实是为了引出正确性的讨论, 我很汗颜地承认我一开始根本没有“在写代码之前思考好测试方案”的意识 (坏习惯!), 代码正确的充分条件, (最最最起码是必要条件)是什么呢?

拍脑袋, 提出两个标准 : 
1. 改造后的 parser 生成的 ast 应该和原先的 ast 语意一致
2. 由 c 给 alloc 出来的结构体, 它的所有成员也都应该是由 buffer 给 alloc 出来的

第二点显然是这篇文章的重点, 但是第一点的保证要感谢之前参与 parser 的研发, 每种类型的 parser 节点都有 Format 方法, 递归下去可以直接打印出整个 parser-tree 的值, Format 保证当两个相同类型的变量有任意一个成员的值不一致时, 打印出来的结果也不会是一样的. 有了这个基础设施在想要证明第一个标准的时候就没那么无力了

好啦, 让我们回到第二点的讨论, 为什么我们需要这个看上去很苛刻的条件, 考虑这样一个例子 : 

```go
type T struct {
    s *S
}
```

T 对象的空间是从一块数组里面划出来的，垃圾回收其实并不知道 T 这个对象。不过只要 Allocator 里面的大块内存不被回收，T 对象还是安全的。但是，对于 T 里面的 S，它是标准方式分配的，这就会有问题了。

假设发生垃圾回收了，GC 会以为那块内存空间就是一个大的数组，而不会被扫描对象 T，那么t.s 的空间未被任何对象引用到，它会被清理掉。最后t.s 就变成一个悬挂指针了. 下面我会展开讨论这个事情...


https://studygolang.com/articles/7560

- cgocheck=2
	- 源代码解析
	 - runtime. Pinner
 - 源代码解析
	 - 怎么 pin
 - 坑
	 - 字符串的的赋值
	 - 结构体内嵌结构体而不是结构体指针
	 - 结构体指针的新的小问题 (map?)

没有触及到的问题 : 

虽然上面铺垫的模型或者是细节都很简单, 我还是想当一下复读机 (但我克制住了!)

下面的内容会逐渐地加深, 拓展这个模型的内涵和外延, 所以这也许是个合适的节点休息一下.

## 处理细节与坑

看来你已经休息好了, 那我们继续吧!

### example1 : 展开讲讲前面的用例


```
write of unpinned Go pointer 0x140169f9b20 to non-Go memory 0x113aec378
fatal error: unpinned Go pointer stored into non-Go memory
```

本质上,  所有指针类型的成员都是需要这么处理的, 所以呃呃呃我们清点一下谁是卧底?
- 指针
- 切片
- 接口
- 字符串

***
- channel 好像也是
- map 可能特殊一点...

### example2 : 无法满足的标准 2

runtime. Pinner 之前的一个丑陋的方案...

#### 接口

```go
type MinValueOption struct {
	Minus bool
	Num   any
}
```

对于这种

### example3 : 邪恶的字符串


### example4 : 可以跳过 CGOCHECK2 检验的方式


## 其他有趣的问题

这个小节记录一些有趣的问题, 和它们的 (可能的) 解决方式

### 鸡生蛋还是蛋生鸡

复习一下 : 前面提到, 一个 `arena` 中的所有对象的生命周期应该是一样的, 对于少部分特殊的情况 `arena` 的接口提供了 Clone 函数将其移动到堆上.

这个事情用在 parser 上似乎有个特有的问题 : 像 `select` 之类的 SQL, 它们的 parser-tree 在执行完后多半就不再有意义了. 但是像 `prepare` 或者 `set` 这样有梦想的 SQL, 它们的生命周期就不再 (只)是 Query 级别的了, 此外还有 planCache 之类的优化也会打破原本对生命周期的假设.
一个直觉的解决方案就是 Session 中不再要只挂一个 buffer 了, 应该根据在 Session 期间有几类不同生命周期的对象, 就创建几个对应的池子.
```go
// Different buffers manage objects with different lifecycles
type sessionBuf struct {
	// normal SQL can be released after doComQuery finishes executing.
	queryLevelBuf *buffer.Buffer

	// the exception is SQL like Prepare, Set.
	sessionLevelBuf *buffer.Buffer
}
```

然后根据 SQL 的类型来判断, 如果这个 SQL 是 query 级别的, 那么在 parser 的时候就把 `queryLevelBuf` 传下去, 否则就用 `sessionLevelBuf`.... 吗 ? 但这里有点诡异的地方是概念上本来就是 parser 完成之后才知道这个 SQL 的类型是什么的, 现在要先知道 SQL 的类型后才好在内部决定改用哪个生命周期级别的 buffer 去申请内存...

暂时的解决方案只能是用正则表达式做字符串扫描啦, 看看 SQL 是不是 prepare 或者是 set. 其中还需要考虑到条件注释的 sql 中也可能会有 prepare 或者是 set 类型的 SQL.
```go
func isPrepareOrSetSQL(sql string) bool {
    // Normalize SQL string by removing leading/trailing whitespace
    sql = strings.TrimSpace(sql)

    // First, check for normal PREPARE or SET at the beginning of the string
    if strings.HasPrefix(strings.ToLower(sql), "prepare ") || strings.HasPrefix(strings.ToLower(sql), "set ") {
        return true
    }

    // Now, compile a regular expression to match executable comments
    // that contain PREPARE or SET
    re := regexp.MustCompile(`\/\*!\d{5}\s(set|prepare)\s`)

    // Check if the SQL contains executable comment with PREPARE or SET
    return re.MatchString(strings.ToLower(sql))
}
```

条件注释是长得像这样的 SQL, 呃呃呃
```SQL
/*!40101 -- SET @saved_cs_client = @@character_set_client */;
/*!40101 -- SET character_set_client = utf8 */;
/*!40000 create database if not exists mysql_ddl_test_db_5 */;
/*!40000 show tables in mysql_ddl_test_db_5 */;
```

事实上, 这个问题就不能说被解决了, 请看下面两个 SQL...
```
prepare stmt1 from select * from t
prepare stmt1 from "select * from t"
```
真正需要被 prepare 的实际上应该是 `select * from t`, 它实际上是整个 stmts 中的一部分,  但是这种识别方式区分不出来, 它只会把所有的 stmts 都给用 session 级别的 buffer 去申请了, 虽然没出什么问题. 但这显然不太对, 但但好像不太有很显然的解决方案...

### 连接不上 Mo -tester (set 的问题)
这个问题很轻松, 只是有意思, 背景是我越过前面写的群山峻岭之后 —— 很长一段时间里面我的 mo 是连启动都启不起来的卑微状态 (从这个角度来说 cgocheck2 真是好样的 2333)到能起起来之后, 我发现虽然我能用 mysql-client 给它连接上, 但 mo-tester 不行, 也就是 jdbc 不行.
这可怎么办呀, ~~当然是摇人~~


## 没有严肃讨论的问题


####  缺失的 `Clone`


#### `AppendSlice` 时候的 `Copy`


#### 序列化和反序列化

呃呃呃,  这是我们的一些超级工程师在技术选型的时候讨论的主题, 但我旁听的时候没跟上 hhhhh, mark 一下, 后面要论述.

实际上在写大纲的时候这里还有几个主题, 但是真写到这里的时候忘记了-.-, 已经很长了这篇文章! 所以就到这里吧.

## 小节 

基础设施清点 :
- CGOCHECK2
- runtime. Pinner

弊病:
边界
仍然不为够的基础设施

# 成果展示


# CGOCheck2 代码阅读
