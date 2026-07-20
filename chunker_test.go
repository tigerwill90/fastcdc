package fastcdc

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"slices"
	"testing"
	"testing/iotest"
	"time"
)

func handleError(err error) {
	if err != nil {
		panic(err)
	}
}

func sekienData(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile("fixtures/SekienAkashita.jpg")
	if err != nil {
		t.Fatal(err)
	}
	return data
}

type chunkInfo struct {
	Offset int64
	Length int
}

var sekienGoldens = map[string]struct {
	Preset  Option
	MaxSize uint
	Want    []chunkInfo
}{
	"16kChunks": {
		Preset:  With16kChunks(),
		MaxSize: 131_072,
		Want: []chunkInfo{
			{0, 22366},
			{22366, 10491},
			{32857, 14094},
			{46951, 18696},
			{65647, 43819},
		},
	},
	"32kChunks": {
		Preset:  With32kChunks(),
		MaxSize: 262_144,
		Want: []chunkInfo{
			{0, 32857},
			{32857, 32790},
			{65647, 43819},
		},
	},
	"64kChunks": {
		Preset:  With64kChunks(),
		MaxSize: 524_288,
		Want: []chunkInfo{
			{0, 109466},
		},
	},
}

// chunkyReader delivers at most n bytes per read.
type chunkyReader struct {
	r io.Reader
	n int
}

func (c *chunkyReader) Read(p []byte) (int, error) {
	if len(p) > c.n {
		p = p[:c.n]
	}
	return c.r.Read(p)
}

// failingReader delivers its data then fails with err.
type failingReader struct {
	data []byte
	err  error
}

func (f *failingReader) Read(p []byte) (int, error) {
	if len(f.data) == 0 {
		return 0, f.err
	}
	n := copy(p, f.data)
	f.data = f.data[n:]
	return n, nil
}

// chunkAll drains the chunker and verifies that the chunks are contiguous
// and match the input content.
func chunkAll(t *testing.T, chunker *Chunker, r io.Reader, input []byte) []chunkInfo {
	t.Helper()
	var chunks []chunkInfo
	var pos int64
	for chunk, err := range chunker.Chunks(r) {
		if err != nil {
			t.Fatal(err)
		}
		if chunk.Offset != pos {
			t.Fatalf("offset: want = %d, got = %d", pos, chunk.Offset)
		}
		if !bytes.Equal(chunk.Data, input[chunk.Offset:chunk.Offset+int64(len(chunk.Data))]) {
			t.Fatalf("chunk content mismatch at offset %d", chunk.Offset)
		}
		chunks = append(chunks, chunkInfo{chunk.Offset, len(chunk.Data)})
		pos += int64(len(chunk.Data))
	}
	if pos != int64(len(input)) {
		t.Fatalf("stream coverage: want = %d bytes, got = %d", len(input), pos)
	}
	return chunks
}

// In this example, the chunker is configured to output chunk of an average of 32kb size.
func Example_basic() {
	file, err := os.Open("fixtures/SekienAkashita.jpg")
	handleError(err)
	defer file.Close()

	c, err := NewChunker(With32kChunks())
	handleError(err)

	for chunk, err := range c.Chunks(file) {
		handleError(err)
		// the chunk is only valid for this iteration step, copy it for later use
		fmt.Printf("offset: %d, length: %d, sum: %x\n", chunk.Offset, len(chunk.Data), sha256.Sum256(chunk.Data))
	}
	// Output:
	// offset: 0, length: 32857, sum: 5a80871bad4588c7278d39707fe68b8b174b1aa54c59169d3c2c72f1e16ef46d
	// offset: 32857, length: 32790, sum: d3868199f4275cde4c235d5a3dbf7ef7a81594007b3f80db30861607ef10ea0d
	// offset: 65647, length: 43819, sum: d6347a2e5bf586d42f2d80559d4f4a2bf160dce8f77eede023ad2314856f3086
}

