package adminauth

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "regenerate protocol/vectors/admin-sign.json")

var vectorsPath = filepath.Join("..", "..", "..", "protocol", "vectors", "admin-sign.json")

// vector 是一组签名测试数据：固定密钥、时间、nonce、body → 期望的待签名串和签名（PRD-admin F-KEY-03 规则 7）。
// Ed25519 给出私钥种子（seed，32 字节）只为了让各语言的示例能复现签名；im-server 只用 publicKey。
type vector struct {
	Name         string `json:"name"`
	Alg          string `json:"alg"`
	Secret       string `json:"secret,omitempty"`
	Seed         string `json:"seed,omitempty"`
	PublicKey    string `json:"publicKey,omitempty"`
	Method       string `json:"method"`
	Path         string `json:"path"`
	Query        string `json:"query"`
	Timestamp    string `json:"timestamp"`
	Nonce        string `json:"nonce"`
	Body         string `json:"body"`
	Operator     string `json:"operator"`
	RequestID    string `json:"requestId"`
	TargetNode   string `json:"targetNode"`
	StringToSign string `json:"stringToSign"`
	Signature    string `json:"signature"`
}

type vectorFile struct {
	Comment string   `json:"comment"`
	Vectors []vector `json:"vectors"`
}

func (v vector) request() Request {
	return Request{Method: v.Method, Path: v.Path, RawQuery: v.Query, Timestamp: v.Timestamp, Nonce: v.Nonce,
		Body: []byte(v.Body), Operator: v.Operator, RequestID: v.RequestID, TargetNode: v.TargetNode}
}

func generateVectors() vectorFile {
	b64 := base64.StdEncoding.EncodeToString
	hmacKey := []byte("tianting-test-hmac-secret-32bytes!!")
	seed := []byte("tianting-test-ed25519-seed-32byt")
	priv := ed25519.NewKeyFromSeed(seed)
	pub := priv.Public().(ed25519.PublicKey)
	cases := []vector{
		{Name: "hmac: POST with JSON body", Method: "POST", Path: "/admin/users/u1/force-logout", Body: `{}`},
		{Name: "hmac: GET with unsorted query and encoding", Method: "GET", Path: "/admin/audit",
			Query: "to=1762156800000&from=1762070400000&caller=console&operator=admin%3Aa_01", Body: ""},
		{Name: "hmac: Chinese body, operator and request id", Method: "POST", Path: "/admin/users/import",
			Body: `{"users":[{"userId":"u1","nickname":"张三"}]}`, Operator: "admin:a_01J0000000", RequestID: "r-1"},
		{Name: "ed25519: POST with operator", Method: "POST", Path: "/admin/groups",
			Body: `{"groupId":"g1","name":"测试群","members":["u1"]}`, Operator: "admin:a_01J0000000", RequestID: "req-ed-1"},
		{Name: "ed25519: GET empty body, repeated keys and spaces in query", Method: "GET", Path: "/admin/groups",
			Query: "q=%E6%B5%8B%E8%AF%95+%E7%BE%A4&limit=20&q=a", Body: ""},
		{Name: "ed25519: node ops call with target node", Method: "GET", Path: "/ops/connections", Query: "userId=u1",
			Body: "", Operator: "ops:o_01", RequestID: "r-ops-1", TargetNode: "n1"},
	}
	for i := range cases {
		c := &cases[i]
		c.Timestamp = "1762156800000"
		c.Nonce = "AAECAwQFBgcICQoLDA0ODw" + string(rune('a'+i))
		c.StringToSign = StringToSign(c.request())
		if i < 3 {
			c.Alg, c.Secret = AlgHMAC, b64(hmacKey)
			c.Signature = EncodeSignature(SignHMAC(hmacKey, c.StringToSign))
		} else {
			c.Alg, c.Seed, c.PublicKey = AlgEd25519, b64(seed), b64(pub)
			c.Signature = EncodeSignature(ed25519.Sign(priv, []byte(c.StringToSign)))
		}
	}
	return vectorFile{
		Comment: "管理接口签名测试向量（PRD-admin F-KEY-03）。secret / seed / publicKey 为 base64；signature 为 base64url 无填充；" +
			"stringToSign 是 8 行、\\n 分隔的待签名串。由 server/internal/adminauth 的 TestVectors -update 生成，不要手改。",
		Vectors: cases,
	}
}

// AC-KEY-03-1：签名校验与向量一致（HMAC、Ed25519 各 3 组，含查询串排序、空 body、中文 body）。
func TestVectors(t *testing.T) {
	if *update {
		b, _ := json.MarshalIndent(generateVectors(), "", "  ")
		if err := os.WriteFile(vectorsPath, append(b, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(vectorsPath)
	if err != nil {
		t.Fatal(err)
	}
	var f vectorFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, v := range f.Vectors {
		counts[v.Alg]++
		if got := StringToSign(v.request()); got != v.StringToSign {
			t.Errorf("%s: stringToSign\n got %q\nwant %q", v.Name, got, v.StringToSign)
		}
		var items []callerJSON
		items = append(items, callerJSON{ID: "vec", Alg: v.Alg, Secret: v.Secret, PublicKey: v.PublicKey, Scopes: []Scope{ScopeUserRead}})
		callers, err := parseCallers(items, nil)
		if err != nil {
			t.Fatalf("%s: %v", v.Name, err)
		}
		if !callers[0].Verify(v.StringToSign, v.Signature) {
			t.Errorf("%s: signature does not verify", v.Name)
		}
		if callers[0].Verify(v.StringToSign+"x", v.Signature) {
			t.Errorf("%s: tampered string verifies", v.Name)
		}
	}
	if counts[AlgHMAC] < 3 || counts[AlgEd25519] < 3 {
		t.Errorf("need at least 3 vectors per algorithm, got %v", counts)
	}
}
