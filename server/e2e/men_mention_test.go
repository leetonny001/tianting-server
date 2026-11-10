package e2e

// C4 @ 和提及计数（BE-476、BE-477）：TC-MEN 里【后端】的用例。环境 E1（c4Env）。本地结果，CI 不运行。
// AC-MEN-02-3（2000 人、在线 200、1000 次 @ 所有人的 ack P99）是性能用例，开发机只记录，判定等 P-12（BE-480），这里不跑。

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"tianting/server/internal/app"
	"tianting/server/internal/protocol"
)

// ownedGroup 用管理接口建一个有群主的托管群（seq 从 1 开始，没有提示消息）。
func (e *c4Env) ownedGroup(gid, owner string, members ...string) {
	e.t.Helper()
	all := append([]string{owner}, members...)
	e.usersNamed(all...)
	code, b := e.adm(http.MethodPost, "/admin/groups", map[string]any{"groupId": gid, "name": gid, "members": all, "owner": owner})
	if code != http.StatusCreated {
		e.t.Fatalf("create group: %d %s", code, b)
	}
}

func mentionsOf(t *testing.T, m protocol.Message) protocol.Mentions {
	t.Helper()
	if m.Body.Mentions == nil {
		t.Fatalf("message %d has no mentions: %+v", m.Seq, m.Body)
	}
	return *m.Body.Mentions
}

// TC-MEN-01-01（AC-MEN-01-1 的服务端部分）、TC-MEN-01-02（AC-MEN-01-2）：u1 @ u2：u2 收到的消息带 mentions.userIds=[u2]，
// mentionUnread=1；u3 是 0。含非成员 u9 时成功、只保留 u2（静默过滤）。
func TestTC_MEN_01_01_MentionMember(t *testing.T) {
	e := newC4Env(t, nil)
	e.ownedGroup("g1", "o", "u1", "u2", "u3")
	e.usersNamed("u9")
	u1, u2, u3 := e.dial("u1", "web"), e.dial("u2", "web"), e.dial("u3", "web")
	ack := mustAck[protocol.SendAck](t, sendMentions(u1, "g1", "cmid-m0101", "@小王 看一下", []string{"u9", "u2"}, false))
	got := u2.waitCmd(protocol.CmdMsg).msg(t)
	if m := mentionsOf(t, got); len(m.UserIDs) != 1 || m.UserIDs[0] != "u2" || m.All {
		t.Errorf("mentions = %+v", m)
	}
	if m := mentionsOf(t, msgBySeq(t, syncConv(t, u3, "group:g1").Messages, ack.Seq)); len(m.UserIDs) != 1 || m.UserIDs[0] != "u2" {
		t.Errorf("synced mentions = %+v", m)
	}
	if n, first := mentionUnread(t, u2, "group:g1"); n != 1 || first != ack.Seq {
		t.Errorf("u2 mentionUnread=%d first=%d", n, first)
	}
	if n, first := mentionUnread(t, u3, "group:g1"); n != 0 || first != 0 {
		t.Errorf("u3 mentionUnread=%d first=%d", n, first)
	}
	if n, _ := mentionUnread(t, u1, "group:g1"); n != 0 {
		t.Errorf("sender mentionUnread=%d", n)
	}
}

// TC-MEN-01-03（AC-MEN-01-3）：21 个 userId 4131；单聊带 mentions 4000；type=image 带 mentions 4000；三次都没有保存。
func TestTC_MEN_01_03_Limits(t *testing.T) {
	e := newC4Env(t, nil)
	var members []string
	for i := range 25 {
		members = append(members, fmt.Sprintf("p%02d", i))
	}
	e.ownedGroup("g1", "o", members...)
	o := e.dial("o", "web")
	before := convByID(t, o, "group:g1").MaxSeq
	mustErr(t, sendMentions(o, "g1", "cmid-m0301", "@21", members[:21], false), protocol.CodeBatchLimitExceeded)
	mustAck[protocol.SendAck](t, sendMentions(o, "g1", "cmid-m0302", "@20", members[:20], false)) // 20 个可以
	e.mutual("o", "p00")
	mustErr(t, sendBody(o, "c2c", "p00", "cmid-m0303", map[string]any{"text": "@p00", "mentions": map[string]any{"userIds": []string{"p00"}}}), protocol.CodeBadRequest)
	mustErr(t, o.call(protocol.CmdSend, map[string]any{"convType": "group", "to": "g1", "clientMsgId": "cmid-m0304", "type": "image",
		"body": map[string]any{"image": map[string]any{"fileId": "f1"}, "mentions": map[string]any{"userIds": []string{"p01"}}}}), protocol.CodeBadRequest)
	if got := convByID(t, o, "group:g1").MaxSeq; got != before+1 {
		t.Errorf("maxSeq %d -> %d (only the 20-mention message is saved)", before, got)
	}
}

