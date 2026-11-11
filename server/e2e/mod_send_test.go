package e2e

// C6 发送检查（BE-494）：TC-MOD-02-01～08、02-10～14、02-16 的后端部分。只用特征词。本地结果，CI 不运行。

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"tianting/server/internal/adminauth"
	"tianting/server/internal/app"
	"tianting/server/internal/protocol"
)

// wordReady 加词并等它在这个节点生效：用一个只含这个词的探针轮询（探针本身会被拦下，不会落库）。
func (e *c6Env) wordReady(c *client, to, word string) {
	e.t.Helper()
	e.addWord(word)
	within(e.t, 10*time.Second, "word "+word, func() bool { return blockedFrame(sendWait(c, "c2c", to, word)) })
	e.mustExec(`DELETE FROM moderation_hits`) // 探针的命中不算进用例
}

func (e *c6Env) mustExec(q string, args ...any) {
	e.t.Helper()
	if _, err := e.db().Exec(q, args...); err != nil {
		e.t.Fatal(err)
	}
}

// TC-MOD-02-01（AC-MOD-02-1）：单聊、群聊发含拦截词：都 4510；u2、u3、o 只收到之后的哨兵；sync 和元数据都没有被拦截的消息。
func TestTC_MOD_02_01_BlockedNotStored(t *testing.T) {
	e := newC6Env(t, nil)
	e.usersNamed("o", "u1", "u2", "u3")
	e.mutual("o", "u1", "u2", "u3")
	cs := map[string]*client{}
	for _, u := range []string{"o", "u1", "u2", "u3"} {
		cs[u] = e.dial(u, "web")
	}
	gid := createG(t, cs["o"], "u1", "u2", "u3").GroupID
	for _, c := range cs {
		c.drain()
	}
	e.wordReady(cs["u1"], "u2", "违规词")
	assertBlocked(t, "c2c", sendWait(cs["u1"], "c2c", "u2", "这是违规词"), "违规词")
	assertBlocked(t, "group", sendWait(cs["u1"], "group", gid, "群里违规词"), "违规词")
	sentinelC2C := mustAck[protocol.SendAck](t, sendWait(cs["u1"], "c2c", "u2", "哨兵"))
	sentinelG := mustAck[protocol.SendAck](t, sendWait(cs["u1"], "group", gid, "哨兵"))
	for _, u := range []string{"u2", "u3", "o"} {
		for {
			m := cs[u].waitCmd(protocol.CmdMsg).msg(t)
			if strings.Contains(m.Body.Text, "违规词") {
				t.Fatalf("%s received a blocked message: %+v", u, m)
			}
			if m.MsgID == sentinelG.MsgID {
				break
			}
		}
	}
	for _, conv := range []string{sentinelC2C.ConvID, sentinelG.ConvID} {
		for _, m := range syncConv(t, cs["u2"], conv).Messages {
			if strings.Contains(m.Body.Text, "违规词") {
				t.Errorf("%s stored a blocked message", conv)
			}
		}
		code, b := e.as(e.cons, http.MethodPost, "/admin/convs/messages/query", map[string]any{"convId": conv})
		if code != http.StatusOK || bytes.Count(b, []byte(`"type":"text"`)) != 1 { // 只有哨兵（群里另有建群的 tip）
			t.Errorf("meta of %s: %d %s", conv, code, b)
		}
	}
}