func TestLogarithm2(t *testing.T) {
	tests := []struct {
		Value, Result uint
	}{
		{65537, 16},
		{65536, 16},
		{65535, 16},
		{32769, 15},
		{32768, 15},
		{32767, 15},
		{AverageMin, 8},
		{AverageMax, 28},
	}

	for _, tc := range tests {
		got := logarithm2(tc.Value)
		if got != tc.Result {
			t.Errorf("want = %d, got = %d", tc.Result, got)
		}
	}
}

func TestCeilDiv(t *testing.T) {
	tests := []struct {
		X, Y, Result uint
	}{
		{10, 5, 2},
		{11, 5, 3},
		{10, 3, 4},
		{9, 3, 3},
		{6, 2, 3},
		{5, 2, 3},
		{1, 2, 1},
	}

	for _, tc := range tests {
		got := ceilDiv(tc.X, tc.Y)
		if got != tc.Result {
			t.Errorf("want = %d, got = %d", tc.Result, got)
		}
	}
}

func TestCenterSize(t *testing.T) {
	tests := []struct {
		Average, Min, SourceSize, Result uint
	}{
		{50, 100, 50, 0},
		{200, 100, 50, 50},
		{200, 100, 40, 40},
	}

	for _, tc := range tests {
		got := centerSize(tc.Average, tc.Min, tc.SourceSize)
		if got != tc.Result {
			t.Errorf("want = %d, got = %d", tc.Result, got)
		}
	}
}

func TestMask(t *testing.T) {
	tests := []struct {
		Bits   uint
		Result uint64
	}{
		{24, 16_777_215},
		{16, 65535},
		{10, 1023},
		{8, 255},
	}

	for _, tc := range tests {
		got := mask(tc.Bits)
		if got != tc.Result {
			t.Errorf("want = %d, got = %d", tc.Result, got)
		}
	}
}

func TestMaskPanic(t *testing.T) {
	tests := []struct {
		Name     string
		Bits     uint
		PanicMsg string
	}{
		{"too low", 0, "bits too low"},
		{"too high", 32, "bits too high"},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Error("the code did not panic")
				} else {
					panicMsg := r.(string)
					if panicMsg != tc.PanicMsg {
						t.Errorf("want = %s, got = %s", tc.PanicMsg, r)
					}
				}
			}()
			mask(tc.Bits)
		})
	}
}

func TestChunkerValidation(t *testing.T) {
	tests := map[string]struct {
		MinSize, AvgSize, MaxSize, BufferSize uint
		Want                                  error
	}{
		"minimum min size": {
			MinSize:    MinimumMin - 1,
			AvgSize:    AverageMin,
			MaxSize:    MaximumMin,
			BufferSize: MaximumMin,
			Want:       ErrInvalidChunkSize,
		},
		"average min size": {
			MinSize:    MinimumMin,
			AvgSize:    AverageMin - 1,
			MaxSize:    MaximumMin,
			BufferSize: MaximumMin,
			Want:       ErrInvalidChunkSize,
		},
		"maximum min size": {
			MinSize:    MinimumMin,
			AvgSize:    AverageMin,
			MaxSize:    MaximumMin - 1,
			BufferSize: MaximumMin,
			Want:       ErrInvalidChunkSize,
		},
		"minimum max size": {
			MinSize:    MinimumMax + 1,
			AvgSize:    AverageMax,
			MaxSize:    MaximumMax,
			BufferSize: MaximumMax,
			Want:       ErrInvalidChunkSize,
		},
		"average max size": {
			MinSize:    MinimumMax,
			AvgSize:    AverageMax + 1,
			MaxSize:    MaximumMax,
			BufferSize: MaximumMax,
			Want:       ErrInvalidChunkSize,
		},
		"maximum max size": {
			MinSize:    MinimumMax,
			AvgSize:    AverageMax,
			MaxSize:    MaximumMax + 1,
			BufferSize: MaximumMax,
			Want:       ErrInvalidChunkSize,
		},
		"minimum buffer size": {
			MinSize:    MinimumMin,
			AvgSize:    AverageMin,
			MaxSize:    MaximumMin,
			BufferSize: MaximumMin - 1,
			Want:       ErrInvalidBufferSize,
		},
		"min size bigger or equal than avg size": {
			MinSize:    AverageMin,
			AvgSize:    AverageMin,
			MaxSize:    MaximumMin,
			BufferSize: MaximumMin,
			Want:       ErrInvalidChunkSize,
		},
		"max size smaller or equal than avg size": {
			MinSize:    MinimumMin,
			AvgSize:    MaximumMin,
			MaxSize:    MaximumMin,
			BufferSize: MaximumMin,
			Want:       ErrInvalidChunkSize,
		},
		"proportional cut point": {
			MinSize:    1048,
			AvgSize:    2048,
			MaxSize:    3096,
			BufferSize: 2 * 3096,
			Want:       ErrInvalidChunkSize,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := NewChunker(WithChunksSize(tc.MinSize, tc.AvgSize, tc.MaxSize), WithBufferSize(tc.BufferSize))
			if !errors.Is(err, tc.Want) {
				t.Errorf("want = %s, got = %s", tc.Want, err)
			}
		})
	}
}

