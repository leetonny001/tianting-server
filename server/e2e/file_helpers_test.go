package e2e

// C2 图片和文件 e2e 的公共设施（BE-440 第 1 条的后端部分）。对象存储用进程内的假 S3（objstoretest，独立端口，校验 SigV4），
// 公开桶另起一个代理当作独立域名（F-ACC-06 规则 4）。**假 S3 只验调度逻辑，不证明厂商行为**（AC-FILE-90-1、90-2）。

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"tianting/server/internal/app"
	"tianting/server/internal/objstore"
	"tianting/server/internal/objstore/objstoretest"
	"tianting/server/internal/protocol"
	"tianting/server/internal/store"
)

type fileEnv struct {
	*acctEnv
	s3  *objstoretest.Server
	pub *httptest.Server // 公开桶的独立域名
}

// newFileEnv 启动 IM_FILES_ENABLED=1 的环境（E1 + 假 S3）。
func newFileEnv(t *testing.T, mutate func(*app.Config)) *fileEnv {
	t.Helper()
	s3 := objstoretest.New(t)
	pub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, _ := http.NewRequestWithContext(r.Context(), r.Method, s3.URL()+"/"+s3.Public+r.URL.Path, nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		for k, v := range resp.Header {
			w.Header()[k] = v
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	}))
	t.Cleanup(pub.Close)
	e := newAcctEnv(t, func(c *app.Config) {
		c.Files.Enabled = true
		c.ObjStore = s3.Config()
		c.ObjStore.PublicBaseURL = pub.URL
		if mutate != nil {
			mutate(c)
		}
	})
	return &fileEnv{acctEnv: e, s3: s3, pub: pub}
}

// call 发 /v1/*，带 Bearer。body 为 nil 时没有请求体。
func (e *fileEnv) call(method, path, token string, body any) (int, []byte) {
	e.t.Helper()
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, e.srv.URL+path, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, out
}

type uploadResp struct {
	FileID   string              `json:"fileId"`
	Put      objstore.Presigned  `json:"put"`
	ThumbPut *objstore.Presigned `json:"thumbPut"`
}

func (e *fileEnv) requestUpload(token string, body map[string]any) (int, uploadResp, []byte) {
	e.t.Helper()
	code, b := e.call(http.MethodPost, "/v1/files/uploads", token, body)
	var r uploadResp
	_ = json.Unmarshal(b, &r)
	return code, r, b
}

func (e *fileEnv) mustRequest(token string, body map[string]any) uploadResp {
	e.t.Helper()
	code, r, b := e.requestUpload(token, body)
	if code != http.StatusOK || r.FileID == "" {
		e.t.Fatalf("request upload: %d %s", code, b)
	}
	return r
}

