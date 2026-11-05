package logic

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"tianting/server/internal/delivery"
	"tianting/server/internal/protocol"
	"tianting/server/internal/ratelimit"
	"tianting/server/internal/store"
)

// 系统推送（PRD-live v1.0 第 4 节，F-PUSH-01～04、06、07、09、11）。

// PushRecordRetention 是幂等记录的保留期：晚于收件箱到期（4.4 节）。
const PushRecordRetention = 8 * 24 * time.Hour

// PushConfig 是推送的限速和上限（都未校准，F-PUSH-09）。
type PushConfig struct {
	AllRate  int // target=all，每调用方每分钟
	UserRate int // target=user，每调用方每分钟
	SyncRate int // pushSync，每用户每分钟
	InboxMax int // 单用户收件箱上限
}

type pushState struct {
	broadcaster delivery.Broadcaster
	allLimit    *ratelimit.TokenBucket
	userLimit   *ratelimit.TokenBucket
	syncLimit   *ratelimit.TokenBucket
	inboxMax    int
}

func perMinute(n int, s *Service) *ratelimit.TokenBucket {
	return ratelimit.NewPer(time.Minute/time.Duration(n), n, s.clock)
}

// EnablePush 打开系统推送（IM_PUSH_ENABLED）。路由层缺少广播能力时返回错误，由启动流程拒绝启动，
// 不能像 socialSettingsChanged 那样静默跳过（F-PUSH-04 规则 2）。
func (s *Service) EnablePush(ctx context.Context, cfg PushConfig) error {
	b, ok := s.router.(delivery.Broadcaster)
	if !ok {
		return errors.New("push requires a delivery router with broadcast capability")
	}
	n, err := s.store.CountPushInbox(ctx)
	if err != nil {
		return fmt.Errorf("count push inbox: %w", err)
	}
	s.metrics.PushInboxRows.Set(float64(n))
	s.push = &pushState{
		broadcaster: b,
		allLimit:    perMinute(cfg.AllRate, s),
		userLimit:   perMinute(cfg.UserRate, s),
		syncLimit:   perMinute(cfg.SyncRate, s),
		inboxMax:    cfg.InboxMax,
	}
	return nil
}

// PushCommand 是一次已通过参数校验的 POST /admin/push。
type PushCommand struct {
	Caller  string
	Target  string
	UserIDs []string // 已去重
	Type    string
	Data    json.RawMessage
	PushID  string
	TTL     int64 // 生效的 ttlSeconds
}

// PushResult 是 POST /admin/push 的结果。
type PushResult struct {
	Created  bool
	Accepted *int
}

// Push 处理一次推送。错误优先级见 F-PUSH-02 规则 7：参数和白名单由调用方先校验，这里从 4094 / 幂等命中开始。
func (s *Service) Push(ctx context.Context, cmd PushCommand) (PushResult, *protocol.Error) {
	hash, err := pushContentHash(cmd)
	if err != nil {
		return PushResult{}, s.internal("hash push content", err)
	}
	rec, err := s.store.GetPushRecord(ctx, cmd.PushID)
	switch {
	case err == nil:
		return s.pushHit(ctx, cmd, hash, rec)
	case !errors.Is(err, store.ErrNotFound):
		return PushResult{}, s.internal("get push record", err)
	}
	if len(cmd.UserIDs) > protocol.MaxPushUsers {
		return PushResult{}, protocol.Errorf(protocol.CodeBatchLimitExceeded, "at most %d userIds per push", protocol.MaxPushUsers)
	}
	if perr := s.checkPushUsers(ctx, cmd.UserIDs); perr != nil {
		return PushResult{}, perr
	}
	limiter := s.push.userLimit
	if cmd.Target == protocol.PushTargetAll {
		limiter = s.push.allLimit
	}
	if !limiter.Allow(cmd.Caller) {
		return PushResult{}, protocol.Errorf(protocol.CodeRateLimited, "too many pushes")
	}
	now := s.clock.Now().UnixMilli()
	rec = store.PushRecord{PushID: cmd.PushID, Caller: cmd.Caller, ContentHash: hash, Target: cmd.Target, FirstTS: now, CreatedAt: now}
	if cmd.Target == protocol.PushTargetUser {
		n := len(cmd.UserIDs)
		rec.Accepted = &n
	}
	var res PushResult
	var perr *protocol.Error
	if cmd.Target == protocol.PushTargetUser && cmd.TTL > 0 {
		res, perr = s.pushToInbox(ctx, cmd, rec)
	} else {
		res, perr = s.pushOnline(ctx, cmd, rec)
	}
	if perr != nil || !res.Created {
		limiter.Refund(cmd.Caller) // 失败、并发提交命中幂等：不计入限速
	}
	return res, perr
}