func TestAllZeros(t *testing.T) {
	input := make([]byte, 10240)
	chunker, err := NewChunker(WithChunksSize(64, 256, 1024), WithBufferSize(1024))
	if err != nil {
		t.Fatal(err)
	}

	chunks := chunkAll(t, chunker, bytes.NewReader(input), input)
	for _, chunk := range chunks {
		if chunk.Offset%1024 != 0 {
			t.Errorf("offset: want = 0, got = %d", chunk.Offset%1024)
		}
		if chunk.Length != 1024 {
			t.Errorf("length: want = 1024, got = %d", chunk.Length)
		}
	}
}

func TestSekienChunks(t *testing.T) {
	data := sekienData(t)

	for name, tc := range sekienGoldens {
		t.Run(name, func(t *testing.T) {
			chunker, err := NewChunker(tc.Preset, WithBufferSize(tc.MaxSize))
			if err != nil {
				t.Fatal(err)
			}

			chunks := chunkAll(t, chunker, bytes.NewReader(data), data)
			if !slices.Equal(chunks, tc.Want) {
				t.Errorf("chunks: want = %v, got = %v", tc.Want, chunks)
			}
		})
	}
}

// TestSekienReaderFragmentation checks that the read pattern of the reader
// has no impact on the chunk output.
func TestSekienReaderFragmentation(t *testing.T) {
	data := sekienData(t)

	readers := map[string]func(io.Reader) io.Reader{
		"full":       func(r io.Reader) io.Reader { return r },
		"one byte":   iotest.OneByteReader,
		"half":       iotest.HalfReader,
		"data err":   iotest.DataErrReader,
		"7 bytes":    func(r io.Reader) io.Reader { return &chunkyReader{r, 7} },
		"1000 bytes": func(r io.Reader) io.Reader { return &chunkyReader{r, 1000} },
	}

	for name, tc := range sekienGoldens {
		t.Run(name, func(t *testing.T) {
			chunker, err := NewChunker(tc.Preset)
			if err != nil {
				t.Fatal(err)
			}
			for readerName, wrap := range readers {
				chunks := chunkAll(t, chunker, wrap(bytes.NewReader(data)), data)
				if !slices.Equal(chunks, tc.Want) {
					t.Errorf("%s reader: chunks: want = %v, got = %v", readerName, tc.Want, chunks)
				}
			}
		})
	}
}

