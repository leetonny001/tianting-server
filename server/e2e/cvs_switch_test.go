package e2e

// C5 开关和成员变动的清理（BE-483）：TC-CVS-02-07、TC-CVS-01-05 和 TC-CVS-03-12 的清理部分、TC-CHAT-90-35。本地结果，CI 不运行。

import (
	"net/http"
	"strings"
	"testing"

	"tianting/server/internal/app"
	"tianting/server/internal/protocol"
)

// TC-CVS-02-07、TC-CHAT-90-35（AC-CVS-02-7、AC-CHAT-90-2 的 C5 部分）：IM_CONV_SETTINGS_ENABLED=0（默认）时 convSet、convDelete 403 / 4030
// featureDisabled，没有写入；格式错误仍先报 4000（校验顺序 格式 → 开关）；features.convSettings=false；没有设置时 convs 不带任何 C5 字段。
// 现有 e2e 在这个默认配置下不改代码全过（同一次 go test ./e2e 的结果）。
func TestTC_CVS_02_07_SwitchOff(t *testing.T) {
	e := newCVSEnv(t, func(c *app.Config) { c.ConvSettings.Enabled = false })
	e.usersNamed("u1", "u2")
	e.mutual("u1", "u2")
	u1, u2 := e.dial("u1", "web"), e.dial("u2", "web")
	ack := mustAck[protocol.SendAck](t, u1.sendText("c2c", "u2", "cmid-cvs0207", "hi"))
	conv := ack.ConvID
	for label, f := range map[string]frame{
		"convSet pinned":    convSetCall(u1, conv, map[string]any{"pinned": true}),
		"convSet notifyOff": convSetCall(u1, conv, map[string]any{"notifyOff": true}),
		"convDelete":        convDeleteCall(u1, conv, 1),
	} {
		if f.Error == nil || f.Error.Code != protocol.CodeForbidden || f.Error.Reason != protocol.ReasonFeatureDisabled {
			t.Errorf("%s: %+v", label, f.Error)
		}
	}
	mustErr(t, convSetCall(u1, conv, nil), protocol.CodeBadRequest)
	mustErr(t, u1.call(protocol.CmdConvDelete, map[string]any{"convId": conv}), protocol.CodeBadRequest)
	if n := e.settingCount("1 = 1"); n != 0 {
		t.Errorf("conv_settings has %d rows with the switch off", n)
	}
	if f := publicConfig(t, e.env)["features"].(map[string]any); f["convSettings"] != false {
		t.Errorf("features = %v", f)
	}
	for _, c := range []*client{u1, u2} {
		raw := string(c.call(protocol.CmdConvs, map[string]any{}).Data)
		for _, field := range []string{"pinned", "pinnedAt", "notifyOff", "deleted", "deletedTruncated"} {
			if strings.Contains(raw, `"`+field+`"`) {
				t.Errorf("%s: convs has %q with no settings: %s", c.uid, field, raw)
			}
		}
	}
	if m := msgBySeq(t, syncConv(t, u1, conv).Messages, ack.Seq); m.Body.Text != "hi" {
		t.Errorf("message changed: %+v", m)
	}
}

// 开关打开：features.convSettings=true（backend-chat 第 75 条）。
func TestCVSFeatureOn(t *testing.T) {
	e := newCVSEnv(t, nil)
	if f := publicConfig(t, e.env)["features"].(map[string]any); f["convSettings"] != true {
		t.Errorf("features = %v", f)
	}
}

// TC-CVS-01-05、TC-CVS-03-12 的清理部分（AC-CVS-01-5、AC-CVS-03-12，F-CVS-01 规则 6、F-CVS-03 规则 6a）：
// 群主移出、主动退群、后台移除、客户端解散、后台解散都在同一事务里删除这个成员（解散时整个群）的 conv_settings 行（含删除位置）；
// 其他成员、其他群、单聊的行不受影响。
func TestTC_CVS_01_05_CleanupOnLeave(t *testing.T) {
	e := newCVSEnv(t, nil)
	e.usersNamed("o", "u1", "u2", "u3")
	e.mutual("o", "u1", "u2", "u3")
	o, u1 := e.dial("o", "web"), e.dial("u1", "web")
	gids := make([]string, 5)
	for i := range gids {
		gids[i] = createG(t, o, "u1", "u2", "u3").GroupID
		for _, u := range []string{"u1", "u2"} {
			e.seedSetting(u, "group:"+gids[i], 100, true, 200, 1)
		}
	}
	mustAck[protocol.SendAck](t, u1.sendText("c2c", "u2", "cmid-cvs0105", "hi"))
	e.seedSetting("u1", protocol.C2CConvID("u1", "u2"), 100, true, 0, 0)

	mustAck[protocol.GroupKickAck](t, o.call(protocol.CmdGroupKick, map[string]any{"groupId": gids[0], "members": []string{"u1"}}))
	mustAck[protocol.Empty](t, u1.call(protocol.CmdGroupLeave, map[string]any{"groupId": gids[1]}))
	if code, b := e.adminDo(http.MethodDelete, "/admin/groups/"+gids[2]+"/members", map[string]any{"members": []string{"u1"}}, adminKey); code != http.StatusOK {
		t.Fatalf("admin remove: %d %s", code, b)
	}
	mustAck[protocol.Empty](t, o.call(protocol.CmdGroupDissolve, map[string]any{"groupId": gids[3]}))
	if code, b := e.dissolve(gids[4]); code != http.StatusOK {
		t.Fatalf("admin dissolve: %d %s", code, b)
	}
	for i, gid := range gids {
		if _, ok := e.settingRow("u1", "group:"+gid); ok {
			t.Errorf("group %d: u1 still has a conv_settings row", i)
		}
		_, ok := e.settingRow("u2", "group:"+gid)
		if want := i < 3; ok != want {
			t.Errorf("group %d: u2 row = %v, want %v (dissolve removes the whole group)", i, ok, want)
		}
	}
	if _, ok := e.settingRow("u1", protocol.C2CConvID("u1", "u2")); !ok {
		t.Error("the c2c row was removed")
	}
}
