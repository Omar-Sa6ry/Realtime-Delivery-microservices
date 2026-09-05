package snowflake

import (
	"errors"
	"sync"
	"time"
)

// Snowflake represents a Twitter Snowflake ID generator.
type Snowflake struct {
	mu        sync.Mutex
	timestamp int64
	workerID  int64
	sequence  int64
}

// Config holds the configuration for the Snowflake ID generator.
type Config struct {
	WorkerID      int64 // 0-31
	DatacenterID  int64 // 0-31 (optional, can be combined with WorkerID)
	Epoch         int64 // Custom epoch (default: Twitter's epoch: 1288834974657)
	SequenceBits  uint8 // Number of bits for sequence (default: 12)
}

// DefaultConfig returns a default configuration.
func DefaultConfig() Config {
	return Config{
		Epoch:        1288834974657, // Twitter epoch in milliseconds
		SequenceBits: 12,
	}
}

func NewSnowflake(cfg Config) (*Snowflake, error) {
	if cfg.WorkerID < 0 || cfg.WorkerID > 31 {
		return nil, errors.New("worker_id must be between 0 and 31")
	}
	if cfg.DatacenterID < 0 || cfg.DatacenterID > 31 {
		return nil, errors.New("datacenter_id must be between 0 and 31")
	}
	if cfg.SequenceBits == 0 {
		cfg.SequenceBits = 12
	}
	if cfg.SequenceBits > 12 {
		return nil, errors.New("sequence_bits must not exceed 12")
	}

	workerIDCombined := cfg.WorkerID<<cfg.SequenceBits | cfg.DatacenterID<<(cfg.SequenceBits+5)

	sf := &Snowflake{
		timestamp: time.Now().UnixNano() / 1e6, // current timestamp in ms
		workerID:  workerIDCombined,
		sequence:  0,
	}

	// Initialize to current time to avoid generating IDs in the past
	sf.timestamp = time.Now().UnixNano() / 1e6

	return sf, nil
}

// NextID generates the next Snowflake ID.
func (sf *Snowflake) NextID() int64 {
	sf.mu.Lock()
	defer sf.mu.Unlock()

	now := time.Now().UnixNano() / 1e6 // current timestamp in ms

	// If we're generating IDs too fast (same millisecond),
	// wait until the next millisecond
	if now == sf.timestamp {
		// Wait for next millisecond (simple spin loop)
		for time.Now().UnixNano()/1e6 <= now {
		}
		now = time.Now().UnixNano() / 1e6
	} else {
		sf.timestamp = now
	}

	// Increment sequence
	sf.sequence++

	// If sequence overflows for this millisecond, wait for next millisecond
	if sf.sequence > int64(1<<12-1) {
		for time.Now().UnixNano()/1e6 <= now {
		}
		now = time.Now().UnixNano() / 1e6
		sf.timestamp = now
		sf.sequence = 0
	}

	id := int64((now<<22)|sf.workerID|sf.sequence) & 0x1FFFFFFFFF

	return id
}

// ID returns the generated ID as int64.
func (sf *Snowflake) ID() int64 {
	return sf.NextID()
}

// WorkerID returns the worker ID embedded in the Snowflake.
func (sf *Snowflake) WorkerID() int64 {
	return sf.workerID & 0x1F // 5 bits
}

// Sequence returns the sequence number embedded in the Snowflake.
func (sf *Snowflake) Sequence() int64 {
	return sf.sequence & 0xFFF // 12 bits
}

func Parse(id int64) (int64, int64, int64, int64, error) {
	const (
		timestampBits = 41
		datacenterBits = 5
		workerBits = 5
		sequenceBits = 12
	)

	// Mask for each field
	timestampMask := int64(1<<timestampBits - 1)
	datacenterMask := int64(1<<datacenterBits - 1) << (timestampBits + workerBits)
	workerMask := int64(1<<workerBits - 1) << (timestampBits)
	sequenceMask := int64(1<<sequenceBits - 1)

	// Extract timestamp (remove epoch)
	timestamp := (id & timestampMask)

	// Extract datacenter ID
	datacenterID := (id & datacenterMask) >> (timestampBits + workerBits)

	// Extract worker ID
	workerID := (id & workerMask) >> timestampBits

	// Extract sequence
	sequence := id & sequenceMask

	return timestamp, datacenterID, workerID, sequence, nil
}

// Validate checks if the given ID looks like a valid Snowflake ID.
func Validate(id int64) bool {
	if id <= 0 {
		return false
	}
	if id > 0x1FFFFFFFFF { // 41+5+5+12 = 63 bits, max positive int64
		return false
	}
	return true
}

// DefaultSnowflake is a pre-configured Snowflake using worker ID 1.
// This is useful for quick startup without configuration.
var DefaultSnowflake *Snowflake
var defaultSnowflakeOnce sync.Once

func initDefaultSnowflake() {
	sf, err := NewSnowflake(Config{
		WorkerID: 1,
	})
	if err != nil {
		// Fallback: create with minimal config
		sf, _ = NewSnowflake(Config{})
	}
	DefaultSnowflake = sf
}

// Generate generates a new Snowflake ID using the default generator.
func Generate() int64 {
	defaultSnowflakeOnce.Do(initDefaultSnowflake)
	return DefaultSnowflake.ID()
}