package protocol

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// compileHTTPDef 编译 http-client.schema.json 里的一个 $defs 项（BE-400）。
func compileHTTPDef(t *testing.T, def string) *jsonschema.Schema {
	t.Helper()
	f, err := os.Open(filepath.Join(protocolDir, "schema", "http-client.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	doc, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	const id = "https://im-mvp.local/protocol/http-client.schema.json"
	if err := c.AddResource(id, doc); err != nil {
		t.Fatal(err)
	}
	s, err := c.Compile(id + "#/$defs/" + def)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// TC-CHAT-90-09（AC-CHAT-90-10）：HTTP Schema 的合法和非法样例；每个接口引用的 $defs 都存在。
func TestHTTPClientSchema(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(protocolDir, "schema", "http-client.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Defs      map[string]any `json:"$defs"`
		Endpoints map[string]struct {
			Request  string `json:"request"`
			Response string `json:"response"`
		} `json:"endpoints"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	for name, ep := range doc.Endpoints {
		for _, def := range []string{ep.Request, ep.Response} {
			if _, ok := doc.Defs[def]; def != "" && !ok {
				t.Errorf("%s: unknown def %q", name, def)
			}
		}
	}
	for _, tc := range []struct {
		def   string
		json  string
		valid bool
	}{
		{"RegisterRequest", `{"username":"Alice_1","password":"x","platform":"android"}`, true},
		{"RegisterRequest", `{"username":"Alice_1","password":"x","platform":"tv"}`, false},
		{"RegisterRequest", `{"password":"x","platform":"ios"}`, false},
		{"AuthResponse", `{"userId":"alice_1","accessToken":"a","accessExpiresAt":1,"refreshToken":"k1.c2lk.1.bm9uY2U.bWFj","refreshExpiresAt":2,"absoluteExpiresAt":3}`, true},
		{"AuthResponse", `{"userId":"alice_1","accessToken":"a","accessExpiresAt":1,"refreshToken":"k1.c2lk.0.bm9uY2U.bWFj","refreshExpiresAt":2,"absoluteExpiresAt":3}`, false},
		{"AuthResponse", `{"userId":"Alice_1","accessToken":"a","accessExpiresAt":1,"refreshToken":"k1.c2lk.1.bm9uY2U.bWFj","refreshExpiresAt":2,"absoluteExpiresAt":3}`, false},
		{"ErrorResponse", `{"code":4018,"msg":"x"}`, true},
		{"ErrorResponse", `{"msg":"x"}`, false},
		{"ConfigResponse", `{"registerMode":"invite","demoLogin":false,"features":{"files":false},"limits":{"usernameMinLen":4,"usernameMaxLen":32,"passwordMinLen":8,"passwordMaxLen":128,"nicknameMaxLen":32,"deviceNameMaxLen":64}}`, true},
		{"ConfigResponse", `{"registerMode":"public","demoLogin":false,"features":{"files":false},"limits":{}}`, false},
	} {
		v, err := jsonschema.UnmarshalJSON(jsonReader(tc.json))
		if err != nil {
			t.Fatal(err)
		}
		err = compileHTTPDef(t, tc.def).Validate(v)
		if (err == nil) != tc.valid {
			t.Errorf("%s %s: valid=%v, err=%v", tc.def, tc.json, tc.valid, err)
		}
	}
}

func jsonReader(s string) *strings.Reader { return strings.NewReader(s) }
