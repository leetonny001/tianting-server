package e2e

// PRD-social F-SOC-01～05（BE-102、BE-106、BE-107、BE-109）：配置、好友申请、处理申请、红点、删除与备注。

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"tianting/server/internal/app"
	"tianting/server/internal/protocol"
)

// usersNamed 注册用户，昵称与 userId 相同（PRD-social 第 6 节约定）。
func (e *env) usersNamed(ids ...string) {
	e.t.Helper()
	list := make([]map[string]any, len(ids))
	for i, id := range ids {
		list[i] = map[string]any{"userId": id, "nickname": id}
	}
	if code, body := e.adminPost("/admin/users/import", map[string]any{"users": list}, adminKey); code != http.StatusOK {
		e.t.Fatalf("import users: %d %s", code, body)
	}
}

type lockedBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuf) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuf) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// newSocEnvWithLog 把服务端日志（Debug 级别）写进 buf，用于日志扫描用例。
func newSocEnvWithLog(t *testing.T) (*socEnv, *lockedBuf) {
	buf := &lockedBuf{}
	return newSocEnvCfg(t, func(c *app.Config) {
		c.Logger = slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}), buf
}

// ---- F-SOC-01 配置 ----

// TC-SOC-01-01（AC-SOC-01-1）
func TestTCSOC0101Defaults(t *testing.T) {
	s := newSocEnv(t)
	var cfg map[string]any
	json.Unmarshal(s.mustAdm(http.MethodGet, "/admin/settings/social", nil), &cfg)
	want := map[string]any{"mode": "open", "strangerMsgLimit": 0.0, "newPeerHourlyLimit": 0.0, "newPeerRejectDailyLimit": 100.0,
		"maxFriends": 2000.0, "maxBlocks": 1000.0, "requestExpireDays": 7.0, "applyDailyLimit": 30.0, "applyPerTargetDailyLimit": 3.0,
		"blockDailyLimit": 100.0, "searchableDefault": true, "searchDailyLimit": 100.0, "clientGroupEnabled": true,
		"clientGroupMaxMembers": 500.0, "maxJoinedGroups": 500.0, "groupCreateDailyLimit": 10.0}
	for k, v := range want {
		if cfg[k] != v {
			t.Errorf("%s = %v, want %v", k, cfg[k], v)
		}
	}
	if len(cfg) != len(want) {
		t.Errorf("settings has %d fields, want %d: %v", len(cfg), len(want), cfg)
	}
}

// TC-SOC-01-03（AC-SOC-01-3）
func TestTCSOC0103InvalidSettings(t *testing.T) {
	s := newSocEnv(t)
	for _, p := range []map[string]any{{"mode": "friends", "maxFriends": 99}, {"strangerMsgLimit": -1}, {"foo": 1}, {"mode": "x"}, {"mode": nil}} {
		code, body := s.adm(http.MethodPatch, "/admin/settings/social", p)
		assertAdminErr(t, code, body, http.StatusBadRequest, protocol.CodeBadRequest)
	}
	var cfg map[string]any
	json.Unmarshal(s.mustAdm(http.MethodGet, "/admin/settings/social", nil), &cfg)
	if cfg["mode"] != "open" || cfg["maxFriends"] != 2000.0 {
		t.Fatalf("invalid patch took effect: %v", cfg)
	}
}

// TC-SOC-01-02、01-04（AC-SOC-01-2、01-4、08-13 的模式切换部分）
func TestTCSOC0102ModeSwitch(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob")
	alice := s.dial("alice", "web")
	for i := range 5 {
		mustAck[protocol.SendAck](t, alice.sendText("c2c", "bob", "m-mode-0"+string(rune('0'+i)), "hi"))
	}
	s.settings(map[string]any{"mode": "friends"})
	mustErr(t, alice.sendText("c2c", "bob", "m-mode-10", "hi"), protocol.CodeNotFriend)
	convs := mustAck[protocol.ConvsAck](t, alice.call(protocol.CmdConvs, map[string]any{}))
	if len(convs.Convs) != 1 || convs.Convs[0].ConvID != "c2c:alice:bob" {
		t.Fatalf("history conversation should stay: %+v", convs.Convs)
	}
	sync := mustAck[protocol.SyncAck](t, alice.call(protocol.CmdSync, map[string]any{"convId": "c2c:alice:bob", "afterSeq": 0}))
	if len(sync.Messages) != 5 {
		t.Fatalf("sync after mode switch: %d messages", len(sync.Messages))
	}
	s.settings(map[string]any{"mode": "open"})
	mustAck[protocol.SendAck](t, alice.sendText("c2c", "bob", "m-mode-11", "hi"))
}

// TC-SOC-01-05（F-SOC-01 规则 5）：待处理申请跨模式切换仍有效。
func TestTCSOC0105PendingSurvivesModeSwitch(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob")
	alice, bob := s.dial("alice", "web"), s.dial("bob", "web")
	alice.apply("bob", nil)
	s.settings(map[string]any{"mode": "friends"})
	mustAck[socialAck](t, bob.respond("alice", "accept", nil))
	mustAck[protocol.SendAck](t, alice.sendText("c2c", "bob", "m-pend-01", "hi"))
}

