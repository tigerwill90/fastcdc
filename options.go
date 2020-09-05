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
		bufferSize: 1 * 1024 * 1024,
		minSize:    32_768,
		avgSize:    65_536,
		maxSize:    131_072,
	}
}

// Set the internal buffer size.
// It must be at least equal to the max chunk size.
// Two different buffer size for two identical size
// may output chunk of different size.
func WithBufferSize(n uint) Option {
	return func(c *config) {
		c.bufferSize = n
	}
}

// Set custom chunks size.
func WithChunksSize(min, avg, max uint) Option {
	return func(c *config) {
		c.minSize = min
		c.avgSize = avg
		c.maxSize = max
	}
}

// Set the 16k average chunks size preset.
func With16kChunks() Option {
	return func(c *config) {
		c.minSize = 8192
		c.avgSize = 16_834
		c.maxSize = 32_768
	}
}

// Set the 32k average chunks size preset.
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
// When active, the chunking output will vary depending of the internal buffer size so you
// need to stick with a predefined size in order to get deterministic chunk length.
func WithAdaptiveThreshold() Option {
	return func(c *config) {
		c.optimization = true
	}
}
