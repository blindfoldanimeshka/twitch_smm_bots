package storage

import (
	"context"
	"fmt"
)

// Ping проверяет соединение с базой данных
func (s *Store) Ping(ctx context.Context) error {
	if s.pool == nil {
		return fmt.Errorf("database pool is nil")
	}

	if err := s.pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	return nil
}
