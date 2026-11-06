package store

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

func testMigrations(t *testing.T, files map[string]string) []Migration {
	t.Helper()
	fsys := fstest.MapFS{}
	for name, sql := range files {
		fsys["m/"+name] = &fstest.MapFile{Data: []byte(sql)}
	}
	ms, err := LoadMigrations(fsys, "m")
	if err != nil {
		t.Fatal(err)
	}
	return ms
}

func rawDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := openSQLiteDB(filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// BE-060：按编号顺序执行、只执行一次、记录 checksum。
func TestMigrateAppliesInOrderOnce(t *testing.T) {
	ctx := context.Background()
	db := rawDB(t)

	ms := testMigrations(t, map[string]string{
		"0002_add_col.sql": `ALTER TABLE t ADD COLUMN b TEXT NOT NULL DEFAULT '';`,
		"0001_init.sql":    `CREATE TABLE t (a TEXT);`,
	})
	done, err := Migrate(ctx, db, ms)
	if err != nil || !slices.Equal(done, []int{1, 2}) {
		t.Fatalf("first run: done=%v err=%v", done, err)
	}
	done, err = Migrate(ctx, db, ms)
	if err != nil || len(done) != 0 {
		t.Fatalf("second run must be a no-op: done=%v err=%v", done, err)
	}
	if _, err := db.Exec(`INSERT INTO t (a, b) VALUES ('x', 'y')`); err != nil {
		t.Fatalf("schema after migrations: %v", err)
	}
}

// BE-060：失败的迁移整体回滚，不登记版本；修好后可以重跑。
func TestMigrateFailureRollsBack(t *testing.T) {
	ctx := context.Background()
	db := rawDB(t)

	bad := testMigrations(t, map[string]string{
		"0001_init.sql": `CREATE TABLE t (a TEXT); INSERT INTO nope VALUES (1);`,
	})
	if _, err := Migrate(ctx, db, bad); err == nil {
		t.Fatal("expected failure")
	}
	var n int
	db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name = 't'`).Scan(&n)
	if n != 0 {
		t.Fatal("table from failed migration must be rolled back")
	}
	st, err := Status(ctx, db, bad)
	if err != nil || len(st.Applied) != 0 {
		t.Fatalf("failed migration must not be recorded: %+v %v", st, err)
	}
}

// BE-060：已执行的文件被修改 → 拒绝；数据库里有代码不认识的版本 → 拒绝。
func TestMigrateDetectsDrift(t *testing.T) {
	ctx := context.Background()
	db := rawDB(t)

	if _, err := Migrate(ctx, db, testMigrations(t, map[string]string{"0001_init.sql": `CREATE TABLE t (a TEXT);`})); err != nil {
		t.Fatal(err)
	}
	_, err := Status(ctx, db, testMigrations(t, map[string]string{"0001_init.sql": `CREATE TABLE t (a TEXT, b TEXT);`}))
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Errorf("modified migration: err=%v, want checksum mismatch", err)
	}
	_, err = Status(ctx, db, nil)
	if err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Errorf("older binary: err=%v, want unknown migration", err)
	}
}

func TestLoadMigrationsRejectsBadNames(t *testing.T) {
	for _, files := range []map[string]string{
		{"init.sql": ""},
		{"0001_a.sql": "", "1_b.sql": ""}, // 同一个版本号
	} {
		fsys := fstest.MapFS{}
		for n := range files {
			fsys["m/"+n] = &fstest.MapFile{}
		}
		if _, err := LoadMigrations(fsys, "m"); err == nil {
			t.Errorf("%v: expected error", files)
		}
	}
}

// BE-060：生产模式（不自动迁移）遇到未迁移的库拒绝打开；migrate up 之后可以打开。
func TestOpenWithoutAutoMigrateRequiresMigrateUp(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "im.db")
	if _, err := OpenSQLite(path, SQLiteOptions{}); err == nil || !strings.Contains(err.Error(), "migrate up") {
		t.Fatalf("fresh db without auto-migrate: err=%v", err)
	}
	if _, err := MigrateSQLite(ctx, path); err != nil {
		t.Fatal(err)
	}
	s, err := OpenSQLite(path, SQLiteOptions{})
	if err != nil {
		t.Fatalf("after migrate up: %v", err)
	}
	s.Close()
	st, err := SQLiteStatus(ctx, path)
	if err != nil || len(st.Pending) != 0 || len(st.Applied) == 0 {
		t.Errorf("status = %+v, %v", st, err)
	}
}

// 打包进二进制的真实迁移文件必须能从空库执行成功，且版本号连续。
func TestEmbeddedMigrationsAreContiguous(t *testing.T) {
	ms, err := SQLiteMigrations()
	if err != nil {
		t.Fatal(err)
	}
	for i, m := range ms {
		if m.Version != i+1 {
			t.Fatalf("migration versions must be 1..N without gaps; got %d at position %d", m.Version, i)
		}
	}
}

// BE-040：迁移 0003 从已有数据回填会话列表（最后一条消息、单聊双方）。
// 模拟 B3 时期（只执行到 0002）的数据库，写入数据后再升级。
func TestMigration0003Backfill(t *testing.T) {
	ctx := context.Background()
	db := rawDB(t)
	all, err := SQLiteMigrations()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Migrate(ctx, db, all[:2]); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`INSERT INTO conv_seq (conv_id, max_seq) VALUES ('c2c:alice:bob', 2), ('group:g1', 1)`,
		`INSERT INTO messages VALUES ('m1', 'c2c:alice:bob', 1, 'alice', 'c1', 'text', '{"text":"first"}', 100)`,
		`INSERT INTO messages VALUES ('m2', 'c2c:alice:bob', 2, 'bob', 'c2', 'text', '{"text":"` + strings.Repeat("长", 120) + `"}', 200)`,
		`INSERT INTO messages VALUES ('m3', 'group:g1', 1, 'alice', 'c3', 'text', '{"text":"group"}', 300)`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if _, err := Migrate(ctx, db, all); err != nil {
		t.Fatal(err)
	}
	s := &SQLite{db: db}
	for _, uid := range []string{"alice", "bob"} {
		convs, _, err := s.ListConversations(ctx, uid, 10)
		if err != nil || len(convs) != 1 {
			t.Fatalf("%s: %+v %v", uid, convs, err)
		}
		c := convs[0]
		want := map[string]string{"alice": "bob", "bob": "alice"}[uid]
		if c.ConvID != "c2c:alice:bob" || c.PeerID != want || c.MaxSeq != 2 || c.Last.MsgID != "m2" || c.UpdatedAt != 200 {
			t.Errorf("%s: %+v", uid, c)
		}
		if n := len([]rune(c.Last.Text)); n != 100 {
			t.Errorf("backfilled summary has %d code points, want 100", n)
		}
	}
	var last string
	db.QueryRow(`SELECT last_text FROM conv_seq WHERE conv_id = 'group:g1'`).Scan(&last)
	if last != "group" {
		t.Errorf("group last text = %q", last)
	}
}

// TC-CHAT-90-06（AC-CHAT-90-6）：全新库和 0014 版本的库（含 Alice、alice，有消息和群）都能升级，已有数据不变；
// Alice、alice 都在，accounts 没有行；升级后旧版本二进制（只认识到 0014）拒绝启动。
func TestTC_CHAT_90_06_MigrateAccounts(t *testing.T) {
	ctx := context.Background()
	ms, err := SQLiteMigrations()
	if err != nil {
		t.Fatal(err)
	}
	idx := slices.IndexFunc(ms, func(m Migration) bool { return m.Version == 14 })
	if idx < 0 || len(ms) <= idx+1 {
		t.Fatal("expected migrations after 0014")
	}
	old := ms[:idx+1]

	t.Run("全新库", func(t *testing.T) {
		db := rawDB(t)
		if _, err := Migrate(ctx, db, ms); err != nil {
			t.Fatal(err)
		}
		for _, table := range []string{"accounts", "recovery_codes", "auth_throttle", "account_events", "device_sessions", "invites"} {
			var n int
			if err := db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil || n != 0 {
				t.Errorf("%s: n=%d err=%v", table, n, err)
			}
		}
	})

	t.Run("0014 库", func(t *testing.T) {
		db := rawDB(t)
		if _, err := Migrate(ctx, db, old); err != nil {
			t.Fatal(err)
		}
		for _, stmt := range []string{
			`INSERT INTO users (user_id, nickname, created_at) VALUES ('Alice', '大写', 1), ('alice', '小写', 2)`,
			`INSERT INTO chat_groups (group_id, name, created_at) VALUES ('g1', '群', 3)`,
			`INSERT INTO group_members (group_id, user_id, join_seq, joined_at) VALUES ('g1', 'Alice', 0, 3), ('g1', 'alice', 0, 3)`,
			`INSERT INTO conv_seq (conv_id, max_seq) VALUES ('group:g1', 1)`,
			`INSERT INTO messages (msg_id, conv_id, seq, from_uid, client_msg_id, type, body, ts) VALUES ('m1', 'group:g1', 1, 'Alice', 'c1', 'text', '{"text":"hi"}', 4)`,
		} {
			if _, err := db.Exec(stmt); err != nil {
				t.Fatalf("%s: %v", stmt, err)
			}
		}
		dump := func() string {
			var b strings.Builder
			for _, q := range []string{
				`SELECT user_id, nickname, created_at, token_version FROM users ORDER BY user_id`,
				`SELECT group_id, user_id, join_seq FROM group_members ORDER BY user_id`,
				`SELECT msg_id, conv_id, seq, from_uid, body FROM messages`,
			} {
				rows, err := db.Query(q)
				if err != nil {
					t.Fatal(err)
				}
				cols, _ := rows.Columns()
				vals := make([]any, len(cols))
				ptrs := make([]any, len(cols))
				for i := range vals {
					ptrs[i] = &vals[i]
				}
				for rows.Next() {
					if err := rows.Scan(ptrs...); err != nil {
						t.Fatal(err)
					}
					fmt.Fprintln(&b, vals...)
				}
				rows.Close()
			}
			return b.String()
		}
		before := dump()
		if _, err := Migrate(ctx, db, ms); err != nil {
			t.Fatal(err)
		}
		if after := dump(); after != before {
			t.Errorf("existing data changed:\n%s\nvs\n%s", before, after)
		}
		var accounts, deleted int
		db.QueryRow(`SELECT COUNT(*) FROM accounts`).Scan(&accounts)
		db.QueryRow(`SELECT COUNT(*) FROM users WHERE deleted_at <> 0`).Scan(&deleted)
		if accounts != 0 || deleted != 0 {
			t.Errorf("accounts=%d deleted=%d", accounts, deleted)
		}
		// 旧版本二进制按现有规则拒绝：数据库里有它不认识的迁移。
		if _, err := Status(ctx, db, old); err == nil {
			t.Error("an old binary must refuse a database with newer migrations")
		}
	})
}
