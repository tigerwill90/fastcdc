package fastcdc

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"
)

const (
	MinimumMin uint = 64
	MinimumMax uint = 67_108_864
	AverageMin uint = 256
	AverageMax uint = 268_435_456
	MaximumMin uint = 1024
	MaximumMax uint = 1_073_741_824
)

type FastCDC struct {
	buffer            []byte
	carry             uint
	realOffset        uint
	offset            uint
	minSize           uint
	avgSize           uint
	maxSize           uint
	maskS             uint
	maskL             uint
	previousBytesRead uint
	streamMode        bool
	firstCall         bool
	ctx               context.Context
}

var (
	ErrInvalidChunksSizePoint = errors.New("invalid chunks size")
	ErrInvalidBufferLength    = errors.New("invalid buffer length")
)

// NewFastCDC return a cancelable blazing fast chunker
func NewFastCDC(ctx context.Context, opts ...Option) (*FastCDC, error) {
	config := defaultConfig()

	for _, opt := range opts {
		opt(config)
	}

	const (
		errMinMsg = "chunks size must be at least"
		errMaxMsg = "chunks size must be equal or lesser than"
	)

	if config.minSize < MinimumMin {
		return nil, fmt.Errorf("the minimum %s %d: %w", errMinMsg, MinimumMin, ErrInvalidChunksSizePoint)
	}
	if config.minSize > MinimumMax {
		return nil, fmt.Errorf("the minimum %s %d: %w", errMaxMsg, MinimumMax, ErrInvalidChunksSizePoint)
	}
	if config.avgSize < AverageMin {
		return nil, fmt.Errorf("the average %s %d: %w", errMinMsg, AverageMin, ErrInvalidChunksSizePoint)
	}
	if config.avgSize > AverageMax {
		return nil, fmt.Errorf("the average %s %d: %w", errMaxMsg, AverageMax, ErrInvalidChunksSizePoint)
	}
	if config.maxSize < MaximumMin {
		return nil, fmt.Errorf("the maximum %s %d: %w", errMinMsg, MaximumMin, ErrInvalidChunksSizePoint)
	}
	if config.maxSize > MaximumMax {
		return nil, fmt.Errorf("the maximum %s %d: %w", errMaxMsg, MaximumMax, ErrInvalidChunksSizePoint)
	}
	if config.bufferSize < config.maxSize {
		return nil, fmt.Errorf("the buffer size must be greater or equal than the maximum cutting point (%d): %w", config.maxSize, ErrInvalidBufferLength)
	}
	if config.minSize >= config.avgSize {
		return nil, fmt.Errorf("the minimum chunks size must be smaller than the average: %w", ErrInvalidChunksSizePoint)
	}
	if config.maxSize <= config.avgSize {
		return nil, fmt.Errorf("the maximum chunks size must be bigger than the average: %w", ErrInvalidChunksSizePoint)
	}
	if config.maxSize-config.minSize <= config.avgSize {
		return nil, fmt.Errorf("maximum - minimum chunks size must be bigger than the average chunk size: %w", ErrInvalidChunksSizePoint)
	}

	bits := logarithm2(config.avgSize)
	maskS := mask(bits + 1)
	maskL := mask(bits - 1)

	return &FastCDC{
		buffer:     make([]byte, config.bufferSize),
		minSize:    config.minSize,
		avgSize:    config.avgSize,
		maxSize:    config.maxSize,
		streamMode: config.stream,
		maskS:      maskS,
		maskL:      maskL,
		ctx:        ctx,
	}, nil
}

// ChunkFn is called by the split function when a chunk is found.
// The chunk is only valid in the callback and must be copied for
// later use.
type ChunkFn func(offset, length uint, chunk []byte) error

// Split take the current reader and try to find chunk of the defined average size. When a chunk is
// found, Split call the callback function with the offset, length and chunk. Split reuse it's
// internal buffer, thereby the chunk is only valid within the callback. For later use, you most perform a copy value
// of the chunk. If the Split is called more than once, the offset represents the position after merging
// all input reader since a chunk can start in one buffer and end in another.
func (f *FastCDC) Split(data io.Reader, fn ChunkFn) error {
	if !f.streamMode && f.firstCall {
		panic("split must not be call multiple time in regular mode, use stream mode instead")
	}
	return f.split(data, fn, nil)
}

// Finalize must be called at the end of the file or stream.
// It return the remaining chunk from the last buffer.
// If finalize is called before split, it wil panic,
func (f *FastCDC) Finalize(fn ChunkFn) error {
	if !f.firstCall {
		panic("finalize most succeed a split, call split first")
	}
	defer func() {
		f.offset = 0
		f.carry = 0
		f.realOffset = 0
		f.previousBytesRead = 0
		f.firstCall = false
	}()
	reader := bytes.NewReader(nil)
	// chunk the remaining part
	if f.streamMode {
		if err := f.split(reader, fn, io.EOF); err != nil {
			return err
		}
	}
	if f.carry > 0 {
		return fn(f.realOffset, f.carry, f.buffer[0:f.carry])
	}
	return nil
}

