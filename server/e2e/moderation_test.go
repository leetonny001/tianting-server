package e2e

// PRD-admin 第二批（BE-220～BE-227）：封禁、删除消息、加密隔离、禁言、正文授权。

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"tianting/server/internal/adminauth"
	"tianting/server/internal/app"
	"tianting/server/internal/protocol"
)

const testQuarantineKey = "e2e-quarantine-key-0123456789-abcdef"

// modEnv：mod 是有全部管理 scope 的调用方，body 是唯一的正文调用方（console-msgread）。
type modEnv struct {
	*socEnv
	mod, body, biz testCaller
	dbPath         string
}

func newModEnv(t *testing.T, mutate func(*app.Config)) *modEnv {
	t.Helper()
	mod := hmacCaller("mod", adminauth.ScopeUserBan, adminauth.ScopeMsgDelete, adminauth.ScopeMuteWrite, adminauth.ScopeAuditRead, adminauth.ScopeUserRead, adminauth.ScopeGroupRead, adminauth.ScopeMsgMeta)
	biz := hmacCaller("biz", adminauth.ScopeMsgDelete)
	body := hmacCaller("console-msgread", adminauth.ScopeMsgBodyGroup, adminauth.ScopeMsgBodyC2C, adminauth.ScopeMsgQuarantine)
	body.cfg.BodyAccess = true
	var db string
	s := newSocEnvCfg(t, func(c *app.Config) {
		c.V13AdminFeatures = true
		c.QuarantineKey = testQuarantineKey
		c.AdminCallers = append(c.AdminCallers, mod.cfg, biz.cfg, body.cfg)
		if mutate != nil {
			mutate(c)
		}
		db = c.DBPath
	})
	return &modEnv{socEnv: s, mod: mod, body: body, biz: biz, dbPath: db}
}

func (m *modEnv) call(c testCaller, path string, body any) (int, []byte) {
	m.t.Helper()
	return m.signed(c, http.MethodPost, path, body, signOpts{ts: timeMs(m.now), operator: "op1", requestID: fmt.Sprintf("r-%d", reqSeq.Add(1))})
}

func (m *modEnv) must(path string, body any) []byte {
	m.t.Helper()
	code, b := m.call(m.mod, path, body)
	if code != http.StatusOK {
		m.t.Fatalf("%s: %d %s", path, code, b)
	}
	return b
}

func (m *modEnv) ban(uid string, seconds any) {
	m.must("/admin/users/"+uid+"/ban", map[string]any{"durationSeconds": seconds, "reason": "spam"})
}

func errDetails(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var e struct {
		Details map[string]any `json:"details"`
		Reason  string         `json:"reason"`
	}
	if err := json.Unmarshal(body, &e); err != nil {
		t.Fatalf("%v: %s", err, body)
	}
	return e.Details
}

// ---- 封禁 ----

// TC-ADM-13-01（AC-ADM-13-1）：两个在线连接收到 suspended、until，关闭码 4012。
func TestAdmBanKicksAllConns(t *testing.T) {
	m := newModEnv(t, nil)
	a, b := m.dial("u1", "web"), m.dial("u1", "ios")
	m.ban("u1", 86400)
	want := m.now + 86400_000
	for _, c := range []*client{a, b} {
		f := c.waitCmd(protocol.CmdEvent)
		var ev struct {
			Type, Reason string
			Until        *int64
		}
		json.Unmarshal(f.Data, &ev)
		if ev.Type != "kicked" || ev.Reason != "suspended" || ev.Until == nil || *ev.Until != want {
			t.Errorf("event = %s", f.Data)
		}
		if code := c.closeCode(waitTimeout); int(code) != protocol.CloseTokenRevoked {
			t.Errorf("close code %d", code)
		}
	}
}

// TC-ADM-13-02、13-03、13-07：封禁中签发 4034（永久为 null），旧 token 收到 kicked + 4012，演示登录 4034 且不改昵称。
func TestAdmBanBlocksIssue(t *testing.T) {
	m := newModEnv(t, func(c *app.Config) { c.DemoLogin = true })
	old := m.token("u1")
	m.ban("u1", 86400)

	code, body := m.adminPost("/admin/users/token", map[string]any{"userId": "u1"}, adminKey)
	expectCode(t, "issue", code, body, 403, protocol.CodeUserBanned)
	if got, _ := errDetails(t, body)["bannedUntil"].(float64); int64(got) != m.now+86400_000 {
		t.Errorf("bannedUntil = %s", body)
	}

	c := m.dialURL("old", m.wsURL(old, "web"))
	c.expectKicked(protocol.KickSuspended, protocol.CloseTokenRevoked, waitTimeout)

	code, body = m.publicPost("/demo/login", map[string]any{"username": "u1", "nickname": "changed"})
	expectCode(t, "demo", code, body, 403, protocol.CodeUserBanned)
	_, raw := m.call(m.mod, "/admin/users/u1/unban", map[string]any{"reason": "probe"})
	_ = raw
	_, u := m.signed(m.mod, http.MethodGet, "/admin/users/u1", nil, signOpts{ts: timeMs(m.now)})
	if bytes.Contains(u, []byte("changed")) {
		t.Errorf("nickname changed by a rejected demo login: %s", u)
	}

	m.ban("u1", nil) // 永久
	code, body = m.adminPost("/admin/users/token", map[string]any{"userId": "u1"}, adminKey)
	expectCode(t, "issue permanent", code, body, 403, protocol.CodeUserBanned)
	d := errDetails(t, body)
	if v, ok := d["bannedUntil"]; !ok || v != nil {
		t.Errorf("permanent bannedUntil = %s", body)
	}
}

// TC-ADM-13-04（AC-ADM-13-4）：到期自动解封，不需要定时任务。
func TestAdmBanExpires(t *testing.T) {
	m := newModEnv(t, nil)
	m.token("u1")
	m.ban("u1", 3600)
	m.advance(61 * time.Minute)
	c := m.dial("u1", "web")
	if got := mustAck[protocol.SendAck](t, c.sendText("c2c", "u1", "cmid-00c1", "hi")); got.MsgID == "" {
		t.Fatal("no msgId")
	}
	_, u := m.signed(m.mod, http.MethodGet, "/admin/users/u1", nil, signOpts{ts: timeMs(m.now)})
	if !bytes.Contains(u, []byte(`"ban":null`)) {
		t.Errorf("expired ban still shown: %s", u)
	}
}