// TC-SOC-01-08（AC-SOC-01-6 服务端部分）
func TestTCSOC0108SettingsChangedEvent(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice")
	web, pc := s.dial("alice", "web"), s.dial("alice", "pc")
	s.settings(map[string]any{"strangerMsgLimit": 3})
	for _, c := range []*client{web, pc} {
		var ev protocol.SocialSettingsChangedEvent
		json.Unmarshal(c.waitEvent(protocol.EventSocialSettingsChanged), &ev)
		if ev.Mode != "open" || ev.StrangerMsgLimit != 3 {
			t.Fatalf("%s: event %+v", c.uid, ev)
		}
	}
	// TC-SOC-01-09：同步应答头带当前值。
	_, last := web.state()
	if last.Mode != "open" || last.StrangerMsgLimit != 3 {
		t.Fatalf("sync header: mode=%s limit=%d", last.Mode, last.StrangerMsgLimit)
	}
}

// TC-SOC-01-10（AC-SOC-01-7）：累计口径。
func TestTCSOC0110StrangerCumulative(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob")
	alice := s.dial("alice", "web")
	for i := range 5 {
		mustAck[protocol.SendAck](t, alice.sendText("c2c", "bob", "m-cum-0"+string(rune('0'+i)), "hi"))
	}
	s.settings(map[string]any{"strangerMsgLimit": 3})
	mustErr(t, alice.sendText("c2c", "bob", "m-cum-10", "hi"), protocol.CodeStrangerMsgLimit)
}

// TC-SOC-01-11（AC-SOC-01-8）
func TestTCSOC0111LowerMaxFriends(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "carol")
	s.seedFriends("alice", "f", 150)
	s.settings(map[string]any{"maxFriends": 100})
	alice, carol := s.dial("alice", "web"), s.dial("carol", "web")
	carol.apply("alice", nil)
	mustErr(t, alice.respond("carol", "accept", nil), protocol.CodeFriendLimitExceeded)
	for i := range 51 {
		if i%15 == 0 {
			s.resetLimits("alice")
		}
		mustAck[socialAck](t, alice.call(protocol.CmdFriendDelete, map[string]any{"userId": "f" + pad4(i)}))
	}
	mustAck[socialAck](t, alice.respond("carol", "accept", nil))
	all, _ := alice.state()
	if len(all.Friends) != 100 {
		t.Fatalf("friends = %d, want 100", len(all.Friends))
	}
}

// TC-SOC-01-12（AC-SOC-01-9）
func TestTCSOC0112StartupDefaultsOnlyOnce(t *testing.T) {
	db := filepath.Join(t.TempDir(), "im.db")
	start := func() *socEnv {
		return newSocEnvCfg(t, func(c *app.Config) { c.DBPath, c.SocialDefaults = db, `{"mode":"friends"}` })
	}
	s := start()
	var cfg map[string]any
	json.Unmarshal(s.mustAdm(http.MethodGet, "/admin/settings/social", nil), &cfg)
	if cfg["mode"] != "friends" {
		t.Fatalf("mode = %v, want friends", cfg["mode"])
	}
	s.settings(map[string]any{"mode": "open"})
	s2 := start()
	json.Unmarshal(s2.mustAdm(http.MethodGet, "/admin/settings/social", nil), &cfg)
	if cfg["mode"] != "open" {
		t.Fatalf("restart overwrote settings: mode = %v", cfg["mode"])
	}
}

// ---- F-SOC-02 好友申请 ----

// TC-SOC-02-01（AC-SOC-02-1）
func TestTCSOC0201Apply(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob")
	alice := s.dial("alice", "web")
	b1, b2 := s.dial("bob", "web"), s.dial("bob", "pc")
	ack := alice.apply("bob", map[string]any{"message": "我是 alice", "source": "search"})
	if ack.Status != "pending" || !ack.Changed || len(ack.Items.Requests) != 1 || ack.Items.Requests[0].Profile != nil {
		t.Fatalf("ack = %+v", ack)
	}
	b1.waitSocialChanged()
	b2.waitSocialChanged()
	all, _ := b1.state()
	r, ok := findReq(all.Requests, "alice", "in")
	if !ok || r.Message != "我是 alice" || r.Source != "search" || r.Status != "pending" || r.Profile == nil || r.Profile.Nickname != "alice" {
		t.Fatalf("bob's request = %+v (ok=%v)", r, ok)
	}
	alice.expectNoFrame(noFrameWindow) // 发起连接不收 socialChanged
}

