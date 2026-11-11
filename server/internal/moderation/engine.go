package moderation

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"time"
)

// TopicBlockedWords 是词库的失效主题（规则 2，第 11.4 节）：词库写操作提交后发布，key 为空。各节点收到后重建快照。
const TopicBlockedWords = "blockedWords"

// TTL 是兜底的重新加载间隔（规则 2）：没收到失效通知（例如广播丢了）时，按 [IM 钟] 超过这个时长的下一次匹配触发重新加载。
const TTL = 60 * time.Second

// RetryDelay 是重新加载失败之后的重试间隔。
const RetryDelay = 2 * time.Second

// Word 是一个启用的词：ID 只用于记录命中，Norm、Compact 是 Normalize 的两份结果。
type Word struct {
	ID            int64
	Norm, Compact string
}

// Hit 是一次命中：只有词 ID，不带词的内容（规则 6、7：响应、日志、审计都不记命中的词）。
type Hit struct {
	WordID int64
}

// Deps 是引擎的依赖。
type Deps struct {
	Load func(ctx context.Context) ([]Word, error) // 读启用的词（store.ActiveWords）
	Now  func() time.Time                          // [IM 钟]
	// ReloadFails 是测试模式的故障注入 wordsReload（第 4.5 节）；生产为 nil。
	ReloadFails func() bool
	// ReloadError 在重新加载失败时调用（im_mod_reload_errors_total 加 1，AC-MOD-01-8 第 1 种）。
	ReloadError func()
	Log         *slog.Logger
}

// snapshot 是一份不可变的词库：两个自动机和词 ID。整体替换，匹配不加锁（规则 4）。
type snapshot struct {
	norm, compact *automaton
	ids           []int64
	loadedAt      time.Time
}

// Engine 持有当前快照，负责启动加载、失效重建、TTL 兜底。
type Engine struct {
	deps    Deps
	cur     atomic.Pointer[snapshot]
	kick    chan struct{} // 合并重建请求：失效回调不能阻塞（cache.Invalidator 的约定）
	stop    chan struct{}
	reloads atomic.Int64 // 成功的加载次数（测试用来等重建完成）
}

// ErrInjected 是故障注入的加载失败。
var ErrInjected = errors.New("injected blocked words load failure")

// New 创建引擎，不加载。
func New(d Deps) *Engine {
	return &Engine{deps: d, kick: make(chan struct{}, 1), stop: make(chan struct{})}
}

// Start 同步加载一次，然后在后台处理重建请求。IM_MOD_ENABLED=1 时加载失败启动失败（F-MOD-01 异常，AC-MOD-01-8 第 2 种）：
// 不做"放行但没人知道"（R-CS-30）。failStartup 是测试模式的 IM_TEST_WORDS_STARTUP_FAIL。
func (e *Engine) Start(ctx context.Context, failStartup bool) error {
	if failStartup {
		return ErrInjected
	}
	if err := e.load(ctx); err != nil {
		return err
	}
	go e.loop()
	return nil
}

// Close 停止后台重建。
func (e *Engine) Close() { close(e.stop) }

// Invalidate 请求重建（失效回调、TTL 都走这里），不阻塞。
func (e *Engine) Invalidate() {
	select {
	case e.kick <- struct{}{}:
	default:
	}
}

// Reloads 返回成功加载的次数（含启动那一次）。
func (e *Engine) Reloads() int64 { return e.reloads.Load() }

// Size 返回当前快照的词数。
func (e *Engine) Size() int {
	if s := e.cur.Load(); s != nil {
		return len(s.ids)
	}
	return 0
}

func (e *Engine) loop() {
	for {
		select {
		case <-e.stop:
			return
		case <-e.kick:
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			err := e.reload(ctx)
			cancel()
			if err != nil {
				// 沿用内存里的旧快照并告警（F-MOD-01 异常）。日志只有错误，不含任何词。
				if e.deps.ReloadError != nil {
					e.deps.ReloadError()
				}
				if e.deps.Log != nil {
					e.deps.Log.Error("blocked words reload failed; keeping the previous list", "err", err)
				}
				// 过一会儿再试：故障恢复后不用等下一次失效通知或 TTL（AC-MOD-01-8：解除注入后 10 秒内新词生效）。
				time.AfterFunc(RetryDelay, e.Invalidate)
			}
		}
	}
}

func (e *Engine) reload(ctx context.Context) error {
	if e.deps.ReloadFails != nil && e.deps.ReloadFails() {
		return ErrInjected
	}
	return e.load(ctx)
}

func (e *Engine) load(ctx context.Context) error {
	words, err := e.deps.Load(ctx)
	if err != nil {
		return err
	}
	e.cur.Store(build(words, e.deps.Now()))
	e.reloads.Add(1)
	return nil
}

func build(words []Word, now time.Time) *snapshot {
	norms, compacts := make([]string, len(words)), make([]string, len(words))
	ids := make([]int64, len(words))
	for i, w := range words {
		norms[i], compacts[i], ids[i] = w.Norm, w.Compact, w.ID
	}
	return &snapshot{norm: buildAutomaton(norms), compact: buildAutomaton(compacts), ids: ids, loadedAt: now}
}

// Match 检查一段文本：标准化后先查 norm，再查 compact（规则 3）。词库为空时不拦截（规则 6）。
// 按 [IM 钟] 超过 TTL（或时钟往回调了）时顺带请求一次重建，这一次仍用当前快照。
func (e *Engine) Match(text string) (Hit, bool) {
	s := e.cur.Load()
	if s == nil {
		return Hit{}, false
	}
	if now := e.deps.Now(); now.Sub(s.loadedAt) > TTL || now.Before(s.loadedAt) {
		e.Invalidate()
	}
	if text == "" || len(s.ids) == 0 {
		return Hit{}, false
	}
	n, c := Normalize(text)
	if i := s.norm.find(n); i >= 0 {
		return Hit{WordID: s.ids[i]}, true
	}
	if i := s.compact.find(c); i >= 0 {
		return Hit{WordID: s.ids[i]}, true
	}
	return Hit{}, false
}

// 检查场景（moderation_hits.scene、im_blocked_total 的 scene，backend-chat 第 110 条），和 metrics.BlockScenes 一致，取值有界。
const (
	SceneText        = "text"
	SceneFileName    = "fileName"
	SceneNickname    = "nickname"
	SceneUsername    = "username"
	SceneGroupName   = "groupName"
	SceneGroupNotice = "groupNotice"
	SceneFriendApply = "friendApply"
)

// Scenes 是全部场景（测试用来核对和 metrics.BlockScenes 一致）。
var Scenes = []string{SceneText, SceneFileName, SceneNickname, SceneUsername, SceneGroupName, SceneGroupNotice, SceneFriendApply}
