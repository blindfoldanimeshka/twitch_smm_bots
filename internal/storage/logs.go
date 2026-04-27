package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

type BotLog struct {
	ID        int64
	BotID     string
	LogType   string
	Level     string
	Message   string
	Context   map[string]interface{}
	CreatedAt time.Time
}

type SettingsLog struct {
	ID        int64
	Level     string
	Message   string
	Changes   map[string]interface{}
	CreatedAt time.Time
}

func (s *Store) CreateBotLog(ctx context.Context, botLog BotLog) error {
	ctx, cancel := context.WithTimeout(ctx, dbOperationTimeout)
	defer cancel()

	contextJSON, err := json.Marshal(botLog.Context)
	if err != nil {
		return fmt.Errorf("marshal context: %w", err)
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO bot_logs (bot_id, log_type, level, message, context)
		VALUES ($1, $2, $3, $4, $5)`,
		botLog.BotID, botLog.LogType, botLog.Level, botLog.Message, contextJSON,
	)
	if err != nil {
		return fmt.Errorf("create bot log: %w", err)
	}
	return nil
}

func (s *Store) GetBotLogs(ctx context.Context, botID string, limit int) ([]BotLog, error) {
	ctx, cancel := context.WithTimeout(ctx, dbOperationTimeout)
	defer cancel()

	if limit <= 0 {
		limit = 100
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id, bot_id, log_type, level, message, context, created_at
		FROM bot_logs
		WHERE bot_id = $1
		ORDER BY created_at DESC
		LIMIT $2`,
		botID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query bot logs: %w", err)
	}
	defer rows.Close()

	result := make([]BotLog, 0)
	for rows.Next() {
		var bl BotLog
		var contextJSON []byte

		if err := rows.Scan(
			&bl.ID,
			&bl.BotID,
			&bl.LogType,
			&bl.Level,
			&bl.Message,
			&contextJSON,
			&bl.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan bot log: %w", err)
		}

		if len(contextJSON) > 0 {
			if err := json.Unmarshal(contextJSON, &bl.Context); err != nil {
				return nil, fmt.Errorf("unmarshal context: %w", err)
			}
		}

		result = append(result, bl)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate bot logs: %w", err)
	}

	return result, nil
}

func (s *Store) CreateSettingsLog(ctx context.Context, settingsLog SettingsLog) error {
	ctx, cancel := context.WithTimeout(ctx, dbOperationTimeout)
	defer cancel()

	changesJSON, err := json.Marshal(settingsLog.Changes)
	if err != nil {
		return fmt.Errorf("marshal changes: %w", err)
	}

	_, err = s.pool.Exec(ctx, `
		INSERT INTO settings_logs (level, message, changes)
		VALUES ($1, $2, $3)`,
		settingsLog.Level, settingsLog.Message, changesJSON,
	)
	if err != nil {
		return fmt.Errorf("create settings log: %w", err)
	}
	return nil
}

func (s *Store) GetSettingsLogs(ctx context.Context, limit int) ([]SettingsLog, error) {
	ctx, cancel := context.WithTimeout(ctx, dbOperationTimeout)
	defer cancel()

	if limit <= 0 {
		limit = 100
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id, level, message, changes, created_at
		FROM settings_logs
		ORDER BY created_at DESC
		LIMIT $1`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query settings logs: %w", err)
	}
	defer rows.Close()

	result := make([]SettingsLog, 0)
	for rows.Next() {
		var sl SettingsLog
		var changesJSON []byte

		if err := rows.Scan(
			&sl.ID,
			&sl.Level,
			&sl.Message,
			&changesJSON,
			&sl.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan settings log: %w", err)
		}

		if len(changesJSON) > 0 {
			if err := json.Unmarshal(changesJSON, &sl.Changes); err != nil {
				return nil, fmt.Errorf("unmarshal changes: %w", err)
			}
		}

		result = append(result, sl)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate settings logs: %w", err)
	}

	return result, nil
}

func (s *Store) CleanupOldLogs(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, dbOperationTimeout)
	defer cancel()

	cutoffTime := time.Now().Add(-24 * time.Hour)

	botLogsCmd, err := s.pool.Exec(ctx, `
		DELETE FROM bot_logs
		WHERE created_at < $1`,
		cutoffTime,
	)
	if err != nil {
		return fmt.Errorf("cleanup bot logs: %w", err)
	}

	settingsLogsCmd, err := s.pool.Exec(ctx, `
		DELETE FROM settings_logs
		WHERE created_at < $1`,
		cutoffTime,
	)
	if err != nil {
		return fmt.Errorf("cleanup settings logs: %w", err)
	}

	botLogsDeleted := botLogsCmd.RowsAffected()
	settingsLogsDeleted := settingsLogsCmd.RowsAffected()

	log.Printf("Cleaned up %d bot logs and %d settings logs older than 24 hours", botLogsDeleted, settingsLogsDeleted)

	return nil
}