// TC-MOD-02-02（AC-MOD-02-2）：badword 的全角、变音、大小写、零宽、空格、标点写法放在一句话中间，都 4510；
// bad 和 word 分在两条消息里都成功（只在单条消息内匹配）。
// TC-MOD-02-03（AC-MOD-02-3）：带全角字符、全角空格、零宽字符的普通文本发送成功，u2 收到和 sync 到的 text 逐字节相同（没有被标准化）。
func TestTC_MOD_02_02_VariantsAndOriginalText(t *testing.T) {
	e := newC6Env(t, nil)
	e.usersNamed("u1", "u2")
	u1, u2 := e.dial("u1", "web"), e.dial("u2", "web")
	e.wordReady(u1, "u2", "badword")
	for _, v := range []string{"ＢＡＤＷＯＲＤ", "bádwörd", "BadWord", "bad​word", "bad word", "bad-word"} {
		assertBlocked(t, v, sendWait(u1, "c2c", "u2", "今天说一句"+v+"就走"), "badword")
	}
	mustAck[protocol.SendAck](t, sendWait(u1, "c2c", "u2", "bad"))
	mustAck[protocol.SendAck](t, sendWait(u1, "c2c", "u2", "word"))
	const raw = "ＡＢＣ　１２３，ｇｏｏｄ​！"
	ack := mustAck[protocol.SendAck](t, sendWait(u1, "c2c", "u2", raw))
	for {
		m := u2.waitCmd(protocol.CmdMsg).msg(t)
		if m.MsgID == ack.MsgID {
			if m.Body.Text != raw {
				t.Errorf("u2 got %q, want the original %q", m.Body.Text, raw)
			}
			break
		}
	}
	if m := msgBySeq(t, syncConv(t, u2, ack.ConvID).Messages, ack.Seq); m.Body.Text != raw {
		t.Errorf("sync = %q", m.Body.Text)
	}
}

// TC-MOD-02-04（AC-MOD-02-4）：文件名含拦截词，发图片、发文件都 4510，消息不保存，文件仍是 ready（没有绑定）；
// 上行 body 里写一个普通的 name，服务端记录的名字含拦截词：同样 4510（查服务端记录，backend-chat 第 111 条）。
func TestTC_MOD_02_04_FileNames(t *testing.T) {
	e := newC6FileEnv(t, nil)
	e.addWord("违规词")
	e.addWord("badword")
	tok := e.token("u1")
	e.token("u2")
	img := e.uploadImage(tok, "违规词.png", fakeJPEG(800, 600, 32<<10), smallThumb())
	doc := e.uploadFile(tok, "ＢＡＤＷＯＲＤ.pdf", "application/pdf", pad([]byte("%PDF-1.4"), 2048))
	other := e.uploadFile(tok, "badword-report.pdf", "application/pdf", pad([]byte("%PDF-1.4"), 2048))
	ok := e.uploadFile(tok, "普通文件.pdf", "application/pdf", pad([]byte("%PDF-1.4"), 2048))
	c := e.dial("u1", "web")
	assertBlocked(t, "image", sendFile(c, "c2c", "u2", "f-img-0001", "image", img, nil), "违规词")
	assertBlocked(t, "file", sendFile(c, "c2c", "u2", "f-doc-0001", "file", doc, nil), "badword")
	assertBlocked(t, "client name", sendFile(c, "c2c", "u2", "f-oth-0001", "file", other, map[string]any{"name": "normal.pdf"}), "badword")
	for _, id := range []string{img, doc, other} {
		if f := e.file(id); f.State != "ready" || f.MsgID != "" {
			t.Errorf("%s: state %s msg %q", id, f.State, f.MsgID)
		}
	}
	mustAck[protocol.SendAck](t, sendFile(c, "c2c", "u2", "f-ok-0001", "file", ok, nil))
}