func (f *FastCDC) split(data io.Reader, fn ChunkFn, eof error) error {
	f.firstCall = true
	for {
		select {
		case <-f.ctx.Done():
			return f.ctx.Err()
		default:
		}

		// Fill the buffer with data but do not erase an eventual carry
		bytesRead, err := data.Read(f.buffer[f.carry+f.previousBytesRead:])
		if err != nil && err != io.EOF {
			return err
		}

		if err == io.EOF && f.previousBytesRead == 0 {
			return nil
		}

		// When finalize is called, we pass EOF to indicate that their is no more data.
		// It's robust because and empty part during a stream will not trigger the chunking
		// process and break the deterministic chunking judgement.
		if f.streamMode {
			// Return until the internal buffer is completely filled or until EOF
			remaining := uint(len(f.buffer)) - f.carry - f.previousBytesRead - uint(bytesRead)
			if remaining != 0 && eof != io.EOF {
				f.previousBytesRead += uint(bytesRead)
				return nil
			}
		}
		// byteReadWithCarry is the real upper bound up to which we must iterate
		bytesReadWithCarry := uint(bytesRead) + f.carry + f.previousBytesRead
		f.offset = f.carry
		f.previousBytesRead = 0

		// Find chunk for the current buffer
		for f.offset < bytesReadWithCarry {
			breakpoint, found := f.breakpoint(f.buffer[f.offset:bytesReadWithCarry])
			if found {
				endOffset := breakpoint + f.offset
				// If there is a carry, the length need to be calculate form the beginning of the buffer
				chunkLength := endOffset - (f.offset - f.carry)
				chunk := f.buffer[f.offset-f.carry : endOffset]
				if err := fn(f.realOffset, chunkLength, chunk); err != nil {
					return err
				}
				f.realOffset += breakpoint + f.carry
				f.carry = 0
				f.offset = endOffset
			} else {
				previousCarry := f.carry
				currentCarry := bytesReadWithCarry - f.offset
				f.carry += bytesReadWithCarry - f.offset

				// for the length of the current carry,
				// copy the part of the buffer where we can't find
				// a chunk to the buffer from the previous carry position
				for i := uint(0); i < currentCarry; i++ {
					f.buffer[previousCarry+i] = f.buffer[f.offset+i]
				}
				break
			}
		}
	}
}

// Breakpoint return the next chunk breakpoint on the buffer.
// If there is no breakpoint found, it return 0, false.
func (f *FastCDC) breakpoint(buffer []byte) (uint, bool) {
	// if there is bytes carried from the last breakpoint call,
	// reduce the expected chunk size to match the required size.
	minSize := minus(f.minSize, f.carry, 1)
	avgSize := minus(f.avgSize, f.carry, 1)
	maxSize := minus(f.maxSize, f.carry, 1)

	bufferLength := uint(len(buffer))

	var hash uint = 0

	// Sub-minimum chunk cut-point skipping
	if bufferLength <= minSize {
		return 0, false
	}

	// Set to min size since we do not want to
	// find a breakpoint bellow the min size
	breakPoint := minSize

	// If the buffer length is bigger than the maxSize
	// use the max size as buffer length. Over time
	// the buffer will become smaller and smaller until
	// it's not possible to find a new chunk or until
	// the buffer length is exactly equals to the maxSize
	if bufferLength > maxSize {
		bufferLength = maxSize
	}

	normalSize := normalSize(avgSize, minSize, bufferLength)

	// Start by using the "harder" chunking judgement to find
	// chunks that run smaller than the desired normal size.
	for breakPoint < normalSize {
		index := uint(buffer[breakPoint])
		breakPoint += 1
		hash = (hash >> 1) + table[index]
		if hash&f.maskS == 0 {
			return breakPoint, true
		}
	}

	// Fall back to using the "easier" chunking judgement to find chunks
	// that run larger than the desired normal size but never bigger than
	// the maxSize.
	for breakPoint < bufferLength {
		index := uint(buffer[breakPoint])
		breakPoint += 1
		hash = (hash >> 1) + table[index]
		if hash&f.maskL == 0 {
			return breakPoint, true
		}
	}

	// We are unable to find an end offset chunk with the chunking judgement
	// At this point breakPoint == bufferLength
	// If the buffer is exactly of the size of the max chunk size, the chunk reach
	// the max size allowed and we should emit.
	// It's also ensure than the carry is never bigger than the max size.
	if breakPoint == maxSize {
		return breakPoint, true
	}

	// If the breakPoint is < maxSize, the buffer we got is too small to find a chunk
	// and we should try with a bigger buffer.
	return 0, false
}

// minus reduce a cut-point with the carry bytes length
// if the carry is >= than the cut-point, minus return the
// min acceptable cut-point.
func minus(point, carry, min uint) uint {
	if carry < point {
		return point - carry
	}
	return min
}

// Find the middle of the desired chunk size. This is what the
// FastCDC paper refer as "normal size", but with a more adaptive
// threshold based on a combination of average and minimum chunk size
// to decide the pivot point at which to switch masks.
// https://github.com/ronomon/deduplication#content-dependent-chunking
func normalSize(average, minimum, sourceSize uint) uint {
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

// Integer division than rounds up instead of down.
func ceilDiv(x, y uint) uint {
	return (x + y - 1) / y
}

// Masks use 1 bit of chunk size normalization instead of 2 bits of chunk size normalization.
// https://github.com/ronomon/deduplication#content-dependent-chunking
func mask(bits uint) uint {
	if bits < 1 {
		panic("bits too low")
	}
	if bits > 31 {
		panic("bits too high")
	}
	return uint(math.Pow(2, float64(bits)) - 1)
}

// Base 2 logarithm
func logarithm2(value uint) uint {
	return uint(math.Round(math.Log2(float64(value))))
}

var table = [256]uint{
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
