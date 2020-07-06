package lz4

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// test extended compression/decompression w/ headers

func TestCompressHdrRatio(t *testing.T) {
	input, err := ioutil.ReadFile("sample.txt")
	if err != nil {
		t.Fatal(err)
	}
	output := make([]byte, CompressBoundHdr(input))
	outSize, err := CompressHdr(output, input)
	if err != nil {
		t.Fatal(err)
	}

	outputNoHdr := make([]byte, CompressBoundHdr(input))
	outNoHdrSize, err := Compress(outputNoHdr, input)
	if err != nil {
		t.Fatal(err)
	}
	if outSize != outNoHdrSize+4 {
		t.Fatalf("Compressed output length != expected: %d != %d", outSize, outNoHdrSize+4)
	}
}

func TestCompressionHdr(t *testing.T) {
	input := []byte(strings.Repeat("Hello world, this is quite something", 10))
	output := make([]byte, CompressBoundHdr(input))
	outSize, err := CompressHdr(output, input)
	if err != nil {
		t.Fatalf("Compression failed: %v", err)
	}
	if outSize == 0 {
		t.Fatal("Output buffer is empty.")
	}
	output = output[:outSize]
	decompressed, err := UncompressAllocHdr(nil, output)
	if err != nil {
		t.Fatalf("Decompression failed: %v", err)
	}
	if string(decompressed) != string(input) {
		t.Fatalf("Decompressed output != input: %q != %q", decompressed, input)
	}
}

func TestEmptyCompressionHdr(t *testing.T) {
	input := []byte("")
	output := make([]byte, CompressBoundHdr(input))
	outSize, err := CompressHdr(output, input)
	if err != nil {
		t.Fatalf("Compression failed: %v", err)
	}
	if outSize == 0 {
		t.Fatal("Output buffer is empty.")
	}
	output = output[:outSize]
	decompressed := make([]byte, len(input))
	err = UncompressHdr(decompressed, output)
	if err != nil {
		t.Fatalf("Decompression failed: %v", err)
	}
	if string(decompressed) != string(input) {
		t.Fatalf("Decompressed output != input: %q != %q", decompressed, input)
	}
}

func TestUncompressHdrShort(t *testing.T) {
	// calling Uncompress(Alloc)Hdr with input that is too short should return an error, not panic
	output := make([]byte, 1)
	for i := 0; i < 4; i++ {
		tooShortInput := make([]byte, i)
		out2, err := UncompressAllocHdr(output, tooShortInput)
		if err != errTooShort {
			t.Errorf("UncompressAllocHdr(output, [%d zero bytes]) returned unexpected err=%v",
				len(tooShortInput), err)
		}
		// UncompressAllocHdr should always returns its first argument
		// sadly slice identity is hard; cheat with Sprintf("%p")
		if fmt.Sprintf("%p", out2) != fmt.Sprintf("%p", output) {
			t.Errorf("UncompressAllocHdr([%p], [%d zero bytes]) returned output=%p",
				output, len(tooShortInput), out2)
		}

		err = UncompressHdr(output, tooShortInput)
		if err != errTooShort {
			t.Errorf("UncompressHdr(output, [%d zero bytes]) returned unexpected err=%v",
				len(tooShortInput), err)
		}
	}
}

// test python interoperability

// pymod returns whether or not a python module is importable.  For checking
// whether or not we can test the python lz4 interop
func pymod(module string) bool {
	cmd := exec.Command("python", "-c", fmt.Sprintf("import %s", module))
	err := cmd.Run()
	if err != nil {
		return false
	}
	return cmd.ProcessState.Success()
}

func TestPythonIntegration(t *testing.T) {
	if !pymod("os") {
		t.Errorf("pymod could not find 'os' module")
	}
	if pymod("faojfeiajwofe") {
		t.Errorf("pymod found non-existent junk module")
	}
}

func TestPythonInterop(t *testing.T) {
	pycompat := pymod("lz4")

	if !pycompat {
		t.Log("Warning: not testing python module compat: no module lz4 found")
		t.Skip()
		return
	}

	corpus, err := ioutil.ReadFile("sample.txt")
	if err != nil {
		t.Fatal(err)
	}

	out := make([]byte, CompressBoundHdr(corpus))
	count, err := CompressHdr(out, corpus)
	if err != nil {
		t.Fatal(err)
	}

	out = out[:count]

	dst := "/tmp/lz4test.z"
	err = ioutil.WriteFile(dst, out, 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(dst)

	err = pythonLz4Compat(dst, len(corpus))
	if err != nil {
		t.Fatal(err)
	}
}

// given the original length of an lz4 encoded file, check that the python
// lz4 library returns the correct length.
func pythonLz4Compat(path string, length int) error {
	var out bytes.Buffer
	cmd := exec.Command("python", "-c", fmt.Sprintf(`import lz4; print len(lz4.loads(open("%s", "rb").read()))`, path))
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	output := out.String()
	if err != nil {
		return errors.New(output)
	}
	output = strings.Trim(output, "\n")
	l, err := strconv.Atoi(output)
	if err != nil {
		return err
	}
	if l == length {
		return nil
	}
	return fmt.Errorf("Expected length %d, got %d", length, l)
}
