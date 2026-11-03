package e2e

// PRD-social F-SOC-09～13（BE-112～BE-115）：搜索与隐私、批量查资料、通讯录、业务方管理好友、官方账号。

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"tianting/server/internal/adminauth"
	"tianting/server/internal/protocol"
)

// ---- F-SOC-09 搜索 ----

// TC-SOC-09-01（AC-SOC-09-1）、TC-SOC-09-02（relation 优先级）、TC-SOC-09-07（AC-SOC-09-6）
func TestTCSOC0901SearchRelations(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob", "carol", "dave", "eve", "frank")
	alice := s.dial("alice", "web")
	rel := func(kw string) *protocol.SearchUser {
		t.Helper()
		return mustAck[protocol.UserSearchAck](t, alice.call(protocol.CmdUserSearch, map[string]any{"keyword": kw})).User
	}
	if u := rel("bob"); u == nil || u.Nickname != "bob" || u.Relation != "none" {
		t.Fatalf("bob = %+v", u)
	}
	alice.apply("bob", nil)
	s.dial("carol", "web").apply("alice", nil)
	s.importFriends([2]string{"alice", "dave"})
	mustAck[socialAck](t, alice.call(protocol.CmdBlock, map[string]any{"userId": "eve"}))
	mustAck[socialAck](t, alice.call(protocol.CmdBlock, map[string]any{"userId": "dave"})) // 拉黑优先于好友
	mustAck[protocol.VerAck](t, alice.call(protocol.CmdSetPrivacy, map[string]any{"searchable": false}))
	for kw, want := range map[string]string{"alice": "self", "bob": "pendingOut", "carol": "pendingIn", "dave": "blocked", "eve": "blocked", "frank": "none"} {
		if u := rel(kw); u == nil || u.Relation != want {
			t.Errorf("%s: %+v, want relation %s", kw, u, want)
		}
	}
	for _, kw := range []string{"a b", strings.Repeat("a", 65)} {
		mustErr(t, alice.call(protocol.CmdUserSearch, map[string]any{"keyword": kw}), protocol.CodeInvalidID)
	}
	if u := rel("  bob  "); u == nil {
		t.Fatal("keyword should be trimmed")
	}
}

// TC-SOC-09-03（AC-SOC-09-2）、TC-SOC-09-04（AC-SOC-09-3）、TC-SOC-09-06（AC-SOC-09-5）
func TestTCSOC0903SearchIndistinguishable(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob", "carol")
	alice, bob, carol := s.dial("alice", "web"), s.dial("bob", "web"), s.dial("carol", "web")
	if u := mustAck[protocol.UserSearchAck](t, alice.call(protocol.CmdUserSearch, map[string]any{"keyword": "bo"})); u.User != nil {
		t.Fatal("prefix match should return null")
	}
	before := string(alice.call(protocol.CmdUserSearch, map[string]any{"keyword": "carol"}).Data)
	mustAck[socialAck](t, carol.call(protocol.CmdBlock, map[string]any{"userId": "alice"}))
	if after := string(alice.call(protocol.CmdUserSearch, map[string]any{"keyword": "carol"}).Data); after != before {
		t.Fatalf("search changed after being blocked: %s vs %s", after, before)
	}
	mustAck[protocol.VerAck](t, bob.call(protocol.CmdSetPrivacy, map[string]any{"searchable": false}))
	a := alice.call(protocol.CmdUserSearch, map[string]any{"keyword": "bob"})
	b := alice.call(protocol.CmdUserSearch, map[string]any{"keyword": "nobody"})
	if string(a.Data) != string(b.Data) || string(a.Data) != `{"user":null}` {
		t.Fatalf("unsearchable %s vs nobody %s", a.Data, b.Data)
	}
}

