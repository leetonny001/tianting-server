// Package files 是 C2 图片和文件的服务端（PRD-chat 第 6 节，BE-431～BE-437）：申请上传、complete、取消、批量取地址、
// file_purge 和头像绑定。文件内容不经过 im-server：客户端直接 PUT 到对象存储的临时键，complete 校验后条件 Copy 到最终键。
//
// 日志里不写预签名地址、查询串和文件名（AC-FILE-04-7），只写 fileId 和错误类别。
package files

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"mime"
	"strings"
	"time"

	"tianting/server/internal/clock"
	"tianting/server/internal/metrics"
	"tianting/server/internal/objstore"
	"tianting/server/internal/protocol"
	"tianting/server/internal/ratelimit"
	"tianting/server/internal/store"
)

// Service 是文件服务。
type Service struct {
	cfg     Config
	store   store.Store
	obj     *objstore.Client
	clock   clock.Clock
	hooks   *Hooks // 只在测试模式非 nil
	log     *slog.Logger
	metrics *metrics.Metrics
	block   Blocklist
	upRate  ratelimit.Limiter
	urlRate ratelimit.Limiter
}

// Deps 是 Service 的依赖。
type Deps struct {
	Config  Config
	Store   store.Store
	Obj     *objstore.Client
	Clock   clock.Clock
	Hooks   *Hooks
	Log     *slog.Logger
	Metrics *metrics.Metrics
}

// New 创建文件服务。
func New(d Deps) (*Service, error) {
	cfg := d.Config.WithDefaults()
	block, err := LoadBlocklist(cfg.BlocklistPath)
	if err != nil {
		return nil, err
	}
	return &Service{
		cfg: cfg, store: d.Store, obj: d.Obj, clock: d.Clock, hooks: d.Hooks, log: d.Log, metrics: d.Metrics, block: block,
		upRate:  ratelimit.NewPer(time.Minute/time.Duration(cfg.UploadRate), cfg.UploadRate, d.Clock),
		urlRate: ratelimit.NewPer(time.Minute/time.Duration(cfg.URLRate), cfg.URLRate, d.Clock),
	}, nil
}

// Config 返回生效的配置。
func (s *Service) Config() Config { return s.cfg }

// Hooks 返回测试模式的故障注入（生产为 nil）。
func (s *Service) Hooks() *Hooks { return s.hooks }

// 中国不使用夏令时：用固定时区，不依赖运行环境的 tzdata（和 admin 的配额一样）。
var shanghai = time.FixedZone("Asia/Shanghai", 8*3600)

func (s *Service) now() time.Time { return s.clock.Now() }

// day 是 [IM 钟] 的 Asia/Shanghai 自然日（第 4.1 节）。
func (s *Service) day() string { return s.now().In(shanghai).Format("2006-01-02") }

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func objectStoreUnavailable() *protocol.Error {
	return protocol.Errorf(protocol.CodeObjectStoreUnavailable, "object store unavailable")
}

func fileNotFound() *protocol.Error {
	return protocol.Errorf(protocol.CodeFileNotFound, "file not found")
}

func (s *Service) internal(op string, err error) *protocol.Error {
	s.log.Error(op, "err", err)
	return protocol.Errorf(protocol.CodeInternal, "internal error")
}

// storeErr 把对象存储的错误转成协议错误：不可用 503 / 5032，其他 5000。
func (s *Service) storeErr(op, fileID string, err error) *protocol.Error {
	if objstore.IsUnavailable(err) {
		s.log.Warn(op, "fileId", fileID, "err", err)
		return objectStoreUnavailable()
	}
	s.log.Error(op, "fileId", fileID, "err", err)
	return protocol.Errorf(protocol.CodeInternal, "internal error")
}

// UploadInput 是 POST /v1/files/uploads（F-FILE-01 规则 1）。
type UploadInput struct {
	Kind  string     `json:"kind"`
	Name  string     `json:"name"`
	Size  int64      `json:"size"`
	Mime  string     `json:"mime"`
	Thumb *ThumbSpec `json:"thumb"`
}

