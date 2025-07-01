package lz4

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math/rand"
	"strings"
	"testing"
)

// Test basic in-place decompression functionality
func TestUncompressInplace(t *testing.T) {
	// Test data
	input := []byte(strings.Repeat("Hello world, this is quite something", 10))

	// First compress normally to get compressed data
	compressed := make([]byte, CompressBound(input))
	compressedSize, err := Compress(compressed, input)
	if err != nil {
		t.Fatalf("Compression failed: %v", err)
	}
	compressed = compressed[:compressedSize]

	// Create buffer for in-place decompression
	decompressedSize := len(input)
	bufferSize := DecompressInplaceBufferSize(decompressedSize)

	// Ensure buffer is large enough to avoid overlap
	minRequired := decompressedSize + compressedSize
	if bufferSize < minRequired {
		bufferSize = minRequired + 100 // Add extra margin
	}

	buffer := make([]byte, bufferSize)

	// Copy compressed data to the end of the buffer
	compressedOffset := bufferSize - compressedSize
	copy(buffer[compressedOffset:], compressed)

	// Perform in-place decompression
	outSize, err := UncompressInplace(buffer, compressedSize, decompressedSize, compressedOffset)
	if err != nil {
		t.Fatalf("In-place decompression failed: %v", err)
	}

	if outSize != decompressedSize {
		t.Fatalf("Expected decompressed size %d, got %d", decompressedSize, outSize)
	}

	// Check that decompressed data matches original
	decompressed := buffer[:decompressedSize]
	if !bytes.Equal(decompressed, input) {
		t.Fatalf("Decompressed data doesn't match original")
	}
}

// Test basic in-place compression functionality
func TestCompressInplace(t *testing.T) {
	// Test data
	input := []byte(strings.Repeat("Hello world, this is quite something", 10))

	// Create buffer for in-place compression
	maxCompressedSize := CompressBound(input)
	bufferSize := CompressInplaceBufferSize(maxCompressedSize)
	buffer := make([]byte, bufferSize)

	// Copy input data to the end of the buffer
	inputOffset := bufferSize - len(input)
	copy(buffer[inputOffset:], input)

	// Perform in-place compression
	compressedSize, err := CompressInplace(buffer, len(input), maxCompressedSize, inputOffset)
	if err != nil {
		t.Fatalf("In-place compression failed: %v", err)
	}

	if compressedSize <= 0 {
		t.Fatalf("Expected positive compressed size, got %d", compressedSize)
	}

	// Verify by decompressing normally
	compressed := buffer[:compressedSize]
	decompressed := make([]byte, len(input))
	_, err = Uncompress(decompressed, compressed)
	if err != nil {
		t.Fatalf("Decompression of in-place compressed data failed: %v", err)
	}

	// Check that decompressed data matches original
	if !bytes.Equal(decompressed, input) {
		t.Fatalf("Decompressed data doesn't match original")
	}
}

// Test round-trip in-place compression and decompression
func TestInplaceRoundTrip(t *testing.T) {
	testCases := [][]byte{
		[]byte("Hello world"),
		[]byte(strings.Repeat("A", 1000)),
		[]byte(strings.Repeat("Hello world, this is quite something", 50)),
		plaintext0,
	}

	for i, input := range testCases {
		t.Run(fmt.Sprintf("case_%d", i), func(t *testing.T) {
			// Step 1: In-place compression
			maxCompressedSize := CompressBound(input)
			compressBufferSize := CompressInplaceBufferSize(maxCompressedSize)
			compressBuffer := make([]byte, compressBufferSize)

			// Copy input to end of buffer
			inputOffset := compressBufferSize - len(input)
			copy(compressBuffer[inputOffset:], input)

			// Compress in-place
			compressedSize, err := CompressInplace(compressBuffer, len(input), maxCompressedSize, inputOffset)
			if err != nil {
				t.Fatalf("In-place compression failed: %v", err)
			}

			// Step 2: In-place decompression
			decompressedSize := len(input)
			decompressBufferSize := DecompressInplaceBufferSize(decompressedSize)

			// Ensure buffer is large enough to avoid overlap
			minRequired := decompressedSize + compressedSize
			if decompressBufferSize < minRequired {
				decompressBufferSize = minRequired + 100 // Add extra margin
			}

			decompressBuffer := make([]byte, decompressBufferSize)

			// Copy compressed data to end of buffer
			compressedOffset := decompressBufferSize - compressedSize
			copy(decompressBuffer[compressedOffset:], compressBuffer[:compressedSize])

			// Decompress in-place
			outSize, err := UncompressInplace(decompressBuffer, compressedSize, decompressedSize, compressedOffset)
			if err != nil {
				t.Fatalf("In-place decompression failed: %v", err)
			}

			if outSize != decompressedSize {
				t.Fatalf("Expected decompressed size %d, got %d", decompressedSize, outSize)
			}

			// Verify data integrity
			decompressed := decompressBuffer[:decompressedSize]
			if !bytes.Equal(decompressed, input) {
				t.Fatalf("Round-trip failed: decompressed data doesn't match original")
			}
		})
	}
}

