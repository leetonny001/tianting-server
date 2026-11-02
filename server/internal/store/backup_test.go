package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// convStats 返回每个会话的（消息条数，最大 seq），用于比较备份前后是否一致。
func convStats(t *testing.T, s *SQLite) map[string][2]int64 {
	t.Helper()
	rows, err := s.db.Query(`SELECT conv_id, count(*), max(seq) FROM messages GROUP BY conv_id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string][2]int64{}
	for rows.Next() {
		var conv string
		var n, max int64
		rows.Scan(&conv, &n, &max)
		out[conv] = [2]int64{n, max}
	}
	return out
}

// BE-063 恢复演练：写入 → 备份（服务运行中、并发写入）→ 删除数据库 → 恢复 → 逐会话核对条数和最大 seq。
func TestBackupRestoreDrill(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "im.db")
	s, err := OpenSQLite(dbPath, SQLiteOptions{AutoMigrate: true})
	if err != nil {
		t.Fatal(err)
	}
	save := func(conv, from, cid string) {
		if _, _, err := s.SaveMessage(ctx, Message{MsgID: cid, ClientMsgID: cid, ConvID: conv, From: from, Type: "text", Body: []byte(`{"text":"x"}`), TS: 1}); err != nil {
			t.Error(err)
		}
	}
	for i := range 30 {
		save("c2c:a:b", "a", fmt.Sprintf("ab-%03d", i))
		save("group:g1", "b", fmt.Sprintf("g1-%03d", i))
	}
	s.UpsertUsers(ctx, []User{{UserID: "a", Nickname: "阿"}})
	before := convStats(t, s)

	// 备份期间继续写另一个会话：备份要么包含某条消息，要么不包含，不能是半条。
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := range 50 {
			save("c2c:c:d", "c", fmt.Sprintf("cd-%03d", i))
		}
	}()
	backup := filepath.Join(dir, "backup.db")
	if err := BackupSQLite(ctx, dbPath, backup); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	if info, _ := os.Stat(backup); info.Mode().Perm() != 0o600 {
		t.Errorf("backup permissions = %v, want 0600", info.Mode().Perm())
	}
	if err := BackupSQLite(ctx, dbPath, backup); err == nil {
		t.Error("backup must not overwrite an existing file")
	}
	s.Close()

	// 删库。
	for _, f := range []string{dbPath, dbPath + "-wal", dbPath + "-shm"} {
		os.Remove(f)
	}
	if err := RestoreSQLite(ctx, backup, dbPath, false); err != nil {
		t.Fatal(err)
	}
	r, err := OpenSQLite(dbPath, SQLiteOptions{}) // 恢复后的库不需要再迁移
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	after := convStats(t, r)
	for conv, want := range before {
		if after[conv] != want {
			t.Errorf("%s: after restore %v, before backup %v", conv, after[conv], want)
		}
	}
	// 并发写入的会话：seq 必须从 1 连续到条数（快照不会出现空洞）。
	if cd := after["c2c:c:d"]; cd[0] != cd[1] {
		t.Errorf("c2c:c:d in backup: count %d, max seq %d (must be contiguous)", cd[0], cd[1])
	}
	if u, err := r.GetUser(ctx, "a"); err != nil || u.Nickname != "阿" {
		t.Errorf("user after restore: %+v %v", u, err)
	}
	// 恢复后可以继续写，seq 接着备份里的最大值。
	m, _, err := r.SaveMessage(ctx, Message{MsgID: "new", ClientMsgID: "new-0001", ConvID: "c2c:a:b", From: "a", Type: "text", Body: []byte(`{}`), TS: 1})
	if err != nil || m.Seq != 31 {
		t.Errorf("save after restore: seq=%d err=%v, want 31", m.Seq, err)
	}
}

func TestRestoreRefusesBadInput(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "im.db")
	s, _ := OpenSQLite(dbPath, SQLiteOptions{AutoMigrate: true})
	s.Close()
	backup := filepath.Join(dir, "b.db")
	if err := BackupSQLite(ctx, dbPath, backup); err != nil {
		t.Fatal(err)
	}

	if err := RestoreSQLite(ctx, backup, dbPath, false); err == nil || !strings.Contains(err.Error(), "-force") {
		t.Errorf("restore over existing db without force: %v", err)
	}
	if err := RestoreSQLite(ctx, backup, dbPath, true); err != nil {
		t.Errorf("restore with force: %v", err)
	}

	garbage := filepath.Join(dir, "garbage.db")
	os.WriteFile(garbage, []byte("this is not a database"), 0o600)
	if err := RestoreSQLite(ctx, garbage, filepath.Join(dir, "x.db"), false); err == nil {
		t.Error("restore from a corrupt file must fail")
	}
	if err := RestoreSQLite(ctx, filepath.Join(dir, "missing.db"), filepath.Join(dir, "y.db"), false); err == nil {
		t.Error("restore from a missing file must fail")
	}
	if _, err := os.Stat(filepath.Join(dir, "y.db")); err == nil {
		t.Error("failed restore must not create the target")
	}
}