// pushHit 处理 pushId 已有记录的情况（4.4 节）：调用方或内容不同 4094；pending 重新广播，帧的 ts 沿用首次受理时间。
func (s *Service) pushHit(ctx context.Context, cmd PushCommand, hash string, rec store.PushRecord) (PushResult, *protocol.Error) {
	if rec.Caller != cmd.Caller || rec.ContentHash != hash {
		return PushResult{}, protocol.Errorf(protocol.CodePushIDConflict, "pushId %s was used by another caller or with different content", cmd.PushID)
	}
	if rec.State == store.PushPending {
		if perr := s.broadcastOnline(ctx, cmd, rec.FirstTS); perr != nil {
			return PushResult{}, perr
		}
	}
	return PushResult{Accepted: rec.Accepted}, nil
}

func (s *Service) checkPushUsers(ctx context.Context, userIDs []string) *protocol.Error {
	if len(userIDs) == 0 {
		return nil
	}
	missing, err := s.store.MissingUsers(ctx, userIDs)
	if err != nil {
		return s.internal("check push users", err)
	}
	if len(missing) > 0 {
		e := protocol.Errorf(protocol.CodeUserNotFound, "%d users not found", len(missing))
		e.Details = map[string]any{"userIds": missing}
		return e
	}
	return nil
}

// pushToInbox：幂等记录、序号、收件箱同一事务（state=done），提交后下发。提交后、下发前崩溃时靠下一次补拉（4.6 节）。
func (s *Service) pushToInbox(ctx context.Context, cmd PushCommand, rec store.PushRecord) (PushResult, *protocol.Error) {
	w := store.PushInboxWrite{
		Record: rec, UserIDs: cmd.UserIDs, Type: cmd.Type, Data: cmd.Data,
		ExpiresAt: rec.FirstTS + cmd.TTL*1000, InboxMax: s.push.inboxMax,
	}
	if i, ok := s.faults.takePushFailAt(); ok {
		w.FailAt = func(at int) error {
			if at == i {
				return errInjectedPushWrite
			}
			return nil
		}
	}
	res, err := s.store.WritePushInbox(ctx, w)
	if err != nil {
		return PushResult{}, s.internal("write push inbox", err)
	}
	if res.Existing != nil { // 并发提交同一个 pushId
		return s.pushHit(ctx, cmd, rec.ContentHash, *res.Existing)
	}
	s.metrics.Pushes.WithLabelValues(cmd.Target).Inc()
	s.metrics.PushInboxRows.Add(float64(len(res.Seqs) - res.Evicted))
	if !s.faults.takePushSkipDelivery(cmd.PushID) {
		for _, uid := range cmd.UserIDs {
			s.deliverPush(uid, protocol.PushData{PushID: cmd.PushID, Type: cmd.Type, Data: cmd.Data, TS: rec.FirstTS, Target: cmd.Target, PushSeq: res.Seqs[uid]})
		}
	}
	return PushResult{Created: true, Accepted: rec.Accepted}, nil
}

// pushOnline：target=all 和 ttlSeconds=0。先写 pending，再广播，广播发出后改 done（4.4 节）。
func (s *Service) pushOnline(ctx context.Context, cmd PushCommand, rec store.PushRecord) (PushResult, *protocol.Error) {
	existing, err := s.store.CreatePendingPush(ctx, rec)
	if err != nil {
		return PushResult{}, s.internal("create pending push", err)
	}
	if existing != nil { // 并发提交同一个 pushId
		return s.pushHit(ctx, cmd, rec.ContentHash, *existing)
	}
	s.metrics.Pushes.WithLabelValues(cmd.Target).Inc()
	s.faults.exitBeforeBroadcast(cmd.PushID)
	if perr := s.broadcastOnline(ctx, cmd, rec.FirstTS); perr != nil {
		return PushResult{}, perr
	}
	return PushResult{Created: true, Accepted: rec.Accepted}, nil
}

// broadcastOnline 投递给当前在线的连接，然后把记录改成 done。改失败时记录还是 pending，重试会再广播一次，靠 SDK 去重。
func (s *Service) broadcastOnline(ctx context.Context, cmd PushCommand, ts int64) *protocol.Error {
	p := protocol.PushData{PushID: cmd.PushID, Type: cmd.Type, Data: cmd.Data, TS: ts, Target: cmd.Target}
	if !s.faults.takePushSkipDelivery(cmd.PushID) {
		if cmd.Target == protocol.PushTargetAll {
			frame, err := pushFrame(p)
			if err != nil {
				return s.internal("marshal push frame", err)
			}
			n := s.push.broadcaster.DeliverToAll(frame)
			s.metrics.PushDeliveries.WithLabelValues(cmd.Target).Add(float64(n))
		} else {
			for _, uid := range cmd.UserIDs {
				s.deliverPush(uid, p)
			}
		}
	}
	// 广播已经发出：调用方断开也要把状态改掉。
	if err := s.store.MarkPushDone(context.WithoutCancel(ctx), cmd.PushID); err != nil {
		s.log.Error("mark push done", "pushId", cmd.PushID, "err", err)
	}
	return nil
}

