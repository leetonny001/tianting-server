package store

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
)

// MigrationRow 是一条迁移的状态（PRD-ops S-OPS-22）。im-server migrate status 和 /internal/v1/db 用同一个函数，内容不会不一致。
type MigrationRow struct {
	Version    int    `json:"version"`
	Name       string `json:"name"`
	AppliedAt  int64  `json:"appliedAt"` // 0 表示未执行
	ChecksumOK bool   `json:"checksumOk"`
	Pending    bool   `json:"pending"`
	Unknown    bool   `json:"unknown,omitempty"` // 数据库里有、这个二进制不认识（通常是回滚到了旧版本）
}

// OpenSQLiteReadOnly 打开只读连接：不和消息写入共用连接，写操作返回 SQLITE_READONLY（AC-OPS-11-4）。
func OpenSQLiteReadOnly(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro&_pragma=busy_timeout(1000)&_pragma=query_only(1)")
	if err != nil {
		return nil, fmt.Errorf("open sqlite read-only: %w", err)
	}
	db.SetMaxOpenConns(2)
	return db, nil
}

// MigrationReport 只读地比较已执行的迁移和二进制里的迁移，校验和不一致时标出来，不报错。
func MigrationReport(ctx context.Context, db *sql.DB, ms []Migration) ([]MigrationRow, error) {
	var exists int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'schema_migrations'`).Scan(&exists); err != nil {
		return nil, err
	}
	type applied struct {
		name, sum string
		at        int64
	}
	done := map[int]applied{}
	if exists > 0 {
		rows, err := db.QueryContext(ctx, `SELECT version, name, checksum, applied_at FROM schema_migrations`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var v int
			var a applied
			if err := rows.Scan(&v, &a.name, &a.sum, &a.at); err != nil {
				return nil, err
			}
			done[v] = a
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	out := make([]MigrationRow, 0, len(ms))
	for _, m := range ms {
		a, ok := done[m.Version]
		if !ok {
			out = append(out, MigrationRow{Version: m.Version, Name: m.Name, Pending: true, ChecksumOK: true})
			continue
		}
		out = append(out, MigrationRow{Version: m.Version, Name: m.Name, AppliedAt: a.at, ChecksumOK: a.sum == m.Checksum})
		delete(done, m.Version)
	}
	for v, a := range done {
		out = append(out, MigrationRow{Version: v, Name: a.name, AppliedAt: a.at, Unknown: true})
	}
	slices.SortFunc(out, func(x, y MigrationRow) int { return x.Version - y.Version })
	return out, nil
}

// SQLiteMigrationReport 是 im-server migrate status 用的入口。
func SQLiteMigrationReport(ctx context.Context, path string) ([]MigrationRow, error) {
	db, err := OpenSQLiteReadOnly(path)
	if err != nil {
		return nil, err
	}
	defer db.Close()
	ms, err := SQLiteMigrations()
	if err != nil {
		return nil, err
	}
	return MigrationReport(ctx, db, ms)
}
