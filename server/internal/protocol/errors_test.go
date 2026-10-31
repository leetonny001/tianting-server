package protocol

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type errorsFile struct {
	Errors []struct {
		Code int    `json:"code"`
		Name string `json:"name"`
		HTTP *int   `json:"http"`
	} `json:"errors"`
	CloseCodes []struct {
		Code int    `json:"code"`
		Name string `json:"name"`
	} `json:"closeCodes"`
	KickReasons []struct {
		Reason    string `json:"reason"`
		CloseCode *int   `json:"closeCode"`
	} `json:"kickReasons"`
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

func TestHTTPStatus(t *testing.T) {
	for code, want := range map[int]int{4042: 404, 4090: 409, 4013: 401, 4131: 400, 4290: 429, 5030: 503, 4049: 404, 5999: 599} {
		if got := HTTPStatus(code); got != want {
			t.Errorf("HTTPStatus(%d) = %d, want %d", code, got, want)
		}
	}
}
