package fastcdc

import (
	"bytes"
	"encoding/binary"
	"math/rand/v2"
	"testing"
)

func randomData(seed uint64, size int) []byte {
	var s [32]byte
	binary.LittleEndian.PutUint64(s[:8], seed)
	rng := rand.NewChaCha8(s)
	data := make([]byte, size)
	rng.Read(data)
	return data
}

func benchmark(b *testing.B, data []byte, opts ...Option) {
	chunker, err := NewChunker(opts...)
	if err != nil {
		b.Fatal(err)
	}

	b.SetBytes(int64(len(data)))
	b.ReportAllocs()

	var chunks int
	var totalLength int
	reader := bytes.NewReader(data)
	for b.Loop() {
		reader.Reset(data)
		for chunk, err := range chunker.Chunks(reader) {
			if err != nil {
				b.Fatal(err)
			}
			chunks++
			totalLength += len(chunk.Data)
		}
	}

	b.Logf("average chunks size: %d", totalLength/chunks)
}

func Benchmark16kChunks(b *testing.B) {
	benchmark(b, randomData(155, 32*1024*1024), With16kChunks())
}

func Benchmark32kChunks(b *testing.B) {
	benchmark(b, randomData(155, 32*1024*1024), With32kChunks())
}

func Benchmark64kChunks(b *testing.B) {
	benchmark(b, randomData(155, 32*1024*1024), With64kChunks())
}
