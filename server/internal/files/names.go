package files

import (
	_ "embed"
	"fmt"
	"os"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// maxNameBytes 是清洗后文件名的上限（F-FILE-01 规则 7）。
const maxNameBytes = 255

// SanitizeName 按 F-FILE-01 规则 7 清洗文件名：先 NFC；去掉 `::` 之后的内容、路径分隔符、控制字符、Bidi 控制和不可见字符
// （Default_Ignorable）、开头的点、结尾的点和空格。清洗后为空用 file。返回值超过 255 字节时由调用方拒绝（fileNameTooLong）。
func SanitizeName(name string) string {
	name = norm.NFC.String(strings.ToValidUTF8(name, ""))
	if i := strings.Index(name, "::"); i >= 0 { // NTFS 备用数据流：a.exe::$DATA
		name = name[:i]
	}
	name = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || unicode.IsControl(r) || isBidiOrIgnorable(r) {
			return -1
		}
		return r
	}, name)
	name = strings.TrimLeft(name, ".")
	name = strings.TrimRight(name, ". ")
	if name == "" {
		return "file"
	}
	return name
}

// isBidiOrIgnorable 是 Bidi 控制字符和 Default_Ignorable_Code_Point（近似：格式字符 Cf、Other_Default_Ignorable、变体选择符、
// 韩文填充符、组合用字形连接符），这些字符不可见，会让显示的名字和真实扩展名不同（photo\u202Egpj.html，RC2-25）。
func isBidiOrIgnorable(r rune) bool {
	switch r {
	case 0x034F, 0x115F, 0x1160, 0x3164, 0xFFA0, 0x061C, 0x180E:
		return true
	}
	return unicode.Is(unicode.Cf, r) || unicode.Is(unicode.Other_Default_Ignorable_Code_Point, r) || unicode.Is(unicode.Variation_Selector, r) ||
		unicode.Is(unicode.Bidi_Control, r)
}

// Extension 是清洗后文件名的扩展名（小写，不含点）。没有点时为空。
func Extension(name string) string {
	i := strings.LastIndexByte(name, '.')
	if i < 0 {
		return ""
	}
	return strings.ToLower(name[i+1:])
}

//go:embed blocked_extensions.txt
var defaultBlocked string

// Blocklist 是文件禁止的扩展名（F-FILE-05 规则 1，配置文件维护）。
type Blocklist map[string]bool

// LoadBlocklist 读内置名单，再并上 path（IM_FILE_BLOCKLIST，一行一个扩展名，# 开头是注释）里的条目。
// 只能加不能减：配置文件写错不会把可执行文件放出去。改配置要重启（AC-FILE-05-2）。
func LoadBlocklist(path string) (Blocklist, error) {
	b := Blocklist{}
	add := func(text string) {
		for _, line := range strings.Split(text, "\n") {
			line = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), ".")))
			if line != "" && !strings.HasPrefix(line, "#") {
				b[line] = true
			}
		}
	}
	add(defaultBlocked)
	if path != "" {
		raw, err := os.ReadFile(path) // #nosec G304 -- 运维配置的路径
		if err != nil {
			return nil, fmt.Errorf("IM_FILE_BLOCKLIST: %w", err)
		}
		add(string(raw))
	}
	return b, nil
}

// Blocked 判断清洗后的文件名是否是禁止的扩展名。
func (b Blocklist) Blocked(name string) bool { return b[Extension(name)] }

// Disposition 生成 response-content-disposition（F-FILE-03 规则 4，RS4-20）：文件名只用 RFC 5987 / 6266 的
// filename* 参数（UTF-8 百分号编码），加一个 ASCII 安全的回退名（不含 `"` `;` `%` `\` 和控制字符），不拼接原始字符串。
func Disposition(kind, name string) string {
	if kind != "attachment" {
		return "inline"
	}
	var fb strings.Builder
	for _, r := range name {
		if r < 0x20 || r > 0x7E || r == '"' || r == ';' || r == '%' || r == '\\' {
			fb.WriteByte('_')
			continue
		}
		fb.WriteRune(r)
	}
	var enc strings.Builder
	for _, c := range []byte(name) {
		if c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || strings.IndexByte("!#$&+-.^_`|~", c) >= 0 {
			enc.WriteByte(c)
			continue
		}
		fmt.Fprintf(&enc, "%%%02X", c)
	}
	return `attachment; filename="` + fb.String() + `"; filename*=UTF-8''` + enc.String()
}
