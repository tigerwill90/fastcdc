package fastcdc

import (
	"errors"
	"fmt"
	"io"
	"iter"
	"math"
	"sync/atomic"
)

// Bounds within which the chunk size configuration must fit.
// They are enforced by NewChunker.
const (
	MinimumMin uint = 64
	MinimumMax uint = 67_108_864
	AverageMin uint = 256
	AverageMax uint = 268_435_456
	MaximumMin uint = 1024
	MaximumMax uint = 1_073_741_824
)

var (
	ErrInvalidChunkSize  = errors.New("invalid chunk size")
	ErrInvalidBufferSize = errors.New("invalid buffer size")
)

// Chunk is a single chunk of the input stream.
type Chunk struct {
	// Offset is the position of the chunk in the input stream.
	Offset int64
	// Data is the chunk content. It aliases the chunker's internal
	// buffer and is only valid for the current iteration. Copy
	// it for later use.
	Data []byte
}

// Chunker splits a stream into content-defined chunks. Identical input
// with an identical chunk size configuration always produces identical
// chunks, whatever the buffer size or the read pattern of the reader.
type Chunker struct {
	buffer  []byte
	minSize uint
	avgSize uint
	maxSize uint
	maskS   uint64
	maskL   uint64
	busy    atomic.Bool
}

// NewChunker returns a blazing fast chunker.
func NewChunker(opts ...Option) (*Chunker, error) {
	config := defaultConfig()

	for _, opt := range opts {
		opt(config)
	}

	if config.bufferSize == 0 {
		config.bufferSize = 2 * config.maxSize
	}

	const (
		errMinMsg = "chunk size must be at least"
		errMaxMsg = "chunk size must be equal or lesser than"
	)

	if config.minSize < MinimumMin {
		return nil, fmt.Errorf("the minimum %s %d: %w", errMinMsg, MinimumMin, ErrInvalidChunkSize)
	}
	if config.minSize > MinimumMax {
		return nil, fmt.Errorf("the minimum %s %d: %w", errMaxMsg, MinimumMax, ErrInvalidChunkSize)
	}
	if config.avgSize < AverageMin {
		return nil, fmt.Errorf("the average %s %d: %w", errMinMsg, AverageMin, ErrInvalidChunkSize)
	}
	if config.avgSize > AverageMax {
		return nil, fmt.Errorf("the average %s %d: %w", errMaxMsg, AverageMax, ErrInvalidChunkSize)
	}
	if config.maxSize < MaximumMin {
		return nil, fmt.Errorf("the maximum %s %d: %w", errMinMsg, MaximumMin, ErrInvalidChunkSize)
	}
	if config.maxSize > MaximumMax {
		return nil, fmt.Errorf("the maximum %s %d: %w", errMaxMsg, MaximumMax, ErrInvalidChunkSize)
	}
	if config.bufferSize < config.maxSize {
		return nil, fmt.Errorf("the buffer size must be greater or equal than the maximum chunk size (%d): %w", config.maxSize, ErrInvalidBufferSize)
	}
	if config.minSize >= config.avgSize {
		return nil, fmt.Errorf("the minimum chunk size must be smaller than the average: %w", ErrInvalidChunkSize)
	}
	if config.maxSize <= config.avgSize {
		return nil, fmt.Errorf("the maximum chunk size must be bigger than the average: %w", ErrInvalidChunkSize)
	}
	if config.maxSize-config.minSize <= config.avgSize {
		return nil, fmt.Errorf("maximum - minimum chunk size must be bigger than the average chunk size: %w", ErrInvalidChunkSize)
	}

	bits := logarithm2(config.avgSize)

	return &Chunker{
		buffer:  make([]byte, config.bufferSize),
		minSize: config.minSize,
		avgSize: config.avgSize,
		maxSize: config.maxSize,
		// Masks use 1 bit normalization.
		// https://github.com/ronomon/deduplication#content-dependent-chunking
		maskS: mask(bits + 1),
		maskL: mask(bits - 1),
	}, nil
}

