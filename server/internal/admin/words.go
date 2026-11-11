package admin

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"tianting/server/internal/adminauth"
	"tianting/server/internal/moderation"
	"tianting/server/internal/protocol"
	"tianting/server/internal/store"
)

// C6 词库管理接口（PRD-chat 第 11.6 节、F-MOD-01，BE-493；形状见 protocol/ADMIN.md"敏感词和举报"一节）。scope moderation.words。
// 不跟 IM_MOD_ENABLED、IM_V13_ADMIN_FEATURES 注册：开关关闭时管理接口照常（P-48 ④）。写操作提交后发失效主题 blockedWords（规则 2）。
// 审计只记操作、wordId 和数量，不记词内容（F-KEY-04 的白名单只记字段名，AC-MOD-01-6）；请求日志不打请求体。

const (
	maxWordImport  = 5000 // 一次最多 5000 条，超出 4131、整批不写入（AC-MOD-01-4）
	defaultWordsPg = 50
	maxWordsPg     = 200
)

type wordJSON struct {
	WordID    int64  `json:"wordId"`
	Word      string `json:"word"`
	State     string `json:"state"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

func toWordJSON(w store.BlockedWord) wordJSON {
	return wordJSON{WordID: w.WordID, Word: w.Word, State: w.State, CreatedAt: w.CreatedAt, UpdatedAt: w.UpdatedAt}
}

func (a *Admin) registerWords(mux *http.ServeMux) {
	a.route(mux, "GET", "/admin/moderation/words", adminauth.ScopeModerationWords, a.listWords)
	a.route(mux, "POST", "/admin/moderation/words", adminauth.ScopeModerationWords, a.addWord)
	a.route(mux, "POST", "/admin/moderation/words/import", adminauth.ScopeModerationWords, a.importWords)
	a.route(mux, "PATCH", "/admin/moderation/words/{wordId}", adminauth.ScopeModerationWords, a.patchWord)
	a.route(mux, "DELETE", "/admin/moderation/words/{wordId}", adminauth.ScopeModerationWords, a.deleteWord)
	if a.content != nil {
		a.route(mux, "GET", "/admin/moderation/hits", adminauth.ScopeReportRead, a.userHits)
	}
}

// wordsChanged 通知各节点重建词库快照（10 秒内生效，规则 2）。
func (a *Admin) wordsChanged() { a.inv.Invalidate(moderation.TopicBlockedWords, "") }

func (a *Admin) listWords(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	query := store.WordQuery{State: q.Get("state"), Limit: defaultWordsPg}
	if query.State != "" && query.State != store.WordActive && query.State != store.WordDisabled {
		writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "state must be active or disabled"))
		return
	}
	if v := q.Get("q"); v != "" {
		query.Q, _ = moderation.Normalize(v) // 按标准化后的子串匹配
	}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > maxWordsPg {
			writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "limit must be 1..%d", maxWordsPg))
			return
		}
		query.Limit = n
	}
	if v := q.Get("cursor"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n < 0 {
			writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "invalid cursor"))
			return
		}
		query.Cursor = n
	}
	rows, next, err := a.store.ListWords(r.Context(), query)
	if err != nil {
		a.internal(w, "list words", err)
		return
	}
	out := make([]wordJSON, len(rows))
	for i, row := range rows {
		out[i] = toWordJSON(row)
	}
	writeJSON(w, http.StatusOK, struct {
		Words      []wordJSON `json:"words"`
		NextCursor int64      `json:"nextCursor"`
	}{out, next})
}

func (a *Admin) addWord(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Word string `json:"word"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	word, normed, compact, ok := moderation.PrepareWord(req.Word)
	if !ok { // 单个添加同样校验（backend-chat 第 102 条）；错误信息不带词
		writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "word must be 1..%d characters after trimming", moderation.MaxWordLen))
		return
	}
	got, created, err := a.store.AddWord(r.Context(), store.BlockedWord{Word: word, Norm: normed, Compact: compact, CreatedAt: a.clock.Now().UnixMilli()})
	if err != nil {
		a.internal(w, "add word", err)
		return
	}
	noteAuditFields(r.Context(), map[string]any{"wordId": got.WordID, "created": created})
	if created {
		a.wordsChanged()
	}
	writeJSON(w, http.StatusOK, struct {
		Word    wordJSON `json:"word"`
		Created bool     `json:"created"`
	}{toWordJSON(got), created})
}

