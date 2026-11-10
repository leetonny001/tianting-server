package e2e

// C5 多端同步（BE-486）：TC-CVS-04-01、04-02、04-05 的服务端部分（客户端部分随 FE-680～686）。环境 E1（cvsEnv）。本地结果，CI 不运行。

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/coder/websocket"

	"tianting/server/internal/protocol"
)

// TC-CVS-04-01（AC-CVS-04-1）：D1 置顶 c1、再设免打扰：D2 2 秒内（真实时间）收到两条 convSettingsChanged（pinned=true 带 pinnedAt；notifyOff=true），
// 帧符合 Schema；D1 只拿 ack，不收事件（哨兵法）；对方 u2 也不收。幂等的再次置顶也发，内容是当前状态；取消置顶时没有 pinnedAt。
func TestTC_CVS_04_01_SettingsEvent(t *testing.T) {
	e := newCVSEnv(t, nil)
	schema := frameSchema(t)
	e.usersNamed("u1", "u2")
	e.mutual("u1", "u2")
	d1, d2, u2 := e.dial("u1", "web"), e.dial("u1", "ios"), e.dial("u2", "web")
	conv := mustAck[protocol.SendAck](t, d1.sendText("c2c", "u2", "cmid-cvs0401", "hi")).ConvID
	d2.drain()
	u2.drain()
	start := time.Now()
	pin := mustConvSet(t, d1, conv, map[string]any{"pinned": true})
	raw := d2.waitEvent(protocol.EventConvSettingsChanged)
	ev := eventOf[protocol.ConvSettingsChangedEvent](t, raw)
	if !ev.Pinned || ev.PinnedAt != pin.PinnedAt || ev.NotifyOff || ev.ConvID != conv {
		t.Errorf("first event %s", raw)
	}
	validFrame(t, schema, "convSettingsChanged", map[string]any{"cmd": "event", "data": json.RawMessage(raw)})
	mustConvSet(t, d1, conv, map[string]any{"notifyOff": true})
	raw = d2.waitEvent(protocol.EventConvSettingsChanged)
	if ev := eventOf[protocol.ConvSettingsChangedEvent](t, raw); !ev.Pinned || ev.PinnedAt != pin.PinnedAt || !ev.NotifyOff {
		t.Errorf("second event %s", raw)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("events after %v (AC-CVS-04-1: within 2 seconds)", d)
	}
	mustConvSet(t, d1, conv, map[string]any{"pinned": true}) // 幂等，也发
	if ev := eventOf[protocol.ConvSettingsChangedEvent](t, d2.waitEvent(protocol.EventConvSettingsChanged)); !ev.Pinned || ev.PinnedAt != pin.PinnedAt {
		t.Errorf("idempotent event %+v", ev)
	}
	mustConvSet(t, d1, conv, map[string]any{"pinned": false})
	raw = d2.waitEvent(protocol.EventConvSettingsChanged)
	if ev := eventOf[protocol.ConvSettingsChangedEvent](t, raw); ev.Pinned || ev.PinnedAt != 0 || !ev.NotifyOff {
		t.Errorf("unpin event %s", raw)
	}
	validFrame(t, schema, "convSettingsChanged unpinned", map[string]any{"cmd": "event", "data": json.RawMessage(raw)})
	for _, c := range []*client{d1, u2} {
		expectNoEventBefore(t, c, protocol.EventConvSettingsChanged, func() {
			mustAck[protocol.SendAck](t, d2.sendText("c2c", "u2", "cmid-cvs0401-s-"+c.uid, "sentinel"))
		})
	}
}