// TC-MEN-01-04（AC-MEN-01-4）：正文写 @u2、没有 mentions：u2 不被标记提及，conv_mentions 没有行（服务端不解析正文）。
func TestTC_MEN_01_04_TextIsNotParsed(t *testing.T) {
	e := newC4Env(t, nil)
	e.ownedGroup("g1", "u1", "u2", "u3")
	u1, u2 := e.dial("u1", "web"), e.dial("u2", "web")
	mustAck[protocol.SendAck](t, u1.sendText("group", "g1", "cmid-m0401", "@u2 看一下"))
	if n, _ := mentionUnread(t, u2, "group:g1"); n != 0 {
		t.Errorf("mentionUnread = %d", n)
	}
	var rows int
	e.db().QueryRow(`SELECT count(*) FROM conv_mentions`).Scan(&rows)
	if rows != 0 {
		t.Errorf("conv_mentions rows = %d", rows)
	}
}

// TC-MEN-01-11（AC-MEN-01-11）：2000 人群 groupMembers {keyword:"zh"} 只返回 userId 或昵称以 zh 开头的；mentions 里同一 userId 重复 3 次被去重；
// 1 分钟内第 31 次搜索 4290（不带 keyword 的分页不受影响）。
func TestTC_MEN_01_11_KeywordAndDedup(t *testing.T) {
	e := newC4Env(t, func(c *app.Config) { c.MsgActions.MemberSearchRatePerMin = 0 }) // 默认每分钟 30 次
	users := []map[string]any{{"userId": "o", "nickname": "群主"}, {"userId": "u2", "nickname": "u2"},
		{"userId": "zhang1", "nickname": "张三"}, {"userId": "zhao2", "nickname": "赵四"}, {"userId": "x1", "nickname": "Zhou"},
		{"userId": "x2", "nickname": "zhu"}, {"userId": "Zh3", "nickname": "大写开头的 userId"}, {"userId": "x3", "nickname": "azh"}}
	all := []string{"o", "u2", "zhang1", "zhao2", "x1", "x2", "Zh3", "x3"}
	for i := len(all); i < 2000; i++ {
		id := fmt.Sprintf("m%04d", i)
		users = append(users, map[string]any{"userId": id, "nickname": "成员" + id})
		all = append(all, id)
	}
	for i := 0; i < len(users); i += 500 {
		if code, b := e.adminPost("/admin/users/import", map[string]any{"users": users[i:min(i+500, len(users))]}, adminKey); code != http.StatusOK {
			t.Fatalf("import: %d %s", code, b)
		}
	}
	code, b := e.adm(http.MethodPost, "/admin/groups", map[string]any{"groupId": "big", "name": "big", "members": all, "owner": "o"})
	if code != http.StatusCreated {
		t.Fatalf("create: %d %s", code, b)
	}
	o := e.dial("o", "web")
	page := mustAck[protocol.GroupMembersAck](t, o.call(protocol.CmdGroupMembers, map[string]any{"groupId": "big", "keyword": "zh", "limit": 100}))
	var got []string
	for _, m := range page.Members {
		got = append(got, m.UserID)
	}
	if strings.Join(got, ",") != "zhang1,zhao2,x1,x2" && !sameSet(got, []string{"zhang1", "zhao2", "x1", "x2"}) {
		t.Errorf("keyword zh = %v (userId prefix is case sensitive, nickname prefix is not)", got)
	}
	ack := mustAck[protocol.SendAck](t, sendMentions(o, "big", "cmid-m1101", "@u2", []string{"u2", "u2", "u2"}, false))
	if m := mentionsOf(t, msgBySeq(t, syncConv(t, o, "group:big").Messages, ack.Seq)); len(m.UserIDs) != 1 || m.UserIDs[0] != "u2" {
		t.Errorf("deduplicated mentions = %+v", m)
	}
	for i := 1; i < 30; i++ {
		mustAck[protocol.GroupMembersAck](t, o.call(protocol.CmdGroupMembers, map[string]any{"groupId": "big", "keyword": "zh"}))
	}
	e.advance(time.Second) // 每秒 10 次的通用限速恢复，每分钟 30 次的搜索限速还不够 1 次
	mustErr(t, o.call(protocol.CmdGroupMembers, map[string]any{"groupId": "big", "keyword": "zh"}), protocol.CodeRateLimited)
	mustAck[protocol.GroupMembersAck](t, o.call(protocol.CmdGroupMembers, map[string]any{"groupId": "big"}))
}

func sameSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	m := map[string]int{}
	for _, x := range a {
		m[x]++
	}
	for _, x := range b {
		m[x]--
	}
	for _, v := range m {
		if v != 0 {
			return false
		}
	}
	return true
}

// TC-MEN-02-01（AC-MEN-02-1）：成员 @ 所有人 4030 mentionAllForbidden、消息未保存；管理员、群主成功。
func TestTC_MEN_02_01_MentionAllRoles(t *testing.T) {
	e := newC4Env(t, nil)
	gid, cs := e.selfGroup("o", []string{"a"}, "m")
	before := convByID(t, cs["o"], "group:"+gid).MaxSeq
	f := sendMentions(cs["m"], gid, "cmid-m0201", "@所有人", nil, true)
	if f.Error == nil || f.Error.Code != protocol.CodeForbidden || f.Error.Reason != protocol.ReasonMentionAllForbidden {
		t.Fatalf("member: %+v", f.Error)
	}
	if got := convByID(t, cs["o"], "group:"+gid).MaxSeq; got != before {
		t.Errorf("rejected message saved: %d -> %d", before, got)
	}
	for _, u := range []string{"a", "o"} {
		ack := mustAck[protocol.SendAck](t, sendMentions(cs[u], gid, "cmid-m0202-"+u, "@所有人", nil, true))
		if m := mentionsOf(t, msgBySeq(t, syncConv(t, cs["m"], "group:"+gid).Messages, ack.Seq)); !m.All || len(m.UserIDs) != 0 {
			t.Errorf("%s: mentions = %+v", u, m)
		}
	}
}

// TC-MEN-02-02（AC-MEN-02-2）：Asia/Shanghai 当天 00:00 起，1 小时内第 4 次 4293（retryAfterSeconds > 0）；1 小时后成功；
// 每次间隔 61 分钟到当天第 11 次 4293；次日 00:00 成功。小时窗口是滚动 60 分钟（backend-chat 第 58 条）。
func TestTC_MEN_02_02_MentionAllQuota(t *testing.T) {
	e := newC4Env(t, nil)
	day := time.UnixMilli(t0).In(time.FixedZone("Asia/Shanghai", 8*3600))
	midnight := time.Date(day.Year(), day.Month(), day.Day()+1, 0, 0, 0, 0, day.Location())
	e.setClock(midnight.UnixMilli())
	gid, cs := e.selfGroup("o", nil, "m", "n")
	o := cs["o"]
	n := 0
	send := func() frame {
		n++
		return sendMentions(o, gid, fmt.Sprintf("cmid-m0202-%02d", n), "@所有人", nil, true)
	}
	for i := range 3 {
		mustAck[protocol.SendAck](t, send())
		if i < 2 {
			e.advance(3 * time.Minute)
		}
	}
	e.advance(3 * time.Minute) // 00:09
	f := send()
	mustErr(t, f, protocol.CodeMentionAllLimited)
	if r, _ := f.Error.Details["retryAfterSeconds"].(float64); r <= 0 || r > 3600 {
		t.Errorf("retryAfterSeconds = %v", f.Error.Details)
	}
	e.advance(time.Hour) // 01:09：00:00、00:03、00:06 都出了窗口
	mustAck[protocol.SendAck](t, send())
	ok := 4
	for ok < 10 {
		e.advance(61 * time.Minute)
		mustAck[protocol.SendAck](t, send())
		ok++
	}
	e.advance(61 * time.Minute)
	f = send()
	mustErr(t, f, protocol.CodeMentionAllLimited) // 当天第 11 次
	if r, _ := f.Error.Details["retryAfterSeconds"].(float64); time.Duration(r)*time.Second > 24*time.Hour || r <= 0 {
		t.Errorf("daily retryAfterSeconds = %v", f.Error.Details)
	}
	e.setClock(midnight.Add(24 * time.Hour).UnixMilli())
	mustAck[protocol.SendAck](t, send())
}