// TC-MOD-02-05（AC-MOD-02-5）：注册的昵称、用户名（含已被占用的用户名）、改昵称、建群的群名、改群名和公告、好友申请附言含拦截词：
// 都 4510，内容未写入；注册没有账号、邀请码次数不变；对存在和不存在的对象发申请响应逐字节相同；管理接口写含词的内容不查（规则 3）。
func TestTC_MOD_02_05_OtherScenes(t *testing.T) {
	cons, reader := c6Callers()
	e := newAcctEnv(t, func(c *app.Config) { applyC6(c, cons, reader) })
	if code, b := e.signed(cons, http.MethodPost, "/admin/moderation/words/import", map[string]any{"words": []string{"违规词", "badword"}}, signOpts{operator: "op1"}); code != http.StatusOK {
		t.Fatalf("import: %d %s", code, b)
	}
	time.Sleep(300 * time.Millisecond)
	e.newInvite("INVITE-MOD-0001", 10, time.Now().Add(24*time.Hour))
	taken := e.register("takenuser", map[string]any{"inviteCode": "INVITE-MOD-0001"})
	usedBefore := e.count(`SELECT used_count FROM invites WHERE invite_id = 'inv-INVITE-MOD-0001'`)
	accountsBefore := e.count(`SELECT COUNT(*) FROM accounts`)
	var bodies []string
	for _, tc := range []struct{ username, nickname string }{{"newuser1", "违规词"}, {"badwordx", ""}, {"takenuser", "违规词"}} {
		code, b, _ := e.post("/v1/account/register", e.freshIP(), regBody(tc.username, map[string]any{"inviteCode": "INVITE-MOD-0001", "nickname": tc.nickname}))
		if code != 451 || parseErr(t, b).Code != protocol.CodeContentBlocked || parseErr(t, b).Details != nil {
			t.Errorf("register %s / %s: %d %s", tc.username, tc.nickname, code, b)
		}
		bodies = append(bodies, string(b))
	}
	if bodies[2] != bodies[0] {
		t.Errorf("a taken username answers differently: %s vs %s", bodies[2], bodies[0])
	}
	if e.count(`SELECT COUNT(*) FROM accounts`) != accountsBefore || e.count(`SELECT used_count FROM invites WHERE invite_id = 'inv-INVITE-MOD-0001'`) != usedBefore {
		t.Error("a blocked registration wrote an account or used the invite")
	}
	if n := e.count(`SELECT COUNT(*) FROM moderation_hits`); n != 0 {
		t.Errorf("registration hits must not write rows (no user yet): %d", n)
	}
	// 改昵称。
	code, b := e.callV1(http.MethodPatch, "/v1/account/profile", taken.AccessToken, map[string]any{"nickname": "我是违规词"})
	if code != 451 || parseErr(t, b).Code != protocol.CodeContentBlocked {
		t.Errorf("profile: %d %s", code, b)
	}
	if n := e.count(`SELECT COUNT(*) FROM users WHERE user_id = 'takenuser' AND nickname LIKE '%违规词%'`); n != 0 {
		t.Error("nickname changed")
	}
	// 建群、改群名和公告、好友申请。
	e.usersNamed("u1", "u2", "u3")
	soc := &socEnv{env: e.env}
	soc.mutual("u1", "u2", "u3")
	u1 := e.dial("u1", "web")
	f := u1.call(protocol.CmdGroupCreate, map[string]any{"clientReqId": "req-mod-00001", "name": "违规词群", "members": []string{"u2", "u3"}})
	assertBlocked(t, "groupCreate", f, "违规词")
	if n := e.count(`SELECT COUNT(*) FROM chat_groups`); n != 0 {
		t.Errorf("a group was created: %d", n)
	}
	gid := createG(t, u1, "u2", "u3").GroupID
	for _, data := range []map[string]any{{"groupId": gid, "name": "违规词"}, {"groupId": gid, "notice": "公告 badword"}} {
		assertBlocked(t, "groupUpdate", u1.call(protocol.CmdGroupUpdate, data), "违规词")
	}
	if n := e.count(`SELECT COUNT(*) FROM chat_groups WHERE name LIKE '%违规词%' OR notice LIKE '%badword%'`); n != 0 {
		t.Error("group profile changed")
	}
	// 非群主改群名：先是权限 4030，不是 4510（检查在权限判定之后）。
	u2 := e.dial("u2", "web")
	mustErr(t, u2.call(protocol.CmdGroupUpdate, map[string]any{"groupId": gid, "name": "违规词"}), protocol.CodeForbidden)
	e.usersNamed("u5")
	a := u1.call(protocol.CmdFriendApply, map[string]any{"to": "u5", "message": "加我 违规词"})
	n := u1.call(protocol.CmdFriendApply, map[string]any{"to": "nouser", "message": "加我 违规词"})
	assertBlocked(t, "friendApply", a, "违规词")
	if errBody(a) != errBody(n) {
		t.Errorf("friendApply answers depend on existence: %s vs %s", errBody(a), errBody(n))
	}
	if c := e.count(`SELECT COUNT(*) FROM friend_requests WHERE owner_id = 'u5'`); c != 0 {
		t.Errorf("u5 has a request: %d", c)
	}
	// 命中记录：改昵称、建群、改群名、改公告、附言两次；注册没有。
	rows, err := e.rawDB().Query(`SELECT user_id, scene FROM moderation_hits ORDER BY rowid`)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for rows.Next() {
		var u, s string
		rows.Scan(&u, &s)
		got = append(got, u+":"+s)
	}
	rows.Close()
	want := "takenuser:nickname u1:groupName u1:groupName u1:groupNotice u1:friendApply u1:friendApply"
	if strings.Join(got, " ") != want {
		t.Errorf("hits = %v, want %s", got, want)
	}
	// 规则 3：管理接口写入的内容不查。
	if code, b := e.signed(cons, http.MethodPatch, "/admin/groups/"+gid, map[string]any{"name": "违规词"}, signOpts{operator: "op1"}); code != http.StatusOK {
		t.Errorf("admin rename: %d %s", code, b)
	}
	if code, b := e.signed(cons, http.MethodPost, "/admin/users/import", map[string]any{"users": []any{map[string]any{"userId": "u3", "nickname": "违规词"}}}, signOpts{operator: "op1"}); code != http.StatusOK {
		t.Errorf("admin nickname: %d %s", code, b)
	}
}

