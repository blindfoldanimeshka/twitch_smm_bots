package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// BotMessage представляет сообщение бота для логирования
type BotMessage struct {
	BotID       string    `json:"bot_id"`
	Message     string    `json:"message"`
	Personality string    `json:"personality"`
	Timestamp   time.Time `json:"timestamp"`
}

// AddBotMessageLog добавляет сообщение в лог бота
func (c *Client) AddBotMessageLog(ctx context.Context, botID, message, personality string) error {
	msg := BotMessage{
		BotID:       botID,
		Message:     message,
		Personality: personality,
		Timestamp:   time.Now(),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("bot:%s:messages", botID)

	// Добавляем в список (последние 100 сообщений)
	pipe := c.rdb.Pipeline()
	pipe.LPush(ctx, key, data)
	pipe.LTrim(ctx, key, 0, 99) // Храним только последние 100
	pipe.Expire(ctx, key, 24*time.Hour)

	_, err = pipe.Exec(ctx)
	return err
}

// GetBotMessageLog получает последние N сообщений бота
func (c *Client) GetBotMessageLog(ctx context.Context, botID string, limit int64) ([]BotMessage, error) {
	if limit <= 0 {
		limit = 20
	}

	key := fmt.Sprintf("bot:%s:messages", botID)
	data, err := c.rdb.LRange(ctx, key, 0, limit-1).Result()
	if err != nil {
		return nil, err
	}

	messages := make([]BotMessage, 0, len(data))
	for _, item := range data {
		var msg BotMessage
		if err := json.Unmarshal([]byte(item), &msg); err != nil {
			continue
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

// IncrementBotStats увеличивает счетчик сообщений бота
func (c *Client) IncrementBotStats(ctx context.Context, botID string) error {
	key := fmt.Sprintf("bot:%s:stats:messages", botID)
	return c.rdb.Incr(ctx, key).Err()
}

// GetBotStats получает статистику бота
func (c *Client) GetBotStats(ctx context.Context, botID string) (int64, error) {
	key := fmt.Sprintf("bot:%s:stats:messages", botID)
	return c.rdb.Get(ctx, key).Int64()
}

// GetGlobalMessageCount получает общее количество сообщений за последнюю минуту
func (c *Client) GetGlobalMessageCount(ctx context.Context) (int64, error) {
	return c.rdb.Get(ctx, GlobalRateLimitKey).Int64()
}

