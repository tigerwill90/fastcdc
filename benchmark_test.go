package fastcdc

import (
	"bytes"
	"context"
	"math/rand"
	"testing"
)

func randomData(seed, size int) []byte {
	rand.Seed(int64(seed))
	data := make([]byte, size)
	rand.Read(data)
	return data
}

func benchmark(b *testing.B, size int, data []byte, opts ...Option) {
	b.ResetTimer()
	b.SetBytes(int64(size))
	b.ReportAllocs()

	chunker, err := NewChunker(context.Background(), opts...)
	if err != nil {
		b.Fatal(err)
	}

	var chunks uint
	var totalLength uint
	for i := 0; i < b.N; i++ {
		if err := chunker.Split(bytes.NewReader(data), func(offset, length uint, chunk []byte) error {
			chunks++
			totalLength += length
			return nil
		}); err != nil {
			b.Fatal(err)
		}

		if err := chunker.Finalize(func(offset, length uint, chunk []byte) error {
			chunks++
			totalLength += length
			return nil
		}); err != nil {
			b.Fatal(err)
		}
	}

	b.Logf("average chunks size: %d", totalLength/chunks)
}

func Benchmark16kChunks(b *testing.B) {
	size := 32 * 1024 * 1024
	data := randomData(155, size)
	benchmark(b, size, data, With16kChunks())
}

func Benchmark16kChunksWithOptimization(b *testing.B) {
	size := 32 * 1024 * 1024
	data := randomData(155, size)
	benchmark(b, size, data, With16kChunks(), WithOptimization())
}