// TestSekienBufferSizeInvariance checks that the buffer size has no impact
// on the chunk output, whatever the read pattern of the reader.
func TestSekienBufferSizeInvariance(t *testing.T) {
	data := sekienData(t)

	seed := time.Now().UnixNano()
	rng := rand.New(rand.NewPCG(uint64(seed), 0))
	t.Logf("seed: %d", seed)

	for name, tc := range sekienGoldens {
		t.Run(name, func(t *testing.T) {
			for range 200 {
				bufSize := tc.MaxSize + uint(rng.IntN(int(1<<20-tc.MaxSize)+1))
				frag := 1 + rng.IntN(1<<17)

				chunker, err := NewChunker(tc.Preset, WithBufferSize(bufSize))
				if err != nil {
					t.Fatal(err)
				}
				chunks := chunkAll(t, chunker, &chunkyReader{bytes.NewReader(data), frag}, data)
				if !slices.Equal(chunks, tc.Want) {
					t.Fatalf("chunks: want = %v, got = %v, buffer size = %d, read size = %d", tc.Want, chunks, bufSize, frag)
				}
			}
		})
	}
}

// TestRandomInput checks on random input that the chunk sizes stay within
// bounds and that the chunk output does not depend on the buffer size or
// the read pattern of the reader.
func TestRandomInput(t *testing.T) {
	tests := []struct {
		Name    string
		MinSize int
		MaxSize int
		Opt     Option
	}{
		{"16kChunks", 4096, 131_072, With16kChunks()},
		{"32kChunks", 8192, 262_144, With32kChunks()},
		{"64kChunks", 16_384, 524_288, With64kChunks()},
	}

	seed := time.Now().UnixNano()
	rng := rand.New(rand.NewPCG(uint64(seed), 0))
	t.Logf("seed: %d", seed)

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			reference, err := NewChunker(tc.Opt)
			if err != nil {
				t.Fatal(err)
			}

			for range 150 {
				size := 1000 + rng.IntN(4<<20)
				data := make([]byte, size)
				for i := range data {
					data[i] = byte(rng.Uint64())
				}

				want := chunkAll(t, reference, bytes.NewReader(data), data)
				for i, chunk := range want {
					if i < len(want)-1 && (chunk.Length < tc.MinSize || chunk.Length > tc.MaxSize) {
						t.Errorf("chunk size out of bounds: %d < %d < %d", tc.MinSize, chunk.Length, tc.MaxSize)
					}
					if i == len(want)-1 && chunk.Length > tc.MaxSize {
						t.Errorf("last chunk size out of bounds: %d > %d", chunk.Length, tc.MaxSize)
					}
				}

				bufSize := uint(tc.MaxSize) + uint(rng.IntN(1<<20-tc.MaxSize+1))
				frag := 512 + rng.IntN(1<<17)

				chunker, err := NewChunker(tc.Opt, WithBufferSize(bufSize))
				if err != nil {
					t.Fatal(err)
				}
				got := chunkAll(t, chunker, &chunkyReader{bytes.NewReader(data), frag}, data)
				if !slices.Equal(got, want) {
					t.Fatalf("chunks: want = %v, got = %v, input size = %d, buffer size = %d, read size = %d", want, got, size, bufSize, frag)
				}
			}
		})
	}
}

func TestReadError(t *testing.T) {
	sentinel := errors.New("read failure")
	data := randomData(42, 1<<20)

	chunker, err := NewChunker(With64kChunks())
	if err != nil {
		t.Fatal(err)
	}

	var chunks int
	var gotErr error
	for chunk, err := range chunker.Chunks(&failingReader{data: data, err: sentinel}) {
		if err != nil {
			gotErr = err
			if chunk.Offset != 0 || len(chunk.Data) != 0 {
				t.Error("chunk must be zero when an error is yielded")
			}
			continue
		}
		chunks++
	}

	if !errors.Is(gotErr, sentinel) {
		t.Errorf("want = %s, got = %s", sentinel, gotErr)
	}
	if chunks == 0 {
		t.Error("chunks found before the read failure must be yielded")
	}
}

