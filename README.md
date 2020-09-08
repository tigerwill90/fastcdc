[![PkgGoDev](https://pkg.go.dev/badge/github.com/tigerwill90/fastcdc?tab=doc)](https://pkg.go.dev/github.com/tigerwill90/fastcdc?tab=doc)
# FastCDC
This package implements the FastCDC content defined chunking algorithm based on the gear-rolling hash and implements optimizations proposed by Wen Xia et al. in their 2016 [paper](https://www.usenix.org/system/files/conference/atc16/atc16-paper-xia.pdf) FastCDC:
a Fast and Efficient Content-Defined Chunking Approach for Data Deduplication.

````
go get -u github.com/tigerwill90/fastcdc
````

### Objective
For another project, I need to deduplicate on the fly a stream of file served from a plugin through grpc. I struggled to find a chunker package
with this capability, so a friend and I developed our own. This is a pure go implementation of the FastCDC algorithm with a copyleft license. 
The interface differs significantly from other chunker that I know. It's designed to be easy to use, especially in streaming fashion.

### Example

In this example, we configure the chunker to split the given file into chunk of an average of 32k.
We also enable optimization: a more adaptive threshold to speed up the process and we use masks with
1 bit chunk size normalization instead of 2.
````go
file, err := os.Open("fixtures/SekienAkashita.jpg")
handleError(err)
defer file.Close()

chunker, err := NewChunker(context.Background(), With32kChunks(), WithOptimization())
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
````

Now let's say you want to process a file stream part by part. We configure the chunker in stream mode and keep the
same option as before. For the sake of simplicity, we simulate the file stream by creating 
smaller part before splitting them into chunk of 32k average size.

````go
file, err := os.Open("fixtures/SekienAkashita.jpg")
handleError(err)
defer file.Close()

chunker, err := NewChunker(context.Background(), WithStreamMode(), With32kChunks(), WithOptimization(), WithBufferSize(65_536))
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
````

### Further optimization
This package is based on optimizations and variations introduce by [ronomon/deduplication](https://github.com/ronomon/deduplication).
The explanation below is copied from its repository.

> The following optimizations and variations on FastCDC are involved in the chunking algorithm:
> 
> - 31 bit integers to avoid 64 bit integers for the sake of the Javascript reference implementation.
>  
> - A right shift instead of a left shift to remove the need for an additional modulus operator, which would otherwise have been necessary to prevent overflow.
>  
> - Masks are no longer zero-padded since a right shift is used instead of a left shift.
>  
> - A more adaptive threshold based on a combination of average and minimum chunk size (rather than just average chunk size) to decide the pivot point at which to switch masks. A larger minimum chunk size now switches from the strict mask to the eager mask earlier.
>  
> - Masks use 1 bit of chunk size normalization instead of 2 bits of chunk size normalization.

These optimizations can be enabled with `WithOptimization()` option.

### Benchmark
Setup: Intel Core i9-9900k, Linux Mint 20 Ulyana.
````
Benchmark16kChunks-16                    99	  11471849 ns/op	2924.94 MB/s	     792 B/op	       2 allocs/op
Benchmark16kChunksWithOptimization-16   120	   9766584 ns/op	3435.64 MB/s	     670 B/op	       2 allocs/op
````

### Invariants
FastCDC will ensure that all chunks meet your minimum and maximum chunk size requirement, except for the last chunk which can
be smaller than the minimum. In addition, whether you use this package in streaming or normal mode, it will always produce the same
chunk for identical input as long as the configuration remain the same (except for the internal buffer size which has no impact 
on the chunk output). Finally, all custom input are validated and return error.

### Other implementations
- [ronomon/deduplication](https://github.com/ronomon/deduplication)
- [nlfiedler/fastcdc-rs](https://github.com/nlfiedler/fastcdc-rs)
- [jotfs/fastcdc-go](https://github.com/jotfs/fastcdc-go)
- [iscc/fastcdc-py](https://github.com/iscc/fastcdc-py)

### Authors
[Samuel Aeberhard](https://github.com/isam2k) & [Sylvain Muller](https://github.com/tigerwill90)