// TC-SOC-02-03（AC-SOC-02-2）
func TestTCSOC0203MergeAndPerTargetQuota(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob")
	alice, bob := s.dial("alice", "web"), s.dial("bob", "web")
	for _, m := range []string{"a", "b", "c"} {
		alice.apply("bob", map[string]any{"message": m})
		s.advance(time.Minute)
	}
	mustErr(t, alice.call(protocol.CmdFriendApply, map[string]any{"to": "bob", "message": "d"}), protocol.CodeQuotaExceeded)
	all, _ := bob.state()
	if len(all.Requests) != 1 || all.Requests[0].Message != "c" || all.Requests[0].ApplyCount != 3 {
		t.Fatalf("bob's requests = %+v", all.Requests)
	}
}

// TC-SOC-02-04（F-SOC-02 规则 3）：被拒绝后再申请复用同一对申请行，状态回到 pending。
func TestTCSOC0204ReapplyAfterReject(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob")
	alice, bob := s.dial("alice", "web"), s.dial("bob", "web")
	alice.apply("bob", nil)
	mustAck[socialAck](t, bob.respond("alice", "reject", nil))
	alice.apply("bob", nil)
	all, _ := bob.state()
	if len(all.Requests) != 1 || all.Requests[0].Status != "pending" || all.Requests[0].ApplyCount != 2 {
		t.Fatalf("bob's requests = %+v", all.Requests)
	}
}

// TC-SOC-02-05（AC-SOC-02-3）
func TestTCSOC0205MutualApplyAutoAccept(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob")
	alice, bob := s.dial("alice", "web"), s.dial("bob", "web")
	alice.apply("bob", nil)
	av, bv := alice.ver(), bob.ver()
	ack := bob.apply("alice", nil)
	if ack.Status != "accepted" || ack.Ver != bv+1 {
		t.Fatalf("bob's ack = %+v (bob ver was %d)", ack, bv)
	}
	if got := alice.ver(); got != av+1 {
		t.Fatalf("alice ver = %d, want %d", got, av+1)
	}
	for _, c := range []*client{alice, bob} {
		all, _ := c.state()
		peer := map[string]string{"alice": "bob", "bob": "alice"}[c.uid]
		if _, ok := findFriend(all.Friends, peer); !ok {
			t.Fatalf("%s: no friend %s", c.uid, peer)
		}
		for _, r := range all.Requests {
			if r.Status != "accepted" {
				t.Fatalf("%s: request %+v not accepted", c.uid, r)
			}
		}
	}
	bobAll, _ := bob.state()
	if _, ok := findReq(bobAll.Requests, "alice", "out"); ok {
		t.Fatal("bob should not have an out request to alice")
	}
}

// TC-SOC-02-06：已互为好友时再申请 accepted、不改数据。
func TestTCSOC0206AlreadyFriends(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob")
	s.importFriends([2]string{"alice", "bob"})
	alice := s.dial("alice", "web")
	v := alice.ver()
	ack := alice.apply("bob", nil)
	if ack.Status != "accepted" || ack.Changed || ack.Ver != v {
		t.Fatalf("ack = %+v", ack)
	}
}

// TC-SOC-02-07（AC-SOC-02-4）、TC-SOC-02-09（AC-SOC-02-6）：被拉黑 / 不存在与正常申请同构。
func TestTCSOC0207ApplyIndistinguishable(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob", "carol")
	alice, bob := s.dial("alice", "web"), s.dial("bob", "web")
	mustAck[socialAck](t, bob.call(protocol.CmdBlock, map[string]any{"userId": "alice"}))
	bv := bob.ver()
	toBob := alice.call(protocol.CmdFriendApply, map[string]any{"to": "bob", "message": "hi"})
	toCarol := alice.call(protocol.CmdFriendApply, map[string]any{"to": "carol", "message": "hi"})
	toNobody := alice.call(protocol.CmdFriendApply, map[string]any{"to": "nobody", "message": "hi"})
	want := stripVolatile(t, toCarol.Data)
	for name, f := range map[string]frame{"blocked": toBob, "nobody": toNobody} {
		if got := stripVolatile(t, f.Data); got != want {
			t.Errorf("%s ack differs:\n got %s\nwant %s", name, got, want)
		}
	}
	bob.expectNoFrame(noFrameWindow)
	if bob.ver() != bv {
		t.Fatal("bob's ver changed")
	}
	all, _ := bob.state()
	if _, ok := findReq(all.Requests, "alice", "in"); ok {
		t.Fatal("bob should not see alice's request")
	}
	aAll, _ := alice.state()
	if r, ok := findReq(aAll.Requests, "nobody", "out"); !ok || r.Profile != nil {
		t.Fatalf("alice's out request to nobody = %+v (ok=%v)", r, ok)
	}
	s.usersNamed("nobody")
	nb := s.dial("nobody", "web")
	nAll, _ := nb.state()
	if len(nAll.Requests) != 0 {
		t.Fatalf("nobody sees requests after import: %+v", nAll.Requests)
	}
}

