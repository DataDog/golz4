golz4
=====

[![Godoc](http://img.shields.io/badge/godoc-reference-blue.svg?style=flat)](https://godoc.org/github.com/DataDog/golz4) [![license](http://img.shields.io/badge/license-BSD-red.svg?style=flat)](https://raw.githubusercontent.com/DataDog/golz4/master/LICENSE)

Golang interface to LZ4 compression.

Forked from `github.com/cloudflare/golz4` but with significant differences:

* input/output arg order has been swapped to follow Go convention, ie `Compress(in, out)` -> `Compress(out, in)`
* lz4 131 used which fixes [several segfaults](https://github.com/cloudflare/golz4/pull/7)

