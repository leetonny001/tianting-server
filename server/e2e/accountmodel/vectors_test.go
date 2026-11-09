package accountmodel

// protocol/vectors/account-session.json（PRD-chat F-ACC-03，BE-402）：步骤式，写法同 push-sync.json；期望结果由本参考客户端生成：
//
//	go test ./e2e/accountmodel -run TestAccountSessionVectors -update
//
// 普通运行时校验文件和参考客户端的输出一致；三套 SDK 跑同一份文件（FE-600～602）。

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"testing"
)

var update = flag.Bool("update", false, "rewrite protocol/vectors/account-session.json from the reference client")

const vectorsPath = "../../../protocol/vectors/account-session.json"

type step struct {
	Op          string   `json:"op"` // trigger | http4011 | requestTimeout | refreshOk | refreshFailed
	Source      string   `json:"source,omitempty"`
	NowMs       int64    `json:"nowMs,omitempty"`
	RejectedGen int64    `json:"rejectedGen,omitempty"`
	Attempt     int64    `json:"attempt,omitempty"`
	Tokens      *Tokens  `json:"tokens,omitempty"`
	Failure     *Failure `json:"failure,omitempty"`
	Expect      Outcome  `json:"expect"`
	LocalGen    int64    `json:"localGen"` // 这一步之后本地 token 的代号
	Inflight    bool     `json:"inflight"`
}

type start struct {
	Mode              string `json:"mode"`
	AccessGen         int64  `json:"accessGen"`
	AccessExpiresAt   int64  `json:"accessExpiresAt"`
	AbsoluteExpiresAt int64  `json:"absoluteExpiresAt"`
}

type vcase struct {
	Name  string `json:"name"`
	Rule  string `json:"rule"`
	Start start  `json:"start"`
	Steps []step `json:"steps"`
}

type file struct {
	Description string       `json:"description"`
	Scope       string       `json:"scope"`
	Cases       []vcase      `json:"cases"`
	ServerCases []ServerCase `json:"serverCases"`
	// 附件发送状态机（PRD-chat F-FILE-07 规则 1、3、4，BE-403）。
	AttachmentFormat map[string]string `json:"attachmentFormat"`
	AttachmentCases  []attachCase      `json:"attachmentCases"`
}

func rf(gen int64) ServerStep         { return ServerStep{Op: "refresh", Gen: gen} }
func ac(gen int64) ServerStep         { return ServerStep{Op: "access", Gen: gen} }
func adv(ms int64) ServerStep         { return ServerStep{Op: "advance", Ms: ms} }
func rfN(gen int64, n int) ServerStep { return ServerStep{Op: "refresh", Gen: gen, Repeat: n} }

const minute = int64(60 * 1000)

var serverCases = []ServerCase{
	{Name: "access 等于 current 放行", Rule: "规则 4 判定表", Steps: []ServerStep{ac(1)}},
	{Name: "access 属于 pending 转正放行，旧 current 随即被拒绝", Rule: "规则 4 判定表", Steps: []ServerStep{rf(1), ac(2), ac(1)}},
	{Name: "被替换的 pending：不转正，不吊销", Rule: "规则 4 判定表，AC-ACC-03-21", Steps: []ServerStep{rf(1), rf(1), ac(3), ac(2)}},
	{Name: "被淘汰的 pending 的 access：不转正，R1 仍可用", Rule: "规则 4 判定表，AC-ACC-03-28", Steps: []ServerStep{rf(1), rfN(1, 4), ac(2), rf(1)}},
	{Name: "用 current 刷新：current 不变，加入 pending", Rule: "规则 4", Steps: []ServerStep{rf(1), rf(1), rf(1)}},
	{Name: "用 pending 刷新：转正并签发新的 pending", Rule: "规则 4", Steps: []ServerStep{rf(1), rf(2)}},
	{Name: "转正后窗口内旧值 superseded，窗口后重放", Rule: "规则 4，AC-ACC-03-30", Steps: []ServerStep{rf(1), ac(2), adv(minute), rf(1), adv(2 * minute), rf(1), rf(2)}},
	{Name: "窗口内被替换的 pending 也 superseded", Rule: "规则 4，AC-ACC-03-31", Steps: []ServerStep{rf(1), rf(1), ac(2), rf(3), adv(3 * minute), rf(3)}},
	{Name: "连续转正：更早的值立即重放", Rule: "规则 4，RS5-01，AC-ACC-03-38", Steps: []ServerStep{rf(1), ac(2), adv(110 * 1000), rf(2), ac(3), adv(5 * 1000), rf(1), rf(3)}},
	{Name: "代号不复用：作废的代一直被拒绝", Rule: "规则 1，RS5-05，AC-ACC-03-40", Steps: []ServerStep{rf(1), rf(1), rf(1), ac(4), rf(4), ac(5), ac(3)}},
	{Name: "没有转正过时，被淘汰的值按重放", Rule: "规则 4", Steps: []ServerStep{rf(1), rfN(1, 4), rf(2), ac(1)}},
	{Name: "用满签发次数：current 4290；旧值仍直接重放", Rule: "规则 4 第 ⑥ 步，RS6-05，AC-ACC-03-48", Steps: []ServerStep{rf(1), ac(2), rfN(2, 19), rf(2), adv(3 * minute), rf(1), rf(2)}},
}

