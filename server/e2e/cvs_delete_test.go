package e2e

// C5 删除会话（BE-485）：TC-CVS-03-01～03-04、03-06、03-07、03-09、03-11～03-15，TC-CVS-04-04 的混合部分，
// 清除范围内的撤回 4044、引用 4040（backend-chat 第 77 条），开关关闭后已有设置照常生效（P-46 ②）。环境 E1（cvsEnv）、E1 + 假 S3。
// 本地结果，CI 不运行。

import (
	"bytes"
	"fmt"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"tianting/server/internal/app"
	"tianting/server/internal/protocol"
)

// c2cTen：u1、u2 互为好友，u2 给 u1 发 10 条（第 3 条含 ZXQ-CVS-1）。u1 没有读。
func (e *cvsEnv) c2cTen(u1, u2 *client, tag string) string {
	e.t.Helper()
	var conv string
	for i := 1; i <= 10; i++ {
		text := fmt.Sprintf("m%d", i)
		if i == 3 {
			text = "secret ZXQ-CVS-1"
		}
		conv = mustAck[protocol.SendAck](e.t, u2.sendText("c2c", u1.uid, fmt.Sprintf("cmid-%s-%02d", tag, i), text)).ConvID
	}
	return conv
}

// TC-CVS-03-01、03-02（AC-CVS-03-1、03-2）：u1 删除 10 条的单聊：ack clearedSeq=10，列表没有、deleted 含它、sync 空；
// u2 的会话和 10 条消息都在、未读不变；后台元数据和授权后的正文照常 10 条（第 87 条）。
// u2 再发 1 条：重新出现，unread=1、minSeq=11，deleted 不再含它，sync 两个方向都只有这 1 条。
func TestTC_CVS_03_01_DeleteC2C(t *testing.T) {
	e := newCVSEnv(t, func(c *app.Config) { c.C2CBodyAccess = true })
	e.usersNamed("u1", "u2")
	e.mutual("u1", "u2")
	u1, u2 := e.dial("u1", "web"), e.dial("u2", "web")
	conv := e.c2cTen(u1, u2, "d0301")
	before, _ := lookupConv(convsOf(t, u2), conv)
	if got := mustConvDelete(t, u1, conv, 10); got != 10 {
		t.Fatalf("clearedSeq = %d", got)
	}
	ack := convsOf(t, u1)
	if _, ok := lookupConv(ack, conv); ok {
		t.Error("the deleted conversation is still in convs")
	}
	if cs, ok := deletedSeq(ack, conv); !ok || cs != 10 || ack.DeletedTruncated {
		t.Errorf("deleted = %+v, truncated %v", ack.Deleted, ack.DeletedTruncated)
	}
	if s := syncConv(t, u1, conv); len(s.Messages) != 0 || s.HasMore {
		t.Errorf("u1 sync after delete: %d messages", len(s.Messages))
	}
	after, ok := lookupConv(convsOf(t, u2), conv)
	if !ok || after.Unread != before.Unread || after.MinSeq != 1 || len(syncConv(t, u2, conv).Messages) != 10 {
		t.Errorf("u2 affected: before %+v after %+v", before, after)
	}
	// 后台查询不受影响：元数据和正文（授权后）照常 10 条。
	code, meta := e.call(e.mod, "/admin/convs/messages/query", map[string]any{"convId": conv})
	if code != http.StatusOK || strings.Count(string(meta), `"msgId"`) != 10 {
		t.Errorf("admin meta: %d %s", code, meta)
	}
	code, body := e.readBody(e.mustGrant(conv, false), conv)
	if code != http.StatusOK || !bytes.Contains(body, []byte("ZXQ-CVS-1")) || strings.Count(string(body), `"msgId"`) != 10 {
		t.Errorf("admin body: %d %s", code, body)
	}

	m11 := mustAck[protocol.SendAck](t, u2.sendText("c2c", "u1", "cmid-d0302", "new one"))
	ack = convsOf(t, u1)
	cv, ok := lookupConv(ack, conv)
	if !ok || cv.Unread != 1 || cv.MinSeq != 11 || cv.LastMsg == nil || cv.LastMsg.MsgID != m11.MsgID {
		t.Fatalf("after the new message: ok=%v %+v", ok, cv)
	}
	if _, ok := deletedSeq(ack, conv); ok {
		t.Error("deleted still has the conversation")
	}
	s := syncConv(t, u1, conv)
	back := mustAck[protocol.SyncAck](t, u1.call(protocol.CmdSync, map[string]any{"convId": conv, "beforeSeq": 12, "limit": 50}))
	if len(s.Messages) != 1 || s.Messages[0].Seq != 11 || len(back.Messages) != 1 || back.HasMore {
		t.Errorf("sync after: %d messages, before-page %d hasMore %v", len(s.Messages), len(back.Messages), back.HasMore)
	}
	if bytes.Contains(syncRaw(t, u1, conv), []byte("ZXQ-CVS-1")) {
		t.Error("u1 sync still has the old text")
	}
}

