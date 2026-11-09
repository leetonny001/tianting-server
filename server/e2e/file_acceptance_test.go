package e2e

// C2 后端验收（BE-440）：F-FILE-01 攻击序列和测试的对应表；对象存储不可用时文本收发（AC-CHAT-90-3）；
// C2 流程的日志和审计特征串（AC-CHAT-90-5）；文件指标和配置检查（AC-CHAT-90-17）。
// 性能数字只在开发机记录，**不算通过**，发布判定等 P-12。假 S3 只验调度逻辑，不证明厂商行为（AC-FILE-90-1、90-2）。

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"tianting/server/internal/app"
	"tianting/server/internal/objstore/objstoretest"
	"tianting/server/internal/protocol"
)

// F-FILE-01（及头像、清理里同一套键结构）在评审里的攻击序列 → 用例。谁、什么顺序、时钟怎么推写在各用例的注释里。
// 攻击者都是持 PUT 地址的上传者 u1：预签名 PUT 在 15 分钟内可以反复使用（规则 1），所以 complete 的每一步之间都可能换对象。
var fileAttackSequences = map[string][]string{
	"RC2-07 HEAD 之后、嗅探之前换对象；嗅探之后、Copy 之前换对象（条件读、条件 Copy 挡住）": {"TestTC_FILE_01_15_ReplaceDuringVerify"},
	"RC2-08 complete 之后用原地址再 PUT 恶意内容；临时前缀靠生命周期清除":           {"TestTC_FILE_01_09_OverwriteAfterComplete"},
	"RC2-09 弱网失败多轮不被未完成数锁住；取消释放名额":                           {"TestTC_FILE_01_16_PendingFreed", "TestTC_FILE_01_11_RateAndPending"},
	"RS4-16 甲 两个并发 complete，第一个校验后换对象":                       {"TestTC_FILE_01_19_ConcurrentComplete"},
	"RS4-16 乙 缺缩略图 4096 后换原图再补传：内容和记录来自同一次尝试":                {"TestTC_FILE_01_20_ContentFromLastAttempt"},
	"RS4-16 丙 原图 Copy 成功、缩略图 Copy 失败后重试：不跳过已成功的变体":           {"TestTC_FILE_01_10_RetryRedoesAll"},
	"RS4-17 / RS5-21 PUT 带恶意头和标签：最终对象只有服务端写的 Content-Type":   {"TestTC_FILE_01_21_MaliciousHeaders", "TestTC_ACC_06_07_AvatarExposure"},
	"RS4-18 Copy 后按 Copy 响应的 ETag 核对，412 重试 1 次":             {"TestTC_FILE_01_15_ReplaceDuringVerify"},
	"RS4-19 每 15 分钟申请 20 个 100 MiB，集中 complete：当日最多超额一个文件":   {"TestTC_FILE_01_18_QuotaRecheck"},
	"RS4-20 文件名 evil.exe\"; x=\".txt 进 Content-Disposition":  {"TestTC_FILE_01_25_Disposition"},
	"RS4-21 缩略图、头像只由客户端保证尺寸：服务端按头部真实宽高拒绝":                    {"TestTC_FILE_01_22_Dimensions"},
	"RS4-23 甲 DELETE 别人的、已 ready 的上传：响应相同，不暴露存在性":            {"TestTC_FILE_01_23_CancelHidesExistence"},
	"RS4-23 乙 complete 在 Copy 之后被 file_purge 抢先改成 purging":   {"TestTC_FILE_01_24_PurgeBeatsComplete"},
	"RS5-06 Copy 之后、提交之前崩溃：残留按 final/{fileId}/ 前缀清除":         {"TestTC_FILE_01_26_CrashLeftovers"},
	"RS5-07 租约释放、Q1 过期后 Q2 提交、两个尝试同时提交":                      {"TestTC_FILE_01_27_LeaseAndRace"},
	"RS5-16 甲 取消和 complete 并发；取消后再 complete":                 {"TestTC_FILE_01_28_CancelAndComplete"},
	"RS5-16 乙 反复申请再取消，制造不计费的临时对象":                            {"TestTC_FILE_01_29_DeclaredLimit"},
	"RS6-04 头像公开键先写库再 Copy；崩溃后不重试也能清除":                       {"TestTC_ACC_06_12_CrashAndCleanup", "TestTC_ACC_06_13_CrashNoRetry"},
	"RS7-03 任何路径进入 purging 都先删公开对象":                          {"TestTC_ACC_06_13_CrashNoRetry", "TestTC_ACC_06_08_11_Replace"},
	"F-FILE-02 规则 1 客户端伪造 size、mime、宽高":                      {"TestTC_FILE_02_01_SendImage", "TestTC_FILE_01_22_Dimensions"},
	"F-FILE-05 规则 1 png 后面藏 HTML：下载响应是 image/png":            {"TestTC_FILE_05_05_PolyglotServedAsImage", "TestTC_FILE_01_03_SniffRejects"},
	"F-FILE-06 规则 2 purging 和 send 绑定竞争；同一 fileId 两条消息并发":    {"TestTC_FILE_06_06_PurgingCannotBind", "TestTC_FILE_06_07_ConcurrentBind"},
}

