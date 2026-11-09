package protocol

// C3 离线推送的上行命令（PRD-chat 第 11.1 节、F-APN-01～03，BE-450）。名字避开系统通知的 push、pushSync（R-CF-26）。
// 字段和 frame.schema.json 的 DeviceNotificationRegisterReq 等一一对应。

const (
	CmdDeviceNotificationRegister   = "deviceNotificationRegister"
	CmdDeviceNotificationUnregister = "deviceNotificationUnregister"
	CmdDeviceNotificationSettings   = "deviceNotificationSettings"
	CmdPresence                     = "presence"
)

// 通道和环境。
const (
	PushProviderAPNs = "apns"
	PushProviderFCM  = "fcm"
	PushEnvSandbox   = "sandbox"
	PushEnvProd      = "production"
)

// presence 的状态（F-APN-02 规则 3）。
const (
	PresenceForeground = "foreground"
	PresenceBackground = "background"
)

const (
	// MaxDeviceTokenBytes 是令牌长度上限（PRD 只说"超长"；APNs 十六进制令牌和 FCM 令牌都放得下，BE-450 第 1 条）。
	MaxDeviceTokenBytes = 4096
	// MaxAppIDBytes 是 appId 的上限（bundle id、包名）。
	MaxAppIDBytes = 255
)

// DeviceNotificationRegisterReq 是 deviceNotificationRegister 的 data。env 缺省按 production（FCM 不区分环境）。
type DeviceNotificationRegisterReq struct {
	Provider string `json:"provider"`
	Token    string `json:"token"`
	Env      string `json:"env,omitempty"`
	AppID    string `json:"appId"`
}

// Validate 校验请求（空或超长的令牌 4000，F-APN-01 异常），返回生效的 env。
func (r *DeviceNotificationRegisterReq) Validate() (string, *Error) {
	if r.Provider != PushProviderAPNs && r.Provider != PushProviderFCM {
		return "", Errorf(CodeBadRequest, "provider must be apns or fcm")
	}
	if r.Token == "" {
		return "", Errorf(CodeBadRequest, "token is required")
	}
	if len(r.Token) > MaxDeviceTokenBytes {
		return "", Errorf(CodeBadRequest, "token must be at most %d bytes", MaxDeviceTokenBytes)
	}
	if r.AppID == "" || len(r.AppID) > MaxAppIDBytes {
		return "", Errorf(CodeBadRequest, "appId must be 1..%d bytes", MaxAppIDBytes)
	}
	switch r.Env {
	case "":
		return PushEnvProd, nil
	case PushEnvSandbox, PushEnvProd:
		return r.Env, nil
	}
	return "", Errorf(CodeBadRequest, "env must be sandbox or production")
}

// DeviceNotificationUnregisterReq 是 deviceNotificationUnregister 的 data（空对象）。
type DeviceNotificationUnregisterReq struct{}

// DeviceNotificationSettingsReq 是 deviceNotificationSettings 的 data：不带 showDetail 只读取。
type DeviceNotificationSettingsReq struct {
	ShowDetail *bool `json:"showDetail"`
}

// DeviceNotificationSettingsAck 是应答：当前值（R-CF-22）。
type DeviceNotificationSettingsAck struct {
	ShowDetail bool `json:"showDetail"`
}

// PresenceReq 是 presence 的 data（F-APN-02 规则 3，普通带 id 的命令，R-CB-26）。
type PresenceReq struct {
	State string `json:"state"`
}

// Validate 校验 state。
func (r *PresenceReq) Validate() *Error {
	if r.State != PresenceForeground && r.State != PresenceBackground {
		return Errorf(CodeBadRequest, "state must be foreground or background")
	}
	return nil
}
