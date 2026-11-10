package e2e

// C4 撤回的文件和特征串（BE-473、BE-474 的文件部分）：TC-RCL-01-06、04-06、04-07（文件）、04-08、04-09，TC-FILE-06-13。
// 环境 E1 + 假 S3 + IM_V13_ADMIN_FEATURES=1。本地结果，CI 不运行；console-server 代理（CS-202）不在这一波。

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"tianting/server/internal/adminauth"
	"tianting/server/internal/app"
	"tianting/server/internal/protocol"
	"tianting/server/internal/quarantine"
)

type c4FileEnv struct {
	*fileEnv
	mod, body, plain testCaller
	now              time.Time
}

func newC4FileEnv(t *testing.T, mutate func(*app.Config)) *c4FileEnv {
	t.Helper()
	mod := hmacCaller("mod", adminauth.ScopeMsgDelete, adminauth.ScopeMsgMeta, adminauth.ScopeAuditRead)
	body := hmacCaller("console-msgread", adminauth.ScopeMsgBodyGroup, adminauth.ScopeMsgBodyC2C, adminauth.ScopeMsgQuarantine)
	body.cfg.BodyAccess = true
	plain := hmacCaller("biz", adminauth.ScopeMsgMeta)
	e := newFileEnv(t, func(c *app.Config) {
		c.V13AdminFeatures = true
		c.QuarantineKey = testQuarantineKey
		c.MsgActions.Enabled = true
		c.MsgActions.RecallRatePerMin = 10000
		c.SendRate = 10000
		c.AdminCallers = append(c.AdminCallers, mod.cfg, body.cfg, plain.cfg)
		if mutate != nil {
			mutate(c)
		}
	})
	f := &c4FileEnv{fileEnv: e, mod: mod, body: body, plain: plain}
	f.now = e.freezeDay()
	return f
}

func (e *c4FileEnv) advanceTo(ts time.Time) {
	e.t.Helper()
	e.setClock(ts)
	e.now = ts
}

func (e *c4FileEnv) call(c testCaller, path string, body any) (int, []byte, http.Header) {
	e.t.Helper()
	req := e.signedRequest(c, http.MethodPost, path, body, signOpts{ts: e.now, operator: "op1", requestID: fmt.Sprintf("r-%d", reqSeq.Add(1))})
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	var buf bytes.Buffer
	buf.ReadFrom(resp.Body)
	return resp.StatusCode, buf.Bytes(), resp.Header
}

// pairWithImage：u1 给 u2 发一张图片和一个文件名带 ZXQ-RCL-3 的文件、一条带 ZXQ-RCL-1 的文本。
func (e *c4FileEnv) pairWithFiles() (u1, u2 *client, tok2 string, img, doc, text protocol.SendAck, imgID, docID string, orig []byte) {
	e.t.Helper()
	tok1 := e.token("u1")
	tok2 = e.token("u2")
	orig = fakeJPEG(800, 600, 64<<10)
	imgID = e.uploadImage(tok1, "photo.jpg", orig, smallThumb())
	docID = e.uploadFile(tok1, "report-ZXQ-RCL-3.pdf", "application/pdf", bytes.Repeat([]byte("%PDF-1.4 "), 100))
	u1, u2 = e.dial("u1", "web"), e.dial("u2", "web")
	img = mustAck[protocol.SendAck](e.t, sendFile(u1, "c2c", "u2", "cmid-f0001", "image", imgID, nil))
	doc = mustAck[protocol.SendAck](e.t, sendFile(u1, "c2c", "u2", "cmid-f0002", "file", docID, nil))
	text = mustAck[protocol.SendAck](e.t, u1.sendText("c2c", "u2", "cmid-f0003", "text with ZXQ-RCL-1 inside"))
	return
}