// ThumbSpec 是缩略图的声明。
type ThumbSpec struct {
	Size int64  `json:"size"`
	Mime string `json:"mime"`
}

// UploadResponse 是申请上传的响应。
type UploadResponse struct {
	FileID   string              `json:"fileId"`
	Put      objstore.Presigned  `json:"put"`
	ThumbPut *objstore.Presigned `json:"thumbPut,omitempty"`
}

func tooLarge(max int64) *protocol.Error {
	e := protocol.Errorf(protocol.CodeFileTooLarge, "file too large")
	e.Details = map[string]any{"maxBytes": max}
	return e
}

func typeNotAllowed() *protocol.Error {
	return protocol.Errorf(protocol.CodeFileTypeNotAllowed, "file type not allowed")
}

// normMime 规范化声明的类型：必须是合法的媒体类型，去掉参数，小写；image/jpg 按 image/jpeg。
func normMime(m string) (string, bool) {
	if len(m) > 127 {
		return "", false
	}
	mt, _, err := mime.ParseMediaType(m)
	if err != nil || !strings.Contains(mt, "/") {
		return "", false
	}
	if mt == "image/jpg" {
		mt = "image/jpeg"
	}
	return mt, true
}

// CreateUpload 申请上传（F-FILE-01 规则 1、6、6a、7，F-FILE-05 规则 1）。只签名，不访问存储（规则 8）。
// 判定顺序：频率 4290 → 参数 400 → 类型 415 → 大小 413 → 额度、声明合计 4292 → 未完成数 4290。
func (s *Service) CreateUpload(ctx context.Context, uid string, in UploadInput) (*UploadResponse, *protocol.Error) {
	if !s.upRate.Allow(uid) {
		return nil, protocol.Errorf(protocol.CodeRateLimited, "too many upload requests")
	}
	if in.Size < 1 {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "size must be positive")
	}
	mt, ok := normMime(in.Mime)
	if !ok {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "invalid mime")
	}
	name := SanitizeName(in.Name)
	if len(name) > maxNameBytes {
		return nil, protocol.WithReason(protocol.CodeBadRequest, protocol.ReasonFileNameTooLong, "file name too long")
	}
	f := store.File{Owner: uid, Kind: in.Kind, Name: name, DeclaredSize: in.Size, DeclaredMime: mt}
	switch in.Kind {
	case store.KindImage:
		if in.Thumb == nil {
			return nil, protocol.Errorf(protocol.CodeBadRequest, "image upload requires thumb")
		}
		tm, ok := normMime(in.Thumb.Mime)
		if !ok || in.Thumb.Size < 1 {
			return nil, protocol.Errorf(protocol.CodeBadRequest, "invalid thumb")
		}
		if !imageMimes[mt] || !imageMimes[tm] || s.block.Blocked(name) {
			return nil, typeNotAllowed()
		}
		if in.Size > s.cfg.ImageMaxBytes {
			return nil, tooLarge(s.cfg.ImageMaxBytes)
		}
		if in.Thumb.Size > s.cfg.ThumbMaxBytes {
			return nil, tooLarge(s.cfg.ThumbMaxBytes)
		}
		f.ThumbDeclaredSize, f.ThumbDeclaredMime = in.Thumb.Size, tm
	case store.KindAvatar:
		if in.Thumb != nil {
			return nil, protocol.Errorf(protocol.CodeBadRequest, "avatar has no thumb")
		}
		if !avatarMimes[mt] {
			return nil, typeNotAllowed()
		}
		if in.Size > s.cfg.AvatarMaxBytes {
			return nil, tooLarge(s.cfg.AvatarMaxBytes)
		}
	case store.KindFile:
		if in.Thumb != nil {
			return nil, protocol.Errorf(protocol.CodeBadRequest, "file has no thumb")
		}
		// 文件消息只有禁止名单（F-FILE-05 规则 1）：先规范化再判断扩展名（AC-FILE-05-2）。
		if s.block.Blocked(name) {
			return nil, typeNotAllowed()
		}
		if in.Size > s.cfg.FileMaxBytes {
			return nil, tooLarge(s.cfg.FileMaxBytes)
		}
	default:
		return nil, protocol.Errorf(protocol.CodeBadRequest, "invalid kind")
	}
	daily, perr := s.dailyLimit(ctx, uid)
	if perr != nil {
		return nil, perr
	}
	now := s.now()
	nowReal := time.Now()
	f.FileID = randomHex(16)
	f.TmpKey = "tmp/" + randomHex(16)
	if f.ThumbDeclaredSize > 0 {
		f.TmpThumbKey = "tmp/" + randomHex(16)
	}
	f.CreatedAt = now.UnixMilli()
	f.PutExpiresAt = nowReal.Add(s.cfg.PutTTL).UnixMilli()
	err := s.store.CreateUpload(ctx, f, store.UploadLimits{
		Day: s.day(), DailyBytes: daily, DailyDeclared: s.cfg.DailyDeclared, PendingMax: s.cfg.PendingMax, NowReal: nowReal.UnixMilli(),
	})
	switch {
	case errors.Is(err, store.ErrUploadQuota), errors.Is(err, store.ErrUploadDeclared):
		return nil, protocol.Errorf(protocol.CodeQuotaExceeded, "daily upload quota exceeded")
	case errors.Is(err, store.ErrUploadPendingLimit):
		return nil, protocol.WithReason(protocol.CodeRateLimited, protocol.ReasonUploadPendingLimit, "too many pending uploads")
	case err != nil:
		return nil, s.internal("create upload", err)
	}
	out := &UploadResponse{FileID: f.FileID, Put: s.obj.PresignPut(f.TmpKey, f.DeclaredSize, f.DeclaredMime, s.cfg.PutTTL)}
	if f.TmpThumbKey != "" {
		p := s.obj.PresignPut(f.TmpThumbKey, f.ThumbDeclaredSize, f.ThumbDeclaredMime, s.cfg.PutTTL)
		out.ThumbPut = &p
	}
	return out, nil
}

