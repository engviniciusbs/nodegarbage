package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

func (s *Store) PutAIReport(ctx context.Context, key, provider, model string, body []byte) error {
	if body == nil {
		body = []byte{}
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO ai_reports(key,provider,model,created_at,body) VALUES(?,?,?,?,?)
		ON CONFLICT(key) DO UPDATE SET provider=excluded.provider, model=excluded.model, created_at=excluded.created_at, body=excluded.body`,
		key, provider, model, time.Now().UnixMilli(), body)
	return err
}

func (s *Store) GetAIReport(ctx context.Context, key string) ([]byte, bool, error) {
	return s.getBlob(ctx, `SELECT body FROM ai_reports WHERE key=?`, key)
}

func (s *Store) GetSetting(ctx context.Context, key string) ([]byte, bool, error) {
	return s.getBlob(ctx, `SELECT value FROM settings WHERE key=?`, key)
}

func (s *Store) SetSetting(ctx context.Context, key string, value []byte) error {
	if value == nil {
		value = []byte{}
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

func (s *Store) getBlob(ctx context.Context, q, key string) ([]byte, bool, error) {
	var b []byte
	err := s.db.QueryRowContext(ctx, q, key).Scan(&b)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return b, true, nil
}
