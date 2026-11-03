package e2e

// PRD-social 5.6 节双节点测试环境（BE-104）：两个 im-server 共享同一个 SQLite 文件，各自独立的测试时钟；
// 测试模式的 Invalidator 把失效消息 POST 给另一个进程。只验证配置和缓存失效（跨节点投递等 B6）。

import (
	"path/filepath"
	"testing"
	"time"

	"tianting/server/internal/app"
	"tianting/server/internal/protocol"
)

func twoNodes(t *testing.T, off bool) (*socEnv, *socEnv) {
	db := filepath.Join(t.TempDir(), "shared.db")
	n1 := newSocEnvCfg(t, func(c *app.Config) { c.DBPath = db })
	n2 := newSocEnvCfg(t, func(c *app.Config) { c.DBPath, c.TestInvalidationOff = db, off })
	n1.app.SetTestPeers([]string{n2.adminSrv.URL})
	n2.app.SetTestPeers([]string{n1.adminSrv.URL})
	return n1, n2
}

// TC-SOC-01-06（AC-SOC-01-5 前半）：节点 1 改模式，连在节点 2 的用户 1 秒内按新模式判定。
func TestTCSOC0106TwoNodeInvalidation(t *testing.T) {
	n1, n2 := twoNodes(t, false)
	n1.usersNamed("alice", "bob")
	alice := n2.dial("alice", "web")
	mustAck[protocol.SendAck](t, alice.sendText("c2c", "bob", "m-2n-0001", "hi")) // 节点 2 缓存了 open
	n1.settings(map[string]any{"mode": "friends"})
	deadline := time.Now().Add(time.Second)
	for i := 2; ; i++ {
		f := alice.sendText("c2c", "bob", cid("m-2n", i), "hi")
		if f.Error != nil && f.Error.Code == protocol.CodeNotFriend {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("node 2 still uses the old mode after 1s: %+v", f.Error)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// TC-SOC-01-07（AC-SOC-01-5 后半）：节点 2 关闭广播时只靠 TTL：推进 59 秒仍旧、61 秒后新。
func TestTCSOC0107TwoNodeTTL(t *testing.T) {
	n1, n2 := twoNodes(t, true)
	n1.usersNamed("alice", "bob")
	alice := n2.dial("alice", "web")
	mustAck[protocol.SendAck](t, alice.sendText("c2c", "bob", "m-ttl-0001", "hi"))
	n1.settings(map[string]any{"mode": "friends"})
	n2.advance(59 * time.Second)
	mustAck[protocol.SendAck](t, alice.sendText("c2c", "bob", "m-ttl-0002", "hi"))
	n2.advance(2 * time.Second)
	mustErr(t, alice.sendText("c2c", "bob", "m-ttl-0003", "hi"), protocol.CodeNotFriend)
}
