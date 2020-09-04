# FastCDC

**In development, interface may change in the future**

This package implements the FastCDC content defined chunking algorithm based on the gear-rolling hash and implements optimizations proposed by Wen Xia et al. in their 2016 [paper](https://www.usenix.org/system/files/conference/atc16/atc16-paper-xia.pdf) FastCDC:
a Fast and Efficient Content-Defined Chunking Approach for Data Deduplication.

### Further optimization

This package is based on optimizations and variations introduce by [ronomon/deduplication](https://github.com/ronomon/deduplication).
The primary objective was to have a pure go implementation of the FastCDC algorithm with a copyleft license that I needed for another
project. Otherwise, the interface differs significantly and is designed to be easy to use, especially in streaming fashion.

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

### Invariants

go-fastcdc will ensure that all chunks meet your minimum and maximum chunk size requirement, except for the last chunk which can
be smaller than the minimum. In addition, whether you use go-fastcdc in streaming or normal mode, it will always produce the same
chunk for identical input as long as the configuration remain the same (chunk size and internal buffer size).

### Other implementations
- [ronomon/deduplication](https://github.com/ronomon/deduplication)
- [nlfiedler/fastcdc-rs](https://github.com/nlfiedler/fastcdc-rs)
- [iscc/fastcdc-py](https://github.com/iscc/fastcdc-py)

### Authors

[Samuel Aeberhard](https://github.com/isam2k) & [Sylvain Muller](https://github.com/tigerwill90)