// 假 S3 证明不了的（厂商行为），这里没有测，不算通过：AC-FILE-90-1（真实 MinIO）、AC-FILE-90-2（云厂商）。
var fileAttackVendorOnly = []string{
	"PUT 大小和签入的 Content-Length 不符时存储先拒绝（假 S3 按签名拒绝，真实存储的行为见 GATE-objstore）",
	"未签入的 x-amz-* 头被拒绝还是被丢弃（厂商各不相同）",
	"服务端加密下 Copy 前后的 ETag 关系；条件 Copy 在云厂商上的 412",
	"临时前缀生命周期的实际清除时间；CORS 按来源限定",
}

func TestFileAttackSequencesHaveTests(t *testing.T) {
	var src strings.Builder
	files, err := filepath.Glob("*_test.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		src.Write(b)
	}
	for seq, tests := range fileAttackSequences {
		if len(tests) == 0 {
			t.Errorf("%s: no test", seq)
		}
		for _, name := range tests {
			if !regexp.MustCompile(`func ` + name + `\(`).MatchString(src.String()) {
				t.Errorf("%s: no test %s", seq, name)
			}
		}
	}
	if len(fileAttackVendorOnly) == 0 {
		t.Error("list what the fake S3 cannot prove")
	}
}

// deadObjStore 返回一个不可用的对象存储地址。refuse：端口已关闭，连接被拒绝；blackhole：接受连接，从不响应。
func deadObjStore(t *testing.T, mode string) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	if mode == objstoretest.DownRefuse {
		l.Close()
		return "http://" + addr
	}
	var mu sync.Mutex
	var conns []net.Conn
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			c, err := l.Accept()
			if err != nil {
				return // 监听关闭
			}
			mu.Lock()
			conns = append(conns, c) // 持有连接，不读不写
			mu.Unlock()
		}
	}()
	t.Cleanup(func() {
		l.Close()
		<-done
		mu.Lock()
		defer mu.Unlock()
		for _, c := range conns {
			c.Close()
		}
	})
	return "http://" + addr
}

