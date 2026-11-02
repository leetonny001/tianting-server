// Package archtest 用测试守住架构约束（ARCH 2.2），防止以后的改动悄悄破坏分布式扩展能力。
package archtest

import (
	"os/exec"
	"strings"
	"testing"
)

// 业务代码只能通过 delivery.Router 投递，不能直接接触本节点的在线表；
// 否则换成集群实现后，这些调用只能送达本节点的连接。
func TestBusinessCodeDoesNotDependOnHub(t *testing.T) {
	const hub = "im-mvp/server/internal/hub"
	for _, pkg := range []string{"im-mvp/server/internal/logic", "im-mvp/server/internal/admin"} {
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
