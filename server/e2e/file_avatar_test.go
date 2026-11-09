package e2e

// BE-437：头像（F-ACC-06 的 C2 部分，TC-ACC-06-03、06-07～06-13、08-07）。

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"tianting/server/internal/account"
	"tianting/server/internal/app"
	"tianting/server/internal/files"
	"tianting/server/internal/objstore/objstoretest"
	"tianting/server/internal/protocol"
)

func newAvatarEnv(t *testing.T, mutate func(*app.Config)) *fileEnv {
	return newFileEnv(t, func(c *app.Config) {
		c.Account.RegisterMode = account.ModeOpen
		c.ObjStore.Timeout = 300 * time.Millisecond
		if mutate != nil {
			mutate(c)
		}
	})
}

// uploadAvatar 上传一个 kind=avatar（512×512 的 png）并 complete。
func (e *fileEnv) uploadAvatar(token string) string {
	e.t.Helper()
	img := fakePNG(512, 512, 4096)
	r := e.mustRequest(token, map[string]any{"kind": "avatar", "name": "avatar.png", "size": len(img), "mime": "image/png"})
	e.mustPut(r.Put, img)
	e.mustComplete(token, r.FileID)
	return r.FileID
}

func (e *fileEnv) patchProfile(token string, body map[string]any) (int, []byte) {
	e.t.Helper()
	return e.call(http.MethodPatch, "/v1/account/profile", token, body)
}

func (e *fileEnv) avatarURL(uid string) string {
	e.t.Helper()
	var u string
	if err := e.rawDB().QueryRow(`SELECT avatar FROM users WHERE user_id = ?`, uid).Scan(&u); err != nil {
		e.t.Fatal(err)
	}
	return u
}

// TC-ACC-06-03（AC-ACC-06-3）：kind=avatar 的已完成上传成功，users.avatar 是 https 地址（测试环境是公开桶独立域名的 http 代理）；
// kind=file 的上传、别人的上传：404 / 4048。未 complete 的同样 4048。
func TestTC_ACC_06_03_BindAvatar(t *testing.T) {
	e := newAvatarEnv(t, nil)
	u1, u2 := e.register("alice_1", nil), e.register("bobby_1", nil)
	mine := e.uploadAvatar(u1.AccessToken)
	if code, b := e.patchProfile(u1.AccessToken, map[string]any{"avatarFileId": mine}); code != 200 {
		t.Fatalf("bind: %d %s", code, b)
	}
	got := e.avatarURL("alice_1")
	f := e.file(mine)
	if got != e.pub.URL+"/"+f.PublicKey || f.State != "attached" || strings.Contains(got, mine) || strings.Contains(got, "alice") {
		t.Fatalf("users.avatar = %s, file %+v", got, f)
	}
	fileKind := e.uploadFile(u1.AccessToken, "a.png", "image/png", fakePNG(10, 10, 1024))
	others := e.uploadAvatar(u2.AccessToken)
	pending := e.mustRequest(u1.AccessToken, map[string]any{"kind": "avatar", "name": "a.png", "size": 100, "mime": "image/png"}).FileID
	for _, id := range []string{fileKind, others, pending, "doesnotexist0000"} {
		code, b := e.patchProfile(u1.AccessToken, map[string]any{"avatarFileId": id})
		wantErr(t, "avatar "+id, code, b, 404, protocol.CodeFileNotFound, "")
	}
	if e.avatarURL("alice_1") != got {
		t.Fatal("avatar changed by a failed bind")
	}
}

// TC-ACC-06-07（AC-ACC-06-7）：临时键不带签名访问被拒绝；绑定前公开桶没有这个对象；PATCH 后最终地址可读、Content-Type 是图片；
// 域名和消息文件不同；对公开桶列举被拒绝。nosniff 是厂商能力，假 S3 不验（见 BE-438、AC-FILE-90-1）。
func TestTC_ACC_06_07_AvatarExposure(t *testing.T) {
	e := newAvatarEnv(t, nil)
	u := e.register("alice_1", nil)
	img := fakePNG(512, 512, 4096)
	r := e.mustRequest(u.AccessToken, map[string]any{"kind": "avatar", "name": "a.png", "size": len(img), "mime": "image/png"})
	e.mustPut(r.Put, img)
	tmp := e.file(r.FileID).TmpKey
	if code, _, _ := download(t, e.s3.URL()+"/"+e.s3.Private+"/"+tmp); code != http.StatusForbidden {
		t.Fatalf("anonymous tmp key = %d", code)
	}
	e.mustComplete(u.AccessToken, r.FileID)
	if keys := e.s3.Keys(e.s3.Public, ""); len(keys) != 0 {
		t.Fatalf("public bucket before bind: %v", keys)
	}
	if code, b := e.patchProfile(u.AccessToken, map[string]any{"avatarFileId": r.FileID}); code != 200 {
		t.Fatalf("bind: %d %s", code, b)
	}
	code, h, body := download(t, e.avatarURL("alice_1"))
	if code != 200 || h.Get("Content-Type") != "image/png" || string(body) != string(img) {
		t.Fatalf("public avatar = %d %s", code, h.Get("Content-Type"))
	}
	if strings.HasPrefix(e.avatarURL("alice_1"), e.s3.URL()) {
		t.Fatal("avatar uses the same domain as message files")
	}
	if code, _, _ := download(t, e.s3.URL()+"/"+e.s3.Public+"?list-type=2"); code != http.StatusForbidden {
		t.Fatalf("public bucket list = %d", code)
	}
}