// Chunks returns an iterator that reads the stream and yields its chunks
// in order. Every chunk size is within [min, max], except the last chunk
// of the stream which can be smaller than min. On a read error, the
// iterator yields a zero Chunk with the error and stops. The part of the
// stream read so far but not yet chunked is dropped.
//
// The yielded Chunk.Data aliases the chunker's internal buffer. It is
// only valid for the current iteration and must be copied for later
// use.
//
// The chunker can be reused for another stream once the previous
// iteration is over, but only one iteration must run at a time.
func (c *Chunker) Chunks(r io.Reader) iter.Seq2[Chunk, error] {
	return func(yield func(Chunk, error) bool) {
		if !c.busy.CompareAndSwap(false, true) {
			panic("fastcdc: chunker already in use")
		}
		defer c.busy.Store(false)

		var (
			offset int64 // stream position of buffer[0]
			start  uint  // start of the current chunk in the buffer
			end    uint  // end of the buffered data
			eof    bool
		)
		for {
			if !eof && end-start < c.maxSize {
				// Move the pending bytes at the front of the buffer, then
				// fill it completely. The chunker only looks for a cut point
				// with at least max size bytes ahead, or on end of stream,
				// so the read pattern of the reader cannot influence the
				// chunk boundaries.
				copy(c.buffer, c.buffer[start:end])
				offset += int64(start)
				end -= start
				start = 0
				for end < uint(len(c.buffer)) {
					n, err := r.Read(c.buffer[end:])
					end += uint(n)
					if err == io.EOF {
						eof = true
						break
					}
					if err != nil {
						yield(Chunk{}, err)
						return
					}
				}
			}

			if start == end {
				return
			}

			length := c.breakpoint(c.buffer[start:end])
			if length == 0 {
				// No cut point in the pending bytes means this is the
				// last chunk of the stream.
				length = end - start
			}
			if !yield(Chunk{Offset: offset + int64(start), Data: c.buffer[start : start+length]}, nil) {
				return
			}
			start += length
		}
	}
}

// breakpoint returns the size of the next chunk in the window, or 0 when
// no cut point can be found before the end of the window.
func (c *Chunker) breakpoint(window []byte) uint {
	length := uint(len(window))

	// Sub-minimum chunk cut-point skipping.
	if length <= c.minSize {
		return 0
	}

	// Never look past max size bytes. Over that limit the chunk is cut
	// at max size whatever its content.
	if length > c.maxSize {
		length = c.maxSize
	}

	normalSize := centerSize(c.avgSize, c.minSize, length)

	var hash uint64
	cut := c.minSize

	// Start by using the "harder" chunking judgement to find
	// chunks that run smaller than the desired normal size.
	for cut < normalSize {
		hash = (hash >> 1) + table[window[cut]]
		cut++
		if hash&c.maskS == 0 {
			return cut
		}
	}

	// Fall back to using the "easier" chunking judgement to find chunks
	// that run larger than the desired normal size but never bigger than
	// the max size.
	for cut < length {
		hash = (hash >> 1) + table[window[cut]]
		cut++
		if hash&c.maskL == 0 {
			return cut
		}
	}

	// We are unable to find a cut point with the chunking judgement.
	// If the window is exactly max size long, the chunk reaches the max
	// size allowed and must be cut.
	if cut == c.maxSize {
		return cut
	}
	return 0
}

// centerSize finds the middle of the desired chunk size. This is what the
// FastCDC paper refers as "normal size", but with a more adaptive
// threshold based on a combination of average and minimum chunk size
// to decide the pivot point at which to switch masks.
// https://github.com/ronomon/deduplication#content-dependent-chunking
func centerSize(average, minimum, sourceSize uint) uint {
	offset := minimum + ceilDiv(minimum, 2)
	if offset > average {
		offset = average
	}
	size := average - offset
	if size > sourceSize {
		return sourceSize
	}
	return size
}

// Integer division that rounds up instead of down.
func ceilDiv(x, y uint) uint {
	return (x + y - 1) / y
}

