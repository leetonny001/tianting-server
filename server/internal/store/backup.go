package store

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
)

// 备份与恢复（ARCH 3.4）。
//
// 备份用 `VACUUM INTO` 生成一致性快照：在独立的连接上执行，WAL 模式下读的是某一时刻的快照，
// 不阻塞服务端写入，所以服务运行时也可以备份。只提供命令行（`im-server backup`），
// 不提供 HTTP 接口：让请求方指定服务器上的文件路径，等于开放任意写文件的能力。

// BackupSQLite 把 srcPath 的一致性快照写到 dst。dst 已存在时报错，不覆盖。
func BackupSQLite(ctx context.Context, srcPath, dst string) error {
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("backup target %s already exists", dst)
	}
	if _, err := os.Stat(srcPath); err != nil {
		return fmt.Errorf("database %s: %w", srcPath, err)
	}
	db, err := openSQLiteDB(srcPath)
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, `VACUUM INTO ?`, dst); err != nil {
		os.Remove(dst)
		return fmt.Errorf("vacuum into: %w", err)
	}
	// 备份文件只给属主读写：里面是全部聊天记录。
	return os.Chmod(dst, 0o600)
}

// VerifySQLiteFile 检查一个数据库文件（通常是备份）是否完好：
// 完整性检查通过，并且表结构版本是当前二进制认识的。
func VerifySQLiteFile(ctx context.Context, path string) (MigrationStatus, error) {
	if _, err := os.Stat(path); err != nil {
		return MigrationStatus{}, err
	}
	db, err := openSQLiteDB(path)
	if err != nil {
		return MigrationStatus{}, err
	}
	defer db.Close()
	var result string
	if err := db.QueryRowContext(ctx, `PRAGMA integrity_check`).Scan(&result); err != nil {
		return MigrationStatus{}, fmt.Errorf("integrity check: %w", err)
	}
	if result != "ok" {
		return MigrationStatus{}, fmt.Errorf("integrity check failed: %s", result)
	}
	ms, err := SQLiteMigrations()
	if err != nil {
		return MigrationStatus{}, err
	}
	return Status(ctx, db, ms)
}

// RestoreSQLite 用备份文件替换 dbPath。**必须先停止服务。**
// 先校验备份，再复制到临时文件并 fsync，最后原子改名；dbPath 已存在时需要 force。
func RestoreSQLite(ctx context.Context, backup, dbPath string, force bool) error {
	if _, err := VerifySQLiteFile(ctx, backup); err != nil {
		return fmt.Errorf("backup %s is not usable: %w", backup, err)
	}
	if _, err := os.Stat(dbPath); err == nil && !force {
		return fmt.Errorf("%s already exists; stop the server and pass -force to replace it", dbPath)
	}
	tmp := dbPath + ".restore-tmp"
	if err := copyFile(backup, tmp); err != nil {
		os.Remove(tmp)
		return err
	}
	// 旧库的 WAL 和共享内存文件必须删掉，否则会被当成新库的日志回放。
	for _, suffix := range []string{"-wal", "-shm"} {
		if err := os.Remove(dbPath + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			os.Remove(tmp)
			return err
		}
	}
	return os.Rename(tmp, dbPath)
}

func copyFile(src, dst string) error {
	// 路径来自运维在命令行传入的参数（im-server restore -i），不是外部请求，不存在越权读取。
	in, err := os.Open(src) // #nosec G304 -- operator-supplied CLI path
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600) // #nosec G304 -- operator-supplied CLI path
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
