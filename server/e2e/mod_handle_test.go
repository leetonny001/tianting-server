package e2e

// C6 举报处理（BE-496）：TC-MOD-03-05 / 03-10 / 03-11 的审核员读取部分、TC-MOD-04-01～05、07、10、11、13、14、16、17 的后端部分、
// TC-CHAT-90-47 的 im_reports_pending。本地结果，CI 不运行；console-server 的理由、配额、动态码、水印随 CS-206～208。

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"tianting/server/internal/adminauth"
	"tianting/server/internal/app"
	"tianting/server/internal/protocol"
)

type evidenceResp struct {
	Messages []struct {
		MsgID    string            `json:"msgId"`
		Type     string            `json:"type"`
		Body     *protocol.MsgBody `json:"body"`
		Recalled bool              `json:"recalled"`
	} `json:"messages"`
	Note          *string `json:"note"`
	Undecryptable bool    `json:"undecryptable"`
}

func (e *c6Env) grant(reportID string) (int, []byte) {
	e.t.Helper()
	return e.as(e.reader, http.MethodPost, "/admin/report-grants", map[string]any{"reportId": reportID, "reason": "核查举报"})
}

func (e *c6Env) mustGrant(reportID string) string {
	e.t.Helper()
	code, b := e.grant(reportID)
	var g struct{ GrantID string }
	if code != http.StatusOK || json.Unmarshal(b, &g) != nil || g.GrantID == "" {
		e.t.Fatalf("grant: %d %s", code, b)
	}
	return g.GrantID
}

func (e *c6Env) evidence(reportID, grantID string) evidenceResp {
	e.t.Helper()
	code, b := e.as(e.reader, http.MethodPost, "/admin/reports/"+reportID+"/evidence", map[string]any{"grantId": grantID})
	var r evidenceResp
	if code != http.StatusOK || json.Unmarshal(b, &r) != nil {
		e.t.Fatalf("evidence: %d %s", code, b)
	}
	return r
}

func (e *c6Env) resolve(c testCaller, reportID, result string) (int, []byte) {
	e.t.Helper()
	return e.as(c, http.MethodPost, "/admin/reports/"+reportID+"/resolve", map[string]any{"result": result, "reason": "已核实"})
}