// TC-ADM-13-05、13-06：解封立即可以签发；资料、好友、群、历史都不变；审计每次都有。
func TestAdmUnbanKeepsEverything(t *testing.T) {
	m := newModEnv(t, nil)
	m.users("u1", "u2")
	m.importFriends([2]string{"u1", "u2"})
	m.createGroup("g1", "u1", "u2")
	c := m.dial("u1", "web")
	mustAck[protocol.SendAck](t, c.sendText("group", "g1", "cmid-00c1", "before"))

	m.ban("u1", 3600)
	m.ban("u1", 7200) // 重复封禁以最后一次为准
	_, u := m.signed(m.mod, http.MethodGet, "/admin/users/u1", nil, signOpts{ts: timeMs(m.now)})
	if !bytes.Contains(u, []byte(fmt.Sprintf(`"until":%d`, m.now+7200_000))) || !bytes.Contains(u, []byte(`"by":"mod:op1"`)) || !bytes.Contains(u, []byte(`"reason":"spam"`)) {
		t.Errorf("user = %s", u)
	}
	_, list := m.signed(m.mod, http.MethodGet, "/admin/users?q=u1", nil, signOpts{ts: timeMs(m.now)})
	if !bytes.Contains(list, []byte(`"by":"mod:op1"`)) {
		t.Errorf("list has no ban: %s", list)
	}
	m.must("/admin/users/u1/unban", map[string]any{"reason": "appeal ok"})

	c2 := m.dial("u1", "web")
	sync := mustAck[protocol.SyncAck](t, c2.call(protocol.CmdSync, map[string]any{"convId": "group:g1", "afterSeq": 0, "limit": 10}))
	if len(sync.Messages) != 1 || sync.Messages[0].Body.Text != "before" {
		t.Errorf("history = %+v", sync.Messages)
	}
	if _, f := c2.state(); len(friendIDs(f.Friends)) != 1 {
		t.Errorf("friends = %+v", f.Friends)
	}
	entries := m.auditOf(m.mod)
	var bans, unbans int
	for _, e := range entries {
		if e.Phase == "result" && strings.HasSuffix(e.Route, "/ban") {
			bans++
			if !strings.Contains(string(e.Summary), "spam") {
				t.Errorf("ban audit has no reason: %s", e.Summary)
			}
		}
		if e.Phase == "result" && strings.HasSuffix(e.Route, "/unban") {
			unbans++
		}
	}
	if bans != 2 || unbans != 1 {
		t.Errorf("audit: %d bans, %d unbans", bans, unbans)
	}
}

// TC-ADM-13-08 及负向：不存在、漏传时长、时长越界、没有理由、旧密钥、没有 scope。
func TestAdmBanValidation(t *testing.T) {
	m := newModEnv(t, nil)
	m.token("u1")
	code, b := m.call(m.mod, "/admin/users/nobody/ban", map[string]any{"durationSeconds": 3600, "reason": "spam"})
	expectCode(t, "nobody", code, b, 404, protocol.CodeUserNotFound)
	code, b = m.call(m.mod, "/admin/users/u1/ban", map[string]any{"reason": "spam"})
	expectCode(t, "no duration", code, b, 400, protocol.CodeBadRequest)
	for _, secs := range []int{0, 59, 30*24*3600 + 1} {
		code, b = m.call(m.mod, "/admin/users/u1/ban", map[string]any{"durationSeconds": secs, "reason": "spam"})
		expectCode(t, fmt.Sprint("duration ", secs), code, b, 400, protocol.CodeBadRequest)
	}
	code, b = m.call(m.mod, "/admin/users/u1/ban", map[string]any{"durationSeconds": 3600, "reason": "  "})
	expectCode(t, "blank reason", code, b, 400, protocol.CodeBadRequest)
	code, b = m.adminPost("/admin/users/u1/ban", map[string]any{"durationSeconds": 3600, "reason": "spam"}, adminKey)
	expectCode(t, "legacy", code, b, 403, protocol.CodeForbidden)
	code, b = m.call(m.biz, "/admin/users/u1/ban", map[string]any{"durationSeconds": 3600, "reason": "spam"})
	expectCode(t, "no scope", code, b, 403, protocol.CodeForbidden)
}

// IM_V13_ADMIN_FEATURES 关闭时（默认）：第二批路由 404，不下发新字段。
func TestAdmFeatureFlagOff(t *testing.T) {
	s := newSocEnv(t)
	for _, p := range []string{"/admin/users/u1/ban", "/admin/users/u1/unban", "/admin/messages/delete", "/admin/mute", "/admin/unmute", "/admin/msg-grants", "/admin/convs/messages/body"} {
		if code, b := s.adm(http.MethodPost, p, map[string]any{}); code != 404 {
			t.Errorf("%s: %d %s", p, code, b)
		}
	}
	s.token("u1")
	_, u := s.adm(http.MethodGet, "/admin/users/u1", nil)
	if bytes.Contains(u, []byte(`"ban"`)) || bytes.Contains(u, []byte(`"mutedUntil"`)) {
		t.Errorf("flag off exposes new fields: %s", u)
	}
}

// ---- 删除消息 ----

func sendG(t *testing.T, c *client, gid, cid, text string) protocol.SendAck {
	t.Helper()
	return mustAck[protocol.SendAck](t, c.sendText("group", gid, cid, text))
}

