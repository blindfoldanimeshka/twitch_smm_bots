package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Temp mail sessions store the Guerrilla sid_token blob under
// "tempmail:<id>". The payload never leaves Redis and errors here only ever
// quote the opaque id.
func tempMailSessionKey(id string) string {
	return fmt.Sprintf("tempmail:%s", id)
}

// SetTempMailSession stores the session payload with a TTL; a zero ttl means
// no expiry and is a caller bug, not a supported case.
func (c *Client) SetTempMailSession(ctx context.Context, id string, data []byte, ttl time.Duration) error {
	if err := c.rdb.Set(ctx, tempMailSessionKey(id), data, ttl).Err(); err != nil {
		return fmt.Errorf("set temp mail session %s: %w", id, err)
	}
	return nil
}

// GetTempMailSession returns the stored payload; a missing key is (nil, nil).
func (c *Client) GetTempMailSession(ctx context.Context, id string) ([]byte, error) {
	val, err := c.rdb.Get(ctx, tempMailSessionKey(id)).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get temp mail session %s: %w", id, err)
	}
	return []byte(val), nil
}

// DeleteTempMailSession drops the key. Deleting a missing key is not an error.
func (c *Client) DeleteTempMailSession(ctx context.Context, id string) error {
	if err := c.rdb.Del(ctx, tempMailSessionKey(id)).Err(); err != nil {
		return fmt.Errorf("delete temp mail session %s: %w", id, err)
	}
	return nil
}
