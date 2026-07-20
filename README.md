[![PkgGoDev](https://pkg.go.dev/badge/github.com/tigerwill90/fastcdc/v2?tab=doc)](https://pkg.go.dev/github.com/tigerwill90/fastcdc/v2?tab=doc)
[![Build Status](https://github.com/tigerwill90/fastcdc/actions/workflows/test.yml/badge.svg?branch=main)](https://github.com/tigerwill90/fastcdc/actions/workflows/test.yml)
[![codecov](https://codecov.io/gh/tigerwill90/fastcdc/graph/badge.svg?token=HWX8OHXGAF)](https://codecov.io/gh/tigerwill90/fastcdc)
# FastCDC
This package implements the FastCDC content defined chunking algorithm based on the gear-rolling hash and implements optimizations proposed by Wen Xia et al. in their 2016 paper [FastCDC:
a Fast and Efficient Content-Defined Chunking Approach for Data Deduplication](https://www.usenix.org/system/files/conference/atc16/atc16-paper-xia.pdf).

**Install** (requires Go 1.26+):
````
go get -u github.com/tigerwill90/fastcdc/v2
````

### Objective
This is a fast and efficient pure go implementation of the FastCDC algorithm with a copyleft license. The chunker consumes
any `io.Reader` and yields its chunks through a native Go iterator, with zero allocation in the chunking path. This package
is based on optimizations and variations introduced by [ronomon/deduplication](https://github.com/ronomon/deduplication).

### Example

In this example, the chunker is configured to split the given file into chunk of an average of 32kb.
````go
package main

import (
	"crypto/sha256"
	"fmt"
	"os"

	"github.com/tigerwill90/fastcdc/v2"
)

func main() {
	file, err := os.Open("fixtures/SekienAkashita.jpg")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	c, err := fastcdc.NewChunker(fastcdc.With32kChunks())
	if err != nil {
		panic(err)
	}

	for chunk, err := range c.Chunks(file) {
		if err != nil {
			panic(err)
		}
		// the chunk is only valid for this iteration step, copy it for later use
		fmt.Printf("offset: %d, length: %d, sum: %x\n", chunk.Offset, len(chunk.Data), sha256.Sum256(chunk.Data))
	}
}
````

The chunker can be reused for another stream once the previous iteration is over, and keeps its internal buffer
from one stream to the next.

### Benchmark
Setup: Apple M4 Max, macOS.
````
Benchmark16kChunks-16    88    13343829 ns/op    2514.60 MB/s    0 B/op    0 allocs/op
Benchmark32kChunks-16    90    13147409 ns/op    2552.17 MB/s    0 B/op    0 allocs/op
Benchmark64kChunks-16    91    13188992 ns/op    2544.12 MB/s    0 B/op    0 allocs/op
````

### Upgrading from v1
The chunk size presets changed in v2: `With16k/32k/64kChunks` and the default configuration now use
`min = avg/4, max = avg×8` and therefore produce different chunks than
[v1](https://github.com/tigerwill90/fastcdc/tree/v1.2.2). To reproduce a v1 preset, configure the
chunker explicitly: `WithChunksSize(8192, 16_384, 32_768)` (16k), `WithChunksSize(16_384, 32_768, 65_536)`
(32k) or `WithChunksSize(32_768, 65_536, 131_072)` (64k).

### Invariants
FastCDC will ensure that all chunks meet your minimum and maximum chunk size requirement, except for the last chunk which can
be smaller than the minimum. The chunking is deterministic: identical input with an identical chunk size configuration always
produces identical chunks, whatever the internal buffer size or the read pattern of the reader (a network stream delivering
one byte at a time and an in-memory reader produce the same chunks). Finally, all custom input are validated when creating
the chunker.

### Other implementations
- [ronomon/deduplication](https://github.com/ronomon/deduplication)
- [nlfiedler/fastcdc-rs](https://github.com/nlfiedler/fastcdc-rs)
- [jotfs/fastcdc-go](https://github.com/jotfs/fastcdc-go)
- [iscc/fastcdc-py](https://github.com/iscc/fastcdc-py)

### Authors
[Samuel Aeberhard](https://github.com/isam2k) & [Sylvain Muller](https://github.com/tigerwill90)