// TC-ACC-06-08（AC-ACC-06-8）[IM 钟]：已绑定的头像 25 小时后不被清理；换头像 8 天后旧对象被删。
// TC-ACC-06-11（AC-ACC-06-11）[IM 钟]：相同 avatarFileId 重试成功；已绑定的头像过 181 天还在；F1 → F2 → 5 天内绑回 F1，第 8 天运行清理 F1 不被删除。
func TestTC_ACC_06_08_11_Replace(t *testing.T) {
	e := newAvatarEnv(t, nil)
	clk := e.freeze()
	u := e.register("alice_1", nil)
	f1 := e.uploadAvatar(u.AccessToken)
	e.patchProfile(u.AccessToken, map[string]any{"avatarFileId": f1})
	if code, b := e.patchProfile(u.AccessToken, map[string]any{"avatarFileId": f1}); code != 200 {
		t.Fatalf("retry: %d %s", code, b)
	}
	if keys := e.s3.Keys(e.s3.Public, ""); len(keys) != 1 {
		t.Fatalf("public objects after retry: %v", keys)
	}
	clk.advance(25 * time.Hour)
	e.runPurge()
	if e.fileGone(f1) {
		t.Fatal("bound avatar purged after 25 hours")
	}
	clk.advance(181 * oneDay)
	e.runPurge()
	if e.fileGone(f1) || len(e.s3.Keys(e.s3.Public, "")) != 1 {
		t.Fatal("bound avatar purged after 181 days")
	}
	// 换头像：F1 → F2，8 天后 F1 被删。
	tok := e.mustLogin("alice_1", goodPassword, testPlatform).AccessToken
	f2 := e.uploadAvatar(tok)
	e.patchProfile(tok, map[string]any{"avatarFileId": f2})
	oldURL := e.pub.URL + "/" + e.file(f1).PublicKey
	clk.advance(8 * oneDay)
	e.runPurge()
	if !e.fileGone(f1) || len(e.s3.Keys(e.s3.Private, "final/"+f1+"/")) != 0 {
		t.Fatal("old avatar not purged 8 days after the change")
	}
	if code, _, _ := download(t, oldURL); code != http.StatusNotFound {
		t.Fatalf("old avatar url = %d", code)
	}
	if e.fileGone(f2) {
		t.Fatal("current avatar purged")
	}
	// F2 → F3 → 5 天内绑回 F2：第 8 天 F2 还在。
	tok = e.mustLogin("alice_1", goodPassword, testPlatform).AccessToken
	f3 := e.uploadAvatar(tok)
	e.patchProfile(tok, map[string]any{"avatarFileId": f3})
	clk.advance(5 * oneDay)
	tok = e.mustLogin("alice_1", goodPassword, testPlatform).AccessToken
	if code, b := e.patchProfile(tok, map[string]any{"avatarFileId": f2}); code != 200 {
		t.Fatalf("bind back: %d %s", code, b)
	}
	clk.advance(3 * oneDay)
	e.runPurge()
	if e.fileGone(f2) {
		t.Fatal("re-bound avatar purged")
	}
	if e.avatarURL("alice_1") != e.pub.URL+"/"+e.file(f2).PublicKey {
		t.Fatal("users.avatar is not the re-bound one")
	}
}

// TC-ACC-06-09（AC-ACC-06-9）：对象存储不可用：改昵称成功；改头像 503 / 5032。
func TestTC_ACC_06_09_StoreDown(t *testing.T) {
	e := newAvatarEnv(t, nil)
	u := e.register("alice_1", nil)
	f := e.uploadAvatar(u.AccessToken)
	e.s3.SetDown(objstoretest.DownRefuse)
	if code, b := e.patchProfile(u.AccessToken, map[string]any{"nickname": "小明"}); code != 200 {
		t.Fatalf("nickname: %d %s", code, b)
	}
	code, b := e.patchProfile(u.AccessToken, map[string]any{"avatarFileId": f})
	wantErr(t, "avatar with store down", code, b, 503, protocol.CodeObjectStoreUnavailable, "")
	e.s3.SetDown(objstoretest.UpNormal)
	if code, b := e.patchProfile(u.AccessToken, map[string]any{"avatarFileId": f}); code != 200 {
		t.Fatalf("retry: %d %s", code, b)
	}
}

