package metrics

import (
	"strings"
	"testing"
)

// AC-OPS-90-4：全部 im_* 指标的标签没有用户维度，取值都在枚举内。
func TestLabelsAreBounded(t *testing.T) {
	m := New()
	m.RegisterNode("v1", "go1.27", "n1", func() bool { return true }, func() bool { return false })
	m.Connections.WithLabelValues("web").Inc()
	m.Handshakes.WithLabelValues(HandshakeOriginDenied).Inc()
	m.Disconnects.WithLabelValues(Code(4011), "server", "handshake").Inc()
	m.Kicks.WithLabelValues("banned").Inc()
	m.Commands.WithLabelValues(CmdLabel("whatever"), Code(0)).Inc()
	m.CommandDuration.WithLabelValues(CmdLabel("send")).Observe(0.1)
	m.Messages.WithLabelValues("group").Inc()
	m.AdminAuthFailures.WithLabelValues("nonce").Inc()
	m.OpsRequests.WithLabelValues("node", OpsOK).Inc()
	if p := m.LabelProblems(); len(p) != 0 {
		t.Errorf("problems: %v", p)
	}
}

// 变异：把 userId 放进标签、把未知值放进枚举标签，检查必须报出来。
func TestLabelCheckCatchesViolations(t *testing.T) {
	for _, tc := range []struct{ name, value, want string }{
		{"userId", "u1", "forbidden label"},
		{"uid", "u1", "forbidden label"},
		{"conv_id", "c", "forbidden label"},
		{"code", "abc", "code value"},
		{"cmd", "evil", "cmd value"},
		{"phase", "other", "phase value"},
		{"somenew", "x", "allow list"},
	} {
		if got := labelProblem(tc.name, tc.value); !strings.Contains(got, tc.want) {
			t.Errorf("labelProblem(%s, %s) = %q, want %q", tc.name, tc.value, got, tc.want)
		}
	}
	if got := labelProblem("code", "unknown"); got != "" {
		t.Errorf("unknown code: %q", got)
	}
}

// S-OPS-05：内存和 GC 面板需要的 Go 运行时指标都导出。
func TestRuntimeMetricsExported(t *testing.T) {
	mfs, err := New().registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, mf := range mfs {
		have[mf.GetName()] = true
	}
	for _, name := range []string{"go_gc_gomemlimit_bytes", "go_gc_heap_live_bytes", "go_cpu_classes_gc_total_cpu_seconds_total", "go_cpu_classes_total_cpu_seconds_total", "go_goroutines"} {
		if !have[name] {
			t.Errorf("missing %s", name)
		}
	}
}
