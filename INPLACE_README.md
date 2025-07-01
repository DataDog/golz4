# In-Place Compression and Decompression Support

This document describes the new in-place compression and decompression functionality added to the lz4 Go package. These features enable memory-efficient compression operations by reusing the same buffer for both input and output data.

## Overview

In-place compression and decompression allow you to use a single buffer for both input and output, reducing memory allocations and improving performance in memory-constrained environments. This implementation follows the liblz4 specifications for in-place operations.

## Buffer Layout

### In-Place Decompression
```
|<------------------------buffer--------------------------------->|
                             |<-----------compressed data--------->|
|<-----------decompressed size------------------>|
                                                 |<----margin---->|
```

### In-Place Compression  
```
|<------------------------buffer--------------------------------->|
                                          |<----input data----->|
|<-----------compressed output----------->|
                                          |<----margin--------->|
```

## API Functions

### Buffer Size Calculation

#### `DecompressInplaceMargin(compressedSize int) int`
Calculates the margin needed for in-place decompression based on compressed data size.

#### `DecompressInplaceBufferSize(decompressedSize int) int`
Calculates the minimum buffer size needed for in-place decompression.

#### `CompressInplaceBufferSize(maxCompressedSize int) int`
Calculates the minimum buffer size needed for in-place compression.

### Compression and Decompression

#### `UncompressInplace(buffer []byte, compressedSize, decompressedSize, compressedOffset int) (outSize int, err error)`
Decompresses data in-place within the same buffer.

**Parameters:**
- `buffer`: Buffer containing compressed data at the end
- `compressedSize`: Size of the compressed data
- `decompressedSize`: Expected size of the decompressed data
- `compressedOffset`: Offset where compressed data starts in the buffer

**Returns:** Number of bytes decompressed or error

#### `CompressInplace(buffer []byte, inputSize, maxCompressedSize, inputOffset int) (outSize int, err error)`
Compresses data in-place within the same buffer.

**Parameters:**
- `buffer`: Buffer containing input data at the end
- `inputSize`: Size of the input data
- `maxCompressedSize`: Maximum allowed size for compressed output
- `inputOffset`: Offset where input data starts in the buffer

**Returns:** Number of bytes compressed or error

## Usage Examples

### Basic In-Place Decompression

```go
// Assume you have compressed data from somewhere
compressedData := getCompressedData()
decompressedSize := getOriginalSize()

// Calculate required buffer size
bufferSize := lz4.DecompressInplaceBufferSize(decompressedSize)
buffer := make([]byte, bufferSize)

// Place compressed data at the end of the buffer
compressedOffset := bufferSize - len(compressedData)
copy(buffer[compressedOffset:], compressedData)

// Decompress in-place
outSize, err := lz4.UncompressInplace(buffer, len(compressedData), decompressedSize, compressedOffset)
if err != nil {
    log.Fatal(err)
}

// Decompressed data is now at the beginning of the buffer
decompressed := buffer[:outSize]
```

### Basic In-Place Compression

```go
input := []byte("Hello, world!")

// Calculate required buffer size
maxCompressedSize := lz4.CompressBound(input)
bufferSize := lz4.CompressInplaceBufferSize(maxCompressedSize)
buffer := make([]byte, bufferSize)

// Place input data at the end of the buffer
inputOffset := bufferSize - len(input)
copy(buffer[inputOffset:], input)

// Compress in-place
compressedSize, err := lz4.CompressInplace(buffer, len(input), maxCompressedSize, inputOffset)
if err != nil {
    log.Fatal(err)
}

// Compressed data is now at the beginning of the buffer
compressed := buffer[:compressedSize]
```

### Round-Trip Example

```go
data := []byte("This is test data for compression")

// Step 1: In-place compression
maxCompressedSize := lz4.CompressBound(data)
compressBufferSize := lz4.CompressInplaceBufferSize(maxCompressedSize)
compressBuffer := make([]byte, compressBufferSize)

inputOffset := compressBufferSize - len(data)
copy(compressBuffer[inputOffset:], data)

compressedSize, err := lz4.CompressInplace(compressBuffer, len(data), maxCompressedSize, inputOffset)
if err != nil {
    log.Fatal(err)
}

// Step 2: In-place decompression
decompressBufferSize := lz4.DecompressInplaceBufferSize(len(data))
decompressBuffer := make([]byte, decompressBufferSize)

compressedOffset := decompressBufferSize - compressedSize
copy(decompressBuffer[compressedOffset:], compressBuffer[:compressedSize])

outSize, err := lz4.UncompressInplace(decompressBuffer, compressedSize, len(data), compressedOffset)
if err != nil {
    log.Fatal(err)
}

// Verify round-trip
result := decompressBuffer[:outSize]
if !bytes.Equal(result, data) {
    log.Fatal("Round-trip failed")
}
```

## Memory Benefits

In-place operations can significantly reduce memory usage:

- **Traditional approach**: Requires separate buffers for input, compressed data, and output
- **In-place approach**: Uses a single buffer for each operation
- **Memory savings**: Especially significant for large data where buffer overhead matters

Example memory comparison for 1MB input:
- Traditional: ~3.1MB (input + compressed + output buffers)
- In-place: ~2.1MB (single buffer per operation)
- Savings: ~32% memory reduction

## Important Notes

1. **Buffer Requirements**: Always use the provided buffer size calculation functions to ensure adequate buffer space.

2. **Data Placement**: Input/compressed data must be placed at the correct offset (typically at the end of the buffer).

3. **No Overlap**: The functions include validation to ensure input and output regions don't overlap.

4. **Error Handling**: Always check for errors, especially with incompressible data or insufficient buffer space.

5. **Incompressible Data**: Even data that doesn't compress well will be handled correctly, though with less space savings.

## Testing

Comprehensive tests are included in `lz4_inplace_test.go`:
- Basic functionality tests
- Round-trip tests with various data types
- Buffer size calculation tests
- Error condition tests
- Performance benchmarks
- Edge cases (empty data, incompressible data, etc.)

Run tests with:
```bash
go test -v -run TestInplace
```

Run benchmarks with:
```bash
go test -bench=BenchmarkInplace
```