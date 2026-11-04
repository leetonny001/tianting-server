package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"tianting/server/internal/protocol"
)

// TC-MSG-04-*（AC-MSG-04-6）：服务端返回 ack 后被 kill -9，重启后消息仍然能 sync 到。
// 用真实二进制 + 子进程，验证 SQLite synchronous=FULL 确实落盘（BE-005）。
func TestAckedMessageSurvivesKill9(t *testing.T) {
	bin := buildServer(t)
	db := filepath.Join(t.TempDir(), "im.db")

	srv := startServer(t, bin, db)
	srv.token(t, "bob") // 签发凭证即注册收件人
	ws := srv.dial(t, srv.token(t, "alice"))
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
	ws = srv.dial(t, srv.token(t, "alice"))
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
	cmd       *exec.Cmd
	addr      string
	adminAddr string
}

var (
	buildOnce sync.Once
	builtBin  string
	buildErr  error
)

// buildServer 编译一次服务端二进制，供所有进程级测试共用。
func buildServer(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "im-server-e2e")
		if err != nil {
			buildErr = err
			return
		}
		builtBin = filepath.Join(dir, "im-server")
		if out, err := exec.Command("go", "build", "-o", builtBin, "../cmd/im-server").CombinedOutput(); err != nil {
			buildErr = fmt.Errorf("build: %v\n%s", err, out)
		}
	})
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	return builtBin
}

func startServer(t *testing.T, bin, db string, extraEnv ...string) *serverProc {
	t.Helper()
	logPath := filepath.Join(t.TempDir(), "server.log")
	// freeAddr 探到的端口关掉之后可能被别的进程抢走，服务端就起不来：换一组端口重试。
	for range 3 {
		addr, adminAddr := freeAddr(t), freeAddr(t)
		cmd := exec.Command(bin)
		// IM_METRICS_ADDR=off：默认的 127.0.0.1:9090 会和本机正在运行的其他实例冲突；管理端口同理用随机端口。
		cmd.Env = append(os.Environ(), "IM_ADDR="+addr, "IM_ADMIN_ADDR="+adminAddr, "IM_DB="+db, "IM_ADMIN_KEY="+adminKey, "IM_JWT_SECRET=e2e-secret-0123456789", "IM_METRICS_ADDR=off")
		cmd.Env = append(cmd.Env, extraEnv...)
		log, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		cmd.Stdout, cmd.Stderr = log, log
		err = cmd.Start()
		log.Close()
		if err != nil {
			t.Fatal(err)
		}
		for range 200 {
			if resp, err := http.Get("http://" + addr + "/healthz"); err == nil {
				resp.Body.Close()
				t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
				return &serverProc{cmd: cmd, addr: addr, adminAddr: adminAddr}
			}
			time.Sleep(50 * time.Millisecond)
		}
		cmd.Process.Kill()
		cmd.Wait()
	}
	out, _ := os.ReadFile(logPath)
	t.Fatalf("server did not become healthy:\n%s", out)
	return nil
}

func (p *serverProc) kill9(t *testing.T) {
	t.Helper()
	if err := p.cmd.Process.Kill(); err != nil { // os.Process.Kill 在 Unix 上发送 SIGKILL
		t.Fatal(err)
	}
	p.cmd.Wait()
}

// dial 连接并完成首帧 auth（C-52）。
func (p *serverProc) dial(t *testing.T, token string) *websocket.Conn {
	t.Helper()
	ws, _, err := websocket.Dial(context.Background(), fmt.Sprintf("ws://%s/ws?platform=web", p.addr), nil)
	if err != nil {
		t.Fatal(err)
	}
	ws.Write(context.Background(), websocket.MessageText, []byte(`{"id":"a1","cmd":"auth","data":{"token":"`+token+`"}}`))
	_, b, err := ws.Read(context.Background())
	if err != nil || !bytes.Contains(b, []byte(`"userId"`)) {
		t.Fatalf("auth: %s %v", b, err)
	}
	return ws
}

func (p *serverProc) token(t *testing.T, uid string) string {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, "http://"+p.adminAddr+"/admin/users/token", bytes.NewReader([]byte(`{"userId":"`+uid+`"}`)))
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
