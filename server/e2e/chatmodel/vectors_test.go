package chatmodel

// protocol/vectors/chat-sync.json（PRD-chat F-RCL-03 规则 2、2a，第 15 节表 C2 的"合并和过期"，BE-403）：步骤式，写法同
// account-session.json；期望结果由本参考客户端生成：
//
//	go test ./e2e/chatmodel -run TestChatSyncVectors -update
//
// 普通运行时校验文件和参考客户端的输出一致；三套 SDK 跑同一份文件（AC-CHAT-90-15）。

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"slices"
	"testing"
)

var update = flag.Bool("update", false, "rewrite protocol/vectors/chat-sync.json from the reference client")

const vectorsPath = "../../../protocol/vectors/chat-sync.json"

type expect struct {
	Syncs        *[]string        `json:"syncs,omitempty"` // 只有 connect
	FilesExpired []string         `json:"filesExpired"`
	Messages     map[string][]Msg `json:"messages"`
}

type step struct {
	Op       string           `json:"op"` // connect | disconnect | load
	Convs    []Conv           `json:"convs,omitempty"`
	Replies  map[string][]Msg `json:"replies,omitempty"`
	Errors   map[string]int   `json:"errors,omitempty"`
	ConvID   string           `json:"convId,omitempty"`
	Messages []Msg            `json:"messages,omitempty"`
	Expect   *expect          `json:"expect,omitempty"`
}

type vcase struct {
	Name  string   `json:"name"`
	Rule  string   `json:"rule"`
	ACs   []string `json:"acs"`
	Steps []step   `json:"steps"`
}

type file struct {
	Description string            `json:"description"`
	Scope       string            `json:"scope"`
	Format      map[string]string `json:"format"`
	Self        string            `json:"self"`
	Cases       []vcase           `json:"cases"`
}

const (
	ab = "c2c:alice:bob"
	g1 = "group:g1"
	// 文件名是特征串：过期后内存、本地库都不能再有（F-FILE-06 规则 3）。
	secretName = "ZXQ-FILE-NAME.pdf"
)

func v(n int64) *int64 { return &n }

func img(seq int64, fileID string, expired bool) Msg {
	return Msg{Seq: seq, Type: "image", FileID: fileID, Expired: expired}
}

func doc(seq int64, fileID string, expired bool) Msg {
	m := Msg{Seq: seq, Type: "file", FileID: fileID, Name: secretName, Expired: expired}
	if expired {
		m.Name = ""
	}
	return m
}

// staleDoc 是迟到的旧应答：服务端读库时还没过期，带着文件名。
func staleDoc(seq int64, fileID string) Msg { return doc(seq, fileID, false) }

func conn(convs ...Conv) step { return step{Op: "connect", Convs: convs} }

func connReply(replies map[string][]Msg, convs ...Conv) step {
	return step{Op: "connect", Convs: convs, Replies: replies}
}

func load(convID string, msgs ...Msg) step { return step{Op: "load", ConvID: convID, Messages: msgs} }

var drop = step{Op: "disconnect"}

func cv(id string, maxSeq int64, modVer *int64) Conv {
	return Conv{ConvID: id, MaxSeq: maxSeq, ModVer: modVer}
}

// 两个会话都已加载：AB 有一张图片 F1、一个文件 F2，G1 有一张图片 G1F。
func loadedBoth(abVer, g1Ver *int64) []step {
	return []step{
		conn(cv(ab, 2, abVer), cv(g1, 1, g1Ver)),
		load(ab, img(1, "F1", false), doc(2, "F2", false)),
		load(g1, img(1, "G1F", false)),
	}
}