// TC-SOC-02-08（AC-SOC-02-5）
func TestTCSOC0208ApplyFormat(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob")
	alice := s.dial("alice", "web")
	mustErr(t, alice.call(protocol.CmdFriendApply, map[string]any{"to": "alice"}), protocol.CodeBadRequest)
	mustErr(t, alice.call(protocol.CmdFriendApply, map[string]any{"to": "bob", "message": strings.Repeat("字", 51)}), protocol.CodeBadRequest)
	mustErr(t, alice.call(protocol.CmdFriendApply, map[string]any{"to": "a b"}), protocol.CodeInvalidID)
	mustErr(t, alice.call(protocol.CmdFriendApply, map[string]any{"to": "bob", "x": 1}), protocol.CodeBadRequest)
	mustErr(t, alice.call(protocol.CmdFriendApply, map[string]any{"to": "bob", "source": "group"}), protocol.CodeBadRequest)
	// 50 个码点（去首尾空白后）可以
	alice.apply("bob", map[string]any{"message": "  " + strings.Repeat("字", 50) + "  "})
}

// TC-SOC-02-10（AC-SOC-02-7）
func TestTCSOC0210FriendLimit(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob", "carol")
	s.seedFriends("alice", "f", 2000)
	alice := s.dial("alice", "web")
	mustErr(t, alice.call(protocol.CmdFriendApply, map[string]any{"to": "bob"}), protocol.CodeFriendLimitExceeded)
	mustErr(t, alice.call(protocol.CmdFriendApply, map[string]any{"to": "nobody"}), protocol.CodeFriendLimitExceeded)

	s.seedFriends("bob", "g", 2000)
	carol, bob := s.dial("carol", "web"), s.dial("bob", "web")
	if ack := carol.apply("bob", nil); ack.Status != "pending" {
		t.Fatalf("carol → bob: %+v", ack)
	}
	mustErr(t, bob.respond("carol", "accept", nil), protocol.CodeFriendLimitExceeded)
	// bob 满了时 carol 申请 → bob 的 in 行未过期，bob 申请 carol（自动通过）对 bob 是 4132
	mustErr(t, bob.call(protocol.CmdFriendApply, map[string]any{"to": "carol"}), protocol.CodeFriendLimitExceeded)
	all, _ := bob.state()
	if r, ok := findReq(all.Requests, "carol", "in"); !ok || r.Status != "pending" {
		t.Fatalf("carol's request should stay pending: %+v", r)
	}
}

// TC-SOC-02-10 后半（AC-SOC-02-7）：对方好友已满时，自动成为好友 / 同意都返回 4133，不改数据。
func TestTCSOC0210PeerFullOnAutoAccept(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob")
	alice, bob := s.dial("alice", "web"), s.dial("bob", "web")
	bob.apply("alice", nil)
	s.seedFriends("bob", "g", 2000)
	mustErr(t, alice.call(protocol.CmdFriendApply, map[string]any{"to": "bob"}), protocol.CodePeerFriendLimitExceeded)
	mustErr(t, alice.respond("bob", "accept", nil), protocol.CodePeerFriendLimitExceeded)
	all, _ := alice.state()
	if r, _ := findReq(all.Requests, "bob", "in"); r.Status != "pending" || len(all.Friends) != 0 {
		t.Fatalf("data changed: %+v", all)
	}
}

// TC-SOC-02-11（AC-SOC-02-8）
func TestTCSOC0211GroupSource(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob", "carol")
	s.createGroup("g1", "alice", "bob")
	alice := s.dial("alice", "web")
	alice.apply("bob", map[string]any{"source": "group", "sourceRef": "g1"})
	alice.apply("carol", map[string]any{"source": "group", "sourceRef": "g1"})
	for peer, want := range map[string]string{"bob": "group", "carol": "other"} {
		c := s.dial(peer, "web")
		all, _ := c.state()
		if r, _ := findReq(all.Requests, "alice", "in"); r.Source != want {
			t.Errorf("%s side source = %q, want %q", peer, r.Source, want)
		}
	}
	all, _ := alice.state()
	for _, r := range all.Requests {
		if r.Source != "group" {
			t.Errorf("alice side source for %s = %q, want group", r.UserID, r.Source)
		}
	}
}

// TC-SOC-02-12（AC-SOC-02-9）
func TestTCSOC0212ReapplyAfterOneWayDelete(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob", "carol")
	s.importFriends([2]string{"alice", "bob"}, [2]string{"alice", "carol"})
	alice, bob, carol := s.dial("alice", "web"), s.dial("bob", "web"), s.dial("carol", "web")
	mustAck[socialAck](t, bob.call(protocol.CmdFriendDelete, map[string]any{"userId": "alice"}))
	if ack := alice.apply("bob", nil); ack.Status != "pending" {
		t.Fatalf("alice → bob after bob deleted alice: %+v", ack)
	}
	bob.waitSocialChanged()

	mustAck[socialAck](t, alice.call(protocol.CmdFriendDelete, map[string]any{"userId": "carol"}))
	cv := carol.ver()
	if ack := alice.apply("carol", nil); ack.Status != "accepted" {
		t.Fatalf("alice → carol after alice deleted carol: %+v", ack)
	}
	all, _ := alice.state()
	if _, ok := findFriend(all.Friends, "carol"); !ok {
		t.Fatal("alice's friend list should have carol again")
	}
	carol.expectNoFrame(noFrameWindow)
	if carol.ver() != cv {
		t.Fatal("carol's ver changed")
	}
}

