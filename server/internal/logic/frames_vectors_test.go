package logic

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"tianting/server/internal/protocol"
)

// TC-CHAT-90-09（AC-CHAT-90-10 的服务端部分，BE-403）：frames.json 里 cmd=send 的用例用服务端真实的帧解析跑一遍——
// 和 gateway、Handle、send 同样的顺序：InFrame、缺 id、严格解码（DisallowUnknownFields）、SendReq.Validate。
// valid 必须通过，invalid 必须被拒绝，所以 Schema 和服务端的判断（F-MOD-02 规则 4a 的 MsgBody 互斥）不会各说各的。
// image、file 的下行 msg 帧和 ack sync 里的消息用服务端的模型解码再编码，结果和用例相同（服务端发得出这些帧）。
func TestFrameVectorsServerParse(t *testing.T) {
	raw, err := os.ReadFile("../../../protocol/vectors/frames.json")
	if err != nil {
		t.Fatal(err)
	}
	var vf struct {
		Valid, Invalid []struct {
			Name  string          `json:"name"`
			Frame json.RawMessage `json:"frame"`
		}
	}
	if err := json.Unmarshal(raw, &vf); err != nil {
		t.Fatal(err)
	}
	parse := func(frame json.RawMessage) *protocol.Error {
		var in protocol.InFrame
		if err := json.Unmarshal(frame, &in); err != nil || in.Cmd == "" {
			return protocol.Errorf(protocol.CodeInvalidFrame, "invalid frame")
		}
		if in.ID == "" {
			return protocol.Errorf(protocol.CodeMissingRequestID, "missing id")
		}
		req, perr := decode[protocol.SendReq](in.Data)
		if perr != nil {
			return perr
		}
		return req.Validate()
	}
	isSend := func(frame json.RawMessage) bool {
		var f struct{ Cmd string }
		_ = json.Unmarshal(frame, &f)
		return f.Cmd == protocol.CmdSend
	}
	var sends, files int
	for _, v := range vf.Valid {
		if isSend(v.Frame) {
			sends++
			if perr := parse(v.Frame); perr != nil {
				t.Errorf("valid/%s: server rejects: %v", v.Name, perr)
			}
		}
		for _, m := range fileMessages(t, v.Frame) {
			files++
			var msg protocol.Message
			if err := json.Unmarshal(m, &msg); err != nil {
				t.Errorf("valid/%s: decode: %v", v.Name, err)
				continue
			}
			got, _ := json.Marshal(msg)
			if !sameJSON(t, got, m) {
				t.Errorf("valid/%s: server encodes\n%s\nwant\n%s", v.Name, got, m)
			}
		}
	}
	for _, v := range vf.Invalid {
		if isSend(v.Frame) {
			sends++
			if parse(v.Frame) == nil {
				t.Errorf("invalid/%s: server accepts", v.Name)
			}
		}
	}
	if sends < 20 || files < 5 {
		t.Fatalf("frames.json has %d send and %d image/file message vectors; the C2 cases are missing", sends, files)
	}
}

// fileMessages 取出帧里 type 为 image、file 的下行消息（msg 帧、ack sync 的 messages）。
func fileMessages(t *testing.T, frame json.RawMessage) []json.RawMessage {
	t.Helper()
	var f struct {
		Cmd  string          `json:"cmd"`
		Data json.RawMessage `json:"data"`
	}
	_ = json.Unmarshal(frame, &f)
	var list []json.RawMessage
	switch f.Cmd {
	case protocol.CmdMsg:
		list = []json.RawMessage{f.Data}
	case protocol.CmdAck:
		var a struct {
			Messages []json.RawMessage `json:"messages"`
		}
		_ = json.Unmarshal(f.Data, &a)
		list = a.Messages
	}
	var out []json.RawMessage
	for _, m := range list {
		var h struct{ Type string }
		_ = json.Unmarshal(m, &h)
		if h.Type == protocol.MsgImage || h.Type == protocol.MsgFile {
			out = append(out, m)
		}
	}
	return out
}

func sameJSON(t *testing.T, a, b []byte) bool {
	t.Helper()
	norm := func(x []byte) string {
		var v any
		dec := json.NewDecoder(bytes.NewReader(x))
		dec.UseNumber()
		if err := dec.Decode(&v); err != nil {
			t.Fatal(err)
		}
		out, _ := json.Marshal(v)
		return strings.TrimSpace(string(out))
	}
	return norm(a) == norm(b)
}

