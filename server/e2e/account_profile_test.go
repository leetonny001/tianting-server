package e2e

// F-ACC-06（C1 部分）和 F-ACC-07（BE-419）。

import (
	"net/http"
	"strings"
	"testing"

	"tianting/server/internal/protocol"
)

func (e *acctEnv) patchProfile(bearer string, body map[string]any) (int, []byte) {
	e.t.Helper()
	return e.bearerDo(http.MethodPatch, "/v1/account/profile", bearer, e.freshIP(), body)
}

// TC-ACC-06-01（AC-ACC-06-1）：u1 改昵称，u2 用 users 查到新昵称。
func TestTC_ACC_06_01_Nickname(t *testing.T) {
	e := openEnv(t, nil)
	u1 := e.register("alice_1", nil)
	u2 := e.register("bobby_1", nil)
	e.createGroup("g_share", "alice_1", "bobby_1") // users 只返回有关系的人（F-SOC-10）
	if code, b := e.patchProfile(u1.AccessToken, map[string]any{"nickname": "小明"}); code != http.StatusOK {
		t.Fatalf("%d %s", code, b)
	}
	c := e.authOK("u2", u2.AccessToken, testPlatform)
	f := c.call(protocol.CmdUsers, map[string]any{"userIds": []string{"alice_1"}})
	if f.Error != nil || !strings.Contains(string(f.Data), "小明") {
		t.Errorf("%+v %s", f.Error, f.Data)
	}
}

// TC-ACC-06-02（AC-ACC-06-2）：33 个码点、空白、没有字段都是 400；C1 带 avatarFileId 404 / 4048。
func TestTC_ACC_06_02_BadNickname(t *testing.T) {
	e := openEnv(t, nil)
	u := e.register("alice_1", nil)
	for label, body := range map[string]map[string]any{
		"33": {"nickname": strings.Repeat("字", 33)}, "blank": {"nickname": "  \t "}, "none": {},
	} {
		code, b := e.patchProfile(u.AccessToken, body)
		wantErr(t, label, code, b, http.StatusBadRequest, protocol.CodeBadRequest, "")
	}
	if code, b := e.patchProfile(u.AccessToken, map[string]any{"nickname": strings.Repeat("字", 32)}); code != http.StatusOK {
		t.Errorf("32: %d %s", code, b)
	}
	code, b := e.patchProfile(u.AccessToken, map[string]any{"avatarFileId": "f1"})
	wantErr(t, "avatar", code, b, http.StatusNotFound, protocol.CodeFileNotFound, "")
}
