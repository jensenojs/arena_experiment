// package main

// import (
// 	"arena_experiment/pkg/buffer"
// )

// type Person struct {
// 	name string
// }

// func NewPerson(name string, buf *buffer.Buffer) *Person {
// 	p := buffer.Alloc[Person](buf) // memory from mmap
// 	p.name = name
// 	return p
// }

// func main() {
// 	buf := buffer.New()
// 	defer buf.Free()

// 	name := "tom"

// 	p := NewPerson(name, buf)
// 	println(p.name)

// }

package main

/*
#include <sys/mman.h>
#include <string.h>
#include <unistd.h>

void set_string(char** dst, char* src) {
    *dst = src;
}
*/
import "C"
import (
	"runtime"
	"unsafe"
)

func main() {
    pageSize := C.getpagesize()

    // 分配一页可以写的内存
    memory := C.mmap(nil, C.size_t(pageSize), C.PROT_READ|C.PROT_WRITE, C.MAP_ANON|C.MAP_PRIVATE, -1, 0)
    defer C.munmap(memory, C.size_t(pageSize))

    // 这里应该会导致 cgocheck2 抓到错误
    goString := "hello"
	runtime.GC()
    C.set_string((**C.char)(unsafe.Pointer(&memory)), C.CString(goString))
}