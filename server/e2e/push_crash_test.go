package e2e

// 系统推送的进程级场景：kill -9（AC-PUSH-11-1）、广播前退出后重播（AC-PUSH-06-7）。用真实二进制 + 子进程。

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"

	"tianting/server/internal/adminauth"
	"tianting/server/internal/protocol"
	"tianting/server/internal/store"
)

type pushProc struct {
	*serverProc
	e  *env
	c  testCaller
	db string
}

var pushProcEnv = []string{"IM_TEST_MODE=1", "IM_PUSH_ENABLED=1", "IM_PUSH_USER_RATE=1000000", "IM_PUSH_ALL_RATE=1000000", "IM_PUSH_INBOX_MAX=1000000"}

// startPushProc 启动开启推送的子进程：调用方写进 0600 的配置文件（密钥只在这个测试的临时目录里）。
func startPushProc(t *testing.T, bin, db, callers string, c testCaller) *pushProc {
	t.Helper()
	s := startServer(t, bin, db, append(pushProcEnv, "IM_ADMIN_CALLERS="+callers)...)
	return &pushProc{serverProc: s, c: c, db: db, e: &env{t: t, adminSrv: &httptest.Server{URL: "http://" + s.adminAddr}}}
}

func writePushCallers(t *testing.T) (string, testCaller) {
	t.Helper()
	c := hmacCaller("push-all", adminauth.ScopePushUser, adminauth.ScopePushAll)
	raw, err := json.Marshal([]map[string]any{{
		"id": c.cfg.ID, "alg": adminauth.AlgHMAC, "secret": base64.StdEncoding.EncodeToString(c.secret),
		"scopes": []string{"push.user", "push.all"}, "pushTypes": []string{"notice"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "callers.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return path, c
}

func (p *pushProc) post(t *testing.T, body map[string]any) (pushResp, int) {
	t.Helper()
	code, b := p.e.signed(p.c, http.MethodPost, "/admin/push", body, signOpts{})
	var r pushResp
	json.Unmarshal(b, &r)
	return r, code
}

func wsCall(t *testing.T, ws *websocket.Conn, id, cmd string, data any) frame {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"id": id, "cmd": cmd, "data": data})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := ws.Write(ctx, websocket.MessageText, b); err != nil {
		t.Fatal(err)
	}
	for {
		_, raw, err := ws.Read(ctx)
		if err != nil {
			t.Fatalf("read %s: %v", cmd, err)
		}
		var f frame
		json.Unmarshal(raw, &f)
		if f.Cmd == protocol.CmdAck && f.ID == id {
			return f
		}
	}
}

// AC-PUSH-11-1：持续按用户推送并记录已 ack 的 pushId；kill -9 后重启：全部能被 pushSync 取到，pushSeq 不重复；序号接着往后走。
func TestPushAckedSurvivesKill9(t *testing.T) {
	bin := buildServer(t)
	callers, c := writePushCallers(t)
	db := filepath.Join(t.TempDir(), "im.db")
	p := startPushProc(t, bin, db, callers, c)
	p.token(t, "u1")

	var mu sync.Mutex
	acked := map[string]bool{}
	var n atomic.Int64
	done := make(chan struct{})
	var wg sync.WaitGroup
	for w := range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
				}
				id := fmt.Sprintf("p%d-%d", w, n.Add(1))
				// 进程被杀后请求会失败：用 send 的底层请求，不能走会 Fatal 的 e.send。
				resp, err := http.DefaultClient.Do(p.e.signedRequest(c, http.MethodPost, "/admin/push", userPush(id, "u1"), signOpts{}))
				if err != nil {
					continue
				}
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					mu.Lock()
					acked[id] = true
					mu.Unlock()
				}
			}
		}()
	}
	time.Sleep(1500 * time.Millisecond)
	p.kill9(t)
	close(done)
	wg.Wait()
	if len(acked) < 20 {
		t.Fatalf("only %d acked pushes before the kill", len(acked))
	}

	p = startPushProc(t, bin, db, callers, c)
	ws := p.dial(t, p.token(t, "u1"))
	seen, seqs := map[string]bool{}, map[int64]bool{}
	after, latest := int64(0), int64(0)
	for i := 0; ; i++ {
		f := wsCall(t, ws, fmt.Sprintf("r%d", i), protocol.CmdPushSync, map[string]any{"afterSeq": after, "limit": 100})
		a := mustAck[protocol.PushSyncAck](t, f)
		for _, d := range a.Pushes {
			if seen[d.PushID] || seqs[d.PushSeq] {
				t.Fatalf("duplicate %s seq %d", d.PushID, d.PushSeq)
			}
			seen[d.PushID], seqs[d.PushSeq] = true, true
			after = d.PushSeq
		}
		latest = a.LatestSeq
		if !a.HasMore {
			break
		}
	}
	for id := range acked {
		if !seen[id] {
			t.Errorf("acked %s lost after kill -9", id)
		}
	}
	if int64(len(seen)) != latest {
		t.Errorf("rows %d, latestSeq %d: seq not contiguous", len(seen), latest)
	}
	if r, code := p.post(t, userPush("after-restart", "u1")); code != http.StatusOK || !r.Created {
		t.Fatalf("after restart: %d %+v", code, r)
	}
	f := wsCall(t, ws, "last", protocol.CmdPushSync, map[string]any{"afterSeq": latest})
	if a := mustAck[protocol.PushSyncAck](t, f); len(a.Pushes) != 1 || a.Pushes[0].PushSeq != latest+1 {
		t.Fatalf("seq after restart = %+v, latest was %d", a, latest)
	}
}

