package e2e

// C1 后端验收（BE-422）：评审攻击序列和测试的对应表、日志特征串、指标和配置检查，开发机的性能记录（不算通过）。

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"tianting/server/internal/account"
	"tianting/server/internal/app"
	"tianting/server/internal/protocol"
)

// 评审（docs/review/RECHECK-chat-v0.4～v0.7-security.md）里的攻击序列 → 用例。谁、什么顺序、时钟怎么推写在各用例里。
// notReachable 里的只有 Postgres 下可达，SQLite 事务串行，跑不到，不算通过。
var attackSequences = map[string][]string{
	"RS4-01 改密拿新码再用新码找回，旧码还在":             {"TestTC_ACC_05_16_OldCodeWins", "TestTC_ACC_04_12_OldCodeRetention"},
	"RS4-02 被替换代的 access 不放行不转正":          {"TestTC_ACC_03_28_EvictedAccess", "TestTC_ACC_03_30_VictimKicked", "TestAccountSessionServerVectors"},
	"RS4-03 甲 共用 sid 改密清除攻击者":             {"TestTC_ACC_04_14_SharedSIDCleared"},
	"RS4-03 乙 响应丢失不算重放":                   {"TestTC_ACC_04_15_ResponseLost"},
	"RS4-03 丙 刷新和改密并发":                    {"TestTC_ACC_04_08_RefreshRacePassword"},
	"RS4-04 甲 乱序到达不误吊销":                   {"TestTC_ACC_03_31_OutOfOrder", "TestTC_ACC_03_21_ConcurrentRefresh"},
	"RS4-04 乙 窗口内软拒绝":                     {"TestTC_ACC_03_31_OutOfOrder", "TestTC_ACC_03_03_PendingEviction"},
	"RS4-05 只知道 sid":                      {"TestTC_ACC_03_32_SIDOnly", "TestTC_ACC_03_33_SIDNotLogged"},
	"RS4-06 甲 登录在途":                       {"TestHookLoginVsRecover"},
	"RS4-06 乙 改密在途":                       {"TestHookPasswordVsRecover"},
	"RS4-06 丙 同一个码并发":                     {"TestTC_ACC_04_19_ConcurrentSameCode"},
	"RS4-06 丁 撤销注销和在途 delete":             {"TestHookDeleteVsRecover", "TestFinishDeletionRespectsCancel"},
	"RS4-07 离线受害者看到截止日期":                  {"TestTC_ACC_03_34_OfflineVictim"},
	"RS4-08 每次比较 IP 前缀，旧代 access 拒绝":      {"TestTC_ACC_03_35_NewPrefix", "TestTC_ACC_03_30_VictimKicked"},
	"RS5-01 连续转正续不了窗口":                    {"TestTC_ACC_03_40_ConsecutivePromotion"},
	"RS5-02 甲 循环改密压制不了 recover":           {"TestTC_ACC_04_22_PasswordLoopCannotSuppress", "TestHookRecoverIgnoresVersion"},
	"RS5-02 乙 丙 锁定、单哈希、专用位":               {"TestHookLockedAndHashingRecover", "TestTC_ACC_05_03_MissLimited"},
	"RS5-03 甲乙丙丁 四条序列的 recoverBefore":     {"TestTC_ACC_04_24_RecoverBeforeSequences", "TestTC_ACC_04_24_DeleteSequence"},
	"RS5-04 密钥台账":                         {"TestTC_ACC_03_48_KeyLedger"},
	"RS5-05 代号不复用":                        {"TestTC_ACC_03_42_GenNotReused"},
	"RS6-01 改密先提交，recover 随后作废新代":         {"TestHookRecoverIgnoresVersion"},
	"RS6-02 持新码占专用位":                      {"TestTC_ACC_04_28_FairSlot"},
	"RS6-05 用满签发次数压不住重放判定":                {"TestTC_ACC_03_38_LimitOnlyOnIssueBranch"},
	"RS7-01 logout-others 和登录在 Argon2 之后": {"TestHookLogoutOthersVsRecover", "TestHookLoginVsRecover"},
	"RS7-02 上限只约束最新一代":                    {"TestTC_ACC_04_29_LimitOnlyLatest", "TestRecoverLimitSingleCode"},
}

