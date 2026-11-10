package files

import (
	"context"
	"errors"
	"time"

	"tianting/server/internal/protocol"
	"tianting/server/internal/store"
)

// URLItem 是批量取地址的一项。
type URLItem struct {
	FileID  string `json:"fileId"`
	Variant string `json:"variant"`
}

// URLsInput 是 POST /v1/files/urls（F-FILE-04 规则 1）。BareDisposition：原生客户端带 true，文件的
// Content-Disposition 只签 `attachment`，不带文件名（F-FILE-03 规则 4，PRD-chat v1.7，P-45）；不带或 false 时不变（H5）。
type URLsInput struct {
	Items           []URLItem `json:"items"`
	BareDisposition bool      `json:"bareDisposition,omitempty"`
}

// URLResult 是一项的结果：有地址，或 code=4048。
type URLResult struct {
	FileID    string `json:"fileId"`
	Variant   string `json:"variant"`
	URL       string `json:"url,omitempty"`
	ExpiresAt int64  `json:"expiresAt,omitempty"`
	Code      int    `json:"code,omitempty"`
}

// URLsResponse 是批量取地址的响应，顺序和请求相同。
type URLsResponse struct {
	Items []URLResult `json:"items"`
}

const (
	variantOriginal = "original"
	variantThumb    = "thumb"
)

// URLs 批量取下载地址（F-FILE-04）。单次 ≤ 50（4131）；每人每分钟 120 批（4290）。
// 不可访问的项一律 code=4048，和不存在的 fileId 完全相同（规则 3），不影响同一批的其他项。只签名，不访问存储。
// 预签名地址和文件名不写日志（AC-FILE-04-7）。
func (s *Service) URLs(ctx context.Context, uid string, in URLsInput) (*URLsResponse, *protocol.Error) {
	if len(in.Items) == 0 {
		return nil, protocol.Errorf(protocol.CodeBadRequest, "items is empty")
	}
	if len(in.Items) > maxURLItems {
		return nil, protocol.Errorf(protocol.CodeBatchLimitExceeded, "at most %d items", maxURLItems)
	}
	for _, it := range in.Items {
		if it.Variant != variantOriginal && it.Variant != variantThumb {
			return nil, protocol.Errorf(protocol.CodeBadRequest, "invalid variant")
		}
	}
	if !s.urlRate.Allow(uid) {
		return nil, protocol.Errorf(protocol.CodeRateLimited, "too many url requests")
	}
	ids := make([]string, 0, len(in.Items))
	for _, it := range in.Items {
		if protocol.ValidID(it.FileID) {
			ids = append(ids, it.FileID)
		}
	}
	info, err := s.store.FileAccessInfo(ctx, ids)
	if err != nil {
		return nil, s.internal("file access info", err)
	}
	out := &URLsResponse{Items: make([]URLResult, len(in.Items))}
	allowed := map[string]bool{} // 同一批里同一个文件只判断一次
	issued := 0
	for i, it := range in.Items {
		res := URLResult{FileID: it.FileID, Variant: it.Variant, Code: protocol.CodeFileNotFound}
		a, ok := info[it.FileID]
		if ok {
			can, seen := allowed[it.FileID]
			if !seen {
				var perr *protocol.Error
				if can, perr = s.canAccess(ctx, uid, a); perr != nil {
					return nil, perr
				}
				allowed[it.FileID] = can
			}
			if can {
				if u, exp, ok := s.sign(a.File, it.Variant, in.BareDisposition); ok {
					res = URLResult{FileID: it.FileID, Variant: it.Variant, URL: u, ExpiresAt: exp}
					issued++
				}
			}
		}
		out.Items[i] = res
	}
	if s.metrics != nil && issued > 0 {
		s.metrics.FileURLsIssued.Add(float64(issued))
	}
	return out, nil
}

// canAccess 是规则 2：请求者当前能访问这条消息——单聊参与者，或当前群成员且 seq ≥ minSeq；消息没有被删除；文件没有过期。
// 发送者在绑定消息前可以访问自己的上传。消息状态和文件状态不一致时一律不可访问。清空位置（F-CVS-03）和撤回随 C4、C5。
func (s *Service) canAccess(ctx context.Context, uid string, a store.FileAccess) (bool, *protocol.Error) {
	switch a.State {
	case store.FileReady:
		return a.Owner == uid, nil
	case store.FileAttached:
	default: // pending、expired、revoked、purging
		return false, nil
	}
	if a.MsgID == "" || !a.MsgFound || a.MsgDeleted || a.ConvID == "" {
		return false, nil
	}
	conv, perr := protocol.ParseConvID(a.ConvID)
	if perr != nil {
		return false, nil
	}
	switch conv.Type {
	case protocol.ConvC2C:
		return conv.Has(uid), nil
	case protocol.ConvGroup:
		g, err := s.store.GetGroup(ctx, conv.A)
		if errors.Is(err, store.ErrNotFound) { // 不存在或已解散（AC-FILE-06-11）
			return false, nil
		}
		if err != nil {
			return false, s.internal("get group", err)
		}
		m, ok, err := s.store.GroupMember(ctx, conv.A, uid)
		if err != nil {
			return false, s.internal("group member", err)
		}
		if !ok {
			return false, nil
		}
		minSeq := int64(1)
		if !g.HistoryVisible {
			minSeq = m.JoinSeq + 1
		}
		return a.MsgSeq >= minSeq, nil
	}
	return false, nil
}

// sign 预签名一个变体的 GET：签入服务端记录的类型和 Content-Disposition（F-FILE-03 规则 4）。图片 inline，其他 attachment，
// 带清洗后的文件名；bare（原生客户端）时只有 `attachment`，地址里没有文件名（v1.7，P-45）。
// 有效期：图片和缩略图 120 秒，文件 300 秒（规则 1）。
func (s *Service) sign(f store.File, v string, bare bool) (string, int64, bool) {
	key, mt, ttl := f.ObjectKey, f.Mime, s.cfg.URLTTLFile
	disp := Disposition("attachment", f.Name)
	if bare {
		disp = "attachment"
	}
	if f.Kind != store.KindFile {
		ttl, disp = s.cfg.URLTTLImage, Disposition("inline", "")
	}
	if v == variantThumb {
		if f.Kind != store.KindImage || f.ThumbKey == "" {
			return "", 0, false
		}
		key, mt = f.ThumbKey, f.ThumbMime
	}
	if key == "" {
		return "", 0, false
	}
	u, exp := s.obj.PresignGet(key, ttl, mt, disp)
	return u, exp, true
}

// URLTTL 返回某种文件的地址有效期（测试用）。
func (s *Service) URLTTL(kind string) time.Duration {
	if kind == store.KindFile {
		return s.cfg.URLTTLFile
	}
	return s.cfg.URLTTLImage
}
