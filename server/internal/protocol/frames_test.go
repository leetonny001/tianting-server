package protocol

import (
	"encoding/json"
	"testing"
)

// Go 结构体编码出来的下行帧必须符合 Schema，防止代码和协议文档悄悄分叉。
func TestOutFramesMatchSchema(t *testing.T) {
	schema := compileSchema(t)
	msg := Message{
		MsgID: "m1", ClientMsgID: "cid-00000001", ConvID: C2CConvID("u2", "u1"), Seq: 1,
		From: "u1", Type: MsgText, Body: MsgBody{Text: "hi"}, TS: 1730000000000,
	}
	frames := map[string]OutFrame{
		"send ack":       AckOK("r1", SendAck{MsgID: "m1", Seq: 1, TS: 1}),
		"sync ack":       AckOK("r2", SyncAck{Messages: []Message{msg}, HasMore: true}),
		"sync ack empty": AckOK("r2", SyncAck{Messages: []Message{}}),
		"empty ack":      AckOK("r3", Empty{}),
		"error ack":      AckErr("r4", Errorf(CodeForbidden, "not a member")),
		"error no id":    AckErr("", Errorf(CodeBadRequest, "invalid json")),
		"msg":            {Cmd: CmdMsg, Data: msg},
		"custom msg": {Cmd: CmdMsg, Data: Message{
			MsgID: "m2", ConvID: GroupConvID("g1"), Seq: 2, From: "u1", Type: MsgCustom,
			Body: MsgBody{Data: json.RawMessage(`{"k":1}`)}, TS: 1,
		}},
		"pong": {Cmd: CmdPong},
	}
	for name, f := range frames {
		t.Run(name, func(t *testing.T) {
			b, err := json.Marshal(f)
			if err != nil {
				t.Fatal(err)
			}
			var v any
			if err := json.Unmarshal(b, &v); err != nil {
				t.Fatal(err)
			}
			if err := schema.Validate(v); err != nil {
				t.Errorf("%s does not match schema: %v", b, err)
			}
		})
	}
}

func TestParseConvID(t *testing.T) {
	ok := map[string]Conv{
		"c2c:a:b":  {Type: ConvC2C, A: "a", B: "b"},
		"c2c:u:u":  {Type: ConvC2C, A: "u", B: "u"},
		"group:g1": {Type: ConvGroup, A: "g1"},
		"room:r-1": {Type: ConvRoom, A: "r-1"},
	}
	for s, want := range ok {
		got, err := ParseConvID(s)
		if err != nil || got != want {
			t.Errorf("ParseConvID(%q) = %+v, %v; want %+v", s, got, err, want)
		}
	}
	for _, s := range []string{"", "c2c:b:a", "c2c:a", "group:", "group:a:b", "chan:x", "c2c:a:b c"} {
		if _, err := ParseConvID(s); err == nil {
			t.Errorf("ParseConvID(%q) should fail", s)
		}
	}
	if C2CConvID("b", "a") != "c2c:a:b" {
		t.Error("C2CConvID must sort user ids")
	}
}

func TestSendReqValidate(t *testing.T) {
	base := func() SendReq {
		return SendReq{ConvType: ConvC2C, To: "u2", ClientMsgID: "12345678", Type: MsgText, Body: MsgBody{Text: "hi"}}
	}
	if err := (&SendReq{}).Validate(); err == nil {
		t.Error("empty request should fail")
	}
	r := base()
	if err := r.Validate(); err != nil {
		t.Fatalf("valid request failed: %v", err)
	}
	cases := map[string]func(*SendReq){
		"bad convType":    func(r *SendReq) { r.ConvType = "x" },
		"bad to":          func(r *SendReq) { r.To = "a b" },
		"short clientId":  func(r *SendReq) { r.ClientMsgID = "1" },
		"empty text":      func(r *SendReq) { r.Body.Text = "" },
		"custom no data":  func(r *SendReq) { r.Type = MsgCustom },
		"custom null":     func(r *SendReq) { r.Type = MsgCustom; r.Body.Data = json.RawMessage("null") },
		"unknown msgType": func(r *SendReq) { r.Type = "image" },
	}
	for name, mutate := range cases {
		r := base()
		mutate(&r)
		if err := r.Validate(); err == nil || err.Code != CodeBadRequest {
			t.Errorf("%s: got %v, want 4000", name, err)
		}
	}
}

func TestSyncReqDefaultsLimit(t *testing.T) {
	r := SyncReq{ConvID: "group:g1"}
	if err := r.Validate(); err != nil || r.Limit != defaultSyncLimit {
		t.Fatalf("got limit %d err %v", r.Limit, err)
	}
	r = SyncReq{ConvID: "group:g1", Limit: maxSyncLimit + 1}
	if err := r.Validate(); err == nil {
		t.Error("limit over max should fail")
	}
}