// TC-ADM-31-01、31-02、31-05：4 个连接收到事件；离线成员 sync seq 连续、没有 body；最后一条摘要占位；未读不变。
func TestAdmDeleteMessage(t *testing.T) {
	m := newModEnv(t, nil)
	m.users("u2", "u3", "off")
	m.createGroup("g1", "u1", "u2", "u3", "off")
	u1a, u1b, u2, u3 := m.dial("u1", "web"), m.dial("u1", "ios"), m.dial("u2", "web"), m.dial("u3", "web")
	var acks []protocol.SendAck
	for i := range 6 {
		acks = append(acks, sendG(t, u1a, "g1", fmt.Sprintf("cmid-00%d", i), fmt.Sprintf("text%d", i)))
	}
	for _, c := range []*client{u1b, u2, u3} { // 收掉 msg 帧
		for range 6 {
			c.waitCmd(protocol.CmdMsg)
		}
	}
	target := acks[4] // seq 5
	m.must("/admin/messages/delete", map[string]any{"convId": "group:g1", "msgIds": []string{target.MsgID}, "reason": "violation"})
	for _, c := range []*client{u1a, u1b, u2, u3} {
		for {
			f := c.waitCmd(protocol.CmdEvent)
			var ev protocol.MsgDeletedEvent
			json.Unmarshal(f.Data, &ev)
			if ev.Type == "readUpdated" {
				continue
			}
			if ev.Type != "msgDeleted" || ev.ConvID != "group:g1" || len(ev.MsgIDs) != 1 || ev.MsgIDs[0] != target.MsgID || ev.Seqs[0] != 5 {
				t.Errorf("%s event = %s", c.uid, f.Data)
			}
			break
		}
	}
	off := m.dial("off", "web")
	raw := off.call(protocol.CmdSync, map[string]any{"convId": "group:g1", "afterSeq": 3, "limit": 10})
	var sync struct {
		Messages []map[string]any `json:"messages"`
	}
	json.Unmarshal(raw.Data, &sync)
	if len(sync.Messages) != 3 {
		t.Fatalf("sync = %s", raw.Data)
	}
	for i, want := range []float64{4, 5, 6} {
		if sync.Messages[i]["seq"] != want {
			t.Errorf("seq[%d] = %v", i, sync.Messages[i]["seq"])
		}
	}
	if d := sync.Messages[1]; d["deleted"] != true || d["body"] != nil || d["deletedAt"] == nil {
		t.Errorf("deleted message = %v", d)
	}
	if sync.Messages[0]["body"] == nil {
		t.Errorf("neighbour lost its body: %v", sync.Messages[0])
	}
	before := convOf(t, off, "g1").Unread

	// 删最后一条：摘要占位，没有 text；未读不变。
	m.must("/admin/messages/delete", map[string]any{"convId": "group:g1", "msgIds": []string{acks[5].MsgID}, "reason": "violation"})
	raw2 := off.call(protocol.CmdConvs, map[string]any{})
	if bytes.Contains(raw2.Data, []byte("text5")) || !bytes.Contains(raw2.Data, []byte(`"deleted":true`)) {
		t.Errorf("convs = %s", raw2.Data)
	}
	var cv struct {
		Convs []struct {
			LastMsg map[string]any `json:"lastMsg"`
		} `json:"convs"`
	}
	json.Unmarshal(raw2.Data, &cv)
	if _, has := cv.Convs[0].LastMsg["text"]; has {
		t.Errorf("deleted lastMsg has text: %v", cv.Convs[0].LastMsg)
	}
	if got := convOf(t, off, "g1"); got.Unread != before {
		t.Errorf("unread %d -> %d", before, got.Unread)
	}
	// 新消息之后摘要恢复。
	sendG(t, u1a, "g1", "cmid-00c9", "fresh")
	if got := convOf(t, off, "g1"); got.LastMsg.Deleted || got.LastMsg.Text != "fresh" {
		t.Errorf("after new message: %+v", got.LastMsg)
	}
	// 元数据接口不因 body 被清空而报错。
	code, b := m.call(m.mod, "/admin/convs/messages/query", map[string]any{"convId": "group:g1"})
	if code != 200 || !bytes.Contains(b, []byte(`"deleted":true`)) {
		t.Errorf("meta: %d %s", code, b)
	}
}

// TC-ADM-31-06、31-08：边界和错误码；已删除的幂等且不再发事件。
func TestAdmDeleteBoundaries(t *testing.T) {
	m := newModEnv(t, nil)
	m.users("u2")
	m.createGroup("g1", "u1", "u2")
	m.createGroup("g2", "u1", "u2")
	c, u2 := m.dial("u1", "web"), m.dial("u2", "web")
	a1 := sendG(t, c, "g1", "cmid-00c1", "one")
	a2 := sendG(t, c, "g2", "cmid-00c2", "two")
	del := func(conv string, ids ...string) (int, []byte) {
		return m.call(m.mod, "/admin/messages/delete", map[string]any{"convId": conv, "msgIds": ids, "reason": "violation"})
	}
	many := make([]string, 51)
	for i := range many {
		many[i] = fmt.Sprintf("m%d", i)
	}
	code, b := del("group:g1", many...)
	expectCode(t, "51", code, b, 400, protocol.CodeBatchLimitExceeded)
	code, b = del("group:g1", a2.MsgID)
	expectCode(t, "cross conv", code, b, 400, protocol.CodeBadRequest)
	code, b = del("group:g1", "ghost")
	expectCode(t, "ghost", code, b, 404, protocol.CodeMessageNotFound)
	code, b = del("group:g1")
	expectCode(t, "empty", code, b, 400, protocol.CodeBadRequest)
	// 一批里有一条不合法：整批不删。
	code, b = del("group:g1", a1.MsgID, "ghost")
	expectCode(t, "partial", code, b, 404, protocol.CodeMessageNotFound)
	if s := mustAck[protocol.SyncAck](t, u2.call(protocol.CmdSync, map[string]any{"convId": "group:g1", "afterSeq": 0, "limit": 10})); s.Messages[0].Deleted {
		t.Error("partial batch deleted a message")
	}

	if code, b = del("group:g1", a1.MsgID); code != 200 {
		t.Fatalf("%d %s", code, b)
	}
	drainUntilEvent(t, u2, "msgDeleted")
	u2.pending = nil // 等事件时顺带收下的 msg 帧
	code, b = del("group:g1", a1.MsgID)
	if code != 200 || !bytes.Contains(b, []byte(`"deleted":[]`)) {
		t.Errorf("idempotent: %d %s", code, b)
	}
	u2.expectNoFrame(200 * time.Millisecond)

	// 群提示消息不能删。
	m.users("owner1", "x1", "x2")
	m.mutual("owner1", "x1", "x2")
	m.token("owner1")
	own := m.dial("owner1", "web")
	gid := createG(t, own, "x1", "x2").GroupID
	tip := tips(t, own, gid)
	if len(tip) == 0 {
		t.Fatal("no tip")
	}
	code, b = del("group:"+gid, tip[0].MsgID)
	expectCode(t, "tip", code, b, 400, protocol.CodeBadRequest)
}