// TC-SOC-09-05（AC-SOC-09-4）
func TestTCSOC0905SearchLimits(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice")
	alice := s.dial("alice", "web")
	for range 10 {
		mustAck[protocol.UserSearchAck](t, alice.call(protocol.CmdUserSearch, map[string]any{"keyword": "x"}))
	}
	mustErr(t, alice.call(protocol.CmdUserSearch, map[string]any{"keyword": "x"}), protocol.CodeRateLimited)
	s.resetLimits("alice")
	for range 100 {
		s.advance(6 * time.Second)
		mustAck[protocol.UserSearchAck](t, alice.call(protocol.CmdUserSearch, map[string]any{"keyword": "x"}))
	}
	s.advance(6 * time.Second)
	mustErr(t, alice.call(protocol.CmdUserSearch, map[string]any{"keyword": "x"}), protocol.CodeQuotaExceeded)
}

// TC-SOC-09-09（AC-SOC-09-8）
func TestTCSOC0909SearchLogScan(t *testing.T) {
	s, logs := newSocEnvWithLog(t)
	s.usersNamed("alice")
	alice := s.dial("alice", "web")
	mustAck[protocol.UserSearchAck](t, alice.call(protocol.CmdUserSearch, map[string]any{"keyword": "ZXQ-SEARCH-9"}))
	alice.call(protocol.CmdUserSearch, map[string]any{"keyword": "ZXQ-SEARCH-9 x"}) // 4004
	if strings.Contains(logs.String(), "ZXQ-SEARCH-9") {
		t.Fatal("server log contains the search keyword")
	}
	if !strings.Contains(logs.String(), "user search") {
		t.Fatal("expected a debug log line for the search")
	}
}

// ---- F-SOC-10 批量查资料 ----

// TC-SOC-10-01（AC-SOC-10-1）、TC-SOC-10-03（AC-SOC-10-2）
func TestTCSOC1001VisibleUsers(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob", "carol", "dave")
	s.createGroup("g1", "alice", "bob")
	alice := s.dial("alice", "web")
	mustAck[protocol.SendAck](t, s.dial("carol", "web").sendText("c2c", "alice", "m-vis-001", "hi"))
	ids := func() []string {
		u := mustAck[protocol.UsersAck](t, alice.call(protocol.CmdUsers, map[string]any{"userIds": []string{"bob", "carol", "dave", "nobody", "bob"}}))
		var out []string
		for _, x := range u.Users {
			out = append(out, x.UserID)
		}
		return out
	}
	if got := fmt.Sprint(ids()); got != "[bob carol]" {
		t.Fatalf("visible = %s", got)
	}
	if code, b := s.adminDo(http.MethodDelete, "/admin/groups/g1/members", map[string]any{"members": []string{"alice"}}, adminKey); code != http.StatusOK {
		t.Fatalf("remove member: %d %s", code, b)
	}
	if got := fmt.Sprint(ids()); got != "[carol]" {
		t.Fatalf("after leaving g1: %s", got)
	}
}

// TC-SOC-10-04（AC-SOC-10-3）
func TestTCSOC1004UsersParams(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice")
	alice := s.dial("alice", "web")
	var many []string
	for i := range 101 {
		many = append(many, "u"+pad4(i))
	}
	mustErr(t, alice.call(protocol.CmdUsers, map[string]any{"userIds": many}), protocol.CodeBadRequest)
	mustErr(t, alice.call(protocol.CmdUsers, map[string]any{"userIds": []string{}}), protocol.CodeBadRequest)
	mustErr(t, alice.call(protocol.CmdUsers, map[string]any{"userIds": []string{"ok", "a b"}}), protocol.CodeInvalidID)
	for range 30 {
		mustAck[protocol.UsersAck](t, alice.call(protocol.CmdUsers, map[string]any{"userIds": []string{"alice"}}))
	}
	mustErr(t, alice.call(protocol.CmdUsers, map[string]any{"userIds": []string{"alice"}}), protocol.CodeRateLimited)
}

// TC-SOC-10-02、10-05（AC-SOC-10-4）
func TestTCSOC1005RequestVisibility(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "dave")
	s.dial("dave", "web").apply("alice", nil)
	alice := s.dial("alice", "web")
	if u := mustAck[protocol.UsersAck](t, alice.call(protocol.CmdUsers, map[string]any{"userIds": []string{"dave", "alice"}})); len(u.Users) != 2 {
		t.Fatalf("want dave and self: %+v", u.Users)
	}
	s.advance(30*24*time.Hour + time.Second)
	alice = s.dial("alice", "pc")
	if u := mustAck[protocol.UsersAck](t, alice.call(protocol.CmdUsers, map[string]any{"userIds": []string{"dave"}})); len(u.Users) != 0 {
		t.Fatalf("dave still visible after 30 days: %+v", u.Users)
	}
}