// TC-CVS-03-03（AC-CVS-03-3）：A upToSeq=999 清到 10，隐藏、在 deleted；B upToSeq=8 清到 8，仍在列表，unread=2、minSeq=9，sync 只有 9、10。
func TestTC_CVS_03_03_UpToSeq(t *testing.T) {
	e := newCVSEnv(t, nil)
	e.usersNamed("u1", "ua", "ub")
	e.mutual("u1", "ua", "ub")
	u1 := e.dial("u1", "web")
	a := e.c2cTen(u1, e.dial("ua", "web"), "d0303a")
	b := e.c2cTen(u1, e.dial("ub", "web"), "d0303b")
	if got := mustConvDelete(t, u1, a, 999); got != 10 {
		t.Errorf("A clearedSeq = %d", got)
	}
	if got := mustConvDelete(t, u1, b, 8); got != 8 {
		t.Errorf("B clearedSeq = %d", got)
	}
	ack := convsOf(t, u1)
	if _, ok := lookupConv(ack, a); ok {
		t.Error("A still listed")
	}
	if cs, ok := deletedSeq(ack, a); !ok || cs != 10 {
		t.Errorf("A deleted = %v %v", cs, ok)
	}
	cv, ok := lookupConv(ack, b)
	if !ok || cv.Unread != 2 || cv.MinSeq != 9 || cv.ReadSeq != 8 {
		t.Errorf("B = ok %v %+v", ok, cv)
	}
	if _, ok := deletedSeq(ack, b); ok {
		t.Error("B is in deleted")
	}
	s := syncConv(t, u1, b)
	if len(s.Messages) != 2 || s.Messages[0].Seq != 9 || s.Messages[1].Seq != 10 {
		t.Errorf("B sync = %d messages", len(s.Messages))
	}
}