func drainUntilEvent(t *testing.T, c *client, typ string) {
	t.Helper()
	for {
		f := c.waitCmd(protocol.CmdEvent)
		var ev struct{ Type string }
		json.Unmarshal(f.Data, &ev)
		if ev.Type == typ {
			return
		}
	}
}

// TC-ADM-31-07、31-09：审计有 msgId 和理由，没有原文；业务方调用方效果相同，deletedBy 带它的 id。
func TestAdmDeleteAuditAndBusinessCaller(t *testing.T) {
	m := newModEnv(t, nil)
	m.users("u2")
	m.createGroup("g1", "u1", "u2")
	c := m.dial("u1", "web")
	a1 := sendG(t, c, "g1", "cmid-00c1", "ZXQ-AUDIT-777")
	m.must("/admin/messages/delete", map[string]any{"convId": "group:g1", "msgIds": []string{a1.MsgID}, "reason": "violation-A"})
	found := false
	for _, e := range m.auditOf(m.mod) {
		s := auditText(e)
		if strings.Contains(s, "ZXQ-AUDIT-777") {
			t.Errorf("audit has plaintext: %s", s)
		}
		if strings.Contains(s, a1.MsgID) && strings.Contains(s, "violation-A") {
			found = true
		}
	}
	if !found {
		t.Error("audit has no msgId + reason")
	}
	a2 := sendG(t, c, "g1", "cmid-00c2", "two")
	if code, b := m.signed(m.biz, http.MethodPost, "/admin/messages/delete", map[string]any{"convId": "group:g1", "msgIds": []string{a2.MsgID}, "reason": "biz"}, signOpts{ts: timeMs(m.now)}); code != 200 {
		t.Fatalf("%d %s", code, b)
	}
	db, err := sql.Open("sqlite", m.dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var by string
	if err := db.QueryRow(`SELECT deleted_by FROM messages WHERE msg_id = ?`, a2.MsgID).Scan(&by); err != nil || by != "biz:" {
		t.Errorf("deleted_by = %q, %v", by, err)
	}
}

// TC-ADM-32-01：检查文件原始字节：messages、quarantine、会话摘要都搜不到明文。
func TestAdmQuarantineRawBytes(t *testing.T) {
	m := newModEnv(t, func(c *app.Config) { c.SendRate = 1000 }) // 冻结时钟下令牌不恢复
	m.users("u2")
	m.createGroup("g1", "u1", "u2")
	c := m.dial("u1", "web")
	const secret = "ZXQ-QUAR-321"
	// 被删的那条夹在中间、比别的长：不是页面里最后一行，也不能靠新内容盖住旧内容。
	for i := range 10 {
		sendG(t, c, "g1", fmt.Sprintf("cmid-a%02d", i), strings.Repeat("a", 200))
	}
	a := sendG(t, c, "g1", "cmid-00c1", secret+strings.Repeat("b", 500)+secret)
	for i := range 10 {
		sendG(t, c, "g1", fmt.Sprintf("cmid-z%02d", i), strings.Repeat("z", 200))
	}
	db, err := sql.Open("sqlite", m.dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	checkpoint := func() {
		t.Helper()
		if _, err := db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
			t.Fatal(err)
		}
	}
	read := func() []byte {
		t.Helper()
		var all []byte
		for _, suffix := range []string{"", "-wal"} {
			b, err := os.ReadFile(m.dbPath + suffix)
			if err != nil && !os.IsNotExist(err) {
				t.Fatal(err)
			}
			all = append(all, b...)
		}
		return all
	}
	checkpoint()
	if !bytes.Contains(read(), []byte(secret)) {
		t.Fatal("control failed: plaintext should be on disk before the delete")
	}
	m.must("/admin/messages/delete", map[string]any{"convId": "group:g1", "msgIds": []string{a.MsgID}, "reason": "violation"})
	checkpoint()
	if bytes.Contains(read(), []byte(secret)) {
		t.Fatal("plaintext still in the database file after delete")
	}
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM message_quarantine WHERE msg_id = ?`, a.MsgID).Scan(&n); err != nil || n != 1 {
		t.Fatalf("quarantine rows = %d, %v", n, err)
	}
}

// ---- 隔离原文与正文授权 ----

func (m *modEnv) grant(conv string, quarantine bool) (int, []byte) {
	m.t.Helper()
	return m.call(m.body, "/admin/msg-grants", map[string]any{"convId": conv, "operator": "op1", "reason": "case review", "ticket": "T-1", "quarantine": quarantine})
}

func (m *modEnv) mustGrant(conv string, quarantine bool) string {
	m.t.Helper()
	code, b := m.grant(conv, quarantine)
	if code != 200 {
		m.t.Fatalf("grant: %d %s", code, b)
	}
	var g struct {
		GrantID   string `json:"grantId"`
		ExpiresAt int64  `json:"expiresAt"`
	}
	json.Unmarshal(b, &g)
	if g.ExpiresAt != m.now+15*60_000 {
		m.t.Errorf("expiresAt = %d", g.ExpiresAt)
	}
	return g.GrantID
}

func (m *modEnv) readBody(grant, conv string) (int, []byte) {
	m.t.Helper()
	return m.call(m.body, "/admin/convs/messages/body", map[string]any{"grantId": grant, "convId": conv, "limit": 50})
}

// TC-ADM-32-02、32-03：隔离原文能看；到期后 404；清理任务把行物理删除。
func TestAdmQuarantineGrantAndExpiry(t *testing.T) {
	m := newModEnv(t, nil)
	m.users("u2")
	m.createGroup("g1", "u1", "u2")
	c := m.dial("u1", "web")
	a := sendG(t, c, "g1", "cmid-00c1", "ZXQ-QUAR-321")
	sendG(t, c, "g1", "cmid-00c2", "kept")
	m.must("/admin/messages/delete", map[string]any{"convId": "group:g1", "msgIds": []string{a.MsgID}, "reason": "violation"})

	// 普通授权看不到原文，只有占位。
	g := m.mustGrant("group:g1", false)
	code, b := m.readBody(g, "group:g1")
	if code != 200 || bytes.Contains(b, []byte("ZXQ-QUAR-321")) || !bytes.Contains(b, []byte("kept")) {
		t.Errorf("body read: %d %s", code, b)
	}
	// 隔离授权看到原文，只有被删的那条。
	q := m.mustGrant("group:g1", true)
	code, b = m.readBody(q, "group:g1")
	if code != 200 || !bytes.Contains(b, []byte("ZXQ-QUAR-321")) || bytes.Contains(b, []byte("kept")) {
		t.Errorf("quarantine read: %d %s", code, b)
	}
	for _, e := range m.auditOf(m.body) {
		if strings.Contains(auditText(e), "ZXQ-QUAR-321") {
			t.Errorf("audit has plaintext: %s", e.Summary)
		}
	}

	// 30 天零 1 小时：还没跑清理也看不到；跑完行没了。
	m.advance(30*24*time.Hour + time.Hour)
	q2 := m.mustGrant("group:g1", true)
	code, b = m.readBody(q2, "group:g1")
	expectCode(t, "expired", code, b, 404, protocol.CodeMessageNotFound)
	if code, b := m.adminPost("/admin/test/jobs/quarantine_purge/run", map[string]any{}, adminKey); code != 200 {
		t.Fatalf("run job: %d %s", code, b)
	}
	db, err := sql.Open("sqlite", m.dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM message_quarantine`).Scan(&n); err != nil || n != 0 {
		t.Errorf("rows = %d, %v", n, err)
	}
	code, b = m.adminPost("/admin/test/jobs/nope/run", map[string]any{}, adminKey)
	expectCode(t, "unknown job", code, b, 400, protocol.CodeBadRequest)
}

