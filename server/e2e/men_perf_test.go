package e2e

// AC-MEN-02-3（TC-MEN-02-03）的开发机记录：2000 人群、在线 200、群主发 1000 次 @ 所有人的 ack 延迟。
// 性能用例的判定在 EP（P-12）：开发机结果**只记录，不算通过**；IM_PERF_DEV=1 才运行，3 次用 -count=3。
// 顺序发送（一条 ack 回来再发下一条），不是压测速率；在线的 200 个连接都要收到 1000 条（错误率 0）。

import (
	"fmt"
	"net/http"
	"os"
	"slices"
	"sync"
	"testing"
	"time"

	"tianting/server/internal/app"
	"tianting/server/internal/protocol"
)

func perfDevOnly(t *testing.T) {
	t.Helper()
	if os.Getenv("IM_PERF_DEV") != "1" {
		t.Skip("set IM_PERF_DEV=1: development machine record, not a pass (P-12)")
	}
}

// ackLatencies 顺序调用 send n 次，返回排好序的 ack 延迟；任何一次出错都算失败。
func ackLatencies(t *testing.T, n int, send func(i int) frame) []time.Duration {
	t.Helper()
	all := make([]time.Duration, 0, n)
	for i := range n {
		start := time.Now()
		if f := send(i); f.Error != nil {
			t.Fatalf("send %d: %+v", i, f.Error)
		}
		all = append(all, time.Since(start))
	}
	slices.Sort(all)
	return all
}

func logPerf(t *testing.T, label string, d []time.Duration) {
	t.Helper()
	t.Logf("PERF %s n=%d p50=%v p99=%v max=%v (development machine, sequential; not a pass, P-12)", label, len(d), d[len(d)/2], d[len(d)*99/100], d[len(d)-1])
}

// waitMsgs 等每个连接都收到 n 条 msg 帧（并发等，避免顺序等待放大超时）。
func waitMsgs(t *testing.T, cs []*client, n int) {
	t.Helper()
	var wg sync.WaitGroup
	errs := make(chan string, len(cs))
	for _, c := range cs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got := 0
			for _, f := range c.pending {
				if f.Cmd == protocol.CmdMsg {
					got++
				}
			}
			c.pending = nil
			deadline := time.After(60 * time.Second)
			for got < n {
				select {
				case f, ok := <-c.in:
					if !ok {
						errs <- fmt.Sprintf("%s: closed after %d messages", c.uid, got)
						return
					}
					if f.Cmd == protocol.CmdMsg {
						got++
					}
				case <-deadline:
					errs <- fmt.Sprintf("%s: %d of %d messages", c.uid, got, n)
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
}

// TC-MEN-02-03（AC-MEN-02-3，开发机记录）：2000 人群（群主 o + 1999 个成员），其中 200 人在线；o 顺序发 1000 次 @ 所有人。
// 次数上限调高（TC 前置条件）。断言只有功能部分：1000 次都成功、200 个在线连接都收到 1000 条；延迟只打印。
func TestTC_MEN_02_03_MentionAllAckDevRecord(t *testing.T) {
	perfDevOnly(t)
	e := newC4Env(t, func(c *app.Config) {
		c.MsgActions.MentionAllHourly = 1 << 20
		c.MsgActions.MentionAllDaily = 1 << 20
		c.SendRate = 1 << 20
		c.Gateway.Guard.HandshakeBurst = 1000 // 测试从一个 IP 建 201 个连接，[IM 钟] 冻结时令牌不恢复
	})
	users := []map[string]any{{"userId": "o", "nickname": "o"}}
	all := []string{"o"}
	for i := 1; i < 2000; i++ {
		id := fmt.Sprintf("p%04d", i)
		users = append(users, map[string]any{"userId": id, "nickname": id})
		all = append(all, id)
	}
	for i := 0; i < len(users); i += 500 {
		if code, b := e.adminPost("/admin/users/import", map[string]any{"users": users[i:min(i+500, len(users))]}, adminKey); code != http.StatusOK {
			t.Fatalf("import: %d %s", code, b)
		}
	}
	if code, b := e.adm(http.MethodPost, "/admin/groups", map[string]any{"groupId": "big", "name": "big", "members": all, "owner": "o"}); code != http.StatusCreated {
		t.Fatalf("create: %d %s", code, b)
	}
	o := e.dial("o", "web")
	online := make([]*client, 0, 200)
	for _, id := range all[1:201] {
		online = append(online, e.dial(id, "web"))
	}
	d := ackLatencies(t, 1000, func(i int) frame {
		return sendMentions(o, "big", fmt.Sprintf("cmid-perf-%06d", i), "@所有人 perf", nil, true)
	})
	logPerf(t, "mention-all group=2000 online=200", d)
	waitMsgs(t, online, 1000)
}