// TC-CVS-03-04（AC-CVS-03-4）：删除群会话后仍是成员、在 myGroups、能发；deleted 含 g1；发送后回到列表，只看到自己这条。
func TestTC_CVS_03_04_DeleteGroup(t *testing.T) {
	e := newCVSEnv(t, nil)
	gid, cs := e.selfGroup("o", nil, "u1", "u2")
	conv := "group:" + gid
	for i := range 5 {
		sendG(t, cs["o"], gid, fmt.Sprintf("cmid-d0304-%d", i), "hi")
	}
	u1 := cs["u1"]
	maxSeq := convByID(t, u1, conv).MaxSeq
	if got := mustConvDelete(t, u1, conv, maxSeq); got != maxSeq {
		t.Fatalf("clearedSeq = %d, want %d", got, maxSeq)
	}
	ack := convsOf(t, u1)
	if _, ok := lookupConv(ack, conv); ok {
		t.Error("g1 still listed")
	}
	if got, ok := deletedSeq(ack, conv); !ok || got != maxSeq {
		t.Errorf("deleted = %+v", ack.Deleted)
	}
	page := mustAck[protocol.GroupMembersAck](t, u1.call(protocol.CmdGroupMembers, map[string]any{"groupId": gid}))
	if !slices.ContainsFunc(page.Members, func(m protocol.GroupMemberItem) bool { return m.UserID == "u1" }) {
		t.Error("u1 is not a member any more")
	}
	mine := mustAck[protocol.MyGroupsAck](t, u1.call(protocol.CmdMyGroups, map[string]any{}))
	if !slices.ContainsFunc(mine.Groups, func(g protocol.MyGroupItem) bool { return g.GroupID == gid }) {
		t.Error("myGroups lost g1")
	}
	own := sendG(t, u1, gid, "cmid-d0304-own", "mine")
	cv, ok := lookupConv(convsOf(t, u1), conv)
	if !ok || cv.MinSeq != maxSeq+1 || cv.Unread != 0 {
		t.Errorf("after sending: ok %v %+v", ok, cv)
	}
	if s := syncConv(t, u1, conv); len(s.Messages) != 1 || s.Messages[0].MsgID != own.MsgID {
		t.Errorf("sync = %d messages", len(s.Messages))
	}
}

// TC-CVS-03-06（AC-CVS-03-6）：置顶且免打扰的群删除后有新消息：重新出现，pinned=false、没有 pinnedAt，notifyOff=true，按 updatedAt 排在普通会话里。
func TestTC_CVS_03_06_DeleteClearsPinKeepsNotifyOff(t *testing.T) {
	e := newCVSEnv(t, nil)
	gid, cs := e.selfGroup("o", nil, "u1", "u2")
	conv := "group:" + gid
	u1 := cs["u1"]
	other := e.friendsOf(u1, "px")[0]
	mustConvSet(t, u1, other, map[string]any{"pinned": true})
	mustConvSet(t, u1, conv, map[string]any{"pinned": true, "notifyOff": true})
	mustConvDelete(t, u1, conv, convByID(t, u1, conv).MaxSeq)
	if row, _ := e.settingRow("u1", conv); row.PinnedAt != 0 || row.NotifyOff != 1 {
		t.Errorf("row after delete = %+v", row)
	}
	e.advance(time.Second)
	e.usersNamed("py")
	e.mutual("u1", "py")
	mustAck[protocol.SendAck](t, u1.sendText("c2c", "py", "cmid-d0306-py", "older"))
	e.advance(time.Second)
	sendG(t, cs["o"], gid, "cmid-d0306", "back")
	ack := convsOf(t, u1)
	cv, ok := lookupConv(ack, conv)
	if !ok || cv.Pinned || cv.PinnedAt != 0 || !cv.NotifyOff || cv.Unread != 1 {
		t.Fatalf("g1 = ok %v %+v", ok, cv)
	}
	if got := convOrder(ack); !slices.Equal(got[:3], []string{other, conv, protocol.C2CConvID("u1", "py")}) {
		t.Errorf("order = %v", got)
	}
}

// TC-CVS-03-09（AC-CVS-03-9）：非参与方 4032、非成员 4033、已解散的群 4043，都没有写入；没有消息的单聊幂等成功（clearedSeq=0，不写）。
func TestTC_CVS_03_09_DeleteErrors(t *testing.T) {
	e := newCVSEnv(t, nil)
	gid, cs := e.selfGroup("o", nil, "u1", "u2")
	g2, cs2 := e.selfGroup("o2", nil, "u1", "u3")
	mustAck[protocol.Empty](t, cs2["o2"].call(protocol.CmdGroupDissolve, map[string]any{"groupId": g2}))
	e.usersNamed("u9")
	e.mutual("u1", "u2", "u3")
	u1, u9 := cs["u1"], e.dial("u9", "web")
	c12 := mustAck[protocol.SendAck](t, u1.sendText("c2c", "u2", "cmid-d0309", "hi")).ConvID
	mustErr(t, convDeleteCall(u9, c12, 1), protocol.CodeNotConvParticipant)
	mustErr(t, convDeleteCall(u9, "group:"+gid, 1), protocol.CodeNotGroupMember)
	mustErr(t, convDeleteCall(u1, "group:"+g2, 1), protocol.CodeGroupNotFound)
	if got := mustConvDelete(t, u1, protocol.C2CConvID("u1", "u3"), 5); got != 0 {
		t.Errorf("empty c2c clearedSeq = %d", got)
	}
	mustErr(t, u1.call(protocol.CmdConvDelete, map[string]any{"convId": c12, "upToSeq": -1}), protocol.CodeBadRequest)
	if n := e.settingCount("1 = 1"); n != 0 {
		t.Errorf("conv_settings has %d rows", n)
	}
}