// TC-CHAT-90-24（AC-CHAT-90-10 的服务端部分，C3，BE-450）：frames.json 里设备通知命令和 presence 的用例用服务端真实的解析跑一遍
// （严格解码 + Validate），valid 必须通过、invalid 必须被拒绝；还要断言用例覆盖了任务单要求的情形。
func TestTC_CHAT_90_24_DeviceFrameVectorsServerParse(t *testing.T) {
	raw, err := os.ReadFile("../../../protocol/vectors/frames.json")
	if err != nil {
		t.Fatal(err)
	}
	var vf struct {
		Valid, Invalid []struct {
			Name  string          `json:"name"`
			Frame json.RawMessage `json:"frame"`
		}
	}
	if err := json.Unmarshal(raw, &vf); err != nil {
		t.Fatal(err)
	}
	parse := func(frame json.RawMessage) (string, *protocol.Error) {
		var in protocol.InFrame
		if err := json.Unmarshal(frame, &in); err != nil || in.Cmd == "" {
			return "", protocol.Errorf(protocol.CodeInvalidFrame, "invalid frame")
		}
		if in.ID == "" {
			return in.Cmd, protocol.Errorf(protocol.CodeMissingRequestID, "missing id")
		}
		switch in.Cmd {
		case protocol.CmdDeviceNotificationRegister:
			req, perr := decode[protocol.DeviceNotificationRegisterReq](in.Data)
			if perr != nil {
				return in.Cmd, perr
			}
			_, perr = req.Validate()
			return in.Cmd, perr
		case protocol.CmdDeviceNotificationUnregister:
			_, perr := decode[protocol.DeviceNotificationUnregisterReq](in.Data)
			return in.Cmd, perr
		case protocol.CmdDeviceNotificationSettings:
			_, perr := decode[protocol.DeviceNotificationSettingsReq](in.Data)
			return in.Cmd, perr
		case protocol.CmdPresence:
			req, perr := decode[protocol.PresenceReq](in.Data)
			if perr != nil {
				return in.Cmd, perr
			}
			return in.Cmd, req.Validate()
		}
		return "", nil
	}
	cmdOf := func(frame json.RawMessage) string {
		var f struct{ Cmd string }
		_ = json.Unmarshal(frame, &f)
		return f.Cmd
	}
	device := map[string]bool{protocol.CmdDeviceNotificationRegister: true, protocol.CmdDeviceNotificationUnregister: true,
		protocol.CmdDeviceNotificationSettings: true, protocol.CmdPresence: true}
	seen := map[string]int{}
	for _, v := range vf.Valid {
		if cmd := cmdOf(v.Frame); device[cmd] {
			seen["valid/"+cmd]++
			if _, perr := parse(v.Frame); perr != nil {
				t.Errorf("valid/%s: server rejects: %v", v.Name, perr)
			}
		}
	}
	for _, v := range vf.Invalid {
		if cmd := cmdOf(v.Frame); device[cmd] {
			seen["invalid/"+cmd]++
			if _, perr := parse(v.Frame); perr == nil {
				t.Errorf("invalid/%s: server accepts", v.Name)
			} else if perr.Code != protocol.CodeBadRequest && perr.Code != protocol.CodeMissingRequestID {
				t.Errorf("invalid/%s: code %d, want 4000", v.Name, perr.Code)
			}
		}
	}
	for cmd := range device {
		if seen["valid/"+cmd] == 0 || seen["invalid/"+cmd] == 0 {
			t.Errorf("frames.json lacks valid or invalid vectors for %s: %v", cmd, seen)
		}
	}
	if seen["valid/"+protocol.CmdDeviceNotificationRegister] < 3 || seen["invalid/"+protocol.CmdDeviceNotificationRegister] < 5 {
		t.Errorf("register vectors must cover provider, env default, empty and over-long tokens: %v", seen)
	}
}