// TC-SOC-10-06（AC-SOC-10-5）、TC-SOC-10-07（AC-SOC-10-6）
func TestTCSOC1006NoVisibilityByApplyOrBlock(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "eve", "frank")
	eve := s.dial("eve", "web")
	mustAck[protocol.VerAck](t, eve.call(protocol.CmdSetPrivacy, map[string]any{"searchable": false}))
	alice := s.dial("alice", "web")
	alice.apply("eve", nil)
	a := alice.call(protocol.CmdUsers, map[string]any{"userIds": []string{"eve"}})
	b := alice.call(protocol.CmdUsers, map[string]any{"userIds": []string{"nobody"}})
	if string(a.Data) != string(b.Data) || string(a.Data) != `{"users":[]}` {
		t.Fatalf("eve %s vs nobody %s", a.Data, b.Data)
	}
	mustAck[socialAck](t, eve.respond("alice", "accept", nil))
	if u := mustAck[protocol.UsersAck](t, alice.call(protocol.CmdUsers, map[string]any{"userIds": []string{"eve"}})); len(u.Users) != 1 {
		t.Fatal("eve should be visible after accepting")
	}
	mustAck[socialAck](t, alice.call(protocol.CmdBlock, map[string]any{"userId": "frank"}))
	if u := mustAck[protocol.UsersAck](t, alice.call(protocol.CmdUsers, map[string]any{"userIds": []string{"frank"}})); len(u.Users) != 0 {
		t.Fatal("blocking must not grant visibility")
	}
	all, _ := alice.state()
	if len(all.Blocks) != 1 || all.Blocks[0].Profile != nil {
		t.Fatalf("block item %+v", all.Blocks)
	}
}

// ---- F-SOC-11 通讯录 ----

// TC-SOC-11-01（AC-SOC-11-1）
func TestTCSOC1101MemberCountConsistent(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "u1", "u2", "u3", "u4", "u5", "u6")
	s.createGroup("g1", "alice", "u1", "u2", "u3", "u4")
	alice := s.dial("alice", "web")
	check := func(want int) {
		t.Helper()
		convs := mustAck[protocol.ConvsAck](t, alice.call(protocol.CmdConvs, map[string]any{}))
		groups := mustAck[protocol.MyGroupsAck](t, alice.call(protocol.CmdMyGroups, map[string]any{}))
		if len(convs.Convs) != 1 || *convs.Convs[0].MemberCount != want || len(groups.Groups) != 1 || groups.Groups[0].MemberCount != want {
			t.Fatalf("want %d: convs %+v groups %+v", want, convs.Convs, groups.Groups)
		}
	}
	check(5)
	s.adminPost("/admin/groups/g1/members", map[string]any{"members": []string{"u5", "u6"}}, adminKey)
	check(7)
	s.adminDo(http.MethodDelete, "/admin/groups/g1/members", map[string]any{"members": []string{"u1"}}, adminKey)
	check(6)
}

// TC-SOC-11-02（AC-SOC-11-2）
func TestTCSOC1102MyGroupsBeyondConvsLimit(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice")
	var ms []any
	for i := range 600 {
		ms = append(ms, map[string]any{"groupId": "g" + pad4(i), "userId": "alice"})
	}
	s.seed(map[string]any{"groupMembers": ms})
	alice := s.dial("alice", "web")
	if c := mustAck[protocol.ConvsAck](t, alice.call(protocol.CmdConvs, map[string]any{})); len(c.Convs) != 500 || !c.Truncated {
		t.Fatalf("convs = %d truncated=%v", len(c.Convs), c.Truncated)
	}
	seen := map[string]bool{}
	cursor, pages := "", 0
	for {
		data := map[string]any{"limit": 200}
		if cursor != "" {
			data["cursor"] = cursor
		}
		p := mustAck[protocol.MyGroupsAck](t, alice.call(protocol.CmdMyGroups, data))
		pages++
		for _, g := range p.Groups {
			if seen[g.GroupID] {
				t.Fatalf("duplicate %s", g.GroupID)
			}
			seen[g.GroupID] = true
		}
		if cursor = p.NextCursor; cursor == "" {
			break
		}
	}
	if pages != 3 || len(seen) != 600 {
		t.Fatalf("pages=%d groups=%d", pages, len(seen))
	}
}