func runServer(c ServerCase) ServerCase {
	m := NewServerSession()
	m.Now = now
	out := c
	out.Steps = make([]ServerStep, len(c.Steps))
	for i, st := range c.Steps {
		switch st.Op {
		case "advance":
			m.Now += st.Ms
		case "refresh":
			for range st.Repeat {
				m.Refresh(st.Gen)
			}
			if st.Repeat == 0 {
				st.Result, st.NewGen = m.Refresh(st.Gen)
			}
		case "access":
			st.Result = m.Access(st.Gen)
		default:
			panic("unknown op " + st.Op)
		}
		st.Current, st.Revoked, st.MinGen, st.InWindow = m.Current, m.Revoked, m.OverlapMinGen, m.Now < m.OverlapUntil
		st.Pendings = append([]int64{}, m.Pendings...)
		out.Steps[i] = st
	}
	return out
}

const (
	now  = int64(1800000000000)
	hour = int64(3600 * 1000)
	day  = 24 * hour
)

func acct(accessLeft int64) start {
	return start{Mode: "account", AccessGen: 1, AccessExpiresAt: now + accessLeft, AbsoluteExpiresAt: now + 300*day}
}

func ok(a, gen int64) step {
	return step{Op: "refreshOk", Attempt: a, Tokens: &Tokens{AccessGen: gen, AccessExpiresAt: now + day, AbsoluteExpiresAt: now + 300*day}}
}

func fail(a int64, f Failure) step { return step{Op: "refreshFailed", Attempt: a, Failure: &f} }

func trig(source string) step { return step{Op: "trigger", Source: source, NowMs: now} }