// TC-ADM-08-*、AC-KEY-02-7、AC-ADM-32-5：单聊开关、配额、主凭证、换会话、过期、no-store。
func TestAdmMsgGrants(t *testing.T) {
	m := newModEnv(t, nil)
	m.users("u2")
	m.createGroup("g1", "u1", "u2")
	m.createGroup("g2", "u1", "u2")
	c := m.dial("u1", "web")
	sendG(t, c, "g1", "cmid-00c1", "hello")
	ack := mustAck[protocol.SendAck](t, c.sendText("c2c", "u2", "cmid-00c2", "private"))

	// 单聊开关关闭：任何凭证都读不到，隔离原文也一样，不产生授权。
	m.must("/admin/messages/delete", map[string]any{"convId": ack.ConvID, "msgIds": []string{ack.MsgID}, "reason": "violation"})
	for _, q := range []bool{false, true} {
		code, b := m.grant(ack.ConvID, q)
		expectCode(t, "c2c disabled", code, b, 403, protocol.CodeForbidden)
		if !bytes.Contains(b, []byte(`"reason":"c2cBodyDisabled"`)) {
			t.Errorf("reason: %s", b)
		}
	}
	// 主凭证没有正文 scope。
	code, b := m.call(m.mod, "/admin/msg-grants", map[string]any{"convId": "group:g1", "operator": "op1", "reason": "r", "ticket": "T"})
	expectCode(t, "main caller", code, b, 403, protocol.CodeForbidden)
	code, b = m.call(m.mod, "/admin/convs/messages/body", map[string]any{"grantId": "x", "convId": "group:g1"})
	expectCode(t, "main caller read", code, b, 403, protocol.CodeForbidden)
	// 校验：operator 必须和签名的请求头一致，工单号、理由必填。
	for name, req := range map[string]map[string]any{
		"operator mismatch": {"convId": "group:g1", "operator": "other", "reason": "r", "ticket": "T"},
		"no ticket":         {"convId": "group:g1", "operator": "op1", "reason": "r"},
		"no reason":         {"convId": "group:g1", "operator": "op1", "ticket": "T"},
	} {
		code, b := m.call(m.body, "/admin/msg-grants", req)
		expectCode(t, name, code, b, 400, protocol.CodeBadRequest)
	}

	g := m.mustGrant("group:g1", false)
	code, b = m.readBody(g, "group:g1")
	if code != 200 || !bytes.Contains(b, []byte("hello")) {
		t.Errorf("read: %d %s", code, b)
	}
	code, b = m.readBody(g, "group:g2")
	expectCode(t, "other conv", code, b, 403, protocol.CodeMsgReadGrantRequired)
	code, b = m.readBody("ghost", "group:g1")
	expectCode(t, "ghost grant", code, b, 403, protocol.CodeMsgReadGrantRequired)
	req := m.signedRequest(m.body, http.MethodPost, "/admin/convs/messages/body", map[string]any{"grantId": g, "convId": "group:g1"}, signOpts{ts: timeMs(m.now), operator: "op1"})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.Header.Get("Cache-Control") != "no-store" {
		t.Errorf("Cache-Control = %q", resp.Header.Get("Cache-Control"))
	}
	// 别的操作人拿到 grantId 也不能用。
	code, b = m.signed(m.body, http.MethodPost, "/admin/convs/messages/body", map[string]any{"grantId": g, "convId": "group:g1"}, signOpts{ts: timeMs(m.now), operator: "op2"})
	expectCode(t, "other operator", code, b, 403, protocol.CodeMsgReadGrantRequired)
	m.advance(16 * time.Minute)
	code, b = m.readBody(g, "group:g1")
	expectCode(t, "expired", code, b, 403, protocol.CodeMsgReadGrantRequired)
}

