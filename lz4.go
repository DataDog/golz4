package lz4

// #cgo pkg-config: liblz4
// #include <lz4.h>
// #include <stdlib.h>
import "C"

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"unsafe"
)

const (
	streamingBlockSize        = 1024 * 64
	blockHeaderSize           = 4
	boundedStreamingBlockSize = streamingBlockSize + streamingBlockSize/255 + 16

	hugeStreamingBlockSize        = 1024 * 1024 * 5
	boundedHugeStreamingBlockSize = hugeStreamingBlockSize + hugeStreamingBlockSize/255 + 16
)

// p gets a char pointer to the first byte of a []byte slice
func p(in []byte) *C.char {
	if len(in) == 0 {
		return (*C.char)(unsafe.Pointer(nil))
	}
	return (*C.char)(unsafe.Pointer(&in[0]))
}

// clen gets the length of a []byte slice as a char *
func clen(s []byte) C.int {
	return C.int(len(s))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Uncompress with a known output size. len(out) should be equal to
// the length of the uncompressed out.
func Uncompress(out, in []byte) (outSize int, err error) {
	outSize = int(C.LZ4_decompress_safe(p(in), p(out), clen(in), clen(out)))
	if outSize < 0 {
		err = errors.New("Malformed compression stream")
	}
	return
}

// CompressBound calculates the size of the output buffer needed by
// Compress. This is based on the following macro:
//
// #define LZ4_COMPRESSBOUND(isize)
//      ((unsigned int)(isize) > (unsigned int)LZ4_MAX_INPUT_SIZE ? 0 : (isize) + ((isize)/255) + 16)
func CompressBound(in []byte) int {
	return len(in) + ((len(in) / 255) + 16)
}

// Compress compresses in and puts the content in out. len(out)
// should have enough space for the compressed data (use CompressBound
// to calculate). Returns the number of bytes in the out slice.
func Compress(out, in []byte) (outSize int, err error) {
	outSize = int(C.LZ4_compress_default(p(in), p(out), clen(in), clen(out)))
	if outSize == 0 {
		err = errors.New("Insufficient space for compression")
	}
	return
}

// Writer is an io.WriteCloser that lz4 compress its input.
type Writer struct {
	compressionBuffer      [2]unsafe.Pointer
	mallocBuffer           unsafe.Pointer
	lz4Stream              *C.LZ4_stream_t
	underlyingWriter       io.Writer
	inpBufIndex            int
	totalCompressedWritten int
}

// NewWriter creates a new Writer. Writes to
// the writer will be written in compressed form to w.
func NewWriter(w io.Writer) *Writer {
	// The input buffers MUST NOT be contiguous in memory. LZ4_compress_fast_continue has the
	// following comment:
	//
	// When input is structured as a double-buffer, each buffer can have any size, including < 64 KB.
	// Make sure that buffers are separated, by at least one byte.
	//
	// If the buffers are contiguous, then LZ4 assumes small writes can be "combined" for better
	// compression. For example, Write(65536); Write(1000). For the second block, it will look back
	// 64 kiB into the previous block. This means when decompressing, you will need
	// to similarly keep the buffers contiguous in memory. This code was written assuming the buffers
	// are separate. With glibc on Linux, it appears that malloc will usually allocate the buffers
	// with some space between them. However, on Mac OS X, the buffers are often contiguous.
	// See: https://github.com/lz4/lz4/issues/473#issuecomment-366537441

	// separate the buffers by 8 bytes, so LZ4 treats them as separate. 8 bytes means the buffer
	// start is 8 byte aligned, which may permit optimizations on 64-bit CPUs.
	mallocBuffer := C.malloc(2*streamingBlockSize + 8)
	buffer1 := mallocBuffer
	buffer2 := unsafe.Pointer(uintptr(mallocBuffer) + streamingBlockSize + 8)

	return &Writer{
		compressionBuffer: [2]unsafe.Pointer{buffer1, buffer2},
		mallocBuffer:      mallocBuffer,
		lz4Stream:         C.LZ4_createStream(),
		underlyingWriter:  w,
	}
}

// Write writes a compressed form of src to the underlying io.Writer.
func (w *Writer) Write(src []byte) (int, error) {
	remainingBytes := len(src)
	totalWritten := 0

	for remainingBytes > 0 {
		endIdx := totalWritten + streamingBlockSize
		if endIdx > len(src) {
			endIdx = len(src)
		}
		written, err := w.writeFrame(src[totalWritten:endIdx])
		if err != nil {
			return totalWritten, err
		}
		totalWritten += written
		remainingBytes -= written
	}

	return totalWritten, nil
}

func (w *Writer) writeFrame(src []byte) (int, error) {
	var compressedBuf [boundedStreamingBlockSize]byte
	inpPtr := w.nextInputBuffer()

	copy(inpPtr, src)

	written := int(C.LZ4_compress_fast_continue(
		w.lz4Stream,
		p(inpPtr),
		p(compressedBuf[:]),
		C.int(len(src)),
		C.int(len(compressedBuf)),
		1))
	if written <= 0 {
		return 0, errors.New("error compressing")
	}

	// Write "header" to the buffer for decompression
	var header [4]byte
	binary.LittleEndian.PutUint32(header[:], uint32(written))
	_, err := w.underlyingWriter.Write(header[:])
	if err != nil {
		return 0, err
	}

	// Write to underlying buffer
	_, err = w.underlyingWriter.Write(compressedBuf[:written])
	if err != nil {
		return 0, err
	}

	w.totalCompressedWritten += written + 4
	return len(src), nil
}

func (w *Writer) nextInputBuffer() []byte {
	w.inpBufIndex = (w.inpBufIndex + 1) % 2
	return unsafe.Slice((*byte)(w.compressionBuffer[w.inpBufIndex]), streamingBlockSize)
}

// Close releases all the resources occupied by Writer.
// w cannot be used after the release.
func (w *Writer) Close() error {
	if w.lz4Stream != nil {
		C.LZ4_freeStream(w.lz4Stream)
		w.lz4Stream = nil
		C.free(w.mallocBuffer)
		w.mallocBuffer = nil
	}
	return nil
}

// reader is an io.ReadCloser that decompresses when read from.
type reader struct {
	lz4Stream        *C.LZ4_streamDecode_t
	pending          []byte
	left             unsafe.Pointer
	right            unsafe.Pointer
	underlyingReader io.Reader
	isLeft           bool
}

// NewReader creates a new io.ReadCloser.  Reads from the returned ReadCloser
// read and decompress data from r.  It is the caller's responsibility to call
// Close on the ReadCloser when done.  If this is not done, underlying objects
// in the lz4 library will not be freed.
//
// Deprecated: Use NewDecompressReader instead. It can decompress the output
// of NewWriter, but uses fewer allocations.
func NewReader(r io.Reader) io.ReadCloser {
	return &reader{
		lz4Stream:        C.LZ4_createStreamDecode(),
		underlyingReader: r,
		isLeft:           true,
		// As per lz4 docs:
		//
		//   *_continue() :
		//     These decoding functions allow decompression of multiple blocks in "streaming" mode.
		//     Previously decoded blocks must still be available at the memory position where they were decoded.
		//
		// double buffer needs to use C.malloc to make sure the same memory address
		// allocate buffers in go memory will fail randomly since GC may move the memory
		left:  C.malloc(boundedStreamingBlockSize),
		right: C.malloc(boundedStreamingBlockSize),
	}
}

// Close releases all the resources occupied by r.
// r cannot be used after the release.
func (r *reader) Close() error {
	if r.lz4Stream != nil {
		C.LZ4_freeStreamDecode(r.lz4Stream)
		r.lz4Stream = nil
	}

	C.free(r.left)
	C.free(r.right)
	return nil
}

// Read decompresses `compressionBuffer` into `dst`.
// dst buffer must of at least streamingBlockSize bytes large
func (r *reader) Read(dst []byte) (int, error) {
	if len(dst) == 0 {
		return 0, nil
	}
	// Write data read from a previous call
	if r.pending != nil {
		return r.readFromPending(dst)
	}

	blockSize, err := r.readSize(r.underlyingReader)
	if err != nil {
		return 0, err
	}

	// read blockSize from r.underlyingReader --> readBuffer
	var uncompressedBuf [boundedStreamingBlockSize]byte
	_, err = io.ReadFull(r.underlyingReader, uncompressedBuf[:blockSize])
	if err != nil {
		return 0, err
	}

	var ptr unsafe.Pointer
	if r.isLeft {
		ptr = r.left
		r.isLeft = false
	} else {
		ptr = r.right
		r.isLeft = true
	}

	decompressed := int(C.LZ4_decompress_safe_continue(
		r.lz4Stream,
		(*C.char)(unsafe.Pointer(&uncompressedBuf[0])),
		(*C.char)(ptr),
		C.int(blockSize),
		C.int(streamingBlockSize),
	))

	if decompressed < 0 {
		// io.Reader requires Read to return a value in range [0, len(dst)]
		return 0, fmt.Errorf("error decompressing; result=%d", decompressed)
	}

	mySlice := C.GoBytes(ptr, C.int(decompressed))
	copySize := min(decompressed, len(dst))

	copied := copy(dst, mySlice[:copySize])

	if decompressed > len(dst) {
		// Save data for future reads
		r.pending = mySlice[copied:]
	}

	return copied, nil
}

// read the 4-byte little endian size from the head of each stream compressed block
func (r *reader) readSize(rdr io.Reader) (int, error) {
	var temp [4]byte
	_, err := io.ReadFull(rdr, temp[:])
	if err != nil {
		return 0, err
	}

	return int(binary.LittleEndian.Uint32(temp[:])), nil
}

func (r *reader) readFromPending(dst []byte) (int, error) {
	copySize := min(len(dst), len(r.pending))
	copied := copy(dst, r.pending[:copySize])

	if copied == len(r.pending) {
		r.pending = nil
	} else {
		r.pending = r.pending[copied:]
	}
	return copied, nil
}

// CompressReader reads input and creates an io.ReadCloser for reading
// compressed output
type CompressReader struct {
	underlyingReader       io.Reader
	compressionBuffer      [2]unsafe.Pointer
	outputBuffer           *bytes.Reader
	lz4Stream              *C.LZ4_stream_t
	inpBufIndex            int
	totalCompressedWritten int
	compressedBuffer       unsafe.Pointer
}

// NewCompressReader creates a new io.ReadCloser.  Reads from the returned ReadCloser
// read and compress data from r.  It is the caller's responsibility to call
// Close on the ReadCloser when done.  If this is not done, underlying objects
// in the lz4 library will not be freed. The compressed output must be decompressed
// using NewDecompressReader.
func NewCompressReader(r io.Reader) *CompressReader {
	return &CompressReader{
		compressionBuffer: [2]unsafe.Pointer{
			C.malloc(hugeStreamingBlockSize),
			C.malloc(hugeStreamingBlockSize),
		},
		lz4Stream:        C.LZ4_createStream(),
		underlyingReader: r,
		outputBuffer:     bytes.NewReader(nil),
		compressedBuffer: C.malloc(boundedHugeStreamingBlockSize + blockHeaderSize),
	}
}

// Read compresses data from the underlyingReader into dst.
func (r *CompressReader) Read(dst []byte) (int, error) {
	// try to consume from the buffer
	n, _ := r.outputBuffer.Read(dst)
	// ignoring err which can only be EOF in which case bytes read is 0
	if n > 0 {
		// if the buffer contains anything it's leftover from a previous call
		return n, nil
	}

	// the buffer is empty, we are going to write into it so we reset it first
	totalBlockSize := boundedHugeStreamingBlockSize + blockHeaderSize
	inpPtr := r.nextInputBuffer()
	outPtr := ptrToByteSlice(r.compressedBuffer, totalBlockSize, totalBlockSize)

	bytesRead, err := io.ReadFull(r.underlyingReader, inpPtr)
	if err == io.EOF {
		// nothing left to read from the source
		return 0, err
	}
	if err != nil && err != io.ErrUnexpectedEOF {
		// ErrUnexpectedEOF occurs when some bytes are read but not all the bytes (n > 0)
		return 0, fmt.Errorf("error reading source: %s", err)
	}

	// compress and write the data into compressedBuf, leaving space for the
	// 4 byte header
	written := int(C.LZ4_compress_fast_continue(
		r.lz4Stream,
		p(inpPtr),
		p(outPtr[blockHeaderSize:]),
		C.int(bytesRead),
		C.int(boundedHugeStreamingBlockSize),
		1))
	if written <= 0 {
		return 0, errors.New("error compressing")
	}

	// write "header" to the buffer for decompression at the first 4 bytes
	binary.LittleEndian.PutUint32(outPtr[:blockHeaderSize], uint32(written))

	// populate the buffer with our internal slice and consume from it
	r.outputBuffer = bytes.NewReader(outPtr[:written+blockHeaderSize])
	n, _ = r.outputBuffer.Read(dst)
	// here we ignore any EOF because the buffer contains partial data only
	// EOF will be communicated on the next call if the underlying Reader is exhausted

	r.totalCompressedWritten += written + 4
	return n, nil
}

func (r *CompressReader) nextInputBuffer() []byte {
	r.inpBufIndex = (r.inpBufIndex + 1) % 2
	return ptrToByteSlice(r.compressionBuffer[r.inpBufIndex], hugeStreamingBlockSize, hugeStreamingBlockSize)
}

// Close releases all the resources occupied by Reader.
// r cannot be used after the release.
func (r *CompressReader) Close() error {
	if r.lz4Stream != nil {
		C.LZ4_freeStream(r.lz4Stream)
		r.lz4Stream = nil
	}
	C.free(r.compressionBuffer[0])
	C.free(r.compressionBuffer[1])
	C.free(r.compressedBuffer)
	return nil
}

// DecompressReader is an io.ReadCloser that decompresses when read from.
type DecompressReader struct {
	lz4Stream           *C.LZ4_streamDecode_t
	outputBuffer        *bytes.Reader
	decompressionBuffer [2]unsafe.Pointer
	underlyingReader    io.Reader
	inpBufIndex         int
	compressedBuffer    unsafe.Pointer
}

// NewDecompressReader creates a new io.ReadCloser. This function mirrors the
// behavior of NewReader but provides better performance.
// It is the caller's responsibility to call Close on the ReadCloser when done.
// If this is not done, underlying objects in the lz4 library will not be freed.
func NewDecompressReader(r io.Reader) io.ReadCloser {
	return &DecompressReader{
		lz4Stream:        C.LZ4_createStreamDecode(),
		underlyingReader: r,
		decompressionBuffer: [2]unsafe.Pointer{
			// double buffer needs to use C.malloc to make sure the same memory address
			// allocate buffers in go memory will fail randomly since GC may move the memory
			C.malloc(hugeStreamingBlockSize),
			C.malloc(hugeStreamingBlockSize),
		},
		outputBuffer:     bytes.NewReader(nil),
		compressedBuffer: C.malloc(boundedHugeStreamingBlockSize),
	}
}

// Read decompresses data from the underlying reader into `dst`.
func (r *DecompressReader) Read(dst []byte) (int, error) {
	// write data read from a previous call
	n, _ := r.outputBuffer.Read(dst)
	// ignoring err which can only be EOF in which case bytes read is 0
	if n > 0 {
		// if the buffer contains anything it's leftover from a previous call
		return n, nil
	}

	compressedBlockSize, err := r.readSize(r.underlyingReader)
	if err != nil {
		return 0, err
	}

	inPtr := ptrToByteSlice(r.compressedBuffer, boundedHugeStreamingBlockSize, boundedHugeStreamingBlockSize)
	outPtr := r.nextDecompressionBuffer()

	// read the compressed blockSize from r.underlyingReader
	_, err = io.ReadFull(r.underlyingReader, inPtr[:compressedBlockSize])
	if err != nil {
		return 0, err
	}

	decompressed := int(C.LZ4_decompress_safe_continue(
		r.lz4Stream,
		p(inPtr),
		p(outPtr),
		C.int(compressedBlockSize),
		C.int(hugeStreamingBlockSize),
	))

	if decompressed < 0 {
		return decompressed, errors.New("error decompressing")
	}

	// write the decompressed data to the output buffer
	r.outputBuffer = bytes.NewReader(outPtr[:decompressed])
	// read as much as we can into dst, ignoring any EOF
	n, _ = r.outputBuffer.Read(dst)

	return n, nil
}

// Close releases all the resources occupied by r.
// r cannot be used after the release.
func (r *DecompressReader) Close() error {
	if r.lz4Stream != nil {
		C.LZ4_freeStreamDecode(r.lz4Stream)
		r.lz4Stream = nil
	}

	C.free(r.decompressionBuffer[0])
	C.free(r.decompressionBuffer[1])
	C.free(r.compressedBuffer)
	return nil
}

func (r *DecompressReader) nextDecompressionBuffer() []byte {
	r.inpBufIndex = (r.inpBufIndex + 1) % 2
	return ptrToByteSlice(r.decompressionBuffer[r.inpBufIndex], hugeStreamingBlockSize, hugeStreamingBlockSize)
}

// read the 4-byte little endian size from the head of each stream compressed block
func (r *DecompressReader) readSize(rdr io.Reader) (int, error) {
	var temp [blockHeaderSize]byte
	_, err := io.ReadFull(rdr, temp[:])
	if err != nil {
		return 0, err
	}
	return int(binary.LittleEndian.Uint32(temp[:])), nil
}

func ptrToByteSlice(dataPtr unsafe.Pointer, _len, _cap int) []byte {
	return unsafe.Slice((*byte)(dataPtr), _len)
}
