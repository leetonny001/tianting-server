package account

import (
	"context"
	"slices"
	"sync"
	"time"
)

// recoverSched 是 recover 专用位的公平调度（F-ACC-05 规则 2，RS6-02）：
//   - 同一个码（userId, gen）同时只算 1 个；
//   - 有空位时按账号轮转，一个账号循环发请求压不住其他账号；
//   - 同一账号里代号更老的码优先，否则持有新码的攻击者能压住受害者的旧码。
//
// 按单个实例生效，集群时随 B6 重新看。
type recoverSched struct {
	mu      sync.Mutex
	slots   int
	running int
	active  map[recoverKey]bool
	waiting map[string][]*recoverWaiter // 账号 → 等待的请求
	order   []string                    // 轮转顺序：排在前面的账号先得到位，得到后移到队尾
}

type recoverKey struct {
	userID string
	gen    int64
}

type recoverWaiter struct {
	gen     int64
	seq     uint64
	granted chan struct{}
}

func newRecoverSched(slots int) *recoverSched {
	return &recoverSched{slots: slots, active: map[recoverKey]bool{}, waiting: map[string][]*recoverWaiter{}}
}

// dispatch 把空位分给等待的请求。调用方持锁。
func (q *recoverSched) dispatch() {
	for q.running < q.slots {
		if !q.grantOne() {
			return
		}
	}
}

func (q *recoverSched) grantOne() bool {
	for i, uid := range q.order {
		list := q.waiting[uid]
		// 代号最老的、这个码现在没有在算的。同代按到达顺序。
		var pick *recoverWaiter
		for _, w := range list {
			if q.active[recoverKey{uid, w.gen}] {
				continue
			}
			if pick == nil || w.gen < pick.gen {
				pick = w
			}
		}
		if pick == nil {
			continue
		}
		q.waiting[uid] = slices.DeleteFunc(list, func(w *recoverWaiter) bool { return w == pick })
		q.order = append(slices.Delete(q.order, i, i+1), uid)
		if len(q.waiting[uid]) == 0 {
			delete(q.waiting, uid)
			q.order = q.order[:len(q.order)-1]
		}
		q.active[recoverKey{uid, pick.gen}] = true
		q.running++
		close(pick.granted)
		return true
	}
	return false
}

// acquire 排队直到拿到位或超时。
func (q *recoverSched) acquire(ctx context.Context, userID string, gen int64, wait time.Duration) (func(), error) {
	w := &recoverWaiter{gen: gen, granted: make(chan struct{})}
	q.mu.Lock()
	if _, ok := q.waiting[userID]; !ok {
		q.order = append(q.order, userID)
	}
	q.waiting[userID] = append(q.waiting[userID], w)
	q.dispatch()
	q.mu.Unlock()

	free := func() {
		q.mu.Lock()
		defer q.mu.Unlock()
		delete(q.active, recoverKey{userID, gen})
		q.running--
		q.dispatch()
	}
	t := time.NewTimer(wait)
	defer t.Stop()
	var cause error
	select {
	case <-w.granted:
		return free, nil
	case <-t.C:
		cause = ErrGateBusy
	case <-ctx.Done():
		cause = ctx.Err()
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	select {
	case <-w.granted: // 超时和拿到位同时发生
		q.running--
		delete(q.active, recoverKey{userID, gen})
		q.dispatch()
		return nil, cause
	default:
	}
	list := slices.DeleteFunc(q.waiting[userID], func(x *recoverWaiter) bool { return x == w })
	if len(list) == 0 {
		delete(q.waiting, userID)
		q.order = slices.DeleteFunc(q.order, func(u string) bool { return u == userID })
	} else {
		q.waiting[userID] = list
	}
	return nil, cause
}