// Test in-place buffer size calculations
func TestInplaceBufferSizes(t *testing.T) {
	testSizes := []int{0, 1, 100, 1000, 10000, 100000}

	for _, size := range testSizes {
		t.Run(fmt.Sprintf("size_%d", size), func(t *testing.T) {
			// Test decompression buffer size calculation
			margin := DecompressInplaceMargin(size)
			bufferSize := DecompressInplaceBufferSize(size)

			if margin < 32 {
				t.Errorf("Expected margin >= 32, got %d", margin)
			}
			if bufferSize != size+margin {
				t.Errorf("Expected buffer size %d, got %d", size+margin, bufferSize)
			}

			// Test compression buffer size calculation
			compressBound := CompressBound(make([]byte, size))
			compressBufferSize := CompressInplaceBufferSize(compressBound)

			if compressBufferSize != compressBound+CompressInplaceMargin {
				t.Errorf("Expected compress buffer size %d, got %d", compressBound+CompressInplaceMargin, compressBufferSize)
			}
		})
	}
}

// Test error cases for in-place operations
func TestInplaceErrors(t *testing.T) {
	t.Run("UncompressInplace_buffer_too_small", func(t *testing.T) {
		buffer := make([]byte, 10) // Too small
		_, err := UncompressInplace(buffer, 5, 10, 0)
		if err == nil {
			t.Fatal("Expected error for buffer too small")
		}
	})

	t.Run("UncompressInplace_compressed_data_exceeds_bounds", func(t *testing.T) {
		buffer := make([]byte, 100)
		_, err := UncompressInplace(buffer, 50, 10, 80) // 80 + 50 > 100
		if err == nil {
			t.Fatal("Expected error for compressed data exceeding bounds")
		}
	})

	t.Run("UncompressInplace_decompressed_overlaps_compressed", func(t *testing.T) {
		buffer := make([]byte, 100)
		_, err := UncompressInplace(buffer, 10, 50, 40) // 50 > 40, would overlap
		if err == nil {
			t.Fatal("Expected error for overlapping data")
		}
	})

	t.Run("CompressInplace_buffer_too_small", func(t *testing.T) {
		buffer := make([]byte, 10) // Too small
		_, err := CompressInplace(buffer, 5, 20, 5)
		if err == nil {
			t.Fatal("Expected error for buffer too small")
		}
	})

	t.Run("CompressInplace_input_exceeds_bounds", func(t *testing.T) {
		buffer := make([]byte, 100)
		_, err := CompressInplace(buffer, 50, 20, 80) // 80 + 50 > 100
		if err == nil {
			t.Fatal("Expected error for input exceeding bounds")
		}
	})

	t.Run("CompressInplace_compressed_overlaps_input", func(t *testing.T) {
		buffer := make([]byte, 100)
		_, err := CompressInplace(buffer, 10, 50, 40) // 50 > 40, would overlap
		if err == nil {
			t.Fatal("Expected error for overlapping data")
		}
	})
}

// Test in-place operations with sample file
func TestInplaceWithSampleFile(t *testing.T) {
	input, err := ioutil.ReadFile(sampleFilePath)
	if err != nil {
		t.Fatal(err)
	}

	// Test in-place compression
	maxCompressedSize := CompressBound(input)
	compressBufferSize := CompressInplaceBufferSize(maxCompressedSize)
	compressBuffer := make([]byte, compressBufferSize)

	inputOffset := compressBufferSize - len(input)
	copy(compressBuffer[inputOffset:], input)

	compressedSize, err := CompressInplace(compressBuffer, len(input), maxCompressedSize, inputOffset)
	if err != nil {
		t.Fatalf("In-place compression failed: %v", err)
	}

	// Test in-place decompression
	decompressedSize := len(input)
	decompressBufferSize := DecompressInplaceBufferSize(decompressedSize)

	// Ensure buffer is large enough to avoid overlap
	minRequired := decompressedSize + compressedSize
	if decompressBufferSize < minRequired {
		decompressBufferSize = minRequired + 100 // Add extra margin
	}

	decompressBuffer := make([]byte, decompressBufferSize)

	compressedOffset := decompressBufferSize - compressedSize
	copy(decompressBuffer[compressedOffset:], compressBuffer[:compressedSize])

	outSize, err := UncompressInplace(decompressBuffer, compressedSize, decompressedSize, compressedOffset)
	if err != nil {
		t.Fatalf("In-place decompression failed: %v", err)
	}

	if outSize != decompressedSize {
		t.Fatalf("Expected decompressed size %d, got %d", decompressedSize, outSize)
	}

	// Verify data integrity
	decompressed := decompressBuffer[:decompressedSize]
	if !bytes.Equal(decompressed, input) {
		t.Fatalf("Sample file round-trip failed")
	}
}