// Postgres 下才可达，这里没有测（AC-ACC-04-29 未执行）。
var notReachable = []string{
	"RS6-01 recover 第一条语句之后，改密插进恢复码检查和版本递增之间",
	"RS7-01 recover 持锁时 logout-others 或登录已读旧版本、未提交",
}

// 攻击序列对应的测试函数都存在。
func TestAttackSequencesHaveTests(t *testing.T) {
	var src strings.Builder
	files, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		src.Write(b)
	}
	for seq, tests := range attackSequences {
		for _, name := range tests {
			if !regexp.MustCompile(`func ` + name + `\(`).MatchString(src.String()) {
				t.Errorf("%s: no test %s", seq, name)
			}
		}
	}
	if len(notReachable) == 0 {
		t.Error("list what is not executed")
	}
}

// TC-CHAT-90-04（AC-CHAT-90-5 的 im-server 部分）：注册、登录、刷新、改密、找回（含失败路径），日志搜不到密码、恢复码、邀请码、token、sid、设备名。
// 用 deploy/ops/scan-logs 扫。console-server 的日志和审计随 CS-200。
func TestTC_CHAT_90_04_NoSecretsInLogs(t *testing.T) {
	var logs *syncBuffer
	e := newAcctEnv(t, func(c *app.Config) { logs = captureLogs(c) })
	const invite, password, newPw, device = "ZXQIN-invite-code-1", "ZXQPW-pass-2026!", "ZXQPW-new-2026!", "ZXQDEV-device"
	e.newInvite(invite, 5, time.Now().Add(oneDay))
	base := map[string]any{"username": "alice_1", "password": password, "inviteCode": invite, "platform": testPlatform, "deviceName": device}
	code, b, _ := e.post("/v1/account/register", e.freshIP(), base)
	if code != http.StatusOK {
		t.Fatalf("%d %s", code, b)
	}
	var r authResp
	mustJSON(t, b, &r)
	e.ack(r.AccessToken, r.RecoveryCodeGen)
	// 失败路径。
	e.post("/v1/account/register", e.freshIP(), map[string]any{"username": "ZXQPW-as-username", "password": password, "inviteCode": "ZXQIN-wrong", "platform": testPlatform})
	e.login("alice_1", "ZXQPW-wrong-2026", e.freshIP())
	e.login("ZXQPW-in-user-box", password, e.freshIP())
	e.refresh("t1." + sidOf(r.RefreshToken) + ".1.bm9uY2U.AAAA")
	e.changePassword(r.AccessToken, "ZXQPW-wrong-2026", newPw)
	e.recover("alice_1", "ZXQRC-wrong-code", newPw, "")
	// 成功路径。
	l := e.mustLogin("alice_1", password, "ios")
	r2 := e.mustRefresh(r.RefreshToken)
	p := e.mustChangePassword(l.AccessToken, password, newPw)
	got := e.mustRecover("alice_1", r.RecoveryCode, "ZXQPW-third-2026!")
	markers := []string{password, newPw, "ZXQPW-third-2026!", "ZXQPW", invite, "ZXQIN", device, "ZXQDEV", r.RecoveryCode, p.RecoveryCode, got.RecoveryCode, "ZXQRC",
		r.AccessToken, r.RefreshToken, r2.AccessToken, r2.RefreshToken, l.AccessToken, sidOf(r.RefreshToken), sidOf(l.RefreshToken), sidOf(got.RefreshToken)}
	path := filepath.Join(t.TempDir(), "im.log")
	if err := os.WriteFile(path, []byte(logs.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	if logs.String() == "" {
		t.Fatal("no logs captured: the scan would prove nothing")
	}
	args := []string{"-I", "../../deploy/ops/scan-logs"}
	for _, m := range markers {
		args = append(args, "--marker", m)
	}
	out, err := exec.Command("python3", append(args, path)...).CombinedOutput()
	if err != nil {
		t.Errorf("scan-logs: %v\n%s", err, out)
	}
	if n := e.count(`SELECT COUNT(*) FROM accounts WHERE password_hash LIKE ?`, "%ZXQPW%"); n != 0 {
		t.Errorf("plaintext password in accounts: %d", n)
	}
}

// TC-CHAT-90-15（AC-CHAT-90-17）：账号指标都出现，配置检查有这些项，标签没有无界字段。
func TestTC_CHAT_90_15_MetricsAndChecks(t *testing.T) {
	e := openEnv(t, nil)
	clk := e.freeze()
	r := e.register("alice_1", nil)
	e.login("alice_1", "wrong-password-1", e.freshIP())
	r2 := e.mustRefresh(r.RefreshToken)
	e.mustRefresh(r.RefreshToken)
	e.mustRefresh(r2.RefreshToken)
	clk.advance(3 * time.Minute)
	e.wantRefreshErr("replay", r.RefreshToken, protocol.ReasonReplayDetected)
	m := e.metricsText()
	for _, name := range []string{`im_account_logins_total{result="invalid"}`, `im_registers_total{result="ok"}`, "im_refresh_reissue_total", "im_refresh_replay_total"} {
		if !strings.Contains(m, name) {
			t.Errorf("no metric %s", name)
		}
	}
	for line := range strings.Lines(m) {
		if strings.HasPrefix(line, "im_") && (strings.Contains(line, "alice_1") || strings.Contains(line, sidOf(r.RefreshToken))) {
			t.Errorf("unbounded label: %s", line)
		}
	}
	ids := []string{}
	for _, c := range e.app.DynamicChecks(t.Context()) {
		ids = append(ids, c.ID)
	}
	for _, want := range []string{"refresh_key_ledger", "invite_total"} {
		if !slices.Contains(ids, want) {
			t.Errorf("no check %s in %v", want, ids)
		}
	}
}

// 开发机的性能记录（AC-CHAT-90-1、90-11）：只记录，**不算通过**，发布判定等 P-12。IM_PERF_DEV=1 才跑，生产 Argon2 参数。
func TestPerfDevRecord(t *testing.T) {
	if os.Getenv("IM_PERF_DEV") != "1" {
		t.Skip("set IM_PERF_DEV=1: development machine record, not a pass")
	}
	e := openEnv(t, func(c *app.Config) {
		c.Account.Argon = account.DefaultArgon
		c.Account.TestDisableLock = true
		c.SendRate = 100000
	})
	e.register("alice_1", nil)
	measure := func(label string, workers, total int) {
		var mu sync.Mutex
		var all []time.Duration
		var wg sync.WaitGroup
		for w := range workers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range total / workers {
					start := time.Now()
					code, _, _ := e.post("/v1/account/login", fmt.Sprintf("198.51.%d.1", w+1), map[string]any{"username": "alice_1", "password": goodPassword, "platform": testPlatform})
					if code != http.StatusOK {
						t.Errorf("login: %d", code)
					}
					mu.Lock()
					all = append(all, time.Since(start))
					mu.Unlock()
				}
			}()
		}
		wg.Wait()
		slices.Sort(all)
		t.Logf("PERF %s n=%d p50=%v p99=%v", label, len(all), all[len(all)/2], all[len(all)*99/100])
	}
	measure("login serial", 1, 200)
	measure("login 4 workers", 4, 200)
	l := e.mustLogin("alice_1", goodPassword, "ios")
	e.users("bob_peer")
	c := e.authOK("alice", l.AccessToken, "ios")
	var acks []time.Duration
	for i := range 1000 {
		start := time.Now()
		if f := c.sendText("c2c", "bob_peer", fmt.Sprintf("perf-%06d", i), "hi"); f.Error != nil {
			t.Fatalf("%+v", f.Error)
		}
		acks = append(acks, time.Since(start))
	}
	slices.Sort(acks)
	t.Logf("PERF c2c text with account token n=%d p50=%v p99=%v (sequential, not 500/s)", len(acks), acks[len(acks)/2], acks[len(acks)*99/100])
}
