// Copyright 2021 - 2023 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package buffer

import (
	"runtime"
	"unsafe"

	"golang.org/x/sys/unix"
)

func New() *Buffer {
	b := new(Buffer)
	b.pinner = new(runtime.Pinner)
	b.pinner.Pin(b)
	return b
}

func (b *Buffer) Pin(os ...any) {
	for _, o := range os {
		b.pinner.Pin(o)
	}
}

// Don't use this function unless you're pretty sure that the structure's string won't be reassigned later. 
// If a string from go memory replaces it, there is a risk of a segmentation error.
func (b *Buffer) CopyString(src string) string {
	sv := MakeSlice[byte](b, len(src), len(src))
	copy(sv, []byte(src))
	return bytes2String(sv)
}

func bytes2String(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return unsafe.String(&b[0], len(b))
}

func (b *Buffer) Free() {
	if b == (*Buffer)(nil) {
		panic("free with nil buffer")
	}
	b.Lock()
	b.pinner.Unpin()
	defer b.Unlock()
	for i := range b.chunks {
		unix.Munmap(b.chunks[i].data)
	}
	b.chunks = nil
}

func Alloc[T any](b *Buffer) *T {
	if b == (*Buffer)(nil) {
		panic("allow with nil buffer")
	}
	var v T

	data := b.alloc(int(unsafe.Sizeof(v)))
	return (*T)(unsafe.Pointer(unsafe.SliceData(data)))
}

func Free[T any](b *Buffer, v *T) {
	if b == (*Buffer)(nil) {
		panic("free with nil buffer")
	}
	b.free(unsafe.Slice((*byte)(unsafe.Pointer(v)), unsafe.Sizeof(*v)))
}

func MakeSlice[T any](b *Buffer, len_and_cap ...int) []T {
	if b == (*Buffer)(nil) {
		panic("make slice with nil buffer")
	}

	var l int
	var c int
	if len_and_cap == nil {
		l = 0
		c = 4
	} else {
		if len(len_and_cap) != 2 {
			panic("slice should set len and cap at the same time")
		}
		l = len_and_cap[0]
		c = len_and_cap[1]
		if c < l || c < 0 {
			panic("illegal slice len or illegal cap")
		}
	}

	var v T
	data := b.alloc(int(unsafe.Sizeof(v)) * c)
	return unsafe.Slice((*T)(unsafe.Pointer(unsafe.SliceData(data))), c)[:l]
}

// https://github.com/golang/go/issues/45380

func AppendSlice[T any](b *Buffer, slice []T, vs ...T) []T {
	if b == (*Buffer)(nil) {
		panic("append slice with nil buffer")
	}

	var newCap int
	if len(slice)+len(vs) <= cap(slice) {
		for _, v := range vs {
			slice = append(slice, v)
		}
		return slice
	} else {
		newCap = cap(slice) * 2
		if newCap < 4 {
			newCap = 4
		}
		for len(slice)+len(vs) > newCap {
			newCap = newCap * 2
		}
	}

	newSlice := MakeSlice[T](b, len(slice), newCap)

	// don't know why copy will occur some "fatal error: bulkBarrierPreWrite: unaligned arguments"
	// currently AppendSlice can't check if slice and vs both alloced by buffer,
	
	// copy(newSlice, slice)
	for i := 0; i < len(slice); i++ {
		newSlice[i] = slice[i]
	}
	
	defer FreeSlice[T](b, slice)

	for _, v := range vs {
		newSlice = append(newSlice, v)
	}

	return newSlice
}

func FreeSlice[T any](b *Buffer, vs []T) {
	if b == (*Buffer)(nil) {
		panic("free slice with nil buffer")
	}

	var v T

	b.free(unsafe.Slice((*byte)(unsafe.Pointer(unsafe.SliceData(vs))),
		cap(vs)*int(unsafe.Sizeof(v))))
}

func (b *Buffer) pop() *chunk {
	b.Lock()
	defer b.Unlock()
	if len(b.chunks) == 0 {
		return nil
	}
	c := b.chunks[0]
	b.chunks = b.chunks[1:]
	return c
}

func (b *Buffer) push(c *chunk) {
	b.Lock()
	defer b.Unlock()
	b.chunks = append(b.chunks, c)
}

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

func (b *Buffer) alloc(sz int) []byte {
	c := b.pop()
	if c == nil {
		c = b.newChunk()
	}
	data := c.alloc(sz)
	if data == nil {
		c = b.newChunk()
		data = c.alloc(sz)
	}
	b.push(c)
	return data
}

func (b *Buffer) free(data []byte) {
	ptr := *((*unsafe.Pointer)(unsafe.Add(unsafe.Pointer(unsafe.SliceData(data)), -PointerSize)))
	c := (*chunk)(ptr)
	c.free()
}
