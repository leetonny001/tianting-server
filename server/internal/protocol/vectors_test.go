// Package protocol 的测试保证 protocol/ 下的 Schema 与用例一致。
package protocol

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// 相对本包目录：server/internal/protocol → im-mvp/protocol
const protocolDir = "../../../protocol"

type vectorFile struct {
	Valid   []vector `json:"valid"`
	Invalid []vector `json:"invalid"`
}

type vector struct {
	Name  string `json:"name"`
	Frame any    `json:"frame"`
}

func TestFrameVectors(t *testing.T) {
	schema := compileSchema(t)

	raw, err := os.ReadFile(filepath.Join(protocolDir, "vectors", "frames.json"))
	if err != nil {
		t.Fatal(err)
	}
	var vf vectorFile
	if err := json.Unmarshal(raw, &vf); err != nil {
		t.Fatalf("parse frames.json: %v", err)
	}
	if len(vf.Valid) == 0 || len(vf.Invalid) == 0 {
		t.Fatal("frames.json must contain both valid and invalid vectors")
	}

	for _, v := range vf.Valid {
		t.Run("valid/"+v.Name, func(t *testing.T) {
			if err := schema.Validate(v.Frame); err != nil {
				t.Errorf("expected valid, got: %v", err)
			}
		})
	}
	for _, v := range vf.Invalid {
		t.Run("invalid/"+v.Name, func(t *testing.T) {
			if err := schema.Validate(v.Frame); err == nil {
				t.Error("expected schema to reject frame")
			}
		})
	}
}

func compileSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	path, err := filepath.Abs(filepath.Join(protocolDir, "schema", "frame.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	doc, err := jsonschema.UnmarshalJSON(f)
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}

	c := jsonschema.NewCompiler()
	const id = "https://im-mvp.local/protocol/frame.schema.json"
	if err := c.AddResource(id, doc); err != nil {
		t.Fatal(err)
	}
	s, err := c.Compile(id)
	if err != nil {
		t.Fatalf("compile schema: %v", err)
	}
	return s
}
