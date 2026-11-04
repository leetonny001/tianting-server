package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
)

// 版本化迁移（ARCH 3.1）：migrations/<方言>/NNNN_名称.sql，按编号顺序执行，每个文件一个事务。
// 已执行过的文件不允许再修改（checksum 不一致时拒绝启动），表结构变更只能追加新文件。

//go:embed migrations
var migrationFS embed.FS

// Migration 是一个迁移文件。
type Migration struct {
	Version  int
	Name     string
	SQL      string
	Checksum string
}

// MigrationStatus 是数据库当前的迁移状态。
type MigrationStatus struct {
	Applied []int       // 已执行的版本号
	Pending []Migration // 待执行的迁移，按版本号升序
}

const migrationsTable = `CREATE TABLE IF NOT EXISTS schema_migrations (
	version    INTEGER PRIMARY KEY,
	name       TEXT NOT NULL,
	checksum   TEXT NOT NULL,
	applied_at INTEGER NOT NULL
)`

// LoadMigrations 读取某个方言目录下的全部迁移，按版本号排序。
func LoadMigrations(fsys fs.FS, dir string) ([]Migration, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, err
	}
	var out []Migration
	seen := map[int]string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		num, name, ok := strings.Cut(strings.TrimSuffix(e.Name(), ".sql"), "_")
		v, err := strconv.Atoi(num)
		if !ok || err != nil || v <= 0 {
			return nil, fmt.Errorf("migration %s: file name must be NNNN_name.sql", e.Name())
		}
		if prev, dup := seen[v]; dup {
			return nil, fmt.Errorf("migration version %d used by both %s and %s", v, prev, e.Name())
		}
		seen[v] = e.Name()
		b, err := fs.ReadFile(fsys, path.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(b)
		out = append(out, Migration{Version: v, Name: name, SQL: string(b), Checksum: hex.EncodeToString(sum[:])})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

// Status 对比数据库里已执行的迁移和代码里的迁移；已执行的文件被改过时返回错误。
func Status(ctx context.Context, db *sql.DB, ms []Migration) (MigrationStatus, error) {
	if _, err := db.ExecContext(ctx, migrationsTable); err != nil {
		return MigrationStatus{}, fmt.Errorf("create schema_migrations: %w", err)
	}
	rows, err := db.QueryContext(ctx, `SELECT version, checksum FROM schema_migrations ORDER BY version`)
	if err != nil {
		return MigrationStatus{}, err
	}
	defer rows.Close()
	applied := map[int]string{}
	var st MigrationStatus
	for rows.Next() {
		var v int
		var sum string
		if err := rows.Scan(&v, &sum); err != nil {
			return MigrationStatus{}, err
		}
		applied[v] = sum
		st.Applied = append(st.Applied, v)
	}
	if err := rows.Err(); err != nil {
		return MigrationStatus{}, err
	}
	known := map[int]bool{}
	for _, m := range ms {
		known[m.Version] = true
		sum, ok := applied[m.Version]
		switch {
		case !ok:
			st.Pending = append(st.Pending, m)
		case sum != m.Checksum:
			return MigrationStatus{}, fmt.Errorf("migration %04d_%s was modified after being applied (checksum mismatch); add a new migration instead", m.Version, m.Name)
		}
	}
	for _, v := range st.Applied {
		if !known[v] {
			// 数据库比代码新：通常是回滚到了旧版本的二进制，继续运行可能读写不认识的表结构。
			return MigrationStatus{}, fmt.Errorf("database has migration %d unknown to this binary (newer schema?)", v)
		}
	}
	return st, nil
}

// Migrate 执行全部待执行的迁移，返回本次执行的版本号。每个迁移和它的登记记录在同一个事务里。
func Migrate(ctx context.Context, db *sql.DB, ms []Migration) ([]int, error) {
	st, err := Status(ctx, db, ms)
	if err != nil {
		return nil, err
	}
	var done []int
	for _, m := range st.Pending {
		if m.Name == "group_roles" {
			if err := checkNoReservedGroupIDs(ctx, db); err != nil {
				return done, err
			}
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return done, err
		}
		if _, err := tx.ExecContext(ctx, m.SQL); err != nil {
			tx.Rollback()
			return done, fmt.Errorf("migration %04d_%s: %w", m.Version, m.Name, err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (version, name, checksum, applied_at) VALUES (?, ?, ?, ?)`,
			m.Version, m.Name, m.Checksum, time.Now().UnixMilli()); err != nil {
			tx.Rollback()
			return done, err
		}
		if err := tx.Commit(); err != nil {
			return done, err
		}
		done = append(done, m.Version)
	}
	return done, nil
}

// checkNoReservedGroupIDs：ug- 前缀保留给自建群，已有群占用时中止迁移并列出，由运维处理后再升级（PRD-social 8.5 节第 4 条）。
func checkNoReservedGroupIDs(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `SELECT group_id FROM chat_groups WHERE group_id LIKE 'ug-%' ORDER BY group_id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(ids) > 0 {
		return fmt.Errorf("migration group_roles: group IDs %v use the reserved ug- prefix; rename them before upgrading", ids)
	}
	return nil
}