// TC-ADM-08-08：每操作人每天 20 个、调用方每天 200 个，正文和隔离合并计算；Asia/Shanghai 零点重置。
func TestAdmMsgGrantQuota(t *testing.T) {
	m := newModEnv(t, nil)
	m.createGroup("g1", "u1")
	// 起点放在上海时间当天 01:00，后面推进到次日。
	day := time.UnixMilli(t0).In(time.FixedZone("CST", 8*3600))
	m.setClock(time.Date(day.Year(), day.Month(), day.Day(), 1, 0, 0, 0, day.Location()).UnixMilli())
	for i := range 20 {
		if code, b := m.grant("group:g1", i%2 == 0); code != 200 {
			t.Fatalf("grant %d: %d %s", i, code, b)
		}
	}
	code, b := m.grant("group:g1", false)
	expectCode(t, "21st", code, b, 429, protocol.CodeQuotaExceeded)
	grantFor := func(op string) (int, []byte) {
		return m.signed(m.body, http.MethodPost, "/admin/msg-grants", map[string]any{"convId": "group:g1", "operator": op, "reason": "r", "ticket": "T"}, signOpts{ts: timeMs(m.now), operator: op})
	}
	if code, b := grantFor("op2"); code != 200 {
		t.Errorf("other operator is not limited: %d %s", code, b)
	}
	for i := range 179 {
		if code, b := grantFor(fmt.Sprintf("x%d", i)); code != 200 {
			t.Fatalf("grant %d: %d %s", i, code, b)
		}
	}
	code, b = grantFor("fresh")
	expectCode(t, "201st for caller", code, b, 429, protocol.CodeQuotaExceeded)
	m.advance(23 * time.Hour) // 上海次日 00:00
	if code, b := m.grant("group:g1", false); code != 200 {
		t.Errorf("next day: %d %s", code, b)
	}
}

// ---- 禁言 ----

func (m *modEnv) muteUser(uid string, seconds any) []byte {
	m.t.Helper()
	return m.must("/admin/mute", map[string]any{"scope": "user", "userId": uid, "durationSeconds": seconds, "reason": "spam"})
}

// TC-ADM-40-01、40-02、40-03：单聊群聊都 4031；接收、sync、read 正常；到期恢复。
func TestAdmUserMute(t *testing.T) {
	m := newModEnv(t, nil)
	m.users("u2")
	m.createGroup("g1", "u1", "u2")
	u1, u2 := m.dial("u1", "web"), m.dial("u2", "web")
	m.muteUser("u1", 600)
	want := m.now + 600_000
	drainUntilEvent(t, u1, "muted")
	for _, f := range []frame{u1.sendText("c2c", "u2", "cmid-00c1", "x"), u1.sendText("group", "g1", "cmid-00c2", "x")} {
		mustErr(t, f, protocol.CodeMuted)
		if f.Error.Details["scope"] != "user" || int64(f.Error.Details["mutedUntil"].(float64)) != want {
			t.Errorf("details = %v", f.Error.Details)
		}
	}
	a := mustAck[protocol.SendAck](t, u2.sendText("c2c", "u1", "cmid-00c3", "to u1"))
	u1.waitCmd(protocol.CmdMsg)
	if s := mustAck[protocol.SyncAck](t, u1.call(protocol.CmdSync, map[string]any{"convId": a.ConvID, "afterSeq": 0, "limit": 10})); len(s.Messages) != 1 {
		t.Errorf("sync = %+v", s)
	}
	if f := u1.call(protocol.CmdRead, map[string]any{"convId": a.ConvID, "seq": a.Seq}); f.Error != nil {
		t.Errorf("read: %+v", f.Error)
	}
	m.advance(11 * time.Minute)
	mustAck[protocol.SendAck](t, u1.sendText("c2c", "u2", "cmid-00c4", "back"))
}

// TC-ADM-40-04、40-06、40-08：解除事件；离线禁言 auth ack 带 mutedUntil（永久 null）；4031 不扣令牌。
func TestAdmMuteEventsAndRateLimit(t *testing.T) {
	m := newModEnv(t, nil)
	m.users("u1", "u2")
	m.muteUser("u1", nil)
	c := m.dial("u1", "web") // dial 的 ack 里看不到，下面用裸连接断言
	f := c.sendText("c2c", "u2", "cmid-00c0", "x")
	mustErr(t, f, protocol.CodeMuted)
	if v, ok := f.Error.Details["mutedUntil"]; !ok || v != nil {
		t.Errorf("permanent details = %v", f.Error.Details)
	}
	for i := range 30 {
		mustErr(t, c.sendText("c2c", "u2", fmt.Sprintf("flood-%03d", i), "x"), protocol.CodeMuted)
	}
	m.must("/admin/unmute", map[string]any{"scope": "user", "userId": "u1", "reason": "ok"})
	drainUntilEvent(t, c, "unmuted")
	mustAck[protocol.SendAck](t, c.sendText("c2c", "u2", "cmid-after", "ok"))
	// 重复解除：成功，不再发事件。
	m.must("/admin/unmute", map[string]any{"scope": "user", "userId": "u1", "reason": "again"})
	c.expectNoFrame(200 * time.Millisecond)

	m.muteUser("u1", 86400)
	drainUntilEvent(t, c, "muted")
	raw := m.dialRaw("auth", m.wsURL("", "web"), nil)
	raw.writeRaw([]byte(fmt.Sprintf(`{"id":"a1","cmd":"auth","data":{"token":%q}}`, m.token("u1"))))
	ack := raw.waitCmd(protocol.CmdAck)
	if !bytes.Contains(ack.Data, []byte(fmt.Sprintf(`"mutedUntil":%d`, m.now+86400_000))) {
		t.Errorf("auth ack = %s", ack.Data)
	}
	raw2 := m.dialRaw("auth2", m.wsURL("", "web"), nil)
	raw2.writeRaw([]byte(fmt.Sprintf(`{"id":"a1","cmd":"auth","data":{"token":%q}}`, m.token("u2"))))
	if ack := raw2.waitCmd(protocol.CmdAck); bytes.Contains(ack.Data, []byte("mutedUntil")) {
		t.Errorf("unmuted auth ack = %s", ack.Data)
	}
	_, u := m.signed(m.mod, http.MethodGet, "/admin/users/u1", nil, signOpts{ts: timeMs(m.now)})
	if !bytes.Contains(u, []byte(fmt.Sprintf(`"mutedUntil":%d`, m.now+86400_000))) {
		t.Errorf("user = %s", u)
	}
	_, u = m.signed(m.mod, http.MethodGet, "/admin/users/u2", nil, signOpts{ts: timeMs(m.now)})
	if bytes.Contains(u, []byte("mutedUntil")) {
		t.Errorf("unmuted user = %s", u)
	}
}

