package moderation

import (
	"context"
	"errors"
	"log/slog"
	"math/rand/v2"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// 只用特征词（F-MOD-01 规则 5）：badword、违规词、ZXQ-*。

// TC-MOD-02-02 的标准化部分（AC-MOD-02-2，规则 3）：全角、变音、大小写、零宽、空格、标点都归一到同一个 norm 或 compact。
func TestNormalizeVariants(t *testing.T) {
	wantNorm, wantCompact := Normalize("badword")
	if wantNorm != "badword" || wantCompact != "badword" {
		t.Fatalf("badword -> %q %q", wantNorm, wantCompact)
	}
	for _, in := range []string{"ＢＡＤＷＯＲＤ", "bádwörd", "BadWord", "bad​word", "bad⁠word", "bad️word", "bad‮word"} {
		if n, _ := Normalize(in); n != "badword" {
			t.Errorf("norm(%q) = %q", in, n)
		}
	}
	for _, in := range []string{"bad word", "bad-word", "bad　word", "b.a.d,w!o?r…d", "bad_word"} {
		n, c := Normalize(in)
		if c != "badword" {
			t.Errorf("compact(%q) = %q", in, c)
		}
		if n == "badword" {
			t.Errorf("norm(%q) should keep the separator: %q", in, n)
		}
	}
	// 词里本来有空格、标点的，compact 里也去掉（backend-chat 第 101 条）：词 "违 规-词" 和文本 "违规词" 的 compact 相同。
	if _, c := Normalize("违 规-词"); c != "违规词" {
		t.Errorf("compact of a word with separators = %q", c)
	}
	// 韩文这类没有变音符的组合字符：去掉 Mn 之后合成回来，词和文本两边一致。
	if n, _ := Normalize("한국"); n != "한국" {
		t.Errorf("hangul = %q", n)
	}
	// 不做拼音、形近、谐音（规则 3）。
	if n, c := Normalize("违归词"); n == "违规词" || c == "违规词" {
		t.Error("homophones must not be folded")
	}
}

// 自动机和朴素查找结果一致：随机中英文词和文本，比较"是否命中"和"命中的是不是一个真的出现在文本里的词"。
func TestAutomatonMatchesNaive(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	alphabet := []rune("abcde违规词举报")
	word := func(n int) string {
		b := make([]rune, n)
		for i := range b {
			b[i] = alphabet[r.IntN(len(alphabet))]
		}
		return string(b)
	}
	for round := range 300 {
		var words []string
		for range 1 + r.IntN(20) {
			words = append(words, word(1+r.IntN(4)))
		}
		a := buildAutomaton(words)
		for range 20 {
			text := word(r.IntN(30))
			got := a.find(text)
			hit := slices.ContainsFunc(words, func(w string) bool { return strings.Contains(text, w) })
			switch {
			case (got >= 0) != hit:
				t.Fatalf("round %d: words %q text %q: got %d want hit=%v", round, words, text, got, hit)
			case got >= 0 && !strings.Contains(text, words[got]):
				t.Fatalf("round %d: reported %q which is not in %q", round, words[got], text)
			}
		}
	}
	if (&automaton{}).find("x") != -1 || buildAutomaton(nil).find("anything") != -1 || buildAutomaton([]string{""}).find("x") != -1 {
		t.Error("empty automaton must not match (an empty word would match everything)")
	}
}

type fakeLoader struct {
	mu    sync.Mutex
	words []Word
	err   error
	calls int
}

func (f *fakeLoader) load(context.Context) ([]Word, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	return slices.Clone(f.words), f.err
}

func (f *fakeLoader) set(err error, words ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
	f.words = nil
	for i, w := range words {
		n, c := Normalize(w)
		f.words = append(f.words, Word{ID: int64(i + 1), Norm: n, Compact: c})
	}
}

type clk struct{ ms atomic.Int64 }

func (c *clk) now() time.Time      { return time.UnixMilli(c.ms.Load()) }
func (c *clk) add(d time.Duration) { c.ms.Add(d.Milliseconds()) }

func waitReloads(t *testing.T, e *Engine, n int64) {
	t.Helper()
	deadline := time.Now().Add(RetryDelay + 2*time.Second)
	for e.Reloads() < n {
		if time.Now().After(deadline) {
			t.Fatalf("reloads = %d, want %d", e.Reloads(), n)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// AC-MOD-01-8 的引擎部分：启动加载失败返回错误（第 2 种）；重新加载失败沿用旧快照、计数加 1、日志没有词（第 1 种）；
// 词库为空不拦截（第 3 种）。失效后重建；TTL 兜底按 [IM 钟]。
func TestEngineReloadAndFailures(t *testing.T) {
	var c clk
	c.ms.Store(1_800_000_000_000)
	ld := &fakeLoader{}
	ld.set(nil, "违规词")
	var fail atomic.Bool
	var errs atomic.Int64
	var logBuf strings.Builder
	var logMu sync.Mutex
	e := New(Deps{Load: ld.load, Now: c.now, ReloadFails: fail.Load, ReloadError: func() { errs.Add(1) }, Log: newTestLogger(&logBuf, &logMu)})

	if err := e.Start(context.Background(), true); !errors.Is(err, ErrInjected) {
		t.Fatalf("startup fault: %v", err)
	}
	ld.set(errors.New("db down"), "违规词")
	if err := New(Deps{Load: ld.load, Now: c.now}).Start(context.Background(), false); err == nil {
		t.Fatal("startup load error must fail the start")
	}
	ld.set(nil, "违规词")
	if err := e.Start(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	if h, ok := e.Match("这是违规词"); !ok || h.WordID != 1 {
		t.Fatalf("match = %+v %v", h, ok)
	}
	// 重新加载失败：旧词库继续生效，新词没加载上。
	fail.Store(true)
	ld.set(nil, "违规词", "badword")
	e.Invalidate()
	deadline := time.Now().Add(2 * time.Second)
	for errs.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if errs.Load() != 1 {
		t.Fatalf("reload errors = %d", errs.Load())
	}
	if _, ok := e.Match("违规词"); !ok {
		t.Error("the old list must stay in effect")
	}
	if _, ok := e.Match("badword"); ok {
		t.Error("the new word must not be loaded")
	}
	logMu.Lock()
	if logs := logBuf.String(); !strings.Contains(logs, "reload failed") || strings.Contains(logs, "违规词") || strings.Contains(logs, "badword") {
		t.Errorf("log = %s", logs)
	}
	logMu.Unlock()
	// 恢复后不用等失效通知：失败之后 RetryDelay 自动重试（AC-MOD-01-8"解除注入后 10 秒内"）。
	before := e.Reloads()
	fail.Store(false)
	waitReloads(t, e, before+1)
	if _, ok := e.Match("bad word"); !ok {
		t.Error("after recovery the new word must match (compact)")
	}
	// TTL 兜底：没有失效通知，[IM 钟] 过了 60 秒的下一次匹配触发重建。
	ld.set(nil, "违规词", "badword", "ZXQ-TTL")
	c.add(59 * time.Second)
	if _, ok := e.Match("ZXQ-TTL"); ok {
		t.Fatal("59s: no reload yet")
	}
	before = e.Reloads()
	c.add(2 * time.Second)
	e.Match("ZXQ-TTL") // 触发
	waitReloads(t, e, before+1)
	if h, ok := e.Match("zxq-ttl"); !ok || h.WordID != 3 {
		t.Errorf("after TTL: %+v %v", h, ok)
	}
	// 词库为空：不拦截。
	ld.set(nil)
	before = e.Reloads()
	e.Invalidate()
	waitReloads(t, e, before+1)
	if _, ok := e.Match("违规词 badword"); ok || e.Size() != 0 {
		t.Error("an empty list must not block")
	}
}

// 并发：匹配和重建同时进行（go test -race 下跑），匹配不加锁。
func TestEngineConcurrentMatchAndReload(t *testing.T) {
	var c clk
	c.ms.Store(1)
	ld := &fakeLoader{}
	ld.set(nil, "违规词")
	e := New(Deps{Load: ld.load, Now: c.now})
	if err := e.Start(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 2000 {
				if _, ok := e.Match("xx违规词xx"); !ok {
					t.Error("lost a match during reload")
					return
				}
			}
		}()
	}
	for i := range 50 {
		ld.set(nil, "违规词", strings.Repeat("z", i+1))
		e.Invalidate()
	}
	wg.Wait()
}

type lockedWriter struct {
	b  *strings.Builder
	mu *sync.Mutex
}

func (w lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.Write(p)
}

func newTestLogger(b *strings.Builder, mu *sync.Mutex) *slog.Logger {
	return slog.New(slog.NewTextHandler(lockedWriter{b, mu}, nil))
}
