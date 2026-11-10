package e2e

// C5 会话置顶、免打扰、删除的 e2e 公共设施（BE-483～BE-486）。环境 E1：测试模式、SQLite、IM_CONV_SETTINGS_ENABLED=1；
// 涉及 @ 的用例要 IM_MSG_ACTIONS_ENABLED=1，所以在 c4Env 上加 C5 的开关（隔离密钥用测试密钥）。时钟是 [IM 钟]（冻结在 t0）。
// 本地结果，CI 不运行。

import (
	"encoding/json"
	"testing"

	"tianting/server/internal/app"
	"tianting/server/internal/protocol"
)

type cvsEnv struct {
	*c4Env
}

// newCVSEnv：会话设置的频率调高（冻结时钟下令牌不恢复）；只有测频率的用例用默认值。
func newCVSEnv(t *testing.T, mutate func(*app.Config)) *cvsEnv {
	t.Helper()
	return &cvsEnv{c4Env: newC4Env(t, func(c *app.Config) {
		c.ConvSettings.Enabled = true
		c.ConvSettings.RatePerSec = 10000
		if mutate != nil {
			mutate(c)
		}
	})}
}

func convSetCall(c *client, convID string, fields map[string]any) frame {
	c.t.Helper()
	data := map[string]any{"convId": convID}
	for k, v := range fields {
		data[k] = v
	}
	return c.call(protocol.CmdConvSet, data)
}

func mustConvSet(t *testing.T, c *client, convID string, fields map[string]any) protocol.Conversation {
	t.Helper()
	return mustAck[protocol.ConvSetAck](t, convSetCall(c, convID, fields)).Conv
}

func convDeleteCall(c *client, convID string, upToSeq int64) frame {
	c.t.Helper()
	return c.call(protocol.CmdConvDelete, map[string]any{"convId": convID, "upToSeq": upToSeq})
}

func mustConvDelete(t *testing.T, c *client, convID string, upToSeq int64) int64 {
	t.Helper()
	return mustAck[protocol.ConvDeleteAck](t, convDeleteCall(c, convID, upToSeq)).ClearedSeq
}

// lookupConv 在 convs 里找一个会话；没有时 ok=false（删除后隐藏的会话不在列表里）。
func lookupConv(a protocol.ConvsAck, convID string) (protocol.Conversation, bool) {
	for _, x := range a.Convs {
		if x.ConvID == convID {
			return x, true
		}
	}
	return protocol.Conversation{}, false
}

func deletedSeq(a protocol.ConvsAck, convID string) (int64, bool) {
	for _, d := range a.Deleted {
		if d.ConvID == convID {
			return d.ClearedSeq, true
		}
	}
	return 0, false
}

func convOrder(a protocol.ConvsAck) []string {
	out := make([]string, len(a.Convs))
	for i, x := range a.Convs {
		out[i] = x.ConvID
	}
	return out
}

// settingRow 读数据库里的 conv_settings 行（"没有写入""行不存在"直接查库）。
func (e *cvsEnv) settingRow(uid, convID string) (row struct{ PinnedAt, NotifyOff, DeletedAt, ClearedSeq int64 }, ok bool) {
	e.t.Helper()
	err := e.db().QueryRow(`SELECT pinned_at, notify_off, deleted_at, cleared_seq FROM conv_settings WHERE user_id = ? AND conv_id = ?`, uid, convID).
		Scan(&row.PinnedAt, &row.NotifyOff, &row.DeletedAt, &row.ClearedSeq)
	return row, err == nil
}

func (e *cvsEnv) settingCount(where string, args ...any) int {
	e.t.Helper()
	var n int
	if err := e.db().QueryRow(`SELECT count(*) FROM conv_settings WHERE `+where, args...).Scan(&n); err != nil {
		e.t.Fatal(err)
	}
	return n
}

// seedSetting 直接写一行设置（只用于验证清理路径和开关关闭后的行为，不经过 convSet）。
func (e *cvsEnv) seedSetting(uid, convID string, pinnedAt int64, notifyOff bool, deletedAt, clearedSeq int64) {
	e.t.Helper()
	off := 0
	if notifyOff {
		off = 1
	}
	if _, err := e.db().Exec(`INSERT INTO conv_settings (user_id, conv_id, pinned_at, notify_off, deleted_at, cleared_seq) VALUES (?, ?, ?, ?, ?, ?)`,
		uid, convID, pinnedAt, off, deletedAt, clearedSeq); err != nil {
		e.t.Fatal(err)
	}
}

// eventOf 解析一个事件。
func eventOf[T any](t *testing.T, raw json.RawMessage) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// expectNoEventBefore 用哨兵法断言 c 在收到 sentinel 产生的 msg 之前没有 typ 事件（第 4.4 节）。
func expectNoEventBefore(t *testing.T, c *client, typ string, sentinel func()) {
	t.Helper()
	check := func(f frame) {
		if f.Cmd != protocol.CmdEvent {
			return
		}
		var head struct{ Type string }
		json.Unmarshal(f.Data, &head)
		if head.Type == typ {
			t.Fatalf("%s: unexpected %s %s", c.uid, typ, f.Data)
		}
	}
	for _, f := range c.pending {
		check(f)
	}
	c.pending = nil
	sentinel()
	for {
		f := c.next()
		if f.Cmd == protocol.CmdMsg {
			return
		}
		check(f)
	}
}
