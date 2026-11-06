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
	Description string  `json:"description"`
	Scope       string  `json:"scope"`
	Cases       []vcase `json:"cases"`
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
		Description: "账号会话共享行为用例（PRD-chat F-ACC-03 规则 5、6、6a、7）。按顺序执行 steps，每步之后断言 expect、localGen、inflight。期望结果由服务端的 Go 参考客户端（server/e2e/accountmodel）生成。",
		Scope:       "本批只有客户端判断：刷新单飞、迟到响应丢弃、失败分类、4011 比较代号。规则 4 的判定表和转正随 BE-415 追加，附件状态机随 BE-403。",
	}
	for _, c := range cases {
		f.Cases = append(f.Cases, run(c))
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
}
