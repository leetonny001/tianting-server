package store

import (
	"context"
	"path/filepath"
	"testing"
)

// BE-241：今日新增用户的区间是 [from, to)。created_at 由存储层用真实时间写入，这里直接改值测边界。
func TestAdminStatsNewUsersBoundary(t *testing.T) {
	s, err := OpenSQLite(filepath.Join(t.TempDir(), "im.db"), SQLiteOptions{AutoMigrate: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	ctx := context.Background()
	for id, at := range map[string]int64{"before": 999, "first": 1000, "last": 1999, "after": 2000} {
		if err := s.EnsureUser(ctx, id); err != nil {
			t.Fatal(err)
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE users SET created_at = ? WHERE user_id = ?`, at, id); err != nil {
			t.Fatal(err)
		}
	}
	st, err := s.AdminStats(ctx, 1000, 2000)
	if err != nil {
		t.Fatal(err)
	}
	if st.NewUsers != 2 {
		t.Errorf("NewUsers = %d, want 2", st.NewUsers)
	}
}
