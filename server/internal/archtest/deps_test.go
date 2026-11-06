// Package archtest 用测试守住架构约束（ARCH 2.2），防止以后的改动悄悄破坏分布式扩展能力。
package archtest

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// 业务代码只能通过 delivery.Router 投递，不能直接接触本节点的在线表；
// 否则换成集群实现后，这些调用只能送达本节点的连接。
func TestBusinessCodeDoesNotDependOnHub(t *testing.T) {
	const hub = "tianting/server/internal/hub"
	for _, pkg := range []string{"tianting/server/internal/logic", "tianting/server/internal/admin"} {
		out, err := exec.Command("go", "list", "-deps", "-f", "{{.ImportPath}}", pkg).CombinedOutput()
		if err != nil {
			t.Fatalf("go list %s: %v\n%s", pkg, err, out)
		}
		for _, dep := range strings.Fields(string(out)) {
			if dep == hub {
				t.Errorf("%s depends on %s; use delivery.Router instead", pkg, hub)
			}
		}
	}
}

// TC-CHAT-90-17（AC-CHAT-90-18）：账号并发钩子用构建标签隔离。不带标签的生产构建没有它的任何符号；
// 带标签的构建有，证明这个断言能发现钩子（PRD-chat 第 4.5 节，BE-405）。
func TestProductionBuildHasNoTestHook(t *testing.T) {
	const pkg = "tianting/server/internal/testhook."
	symbols := func(tags string) string {
		bin := filepath.Join(t.TempDir(), "im-server")
		out, err := exec.Command("go", "build", "-tags", tags, "-o", bin, "tianting/server/cmd/im-server").CombinedOutput()
		if err != nil {
			t.Fatalf("go build -tags %q: %v\n%s", tags, err, out)
		}
		nm, err := exec.Command("go", "tool", "nm", bin).CombinedOutput()
		if err != nil {
			t.Fatalf("go tool nm: %v\n%s", err, nm)
		}
		return string(nm)
	}
	for line := range strings.Lines(symbols("")) {
		if strings.Contains(line, pkg) {
			t.Errorf("production build contains a test hook symbol: %s", line)
		}
	}
	if !strings.Contains(symbols("imhooks"), pkg) {
		t.Error("the imhooks build has no hook symbol: this check cannot detect a hook")
	}
}