var cases = []vcase{
	{Name: "剩余不足 6 小时才刷新", Rule: "规则 5", Start: acct(5 * hour), Steps: []step{trig("timer"), ok(1, 2)}},
	{Name: "剩余超过 6 小时不刷新", Rule: "规则 5", Start: acct(7 * hour), Steps: []step{trig("start"), trig("foreground"), trig("timer")}},
	{Name: "四个入口合并成一次", Rule: "规则 5 单飞", Start: acct(hour), Steps: []step{trig("start"), trig("foreground"), trig("timer"), step{Op: "trigger", Source: "kicked", NowMs: now, RejectedGen: 1}, ok(1, 2)}},
	{Name: "超时放弃后重试，迟到响应丢弃", Rule: "规则 5", Start: acct(hour), Steps: []step{trig("timer"), {Op: "requestTimeout"}, trig("timer"), ok(1, 2), ok(2, 3)}},
	{Name: "迟到的失败响应也丢弃", Rule: "规则 5", Start: acct(hour), Steps: []step{trig("timer"), {Op: "requestTimeout"}, trig("timer"), fail(1, Failure{Code: 4019, Reason: "revoked"}), ok(2, 2)}},
	{Name: "access 到期等于绝对上限时不再主动刷新", Rule: "规则 5、RS5-11", Start: start{Mode: "account", AccessGen: 1, AccessExpiresAt: now + hour, AbsoluteExpiresAt: now + hour}, Steps: []step{trig("timer"), trig("foreground")}},
	{Name: "superseded 留在当前页，3 次后回登录页", Rule: "规则 6", Start: acct(hour), Steps: []step{trig("timer"), fail(1, Failure{Code: 4019, Reason: "superseded", RetryAfterSeconds: 120}), trig("timer"), fail(2, Failure{Code: 4019, Reason: "superseded", RetryAfterSeconds: 120}), trig("timer"), fail(3, Failure{Code: 4019, Reason: "superseded", RetryAfterSeconds: 120})}},
	{Name: "superseded 之后刷新成功清零", Rule: "规则 6", Start: acct(hour), Steps: []step{trig("timer"), fail(1, Failure{Code: 4019, Reason: "superseded", RetryAfterSeconds: 120}), trig("timer"), ok(2, 2), trig("timer")}},
	{Name: "replayDetected 回登录页前弹窗", Rule: "规则 6", Start: acct(hour), Steps: []step{trig("timer"), fail(1, Failure{Code: 4019, Reason: "replayDetected"})}},
	{Name: "credentialsChanged 带截止日期", Rule: "规则 6a", Start: acct(hour), Steps: []step{trig("timer"), fail(1, Failure{Code: 4019, Reason: "credentialsChanged", RecoverBefore: now + 6*day})}},
	{Name: "credentialsChanged 没有截止日期按原因提示", Rule: "规则 6a", Start: acct(hour), Steps: []step{trig("timer"), fail(1, Failure{Code: 4019, Reason: "credentialsChanged"})}},
	{Name: "revoked、expired、invalid 回登录页", Rule: "规则 6", Start: acct(hour), Steps: []step{trig("timer"), fail(1, Failure{Code: 4019, Reason: "expired"})}},
	{Name: "不认识的 reason 按回登录页处理", Rule: "规则 6", Start: acct(hour), Steps: []step{trig("timer"), fail(1, Failure{Code: 4019, Reason: "somethingNew"})}},
	{Name: "429、5xx、网络错误留在当前页面", Rule: "规则 6", Start: acct(hour), Steps: []step{trig("timer"), fail(1, Failure{Code: 4290}), trig("timer"), fail(2, Failure{Code: 5030}), trig("timer"), fail(3, Failure{Code: 0}), trig("timer"), ok(4, 2)}},
	{Name: "4034 是兜底，显示封禁文案", Rule: "规则 6、10", Start: acct(hour), Steps: []step{trig("timer"), fail(1, Failure{Code: 4034})}},
	{Name: "kicked/tokenExpired：本地代号更新就不刷新", Rule: "规则 7", Start: start{Mode: "account", AccessGen: 5, AccessExpiresAt: now + day, AbsoluteExpiresAt: now + 300*day}, Steps: []step{{Op: "trigger", Source: "kicked", NowMs: now, RejectedGen: 4}}},
	{Name: "kicked/tokenExpired：本地不比它新就刷新一次，不看剩余时间", Rule: "规则 7", Start: acct(day), Steps: []step{{Op: "trigger", Source: "kicked", NowMs: now, RejectedGen: 1}, ok(1, 2)}},
	{Name: "HTTP 4011：本地代号更新就直接重试", Rule: "规则 7、RS5-10", Start: start{Mode: "account", AccessGen: 5, AccessExpiresAt: now + day, AbsoluteExpiresAt: now + 300*day}, Steps: []step{{Op: "http4011", RejectedGen: 4}, {Op: "http4011", RejectedGen: 4}, {Op: "http4011", RejectedGen: 3}}},
	{Name: "HTTP 4011：本地不比它新就刷新，并发合并", Rule: "规则 7", Start: acct(day), Steps: []step{{Op: "http4011", RejectedGen: 1}, {Op: "http4011", RejectedGen: 1}, ok(1, 2), {Op: "http4011", RejectedGen: 1}}},
	{Name: "演示模式收到 4011 回登录页", Rule: "规则 7", Start: start{Mode: "demo", AccessGen: 0, AccessExpiresAt: now + hour}, Steps: []step{trig("timer"), {Op: "trigger", Source: "kicked", NowMs: now}}},
}

func run(c vcase) vcase {
	s := &Session{Mode: c.Start.Mode, AccessGen: c.Start.AccessGen, AccessExpiresAt: c.Start.AccessExpiresAt, AbsoluteExpiresAt: c.Start.AbsoluteExpiresAt}
	out := c
	out.Steps = make([]step, len(c.Steps))
	for i, st := range c.Steps {
		switch st.Op {
		case "trigger":
			st.Expect = s.Trigger(st.Source, st.NowMs, st.RejectedGen)
		case "http4011":
			st.Expect = s.HTTP4011(st.RejectedGen)
		case "requestTimeout":
			st.Expect = s.RequestTimeout()
		case "refreshOk":
			st.Expect = s.RefreshOK(st.Attempt, *st.Tokens)
		case "refreshFailed":
			st.Expect = s.RefreshFailed(st.Attempt, *st.Failure)
		default:
			panic("unknown op " + st.Op)
		}
		st.LocalGen, st.Inflight = s.AccessGen, s.Inflight
		out.Steps[i] = st
	}
	return out
}