// TC-CVS-03-11（AC-CVS-03-11）：两个没有消息的托管群：只设免打扰的 g3 照常显示（lastMsg=null）；删除空群 g4（upToSeq=0）隐藏、deleted 含 {g4, 0}；
// g4 有消息后重新出现、deleted 不再含它。
func TestTC_CVS_03_11_EmptyGroups(t *testing.T) {
	e := newCVSEnv(t, nil)
	e.createGroup("g3", "u1", "u2")
	e.createGroup("g4", "u1", "u2")
	u1, u2 := e.dial("u1", "web"), e.dial("u2", "web")
	mustConvSet(t, u1, "group:g3", map[string]any{"notifyOff": true})
	if got := mustConvDelete(t, u1, "group:g4", 0); got != 0 {
		t.Errorf("clearedSeq = %d", got)
	}
	ack := convsOf(t, u1)
	g3, ok := lookupConv(ack, "group:g3")
	if !ok || g3.LastMsg != nil || !g3.NotifyOff {
		t.Errorf("g3 = ok %v %+v", ok, g3)
	}
	if _, ok := lookupConv(ack, "group:g4"); ok {
		t.Error("g4 still listed")
	}
	if cs, ok := deletedSeq(ack, "group:g4"); !ok || cs != 0 {
		t.Errorf("deleted = %+v", ack.Deleted)
	}
	sendG(t, u2, "g4", "cmid-d0311", "first")
	ack = convsOf(t, u1)
	if cv, ok := lookupConv(ack, "group:g4"); !ok || cv.Unread != 1 || cv.MinSeq != 1 {
		t.Errorf("g4 after a message = ok %v %+v", ok, cv)
	}
	if _, ok := deletedSeq(ack, "group:g4"); ok {
		t.Error("deleted still has g4")
	}
}

// TC-CVS-03-12（AC-CVS-03-12）：historyVisible=true 的 g1 有 5 条，u1 删除后被移出再加入：deleted 不含 g1、minSeq=1，sync 看到全部 5 条。
func TestTC_CVS_03_12_RejoinSeesHistory(t *testing.T) {
	e := newCVSEnv(t, nil)
	e.users("u1", "u2")
	if code, b := e.adminPost("/admin/groups", map[string]any{"groupId": "g1", "name": "g1", "members": []string{"u1", "u2"}, "historyVisible": true}, adminKey); code != http.StatusCreated {
		t.Fatalf("create: %d %s", code, b)
	}
	u1, u2 := e.dial("u1", "web"), e.dial("u2", "web")
	var ids []string
	for i := range 5 {
		ids = append(ids, sendG(t, u2, "g1", fmt.Sprintf("cmid-d0312-%d", i), "hi").MsgID)
	}
	mustConvDelete(t, u1, "group:g1", 5)
	if s := syncConv(t, u1, "group:g1"); len(s.Messages) != 0 {
		t.Fatalf("sync after delete = %d", len(s.Messages))
	}
	e.adminDo(http.MethodDelete, "/admin/groups/g1/members", map[string]any{"members": []string{"u1"}}, adminKey)
	e.adminPost("/admin/groups/g1/members", map[string]any{"members": []string{"u1"}}, adminKey)
	ack := convsOf(t, u1)
	if _, ok := deletedSeq(ack, "group:g1"); ok {
		t.Error("deleted still has g1")
	}
	cv, ok := lookupConv(ack, "group:g1")
	if !ok || cv.MinSeq != 1 {
		t.Errorf("g1 = ok %v %+v", ok, cv)
	}
	got := map[string]bool{}
	for _, m := range syncConv(t, u1, "group:g1").Messages {
		got[m.MsgID] = true
	}
	for _, id := range ids {
		if !got[id] {
			t.Errorf("message %s not visible after rejoining", id)
		}
	}
}

