package ingestion

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type Outcome int

const (
	OutcomeProcessed Outcome = iota
	OutcomeDuplicate
)

type PermanentError struct {
	Err error
}

func (e *PermanentError) Error() string {
	return "permanent: " + e.Err.Error()
}

func (e *PermanentError) Unwrap() error {
	return e.Err
}

func IsPermanent(err error) bool {
	var perr *PermanentError
	return errors.As(err, &perr)
}

func permanent(err error) error {
	var perr *PermanentError
	if errors.As(err, &perr) {
		return err
	}
	return &PermanentError{Err: err}
}

type IngestionPipeline struct {
	validator *Validator
	dedup     *Deduplicator
	router    *Router
	batches   *BatchWriter
}

func NewIngestionPipeline(validator *Validator, dedup *Deduplicator, router *Router, batches *BatchWriter) *IngestionPipeline {
	return &IngestionPipeline{validator: validator, dedup: dedup, router: router, batches: batches}
}

func (p *IngestionPipeline) Process(ctx context.Context, topic string, partition int32, offset int64, data []byte) (Outcome, error) {
	env, payload, err := DecodeEnvelope(data)
	if err != nil {
		return OutcomeProcessed, permanent(fmt.Errorf("decode: %w", err))
	}
	env.SourceTopic = topic
	env.SourcePartition = partition
	env.SourceOffset = offset
	env.IngestedAt = time.Now().UTC()

	if err := p.validator.ValidateEnvelope(env); err != nil {
		return OutcomeProcessed, permanent(fmt.Errorf("envelope: %w", err))
	}
	if err := p.validator.ValidateSchema(env, payload); err != nil {
		return OutcomeProcessed, permanent(fmt.Errorf("schema: %w", err))
	}

	duplicate, err := p.dedup.IsDuplicate(ctx, env.EventID)
	if err != nil {
		// Fail closed: store errors are transient, never ack past them.
		return OutcomeProcessed, fmt.Errorf("deduplicate: %w", err)
	}
	if duplicate {
		return OutcomeDuplicate, nil
	}

	handler, err := p.router.Route(env.EventType)
	if err != nil {
		return OutcomeProcessed, permanent(fmt.Errorf("route: %w", err))
	}
	batch, err := handler.Handle(ctx, env, payload)
	if err != nil {
		return OutcomeProcessed, permanent(fmt.Errorf("transform: %w", err))
	}
	if _, err := p.batches.Add(ctx, batch); err != nil {
		return OutcomeProcessed, fmt.Errorf("buffer: %w", err)
	}
	return OutcomeProcessed, nil
}