// TC-CVS-04-05（AC-CVS-04-5）：D1 删除单聊：D2 2 秒内收到 convDeleted {convId, clearedSeq}，符合 Schema；D2 不另收 readUpdated、convSettingsChanged
// （第 84 条）；D1 只拿 ack。D2 之后的 convs 里会话消失、在 deleted。
func TestTC_CVS_04_05_DeletedEvent(t *testing.T) {
	e := newCVSEnv(t, nil)
	e.usersNamed("u1", "u2")
	e.mutual("u1", "u2")
	d1, d2, u2 := e.dial("u1", "web"), e.dial("u1", "ios"), e.dial("u2", "web")
	conv := e.c2cTen(d1, u2, "cvs0405")
	mustConvSet(t, d1, conv, map[string]any{"pinned": true})
	d2.waitEvent(protocol.EventConvSettingsChanged)
	d1.drain()
	d2.drain()
	start := time.Now()
	if got := mustConvDelete(t, d1, conv, 10); got != 10 {
		t.Fatalf("clearedSeq = %d", got)
	}
	var raw json.RawMessage
	for raw == nil { // 只看事件，记下中间有没有 readUpdated、convSettingsChanged
		f := d2.waitCmd(protocol.CmdEvent)
		var head struct{ Type string }
		json.Unmarshal(f.Data, &head)
		switch head.Type {
		case protocol.EventConvDeleted:
			raw = f.Data
		case protocol.EventReadUpdated, protocol.EventConvSettingsChanged:
			t.Errorf("D2 got %s with the delete: %s", head.Type, f.Data)
		}
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("convDeleted after %v", d)
	}
	if ev := eventOf[protocol.ConvDeletedEvent](t, raw); ev.ConvID != conv || ev.ClearedSeq != 10 {
		t.Errorf("event %s", raw)
	}
	validFrame(t, frameSchema(t), "convDeleted", map[string]any{"cmd": "event", "data": json.RawMessage(raw)})
	for _, typ := range []string{protocol.EventReadUpdated, protocol.EventConvSettingsChanged} {
		expectNoEventBefore(t, d2, typ, func() {
			mustAck[protocol.SendAck](t, u2.sendText("c2c", "u1", "cmid-cvs0405-s-"+typ, "sentinel"))
		})
	}
	expectNoEventBefore(t, d1, protocol.EventConvDeleted, func() {
		mustAck[protocol.SendAck](t, u2.sendText("c2c", "u1", "cmid-cvs0405-s-d1", "sentinel"))
	})
	ack := convsOf(t, d2)
	if cv, ok := lookupConv(ack, conv); !ok || cv.MinSeq != 11 || cv.Pinned { // 哨兵消息让它重新出现，只剩新消息
		t.Errorf("D2 convs after the sentinels: ok %v %+v", ok, cv)
	}
}

// TC-CVS-04-02（AC-CVS-04-2）：D2 离线时 D1 置顶 c1、删除 c2：D2 重连后 convs 和 D1 完全相同（顺序、置顶、免打扰、deleted）。
func TestTC_CVS_04_02_OfflineDeviceCatchesUp(t *testing.T) {
	e := newCVSEnv(t, nil)
	e.usersNamed("u1")
	d1, d2 := e.dial("u1", "web"), e.dial("u1", "ios")
	c := e.friendsOf(d1, "s1", "s2", "s3")
	d2.ws.Close(websocket.StatusNormalClosure, "offline")
	mustConvSet(t, d1, c[0], map[string]any{"pinned": true, "notifyOff": true})
	mustConvDelete(t, d1, c[1], 1)
	want := string(d1.call(protocol.CmdConvs, map[string]any{}).Data)
	got := string(e.dial("u1", "ios").call(protocol.CmdConvs, map[string]any{}).Data)
	if got != want {
		t.Errorf("D2 after reconnect:\n%s\nD1:\n%s", got, want)
	}
	var ack protocol.ConvsAck
	json.Unmarshal([]byte(got), &ack)
	if convOrder(ack)[0] != c[0] || len(ack.Convs) != 2 || len(ack.Deleted) != 1 || ack.Deleted[0].ConvID != c[1] {
		t.Errorf("convs = %s", got)
	}
}
