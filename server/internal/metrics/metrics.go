// Package metrics 定义服务端的 Prometheus 指标（BE-051）。
//
// 指标端点只在单独的内网地址上暴露（IM_METRICS_ADDR，默认 127.0.0.1:9090），不和对外的 WebSocket / 管理接口共用端口：
// 指标里有在线人数、错误分布等内部运行数据，不应该被公网访问。
//
// 标签只用取值有限的字段（平台、命令名、错误码、关闭码、原因），不放 userId、convId 等：
// 否则标签组合无限增长，会把 Prometheus 和服务端内存一起拖垮。客户端传来的未知命令统一记为 "unknown"。
package metrics

import (
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// 握手结果（im_handshakes_total 的 result 标签）。
const (
	HandshakeOK           = "ok"
	HandshakeRateLimited  = "rate_limited"
	HandshakeBadPlatform  = "bad_platform"
	HandshakeAuthRequired = "auth_required"
	HandshakeTokenInvalid = "token_invalid"
	HandshakeTokenRevoked = "token_revoked"
	HandshakeTokenExpired = "token_expired"
	HandshakeError        = "error"
	HandshakeOriginDenied = "origin_rejected"
)

// 账号的结果（im_account_logins_total、im_registers_total 的 result 标签，PRD-chat 第 14.2 节）。
const (
	AccountOK            = "ok"
	AccountInvalid       = "invalid"
	AccountLocked        = "locked"
	AccountRateLimited   = "rate_limited"
	AccountBusy          = "busy"
	AccountBanned        = "banned"
	AccountPending       = "deletion_pending"
	AccountError         = "error"
	AccountBadRequest    = "bad_request"
	AccountClosed        = "closed"
	AccountInviteInvalid = "invite_invalid"
	AccountTaken         = "taken"
)

// complete 的结果（im_file_uploads_total 的 result 标签，PRD-chat 第 14.2 节）。按错误码归类，取值有限。
const (
	FileOK          = "ok"
	FileMismatch    = "mismatch"    // 4000 uploadMismatch 等
	FileTypeDenied  = "type_denied" // 4150
	FileNotReady    = "not_ready"   // 4096
	FileQuota       = "quota"       // 4292
	FileUnavailable = "unavailable" // 5032
	FileGone        = "gone"        // 4048（取消、清理中）
	FileError       = "error"
)

// 文件种类和对象存储操作（im_file_uploads_total 的 kind、im_objstore_* 的 op 标签）。
var (
	FileKinds   = []string{"image", "file", "avatar"}
	ObjStoreOps = []string{"head", "get", "list", "put", "copy", "delete"}
)

// 运维接口的结果（im_ops_requests_total 的 result 标签，PRD-ops S-OPS-06）。
const (
	OpsOK           = "ok"
	OpsUnauthorized = "unauthorized"
	OpsForbidden    = "forbidden_cidr"
	OpsThrottled    = "throttled"
	OpsError        = "error"
)

// goRuntimeMetrics 是内存和 GC 面板用到的 runtime/metrics（PRD-ops S-OPS-05）：
// go_gc_gomemlimit_bytes、go_gc_heap_live_bytes、go_cpu_classes_gc_total_cpu_seconds_total、go_cpu_classes_total_cpu_seconds_total。
var goRuntimeMetrics = regexp.MustCompile(`^/(gc/gomemlimit:bytes|gc/heap/live:bytes|cpu/classes/gc/total:cpu-seconds|cpu/classes/total:cpu-seconds)$`)

// Metrics 是全部指标。每个 App 一份，测试之间互不影响。
type Metrics struct {
	registry *prometheus.Registry

	Connections     *prometheus.GaugeVec     // 当前在线连接数，按平台
	Handshakes      *prometheus.CounterVec   // 握手结果
	Disconnects     *prometheus.CounterVec   // 连接关闭，按关闭码和关闭方（client / server）
	Kicks           *prometheus.CounterVec   // 被踢原因
	Commands        *prometheus.CounterVec   // 上行命令数，按命令和结果码（0 表示成功）
	CommandDuration *prometheus.HistogramVec // 命令处理耗时（收到请求到产生 ack），按命令
	Messages        *prometheus.CounterVec   // 新保存的消息数（不含去重命中的重发），按会话类型
	SlowConsumers   prometheus.Counter       // 因下行积压被断开的连接数
	// AdminAuthFailures 是管理接口签名鉴权失败数，按原因（missing / caller / signature / window / nonce / node）。
	// 失败不写审计（防刷爆），靠这个指标发现异常（PRD-admin F-KEY-03 规则 6）。
	AdminAuthFailures *prometheus.CounterVec
	OpsRequests       *prometheus.CounterVec // 运维接口请求数，按接口和结果
	OpsInflight       prometheus.Gauge       // 正在处理的运维请求数
	JobRuns           *prometheus.CounterVec // 定时任务运行次数，按任务和结果（BE-220）
	JobDuration       *prometheus.HistogramVec
	// 系统推送（PRD-live F-PUSH-10）。pushSync 的次数和耗时在 Commands、CommandDuration 里（cmd="pushSync"）。type 由调用方任意填写，不作为标签。
	Pushes         *prometheus.CounterVec // 新受理的推送数（不含幂等命中的重放），按 target
	PushDeliveries *prometheus.CounterVec // 接受投递的连接数（含重播），按 target
	PushInboxRows  prometheus.Gauge       // 收件箱行数：启动时从库里读，写入和清理时增减
	// 账号（PRD-chat 第 14.2 节）。result 取值见 account 包，都是枚举。
	AccountLogins      *prometheus.CounterVec
	Registers          *prometheus.CounterVec
	RefreshReissue     prometheus.Counter
	RefreshReplay      prometheus.Counter
	InviteAutoDisabled prometheus.Counter // 邀请码因为探测用户名被自动停用（F-ACC-01 规则 7 的告警）
	// 文件（PRD-chat 第 14.2 节，F-FILE-04 规则 6）：没有用户标签，按用户的量在 files 包里统计。
	FileURLsIssued    prometheus.Counter
	FileUploadedBytes prometheus.Counter     // im_file_bytes_total：complete 成功的实际字节数
	FileUploads       *prometheus.CounterVec // complete 尝试的结果，kind × result 都是有界取值
	ObjStoreRequests  *prometheus.CounterVec // 按 op
	ObjStoreErrors    *prometheus.CounterVec // 按 op：网络错误和对象存储返回的错误（不含 404、412）
}

// New 创建并注册全部指标，另外带上 Go 运行时和进程指标。
func New() *Metrics {
	r := prometheus.NewRegistry()
	f := func(c prometheus.Collector) { r.MustRegister(c) }
	m := &Metrics{
		registry: r,
		Connections: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "im_connections", Help: "Current authenticated WebSocket connections on this node.",
		}, []string{"platform"}),
		Handshakes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "im_handshakes_total", Help: "WebSocket handshakes by result.",
		}, []string{"result"}),
		Disconnects: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "im_disconnects_total", Help: "Closed connections by close code, which side closed and phase (handshake / session).",
		}, []string{"code", "by", "phase"}),
		Kicks: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "im_kicks_total", Help: "Kicked connections by reason.",
		}, []string{"reason"}),
		Commands: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "im_commands_total", Help: "Client commands by command and result code (0 = ok).",
		}, []string{"cmd", "code"}),
		CommandDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "im_command_duration_seconds",
			Help:    "Time from receiving a command to producing its ack.",
			Buckets: []float64{.0005, .001, .0025, .005, .01, .025, .05, .1, .25, .5, 1, 2.5},
		}, []string{"cmd"}),
		Messages: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "im_messages_total", Help: "Newly stored messages by conversation type.",
		}, []string{"conv_type"}),
		SlowConsumers: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "im_slow_consumers_total", Help: "Connections closed because their send queue was full.",
		}),
		AdminAuthFailures: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "im_admin_auth_failures_total", Help: "Rejected signed admin API requests by reason.",
		}, []string{"reason"}),
		OpsRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "im_ops_requests_total", Help: "Node ops API requests by endpoint and result.",
		}, []string{"endpoint", "result"}),
		OpsInflight: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "im_ops_inflight", Help: "Node ops API requests being processed.",
		}),
		JobRuns: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "im_job_runs_total", Help: "Scheduled job runs by job and result.",
		}, []string{"job", "result"}),
		JobDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "im_job_duration_seconds",
			Help:    "Scheduled job duration.",
			Buckets: []float64{.01, .05, .1, .5, 1, 5, 15, 60, 300},
		}, []string{"job"}),
	}
	m.Pushes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "im_pushes_total", Help: "Newly accepted system pushes by target (idempotent replays are not counted).",
	}, []string{"target"})
	m.PushDeliveries = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "im_push_deliveries_total", Help: "Connections that accepted a realtime push frame, by target.",
	}, []string{"target"})
	m.PushInboxRows = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "im_push_inbox_rows", Help: "Rows in the per-user push inbox.",
	})
	m.AccountLogins = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "im_account_logins_total", Help: "Account logins by result.",
	}, []string{"result"})
	m.RefreshReissue = prometheus.NewCounter(prometheus.CounterOpts{Name: "im_refresh_reissue_total", Help: "Refresh tokens reissued from the current generation while a pending one exists."})
	m.RefreshReplay = prometheus.NewCounter(prometheus.CounterOpts{Name: "im_refresh_replay_total", Help: "Device sessions revoked by refresh token replay."})
	m.Registers = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "im_registers_total", Help: "Account registrations by result.",
	}, []string{"result"})
	m.InviteAutoDisabled = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "im_invites_auto_disabled_total", Help: "Invite codes automatically disabled after too many taken usernames.",
	})
	m.FileURLsIssued = prometheus.NewCounter(prometheus.CounterOpts{Name: "im_file_url_issued_total", Help: "Presigned download URLs issued."})
	// 第 14.2 节的文件指标（AC-CHAT-90-17）。标签没有 userId、fileId、文件名（AC-OPS-90-4）。
	m.FileUploadedBytes = prometheus.NewCounter(prometheus.CounterOpts{Name: "im_file_bytes_total", Help: "Bytes of completed uploads (original and thumbnail)."})
	m.FileUploads = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "im_file_uploads_total", Help: "Upload complete attempts by kind and result."}, []string{"kind", "result"})
	m.ObjStoreRequests = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "im_objstore_requests_total", Help: "Object store requests (not presigned) by operation."}, []string{"op"})
	m.ObjStoreErrors = prometheus.NewCounterVec(prometheus.CounterOpts{Name: "im_objstore_errors_total", Help: "Object store requests that failed (network errors and store errors; not found and precondition failures excluded) by operation."}, []string{"op"})
	f(m.FileURLsIssued)
	f(m.FileUploadedBytes)
	f(m.FileUploads)
	f(m.ObjStoreRequests)
	f(m.ObjStoreErrors)
	f(m.AccountLogins)
	f(m.Registers)
	f(m.RefreshReissue)
	f(m.RefreshReplay)
	f(m.InviteAutoDisabled)
	f(m.Pushes)
	f(m.PushDeliveries)
	f(m.PushInboxRows)
	f(m.JobRuns)
	f(m.JobDuration)
	f(m.OpsRequests)
	f(m.OpsInflight)
	f(m.Connections)
	f(m.Handshakes)
	f(m.Disconnects)
	f(m.Kicks)
	f(m.Commands)
	f(m.CommandDuration)
	f(m.Messages)
	f(m.AdminAuthFailures)
	f(m.SlowConsumers)
	f(collectors.NewGoCollector(collectors.WithGoCollectorRuntimeMetrics(collectors.GoRuntimeMetricsRule{Matcher: goRuntimeMetrics})))
	f(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	return m
}

