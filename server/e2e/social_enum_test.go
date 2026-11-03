package e2e

// PRD-social 11.2 节逐路径的防枚举用例、日志 / 审计扫描、搜索耗时不可区分（BE-120，BE-119 第 4 条）。
// 统一用"除 id / ver / 时间戳 / userId 外 JSON 逐字段比较"（stripVolatile）。
// 隐私负向断言（被单向删除、被拒绝、被拉黑、被静默的一方零帧、ver 不变）见 TestTCSOC0614VerPerOperation 的逐行检查。

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"tianting/server/internal/protocol"
)

// 11.2 节每一行：存在但对我不可见 / 拉黑了我 的用户，与不存在的用户应答相同。
func TestTCSOC1120EnumerationPaths(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "hidden", "blocker")
	hidden, blocker := s.dial("hidden", "web"), s.dial("blocker", "web")
	mustAck[protocol.VerAck](t, hidden.call(protocol.CmdSetPrivacy, map[string]any{"searchable": false}))
	mustAck[socialAck](t, blocker.call(protocol.CmdBlock, map[string]any{"userId": "alice"}))
	alice := s.dial("alice", "web")
	same := func(path string, a, b frame) {
		t.Helper()
		if a.Error != nil || b.Error != nil {
			if errBody(a) != errBody(b) {
				t.Errorf("%s: %s vs %s", path, errBody(a), errBody(b))
			}
			return
		}
		if stripVolatile(t, a.Data) != stripVolatile(t, b.Data) {
			t.Errorf("%s:\n %s\n %s", path, a.Data, b.Data)
		}
	}
	// 搜索：不可被搜索与不存在相同。可被搜索的用户能被确认存在，在 searchDailyLimit 预算内（11.2 节）。
	same("userSearch", alice.call(protocol.CmdUserSearch, map[string]any{"keyword": "hidden"}), alice.call(protocol.CmdUserSearch, map[string]any{"keyword": "nobody1"}))
	same("friendApply", alice.call(protocol.CmdFriendApply, map[string]any{"to": "blocker"}), alice.call(protocol.CmdFriendApply, map[string]any{"to": "nobody2"}))
	same("block", alice.call(protocol.CmdBlock, map[string]any{"userId": "hidden"}), alice.call(protocol.CmdBlock, map[string]any{"userId": "nobody3"}))
	same("unblock", alice.call(protocol.CmdUnblock, map[string]any{"userId": "hidden"}), alice.call(protocol.CmdUnblock, map[string]any{"userId": "nobody3"}))
	same("users", alice.call(protocol.CmdUsers, map[string]any{"userIds": []string{"hidden"}}), alice.call(protocol.CmdUsers, map[string]any{"userIds": []string{"nobody4"}}))
	same("friendRemark", alice.call(protocol.CmdFriendRemark, map[string]any{"userId": "hidden", "remark": "x"}), alice.call(protocol.CmdFriendRemark, map[string]any{"userId": "nobody5", "remark": "x"}))
	same("friendDelete", alice.call(protocol.CmdFriendDelete, map[string]any{"userId": "hidden"}), alice.call(protocol.CmdFriendDelete, map[string]any{"userId": "nobody6"}))
	same("friendRespond", alice.respond("hidden", "accept", nil), alice.respond("nobody7", "accept", nil))
	all, _ := alice.state()
	for _, b := range all.Blocks {
		if b.Profile != nil {
			t.Errorf("block item %s exposes a profile", b.UserID)
		}
	}
	for _, r := range all.Requests {
		if r.Direction == "out" && r.Profile != nil {
			t.Errorf("out request to %s exposes a profile", r.UserID)
		}
	}
}

// 1.3 节指标：对方无感知的前提下，一个账号一天最多确认 100（私聊被拒）+ 100（搜索）个 userId。
func TestTCSOC1120DailyBudget(t *testing.T) {
	s := newSocEnvFastSend(t)
	s.usersNamed("alice")
	alice := s.dial("alice", "web")
	rejected := 0
	for i := 0; i < 150; i++ {
		f := alice.sendText("c2c", "probe"+pad4(i), cid("m-bud", i), "hi")
		if f.Error != nil && f.Error.Code == protocol.CodeUserNotFound {
			rejected++
		}
	}
	searched := 0
	for i := 0; i < 150; i++ {
		s.advance(6 * time.Second) // 分钟限流每 6 秒恢复 1 次；每日配额按 864 秒恢复 1 次
		if alice.call(protocol.CmdUserSearch, map[string]any{"keyword": "probe" + pad4(i)}).Error == nil {
			searched++
		}
	}
	if rejected != 100 || searched > 101 {
		t.Fatalf("confirmed via send %d (want 100), via search %d (want <= 101)", rejected, searched)
	}
}