// TC-ACC-06-10（AC-ACC-06-10）[IM 钟]：complete 后没有 PATCH profile 的头像，推进 25 小时运行清理：对象和记录被删除。
func TestTC_ACC_06_10_UnboundAvatar(t *testing.T) {
	e := newAvatarEnv(t, nil)
	clk := e.freeze()
	u := e.register("alice_1", nil)
	f := e.uploadAvatar(u.AccessToken)
	clk.advance(25 * time.Hour)
	e.runPurge()
	if !e.fileGone(f) || len(e.s3.Keys(e.s3.Private, "")) != 0 {
		t.Fatal("unbound avatar not purged")
	}
}

// TC-ACC-06-12（AC-ACC-06-12）[IM 钟]：Copy 到公开桶后杀进程，重试成功：公开桶只有 1 个对象；再换头像后 8 天、另一用户注销到期：
// 运行清理后公开桶、final/{fileId}/ 前缀和 files 行三处都被清除。TC-ACC-08-07（AC-ACC-08-7）：注销清理后访问旧头像地址，对象不存在。
func TestTC_ACC_06_12_CrashAndCleanup(t *testing.T) {
	e := newAvatarEnv(t, nil)
	clk := e.freeze()
	u := e.register("alice_1", nil)
	v := e.register("bobby_1", nil)
	f1 := e.uploadAvatar(u.AccessToken)
	e.app.Files().Hooks().Crash(files.PointAfterPublicCopy, f1)
	if code, _ := e.patchProfile(u.AccessToken, map[string]any{"avatarFileId": f1}); code != 500 {
		t.Fatalf("crashed bind = %d", code)
	}
	if code, b := e.patchProfile(u.AccessToken, map[string]any{"avatarFileId": f1}); code != 200 {
		t.Fatalf("retry: %d %s", code, b)
	}
	if keys := e.s3.Keys(e.s3.Public, ""); len(keys) != 1 {
		t.Fatalf("public objects after the retry: %v", keys)
	}
	f2 := e.uploadAvatar(u.AccessToken)
	e.patchProfile(u.AccessToken, map[string]any{"avatarFileId": f2})
	g := e.uploadAvatar(v.AccessToken)
	e.patchProfile(v.AccessToken, map[string]any{"avatarFileId": g})
	gURL := e.avatarURL("bobby_1")
	gPub := e.file(g).PublicKey
	e.mustDelete(v.AccessToken, goodPassword)
	clk.advance(8 * oneDay)
	e.cleanup()
	e.runPurge()
	for _, id := range []string{f1, g} {
		if !e.fileGone(id) || len(e.s3.Keys(e.s3.Private, "final/"+id+"/")) != 0 {
			t.Errorf("%s not purged", id)
		}
	}
	if _, ok := e.s3.Get(e.s3.Public, gPub); ok {
		t.Error("deleted user's public avatar still exists")
	}
	if code, _, _ := download(t, gURL); code != http.StatusNotFound {
		t.Errorf("deleted user's avatar url = %d", code)
	}
	if keys := e.s3.Keys(e.s3.Public, ""); len(keys) != 1 || e.fileGone(f2) {
		t.Fatalf("public bucket = %v, current avatar gone=%v", keys, e.fileGone(f2))
	}
}

// TC-ACC-06-13（AC-ACC-06-13）[IM 钟]：Copy 到公开桶后杀进程、不重试，推进 25 小时运行 file_purge：公开桶、私有前缀和 files 行都已清除。
func TestTC_ACC_06_13_CrashNoRetry(t *testing.T) {
	e := newAvatarEnv(t, nil)
	clk := e.freeze()
	u := e.register("alice_1", nil)
	f := e.uploadAvatar(u.AccessToken)
	e.app.Files().Hooks().Crash(files.PointAfterPublicCopy, f)
	e.patchProfile(u.AccessToken, map[string]any{"avatarFileId": f})
	if len(e.s3.Keys(e.s3.Public, "")) != 1 {
		t.Fatal("no public object after the crash")
	}
	clk.advance(25 * time.Hour)
	e.runPurge()
	if !e.fileGone(f) || len(e.s3.Keys(e.s3.Public, "")) != 0 || len(e.s3.Keys(e.s3.Private, "")) != 0 {
		t.Fatalf("left: gone=%v public=%v private=%v", e.fileGone(f), e.s3.Keys(e.s3.Public, ""), e.s3.Keys(e.s3.Private, ""))
	}
}
