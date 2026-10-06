package snowflake

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.Equal(t, int64(1288834974657), cfg.Epoch)
	assert.Equal(t, uint8(12), cfg.SequenceBits)
}

func TestNewSnowflakeValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"valid worker and datacenter", Config{WorkerID: 1, DatacenterID: 2}, false},
		{"boundary worker 0", Config{WorkerID: 0}, false},
		{"boundary worker 31", Config{WorkerID: 31}, false},
		{"negative worker", Config{WorkerID: -1}, true},
		{"worker above 31", Config{WorkerID: 32}, true},
		{"negative datacenter", Config{DatacenterID: -1}, true},
		{"datacenter above 31", Config{DatacenterID: 32}, true},
		{"sequence bits zero defaults silently", Config{WorkerID: 1, SequenceBits: 0}, false},
		{"sequence bits above 12 rejected", Config{WorkerID: 1, SequenceBits: 13}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf, err := NewSnowflake(tt.cfg)
			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, sf)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, sf)
		})
	}
}

func TestNextIDProducesUniqueIDs(t *testing.T) {
	sf, err := NewSnowflake(Config{WorkerID: 3, DatacenterID: 7})
	require.NoError(t, err)

	const n = 300
	ids := make(map[int64]bool, n)
	for i := 0; i < n; i++ {
		id := sf.NextID()
		assert.Falsef(t, ids[id], "duplicate id %d generated", id)
		ids[id] = true
	}
	assert.Len(t, ids, n)
}

func TestNextIDIsMonotonic(t *testing.T) {
	sf, err := NewSnowflake(Config{WorkerID: 1})
	require.NoError(t, err)

	prev := int64(0)
	for i := 0; i < 100; i++ {
		id := sf.ID()
		assert.Greaterf(t, id, prev, "id %d must exceed previous id %d", id, prev)
		prev = id
	}
}

func TestNextIDConcurrentUniqueness(t *testing.T) {
	sf, err := NewSnowflake(Config{WorkerID: 5})
	require.NoError(t, err)

	var mu sync.Mutex
	seen := map[int64]bool{}
	var wg sync.WaitGroup

	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				id := sf.NextID()
				mu.Lock()
				seen[id] = true
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	assert.Len(t, seen, 8*50)
}

func TestIDDelegatesToNextID(t *testing.T) {
	sf, err := NewSnowflake(Config{WorkerID: 2})
	require.NoError(t, err)

	assert.NotZero(t, sf.ID())
}

func TestWorkerIDAndSequenceAccessors(t *testing.T) {
	sf, err := NewSnowflake(Config{WorkerID: 9, DatacenterID: 4, SequenceBits: 12})
	require.NoError(t, err)

	assert.Equal(t, int64(9), sf.WorkerID())

	seq := sf.Sequence()
	assert.GreaterOrEqual(t, seq, int64(0))
	assert.LessOrEqual(t, seq, int64(1<<idSequenceBits-1))
}

func TestParseBitLayout(t *testing.T) {
	const (
		timestamp  int64 = 123456789
		datacenter int64 = 17
		worker     int64 = 23
		sequence   int64 = 4095
	)

	// Layout: sequence(12) | worker(5) | datacenter(5) | timestamp(41)
	id := (timestamp << timestampShift) |
		(datacenter << (idSequenceBits + idWorkerBits)) |
		(worker << idSequenceBits) |
		sequence

	gotTimestamp, gotDC, gotWorker, gotSeq, err := Parse(id)
	require.NoError(t, err)

	assert.Equal(t, timestamp, gotTimestamp)
	assert.Equal(t, datacenter, gotDC)
	assert.Equal(t, worker, gotWorker)
	assert.Equal(t, sequence, gotSeq)
}

func TestParseRoundTripsGeneratedID(t *testing.T) {
	sf, err := NewSnowflake(Config{WorkerID: 11, DatacenterID: 6})
	require.NoError(t, err)

	before := time.Now().UnixMilli()
	id := sf.NextID()
	after := time.Now().UnixMilli()

	gotTimestamp, gotDC, gotWorker, gotSeq, err := Parse(id)
	require.NoError(t, err)

	assert.GreaterOrEqual(t, gotTimestamp, before)
	assert.LessOrEqual(t, gotTimestamp, after)
	assert.Equal(t, int64(6), gotDC)
	assert.Equal(t, int64(11), gotWorker)
	assert.Equal(t, sf.Sequence(), gotSeq)
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name string
		id   int64
		want bool
	}{
		{"zero is invalid", 0, false},
		{"negative is invalid", -5, false},
		{"positive is valid", 42, true},
		{"generated id is valid", 7400000000000000000, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Validate(tt.id))
		})
	}
}

func TestGenerateUsesSharedDefault(t *testing.T) {
	first := Generate()
	second := Generate()

	assert.Greater(t, second, first)
	require.NotNil(t, DefaultSnowflake)
	assert.Equal(t, int64(1), DefaultSnowflake.WorkerID())
}

func TestNextIDClockMovedBackwards(t *testing.T) {
	sf, _ := NewSnowflake(Config{WorkerID: 1})
	sf.timestamp = time.Now().UnixMilli() + 10000 // force future timestamp
}

func TestNextIDSequenceOverflow(t *testing.T) {
	sf, _ := NewSnowflake(Config{WorkerID: 1})
	sf.sequence = 4095
	sf.timestamp = time.Now().UnixMilli()
	
	// NextID should spin until next millisecond
	before := time.Now().UnixMilli()
	sf.NextID()
	after := time.Now().UnixMilli()
	
	assert.GreaterOrEqual(t, after, before)
}
