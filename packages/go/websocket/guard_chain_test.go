package websocket

import (
	"context"
	"errors"
	"testing"
)

func TestWsGuardChainAuthenticateStep(t *testing.T) {
	chain := NewWsGuardChain(RateLimitFunc(func(ctx context.Context, userID, action string) (bool, error) {
		return true, nil
	}), WsGuardChainOptions{})

	msg, err := ParseClientMessage([]byte(`{"type":"LOCATION_UPDATE"}`))
	if err != nil {
		t.Fatalf("failed to parse message: %v", err)
	}

	err = chain.Run(context.Background(), WsGuardChainContext{Message: msg, Socket: WsSocketLike{UserID: ""}})
	if err == nil {
		t.Fatal("expected UNAUTHENTICATED error for empty user")
	}
	wsErr, ok := err.(*WsException)
	if !ok || wsErr.Code != WsErrorUnauthenticated {
		t.Errorf("expected WsException UNAUTHENTICATED, got %v", err)
	}
}

func TestWsGuardChainRateLimitStep(t *testing.T) {
	chain := NewWsGuardChain(RateLimitFunc(func(ctx context.Context, userID, action string) (bool, error) {
		if action == "location" {
			return false, nil
		}
		return true, nil
	}), WsGuardChainOptions{
		RateActions: map[ClientMessageType]string{
			ClientMessageLocationUpdate: "location",
		},
	})

	msg, err := ParseClientMessage([]byte(`{"type":"LOCATION_UPDATE"}`))
	if err != nil {
		t.Fatalf("failed to parse message: %v", err)
	}

	err = chain.Run(context.Background(), WsGuardChainContext{Message: msg, Socket: WsSocketLike{UserID: "u-1"}})
	if err == nil {
		t.Fatal("expected RATE_LIMITED error")
	}
	wsErr, ok := err.(*WsException)
	if !ok || wsErr.Code != WsErrorRateLimited || !wsErr.Retryable {
		t.Errorf("expected retryable WsException RATE_LIMITED, got %v", err)
	}
}

func TestWsGuardChainRateLimiterError(t *testing.T) {
	boom := errors.New("redis down")
	chain := NewWsGuardChain(RateLimitFunc(func(ctx context.Context, userID, action string) (bool, error) {
		return false, boom
	}), WsGuardChainOptions{
		RateActions: map[ClientMessageType]string{
			ClientMessageLocationUpdate: "location",
		},
	})

	msg, err := ParseClientMessage([]byte(`{"type":"LOCATION_UPDATE"}`))
	if err != nil {
		t.Fatalf("failed to parse message: %v", err)
	}

	err = chain.Run(context.Background(), WsGuardChainContext{Message: msg, Socket: WsSocketLike{UserID: "u-1"}})
	if !errors.Is(err, boom) {
		t.Errorf("expected underlying rate limiter error, got %v", err)
	}
}

func TestWsGuardChainValidationStep(t *testing.T) {
	validationErr := NewWsException(WsErrorInvalidMessage, "bad payload", false)
	chain := NewWsGuardChain(RateLimitFunc(func(ctx context.Context, userID, action string) (bool, error) {
		return true, nil
	}), WsGuardChainOptions{
		Validators: map[ClientMessageType]WsMessageValidator{
			ClientMessageLocationUpdate: func(data []byte) error {
				if len(data) == 0 {
					return validationErr
				}
				return nil
			},
		},
	})

	t.Run("validator rejects empty payload", func(t *testing.T) {
		msg, err := ParseClientMessage([]byte(`{"type":"LOCATION_UPDATE"}`))
		if err != nil {
			t.Fatalf("failed to parse message: %v", err)
		}
		err = chain.Run(context.Background(), WsGuardChainContext{Message: msg, Socket: WsSocketLike{UserID: "u-1"}})
		if !errors.Is(err, validationErr) {
			t.Errorf("expected validator error, got %v", err)
		}
	})

	t.Run("validator accepts valid payload", func(t *testing.T) {
		msg, err := ParseClientMessage([]byte(`{"type":"LOCATION_UPDATE","data":{"lat":1}}`))
		if err != nil {
			t.Fatalf("failed to parse message: %v", err)
		}
		err = chain.Run(context.Background(), WsGuardChainContext{Message: msg, Socket: WsSocketLike{UserID: "u-1"}})
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})
}

func TestWsGuardChainNoRateAction(t *testing.T) {
	called := false
	chain := NewWsGuardChain(RateLimitFunc(func(ctx context.Context, userID, action string) (bool, error) {
		called = true
		return true, nil
	}), WsGuardChainOptions{})

	msg, err := ParseClientMessage([]byte(`{"type":"PING"}`))
	if err != nil {
		t.Fatalf("failed to parse message: %v", err)
	}

	if err := chain.Run(context.Background(), WsGuardChainContext{Message: msg, Socket: WsSocketLike{UserID: "u-1"}}); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if called {
		t.Error("rate limiter should not be called when no rate action is configured")
	}
}