// TC-CHAT-90-32（AC-CHAT-90-10 的 C4 服务端部分，BE-470）、TC-RCL-03-07（AC-RCL-03-7）：frames.json 里名字以"C4）"结尾的用例用服务端真实的解析跑一遍。
// 上行 recall、groupMembers 走严格解码 + Validate（send 已在 TestFrameVectorsServerParse 里跑过）；valid 必须通过、invalid 必须被拒绝。
// 下行的 msg、ack sync、ack convs、msgDeleted、convAdded 用服务端的模型解码再编码，结果和用例逐字段相同（服务端发得出这些帧，
// 撤回的消息没有 body、deleted 和 recalled 都是 true）。
func TestTC_CHAT_90_32_C4FrameVectorsServerParse(t *testing.T) {
	raw, err := os.ReadFile("../../../protocol/vectors/frames.json")
	if err != nil {
		t.Fatal(err)
	}
	var vf struct {
		Valid, Invalid []struct {
			Name  string          `json:"name"`
			Frame json.RawMessage `json:"frame"`
		}
	}
	if err := json.Unmarshal(raw, &vf); err != nil {
		t.Fatal(err)
	}
	type head struct {
		ID   string          `json:"id"`
		Cmd  string          `json:"cmd"`
		Data json.RawMessage `json:"data"`
	}
	parseUp := func(h head) *protocol.Error {
		if h.Cmd != protocol.CmdRecall && h.Cmd != protocol.CmdGroupMembers {
			return nil
		}
		if h.ID == "" {
			return protocol.Errorf(protocol.CodeMissingRequestID, "missing id")
		}
		switch h.Cmd {
		case protocol.CmdRecall:
			req, perr := decode[protocol.RecallReq](h.Data)
			if perr != nil {
				return perr
			}
			return req.Validate()
		case protocol.CmdGroupMembers:
			_, perr := decode[protocol.GroupMembersPageReq](h.Data)
			return perr
		}
		return nil
	}
	// roundTrip 用服务端的类型解码再编码，返回编码结果和用例原文。
	roundTrip := func(t *testing.T, name string, h head) {
		t.Helper()
		check := func(v any, want json.RawMessage) {
			t.Helper()
			if err := json.Unmarshal(want, v); err != nil {
				t.Errorf("%s: decode: %v", name, err)
				return
			}
			got, _ := json.Marshal(v)
			if !sameJSON(t, got, want) {
				t.Errorf("%s: server encodes\n%s\nwant\n%s", name, got, want)
			}
		}
		switch h.Cmd {
		case protocol.CmdMsg:
			check(&protocol.Message{}, h.Data)
		case protocol.CmdAck:
			var probe map[string]json.RawMessage
			_ = json.Unmarshal(h.Data, &probe)
			switch {
			case probe["messages"] != nil:
				check(&protocol.SyncAck{}, h.Data)
			case probe["convs"] != nil:
				check(&protocol.ConvsAck{}, h.Data)
			}
		case protocol.CmdEvent:
			var ev struct{ Type string }
			_ = json.Unmarshal(h.Data, &ev)
			switch ev.Type {
			case protocol.EventMsgDeleted:
				check(&protocol.MsgDeletedEvent{}, h.Data)
			case protocol.EventConvAdded:
				check(&protocol.ConvAddedEvent{}, h.Data)
			}
		}
	}
	seen := map[string]int{}
	for _, v := range vf.Valid {
		if !strings.Contains(v.Name, "C4）") {
			continue
		}
		var h head
		_ = json.Unmarshal(v.Frame, &h)
		seen["valid/"+h.Cmd]++
		if perr := parseUp(h); perr != nil {
			t.Errorf("valid/%s: server rejects: %v", v.Name, perr)
		}
		roundTrip(t, "valid/"+v.Name, h)
	}
	for _, v := range vf.Invalid {
		if !strings.Contains(v.Name, "C4）") {
			continue
		}
		var h head
		_ = json.Unmarshal(v.Frame, &h)
		seen["invalid/"+h.Cmd]++
		if h.Cmd != protocol.CmdRecall && h.Cmd != protocol.CmdGroupMembers {
			continue // send 在 TestFrameVectorsServerParse；下行的非法帧只给 Schema 和 SDK 用
		}
		if parseUp(h) == nil {
			t.Errorf("invalid/%s: server accepts", v.Name)
		}
	}
	for k, n := range map[string]int{"valid/recall": 2, "invalid/recall": 5, "valid/send": 7, "invalid/send": 12, "valid/msg": 9,
		"invalid/msg": 7, "valid/ack": 3, "valid/event": 2, "valid/groupMembers": 2, "invalid/groupMembers": 1} {
		if seen[k] < n {
			t.Errorf("frames.json has %d C4 vectors of %s, want >= %d (BE-470 第 5 条): %v", seen[k], k, n, seen)
		}
	}
}

