package accountmodel

// attach.go 是附件发送状态机的参考客户端（PRD-chat F-FILE-07 规则 1、3、4，BE-403）。
// 只管一条附件消息：处理中 → 上传中 → 待发送 → 发送中 → 已发送，任何一步可以转失败；冷启动；手动重发；send 回 4096、4048。

// 附件消息的状态（F-FILE-07 规则 1）。
const (
	StateProcessing = "processing" // 压缩、生成缩略图
	StateUploading  = "uploading"  // 申请、PUT、complete
	StateQueued     = "queued"     // 待发送：已 complete，等连接
	StateSending    = "sending"
	StateSent       = "sent"
	StateFailed     = "failed"
)

// 失败原因里 PRD 定了的部分（F-FILE-07 规则 4、9）。冷启动转失败的原因各端自己定，不在用例里比较。
const (
	ErrNetwork       = "network"       // 上传时网络或对象存储失败
	ErrTimeout       = "timeout"       // send 10 秒没有 ack
	ErrSourceMissing = "sourceMissing" // 要重新上传但源文件已不在："原文件已不可用，请重新选择"
)

const (
	codeFileNotFound = 4048
	codeFileNotReady = 4096
)

// Attachment 是一条附件消息和它所在 SDK 实例的相关状态。
type Attachment struct {
	State     string
	FileID    string          // 当前 body 里的 fileId；没上传完是空
	Error     any             // string 或错误码；nil 表示没有
	Source    bool            // 有沙盒副本（原生）或内存里的源（H5）
	Own       map[string]bool // 这台设备自己上传、complete 成功的 fileId（规则 4，v1.4）
	Connected bool
	AutoUsed  bool // 自动补发的一次已经用掉（F-MSG-03 规则 2）
}

// AttachOutcome 是一步之后的结果。Uploads 是这一步发出的申请上传请求数；Sends 是这一步发出的 send 带的 fileId。
type AttachOutcome struct {
	State   string   `json:"state"`
	FileID  string   `json:"fileId"`
	Error   any      `json:"error,omitempty"`
	Uploads int      `json:"uploads"`
	Sends   []string `json:"sends"`
}

// AttachStart 是用例的起点。ForeignFileID 不为空时，是一条带别人的 fileId（例如转发）的失败消息，没有源。
type AttachStart struct {
	Kind          string `json:"kind"`                    // image | file
	Processing    bool   `json:"processing,omitempty"`    // 图片要 App 先处理（只有 image）
	ForeignFileID string `json:"foreignFileId,omitempty"` // 有它时 kind、processing 不用
}

// NewAttachment 是用户选好文件（pick）之后的状态，连接在线。
func NewAttachment(s AttachStart) (*Attachment, AttachOutcome) {
	a := &Attachment{Own: map[string]bool{}, Connected: true}
	var o AttachOutcome
	switch {
	case s.ForeignFileID != "":
		a.State, a.FileID, a.Error = StateFailed, s.ForeignFileID, ErrTimeout
	case s.Processing:
		a.State, a.Source = StateProcessing, true
	default:
		a.State, a.Source = StateUploading, true
		o.Uploads = 1
	}
	return a, a.out(o)
}

func (a *Attachment) out(o AttachOutcome) AttachOutcome {
	o.State, o.FileID, o.Error = a.State, a.FileID, a.Error
	if o.Sends == nil {
		o.Sends = []string{}
	}
	return o
}

func (a *Attachment) send() AttachOutcome {
	a.State, a.Error = StateSending, nil
	return a.out(AttachOutcome{Sends: []string{a.FileID}})
}

func (a *Attachment) upload() AttachOutcome {
	a.State, a.Error = StateUploading, nil
	return a.out(AttachOutcome{Uploads: 1})
}

func (a *Attachment) fail(err any) AttachOutcome {
	a.State, a.Error = StateFailed, err
	return a.out(AttachOutcome{})
}

// Processed：App 处理完图片，开始上传。
func (a *Attachment) Processed() AttachOutcome {
	if a.State != StateProcessing {
		return a.out(AttachOutcome{})
	}
	return a.upload()
}

