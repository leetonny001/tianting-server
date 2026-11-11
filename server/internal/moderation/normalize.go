// Package moderation 是 C6 的敏感词（PRD-chat 第 10 节 F-MOD-01、02，BE-493）：标准化、多模式匹配、不可变快照和重新加载。
// **仓库和代码里不放真实词库**（规则 5）：词只从数据库来，测试只用特征词。
package moderation

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Normalize 是规则 3 的标准化，**词和待查文本用同一个函数**（backend-chat 第 101 条）：
// NFKC → 去变音符（NFD 后去掉 Mn）→ 小写 → 去掉 Default_Ignorable（零宽、变体选择符、Bidi 控制等）得到 norm；
// 再去掉空白和标点得到 compact（防"违 规 词"、bad-word）。只用于匹配和去重，存储和下发的仍是原文（AC-MOD-02-3）。
// 不做拼音、形近、谐音。
func Normalize(s string) (normed, compact string) {
	s = norm.NFKC.String(s)
	s = norm.NFD.String(s)
	var nb, cb strings.Builder
	nb.Grow(len(s))
	cb.Grow(len(s))
	for _, r := range s {
		if unicode.Is(unicode.Mn, r) || defaultIgnorable(r) {
			continue
		}
		r = unicode.ToLower(r)
		nb.WriteRune(r)
		if !unicode.Is(unicode.White_Space, r) && !unicode.IsPunct(r) {
			cb.WriteRune(r)
		}
	}
	// 去掉变音符之后再合成一次：没有变音符可去的组合字符（如韩文字母）恢复成合成形式，词和文本两边一致。
	return norm.NFC.String(nb.String()), norm.NFC.String(cb.String())
}

// defaultIgnorable 是 Unicode 的 Default_Ignorable_Code_Point（DerivedCoreProperties.txt 的定义）：
// Other_Default_Ignorable_Code_Point + Cf + Variation_Selector − White_Space − FFF9..FFFB − 13430..1343F − Prepended_Concatenation_Mark。
// 标准库没有这个派生属性，按定义用它的组成部分算（不加依赖）。
func defaultIgnorable(r rune) bool {
	if !unicode.In(r, unicode.Other_Default_Ignorable_Code_Point, unicode.Cf, unicode.Variation_Selector) {
		return false
	}
	switch {
	case unicode.Is(unicode.White_Space, r), r >= 0xFFF9 && r <= 0xFFFB, r >= 0x13430 && r <= 0x1343F, unicode.Is(unicode.Prepended_Concatenation_Mark, r):
		return false
	}
	return true
}

// MaxWordLen 是一个词的码点上限（F-MOD-01 异常：超过 64 个码点的不合法）。
const MaxWordLen = 64

// PrepareWord 校验并标准化一个要入库的词：去首尾空白后为空、超过 64 个码点、或者标准化之后什么都不剩（只有零宽字符、变音符之类，
// 匹配不到任何东西，backend-chat 第 128 条），都不合法。只有标点的词 compact 为空，只按 norm 匹配。返回入库的原文和两份标准化结果。
func PrepareWord(raw string) (word, normed, compact string, ok bool) {
	word = strings.TrimSpace(raw)
	if word == "" || utf8.RuneCountInString(word) > MaxWordLen {
		return "", "", "", false
	}
	normed, compact = Normalize(word)
	if normed == "" {
		return "", "", "", false
	}
	return word, normed, compact, true
}