// TC-SOC-11-03
func TestTCSOC1103MyGroupsParams(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice")
	alice := s.dial("alice", "web")
	for _, l := range []int{0, 201} {
		mustErr(t, alice.call(protocol.CmdMyGroups, map[string]any{"limit": l}), protocol.CodeBadRequest)
	}
	mustErr(t, alice.call(protocol.CmdMyGroups, map[string]any{"cursor": "a b"}), protocol.CodeBadRequest)
	for range 30 {
		mustAck[protocol.MyGroupsAck](t, alice.call(protocol.CmdMyGroups, map[string]any{}))
	}
	mustErr(t, alice.call(protocol.CmdMyGroups, map[string]any{}), protocol.CodeRateLimited)
}

// ---- F-SOC-12 业务方管理好友 ----

// TC-SOC-12-01（AC-SOC-12-1）、TC-SOC-12-03（AC-SOC-12-2）
func TestTCSOC1201ImportIdempotent(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob")
	alice, bob := s.dial("alice", "web"), s.dial("bob", "web")
	body := map[string]any{"pairs": []any{map[string]any{"a": "alice", "b": "bob", "remarkA": "B"}}}
	if got := string(s.mustAdm(http.MethodPost, "/admin/friends/import", body)); !strings.Contains(got, `"imported":1`) {
		t.Fatalf("import = %s", got)
	}
	alice.waitSocialChanged()
	bob.waitSocialChanged()
	av, bv := alice.ver(), bob.ver()
	for range 2 {
		s.mustAdm(http.MethodPost, "/admin/friends/import", body)
	}
	if alice.ver() != av || bob.ver() != bv {
		t.Fatal("repeated import changed vers")
	}
	all, _ := alice.state()
	if f, _ := findFriend(all.Friends, "bob"); f.Remark != "B" || f.Source != "admin" || len(all.Friends) != 1 {
		t.Fatalf("alice's friends %+v", all.Friends)
	}
	s.mustAdm(http.MethodPost, "/admin/friends/import", map[string]any{"pairs": []any{map[string]any{"a": "alice", "b": "bob", "remarkA": "C"}}})
	if alice.ver() != av+1 || bob.ver() != bv {
		t.Fatalf("remark change: alice %d→%d bob %d→%d", av, alice.ver(), bv, bob.ver())
	}
}

// TC-SOC-12-02：导入不受拉黑影响，也不解除拉黑；同时把待处理申请置为 accepted。
func TestTCSOC1202ImportIgnoresBlock(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob")
	alice, bob := s.dial("alice", "web"), s.dial("bob", "web")
	bob.apply("alice", nil)
	mustAck[socialAck](t, alice.call(protocol.CmdBlock, map[string]any{"userId": "bob"}))
	s.importFriends([2]string{"alice", "bob"})
	all, _ := alice.state()
	if len(all.Friends) != 1 || len(all.Blocks) != 1 {
		t.Fatalf("friends %+v blocks %+v", all.Friends, all.Blocks)
	}
	bAll, _ := bob.state()
	if r, _ := findReq(bAll.Requests, "alice", "out"); r.Status != "accepted" {
		t.Fatalf("bob's request %+v", r)
	}
}

