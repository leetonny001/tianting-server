package e2e

// PRD-social M3 的测试辅助（BE-103、BE-116）：社交管理接口的签名调用方、造数据、测试时钟、负向断言。

import (
	"encoding/json"
	"net/http"
	"slices"
	"testing"
	"time"

	"tianting/server/internal/adminauth"
	"tianting/server/internal/app"
	"tianting/server/internal/protocol"
)

// noFrameWindow 是负向断言的观察窗口（5.6 节："ack 之后 2 秒内收不到任何帧"）。
// 测试里缩短到 300ms：投递在同一进程内同步完成，事件若会发出在 ack 之前就已经写进连接。
const noFrameWindow = 300 * time.Millisecond

type socEnv struct {
	*env
	soc testCaller // 持有 social.settings、social.friends、user.read、audit.read、group.write
	dis testCaller // 持有 group.dissolve
	now int64      // 测试时钟（管理接口签名的时间戳要落在服务端时钟 ±300 秒内）
}

const t0 = int64(1_800_000_000_000) // 测试时钟起点

func newSocEnv(t *testing.T) *socEnv { return newSocEnvCfg(t, nil) }

func newSocEnvCfg(t *testing.T, mutate func(*app.Config)) *socEnv {
	if socialMatrixMode() != "" {
		t.Skip("社交用例自己设置好友模式，不参加 IM_E2E_SOCIAL_MODE 矩阵")
	}
	soc := hmacCaller("soc", adminauth.ScopeSocialSettings, adminauth.ScopeSocialFriends, adminauth.ScopeUserRead, adminauth.ScopeAuditRead, adminauth.ScopeGroupWrite)
	dis := hmacCaller("dis", adminauth.ScopeGroupDissolve)
	e := newEnvCfg(t, func(c *app.Config) {
		c.AdminCallers = append(c.AdminCallers, soc.cfg, dis.cfg)
		if mutate != nil {
			mutate(c)
		}
	})
	s := &socEnv{env: e, soc: soc, dis: dis}
	s.setClock(t0)
	return s
}

// adm 用社交调用方签名调用管理接口。
func (s *socEnv) adm(method, path string, body any) (int, []byte) {
	s.t.Helper()
	return s.signed(s.soc, method, path, body, signOpts{ts: timeMs(s.now)})
}

func (s *socEnv) mustAdm(method, path string, body any) []byte {
	s.t.Helper()
	code, b := s.adm(method, path, body)
	if code != http.StatusOK {
		s.t.Fatalf("%s %s: %d %s", method, path, code, b)
	}
	return b
}

func (s *socEnv) settings(patch map[string]any) {
	s.t.Helper()
	s.mustAdm(http.MethodPatch, "/admin/settings/social", patch)
}

func (s *socEnv) setClock(ms int64) {
	s.t.Helper()
	if code, b := s.adminPost("/admin/test/clock", map[string]any{"nowMs": ms}, adminKey); code != http.StatusOK {
		s.t.Fatalf("set clock: %d %s", code, b)
	}
	s.now = ms
}

// advance 推进测试时钟。
func (s *socEnv) advance(d time.Duration) { s.t.Helper(); s.setClock(s.now + d.Milliseconds()) }

func (s *socEnv) seed(body map[string]any) {
	s.t.Helper()
	if code, b := s.adminPost("/admin/test/social/seed", body, adminKey); code != http.StatusOK {
		s.t.Fatalf("seed: %d %s", code, b)
	}
}

func (s *socEnv) purge() {
	s.t.Helper()
	if code, b := s.adminPost("/admin/test/social/purge", map[string]any{}, adminKey); code != http.StatusOK {
		s.t.Fatalf("purge: %d %s", code, b)
	}
}

func (s *socEnv) resetLimits(uid string) {
	s.t.Helper()
	s.adminPost("/admin/test/limits/reset", map[string]any{"userId": uid}, adminKey)
}

func (s *socEnv) importFriends(pairs ...[2]string) {
	s.t.Helper()
	var ps []any
	for _, p := range pairs {
		ps = append(ps, map[string]any{"a": p[0], "b": p[1]})
	}
	s.mustAdm(http.MethodPost, "/admin/friends/import", map[string]any{"pairs": ps})
}

// seedFriends 给 owner 造 n 个单向好友（f0001…），不注册这些用户。
func (s *socEnv) seedFriends(owner, prefix string, n int) {
	s.t.Helper()
	var fs []any
	for i := range n {
		fs = append(fs, map[string]any{"owner": owner, "peer": prefix + pad4(i)})
	}
	s.seed(map[string]any{"friends": fs})
}

func pad4(i int) string {
	b := []byte("0000")
	for k := 3; k >= 0 && i > 0; k-- {
		b[k] = byte('0' + i%10)
		i /= 10
	}
	return string(b)
}