// TC-RCL-01-06（AC-RCL-01-6）、TC-RCL-04-08（AC-RCL-04-8）：撤回文本和文件之后做检查点，SQLite 主库和 WAL 的原始字节里搜不到 ZXQ-RCL-1、ZXQ-RCL-3；
// files.name 为空；隔离表只有密文（原始字节搜不到），用同一把密钥解密后正文和文件名对得上（原文进了隔离区）。
func TestTC_RCL_01_06_NoPlaintextAfterRecall(t *testing.T) {
	e := newC4FileEnv(t, nil)
	u1, _, _, _, doc, text, _, docID, _ := e.pairWithFiles()
	conv := protocol.C2CConvID("u1", "u2")
	raw := rawDB(t, e.dbPath)
	if !bytes.Contains(raw, []byte("ZXQ-RCL-1")) || !bytes.Contains(raw, []byte("ZXQ-RCL-3")) {
		t.Fatal("control failed: the strings should be on disk before the recall")
	}
	mustRecall(t, u1, conv, text.MsgID)
	mustRecall(t, u1, conv, doc.MsgID)
	raw = rawDB(t, e.dbPath)
	for _, s := range []string{"ZXQ-RCL-1", "ZXQ-RCL-3"} {
		if bytes.Contains(raw, []byte(s)) {
			t.Errorf("%s still in the database file after the recall", s)
		}
	}
	if f := e.file(docID); f.Name != "" || f.State != "revoked" {
		t.Errorf("files row = state %s name %q", f.State, f.Name)
	}
	db := e.db()
	cipher, _ := quarantine.New(testQuarantineKey)
	plain := map[string]string{}
	rows, err := db.Query(`SELECT msg_id, conv_id, nonce, ciphertext, kid FROM message_quarantine`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id, cv, kid string
		var nonce, ct []byte
		rows.Scan(&id, &cv, &nonce, &ct, &kid)
		if bytes.Contains(ct, []byte("ZXQ-RCL")) {
			t.Errorf("ciphertext of %s has plaintext", id)
		}
		p, err := cipher.Open(id, cv, nonce, ct, kid)
		if err != nil {
			t.Fatalf("open %s: %v", id, err)
		}
		plain[id] = string(p)
	}
	rows.Close()
	if !strings.Contains(plain[text.MsgID], "text with ZXQ-RCL-1 inside") || !strings.Contains(plain[doc.MsgID], "report-ZXQ-RCL-3.pdf") {
		t.Errorf("quarantine plaintext = %v", plain)
	}
}

// db 是第二个连接（读断言）。
func (e *c4FileEnv) db() *sql.DB {
	e.t.Helper()
	db, err := sql.Open("sqlite", e.dbPath)
	if err != nil {
		e.t.Fatal(err)
	}
	e.t.Cleanup(func() { db.Close() })
	return db
}

// TC-RCL-04-06（AC-RCL-04-6）：图片被撤回后参与者申请地址，该项 code=4048，和不存在的 fileId 完全相同。
func TestTC_RCL_04_06_RecalledImageURL(t *testing.T) {
	e := newC4FileEnv(t, nil)
	u1, _, tok2, img, _, _, imgID, _, _ := e.pairWithFiles()
	e.mustURL(tok2, imgID, "original")
	mustRecall(t, u1, protocol.C2CConvID("u1", "u2"), img.MsgID)
	for _, v := range []string{"original", "thumb"} {
		_, got, _ := e.urls(tok2, [2]string{imgID, v})
		_, ghost, _ := e.urls(tok2, [2]string{"0193a7c4-0000-7000-8000-00000000dead", v})
		if len(got) != 1 || got[0].Code != protocol.CodeFileNotFound || got[0].URL != "" {
			t.Errorf("%s: %+v", v, got)
		}
		a, _ := json.Marshal(struct{ V, C, U string }{got[0].Variant, fmt.Sprint(got[0].Code), got[0].URL})
		b, _ := json.Marshal(struct{ V, C, U string }{ghost[0].Variant, fmt.Sprint(ghost[0].Code), ghost[0].URL})
		if !bytes.Equal(a, b) {
			t.Errorf("%s: recalled %s differs from missing %s", v, a, b)
		}
	}
}