// TC-MEN-02-06（AC-MEN-02-6）：500 人群 @ 所有人一次：conv_mentions 没有这条的行，group_mention_all 一行。
func TestTC_MEN_02_06_NotExpanded(t *testing.T) {
	e := newC4Env(t, nil)
	var members []string
	for i := range 499 {
		members = append(members, fmt.Sprintf("p%03d", i))
	}
	e.ownedGroup("g1", "o", members...)
	o := e.dial("o", "web")
	ack := mustAck[protocol.SendAck](t, sendMentions(o, "g1", "cmid-m0601", "@所有人", nil, true))
	db := e.db()
	var cm, all int
	db.QueryRow(`SELECT count(*) FROM conv_mentions WHERE conv_id = 'group:g1' AND seq = ?`, ack.Seq).Scan(&cm)
	db.QueryRow(`SELECT count(*) FROM group_mention_all WHERE group_id = 'g1' AND seq = ?`, ack.Seq).Scan(&all)
	if cm != 0 || all != 1 {
		t.Errorf("conv_mentions=%d group_mention_all=%d", cm, all)
	}
	p := e.dial("p042", "web")
	if n, first := mentionUnread(t, p, "group:g1"); n != 1 || first != ack.Seq {
		t.Errorf("member mentionUnread=%d first=%d", n, first)
	}
}

// TC-MEN-02-07（AC-MEN-02-7）：IM_V13_ADMIN_FEATURES=1；托管群没有群主时成员 @ 所有人 403 / 4030；全群禁言中群主 @ 所有人成功。
func TestTC_MEN_02_07_ManagedAndAllMuted(t *testing.T) {
	e := newC4Env(t, nil)
	e.usersNamed("p", "q")
	e.createGroup("h1", "p", "q")
	p := e.dial("p", "web")
	f := sendMentions(p, "h1", "cmid-m0701", "@所有人", nil, true)
	if f.Error == nil || f.Error.Code != protocol.CodeForbidden || f.Error.Reason != protocol.ReasonMentionAllForbidden {
		t.Errorf("managed group: %+v", f.Error)
	}
	// 群主设了管理员 q 之后被收回：没有群主时管理员也不能（F-MEN-02 规则 1）。
	e.mustAdm(http.MethodPatch, "/admin/groups/h1", map[string]any{"owner": "p"})
	mustAck[protocol.Empty](t, p.call(protocol.CmdGroupSetAdmin, map[string]any{"groupId": "h1", "userId": "q", "admin": true}))
	mustAck[protocol.SendAck](t, sendMentions(p, "h1", "cmid-m0704", "@所有人（有群主）", nil, true))
	e.mustAdm(http.MethodPatch, "/admin/groups/h1", map[string]any{"owner": nil})
	q := e.dial("q", "web")
	if f := sendMentions(q, "h1", "cmid-m0705", "@所有人", nil, true); f.Error == nil || f.Error.Reason != protocol.ReasonMentionAllForbidden {
		t.Errorf("admin in an ownerless group: %+v", f.Error)
	}
	gid, cs := e.selfGroup("o", nil, "m", "n")
	e.must("/admin/mute", map[string]any{"scope": "groupAll", "groupId": gid, "reason": "quiet"})
	mustErr(t, cs["m"].sendText("group", gid, "cmid-m0702", "muted member"), protocol.CodeMuted)
	mustAck[protocol.SendAck](t, sendMentions(cs["o"], gid, "cmid-m0703", "@所有人 开会", nil, true))
}

// TC-MEN-02-08（AC-MEN-02-8 的 C4 部分）：发 3 次 @ 所有人再撤回 3 次，第 4 次仍 4293（撤回不退）；计数器故障注入后 503 / 5030，消息未保存。
func TestTC_MEN_02_08_RecallNoRefundAndCounterFault(t *testing.T) {
	e := newC4Env(t, nil)
	gid, cs := e.selfGroup("o", nil, "m", "n")
	conv := "group:" + gid
	for i := range 3 {
		ack := mustAck[protocol.SendAck](t, sendMentions(cs["o"], gid, fmt.Sprintf("cmid-m0801-%d", i), "@所有人", nil, true))
		mustRecall(t, cs["o"], conv, ack.MsgID)
	}
	mustErr(t, sendMentions(cs["o"], gid, "cmid-m0802", "@所有人", nil, true), protocol.CodeMentionAllLimited)
	g2, c2 := e.selfGroup("o2", nil, "x", "y")
	if code, b := e.adminPost("/admin/test/faults", map[string]any{"mentionAllCounter": true}, adminKey); code != http.StatusOK {
		t.Fatalf("faults: %d %s", code, b)
	}
	before := convByID(t, c2["o2"], "group:"+g2).MaxSeq
	mustErr(t, sendMentions(c2["o2"], g2, "cmid-m0803", "@所有人", nil, true), protocol.CodeServiceUnavailable)
	if got := convByID(t, c2["o2"], "group:"+g2).MaxSeq; got != before {
		t.Errorf("message saved despite the counter fault: %d -> %d", before, got)
	}
	mustAck[protocol.SendAck](t, c2["o2"].sendText("group", g2, "cmid-m0804", "plain text is not affected"))
	e.adminPost("/admin/test/faults", map[string]any{"mentionAllCounter": false}, adminKey)
	mustAck[protocol.SendAck](t, sendMentions(c2["o2"], g2, "cmid-m0805", "@所有人", nil, true))
}

