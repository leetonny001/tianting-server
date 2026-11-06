package account

import (
	_ "embed"
	"regexp"
	"strings"
	"unicode/utf8"
)

// 用户名：去首尾空白并转小写后 4～32 位，[a-z0-9_-]，字母开头（F-ACC-01 规则 3）。转小写之前先判断，只接受 ASCII：
// 否则 Kelvin 符号 K 这样的字符转小写后会变成 k，和别人的用户名撞在一起。
var usernameRE = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{3,31}$`)

// NormalizeUsername 去首尾空白并转小写。结果不一定合法，注册要另外判断 ValidUsername，登录的不合法用户名按不存在处理。
func NormalizeUsername(raw string) string { return strings.ToLower(strings.TrimSpace(raw)) }

// ValidUsername 判断原始输入去首尾空白后是否符合规则。
func ValidUsername(raw string) bool { return usernameRE.MatchString(strings.TrimSpace(raw)) }

//go:embed reserved_names.txt
var reservedNamesFile string

//go:embed weak_passwords.txt
var weakPasswordsFile string

func listFile(s string) []string {
	var out []string
	for line := range strings.Lines(s) {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			out = append(out, strings.ToLower(line))
		}
	}
	return out
}

// reservedNames 是内置保留名加配置文件里的（F-ACC-01 规则 4）。
func reservedNames(extra []string) map[string]bool {
	out := map[string]bool{}
	for _, n := range listFile(reservedNamesFile) {
		out[n] = true
	}
	for _, n := range extra {
		out[strings.ToLower(strings.TrimSpace(n))] = true
	}
	return out
}

var weakPasswords = func() map[string]bool {
	out := map[string]bool{}
	for _, p := range listFile(weakPasswordsFile) {
		out[p] = true
	}
	return out
}()

// passwordOK 按 F-ACC-01 规则 5：8～128 个码点；不能等于用户名；不能全是同一个字符；不能在内置弱口令表里。
func passwordOK(password, usernameLower string) bool {
	n := utf8.RuneCountInString(password)
	if n < PasswordMinLen || n > PasswordMaxLen {
		return false
	}
	lower := strings.ToLower(password)
	if lower == usernameLower || weakPasswords[lower] {
		return false
	}
	first, _ := utf8.DecodeRuneInString(password)
	return strings.Trim(password, string(first)) != ""
}
