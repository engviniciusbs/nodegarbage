package store

import (
	"context"
	"time"
)

// CleanupRecord is one entry of the deletion history.
type CleanupRecord struct {
	Path                                 string
	Size                                 int64
	Mode, Source, RuleID, Status, Reason string
	Freed                                int64
	At                                   time.Time
}

func (s *Store) AddCleanup(ctx context.Context, r CleanupRecord) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO cleanups(path,size,mode,source,rule_id,status,reason,freed,at) VALUES(?,?,?,?,?,?,?,?,?)`,
		r.Path, r.Size, r.Mode, r.Source, r.RuleID, r.Status, r.Reason, r.Freed, r.At.UnixMilli())
	return err
}

// ListCleanups returns the most recent records first; limit <= 0 means no limit.
func (s *Store) ListCleanups(ctx context.Context, limit int) ([]CleanupRecord, error) {
	if limit <= 0 {
		limit = -1
	}
	rows, err := s.db.QueryContext(ctx, `SELECT path,size,mode,source,rule_id,status,reason,freed,at FROM cleanups ORDER BY at DESC, id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CleanupRecord
	for rows.Next() {
		var r CleanupRecord
		var at int64
		if err := rows.Scan(&r.Path, &r.Size, &r.Mode, &r.Source, &r.RuleID, &r.Status, &r.Reason, &r.Freed, &at); err != nil {
			return nil, err
		}
		r.At = time.UnixMilli(at).UTC()
		out = append(out, r)
	}
	return out, rows.Err()
}

// TotalFreed sums Freed over records with Status "ok".
func (s *Store) TotalFreed(ctx context.Context) (int64, error) {
	var n int64
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(freed),0) FROM cleanups WHERE status='ok'`).Scan(&n)
	return n, err
}
