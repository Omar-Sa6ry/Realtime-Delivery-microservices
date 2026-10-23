package replay

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/application/ingestion"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSource struct {
	messages []Message
	index    int
	err      error
}

func (m *mockSource) Next(ctx context.Context) (Message, error) {
	if m.err != nil {
		return Message{}, m.err
	}
	select {
	case <-ctx.Done():
		return Message{}, ctx.Err()
	default:
	}
	if m.index >= len(m.messages) {
		return Message{}, io.EOF
	}
	msg := m.messages[m.index]
	m.index++
	return msg, nil
}

func (m *mockSource) Close() error {
	return nil
}

type mockProcessor struct {
	outcome ingestion.Outcome
	err     error
	called  int
}

func (m *mockProcessor) Process(ctx context.Context, topic string, partition int32, offset int64, data []byte) (ingestion.Outcome, error) {
	m.called++
	return m.outcome, m.err
}

func TestJob_Validate(t *testing.T) {
	tests := []struct {
		name        string
		job         Job
		expectError bool
	}{
		{
			name: "valid job",
			job: Job{
				Topics: []string{"delivery.events"},
				From:   time.Now().UTC().Add(-24 * time.Hour),
				To:     time.Now().UTC(),
			},
			expectError: false,
		},
		{
			name: "empty topics",
			job: Job{
				Topics: []string{},
				From:   time.Now().UTC().Add(-24 * time.Hour),
				To:     time.Now().UTC(),
			},
			expectError: true,
		},
		{
			name: "zero from",
			job: Job{
				Topics: []string{"delivery.events"},
				From:   time.Time{},
				To:     time.Now().UTC(),
			},
			expectError: true,
		},
		{
			name: "zero to",
			job: Job{
				Topics: []string{"delivery.events"},
				From:   time.Now().UTC().Add(-24 * time.Hour),
				To:     time.Time{},
			},
			expectError: true,
		},
		{
			name: "to before from",
			job: Job{
				Topics: []string{"delivery.events"},
				From:   time.Now().UTC(),
				To:     time.Now().UTC().Add(-24 * time.Hour),
			},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.job.Validate()
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestService_Run_Success(t *testing.T) {
	source := &mockSource{
		messages: []Message{
			{Topic: "delivery.events", Partition: 0, Offset: 100, OccurredAt: time.Now().UTC().Add(-1 * time.Hour), Data: []byte(`{"eventId":"evt-1"}`)},
			{Topic: "delivery.events", Partition: 0, Offset: 101, OccurredAt: time.Now().UTC().Add(-30 * time.Minute), Data: []byte(`{"eventId":"evt-2"}`)},
		},
	}

	processor := &mockProcessor{
		outcome: ingestion.OutcomeProcessed,
	}

	svc := NewService()
	job := Job{
		Topics: []string{"delivery.events"},
		From:   time.Now().UTC().Add(-2 * time.Hour),
		To:     time.Now().UTC(),
	}

	result, err := svc.Run(context.Background(), job, source, processor)

	require.NoError(t, err)
	assert.Equal(t, int64(2), result.Processed)
	assert.Equal(t, int64(0), result.Duplicates)
	assert.Equal(t, int64(0), result.Skipped)
	assert.Equal(t, 2, processor.called)
}

func TestService_Run_Duplicates(t *testing.T) {
	source := &mockSource{
		messages: []Message{
			{Topic: "delivery.events", Partition: 0, Offset: 100, OccurredAt: time.Now().UTC().Add(-1 * time.Hour), Data: []byte(`{"eventId":"evt-1"}`)},
			{Topic: "delivery.events", Partition: 0, Offset: 101, OccurredAt: time.Now().UTC().Add(-30 * time.Minute), Data: []byte(`{"eventId":"evt-2"}`)},
		},
	}

	processor := &mockProcessor{
		outcome: ingestion.OutcomeDuplicate,
	}

	svc := NewService()
	job := Job{
		Topics: []string{"delivery.events"},
		From:   time.Now().UTC().Add(-2 * time.Hour),
		To:     time.Now().UTC(),
	}

	result, err := svc.Run(context.Background(), job, source, processor)

	require.NoError(t, err)
	assert.Equal(t, int64(0), result.Processed)
	assert.Equal(t, int64(2), result.Duplicates)
	assert.Equal(t, int64(0), result.Skipped)
}

func TestService_Run_Skipped(t *testing.T) {
	now := time.Now().UTC()
	source := &mockSource{
		messages: []Message{
			{Topic: "delivery.events", Partition: 0, Offset: 100, OccurredAt: now.Add(-3 * time.Hour), Data: []byte(`{"eventId":"evt-1"}`)}, // Before window
			{Topic: "delivery.events", Partition: 0, Offset: 101, OccurredAt: now.Add(-1 * time.Hour), Data: []byte(`{"eventId":"evt-2"}`)},   // In window
			{Topic: "delivery.events", Partition: 0, Offset: 102, OccurredAt: now.Add(1 * time.Hour), Data: []byte(`{"eventId":"evt-3"}`)},    // After window
		},
	}

	processor := &mockProcessor{
		outcome: ingestion.OutcomeProcessed,
	}

	svc := NewService()
	job := Job{
		Topics: []string{"delivery.events"},
		From:   now.Add(-2 * time.Hour),
		To:     now,
	}

	result, err := svc.Run(context.Background(), job, source, processor)

	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Processed)
	assert.Equal(t, int64(0), result.Duplicates)
	assert.Equal(t, int64(2), result.Skipped)
}

func TestService_Run_ProcessorError(t *testing.T) {
	source := &mockSource{
		messages: []Message{
			{Topic: "delivery.events", Partition: 0, Offset: 100, OccurredAt: time.Now().UTC().Add(-1 * time.Hour), Data: []byte(`{"eventId":"evt-1"}`)},
		},
	}

	processor := &mockProcessor{
		err: errors.New("processor error"),
	}

	svc := NewService()
	job := Job{
		Topics: []string{"delivery.events"},
		From:   time.Now().UTC().Add(-2 * time.Hour),
		To:     time.Now().UTC(),
	}

	result, err := svc.Run(context.Background(), job, source, processor)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "processor error")
	assert.Equal(t, int64(0), result.Processed)
}

func TestService_Run_SourceError(t *testing.T) {
	source := &mockSource{
		err: errors.New("source error"),
	}

	processor := &mockProcessor{
		outcome: ingestion.OutcomeProcessed,
	}

	svc := NewService()
	job := Job{
		Topics: []string{"delivery.events"},
		From:   time.Now().UTC().Add(-2 * time.Hour),
		To:     time.Now().UTC(),
	}

	_, err := svc.Run(context.Background(), job, source, processor)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "source error")
}

func TestService_Run_ContextCancelled(t *testing.T) {
	source := &mockSource{
		messages: []Message{
			{Topic: "delivery.events", Partition: 0, Offset: 100, OccurredAt: time.Now().UTC().Add(-1 * time.Hour), Data: []byte(`{"eventId":"evt-1"}`)},
		},
	}

	processor := &mockProcessor{
		outcome: ingestion.OutcomeProcessed,
	}

	svc := NewService()
	job := Job{
		Topics: []string{"delivery.events"},
		From:   time.Now().UTC().Add(-2 * time.Hour),
		To:     time.Now().UTC(),
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := svc.Run(ctx, job, source, processor)

	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestNewService(t *testing.T) {
	svc := NewService()
	assert.NotNil(t, svc)
}