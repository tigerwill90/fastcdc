package fastcdc

type Option func(*config)

type config struct {
	bufferSize   uint
	minSize      uint
	avgSize      uint
	maxSize      uint
	stream       bool
	optimization bool
}

func defaultConfig() *config {
	return &config{
		minSize: 32_768,
		avgSize: 65_536,
		maxSize: 131_072,
	}
}

// WithBufferSize set the internal buffer size.
// Increasing the buffer size can speed up the chunking.
// It must be at least equal to the max chunk size and
// internally a correction of maximum "max size - 1" may
// be added to the buffer to guarantees the chunk output stay
// the same whatever the buffer size is set to.
// Default is set to 2 * max size.
func WithBufferSize(n uint) Option {
	return func(c *config) {
		c.bufferSize = n
	}
}

// WithChunksSize set custom chunk size.
func WithChunksSize(min, avg, max uint) Option {
	return func(c *config) {
		c.minSize = min
		c.avgSize = avg
		c.maxSize = max
	}
}

// With16kChunks set the 16k average chunks size preset.
func With16kChunks() Option {
	return func(c *config) {
		c.minSize = 8192
		c.avgSize = 16_834
		c.maxSize = 32_768
	}
}

// With32kChunks set the 32k average chunks size preset.
func With32kChunks() Option {
	return func(c *config) {
		c.minSize = 16384
		c.avgSize = 32_768
		c.maxSize = 65_536
	}
}

// With64kChunks set the 64k average chunks size preset.
// It's the default and recommended chunks size
// for optimal end-to-end deduplication and compression.
// https://www.usenix.org/system/files/conference/atc12/atc12-final293.pdf
func With64kChunks() Option {
	return func(c *config) {
		c.minSize = 32_768
		c.avgSize = 65_536
		c.maxSize = 131_072
	}
}

// WithStreamMode set the chunker in stream mode.
func WithStreamMode() Option {
	return func(c *config) {
		c.stream = true
	}
}

// WithAdaptiveThreshold activate adaptive threshold optimization.
// A larger minimum chunk size now switches from the strict mask to the eager mask earlier.
// This optimization speed up the chunking process.
func WithAdaptiveThreshold() Option {
	return func(c *config) {
		c.optimization = true
	}
}