// TC-RCL-04-07（AC-RCL-04-7，文件部分）：撤回的文件消息在元数据里有 recalled*，没有文件名，也没有 fileSize（和后台删除一致）。
func TestTC_RCL_04_07_MetaRecalledFile(t *testing.T) {
	e := newC4FileEnv(t, nil)
	u1, _, _, _, doc, _, _, _, _ := e.pairWithFiles()
	conv := protocol.C2CConvID("u1", "u2")
	mustRecall(t, u1, conv, doc.MsgID)
	code, b, _ := e.call(e.mod, "/admin/convs/messages/query", map[string]any{"convId": conv})
	if code != 200 || bytes.Contains(b, []byte("ZXQ-RCL-3")) || bytes.Contains(b, []byte(`"body"`)) {
		t.Fatalf("meta: %d %s", code, b)
	}
	var r struct {
		Messages []metaItem `json:"messages"`
	}
	json.Unmarshal(b, &r)
	m := metaOf(t, r.Messages, doc.MsgID)
	if !m.Recalled || m.RecalledBy != "u1" || m.FileSize != nil || m.BodyLength != 0 {
		t.Errorf("meta of the recalled file = %+v (%s)", m, b)
	}
}

// TC-FILE-06-13（AC-FILE-06-3）：撤回的图片第 29 天对象还在、files.state=revoked、用户申请 4048；第 31 天 file_purge 按前缀删除对象，行保留。
func TestTC_FILE_06_13_RecalledFilePurge(t *testing.T) {
	e := newC4FileEnv(t, nil)
	u1, _, tok2, img, _, _, imgID, _, _ := e.pairWithFiles()
	mustRecall(t, u1, protocol.C2CConvID("u1", "u2"), img.MsgID)
	start := e.now
	e.advanceTo(start.Add(29 * 24 * time.Hour))
	e.runPurge()
	if keys := e.s3.Keys(e.s3.Private, "final/"+imgID+"/"); len(keys) < 2 {
		t.Errorf("objects on day 29 = %v (original and thumbnail)", keys)
	}
	if f := e.file(imgID); f.State != "revoked" || f.ObjectsDeleted {
		t.Errorf("day 29 file = %+v", f)
	}
	_ = tok2
	e.denied(e.token("u2"), imgID, "original") // 推进时钟后旧 token 过期，重新签发
	e.advanceTo(start.Add(31 * 24 * time.Hour))
	e.runPurge()
	if keys := e.s3.Keys(e.s3.Private, "final/"+imgID+"/"); len(keys) != 0 {
		t.Errorf("objects on day 31 = %v", keys)
	}
	if f := e.file(imgID); f.State != "revoked" || !f.ObjectsDeleted {
		t.Errorf("day 31 file = %+v", f)
	}
}

