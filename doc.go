// Package fastcdc implements the FastCDC content defined chunking algorithm based on the gear-rolling hash
// and implements optimizations proposed by Wen Xia et al. in their 2016 paper FastCDC: a Fast and Efficient
// Content-Defined Chunking Approach for Data Deduplication.
//
// The chunking is deterministic. Identical input with an identical chunk size configuration always produces
// identical chunks, whatever the buffer size or the read pattern of the reader.
package fastcdc
