golz4
=====

[![Godoc](http://img.shields.io/badge/godoc-reference-blue.svg?style=flat)](https://godoc.org/github.com/DataDog/golz4) [![license](http://img.shields.io/badge/license-BSD-red.svg?style=flat)](https://raw.githubusercontent.com/DataDog/golz4/master/LICENSE)

Golang interface to LZ4 compression.

Forked from `github.com/cloudflare/golz4` but with significant differences:

* input/output arg order has been swapped to follow Go convention, ie `Compress(in, out)` -> `Compress(out, in)`
* builds against the liblz4 that it detects using pkgconfig

Benchmark 
```
BenchmarkCompress-8             	 5000000	       234 ns/op	 183.73 MB/s	       0 B/op	       0 allocs/op
BenchmarkCompressUncompress-8   	20000000	        62.4 ns/op	 688.60 MB/s	       0 B/op	       0 allocs/op
BenchmarkStreamCompress-8       	   50000	     32842 ns/op	2003.41 MB/s	  278537 B/op	       4 allocs/op
BenchmarkStreamUncompress-8     	  500000	      2867 ns/op	22855.34 MB/s	      52 B/op	       2 allocs/op
```