// TC-CVS-03-13（AC-CVS-03-13）：删除后只有一条 tip 也重新出现（未读 1，只看到 tip）；upToSeq 小于已有 clearedSeq 时不回退（ack 和数据库都是原值）；
// 重复删除幂等成功。
func TestTC_CVS_03_13_TipAndMonotonic(t *testing.T) {
	e := newCVSEnv(t, nil)
	gid, cs := e.selfGroup("o", nil, "u1", "u2")
	conv := "group:" + gid
	for i := range 6 {
		sendG(t, cs["o"], gid, fmt.Sprintf("cmid-d0313-%d", i), "hi")
	}
	u1 := cs["u1"]
	n := convByID(t, u1, conv).MaxSeq
	mustConvDelete(t, u1, conv, n)
	mustAck[protocol.Empty](t, cs["o"].call(protocol.CmdGroupUpdate, map[string]any{"groupId": gid, "name": "renamed"}))
	cv, ok := lookupConv(convsOf(t, u1), conv)
	if !ok || cv.Unread != 1 || cv.MinSeq != n+1 {
		t.Fatalf("after the tip: ok %v %+v", ok, cv)
	}
	if s := syncConv(t, u1, conv); len(s.Messages) != 1 || s.Messages[0].Type != protocol.MsgTip {
		t.Errorf("sync = %+v", s.Messages)
	}
	if got := mustConvDelete(t, u1, conv, 3); got != n {
		t.Errorf("upToSeq 3: clearedSeq = %d, want %d", got, n)
	}
	if row, _ := e.settingRow("u1", conv); row.ClearedSeq != n {
		t.Errorf("row = %+v", row)
	}
	for range 2 {
		if got := mustConvDelete(t, u1, conv, n+1); got != n+1 {
			t.Errorf("clearedSeq = %d, want %d", got, n+1)
		}
	}
	if _, ok := lookupConv(convsOf(t, u1), conv); ok {
		t.Error("g1 still listed after deleting the tip")
	}
}

// TC-CVS-03-14（AC-CVS-03-14，go test -race 也跑）：upToSeq=10 的删除和 seq 11 的发送并发，50 次（每次新会话）：会话最终可见、clearedSeq=10、
// seq 11 能拉到。两种先后都要出现：并发的 50 次之外，再各做一次确定的先后（先删后发、先发后删）。
func TestTC_CVS_03_14_DeleteRacesSend(t *testing.T) {
	e := newCVSEnv(t, func(c *app.Config) { c.Gateway.Guard.HandshakeBurst = 1000 }) // 52 peers dial from one IP
	e.usersNamed("u1")
	u1 := e.dial("u1", "web")
	check := func(conv string, label string) {
		t.Helper()
		cv, ok := lookupConv(convsOf(t, u1), conv)
		if !ok || cv.MaxSeq != 11 || cv.MinSeq != 11 {
			t.Errorf("%s: ok %v %+v", label, ok, cv)
		}
		s := mustAck[protocol.SyncAck](t, u1.call(protocol.CmdSync, map[string]any{"convId": conv, "afterSeq": 10, "limit": 10}))
		if len(s.Messages) != 1 || s.Messages[0].Seq != 11 {
			t.Errorf("%s: sync afterSeq 10 = %d messages", label, len(s.Messages))
		}
	}
	peer := func(i int) (*client, string) {
		p := fmt.Sprintf("race%02d", i)
		e.usersNamed(p)
		e.mutual("u1", p)
		pc := e.dial(p, "web")
		return pc, e.c2cTen(u1, pc, p)
	}
	for i := range 50 {
		pc, conv := peer(i)
		var wg sync.WaitGroup
		var cleared int64
		wg.Add(1)
		go func() {
			defer wg.Done()
			if f := pc.sendText("c2c", "u1", fmt.Sprintf("cmid-race-%02d", i), "eleven"); f.Error != nil {
				t.Errorf("run %d: send: %+v", i, f.Error)
			}
		}()
		cleared = mustConvDelete(t, u1, conv, 10)
		wg.Wait()
		if cleared != 10 {
			t.Errorf("run %d: clearedSeq = %d", i, cleared)
		}
		u1.pending = nil
		check(conv, fmt.Sprintf("run %d", i))
	}
	pc, conv := peer(50)
	mustConvDelete(t, u1, conv, 10)
	mustAck[protocol.SendAck](t, pc.sendText("c2c", "u1", "cmid-race-del-first", "eleven"))
	check(conv, "delete first")
	pc, conv = peer(51)
	mustAck[protocol.SendAck](t, pc.sendText("c2c", "u1", "cmid-race-send-first", "eleven"))
	mustConvDelete(t, u1, conv, 10)
	check(conv, "send first")
}