// RegisterNode 注册构建信息和就绪 / 下线状态（PRD-ops S-OPS-03、04）。ready、draining 和 /readyz、优雅下线同一来源，抓取时现算。
func (m *Metrics) RegisterNode(version, goVersion, nodeID string, ready, draining func() bool) {
	build := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "im_build_info", Help: "Build information; the value is always 1.",
	}, []string{"version", "go_version", "node_id"})
	build.WithLabelValues(version, goVersion, nodeID).Set(1)
	m.registry.MustRegister(build,
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "im_ready", Help: "1 when the node can accept traffic (same check as /readyz)."}, boolGauge(ready)),
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "im_draining", Help: "1 while the node is shutting down."}, boolGauge(draining)),
	)
}

func boolGauge(fn func() bool) func() float64 {
	return func() float64 {
		if fn() {
			return 1
		}
		return 0
	}
}

// Handler 返回 /metrics 的 http.Handler。
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

// knownCmds 是允许作为标签值的命令名。
var knownCmds = map[string]bool{
	"send": true, "sync": true, "convs": true, "read": true, "joinRoom": true, "leaveRoom": true, "auth": true,
	// PRD-social M3（与 protocol.SocialCmds 一致，metrics 不依赖 protocol 包）
	"friendApply": true, "friendRespond": true, "friendDelete": true, "friendRemark": true, "friendRequestsSeen": true,
	"socialSync": true, "block": true, "unblock": true, "userSearch": true, "setPrivacy": true, "users": true, "myGroups": true,
	// PRD-social M4
	"groupCreate": true, "groupInvite": true, "groupLeave": true, "groupKick": true, "groupTransfer": true, "groupSetAdmin": true,
	"groupDissolve": true, "groupUpdate": true, "groupInfo": true, "groupMembers": true,
	// PRD-live L1
	"pushSync": true,
}

