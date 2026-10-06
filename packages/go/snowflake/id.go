package snowflake

import (
	"errors"
	"sync"
	"time"
)

const (
	idSequenceBits = 12
	idWorkerBits   = 5
	idDatacenter   = 5
	timestampShift = idSequenceBits + idWorkerBits + idDatacenter
)

type Snowflake struct {
	mu        sync.Mutex
	timestamp int64
	workerID  int64
	sequence  int64
	seqBits   uint8
}

type Config struct {
	WorkerID      int64 // 0-31
	DatacenterID  int64 // 0-31 (optional, can be combined with WorkerID)
	Epoch         int64 // Custom epoch (default: Twitter's epoch: 1288834974657)
	SequenceBits  uint8 // Number of bits for sequence (default: 12)
}

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

	workerIDCombined := cfg.WorkerID<<cfg.SequenceBits | cfg.DatacenterID<<(cfg.SequenceBits+idWorkerBits)

	sf := &Snowflake{
		timestamp: time.Now().UnixNano() / 1e6, // current timestamp in ms
		workerID:  workerIDCombined,
		sequence:  0,
		seqBits:   cfg.SequenceBits,
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

	if now == sf.timestamp {
		sf.sequence++
		if sf.sequence > int64(1<<idSequenceBits-1) {
			for time.Now().UnixNano()/1e6 <= now {
			}
			now = time.Now().UnixNano() / 1e6
			sf.timestamp = now
			sf.sequence = 0
		}
	} else {
		sf.timestamp = now
		sf.sequence = 0
	}

	id := int64((now << timestampShift)) | sf.workerID | sf.sequence

	return id
}

func (sf *Snowflake) ID() int64 {
	return sf.NextID()
}

func (sf *Snowflake) WorkerID() int64 {
	return (sf.workerID >> sf.seqBits) & 0x1F // 5 bits
}

func (sf *Snowflake) Sequence() int64 {
	return sf.sequence & (int64(1)<<idSequenceBits - 1) // 12 bits
}

// Parse decomposes an ID produced by NextID.
//
// Layout (little-endian bit fields): sequence(12) | worker(5) | datacenter(5) |
// timestamp(41). The sequence width is the package default of 12 bits.
func Parse(id int64) (int64, int64, int64, int64, error) {
	timestamp := id >> timestampShift
	workerID := (id >> idSequenceBits) & (int64(1)<<idWorkerBits - 1)
	datacenterID := (id >> (idSequenceBits + idWorkerBits)) & (int64(1)<<idDatacenter - 1)
	sequence := id & (int64(1)<<idSequenceBits - 1)

	return timestamp, datacenterID, workerID, sequence, nil
}

// Validate reports whether id could have been produced by NextID. The layout
// spans the whole positive int64 range, so only non-positive values are
// structurally impossible.
func Validate(id int64) bool {
	return id > 0
}

var DefaultSnowflake *Snowflake
var defaultSnowflakeOnce sync.Once

func initDefaultSnowflake() {
	sf, _ := NewSnowflake(Config{
		WorkerID: 1,
	})
	DefaultSnowflake = sf
}

func Generate() int64 {
	defaultSnowflakeOnce.Do(initDefaultSnowflake)
	return DefaultSnowflake.ID()
}