// TC-CHAT-90-13（AC-CHAT-90-15，服务端产出期望值的部分）；TC-CHAT-90-09（AC-CHAT-90-10）。
func TestAccountSessionVectors(t *testing.T) {
	f := file{
		Description:      "账号会话共享行为用例（PRD-chat F-ACC-03 规则 4、5、6、6a、7）。cases：按顺序执行 steps，每步之后断言 expect、localGen、inflight。serverCases：规则 4 的判定表，从登录后的状态开始，每步之后断言 result、current、pendings。attachmentCases：附件发送状态机（F-FILE-07 规则 1、3、4），每步之后断言 expect，格式见 attachmentFormat。期望结果由服务端的 Go 参考客户端（server/e2e/accountmodel）生成。",
		Scope:            "cases 是客户端判断：刷新单飞、迟到响应丢弃、失败分类、4011 比较代号，三套 SDK 跑。serverCases 由真实服务端跑（server/e2e/account_vectors_test.go），三套 SDK 用它搭假服务端。attachmentCases 是附件发送状态机（PRD-chat F-FILE-07 规则 1、3、4），三套 SDK 用假 HTTP、假对象存储、假传输层跑真实的客户端；persistence=true 的只有带持久化的 SDK 跑（sdk-ts 跳过，H5 刷新后消息不在，AC-FILE-07-3）。",
		AttachmentFormat: attachmentFormat,
	}
	for _, c := range cases {
		f.Cases = append(f.Cases, run(c))
	}
	for _, c := range serverCases {
		f.ServerCases = append(f.ServerCases, runServer(c))
	}
	for _, c := range attachCases {
		f.AttachmentCases = append(f.AttachmentCases, runAttach(c))
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(f); err != nil {
		t.Fatal(err)
	}
	if *update {
		if err := os.WriteFile(vectorsPath, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(vectorsPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, buf.Bytes()) {
		t.Fatalf("%s differs from the reference client; run with -update and review the diff", vectorsPath)
	}
}

// 关键断言不依赖生成的文件，防止 -update 把错误的行为写进去。
func TestAccountModelRules(t *testing.T) {
	last := func(c vcase) step { r := run(c); return r.Steps[len(r.Steps)-1] }
	if s := last(cases[6]); s.Expect.Action != ActionLogin || s.Expect.Dialog != DialogSupersededMax {
		t.Errorf("third superseded: %+v", s)
	}
	r := run(cases[3])
	if r.Steps[3].Expect.Action != ActionNone || r.Steps[3].LocalGen != 1 || r.Steps[4].LocalGen != 3 {
		t.Errorf("late response must be dropped: %+v", r.Steps)
	}
	for _, i := range []int{1, 2, 3, 5} { // 限流、服务端错误、网络错误不能回登录页
		if a := run(cases[13]).Steps[i].Expect.Action; a == ActionLogin {
			t.Errorf("step %d must not go to login", i)
		}
	}
	if s := last(cases[9]); s.Expect.Dialog != DialogRecoverBefore || s.Expect.RecoverBefore == 0 {
		t.Errorf("recoverBefore: %+v", s)
	}
	if s := last(cases[15]); s.Expect.Action != ActionUseLocal {
		t.Errorf("newer local token must not refresh: %+v", s)
	}
	// 规则 4 的关键结论。
	find := func(name string) ServerCase {
		for _, c := range serverCases {
			if c.Name == name {
				return runServer(c)
			}
		}
		t.Fatalf("no case %q", name)
		return ServerCase{}
	}
	if s := find("连续转正：更早的值立即重放").Steps[6]; s.Result != ResultReplay || !s.Revoked {
		t.Errorf("consecutive promotion: %+v", s)
	}
	limited := find("用满签发次数：current 4290；旧值仍直接重放")
	if limited.Steps[3].Result != ResultLimited || limited.Steps[5].Result != ResultReplay {
		t.Errorf("rate limit: %+v", limited.Steps)
	}
	if s := find("被淘汰的 pending 的 access：不转正，R1 仍可用").Steps[2]; s.Result != AccessRejected || s.Current != 1 {
		t.Errorf("evicted access: %+v", s)
	}
}