// Test in-place operations with incompressible data
func TestInplaceIncompressibleData(t *testing.T) {
	// Generate random data that should be incompressible
	input := make([]byte, 1000)
	for i := range input {
		input[i] = byte(rand.Intn(256))
	}

	// Try in-place compression
	maxCompressedSize := CompressBound(input)
	compressBufferSize := CompressInplaceBufferSize(maxCompressedSize)
	compressBuffer := make([]byte, compressBufferSize)

	inputOffset := compressBufferSize - len(input)
	copy(compressBuffer[inputOffset:], input)

	compressedSize, err := CompressInplace(compressBuffer, len(input), maxCompressedSize, inputOffset)
	if err != nil {
		t.Fatalf("In-place compression failed: %v", err)
	}

	// Even incompressible data should compress to something (with header overhead)
	if compressedSize <= 0 {
		t.Fatalf("Expected positive compressed size, got %d", compressedSize)
	}

	// Test decompression works
	decompressedSize := len(input)
	decompressBufferSize := DecompressInplaceBufferSize(decompressedSize)

	// Ensure buffer is large enough to avoid overlap
	minRequired := decompressedSize + compressedSize
	if decompressBufferSize < minRequired {
		decompressBufferSize = minRequired + 100 // Add extra margin
	}

	decompressBuffer := make([]byte, decompressBufferSize)

	compressedOffset := decompressBufferSize - compressedSize
	copy(decompressBuffer[compressedOffset:], compressBuffer[:compressedSize])

	outSize, err := UncompressInplace(decompressBuffer, compressedSize, decompressedSize, compressedOffset)
	if err != nil {
		t.Fatalf("In-place decompression failed: %v", err)
	}

	if outSize != decompressedSize {
		t.Fatalf("Expected decompressed size %d, got %d", decompressedSize, outSize)
	}

	// Verify data integrity
	decompressed := decompressBuffer[:decompressedSize]
	if !bytes.Equal(decompressed, input) {
		t.Fatalf("Incompressible data round-trip failed")
	}
}

// Benchmark in-place compression
func BenchmarkCompressInplace(b *testing.B) {
	input, err := ioutil.ReadFile(sampleFilePath)
	if err != nil {
		b.Fatal(err)
	}

	maxCompressedSize := CompressBound(input)
	bufferSize := CompressInplaceBufferSize(maxCompressedSize)
	buffer := make([]byte, bufferSize)
	inputOffset := bufferSize - len(input)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		copy(buffer[inputOffset:], input)
		_, err := CompressInplace(buffer, len(input), maxCompressedSize, inputOffset)
		if err != nil {
			b.Fatal(err)
		}
		b.SetBytes(int64(len(input)))
	}
}

// Benchmark in-place decompression
func BenchmarkUncompressInplace(b *testing.B) {
	input, err := ioutil.ReadFile(sampleFilePath)
	if err != nil {
		b.Fatal(err)
	}

	// Pre-compress the data
	compressed := make([]byte, CompressBound(input))
	compressedSize, err := Compress(compressed, input)
	if err != nil {
		b.Fatal(err)
	}
	compressed = compressed[:compressedSize]

	decompressedSize := len(input)
	bufferSize := DecompressInplaceBufferSize(decompressedSize)
	buffer := make([]byte, bufferSize)
	compressedOffset := bufferSize - compressedSize

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		copy(buffer[compressedOffset:], compressed)
		_, err := UncompressInplace(buffer, compressedSize, decompressedSize, compressedOffset)
		if err != nil {
			b.Fatal(err)
		}
		b.SetBytes(int64(len(input)))
	}
}