func (a *Admin) importWords(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Words []string `json:"words"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if len(req.Words) > maxWordImport {
		writeErr(w, protocol.Errorf(protocol.CodeBatchLimitExceeded, "at most %d words per import", maxWordImport))
		return
	}
	now := a.clock.Now().UnixMilli()
	rows := make([]store.BlockedWord, 0, len(req.Words))
	invalid := 0
	for _, raw := range req.Words {
		word, normed, compact, ok := moderation.PrepareWord(raw)
		if !ok { // 空行、超过 64 个码点：跳过并计入结果（AC-MOD-01-9）
			invalid++
			continue
		}
		rows = append(rows, store.BlockedWord{Word: word, Norm: normed, Compact: compact, CreatedAt: now})
	}
	added, dup, err := a.store.ImportWords(r.Context(), rows)
	if err != nil {
		a.internal(w, "import words", err)
		return
	}
	noteAuditFields(r.Context(), map[string]any{"added": added, "duplicate": dup, "invalid": invalid})
	noteAuditCount(r.Context(), len(req.Words))
	if added > 0 {
		a.wordsChanged()
	}
	writeJSON(w, http.StatusOK, struct {
		Added     int `json:"added"`
		Duplicate int `json:"duplicate"`
		Invalid   int `json:"invalid"`
	}{added, dup, invalid})
}

func wordIDOf(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("wordId"), 10, 64)
	if err != nil || id < 1 {
		writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "invalid wordId"))
		return 0, false
	}
	return id, true
}

func (a *Admin) patchWord(w http.ResponseWriter, r *http.Request) {
	id, ok := wordIDOf(w, r)
	if !ok {
		return
	}
	var req struct {
		State string `json:"state"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if req.State != store.WordActive && req.State != store.WordDisabled {
		writeErr(w, protocol.Errorf(protocol.CodeBadRequest, "state must be active or disabled"))
		return
	}
	noteAuditFields(r.Context(), map[string]any{"wordId": id, "state": req.State})
	got, err := a.store.SetWordState(r.Context(), id, req.State, a.clock.Now().UnixMilli())
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeErr(w, protocol.Errorf(protocol.CodeNotFound, "word not found"))
		return
	case err != nil:
		a.internal(w, "set word state", err)
		return
	}
	a.wordsChanged()
	writeJSON(w, http.StatusOK, struct {
		Word wordJSON `json:"word"`
	}{toWordJSON(got)})
}

func (a *Admin) deleteWord(w http.ResponseWriter, r *http.Request) {
	id, ok := wordIDOf(w, r)
	if !ok {
		return
	}
	noteAuditFields(r.Context(), map[string]any{"wordId": id})
	switch err := a.store.DeleteWord(r.Context(), id); {
	case errors.Is(err, store.ErrNotFound):
		writeErr(w, protocol.Errorf(protocol.CodeNotFound, "word not found"))
		return
	case err != nil:
		a.internal(w, "delete word", err)
		return
	}
	a.wordsChanged()
	writeJSON(w, http.StatusOK, struct{}{})
}

// ContentService 是 C6 的业务层部分（由 logic.Service 实现）。
type ContentService interface {
	HitsDays() int
}

// WithContent 接上 C6 的业务层。要在 Register 之前调用；不跟开关（P-48 ④）。
func (a *Admin) WithContent(svc ContentService) { a.content = svc }

const maxRecentHits = 50

// userHits 是 GET /admin/moderation/hits?userId=（scope report.read，P-48 ③：命中次数只给有 report.read 的）：
// 保留期内按场景的次数和最近 50 条（时间、场景、wordId），没有任何内容（backend-chat 第 114 条）。不检查用户是否存在。写审计（targets 是 userId）。
func (a *Admin) userHits(w http.ResponseWriter, r *http.Request) {
	uid := r.URL.Query().Get("userId")
	if !protocol.ValidID(uid) {
		writeErr(w, protocol.Errorf(protocol.CodeInvalidID, "invalid userId"))
		return
	}
	days := a.content.HitsDays()
	since := a.clock.Now().Add(-time.Duration(days) * 24 * time.Hour).UnixMilli()
	counts, recent, err := a.store.UserHits(r.Context(), uid, since, maxRecentHits)
	if err != nil {
		a.internal(w, "user hits", err)
		return
	}
	type hitJSON struct {
		At     int64  `json:"at"`
		Scene  string `json:"scene"`
		WordID int64  `json:"wordId"`
	}
	out := make([]hitJSON, len(recent))
	for i, h := range recent {
		out[i] = hitJSON{h.At, h.Scene, h.WordID}
	}
	noteAuditCount(r.Context(), len(out))
	writeJSON(w, http.StatusOK, struct {
		UserID string         `json:"userId"`
		Days   int            `json:"days"`
		Counts map[string]int `json:"counts"`
		Recent []hitJSON      `json:"recent"`
	}{uid, days, counts, out})
}
