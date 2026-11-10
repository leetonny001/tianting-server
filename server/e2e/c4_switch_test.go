package e2e

// C4 开关（BE-471）：TC-RCL-01-10、TC-CHAT-90-28 的开关部分。本地结果，CI 不运行。

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"tianting/server/internal/app"
	"tianting/server/internal/gateway"
	"tianting/server/internal/logic"
	"tianting/server/internal/protocol"
)

func publicConfig(t *testing.T, e *env) map[string]any {
	t.Helper()
	resp, err := http.Get(e.srv.URL + "/v1/config")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var cfg map[string]any
	json.Unmarshal(b, &cfg)
	return cfg
}

// TC-RCL-01-10 ①、TC-CHAT-90-28（AC-RCL-01-10、AC-CHAT-90-2 的 C4 部分）：IM_MSG_ACTIONS_ENABLED=0（默认）时 recall 403 / 4030 featureDisabled，
// send 带 quote、mentions 同样；features.msgActions=false；convs 不带 mentionUnread；groupMembers 的 keyword 照常可用（只读，backend-chat 第 59 条）。
// 现有 e2e 在这个默认配置下不改代码全过（同一次 go test ./e2e 的结果）。
func TestTC_RCL_01_10_SwitchOff(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("u1", "u2", "zhang1", "li")
	s.mutual("u1", "u2", "zhang1", "li")
	u1, u2 := s.dial("u1", "web"), s.dial("u2", "web")
	ack := mustAck[protocol.SendAck](t, u1.sendText("c2c", "u2", "cmid-s0001", "hi"))
	for label, f := range map[string]frame{
		"recall":   recallCall(u1, ack.ConvID, ack.MsgID),
		"quote":    sendQuote(u1, "c2c", "u2", "cmid-s0002", "re", ack.MsgID),
		"mentions": sendMentions(u1, "nope", "cmid-s0003", "@u2", []string{"u2"}, false),
	} {
		if f.Error == nil || f.Error.Code != protocol.CodeForbidden || f.Error.Reason != protocol.ReasonFeatureDisabled {
			t.Errorf("%s: %+v", label, f.Error)
		}
	}
	if m := msgBySeq(t, syncConv(t, u2, ack.ConvID).Messages, ack.Seq); m.Deleted || m.Body.Text != "hi" {
		t.Errorf("message changed: %+v", m)
	}
	raw := u2.call(protocol.CmdConvs, map[string]any{})
	if strings.Contains(string(raw.Data), "mentionUnread") {
		t.Errorf("convs with the switch off: %s", raw.Data)
	}
	if f := publicConfig(t, s.env)["features"].(map[string]any); f["msgActions"] != false {
		t.Errorf("features = %v", f)
	}
	gid := createG(t, u1, "u2", "zhang1", "li").GroupID
	var page protocol.GroupMembersAck
	page = mustAck[protocol.GroupMembersAck](t, u2.call(protocol.CmdGroupMembers, map[string]any{"groupId": gid, "keyword": "zh"}))
	if len(page.Members) != 1 || page.Members[0].UserID != "zhang1" {
		t.Errorf("keyword with the switch off: %+v", page.Members)
	}
}

// TC-RCL-01-10 ②（AC-RCL-01-10）：开关打开但没有隔离密钥，im-server 启动失败（app 层；config 层的检查见 internal/config 的同名用例）。
// ③：开关打开、IM_V13_ADMIN_FEATURES=0 也能撤回（RC2-15）；features.msgActions=true。
func TestTC_RCL_01_10_StartupAndNoV13(t *testing.T) {
	_, err := app.New(app.Config{
		DBPath: filepath.Join(t.TempDir(), "im.db"), AutoMigrate: true, JWTSecret: "e2e-secret-0123456789", AdminKey: adminKey,
		Gateway: gateway.DefaultConfig(), TestMode: true, MsgActions: msgActionsOn(),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err == nil || !strings.Contains(err.Error(), "IM_QUARANTINE_KEY") {
		t.Fatalf("startup without a quarantine key: %v", err)
	}
	e := newEnvCfg(t, func(c *app.Config) {
		c.MsgActions = msgActionsOn()
		c.QuarantineKey = testQuarantineKey
		c.V13AdminFeatures = false
	})
	e.users("u2")
	u1, u2 := e.dial("u1", "web"), e.dial("u2", "web")
	ack := mustAck[protocol.SendAck](t, u1.sendText("c2c", "u2", "cmid-s0101", "hi"))
	mustRecall(t, u1, ack.ConvID, ack.MsgID)
	if ev, _ := waitMsgDeleted(t, u2); ev.Recalled == nil || ev.Recalled.By != "u1" {
		t.Errorf("event = %+v", ev)
	}
	if f := publicConfig(t, e)["features"].(map[string]any); f["msgActions"] != true {
		t.Errorf("features = %v", f)
	}
}

func msgActionsOn() logic.MsgActionsConfig { return logic.MsgActionsConfig{Enabled: true} }