// AC-SOC-02-11、09-8、12-9：四个特征串处理完后，整个服务端日志和 admin_audit 全部字段都搜不到。
func TestTCSOC1120LogAndAuditScan(t *testing.T) {
	s, logs := newSocEnvWithLog(t)
	s.usersNamed("alice", "bob")
	alice := s.dial("alice", "web")
	markers := []string{"ZXQ-SOCIAL-123", "ZXQ-REMARK-456", "ZXQ-SEARCH-9", "ZXQ-REMARK-789"}
	alice.call(protocol.CmdFriendApply, map[string]any{"to": "bob", "message": markers[0], "remark": markers[1]})
	alice.call(protocol.CmdFriendApply, map[string]any{"to": "bob", "message": strings.Repeat(markers[0], 4)}) // 4000
	alice.call(protocol.CmdUserSearch, map[string]any{"keyword": markers[2]})
	alice.call(protocol.CmdUserSearch, map[string]any{"keyword": markers[2] + " x"}) // 4004
	s.mustAdm(http.MethodPost, "/admin/friends/import", map[string]any{"pairs": []any{map[string]any{"a": "alice", "b": "bob", "remarkA": markers[3]}}})
	s.adm(http.MethodPost, "/admin/friends/import", map[string]any{"pairs": []any{map[string]any{"a": "alice", "b": "nobody", "remarkA": markers[3]}}}) // 4042
	mustAck[socialAck](t, alice.call(protocol.CmdFriendRemark, map[string]any{"userId": "bob", "remark": markers[1]}))
	audit := string(s.mustAdm(http.MethodGet, fmt.Sprintf("/admin/audit?from=%d&to=%d&limit=200", s.now-60000, s.now+1000), nil))
	if !strings.Contains(audit, "/admin/friends/import") {
		t.Fatalf("audit query returned nothing useful: %s", audit)
	}
	leaks := func(text string) []string {
		var out []string
		for _, m := range markers {
			if strings.Contains(text, m) {
				out = append(out, m)
			}
		}
		return out
	}
	if l := leaks(logs.String()); len(l) > 0 {
		t.Errorf("server log leaks %v", l)
	}
	if l := leaks(audit); len(l) > 0 {
		t.Errorf("admin_audit leaks %v", l)
	}
	// 变异测试：扫描本身能发现特征串（防止扫描逻辑失效后用例永远通过）。
	logs.Write([]byte("planted " + markers[2] + "\n"))
	if l := leaks(logs.String()); !slices.Contains(l, markers[2]) {
		t.Fatal("scanner failed to find a planted marker")
	}
}

// AC-SOC-09-9（BE-119 第 4 条 BenchmarkSearchTiming）：不可被搜索的 bob 与不存在的 nobody 各搜 1000 次（关闭限流），
// 两组耗时中位数之差不超过较小者的 20%。两组交替进行，抵消机器负载的漂移。
func TestTCSOC0910SearchTiming(t *testing.T) {
	if testing.Short() {
		t.Skip("2000 searches")
	}
	s := newSocEnv(t)
	s.usersNamed("alice", "bob")
	bob := s.dial("bob", "web")
	mustAck[protocol.VerAck](t, bob.call(protocol.CmdSetPrivacy, map[string]any{"searchable": false}))
	alice := s.dial("alice", "web")
	var hidden, missing []time.Duration
	for i := 0; i < 2000; i++ {
		if i%9 == 0 {
			s.resetLimits("alice")
		}
		kw := "bob"
		if i%2 == 1 {
			kw = "nobody"
		}
		start := time.Now()
		f := alice.call(protocol.CmdUserSearch, map[string]any{"keyword": kw})
		d := time.Since(start)
		if f.Error != nil {
			t.Fatalf("search %d: %+v", i, f.Error)
		}
		if kw == "bob" {
			hidden = append(hidden, d)
		} else {
			missing = append(missing, d)
		}
	}
	med := func(ds []time.Duration) time.Duration { slices.Sort(ds); return ds[len(ds)/2] }
	h, m := med(hidden), med(missing)
	diff, small := h-m, min(h, m)
	if diff < 0 {
		diff = -diff
	}
	t.Logf("median: unsearchable %v, nobody %v", h, m)
	if float64(diff) > 0.2*float64(small) {
		t.Fatalf("median differs by %v (> 20%% of %v)", diff, small)
	}
}
