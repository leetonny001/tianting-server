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
	"strconv"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite protocol/vectors/chat-sync.json from the reference client")

const vectorsPath = "../../../protocol/vectors/chat-sync.json"

type expect struct {
	Syncs        *[]string        `json:"syncs,omitempty"` // 只有 connect
	FilesExpired []string         `json:"filesExpired"`
	Messages     map[string][]Msg `json:"messages"`
	// C4：ConvsFetches 只在 event、advance 之后（这一步 SDK 为了提及计数发了几次 convs）；MentionUnread 只在 convs 带过这个字段时出现。
	ConvsFetches  *int             `json:"convsFetches,omitempty"`
	MentionUnread map[string]int64 `json:"mentionUnread,omitempty"`
}

type step struct {
	Op       string           `json:"op"` // connect | disconnect | load | event | advance
	Convs    []Conv           `json:"convs,omitempty"`
	Replies  map[string][]Msg `json:"replies,omitempty"`
	Errors   map[string]int   `json:"errors,omitempty"`
	ConvID   string           `json:"convId,omitempty"`
	Messages []Msg            `json:"messages,omitempty"`
	Event    *Event           `json:"event,omitempty"` // 只有 event
	Ms       int64            `json:"ms,omitempty"`    // 只有 advance
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
	g2 = "group:g2"
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

// C4 的特征串：撤回、引用清除之后内存和本地库都不能再有（AC-RCL-03-13、AC-QUO-02-6）。
const (
	secretRecall = "ZXQ-RCL-2 原文"
	secretQuote  = "ZXQ-QUO-1 被引用的原文"
)

func text(seq int64, s string) Msg { return Msg{Seq: seq, Type: "text", Text: s} }

func quoting(seq int64, s string, qseq int64, state, qtext string) Msg {
	return Msg{Seq: seq, Type: "text", Text: s, Quote: &Quote{Seq: qseq, State: state, Text: qtext}}
}

func recalledMsg(seq int64, by string) Msg {
	return Msg{Seq: seq, Type: "text", Deleted: true, Recalled: true, RecalledBy: by}
}

func recallEvent(convID, by string, seqs ...int64) step {
	ids := make([]string, len(seqs))
	for i, s := range seqs {
		ids[i] = msgID(convID, s)
	}
	return step{Op: "event", Event: &Event{Type: "msgDeleted", ConvID: convID, MsgIDs: ids, Seqs: seqs, Recalled: &Recalled{By: by, At: 2000}}}
}

func msgID(convID string, seq int64) string { return convID + "#" + strconv.FormatInt(seq, 10) }

func cvm(id string, maxSeq int64, modVer *int64, mentions int64) Conv {
	return Conv{ConvID: id, MaxSeq: maxSeq, ModVer: modVer, MentionUnread: &mentions}
}

func withConvs(st step, convs ...Conv) step { st.Convs = convs; return st }

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
		// P-39 ②（Frank 2026-11-10）：失败的跳过、其余照拉。G1 排在中间，按顺序重拉时它后面的 G2 也要拉。
		Name: "多个会话要重拉、中间一个失败：其余照拉并记新的 modVer，失败的下次重连再拉", Rule: "F-RCL-03 规则 2（P-39 ②）",
		ACs: []string{"AC-RCL-03-11"},
		Steps: []step{
			conn(cv(ab, 2, v(3)), cv(g1, 1, v(7)), cv(g2, 1, v(1))),
			load(ab, img(1, "F1", false), doc(2, "F2", false)),
			load(g1, img(1, "G1F", false)),
			load(g2, img(1, "G2F", false)),
			drop,
			{
				Op: "connect", Convs: []Conv{cv(ab, 2, v(4)), cv(g1, 1, v(8)), cv(g2, 1, v(2))},
				Replies: map[string][]Msg{ab: {img(1, "F1", true), doc(2, "F2", true)}, g2: {img(1, "G2F", true)}},
				Errors:  map[string]int{g1: 5000},
			},
			drop,
			connReply(map[string][]Msg{g1: {img(1, "G1F", true)}}, cv(ab, 2, v(4)), cv(g1, 1, v(8)), cv(g2, 1, v(2))),
			drop,
			conn(cv(ab, 2, v(4)), cv(g1, 1, v(8)), cv(g2, 1, v(2))),
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
	// ---- C4（BE-478） ----
	{
		Name: "撤回：收到带 recalled 的 msgDeleted 后原文清除，引用它的消息清除引用原文；迟到的旧 sync 带 deleted=false 和原文、引用 state=normal，都不恢复",
		Rule: "F-RCL-03 规则 1、2a，F-QUO-02 规则 4（RC2-10）", ACs: []string{"AC-RCL-03-13", "AC-QUO-02-2"},
		Steps: []step{
			conn(cv(ab, 2, v(3))),
			load(ab, text(1, secretRecall), quoting(2, "回复", 1, "normal", secretRecall)),
			recallEvent(ab, "bob", 1),
			drop,
			connReply(map[string][]Msg{ab: {text(1, secretRecall), quoting(2, "回复", 1, "normal", secretRecall)}}, cv(ab, 2, v(4))),
		},
	},
	{
		Name: "撤回：重连后 sync 返回撤回状态才知道（离线期间撤回）；recalled 只能从假变真，之后的旧应答不回写",
		Rule: "F-RCL-03 规则 2、2a", ACs: []string{"AC-RCL-03-13", "AC-RCL-03-11"},
		Steps: []step{
			conn(cv(ab, 2, v(3))),
			load(ab, text(1, secretRecall), text(2, "另一条")),
			drop,
			connReply(map[string][]Msg{ab: {recalledMsg(1, "bob"), text(2, "另一条")}}, cv(ab, 2, v(4))),
			drop,
			connReply(map[string][]Msg{ab: {text(1, secretRecall), text(2, "另一条")}}, cv(ab, 2, v(5))),
		},
	},
	{
		Name: "引用清除：m1 不在已加载范围、m2 在，离线期间 m1 被撤回；重连按 modVer 重拉 m2，引用原文被清，之后不恢复",
		Rule: "F-QUO-02 规则 4，F-RCL-03 规则 2a", ACs: []string{"AC-QUO-02-6"},
		Steps: []step{
			conn(cv(g1, 9, v(7))),
			load(g1, text(8, "前一条"), quoting(9, "回复很早的消息", 1, "normal", secretQuote)),
			drop,
			connReply(map[string][]Msg{g1: {text(8, "前一条"), quoting(9, "回复很早的消息", 1, "recalled", "")}}, cv(g1, 9, v(8))),
			drop,
			connReply(map[string][]Msg{g1: {text(8, "前一条"), quoting(9, "回复很早的消息", 1, "normal", secretQuote)}}, cv(g1, 9, v(9))),
		},
	},
	{
		Name: "引用清除：被引用的消息被后台删除（msgDeleted 不带 recalled），引用 state=deleted，被删的消息没有 recalled",
		Rule: "F-QUO-02 规则 4", ACs: []string{"AC-QUO-02-2"},
		Steps: []step{
			conn(cv(g1, 3, v(1))),
			load(g1, text(1, secretQuote), text(2, "中间"), quoting(3, "回复", 1, "normal", secretQuote)),
			{Op: "event", Event: &Event{Type: "msgDeleted", ConvID: g1, MsgIDs: []string{msgID(g1, 1)}, Seqs: []int64{1}}},
		},
	},
	{
		Name: "提及计数：本地 mentionUnread > 0 时收到 readUpdated（另一端已读）立即重新拉 convs，之后为 0",
		Rule: "F-MEN-03 规则 1（R-CF-18）", ACs: []string{"AC-MEN-03-7"},
		Steps: []step{
			conn(cvm(g1, 9, v(1), 1)),
			withConvs(step{Op: "event", Event: &Event{Type: "readUpdated", ConvID: g1, ReadSeq: 9}}, cvm(g1, 9, v(1), 0)),
		},
	},
	{
		Name: "提及计数：对方撤回提及消息（msgDeleted）后重新拉 convs；5 秒内多个事件只拉一次，窗口结束时补拉；mentionUnread=0 的会话不触发",
		Rule: "F-MEN-03 规则 1（R-CF-18）", ACs: []string{"AC-MEN-03-7"},
		Steps: []step{
			conn(cvm(g1, 9, v(1), 2), cvm(g2, 4, v(1), 1)),
			withConvs(recallEvent(g1, "bob", 5), cvm(g1, 9, v(2), 1), cvm(g2, 4, v(1), 1)),
			{Op: "advance", Ms: 1000},
			{Op: "event", Event: &Event{Type: "readUpdated", ConvID: g2, ReadSeq: 4}},
			{Op: "advance", Ms: 3000},
			withConvs(step{Op: "advance", Ms: 1000}, cvm(g1, 9, v(2), 1), cvm(g2, 4, v(1), 0)),
			{Op: "event", Event: &Event{Type: "readUpdated", ConvID: g2, ReadSeq: 4}},
			{Op: "advance", Ms: 10000},
			withConvs(step{Op: "event", Event: &Event{Type: "readUpdated", ConvID: g1, ReadSeq: 9}}, cvm(g1, 9, v(2), 0), cvm(g2, 4, v(1), 0)),
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
		case "event":
			n := cl.Event(*st.Event, st.Convs)
			e.ConvsFetches = &n
		case "advance":
			n := cl.Advance(st.Ms, st.Convs)
			e.ConvsFetches = &n
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
		if mu := cl.MentionUnread(); len(mu) > 0 {
			e.MentionUnread = mu
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
		Description: "消息同步共享用例（PRD-chat F-RCL-03 规则 2、2a，F-QUO-02 规则 4，F-MEN-03 规则 1；第 15 节表 C2 的\"合并和过期\"、C4 的\"撤回合并、引用清除、提及计数\"）。按顺序执行 steps，connect、load、event、advance 之后断言 expect。期望结果由服务端的 Go 参考客户端（server/e2e/chatmodel）生成，不要手改。",
		Scope:       "C2：单调合并里的 expired 和 modVer 重拉。C4：deleted、recalled 只能从假变真，quote.state 只能从 normal 往后走、quote.text 清除后不恢复，收到 msgDeleted 时清除引用它的原文，提及计数的节流刷新。置顶、删除会话、总未读随 C5 加到这份文件。三套 SDK 用假传输层跑真实的客户端（AC-CHAT-90-15）。",
		Format: map[string]string{
			"self":       "登录用户。会话 c2c:alice:bob 的对方是 bob",
			"消息":         "{seq, type, fileId?, name?, expired?, text?, quote?, deleted?, recalled?, recalledBy?} 展开为下行消息：msgId=<convId>#<seq>，clientMsgId=bob-<convId>-<seq>，from=bob，ts=1000+seq；image 的 body 为 {image:{fileId, width:10, height:10, size:100, mime:\"image/png\", expired?}}，file 为 {file:{fileId, name, size:100, mime:\"application/pdf\", expired?}}，text 为 {text, quote?}；quote {seq, state, text?} 展开为 {msgId:<convId>#<seq>, seq, from:\"bob\", type:\"text\", state, text?}；deleted=true 的消息没有 body，带 deleted:true，recalled=true 时再带 recalled:true、recalledAt=2000+seq、recalledBy；expired、deleted、recalled 只在为 true 时出现",
			"会话":         "{convId, maxSeq, modVer?, mentionUnread?} 展开为 convs 里的会话：type 按前缀，name=convId，readSeq=maxSeq，minSeq=1，unread=0，lastMsg=null，updatedAt=0；modVer 不写表示老服务端没有这个字段；mentionUnread 不写表示没有这个字段（C4 开关关闭或老服务端），firstMentionSeq 在 mentionUnread>0 时等于 maxSeq",
			"connect":    "（重）连接成功，convs 应答为 convs。之后 SDK 发出的每个 sync 都按 convId 回复：errors 里有的回这个错误码，否则回 replies 里的消息（没有就是空列表），hasMore=false，直到不再发 sync。maxSeq 等于已加载的最大 seq，所以只有重拉，没有补拉",
			"disconnect": "连接断开（不是退出登录），之后按 SDK 的重连规则重连，由下一步 connect 完成",
			"load":       "App 打开会话加载最新一页（loadHistory），SDK 发出的 sync 回复 messages，hasMore=false。messages 为空时 SDK 可以不发请求",
			"event":      "服务端下发一个事件，event 是事件帧的 data（msgDeleted、readUpdated，字段和协议相同）。这一步里 SDK 如果为了提及计数重新拉 convs（F-MEN-03 规则 1：这个会话本地 mentionUnread > 0 时，节流每 5 秒最多一次），应答是这一步的 convs；这些 convs 只用来更新 mentionUnread，modVer 和上次相同，不会引起重拉",
			"advance":    "[SDK 钟] 推进 ms 毫秒（每个用例从 0 开始；connect、load、event 不推进时钟）。节流窗口里被推迟的刷新在窗口结束（上次刷新后 5000 毫秒）时发出，应答是这一步的 convs",
			"expect":     "syncs：这一步 SDK 发过 sync 的会话（排好序，空列表表示一个都没有；重拉的顺序各端可以不同；某个会话重拉失败只影响它自己，其余会话照拉（P-39 ②），所以集合和顺序无关）。filesExpired：这一步新变成已过期的 fileId（排好序；sdk-ts 是 filesExpired 事件，原生端是 FilesExpired / .filesExpired），空列表表示没有。messages：已加载的会话的全部消息，按 seq 比较 type、fileId、expired（缺省为 false）、file 的 name、text、quote（seq、state、text）、deleted、recalled、recalledBy（缺省为 false / 空）；deleted 的消息只比较 seq、type、deleted、recalled、recalledBy，原文、文件名、引用都必须已清除（内存和本地库搜不到特征串 ZXQ-RCL-2、ZXQ-QUO-1）。convsFetches：event、advance 这一步 SDK 发了几次 convs。mentionUnread：本地记的 mentionUnread（只有 convs 带过这个字段的会话）",
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
	multi := find("多个会话要重拉、中间一个失败：其余照拉并记新的 modVer，失败的下次重连再拉")
	if s := multi.Steps[5].Expect; !slices.Equal(*s.Syncs, []string{ab, g1, g2}) || !slices.Equal(s.FilesExpired, []string{"F1", "F2", "G2F"}) {
		t.Errorf("a failed re-sync must not stop the others: %+v", s)
	}
	if m := multi.Steps[5].Expect.Messages; !m[g2][0].Expired || m[g1][0].Expired {
		t.Errorf("G2 merged after G1 failed, G1 untouched: %+v", m)
	}
	if s := multi.Steps[7].Expect; !slices.Equal(*s.Syncs, []string{g1}) || !slices.Equal(s.FilesExpired, []string{"G1F"}) {
		t.Errorf("only the failed conversation is retried, the others recorded the new modVer: %+v", s)
	}
	if s := multi.Steps[9].Expect; len(*s.Syncs) != 0 {
		t.Errorf("after the retry succeeds nothing is re-synced: %+v", s)
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
					if m.Deleted && (m.Text != "" || m.Name != "" || m.Quote != nil || m.FileID != "") {
						t.Errorf("%s: deleted message keeps content: %+v", c.Name, m)
					}
					if m.Quote != nil && m.Quote.State != "normal" && m.Quote.Text != "" {
						t.Errorf("%s: cleared quote keeps its text: %+v", c.Name, m.Quote)
					}
				}
			}
		}
	}

	// ---- C4（BE-478） ----
	recall := find("撤回：收到带 recalled 的 msgDeleted 后原文清除，引用它的消息清除引用原文；迟到的旧 sync 带 deleted=false 和原文、引用 state=normal，都不恢复")
	for _, i := range []int{2, 4} {
		m := recall.Steps[i].Expect.Messages[ab]
		if !m[0].Deleted || !m[0].Recalled || m[0].RecalledBy != "bob" || m[1].Quote.State != "recalled" || m[1].Quote.Text != "" || m[1].Text != "回复" {
			t.Errorf("step %d: recall must clear and stay cleared: %+v", i, m)
		}
	}
	if s := recall.Steps[4].Expect; !slices.Equal(*s.Syncs, []string{ab}) {
		t.Errorf("stale reply step re-synced: %+v", s)
	}
	offline := find("撤回：重连后 sync 返回撤回状态才知道（离线期间撤回）；recalled 只能从假变真，之后的旧应答不回写")
	for _, i := range []int{3, 5} {
		if m := offline.Steps[i].Expect.Messages[ab]; !m[0].Recalled || m[0].Text != "" || m[1].Text != "另一条" {
			t.Errorf("step %d: %+v", i, m)
		}
	}
	quote := find("引用清除：m1 不在已加载范围、m2 在，离线期间 m1 被撤回；重连按 modVer 重拉 m2，引用原文被清，之后不恢复")
	for _, i := range []int{3, 5} {
		if m := quote.Steps[i].Expect.Messages[g1]; m[1].Quote == nil || m[1].Quote.State != "recalled" || m[1].Quote.Text != "" {
			t.Errorf("step %d: quote must be cleared: %+v", i, m)
		}
	}
	admin := find("引用清除：被引用的消息被后台删除（msgDeleted 不带 recalled），引用 state=deleted，被删的消息没有 recalled").Steps[2].Expect.Messages[g1]
	if !admin[0].Deleted || admin[0].Recalled || admin[2].Quote.State != "deleted" || admin[1].Text != "中间" {
		t.Errorf("admin delete: %+v", admin)
	}
	read := find("提及计数：本地 mentionUnread > 0 时收到 readUpdated（另一端已读）立即重新拉 convs，之后为 0").Steps[1].Expect
	if *read.ConvsFetches != 1 || read.MentionUnread[g1] != 0 {
		t.Errorf("readUpdated refresh: %+v", read)
	}
	th := find("提及计数：对方撤回提及消息（msgDeleted）后重新拉 convs；5 秒内多个事件只拉一次，窗口结束时补拉；mentionUnread=0 的会话不触发").Steps
	for i, want := range map[int]int{1: 1, 2: 0, 3: 0, 4: 0, 5: 1, 6: 0, 7: 0, 8: 1} {
		if got := *th[i].Expect.ConvsFetches; got != want {
			t.Errorf("throttle step %d: %d convs requests, want %d", i, got, want)
		}
	}
	if mu := th[5].Expect.MentionUnread; mu[g2] != 0 || mu[g1] != 1 {
		t.Errorf("after the trailing refresh: %+v", mu)
	}
	if mu := th[8].Expect.MentionUnread; mu[g1] != 0 {
		t.Errorf("last refresh: %+v", mu)
	}
	for _, c := range cases[len(cases)-6:] {
		last := run(c).Steps[len(c.Steps)-1].Expect
		b, _ := json.Marshal(last.Messages)
		if strings.Contains(string(b), "ZXQ-") {
			t.Errorf("%s: secret still loaded at the end: %s", c.Name, b)
		}
	}
}
