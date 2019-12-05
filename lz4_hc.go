package lz4

// #cgo pkg-config: liblz4
// #include <lz4hc.h>
import "C"

import (
	"fmt"
)

// CompressHC compresses in and puts the content in out. len(out)
// should have enough space for the compressed data (use CompressBound
// to calculate). Returns the number of bytes in the out slice. Determines
// the compression level automatically.
func CompressHC(out, in []byte) (int, error) {
	// 0 automatically sets the compression level.
	return CompressHCLevel(out, in, 0)
}

// CompressHCLevel compresses in at the given compression level and puts the
// content in out. len(out) should have enough space for the compressed data
// (use CompressBound to calculate). Returns the number of bytes in the out
// slice. To automatically choose the compression level, use 0. Otherwise, use
// any value in the inclusive range 1 (worst) through 16 (best). Most
// applications will prefer CompressHC.
func CompressHCLevel(out, in []byte, level int) (outSize int, err error) {
	// LZ4HC does not handle empty buffers. Pass through to Compress.
	if len(in) == 0 || len(out) == 0 {
		return Compress(out, in)
	}

	outSize = int(C.LZ4_compressHC2_limitedOutput(p(in), p(out), clen(in), clen(out), C.int(level)))
	if outSize == 0 {
		err = fmt.Errorf("insufficient space for compression")
	}
	return
}
