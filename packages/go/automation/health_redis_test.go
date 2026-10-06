package automation

import (
	"context"
	"errors"
	"testing"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
)

type stubRedisPinger struct {
	err error
}

func (s stubRedisPinger) Ping(ctx context.Context) *redis.StatusCmd {
	cmd := redis.NewStatusCmd(ctx)
	if s.err != nil {
		cmd.SetErr(s.err)
		return cmd
	}
	cmd.SetVal("PONG")
	return cmd
}

func TestCheckRedisUp(t *testing.T) {
	status := CheckRedis(context.Background(), stubRedisPinger{})
	assert.Equal(t, "UP", status.Status)
	assert.Empty(t, status.Message)
}

func TestCheckRedisPingFailure(t *testing.T) {
	status := CheckRedis(context.Background(), stubRedisPinger{err: errors.New("connection refused")})
	assert.Equal(t, "DOWN", status.Status)
	assert.Equal(t, "connection refused", status.Message)
}