// AC-PUSH-06-7：幂等记录写入后、广播前进程退出；重启后同号重试：created=false，重新广播，ts 等于首次受理时间，state 变为 done。
// ttlSeconds=0 走同一条路径。审计只有 intent（AC-PUSH-06-15 的前提）。
func TestPushExitBeforeBroadcastReplay(t *testing.T) {
	bin := buildServer(t)
	for _, tc := range []struct {
		name string
		body map[string]any
	}{
		{"all", allPush("crash-1")},
		{"ttl0", with(userPush("crash-1", "u1"), "ttlSeconds", 0)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			callers, c := writePushCallers(t)
			db := filepath.Join(t.TempDir(), "im.db")
			p := startPushProc(t, bin, db, callers, c)
			p.token(t, "u1")
			if code, b := p.e.adminPost("/admin/test/faults/push-exit-before-broadcast", map[string]any{"pushId": "crash-1"}, adminKey); code != http.StatusOK {
				t.Fatalf("fault: %d %s", code, b)
			}
			resp, err := http.DefaultClient.Do(p.e.signedRequest(c, http.MethodPost, "/admin/push", tc.body, signOpts{requestID: "req-crash"}))
			if err == nil {
				resp.Body.Close()
				t.Fatalf("server answered %d instead of exiting", resp.StatusCode)
			}
			if err := p.cmd.Wait(); err == nil {
				t.Fatal("server exited cleanly")
			}

			ro, err := store.OpenSQLiteReadOnly(db)
			if err != nil {
				t.Fatal(err)
			}
			var state string
			var firstTS int64
			if err := ro.QueryRow(`SELECT state, first_ts FROM push_idempotency WHERE push_id = 'crash-1'`).Scan(&state, &firstTS); err != nil {
				t.Fatal(err)
			}
			var intents, results int
			ro.QueryRow(`SELECT COUNT(*) FILTER (WHERE phase = 'intent'), COUNT(*) FILTER (WHERE phase = 'result') FROM admin_audit WHERE request_id = 'req-crash'`).Scan(&intents, &results)
			ro.Close()
			if state != store.PushPending || intents != 1 || results != 0 {
				t.Fatalf("state=%s intents=%d results=%d", state, intents, results)
			}

			time.Sleep(20 * time.Millisecond) // 确保重播时刻和首次受理时间不同
			p = startPushProc(t, bin, db, callers, c)
			ws := p.dial(t, p.token(t, "u1"))
			r, code := p.post(t, tc.body)
			if code != http.StatusOK || r.Created {
				t.Fatalf("replay: %d %+v", code, r)
			}
			ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
			defer cancel()
			_, raw, err := ws.Read(ctx)
			if err != nil {
				t.Fatalf("no rebroadcast: %v", err)
			}
			var f frame
			json.Unmarshal(raw, &f)
			var d protocol.PushData
			json.Unmarshal(f.Data, &d)
			if f.Cmd != protocol.CmdPush || d.PushID != "crash-1" || d.TS != firstTS || d.PushSeq != 0 {
				t.Fatalf("frame = %s, want ts %d", raw, firstTS)
			}
			ro, err = store.OpenSQLiteReadOnly(db)
			if err != nil {
				t.Fatal(err)
			}
			defer ro.Close()
			if err := ro.QueryRow(`SELECT state FROM push_idempotency WHERE push_id = 'crash-1'`).Scan(&state); err != nil || state != store.PushDone {
				t.Fatalf("state = %s, %v", state, err)
			}
			// done 之后再重试：不再广播。
			if r, code := p.post(t, tc.body); code != http.StatusOK || r.Created {
				t.Fatalf("second retry: %d %+v", code, r)
			}
			ctx2, cancel2 := context.WithTimeout(context.Background(), noFrameWindow)
			defer cancel2()
			if _, raw, err := ws.Read(ctx2); err == nil {
				t.Fatalf("rebroadcast after done: %s", raw)
			}
		})
	}
}