// UploadOK：申请、PUT、complete 都成功。在线就发送，否则待发送（规则 1、3）。
func (a *Attachment) UploadOK(fileID string) AttachOutcome {
	if a.State != StateUploading {
		return a.out(AttachOutcome{})
	}
	a.FileID = fileID
	a.Own[fileID] = true
	if a.Connected {
		return a.send()
	}
	a.State = StateQueued
	return a.out(AttachOutcome{})
}

// UploadFailed：PUT 时网络或对象存储出错。
func (a *Attachment) UploadFailed() AttachOutcome {
	if a.State != StateUploading {
		return a.out(AttachOutcome{})
	}
	return a.fail(ErrNetwork)
}

// SendAck：send 成功。
func (a *Attachment) SendAck() AttachOutcome {
	if a.State == StateSending {
		a.State, a.Error = StateSent, nil
		a.Source = false // 已发送：删除沙盒副本（规则 2）
	}
	return a.out(AttachOutcome{})
}

// SendError：send 回错误码。4096，或自己上传的 fileId 回 4048，按"已清理"处理：有源的回到上传中从头上传，
// 不算自动重发次数（RC2-23）；没有源的转失败 sourceMissing。别人的 fileId 回 4048 照常失败（规则 4，v1.4）。
func (a *Attachment) SendError(code int) AttachOutcome {
	if a.State != StateSending {
		return a.out(AttachOutcome{})
	}
	if code == codeFileNotReady || (code == codeFileNotFound && a.Own[a.FileID]) {
		a.FileID = ""
		if !a.Source {
			return a.fail(ErrSourceMissing)
		}
		return a.upload()
	}
	return a.fail(code)
}

// SendTimeout：send 10 秒没有 ack（F-MSG-03 规则 1）。
func (a *Attachment) SendTimeout() AttachOutcome {
	if a.State != StateSending {
		return a.out(AttachOutcome{})
	}
	return a.fail(ErrTimeout)
}

// Disconnect：连接断开。发送中的，自动补发还没用过就转待发送，重连后补发一次（F-MSG-03 规则 2）。上传不受影响。
func (a *Attachment) Disconnect() AttachOutcome {
	a.Connected = false
	if a.State == StateSending {
		if a.AutoUsed {
			return a.fail(ErrTimeout)
		}
		a.AutoUsed = true
		a.State = StateQueued
	}
	return a.out(AttachOutcome{})
}

// Connect：连接（或重连、冷启动后的第一次连接）成功。待发送的自动发一次。
func (a *Attachment) Connect() AttachOutcome {
	a.Connected = true
	if a.State == StateQueued {
		a.AutoUsed = true
		return a.send()
	}
	return a.out(AttachOutcome{})
}

// Retry：用户手动重发（规则 4）。clientMsgId 不变。已经有 fileId 的复用，不重传；没有的用源从头上传，源不在了只能删除。
func (a *Attachment) Retry() AttachOutcome {
	if a.State != StateFailed {
		return a.out(AttachOutcome{})
	}
	if a.FileID == "" {
		if !a.Source {
			a.Error = ErrSourceMissing
			return a.out(AttachOutcome{})
		}
		return a.upload()
	}
	if a.Connected {
		return a.send()
	}
	a.AutoUsed = true // 手动重发也只自动补发一次
	a.State, a.Error = StateQueued, nil
	return a.out(AttachOutcome{})
}

// DropSource：源文件没了（原生：沙盒副本被删；H5：刷新后 File 对象没了）。
func (a *Attachment) DropSource() AttachOutcome {
	a.Source = false
	return a.out(AttachOutcome{})
}

// Restart：冷启动（只有带持久化的原生 SDK）。处理中、上传中一律转失败，不自动重发；待发送（含已有 fileId 的发送中）
// 按待发送，连上后自动发 1 次（规则 3）。沙盒副本和"自己上传的 fileId"跨冷启动保留。新进程还没连上。
func (a *Attachment) Restart() AttachOutcome {
	a.Connected, a.AutoUsed = false, false
	switch a.State {
	case StateProcessing, StateUploading:
		a.State, a.Error = StateFailed, nil
		return AttachOutcome{State: a.State, FileID: a.FileID, Sends: []string{}} // 原因各端自己定，不比较
	case StateSending:
		if a.FileID != "" {
			a.State = StateQueued
		}
	}
	return a.out(AttachOutcome{})
}
