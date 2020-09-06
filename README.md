# FastCDC

This package implements the FastCDC content defined chunking algorithm based on the gear-rolling hash and implements optimizations proposed by Wen Xia et al. in their 2016 [paper](https://www.usenix.org/system/files/conference/atc16/atc16-paper-xia.pdf) FastCDC:
a Fast and Efficient Content-Defined Chunking Approach for Data Deduplication.

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

### Objective
For another project I need to deduplicate a stream of files parts by parts on the fly. I struggled to find a chunker package
with this capability, so I developed mine. This is a pure go implementation of the FastCDC algorithm with a copyleft license. 
The interface differs significantly from other chunker that I know. It's designed to be easy to use, especially in streaming fashion.

### Invariants

go-fastcdc will ensure that all chunks meet your minimum and maximum chunk size requirement, except for the last chunk which can
be smaller than the minimum. In addition, whether you use this package in streaming or normal mode, it will always produce the same
chunk for identical input as long as the chunk size configuration remain the same.

### Other implementations
- little more than a translation from [ronomon/deduplication](https://github.com/ronomon/deduplication)
- inspired by [nlfiedler/fastcdc-rs](https://github.com/nlfiedler/fastcdc-rs)
- [iscc/fastcdc-py](https://github.com/iscc/fastcdc-py)
- [jotfs/fastcdc-go](https://github.com/jotfs/fastcdc-go)

### Authors

[Samuel Aeberhard](https://github.com/isam2k) & [Sylvain Muller](https://github.com/tigerwill90)