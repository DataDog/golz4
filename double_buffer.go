package lz4

// #include <stdlib.h>
import "C"

import (
	"reflect"
	"unsafe"
)

// doubleBuffer provides a C malloc'd double buffer.
//
// The reason this is needed is because the streaming API of the lz4 C library
// relies on buffer content from the previous call and this content needs to be
// located at the exact same address in memory.
type doubleBuffer struct {
	bufferSize int
	buffers    [2]unsafe.Pointer
	current    int
}

// newDoubleBuffer creates double buffer, each of the given size in bytes
//
// Unused doubleBuffer need to be released using Free() to avoid memory leaks
func newDoubleBuffer(sizeBytes int) *doubleBuffer {
	return &doubleBuffer{
		bufferSize: sizeBytes,
		buffers: [2]unsafe.Pointer{
			C.malloc(C.ulong(sizeBytes)),
			C.malloc(C.ulong(sizeBytes)),
		},
		current: 0,
	}
}

// Free the internal buffers.
//
// Once this method has been called, the doubleBuffer CANNOT be used anymore
func (b *doubleBuffer) Free() {
	C.free(b.buffers[0])
	b.buffers[0] = nil
	C.free(b.buffers[1])
	b.buffers[1] = nil

	b.bufferSize = 0
	b.current = 0
}

// NextBuffer pulls a buffer, in a round-robin fashion
func (b *doubleBuffer) NextBuffer() []byte {
	ptr := b.buffers[b.current]
	tmpSlice := reflect.SliceHeader{
		Data: uintptr(ptr),
		Len:  b.bufferSize,
		Cap:  b.bufferSize,
	}
	byteSlice := *(*[]byte)(unsafe.Pointer(&tmpSlice))
	b.current = (b.current + 1) % 2
	return byteSlice
}