func steps(parts ...[]step) []step {
	var out []step
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

var cases = []vcase{
	{
		Name: "重连时只有 AB 的 modVer 变了：只重拉 AB，消息变成已过期；modVer 不变就不再拉", Rule: "F-RCL-03 规则 2、2a",
		ACs: []string{"AC-FILE-06-12", "AC-RCL-03-11"},
		Steps: steps(loadedBoth(v(3), v(7)), []step{
			drop,
			connReply(map[string][]Msg{ab: {img(1, "F1", true), doc(2, "F2", true)}}, cv(ab, 2, v(4)), cv(g1, 1, v(7))),
			drop,
			conn(cv(ab, 2, v(4)), cv(g1, 1, v(7))),
		}),
	},
	{
		Name: "expired 只能从假变真：之后的旧应答带 expired=false 和文件名，不回写", Rule: "F-RCL-03 规则 2a（RC2-10）",
		ACs: []string{"AC-FILE-06-12"},
		Steps: steps(loadedBoth(v(3), v(7)), []step{
			drop,
			connReply(map[string][]Msg{ab: {img(1, "F1", true), doc(2, "F2", true)}}, cv(ab, 2, v(4)), cv(g1, 1, v(7))),
			drop,
			connReply(map[string][]Msg{ab: {img(1, "F1", false), staleDoc(2, "F2")}}, cv(ab, 2, v(5)), cv(g1, 1, v(7))),
		}),
	},
	{
		Name: "一次应答里只有一条过期：另一条不受影响，之后单独过期", Rule: "F-RCL-03 规则 2a",
		ACs: []string{"AC-FILE-06-12"},
		Steps: steps(loadedBoth(v(3), v(7)), []step{
			drop,
			connReply(map[string][]Msg{ab: {img(1, "F1", true), doc(2, "F2", false)}}, cv(ab, 2, v(4)), cv(g1, 1, v(7))),
			drop,
			connReply(map[string][]Msg{ab: {img(1, "F1", false), doc(2, "F2", true)}, g1: {img(1, "G1F", true)}}, cv(ab, 2, v(5)), cv(g1, 1, v(8))),
		}),
	},
	{
		Name: "老服务端（convs 没有 modVer）：已加载的会话全部重拉", Rule: "F-RCL-03 规则 2",
		ACs: []string{"AC-RCL-03-11"},
		Steps: steps(loadedBoth(nil, nil), []step{
			drop,
			conn(cv(ab, 2, nil), cv(g1, 1, nil)),
		}),
	},
	{
		Name: "重拉失败：不记新的 modVer，下次重连还拉；成功之后才不拉", Rule: "F-RCL-03 规则 2",
		ACs: []string{"AC-RCL-03-11"},
		Steps: []step{
			conn(cv(ab, 2, v(3))),
			load(ab, img(1, "F1", false), doc(2, "F2", false)),
			drop,
			{Op: "connect", Convs: []Conv{cv(ab, 2, v(4))}, Errors: map[string]int{ab: 5000}},
			drop,
			connReply(map[string][]Msg{ab: {img(1, "F1", true), doc(2, "F2", false)}}, cv(ab, 2, v(4))),
			drop,
			conn(cv(ab, 2, v(4))),
		},
	},
	{
		Name: "没加载过的会话：modVer 变了也不发请求；加载时的 convs 值是基线", Rule: "F-RCL-03 规则 2",
		ACs: []string{"AC-RCL-03-11"},
		Steps: []step{
			conn(cv(ab, 2, v(3)), cv(g1, 1, v(7))),
			drop,
			conn(cv(ab, 2, v(4)), cv(g1, 1, v(7))),
			load(ab, img(1, "F1", true), doc(2, "F2", false)),
			drop,
			conn(cv(ab, 2, v(4)), cv(g1, 1, v(7))),
			drop,
			connReply(map[string][]Msg{ab: {img(1, "F1", false), staleDoc(2, "F2")}}, cv(ab, 2, v(5)), cv(g1, 1, v(7))),
		},
	},
}

func run(c vcase) vcase {
	cl := New()
	out := c
	out.Steps = make([]step, len(c.Steps))
	for i, st := range c.Steps {
		var e expect
		switch st.Op {
		case "connect":
			syncs, exp := cl.Connect(st.Convs, st.Replies, st.Errors)
			s := nonNil(syncs)
			e.Syncs, e.FilesExpired = &s, exp
		case "load":
			e.FilesExpired = cl.Load(st.ConvID, st.Messages)
		case "disconnect":
			out.Steps[i] = st
			continue
		default:
			panic("unknown op " + st.Op)
		}
		e.FilesExpired = nonNil(e.FilesExpired)
		e.Messages = map[string][]Msg{}
		for id := range cl.loaded {
			e.Messages[id] = cl.Messages(id)
		}
		st.Expect = &e
		out.Steps[i] = st
	}
	return out
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// TC-CHAT-90-13（AC-CHAT-90-15，服务端产出期望值的部分）；TC-FILE-06-11（AC-FILE-06-12）的向量。
func TestChatSyncVectors(t *testing.T) {
	f := file{
		Description: "消息同步共享用例（PRD-chat F-RCL-03 规则 2、2a；第 15 节表 C2 的\"合并和过期\"）。按顺序执行 steps，connect、load 之后断言 expect。期望结果由服务端的 Go 参考客户端（server/e2e/chatmodel）生成，不要手改。",
		Scope:       "C2 只有单调合并里的 expired 和 modVer 重拉。撤回、引用、提及、置顶、deleted、总未读随 C4、C5 加到这份文件。三套 SDK 用假传输层跑真实的客户端（AC-CHAT-90-15）。",
		Format: map[string]string{
			"self":       "登录用户。会话 c2c:alice:bob 的对方是 bob",
			"消息":         "{seq, type, fileId, name?, expired?} 展开为下行消息：msgId=<convId>#<seq>，clientMsgId=bob-<convId>-<seq>，from=bob，ts=1000+seq；image 的 body 为 {image:{fileId, width:10, height:10, size:100, mime:\"image/png\", expired?}}，file 为 {file:{fileId, name, size:100, mime:\"application/pdf\", expired?}}；expired 只在为 true 时出现",
			"会话":         "{convId, maxSeq, modVer?} 展开为 convs 里的会话：type 按前缀，name=convId，readSeq=maxSeq，minSeq=1，unread=0，lastMsg=null，updatedAt=0；modVer 不写表示老服务端没有这个字段",
			"connect":    "（重）连接成功，convs 应答为 convs。之后 SDK 发出的每个 sync 都按 convId 回复：errors 里有的回这个错误码，否则回 replies 里的消息（没有就是空列表），hasMore=false，直到不再发 sync。maxSeq 等于已加载的最大 seq，所以只有重拉，没有补拉",
			"disconnect": "连接断开（不是退出登录），之后按 SDK 的重连规则重连，由下一步 connect 完成",
			"load":       "App 打开会话加载最新一页（loadHistory），SDK 发出的 sync 回复 messages，hasMore=false。messages 为空时 SDK 可以不发请求",
			"expect":     "syncs：这一步 SDK 发过 sync 的会话（排好序，空列表表示一个都没有；重拉的顺序各端可以不同，用例里出错的步骤只有一个已加载的会话）。filesExpired：这一步新变成已过期的 fileId（排好序；sdk-ts 是 filesExpired 事件，原生端是 FilesExpired / .filesExpired），空列表表示没有。messages：已加载的会话的全部消息，按 seq 比较 type、fileId、expired（缺省为 false）和 file 的 name",
		},
		Self: "alice",
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
func TestChatModelRules(t *testing.T) {
	find := func(name string) vcase {
		for _, c := range cases {
			if c.Name == name {
				return run(c)
			}
		}
		t.Fatalf("no case %q", name)
		return vcase{}
	}
	r := run(cases[0])
	if s := r.Steps[4].Expect; !slices.Equal(*s.Syncs, []string{ab}) || !slices.Equal(s.FilesExpired, []string{"F1", "F2"}) {
		t.Errorf("only AB re-synced and both expired: %+v", s)
	}
	if s := r.Steps[6].Expect; len(*s.Syncs) != 0 {
		t.Errorf("unchanged modVer must not re-sync: %+v", s)
	}
	stale := find("expired 只能从假变真：之后的旧应答带 expired=false 和文件名，不回写").Steps[6].Expect
	if m := stale.Messages[ab]; !m[0].Expired || !m[1].Expired || m[1].Name != "" || len(stale.FilesExpired) != 0 {
		t.Errorf("stale reply must not write back: %+v", stale)
	}
	old := find("老服务端（convs 没有 modVer）：已加载的会话全部重拉").Steps[4].Expect
	if !slices.Equal(*old.Syncs, []string{ab, g1}) {
		t.Errorf("old server re-syncs all: %+v", old)
	}
	failed := find("重拉失败：不记新的 modVer，下次重连还拉；成功之后才不拉")
	if !slices.Equal(*failed.Steps[5].Expect.Syncs, []string{ab}) || len(*failed.Steps[7].Expect.Syncs) != 0 {
		t.Errorf("failed re-sync must be retried once: %+v", failed.Steps)
	}
	unloaded := find("没加载过的会话：modVer 变了也不发请求；加载时的 convs 值是基线")
	if len(*unloaded.Steps[2].Expect.Syncs) != 0 || len(*unloaded.Steps[5].Expect.Syncs) != 0 {
		t.Errorf("unloaded conversations are not re-synced: %+v", unloaded.Steps)
	}
	for _, c := range cases {
		for _, st := range run(c).Steps {
			if st.Expect == nil {
				continue
			}
			for _, list := range st.Expect.Messages {
				for _, m := range list {
					if m.Expired && m.Name != "" {
						t.Errorf("%s: expired file keeps its name", c.Name)
					}
				}
			}
		}
	}
}
