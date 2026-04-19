package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Client struct {
	rdb *redis.Client
}

func NewClient(redisURL string) (*Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}

	rdb := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}

	return &Client{rdb: rdb}, nil
}

func (c *Client) Close() error {
	if c.rdb != nil {
		return c.rdb.Close()
	}
	return nil
}

func (c *Client) IncrementRateLimit(ctx context.Context, key string, window time.Duration) (int64, error) {
	pipe := c.rdb.Pipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, window)

	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}

	return incr.Val(), nil
}

func (c *Client) GetRateLimit(ctx context.Context, key string) (int64, error) {
	val, err := c.rdb.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}

func (c *Client) SetBotStatus(ctx context.Context, botID string, status string, ttl time.Duration) error {
	key := fmt.Sprintf("bot:status:%s", botID)
	return c.rdb.Set(ctx, key, status, ttl).Err()
}

func (c *Client) GetBotStatus(ctx context.Context, botID string) (string, error) {
	key := fmt.Sprintf("bot:status:%s", botID)
	val, err := c.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	return val, err
}

func (c *Client) AddMessage(ctx context.Context, botID string, message string, ttl time.Duration) error {
	key := fmt.Sprintf("bot:messages:%s", botID)
	pipe := c.rdb.Pipeline()
	pipe.LPush(ctx, key, message)
	pipe.LTrim(ctx, key, 0, 49)
	pipe.Expire(ctx, key, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

func (c *Client) GetMessages(ctx context.Context, botID string, limit int64) ([]string, error) {
	key := fmt.Sprintf("bot:messages:%s", botID)
	return c.rdb.LRange(ctx, key, 0, limit-1).Result()
}