// 清除范围内的消息：撤回 4044（和看不到的消息一样），引用 4040 quoteUnavailable（backend-chat 第 77 条）；范围之后的照常。
func TestCVSClearedRangeRecallAndQuote(t *testing.T) {
	e := newCVSEnv(t, nil)
	e.usersNamed("u1", "u2")
	e.mutual("u1", "u2")
	u1, u2 := e.dial("u1", "web"), e.dial("u2", "web")
	old := mustAck[protocol.SendAck](t, u1.sendText("c2c", "u2", "cmid-cr-1", "old"))
	keep := mustAck[protocol.SendAck](t, u1.sendText("c2c", "u2", "cmid-cr-2", "keep"))
	mustConvDelete(t, u1, old.ConvID, old.Seq)
	mustErr(t, recallCall(u1, old.ConvID, old.MsgID), protocol.CodeMessageNotFound)
	f := sendQuote(u1, "c2c", "u2", "cmid-cr-3", "re", old.MsgID)
	if f.Error == nil || f.Error.Code != protocol.CodeNotFound || f.Error.Reason != protocol.ReasonQuoteUnavailable {
		t.Errorf("quote of a cleared message: %+v", f.Error)
	}
	mustAck[protocol.SendAck](t, sendQuote(u1, "c2c", "u2", "cmid-cr-4", "re", keep.MsgID))
	mustAck[protocol.SendAck](t, sendQuote(u2, "c2c", "u1", "cmid-cr-5", "re", old.MsgID)) // 只影响自己：u2 照常能引用
	mustRecall(t, u1, old.ConvID, keep.MsgID)
}

// TC-CVS-04-04 的混合部分（AC-CVS-04-4，backend-chat 第 73 条）：convSet 和 convDelete 共用一个计数，混合 6 次时第 6 次 4290。
func TestTC_CVS_04_04_MixedWithDelete(t *testing.T) {
	e := newCVSEnv(t, func(c *app.Config) { c.ConvSettings.RatePerSec = 0 })
	e.usersNamed("u1")
	u1 := e.dial("u1", "web")
	c := e.friendsOf(u1, "r1", "r2", "r3")
	e.resetLimits("u1")
	mustConvSet(t, u1, c[0], map[string]any{"notifyOff": true})
	mustConvDelete(t, u1, c[1], 1)
	mustConvSet(t, u1, c[2], map[string]any{"pinned": true})
	mustConvDelete(t, u1, c[1], 1)
	mustConvSet(t, u1, c[0], map[string]any{"notifyOff": false})
	mustErr(t, convDeleteCall(u1, c[2], 1), protocol.CodeRateLimited)
	if _, ok := lookupConv(convsOf(t, u1), c[2]); !ok {
		t.Error("the rate-limited convDelete was applied")
	}
	e.advance(time.Second)
	mustConvDelete(t, u1, c[2], 1)
}