// put 按预签名 PUT。extra 是额外的请求头（恶意头）。
func put(t *testing.T, p objstore.Presigned, data []byte, extra map[string]string) int {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPut, p.URL, bytes.NewReader(data))
	for k, v := range p.Headers {
		if !strings.EqualFold(k, "Content-Length") {
			req.Header.Set(k, v)
		}
	}
	for k, v := range extra {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func (e *fileEnv) mustPut(p objstore.Presigned, data []byte) {
	e.t.Helper()
	if code := put(e.t, p, data, nil); code != http.StatusOK {
		e.t.Fatalf("PUT = %d", code)
	}
}

func (e *fileEnv) complete(token, id string) (int, []byte) {
	e.t.Helper()
	return e.call(http.MethodPost, "/v1/files/"+id+"/complete", token, nil)
}

type completeResp struct {
	FileID string `json:"fileId"`
	Size   int64  `json:"size"`
	Mime   string `json:"mime"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

func (e *fileEnv) mustComplete(token, id string) completeResp {
	e.t.Helper()
	code, b := e.complete(token, id)
	if code != http.StatusOK {
		e.t.Fatalf("complete %s: %d %s", id, code, b)
	}
	var r completeResp
	_ = json.Unmarshal(b, &r)
	return r
}

// uploadImage 申请、PUT 原图和缩略图、complete，返回 fileId。
func (e *fileEnv) uploadImage(token, name string, orig, thumb []byte) string {
	e.t.Helper()
	r := e.mustRequest(token, map[string]any{"kind": "image", "name": name, "size": len(orig), "mime": sniffMime(orig),
		"thumb": map[string]any{"size": len(thumb), "mime": sniffMime(thumb)}})
	e.mustPut(r.Put, orig)
	e.mustPut(*r.ThumbPut, thumb)
	e.mustComplete(token, r.FileID)
	return r.FileID
}

// uploadFile 上传 kind=file。
func (e *fileEnv) uploadFile(token, name, mime string, data []byte) string {
	e.t.Helper()
	r := e.mustRequest(token, map[string]any{"kind": "file", "name": name, "size": len(data), "mime": mime})
	e.mustPut(r.Put, data)
	e.mustComplete(token, r.FileID)
	return r.FileID
}

func sniffMime(b []byte) string {
	switch {
	case bytes.HasPrefix(b, []byte("\x89PNG")):
		return "image/png"
	case bytes.HasPrefix(b, []byte{0xFF, 0xD8}):
		return "image/jpeg"
	case bytes.HasPrefix(b, []byte("GIF8")):
		return "image/gif"
	}
	return "application/octet-stream"
}

type urlItem struct {
	FileID    string `json:"fileId"`
	Variant   string `json:"variant"`
	URL       string `json:"url"`
	ExpiresAt int64  `json:"expiresAt"`
	Code      int    `json:"code"`
}

func (e *fileEnv) urls(token string, items ...[2]string) (int, []urlItem, []byte) {
	e.t.Helper()
	req := make([]map[string]string, len(items))
	for i, it := range items {
		req[i] = map[string]string{"fileId": it[0], "variant": it[1]}
	}
	code, b := e.call(http.MethodPost, "/v1/files/urls", token, map[string]any{"items": req})
	var r struct {
		Items []urlItem `json:"items"`
	}
	_ = json.Unmarshal(b, &r)
	return code, r.Items, b
}

// mustURL 取一个地址，不可访问时失败。
func (e *fileEnv) mustURL(token, id, variant string) urlItem {
	e.t.Helper()
	code, items, b := e.urls(token, [2]string{id, variant})
	if code != http.StatusOK || len(items) != 1 || items[0].URL == "" {
		e.t.Fatalf("urls %s/%s: %d %s", id, variant, code, b)
	}
	return items[0]
}

// denied 断言这一项 code=4048，响应和不存在的 fileId 完全相同（F-FILE-04 规则 3）。
func (e *fileEnv) denied(token, id, variant string) {
	e.t.Helper()
	code, items, b := e.urls(token, [2]string{id, variant})
	if code != http.StatusOK || len(items) != 1 || items[0].Code != protocol.CodeFileNotFound || items[0].URL != "" {
		e.t.Fatalf("urls %s/%s should be 4048: %d %s", id, variant, code, b)
	}
}

func download(t *testing.T, u string) (int, http.Header, []byte) {
	t.Helper()
	resp, err := http.Get(u)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, b
}

func (e *fileEnv) file(id string) store.File {
	e.t.Helper()
	f, err := e.st.GetFile(context.Background(), id)
	if err != nil {
		e.t.Fatalf("get file %s: %v", id, err)
	}
	return f
}

func (e *fileEnv) fileGone(id string) bool {
	e.t.Helper()
	_, err := e.st.GetFile(context.Background(), id)
	return err == store.ErrNotFound
}

func (e *fileEnv) runPurge() {
	e.t.Helper()
	if code, b := e.adminPost("/admin/test/jobs/file_purge/run", nil, adminKey); code != http.StatusOK {
		e.t.Fatalf("run file_purge: %d %s", code, b)
	}
}

// runPurgeCode 跑一次 file_purge，返回 HTTP 状态（失败时测试接口返回 400）。
func (e *fileEnv) runPurgeCode() int {
	e.t.Helper()
	code, _ := e.adminPost("/admin/test/jobs/file_purge/run", nil, adminKey)
	return code
}

func errOf(b []byte) protocol.Error {
	var pe protocol.Error
	_ = json.Unmarshal(b, &pe)
	return pe
}

// ---- 测试图片：只构造头部（服务端只读头部、不解码像素），后面用填充补到指定大小 ----

func pad(b []byte, total int) []byte {
	if len(b) < total {
		b = append(b, bytes.Repeat([]byte{0x5A}, total-len(b))...)
	}
	return b
}

// fakePNG 是头部写着 w×h 的 png。
func fakePNG(w, h, total int) []byte {
	b := []byte("\x89PNG\r\n\x1a\n")
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:4], uint32(w))
	binary.BigEndian.PutUint32(ihdr[4:8], uint32(h))
	ihdr[8], ihdr[9] = 8, 6
	chunk := append([]byte("IHDR"), ihdr...)
	b = binary.BigEndian.AppendUint32(b, 13)
	b = append(b, chunk...)
	b = binary.BigEndian.AppendUint32(b, crc32.ChecksumIEEE(chunk))
	return pad(b, total)
}

// fakeJPEG 是 SOF0 里写着 w×h 的 jpeg；w=0 时没有 SOF（解析不到尺寸）。
func fakeJPEG(w, h, total int) []byte {
	b := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0, 1, 1, 0, 0, 1, 0, 1, 0, 0}
	if w > 0 {
		sof := []byte{0xFF, 0xC0, 0x00, 0x11, 0x08, byte(h >> 8), byte(h), byte(w >> 8), byte(w), 0x03, 1, 0x22, 0, 2, 0x11, 1, 3, 0x11, 1}
		b = append(b, sof...)
	}
	b = append(b, 0xFF, 0xDA, 0x00, 0x08, 1, 1, 0, 0, 0x3F, 0)
	return pad(b, total)
}

// smallThumb 是 240×180 的缩略图。
func smallThumb() []byte { return fakeJPEG(240, 180, 4096) }

// shanghaiNoon 是 Asia/Shanghai 当天 01:00：[IM 钟] 用例从当天开始，避免跨日（R-CQ-41）。
func (e *fileEnv) freezeDay() time.Time {
	t0 := shanghaiMidnight(e.t).Add(time.Hour)
	e.setClock(t0)
	e.t.Cleanup(func() { e.adminPost("/admin/test/clock", map[string]any{"nowMs": nil}, adminKey) })
	return t0
}

// sendFile 用 WS 发 image / file 消息。
func sendFile(c *client, convType, to, clientMsgID, typ, fileID string, extra map[string]any) frame {
	c.t.Helper()
	inner := map[string]any{"fileId": fileID}
	for k, v := range extra {
		inner[k] = v
	}
	return c.call(protocol.CmdSend, map[string]any{"convType": convType, "to": to, "clientMsgId": clientMsgID, "type": typ, "body": map[string]any{typ: inner}})
}

func syncAll(c *client, convID string) []protocol.Message {
	c.t.Helper()
	ack := mustAck[protocol.SyncAck](c.t, c.call(protocol.CmdSync, map[string]any{"convId": convID, "afterSeq": 0, "limit": 200}))
	return ack.Messages
}
