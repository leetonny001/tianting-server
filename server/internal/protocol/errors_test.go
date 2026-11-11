package protocol

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
)

type errorsFile struct {
	Errors []struct {
		Code  int      `json:"code"`
		Name  string   `json:"name"`
		HTTP  *int     `json:"http"`
		Scope []string `json:"scope"`
	} `json:"errors"`
	CloseCodes []struct {
		Code int    `json:"code"`
		Name string `json:"name"`
	} `json:"closeCodes"`
	KickReasons []struct {
		Reason    string `json:"reason"`
		CloseCode *int   `json:"closeCode"`
	} `json:"kickReasons"`
	Reasons map[string]any `json:"reasons"`
}

// TC-SYS-01-*：Go 里的错误码、关闭码、被踢原因必须与 errors.json 完全一致（AC-SYS-01-1）。
func TestCodesMatchErrorsJSON(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(protocolDir, "errors.json"))
	if err != nil {
		t.Fatal(err)
	}
	var f errorsFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}

	seen := map[int]bool{}
	for _, e := range f.Errors {
		// 只属于 console-server 的码（PRD-admin 10.4 节第 5 条）：im-server 不定义常量，console-server 有自己的一致性测试。
		if len(e.Scope) == 1 && e.Scope[0] == "console" {
			continue
		}
		seen[e.Code] = true
		info, ok := codes[e.Code]
		if !ok {
			t.Errorf("errors.json has %d %s, Go does not", e.Code, e.Name)
			continue
		}
		wantHTTP := 0
		if e.HTTP != nil {
			wantHTTP = *e.HTTP
		}
		if info.Name != e.Name || info.HTTP != wantHTTP {
			t.Errorf("%d: Go {%s %d}, errors.json {%s %d}", e.Code, info.Name, info.HTTP, e.Name, wantHTTP)
		}
	}
	for code, info := range codes {
		if !seen[code] {
			t.Errorf("Go defines %d %s, errors.json does not", code, info.Name)
		}
	}

	if len(f.CloseCodes) != len(closeCodes) {
		t.Errorf("close codes: errors.json %d, Go %d", len(f.CloseCodes), len(closeCodes))
	}
	for _, c := range f.CloseCodes {
		if closeCodes[c.Code] != c.Name {
			t.Errorf("close code %d: Go %q, errors.json %q", c.Code, closeCodes[c.Code], c.Name)
		}
	}

	for _, k := range f.KickReasons {
		got, ok := KickCloseCode[k.Reason]
		switch {
		case k.CloseCode == nil && ok:
			t.Errorf("kick %s: errors.json has no close code, Go has %d", k.Reason, got)
		case k.CloseCode != nil && got != *k.CloseCode:
			t.Errorf("kick %s: Go %d, errors.json %d", k.Reason, got, *k.CloseCode)
		}
	}
}

// TC-CHAT-90-12（AC-CHAT-90-14）：一致性测试纳入 client scope。client 通道的码都有 HTTP 状态，
// PRD-chat 第 11.5 节要求的码都在 client 通道里；reason 和 Go 常量逐个一致。三套 SDK 的枚举在各自仓库（FE-600～602）。
func TestClientScopeCodes(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(protocolDir, "errors.json"))
	if err != nil {
		t.Fatal(err)
	}
	var f errorsFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	client := map[int]bool{}
	for _, e := range f.Errors {
		if slices.Contains(e.Scope, "client") {
			client[e.Code] = true
			if e.HTTP == nil {
				t.Errorf("%d %s: client scope needs an http status", e.Code, e.Name)
			}
		}
	}
	for _, c := range []int{4000, 4004, 4010, 4011, 4012, 4018, 4019, 4030, 4034, 4048, 4095, 4096, 4131, 4139, 4150, 4290, 4292, 4294, 4510, 5000, 5030, 5032} {
		if !client[c] {
			t.Errorf("%d is not in the client scope", c)
		}
	}
	for k, v := range f.Reasons {
		if k == "$comment" {
			continue
		}
		code, err := strconv.Atoi(k)
		if err != nil {
			t.Fatalf("reasons key %q", k)
		}
		list, _ := v.([]any)
		want := make([]string, 0, len(list))
		for _, r := range list {
			want = append(want, r.(string))
		}
		if !slices.Equal(reasons[code], want) {
			t.Errorf("reasons of %d: Go %v, errors.json %v", code, reasons[code], want)
		}
	}
	if len(reasons) != len(f.Reasons)-1 {
		t.Errorf("reasons: Go has %d codes, errors.json %d", len(reasons), len(f.Reasons)-1)
	}
}

func TestHTTPStatus(t *testing.T) {
	for code, want := range map[int]int{4042: 404, 4090: 409, 4013: 401, 4131: 400, 4290: 429, 5030: 503, 4049: 404, 5999: 599, 4510: 451, 4098: 409} {
		if got := HTTPStatus(code); got != want {
			t.Errorf("HTTPStatus(%d) = %d, want %d", code, got, want)
		}
	}
}
