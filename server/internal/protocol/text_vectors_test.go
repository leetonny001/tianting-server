package protocol

import (
	"encoding/json"
	"os"
	"testing"
)

// C-26：服务端与各端 SDK 跑同一份文本用例（protocol/vectors/text.json）。
func TestTextVectors(t *testing.T) {
	b, err := os.ReadFile("../../../protocol/vectors/text.json")
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Cases []struct {
			Name, Input, Trimmed string
			Length               int
		}
	}
	if err := json.Unmarshal(b, &spec); err != nil {
		t.Fatal(err)
	}
	for _, c := range spec.Cases {
		if got := TrimText(c.Input); got != c.Trimmed {
			t.Errorf("%s: TrimText = %q, want %q", c.Name, got, c.Trimmed)
		}
		if got := TextLen(c.Trimmed); got != c.Length {
			t.Errorf("%s: TextLen = %d, want %d", c.Name, got, c.Length)
		}
	}
}