// TC-CHAT-90-03（AC-CHAT-90-3 的 C2 部分）：对象存储按"拒绝连接"（端口已关闭）和"黑洞"（接受连接不响应）两种方式不可用，
// 文本收发 ≥ 1000 条全部成功，对方都收到；期间 /readyz 一直 200、申请上传 200、complete 503 / 5032 且不阻塞发送；
// 配置检查 objstore_reachable 不通过。ack P99 只在开发机记录（不算通过，发布判定等 P-12）；5 分钟时长随 P-12。
func TestTC_CHAT_90_03_ObjStoreDownText(t *testing.T) {
	n := 1000
	if testing.Short() {
		n = 100
	}
	for _, mode := range []string{objstoretest.DownRefuse, objstoretest.DownBlack} {
		t.Run(mode, func(t *testing.T) {
			endpoint := deadObjStore(t, mode)
			e := newAcctEnv(t, func(c *app.Config) {
				c.Files.Enabled = true
				c.ObjStore = objstoretest.New(t).Config()
				c.ObjStore.Endpoint, c.ObjStore.PublicBaseURL = endpoint, endpoint
				c.ObjStore.Timeout = time.Second
				c.SendRate = 100000
			})
			fe := &fileEnv{acctEnv: e}
			tok := e.token("u1")
			e.token("u2")
			c1, c2 := e.dial("u1", "web"), e.dial("u2", "web")
			// 申请只签名，不访问存储（F-FILE-01 规则 8）。
			r := fe.mustRequest(tok, fileReq("down.txt", 5, "text/plain"))
			// 后台不停 complete 和请求 /readyz，和发送同时进行。
			stop := make(chan struct{})
			var wg sync.WaitGroup
			var mu sync.Mutex
			completes, readyBad := map[int]int{}, 0
			var completeErr []string
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-stop:
						return
					default:
					}
					req, _ := http.NewRequest(http.MethodPost, e.srv.URL+"/v1/files/"+r.FileID+"/complete", nil)
					req.Header.Set("Authorization", "Bearer "+tok)
					resp, err := http.DefaultClient.Do(req)
					mu.Lock()
					if err != nil {
						completeErr = append(completeErr, err.Error())
					} else {
						b, _ := io.ReadAll(resp.Body)
						resp.Body.Close()
						completes[resp.StatusCode]++
						if resp.StatusCode != 503 || errOf(b).Code != protocol.CodeObjectStoreUnavailable {
							completeErr = append(completeErr, fmt.Sprintf("%d %s", resp.StatusCode, b))
						}
					}
					mu.Unlock()
					ready, err := http.Get(e.srv.URL + "/readyz")
					mu.Lock()
					if err != nil || ready.StatusCode != http.StatusOK {
						readyBad++
					}
					mu.Unlock()
					if err == nil {
						ready.Body.Close()
					}
				}
			}()
			lat := make([]time.Duration, 0, n)
			for i := range n {
				start := time.Now()
				f := c1.sendText("c2c", "u2", fmt.Sprintf("down-%s-%06d", mode, i), "hi")
				lat = append(lat, time.Since(start))
				if f.Error != nil {
					t.Fatalf("send %d: %+v", i, f.Error)
				}
				if m := c2.waitCmd(protocol.CmdMsg).msg(t); m.Body.Text != "hi" {
					t.Fatalf("recv %d: %+v", i, m)
				}
			}
			close(stop)
			wg.Wait()
			if code, _, b := fe.requestUpload(tok, fileReq("down2.txt", 5, "text/plain")); code != http.StatusOK {
				t.Errorf("request upload while down: %d %s", code, b)
			}
			mu.Lock()
			defer mu.Unlock()
			if completes[503] == 0 || len(completeErr) > 0 || readyBad > 0 {
				t.Errorf("complete results %v, unexpected %v, /readyz not ready %d times", completes, completeErr, readyBad)
			}
			for _, c := range e.app.DynamicChecks(context.Background()) {
				if c.ID == "objstore_reachable" && (c.Passed || strings.Contains(c.Detail, "127.0.0.1")) {
					t.Errorf("objstore_reachable = %+v", c)
				}
			}
			slices.Sort(lat)
			t.Logf("PERF objstore %s: text n=%d p50=%v p99=%v max=%v, complete 503 x%d (dev machine, sequential, not 500/s, reference only)",
				mode, len(lat), lat[len(lat)/2], lat[len(lat)*99/100], lat[len(lat)-1], completes[503])
		})
	}
}

