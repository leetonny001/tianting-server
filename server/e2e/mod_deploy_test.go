package e2e

// C6 部署（BE-498）：deploy/im-admin-callers.example.json 换上现场生成的公钥后能被真实的调用方加载，scope 符合 PRD-chat 第 14.1 节；
// 部署说明里有上线门槛清单的 11 条，没有提交任何密钥。deploy/test_deploy.py 的同类检查在 CI 跑（本机缺 PyYAML）。

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"tianting/server/internal/adminauth"
)

func TestModDeployCallersExample(t *testing.T) {
	raw, err := os.ReadFile("../../deploy/im-admin-callers.example.json")
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for strings.Contains(s, "CHANGE_ME_BASE64_ED25519_PUBLIC_KEY") {
		pub, _, _ := ed25519.GenerateKey(rand.Reader)
		s = strings.Replace(s, "CHANGE_ME_BASE64_ED25519_PUBLIC_KEY", base64.StdEncoding.EncodeToString(pub), 1)
	}
	p := filepath.Join(t.TempDir(), "callers.json")
	if err := os.WriteFile(p, []byte(s), 0o600); err != nil {
		t.Fatal(err)
	}
	callers, err := adminauth.LoadCallersFile(p)
	if err != nil {
		t.Fatalf("the example does not load: %v", err)
	}
	by := map[string]adminauth.Caller{}
	for _, c := range callers {
		by[c.ID] = c
	}
	console, reader := by["console"], by["console-msgread"]
	for _, sc := range []adminauth.Scope{adminauth.ScopeModerationWords, adminauth.ScopeReportRead, adminauth.ScopeReportHandle} {
		if !console.Has(sc) {
			t.Errorf("console lacks %s", sc)
		}
	}
	if console.Has(adminauth.ScopeReportEvidence) || !reader.Has(adminauth.ScopeReportEvidence) || !reader.BodyAccess {
		t.Errorf("report.evidence must be on console-msgread only: console=%v reader=%v", console.Scopes, reader.Scopes)
	}
}

func TestModDeployRunbook(t *testing.T) {
	b, err := os.ReadFile("../../deploy/MODERATION-RUNBOOK.md")
	if err != nil {
		t.Fatal(err)
	}
	doc := string(b)
	for _, n := range []string{"①", "②", "③", "④", "⑤", "⑥", "⑦", "⑧", "⑨", "⑩", "⑪"} {
		if !strings.Contains(doc, "| "+n+" |") {
			t.Errorf("the go-live checklist lacks item %s", n)
		}
	}
	for _, want := range []string{"IM_REPORT_KEY", "第一批不支持轮换", "report_purge", "moderation_hits_purge", "词库加载失败", "im_reports_pending", "已部署环境要手动加"} {
		if !strings.Contains(doc, want) {
			t.Errorf("the runbook does not mention %s", want)
		}
	}
	// 不提交密钥：没有像 32 字节 base64 的串（占位和命令除外）。
	if m := regexp.MustCompile(`[A-Za-z0-9+/]{43}=`).FindString(doc); m != "" {
		t.Errorf("the runbook contains something that looks like a key: %s", m)
	}
}
