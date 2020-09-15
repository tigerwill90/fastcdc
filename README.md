[![PkgGoDev](https://pkg.go.dev/badge/github.com/tigerwill90/fastcdc?tab=doc)](https://pkg.go.dev/github.com/tigerwill90/fastcdc?tab=doc)
[![Build Status](https://travis-ci.org/tigerwill90/fastcdc.svg?branch=master)](https://travis-ci.org/tigerwill90/fastcdc)
[![codecov](https://codecov.io/gh/tigerwill90/fastcdc/branch/master/graph/badge.svg)](https://codecov.io/gh/tigerwill90/fastcdc)
[![Go Report Card](https://goreportcard.com/badge/github.com/tigerwill90/fastcdc)](https://goreportcard.com/report/github.com/tigerwill90/fastcdc)
# FastCDC
This package implements the FastCDC content defined chunking algorithm based on the gear-rolling hash and implements optimizations proposed by Wen Xia et al. in their 2016 paper [FastCDC:
a Fast and Efficient Content-Defined Chunking Approach for Data Deduplication](https://www.usenix.org/system/files/conference/atc16/atc16-paper-xia.pdf).

**Install:**
````
go get -u github.com/tigerwill90/fastcdc
````

### Objective
For another project, I need to deduplicate "on the fly" a stream of file served from a plugin through grpc. I struggled to find a chunker package
with this capability, so a friend and I developed our own. This is a pure go implementation of the FastCDC algorithm with a copyleft license. 
The interface differs significantly from other chunker that I know. It's designed to be easy to use, especially in streaming fashion.
This package is based on optimizations and variations introduce by [ronomon/deduplication](https://github.com/ronomon/deduplication). 

### Example

In this example, the chunker is configured to split the given file into chunk of an average of 32kb.
````go
package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/tigerwill90/fastcdc"
	"os"
)

func main() {
	file, err := os.Open("fixtures/SekienAkashita.jpg")
	handleError(err)
	defer file.Close()

	chunker, err := fastcdc.NewChunker(context.Background(), fastcdc.With32kChunks())
	handleError(err)

	err = chunker.Split(file, func(offset, length uint, chunk []byte) error {
		// the chunk is only valid in the callback, copy it for later use
		fmt.Printf("offset: %d, length: %d, sum: %x\n", offset, length, sha256.Sum256(chunk))
		return nil
	})
	handleError(err)

	err = chunker.Finalize(func(offset, length uint, chunk []byte) error {
		// the chunk is only valid in the callback, copy it for later use
		fmt.Printf("offset: %d, length: %d, sum: %x\n", offset, length, sha256.Sum256(chunk))
		return nil
	})
	handleError(err)
}
````

Now let's say you want to process a file stream part by part. We configure the chunker in stream mode and keep the
same options as before. For the sake of simplicity, we simulate the file stream by creating 
smaller part before splitting them into chunk of 32kb average size.
````go
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/tigerwill90/fastcdc"
	"io"
	"os"
)

func main() {
	file, err := os.Open("fixtures/SekienAkashita.jpg")
	handleError(err)
	defer file.Close()

	chunker, err := fastcdc.NewChunker(context.Background(), fastcdc.WithStreamMode(), fastcdc.With32kChunks())
	handleError(err)

	buf := make([]byte, 3*65_536)
	for {
		n, err := file.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			handleError(err)
		}
		err = chunker.Split(bytes.NewReader(buf[:n]), func(offset, length uint, chunk []byte) error {
			// the chunk is only valid in the callback, copy it for later use
			fmt.Printf("offset: %d, length: %d, sum: %x\n", offset, length, sha256.Sum256(chunk))
			return nil
		})
		handleError(err)
	}

	err = chunker.Finalize(func(offset, length uint, chunk []byte) error {
		// the chunk is only valid in the callback, copy it for later use
		fmt.Printf("offset: %d, length: %d, sum: %x\n", offset, length, sha256.Sum256(chunk))
		return nil
	})
	handleError(err)
}
````

### Benchmark
Setup: Intel Core i9-9900k, Linux Mint 20 Ulyana.
````
Benchmark16kChunks-16           100	  11420464 ns/op	2938.10 MB/s	     128 B/op	       2 allocs/op
Benchmark32kChunks-16            97	  11909758 ns/op	2817.39 MB/s	     129 B/op	       2 allocs/op
Benchmark64kChunks-16            94	  12078983 ns/op	2777.92 MB/s	     130 B/op	       2 allocs/op
Benchmark16kChunksStream-16      90	  12238603 ns/op	2741.69 MB/s	   25387 B/op	     513 allocs/op
Benchmark32kChunksStream-16      92	  12791432 ns/op	2623.20 MB/s	   25371 B/op	     513 allocs/op
Benchmark64kChunksStream-16      87	  13152427 ns/op	2551.20 MB/s	   25414 B/op	     513 allocs/op
````

### Invariants
FastCDC will ensure that all chunks meet your minimum and maximum chunk size requirement, except for the last chunk which can
be smaller than the minimum. In addition, whether you use this package in streaming or normal mode, it will always produce the same
chunk for identical input as long as the configuration remain the same (except for the internal buffer size which has no impact 
on the chunk output). Finally, all custom input are validated when creating the chunker.

### Other implementations
- [ronomon/deduplication](https://github.com/ronomon/deduplication)
- [nlfiedler/fastcdc-rs](https://github.com/nlfiedler/fastcdc-rs)
- [jotfs/fastcdc-go](https://github.com/jotfs/fastcdc-go)
- [iscc/fastcdc-py](https://github.com/iscc/fastcdc-py)

### Authors
[Samuel Aeberhard](https://github.com/isam2k) & [Sylvain Muller](https://github.com/tigerwill90)