func (s *Service) deliverPush(uid string, p protocol.PushData) {
	frame, err := pushFrame(p)
	if err != nil {
		s.log.Error("marshal push frame", "pushId", p.PushID, "err", err)
		return
	}
	n := s.router.DeliverToUser(uid, frame, "")
	s.metrics.PushDeliveries.WithLabelValues(p.Target).Add(float64(n))
}

func pushFrame(p protocol.PushData) ([]byte, error) {
	return json.Marshal(protocol.OutFrame{Cmd: protocol.CmdPush, Data: p})
}

// pushContentHash 是 4.4 节的 content_hash：target、去重排序后的 userIds、roomId（L2）、type、
// data 解析后按键排序重新序列化的字节、生效的 ttlSeconds。序列化保留数字原文。
func pushContentHash(cmd PushCommand) (string, error) {
	data, err := canonicalJSON(cmd.Data)
	if err != nil {
		return "", err
	}
	users := slices.Clone(cmd.UserIDs)
	slices.Sort(users)
	b, err := json.Marshal(struct {
		Target  string          `json:"target"`
		UserIDs []string        `json:"userIds"`
		Type    string          `json:"type"`
		Data    json.RawMessage `json:"data"`
		TTL     int64           `json:"ttlSeconds"`
	}{cmd.Target, users, cmd.Type, data, cmd.TTL})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// canonicalJSON 按键排序重新序列化；用 json.Number 保留数字原文，否则超过 2^53 的两个不同整数会被判成同内容（AC-PUSH-06-16）。
func canonicalJSON(raw json.RawMessage) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return json.Marshal(v)
}

// pushSync 是离线补收（F-PUSH-03）。IM_PUSH_ENABLED 关闭时 4001，SDK 静默忽略（F-PUSH-01）。
func (s *Service) pushSync(ctx context.Context, c delivery.Session, raw json.RawMessage) (any, *protocol.Error) {
	if s.push == nil {
		return nil, protocol.Errorf(protocol.CodeUnknownCmd, "unknown cmd %q", protocol.CmdPushSync)
	}
	req, perr := decode[protocol.PushSyncReq](raw)
	if perr != nil {
		return nil, perr
	}
	limit, perr := req.Validate()
	if perr != nil {
		return nil, perr
	}
	uid := c.UserID()
	if !s.push.syncLimit.Allow(uid) {
		return nil, protocol.Errorf(protocol.CodeRateLimited, "too many pushSync requests")
	}
	out := protocol.PushSyncAck{Pushes: []protocol.PushData{}}
	if req.FromLatest != nil {
		latest, err := s.store.LatestPushSeq(ctx, uid)
		if err != nil {
			return nil, s.internal("latest push seq", err)
		}
		out.LatestSeq = latest
		return out, nil
	}
	rows, hasMore, latest, err := s.store.ReadPushInbox(ctx, uid, *req.AfterSeq, limit, s.clock.Now().UnixMilli())
	if err != nil {
		return nil, s.internal("read push inbox", err)
	}
	for _, r := range rows {
		out.Pushes = append(out.Pushes, protocol.PushData{PushID: r.PushID, Type: r.Type, Data: r.Data, TS: r.CreatedAt, Target: protocol.PushTargetUser, PushSeq: r.PushSeq})
	}
	out.HasMore, out.LatestSeq = hasMore, latest
	return out, nil
}

// ListPushInbox 是测试接口 GET /admin/test/push-inbox/{userId}：包括已到期还没清理的行。
func (s *Service) ListPushInbox(ctx context.Context, uid string) ([]store.PushInboxRow, error) {
	return s.store.ListPushInbox(ctx, uid)
}

// PurgePushBatch 清理一批（任务 push_purge）。不跟 IM_PUSH_ENABLED：开关关掉之后，已有的推送也要按时回收。
func (s *Service) PurgePushBatch(ctx context.Context, batch int) (int, error) {
	now := s.clock.Now()
	inbox, records, err := s.store.PurgePush(ctx, now.UnixMilli(), now.Add(-PushRecordRetention).UnixMilli(), batch)
	s.metrics.PushInboxRows.Sub(float64(inbox))
	return inbox + records, err
}
