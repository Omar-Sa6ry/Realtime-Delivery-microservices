package redis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/search-service/internal/config"
	"github.com/redis/go-redis/v9"
)

type Cache struct {
	client *redis.Client
	ttl    time.Duration
}

func NewCache(cfg *config.Config) *Cache {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr(),
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	return &Cache{
		client: rdb,
		ttl:    cfg.RedisTTL,
	}
}

func (c *Cache) Get(ctx context.Context, key string, dest interface{}) bool {
	val, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		return false
	}
	return json.Unmarshal(val, dest) == nil
}

func (c *Cache) Set(ctx context.Context, key string, value interface{}, ttl ...time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	t := c.ttl
	if len(ttl) > 0 && ttl[0] > 0 {
		t = ttl[0]
	}
	return c.client.Set(ctx, key, data, t).Err()
}

func (c *Cache) GenerateKey(prefix string, input interface{}) string {
	b, _ := json.Marshal(input)
	hash := sha256.Sum256(b)
	return fmt.Sprintf("search:%s:%s", prefix, hex.EncodeToString(hash[:]))
}

func (c *Cache) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}