// TC-CHAT-90-38（AC-CHAT-90-10 的 C5 服务端部分，BE-482）：frames.json 里名字以"C5）"结尾的用例用服务端真实的解析跑一遍。
// 上行 convSet、convDelete 走严格解码 + Validate；valid 必须通过、invalid 必须被拒绝（4000、4007 或缺 id 的 4003），
// 所以"都不带字段""upToSeq 为负"这类判断 Schema 和服务端一致。下行的 ack convSet / convDelete / convs、convSettingsChanged、
// convDeleted 用服务端的模型解码再编码，结果和用例逐字段相同（服务端发得出这些帧：没有设置时不带 pinned、notifyOff）。
func TestTC_CHAT_90_38_C5FrameVectorsServerParse(t *testing.T) {
	raw, err := os.ReadFile("../../../protocol/vectors/frames.json")
	if err != nil {
		t.Fatal(err)
	}
	var vf struct {
		Valid, Invalid []struct {
			Name  string          `json:"name"`
			Frame json.RawMessage `json:"frame"`
		}
	}
	if err := json.Unmarshal(raw, &vf); err != nil {
		t.Fatal(err)
	}
	type head struct {
		ID   string          `json:"id"`
		Cmd  string          `json:"cmd"`
		Data json.RawMessage `json:"data"`
	}
	isUp := func(cmd string) bool { return cmd == protocol.CmdConvSet || cmd == protocol.CmdConvDelete }
	parseUp := func(h head) *protocol.Error {
		if h.ID == "" {
			return protocol.Errorf(protocol.CodeMissingRequestID, "missing id")
		}
		switch h.Cmd {
		case protocol.CmdConvSet:
			req, perr := decode[protocol.ConvSetReq](h.Data)
			if perr != nil {
				return perr
			}
			return req.Validate()
		case protocol.CmdConvDelete:
			req, perr := decode[protocol.ConvDeleteReq](h.Data)
			if perr != nil {
				return perr
			}
			return req.Validate()
		}
		return nil
	}
	roundTrip := func(t *testing.T, name string, h head) string {
		t.Helper()
		check := func(v any) {
			t.Helper()
			if err := json.Unmarshal(h.Data, v); err != nil {
				t.Errorf("%s: decode: %v", name, err)
				return
			}
			got, _ := json.Marshal(v)
			if !sameJSON(t, got, h.Data) {
				t.Errorf("%s: server encodes\n%s\nwant\n%s", name, got, h.Data)
			}
		}
		var probe map[string]json.RawMessage
		_ = json.Unmarshal(h.Data, &probe)
		switch h.Cmd {
		case protocol.CmdAck:
			switch {
			case probe["convs"] != nil:
				check(&protocol.ConvsAck{})
				return "convs"
			case probe["conv"] != nil:
				check(&protocol.ConvSetAck{})
				return "convSet"
			case probe["clearedSeq"] != nil:
				check(&protocol.ConvDeleteAck{})
				return "convDelete"
			}
		case protocol.CmdEvent:
			var ev struct{ Type string }
			_ = json.Unmarshal(h.Data, &ev)
			switch ev.Type {
			case protocol.EventConvSettingsChanged:
				check(&protocol.ConvSettingsChangedEvent{})
				return ev.Type
			case protocol.EventConvDeleted:
				check(&protocol.ConvDeletedEvent{})
				return ev.Type
			}
		}
		if !isUp(h.Cmd) {
			t.Errorf("%s: C5 vector not checked by the server (cmd %s)", name, h.Cmd)
		}
		return ""
	}
	seen := map[string]int{}
	for _, v := range vf.Valid {
		if !strings.Contains(v.Name, "C5）") {
			continue
		}
		var h head
		_ = json.Unmarshal(v.Frame, &h)
		seen["valid/"+h.Cmd]++
		if isUp(h.Cmd) {
			if perr := parseUp(h); perr != nil {
				t.Errorf("valid/%s: server rejects: %v", v.Name, perr)
			}
			continue
		}
		if kind := roundTrip(t, "valid/"+v.Name, h); kind != "" {
			seen["down/"+kind]++
		}
	}
	for _, v := range vf.Invalid {
		if !strings.Contains(v.Name, "C5）") {
			continue
		}
		var h head
		_ = json.Unmarshal(v.Frame, &h)
		seen["invalid/"+h.Cmd]++
		if !isUp(h.Cmd) {
			continue // 下行的非法帧只给 Schema 和 SDK 用
		}
		switch perr := parseUp(h); {
		case perr == nil:
			t.Errorf("invalid/%s: server accepts", v.Name)
		case perr.Code != protocol.CodeBadRequest && perr.Code != protocol.CodeInvalidConvID && perr.Code != protocol.CodeMissingRequestID:
			t.Errorf("invalid/%s: code %d, want 4000 / 4007 / 4003", v.Name, perr.Code)
		}
	}
	// BE-482 第 4 条：convSet 只带 pinned、只带 notifyOff、都带、都不带；convDelete 的 upToSeq 为负；下行带设置的 Conversation、
	// 带 deleted / deletedTruncated 的 convs ack、两个新事件。
	for k, n := range map[string]int{"valid/convSet": 4, "invalid/convSet": 8, "valid/convDelete": 3, "invalid/convDelete": 6,
		"down/convSet": 3, "down/convDelete": 2, "down/convs": 2, "down/convSettingsChanged": 2, "down/convDeleted": 2,
		"invalid/ack": 6, "invalid/event": 4} {
		if seen[k] < n {
			t.Errorf("frames.json has %d C5 vectors of %s, want >= %d (BE-482 第 4 条): %v", seen[k], k, n, seen)
		}
	}
}
