package fastcdc

type Option func(*config)

type config struct {
	bufferSize uint
	minSize    uint
	avgSize    uint
	maxSize    uint
}

func defaultConfig() *config {
	return &config{
		minSize: 16_384,
		avgSize: 65_536,
		maxSize: 524_288,
	}
}

// WithBufferSize set the internal buffer size. It must be at least
// equal to the max chunk size. The buffer size has no impact on the
// chunk output, but a bigger buffer reduces the internal copying and
// can speed up the chunking.
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

// With16kChunks set the 16kb average chunks size preset.
func With16kChunks() Option {
	return func(c *config) {
		c.minSize = 4096
		c.avgSize = 16_384
		c.maxSize = 131_072
	}
}

// With32kChunks set the 32kb average chunks size preset.
func With32kChunks() Option {
	return func(c *config) {
		c.minSize = 8192
		c.avgSize = 32_768
		c.maxSize = 262_144
	}
}

// With64kChunks set the 64kb average chunks size preset.
// It's the default and recommended chunks size
// for optimal end-to-end deduplication and compression.
// https://www.usenix.org/system/files/conference/atc12/atc12-final293.pdf
func With64kChunks() Option {
	return func(c *config) {
		c.minSize = 16_384
		c.avgSize = 65_536
		c.maxSize = 524_288
	}
}
