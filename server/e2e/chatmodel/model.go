// Package chatmodel 是消息同步的 Go 参考客户端（PRD-chat F-RCL-03 规则 2、2a，第 15 节表的 C2 部分，BE-403）：只做状态机，不做网络和存储。
// protocol/vectors/chat-sync.json 的期望结果由它生成，三套 SDK 以它为基准。
//
// C2 只有单调合并里的 expired 和 modVer 重拉：expired 只能从假变真，迟到的旧 sync 不回写，重连时只重拉 modVer 变了的已加载会话。
// 撤回、引用、提及、置顶、deleted、总未读随 C4、C5。
package chatmodel

import (
	"encoding/json"
	"sort"
)

// Msg 是一条消息里这批用例关心的部分。image、file 消息才有 FileID。
type Msg struct {
	Seq     int64  `json:"seq"`
	Type    string `json:"type"` // image | file
	FileID  string `json:"fileId"`
	Name    string `json:"name,omitempty"` // 文件名，只有 file
	Expired bool   `json:"expired,omitempty"`
}

// MarshalJSON：file 消息总是带 name（过期后是空串），image 没有 name。
func (m Msg) MarshalJSON() ([]byte, error) {
	type plain Msg
	if m.Type != "file" {
		return json.Marshal(plain(m))
	}
	return json.Marshal(struct {
		Seq     int64  `json:"seq"`
		Type    string `json:"type"`
		FileID  string `json:"fileId"`
		Name    string `json:"name"`
		Expired bool   `json:"expired,omitempty"`
	}{m.Seq, m.Type, m.FileID, m.Name, m.Expired})
}

// Conv 是 convs 应答里的一个会话。ModVer 为 nil 表示老服务端没有这个字段。
type Conv struct {
	ConvID string `json:"convId"`
	MaxSeq int64  `json:"maxSeq"`
	ModVer *int64 `json:"modVer,omitempty"`
}

// Client 是一个 SDK 实例里和这批用例有关的状态。
type Client struct {
	loaded   map[string]map[int64]Msg // 已加载的会话 → seq → 消息
	baseline map[string]int64         // 已加载内容对应的 modVer（最近一次重拉成功时，或加载前 convs 给的值）
}

func New() *Client {
	return &Client{loaded: map[string]map[int64]Msg{}, baseline: map[string]int64{}}
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

// merge 是单调合并（F-RCL-03 规则 2a）：已有的消息 expired 只能从假变真，变真时文件名清空；迟到的旧应答带 expired=false 和文件名不回写。
func (c *Client) merge(convID string, msgs []Msg) []string {
	list := c.loaded[convID]
	if list == nil {
		list = map[int64]Msg{}
		c.loaded[convID] = list
	}
	var expired []string
	for _, m := range msgs {
		old, ok := list[m.Seq]
		if ok {
			if m.Expired && !old.Expired {
				list[m.Seq] = expire(old)
				expired = append(expired, old.FileID)
			}
			continue
		}
		if m.Expired {
			m = expire(m)
			expired = append(expired, m.FileID)
		}
		list[m.Seq] = m
	}
	return expired
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
