// Package chatmodel 是消息同步的 Go 参考客户端（PRD-chat F-RCL-03 规则 2、2a，F-QUO-02 规则 4，F-MEN-03 规则 1，第 15 节表的 C2、C4 部分，
// BE-403、BE-478）：只做状态机，不做网络和存储。protocol/vectors/chat-sync.json 的期望结果由它生成，三套 SDK 以它为基准。
//
// C2：单调合并里的 expired 和 modVer 重拉：expired 只能从假变真，迟到的旧 sync 不回写，重连时只重拉 modVer 变了的已加载会话。
// C4：deleted、recalled 只能从假变真（变真时清除原文、文件名和引用）；quote.state 只能从 normal 往后走，quote.text 清除后不恢复；
// 收到 msgDeleted 时清除本地所有引用这条消息的原文；收到 msgDeleted、readUpdated 且这个会话本地 mentionUnread > 0 时，
// 节流（每 5 秒最多一次）重新拉 convs。置顶、删除会话、总未读随 C5。
package chatmodel

import (
	"encoding/json"
	"sort"
)

// 引用的状态（F-QUO-02 规则 1）。
const (
	quoteNormal   = "normal"
	quoteRecalled = "recalled"
	quoteDeleted  = "deleted"
)

// MentionRefreshMs 是提及计数刷新的节流间隔（F-MEN-03 规则 1，未校准）。
const MentionRefreshMs = 5000

// Quote 是一条消息引用的另一条（同一会话，下行 QuoteView 里这批用例关心的部分）。
type Quote struct {
	Seq   int64  `json:"seq"`
	State string `json:"state"`
	Text  string `json:"text,omitempty"`
}

// Msg 是一条消息里这批用例关心的部分。image、file 消息才有 FileID；text 消息有 Text，可以带 Quote。
type Msg struct {
	Seq        int64  `json:"seq"`
	Type       string `json:"type"` // image | file | text
	FileID     string `json:"fileId,omitempty"`
	Name       string `json:"name,omitempty"` // 文件名，只有 file
	Expired    bool   `json:"expired,omitempty"`
	Text       string `json:"text,omitempty"`
	Quote      *Quote `json:"quote,omitempty"`
	Deleted    bool   `json:"deleted,omitempty"`
	Recalled   bool   `json:"recalled,omitempty"`
	RecalledBy string `json:"recalledBy,omitempty"`
}

// MarshalJSON：没被删除的 file 消息总是带 name（过期后是空串），image 没有 name；删除、撤回的消息只有 seq、type、deleted、recalled*。
func (m Msg) MarshalJSON() ([]byte, error) {
	type plain Msg
	switch {
	case m.Deleted:
		return json.Marshal(struct {
			Seq        int64  `json:"seq"`
			Type       string `json:"type"`
			Deleted    bool   `json:"deleted"`
			Recalled   bool   `json:"recalled,omitempty"`
			RecalledBy string `json:"recalledBy,omitempty"`
		}{m.Seq, m.Type, true, m.Recalled, m.RecalledBy})
	case m.Type == "file":
		return json.Marshal(struct {
			Seq     int64  `json:"seq"`
			Type    string `json:"type"`
			FileID  string `json:"fileId"`
			Name    string `json:"name"`
			Expired bool   `json:"expired,omitempty"`
		}{m.Seq, m.Type, m.FileID, m.Name, m.Expired})
	}
	return json.Marshal(plain(m))
}

// Conv 是 convs 应答里的一个会话。ModVer 为 nil 表示老服务端没有这个字段；MentionUnread 为 nil 表示没有这个字段（C4 开关关闭或老服务端）。
type Conv struct {
	ConvID        string `json:"convId"`
	MaxSeq        int64  `json:"maxSeq"`
	ModVer        *int64 `json:"modVer,omitempty"`
	MentionUnread *int64 `json:"mentionUnread,omitempty"`
}

// Recalled 是 msgDeleted 事件的 recalled（F-RCL-01 规则 5a）。
type Recalled struct {
	By string `json:"by"`
	At int64  `json:"at"`
}

// Event 是下行事件里这批用例用到的两种：msgDeleted（后台删除；带 Recalled 是撤回）和 readUpdated（另一端已读）。字段和协议相同。
type Event struct {
	Type     string    `json:"type"`
	ConvID   string    `json:"convId"`
	MsgIDs   []string  `json:"msgIds,omitempty"`
	Seqs     []int64   `json:"seqs,omitempty"`
	Recalled *Recalled `json:"recalled,omitempty"`
	ReadSeq  int64     `json:"readSeq,omitempty"`
}