// TC-ADM-40-07、41-05 及负向。
func TestAdmMuteValidation(t *testing.T) {
	m := newModEnv(t, nil)
	m.createGroup("g1", "u1")
	m.users("u9")
	for _, tc := range []struct {
		name string
		req  map[string]any
		st   int
		code int
	}{
		{"nobody", map[string]any{"scope": "user", "userId": "nobody", "durationSeconds": 600, "reason": "spam"}, 404, protocol.CodeUserNotFound},
		{"not member", map[string]any{"scope": "group", "groupId": "g1", "userId": "u9", "durationSeconds": 600, "reason": "spam"}, 404, protocol.CodeTargetNotGroupMember},
		{"no group", map[string]any{"scope": "group", "groupId": "gx", "userId": "u9", "durationSeconds": 600, "reason": "spam"}, 404, protocol.CodeGroupNotFound},
		{"bad scope", map[string]any{"scope": "everyone", "groupId": "g1", "userId": "u1", "durationSeconds": 600, "reason": "spam"}, 400, protocol.CodeBadRequest},
		{"too short", map[string]any{"scope": "user", "userId": "u1", "durationSeconds": 59, "reason": "spam"}, 400, protocol.CodeBadRequest},
		{"too long", map[string]any{"scope": "user", "userId": "u1", "durationSeconds": 30*24*3600 + 1, "reason": "spam"}, 400, protocol.CodeBadRequest},
		{"no duration", map[string]any{"scope": "user", "userId": "u1", "reason": "spam"}, 400, protocol.CodeBadRequest},
		{"no reason", map[string]any{"scope": "user", "userId": "u1", "durationSeconds": 600}, 400, protocol.CodeBadRequest},
	} {
		code, b := m.call(m.mod, "/admin/mute", tc.req)
		expectCode(t, tc.name, code, b, tc.st, tc.code)
	}
}

// TC-ADM-41-01、41-02、41-03、41-04、41-06：群内禁言。
func TestAdmGroupMute(t *testing.T) {
	m := newModEnv(t, nil)
	m.users("u2")
	m.createGroup("g1", "u1", "u2")
	m.createGroup("g2", "u1", "u2")
	u1 := m.dial("u1", "web")
	u2 := m.dial("u2", "web")
	m.must("/admin/mute", map[string]any{"scope": "group", "groupId": "g1", "userId": "u1", "durationSeconds": 3600, "reason": "spam"})
	want := m.now + 3600_000
	// 只有本人收到事件。
	f := u1.waitCmd(protocol.CmdEvent)
	if !bytes.Contains(f.Data, []byte(`"scope":"group"`)) || !bytes.Contains(f.Data, []byte(`"groupId":"g1"`)) {
		t.Errorf("event = %s", f.Data)
	}
	u2.expectNoFrame(200 * time.Millisecond)

	e := u1.sendText("group", "g1", "cmid-00c1", "x")
	mustErr(t, e, protocol.CodeMuted)
	if e.Error.Details["scope"] != "group" || e.Error.Details["groupId"] != "g1" || int64(e.Error.Details["mutedUntil"].(float64)) != want {
		t.Errorf("details = %v", e.Error.Details)
	}
	mustAck[protocol.SendAck](t, u1.sendText("group", "g2", "cmid-00c2", "ok"))
	mustAck[protocol.SendAck](t, u1.sendText("c2c", "u2", "cmid-00c3", "ok"))

	// convs：g1 带 mutedUntil，g2 不带。
	if c := convOf(t, u1, "g1"); !bytes.Contains(c.MutedUntil, []byte(fmt.Sprint(want))) {
		t.Errorf("g1 conv = %+v", c)
	}
	if c := convOf(t, u1, "g2"); c.MutedUntil != nil {
		t.Errorf("g2 conv = %s", c.MutedUntil)
	}
	// 成员列表带 mutedUntil。
	_, b := m.signed(m.mod, http.MethodGet, "/admin/groups/g1/members", nil, signOpts{ts: timeMs(m.now)})
	if bytes.Count(b, []byte("mutedUntil")) != 1 || !bytes.Contains(b, []byte(fmt.Sprintf(`"mutedUntil":%d`, want))) {
		t.Errorf("members = %s", b)
	}

	// 同时被用户级禁言：返回 scope=user，mutedUntil 取用户级。
	m.muteUser("u1", 600)
	e = u1.sendText("group", "g1", "cmid-00c4", "x")
	mustErr(t, e, protocol.CodeMuted)
	if e.Error.Details["scope"] != "user" || int64(e.Error.Details["mutedUntil"].(float64)) != m.now+600_000 {
		t.Errorf("both: %v", e.Error.Details)
	}
	m.must("/admin/unmute", map[string]any{"scope": "user", "userId": "u1", "reason": "ok"})

	// 解除后标记消失。
	m.must("/admin/unmute", map[string]any{"scope": "group", "groupId": "g1", "userId": "u1", "reason": "ok"})
	_, b = m.signed(m.mod, http.MethodGet, "/admin/groups/g1/members", nil, signOpts{ts: timeMs(m.now)})
	if bytes.Contains(b, []byte("mutedUntil")) {
		t.Errorf("members after unmute = %s", b)
	}
}