// dailyLimit：open 模式下注册 7 天内的账号 200 MiB（第 4.1 节），其他 IM_UPLOAD_DAILY_BYTES。
func (s *Service) dailyLimit(ctx context.Context, uid string) (int64, *protocol.Error) {
	if !s.cfg.RegisterOpen {
		return s.cfg.DailyBytes, nil
	}
	acc, err := s.store.GetAccount(ctx, uid)
	switch {
	case errors.Is(err, store.ErrNotFound):
		return s.cfg.DailyBytes, nil // 演示和导入的用户没有账号
	case err != nil:
		return 0, s.internal("get account", err)
	}
	if s.now().Sub(time.UnixMilli(acc.CreatedAt)) < newAccountDays*24*time.Hour {
		return min(s.cfg.DailyBytes, s.cfg.NewAccountDaily), nil
	}
	return s.cfg.DailyBytes, nil
}

// CancelUpload 是 DELETE /v1/files/uploads/{fileId}（规则 6a）：只匹配本人且 pending 的上传，其他情况都返回同样的成功，
// 不暴露存在性（RS4-23）。pending → purging 不看租约（RS5-16），尽力删除临时对象，行由 file_purge 删除。
func (s *Service) CancelUpload(ctx context.Context, uid, fileID string) *protocol.Error {
	if !protocol.ValidID(fileID) {
		return nil
	}
	f, err := s.store.GetFile(ctx, fileID)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return s.internal("get file", err)
	}
	ok, err := s.store.CancelUpload(ctx, fileID, uid)
	if err != nil {
		return s.internal("cancel upload", err)
	}
	if ok {
		// 清理不用请求上下文（BE-430 第 3 条）：客户端断开不应打断删除。
		s.deleteKeys(fileID, f.TmpKey, f.TmpThumbKey)
	}
	return nil
}

// deleteKeys 尽力删除私有桶的键（失败靠临时前缀的生命周期规则，F-FILE-01 规则 3a）。
func (s *Service) deleteKeys(fileID string, keys ...string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, k := range keys {
		if k == "" {
			continue
		}
		if err := s.obj.Delete(ctx, s.obj.Bucket(), k); err != nil {
			s.log.Warn("delete object", "fileId", fileID, "err", err)
		}
	}
}