// P-46 ②（backend-chat 第 81 条）：开关打开时写下的置顶、免打扰、删除，在开关关掉之后照常生效（排序、隐藏、deleted、minSeq、sync），
// 只拦新命令（4030）。两个进程先后用同一个 SQLite 文件。
func TestCVSSwitchOffKeepsSettings(t *testing.T) {
	db := filepath.Join(t.TempDir(), "im.db")
	on := newCVSEnv(t, func(c *app.Config) { c.DBPath = db })
	on.usersNamed("u1", "u2", "u3")
	on.mutual("u1", "u2", "u3")
	u1, u2 := on.dial("u1", "web"), on.dial("u2", "web")
	pinned := mustAck[protocol.SendAck](t, u1.sendText("c2c", "u3", "cmid-off-1", "older")).ConvID
	on.advance(time.Second)
	gone := on.c2cTen(u1, u2, "off")
	mustConvSet(t, u1, pinned, map[string]any{"pinned": true, "notifyOff": true})
	mustConvDelete(t, u1, gone, 8)
	wantOrder := convOrder(convsOf(t, u1))

	off := newCVSEnv(t, func(c *app.Config) { c.DBPath = db; c.ConvSettings.Enabled = false })
	v1 := off.dial("u1", "ios")
	ack := convsOf(t, v1)
	if got := convOrder(ack); !slices.Equal(got, wantOrder) || got[0] != pinned {
		t.Errorf("order with the switch off = %v, want %v", got, wantOrder)
	}
	p, _ := lookupConv(ack, pinned)
	g, _ := lookupConv(ack, gone)
	if !p.Pinned || !p.NotifyOff || g.MinSeq != 9 || g.Unread != 2 {
		t.Errorf("pinned %+v, deleted-to-8 %+v", p, g)
	}
	if s := syncConv(t, v1, gone); len(s.Messages) != 2 {
		t.Errorf("sync = %d messages, want 2", len(s.Messages))
	}
	f := convSetCall(v1, pinned, map[string]any{"pinned": false})
	if f.Error == nil || f.Error.Reason != protocol.ReasonFeatureDisabled {
		t.Errorf("convSet with the switch off: %+v", f.Error)
	}
	mustConvDelete(t, u1, gone, 10) // 打开的进程照常
	if _, ok := deletedSeq(convsOf(t, v1), gone); !ok {
		t.Error("the hidden conversation is not in deleted with the switch off")
	}
}