// TC-ADM-41-03：移出再加入、解散都清除禁言。缓存也要跟上：移出之前发过一次，让记录进了缓存。
func TestAdmGroupMuteClearedOnLeave(t *testing.T) {
	m := newModEnv(t, nil)
	m.users("u2")
	m.createGroup("g1", "u1", "u2")
	u1 := m.dial("u1", "web")
	m.must("/admin/mute", map[string]any{"scope": "group", "groupId": "g1", "userId": "u1", "durationSeconds": 3600, "reason": "spam"})
	mustErr(t, u1.sendText("group", "g1", "cmid-00c1", "x"), protocol.CodeMuted) // 进缓存
	if code, b := m.adminDo(http.MethodDelete, "/admin/groups/g1/members", map[string]any{"members": []string{"u1"}}, adminKey); code != 200 {
		t.Fatalf("%d %s", code, b)
	}
	if code, b := m.adminPost("/admin/groups/g1/members", map[string]any{"members": []string{"u1"}}, adminKey); code != 200 {
		t.Fatalf("%d %s", code, b)
	}
	mustAck[protocol.SendAck](t, u1.sendText("group", "g1", "cmid-00c2", "back"))
	db, err := sql.Open("sqlite", m.dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// 解散：记录在同一事务删除。
	m.must("/admin/mute", map[string]any{"scope": "group", "groupId": "g1", "userId": "u2", "durationSeconds": 3600, "reason": "spam"})
	if code, b := m.signed(m.dis, http.MethodDelete, "/admin/groups/g1", map[string]any{}, signOpts{ts: timeMs(m.now)}); code != 200 {
		t.Fatalf("dissolve: %d %s", code, b)
	}
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM mutes WHERE group_id = 'g1'`).Scan(&n); err != nil || n != 0 {
		t.Errorf("mutes rows = %d, %v", n, err)
	}
}

// TC-ADM-40-09（双节点）：节点 1 禁言，连在节点 2 的用户 1 秒内返回 4031；TTL 60 秒兜底。
func TestAdmMuteTwoNodes(t *testing.T) {
	db := t.TempDir() + "/shared.db"
	n1 := newModEnv(t, func(c *app.Config) { c.DBPath = db })
	n2 := newSocEnvCfg(t, func(c *app.Config) {
		c.DBPath, c.V13AdminFeatures, c.QuarantineKey = db, true, testQuarantineKey
		c.AdminCallers = append(c.AdminCallers, n1.mod.cfg)
	})
	n1.app.SetTestPeers([]string{n2.adminSrv.URL})
	n2.app.SetTestPeers([]string{n1.adminSrv.URL})
	n1.users("u2")
	c := n2.dial("u1", "web")
	mustAck[protocol.SendAck](t, c.sendText("c2c", "u2", "cmid-00c0", "before")) // 缓存里是"没有禁言"
	n1.muteUser("u1", 600)
	deadline := time.Now().Add(time.Second)
	for i := 0; ; i++ {
		f := c.sendText("c2c", "u2", fmt.Sprintf("cmid-00%d", i+1), "x")
		if f.Error != nil && f.Error.Code == protocol.CodeMuted {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("node 2 did not see the mute within 1s")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// ---- 定时任务 ----

// AC-ADM-32-3 的机制：审计只留 1 年。
func TestAdmAuditPurgeJob(t *testing.T) {
	m := newModEnv(t, nil)
	m.token("u1")
	m.ban("u1", 3600)
	if len(m.auditOf(m.mod)) == 0 {
		t.Fatal("no audit")
	}
	m.advance(366 * 24 * time.Hour)
	if code, b := m.adminPost("/admin/test/jobs/admin_audit_purge/run", map[string]any{}, adminKey); code != 200 {
		t.Fatalf("%d %s", code, b)
	}
	db, err := sql.Open("sqlite", m.dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRow(`SELECT count(*) FROM admin_audit WHERE caller = 'mod' AND route LIKE '%/ban'`).Scan(&n); err != nil || n != 0 {
		t.Errorf("rows = %d, %v", n, err)
	}
}

// auditText 是一条审计的全部文本，用来搜特征串。
func auditText(e auditEntry) string {
	b, _ := json.Marshal(e)
	return string(b)
}

// auditOf 查这个调用方的审计（按测试时钟）。
func (m *modEnv) auditOf(c testCaller) []auditEntry {
	m.t.Helper()
	path := fmt.Sprintf("/admin/audit?from=%d&to=%d&limit=200&caller=%s", m.now-86_400_000, m.now+60_000, c.cfg.ID)
	code, body := m.signed(m.mod, http.MethodGet, path, nil, signOpts{ts: timeMs(m.now)})
	if code != http.StatusOK {
		m.t.Fatalf("query audit: %d %s", code, body)
	}
	var r struct{ Entries []auditEntry }
	json.Unmarshal(body, &r)
	return r.Entries
}

// AC-KEY-02-2 的第二批部分（IM_V13_ADMIN_FEATURES 开启才注册）：全部路由 × 有 / 没有 scope。
func TestEverySecondBatchRouteChecksScope(t *testing.T) {
	samples := map[string]struct {
		path       string
		body       map[string]any
		want, code int // 有 scope 时的结果
	}{
		"POST /admin/users/{userId}/ban":   {"/admin/users/alice/ban", map[string]any{"durationSeconds": 3600, "reason": "spam"}, 200, 0},
		"POST /admin/users/{userId}/unban": {"/admin/users/alice/unban", map[string]any{"reason": "ok"}, 200, 0},
		"POST /admin/messages/delete":      {"/admin/messages/delete", map[string]any{"convId": "group:g1", "msgIds": []string{"ghost"}, "reason": "spam"}, 404, protocol.CodeMessageNotFound},
		"POST /admin/mute":                 {"/admin/mute", map[string]any{"scope": "user", "userId": "alice", "durationSeconds": 600, "reason": "spam"}, 200, 0},
		"POST /admin/unmute":               {"/admin/unmute", map[string]any{"scope": "user", "userId": "alice", "reason": "ok"}, 200, 0},
		"POST /admin/msg-grants":           {"/admin/msg-grants", map[string]any{"convId": "group:g1", "operator": "op1", "reason": "r", "ticket": "T"}, 200, 0},
		"POST /admin/convs/messages/body":  {"/admin/convs/messages/body", map[string]any{"grantId": "ghost", "convId": "group:g1"}, 403, protocol.CodeMsgReadGrantRequired},
	}
	seen := 0
	for _, r := range newModEnv(t, nil).app.AdminRoutes() {
		key := r.Method + " " + r.Pattern
		if _, first := adminSamples[key]; first {
			continue
		}
		s, ok := samples[key]
		if !ok {
			t.Errorf("route %s (scope %s) has no sample", key, r.Scope)
			continue
		}
		seen++
		with, without := hmacCaller("with", r.Scope), hmacCaller("without", nonBodyScopes(r.Scope)...)
		e := newModEnv(t, func(c *app.Config) { c.AdminCallers = append(c.AdminCallers, with.cfg, without.cfg) })
		e.users("alice")
		code, body := e.signed(without, http.MethodPost, s.path, s.body, signOpts{ts: timeMs(e.now), operator: "op1"})
		expectCode(t, key+" without", code, body, http.StatusForbidden, protocol.CodeForbidden)
		code, body = e.signed(with, http.MethodPost, s.path, s.body, signOpts{ts: timeMs(e.now), operator: "op1"})
		expectCode(t, key+" with", code, body, s.want, s.code)
	}
	if seen != len(samples) {
		t.Errorf("checked %d routes, want %d", seen, len(samples))
	}
}
