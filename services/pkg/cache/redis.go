// Package cache wraps the Redis client used for sessions, caching and rate limits.
package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Cache wraps a Redis client.
type Cache struct {
	Client *redis.Client
}

// Connect parses a redis:// URL, opens a client and verifies connectivity.
func Connect(ctx context.Context, url string) (*Cache, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}

	client := redis.NewClient(opts)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return &Cache{Client: client}, nil
}

// Close closes the Redis client.
func (c *Cache) Close() error {
	if c.Client != nil {
		return c.Client.Close()
	}
	return nil
}
