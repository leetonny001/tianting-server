package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	mrand "math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"tianting/server/internal/adminauth"
	"tianting/server/internal/store"
)

// PRD-admin 的性能场景（BE-205、BE-210）：这些接口只接受签名调用方，压测工具自己配一个。

type adminClient struct {
	base   string
	secret []byte
}

// newAdminCaller 写调用方文件（0600）并把环境变量加到服务端上。密钥每次随机，只存在于这次压测的工作目录。
func newAdminCaller(s *server, dir string) (*adminClient, error) {
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, err
	}
	raw, err := json.Marshal([]map[string]any{{
		"id": "loadtest", "alg": "hmac-sha256", "secret": base64.StdEncoding.EncodeToString(secret),
		"scopes": []string{"user.read", "msg.meta"},
	}})
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "admin-callers.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return nil, err
	}
	s.env = append(s.env, "IM_ADMIN_CALLERS="+path)
	return &adminClient{secret: secret}, nil
}

func (c *adminClient) do(method, path, query string, body any) (int, []byte, error) {
	var raw []byte
	if body != nil {
		var err error
		if raw, err = json.Marshal(body); err != nil {
			return 0, nil, err
		}
	}
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return 0, nil, err
	}
	ts, n := strconv.FormatInt(time.Now().UnixMilli(), 10), base64.RawURLEncoding.EncodeToString(nonce)
	sts := adminauth.StringToSign(adminauth.Request{Method: method, Path: path, RawQuery: query, Timestamp: ts, Nonce: n, Body: raw})
	target := c.base + path
	if query != "" {
		target += "?" + query
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(raw))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("X-IM-Caller", "loadtest")
	req.Header.Set("X-IM-Timestamp", ts)
	req.Header.Set("X-IM-Nonce", n)
	req.Header.Set("X-IM-Signature", adminauth.EncodeSignature(adminauth.SignHMAC(c.secret, sts)))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return resp.StatusCode, b, err
}

// scenarioAdminSearch：TC-PERF（AC-ADM-90-1 的 SQLite 缩小规模）10 万用户，10 个并发各做 100 次随机前缀搜索，第 2 页与第 1 页不重复。
func scenarioAdminSearch(cfg config) result {
	const users, workers, perWorker = 100_000, 10, 100
	r := result{Case: "AC-ADM-90-1（SQLite）", Title: "用户搜索（10 万用户）", Target: "10 个并发各 100 次随机前缀搜索，P95 ≤ 1 秒；第 2 页与第 1 页不重复"}
	s := newServer(cfg.bin, cfg.dir, "admin-search")
	st, err := store.OpenSQLite(s.db, store.SQLiteOptions{AutoMigrate: true})
	if err != nil {
		r.note("打开数据库失败：%v", err)
		return r
	}
	start := time.Now()
	for i := 0; i < users; i += 500 {
		batch := make([]store.User, 0, 500)
		for j := i; j < i+500; j++ {
			id := fmt.Sprintf("u%06d", j)
			batch = append(batch, store.User{UserID: id, Nickname: fmt.Sprintf("Nick%05d", j%50_000)})
		}
		if err := st.UpsertUsers(context.Background(), batch); err != nil {
			st.Close()
			r.note("造数据失败：%v", err)
			return r
		}
	}
	st.Close()
	r.row("造数据", "%d 个用户，%s", users, time.Since(start).Round(time.Second))
	c, err := newAdminCaller(s, cfg.dir)
	if err != nil {
		r.note("配置调用方失败：%v", err)
		return r
	}
	if err := s.start(); err != nil {
		r.note("启动失败：%v", err)
		return r
	}
	defer s.stop()
	c.base = "http://" + s.adminAddr

	var lat latencies
	var mu sync.Mutex
	var bad []string
	type page struct {
		Users []struct {
			UserID string `json:"userId"`
		} `json:"users"`
		NextCursor string `json:"nextCursor"`
	}
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range perWorker {
				// 随机前缀：一半按 userId（1～4 位数字），一半按昵称（大小写不敏感）。
				q := "u" + strconv.Itoa(mrand.IntN(10_000)) // #nosec G404 -- 随机选搜索词，与安全无关
				if mrand.IntN(2) == 0 {                     // #nosec G404
					q = "nick" + strconv.Itoa(mrand.IntN(100)) // #nosec G404
				}
				query := url.Values{"q": {q}, "limit": {"50"}}.Encode()
				t0 := time.Now()
				code, body, err := c.do("GET", "/admin/users", query, nil)
				lat.add(time.Since(t0))
				var p page
				if err == nil && code == 200 {
					err = json.Unmarshal(body, &p)
				}
				if err != nil || code != 200 {
					mu.Lock()
					bad = append(bad, fmt.Sprintf("q=%s code=%d err=%v", q, code, err))
					mu.Unlock()
					continue
				}
				if p.NextCursor == "" {
					continue
				}
				code, body, err = c.do("GET", "/admin/users", url.Values{"q": {q}, "limit": {"50"}, "cursor": {p.NextCursor}}.Encode(), nil)
				var p2 page
				if err == nil && code == 200 {
					err = json.Unmarshal(body, &p2)
				}
				first := map[string]bool{}
				for _, u := range p.Users {
					first[u.UserID] = true
				}
				for _, u := range p2.Users {
					if first[u.UserID] {
						err = fmt.Errorf("duplicate %s across pages", u.UserID)
					}
				}
				if err != nil || code != 200 {
					mu.Lock()
					bad = append(bad, fmt.Sprintf("q=%s page 2 code=%d err=%v", q, code, err))
					mu.Unlock()
				}
			}
		}()
	}
	wg.Wait()
	sum := lat.summary()
	r.row("第 1 页耗时", "%s", sum)
	r.row("结果不符合预期", "%d 次", len(bad))
	for _, b := range bad[:min(len(bad), 3)] {
		r.note("%s", b)
	}
	r.Pass = len(bad) == 0 && sum.P95 <= time.Second
	return r
}