// TC-MOD-02-06（AC-MOD-02-6）：IM_CLIENT_CUSTOM_MSG=0 时客户端 custom 400 / 4000 customDisabled，消息未保存。
// TC-MOD-02-11（AC-MOD-02-11）：=1（开发默认）时成功，见 TestTC_CHAT_90_41_ModSwitchOff。
func TestTC_MOD_02_06_CustomDisabled(t *testing.T) {
	e := newC6Env(t, func(c *app.Config) { c.Moderation.ClientCustomOff = true })
	e.usersNamed("u1", "u2")
	u1 := e.dial("u1", "web")
	f := u1.call(protocol.CmdSend, map[string]any{"convType": "c2c", "to": "u2", "clientMsgId": "cm-custom-01", "type": protocol.MsgCustom, "body": map[string]any{"data": map[string]any{"k": "v"}}})
	if f.Error == nil || f.Error.Code != protocol.CodeBadRequest || f.Error.Reason != protocol.ReasonCustomDisabled {
		t.Fatalf("custom = %+v", f.Error)
	}
	if n := e.count(`SELECT COUNT(*) FROM messages`); n != 0 {
		t.Errorf("stored: %d", n)
	}
	mustAck[protocol.SendAck](t, sendWait(u1, "c2c", "u2", "text still works"))
}

// TC-MOD-02-07（AC-MOD-02-7）：被拉黑的、被禁言的发拦截词，依次 4036、4031（不是 4510），没有命中记录（检查在权限判定之后）。
func TestTC_MOD_02_07_PermissionFirst(t *testing.T) {
	e := newC6Env(t, nil)
	e.usersNamed("o", "u1", "u2", "u3")
	e.mutual("o", "u1", "u2", "u3")
	u1, u2, u3, o := e.dial("u1", "web"), e.dial("u2", "web"), e.dial("u3", "web"), e.dial("o", "web")
	gid := createG(t, o, "u1", "u3").GroupID
	e.wordReady(u1, "u2", "违规词")
	mustAck[protocol.Empty](t, u2.call(protocol.CmdBlock, map[string]any{"userId": "u1"}))
	e.mustAs(e.cons, http.MethodPost, "/admin/mute", map[string]any{"scope": "user", "userId": "u3", "durationSeconds": 3600, "reason": "spam"}, nil)
	mustErr(t, sendWait(u1, "c2c", "u2", "违规词"), protocol.CodeRejectedByPeer)
	mustErr(t, sendWait(u3, "group", gid, "违规词"), protocol.CodeMuted)
	if n := e.count(`SELECT COUNT(*) FROM moderation_hits`); n != 0 {
		t.Errorf("hits = %d", n)
	}
}