// TC-RCL-04-09（AC-RCL-04-9，im-server 一侧）：POST /admin/files/{fileId}/fetch：没有授权 403 / 4039；有隔离授权成功（字节和原图 SHA-256 一致，
// attachment、nosniff）；普通调用方 403 / 4030。审计有 fileId、msgId，没有文件名；fileId 和 msgId 不匹配、没撤回的、对象删除后 404 / 4048；
// 单聊受 IM_C2C_BODY_ACCESS。
func TestTC_RCL_04_09_Fetch(t *testing.T) {
	e := newC4FileEnv(t, func(c *app.Config) { c.C2CBodyAccess = true })
	tok1 := e.token("o")
	e.createGroup("g1", "o", "m")
	orig := fakeJPEG(640, 480, 32<<10)
	imgID := e.uploadImage(tok1, "ZXQ-RCL-3.jpg", orig, smallThumb())
	keepID := e.uploadImage(tok1, "keep.jpg", fakeJPEG(10, 10, 4096), smallThumb())
	o := e.dial("o", "web")
	img := mustAck[protocol.SendAck](t, sendFile(o, "group", "g1", "cmid-f0901", "image", imgID, nil))
	keep := mustAck[protocol.SendAck](t, sendFile(o, "group", "g1", "cmid-f0902", "image", keepID, nil))
	mustRecall(t, o, "group:g1", img.MsgID)
	path := "/admin/files/" + imgID + "/fetch"
	req := map[string]any{"convId": "group:g1", "msgId": img.MsgID}

	code, b, _ := e.call(e.body, path, req)
	expectCode(t, "no grant", code, b, 403, protocol.CodeMsgReadGrantRequired)
	code, b, _ = e.call(e.plain, path, req)
	expectCode(t, "plain caller", code, b, 403, protocol.CodeForbidden)

	code, b, _ = e.call(e.body, "/admin/msg-grants", map[string]any{"convId": "group:g1", "operator": "op1", "reason": "case review", "ticket": "T-1", "quarantine": true})
	if code != 200 {
		t.Fatalf("grant: %d %s", code, b)
	}
	code, b, h := e.call(e.body, path, req)
	if code != 200 || sha256.Sum256(b) != sha256.Sum256(orig) {
		t.Fatalf("fetch: %d (%d bytes)", code, len(b))
	}
	if h.Get("Content-Disposition") != "attachment" || h.Get("X-Content-Type-Options") != "nosniff" || h.Get("Content-Type") != "image/jpeg" || h.Get("Cache-Control") != "no-store" {
		t.Errorf("headers = %v", h)
	}
	code, b, _ = e.call(e.body, path, map[string]any{"convId": "group:g1", "msgId": img.MsgID, "variant": "thumb"})
	if code != 200 || !bytes.Equal(b, smallThumb()) {
		t.Errorf("thumb: %d", code)
	}
	for label, r := range map[string]struct {
		path string
		body map[string]any
	}{
		"wrong msgId":    {path, map[string]any{"convId": "group:g1", "msgId": keep.MsgID}},
		"not recalled":   {"/admin/files/" + keepID + "/fetch", map[string]any{"convId": "group:g1", "msgId": keep.MsgID}},
		"missing fileId": {"/admin/files/0193a7c4-0000-7000-8000-00000000dead/fetch", req},
	} {
		code, b, _ := e.call(e.body, r.path, r.body)
		expectCode(t, label, code, b, 404, protocol.CodeFileNotFound)
	}
	code, b, _ = e.call(e.body, path, map[string]any{"convId": "group:g1", "msgId": img.MsgID, "variant": "big"})
	expectCode(t, "bad variant", code, b, 400, protocol.CodeBadRequest)
	// 审计：有 fileId、msgId，没有文件名。
	code, b, _ = e.callGet(e.mod, fmt.Sprintf("/admin/audit?from=%d&to=%d&limit=200&caller=%s", e.now.Add(-time.Hour).UnixMilli(), e.now.Add(time.Minute).UnixMilli(), e.body.cfg.ID))
	if code != 200 || !bytes.Contains(b, []byte(imgID)) || !bytes.Contains(b, []byte(img.MsgID)) || bytes.Contains(b, []byte("ZXQ-RCL-3")) {
		t.Errorf("audit: %d %s", code, b)
	}
	// 对象过了保留期被删除之后：4048。
	e.advanceTo(e.now.Add(31 * 24 * time.Hour))
	e.runPurge()
	code, b, _ = e.call(e.body, "/admin/msg-grants", map[string]any{"convId": "group:g1", "operator": "op1", "reason": "again", "ticket": "T-2", "quarantine": true})
	if code != 200 {
		t.Fatalf("grant: %d %s", code, b)
	}
	code, b, _ = e.call(e.body, path, req)
	expectCode(t, "after purge", code, b, 404, protocol.CodeFileNotFound)
}

// TC-RCL-04-09 的单聊部分：IM_C2C_BODY_ACCESS=off 时 fetch 403 / 4030 c2cBodyDisabled（和隔离原文相同）。
func TestTC_RCL_04_09_FetchC2CDisabled(t *testing.T) {
	e := newC4FileEnv(t, nil)
	u1, _, _, img, _, _, imgID, _, _ := e.pairWithFiles()
	conv := protocol.C2CConvID("u1", "u2")
	mustRecall(t, u1, conv, img.MsgID)
	code, b, _ := e.call(e.body, "/admin/files/"+imgID+"/fetch", map[string]any{"convId": conv, "msgId": img.MsgID})
	expectCode(t, "c2c disabled", code, b, 403, protocol.CodeForbidden)
	if !bytes.Contains(b, []byte("c2cBodyDisabled")) {
		t.Errorf("reason: %s", b)
	}
}

func (e *c4FileEnv) callGet(c testCaller, path string) (int, []byte, http.Header) {
	e.t.Helper()
	code, b := e.signed(c, http.MethodGet, path, nil, signOpts{ts: e.now})
	return code, b, nil
}
