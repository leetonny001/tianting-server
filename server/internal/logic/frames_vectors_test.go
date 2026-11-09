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