// TC-MOD-02-08（AC-MOD-02-8）：命中一次：moderation_hits 一行（u1、text、word_id、时间），表、日志、审计搜不到消息全文；
// im_blocked_total{scene="text"} 加 1。
func TestTC_MOD_02_08_HitRecord(t *testing.T) {
	var logs lockedBuf
	e := newC6Env(t, func(c *app.Config) { c.Logger = newDebugLogger(&logs) })
	e.usersNamed("u1", "u2")
	u1 := e.dial("u1", "web")
	e.wordReady(u1, "u2", "违规词")
	before := blockedMetric(t, e.app, "text")
	assertBlocked(t, "hit", sendWait(u1, "c2c", "u2", "ZXQ-MOD-HIT 违规词"))
	var uid, scene string
	var wordID, at int64
	if err := e.db().QueryRow(`SELECT user_id, scene, word_id, at FROM moderation_hits`).Scan(&uid, &scene, &wordID, &at); err != nil {
		t.Fatal(err)
	}
	if uid != "u1" || scene != "text" || wordID == 0 || at != e.now {
		t.Errorf("hit = %s %s %d %d", uid, scene, wordID, at)
	}
	if after := blockedMetric(t, e.app, "text"); after != before+1 {
		t.Errorf("im_blocked_total{scene=text} %v -> %v", before, after)
	}
	if strings.Contains(logs.String(), "ZXQ-MOD-HIT") || !strings.Contains(logs.String(), "content blocked") {
		t.Errorf("logs: %s", logs.String())
	}
	raw := rawDB(t, e.dbPath)
	if bytes.Contains(raw, []byte("ZXQ-MOD-HIT")) {
		t.Error("the blocked text is in the database")
	}
}

// TC-MOD-02-10（AC-MOD-02-10）：IM_MOD_ENABLED=0、词库表里有"违规词"（开关开时加的）：发送成功、u2 收到；features.moderation=false。
func TestTC_MOD_02_10_SwitchOffNoCheck(t *testing.T) {
	e := newC6Env(t, func(c *app.Config) { c.Moderation.Enabled = false })
	e.addWord("违规词") // 管理接口照常（P-48 ④）
	e.usersNamed("u1", "u2")
	u1, u2 := e.dial("u1", "web"), e.dial("u2", "web")
	ack := mustAck[protocol.SendAck](t, sendWait(u1, "c2c", "u2", "这是违规词"))
	if m := u2.waitCmd(protocol.CmdMsg).msg(t); m.MsgID != ack.MsgID || m.Body.Text != "这是违规词" {
		t.Errorf("u2 got %+v", m)
	}
	if f := publicConfig(t, e.env)["features"].(map[string]any); f["moderation"] != false {
		t.Errorf("features = %v", f)
	}
}

// TC-MOD-02-12（AC-MOD-02-12）：单聊给新对象发拦截词：4510；发送令牌减 1（限速 3 条、时钟冻结：3 次 4510 之后第 4 次 4290）；
// newPeerReject 退还（配额 10，连续 12 次都是 4510，不退还的话第 11 次就是 4292；最后正常发送成功）。
func TestTC_MOD_02_12_TokensAndNewPeerRefund(t *testing.T) {
	e := newC6Env(t, func(c *app.Config) { c.SendRate = 3 })
	e.usersNamed("u1", "u5", "u6")
	u1 := e.dial("u1", "web")
	e.addWord("违规词")
	within(t, 10*time.Second, "word", func() bool {
		e.resetLimits("u1")
		return blockedFrame(sendWait(u1, "c2c", "u6", "违规词"))
	})
	e.resetLimits("u1")
	for i := range 3 {
		assertBlocked(t, fmt.Sprint("attempt ", i), sendWait(u1, "c2c", "u5", "违规词"))
	}
	mustErr(t, sendWait(u1, "c2c", "u5", "hi"), protocol.CodeRateLimited)

	r := newC6Env(t, func(c *app.Config) { c.SocialDefaults = `{"newPeerRejectDailyLimit":10}` })
	r.usersNamed("u1", "u5", "u6")
	v1 := r.dial("u1", "web")
	r.wordReady(v1, "u6", "违规词")
	r.resetLimits("u1") // 探针发给 u6 也扣了 newPeerReject（被拦下又退还），这里清零从满额开始
	for i := range 12 {
		assertBlocked(t, fmt.Sprint("new peer ", i), sendWait(v1, "c2c", "u5", "违规词"))
	}
	mustAck[protocol.SendAck](t, sendWait(v1, "c2c", "u5", "hi"))
}