// 协议结构的测试版本（宽松解析）。
type socialAck struct {
	Status  string               `json:"status"`
	Ver     int64                `json:"ver"`
	Changed bool                 `json:"changed"`
	Items   protocol.SocialItems `json:"items"`
}

func (c *client) social(cmd string, data any) frame {
	c.t.Helper()
	return c.call(cmd, data)
}

func (c *client) apply(to string, extra map[string]any) socialAck {
	c.t.Helper()
	d := map[string]any{"to": to}
	for k, v := range extra {
		d[k] = v
	}
	return mustAck[socialAck](c.t, c.call(protocol.CmdFriendApply, d))
}

func (c *client) respond(from, action string, extra map[string]any) frame {
	c.t.Helper()
	d := map[string]any{"from": from, "action": action}
	for k, v := range extra {
		d[k] = v
	}
	return c.call(protocol.CmdFriendRespond, d)
}

// syncRound 从 sinceVer 拉完整一轮，返回各页。
func (c *client) syncRound(since int64, limit int) []protocol.SocialSyncAck {
	c.t.Helper()
	var pages []protocol.SocialSyncAck
	data := map[string]any{"sinceVer": since, "limit": limit}
	for {
		p := mustAck[protocol.SocialSyncAck](c.t, c.call(protocol.CmdSocialSync, data))
		pages = append(pages, p)
		if !p.HasMore {
			return pages
		}
		data = map[string]any{"cursor": p.Cursor, "limit": limit}
	}
}

// state 拉一轮全量，返回合并后的结果和最后一页。
func (c *client) state() (protocol.SocialSyncAck, protocol.SocialSyncAck) {
	c.t.Helper()
	pages := c.syncRound(0, 500)
	var all protocol.SocialSyncAck
	for _, p := range pages {
		all.Friends = append(all.Friends, p.Friends...)
		all.Requests = append(all.Requests, p.Requests...)
		all.Blocks = append(all.Blocks, p.Blocks...)
	}
	return all, pages[len(pages)-1]
}

func (c *client) ver() int64 {
	c.t.Helper()
	_, last := c.state()
	return last.LatestVer
}

func friendIDs(fs []protocol.FriendItem) []string {
	var out []string
	for _, f := range fs {
		out = append(out, f.UserID)
	}
	slices.Sort(out)
	return out
}

func findReq(rs []protocol.RequestItem, uid, dir string) (protocol.RequestItem, bool) {
	for _, r := range rs {
		if r.UserID == uid && r.Direction == dir {
			return r, true
		}
	}
	return protocol.RequestItem{}, false
}

func findFriend(fs []protocol.FriendItem, uid string) (protocol.FriendItem, bool) {
	for _, f := range fs {
		if f.UserID == uid {
			return f, true
		}
	}
	return protocol.FriendItem{}, false
}

// expectNoFrame 断言在 d 内收不到任何帧（pong 除外），也不能有积压的帧。
func (c *client) expectNoFrame(d time.Duration) {
	c.t.Helper()
	for _, f := range c.pending {
		if f.Cmd != protocol.CmdPong {
			c.t.Fatalf("%s: unexpected frame %s %s", c.uid, f.Cmd, f.Data)
		}
	}
	c.pending = nil
	deadline := time.After(d)
	for {
		select {
		case f, ok := <-c.in:
			if !ok {
				return
			}
			if f.Cmd != protocol.CmdPong {
				c.t.Fatalf("%s: unexpected frame %s %s", c.uid, f.Cmd, f.Data)
			}
		case <-deadline:
			return
		}
	}
}

func (c *client) waitSocialChanged() int64 {
	c.t.Helper()
	var ev protocol.SocialChangedEvent
	json.Unmarshal(c.waitEvent(protocol.EventSocialChanged), &ev)
	return ev.Ver
}

// stripVolatile 去掉 ack 里的 id、ver、时间戳，用于"除 ver、时间戳外逐字段相同"的比较。
func stripVolatile(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var v any
	json.Unmarshal(raw, &v)
	var walk func(any) any
	walk = func(x any) any {
		switch m := x.(type) {
		case map[string]any:
			for k := range m {
				switch k {
				case "ver", "createdAt", "updatedAt", "expireAt", "userId":
					delete(m, k)
				default:
					m[k] = walk(m[k])
				}
			}
		case []any:
			for i := range m {
				m[i] = walk(m[i])
			}
		}
		return x
	}
	b, _ := json.Marshal(walk(v))
	return string(b)
}

func errBody(f frame) string {
	b, _ := json.Marshal(f.Error)
	return string(b)
}

func timeMs(ms int64) time.Time { return time.UnixMilli(ms) }