// TC-CVS-03-07、03-15（AC-CVS-03-7、03-15）：u1 删除到 seq 5 后，清除范围内的图片（seq 3）原图、缩略图都是 4048，和不存在的 fileId 相同；
// u2 照常拿到地址、能下载；群里 u1 删除后其他成员照常，对象还在、files 行状态不变。
func TestTC_CVS_03_07_FilesAfterDelete(t *testing.T) {
	e := newFileEnv(t, func(c *app.Config) {
		c.ConvSettings.Enabled = true
		c.ConvSettings.RatePerSec = 10000
		c.SendRate = 10000
	})
	tok1, tok2 := e.token("u1"), e.token("u2")
	u1, u2 := e.dial("u1", "web"), e.dial("u2", "web")
	mustAck[protocol.SendAck](t, u2.sendText("c2c", "u1", "cmid-f0307-1", "one"))
	mustAck[protocol.SendAck](t, u2.sendText("c2c", "u1", "cmid-f0307-2", "two"))
	orig := fakeJPEG(800, 600, 32<<10)
	img := e.uploadImage(tok2, "photo-ZXQ-CVS-2.jpg", orig, smallThumb())
	sent := mustAck[protocol.SendAck](t, sendFile(u2, "c2c", "u1", "cmid-f0307-3", "image", img, nil))
	if sent.Seq != 3 {
		t.Fatalf("image seq = %d", sent.Seq)
	}
	mustAck[protocol.SendAck](t, u2.sendText("c2c", "u1", "cmid-f0307-4", "four"))
	after := mustAck[protocol.SendAck](t, u2.sendText("c2c", "u1", "cmid-f0307-5", "five"))
	e.mustURL(tok1, img, "original") // 删除之前 u1 可以
	if got := mustConvDelete(t, u1, after.ConvID, 5); got != 5 {
		t.Fatalf("clearedSeq = %d", got)
	}
	for _, v := range []string{"original", "thumb"} {
		e.denied(tok1, img, v)
		_, _, deniedBody := e.urls(tok1, [2]string{img, v})
		_, _, ghostBody := e.urls(tok1, [2]string{"0193a7c4-ffff-7000-8000-000000000000", v})
		if strings.ReplaceAll(string(deniedBody), img, "X") != strings.ReplaceAll(string(ghostBody), "0193a7c4-ffff-7000-8000-000000000000", "X") {
			t.Errorf("%s: cleared %s differs from a missing file %s", v, deniedBody, ghostBody)
		}
	}
	u := e.mustURL(tok2, img, "original")
	if code, _, b := download(t, u.URL); code != http.StatusOK || !bytes.Equal(b, orig) {
		t.Errorf("u2 download: %d (%d bytes)", code, len(b))
	}

	// 群（TC-CVS-03-15）：o 发一个文件，u1 删除到最新，u2 照常；对象还在，files 行状态不变。
	e.createGroup("g1", "o", "u1", "u2")
	tokO := e.token("o")
	o := e.dial("o", "web")
	data := bytes.Repeat([]byte("%PDF-1.4 ZXQ "), 64)
	doc := e.uploadFile(tokO, "report.pdf", "application/pdf", data)
	gs := mustAck[protocol.SendAck](t, sendFile(o, "group", "g1", "cmid-f0315", "file", doc, nil))
	before := e.file(doc)
	mustConvDelete(t, u1, "group:g1", gs.Seq)
	e.denied(tok1, doc, "original")
	u = e.mustURL(tok2, doc, "original")
	if code, _, b := download(t, u.URL); code != http.StatusOK || !bytes.Equal(b, data) {
		t.Errorf("u2 group download: %d", code)
	}
	if f := e.file(doc); f.State != before.State || f.ObjectKey != before.ObjectKey {
		t.Errorf("files row changed: %+v -> %+v", before, f)
	}
	if _, ok := e.s3.Get(e.s3.Private, before.ObjectKey); !ok {
		t.Error("the object is gone")
	}
}

// convs.deleted (F-CVS-03 rule 6, issue 79): only hidden ones, newest deletion first, at most IM_CONV_DELETED_MAX, then deletedTruncated=true.
func TestCVSDeletedTruncated(t *testing.T) {
	e := newCVSEnv(t, func(c *app.Config) { c.ConvSettings.DeletedMax = 2 })
	e.usersNamed("u1")
	u1 := e.dial("u1", "web")
	c := e.friendsOf(u1, "t1", "t2", "t3")
	for _, conv := range c {
		e.advance(time.Second)
		mustConvDelete(t, u1, conv, 1)
	}
	ack := convsOf(t, u1)
	if len(ack.Deleted) != 2 || !ack.DeletedTruncated || ack.Deleted[0].ConvID != c[2] || ack.Deleted[1].ConvID != c[1] {
		t.Errorf("deleted = %+v truncated %v", ack.Deleted, ack.DeletedTruncated)
	}
	mustAck[protocol.SendAck](t, u1.sendText("c2c", "t3", "cmid-trunc-back", "back"))
	if ack := convsOf(t, u1); len(ack.Deleted) != 2 || ack.DeletedTruncated || ack.Deleted[0].ConvID != c[1] {
		t.Errorf("after t3 is back: %+v truncated %v", ack.Deleted, ack.DeletedTruncated)
	}
}