// TC-SOC-02-13（AC-SOC-02-10）
func TestTCSOC0213ApplyRateAndDailyQuota(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice")
	alice := s.dial("alice", "web")
	ok := 0
	for i := range 6 {
		f := alice.call(protocol.CmdFriendApply, map[string]any{"to": "p" + pad4(i)})
		if i < 5 {
			mustAck[socialAck](t, f)
			ok++
		} else {
			mustErr(t, f, protocol.CodeRateLimited)
		}
	}
	for i := 6; ok < 30; i++ {
		s.advance(time.Second)
		mustAck[socialAck](t, alice.call(protocol.CmdFriendApply, map[string]any{"to": "p" + pad4(i)}))
		ok++
	}
	s.advance(time.Second)
	mustErr(t, alice.call(protocol.CmdFriendApply, map[string]any{"to": "p9999"}), protocol.CodeQuotaExceeded)
}

// TC-SOC-02-14（AC-SOC-02-11）
func TestTCSOC0214LogHasNoApplyText(t *testing.T) {
	s, logs := newSocEnvWithLog(t)
	s.usersNamed("alice", "bob")
	alice := s.dial("alice", "web")
	alice.apply("bob", map[string]any{"message": "ZXQ-SOCIAL-123", "remark": "ZXQ-REMARK-456"})
	alice.call(protocol.CmdFriendApply, map[string]any{"to": "bob", "message": strings.Repeat("ZXQ-SOCIAL-123", 5)}) // 4000，msg 不拼入输入
	for _, needle := range []string{"ZXQ-SOCIAL-123", "ZXQ-REMARK-456"} {
		if strings.Contains(logs.String(), needle) {
			t.Fatalf("server log contains %s", needle)
		}
	}
	if !strings.Contains(logs.String(), "friend apply") {
		t.Fatal("expected a debug log line for the apply (length only)")
	}
}

// TC-SOC-02-15（AC-SOC-02-12）
func TestTCSOC0215ApplyToBlocked(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob")
	alice := s.dial("alice", "web")
	mustAck[socialAck](t, alice.call(protocol.CmdBlock, map[string]any{"userId": "bob"}))
	mustErr(t, alice.call(protocol.CmdFriendApply, map[string]any{"to": "bob"}), protocol.CodeBadRequest)
	mustAck[socialAck](t, alice.call(protocol.CmdUnblock, map[string]any{"userId": "bob"}))
	if ack := alice.apply("bob", nil); ack.Status != "pending" {
		t.Fatalf("after unblock: %+v", ack)
	}
}

// ---- F-SOC-03 处理申请 ----

// TC-SOC-03-01（AC-SOC-03-1）
func TestTCSOC0301Accept(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob")
	a1, a2 := s.dial("alice", "web"), s.dial("alice", "pc")
	b1, b2 := s.dial("bob", "web"), s.dial("bob", "pc")
	a1.apply("bob", nil)
	b1.waitSocialChanged()
	b2.waitSocialChanged()
	a2.waitSocialChanged()
	mustAck[socialAck](t, b1.respond("alice", "accept", map[string]any{"remark": "同学A"}))
	a1.waitSocialChanged()
	a2.waitSocialChanged()
	b2.waitSocialChanged()
	b1.expectNoFrame(noFrameWindow)
	bAll, _ := b1.state()
	if f, ok := findFriend(bAll.Friends, "alice"); !ok || f.Remark != "同学A" {
		t.Fatalf("bob's friend alice = %+v", f)
	}
	aAll, _ := a1.state()
	if _, ok := findFriend(aAll.Friends, "bob"); !ok {
		t.Fatal("alice has no friend bob")
	}
}

// TC-SOC-03-03（F-SOC-03 规则 7）、TC-SOC-03-04（AC-SOC-03-2）
func TestTCSOC0303AcceptWhileBlockingAndPresetRemark(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob")
	alice, bob := s.dial("alice", "web"), s.dial("bob", "web")
	alice.apply("bob", map[string]any{"remark": "B哥"})
	mustAck[socialAck](t, bob.call(protocol.CmdBlock, map[string]any{"userId": "alice"})) // in 行被置为 rejected
	mustAck[socialAck](t, bob.respond("alice", "accept", nil))
	bAll, _ := bob.state()
	if len(bAll.Blocks) != 1 {
		t.Fatalf("bob's block list changed: %+v", bAll.Blocks)
	}
	aAll, _ := alice.state()
	if f, ok := findFriend(aAll.Friends, "bob"); !ok || f.Remark != "B哥" {
		t.Fatalf("alice's friend bob = %+v", f)
	}
}

