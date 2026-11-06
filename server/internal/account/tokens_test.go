package account

import (
	"strings"
	"testing"
)

// F-ACC-03 规则 3：kid.sid.gen.nonce.mac。mac 错误、改代号、改 sid、不认识的 kid 都是 invalid；旧密钥的 token 还能验。
func TestRefreshTokenFormat(t *testing.T) {
	c, err := newRefreshCodec([]RefreshKey{{ID: "k1", Secret: []byte("secret-1")}, {ID: "k2", Secret: []byte("secret-2")}}, "k2")
	if err != nil {
		t.Fatal(err)
	}
	sid, err := newSID()
	if err != nil {
		t.Fatal(err)
	}
	if len(sid) < 22 || strings.Contains(sid, ".") {
		t.Fatalf("sid %q", sid)
	}
	tok, err := c.issue(sid, 7)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(tok, ".")
	if len(parts) != 5 || parts[0] != "k2" || parts[1] != sid || parts[2] != "7" {
		t.Fatalf("token %q", tok)
	}
	got, err := c.parse(tok)
	if err != nil || got.SID != sid || got.Gen != 7 || got.KID != "k2" {
		t.Fatalf("%+v %v", got, err)
	}
	again, _ := c.issue(sid, 7)
	if again == tok {
		t.Error("the nonce is random: a retry must not return the same value (RS4-04)")
	}
	for name, bad := range map[string]string{
		"改代号":    strings.Join([]string{parts[0], parts[1], "8", parts[3], parts[4]}, "."),
		"改 sid":  strings.Join([]string{parts[0], "x" + parts[1], parts[2], parts[3], parts[4]}, "."),
		"换 kid":  strings.Join([]string{"k1", parts[1], parts[2], parts[3], parts[4]}, "."),
		"未知 kid": strings.Join([]string{"k9", parts[1], parts[2], parts[3], parts[4]}, "."),
		"错误 mac": strings.Join([]string{parts[0], parts[1], parts[2], parts[3], "AAAA"}, "."),
		"段数不对":   strings.Join(parts[:4], "."),
		"空":      "",
	} {
		if _, err := c.parse(bad); err == nil {
			t.Errorf("%s: %q must be invalid", name, bad)
		}
	}
	// 密钥轮换：旧 kid 签的还能验；mac 和 JWT 无关。
	old, _ := newRefreshCodec([]RefreshKey{{ID: "k1", Secret: []byte("secret-1")}}, "k1")
	oldTok, _ := old.issue(sid, 1)
	if _, err := c.parse(oldTok); err != nil {
		t.Errorf("a token signed by a retired key: %v", err)
	}
}

func TestRecoveryCode(t *testing.T) {
	code, err := newRecoveryCode()
	if err != nil {
		t.Fatal(err)
	}
	if n := len(strings.ReplaceAll(code, "-", "")); n != 26 {
		t.Errorf("%q: %d characters, want 26 (128 bits)", code, n)
	}
	h := RecoveryHash("alice_1", code)
	for _, variant := range []string{strings.ToLower(code), strings.ReplaceAll(code, "-", ""), " " + strings.ReplaceAll(code, "-", " ") + " "} {
		if RecoveryHash("alice_1", variant) != h {
			t.Errorf("%q normalizes differently", variant)
		}
	}
	if RecoveryHash("alice_2", code) == h {
		t.Error("the hash depends on userId")
	}
	if strings.Contains(h, code) {
		t.Error("hash contains the code")
	}
	// 长度前缀拼接：不能靠移动边界得到相同的哈希。
	if RecoveryHash("ab", "C") == RecoveryHash("a", "BC") {
		t.Error("length prefix is missing")
	}
}

func TestPasswordRules(t *testing.T) {
	for pw, want := range map[string]bool{
		"1234567":                      false, // 7
		"Tt-pass-2026!":                true,
		"aaaaaaaa":                     false,
		"密密密密密密密密":                     false,
		"password1":                    false,
		"PASSWORD1":                    false,
		"weak_user":                    false,
		"WEAK_USER":                    false,
		"八个码点的中文密码":                    true,
		strings.Repeat("ab", 64):       true, // 128
		strings.Repeat("ab", 64) + "c": false,
	} {
		if got := passwordOK(pw, "weak_user"); got != want {
			t.Errorf("passwordOK(%q) = %v, want %v", pw, got, want)
		}
	}
}

func TestUsernameRules(t *testing.T) {
	for raw, want := range map[string]bool{
		"abc": false, "abcd": true, "Alice_1": true, "1abc": false, "a b c": false, "张三abc": false, "  Valid_User  ": true,
		"a" + strings.Repeat("b", 31): true, "a" + strings.Repeat("b", 32): false, "Kabc": false, "ab-c": true, "_abc": false,
	} {
		if got := ValidUsername(raw); got != want {
			t.Errorf("ValidUsername(%q) = %v, want %v", raw, got, want)
		}
	}
	if NormalizeUsername("  Valid_User ") != "valid_user" {
		t.Error("normalize")
	}
}
