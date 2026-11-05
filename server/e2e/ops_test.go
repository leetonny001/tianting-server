package e2e

// 进程级的运维测试（BE-060、BE-063、BE-064、BE-069）：用真实二进制验证启动保护、迁移、备份恢复、优雅下线。

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/coder/websocket"

	"tianting/server/internal/protocol"
)

func runBin(t *testing.T, bin string, env []string, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// BE-064（ARCH S4）：production 下使用开发默认密钥、开启测试模式等，进程拒绝启动并列出原因。
func TestProductionRefusesUnsafeConfig(t *testing.T) {
	bin := buildServer(t)
	out, err := runBin(t, bin, []string{
		"IM_ENV=production", "IM_QUARANTINE_KEY=ops-test-quarantine-key-0123456789abcdef", "IM_NODE_ID=n1", "IM_OPS_ADDR=off", "IM_ADDR=127.0.0.1:0", "IM_DB=" + filepath.Join(t.TempDir(), "im.db"),
		"IM_JWT_SECRET=dev-secret-change-me-0123456789", "IM_ADMIN_KEY=", "IM_TEST_MODE=1",
	})
	if err == nil {
		t.Fatalf("server started with unsafe production config:\n%s", out)
	}
	for _, want := range []string{"IM_JWT_SECRET must not be the development default", "admin callers are required in production", "IM_TEST_MODE must not be enabled"} {
		if !strings.Contains(out, want) {
			t.Errorf("output does not mention %q:\n%s", want, out)
		}
	}
}

// BE-060：production 默认不自动迁移——新库拒绝启动；执行 migrate up 后可以启动。
func TestProductionRequiresMigrateUp(t *testing.T) {
	bin := buildServer(t)
	db := filepath.Join(t.TempDir(), "im.db")
	prodEnv := []string{
		"IM_ENV=production", "IM_NODE_ID=n1", "IM_OPS_ADDR=off", "IM_ADDR=127.0.0.1:0", "IM_DB=" + db,
		"IM_JWT_SECRET=jwt-0123456789abcdef0123456789abcdef", "IM_ADMIN_KEY=adm-0123456789abcdef0123456789abcdef",
		"IM_ADMIN_LEGACY=1", // 测试用旧密钥调用管理接口（PRD-admin F-KEY-05 规则 3）
		"IM_QUARANTINE_KEY=ops-test-quarantine-key-0123456789abcdef",
	}
	if out, err := runBin(t, bin, prodEnv); err == nil || !strings.Contains(out, "migrate up") {
		t.Fatalf("unmigrated production start: err=%v\n%s", err, out)
	}
	if out, err := runBin(t, bin, []string{"IM_DB=" + db}, "migrate", "up"); err != nil {
		t.Fatalf("migrate up: %v\n%s", err, out)
	}
	if out, err := runBin(t, bin, []string{"IM_DB=" + db}, "migrate", "status"); err != nil || strings.Contains(out, "pending") {
		t.Fatalf("migrate status: %v\n%s", err, out)
	}
	srv := startServer(t, bin, db, prodEnv[0], prodEnv[1], prodEnv[2], prodEnv[5], prodEnv[6], prodEnv[7], prodEnv[8])
	srv.token(t, "alice")
}

// BE-063：服务运行中用命令行备份；停服、恢复到新文件，用恢复的库启动后消息还在。
func TestCLIBackupRestore(t *testing.T) {
	bin := buildServer(t)
	dir := t.TempDir()
	db := filepath.Join(dir, "im.db")
	srv := startServer(t, bin, db)
	srv.token(t, "bob")
	ws := srv.dial(t, srv.token(t, "alice"))
	ws.Write(context.Background(), websocket.MessageText, []byte(`{"id":"r1","cmd":"send","data":{"convType":"c2c","to":"bob","clientMsgId":"cid-backup-1","type":"text","body":{"text":"in the backup"}}}`))
	if _, b, err := ws.Read(context.Background()); err != nil || bytes.Contains(b, []byte(`"error"`)) {
		t.Fatalf("send: %s %v", b, err)
	}

	backup := filepath.Join(dir, "backup.db")
	if out, err := runBin(t, bin, []string{"IM_DB=" + db}, "backup", "-o", backup); err != nil || !strings.Contains(out, "verified") {
		t.Fatalf("backup while running: %v\n%s", err, out)
	}
	srv.kill9(t)

	restored := filepath.Join(dir, "restored.db")
	if out, err := runBin(t, bin, nil, "restore", "-i", backup, "-db", restored); err != nil {
		t.Fatalf("restore: %v\n%s", err, out)
	}
	if out, err := runBin(t, bin, nil, "restore", "-i", backup, "-db", restored); err == nil {
		t.Fatalf("restore over existing db without -force must fail:\n%s", out)
	}
	srv = startServer(t, bin, restored)
	ws = srv.dial(t, srv.token(t, "alice"))
	ws.Write(context.Background(), websocket.MessageText, []byte(`{"id":"r2","cmd":"sync","data":{"convId":"c2c:alice:bob","afterSeq":0}}`))
	_, b, _ := ws.Read(context.Background())
	if !bytes.Contains(b, []byte("in the backup")) {
		t.Fatalf("restored db sync = %s", b)
	}
}

// BE-069（ARCH 2.4、C-55）：SIGTERM 后先 /readyz 变为 503，再给在线连接下发 kicked/serverShutdown 并以 1001 关闭。
func TestGracefulShutdown(t *testing.T) {
	bin := buildServer(t)
	srv := startServer(t, bin, filepath.Join(t.TempDir(), "im.db"), "IM_DRAIN_DELAY=500ms")
	ws := srv.dial(t, srv.token(t, "alice"))

	readyz := func() int {
		resp, err := http.Get("http://" + srv.addr + "/readyz")
		if err != nil {
			return 0
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if code := readyz(); code != http.StatusOK {
		t.Fatalf("readyz before shutdown = %d", code)
	}
	if err := srv.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	// 摘流量阶段：连接还在，/readyz 已经是 503。
	deadline := time.Now().Add(400 * time.Millisecond)
	for readyz() != http.StatusServiceUnavailable {
		if time.Now().After(deadline) {
			t.Fatal("readyz did not turn 503 during drain")
		}
		time.Sleep(20 * time.Millisecond)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, b, err := ws.Read(ctx)
	if err != nil {
		t.Fatalf("expected kicked event, got %v", err)
	}
	var f struct {
		Cmd  string
		Data protocol.KickedEvent
	}
	json.Unmarshal(b, &f)
	if f.Cmd != protocol.CmdEvent || f.Data.Reason != protocol.KickShutdown {
		t.Fatalf("frame = %s, want kicked/serverShutdown", b)
	}
	_, _, err = ws.Read(ctx)
	if code := websocket.CloseStatus(err); code != protocol.CloseGoingAway {
		t.Fatalf("close code %d, want 1001 (%v)", code, err)
	}
	if err := srv.cmd.Wait(); err != nil {
		t.Errorf("server exit: %v (want clean exit)", err)
	}
}

// AC-KEY-01-2、AC-KEY-01-3、AC-KEY-01-5（进程级）：管理端口的启动保护。
func TestAdminAddrStartupGuards(t *testing.T) {
	bin := buildServer(t)
	db := filepath.Join(t.TempDir(), "im.db")
	if out, err := runBin(t, bin, []string{"IM_DB=" + db}, "migrate", "up"); err != nil {
		t.Fatalf("migrate up: %v\n%s", err, out)
	}
	prod := []string{
		"IM_ENV=production", "IM_NODE_ID=n1", "IM_OPS_ADDR=off", "IM_ADDR=127.0.0.1:0", "IM_DB=" + db, "IM_METRICS_ADDR=off",
		"IM_JWT_SECRET=jwt-0123456789abcdef0123456789abcdef", "IM_ADMIN_KEY=adm-0123456789abcdef0123456789abcdef",
		"IM_ADMIN_LEGACY=1", "IM_QUARANTINE_KEY=ops-test-quarantine-key-0123456789abcdef",
	}
	// 01-2：生产环境、非回环管理地址、没有 TLS → 拒绝启动。
	out, err := runBin(t, bin, append(prod, "IM_ADMIN_ADDR=10.0.0.5:8081"))
	if err == nil || !strings.Contains(out, "IM_ADMIN_TLS_CERT") {
		t.Errorf("non-loopback admin addr without TLS: err=%v\n%s", err, out)
	}
	// 01-5：两组地址冲突 → 拒绝启动并指出冲突的两项。
	for _, tc := range []struct {
		env  []string
		want string
	}{
		{[]string{"IM_ADMIN_ADDR=127.0.0.1:9090", "IM_METRICS_ADDR=127.0.0.1:9090"}, "IM_ADMIN_ADDR and IM_METRICS_ADDR"},
		{[]string{"IM_ADMIN_ADDR=127.0.0.1:8081", "IM_OPS_ADDR=127.0.0.1:8081"}, "IM_ADMIN_ADDR and IM_OPS_ADDR"},
	} {
		if out, err := runBin(t, bin, append(append([]string{}, prod...), tc.env...)); err == nil || !strings.Contains(out, tc.want) {
			t.Errorf("%v: err=%v\n%s", tc.env, err, out)
		}
	}
	// 01-3：回环地址允许明文，正常启动并可以签发 token。
	srv := startServer(t, bin, db, prod[0], prod[1], prod[2], prod[6], prod[7], prod[8], prod[9])
	srv.token(t, "alice")
}