// TC-CHAT-90-05（AC-CHAT-90-5 的 C2 部分）：上传、complete、发送、取地址、下载、后台删除、过期清理，含失败路径（415、4292、对象存储不可用、
// 4048），之后用 deploy/ops/scan-logs 扫 im-server 日志，再扫数据库里除 files 以外的表（含 admin_audit）：
// 搜不到 token、预签名查询串和签名、文件名特征串、对象存储密钥。files.name 本来就存文件名（过期后清空，见 TC-FILE-06-09）。
func TestTC_CHAT_90_05_NoSecretsInLogsC2(t *testing.T) {
	var logs *syncBuffer
	var mod testCaller
	e := newFileEnv(t, func(c *app.Config) {
		logs = captureLogs(c)
		mod = fileModCaller(c)
		c.ObjStore.Timeout = 300 * time.Millisecond
		c.Files.DailyBytes = 64 << 10
	})
	t0 := e.freezeDay()
	tok1, tok2 := e.token("u1"), e.token("u2")
	const marker = "ZXQ-FILE"
	// 成功路径：图片、文件，发送、取地址、下载。
	img := e.mustRequest(tok1, map[string]any{"kind": "image", "name": marker + "-img.png", "size": 1024, "mime": "image/png",
		"thumb": map[string]any{"size": 4096, "mime": "image/jpeg"}})
	e.mustPut(img.Put, fakePNG(10, 10, 1024))
	e.mustPut(*img.ThumbPut, smallThumb())
	e.mustComplete(tok1, img.FileID)
	doc := e.mustRequest(tok1, fileReq(marker+"-doc.pdf", 1024, "application/pdf"))
	e.mustPut(doc.Put, pad([]byte("%PDF-1.4"), 1024))
	e.mustComplete(tok1, doc.FileID)
	c := e.dial("u1", "web")
	ackImg := mustAck[protocol.SendAck](t, sendFile(c, "c2c", "u2", "c2log-000001", "image", img.FileID, nil))
	mustAck[protocol.SendAck](t, sendFile(c, "c2c", "u2", "c2log-000002", "file", doc.FileID, nil))
	urls := []urlItem{e.mustURL(tok2, img.FileID, "original"), e.mustURL(tok2, img.FileID, "thumb"), e.mustURL(tok2, doc.FileID, "original")}
	for _, u := range urls {
		if code, _, _ := download(t, u.URL); code != http.StatusOK {
			t.Fatalf("download = %d", code)
		}
	}
	// 失败路径：415（声明 png，内容是 HTML）、对象存储不可用、4292（当日额度用完）、别人取地址 4048。
	bad := e.mustRequest(tok1, map[string]any{"kind": "image", "name": marker + "-evil.png", "size": 1024, "mime": "image/png",
		"thumb": map[string]any{"size": 4096, "mime": "image/jpeg"}})
	e.mustPut(bad.Put, pad([]byte("<html><script>x</script>"), 1024))
	e.mustPut(*bad.ThumbPut, smallThumb())
	down := e.mustRequest(tok1, fileReq(marker+"-down.txt", 5, "text/plain"))
	e.mustPut(down.Put, []byte("hello"))
	e.s3.SetDown(objstoretest.DownRefuse)
	if code, b := e.complete(tok1, down.FileID); code != 503 {
		t.Fatalf("complete while down: %d %s", code, b)
	}
	e.s3.SetDown(objstoretest.UpNormal)
	if code, b := e.complete(tok1, bad.FileID); code != 415 {
		t.Fatalf("html as png: %d %s", code, b)
	}
	if code, _, b := e.requestUpload(tok1, fileReq(marker+"-big.bin", 60<<10, "application/octet-stream")); code != 429 {
		t.Fatalf("quota: %d %s", code, b)
	}
	e.denied(e.token("u3"), doc.FileID, "original")
	// 后台删除（审计）和过期清理。
	e.adminDelete(mod, ackImg.ConvID, ackImg.MsgID, t0)
	e.setClock(t0.Add(181 * 24 * time.Hour))
	e.runPurge()

	markers := []string{marker, e.s3.SecretKey, tok1, tok2, "X-Amz-Signature", "X-Amz-Credential"}
	for _, p := range []string{img.Put.URL, img.ThumbPut.URL, doc.Put.URL, bad.Put.URL, down.Put.URL, urls[0].URL, urls[1].URL, urls[2].URL} {
		u, err := url.Parse(p)
		if err != nil {
			t.Fatal(err)
		}
		markers = append(markers, u.RawQuery, u.Query().Get("X-Amz-Signature"))
	}
	if logs.String() == "" {
		t.Fatal("no logs captured: the scan would prove nothing")
	}
	dir := t.TempDir()
	logPath := filepath.Join(dir, "im.log")
	if err := os.WriteFile(logPath, []byte(logs.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(dir, "tables.txt")
	if err := os.WriteFile(dbPath, []byte(dumpTablesExcept(t, e.dbPath, "files")), 0o600); err != nil {
		t.Fatal(err)
	}
	args := []string{"-I", "../../deploy/ops/scan-logs"}
	for _, m := range markers {
		if m != "" {
			args = append(args, "--marker="+m)
		}
	}
	out, err := exec.Command("python3", append(args, logPath, dbPath)...).CombinedOutput()
	if err != nil {
		t.Errorf("scan-logs: %v\n%s", err, out)
	}
	if n := e.count(`SELECT COUNT(*) FROM admin_audit`); n == 0 {
		t.Error("no admin audit rows: the audit scan would prove nothing")
	}
}

// dumpTablesExcept 把数据库里除 skip 以外的表逐行转成文本（扫描用）。
func dumpTablesExcept(t *testing.T, path string, skip ...string) string {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro&_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='table'`)
	if err != nil {
		t.Fatal(err)
	}
	var tables []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		if !slices.Contains(skip, n) {
			tables = append(tables, n)
		}
	}
	rows.Close()
	var b strings.Builder
	for _, tb := range tables {
		rs, err := db.Query(`SELECT * FROM "` + tb + `"`)
		if err != nil {
			t.Fatal(err)
		}
		cols, _ := rs.Columns()
		for rs.Next() {
			vals := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rs.Scan(ptrs...); err != nil {
				t.Fatal(err)
			}
			b.WriteString(tb)
			for _, v := range vals {
				if bs, ok := v.([]byte); ok {
					v = string(bs)
				}
				fmt.Fprintf(&b, "\t%v", v)
			}
			b.WriteString("\n")
		}
		rs.Close()
	}
	return b.String()
}

// TC-CHAT-90-16（AC-CHAT-90-17 的 C2 部分）：走上传、取地址、下载、失败路径后，/metrics 有第 14.2 节的四个文件指标，
// 配置检查有 objstore_reachable（连通时通过、不可用时不通过；objstore_https 在 config 包的单测里，生产才适用）；
// 标签没有 userId、fileId、文件名，全部 im_* 指标的标签取值都在枚举内（AC-OPS-90-4）。
func TestTC_CHAT_90_16_FileMetricsAndChecks(t *testing.T) {
	e := newFileEnv(t, func(c *app.Config) { c.ObjStore.Timeout = 300 * time.Millisecond })
	tok1, tok2 := e.token("u1"), e.token("u2")
	const marker = "ZXQ-METRIC"
	id := e.uploadImage(tok1, marker+".png", fakePNG(10, 10, 1024), smallThumb())
	c := e.dial("u1", "web")
	mustAck[protocol.SendAck](t, sendFile(c, "c2c", "u2", "metric-00001", "image", id, nil))
	download(t, e.mustURL(tok2, id, "original").URL)
	bad := e.mustRequest(tok1, fileReq(marker+".pdf", 1024, "application/pdf"))
	e.mustPut(bad.Put, pad([]byte("MZ"), 1024))
	e.complete(tok1, bad.FileID) // 415
	mis := e.mustRequest(tok1, fileReq(marker+"-2.txt", 5, "text/plain"))
	e.s3.Put(e.s3.Private, e.file(mis.FileID).TmpKey, []byte("hello world"), "text/plain")
	e.complete(tok1, mis.FileID) // 4000 uploadMismatch
	checks := func() map[string]bool {
		out := map[string]bool{}
		for _, ck := range e.app.DynamicChecks(context.Background()) {
			out[ck.ID] = ck.Passed
		}
		return out
	}
	if passed, ok := checks()["objstore_reachable"]; !ok || !passed {
		t.Errorf("objstore_reachable while up: %v %v", ok, passed)
	}
	e.s3.SetDown(objstoretest.Down503)
	down := e.mustRequest(tok1, fileReq(marker+"-3.txt", 5, "text/plain"))
	e.complete(tok1, down.FileID) // 5032
	if checks()["objstore_reachable"] {
		t.Error("objstore_reachable passes while the store returns 503")
	}
	e.s3.SetDown(objstoretest.UpNormal)
	m := e.metricsText()
	for _, want := range []string{
		`im_file_uploads_total{kind="image",result="ok"} 1`, `im_file_uploads_total{kind="file",result="type_denied"} 1`,
		`im_file_uploads_total{kind="file",result="mismatch"} 1`, `im_file_uploads_total{kind="file",result="unavailable"} 1`,
		"im_file_bytes_total", "im_file_url_issued_total 1", `im_objstore_errors_total{op="head"}`, `im_objstore_requests_total{op="copy"}`,
	} {
		if !strings.Contains(m, want) {
			t.Errorf("no metric %s", want)
		}
	}
	if !regexp.MustCompile(`(?m)^im_file_bytes_total [1-9]`).MatchString(m) {
		t.Error("im_file_bytes_total is 0")
	}
	for line := range strings.Lines(m) {
		if strings.HasPrefix(line, "im_") && (strings.Contains(line, marker) || strings.Contains(line, id) || strings.Contains(line, `"u1"`) || strings.Contains(line, bad.FileID)) {
			t.Errorf("unbounded label: %s", line)
		}
	}
	// 全部标签在枚举内：解析 /metrics 的标签名和值，和 metrics 包的白名单同一套规则由 internal/metrics 的单测保证；这里只看文件指标的取值。
	labels := regexp.MustCompile(`^(im_file_uploads_total|im_objstore_errors_total|im_objstore_requests_total)\{([^}]*)\}`)
	for line := range strings.Lines(m) {
		mm := labels.FindStringSubmatch(line)
		if mm == nil {
			continue
		}
		for _, kv := range strings.Split(mm[2], ",") {
			k, v, _ := strings.Cut(kv, "=")
			var s string
			_ = json.Unmarshal([]byte(v), &s)
			allowed := map[string][]string{
				"kind":   {"image", "file", "avatar", "unknown"},
				"result": {"ok", "mismatch", "type_denied", "not_ready", "quota", "unavailable", "gone", "error"},
				"op":     {"head", "get", "list", "put", "copy", "delete"},
			}[k]
			if !slices.Contains(allowed, s) {
				t.Errorf("label %s=%s in %s", k, s, mm[1])
			}
		}
	}
}
