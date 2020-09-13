package fastcdc

import (
	"bytes"
	"context"
	"io"
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
	chunker, err := NewChunker(context.Background(), opts...)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.SetBytes(int64(size))
	b.ReportAllocs()

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

func benchmarkStream(b *testing.B, size int, data []byte, opts ...Option) {
	chunker, err := NewChunker(context.Background(), opts...)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.SetBytes(int64(size))
	b.ReportAllocs()

	var chunks uint
	var totalLength uint
	buf := make([]byte, 65_536)
	for i := 0; i < b.N; i++ {
		reader := bytes.NewReader(data)
		for {
			n, err := reader.Read(buf)
			if err != nil {
				if err == io.EOF {
					break
				}
				b.Fatal(err)
			}

			if err := chunker.Split(bytes.NewReader(buf[:n]), func(offset, length uint, chunk []byte) error {
				chunks++
				totalLength += length
				return nil
			}); err != nil {
				b.Fatal(err)
			}
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

func Benchmark32kChunks(b *testing.B) {
	size := 32 * 1024 * 1024
	data := randomData(155, size)
	benchmark(b, size, data, With32kChunks())
}

func Benchmark64kChunks(b *testing.B) {
	size := 32 * 1024 * 1024
	data := randomData(155, size)
	benchmark(b, size, data, With64kChunks())
}

func Benchmark16kChunksStream(b *testing.B) {
	size := 32 * 1024 * 1024
	data := randomData(155, size)
	benchmarkStream(b, size, data, With16kChunks(), WithStreamMode())
}

func Benchmark32kChunksStream(b *testing.B) {
	size := 32 * 1024 * 1024
	data := randomData(155, size)
	benchmarkStream(b, size, data, With32kChunks(), WithStreamMode())
}

func Benchmark64kChunksStream(b *testing.B) {
	size := 32 * 1024 * 1024
	data := randomData(155, size)
	benchmarkStream(b, size, data, With64kChunks(), WithStreamMode())
}