// CmdLabel 把客户端传来的命令名规整成有限的标签值。
func CmdLabel(cmd string) string {
	if knownCmds[cmd] {
		return cmd
	}
	return "unknown"
}

// Code 把错误码 / 关闭码转成标签值。
func Code(code int) string { return strconv.Itoa(code) }

// 标签取值的白名单（PRD-ops AC-OPS-90-4）。code 另外按数字检查；build_info 的三个标签来自配置，取值有限。
var forbiddenLabels = map[string]bool{"userid": true, "user_id": true, "uid": true, "conv": true, "convid": true, "conv_id": true, "conn": true, "connid": true, "conn_id": true}

var codeLabel = regexp.MustCompile(`^(\d+|unknown)$`)

var allowedLabelValues = map[string]map[string]bool{
	"by":        set("client", "server"),
	"phase":     set("handshake", "session"),
	"platform":  set("android", "ios", "web", "pc", "unknown"),
	"conv_type": set("c2c", "group", "room", "unknown"),
	"result": set(AccountOK, AccountInvalid, AccountLocked, AccountRateLimited, AccountBusy, AccountBanned, AccountPending,
		AccountError, AccountBadRequest, AccountClosed, AccountInviteInvalid, AccountTaken,
		HandshakeOK, HandshakeRateLimited, HandshakeBadPlatform, HandshakeAuthRequired, HandshakeTokenInvalid,
		HandshakeTokenRevoked, HandshakeTokenExpired, HandshakeError, HandshakeOriginDenied, OpsOK, OpsUnauthorized, OpsForbidden, OpsThrottled, OpsError,
		FileOK, FileMismatch, FileTypeDenied, FileNotReady, FileQuota, FileUnavailable, FileGone, FileError, "unknown"),
	"kind": set(append(FileKinds, "unknown")...),
	"op":   set(ObjStoreOps...),
	"reason": set("tokenInvalid", "tokenExpired", "banned", "suspended", "replaced", "tooManyConnections", "serverShutdown", "passwordChanged", "sessionRevoked", "accountDeleted",
		"missing", "caller", "signature", "window", "nonce", "node", "unknown"),
	"endpoint": set("node", "connections", "disconnects", "config", "db", "unknown"),
	"target":   set("user", "all", "room"),
}