// TC-SOC-03-05（AC-SOC-03-3）
func TestTCSOC0305RejectSilent(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob")
	alice, bob := s.dial("alice", "web"), s.dial("bob", "web")
	alice.apply("bob", nil)
	before, _ := alice.state()
	av := alice.ver()
	mustAck[socialAck](t, bob.respond("alice", "reject", nil))
	alice.expectNoFrame(noFrameWindow)
	if alice.ver() != av {
		t.Fatal("alice's ver changed")
	}
	after, _ := alice.state()
	r, _ := findReq(after.Requests, "bob", "out")
	r0, _ := findReq(before.Requests, "bob", "out")
	if r.Status != "pending" || r.ExpireAt != r0.ExpireAt {
		t.Fatalf("alice's out request = %+v, before %+v", r, r0)
	}
	bAll, _ := bob.state()
	if r, _ := findReq(bAll.Requests, "alice", "in"); r.Status != "rejected" {
		t.Fatalf("bob's in request = %+v", r)
	}
}

// TC-SOC-03-06（AC-SOC-03-4）
func TestTCSOC0306ExpiryBoundary(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob", "carol")
	alice, carol := s.dial("alice", "web"), s.dial("carol", "web")
	alice.apply("bob", nil)
	carol.apply("bob", nil)
	s.advance(7*24*time.Hour - time.Second)
	bob := s.dial("bob", "web")
	mustAck[socialAck](t, bob.respond("alice", "accept", nil))
	s.advance(2 * time.Second)
	mustErr(t, bob.respond("carol", "accept", nil), protocol.CodeFriendRequestNotFound)
	for _, c := range []*client{bob, s.dial("carol", "pc")} {
		all, _ := c.state()
		peer, dir := "carol", "in"
		if c.uid == "carol" {
			peer, dir = "bob", "out"
		}
		if r, _ := findReq(all.Requests, peer, dir); r.Status != "expired" {
			t.Errorf("%s sees %s request as %q, want expired", c.uid, dir, r.Status)
		}
	}
}

// TC-SOC-03-07（AC-SOC-03-5）
func TestTCSOC0307RejectThenAccept(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob")
	alice, bob := s.dial("alice", "web"), s.dial("bob", "web")
	alice.apply("bob", nil)
	mustAck[socialAck](t, bob.respond("alice", "reject", nil))
	if ack := mustAck[socialAck](t, bob.respond("alice", "reject", nil)); ack.Changed {
		t.Fatal("repeated reject should be changed=false")
	}
	mustAck[socialAck](t, bob.respond("alice", "accept", nil))
	mustErr(t, bob.respond("alice", "reject", nil), protocol.CodeFriendRequestHandled)
}

// TC-SOC-03-08（AC-SOC-03-6）
func TestTCSOC0308NotFoundIndistinguishable(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob", "carol", "dave")
	alice, dave := s.dial("alice", "web"), s.dial("dave", "web")
	alice.apply("bob", nil)
	bob := s.dial("bob", "web")
	mustAck[socialAck](t, bob.call(protocol.CmdBlock, map[string]any{"userId": "dave"}))
	dave.apply("bob", nil) // 申请时被拉黑：bob 没有 in 行
	s.advance(8 * 24 * time.Hour)
	bob2 := s.dial("bob", "pc")
	never := bob2.respond("carol", "accept", nil)
	expired := bob2.respond("alice", "accept", nil)
	blocked := bob2.respond("dave", "accept", nil)
	for name, f := range map[string]frame{"expired": expired, "blocked": blocked} {
		mustErr(t, f, protocol.CodeFriendRequestNotFound)
		if errBody(f) != errBody(never) {
			t.Errorf("%s error %s differs from never-applied %s", name, errBody(f), errBody(never))
		}
	}
}

// TC-SOC-03-09（AC-SOC-03-7）
func TestTCSOC0309AcceptIdempotent(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob")
	alice, bob := s.dial("alice", "web"), s.dial("bob", "web")
	alice.apply("bob", nil)
	av, bv := alice.ver(), bob.ver()
	first := mustAck[socialAck](t, bob.respond("alice", "accept", nil))
	second := mustAck[socialAck](t, bob.respond("alice", "accept", nil))
	if !first.Changed || second.Changed || second.Ver != first.Ver || len(second.Items.Friends) != 0 {
		t.Fatalf("first %+v second %+v", first, second)
	}
	if alice.ver() != av+1 || bob.ver() != bv+1 {
		t.Fatalf("vers: alice %d→%d bob %d→%d", av, alice.ver(), bv, bob.ver())
	}
	for _, c := range []*client{alice, bob} {
		all, _ := c.state()
		if len(all.Friends) != 1 {
			t.Fatalf("%s has %d friend edges", c.uid, len(all.Friends))
		}
	}
}

// ---- F-SOC-04 红点 ----