// TC-MEN-02-09（AC-MEN-02-9）：同一群并发 10 条 @ 所有人（o 的 10 个连接）：恰好 3 条成功，7 条 4293；group_mention_all 3 行。
func TestTC_MEN_02_09_Concurrent(t *testing.T) {
	e := newC4Env(t, func(c *app.Config) { c.Gateway.MaxConnsPerUser = 20 })
	gid, _ := e.selfGroup("o", nil, "m", "n")
	var conns []*client
	for range 10 {
		conns = append(conns, e.dial("o", "web"))
	}
	var wg sync.WaitGroup
	results := make([]frame, len(conns))
	for i, c := range conns {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results[i] = sendMentions(c, gid, fmt.Sprintf("cmid-m0901-%02d", i), "@所有人", nil, true)
		}()
	}
	wg.Wait()
	ok, limited := 0, 0
	for _, f := range results {
		switch {
		case f.Error == nil:
			ok++
		case f.Error.Code == protocol.CodeMentionAllLimited:
			limited++
		default:
			t.Errorf("unexpected: %+v", f.Error)
		}
	}
	var rows int
	e.db().QueryRow(`SELECT count(*) FROM group_mention_all WHERE group_id = ?`, gid).Scan(&rows)
	if ok != 3 || limited != 7 || rows != 3 {
		t.Errorf("ok=%d limited=%d rows=%d", ok, limited, rows)
	}
}

// TC-MEN-03-01（AC-MEN-03-1）：u2 未读里第 5、7 条 @ 他，第 9 条 @ 所有人：mentionUnread=3、firstMentionSeq 是第 5 条；已读到第 5 条后 2、第 7 条。
// 建群时的提示消息占了 seq 1，所以这里的 seq 是 base+5、base+7、base+9。mention_purge 清理已读之前的行，结果不变。
func TestTC_MEN_03_01_Counts(t *testing.T) {
	e := newC4Env(t, nil)
	e.ownedGroup("g1", "u1", "u2", "u3")
	u1, u2 := e.dial("u1", "web"), e.dial("u2", "web")
	base := convByID(t, u2, "group:g1").MaxSeq
	for seq := int64(1); seq <= 9; seq++ {
		cid := fmt.Sprintf("cmid-m3101-%d", seq)
		var f frame
		switch seq {
		case 5, 7:
			f = sendMentions(u1, "g1", cid, "@u2", []string{"u2"}, false)
		case 9:
			f = sendMentions(u1, "g1", cid, "@所有人", nil, true)
		default:
			f = u1.sendText("group", "g1", cid, "x")
		}
		if ack := mustAck[protocol.SendAck](t, f); ack.Seq != base+seq {
			t.Fatalf("seq %d, want %d", ack.Seq, base+seq)
		}
	}
	if n, first := mentionUnread(t, u2, "group:g1"); n != 3 || first != base+5 {
		t.Errorf("before read: %d %d", n, first)
	}
	mustAck[protocol.Empty](t, u2.call(protocol.CmdRead, map[string]any{"convId": "group:g1", "seq": base + 5}))
	if n, first := mentionUnread(t, u2, "group:g1"); n != 2 || first != base+7 {
		t.Errorf("after reading 5: %d %d", n, first)
	}
	if code, b := e.adminPost("/admin/test/jobs/mention_purge/run", map[string]any{}, adminKey); code != http.StatusOK {
		t.Fatalf("mention_purge: %d %s", code, b)
	}
	var rows int
	e.db().QueryRow(`SELECT count(*) FROM conv_mentions WHERE user_id = 'u2' AND seq <= ?`, base+5).Scan(&rows)
	if rows != 0 {
		t.Errorf("rows before the read position after purge = %d", rows)
	}
	if n, first := mentionUnread(t, u2, "group:g1"); n != 2 || first != base+7 {
		t.Errorf("after purge: %d %d", n, first)
	}
}

