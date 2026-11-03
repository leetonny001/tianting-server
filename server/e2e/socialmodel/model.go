// Package socialmodel 是关系同步的 Go 参考客户端（PRD-social 10.2 节，BE-119）：只做状态机，不做网络。
// 同步模型随机测试用它驱动真实连接；protocol/vectors/social.json 的期望结果也由它生成，三套 SDK 以它为基准。
package socialmodel

import (
	"time"

	"tianting/server/internal/protocol"
)

// retention 是申请在本地保留的时长（F-SOC-04 规则 1：超过 30 天的申请丢弃）。
const retention = 30 * 24 * time.Hour

// ReqKey 是申请项的键：对方 userId + 方向。
type ReqKey struct{ UserID, Direction string }

// Client 是一个端的本地社交状态。
type Client struct {
	Ver            int64
	Friends        map[string]protocol.FriendItem // 含墓碑（Deleted=true）：按 ver 取较大者时要用
	Requests       map[ReqKey]protocol.RequestItem
	Blocks         map[string]protocol.BlockItem
	RequestSeenVer int64
	Searchable     bool
	Mode           string
	StrangerLimit  int
	Official       bool

	// 同步轮
	inRound   bool
	full      bool
	sinceVer  int64
	untilVer  int64
	staged    *Client // 全量轮收齐前暂存
	pendingEv int64   // 轮进行中收到的、最大的 socialChanged ver
	NeedSync  bool    // 需要（再）开始一轮
}

func New() *Client {
	return &Client{Friends: map[string]protocol.FriendItem{}, Requests: map[ReqKey]protocol.RequestItem{}, Blocks: map[string]protocol.BlockItem{}}
}

// BeginRound 返回本轮第一页的 sinceVer（从本地 ver 开始；新客户端为 0，即全量）。
func (c *Client) BeginRound() int64 {
	c.inRound, c.staged, c.NeedSync = true, nil, false
	return c.Ver
}

// Next 是处理完一页之后客户端要做的事。
type Next int

const (
	Continue Next = iota // 用 cursor 拉下一页
	Restart              // 放弃本轮，用同一个 sinceVer 重新开始
	Done                 // 本轮结束（NeedSync 表示要立即再来一轮）
)

// ApplyPage 应用一页应答（F-SOC-06 规则 3、8）。
func (c *Client) ApplyPage(p protocol.SocialSyncAck) Next {
	if p.Restart {
		c.staged = nil
		return Restart
	}
	if c.staged == nil { // 本轮第一页
		c.full, c.untilVer = p.Full, p.UntilVer
		if p.Full {
			c.staged = New()
		} else {
			c.staged = c // 增量轮逐页直接应用
		}
	}
	target := c.staged
	for _, f := range p.Friends {
		target.mergeFriend(f)
	}
	for _, r := range p.Requests {
		target.mergeRequest(r)
	}
	for _, b := range p.Blocks {
		target.mergeBlock(b)
	}
	if p.HasMore {
		return Continue
	}
	if c.full { // 全量轮收齐后整体替换
		c.Friends, c.Requests, c.Blocks = c.staged.Friends, c.staged.Requests, c.staged.Blocks
	}
	c.Ver = c.untilVer
	c.RequestSeenVer, c.Searchable, c.Mode, c.StrangerLimit, c.Official = p.RequestSeenVer, p.Settings.Searchable, p.Mode, p.StrangerMsgLimit, p.Official
	c.inRound, c.staged = false, nil
	c.NeedSync = p.LatestVer > c.untilVer || c.pendingEv > c.untilVer
	c.pendingEv = 0
	return Done
}

// OnSocialChanged 处理 socialChanged {ver}（F-SOC-06 规则 6；去抖由驱动方负责）。
func (c *Client) OnSocialChanged(ver int64) {
	if c.inRound {
		c.pendingEv = max(c.pendingEv, ver)
		return
	}
	if ver > c.Ver {
		c.NeedSync = true
	}
}

// ApplyAck 处理自己发起的修改的 ack（F-SOC-06 规则 7）。
func (c *Client) ApplyAck(ver int64, changed bool, items protocol.SocialItems) {
	if !changed {
		return
	}
	for _, f := range items.Friends {
		c.mergeFriend(f)
	}
	for _, r := range items.Requests {
		c.mergeRequest(r)
	}
	for _, b := range items.Blocks {
		c.mergeBlock(b)
	}
	if ver == c.Ver+1 {
		c.Ver = ver
	} else if ver > c.Ver+1 {
		c.NeedSync = true // 中间有别的变化：从本地 ver 开始一轮
	}
}

// ApplyVerOnly 处理不带 items 的修改 ack（setPrivacy、friendRequestsSeen）：本地设置已由调用方更新。
func (c *Client) ApplyVerOnly(ver int64, changed bool) {
	if !changed {
		return
	}
	if ver == c.Ver+1 {
		c.Ver = ver
	} else if ver > c.Ver+1 {
		c.NeedSync = true
	}
}

func (c *Client) mergeFriend(f protocol.FriendItem) {
	if old, ok := c.Friends[f.UserID]; !ok || f.Ver >= old.Ver {
		c.Friends[f.UserID] = f
	}
}

func (c *Client) mergeRequest(r protocol.RequestItem) {
	k := ReqKey{r.UserID, r.Direction}
	if old, ok := c.Requests[k]; !ok || r.Ver >= old.Ver {
		c.Requests[k] = r
	}
}

func (c *Client) mergeBlock(b protocol.BlockItem) {
	if old, ok := c.Blocks[b.UserID]; !ok || b.Ver >= old.Ver {
		c.Blocks[b.UserID] = b
	}
}

// View 是用于比较的规范化状态：去掉墓碑和 30 天前的申请，申请状态按服务端时间算出 expired，不比较资料。
type View struct {
	Ver      int64
	Friends  map[string]FriendView
	Requests map[ReqKey]RequestView
	Blocks   map[string]int64 // userId → ver
	Unread   int
	Seen     int64
	Search   bool
}

type FriendView struct {
	Remark, Source string
	CreatedAt, Ver int64
}

type RequestView struct {
	Status, Message, Source string
	ApplyCount              int
	UpdatedAt, ExpireAt     int64
	Ver                     int64
}

// View 按服务端时间 nowMs 计算规范化状态（红点：收到的、pending、未过期、ver > requestSeenVer）。
func (c *Client) View(nowMs int64) View {
	v := View{Ver: c.Ver, Friends: map[string]FriendView{}, Requests: map[ReqKey]RequestView{}, Blocks: map[string]int64{}, Seen: c.RequestSeenVer, Search: c.Searchable}
	for id, f := range c.Friends {
		if !f.Deleted {
			v.Friends[id] = FriendView{f.Remark, f.Source, f.CreatedAt, f.Ver}
		}
	}
	cutoff := nowMs - retention.Milliseconds()
	for k, r := range c.Requests {
		if r.UpdatedAt < cutoff {
			continue
		}
		st := r.Status
		if st != "accepted" && r.ExpireAt <= nowMs {
			st = protocol.StatusExpired
		}
		v.Requests[k] = RequestView{st, r.Message, r.Source, r.ApplyCount, r.UpdatedAt, r.ExpireAt, r.Ver}
		if k.Direction == "in" && st == "pending" && r.Ver > c.RequestSeenVer {
			v.Unread++
		}
	}
	for id, b := range c.Blocks {
		if !b.Deleted {
			v.Blocks[id] = b.Ver
		}
	}
	return v
}