// TC-SOC-04-04（AC-SOC-04-4）
func TestTCSOC0404RequestSeenMonotonic(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("bob")
	for i := range 7 { // 每次造数据给 bob 分配一个 ver
		s.seed(map[string]any{"friends": []any{map[string]any{"owner": "bob", "peer": "x" + pad4(i)}}})
	}
	bob := s.dial("bob", "web")
	type seenAck = protocol.RequestsSeenAck
	a := mustAck[seenAck](t, bob.call(protocol.CmdFriendRequestsSeen, map[string]any{"ver": 5}))
	if a.RequestSeenVer != 5 || a.Ver != 8 || !a.Changed {
		t.Fatalf("seen 5: %+v", a)
	}
	a = mustAck[seenAck](t, bob.call(protocol.CmdFriendRequestsSeen, map[string]any{"ver": 3}))
	if a.RequestSeenVer != 5 || a.Ver != 8 || a.Changed {
		t.Fatalf("seen 3: %+v", a)
	}
	a = mustAck[seenAck](t, bob.call(protocol.CmdFriendRequestsSeen, map[string]any{"ver": 100}))
	if a.RequestSeenVer != 8 || a.Ver != 9 {
		t.Fatalf("seen 100: %+v", a)
	}
}

// TC-SOC-04-02（AC-SOC-04-2 服务端部分）、TC-SOC-04-03（AC-SOC-04-3）
func TestTCSOC0403SeenSyncAndRemerge(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("bob", "carol", "dave")
	carol, dave := s.dial("carol", "web"), s.dial("dave", "web")
	carol.apply("bob", nil)
	dave.apply("bob", nil)
	web, pc := s.dial("bob", "web"), s.dial("bob", "pc")
	all, last := web.state()
	if len(all.Requests) != 2 || all.Requests[0].UserID != "carol" || all.Requests[1].UserID != "dave" {
		t.Fatalf("requests in ver order: %+v", all.Requests)
	}
	mustAck[protocol.RequestsSeenAck](t, web.call(protocol.CmdFriendRequestsSeen, map[string]any{"ver": last.LatestVer}))
	pc.waitSocialChanged()
	_, pcLast := pc.state()
	if unread(pcLast, all.Requests) != 0 {
		t.Fatal("pc should see 0 unread after web marked seen")
	}
	carol.apply("bob", nil) // 合并：in 行得到新 ver，重新计入红点
	all, last = web.state()
	if n := unread(last, all.Requests); n != 1 {
		t.Fatalf("unread after re-apply = %d, want 1", n)
	}
}

func unread(head protocol.SocialSyncAck, rs []protocol.RequestItem) int {
	n := 0
	for _, r := range rs {
		if r.Direction == "in" && r.Status == "pending" && r.Ver > head.RequestSeenVer {
			n++
		}
	}
	return n
}

// TC-SOC-04-07（AC-SOC-04-6）、TC-SOC-04-08（AC-SOC-04-7）
func TestTCSOC0407ThirtyDayFilterAndPurge(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("bob", "carol", "dave")
	s.seed(map[string]any{"requests": []any{map[string]any{"from": "carol", "to": "bob"}}, "updatedAt": t0}) // bob ver 1
	s.seed(map[string]any{"requests": []any{map[string]any{"from": "dave", "to": "bob"}}, "updatedAt": t0})  // bob ver 2
	s.advance(30*24*time.Hour - time.Hour)
	bob := s.dial("bob", "web")
	all, _ := bob.state()
	if len(all.Requests) != 2 {
		t.Fatalf("29d23h: %d requests, want 2", len(all.Requests))
	}
	s.advance(time.Hour + time.Second)
	bob2 := s.dial("bob", "pc")
	all, _ = bob2.state()
	if len(all.Requests) != 0 {
		t.Fatalf("30d+1s without purge: %d requests, want 0", len(all.Requests))
	}
	// 不执行清理时增量轮仍能拿到这两行（清理与查询过滤互相独立）
	if p := mustAck[protocol.SocialSyncAck](t, bob2.call(protocol.CmdSocialSync, map[string]any{"sinceVer": 1})); p.Full || len(p.Requests) != 1 {
		t.Fatalf("incremental before purge: %+v", p)
	}
	s.purge()
	p := mustAck[protocol.SocialSyncAck](t, bob2.call(protocol.CmdSocialSync, map[string]any{"sinceVer": 1}))
	if !p.Full || len(p.Requests) != 0 {
		t.Fatalf("since=1 < purgedVer=2 should be a full round: %+v", p)
	}
	if p := mustAck[protocol.SocialSyncAck](t, bob2.call(protocol.CmdSocialSync, map[string]any{"sinceVer": 2})); p.Full {
		t.Fatalf("since=purgedVer should stay incremental: %+v", p)
	}
}

// ---- F-SOC-05 删除与备注 ----

