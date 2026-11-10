package e2e

// C5 置顶和免打扰（BE-484）：TC-CVS-01-01～01-05、TC-CVS-02-01 的 unread 部分、TC-CVS-04-04。环境 E1（cvsEnv）。本地结果，CI 不运行。

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/coder/websocket"

	"tianting/server/internal/app"
	"tianting/server/internal/protocol"
)

// friendsOf：u1 和 peers 互为好友，u1 按 [IM 钟] 每隔 1 秒给每个 peer 发 1 条（前面的更旧）。返回会话 ID。
func (e *cvsEnv) friendsOf(u1 *client, peers ...string) []string {
	e.t.Helper()
	e.usersNamed(peers...)
	e.mutual(u1.uid, peers...)
	out := make([]string, len(peers))
	for i, p := range peers {
		e.advance(time.Second)
		out[i] = mustAck[protocol.SendAck](e.t, u1.sendText("c2c", p, fmt.Sprintf("cmid-%s-%s", u1.uid, p), "hi "+p)).ConvID
	}
	return out
}

// TC-CVS-01-01（AC-CVS-01-1）：置顶最旧的 c1，1 秒后置顶 c2：ack 的 pinned=true、pinnedAt 是服务端时间；convs 顺序 c2、c1、其余按时间；
// updatedAt 相同的普通会话按 convId 升序（AC-CONV-01-4 不变）。ack 的 conv 和 convs 里这一项完全相同（BE-484 第 3 条）。
func TestTC_CVS_01_01_PinOrder(t *testing.T) {
	e := newCVSEnv(t, nil)
	e.usersNamed("u1")
	u1 := e.dial("u1", "web")
	c := e.friendsOf(u1, "pa", "pb", "pc", "pd") // c1..c4，c4 最新
	e.advance(time.Second)
	a1 := mustConvSet(t, u1, c[0], map[string]any{"pinned": true})
	t1 := e.now
	e.advance(time.Second)
	a2 := mustConvSet(t, u1, c[1], map[string]any{"pinned": true})
	if !a1.Pinned || a1.PinnedAt != t1 || !a2.Pinned || a2.PinnedAt != e.now || a2.PinnedAt <= a1.PinnedAt {
		t.Fatalf("acks: c1 %+v, c2 %+v", a1, a2)
	}
	// 两个 updatedAt 相同的普通会话：[IM 钟] 不动，给 pf、pe 各发一条（convId 升序 pe 在前）。
	e.usersNamed("pe", "pf")
	e.mutual("u1", "pe", "pf")
	e.advance(time.Second)
	mustAck[protocol.SendAck](t, u1.sendText("c2c", "pf", "cmid-tie-f", "tie"))
	mustAck[protocol.SendAck](t, u1.sendText("c2c", "pe", "cmid-tie-e", "tie"))
	ack := convsOf(t, u1)
	want := []string{c[1], c[0], protocol.C2CConvID("u1", "pe"), protocol.C2CConvID("u1", "pf"), c[3], c[2]}
	if got := convOrder(ack); !slices.Equal(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
	raw := u1.call(protocol.CmdConvs, map[string]any{})
	var convs struct {
		Convs []json.RawMessage `json:"convs"`
	}
	json.Unmarshal(raw.Data, &convs)
	again := convSetCall(u1, c[1], map[string]any{"pinned": true})
	var ackConv struct {
		Conv json.RawMessage `json:"conv"`
	}
	json.Unmarshal(again.Data, &ackConv)
	if string(ackConv.Conv) != string(convs.Convs[0]) {
		t.Errorf("convSet ack conv differs from convs:\n%s\n%s", ackConv.Conv, convs.Convs[0])
	}
	validFrame(t, frameSchema(t), "convSet ack", again)
	validFrame(t, frameSchema(t), "convs ack", raw)
	// pinnedAt decides among pinned ones, not updatedAt: re-pin the older c1 later, it moves before c2.
	mustConvSet(t, u1, c[0], map[string]any{"pinned": false})
	e.advance(time.Second)
	mustConvSet(t, u1, c[0], map[string]any{"pinned": true})
	if got := convOrder(convsOf(t, u1)); got[0] != c[0] || got[1] != c[1] {
		t.Errorf("after re-pinning c1: %v", got)
	}
}

// TC-CVS-01-02（AC-CVS-01-2）：已置顶 20 个时第 21 个 4220（没有写入）；已置顶的再次置顶成功、pinnedAt 不变；取消一个后第 21 个成功。
func TestTC_CVS_01_02_PinLimit(t *testing.T) {
	e := newCVSEnv(t, nil)
	e.usersNamed("u1")
	u1 := e.dial("u1", "web")
	peers := make([]string, 21)
	for i := range peers {
		peers[i] = fmt.Sprintf("q%02d", i)
	}
	c := e.friendsOf(u1, peers...)
	for _, conv := range c[:20] {
		mustConvSet(t, u1, conv, map[string]any{"pinned": true})
	}
	f := convSetCall(u1, c[20], map[string]any{"pinned": true})
	mustErr(t, f, protocol.CodePinLimitExceeded)
	if f.Error.Details != nil {
		t.Errorf("4220 has details: %+v", f.Error.Details)
	}
	if _, ok := e.settingRow("u1", c[20]); ok {
		t.Error("the 21st conversation got a row")
	}
	if cv, _ := lookupConv(convsOf(t, u1), c[20]); cv.Pinned {
		t.Error("the 21st conversation is pinned")
	}
	before := mustConvSet(t, u1, c[0], map[string]any{"notifyOff": true}).PinnedAt
	e.advance(time.Minute)
	after := mustConvSet(t, u1, c[0], map[string]any{"pinned": true})
	if !after.Pinned || after.PinnedAt != before {
		t.Errorf("re-pin: before %d, after %+v", before, after)
	}
	unpinned := mustConvSet(t, u1, c[0], map[string]any{"pinned": false})
	if unpinned.Pinned || unpinned.PinnedAt != 0 || !unpinned.NotifyOff {
		t.Errorf("unpin = %+v", unpinned)
	}
	if got := mustConvSet(t, u1, c[20], map[string]any{"pinned": true}); !got.Pinned || got.PinnedAt != e.now {
		t.Errorf("21st after unpin = %+v", got)
	}
}

// TC-CVS-01-03（AC-CVS-01-3）：501 个会话、最旧的置顶：返回 500 个且含它（第一个，pinned=true），truncated=true；其余是最近更新的 499 个。
func TestTC_CVS_01_03_PinnedSurvivesTruncation(t *testing.T) {
	if testing.Short() {
		t.Skip("creates 501 conversations")
	}
	e := newCVSEnv(t, nil)
	peers := make([]string, 501)
	for i := range peers {
		peers[i] = fmt.Sprintf("p%03d", i+1)
	}
	for i := 0; i < len(peers); i += 500 {
		e.users(peers[i:min(i+500, len(peers))]...)
	}
	u1 := e.dial("u1", "web")
	for i, p := range peers {
		e.setClock(t0 + int64(i+1)*1000)
		mustAck[protocol.SendAck](t, u1.sendText("c2c", p, fmt.Sprintf("cid-a-%04d", i), "hi"))
	}
	old := protocol.C2CConvID("u1", "p001")
	mustConvSet(t, u1, old, map[string]any{"pinned": true})
	ack := convsOf(t, u1)
	if len(ack.Convs) != 500 || !ack.Truncated || ack.Convs[0].ConvID != old || !ack.Convs[0].Pinned {
		t.Fatalf("got %d truncated=%v first=%+v", len(ack.Convs), ack.Truncated, ack.Convs[0])
	}
	if _, ok := lookupConv(ack, protocol.C2CConvID("u1", "p002")); ok {
		t.Error("the second oldest conversation must be cut off")
	}
	if ack.Convs[1].ConvID != protocol.C2CConvID("u1", "p501") || ack.Convs[499].ConvID != protocol.C2CConvID("u1", "p003") {
		t.Errorf("rest = %s .. %s", ack.Convs[1].ConvID, ack.Convs[499].ConvID)
	}
}

// TC-CVS-01-04（AC-CVS-01-4）：非参与方 4032、非成员 4033、没有消息的单聊 4040、已解散的群 4043、没有任何字段 4000；都没有写入，
// u1 的其他连接没有 convSettingsChanged（哨兵法）。
func TestTC_CVS_01_04_Errors(t *testing.T) {
	e := newCVSEnv(t, nil)
	gid, cs := e.selfGroup("o", nil, "u1", "u2")
	g2, cs2 := e.selfGroup("o2", nil, "u1", "u3")
	mustAck[protocol.Empty](t, cs2["o2"].call(protocol.CmdGroupDissolve, map[string]any{"groupId": g2}))
	e.usersNamed("u9")
	e.mutual("u1", "u2", "u3")
	u1, u1b, u9 := cs["u1"], e.dial("u1", "ios"), e.dial("u9", "web")
	c12 := mustAck[protocol.SendAck](t, u1.sendText("c2c", "u2", "cmid-cvs0104", "hi")).ConvID
	u1b.drain()
	mustErr(t, convSetCall(u9, c12, map[string]any{"pinned": true}), protocol.CodeNotConvParticipant)
	mustErr(t, convSetCall(u9, "group:"+gid, map[string]any{"notifyOff": true}), protocol.CodeNotGroupMember)
	mustErr(t, convSetCall(u1, protocol.C2CConvID("u1", "u3"), map[string]any{"pinned": true}), protocol.CodeNotFound)
	mustErr(t, convSetCall(u1, "group:"+g2, map[string]any{"notifyOff": true}), protocol.CodeGroupNotFound)
	mustErr(t, convSetCall(u1, c12, nil), protocol.CodeBadRequest)
	if n := e.settingCount("1 = 1"); n != 0 {
		t.Errorf("conv_settings has %d rows", n)
	}
	expectNoEventBefore(t, u1b, protocol.EventConvSettingsChanged, func() {
		mustAck[protocol.SendAck](t, cs["u2"].sendText("c2c", "u1", "cmid-cvs0104-s", "sentinel"))
	})
	for _, cv := range convsOf(t, u1).Convs {
		if cv.Pinned || cv.NotifyOff {
			t.Errorf("%s has settings: %+v", cv.ConvID, cv)
		}
	}
}

// TC-CVS-01-05（AC-CVS-01-5）：u1 把 g1 置顶并设免打扰；群主移出再拉回：convs 里 g1 pinned=false（没有 pinnedAt）、notifyOff=false，
// 数据库没有行；主动退群后被重新邀请同样。
func TestTC_CVS_01_05_RejoinDoesNotInherit(t *testing.T) {
	e := newCVSEnv(t, nil)
	gid, cs := e.selfGroup("o", nil, "u1", "u2")
	conv := "group:" + gid
	o, u1 := cs["o"], cs["u1"]
	for _, leave := range []func(){
		func() {
			mustAck[protocol.GroupKickAck](t, o.call(protocol.CmdGroupKick, map[string]any{"groupId": gid, "members": []string{"u1"}}))
		},
		func() { mustAck[protocol.Empty](t, u1.call(protocol.CmdGroupLeave, map[string]any{"groupId": gid})) },
	} {
		if got := mustConvSet(t, u1, conv, map[string]any{"pinned": true, "notifyOff": true}); !got.Pinned || !got.NotifyOff {
			t.Fatalf("convSet = %+v", got)
		}
		leave()
		if _, ok := e.settingRow("u1", conv); ok {
			t.Error("row survives leaving")
		}
		mustAck[protocol.GroupInviteAck](t, o.call(protocol.CmdGroupInvite, map[string]any{"groupId": gid, "members": []string{"u1"}}))
		cv, ok := lookupConv(convsOf(t, u1), conv)
		if !ok || cv.Pinned || cv.PinnedAt != 0 || cv.NotifyOff {
			t.Errorf("after rejoin: ok=%v %+v", ok, cv)
		}
	}
}

// TC-CVS-02-01 的 unread 部分（AC-CVS-02-1 部分通过；令牌无调用随 BE-490，等 C3）：u2 把和 u1 的单聊设为免打扰后离线，
// u1 发 1 条，u2 上线 convs：unread 增加 1，notifyOff=true。
func TestTC_CVS_02_01_UnreadUnderNotifyOff(t *testing.T) {
	e := newCVSEnv(t, nil)
	e.usersNamed("u1", "u2")
	e.mutual("u1", "u2")
	u1, u2 := e.dial("u1", "web"), e.dial("u2", "web")
	conv := mustAck[protocol.SendAck](t, u1.sendText("c2c", "u2", "cmid-cvs0201-a", "one")).ConvID
	before := mustConvSet(t, u2, conv, map[string]any{"notifyOff": true})
	if !before.NotifyOff || before.Unread != 1 {
		t.Fatalf("ack = %+v", before)
	}
	u2.ws.Close(websocket.StatusNormalClosure, "offline")
	mustAck[protocol.SendAck](t, u1.sendText("c2c", "u2", "cmid-cvs0201-b", "two"))
	cv, ok := lookupConv(convsOf(t, e.dial("u2", "ios")), conv)
	if !ok || cv.Unread != 2 || !cv.NotifyOff || cv.MaxSeq != 2 {
		t.Errorf("after reconnect: %+v", cv)
	}
}

// TC-CVS-04-04（AC-CVS-04-4）：1 秒内 6 次 convSet（不同会话），第 6 次 4290；1 秒后恢复；发送限流不受影响；/admin/test/limits/reset 能清。
func TestTC_CVS_04_04_RateLimit(t *testing.T) {
	e := newCVSEnv(t, func(c *app.Config) { c.ConvSettings.RatePerSec = 0 }) // 默认每人每秒 5 次
	e.usersNamed("u1")
	u1 := e.dial("u1", "web")
	c := e.friendsOf(u1, "r1", "r2", "r3", "r4", "r5", "r6")
	e.resetLimits("u1")
	for _, conv := range c[:5] {
		mustConvSet(t, u1, conv, map[string]any{"notifyOff": true})
	}
	mustErr(t, convSetCall(u1, c[5], map[string]any{"notifyOff": true}), protocol.CodeRateLimited)
	if _, ok := e.settingRow("u1", c[5]); ok {
		t.Error("the rejected convSet was written")
	}
	mustAck[protocol.SendAck](t, u1.sendText("c2c", "r6", "cmid-cvs0404-send", "send is not limited")) // 不计入发送限流
	e.advance(time.Second)
	mustConvSet(t, u1, c[5], map[string]any{"notifyOff": true})
	for range 4 {
		mustConvSet(t, u1, c[0], map[string]any{"pinned": true})
	}
	mustErr(t, convSetCall(u1, c[0], map[string]any{"pinned": true}), protocol.CodeRateLimited)
	e.resetLimits("u1")
	mustConvSet(t, u1, c[0], map[string]any{"pinned": false})
}