// scenarioAdminMeta：AC-ADM-90-2 的 SQLite 缩小规模：单会话 1 万条，随机 beforeSeq 查询元数据 1000 次，P95 ≤ 500ms。
func scenarioAdminMeta(cfg config) result {
	const msgs, queries = 10_000, 1000
	r := result{Case: "AC-ADM-90-2（SQLite）", Title: "消息元数据（单会话 1 万条）", Target: "随机 beforeSeq 查询 1000 次，P95 ≤ 500ms；每页 50 条"}
	s := newServer(cfg.bin, cfg.dir, "admin-meta")
	d, err := seed(s.db, "c2c:alice:bob", []string{"alice", "bob"}, msgs, 100)
	if err != nil {
		r.note("造数据失败：%v", err)
		return r
	}
	r.row("造数据", "%d 条，%s", msgs, d.Round(time.Second))
	c, err := newAdminCaller(s, cfg.dir)
	if err != nil {
		r.note("配置调用方失败：%v", err)
		return r
	}
	if err := s.start(); err != nil {
		r.note("启动失败：%v", err)
		return r
	}
	defer s.stop()
	c.base = "http://" + s.adminAddr
	var lat latencies
	bad := 0
	for range queries {
		before := int64(100 + mrand.IntN(msgs-100)) // #nosec G404 -- 随机选起点，与安全无关
		t0 := time.Now()
		code, body, err := c.do("POST", "/admin/convs/messages/query", "", map[string]any{"convId": "c2c:alice:bob", "beforeSeq": before})
		lat.add(time.Since(t0))
		var p struct {
			Messages []struct {
				Seq int64 `json:"seq"`
			} `json:"messages"`
		}
		if err == nil {
			err = json.Unmarshal(body, &p)
		}
		if err != nil || code != 200 || len(p.Messages) != 50 || p.Messages[0].Seq != before-1 {
			bad++
		}
	}
	sum := lat.summary()
	r.row("查询耗时", "%s", sum)
	r.row("结果不符合预期", "%d 次", bad)
	r.Pass = bad == 0 && sum.P95 <= 500*time.Millisecond
	return r
}