// TC-SOC-05-01（AC-SOC-05-1）、TC-SOC-05-03（AC-SOC-05-2）
func TestTCSOC0501OneWayDelete(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob")
	s.importFriends([2]string{"alice", "bob"})
	alice, bob := s.dial("alice", "web"), s.dial("bob", "web")
	mustAck[protocol.SendAck](t, alice.sendText("c2c", "bob", "m-del-001", "hi"))
	bob.waitCmd(protocol.CmdMsg)
	bv := bob.ver()
	mustAck[socialAck](t, alice.call(protocol.CmdFriendDelete, map[string]any{"userId": "bob"}))
	bob.expectNoFrame(noFrameWindow)
	if bob.ver() != bv {
		t.Fatal("bob's ver changed")
	}
	aAll, _ := alice.state()
	bAll, _ := bob.state()
	if len(aAll.Friends) != 0 || len(bAll.Friends) != 1 {
		t.Fatalf("alice friends %v, bob friends %v", friendIDs(aAll.Friends), friendIDs(bAll.Friends))
	}
	s.settings(map[string]any{"mode": "friends"})
	mustErr(t, alice.sendText("c2c", "bob", "m-del-002", "hi"), protocol.CodeNotFriend)
	mustErr(t, bob.sendText("c2c", "alice", "m-del-003", "hi"), protocol.CodeNotFriend)
	sync := mustAck[protocol.SyncAck](t, bob.call(protocol.CmdSync, map[string]any{"convId": "c2c:alice:bob", "afterSeq": 0}))
	if len(sync.Messages) != 1 {
		t.Fatalf("history after delete: %d", len(sync.Messages))
	}
}

// TC-SOC-05-04（AC-SOC-05-3）
func TestTCSOC0504DeleteNonFriend(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "carol")
	alice := s.dial("alice", "web")
	v := alice.ver()
	for _, u := range []string{"carol", "nobody"} {
		if ack := mustAck[socialAck](t, alice.call(protocol.CmdFriendDelete, map[string]any{"userId": u})); ack.Changed || ack.Ver != v {
			t.Fatalf("delete %s: %+v", u, ack)
		}
	}
}

// TC-SOC-05-05（AC-SOC-05-4）
func TestTCSOC0505RemarkPrivate(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob")
	s.importFriends([2]string{"alice", "bob"})
	web, pc, bob := s.dial("alice", "web"), s.dial("alice", "pc"), s.dial("bob", "web")
	mustAck[protocol.SendAck](t, bob.sendText("c2c", "alice", "m-rmk-001", "hi"))
	mustAck[socialAck](t, web.call(protocol.CmdFriendRemark, map[string]any{"userId": "bob", "remark": "老板"}))
	pc.waitSocialChanged()
	all, _ := pc.state()
	if f, _ := findFriend(all.Friends, "bob"); f.Remark != "老板" {
		t.Fatalf("pc remark = %q", f.Remark)
	}
	var seen []string
	_, last := bob.state()
	b, _ := json.Marshal(last)
	seen = append(seen, string(b))
	seen = append(seen, string(bob.call(protocol.CmdUsers, map[string]any{"userIds": []string{"alice"}}).Data))
	seen = append(seen, string(bob.call(protocol.CmdUserSearch, map[string]any{"keyword": "alice"}).Data))
	seen = append(seen, string(bob.call(protocol.CmdConvs, map[string]any{}).Data))
	seen = append(seen, string(s.mustAdm(http.MethodGet, "/admin/users/alice/friends", nil)))
	for i, v := range seen {
		if strings.Contains(v, "老板") {
			t.Errorf("view %d leaks remark: %s", i, v)
		}
	}
	if !strings.Contains(seen[4], `"userId":"bob"`) || strings.Contains(seen[4], "remark") {
		t.Errorf("admin friend list: %s", seen[4])
	}
}

// TC-SOC-05-06（AC-SOC-05-5）、TC-SOC-05-07（AC-SOC-05-6）
func TestTCSOC0506RemarkBounds(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob", "carol")
	s.importFriends([2]string{"alice", "bob"})
	alice := s.dial("alice", "web")
	mustErr(t, alice.call(protocol.CmdFriendRemark, map[string]any{"userId": "bob", "remark": strings.Repeat("字", 33)}), protocol.CodeBadRequest)
	a := alice.call(protocol.CmdFriendRemark, map[string]any{"userId": "carol", "remark": "x"})
	b := alice.call(protocol.CmdFriendRemark, map[string]any{"userId": "nobody", "remark": "x"})
	mustErr(t, a, protocol.CodeNotFound)
	if errBody(a) != errBody(b) {
		t.Fatalf("non-friend %s vs nobody %s", errBody(a), errBody(b))
	}
	first := mustAck[socialAck](t, alice.call(protocol.CmdFriendRemark, map[string]any{"userId": "bob", "remark": "老板"}))
	second := mustAck[socialAck](t, alice.call(protocol.CmdFriendRemark, map[string]any{"userId": "bob", "remark": "老板"}))
	if !first.Changed || second.Changed || second.Ver != first.Ver {
		t.Fatalf("first %+v second %+v", first, second)
	}
	cleared := mustAck[socialAck](t, alice.call(protocol.CmdFriendRemark, map[string]any{"userId": "bob", "remark": "  "}))
	if !cleared.Changed || cleared.Items.Friends[0].Remark != "" {
		t.Fatalf("clear: %+v", cleared)
	}
}

