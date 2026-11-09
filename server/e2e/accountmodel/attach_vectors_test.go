package accountmodel

// account-session.json 的 attachmentCases（PRD-chat F-FILE-07 规则 1、3、4，BE-403）。生成方式同 cases：
//
//	go test ./e2e/accountmodel -run TestAccountSessionVectors -update

import (
	"slices"
	"testing"
)

type attachStep struct {
	Op     string         `json:"op"`
	FileID string         `json:"fileId,omitempty"`
	Code   int            `json:"code,omitempty"`
	Expect *AttachOutcome `json:"expect,omitempty"`
}

type attachCase struct {
	Name        string       `json:"name"`
	Rule        string       `json:"rule"`
	ACs         []string     `json:"acs"`
	Persistence bool         `json:"persistence,omitempty"`
	Start       AttachStart  `json:"start"`
	Steps       []attachStep `json:"steps"`
}

var attachmentFormat = map[string]string{
	"start":        "一条附件消息，发给单聊的对方 bob，账号已登录在线（登录用户各端的测试设施自己定）。kind=image|file；processing=true 时图片由 App 先处理（处理中），否则直接上传中；foreignFileId 不为空时是一条已经失败（timeout）、带别人的 fileId 的图片消息（例如转发），没有源，这台设备没有上传过它",
	"pick":         "第一步固定是 pick：用户选好文件，SDK 建出这条消息",
	"processed":    "App 处理完图片",
	"uploadOk":     "当前的上传：申请上传返回 fileId，PUT（图片还有缩略图）都 200，complete 成功",
	"uploadFailed": "当前的上传：PUT 时网络出错",
	"sendAck":      "最近一个 send 的 ack 成功",
	"sendError":    "最近一个 send 回错误码 code",
	"sendTimeout":  "最近一个 send 超过 10 秒没有 ack",
	"disconnect":   "连接断开（不是退出登录）",
	"connect":      "重连成功（冷启动之后是第一次连接），convs 为空",
	"retry":        "用户点重发",
	"dropSource":   "源文件没了：原生是沙盒副本被删，H5 是刷新后 File 对象没了",
	"restart":      "冷启动：进程结束后用同一个本地库新建实例，还没连上（只有 persistence=true）",
	"expect":       "每步之后：state（processing、uploading、queued 即待发送、sending、sent、failed）；fileId（body 里的，没有是空串）；error 写了才比较（network、timeout、sourceMissing 或错误码；冷启动转失败的原因各端自己定，不写）；uploads 是这一步发出的申请上传请求数；sends 是这一步发出的 send 带的 fileId。所有 send 的 clientMsgId 都是这条消息最初的那个",
}

func pick() attachStep            { return attachStep{Op: "pick"} }
func op(name string) attachStep   { return attachStep{Op: name} }
func upOK(id string) attachStep   { return attachStep{Op: "uploadOk", FileID: id} }
func sendErr(code int) attachStep { return attachStep{Op: "sendError", Code: code} }
func imgStart() AttachStart       { return AttachStart{Kind: "image"} }
func fileStart() AttachStart      { return AttachStart{Kind: "file"} }

var attachCases = []attachCase{
	{
		Name: "图片：处理中 → 上传中 → 发送中 → 已发送", Rule: "F-FILE-07 规则 1", ACs: []string{"AC-FILE-07-4"},
		Start: AttachStart{Kind: "image", Processing: true},
		Steps: []attachStep{pick(), op("processed"), upOK("F1"), op("sendAck")},
	},
	{
		Name: "离线时上传完成：待发送，连上后自动发 1 次", Rule: "F-FILE-07 规则 1、3", ACs: []string{"AC-FILE-07-6"},
		Start: fileStart(),
		Steps: []attachStep{pick(), op("disconnect"), upOK("F1"), op("connect"), op("sendAck")},
	},
	{
		Name: "上传失败：失败；手动重发从头上传（新的 fileId）", Rule: "F-FILE-07 规则 1、4", ACs: []string{"AC-FILE-07-6"},
		Start: imgStart(),
		Steps: []attachStep{pick(), op("uploadFailed"), op("retry"), upOK("F2"), op("sendAck")},
	},
	{
		Name: "已 complete 的手动重发：复用同一 fileId，不重传", Rule: "F-FILE-07 规则 4", ACs: []string{"AC-FILE-07-2"},
		Start: imgStart(),
		Steps: []attachStep{pick(), upOK("F1"), op("sendTimeout"), op("retry"), op("sendAck")},
	},
	{
		Name: "send 回 4096、有源：回到上传中重新上传，clientMsgId 不变，不占自动重发次数", Rule: "F-FILE-07 规则 4，RC2-23", ACs: []string{"AC-FILE-07-10"},
		Start: imgStart(),
		Steps: []attachStep{pick(), upOK("F1"), sendErr(4096), upOK("F2"), op("disconnect"), op("connect"), op("sendAck")},
	},
	{
		Name: "send 回 4096、源已不在：失败，原文件已不可用；重发也不上传", Rule: "F-FILE-07 规则 4", ACs: []string{"AC-FILE-07-10"},
		Start: fileStart(),
		Steps: []attachStep{pick(), upOK("F1"), op("dropSource"), sendErr(4096), op("retry")},
	},
	{
		Name: "send 回 4048、fileId 是这台设备自己上传的：按已清理处理，重新上传", Rule: "F-FILE-07 规则 4（v1.4，P-37 ①）", ACs: []string{"AC-FILE-07-10"},
		Start: fileStart(),
		Steps: []attachStep{pick(), upOK("F1"), sendErr(4048), upOK("F2"), op("sendAck")},
	},
	{
		Name: "send 回 4048、自己上传的但源已不在：失败，原文件已不可用", Rule: "F-FILE-07 规则 4（v1.4）", ACs: []string{"AC-FILE-07-10"},
		Start: imgStart(),
		Steps: []attachStep{pick(), upOK("F1"), op("dropSource"), sendErr(4048)},
	},
	{
		Name: "send 回 4048、别人的 fileId：照常失败，不重新上传", Rule: "F-FILE-07 规则 4（v1.4），AC-FILE-02-3", ACs: []string{"AC-FILE-07-10"},
		Start: AttachStart{ForeignFileID: "F-OTHER"},
		Steps: []attachStep{pick(), op("retry"), sendErr(4048), op("retry")},
	},
	{
		Name: "冷启动：上传中转失败，不自动上传；重发用沙盒副本从头上传", Rule: "F-FILE-07 规则 3、4", ACs: []string{"AC-FILE-07-1"},
		Persistence: true, Start: imgStart(),
		Steps: []attachStep{pick(), op("restart"), op("connect"), op("retry"), upOK("F2"), op("sendAck")},
	},
	{
		Name: "冷启动：待发送的连上后自动发 1 次；回 4048 时自己上传的记录还在", Rule: "F-FILE-07 规则 3、4", ACs: []string{"AC-FILE-07-10"},
		Persistence: true, Start: fileStart(),
		Steps: []attachStep{pick(), op("disconnect"), upOK("F1"), op("restart"), op("connect"), sendErr(4048), upOK("F2"), op("sendAck")},
	},
}