// TC-MOD-02-13（AC-MOD-02-13）：同场景把失败原因换成 4048、4096、4040，newPeerReject 都退还（C2、C4 已有，C6 补用例）：
// 配额 10，三种失败各 4 次共 12 次都保持原来的错误码（不退还的话第 11 次是 4292），最后正常发送成功。
func TestTC_MOD_02_13_OtherRefunds(t *testing.T) {
	e := newC6FileEnv(t, func(c *app.Config) {
		c.SocialDefaults = `{"newPeerRejectDailyLimit":10}`
		c.MsgActions.Enabled = true
	})
	tok := e.token("u1")
	e.token("u5")
	e.token("u6")
	c := e.dial("u1", "web")
	used := e.uploadFile(tok, "used.pdf", "application/pdf", pad([]byte("%PDF-1.4"), 2048))
	mustAck[protocol.SendAck](t, sendFile(c, "c2c", "u6", "bind-0001", "file", used, nil))
	hidden := mustAck[protocol.SendAck](t, c.sendText("c2c", "u6", "hidden-0001", "u5 看不到"))
	for i := range 4 {
		mustErr(t, sendFile(c, "c2c", "u5", fmt.Sprintf("r4048-%04d", i), "file", "doesnotexist0001", nil), protocol.CodeFileNotFound)
		mustErr(t, sendFile(c, "c2c", "u5", fmt.Sprintf("r4096-%04d", i), "file", used, nil), protocol.CodeFileNotReady)
		mustErr(t, sendQuote(c, "c2c", "u5", fmt.Sprintf("r4040-%04d", i), "re", hidden.MsgID), protocol.CodeNotFound)
	}
	mustAck[protocol.SendAck](t, c.sendText("c2c", "u5", "ok-000001", "hi"))
}

