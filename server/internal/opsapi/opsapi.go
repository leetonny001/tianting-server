// Package opsapi 是节点运维接口（PRD-ops F-OPS-16，只读，只给 console-server）：
// 独立监听 IM_OPS_ADDR，调用方固定为 console-ops，每节点独立的 Ed25519 公钥，和管理接口的调用方互相不能访问。
//
// 中间件顺序：来源网段 → 并发闸门 → 签名校验 → 处理超时。不持全局锁遍历，内存数据只用 runtime/metrics。
package opsapi

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"runtime/metrics"
	"strconv"
	"sync/atomic"
	"time"

	"tianting/server/internal/adminauth"
	"tianting/server/internal/clock"
	"tianting/server/internal/disconnects"
	imetrics "tianting/server/internal/metrics"
	"tianting/server/internal/protocol"
	"tianting/server/internal/sessions"
)

const (
	// CallerID 是运维接口唯一的调用方。它不能出现在 IM_ADMIN_CALLERS 里（配置校验保证）。
	CallerID = "console-ops"

	maxInflight     = 4
	handlerTimeout  = 2 * time.Second
	signatureWindow = 300 * time.Second
	nonceCapacity   = 100_000
	maxBodyBytes    = 16 << 10

	headerCaller     = "X-IM-Caller"
	headerTimestamp  = "X-IM-Timestamp"
	headerNonce      = "X-IM-Nonce"
	headerSignature  = "X-IM-Signature"
	headerOperator   = "X-IM-Operator"
	headerRequestID  = "X-Request-Id"
	headerTargetNode = "X-IM-Target-Node"
)

// 签名失败的原因（写访问日志，PRD-ops F-OPS-16 规则 8）。不写密钥和签名值。
const (
	failMissing   = "missing_signature"
	failCaller    = "unknown_caller"
	failWindow    = "timestamp_expired"
	failNonce     = "nonce_reused"
	failNode      = "target_node_mismatch"
	failSignature = "signature_mismatch"
	failCIDR      = "source_not_allowed"
)

// Key 是一把运维公钥。最多同时两把（轮换）。
type Key struct {
	ID     string
	Public ed25519.PublicKey
}

// DB 是 /internal/v1/db 的内容。
type DB struct {
	Type       string `json:"type"`
	Location   string `json:"location"`
	Connected  bool   `json:"connected"`
	Version    int    `json:"version"`
	Migrations any    `json:"migrations"`
	Pending    any    `json:"pending"`
}

// Deps 是接口用到的数据来源。
type Deps struct {
	NodeID       string
	Version      string
	StartedAt    time.Time
	Keys         []Key
	AllowedCIDRs []netip.Prefix
	Clock        clock.Clock
	Metrics      *imetrics.Metrics
	Log          *slog.Logger // 访问日志

	Ready       func(ctx context.Context) error
	Draining    func() bool
	Platforms   func() map[string]int
	Sessions    sessions.Source
	Disconnects *disconnects.Buffer
	Config      func() any
	DB          func(ctx context.Context) (DB, error)

	// TestDelay 是测试模式的人为延迟（毫秒，AC-OPS-16-8）；生产环境为 nil。
	TestDelay *atomic.Int64
}

// Server 是运维接口。
type Server struct {
	d      Deps
	gate   chan struct{}
	nonces adminauth.NonceStore
	routes []string
	mux    *http.ServeMux
}

// New 创建运维接口。
func New(d Deps) *Server {
	s := &Server{d: d, gate: make(chan struct{}, maxInflight), nonces: adminauth.NewMemoryNonces(nonceCapacity), mux: http.NewServeMux()}
	s.route("GET", "/internal/v1/node", "node", s.node)
	s.route("POST", "/internal/v1/connections:query", "connections", s.connections)
	s.route("POST", "/internal/v1/disconnects:query", "disconnects", s.disconnects)
	s.route("GET", "/internal/v1/config", "config", s.config)
	s.route("GET", "/internal/v1/db", "db", s.db)
	return s
}

// Handler 返回运维端口的 http.Handler。
func (s *Server) Handler() http.Handler { return s.mux }

// Routes 返回已注册的路由（只读路由守护测试用）。
func (s *Server) Routes() []string { return append([]string(nil), s.routes...) }

type resultKey struct{}

// noteCount 记下结果条数，写进访问日志（对账用，PRD-ops F-OPS-14 规则 5）。
func noteCount(ctx context.Context, n int) {
	if p, ok := ctx.Value(resultKey{}).(*int); ok {
		*p = n
	}
}

