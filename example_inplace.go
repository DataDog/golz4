package main

import (
	"fmt"
	"log"

	lz4 "github.com/DataDog/golz4"
)

func main() {
	// Example data to compress
	data := []byte("Hello world, this is a test of in-place compression and decompression using liblz4!")

	fmt.Printf("Original data (%d bytes): %s\n", len(data), string(data))

	// Example 1: In-place compression
	fmt.Println("\n=== In-place Compression ===")

	// Calculate buffer size needed for in-place compression
	maxCompressedSize := lz4.CompressBound(data)
	compressBufferSize := lz4.CompressInplaceBufferSize(maxCompressedSize)
	compressBuffer := make([]byte, compressBufferSize)

	fmt.Printf("Max compressed size: %d bytes\n", maxCompressedSize)
	fmt.Printf("In-place compress buffer size: %d bytes\n", compressBufferSize)

	// Copy input data to the end of the buffer (as required by in-place compression)
	inputOffset := compressBufferSize - len(data)
	copy(compressBuffer[inputOffset:], data)
	fmt.Printf("Input data placed at offset: %d\n", inputOffset)

	// Perform in-place compression
	compressedSize, err := lz4.CompressInplace(compressBuffer, len(data), maxCompressedSize, inputOffset)
	if err != nil {
		log.Fatalf("In-place compression failed: %v", err)
	}

	fmt.Printf("Compressed size: %d bytes (%.1f%% of original)\n",
		compressedSize, float64(compressedSize)*100/float64(len(data)))

	// Example 2: In-place decompression
	fmt.Println("\n=== In-place Decompression ===")

	// Calculate buffer size needed for in-place decompression
	decompressedSize := len(data)
	decompressBufferSize := lz4.DecompressInplaceBufferSize(decompressedSize)
	decompressBuffer := make([]byte, decompressBufferSize)

	fmt.Printf("Decompressed size: %d bytes\n", decompressedSize)
	fmt.Printf("In-place decompress buffer size: %d bytes\n", decompressBufferSize)
	fmt.Printf("Decompress margin: %d bytes\n", lz4.DecompressInplaceMargin(compressedSize))

	// Copy compressed data to the end of the buffer (as required by in-place decompression)
	compressedOffset := decompressBufferSize - compressedSize
	copy(decompressBuffer[compressedOffset:], compressBuffer[:compressedSize])
	fmt.Printf("Compressed data placed at offset: %d\n", compressedOffset)

	// Perform in-place decompression
	outSize, err := lz4.UncompressInplace(decompressBuffer, compressedSize, decompressedSize, compressedOffset)
	if err != nil {
		log.Fatalf("In-place decompression failed: %v", err)
	}

	fmt.Printf("Decompressed %d bytes\n", outSize)

	// Verify the result
	decompressed := decompressBuffer[:outSize]
	fmt.Printf("Decompressed data: %s\n", string(decompressed))

	// Check if round-trip was successful
	if string(decompressed) == string(data) {
		fmt.Println("\n✅ In-place round-trip compression/decompression successful!")
	} else {
		fmt.Println("\n❌ In-place round-trip failed!")
	}

	// Example 3: Memory usage comparison
	fmt.Println("\n=== Memory Usage Comparison ===")

	// Traditional approach (separate buffers)
	traditionalCompressed := make([]byte, lz4.CompressBound(data))
	traditionalDecompressed := make([]byte, len(data))
	traditionalMemory := len(data) + len(traditionalCompressed) + len(traditionalDecompressed)

	// In-place approach (single buffer for each operation)
	inplaceMemory := compressBufferSize + decompressBufferSize

	fmt.Printf("Traditional approach memory: %d bytes\n", traditionalMemory)
	fmt.Printf("In-place approach memory: %d bytes\n", inplaceMemory)
	fmt.Printf("Memory saved: %d bytes (%.1f%%)\n",
		traditionalMemory-inplaceMemory,
		float64(traditionalMemory-inplaceMemory)*100/float64(traditionalMemory))
}
