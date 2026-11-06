package account

import (
	"context"
	"errors"
	"time"
)

// ErrGateBusy：排队超时，调用方返回 503 / 5030。
var ErrGateBusy = errors.New("password hashing is busy")

// GateKind 是闸门的种类（F-ACC-01 异常，R-CB-25、RS5-02）。
type GateKind int

const (
	// GateGeneral：注册、登录、注销、退出其他设备、重新生成恢复码。IM_ARGON_SLOTS 个，排队最多 1 秒。
	GateGeneral GateKind = iota
	// GatePassword：已登录的改密专用位，匿名请求打满时仍能改密。1 个，在普通闸门之外。
	GatePassword
)

// Gate 限制同时计算的 Argon2：每个 64 MiB，同时最多 slots + 2 个（第 4.1 节）。
// 只管算哈希的路径；token/refresh、logout 不进。先过限流，再进闸门（Guard）。
type Gate struct {
	general, password   chan struct{}
	recover             *recoverSched
	queue, recoverQueue time.Duration
	faults              *Faults
}

// NewGate 按配置创建闸门。
func NewGate(c Config, faults *Faults) *Gate {
	c = c.WithDefaults()
	return &Gate{
		general:      make(chan struct{}, c.ArgonSlots),
		password:     make(chan struct{}, 1),
		recover:      newRecoverSched(c.RecoverSlots),
		queue:        c.GateQueue,
		recoverQueue: c.RecoverQueue,
		faults:       faults,
	}
}

// AcquireRecover 拿 recover 专用位（IM_ARGON_RECOVER_SLOTS），恢复码命中才用；排队时限 IM_RECOVER_QUEUE。按 (userId, gen) 公平调度（RS6-02）。
func (g *Gate) AcquireRecover(ctx context.Context, userID string, gen int64) (func(), error) {
	return g.recover.acquire(ctx, userID, gen, g.recoverQueue)
}

// Acquire 拿一个位，返回释放函数。排队用真实时间，不受测试时钟影响（第 4.4 节）。
func (g *Gate) Acquire(ctx context.Context, kind GateKind) (func(), error) {
	sem, wait := g.general, g.queue
	if kind == GatePassword {
		sem = g.password
	}
	t := time.NewTimer(wait)
	defer t.Stop()
	select {
	case sem <- struct{}{}:
	case <-t.C:
		return nil, ErrGateBusy
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	free := func() { <-sem }
	// Argon2 阻塞钩子（第 4.5 节）：拿到位之后停住，让闸门占满，不依赖真实耗时。专用位不受影响。
	if kind == GateGeneral {
		if err := g.faults.waitArgon(ctx); err != nil {
			free()
			return nil, err
		}
	}
	return free, nil
}