// TC-MOD-02-14（AC-MOD-02-14）：消息已保存后词库新增该词，同一 clientMsgId 重发成功（返回第一次的 ack，去重优先，规则 8）；
// 再发 3 条新的都 4510；/admin/moderation/hits 按场景 text: 3、最近 3 条、没有任何内容；[IM 钟] 推进 91 天跑 moderation_hits_purge 后为 0。
// 没有 report.read 的调用方 403（P-48 ③）。
func TestTC_MOD_02_14_ResendHitsPurge(t *testing.T) {
	sup := hmacCaller("support", adminauth.ScopeModerationWords, adminauth.ScopeUserRead, adminauth.ScopeMsgMeta)
	e := newC6Env(t, func(c *app.Config) { c.AdminCallers = append(c.AdminCallers, sup.cfg) })
	e.sup = sup
	e.usersNamed("u1", "u2")
	u1 := e.dial("u1", "web")
	first := mustAck[protocol.SendAck](t, u1.sendText("c2c", "u2", "cm-zxqnew-c1", "今天 ZXQ-NEW"))
	e.wordReady(u1, "u2", "ZXQ-NEW")
	if again := mustAck[protocol.SendAck](t, u1.sendText("c2c", "u2", "cm-zxqnew-c1", "今天 ZXQ-NEW")); again != first {
		t.Errorf("resend = %+v, want %+v", again, first)
	}
	for i := range 3 {
		assertBlocked(t, "new", u1.sendText("c2c", "u2", fmt.Sprintf("cm-zxqnew-n%d", i), "又是 ZXQ-NEW"))
	}
	type hitsResp struct {
		UserID string         `json:"userId"`
		Days   int            `json:"days"`
		Counts map[string]int `json:"counts"`
		Recent []struct {
			At     int64  `json:"at"`
			Scene  string `json:"scene"`
			WordID int64  `json:"wordId"`
		} `json:"recent"`
	}
	var r hitsResp
	code, b := e.as(e.cons, http.MethodGet, "/admin/moderation/hits?userId=u1", nil)
	if code != http.StatusOK || json.Unmarshal(b, &r) != nil {
		t.Fatalf("hits: %d %s", code, b)
	}
	if r.UserID != "u1" || r.Days != 90 || r.Counts["text"] != 3 || len(r.Counts) != 1 || len(r.Recent) != 3 || r.Recent[0].Scene != "text" || r.Recent[0].WordID == 0 {
		t.Errorf("hits = %+v", r)
	}
	if bytes.Contains(b, []byte("ZXQ")) {
		t.Errorf("hits carry content: %s", b)
	}
	code, b = e.as(e.sup, http.MethodGet, "/admin/moderation/hits?userId=u1", nil)
	expectCode(t, "without report.read", code, b, http.StatusForbidden, protocol.CodeForbidden)
	e.advance(91 * 24 * time.Hour)
	if code, b := e.adminPost("/admin/test/jobs/moderation_hits_purge/run", map[string]any{}, adminKey); code != http.StatusOK {
		t.Fatalf("purge: %d %s", code, b)
	}
	code, b = e.as(e.cons, http.MethodGet, "/admin/moderation/hits?userId=u1", nil)
	r = hitsResp{}
	if json.Unmarshal(b, &r) != nil || code != http.StatusOK || len(r.Counts) != 0 || len(r.Recent) != 0 || e.count(`SELECT COUNT(*) FROM moderation_hits`) != 0 {
		t.Errorf("after purge: %d %s", code, b)
	}
	code, b = e.as(e.cons, http.MethodGet, "/admin/moderation/hits?userId=u%201", nil)
	expectCode(t, "bad userId", code, b, http.StatusBadRequest, protocol.CodeInvalidID)
}

// TC-MOD-02-16（AC-MOD-02-16）：群主发 @ 所有人含拦截词：4510，次数不变（在保存事务里才扣），发送令牌减 1，成员没有 mentionUnread。
func TestTC_MOD_02_16_MentionAll(t *testing.T) {
	e := newC6Env(t, func(c *app.Config) {
		c.MsgActions.Enabled = true
		c.MsgActions.MentionAllHourly = 1
		c.SendRate = 3
	})
	e.usersNamed("o", "u1", "u2")
	e.mutual("o", "u1", "u2")
	o, u1 := e.dial("o", "web"), e.dial("u1", "web")
	gid := createG(t, o, "u1", "u2").GroupID
	e.addWord("违规词")
	within(t, 10*time.Second, "word", func() bool { e.resetLimits("o"); return blockedFrame(sendWait(o, "c2c", "u1", "违规词")) })
	e.resetLimits("o")
	for i := range 3 {
		assertBlocked(t, fmt.Sprint("mention all ", i), sendMentions(o, gid, nextCmid("ma"), "@所有人 违规词", nil, true))
	}
	mustErr(t, sendMentions(o, gid, nextCmid("ma"), "@所有人 hi", nil, true), protocol.CodeRateLimited)
	if n := e.count(`SELECT COUNT(*) FROM mention_all_quota`); n != 0 {
		t.Errorf("mention-all counted: %d", n)
	}
	if mu, _ := mentionUnread(t, u1, protocol.GroupConvID(gid)); mu != 0 {
		t.Errorf("mentionUnread = %d", mu)
	}
	e.resetLimits("o")
	mustAck[protocol.SendAck](t, sendMentions(o, gid, nextCmid("ma"), "@所有人 hi", nil, true)) // 次数还在（每小时 1 次）
}
