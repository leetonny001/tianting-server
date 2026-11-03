package store_test

import (
	"path/filepath"
	"testing"

	"tianting/server/internal/store"
	"tianting/server/internal/store/storetest"
)

// SQLite 实现必须通过全部契约用例（BE-061）。
func TestSQLiteContract(t *testing.T) {
	storetest.Run(t, func(t *testing.T) store.Store {
		s, err := store.OpenSQLite(filepath.Join(t.TempDir(), "im.db"), store.SQLiteOptions{AutoMigrate: true})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { s.Close() })
		return s
	})
}
