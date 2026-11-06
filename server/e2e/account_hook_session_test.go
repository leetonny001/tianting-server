//go:build imhooks

package e2e

// 连接校验之后、入表之前的暂停（PRD-chat 第 4.5 节）。

import (
	"testing"
	"time"

	"tianting/server/internal/protocol"
	"tianting/server/internal/testhook"
)

// TC-ACC-03-43（AC-ACC-03-41）：连接校验 gen=1 后暂停，另一请求转正 gen=2；放行后入表后复查发现代号不符，tokenExpired（RS5-08）。
func TestHookConnStaleGen(t *testing.T) {
	e := openEnv(t, nil)
	r1 := e.register("alice_1", nil)
	p := testhook.ArmConn("alice_1")
	got := make(chan *client, 1)
	go func() { got <- e.connect("alice_1", r1.AccessToken, testPlatform) }()
	select {
	case <-p.Reached():
	case <-time.After(5 * time.Second):
		t.Fatal("the connection did not reach the hook")
	}
	r2 := e.mustRefresh(r1.RefreshToken)
	e.authOK("other", r2.AccessToken, testPlatform)
	p.Release()
	c := <-got
	waitKicked(c, protocol.KickTokenExpired, protocol.CloseTokenExpired)
}
