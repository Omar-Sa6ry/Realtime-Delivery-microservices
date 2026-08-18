package websocket

import (
	"context"
	"fmt"
)

// WsRateLimiter is the rate limiter contract the guard chain depends on.
// Implementations stay in the owning service (e.g. the Redis-backed
// ratelimiter package) — mirrors the TS WsRateLimiter interface.
type WsRateLimiter interface {
	Check(ctx context.Context, userID, action string) (bool, error)
}

// RateLimitFunc adapts a plain function to the WsRateLimiter interface.
type RateLimitFunc func(ctx context.Context, userID, action string) (bool, error)

// Check implements WsRateLimiter.
func (f RateLimitFunc) Check(ctx context.Context, userID, action string) (bool, error) {
	return f(ctx, userID, action)
}

// WsMessageValidator validates a message payload; return an error to reject —
// mirrors the TS WsMessageValidator type.
type WsMessageValidator func(data []byte) error

// WsGuardChainOptions holds per-service validators and rate actions —
// mirrors the TS WsGuardChainOptions interface.
type WsGuardChainOptions struct {
	Validators  map[ClientMessageType]WsMessageValidator
	RateActions map[ClientMessageType]string
}

// WsSocketLike is the minimal socket surface the guard chain needs —
// mirrors the TS WsSocketLike interface.
type WsSocketLike struct {
	UserID string
}

// WsGuardChainContext is the per-message guard context —
// mirrors the TS WsGuardChainContext interface.
type WsGuardChainContext struct {
	Message *ClientMessage
	Socket  WsSocketLike
}

// WsGuardChain is the shared per-message guard pipeline:
// authenticate -> rate limit -> validate.
// Domain-specific validators / rate actions are supplied via WsGuardChainOptions —
// mirrors the TS WsGuardChain class.
type WsGuardChain struct {
	rateLimiter WsRateLimiter
	options     WsGuardChainOptions
}

// NewWsGuardChain creates a guard chain with the given rate limiter and options.
func NewWsGuardChain(rateLimiter WsRateLimiter, options WsGuardChainOptions) *WsGuardChain {
	return &WsGuardChain{rateLimiter: rateLimiter, options: options}
}

// Run executes the full guard pipeline for one inbound message.
func (g *WsGuardChain) Run(ctx context.Context, chainCtx WsGuardChainContext) error {
	if err := g.authenticateStep(chainCtx); err != nil {
		return err
	}
	if err := g.rateLimitStep(ctx, chainCtx); err != nil {
		return err
	}
	return g.validationStep(chainCtx)
}

func (g *WsGuardChain) authenticateStep(chainCtx WsGuardChainContext) error {
	if chainCtx.Socket.UserID == "" {
		return NewWsException(WsErrorUnauthenticated, "Unauthenticated", false)
	}
	return nil
}

func (g *WsGuardChain) rateLimitStep(ctx context.Context, chainCtx WsGuardChainContext) error {
	action, ok := g.options.RateActions[chainCtx.Message.Type]
	if !ok || action == "" {
		return nil
	}

	allowed, err := g.rateLimiter.Check(ctx, chainCtx.Socket.UserID, action)
	if err != nil {
		return err
	}
	if !allowed {
		return NewWsException(
			WsErrorRateLimited,
			fmt.Sprintf("Rate limit exceeded for action: %s", action),
			true,
		)
	}
	return nil
}

func (g *WsGuardChain) validationStep(chainCtx WsGuardChainContext) error {
	validator, ok := g.options.Validators[chainCtx.Message.Type]
	if !ok || validator == nil {
		return nil
	}
	return validator(chainCtx.Message.Data)
}