// Package lz4 implements compression using lz4.c. This is its test
// suite.
//
// Copyright (c) 2013 CloudFlare, Inc.

package lz4

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/quick"
	"time"
)

const sampleFilePath = "./testdata/sample.txt"

var plaintext0 = []byte("jkoedasdcnegzb.,ewqegmovobspjikodecedegds[]")

func failOnError(t *testing.T, msg string, err error) {
	t.Helper()
	if err != nil {
		debug.PrintStack()
		t.Fatalf("%s: %s", msg, err)
	}
}

func TestCompressionRatio(t *testing.T) {
	input, err := ioutil.ReadFile(sampleFilePath)
	if err != nil {
		t.Fatal(err)
	}
	output := make([]byte, CompressBound(input))
	outSize, err := Compress(output, input)
	if err != nil {
		t.Fatal(err)
	}

	maxCompressedSize := 100 * len(input) / 85
	if outSize >= maxCompressedSize {
		t.Fatalf("Compressed output length %d should be smaller than %d", outSize, maxCompressedSize)
	}
}

func TestCompression(t *testing.T) {
	input := []byte(strings.Repeat("Hello world, this is quite something", 10))
	output := make([]byte, CompressBound(input))
	outSize, err := Compress(output, input)
	if err != nil {
		t.Fatalf("Compression failed: %v", err)
	}
	if outSize == 0 {
		t.Fatal("Output buffer is empty.")
	}
	t.Logf("Compressed %v -> %v bytes", len(input), outSize)

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

func TestEmptyCompression(t *testing.T) {
	input := []byte("")
	output := make([]byte, CompressBound(input))
	outSize, err := Compress(output, input)
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

func TestNoCompression(t *testing.T) {
	input := []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	output := make([]byte, CompressBound(input))
	outSize, err := Compress(output, input)
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

func TestCompressionError(t *testing.T) {
	input := []byte(strings.Repeat("Hello world, this is quite something", 10))
	output := make([]byte, 1)
	_, err := Compress(output, input)
	if err == nil {
		t.Fatalf("Compression should have failed but didn't")
	}

	output = make([]byte, 0)
	_, err = Compress(output, input)
	if err == nil {
		t.Fatalf("Compression should have failed but didn't")
	}
}

func TestDecompressionError(t *testing.T) {
	input := []byte(strings.Repeat("Hello world, this is quite something", 10000))
	output := make([]byte, CompressBound(input))
	outSize, err := Compress(output, input)
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

func assert(t *testing.T, b bool) {
	if !b {
		t.Fatalf("assert failed")
	}
}

func TestCompressBound(t *testing.T) {
	var input []byte
	assert(t, CompressBound(input) == 16)

	input = make([]byte, 1)
	assert(t, CompressBound(input) == 17)

	input = make([]byte, 254)
	assert(t, CompressBound(input) == 270)

	input = make([]byte, 255)
	assert(t, CompressBound(input) == 272)

	input = make([]byte, 510)
	assert(t, CompressBound(input) == 528)
}

func TestFuzz(t *testing.T) {
	f := func(input []byte) bool {
		output := make([]byte, CompressBound(input))
		outSize, err := Compress(output, input)
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

func TestSimpleCompressDecompress(t *testing.T) {
	data := bytes.NewBuffer(nil)
	// NOTE: make the buffer bigger than 65k to cover all use cases
	for i := 0; i < 3000; i++ {
		data.WriteString(fmt.Sprintf("%04d-abcdefghijklmnopqrstuvwxyz ", i))
	}
	w := bytes.NewBuffer(nil)
	wc := NewWriter(w)
	defer wc.Close()
	_, err := wc.Write(data.Bytes())
	if err != nil {
		t.Fatalf("Compression of %d bytes of data failed: %s", len(data.Bytes()), err)
	}

	// Decompress
	bufOut := bytes.NewBuffer(nil)
	r := NewReader(w)
	_, err = io.Copy(bufOut, r)
	failOnError(t, "Failed writing to file", err)

	if bufOut.String() != data.String() {
		t.Fatalf("Decompressed output != input: %q != %q", bufOut.String(), data)
	}
	err = r.Close()
	if err != nil {
		t.Fatal(err)
	}
}

func TestWriterCompressDecompressSplits(t *testing.T) {
	// Test random write sizes. This found a bug where we were incorrectly reusing a buffer.
	// Write 3 full streaming blocks split into 4 different chunks. This should exercise various
	// combinations of full block writes and smaller writes
	in := make([]byte, streamingBlockSize*3)
	for i := range in {
		in[i] = byte(i)
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	const numSplitPoints = 3
	for i := 0; i < 2000; i++ {
		w := bytes.NewBuffer(nil)
		wc := NewWriter(w)

		// write as separate randomly-sized blocks
		// Note: The first blocks are likely to be larger than the last blocks; unclear if this matters
		lastIndex := 0
		splits := make([]int, 0, numSplitPoints)
		for j := 0; j < numSplitPoints; j++ {
			bytesRemaining := len(in) - lastIndex
			nextIndex := lastIndex + rng.Intn(bytesRemaining)
			splits = append(splits, nextIndex)
			_, err := wc.Write(in[lastIndex:nextIndex])
			if err != nil {
				t.Fatal(err)
			}

			lastIndex = nextIndex
		}
		_, err := wc.Write(in[lastIndex:])
		if err != nil {
			t.Fatal(err)
		}
		err = wc.Close()
		if err != nil {
			t.Fatal(err)
		}

		// Decompress
		bufOut := bytes.NewBuffer(nil)
		r := NewReader(w)
		_, err = io.Copy(bufOut, r)
		failOnError(t, "Failed writing to file", err)
		if !bytes.Equal(bufOut.Bytes(), in) {
			t.Fatalf("Decompressed output != input; splits=%#v", splits)
		}
		err = r.Close()
		if err != nil {
			t.Fatal("close failed:", err)
		}
	}
}

func TestSimpleCompressDecompressSmallBuffer(t *testing.T) {
	// create test data
	var data strings.Builder
	for i := 0; i < 3000; i++ {
		data.WriteString(fmt.Sprintf("%04d-abcdefghijklmnopqrstuvwxyz ", i))
	}
	dataBuf := bytes.NewBufferString(data.String())

	// Compress and Decompress
	bufOut := bytes.NewBuffer(nil) // out buffer
	// read -> compress -> decompress pipeline
	compressReader := NewCompressReader(dataBuf)
	decompressReader := NewDecompressReader(compressReader)
	_, err := io.Copy(bufOut, decompressReader)
	failOnError(t, "Failed writing to file", err)
	err = compressReader.Close()
	failOnError(t, "Failed closing compressReader", err)
	err = decompressReader.Close()
	failOnError(t, "Failed closing decompressReader", err)

	// assert we got out what we put it
	if bufOut.String() != data.String() {
		t.Fatalf("Decompressed output != input: %q != %q", bufOut.String(), data.String())
	}
}

func TestIOCopyStreamSimpleCompressionDecompression(t *testing.T) {
	inputs, err := os.Open(sampleFilePath)
	if err != nil {
		t.Fatal(err)
	}

	testIOCopy(t, inputs, sampleFilePath)
}

func testIOCopy(t *testing.T, src io.Reader, filename string) {
	fname := filename + "testcom" + ".lz4"
	file, err := os.Create(fname)
	failOnError(t, "Failed creating to file", err)

	writer := NewWriter(file)

	_, err = io.Copy(writer, src)
	failOnError(t, "Failed witting to file", err)

	failOnError(t, "Failed to close compress object", writer.Close())
	stat, err := os.Stat(fname)
	failOnError(t, "Stat failed", err)
	filenameSize, err := os.Stat(filename)
	failOnError(t, "Cannot open file", err)

	t.Logf("Compressed %v -> %v bytes", filenameSize.Size(), stat.Size())

	file.Close()

	defer os.Remove(fname)

	// read from the filec
	fi, err := os.Open(fname)
	failOnError(t, "Failed open file", err)
	defer fi.Close()

	// decompress the file againg
	fnameNew := filename + ".copy"

	fileNew, err := os.Create(fnameNew)
	failOnError(t, "Failed writing to file", err)
	defer fileNew.Close()
	defer os.Remove(fnameNew)

	// Decompress with streaming API
	r := NewReader(fi)

	_, err = io.Copy(fileNew, r)
	failOnError(t, "Failed writing to file", err)

	fileOriginstats, err := os.Stat(filename)
	failOnError(t, "Stat failed", err)
	fiNewStats, err := fileNew.Stat()
	failOnError(t, "Stat failed", err)
	if fileOriginstats.Size() != fiNewStats.Size() {
		t.Fatalf("Not same size files: %d != %d", fileOriginstats.Size(), fiNewStats.Size())

	}
	// just a check to make sure the file contents are the same
	if !checkfilecontentIsSame(t, filename, fnameNew) {
		t.Fatalf("Original VS Compressed file contents not same: %s != %s", filename, fnameNew)

	}

	failOnError(t, "Failed to close decompress object", r.Close())
}

func checkfilecontentIsSame(t *testing.T, f1, f2 string) bool {
	// just a check to make sure the file contents are the same
	bytes1, err := ioutil.ReadFile(f1)
	failOnError(t, "Failed reading to file", err)

	bytes2, err := ioutil.ReadFile(f2)
	failOnError(t, "Failed reading to file", err)

	return bytes.Equal(bytes1, bytes2)
}

type testfilenames struct {
	name     string
	filename string
}

func TestDecompConcurrently(t *testing.T) {
	var tests []testfilenames
	// create decomp test file
	decompFileName := filepath.Join(t.TempDir(), "testcom.lz4")

	src, err := os.Open(sampleFilePath)
	failOnError(t, "Failed opening input", err)
	file, err := os.Create(decompFileName)
	failOnError(t, "Failed creating to file", err)
	writer := NewWriter(file)
	_, err = io.Copy(writer, src)
	failOnError(t, "Failed writing to file", err)

	failOnError(t, "Failed to close compress object", writer.Close())
	inputStat, err := os.Stat(sampleFilePath)
	failOnError(t, "Stat failed", err)
	outputStat, err := os.Stat(decompFileName)
	failOnError(t, "Stat failed", err)

	t.Logf("Compressed %v -> %v bytes", inputStat.Size(), outputStat.Size())
	if !(outputStat.Size() < inputStat.Size()) {
		t.Errorf("Compressed size %d must be less than input size %d",
			outputStat.Size(), inputStat.Size())
	}

	file.Close()

	// run 100 times
	for i := 0; i < 100; i++ {
		tmp := testfilenames{
			name: "test" + strconv.Itoa(i),

			filename: "sample.txttestcom.result" + strconv.Itoa(i),
		}
		tests = append(tests, tmp)
	}

	// start goroutines to check decompressing in parallel
	wg := &sync.WaitGroup{}
	for _, tc := range tests {
		tc := tc
		wg.Add(1)
		go func() {
			defer wg.Done()
			IOCopyDecompressionwithName(t, tc.filename, sampleFilePath, decompFileName)
		}()
	}
	wg.Wait()
}

func IOCopyDecompressionwithName(t *testing.T, fileoutcomename string, originalfileName string, decompfilename string) {
	// read from the file
	fi, err := os.Open(decompfilename)
	failOnError(t, "Failed open file", err)
	defer fi.Close()

	fileNew, err := os.Create(fileoutcomename)
	failOnError(t, "Failed writing to file", err)
	defer fileNew.Close()

	// Decompress with streaming API
	r := NewReader(fi)
	_, err = io.Copy(fileNew, r)
	failOnError(t, "Failed writing to file", err)

	if !checkfilecontentIsSame(t, originalfileName, fileoutcomename) {
		info1, _ := os.Stat(originalfileName)
		info2, _ := os.Stat(fileoutcomename)
		t.Fatalf("%s VS %s contents not same, size: %d VS %d", originalfileName, fileoutcomename, info1.Size(), info2.Size())

	}
	err = r.Close()
	failOnError(t, "Failed closing reader", err)

	fileNew.Close()
	os.Remove(fileoutcomename)
}

func TestContinueCompress(t *testing.T) {
	payload := []byte("Hello World!")
	repeat := 100

	var intermediate bytes.Buffer
	w := NewWriter(&intermediate)
	for i := 0; i < repeat; i++ {
		_, err := w.Write(payload)
		failOnError(t, "Failed writing to compress object", err)
	}
	failOnError(t, "Failed closing writer", w.Close())

	// Decompress
	r := NewReader(&intermediate)
	dst := make([]byte, len(payload))
	for i := 0; i < repeat; i++ {
		n, err := r.Read(dst)
		failOnError(t, "Failed to decompress", err)
		if n != len(payload) {
			t.Fatalf("Did not read enough bytes: %v != %v", n, len(payload))
		}
		if string(dst) != string(payload) {
			t.Fatalf("Did not read the same %s != %s", string(dst), string(payload))
		}
	}
	// Check EOF
	n, err := r.Read(dst)
	if err != io.EOF {
		t.Fatalf("Error should have been EOF, was %s instead: (%v bytes read: %s)", err, n, dst[:n])
	}
	failOnError(t, "Failed to close decompress object", r.Close())

}

func TestStreamingFuzz(t *testing.T) {
	f := func(input []byte) bool {
		var w bytes.Buffer
		writer := NewWriter(&w)
		_, err := writer.Write(input)
		failOnError(t, "Failed writing to compress object", err)
		failOnError(t, "Failed to close compress object", writer.Close())

		// Decompress
		r := NewReader(&w)
		dst := make([]byte, len(input))
		n, err := r.Read(dst)

		failOnError(t, "Failed Read", err)

		dst = dst[:n]
		if string(input) != string(dst) { // Only print if we can print
			if len(input) < 100 && len(dst) < 100 {
				t.Fatalf("Cannot compress and decompress: %s != %s", input, dst)
			} else {
				t.Fatalf("Cannot compress and decompress (lengths: %v bytes & %v bytes)", len(input), len(dst))
			}
		}
		// Check EOF
		n, err = r.Read(dst)
		if err != io.EOF && len(dst) > 0 { // If we want 0 bytes, that should work
			t.Fatalf("Error should have been EOF, was %s instead: (%v bytes read: %s)", err, n, dst[:n])
		}
		failOnError(t, "Failed to close decompress object", r.Close())
		return true
	}

	conf := &quick.Config{MaxCount: 100}
	if testing.Short() {
		conf.MaxCount = 1000
	}
	if err := quick.Check(f, conf); err != nil {
		t.Fatal(err)
	}
}

func TestCompressReaderFuzz(t *testing.T) {
	f := func(input []byte) bool {
		inputBuf := bytes.NewBuffer(input)
		cr := NewCompressReader(inputBuf)
		w := bytes.NewBuffer(nil)
		_, err := io.Copy(w, cr)
		failOnError(t, "Failed to compress and read data", err)
		err = cr.Close()
		failOnError(t, "Failed to close", err)

		// Decompress
		r := NewDecompressReader(w)
		dst := bytes.NewBuffer(nil)
		n, err := io.Copy(dst, r)
		failOnError(t, "Failed Read", err)

		if int(n) != len(input) {
			t.Fatalf("Decompress result not equal to original input size: %d != %d", n, len(input))
		}

		if string(input) != dst.String() { // Only print if we can print
			if len(input) < 100 && dst.Len() < 100 {
				t.Fatalf("Cannot compress and decompress: %s != %s", string(input), dst.String())
			} else {
				t.Fatalf("Cannot compress and decompress (lengths: %v bytes & %v bytes)", len(input), dst.Len())
			}
		}
		// Check EOF
		nend, err := r.Read(make([]byte, hugeStreamingBlockSize))
		if nend != 0 {
			t.Fatalf("Error should have read 0 bytes, instead was: %d", nend)
		}
		if err != io.EOF { // If we want 0 bytes, that should work
			t.Fatalf("Error should have been EOF, instead was: %s", err)
		}
		failOnError(t, "Failed to close decompress object", r.Close())
		return true
	}

	conf := &quick.Config{MaxCount: 100}
	if testing.Short() {
		conf.MaxCount = 1000
	}
	if err := quick.Check(f, conf); err != nil {
		t.Fatal(err)
	}
}

func TestReaderBadData(t *testing.T) {
	// Decompressing this previously caused a panic because Reader returned a negative value
	badInput := []byte{0xa, 0x2, 0x0, 0x0, 0xff, 0xf1, 0x0, 0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9, 0xa, 0xb, 0xc, 0xd, 0xe, 0xf, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f, 0x20, 0x21, 0x22, 0x23, 0x24, 0x25, 0x26, 0x27, 0x28, 0x29, 0x2a, 0x2b, 0x2c, 0x2d, 0x2e, 0x2f, 0x30, 0x31, 0x32, 0x33, 0x34, 0x35, 0x36, 0x37, 0x38, 0x39, 0x3a, 0x3b, 0x3c, 0x3d, 0x3e, 0x3f, 0x40, 0x41, 0x42, 0x43, 0x44, 0x45, 0x46, 0x47, 0x48, 0x49, 0x4a, 0x4b, 0x4c, 0x4d, 0x4e, 0x4f, 0x50, 0x51, 0x52, 0x53, 0x54, 0x55, 0x56, 0x57, 0x58, 0x59, 0x5a, 0x5b, 0x5c, 0x5d, 0x5e, 0x5f, 0x60, 0x61, 0x62, 0x63, 0x64, 0x65, 0x66, 0x67, 0x68, 0x69, 0x6a, 0x6b, 0x6c, 0x6d, 0x6e, 0x6f, 0x70, 0x71, 0x72, 0x73, 0x74, 0x75, 0x76, 0x77, 0x78, 0x79, 0x7a, 0x7b, 0x7c, 0x7d, 0x7e, 0x7f, 0x80, 0x81, 0x82, 0x83, 0x84, 0x85, 0x86, 0x87, 0x88, 0x89, 0x8a, 0x8b, 0x8c, 0x8d, 0x8e, 0x8f, 0x90, 0x91, 0x92, 0x93, 0x94, 0x95, 0x96, 0x97, 0x98, 0x99, 0x9a, 0x9b, 0x9c, 0x9d, 0x9e, 0x9f, 0xa0, 0xa1, 0xa2, 0xa3, 0xa4, 0xa5, 0xa6, 0xa7, 0xa8, 0xa9, 0xaa, 0xab, 0xac, 0xad, 0xae, 0xaf, 0xb0, 0xb1, 0xb2, 0xb3, 0xb4, 0xb5, 0xb6, 0xb7, 0xb8, 0xb9, 0xba, 0xbb, 0xbc, 0xbd, 0xbe, 0xbf, 0xc0, 0xc1, 0xc2, 0xc3, 0xc4, 0xc5, 0xc6, 0xc7, 0xc8, 0xc9, 0xca, 0xcb, 0xcc, 0xcd, 0xce, 0xcf, 0xd0, 0xd1, 0xd2, 0xd3, 0xd4, 0xd5, 0xd6, 0xd7, 0xd8, 0xd9, 0xda, 0xdb, 0xdc, 0xdd, 0xde, 0xdf, 0xe0, 0xe1, 0xe2, 0xe3, 0xe4, 0xe5, 0xe6, 0xe7, 0xe8, 0xe9, 0xea, 0xeb, 0xec, 0xed, 0xee, 0xef, 0xf0, 0xf1, 0xf2, 0xf3, 0xf4, 0xf5, 0xf6, 0xf7, 0xf8, 0xf9, 0xfa, 0xfb, 0xfc, 0xfd, 0xfe, 0xff, 0x0, 0x1, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xe7, 0x50, 0xfb, 0xfc, 0xfd, 0xfe, 0xff, 0xe3, 0x0, 0x0, 0x0, 0xf, 0x0, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xb5, 0x50, 0xef, 0xf0, 0xf1, 0xf2, 0xf3, 0xb4, 0x0, 0x0, 0x0, 0xf, 0x0, 0xd9, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x26, 0x50, 0x83, 0x84, 0x85, 0x86, 0x87, 0x91, 0x0, 0x0, 0x0, 0xff, 0x5d, 0x88, 0x89, 0x8a, 0x8b, 0x8c, 0x8d, 0x8e, 0x8f, 0x90}
	r := NewReader(bytes.NewReader(badInput))
	output := &bytes.Buffer{}
	_, err := io.Copy(output, r)
	if err == nil || !strings.Contains(err.Error(), "error decompressing") {
		t.Error("expected error decompressing:", err)
	}
	err = r.Close()
	if err != nil {
		t.Fatal("close failed:", err)
	}
}

func BenchmarkCompress(b *testing.B) {
	b.ReportAllocs()
	dst := make([]byte, CompressBound(plaintext0))
	b.SetBytes(int64(len(plaintext0)))
	for i := 0; i < b.N; i++ {
		_, err := Compress(dst, plaintext0)
		if err != nil {
			b.Errorf("Compress error: %v", err)
		}
	}
}

func BenchmarkCompressUncompress(b *testing.B) {
	b.ReportAllocs()
	compressed := make([]byte, CompressBound(plaintext0))
	n, err := Compress(compressed, plaintext0)
	if err != nil {
		b.Errorf("Compress error: %v", err)
	}
	compressed = compressed[:n]

	dst := make([]byte, len(plaintext0))
	b.SetBytes(int64(len(plaintext0)))
	for i := 0; i < b.N; i++ {
		_, err := Uncompress(dst, compressed)
		if err != nil {
			b.Errorf("Uncompress error: %v", err)
		}
	}
}

type NullReader struct {
}

func (z NullReader) Read(p []byte) (n int, err error) {
	return len(p), nil
}

var Null NullReader

func BenchmarkStreamCompress(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		w := NewWriter(ioutil.Discard)
		if _, err := io.Copy(w, io.LimitReader(Null, 10*1024*1024)); err != nil {
			b.Fatalf("Failed writing to compress object: %s", err)
		}
		b.SetBytes(10 * 1024 * 1024)
		err := w.Close()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStreamCompressReader(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		r := NewCompressReader(io.LimitReader(Null, 10*1024*1024))
		if _, err := io.Copy(ioutil.Discard, r); err != nil {
			b.Fatalf("Failed writing to compress object: %s", err)
		}
		b.SetBytes(10 * 1024 * 1024)
		err := r.Close()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDeprecatedStreamUncompress(b *testing.B) {
	var compressedBuffer bytes.Buffer
	w := NewWriter(&compressedBuffer)
	if _, err := io.Copy(w, io.LimitReader(Null, 10*1024*1024)); err != nil {
		b.Fatalf("Failed writing to NewWriter: %s", err)
	}
	err := w.Close()
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := NewReader(bytes.NewReader(compressedBuffer.Bytes()))
		if _, err := io.Copy(ioutil.Discard, r); err != nil {
			b.Fatalf("Failed writing to compress object: %s", err)
		}
		b.SetBytes(10 * 1024 * 1024)
		err = r.Close()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStreamDecompressReader(b *testing.B) {
	var compressedBuffer bytes.Buffer
	r := NewCompressReader(io.LimitReader(Null, 10*1024*1024))
	if _, err := io.Copy(&compressedBuffer, r); err != nil {
		b.Fatalf("Failed writing to compress object: %s", err)
	}
	err := r.Close()
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r := NewDecompressReader(bytes.NewReader(compressedBuffer.Bytes()))
		if _, err := io.Copy(ioutil.Discard, r); err != nil {
			b.Fatalf("Failed writing to compress object: %s", err)
		}
		b.SetBytes(10 * 1024 * 1024)
		err = r.Close()
		if err != nil {
			b.Fatal(err)
		}
	}
}
