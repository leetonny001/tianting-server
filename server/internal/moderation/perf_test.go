package moderation

import (
	"math/rand/v2"
	"runtime"
	"slices"
	"testing"
	"time"
)

// TC-CHAT-90-42（AC-CHAT-90-4，BE-493 第 4 条）：10 万个测试现场随机生成的词（中英文，长度 2～8，不落盘、不提交），
// 5000 码点以中文为主、不命中的文本，单线程 10000 次匹配（norm 和 compact 两遍都算），P99 ≤ 2 ms（未校准）。
// **开发机结果只记录，不作判定**（P-12）；同时记录构建耗时和内存，供部署参考。-short 时跳过。

func randomWords(r *rand.Rand, n int) []string {
	out := make([]string, n)
	for i := range out {
		l := 2 + r.IntN(7)
		b := make([]rune, l)
		for k := range b {
			if r.IntN(4) == 0 {
				b[k] = rune('a' + r.IntN(26))
			} else {
				b[k] = rune(0x4E00 + r.IntN(0x5000))
			}
		}
		out[i] = string(b)
	}
	return out
}

func randomText(r *rand.Rand, n int) string {
	b := make([]rune, n)
	for i := range b {
		switch k := r.IntN(20); {
		case k == 0:
			b[i] = '，'
		case k == 1:
			b[i] = ' '
		case k < 4:
			b[i] = rune('0' + r.IntN(10)) // 数字不在词里；英文字母两两组合几乎都是词（10 万词里有上千个两个字母的），放进文本就一定命中
		default:
			b[i] = rune(0x4E00 + r.IntN(0x5000))
		}
	}
	return string(b)
}

// perfFixture 建 10 万词的快照和一段不命中的文本（命中就换一段）。
func perfFixture(tb testing.TB) (*Engine, string, time.Duration, uint64) {
	tb.Helper()
	r := rand.New(rand.NewPCG(42, 7))
	raw := randomWords(r, 100_000)
	words := make([]Word, len(raw))
	for i, w := range raw {
		n, c := Normalize(w)
		words[i] = Word{ID: int64(i + 1), Norm: n, Compact: c}
	}
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	start := time.Now()
	s := build(words, time.Unix(0, 0))
	took := time.Since(start)
	runtime.GC()
	runtime.ReadMemStats(&after)
	e := &Engine{deps: Deps{Now: func() time.Time { return time.Unix(0, 0) }}, kick: make(chan struct{}, 1)}
	e.cur.Store(s)
	for attempt := 0; ; attempt++ {
		if attempt > 100 {
			tb.Fatal("could not generate a text without hits")
		}
		text := randomText(r, 5000)
		if _, hit := e.Match(text); !hit {
			return e, text, took, after.HeapAlloc - min(after.HeapAlloc, before.HeapAlloc)
		}
	}
}

func TestTC_CHAT_90_42_MatchP99(t *testing.T) {
	if testing.Short() || raceEnabled {
		t.Skip("performance record (not under -short or -race)")
	}
	e, text, took, heap := perfFixture(t)
	const n = 10_000
	d := make([]time.Duration, n)
	for i := range d {
		start := time.Now()
		if _, hit := e.Match(text); hit {
			t.Fatal("the text must not hit")
		}
		d[i] = time.Since(start)
	}
	slices.Sort(d)
	p50, p99, worst := d[n/2], d[n*99/100], d[n-1]
	t.Logf("TC-CHAT-90-42（开发机，只记录）：10 万词、5000 码点、%d 次：P50 %v，P99 %v，最大 %v；构建 %v，自动机约 %d MiB；%s/%s，%d 核",
		n, p50, p99, worst, took.Round(time.Millisecond), heap>>20, runtime.GOOS, runtime.GOARCH, runtime.NumCPU())
	if p99 > 2*time.Millisecond {
		t.Logf("P99 超过 2 ms 的目标（未校准，开发机结果不作判定，见 P-12）")
	}
}

func BenchmarkMatch100k(b *testing.B) {
	e, text, _, _ := perfFixture(b)
	b.ResetTimer()
	for b.Loop() {
		e.Match(text)
	}
}