// TC-SOC-12-04（AC-SOC-12-3）、TC-SOC-12-05（AC-SOC-12-4）
func TestTCSOC1204ImportErrors(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob", "carol")
	var big []any
	for i := range 501 {
		big = append(big, map[string]any{"a": "alice", "b": "x" + pad4(i)})
	}
	code, body := s.adm(http.MethodPost, "/admin/friends/import", map[string]any{"pairs": big})
	assertAdminErr(t, code, body, http.StatusBadRequest, protocol.CodeBatchLimitExceeded)
	code, body = s.adm(http.MethodPost, "/admin/friends/import", map[string]any{"pairs": []any{map[string]any{"a": "alice", "b": "bob"}, map[string]any{"a": "alice", "b": "nobody"}}})
	assertAdminErr(t, code, body, http.StatusNotFound, protocol.CodeUserNotFound)
	code, body = s.adm(http.MethodPost, "/admin/friends/import", map[string]any{"pairs": []any{map[string]any{"a": "alice", "b": "alice"}}})
	assertAdminErr(t, code, body, http.StatusBadRequest, protocol.CodeBadRequest)
	code, body = s.adm(http.MethodPost, "/admin/friends/import", map[string]any{"pairs": []any{map[string]any{"a": "a b", "b": "alice"}}})
	assertAdminErr(t, code, body, http.StatusBadRequest, protocol.CodeInvalidID)
	s.seedFriends("alice", "f", 1999)
	code, body = s.adm(http.MethodPost, "/admin/friends/import", map[string]any{"pairs": []any{map[string]any{"a": "alice", "b": "bob"}, map[string]any{"a": "carol", "b": "alice"}}})
	assertAdminErr(t, code, body, http.StatusBadRequest, protocol.CodeFriendLimitExceeded)
	if !strings.Contains(string(body), "alice") {
		t.Fatalf("msg should name alice: %s", body)
	}
	bob := s.dial("bob", "web")
	if all, _ := bob.state(); len(all.Friends) != 0 {
		t.Fatalf("partial import: %+v", all.Friends)
	}
}

// TC-SOC-12-06（AC-SOC-12-5）
func TestTCSOC1206AdminRemove(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob")
	s.importFriends([2]string{"alice", "bob"})
	s.settings(map[string]any{"mode": "friends"})
	s.mustAdm(http.MethodDelete, "/admin/friends", map[string]any{"pairs": []any{map[string]any{"a": "bob", "b": "alice"}}})
	alice, bob := s.dial("alice", "web"), s.dial("bob", "web")
	for _, c := range []*client{alice, bob} {
		if all, _ := c.state(); len(all.Friends) != 0 {
			t.Fatalf("%s still has friends %+v", c.uid, all.Friends)
		}
	}
	mustErr(t, alice.sendText("c2c", "bob", "m-adel-001", "hi"), protocol.CodeNotFriend)
	mustErr(t, bob.sendText("c2c", "alice", "m-adel-002", "hi"), protocol.CodeNotFriend)
}

// TC-SOC-12-07（AC-SOC-12-6）
func TestTCSOC1207ListFriends(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice")
	s.seedFriends("alice", "f", 250)
	seen, cursor, pages := map[string]bool{}, "", 0
	for {
		path := "/admin/users/alice/friends?limit=100"
		if cursor != "" {
			path += "&cursor=" + cursor
		}
		raw := s.mustAdm(http.MethodGet, path, nil)
		if strings.Contains(string(raw), "remark") {
			t.Fatalf("friend list contains remark: %s", raw)
		}
		var p struct {
			Friends    []struct{ UserID string }
			NextCursor string
		}
		json.Unmarshal(raw, &p)
		pages++
		for _, f := range p.Friends {
			if seen[f.UserID] {
				t.Fatalf("duplicate %s", f.UserID)
			}
			seen[f.UserID] = true
		}
		if cursor = p.NextCursor; cursor == "" {
			break
		}
	}
	if pages != 3 || len(seen) != 250 {
		t.Fatalf("pages=%d friends=%d", pages, len(seen))
	}
}

// TC-SOC-12-08（AC-SOC-12-7）
func TestTCSOC1208OneVerPerBatch(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob", "carol", "dave")
	alice := s.dial("alice", "web")
	v := alice.ver()
	s.importFriends([2]string{"alice", "bob"}, [2]string{"carol", "alice"}, [2]string{"alice", "dave"})
	if alice.ver() != v+1 {
		t.Fatalf("alice ver %d → %d, want +1", v, alice.ver())
	}
}