func runAttach(c attachCase) attachCase {
	var a *Attachment
	out := c
	out.Steps = make([]attachStep, len(c.Steps))
	for i, st := range c.Steps {
		var o AttachOutcome
		switch st.Op {
		case "pick":
			a, o = NewAttachment(c.Start)
		case "processed":
			o = a.Processed()
		case "uploadOk":
			o = a.UploadOK(st.FileID)
		case "uploadFailed":
			o = a.UploadFailed()
		case "sendAck":
			o = a.SendAck()
		case "sendError":
			o = a.SendError(st.Code)
		case "sendTimeout":
			o = a.SendTimeout()
		case "disconnect":
			o = a.Disconnect()
		case "connect":
			o = a.Connect()
		case "retry":
			o = a.Retry()
		case "dropSource":
			o = a.DropSource()
		case "restart":
			if !c.Persistence {
				panic("restart needs persistence: " + c.Name)
			}
			o = a.Restart()
		default:
			panic("unknown op " + st.Op)
		}
		st.Expect = &o
		out.Steps[i] = st
	}
	return out
}

// 附件状态机的关键结论，不依赖生成的文件。
func TestAttachmentModelRules(t *testing.T) {
	find := func(name string) attachCase {
		for _, c := range attachCases {
			if c.Name == name {
				return runAttach(c)
			}
		}
		t.Fatalf("no case %q", name)
		return attachCase{}
	}
	e := func(c attachCase, i int) AttachOutcome { return *c.Steps[i].Expect }
	// 规则 4：已 complete 的重发不重传。
	if s := e(find("已 complete 的手动重发：复用同一 fileId，不重传"), 3); s.Uploads != 0 || !slices.Equal(s.Sends, []string{"F1"}) {
		t.Errorf("retry must reuse the fileId: %+v", s)
	}
	// 4096 回到上传中，并且不占自动重发：断线后还会补发。
	c := find("send 回 4096、有源：回到上传中重新上传，clientMsgId 不变，不占自动重发次数")
	if s := e(c, 2); s.State != StateUploading || s.FileID != "" || s.Uploads != 1 {
		t.Errorf("4096 re-uploads: %+v", s)
	}
	if s := e(c, 5); !slices.Equal(s.Sends, []string{"F2"}) {
		t.Errorf("auto resend still available after 4096: %+v", s)
	}
	// v1.4：自己的 4048 重传，别人的 4048 不重传。
	if s := e(find("send 回 4048、fileId 是这台设备自己上传的：按已清理处理，重新上传"), 2); s.State != StateUploading || s.Uploads != 1 {
		t.Errorf("own 4048 re-uploads: %+v", s)
	}
	foreign := find("send 回 4048、别人的 fileId：照常失败，不重新上传")
	if s := e(foreign, 2); s.State != StateFailed || s.Error != 4048 || s.Uploads != 0 || s.FileID != "F-OTHER" {
		t.Errorf("foreign 4048 fails: %+v", s)
	}
	// 规则 3：冷启动时上传中转失败，连上也不自动上传；待发送的自动发 1 次。
	cold := find("冷启动：上传中转失败，不自动上传；重发用沙盒副本从头上传")
	if s := e(cold, 2); s.State != StateFailed || s.Uploads != 0 || len(s.Sends) != 0 {
		t.Errorf("cold start must not resume: %+v", s)
	}
	ready := find("冷启动：待发送的连上后自动发 1 次；回 4048 时自己上传的记录还在")
	if s := e(ready, 4); !slices.Equal(s.Sends, []string{"F1"}) {
		t.Errorf("queued attachment is sent once after a cold start: %+v", s)
	}
	if s := e(ready, 5); s.State != StateUploading {
		t.Errorf("own fileId survives a cold start: %+v", s)
	}
	if s := e(find("send 回 4096、源已不在：失败，原文件已不可用；重发也不上传"), 4); s.Error != ErrSourceMissing || s.Uploads != 0 {
		t.Errorf("no source: %+v", s)
	}
}