// Client 是一个 SDK 实例里和这批用例有关的状态。
type Client struct {
	loaded   map[string]map[int64]Msg // 已加载的会话 → seq → 消息
	baseline map[string]int64         // 已加载内容对应的 modVer（最近一次重拉成功时，或加载前 convs 给的值）
	mentions map[string]int64         // 最近一次 convs 给的 mentionUnread（只记有这个字段的会话）
	// 提及计数的节流（F-MEN-03 规则 1）：now 是 [SDK 钟]，lastRefresh 是上一次因为这条规则拉 convs 的时间；
	// 窗口内又有事件时记 pending，窗口结束时补拉一次。
	now          int64
	refreshed    bool
	lastRefresh  int64
	pendingFetch bool
}

func New() *Client {
	return &Client{loaded: map[string]map[int64]Msg{}, baseline: map[string]int64{}, mentions: map[string]int64{}}
}

// Load 是 App 打开会话、loadHistory 拿到的一页。返回这次新变成过期的 fileId。
func (c *Client) Load(convID string, msgs []Msg) []string {
	return c.merge(convID, msgs)
}

// Connect 处理一次（重）连接：convs 应答之后，对已加载的会话按 modVer 决定要不要重拉（F-RCL-03 规则 2）。
// replies 是每个会话重拉的应答，errs 是重拉失败的错误码。每个要重拉的会话独立处理（P-39 ②，Frank 2026-11-10）：
// 一个失败只影响它自己——不记新的 modVer，下次重连还会拉；其余会话这一轮照常重拉并记新的 modVer。返回发过 sync 的会话（排好序）和新变成过期的 fileId。
func (c *Client) Connect(convs []Conv, replies map[string][]Msg, errs map[string]int) (syncs, expired []string) {
	byID := map[string]Conv{}
	for _, cv := range convs {
		byID[cv.ConvID] = cv
		// 还没加载过的会话：之后加载到的就是这个版本以后的内容，记下作为基线。已加载的等重拉成功再更新。
		if _, ok := c.loaded[cv.ConvID]; !ok && cv.ModVer != nil {
			c.baseline[cv.ConvID] = *cv.ModVer
		}
	}
	c.applyMentions(convs)
	ids := make([]string, 0, len(c.loaded))
	for id := range c.loaded {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		cv, inList := byID[id]
		if inList && cv.ModVer != nil {
			if b, ok := c.baseline[id]; ok && b == *cv.ModVer {
				continue
			}
		}
		// 没有 modVer（老服务端）或这次 convs 里没有它：按原来的做法重拉。
		syncs = append(syncs, id)
		if _, failed := errs[id]; failed {
			continue
		}
		expired = append(expired, c.merge(id, replies[id])...)
		if inList && cv.ModVer != nil {
			c.baseline[id] = *cv.ModVer
		}
	}
	sort.Strings(expired)
	return syncs, expired
}

func (c *Client) applyMentions(convs []Conv) {
	for _, cv := range convs {
		if cv.MentionUnread != nil {
			c.mentions[cv.ConvID] = *cv.MentionUnread
		}
	}
}

// Event 处理一个下行事件（[SDK 钟] 不动）。convs 是这一步里 SDK 拉 convs 时的应答（不拉就不用）。返回这一步发出的 convs 请求数。
func (c *Client) Event(ev Event, convs []Conv) int {
	if ev.Type == "msgDeleted" {
		c.applyDeleted(ev)
	}
	if (ev.Type == "msgDeleted" || ev.Type == "readUpdated") && c.mentions[ev.ConvID] > 0 {
		return c.requestRefresh(convs)
	}
	return 0
}

// Advance 把 [SDK 钟] 推进 ms 毫秒；节流窗口结束时补拉一次 convs。返回这一步发出的 convs 请求数。
func (c *Client) Advance(ms int64, convs []Conv) int {
	c.now += ms
	if c.pendingFetch && c.now >= c.lastRefresh+MentionRefreshMs {
		c.pendingFetch = false
		c.lastRefresh = c.lastRefresh + MentionRefreshMs
		c.applyMentions(mustConvs(convs))
		return 1
	}
	return 0
}