// TC-MEN-03-04（AC-MEN-03-4）：u2 有 2 条未读提及，u1 撤回其中一条：mentionUnread=1，firstMentionSeq 是剩下那条。
func TestTC_MEN_03_04_RecallMention(t *testing.T) {
	e := newC4Env(t, nil)
	e.ownedGroup("g1", "u1", "u2", "u3")
	u1, u2 := e.dial("u1", "web"), e.dial("u2", "web")
	a := mustAck[protocol.SendAck](t, sendMentions(u1, "g1", "cmid-m3401", "@u2 one", []string{"u2"}, false))
	b := mustAck[protocol.SendAck](t, sendMentions(u1, "g1", "cmid-m3402", "@u2 two", []string{"u2"}, false))
	if n, _ := mentionUnread(t, u2, "group:g1"); n != 2 {
		t.Fatalf("before = %d", n)
	}
	unread := convByID(t, u2, "group:g1").Unread
	mustRecall(t, u1, "group:g1", a.MsgID)
	if n, first := mentionUnread(t, u2, "group:g1"); n != 1 || first != b.Seq {
		t.Errorf("after recall: %d %d", n, first)
	}
	if c := convByID(t, u2, "group:g1"); c.Unread != unread {
		t.Errorf("unread changed by the recall: %d", c.Unread)
	}
}

// TC-MEN-03-08（AC-MEN-03-8）：自己 @ 自己不计入；已退群的成员被过滤；提及表写失败 500 / 5000，消息未保存。
func TestTC_MEN_03_08_SelfLeftAndWriteFault(t *testing.T) {
	e := newC4Env(t, nil)
	e.ownedGroup("g1", "o", "u1", "u2", "u3")
	u1, u2, u3 := e.dial("u1", "web"), e.dial("u2", "web"), e.dial("u3", "web")
	mustAck[protocol.SendAck](t, sendMentions(u1, "g1", "cmid-m3801", "@u1", []string{"u1"}, false))
	if n, _ := mentionUnread(t, u1, "group:g1"); n != 0 {
		t.Errorf("self mention counted: %d", n)
	}
	var selfRows int
	e.db().QueryRow(`SELECT count(*) FROM conv_mentions WHERE user_id = 'u1'`).Scan(&selfRows)
	if selfRows != 0 {
		t.Errorf("conv_mentions has a row for the sender: %d", selfRows) // 规则 6：自己 @ 自己不写
	}
	mustAck[protocol.Empty](t, u3.call(protocol.CmdGroupLeave, map[string]any{"groupId": "g1"}))
	ack := mustAck[protocol.SendAck](t, sendMentions(u1, "g1", "cmid-m3802", "@u3 @u2", []string{"u3", "u2"}, false))
	if m := mentionsOf(t, msgBySeq(t, syncConv(t, u2, "group:g1").Messages, ack.Seq)); len(m.UserIDs) != 1 || m.UserIDs[0] != "u2" {
		t.Errorf("left member not filtered: %+v", m)
	}
	only := mustAck[protocol.SendAck](t, sendMentions(u1, "g1", "cmid-m3803", "@u3", []string{"u3"}, false))
	if m := msgBySeq(t, syncConv(t, u2, "group:g1").Messages, only.Seq); m.Body.Mentions != nil {
		t.Errorf("all filtered: saved as without mentions, got %+v", m.Body.Mentions)
	}
	e.adminPost("/admin/test/faults", map[string]any{"mentionWrite": true}, adminKey)
	before := convByID(t, u1, "group:g1").MaxSeq
	mustErr(t, sendMentions(u1, "g1", "cmid-m3804", "@u2", []string{"u2"}, false), protocol.CodeInternal)
	if got := convByID(t, u1, "group:g1").MaxSeq; got != before {
		t.Errorf("message saved despite the mention write failure: %d -> %d", before, got)
	}
	e.adminPost("/admin/test/faults", map[string]any{"mentionWrite": false}, adminKey)
	mustAck[protocol.SendAck](t, sendMentions(u1, "g1", "cmid-m3804", "@u2", []string{"u2"}, false)) // 同一 clientMsgId 重试成功
}
