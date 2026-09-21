package replay

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/application/ingestion"
)

type Message struct {
	Topic      string
	Partition  int32
	Offset     int64
	OccurredAt time.Time
	Data       []byte
}

type Source interface {
	Next(ctx context.Context) (Message, error)
	Close() error
}

type Processor interface {
	Process(ctx context.Context, topic string, partition int32, offset int64, data []byte) (ingestion.Outcome, error)
}

type Job struct {
	Topics []string
	From   time.Time
	To     time.Time
}

func (j Job) Validate() error {
	if len(j.Topics) == 0 {
		return fmt.Errorf("replay: no topics")
	}
	if j.From.IsZero() || j.To.IsZero() || !j.To.After(j.From) {
		return fmt.Errorf("replay: invalid window")
	}
	return nil
}

type Result struct {
	Processed  int64
	Duplicates int64
	Skipped    int64
	WindowFrom time.Time
	WindowTo   time.Time
}

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Run(ctx context.Context, job Job, source Source, processor Processor) (Result, error) {
	if err := job.Validate(); err != nil {
		return Result{}, err
	}
	var res Result
	res.WindowFrom = job.From
	res.WindowTo = job.To
	for {
		msg, err := source.Next(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return res, nil
			}
			return res, fmt.Errorf("replay source: %w", err)
		}
		if msg.OccurredAt.Before(job.From) || !msg.OccurredAt.Before(job.To) {
			res.Skipped++
			continue
		}
		outcome, err := processor.Process(ctx, msg.Topic, msg.Partition, msg.Offset, msg.Data)
		if err != nil {
			return res, fmt.Errorf("replay process %s/%d@%d: %w", msg.Topic, msg.Partition, msg.Offset, err)
		}
		switch outcome {
		case ingestion.OutcomeDuplicate:
			res.Duplicates++
		default:
			res.Processed++
		}
	}
}