func mask(bits uint) uint64 {
	if bits < 1 {
		panic("bits too low")
	}
	if bits > 31 {
		panic("bits too high")
	}
	return 1<<bits - 1
}

// Base 2 logarithm, rounded to the nearest integer.
func logarithm2(value uint) uint {
	return uint(math.Round(math.Log2(float64(value))))
}

var table = [256]uint64{
	1553318008, 574654857, 759734804, 310648967, 1393527547, 1195718329,
	694400241, 1154184075, 1319583805, 1298164590, 122602963, 989043992,
	1918895050, 933636724, 1369634190, 1963341198, 1565176104, 1296753019,
	1105746212, 1191982839, 1195494369, 29065008, 1635524067, 722221599,
	1355059059, 564669751, 1620421856, 1100048288, 1018120624, 1087284781,
	1723604070, 1415454125, 737834957, 1854265892, 1605418437, 1697446953,
	973791659, 674750707, 1669838606, 320299026, 1130545851, 1725494449,
	939321396, 748475270, 554975894, 1651665064, 1695413559, 671470969,
	992078781, 1935142196, 1062778243, 1901125066, 1935811166, 1644847216,
	744420649, 2068980838, 1988851904, 1263854878, 1979320293, 111370182,
	817303588, 478553825, 694867320, 685227566, 345022554, 2095989693,
	1770739427, 165413158, 1322704750, 46251975, 710520147, 700507188,
	2104251000, 1350123687, 1593227923, 1756802846, 1179873910, 1629210470,
	358373501, 807118919, 751426983, 172199468, 174707988, 1951167187,
	1328704411, 2129871494, 1242495143, 1793093310, 1721521010, 306195915,
	1609230749, 1992815783, 1790818204, 234528824, 551692332, 1930351755,
	110996527, 378457918, 638641695, 743517326, 368806918, 1583529078,
	1767199029, 182158924, 1114175764, 882553770, 552467890, 1366456705,
	934589400, 1574008098, 1798094820, 1548210079, 821697741, 601807702,
	332526858, 1693310695, 136360183, 1189114632, 506273277, 397438002,
	620771032, 676183860, 1747529440, 909035644, 142389739, 1991534368,
	272707803, 1905681287, 1210958911, 596176677, 1380009185, 1153270606,
	1150188963, 1067903737, 1020928348, 978324723, 962376754, 1368724127,
	1133797255, 1367747748, 1458212849, 537933020, 1295159285, 2104731913,
	1647629177, 1691336604, 922114202, 170715530, 1608833393, 62657989,
	1140989235, 381784875, 928003604, 449509021, 1057208185, 1239816707,
	525522922, 476962140, 102897870, 132620570, 419788154, 2095057491,
	1240747817, 1271689397, 973007445, 1380110056, 1021668229, 12064370,
	1186917580, 1017163094, 597085928, 2018803520, 1795688603, 1722115921,
	2015264326, 506263638, 1002517905, 1229603330, 1376031959, 763839898,
	1970623926, 1109937345, 524780807, 1976131071, 905940439, 1313298413,
	772929676, 1578848328, 1108240025, 577439381, 1293318580, 1512203375,
	371003697, 308046041, 320070446, 1252546340, 568098497, 1341794814,
	1922466690, 480833267, 1060838440, 969079660, 1836468543, 2049091118,
	2023431210, 383830867, 2112679659, 231203270, 1551220541, 1377927987,
	275637462, 2110145570, 1700335604, 738389040, 1688841319, 1506456297,
	1243730675, 258043479, 599084776, 41093802, 792486733, 1897397356,
	28077829, 1520357900, 361516586, 1119263216, 209458355, 45979201,
	363681532, 477245280, 2107748241, 601938891, 244572459, 1689418013,
	1141711990, 1485744349, 1181066840, 1950794776, 410494836, 1445347454,
	2137242950, 852679640, 1014566730, 1999335993, 1871390758, 1736439305,
	231222289, 603972436, 783045542, 370384393, 184356284, 709706295,
	1453549767, 591603172, 768512391, 854125182,
}