// TC-SOC-12-09（AC-SOC-12-8）
func TestTCSOC1209AdminAuth(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "bob")
	body := map[string]any{"pairs": []any{map[string]any{"a": "alice", "b": "bob"}}}
	code, b := s.adminPost("/admin/friends/import", body, "")
	assertAdminErr(t, code, b, http.StatusUnauthorized, protocol.CodeAdminKeyInvalid)
	code, b = s.adminPost("/admin/friends/import", body, "wrong")
	assertAdminErr(t, code, b, http.StatusUnauthorized, protocol.CodeAdminKeyInvalid)
	code, b = s.adminPost("/admin/friends/import", body, adminKey) // 旧密钥不能访问新接口
	assertAdminErr(t, code, b, http.StatusForbidden, protocol.CodeForbidden)
	reader := hmacCaller("reader", adminauth.ScopeUserRead)
	s2 := newEnvWithCallers(t, reader)
	s2.users("alice", "bob")
	code, b = s2.signed(reader, http.MethodPost, "/admin/friends/import", body, signOpts{})
	assertAdminErr(t, code, b, http.StatusForbidden, protocol.CodeForbidden)
}

// TC-SOC-12-10（AC-SOC-12-9）
func TestTCSOC1210AuditWithoutRemark(t *testing.T) {
	s, logs := newSocEnvWithLog(t)
	s.usersNamed("alice", "bob")
	s.settings(map[string]any{"strangerMsgLimit": 3})
	s.mustAdm(http.MethodPost, "/admin/friends/import", map[string]any{"pairs": []any{map[string]any{"a": "alice", "b": "bob", "remarkA": "ZXQ-REMARK-789"}}})
	s.mustAdm(http.MethodDelete, "/admin/friends", map[string]any{"pairs": []any{map[string]any{"a": "alice", "b": "bob"}}})
	raw := s.mustAdm(http.MethodGet, fmt.Sprintf("/admin/audit?from=%d&to=%d", s.now-60000, s.now+1000), nil)
	for _, route := range []string{"PATCH /admin/settings/social", "POST /admin/friends/import", "DELETE /admin/friends"} {
		if !strings.Contains(string(raw), route) {
			t.Errorf("audit has no %s: %s", route, raw)
		}
	}
	if !strings.Contains(string(raw), `\"alice\"`) && !strings.Contains(string(raw), `"alice"`) {
		t.Errorf("audit has no target userId: %s", raw)
	}
	if strings.Contains(string(raw), "ZXQ-REMARK-789") || strings.Contains(logs.String(), "ZXQ-REMARK-789") {
		t.Fatal("remark leaked into audit or logs")
	}
}

// TC-SOC-12-11（AC-SOC-12-10，X-5 已落地后的等价断言）：社交管理接口只在管理端口，对外端口 404。
func TestTCSOC1211NotOnPublicPort(t *testing.T) {
	s := newSocEnv(t)
	for _, p := range []string{"/admin/settings/social", "/admin/friends/import", "/admin/users/alice/official"} {
		code, _ := s.do(s.srv.URL, http.MethodGet, p, nil, adminKey)
		if code != http.StatusNotFound {
			t.Errorf("public %s = %d, want 404", p, code)
		}
	}
}

// ---- F-SOC-13 官方账号 ----

func (s *socEnv) official(uid string, on bool) {
	s.t.Helper()
	s.mustAdm(http.MethodPut, "/admin/users/"+uid+"/official", map[string]any{"official": on})
}

// TC-SOC-13-01（AC-SOC-13-1）、TC-SOC-13-04（AC-SOC-13-4）
func TestTCSOC1301OfficialFriendsMode(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "kefu")
	s.settings(map[string]any{"mode": "friends"})
	s.official("kefu", true)
	kefu, alice := s.dial("kefu", "web"), s.dial("alice", "web")
	mustAck[protocol.SendAck](t, kefu.sendText("c2c", "alice", "m-off-001", "您好"))
	mustAck[protocol.SendAck](t, alice.sendText("c2c", "kefu", "m-off-002", "你好"))
	s.official("kefu", false)
	mustErr(t, kefu.sendText("c2c", "alice", "m-off-003", "hi"), protocol.CodeNotFriend)
}

