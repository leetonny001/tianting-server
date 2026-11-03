package main

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"
)

// latencies 收集延迟样本，计算分位数。
type latencies struct {
	mu sync.Mutex
	d  []time.Duration
}

func (l *latencies) add(d time.Duration) {
	l.mu.Lock()
	l.d = append(l.d, d)
	l.mu.Unlock()
}

type summary struct {
	N             int
	P50, P99, Max time.Duration
}

func (l *latencies) summary() summary {
	l.mu.Lock()
	d := slices.Clone(l.d)
	l.mu.Unlock()
	if len(d) == 0 {
		return summary{}
	}
	slices.Sort(d)
	q := func(p float64) time.Duration { return d[min(len(d)-1, int(float64(len(d))*p))] }
	return summary{N: len(d), P50: q(0.50), P99: q(0.99), Max: d[len(d)-1]}
}

func (s summary) String() string {
	return fmt.Sprintf("n=%d P50=%s P99=%s max=%s", s.N, ms(s.P50), ms(s.P99), ms(s.Max))
}

func ms(d time.Duration) string { return fmt.Sprintf("%.1fms", float64(d.Microseconds())/1000) }

// counter 是按 key 计数的并发安全 map（例如按错误码统计）。
type counter struct {
	mu sync.Mutex
	m  map[string]int
}

func (c *counter) inc(key string) {
	c.mu.Lock()
	if c.m == nil {
		c.m = map[string]int{}
	}
	c.m[key]++
	c.mu.Unlock()
}

func (c *counter) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.m) == 0 {
		return "无"
	}
	keys := make([]string, 0, len(c.m))
	for k := range c.m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%s×%d", k, c.m[k])
	}
	return strings.Join(parts, "，")
}

func (c *counter) total() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, v := range c.m {
		n += v
	}
	return n
}

// windows 按固定窗口统计成功数，用于检查"任意 10 秒窗口不低于目标"。
type windows struct {
	mu    sync.Mutex
	start time.Time
	size  time.Duration
	n     []int
}

func newWindows(size time.Duration) *windows { return &windows{start: time.Now(), size: size} }

func (w *windows) add(t time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()
	i := int(t.Sub(w.start) / w.size)
	for len(w.n) <= i {
		w.n = append(w.n, 0)
	}
	w.n[i]++
}

// rates 返回各个完整窗口的速率（条/秒），丢掉最后一个可能不完整的窗口。
func (w *windows) rates() []float64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	n := w.n
	if len(n) > 1 {
		n = n[:len(n)-1]
	}
	out := make([]float64, len(n))
	for i, v := range n {
		out[i] = float64(v) / w.size.Seconds()
	}
	return out
}

// result 是一个场景的结论，写进报告。
type result struct {
	Case   string // 对应的测试用例编号
	Title  string
	Target string
	Pass   bool
	Rows   [][2]string // 指标名 → 值
	Notes  []string
}

func (r *result) row(k, format string, args ...any) {
	r.Rows = append(r.Rows, [2]string{k, fmt.Sprintf(format, args...)})
}

func (r *result) note(format string, args ...any) {
	r.Notes = append(r.Notes, fmt.Sprintf(format, args...))
}

func (r *result) markdown() string {
	var b strings.Builder
	verdict := "✅ 通过"
	if !r.Pass {
		verdict = "❌ 未通过"
	}
	fmt.Fprintf(&b, "### %s %s — %s\n\n目标：%s\n\n| 指标 | 结果 |\n|---|---|\n", r.Case, r.Title, verdict, r.Target)
	for _, row := range r.Rows {
		fmt.Fprintf(&b, "| %s | %s |\n", row[0], row[1])
	}
	for _, n := range r.Notes {
		fmt.Fprintf(&b, "\n- %s", n)
	}
	b.WriteString("\n\n")
	return b.String()
}

func mb(bytes float64) string { return fmt.Sprintf("%.0f MB", bytes/(1<<20)) }