func (s *Server) route(method, path, endpoint string, h func(http.ResponseWriter, *http.Request)) {
	key := method + " " + path
	s.routes = append(s.routes, key)
	s.mux.HandleFunc(key, func(w http.ResponseWriter, r *http.Request) { s.serve(w, r, endpoint, h) })
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request, endpoint string, h func(http.ResponseWriter, *http.Request)) {
	start := time.Now()
	entry := accessEntry{endpoint: endpoint, operator: r.Header.Get(headerOperator), requestID: r.Header.Get(headerRequestID), peer: peerIP(r)}
	finish := func(result string) {
		s.d.Metrics.OpsRequests.WithLabelValues(endpoint, result).Inc()
		entry.result, entry.duration = result, time.Since(start)
		s.log(entry)
	}
	if !s.peerAllowed(r) {
		entry.failure = failCIDR
		writeErr(w, protocol.Errorf(protocol.CodeForbidden, "source address not allowed"))
		finish(imetrics.OpsForbidden)
		return
	}
	select {
	case s.gate <- struct{}{}:
		defer func() { <-s.gate }()
	default:
		writeErr(w, protocol.Errorf(protocol.CodeRateLimited, "too many concurrent ops requests"))
		finish(imetrics.OpsThrottled)
		return
	}
	s.d.Metrics.OpsInflight.Inc()
	defer s.d.Metrics.OpsInflight.Dec()

	kid, reason := s.verify(w, r)
	if reason != "" {
		entry.failure = reason
		writeErr(w, protocol.Errorf(protocol.CodeAdminSignatureInvalid, "invalid ops signature"))
		finish(imetrics.OpsUnauthorized)
		return
	}
	entry.kid = kid
	count := 0
	ctx, cancel := context.WithTimeout(context.WithValue(r.Context(), resultKey{}, &count), handlerTimeout)
	defer cancel()
	if err := s.testDelay(ctx); err != nil {
		writeErr(w, protocol.Errorf(protocol.CodeServiceUnavailable, "ops request timed out"))
		finish(imetrics.OpsError)
		return
	}
	rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
	h(rec, r.WithContext(ctx))
	entry.count = count
	if rec.status >= 400 {
		finish(imetrics.OpsError)
		return
	}
	finish(imetrics.OpsOK)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *Server) testDelay(ctx context.Context) error {
	if s.d.TestDelay == nil {
		return nil
	}
	ms := s.d.TestDelay.Load()
	if ms <= 0 {
		return nil
	}
	select {
	case <-time.After(time.Duration(ms) * time.Millisecond):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Server) peerAllowed(r *http.Request) bool {
	if len(s.d.AllowedCIDRs) == 0 {
		return true
	}
	ip, err := netip.ParseAddr(peerIP(r))
	if err != nil {
		return false
	}
	ip = ip.Unmap()
	for _, p := range s.d.AllowedCIDRs {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

// peerIP 只看 TCP 对端地址，不解析 X-Forwarded-For。
func peerIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// verify 校验签名，返回验证通过的 kid；失败时返回原因。nonce 只在签名通过之后登记。
func (s *Server) verify(w http.ResponseWriter, r *http.Request) (kid, reason string) {
	h := r.Header
	ts, nonce, sig, target := h.Get(headerTimestamp), h.Get(headerNonce), h.Get(headerSignature), h.Get(headerTargetNode)
	if ts == "" || nonce == "" || sig == "" || target == "" {
		return "", failMissing
	}
	if h.Get(headerCaller) != CallerID {
		return "", failCaller
	}
	ms, err := strconv.ParseInt(ts, 10, 64)
	now := s.d.Clock.Now()
	if err != nil || abs(now.Sub(time.UnixMilli(ms))) > signatureWindow {
		return "", failWindow
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		return "", failSignature
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	sts := adminauth.StringToSign(adminauth.Request{
		Method: r.Method, Path: r.URL.EscapedPath(), RawQuery: r.URL.RawQuery, Timestamp: ts, Nonce: nonce, Body: body,
		Operator: h.Get(headerOperator), RequestID: h.Get(headerRequestID), TargetNode: target,
	})
	for _, k := range s.d.Keys {
		c := adminauth.Caller{ID: CallerID, Alg: adminauth.AlgEd25519, PublicKey: k.Public}
		if c.Verify(sts, sig) {
			kid = k.ID
			break
		}
	}
	if kid == "" {
		return "", failSignature
	}
	// 签名覆盖了目标节点：发给别的节点的请求签名本身有效，但不能在这里用（AC-OPS-16-2）。
	if target != s.d.NodeID {
		return "", failNode
	}
	fresh, err := s.nonces.Add(CallerID, nonce, now)
	if err != nil || !fresh {
		return "", failNonce
	}
	return kid, ""
}

func abs(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

type accessEntry struct {
	endpoint, kid, operator, requestID, peer, result, failure string
	count                                                     int
	duration                                                  time.Duration
}

// log 写访问日志：不写 userId，不写密钥和签名值（PRD-ops S-OPS-10）。
func (s *Server) log(e accessEntry) {
	attrs := []any{"endpoint", e.endpoint, "kid", e.kid, "operator", e.operator, "requestId", e.requestID,
		"count", e.count, "durationMs", e.duration.Milliseconds(), "peer", e.peer, "result", e.result}
	if e.failure != "" {
		attrs = append(attrs, "failure", e.failure)
	}
	s.d.Log.Info("ops request", attrs...)
}

func writeErr(w http.ResponseWriter, e *protocol.Error) {
	writeJSON(w, protocol.HTTPStatus(e.Code), e)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(v); err != nil && !errors.Is(err, io.EOF) {
		writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "invalid JSON body"))
		return false
	}
	return true
}

// NodeInfo 是 GET /internal/v1/node 的响应。
type NodeInfo struct {
	NodeID         string         `json:"nodeId"`
	Version        string         `json:"version"`
	StartedAt      int64          `json:"startedAt"`
	Ready          bool           `json:"ready"`
	Draining       bool           `json:"draining"`
	Connections    map[string]int `json:"connections"`
	TotalConns     int            `json:"totalConns"`
	LiveBytes      uint64         `json:"liveBytes"`
	HeapBytes      uint64         `json:"heapBytes"`
	StackBytes     uint64         `json:"stackBytes"`
	GoMemLimit     uint64         `json:"goMemLimit"`
	Goroutines     uint64         `json:"goroutines"`
	DisconnectFrom int64          `json:"disconnectBufferFrom"` // 断开缓冲最早一条的时间；0 表示空
}

// runtimeSamples 只用 runtime/metrics：不触发 STW（AC-OPS-16-10）。ReadMemStats 会暂停全部 goroutine。
func runtimeSamples() (live, heap, stack, limit, goroutines uint64) {
	names := []metrics.Sample{
		{Name: "/gc/heap/live:bytes"}, {Name: "/memory/classes/heap/objects:bytes"}, {Name: "/memory/classes/heap/stacks:bytes"},
		{Name: "/gc/gomemlimit:bytes"}, {Name: "/sched/goroutines:goroutines"},
	}
	metrics.Read(names)
	get := func(i int) uint64 {
		if names[i].Value.Kind() == metrics.KindUint64 {
			return names[i].Value.Uint64()
		}
		return 0
	}
	return get(0), get(1), get(2), get(3), get(4)
}

func (s *Server) node(w http.ResponseWriter, r *http.Request) {
	live, heap, stack, limit, goroutines := runtimeSamples()
	platforms := s.d.Platforms()
	total := 0
	for _, n := range platforms {
		total += n
	}
	noteCount(r.Context(), 1)
	writeJSON(w, http.StatusOK, NodeInfo{
		NodeID: s.d.NodeID, Version: s.d.Version, StartedAt: s.d.StartedAt.UnixMilli(),
		Ready: s.d.Ready(r.Context()) == nil, Draining: s.d.Draining(), Connections: platforms, TotalConns: total,
		LiveBytes: live + stack, HeapBytes: heap, StackBytes: stack, GoMemLimit: limit, Goroutines: goroutines,
		DisconnectFrom: s.d.Disconnects.Oldest(),
	})
}

type userQuery struct {
	UserID string `json:"userId"`
	Limit  int    `json:"limit"`
}

// connections 不区分"用户不存在"和"不在线"：两者是同样的空列表（AC-OPS-08-2）。
func (s *Server) connections(w http.ResponseWriter, r *http.Request) {
	var q userQuery
	if !readJSON(w, r, &q) {
		return
	}
	if !protocol.ValidID(q.UserID) {
		writeErr(w, protocol.Errorf(protocol.CodeInvalidID, "invalid userId"))
		return
	}
	res := sessions.Query(s.d.Sessions, q.UserID)
	noteCount(r.Context(), len(res.Sessions))
	writeJSON(w, http.StatusOK, struct {
		Connections []sessions.Conn `json:"connections"`
	}{res.Sessions})
}

func (s *Server) disconnects(w http.ResponseWriter, r *http.Request) {
	var q userQuery
	if !readJSON(w, r, &q) {
		return
	}
	if !protocol.ValidID(q.UserID) {
		writeErr(w, protocol.Errorf(protocol.CodeInvalidID, "invalid userId"))
		return
	}
	if q.Limit < 0 || q.Limit > disconnects.MaxPerUser {
		writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "limit must be at most %d", disconnects.MaxPerUser))
		return
	}
	records := s.d.Disconnects.Query(q.UserID, q.Limit)
	noteCount(r.Context(), len(records))
	writeJSON(w, http.StatusOK, struct {
		Records    []disconnects.Record `json:"records"`
		BufferFrom int64                `json:"bufferFrom"`
	}{records, s.d.Disconnects.Oldest()})
}

func (s *Server) config(w http.ResponseWriter, r *http.Request) {
	noteCount(r.Context(), 1)
	writeJSON(w, http.StatusOK, s.d.Config())
}

func (s *Server) db(w http.ResponseWriter, r *http.Request) {
	info, err := s.d.DB(r.Context())
	if err != nil {
		s.d.Log.Error("ops db status", "err", err)
		writeErr(w, protocol.Errorf(protocol.CodeInternal, "database status unavailable"))
		return
	}
	noteCount(r.Context(), 1)
	writeJSON(w, http.StatusOK, info)
}
