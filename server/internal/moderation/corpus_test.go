package moderation

import (
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode"
)

// 语料误判（R-CS-17，BE-493 第 3 条，结果写进 BE-499 的验收报告）：第二遍"去掉空白和标点再匹配"（compact）会把跨越空格、标点的两段
// 拼成一个词。仓库里没有真实语料和真实词库（F-MOD-01 规则 5），这里用**特征语料**测：语料是仓库里现行版本的 PRD 正文（中文为主，夹英文和代码），
// 先去掉里面原样出现的特征词；词库分三组：
//  1. 特征词（违规词、badword、ZXQWORD）；
//  2. 从语料里随机取的 2 个、3 个连续汉字（模拟中文词，2 个字是 compact 误判的最坏情况）；
//  3. 从语料里随机取的英文单词（4～8 个字母）。词数取得小，免得几乎每行都 norm 命中、看不出 compact 的影响。
//
// 每组统计：语料行数、norm 命中的行（词在原文里本来就连着，不算 compact 引起的）、只有 compact 命中的行（跨越空白或标点拼出来的，就是
// compact 多出来的误判）。**只记录，不判定**（没有真实语料，比例只能说明量级）。-short 时跳过。
func TestCorpusCompactFalsePositives(t *testing.T) {
	if testing.Short() {
		t.Skip("corpus measurement")
	}
	lines := corpusLines(t)
	features := []string{"违规词", "badword", "ZXQWORD"}
	for i, l := range lines {
		for _, f := range features {
			l = strings.ReplaceAll(l, f, "")
			l = strings.ReplaceAll(l, strings.ToLower(f), "")
		}
		lines[i] = l
	}
	r := rand.New(rand.NewPCG(2026, 1111))
	groups := []struct {
		name  string
		words []string
	}{
		{"特征词", features},
		{"随机 2 个汉字 ×50", sampleWords(r, lines, 50, hanN(2))},
		{"随机 3 个汉字 ×200", sampleWords(r, lines, 200, hanN(3))},
		{"随机英文单词 ×200", sampleWords(r, lines, 200, latinWord)},
	}
	for _, g := range groups {
		ws := make([]Word, 0, len(g.words))
		for i, w := range g.words {
			if _, n, c, ok := PrepareWord(w); ok {
				ws = append(ws, Word{ID: int64(i + 1), Norm: n, Compact: c})
			}
		}
		s := build(ws, time.Unix(0, 0))
		var normHit, compactOnly int
		var example string
		for _, l := range lines {
			n, c := Normalize(l)
			switch {
			case s.norm.find(n) >= 0:
				normHit++
			case s.compact.find(c) >= 0:
				compactOnly++
				if example == "" {
					example = shorten(l)
				}
			}
		}
		rest := len(lines) - normHit
		rate := 0.0
		if rest > 0 {
			rate = float64(compactOnly) / float64(rest) * 100
		}
		t.Logf("%s：词 %d 个，语料 %d 行；norm 命中 %d 行；只有 compact 命中 %d 行（占 norm 不命中的 %.2f%%）；例：%q",
			g.name, len(ws), len(lines), normHit, compactOnly, rate, example)
	}
}

// corpusLines 读现行版本 PRD 的非空行。
func corpusLines(t *testing.T) []string {
	t.Helper()
	dir := filepath.Join("..", "..", "..", "docs", "prd")
	var out []string
	for _, name := range []string{"PRD-chat-v1.7.md", "PRD-core-v1.8.md", "PRD-social-v1.3.md", "PRD-ui-v1.6.md", "PRD-admin-v1.5.md", "PRD-ops-v1.6.md"} {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		for l := range strings.Lines(string(b)) {
			if l = strings.TrimSpace(l); l != "" {
				out = append(out, l)
			}
		}
	}
	return out
}

func sampleWords(r *rand.Rand, lines []string, n int, pick func(*rand.Rand, []rune) string) []string {
	seen := map[string]bool{}
	var out []string
	for tries := 0; len(out) < n && tries < n*1000; tries++ {
		w := pick(r, []rune(lines[r.IntN(len(lines))]))
		if w != "" && !seen[w] {
			seen[w] = true
			out = append(out, w)
		}
	}
	return out
}

func hanN(n int) func(*rand.Rand, []rune) string {
	return func(r *rand.Rand, rs []rune) string {
		if len(rs) < n {
			return ""
		}
		i := r.IntN(len(rs) - n + 1)
		for _, c := range rs[i : i+n] {
			if !unicode.Is(unicode.Han, c) {
				return ""
			}
		}
		return string(rs[i : i+n])
	}
}

func latinWord(r *rand.Rand, rs []rune) string {
	words := strings.FieldsFunc(string(rs), func(c rune) bool { return c > unicode.MaxASCII || !unicode.IsLetter(c) })
	if len(words) == 0 {
		return ""
	}
	w := strings.ToLower(words[r.IntN(len(words))])
	if len(w) < 4 || len(w) > 8 {
		return ""
	}
	return w
}

func shorten(s string) string {
	rs := []rune(s)
	if len(rs) > 60 {
		return string(rs[:60]) + "…"
	}
	return s
}
