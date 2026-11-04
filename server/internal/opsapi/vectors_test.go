package opsapi

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"tianting/server/internal/adminauth"
)

var update = flag.Bool("update", false, "regenerate protocol/vectors/ops-sign.json")

var vectorsPath = filepath.Join("..", "..", "..", "protocol", "vectors", "ops-sign.json")

// opsVector 是运维接口的签名向量，console-server（CS-111）和 im-server 共用。种子只为了让两边能复现签名；节点只配公钥。
type opsVector struct {
	Name         string `json:"name"`
	Seed         string `json:"seed"`
	PublicKey    string `json:"publicKey"`
	Method       string `json:"method"`
	Path         string `json:"path"`
	Timestamp    string `json:"timestamp"`
	Nonce        string `json:"nonce"`
	Body         string `json:"body"`
	Operator     string `json:"operator"`
	RequestID    string `json:"requestId"`
	TargetNode   string `json:"targetNode"`
	StringToSign string `json:"stringToSign"`
	Signature    string `json:"signature"`
}

type opsVectorFile struct {
	Comment string      `json:"comment"`
	Vectors []opsVector `json:"vectors"`
}

func generateVectors() opsVectorFile {
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	key := ed25519.NewKeyFromSeed(seed)
	cases := []opsVector{
		{Name: "node", Method: "GET", Path: "/internal/v1/node", Timestamp: "1800000000000", Nonce: "AAAAAAAAAAAAAAAAAAAAAAAA", Operator: "ops:a1", RequestID: "req-1", TargetNode: "n1"},
		{Name: "connections", Method: "POST", Path: "/internal/v1/connections:query", Timestamp: "1800000000001", Nonce: "BBBBBBBBBBBBBBBBBBBBBBBB", Body: `{"userId":"u1"}`, Operator: "ops:a1", RequestID: "req-2", TargetNode: "n2"},
		{Name: "probe without operator", Method: "GET", Path: "/internal/v1/db", Timestamp: "1800000000002", Nonce: "CCCCCCCCCCCCCCCCCCCCCCCC", TargetNode: "n1"},
	}
	for i := range cases {
		c := &cases[i]
		c.Seed = base64.StdEncoding.EncodeToString(seed)
		c.PublicKey = base64.StdEncoding.EncodeToString(key.Public().(ed25519.PublicKey))
		c.StringToSign = adminauth.StringToSign(adminauth.Request{
			Method: c.Method, Path: c.Path, Timestamp: c.Timestamp, Nonce: c.Nonce, Body: []byte(c.Body),
			Operator: c.Operator, RequestID: c.RequestID, TargetNode: c.TargetNode,
		})
		c.Signature = adminauth.EncodeSignature(ed25519.Sign(key, []byte(c.StringToSign)))
	}
	return opsVectorFile{
		Comment: "运维接口签名向量（PRD-ops F-OPS-16），由 server/internal/opsapi/vectors_test.go 生成：go test ./internal/opsapi -run TestOpsVectors -update。调用方固定为 console-ops，第 8 行 X-IM-Target-Node 必填。",
		Vectors: cases,
	}
}

func TestOpsVectors(t *testing.T) {
	want := generateVectors()
	if *update {
		b, err := json.MarshalIndent(want, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(vectorsPath, append(b, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(vectorsPath)
	if err != nil {
		t.Fatal(err)
	}
	var got opsVectorFile
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Vectors) != len(want.Vectors) {
		t.Fatalf("vectors = %d, want %d", len(got.Vectors), len(want.Vectors))
	}
	for i, v := range got.Vectors {
		if v != want.Vectors[i] {
			t.Errorf("%s differs from the generator; rerun with -update", v.Name)
		}
		pub, _ := base64.StdEncoding.DecodeString(v.PublicKey)
		c := adminauth.Caller{ID: CallerID, Alg: adminauth.AlgEd25519, PublicKey: pub}
		if !c.Verify(v.StringToSign, v.Signature) {
			t.Errorf("%s: signature does not verify", v.Name)
		}
	}
}
