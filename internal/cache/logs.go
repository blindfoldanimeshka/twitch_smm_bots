package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"twitch-smm-bot/internal/storage"
)

const (
	botLogsKeyPrefix      = "bot_logs:"
	settingsLogsKey       = "settings_logs"
	logCacheTTL           = 1 * time.Hour
	maxCachedLogsPerBot   = 100
	maxCachedSettingsLogs = 50
)

func (c *Client) AddBotLog(ctx context.Context, botID string, log storage.BotLog) error {
	key := botLogsKeyPrefix + botID

	logJSON, err := json.Marshal(log)
	if err != nil {
		return fmt.Errorf("marshal bot log: %w", err)
	}

	pipe := c.rdb.Pipeline()
	pipe.LPush(ctx, key, logJSON)
	pipe.LTrim(ctx, key, 0, maxCachedLogsPerBot-1)
	pipe.Expire(ctx, key, logCacheTTL)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("add bot log for %s: %w", botID, err)
	}

	return nil
}

func (c *Client) GetBotLogs(ctx context.Context, botID string, limit int) ([]storage.BotLog, error) {
	if limit <= 0 {
		limit = maxCachedLogsPerBot
	}

	key := botLogsKeyPrefix + botID

	results, err := c.rdb.LRange(ctx, key, 0, int64(limit-1)).Result()
	if err == redis.Nil {
		return []storage.BotLog{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get bot logs for %s: %w", botID, err)
	}

	logs := make([]storage.BotLog, 0, len(results))
	for _, result := range results {
		var log storage.BotLog
		if err := json.Unmarshal([]byte(result), &log); err != nil {
			continue
		}
		logs = append(logs, log)
	}

	return logs, nil
}

func (c *Client) AddSettingsLog(ctx context.Context, log storage.SettingsLog) error {
	logJSON, err := json.Marshal(log)
	if err != nil {
		return fmt.Errorf("marshal settings log: %w", err)
	}

	pipe := c.rdb.Pipeline()
	pipe.LPush(ctx, settingsLogsKey, logJSON)
	pipe.LTrim(ctx, settingsLogsKey, 0, maxCachedSettingsLogs-1)
	pipe.Expire(ctx, settingsLogsKey, logCacheTTL)

	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("add settings log: %w", err)
	}

	return nil
}

func (c *Client) GetSettingsLogs(ctx context.Context, limit int) ([]storage.SettingsLog, error) {
	if limit <= 0 {
		limit = maxCachedSettingsLogs
	}

	results, err := c.rdb.LRange(ctx, settingsLogsKey, 0, int64(limit-1)).Result()
	if err == redis.Nil {
		return []storage.SettingsLog{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get settings logs: %w", err)
	}

	logs := make([]storage.SettingsLog, 0, len(results))
	for _, result := range results {
		var log storage.SettingsLog
		if err := json.Unmarshal([]byte(result), &log); err != nil {
			continue
		}
		logs = append(logs, log)
	}

	return logs, nil
}
