package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/coder/websocket"

	"im-mvp/server/internal/protocol"
)

// TC-MSG-04-*（AC-MSG-04-6）：服务端返回 ack 后被 kill -9，重启后消息仍然能 sync 到。
// 用真实二进制 + 子进程，验证 SQLite synchronous=FULL 确实落盘（BE-005）。
func TestAckedMessageSurvivesKill9(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "im-server")
	build := exec.Command("go", "build", "-o", bin, "../cmd/im-server")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	db := filepath.Join(dir, "im.db")

	srv := startServer(t, bin, db)
	tok := srv.token(t, "alice")
	ws, _, err := websocket.Dial(context.Background(), srv.ws(tok), nil)
	if err != nil {
		t.Fatal(err)
	}
	send := `{"id":"r1","cmd":"send","data":{"convType":"c2c","to":"bob","clientMsgId":"cid-durable-1","type":"text","body":{"text":"must survive"}}}`
	ws.Write(context.Background(), websocket.MessageText, []byte(send))
	_, b, err := ws.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var ack frame
	json.Unmarshal(b, &ack)
	if ack.Error != nil {
		t.Fatalf("send failed: %+v", ack.Error)
	}

	srv.kill9(t) // 收到 ack 后立刻强杀，不给优雅退出的机会

	srv = startServer(t, bin, db)
	ws, _, err = websocket.Dial(context.Background(), srv.ws(srv.token(t, "alice")), nil)
	if err != nil {
		t.Fatal(err)
	}
	ws.Write(context.Background(), websocket.MessageText, []byte(`{"id":"r2","cmd":"sync","data":{"convId":"c2c:alice:bob","afterSeq":0}}`))
	_, b, err = ws.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var sync struct{ Data protocol.SyncAck }
	json.Unmarshal(b, &sync)
	if len(sync.Data.Messages) != 1 || sync.Data.Messages[0].Body.Text != "must survive" {
		t.Fatalf("after kill -9 sync = %s", b)
	}
}

type serverProc struct {
	cmd  *exec.Cmd
	addr string
}

func startServer(t *testing.T, bin, db string) *serverProc {
	t.Helper()
	addr := freeAddr(t)
	cmd := exec.Command(bin)
	cmd.Env = append(os.Environ(), "IM_ADDR="+addr, "IM_DB="+db, "IM_ADMIN_KEY="+adminKey, "IM_JWT_SECRET=e2e-secret-0123456789")
	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	p := &serverProc{cmd: cmd, addr: addr}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	for i := 0; i < 100; i++ {
		if resp, err := http.Get("http://" + addr + "/healthz"); err == nil {
			resp.Body.Close()
			return p
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("server did not become healthy")
	return nil
}

func (p *serverProc) kill9(t *testing.T) {
	t.Helper()
	if err := p.cmd.Process.Kill(); err != nil { // os.Process.Kill 在 Unix 上发送 SIGKILL
		t.Fatal(err)
	}
	p.cmd.Wait()
}

func (p *serverProc) ws(token string) string {
	return fmt.Sprintf("ws://%s/ws?token=%s&platform=web", p.addr, token)
}

func (p *serverProc) token(t *testing.T, uid string) string {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, "http://"+p.addr+"/admin/users/token", bytes.NewReader([]byte(`{"userId":"`+uid+`"}`)))
	req.Header.Set("X-Admin-Key", adminKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var r struct{ Token string }
	json.NewDecoder(resp.Body).Decode(&r)
	return r.Token
}

func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().String()
}
