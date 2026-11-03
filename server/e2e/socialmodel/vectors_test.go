package socialmodel

// protocol/vectors/social.json（PRD-social 10.3 节，BE-105 第 3 条、BE-118 第 2 条）：给定一串同步应答页 / 事件 / ack，
// 断言最终的好友、申请、黑名单、红点数、socialVer。期望结果由本参考客户端生成：
//
//	go test ./e2e/socialmodel -run TestSocialVectors -update
//
// 普通运行时校验文件与参考客户端的输出一致；三套 SDK 跑同一份文件。

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"tianting/server/internal/protocol"
)

var update = flag.Bool("update", false, "rewrite protocol/vectors/social.json from the reference client")

const vectorsPath = "../../../protocol/vectors/social.json"

type step struct {
	Type string          `json:"type"` // begin | page | event | ack
	Data json.RawMessage `json:"data,omitempty"`
	Ver  int64           `json:"ver,omitempty"` // event
}

type expect struct {
	SocialVer int64             `json:"socialVer"`
	Friends   map[string]string `json:"friends"`  // userId → remark
	Requests  map[string]string `json:"requests"` // "userId/direction" → status
	Blocks    []string          `json:"blocks"`
	Unread    int               `json:"unread"`
	NeedSync  bool              `json:"needSync"` // 处理完后是否要立即（再）开始一轮
	LastNext  string            `json:"lastNext,omitempty"`
}

type vcase struct {
	Name   string `json:"name"`
	Rule   string `json:"rule"`
	NowMs  int64  `json:"nowMs"` // 服务端时间（过期、30 天用它判断，不用本地时钟）
	Steps  []step `json:"steps"`
	Expect expect `json:"expect"`
}

type copyCase struct {
	Cmd  string `json:"cmd"`
	Code int    `json:"code"`
	Text string `json:"text"`
}

type vectorFile struct {
	Description string     `json:"description"`
	Cases       []vcase    `json:"cases"`
	Copy        []copyCase `json:"copy"`
}

const now = int64(1_800_000_000_000)
const dayMs = int64(24 * 3600 * 1000)

func raw(v any) json.RawMessage { b, _ := json.Marshal(v); return b }

func page(full bool, until, latest int64, more bool, fs []protocol.FriendItem, rs []protocol.RequestItem, bs []protocol.BlockItem) step {
	p := protocol.SocialSyncAck{Full: full, UntilVer: until, LatestVer: latest, HasMore: more, Friends: fs, Requests: rs, Blocks: bs,
		Settings: protocol.PrivacySettings{Searchable: true}, Mode: "open"}
	if more {
		p.Cursor = "opaque-cursor"
	}
	for _, x := range []*[]protocol.FriendItem{&p.Friends} {
		if *x == nil {
			*x = []protocol.FriendItem{}
		}
	}
	if p.Requests == nil {
		p.Requests = []protocol.RequestItem{}
	}
	if p.Blocks == nil {
		p.Blocks = []protocol.BlockItem{}
	}
	return step{Type: "page", Data: raw(p)}
}

func fr(id string, ver int64, del bool) protocol.FriendItem {
	return protocol.FriendItem{UserID: id, Remark: "", Source: "other", CreatedAt: now - dayMs, Ver: ver, Deleted: del}
}

func rq(id, dir, status string, ver, updatedAt int64) protocol.RequestItem {
	return protocol.RequestItem{UserID: id, Direction: dir, Status: status, Source: "other", ApplyCount: 1, CreatedAt: updatedAt, UpdatedAt: updatedAt, ExpireAt: updatedAt + 7*dayMs, Ver: ver}
}

var begin = step{Type: "begin"}

