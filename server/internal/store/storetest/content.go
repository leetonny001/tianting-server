package storetest

import (
	"errors"
	"testing"

	"tianting/server/internal/store"
)

// C6 敏感词和举报的契约用例（BE-492～BE-496）。Postgres 实现随 B6 跑同一套。norm、compact 在这里直接给，标准化本身在 moderation 包测。

func contentCases() []struct {
	name string
	fn   func(*testing.T, Opener)
} {
	return []struct {
		name string
		fn   func(*testing.T, Opener)
	}{
		{"WordsAddDedupByNorm", testWordsAddDedup},
		{"WordsImport", testWordsImport},
		{"WordsStateDeleteList", testWordsStateDeleteList},
	}
}

func bw(word, norm string, at int64) store.BlockedWord {
	return store.BlockedWord{Word: word, Norm: norm, Compact: norm, CreatedAt: at}
}

// 按 norm 去重（backend-chat 第 100 条）：已有的（含停用的）返回已有的那条、created=false、状态不变。
func testWordsAddDedup(t *testing.T, open Opener) {
	s := open(t)
	a, created, err := s.AddWord(ctx(), bw("BadWord", "badword", 10))
	if err != nil || !created || a.WordID == 0 || a.Word != "BadWord" || a.State != store.WordActive || a.CreatedAt != 10 || a.UpdatedAt != 10 {
		t.Fatalf("add = %+v %v %v", a, created, err)
	}
	if _, err := s.SetWordState(ctx(), a.WordID, store.WordDisabled, 20); err != nil {
		t.Fatal(err)
	}
	b, created, err := s.AddWord(ctx(), bw("ＢＡＤＷＯＲＤ", "badword", 30))
	if err != nil || created || b.WordID != a.WordID || b.Word != "BadWord" || b.State != store.WordDisabled {
		t.Fatalf("duplicate add = %+v %v %v", b, created, err)
	}
	c, created, err := s.AddWord(ctx(), bw("ZXQ-2", "zxq-2", 40))
	if err != nil || !created || c.WordID == a.WordID {
		t.Fatalf("second word = %+v %v %v", c, created, err)
	}
}

func testWordsImport(t *testing.T, open Opener) {
	s := open(t)
	if _, _, err := s.AddWord(ctx(), bw("old", "old", 1)); err != nil {
		t.Fatal(err)
	}
	added, dup, err := s.ImportWords(ctx(), []store.BlockedWord{bw("a", "a", 2), bw("OLD", "old", 2), bw("b", "b", 2), bw("A", "a", 2)})
	if err != nil || added != 2 || dup != 2 {
		t.Fatalf("import: added=%d dup=%d err=%v", added, dup, err)
	}
	if n, err := s.CountActiveWords(ctx()); err != nil || n != 3 {
		t.Fatalf("count = %d %v", n, err)
	}
}

func testWordsStateDeleteList(t *testing.T, open Opener) {
	s := open(t)
	var ids []int64
	for _, w := range []string{"zxq-a1", "zxq-b2", "zxq-a3", "other"} {
		got, _, err := s.AddWord(ctx(), bw(w, w, 5))
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, got.WordID)
	}
	if _, err := s.SetWordState(ctx(), 9999, store.WordDisabled, 6); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("missing word: %v", err)
	}
	if w, err := s.SetWordState(ctx(), ids[1], store.WordDisabled, 6); err != nil || w.State != store.WordDisabled || w.UpdatedAt != 6 {
		t.Errorf("disable = %+v %v", w, err)
	}
	keys, err := s.ActiveWords(ctx())
	if err != nil || len(keys) != 3 {
		t.Fatalf("active = %+v %v", keys, err)
	}
	for _, k := range keys {
		if k.WordID == ids[1] {
			t.Error("a disabled word must not be loaded")
		}
	}
	// 倒序分页、按 norm 子串、按状态；LIKE 的通配符按字面。
	page, next, err := s.ListWords(ctx(), store.WordQuery{Limit: 2})
	if err != nil || len(page) != 2 || page[0].WordID != ids[3] || next != page[1].WordID {
		t.Fatalf("page 1 = %+v next=%d %v", page, next, err)
	}
	page, next, err = s.ListWords(ctx(), store.WordQuery{Limit: 2, Cursor: next})
	if err != nil || len(page) != 2 || next != 0 || page[1].WordID != ids[0] {
		t.Fatalf("page 2 = %+v next=%d %v", page, next, err)
	}
	if page, _, err := s.ListWords(ctx(), store.WordQuery{Q: "zxq-a", Limit: 50}); err != nil || len(page) != 2 {
		t.Errorf("q = %+v %v", page, err)
	}
	if page, _, err := s.ListWords(ctx(), store.WordQuery{Q: "zxq_a", Limit: 50}); err != nil || len(page) != 0 {
		t.Errorf("_ is literal: %+v %v", page, err)
	}
	if page, _, err := s.ListWords(ctx(), store.WordQuery{State: store.WordDisabled, Limit: 50}); err != nil || len(page) != 1 || page[0].WordID != ids[1] {
		t.Errorf("state = %+v %v", page, err)
	}
	if err := s.DeleteWord(ctx(), ids[0]); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteWord(ctx(), ids[0]); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("delete twice: %v", err)
	}
	if n, _ := s.CountActiveWords(ctx()); n != 2 {
		t.Errorf("count after delete = %d", n)
	}
}