func TestEmptyInput(t *testing.T) {
	chunker, err := NewChunker()
	if err != nil {
		t.Fatal(err)
	}

	for _, err := range chunker.Chunks(bytes.NewReader(nil)) {
		if err != nil {
			t.Fatal(err)
		}
		t.Error("no chunk expected on empty input")
	}
}

// TestEarlyBreakAndReuse checks that a chunker can be reused for another
// stream, even after an iteration stopped early.
func TestEarlyBreakAndReuse(t *testing.T) {
	data := sekienData(t)
	golden := sekienGoldens["64kChunks"]

	chunker, err := NewChunker(golden.Preset)
	if err != nil {
		t.Fatal(err)
	}

	for chunk, err := range chunker.Chunks(bytes.NewReader(data)) {
		if err != nil {
			t.Fatal(err)
		}
		if chunk.Offset != golden.Want[0].Offset || len(chunk.Data) != golden.Want[0].Length {
			t.Errorf("first chunk: want = %v, got = {%d %d}", golden.Want[0], chunk.Offset, len(chunk.Data))
		}
		break
	}

	chunks := chunkAll(t, chunker, bytes.NewReader(data), data)
	if !slices.Equal(chunks, golden.Want) {
		t.Errorf("chunks after reuse: want = %v, got = %v", golden.Want, chunks)
	}
}

func TestConcurrentIterationPanic(t *testing.T) {
	data := sekienData(t)

	chunker, err := NewChunker(With64kChunks())
	if err != nil {
		t.Fatal(err)
	}

	for _, err := range chunker.Chunks(bytes.NewReader(data)) {
		if err != nil {
			t.Fatal(err)
		}
		func() {
			defer func() {
				if r := recover(); r == nil {
					t.Error("the code did not panic")
				} else {
					panicMsg := r.(string)
					want := "fastcdc: chunker already in use"
					if panicMsg != want {
						t.Errorf("want = %s, got = %s", want, r)
					}
				}
			}()
			for range chunker.Chunks(bytes.NewReader(data)) {
			}
		}()
		break
	}
}

func TestChunksAllocs(t *testing.T) {
	data := randomData(7, 4<<20)

	chunker, err := NewChunker(With64kChunks())
	if err != nil {
		t.Fatal(err)
	}

	reader := bytes.NewReader(data)
	avg := testing.AllocsPerRun(5, func() {
		reader.Reset(data)
		for _, err := range chunker.Chunks(reader) {
			if err != nil {
				panic(err)
			}
		}
	})

	// Each run yields dozens of chunks. Only a small constant overhead per
	// Chunks call is allowed, an allocation per chunk must fail here.
	if avg > 4 {
		t.Errorf("allocs per run: want <= 4, got = %g", avg)
	}
}

func TestSmallInput(t *testing.T) {
	input := randomData(3, 8193)

	chunker, err := NewChunker(With16kChunks())
	if err != nil {
		t.Fatal(err)
	}
	chunkAll(t, chunker, bytes.NewReader(input), input)
}

func TestSekienMinChunks(t *testing.T) {
	data := sekienData(t)

	chunker, err := NewChunker(WithChunksSize(64, 256, 1024), WithBufferSize(1024))
	if err != nil {
		t.Fatal(err)
	}
	chunkAll(t, chunker, bytes.NewReader(data), data)
}

func TestSekienMaxChunks(t *testing.T) {
	data := sekienData(t)

	chunker, err := NewChunker(WithChunksSize(67_108_864, 268_435_456, 1_073_741_824), WithBufferSize(1_073_741_824))
	if err != nil {
		t.Fatal(err)
	}

	chunks := chunkAll(t, chunker, bytes.NewReader(data), data)
	if len(chunks) != 1 {
		t.Fatalf("chunks length: want = 1, got = %d", len(chunks))
	}
	if chunks[0].Offset != 0 || chunks[0].Length != 109466 {
		t.Errorf("want offset = 0, got offset = %d, want length = 109466, got length = %d", chunks[0].Offset, chunks[0].Length)
	}
}
