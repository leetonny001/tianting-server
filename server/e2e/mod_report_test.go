package e2e

// C6 举报提交（BE-495）：TC-MOD-03-01～08、03-10、03-12～15 的后端部分。证据用测试密钥解开检查（审核员读取随 BE-496）。
// 本地结果，CI 不运行。

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"tianting/server/internal/app"
	"tianting/server/internal/logic"
	"tianting/server/internal/protocol"
	"tianting/server/internal/quarantine"
	"tianting/server/internal/store"
)

func reportCall(c *client, data map[string]any) frame {
	c.t.Helper()
	return c.call(protocol.CmdReport, data)
}

func reportMsg(c *client, convID string, ids []string, reason string, note *string) frame {
	c.t.Helper()
	d := map[string]any{"targetType": "message", "convId": convID, "msgIds": ids, "reason": reason}
	if note != nil {
		d["note"] = *note
	}
	return reportCall(c, d)
}

func mustReport(t *testing.T, f frame) string {
	t.Helper()
	ack := mustAck[protocol.ReportAck](t, f)
	if !strings.HasPrefix(ack.ReportID, "rp_") {
		t.Fatalf("reportId = %q", ack.ReportID)
	}
	return ack.ReportID
}

func strp(s string) *string { return &s }

// onlySentinel：哨兵法（TC-MOD 通用约定）。rx 之前没有任何帧（pong 除外），send 发的哨兵是它收到的下一条。
func onlySentinel(t *testing.T, rx *client, send func() protocol.SendAck) {
	t.Helper()
	for _, f := range rx.pending {
		if f.Cmd != protocol.CmdPong {
			t.Fatalf("%s: unexpected frame %s %s", rx.uid, f.Cmd, f.Data)
		}
	}
	rx.pending = nil
	time.Sleep(noFrameWindow)
	ack := send()
	for {
		f := rx.next()
		if f.Cmd == protocol.CmdPong {
			continue
		}
		if f.Cmd == protocol.CmdMsg && f.msg(t).MsgID == ack.MsgID {
			return
		}
		t.Fatalf("%s: unexpected frame before the sentinel: %s %s", rx.uid, f.Cmd, f.Data)
	}
}

