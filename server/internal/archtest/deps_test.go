// Package archtest 用测试守住架构约束（ARCH 2.2），防止以后的改动悄悄破坏分布式扩展能力。
package archtest

import (
	"io/fs"
	"os"
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
	touchSources(t)
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

// 带钩子的用例只有 -tags imhooks 才编译（account_hook_test.go），在这里跑一遍，否则普通的 go test 永远不会执行它们。
func TestHookedSuitePasses(t *testing.T) {
	touchSources(t)
	out, err := exec.Command("go", "test", "-count=1", "-v", "-tags", "imhooks", "-run", "TestHook", "tianting/server/e2e").CombinedOutput()
	if err != nil {
		t.Fatalf("go test -tags imhooks: %v\n%s", err, out)
	}
	// 用例改名或被删掉时 go test 仍然成功（no tests to run），所以逐个确认跑过。
	for _, name := range []string{"TestHookLoginAfterCredentialsChange", "TestHookLoginWithoutChange"} {
		if !strings.Contains(string(out), "--- PASS: "+name) {
			t.Errorf("%s did not run:\n%s", name, out)
		}
	}
}

// touchSources 读一遍全部源文件：go test 的结果缓存只看这个包和它打开过的文件，子进程构建的代码变了它不知道，
// 不读的话 go test ./... 会一直显示上一次的缓存结果。
func touchSources(t *testing.T) {
	t.Helper()
	err := filepath.WalkDir("../..", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		_, err = os.ReadFile(path)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
}