// requestRefresh：窗口外立即拉，窗口内记下来等窗口结束（5 秒内多个事件只拉一次）。
func (c *Client) requestRefresh(convs []Conv) int {
	if !c.refreshed || c.now-c.lastRefresh >= MentionRefreshMs {
		c.refreshed, c.lastRefresh = true, c.now
		c.applyMentions(mustConvs(convs))
		return 1
	}
	c.pendingFetch = true
	return 0
}

func mustConvs(convs []Conv) []Conv {
	if convs == nil {
		panic("chatmodel: the SDK refreshes convs here but the step has no convs reply")
	}
	return convs
}

// applyDeleted 是收到 msgDeleted（F-RCL-03 规则 1，F-QUO-02 规则 4）：已加载的这几条变成已删除（带 recalled 时是撤回），清除原文；
// 这个会话里所有引用这几条的消息清除引用原文，state 变成 recalled 或 deleted。没加载的消息不处理（之后加载时服务端给的就是新状态）。
func (c *Client) applyDeleted(ev Event) {
	list := c.loaded[ev.ConvID]
	if list == nil {
		return
	}
	gone := map[int64]bool{}
	for _, seq := range ev.Seqs {
		gone[seq] = true
		if m, ok := list[seq]; ok {
			list[seq] = deleteMsg(m, ev.Recalled)
		}
	}
	state := quoteDeleted
	if ev.Recalled != nil {
		state = quoteRecalled
	}
	for seq, m := range list {
		if m.Quote != nil && gone[m.Quote.Seq] && m.Quote.State == quoteNormal {
			m.Quote = &Quote{Seq: m.Quote.Seq, State: state}
			list[seq] = m
		}
	}
}

func deleteMsg(m Msg, r *Recalled) Msg {
	out := Msg{Seq: m.Seq, Type: m.Type, Deleted: true, Recalled: m.Recalled, RecalledBy: m.RecalledBy}
	if r != nil && !out.Recalled {
		out.Recalled, out.RecalledBy = true, r.By
	}
	return out
}

// merge 是单调合并（F-RCL-03 规则 2a）：已有的消息 expired、deleted、recalled 只能从假变真，变真时清除文件名、原文和引用；
// quote.state 只能从 normal 往后走，quote.text 清除后不恢复；迟到的旧应答不回写。
func (c *Client) merge(convID string, msgs []Msg) []string {
	list := c.loaded[convID]
	if list == nil {
		list = map[int64]Msg{}
		c.loaded[convID] = list
	}
	var expired []string
	for _, m := range msgs {
		old, ok := list[m.Seq]
		if !ok {
			if m.Deleted {
				m = deleteMsg(Msg{Seq: m.Seq, Type: m.Type}, recalledOf(m))
			} else if m.Expired {
				m = expire(m)
				expired = append(expired, m.FileID)
			}
			list[m.Seq] = m
			continue
		}
		switch {
		case old.Deleted:
			if m.Recalled && !old.Recalled {
				old.Recalled, old.RecalledBy = true, m.RecalledBy
			}
		case m.Deleted:
			old = deleteMsg(old, recalledOf(m))
		default:
			if m.Expired && !old.Expired {
				old = expire(old)
				expired = append(expired, old.FileID)
			}
			if old.Quote != nil && m.Quote != nil && old.Quote.State == quoteNormal && m.Quote.State != quoteNormal {
				old.Quote = &Quote{Seq: old.Quote.Seq, State: m.Quote.State}
			}
		}
		list[m.Seq] = old
	}
	return expired
}

func recalledOf(m Msg) *Recalled {
	if !m.Recalled {
		return nil
	}
	return &Recalled{By: m.RecalledBy}
}

func expire(m Msg) Msg {
	m.Expired = true
	if m.Type == "file" {
		m.Name = "" // 服务端过期时同样清除文件名（F-FILE-06 规则 3）
	}
	return m
}

// Messages 返回一个会话已加载的消息，按 seq 升序。
func (c *Client) Messages(convID string) []Msg {
	list := c.loaded[convID]
	out := make([]Msg, 0, len(list))
	for _, m := range list {
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Seq < out[j].Seq })
	return out
}

// MentionUnread 返回本地记的 mentionUnread（只有 convs 带过这个字段的会话）。
func (c *Client) MentionUnread() map[string]int64 {
	out := make(map[string]int64, len(c.mentions))
	for k, v := range c.mentions {
		out[k] = v
	}
	return out
}