// evidenceOf 用测试密钥解开一条举报的证据（库里直接读，不走管理接口）。
func evidenceOf(t *testing.T, dbPath, reportID, key string) (logic.EvidenceHead, []logic.EvidenceMessage, error) {
	t.Helper()
	db := openDB(t, dbPath)
	rows, err := db.Query(`SELECT idx, nonce, ciphertext, kid FROM report_evidence WHERE report_id = ? ORDER BY idx`, reportID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var ev []store.EvidenceRow
	for rows.Next() {
		var r store.EvidenceRow
		if err := rows.Scan(&r.Idx, &r.Nonce, &r.Ciphertext, &r.KID); err != nil {
			t.Fatal(err)
		}
		ev = append(ev, r)
	}
	if len(ev) == 0 {
		t.Fatalf("no evidence for %s", reportID)
	}
	c, err := quarantine.New(key)
	if err != nil {
		t.Fatal(err)
	}
	return logic.OpenEvidence(c, reportID, ev)
}

func (e *c6Env) quotaUsed(uid string) int {
	e.t.Helper()
	return e.count(`SELECT COALESCE(SUM(count), 0) FROM report_quota WHERE user_id = ?`, uid)
}

// TC-MOD-03-01（AC-MOD-03-1 的后端部分）、TC-MOD-03-03（AC-MOD-03-3）：u2 举报 u1 的单聊消息（abuse）成功，reports 一行 pending；
// u1 只收到之后的哨兵（被举报人不收任何通知，规则 7）。同一消息举报 3 次（abuse、spam、abuse）返回同一个 reportId，只有 1 条记录，
// 理由是第一次的，当天次数只算 1；[IM 钟] 推进 25 小时再举报产生新记录。
func TestTC_MOD_03_01_ReportAndIdempotency(t *testing.T) {
	e := newC6Env(t, nil)
	e.usersNamed("u1", "u2")
	u1, u2 := e.dial("u1", "web"), e.dial("u2", "web")
	m1 := mustAck[protocol.SendAck](t, sendWait(u1, "c2c", "u2", "hi"))
	u1.drain()
	id := mustReport(t, reportMsg(u2, m1.ConvID, []string{m1.MsgID}, "abuse", nil))
	onlySentinel(t, u1, func() protocol.SendAck { return mustAck[protocol.SendAck](t, sendWait(u2, "c2c", "u1", "哨兵")) })
	for _, reason := range []string{"spam", "abuse"} {
		if again := mustReport(t, reportMsg(u2, m1.ConvID, []string{m1.MsgID}, reason, nil)); again != id {
			t.Errorf("duplicate got %s, want %s", again, id)
		}
	}
	var state, reason, target, reporter string
	if err := e.db().QueryRow(`SELECT state, reason, target_user, reporter FROM reports WHERE report_id = ?`, id).Scan(&state, &reason, &target, &reporter); err != nil {
		t.Fatal(err)
	}
	if state != "pending" || reason != "abuse" || target != "u1" || reporter != "u2" || e.count(`SELECT COUNT(*) FROM reports`) != 1 || e.quotaUsed("u2") != 1 {
		t.Errorf("row = %s %s %s %s; reports %d; quota %d", state, reason, target, reporter, e.count(`SELECT COUNT(*) FROM reports`), e.quotaUsed("u2"))
	}
	e.advance(25 * time.Hour)
	if again := mustReport(t, reportMsg(u2, m1.ConvID, []string{m1.MsgID}, "spam", nil)); again == id {
		t.Error("after 24 hours a new report must be created")
	}
	if n := e.count(`SELECT COUNT(*) FROM reports`); n != 2 {
		t.Errorf("reports = %d", n)
	}
}

// TC-MOD-03-02（AC-MOD-03-2）：不是参与者、用别的会话的 convId、入群前的消息（historyVisible=false）：都 404 / 4040 reportTargetInvalid，
// 响应逐字节相同，没有记录，都计入当天次数（第 115 条）；21 条 400 / 4131（格式阶段，不计次数）。
func TestTC_MOD_03_02_TargetInvalid(t *testing.T) {
	e := newC6Env(t, nil)
	e.usersNamed("u1", "u2", "u4", "u9")
	e.createGroup("g1", "u1", "u2")
	u1, u2, u4, u9 := e.dial("u1", "web"), e.dial("u2", "web"), e.dial("u4", "web"), e.dial("u9", "web")
	m1 := mustAck[protocol.SendAck](t, sendWait(u1, "c2c", "u2", "hi"))
	g5 := mustAck[protocol.SendAck](t, sendWait(u1, "group", "g1", "入群前"))
	if code, b := e.adminPost("/admin/groups/g1/members", map[string]any{"members": []string{"u4"}}, adminKey); code != http.StatusOK {
		t.Fatalf("add u4: %d %s", code, b)
	}
	var bodies []string
	for label, f := range map[string]frame{
		"not a participant":  reportMsg(u9, m1.ConvID, []string{m1.MsgID}, "abuse", nil),
		"other conversation": reportMsg(u2, "group:g1", []string{m1.MsgID}, "abuse", nil),
		"before joining":     reportMsg(u4, "group:g1", []string{g5.MsgID}, "abuse", nil),
		"no such message":    reportMsg(u2, m1.ConvID, []string{"nomsg0001"}, "abuse", nil),
	} {
		if f.Error == nil || f.Error.Code != protocol.CodeNotFound || f.Error.Reason != protocol.ReasonReportTargetInvalid {
			t.Errorf("%s: %+v", label, f.Error)
		}
		bodies = append(bodies, errBody(f))
	}
	for _, b := range bodies[1:] {
		if b != bodies[0] {
			t.Errorf("responses differ: %s vs %s", b, bodies[0])
		}
	}
	ids := make([]string, 21)
	for i := range ids {
		ids[i] = fmt.Sprintf("m%03d", i)
	}
	mustErr(t, reportMsg(u2, m1.ConvID, ids, "abuse", nil), protocol.CodeBatchLimitExceeded)
	if n := e.count(`SELECT COUNT(*) FROM reports`); n != 0 {
		t.Errorf("reports = %d", n)
	}
	if e.quotaUsed("u9") != 1 || e.quotaUsed("u2") != 2 || e.quotaUsed("u4") != 1 {
		t.Errorf("quota u9=%d u2=%d u4=%d", e.quotaUsed("u9"), e.quotaUsed("u2"), e.quotaUsed("u4"))
	}
}

// TC-MOD-03-04（AC-MOD-03-4）：当天前 20 次成功，第 21 次 429 / 4292、没有记录；[IM 钟] 推进到次日 00:00 之后成功。
func TestTC_MOD_03_04_DailyLimit(t *testing.T) {
	e := newC6Env(t, nil)
	e.usersNamed("u1", "u2")
	u1, u2 := e.dial("u1", "web"), e.dial("u2", "web")
	e.setClock(shanghaiMidnight(t).Add(time.Hour).UnixMilli())
	var acks []protocol.SendAck
	for range 22 {
		acks = append(acks, mustAck[protocol.SendAck](t, sendWait(u1, "c2c", "u2", "hi")))
	}
	for i := range 20 {
		mustReport(t, reportMsg(u2, acks[i].ConvID, []string{acks[i].MsgID}, "spam", nil))
	}
	mustErr(t, reportMsg(u2, acks[20].ConvID, []string{acks[20].MsgID}, "spam", nil), protocol.CodeQuotaExceeded)
	// 限额在对象校验之前（BE-495 第 1 条）：用完之后举报一个不存在的对象也是 4292，不是 4040。
	mustErr(t, reportMsg(u2, acks[20].ConvID, []string{"nomsg0001"}, "spam", nil), protocol.CodeQuotaExceeded)
	if n := e.count(`SELECT COUNT(*) FROM reports`); n != 20 {
		t.Errorf("reports = %d", n)
	}
	e.setClock(shanghaiMidnight(t).Add(24*time.Hour + time.Minute).UnixMilli())
	mustReport(t, reportMsg(u2, acks[21].ConvID, []string{acks[21].MsgID}, "spam", nil))
}

// TC-MOD-03-05（AC-MOD-03-5 的后端部分）、TC-MOD-03-08（AC-MOD-03-8）：举报含 ZXQ-RPT-1 的群消息（note 带 ZXQ-MOD-NOTE）后发送者撤回：
// 检查点后主库和 WAL 的原始字节搜不到两个特征串；用测试密钥解开证据有原文和 note；日志、admin_audit、reports 行搜不到。
func TestTC_MOD_03_05_GroupEvidenceEncrypted(t *testing.T) {
	var logs lockedBuf
	e := newC6Env(t, func(c *app.Config) {
		c.MsgActions.Enabled = true
		c.MsgActions.RecallRatePerMin = 10000
		c.Logger = newDebugLogger(&logs)
	})
	e.usersNamed("o", "u1", "u2")
	e.mutual("o", "u1", "u2")
	o, u1, u2 := e.dial("o", "web"), e.dial("u1", "web"), e.dial("u2", "web")
	gid := createG(t, o, "u1", "u2").GroupID
	m1 := mustAck[protocol.SendAck](t, sendWait(u1, "group", gid, "ZXQ-RPT-1 内容"))
	id := mustReport(t, reportMsg(u2, m1.ConvID, []string{m1.MsgID}, "abuse", strp("ZXQ-MOD-NOTE")))
	mustRecall(t, u1, m1.ConvID, m1.MsgID)
	raw := rawDB(t, e.dbPath)
	for _, marker := range []string{"ZXQ-RPT-1", "ZXQ-MOD-NOTE"} {
		if bytes.Contains(raw, []byte(marker)) {
			t.Errorf("raw database bytes contain %s", marker)
		}
		if strings.Contains(logs.String(), marker) {
			t.Errorf("logs contain %s", marker)
		}
	}
	head, msgs, err := evidenceOf(t, e.dbPath, id, testReportKey)
	if err != nil {
		t.Fatal(err)
	}
	if head.Note == nil || *head.Note != "ZXQ-MOD-NOTE" || head.C2C || len(msgs) != 1 || msgs[0].Body == nil || msgs[0].Body.Text != "ZXQ-RPT-1 内容" || msgs[0].Recalled {
		t.Errorf("evidence = %+v %+v", head, msgs)
	}
	if !strings.Contains(logs.String(), id) || !strings.Contains(logs.String(), "targetType=message") {
		t.Errorf("log has no reportId / targetType")
	}
}

// TC-MOD-03-06（AC-MOD-03-6）：[IM 钟] 推进 179 天跑 report_purge 证据还在；181 天后 report_evidence 没有这条，reports 行也一起删除（P-48 ②）。
func TestTC_MOD_03_06_Purge(t *testing.T) {
	e := newC6Env(t, nil)
	e.usersNamed("u1", "u2")
	e.createGroup("g1", "u1", "u2")
	u1, u2 := e.dial("u1", "web"), e.dial("u2", "web")
	m := mustAck[protocol.SendAck](t, sendWait(u1, "group", "g1", "hi"))
	id := mustReport(t, reportMsg(u2, m.ConvID, []string{m.MsgID}, "spam", nil))
	run := func() {
		if code, b := e.adminPost("/admin/test/jobs/report_purge/run", map[string]any{}, adminKey); code != http.StatusOK {
			t.Fatalf("report_purge: %d %s", code, b)
		}
	}
	e.advance(179 * 24 * time.Hour)
	run()
	if e.count(`SELECT COUNT(*) FROM report_evidence WHERE report_id = ?`, id) == 0 || e.count(`SELECT COUNT(*) FROM reports`) != 1 {
		t.Fatal("purged at 179 days")
	}
	e.advance(2 * 24 * time.Hour)
	run()
	if e.count(`SELECT COUNT(*) FROM report_evidence`) != 0 || e.count(`SELECT COUNT(*) FROM reports`) != 0 {
		t.Error("not purged after 181 days")
	}
}

// TC-MOD-03-07（AC-MOD-03-7）：举报完全没有关联的 u9、不存在的 nouser、自己：都 4040，响应逐字节相同；
// 好友 u1、只有共同群的 u3、只有单聊消息的 u6 都成功（第 116 条）。用户举报只存元数据，note 按单聊对待（P-48 ①）。
func TestTC_MOD_03_07_UserReports(t *testing.T) {
	e := newC6Env(t, nil)
	e.usersNamed("u1", "u2", "u3", "u6", "u9")
	e.mutual("u2", "u1")
	e.createGroup("g1", "u2", "u3")
	u2, u6 := e.dial("u2", "web"), e.dial("u6", "web")
	mustAck[protocol.SendAck](t, sendWait(u6, "c2c", "u2", "hi"))
	user := func(uid string) frame {
		return reportCall(u2, map[string]any{"targetType": "user", "userId": uid, "reason": "fraud", "note": "ZXQ-MOD-NOTE"})
	}
	var bodies []string
	for _, uid := range []string{"u9", "nouser", "u2"} {
		f := user(uid)
		if f.Error == nil || f.Error.Code != protocol.CodeNotFound || f.Error.Reason != protocol.ReasonReportTargetInvalid {
			t.Errorf("%s: %+v", uid, f.Error)
		}
		bodies = append(bodies, errBody(f))
	}
	if bodies[0] != bodies[1] || bodies[0] != bodies[2] {
		t.Errorf("responses differ: %v", bodies)
	}
	for _, uid := range []string{"u1", "u3", "u6"} {
		id := mustReport(t, user(uid))
		head, msgs, err := evidenceOf(t, e.dbPath, id, testReportKey)
		if err != nil || !head.C2C || head.Note == nil || len(msgs) != 0 {
			t.Errorf("%s evidence = %+v %+v %v", uid, head, msgs, err)
		}
	}
}

// TC-MOD-03-10（AC-MOD-03-10 的存储部分）：IM_C2C_BODY_ACCESS=off 时举报含 ZXQ-RPT-2 的单聊消息：原始字节和解开的明文都没有特征串，
// 只有元数据（类型、长度）；同一个库、同一把密钥用 on 重启之后，证据仍然没有正文（读取的接口部分随 BE-496）。
func TestTC_MOD_03_10_C2COffMetadataOnly(t *testing.T) {
	e := newC6Env(t, nil)
	e.usersNamed("u1", "u2")
	u1, u2 := e.dial("u1", "web"), e.dial("u2", "web")
	m := mustAck[protocol.SendAck](t, sendWait(u1, "c2c", "u2", "ZXQ-RPT-2 单聊"))
	id := mustReport(t, reportMsg(u2, m.ConvID, []string{m.MsgID}, "abuse", nil))
	head, msgs, err := evidenceOf(t, e.dbPath, id, testReportKey)
	if err != nil {
		t.Fatal(err)
	}
	plain, _ := json.Marshal([]any{head, msgs})
	if bytes.Contains(plain, []byte("ZXQ-RPT-2")) || len(msgs) != 1 || msgs[0].Body != nil || msgs[0].Type != "text" || msgs[0].BodyLength != 12 || !head.C2C {
		t.Errorf("evidence = %s", plain)
	}
	// 打开开关之后举报的同一条消息（另一个举报人）有正文；之前那条仍然没有。
	on := newC6Env(t, func(c *app.Config) { c.DBPath = e.dbPath; c.C2CBodyAccess = true })
	v1 := on.dial("u1", "web")
	id2 := mustReport(t, reportMsg(v1, m.ConvID, []string{m.MsgID}, "abuse", nil))
	if _, msgs, _ := evidenceOf(t, e.dbPath, id, testReportKey); msgs[0].Body != nil {
		t.Error("the report made while off gained a body")
	}
	if _, msgs, _ := evidenceOf(t, e.dbPath, id2, testReportKey); msgs[0].Body == nil || msgs[0].Body.Text != "ZXQ-RPT-2 单聊" {
		t.Errorf("on: %+v", msgs)
	}
}

// TC-MOD-03-12（AC-MOD-03-12）：u3 没读到（readSeq < seq）的已撤回消息 404 / 4040；u2 读过的已撤回消息在隔离原文过期之后举报成功，
// 证据只有元数据。读过、隔离原文还在的举报到的是原文（AC-MOD-03-11 的后端部分）。
func TestTC_MOD_03_12_Recalled(t *testing.T) {
	e := newC6Env(t, func(c *app.Config) { c.MsgActions.Enabled = true; c.MsgActions.RecallRatePerMin = 10000 })
	e.usersNamed("o", "u1", "u2", "u3")
	e.mutual("o", "u1", "u2", "u3")
	o, u1, u2, u3 := e.dial("o", "web"), e.dial("u1", "web"), e.dial("u2", "web"), e.dial("u3", "web")
	gid := createG(t, o, "u1", "u2", "u3").GroupID
	m1 := mustAck[protocol.SendAck](t, sendWait(u1, "group", gid, "ZXQ-RPT-3 第一条"))
	m2 := mustAck[protocol.SendAck](t, sendWait(u1, "group", gid, "ZXQ-RPT-3 第二条"))
	mustAck[protocol.Empty](t, u2.call(protocol.CmdRead, map[string]any{"convId": m2.ConvID, "seq": m2.Seq}))
	mustRecall(t, u1, m1.ConvID, m1.MsgID)
	mustRecall(t, u1, m2.ConvID, m2.MsgID)
	f := reportMsg(u3, m1.ConvID, []string{m1.MsgID}, "abuse", nil)
	if f.Error == nil || f.Error.Code != protocol.CodeNotFound || f.Error.Reason != protocol.ReasonReportTargetInvalid {
		t.Errorf("unread recalled: %+v", f.Error)
	}
	id := mustReport(t, reportMsg(u2, m1.ConvID, []string{m1.MsgID}, "abuse", nil))
	if _, msgs, err := evidenceOf(t, e.dbPath, id, testReportKey); err != nil || msgs[0].Body == nil || msgs[0].Body.Text != "ZXQ-RPT-3 第一条" || !msgs[0].Recalled {
		t.Errorf("recalled with quarantine: %+v %v", msgs, err)
	}
	e.advance(31 * 24 * time.Hour)
	if code, b := e.adminPost("/admin/test/jobs/quarantine_purge/run", map[string]any{}, adminKey); code != http.StatusOK {
		t.Fatalf("quarantine_purge: %d %s", code, b)
	}
	id2 := mustReport(t, reportMsg(u2, m2.ConvID, []string{m2.MsgID}, "abuse", nil))
	if _, msgs, err := evidenceOf(t, e.dbPath, id2, testReportKey); err != nil || msgs[0].Body != nil || !msgs[0].Recalled || msgs[0].MsgID != m2.MsgID {
		t.Errorf("quarantine expired: %+v %v", msgs, err)
	}
}

// TC-MOD-03-13（AC-MOD-03-13）：举报群：非成员、不存在的群 4040 且响应相同；当前成员成功；reason=xxx、201 个码点的 note（含 emoji）400 / 4000；
// 200 个码点的 note 成功。群举报只存元数据（不快照群名、公告，P-48 ①）。
func TestTC_MOD_03_13_GroupReports(t *testing.T) {
	e := newC6Env(t, nil)
	e.usersNamed("u2", "u9", "u3")
	e.createGroup("g1", "u2", "u3")
	u2, u9 := e.dial("u2", "web"), e.dial("u9", "web")
	group := func(c *client, gid, reason string, note *string) frame {
		d := map[string]any{"targetType": "group", "groupId": gid, "reason": reason}
		if note != nil {
			d["note"] = *note
		}
		return reportCall(c, d)
	}
	a, b := group(u9, "g1", "spam", nil), group(u9, "gnone", "spam", nil)
	if a.Error == nil || a.Error.Reason != protocol.ReasonReportTargetInvalid || errBody(a) != errBody(b) {
		t.Errorf("non-member %s, missing %s", errBody(a), errBody(b))
	}
	id := mustReport(t, group(u2, "g1", "spam", nil))
	if head, msgs, err := evidenceOf(t, e.dbPath, id, testReportKey); err != nil || head.C2C || len(msgs) != 0 {
		t.Errorf("group evidence = %+v %+v %v", head, msgs, err)
	}
	mustErr(t, group(u2, "g1", "xxx", nil), protocol.CodeBadRequest)
	mustErr(t, group(u2, "g1", "illegal", strp(strings.Repeat("😀", 101)+strings.Repeat("举", 100))), protocol.CodeBadRequest)
	mustReport(t, group(u2, "g1", "illegal", strp(strings.Repeat("😀", 100)+strings.Repeat("举", 100))))
}

// TC-MOD-03-14 ①（AC-MOD-03-14 前半）：故障注入 evidenceWrite 后举报 500 / 5000，reports、report_evidence 都没有记录，不计入当天次数；
// 关掉注入后成功。② 开关关闭的 4030 见 TestTC_CHAT_90_41_ModSwitchOff。
func TestTC_MOD_03_14_EvidenceWriteFails(t *testing.T) {
	e := newC6Env(t, nil)
	e.usersNamed("u1", "u2")
	e.createGroup("g1", "u1", "u2")
	u1, u2 := e.dial("u1", "web"), e.dial("u2", "web")
	m := mustAck[protocol.SendAck](t, sendWait(u1, "group", "g1", "hi"))
	e.faults(map[string]any{"evidenceWrite": true})
	mustErr(t, reportMsg(u2, m.ConvID, []string{m.MsgID}, "spam", nil), protocol.CodeInternal)
	if e.count(`SELECT COUNT(*) FROM reports`) != 0 || e.count(`SELECT COUNT(*) FROM report_evidence`) != 0 || e.quotaUsed("u2") != 0 {
		t.Error("a failed report left rows or used the quota")
	}
	e.faults(map[string]any{"evidenceWrite": false})
	mustReport(t, reportMsg(u2, m.ConvID, []string{m.MsgID}, "spam", nil))
}

// TC-MOD-03-15（AC-MOD-03-15）：文件消息被举报（第 10 天），原消息正常：其他成员照常取地址；过了 180 天保留期跑 file_purge 后普通用户 404 / 4048，
// 对象没有被删（evidence_until = 举报时间 + 180 天）；推进到举报后 181 天再跑 file_purge，对象被删除。证据里有文件名和 fileId。
func TestTC_MOD_03_15_FileEvidenceRetention(t *testing.T) {
	e := newC6FileEnv(t, nil)
	tok1, tok3 := e.token("u1"), e.token("u3")
	e.token("u2")
	e.createGroup("g1", "u1", "u2", "u3")
	id := e.uploadFile(tok1, "ZXQ-MOD-FILE.pdf", "application/pdf", pad([]byte("%PDF-1.4"), 2048))
	u1, u2 := e.dial("u1", "web"), e.dial("u2", "web")
	m := mustAck[protocol.SendAck](t, sendFile(u1, "group", "g1", "rf-000001", "file", id, nil))
	sent := e.now
	e.advanceTo(sent.Add(10 * 24 * time.Hour))
	rid := mustReport(t, reportMsg(u2, m.ConvID, []string{m.MsgID}, "illegal", nil))
	if _, msgs, err := evidenceOf(t, e.dbPath, rid, testReportKey); err != nil || msgs[0].FileID != id || msgs[0].Body == nil || msgs[0].Body.File == nil || msgs[0].Body.File.Name != "ZXQ-MOD-FILE.pdf" || msgs[0].FileSize != 2048 {
		t.Fatalf("evidence = %+v %v", msgs, err)
	}
	if f := e.file(id); f.EvidenceUntil != e.now.Add(180*24*time.Hour).UnixMilli() {
		t.Errorf("evidence_until = %d", f.EvidenceUntil)
	}
	tok3 = e.token("u3")
	e.mustURL(tok3, id, "original")
	e.advanceTo(sent.Add(181 * 24 * time.Hour))
	e.runPurge()
	tok3 = e.token("u3") // 7 天的 token 过期了
	e.denied(tok3, id, "original")
	if e.fileGone(id) || e.file(id).ObjectsDeleted {
		t.Fatal("objects deleted while the evidence still needs them")
	}
	e.advanceTo(sent.Add(191 * 24 * time.Hour))
	e.runPurge()
	if !e.file(id).ObjectsDeleted {
		t.Error("objects kept after the evidence expired")
	}
}

// 每天限额在写入事务里条件加 1（BE-495 第 1 条）：同一个人从 10 个连接并发举报 20 条不同的消息、限额 5，正好 5 条成功（-race 下也跑）。
func TestModReportQuotaConcurrent(t *testing.T) {
	e := newC6Env(t, func(c *app.Config) { c.Moderation.ReportDaily = 5 })
	e.usersNamed("u1", "u2")
	e.createGroup("g1", "u1", "u2")
	u1 := e.dial("u1", "web")
	var acks []protocol.SendAck
	for range 20 {
		acks = append(acks, mustAck[protocol.SendAck](t, sendWait(u1, "group", "g1", "hi")))
	}
	conns := make([]*client, 10)
	for i := range conns {
		conns[i] = e.dial("u2", "web")
	}
	var wg sync.WaitGroup
	var ok, quota atomic.Int64
	for i, a := range acks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := conns[i%len(conns)]
			raw, _ := json.Marshal(map[string]any{"id": fmt.Sprintf("q-%02d", i), "cmd": protocol.CmdReport,
				"data": map[string]any{"targetType": "message", "convId": a.ConvID, "msgIds": []string{a.MsgID}, "reason": "spam"}})
			c.writeRaw(raw) // coder/websocket 的 Write 可以并发调用
		}()
	}
	wg.Wait()
	for i, c := range conns {
		for range 2 {
			f := c.waitCmd(protocol.CmdAck)
			switch {
			case f.Error == nil:
				ok.Add(1)
			case f.Error.Code == protocol.CodeQuotaExceeded:
				quota.Add(1)
			default:
				t.Errorf("conn %d: %+v", i, f.Error)
			}
		}
	}
	if ok.Load() != 5 || quota.Load() != 15 || e.count(`SELECT COUNT(*) FROM reports`) != 5 || e.quotaUsed("u2") != 5 {
		t.Errorf("ok=%d quota=%d reports=%d used=%d", ok.Load(), quota.Load(), e.count(`SELECT COUNT(*) FROM reports`), e.quotaUsed("u2"))
	}
}