// TC-MOD-04-01（AC-MOD-04-1 的后端部分）、TC-MOD-03-05 的审核员读取：群消息被举报后撤回；审核员看详情、申请证据授权、读到举报时的原文和 note
// （证据是举报那一刻的快照，不依赖之后的撤回，F-MOD-03 规则 5）、
// 删除消息、结案 handled。im-server 审计按顺序有这几步的路由，审计里搜不到 ZXQ-RPT-4、ZXQ-MOD-NOTE；读证据的响应 no-store。
func TestTC_MOD_04_01_ReviewFlow(t *testing.T) {
	e := newC6Env(t, func(c *app.Config) { c.MsgActions.Enabled = true; c.MsgActions.RecallRatePerMin = 10000 })
	e.usersNamed("o", "u1", "u2")
	e.mutual("o", "u1", "u2")
	o, u1, u2 := e.dial("o", "web"), e.dial("u1", "web"), e.dial("u2", "web")
	gid := createG(t, o, "u1", "u2").GroupID
	m := mustAck[protocol.SendAck](t, sendWait(u1, "group", gid, "ZXQ-RPT-4 原文"))
	other := mustAck[protocol.SendAck](t, sendWait(u1, "group", gid, "另一条"))
	mustAck[protocol.Empty](t, u2.call(protocol.CmdRead, map[string]any{"convId": m.ConvID, "seq": other.Seq}))
	id := mustReport(t, reportMsg(u2, m.ConvID, []string{m.MsgID}, "abuse", strp("ZXQ-MOD-NOTE")))
	mustRecall(t, u1, m.ConvID, m.MsgID)
	var detail map[string]any
	e.mustAs(e.cons, http.MethodGet, "/admin/reports/"+id, nil, &detail)
	if detail["reporter"] != "u2" || detail["targetUser"] != "u1" || detail["state"] != "pending" || detail["convType"] != "group" || detail["targetReportCount"] != 1.0 {
		t.Errorf("detail = %v", detail)
	}
	g := e.mustGrant(id)
	code, b := e.as(e.reader, http.MethodPost, "/admin/reports/"+id+"/evidence", map[string]any{"grantId": g})
	if code != http.StatusOK {
		t.Fatalf("evidence: %d %s", code, b)
	}
	var ev evidenceResp
	json.Unmarshal(b, &ev)
	if len(ev.Messages) != 1 || ev.Messages[0].Body == nil || ev.Messages[0].Body.Text != "ZXQ-RPT-4 原文" || ev.Note == nil || *ev.Note != "ZXQ-MOD-NOTE" {
		t.Errorf("evidence = %s", b)
	}
	if bytes.Contains(b, []byte("另一条")) {
		t.Error("evidence carries context")
	}
	e.mustAs(e.cons, http.MethodPost, "/admin/messages/delete", map[string]any{"convId": m.ConvID, "msgIds": []string{other.MsgID}, "reason": "违规"}, nil)
	if code, b := e.resolve(e.cons, id, "handled"); code != http.StatusOK {
		t.Fatalf("resolve: %d %s", code, b)
	}
	e.mustAs(e.cons, http.MethodGet, "/admin/reports/"+id, nil, &detail)
	if detail["state"] != "handled" || detail["conclusion"] != "已核实" || detail["handledBy"] != "console:op1" {
		t.Errorf("after resolve = %v", detail)
	}
	rows, err := e.db().Query(`SELECT route, targets_json, summary_json FROM admin_audit WHERE phase = 'result' ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var routes []string
	var all strings.Builder
	for rows.Next() {
		var route, targets, summary string
		rows.Scan(&route, &targets, &summary)
		if strings.Contains(route, "report") || strings.Contains(route, "messages/delete") {
			routes = append(routes, route)
		}
		all.WriteString(route + targets + summary)
	}
	want := []string{"GET /admin/reports/{reportId}", "POST /admin/report-grants", "POST /admin/reports/{reportId}/evidence", "POST /admin/messages/delete",
		"POST /admin/reports/{reportId}/resolve", "GET /admin/reports/{reportId}"}
	if strings.Join(routes, ",") != strings.Join(want, ",") {
		t.Errorf("audit routes = %v", routes)
	}
	for _, marker := range []string{"ZXQ-RPT-4", "ZXQ-MOD-NOTE", g} {
		if strings.Contains(all.String(), marker) {
			t.Errorf("audit contains %s", marker)
		}
	}
	if !strings.Contains(all.String(), id) || !strings.Contains(all.String(), g[:8]) {
		t.Errorf("audit lacks reportId or the grant prefix: %s", all.String())
	}
}

// 队列（F-MOD-04 规则 1、第 117 条）：3 个人举报 u1（用户举报）聚合成一项"3 次"；单聊消息举报只有 convType（不带 convId、msgIds）；
// 群消息举报带 convId、msgIds；按状态、理由筛选；没有举报人和 note。TC-MOD-03-01 的"后台队列出现"。
func TestModReportQueue(t *testing.T) {
	e := newC6Env(t, nil)
	e.usersNamed("u1", "u2", "u3", "u6")
	e.mutual("u1", "u2", "u3", "u6")
	e.createGroup("g1", "u1", "u2")
	u1 := e.dial("u1", "web")
	for _, u := range []string{"u2", "u3", "u6"} {
		mustReport(t, reportCall(e.dial(u, "web"), map[string]any{"targetType": "user", "userId": "u1", "reason": "spam", "note": "ZXQ-MOD-NOTE"}))
		e.advance(time.Second)
	}
	u2 := e.dial("u2", "web")
	c2c := mustAck[protocol.SendAck](t, sendWait(u1, "c2c", "u2", "hi"))
	grp := mustAck[protocol.SendAck](t, sendWait(u1, "group", "g1", "hi"))
	mustReport(t, reportMsg(u2, c2c.ConvID, []string{c2c.MsgID}, "abuse", nil))
	e.advance(time.Second)
	mustReport(t, reportMsg(u2, grp.ConvID, []string{grp.MsgID}, "porn", nil))
	type item struct {
		TargetKey, TargetType, ConvType, ConvID, UserID string
		MsgIDs                                          []string
		Count                                           int
		Reports                                         []struct{ ReportID, Reason, State string }
	}
	var page struct {
		Items      []item
		NextCursor string
	}
	code, b := e.as(e.cons, http.MethodGet, "/admin/reports?state=pending", nil)
	if code != http.StatusOK || json.Unmarshal(b, &page) != nil || len(page.Items) != 3 {
		t.Fatalf("queue: %d %s", code, b)
	}
	if it := page.Items[0]; it.TargetType != "message" || it.ConvType != "group" || it.ConvID != grp.ConvID || len(it.MsgIDs) != 1 {
		t.Errorf("group item = %+v", it)
	}
	if it := page.Items[1]; it.ConvType != "c2c" || it.ConvID != "" || it.MsgIDs != nil {
		t.Errorf("c2c item = %+v", it)
	}
	if it := page.Items[2]; it.TargetType != "user" || it.UserID != "u1" || it.Count != 3 || len(it.Reports) != 3 {
		t.Errorf("user item = %+v", it)
	}
	for _, marker := range []string{"u2\"", "u3", "ZXQ-MOD-NOTE", c2c.ConvID} {
		if bytes.Contains(b, []byte(marker)) {
			t.Errorf("queue contains %s: %s", marker, b)
		}
	}
	e.mustAs(e.cons, http.MethodGet, "/admin/reports?reason=porn", nil, &page)
	if len(page.Items) != 1 || page.Items[0].ConvType != "group" {
		t.Errorf("by reason = %+v", page.Items)
	}
	e.mustAs(e.cons, http.MethodGet, "/admin/reports?limit=20&targetType=user", nil, &page)
	if len(page.Items) != 1 || page.NextCursor != "" {
		t.Errorf("by type = %+v", page)
	}
	code, b = e.as(e.cons, http.MethodGet, "/admin/reports?limit=30", nil)
	expectCode(t, "limit 30", code, b, http.StatusBadRequest, protocol.CodeBadRequest)
}

// TC-MOD-04-02 的 im-server 一侧：没有 report.read / report.handle 的调用方读队列、详情、结案都 403 / 4030，状态不变。
// TC-MOD-04-04（AC-MOD-04-4）：console 主凭证调 report.evidence 的两个接口 403 / 4030；console-msgread 成功。
func TestTC_MOD_04_04_Scopes(t *testing.T) {
	words := hmacCaller("operator", adminauth.ScopeModerationWords, adminauth.ScopeUserRead, adminauth.ScopeMsgMeta)
	e := newC6Env(t, func(c *app.Config) { c.AdminCallers = append(c.AdminCallers, words.cfg) })
	e.usersNamed("u1", "u2")
	e.createGroup("g1", "u1", "u2")
	u1, u2 := e.dial("u1", "web"), e.dial("u2", "web")
	m := mustAck[protocol.SendAck](t, sendWait(u1, "group", "g1", "hi"))
	id := mustReport(t, reportMsg(u2, m.ConvID, []string{m.MsgID}, "spam", nil))
	for _, c := range []struct{ method, path string }{{http.MethodGet, "/admin/reports"}, {http.MethodGet, "/admin/reports/" + id}, {http.MethodPost, "/admin/reports/" + id + "/resolve"}} {
		code, b := e.as(words, c.method, c.path, map[string]any{"result": "handled", "reason": "x"})
		expectCode(t, "operator "+c.path, code, b, http.StatusForbidden, protocol.CodeForbidden)
	}
	code, b := e.as(e.cons, http.MethodPost, "/admin/report-grants", map[string]any{"reportId": id, "reason": "r"})
	expectCode(t, "console grant", code, b, http.StatusForbidden, protocol.CodeForbidden)
	code, b = e.as(e.cons, http.MethodPost, "/admin/reports/"+id+"/evidence", map[string]any{"grantId": "x"})
	expectCode(t, "console evidence", code, b, http.StatusForbidden, protocol.CodeForbidden)
	g := e.mustGrant(id)
	if ev := e.evidence(id, g); len(ev.Messages) != 1 || ev.Messages[0].Body.Text != "hi" {
		t.Errorf("reader = %+v", ev)
	}
	// 授权换了操作人、换了举报、过期都是 403 / 4039。
	code, b = e.asOp(e.reader, "op2", http.MethodPost, "/admin/reports/"+id+"/evidence", map[string]any{"grantId": g})
	expectCode(t, "other operator", code, b, http.StatusForbidden, protocol.CodeMsgReadGrantRequired)
	m2 := mustAck[protocol.SendAck](t, sendWait(u1, "group", "g1", "hi2"))
	id2 := mustReport(t, reportMsg(u2, m2.ConvID, []string{m2.MsgID}, "spam", nil))
	code, b = e.as(e.reader, http.MethodPost, "/admin/reports/"+id2+"/evidence", map[string]any{"grantId": g})
	expectCode(t, "other report", code, b, http.StatusForbidden, protocol.CodeMsgReadGrantRequired)
	e.advance(16 * time.Minute)
	code, b = e.as(e.reader, http.MethodPost, "/admin/reports/"+id+"/evidence", map[string]any{"grantId": g})
	expectCode(t, "expired", code, b, http.StatusForbidden, protocol.CodeMsgReadGrantRequired)
	if e.count(`SELECT COUNT(*) FROM reports WHERE state = 'pending'`) != 2 {
		t.Error("state changed")
	}
}

// TC-MOD-04-03（AC-MOD-04-3）、TC-MOD-03-10 的读取部分（AC-MOD-03-10）：IM_C2C_BODY_ACCESS=off 时单聊消息举报和用户举报的元数据照常
// （举报人、被举报人、时间、理由、类型、长度），申请证据授权 403 / 4030 c2cBodyDisabled、不产生授权；同一个库用 on 重启之后授权和读取成功，
// 但 off 期间存的证据没有正文。
func TestTC_MOD_04_03_C2COff(t *testing.T) {
	e := newC6Env(t, nil)
	e.usersNamed("u1", "u2")
	e.mutual("u1", "u2")
	u1, u2 := e.dial("u1", "web"), e.dial("u2", "web")
	m := mustAck[protocol.SendAck](t, sendWait(u1, "c2c", "u2", "ZXQ-RPT-2 单聊"))
	id := mustReport(t, reportMsg(u2, m.ConvID, []string{m.MsgID}, "abuse", strp("ZXQ-MOD-NOTE")))
	uid := mustReport(t, reportCall(u2, map[string]any{"targetType": "user", "userId": "u1", "reason": "fraud"}))
	var detail struct {
		Reporter, TargetUser, Reason, ConvType string
		Messages                               []struct {
			MsgID, Type string
			BodyLength  int
		}
	}
	e.mustAs(e.cons, http.MethodGet, "/admin/reports/"+id, nil, &detail)
	if detail.Reporter != "u2" || detail.TargetUser != "u1" || detail.Reason != "abuse" || detail.ConvType != "c2c" || len(detail.Messages) != 1 ||
		detail.Messages[0].Type != "text" || detail.Messages[0].BodyLength != 12 {
		t.Errorf("metadata = %+v", detail)
	}
	for _, r := range []string{id, uid} {
		code, b := e.grant(r)
		if code != http.StatusForbidden || !strings.Contains(string(b), `"reason":"c2cBodyDisabled"`) {
			t.Errorf("grant %s: %d %s", r, code, b)
		}
	}
	if n := e.count(`SELECT COUNT(*) FROM msg_grants`); n != 0 {
		t.Errorf("grants created: %d", n)
	}
	on := newC6Env(t, func(c *app.Config) { c.DBPath = e.dbPath; c.C2CBodyAccess = true })
	ev := on.evidence(id, on.mustGrant(id))
	if len(ev.Messages) != 1 || ev.Messages[0].Body != nil || ev.Note == nil {
		t.Errorf("on after off = %+v", ev)
	}
}

// TC-MOD-04-05（AC-MOD-04-5）：两个审核员并发结案（一个 handled、一个 rejected），重复 20 次：每次只有一个成功，另一个 409 / 4098，
// 状态是成功那个的结论，之后不再变。
func TestTC_MOD_04_05_ConcurrentResolve(t *testing.T) {
	e := newC6Env(t, nil)
	e.usersNamed("u1", "u2")
	e.createGroup("g1", "u1", "u2")
	u1, u2 := e.dial("u1", "web"), e.dial("u2", "web")
	for i := range 20 {
		m := mustAck[protocol.SendAck](t, sendWait(u1, "group", "g1", fmt.Sprint("hi", i)))
		id := mustReport(t, reportMsg(u2, m.ConvID, []string{m.MsgID}, "spam", nil))
		var wg sync.WaitGroup
		codes := make([]int, 2)
		for k, result := range []string{"handled", "rejected"} {
			wg.Add(1)
			go func() {
				defer wg.Done()
				code, b := e.asOp(e.cons, fmt.Sprint("mod", k), http.MethodPost, "/admin/reports/"+id+"/resolve", map[string]any{"result": result, "reason": "并发"})
				if code == http.StatusConflict && !bytes.Contains(b, []byte(`"code":4098`)) {
					t.Errorf("409 without 4098: %s", b)
				}
				codes[k] = code
			}()
		}
		wg.Wait()
		ok := 0
		for _, c := range codes {
			if c == http.StatusOK {
				ok++
			} else if c != http.StatusConflict {
				t.Errorf("round %d: code %d", i, c)
			}
		}
		if ok != 1 {
			t.Fatalf("round %d: %v", i, codes)
		}
		var state, by string
		e.db().QueryRow(`SELECT state, handled_by FROM reports WHERE report_id = ?`, id).Scan(&state, &by)
		winner := 0
		if codes[1] == http.StatusOK {
			winner = 1
		}
		if want := []string{"handled", "rejected"}[winner]; state != want || by != fmt.Sprint("console:mod", winner) {
			t.Errorf("round %d: state %s by %s, want %s", i, state, by, want)
		}
		code, _ := e.resolve(e.cons, id, "handled")
		if code != http.StatusConflict {
			t.Errorf("round %d: third resolve %d", i, code)
		}
	}
}

// TC-MOD-04-07（AC-MOD-04-7）：结案（handled、rejected 各一次）之后举报人和被举报人都只收到之后的哨兵（第一批不发处理结果，规则 6）。
func TestTC_MOD_04_07_NoFrames(t *testing.T) {
	e := newC6Env(t, nil)
	e.usersNamed("u1", "u2", "s")
	u1, u2, s := e.dial("u1", "web"), e.dial("u2", "web"), e.dial("s", "web")
	var ids []string
	for range 2 {
		m := mustAck[protocol.SendAck](t, sendWait(u1, "c2c", "u2", "hi"))
		ids = append(ids, mustReport(t, reportMsg(u2, m.ConvID, []string{m.MsgID}, "spam", nil)))
	}
	u1.drain()
	u2.drain()
	for i, result := range []string{"handled", "rejected"} {
		if code, b := e.resolve(e.cons, ids[i], result); code != http.StatusOK {
			t.Fatalf("resolve: %d %s", code, b)
		}
	}
	onlySentinel(t, u1, func() protocol.SendAck { return mustAck[protocol.SendAck](t, sendWait(s, "c2c", "u1", "哨兵")) })
	onlySentinel(t, u2, func() protocol.SendAck { return mustAck[protocol.SendAck](t, sendWait(s, "c2c", "u2", "哨兵")) })
}

// TC-MOD-04-10 / 04-11（AC-MOD-04-10 第 3 种、AC-MOD-04-11，im-server 一侧）：举报详情和 msg.meta 的元数据接口都搜不到 note 和文件名；
// 读证据（授权之后）才有。TC-MOD-04-16（AC-MOD-04-16）：被举报的群文件 fetch：不带 grantId、带别的举报的授权、别的操作人的授权、过期的授权都
// 403 / 4039；有效的举报授权取缩略图和原图都成功，响应头 attachment、nosniff、no-store。TC-MOD-03-15 第 3 步：原消息过了保留期之后证据文件仍可取。
func TestTC_MOD_04_16_EvidenceFiles(t *testing.T) {
	e := newC6FileEnv(t, nil)
	tok := e.token("u1")
	e.token("u2")
	e.token("u3")
	e.createGroup("g1", "u1", "u2", "u3")
	orig := fakeJPEG(800, 600, 32<<10)
	img := e.uploadImage(tok, "ZXQ-MOD-FILE.jpg", orig, smallThumb())
	other := e.uploadFile(tok, "ZXQ-MOD-FILE.pdf", "application/pdf", pad([]byte("%PDF-1.4"), 2048))
	u1, u2 := e.dial("u1", "web"), e.dial("u2", "web")
	m := mustAck[protocol.SendAck](t, sendFile(u1, "group", "g1", "ev-img-01", "image", img, nil))
	m2 := mustAck[protocol.SendAck](t, sendFile(u1, "group", "g1", "ev-doc-01", "file", other, nil))
	e.advanceTo(e.now.Add(10 * 24 * time.Hour)) // 第 10 天举报（TC-MOD-03-15）
	r1 := mustReport(t, reportMsg(u2, m.ConvID, []string{m.MsgID}, "porn", strp("ZXQ-MOD-NOTE")))
	r2 := mustReport(t, reportMsg(u2, m2.ConvID, []string{m2.MsgID}, "spam", nil))
	_, detail := e.as(e.cons, http.MethodGet, "/admin/reports/"+r1, nil)
	_, detail2 := e.as(e.cons, http.MethodGet, "/admin/reports/"+r2, nil)
	_, meta := e.as(e.cons, http.MethodPost, "/admin/convs/messages/query", map[string]any{"convId": m.ConvID})
	if !bytes.Contains(detail2, []byte(`"type":"file"`)) || !bytes.Contains(detail2, []byte(`"fileSize":2048`)) {
		t.Errorf("file report detail lacks the metadata: %s", detail2)
	}
	for label, b := range map[string][]byte{"detail": detail, "detail of the file report": detail2, "meta": meta} {
		if bytes.Contains(b, []byte("ZXQ-MOD-FILE")) || bytes.Contains(b, []byte("ZXQ-MOD-NOTE")) {
			t.Errorf("%s has the file name or note: %s", label, b)
		}
	}
	grant := func(op, rid string) string {
		code, b := e.asOp(e.reader, op, http.MethodPost, "/admin/report-grants", map[string]any{"reportId": rid, "reason": "核查"})
		var g struct{ GrantID string }
		if code != http.StatusOK || json.Unmarshal(b, &g) != nil {
			t.Fatalf("grant: %d %s", code, b)
		}
		return g.GrantID
	}
	fetch := func(op, grantID, variant string) (int, http.Header, []byte) {
		body := map[string]any{"convId": m.ConvID, "msgId": m.MsgID, "variant": variant}
		if grantID != "" {
			body["grantId"] = grantID
		}
		req := e.signedRequest(e.reader, http.MethodPost, "/admin/files/"+img+"/fetch", body, signOpts{ts: e.now, operator: op})
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, resp.Header, b
	}
	g1, g2, gOther := grant("op1", r1), grant("op1", r2), grant("op2", r1)
	_, evb := e.as(e.reader, http.MethodPost, "/admin/reports/"+r1+"/evidence", map[string]any{"grantId": g1})
	if !bytes.Contains(evb, []byte("ZXQ-MOD-NOTE")) || !bytes.Contains(evb, []byte(img)) {
		t.Errorf("evidence lacks the note or the file: %s", evb)
	}
	_, evb = e.as(e.reader, http.MethodPost, "/admin/reports/"+r2+"/evidence", map[string]any{"grantId": g2})
	if !bytes.Contains(evb, []byte("ZXQ-MOD-FILE.pdf")) { // 图片没有文件名字段，文件的名字在证据里
		t.Errorf("evidence lacks the file name: %s", evb)
	}
	for label, c := range map[string]struct{ op, grant string }{"no grant": {"op1", ""}, "other report": {"op1", g2}, "other operator": {"op1", gOther}} {
		code, _, b := fetch(c.op, c.grant, "original")
		expectCode(t, label, code, b, http.StatusForbidden, protocol.CodeMsgReadGrantRequired)
	}
	for _, v := range []string{"original", "thumb"} {
		code, h, b := fetch("op1", g1, v)
		if code != http.StatusOK || h.Get("Content-Disposition") != "attachment" || h.Get("X-Content-Type-Options") != "nosniff" || h.Get("Cache-Control") != "no-store" || len(b) == 0 {
			t.Errorf("%s: %d %v", v, code, h)
		}
		if v == "original" && !bytes.Equal(b, orig) {
			t.Error("original bytes differ")
		}
	}
	// 原消息过了保留期（发送后 181 天，文件 expired，名字清空）：证据文件仍可取（按 evidence_until 保留）。
	e.advanceTo(e.now.Add(171 * 24 * time.Hour))
	e.runPurge()
	g3 := grant("op1", r1)
	if code, _, b := fetch("op1", g3, "original"); code != http.StatusOK || !bytes.Equal(b, orig) {
		t.Errorf("after the retention period: %d", code)
	}
	e.advanceTo(e.now.Add(16 * time.Minute))
	code, _, b := fetch("op1", g3, "original")
	expectCode(t, "expired grant", code, b, http.StatusForbidden, protocol.CodeMsgReadGrantRequired)
}

// TC-MOD-04-13（AC-MOD-04-13）：IM_C2C_BODY_ACCESS=off 时单聊文件证据：带任意举报授权 fetch 都 403 / 4030 c2cBodyDisabled；
// 文件的 evidence_until 没有延长（off 时不存文件引用，第 95 条 ①）。
func TestTC_MOD_04_13_C2CFileOff(t *testing.T) {
	e := newC6FileEnv(t, nil)
	tok := e.token("u1")
	e.token("u2")
	e.createGroup("g1", "u1", "u2")
	doc := e.uploadFile(tok, "ZXQ-MOD-FILE.pdf", "application/pdf", pad([]byte("%PDF-1.4"), 2048))
	gdoc := e.uploadFile(tok, "g.pdf", "application/pdf", pad([]byte("%PDF-1.4"), 2048))
	u1, u2 := e.dial("u1", "web"), e.dial("u2", "web")
	m := mustAck[protocol.SendAck](t, sendFile(u1, "c2c", "u2", "c2c-doc-01", "file", doc, nil))
	gm := mustAck[protocol.SendAck](t, sendFile(u1, "group", "g1", "g-doc-01", "file", gdoc, nil))
	mustReport(t, reportMsg(u2, m.ConvID, []string{m.MsgID}, "spam", nil))
	gr := mustReport(t, reportMsg(u2, gm.ConvID, []string{gm.MsgID}, "spam", nil))
	if f := e.file(doc); f.EvidenceUntil != 0 {
		t.Errorf("c2c file evidence_until = %d", f.EvidenceUntil)
	}
	code, b := e.as(e.reader, http.MethodPost, "/admin/report-grants", map[string]any{"reportId": gr, "reason": "核查"})
	var g struct{ GrantID string }
	json.Unmarshal(b, &g)
	if code != http.StatusOK {
		t.Fatalf("grant: %d %s", code, b)
	}
	code, b = e.as(e.reader, http.MethodPost, "/admin/files/"+doc+"/fetch", map[string]any{"convId": m.ConvID, "msgId": m.MsgID, "grantId": g.GrantID})
	if code != http.StatusForbidden || !bytes.Contains(b, []byte(`"reason":"c2cBodyDisabled"`)) {
		t.Errorf("c2c fetch: %d %s", code, b)
	}
}

// TC-MOD-04-14（AC-MOD-04-14 的 im-server 一侧）：换一把 IM_REPORT_KEY 重启（同一个库），读证据返回 {messages: [], undecryptable: true}，
// im_report_undecryptable_total 加 1，详情只有 msgId；结案照常成功。im-server 审计写失败时结案 503 / 5030、状态不变（console 的 5031 随 CS-207）。
func TestTC_MOD_04_14_WrongKeyAndAuditFailure(t *testing.T) {
	e := newC6Env(t, nil)
	e.usersNamed("u1", "u2")
	e.createGroup("g1", "u1", "u2")
	u1, u2 := e.dial("u1", "web"), e.dial("u2", "web")
	m := mustAck[protocol.SendAck](t, sendWait(u1, "group", "g1", "hi"))
	id := mustReport(t, reportMsg(u2, m.ConvID, []string{m.MsgID}, "spam", nil))
	k2 := newC6Env(t, func(c *app.Config) { c.DBPath = e.dbPath; c.ReportKey = []byte(testReportKey2) })
	ev := k2.evidence(id, k2.mustGrant(id))
	if !ev.Undecryptable || len(ev.Messages) != 0 {
		t.Errorf("wrong key = %+v", ev)
	}
	if v := metricValue(t, k2.app.MetricsHandler(), "im_report_undecryptable_total"); v != 1 {
		t.Errorf("undecryptable metric = %v", v)
	}
	var detail struct{ Messages []map[string]any }
	k2.mustAs(k2.cons, http.MethodGet, "/admin/reports/"+id, nil, &detail)
	if len(detail.Messages) != 1 || len(detail.Messages[0]) != 1 {
		t.Errorf("detail with the wrong key = %+v", detail)
	}
	m3 := mustAck[protocol.SendAck](t, sendWait(u1, "group", "g1", "hi again"))
	id3 := mustReport(t, reportMsg(u2, m3.ConvID, []string{m3.MsgID}, "spam", nil))
	k2.faults(map[string]any{"adminAuditWrite": "fail"})
	code, b := k2.resolve(k2.cons, id3, "handled")
	expectCode(t, "audit failure", code, b, http.StatusServiceUnavailable, protocol.CodeServiceUnavailable)
	k2.faults(map[string]any{"adminAuditWrite": ""})
	if code, b := k2.resolve(k2.cons, id, "rejected"); code != http.StatusOK {
		t.Errorf("resolve with the wrong key: %d %s", code, b)
	}
	if n := e.count(`SELECT COUNT(*) FROM reports WHERE report_id = ? AND state = 'pending'`, id3); n != 1 {
		t.Error("resolved although the audit failed")
	}
}

// TC-MOD-04-17（AC-MOD-04-17）：[IM 钟] 推进 179 天跑 account_events_purge 登录事件还在，181 天后被清除（C1 已有的任务，C6 补用例）。
func TestTC_MOD_04_17_AccountEventsPurge(t *testing.T) {
	e := newAcctEnv(t, nil)
	t0 := shanghaiMidnight(t).Add(time.Hour)
	e.setClock(t0)
	e.newInvite("INVITE-EVT-0001", 1, t0.Add(24*time.Hour))
	e.register("evtuser", map[string]any{"inviteCode": "INVITE-EVT-0001"})
	e.mustLogin("evtuser", goodPassword, testPlatform)
	before := e.count(`SELECT COUNT(*) FROM account_events WHERE user_id = 'evtuser'`)
	if before == 0 {
		t.Fatal("no account events")
	}
	run := func(at time.Time) {
		e.setClock(at)
		if code, b := e.adminPost("/admin/test/jobs/account_events_purge/run", map[string]any{}, adminKey); code != http.StatusOK {
			t.Fatalf("purge: %d %s", code, b)
		}
	}
	run(t0.Add(179 * 24 * time.Hour))
	if n := e.count(`SELECT COUNT(*) FROM account_events WHERE user_id = 'evtuser'`); n != before {
		t.Fatalf("179 days: %d -> %d", before, n)
	}
	run(t0.Add(181 * 24 * time.Hour))
	if n := e.count(`SELECT COUNT(*) FROM account_events WHERE user_id = 'evtuser'`); n != 0 {
		t.Errorf("181 days: %d left", n)
	}
}

// TC-CHAT-90-47 的 im_reports_pending（F-MOD-04 规则 7，第 107 条）：一条 25 小时前的待处理举报是 1，结案后是 0；刚提交的不算。
func TestModReportsPendingMetric(t *testing.T) {
	e := newC6Env(t, nil)
	e.usersNamed("u1", "u2")
	e.createGroup("g1", "u1", "u2")
	u1, u2 := e.dial("u1", "web"), e.dial("u2", "web")
	m := mustAck[protocol.SendAck](t, sendWait(u1, "group", "g1", "hi"))
	id := mustReport(t, reportMsg(u2, m.ConvID, []string{m.MsgID}, "spam", nil))
	if v := metricValue(t, e.app.MetricsHandler(), "im_reports_pending"); v != 0 {
		t.Errorf("fresh = %v", v)
	}
	e.advance(25 * time.Hour)
	m2 := mustAck[protocol.SendAck](t, sendWait(u1, "group", "g1", "hi2"))
	mustReport(t, reportMsg(u2, m2.ConvID, []string{m2.MsgID}, "spam", nil))
	if v := metricValue(t, e.app.MetricsHandler(), "im_reports_pending"); v != 1 {
		t.Errorf("25h = %v", v)
	}
	if code, b := e.resolve(e.cons, id, "rejected"); code != http.StatusOK {
		t.Fatalf("resolve: %d %s", code, b)
	}
	if v := metricValue(t, e.app.MetricsHandler(), "im_reports_pending"); v != 0 {
		t.Errorf("after resolve = %v", v)
	}
}