// TC-SOC-13-02（AC-SOC-13-2）
func TestTCSOC1302OfficialExemptions(t *testing.T) {
	s := newSocEnvFastSend(t)
	var peers []string
	for i := range 50 {
		peers = append(peers, "u"+pad4(i))
	}
	s.usersNamed(append([]string{"alice", "kefu"}, peers...)...)
	s.settings(map[string]any{"strangerMsgLimit": 3, "newPeerHourlyLimit": 30})
	s.official("kefu", true)
	kefu := s.dial("kefu", "web")
	for i := range 10 {
		mustAck[protocol.SendAck](t, kefu.sendText("c2c", "alice", cid("m-ofx", i), "hi"))
	}
	for i, p := range peers {
		mustAck[protocol.SendAck](t, kefu.sendText("c2c", p, cid("m-ofp", i), "hi"))
	}
}

// TC-SOC-13-03（AC-SOC-13-3）、TC-SOC-13-08（AC-SOC-13-8）
func TestTCSOC1303OfficialNotExempt(t *testing.T) {
	s := newSocEnvFastSend(t)
	s.usersNamed("alice", "bob", "kefu")
	s.official("kefu", true)
	alice, kefu := s.dial("alice", "web"), s.dial("kefu", "web")
	mustAck[socialAck](t, alice.call(protocol.CmdBlock, map[string]any{"userId": "kefu"}))
	mustErr(t, kefu.sendText("c2c", "alice", "m-ofb-001", "hi"), protocol.CodeRejectedByPeer)
	for i := range 99 { // 上面那次 4036 已经扣了 1 个
		mustErr(t, kefu.sendText("c2c", "ghost"+pad4(i), cid("m-ofg", i), "hi"), protocol.CodeUserNotFound)
	}
	mustErr(t, kefu.sendText("c2c", "bob", "m-ofb-bob", "hi"), protocol.CodeQuotaExceeded)
}

// TC-SOC-13-05（AC-SOC-13-5）、TC-SOC-13-06（AC-SOC-13-6）
func TestTCSOC1305OfficialProfileAndAudit(t *testing.T) {
	s := newSocEnv(t)
	s.usersNamed("alice", "kefu")
	code, body := s.adm(http.MethodPut, "/admin/users/nobody/official", map[string]any{"official": true})
	assertAdminErr(t, code, body, http.StatusNotFound, protocol.CodeUserNotFound)
	s.official("kefu", true)
	alice, kefu := s.dial("alice", "web"), s.dial("kefu", "web")
	if u := mustAck[protocol.UserSearchAck](t, alice.call(protocol.CmdUserSearch, map[string]any{"keyword": "kefu"})).User; u == nil || !u.Official {
		t.Fatalf("search kefu = %+v", u)
	}
	mustAck[protocol.SendAck](t, kefu.sendText("c2c", "alice", "m-ofa-001", "hi"))
	if u := mustAck[protocol.UsersAck](t, alice.call(protocol.CmdUsers, map[string]any{"userIds": []string{"kefu"}})); len(u.Users) != 1 || !u.Users[0].Official {
		t.Fatalf("users kefu = %+v", u)
	}
	if _, last := kefu.state(); !last.Official {
		t.Fatal("kefu's sync header should have official=true")
	}
	if got := string(s.mustAdm(http.MethodGet, "/admin/users/kefu", nil)); !strings.Contains(got, `"official":true`) {
		t.Fatalf("GET user = %s", got)
	}
	s.official("kefu", false)
	raw := string(s.mustAdm(http.MethodGet, fmt.Sprintf("/admin/audit?from=%d&to=%d", s.now-60000, s.now+1000), nil))
	if strings.Count(raw, "PUT /admin/users/{userId}/official") < 3 { // 失败 1 次 + 成功 2 次，各有 intent / result
		t.Fatalf("audit = %s", raw)
	}
}
