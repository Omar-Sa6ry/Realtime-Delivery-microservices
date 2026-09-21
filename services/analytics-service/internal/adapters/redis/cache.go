package redis

import (
	"context"
	"errors"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/Omar-Sa6ry/Realtime-Delivery-microservices/services/analytics-service/internal/ports"
)

const keyPrefix = "analytics:"

type Cache struct {
	client *goredis.Client
}

func NewCache(host, port string) *Cache {
	return &Cache{
		client: goredis.NewClient(&goredis.Options{
			Addr:        host + ":" + port,
			DialTimeout: 3 * time.Second,
		}),
	}
}

var _ ports.CacheRepository = (*Cache)(nil)

func namespaced(key string) string {
	return keyPrefix + key
}

func (c *Cache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	val, err := c.client.Get(ctx, namespaced(key)).Bytes()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return nil, false, nil
		}
		return nil, false, nil
	}
	return val, true, nil
}

func (c *Cache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := c.client.Set(ctx, namespaced(key), value, ttl).Err(); err != nil {
		return nil
	}
	return nil
}

func (c *Cache) Delete(ctx context.Context, key string) error {
	if err := c.client.Del(ctx, namespaced(key)).Err(); err != nil {
		return nil
	}
	return nil
}

func (c *Cache) Close() error {
	return c.client.Close()
}
