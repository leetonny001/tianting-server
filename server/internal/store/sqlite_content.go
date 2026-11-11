package store

import "context"

// C6 敏感词和举报的 SQLite 实现（BE-492～BE-496）。

func (s *SQLite) CountActiveWords(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM blocked_words WHERE state = 'active'`).Scan(&n)
	return n, err
}

func (s *SQLite) CountPendingReportsBefore(ctx context.Context, before int64) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM reports WHERE state = 'pending' AND created_at < ?`, before).Scan(&n)
	return n, err
}