func cases() []vcase {
	return []vcase{
		{Name: "同一个 ver 跨页", Rule: "F-SOC-06 规则 3：游标按排序键严格大于，同 ver 的多行跨页不丢不重", NowMs: now, Steps: []step{
			begin, page(true, 4, 4, false, []protocol.FriendItem{fr("a", 4, false)}, nil, nil),
			begin, page(false, 5, 5, true, []protocol.FriendItem{fr("b", 5, false), fr("c", 5, false)}, nil, nil),
			page(false, 5, 5, false, []protocol.FriendItem{fr("d", 5, false)}, nil, nil)}},
		{Name: "全量轮收齐后整体替换", Rule: "F-SOC-06 规则 8：全量轮不含墓碑，本地有而全量没有的项要删除", NowMs: now, Steps: []step{
			begin, page(true, 3, 3, false, []protocol.FriendItem{fr("x", 2, false), fr("y", 3, false)}, nil, nil),
			begin, page(true, 9, 9, true, []protocol.FriendItem{fr("z", 8, false)}, nil, nil),
			page(true, 9, 9, false, nil, nil, []protocol.BlockItem{{UserID: "w", CreatedAt: now, Ver: 9}})}},
		{Name: "增量轮的墓碑删除本地项", Rule: "F-SOC-06 规则 2、8：deleted=true 表示删除，按键保留 ver 较大的一项", NowMs: now, Steps: []step{
			begin, page(true, 3, 3, false, []protocol.FriendItem{fr("x", 2, false), fr("y", 3, false)}, nil, nil),
			begin, page(false, 4, 4, false, []protocol.FriendItem{fr("x", 4, true)}, nil, nil)}},
		{Name: "轮内收到更大的事件：本轮结束后立即再来一轮", Rule: "10.2 节：同步轮进行中收到的事件在本轮结束后处理", NowMs: now, Steps: []step{
			begin, page(true, 8, 8, true, []protocol.FriendItem{fr("a", 8, false)}, nil, nil),
			{Type: "event", Ver: 9},
			page(true, 8, 8, false, []protocol.FriendItem{fr("b", 8, false)}, nil, nil)}},
		{Name: "最后一页 latestVer > untilVer：立即再来一轮", Rule: "F-SOC-06 规则 8", NowMs: now, Steps: []step{
			begin, page(true, 8, 10, false, []protocol.FriendItem{fr("a", 8, false)}, nil, nil)}},
		{Name: "restart：放弃本轮，用同一个 sinceVer 重新开始", Rule: "F-SOC-06 规则 3：增量轮中途执行过清理", NowMs: now, Steps: []step{
			begin, page(true, 5, 5, false, []protocol.FriendItem{fr("a", 5, false)}, nil, nil),
			begin, page(false, 7, 7, true, []protocol.FriendItem{fr("b", 6, false)}, nil, nil),
			{Type: "page", Data: raw(protocol.SocialSyncAck{Restart: true, Friends: []protocol.FriendItem{}, Requests: []protocol.RequestItem{}, Blocks: []protocol.BlockItem{}})},
			page(true, 7, 7, false, []protocol.FriendItem{fr("b", 6, false), fr("c", 7, false)}, nil, nil)}},
		{Name: "sinceVer 小于 purgedVer：服务端判定全量", Rule: "F-SOC-06 规则 5：本地 ver < purgedVer 可能错过了被清理的墓碑", NowMs: now, Steps: []step{
			begin, page(true, 3, 3, false, []protocol.FriendItem{fr("gone", 3, false)}, nil, nil),
			begin, page(true, 40, 40, false, []protocol.FriendItem{fr("kept", 12, false)}, nil, nil)}},
		{Name: "ack 的 ver 连续：应用 items 并前进", Rule: "F-SOC-06 规则 7", NowMs: now, Steps: []step{
			begin, page(true, 5, 5, false, nil, nil, nil),
			{Type: "ack", Data: raw(map[string]any{"ver": 6, "changed": true, "items": protocol.SocialItems{
				Friends: []protocol.FriendItem{fr("z", 6, false)}, Requests: []protocol.RequestItem{rq("z", "in", "accepted", 6, now)}, Blocks: []protocol.BlockItem{}}})}}},
		{Name: "ack 的 ver 不连续：先应用 items，再从本地 ver 补一轮", Rule: "F-SOC-06 规则 7", NowMs: now, Steps: []step{
			begin, page(true, 5, 5, false, nil, nil, nil),
			{Type: "ack", Data: raw(map[string]any{"ver": 7, "changed": true, "items": protocol.SocialItems{
				Friends: []protocol.FriendItem{fr("z", 7, false)}, Requests: []protocol.RequestItem{}, Blocks: []protocol.BlockItem{}}})},
			{Type: "ack", Data: raw(map[string]any{"ver": 5, "changed": false, "items": protocol.SocialItems{Friends: []protocol.FriendItem{}, Requests: []protocol.RequestItem{}, Blocks: []protocol.BlockItem{}}})}}},
		{Name: "30 天丢弃与过期、红点", Rule: "F-SOC-04 规则 1、2：按服务端时间；红点 = 收到的、pending、未过期、ver > requestSeenVer", NowMs: now, Steps: []step{
			begin, page(true, 9, 9, false, nil, []protocol.RequestItem{
				rq("old", "in", "pending", 2, now-31*dayMs),
				rq("expired", "in", "pending", 3, now-8*dayMs),
				rq("rejected", "in", "rejected", 4, now-dayMs),
				rq("fresh1", "in", "pending", 8, now-dayMs),
				rq("fresh2", "in", "pending", 9, now-dayMs),
				rq("mine", "out", "pending", 7, now-dayMs),
			}, nil)}},
		{Name: "事件 ver 不大于本地 ver：忽略", Rule: "10.2 节", NowMs: now, Steps: []step{
			begin, page(true, 5, 5, false, nil, nil, nil), {Type: "event", Ver: 5}, {Type: "event", Ver: 3}}},
	}
}

