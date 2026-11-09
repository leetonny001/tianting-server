package protocol

import (
	"encoding/json"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// BE-401（AC-CHAT-90-10 的服务端部分）：image、file 消息和 modVer 的下行帧符合 Schema；MsgBody 按 type 互斥（F-MOD-02 规则 4a），
// Schema 和 Validate 一致；上行 image 里伪造的 size、width 不报错（AC-FILE-02-2、02-11，所以 image 内部不能 additionalProperties:false）。
func TestFileFramesMatchSchema(t *testing.T) {
	schema := compileSchema(t)
	conv := Conversation{ConvID: "c2c:u1:u2", Type: ConvC2C, Name: "u2", MaxSeq: 1, MinSeq: 1, UpdatedAt: 1, ModVer: 3,
		LastMsg: &LastMsg{MsgID: "m1", From: "u1", Type: MsgImage, Text: "", TS: 1}}
	frames := map[string]OutFrame{
		"image msg": {Cmd: CmdMsg, Data: Message{MsgID: "m1", ClientMsgID: "cid-00000001", ConvID: "c2c:u1:u2", Seq: 1, From: "u1", Type: MsgImage,
			Body: MsgBody{Image: &ImageBody{FileID: "f1", Width: 640, Height: 480, Size: 1024, Mime: "image/jpeg"}}, TS: 1}},
		"expired file msg": {Cmd: CmdMsg, Data: Message{MsgID: "m2", ClientMsgID: "cid-00000002", ConvID: "group:g1", Seq: 2, From: "u1", Type: MsgFile,
			Body: MsgBody{File: &FileBody{FileID: "f2", Size: 5, Mime: "application/pdf", Expired: true}}, TS: 1}},
		"convs with modVer": AckOK("r1", ConvsAck{Convs: []Conversation{conv}}),
	}
	for name, f := range frames {
		b, _ := json.Marshal(f)
		var v any
		_ = json.Unmarshal(b, &v)
		if err := schema.Validate(v); err != nil {
			t.Errorf("%s: %s does not match: %v", name, b, err)
		}
	}
}

// TC-FILE-02-15（AC-FILE-02-15）的协议部分：type=image 同时带 text、data；type=text 带 data、image：Schema 和 Validate 都拒绝。
func TestMsgBodyExclusive(t *testing.T) {
	schema := compileSchema(t)
	for _, tc := range []struct {
		name  string
		typ   string
		body  string
		valid bool
	}{
		{"image", "image", `{"image":{"fileId":"f1"}}`, true},
		{"image with forged size", "image", `{"image":{"fileId":"f1","size":1,"mime":"text/html","width":1,"extra":true}}`, true},
		{"file", "file", `{"file":{"fileId":"f1"}}`, true},
		{"text", "text", `{"text":"hi"}`, true},
		{"custom", "custom", `{"data":{"k":1}}`, true},
		{"image with text", "image", `{"image":{"fileId":"f1"},"text":"x"}`, false},
		{"image with data", "image", `{"image":{"fileId":"f1"},"data":{}}`, false},
		{"image without image", "image", `{"text":"x"}`, false},
		{"image without fileId", "image", `{"image":{}}`, false},
		{"file with image", "file", `{"file":{"fileId":"f1"},"image":{"fileId":"f1"}}`, false},
		{"text with data", "text", `{"text":"hi","data":{}}`, false},
		{"text with image", "text", `{"text":"hi","image":{"fileId":"f1"}}`, false},
		{"custom with text", "custom", `{"data":{},"text":"x"}`, false},
	} {
		raw := `{"id":"r1","cmd":"send","data":{"convType":"c2c","to":"u2","clientMsgId":"cid-00000001","type":"` + tc.typ + `","body":` + tc.body + `}}`
		v, err := jsonschema.UnmarshalJSON(jsonReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		if err := schema.Validate(v); (err == nil) != tc.valid {
			t.Errorf("schema %s: valid=%v, err=%v", tc.name, tc.valid, err)
		}
		var in InFrame
		_ = json.Unmarshal([]byte(raw), &in)
		var req SendReq
		if err := json.Unmarshal(in.Data, &req); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if perr := req.Validate(); (perr == nil) != tc.valid {
			t.Errorf("Validate %s: valid=%v, err=%v", tc.name, tc.valid, perr)
		}
	}
}

// BE-401 第 6 条：HTTP Schema 的文件部分有合法和非法样例。
func TestHTTPFileSchema(t *testing.T) {
	for _, tc := range []struct {
		def   string
		json  string
		valid bool
	}{
		{"UploadRequest", `{"kind":"image","name":"a.png","size":10,"mime":"image/png","thumb":{"size":1,"mime":"image/jpeg"}}`, true},
		{"UploadRequest", `{"kind":"video","name":"a","size":10,"mime":"video/mp4"}`, false},
		{"UploadResponse", `{"fileId":"abc","put":{"url":"https://s3/x","headers":{"Content-Type":"image/png"},"expiresAt":1}}`, true},
		{"UploadResponse", `{"fileId":"abc"}`, false},
		{"CompleteResponse", `{"fileId":"abc","size":1,"mime":"image/png","width":1,"height":1}`, true},
		{"FileUrlsRequest", `{"items":[{"fileId":"abc","variant":"thumb"}]}`, true},
		{"FileUrlsRequest", `{"items":[{"fileId":"abc","variant":"big"}]}`, false},
		{"FileUrlsResponse", `{"items":[{"fileId":"a","variant":"original","url":"https://x","expiresAt":1},{"fileId":"b","variant":"thumb","code":4048}]}`, true},
		{"FileUrlsResponse", `{"items":[{"fileId":"a","variant":"original","url":"https://x","expiresAt":1,"code":4048}]}`, false},
		{"ConfigResponse", `{"registerMode":"invite","demoLogin":false,"features":{"files":true},"limits":{"usernameMinLen":4,"usernameMaxLen":32,"passwordMinLen":8,"passwordMaxLen":128,"nicknameMaxLen":32,"deviceNameMaxLen":64,"imageMaxBytes":1,"thumbMaxBytes":1,"fileMaxBytes":1,"avatarMaxBytes":1,"uploadDailyBytes":1}}`, true},
	} {
		v, err := jsonschema.UnmarshalJSON(jsonReader(tc.json))
		if err != nil {
			t.Fatal(err)
		}
		if err := compileHTTPDef(t, tc.def).Validate(v); (err == nil) != tc.valid {
			t.Errorf("%s %s: valid=%v, err=%v", tc.def, tc.json, tc.valid, err)
		}
	}
}