func set(values ...string) map[string]bool {
	m := make(map[string]bool, len(values))
	for _, v := range values {
		m[v] = true
	}
	return m
}

// LabelProblems 检查全部 im_* 指标的标签：没有用户、会话、连接维度，取值都在枚举内。返回发现的问题。
func (m *Metrics) LabelProblems() []string {
	mfs, err := m.registry.Gather()
	if err != nil {
		return []string{err.Error()}
	}
	var out []string
	for _, mf := range mfs {
		name := mf.GetName()
		if len(name) < 3 || name[:3] != "im_" || name == "im_build_info" {
			continue
		}
		for _, metric := range mf.GetMetric() {
			for _, l := range metric.GetLabel() {
				if p := labelProblem(l.GetName(), l.GetValue()); p != "" {
					out = append(out, name+": "+p)
				}
			}
		}
	}
	return out
}

func labelProblem(name, value string) string {
	if forbiddenLabels[strings.ToLower(name)] {
		return "forbidden label " + name
	}
	switch name {
	case "code":
		if !codeLabel.MatchString(value) {
			return "code value " + value
		}
	case "cmd":
		if value != "unknown" && !knownCmds[value] {
			return "cmd value " + value
		}
	default:
		allowed, ok := allowedLabelValues[name]
		if !ok {
			return "label not in the allow list: " + name
		}
		if !allowed[value] {
			return name + " value " + value
		}
	}
	return ""
}
