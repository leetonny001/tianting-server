package e2e

// C5 和注销（BE-488 第 5 条，P-46 ③，backend-chat 第 82 条）：account_cleanup 第 ② 步删除这个用户的全部会话设置
// （单聊的置顶、免打扰、删除位置，和群的一起）；对方的设置不动。本地结果，CI 不运行。

import (
	"testing"

	"tianting/server/internal/app"
	"tianting/server/internal/protocol"
)

func TestCVSAccountCleanupRemovesSettings(t *testing.T) {
	e := openEnv(t, func(c *app.Config) { c.ConvSettings.Enabled = true; c.ConvSettings.RatePerSec = 1000 })
	clk := e.freeze()
	a, b := e.register("alice_1", nil), e.register("bobby_1", nil)
	e.createGroup("g1", "alice_1", "bobby_1")
	ca, cb := e.authOK("a", a.AccessToken, testPlatform), e.authOK("b", b.AccessToken, testPlatform)
	conv := mustAck[protocol.SendAck](t, ca.sendText("c2c", "bobby_1", "msg-cvs-1", "hi")).ConvID
	mustAck[protocol.SendAck](t, ca.sendText("c2c", "alice_1", "msg-cvs-2", "self")) // 自己的单聊
	mustConvSet(t, ca, conv, map[string]any{"pinned": true, "notifyOff": true})
	mustConvSet(t, ca, "group:g1", map[string]any{"notifyOff": true})
	mustConvDelete(t, ca, protocol.C2CConvID("alice_1", "alice_1"), 1)
	mustConvSet(t, cb, conv, map[string]any{"notifyOff": true})
	if n := e.count(`SELECT COUNT(*) FROM conv_settings WHERE user_id = 'alice_1'`); n != 3 {
		t.Fatalf("control: alice has %d rows, want 3", n)
	}
	e.mustDelete(a.AccessToken, goodPassword)
	clk.advance(8 * oneDay)
	e.cleanup()
	if n := e.count(`SELECT COUNT(*) FROM conv_settings WHERE user_id = 'alice_1'`); n != 0 {
		t.Errorf("alice still has %d conv_settings rows after cleanup", n)
	}
	if n := e.count(`SELECT COUNT(*) FROM conv_settings WHERE user_id = 'bobby_1' AND notify_off = 1`); n != 1 {
		t.Errorf("bob's setting = %d rows, want 1", n)
	}
}