// copyTable 是"命令 + 错误码"文案（PRD-social 9.4 节，ERRORS.md 第 2 节）。
func copyTable() []copyCase {
	return []copyCase{
		{"send", 4290, "发送太频繁，请稍后再试"},
		{"friendApply", 4290, "操作太频繁，请稍后再试"},
		{"send", 4035, "对方开启了好友验证，你还不是他的好友"},
		{"send", 4036, "消息已发出，但被对方拒收了"},
		{"send", 4137, "对方回复之前，最多只能发送 {N} 条消息"},
		{"send", 4292, "今天发起新聊天的次数已用完，请稍后再试"},
		{"friendApply", 4132, "你的好友数量已达上限"},
		{"friendRespond", 4132, "你的好友数量已达上限"},
		{"friendRespond", 4133, "对方好友数量已达上限"},
		{"friendApply", 4133, "对方好友数量已达上限"},
		{"friendRespond", 4045, "该申请已过期"},
		{"block", 4134, "黑名单已满"},
		{"friendApply", 4292, "今天的次数已用完，明天再试"},
		{"userSearch", 4292, "今天的次数已用完，明天再试"},
		{"block", 4292, "今天的次数已用完，明天再试"},
		{"unblock", 4292, "今天的次数已用完，明天再试"},
	}
}

func run(c vcase) (expect, error) {
	m := New()
	last := ""
	for _, s := range c.Steps {
		switch s.Type {
		case "begin":
			m.BeginRound()
		case "page":
			var p protocol.SocialSyncAck
			if err := json.Unmarshal(s.Data, &p); err != nil {
				return expect{}, err
			}
			last = [...]string{"continue", "restart", "done"}[m.ApplyPage(p)]
		case "event":
			m.OnSocialChanged(s.Ver)
		case "ack":
			var a struct {
				Ver     int64                `json:"ver"`
				Changed bool                 `json:"changed"`
				Items   protocol.SocialItems `json:"items"`
			}
			if err := json.Unmarshal(s.Data, &a); err != nil {
				return expect{}, err
			}
			m.ApplyAck(a.Ver, a.Changed, a.Items)
		}
	}
	v := m.View(c.NowMs)
	e := expect{SocialVer: v.Ver, Friends: map[string]string{}, Requests: map[string]string{}, Blocks: []string{}, Unread: v.Unread, NeedSync: m.NeedSync, LastNext: last}
	for id, f := range v.Friends {
		e.Friends[id] = f.Remark
	}
	for k, r := range v.Requests {
		e.Requests[k.UserID+"/"+k.Direction] = r.Status
	}
	for id := range v.Blocks {
		e.Blocks = append(e.Blocks, id)
	}
	slices.Sort(e.Blocks)
	return e, nil
}

func TestSocialVectors(t *testing.T) {
	path := filepath.Clean(vectorsPath)
	if *update {
		f := vectorFile{
			Description: "关系同步共享用例（PRD-social 10.3 节）：按顺序执行 steps（begin = 开始一轮；page = 一页 socialSync 应答；event = socialChanged {ver}；ack = 自己发起的修改的 ack），然后按 nowMs（服务端时间）断言 expect。expect.requests 的键为 userId/direction，值为按服务端时间算出的状态；needSync 表示是否要立即（再）开始一轮。copy 是\"命令 + 错误码\"文案（{N} 取 strangerMsgLimit）。期望结果由服务端的 Go 参考客户端（server/e2e/socialmodel）生成。",
			Copy:        copyTable(),
		}
		for _, c := range cases() {
			e, err := run(c)
			if err != nil {
				t.Fatal(err)
			}
			c.Expect = e
			f.Cases = append(f.Cases, c)
		}
		b, _ := json.MarshalIndent(f, "", "  ")
		if err := os.WriteFile(path, append(b, '\n'), 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var f vectorFile
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Cases) < 8 {
		t.Fatalf("social.json has %d cases, PRD 10.3 requires at least 8 kinds", len(f.Cases))
	}
	for _, c := range f.Cases {
		got, err := run(c)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, c.Expect) {
			t.Errorf("%s:\n got %+v\nwant %+v", c.Name, got, c.Expect)
		}
	}
	if !reflect.DeepEqual(f.Copy, copyTable()) {
		t.Error("copy table differs from PRD-social 9.4; rerun with -update")
	}
}
