// Package lz4 implements compression using lz4.c. This is its test
// suite.
//
// Copyright (c) 2013 CloudFlare, Inc.

package lz4

import (
	"io/ioutil"
	"strings"
	"testing"
	"testing/quick"
)

func TestCompressionHCRatio(t *testing.T) {
	input, err := ioutil.ReadFile(sampleFilePath)
	if err != nil {
		t.Fatal(err)
	}
	output := make([]byte, CompressBound(input))
	outSize, err := CompressHC(output, input)
	if err != nil {
		t.Fatal(err)
	}

	// Should be at most 85% of the input size
	maxSize := 85 * len(input) / 100

	if outSize > maxSize {
		t.Fatalf("HC Compressed output length should be at most 85%% of input size: input=%d, compressed=%d, maxExpected=%d",
			len(input), outSize, maxSize)
	}
}

func TestCompressionHCLevels(t *testing.T) {
	input, err := ioutil.ReadFile(sampleFilePath)
	if err != nil {
		t.Fatal(err)
	}

	// Should be at most 85% of the input size
	previousCompressedSize := 85 * len(input) / 100

	// NOTE: lvl == 0 means auto, 1 worst, 16 best
	for lvl := 1; lvl <= 16; lvl++ {
		output := make([]byte, CompressBound(input))
		outSize, err := CompressHCLevel(output, input, lvl)
		if err != nil {
			t.Fatal(err)
		}

		if outSize > previousCompressedSize {
			t.Errorf("HC level %d should lead to a better or equal compression than HC level %d (previousSize=%d, currentSize=%d)",
				lvl, lvl-1, previousCompressedSize, outSize)
		}
		previousCompressedSize = outSize
	}
}

func TestCompressionHC(t *testing.T) {
	input := []byte(strings.Repeat("Hello world, this is quite something", 10))
	output := make([]byte, CompressBound(input))
	outSize, err := CompressHC(output, input)
	if err != nil {
		t.Fatalf("Compression failed: %v", err)
	}
	if outSize == 0 {
		t.Fatal("Output buffer is empty.")
	}
	output = output[:outSize]
	decompressed := make([]byte, len(input))
	_, err = Uncompress(decompressed, output)
	if err != nil {
		t.Fatalf("Decompression failed: %v", err)
	}
	if string(decompressed) != string(input) {
		t.Fatalf("Decompressed output != input: %q != %q", decompressed, input)
	}
}

func TestEmptyCompressionHC(t *testing.T) {
	input := []byte("")
	output := make([]byte, CompressBound(input))

	outSize, err := CompressHC(output, input)
	if err != nil {
		t.Fatalf("Compression failed: %v", err)
	}
	if outSize == 0 {
		t.Fatal("Output buffer is empty.")
	}
	output = output[:outSize]
	decompressed := make([]byte, len(input))
	_, err = Uncompress(decompressed, output)
	if err != nil {
		t.Fatalf("Decompression failed: %v", err)
	}
	if string(decompressed) != string(input) {
		t.Fatalf("Decompressed output != input: %q != %q", decompressed, input)
	}
}

func TestNoCompressionHC(t *testing.T) {
	input := []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	output := make([]byte, CompressBound(input))
	outSize, err := CompressHC(output, input)
	if err != nil {
		t.Fatalf("Compression failed: %v", err)
	}
	if outSize == 0 {
		t.Fatal("Output buffer is empty.")
	}
	output = output[:outSize]
	decompressed := make([]byte, len(input))
	_, err = Uncompress(decompressed, output)
	if err != nil {
		t.Fatalf("Decompression failed: %v", err)
	}
	if string(decompressed) != string(input) {
		t.Fatalf("Decompressed output != input: %q != %q", decompressed, input)
	}
}

func TestCompressionErrorHC(t *testing.T) {
	input := []byte(strings.Repeat("Hello world, this is quite something", 10))
	var output []byte
	outSize, err := CompressHC(output, input)

	if outSize != 0 {
		t.Fatalf("%d", outSize)
	}

	if err == nil {
		t.Fatalf("Compression should have failed but didn't")
	}

	output = make([]byte, 1)
	_, err = CompressHC(output, input)
	if err == nil {
		t.Fatalf("Compression should have failed but didn't")
	}
}

func TestDecompressionErrorHC(t *testing.T) {
	input := []byte(strings.Repeat("Hello world, this is quite something", 10))
	output := make([]byte, CompressBound(input))
	outSize, err := CompressHC(output, input)
	if err != nil {
		t.Fatalf("Compression failed: %v", err)
	}
	if outSize == 0 {
		t.Fatal("Output buffer is empty.")
	}
	output = output[:outSize]
	decompressed := make([]byte, len(input)-1)
	_, err = Uncompress(decompressed, output)
	if err == nil {
		t.Fatalf("Decompression should have failed")
	}

	decompressed = make([]byte, 1)
	_, err = Uncompress(decompressed, output)
	if err == nil {
		t.Fatalf("Decompression should have failed")
	}

	decompressed = make([]byte, 0)
	_, err = Uncompress(decompressed, output)
	if err == nil {
		t.Fatalf("Decompression should have failed")
	}
}

func TestFuzzHC(t *testing.T) {
	f := func(input []byte) bool {
		output := make([]byte, CompressBound(input))
		outSize, err := CompressHC(output, input)
		if err != nil {
			t.Fatalf("Compression failed: %v", err)
		}
		if outSize == 0 {
			t.Fatal("Output buffer is empty.")
		}
		output = output[:outSize]
		decompressed := make([]byte, len(input))
		_, err = Uncompress(decompressed, output)
		if err != nil {
			t.Fatalf("Decompression failed: %v", err)
		}
		if string(decompressed) != string(input) {
			t.Fatalf("Decompressed output != input: %q != %q", decompressed, input)
		}

		return true
	}

	conf := &quick.Config{MaxCount: 20000}
	if testing.Short() {
		conf.MaxCount = 1000
	}
	if err := quick.Check(f, conf); err != nil {
		t.Fatal(err)
	}
}
