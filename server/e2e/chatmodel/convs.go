package chatmodel

import "sort"

// C5 会话置顶、免打扰、删除（PRD-chat 第 9 节 F-CVS-01～04，BE-487）：会话列表的排序、convs 的 deleted 判断、本地清除、
// convSettingsChanged / convDeleted 之后的状态、总未读。和三套 SDK 的口径相同（frontend-chat 第 54、55、62、63 条）。

// Deleted 是 convs 的 deleted 一项（F-CVS-03 规则 6）。
type Deleted struct {
	ConvID     string `json:"convId"`
	ClearedSeq int64  `json:"clearedSeq"`
}

// ConvsMeta 是 convs 应答里会话以外的部分。
type ConvsMeta struct {
	Truncated        bool
	Deleted          []Deleted
	DeletedTruncated bool
}

// ConvView 是一个会话在 SDK 里和这批用例有关的状态。
type ConvView struct {
	MinSeq    int64 `json:"minSeq"`
	Unread    int64 `json:"unread"`
	Pinned    bool  `json:"pinned,omitempty"`
	PinnedAt  int64 `json:"pinnedAt,omitempty"`
	NotifyOff bool  `json:"notifyOff,omitempty"`
}

func minSeqOf(cv Conv) int64 { return max(cv.MinSeq, 1) }

// applyConvs 处理一次 convs 应答（Connect 里，重拉之前）：先做 deleted 判断（F-CVS-03 规则 6 ①，F-GRP-03 规则 4），
// 再按 minSeq 清除本地（规则 6 ②）。返回清除的消息里的 fileId（SDK 用 filesExpired 让页面丢掉缓存）。
func (c *Client) applyConvs(convs []Conv, meta ConvsMeta) []string {
	present := map[string]bool{}
	for _, cv := range convs {
		present[cv.ConvID] = true
	}
	inDeleted := map[string]bool{}
	for _, d := range meta.Deleted {
		inDeleted[d.ConvID] = true
	}
	// convs 里消失的群：在 deleted 里的是删除会话，从列表移除，不生成 removed 快照；不在里面的推断为"你已不在该群"；
	// truncated 或 deletedTruncated 时不推断（frontend-chat 第 54 条）。
	if !meta.Truncated && !meta.DeletedTruncated {
		for id := range c.list {
			if isGroup(id) && !present[id] && !inDeleted[id] {
				c.removed[id] = true
			}
		}
	}
	c.list = map[string]Conv{}
	for _, cv := range convs {
		c.list[cv.ConvID] = cv
		delete(c.removed, cv.ConvID) // 重新入群
	}
	var files []string
	for _, d := range meta.Deleted {
		files = append(files, c.clearBelow(d.ConvID, d.ClearedSeq+1)...)
	}
	for _, cv := range convs {
		files = append(files, c.clearBelow(cv.ConvID, minSeqOf(cv))...)
	}
	return files
}

func isGroup(convID string) bool { return len(convID) > 6 && convID[:6] == "group:" }

// clearBelow 物理清除 seq < minSeq 的已加载消息（F-CVS-03 规则 6 ②），记下下限，之后迟到的应答不写回。
// 保留下来的消息里引用被清除消息的摘要照常显示（P-46 ④，frontend-chat 第 62 条）。会话全部清空时不再算已加载（不重拉）。
func (c *Client) clearBelow(convID string, minSeq int64) []string {
	if minSeq > c.floor[convID] {
		c.floor[convID] = minSeq
	}
	list := c.loaded[convID]
	var files []string
	for seq, m := range list {
		if seq < minSeq {
			if m.FileID != "" && !m.Expired {
				files = append(files, m.FileID)
			}
			delete(list, seq)
		}
	}
	if list != nil && len(list) == 0 {
		delete(c.loaded, convID)
		delete(c.baseline, convID)
	}
	return files
}

// applySettings 是 convSettingsChanged（F-CVS-04 规则 1）：覆盖本地的置顶、免打扰。
func (c *Client) applySettings(ev Event) {
	cv, ok := c.list[ev.ConvID]
	if !ok {
		return
	}
	cv.Pinned, cv.PinnedAt = ev.Pinned != nil && *ev.Pinned, ev.PinnedAt
	if !cv.Pinned {
		cv.PinnedAt = 0
	}
	cv.NotifyOff = ev.NotifyOff != nil && *ev.NotifyOff
	c.list[ev.ConvID] = cv
}

// applyConvDeleted 是 convDeleted（F-CVS-03 规则 2、3，backend-chat 第 84 条）：minSeq = clearedSeq + 1，已读推进到 clearedSeq，
// 不再置顶，保留免打扰，按 minSeq 清除；没有更新的消息（maxSeq ≤ clearedSeq）时从列表移除（不是"你已不在该群"）。
func (c *Client) applyConvDeleted(ev Event) []string {
	cleared := *ev.ClearedSeq
	files := c.clearBelow(ev.ConvID, cleared+1)
	cv, ok := c.list[ev.ConvID]
	if !ok {
		return files
	}
	if cv.MaxSeq <= cleared {
		delete(c.list, ev.ConvID)
		return files
	}
	cv.MinSeq = max(minSeqOf(cv), cleared+1)
	cv.Unread = min(cv.Unread, cv.MaxSeq-cleared)
	cv.Pinned, cv.PinnedAt = false, 0
	c.list[ev.ConvID] = cv
	return files
}

// Conversations 是会话列表的顺序（F-CVS-01 规则 3）：置顶的在前（pinnedAt 倒序），其余 updatedAt 倒序，时间相同按 convId。
func (c *Client) Conversations() []string {
	out := make([]Conv, 0, len(c.list))
	for _, cv := range c.list {
		out = append(out, cv)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Pinned != b.Pinned {
			return a.Pinned
		}
		if a.Pinned && a.PinnedAt != b.PinnedAt {
			return a.PinnedAt > b.PinnedAt
		}
		if a.UpdatedAt != b.UpdatedAt {
			return a.UpdatedAt > b.UpdatedAt
		}
		return a.ConvID < b.ConvID
	})
	ids := make([]string, len(out))
	for i, cv := range out {
		ids[i] = cv.ConvID
	}
	return ids
}

// Views 返回列表里每个会话的状态。
func (c *Client) Views() map[string]ConvView {
	out := make(map[string]ConvView, len(c.list))
	for id, cv := range c.list {
		out[id] = ConvView{MinSeq: minSeqOf(cv), Unread: cv.Unread, Pinned: cv.Pinned, PinnedAt: cv.PinnedAt, NotifyOff: cv.NotifyOff}
	}
	return out
}

// Removed 返回推断为"你已不在该群"的群（排好序）。
func (c *Client) Removed() []string {
	out := make([]string, 0, len(c.removed))
	for id := range c.removed {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// TotalUnread 是总未读（F-CVS-02 规则 4）：普通会话的 unread 之和 + 免打扰会话的 mentionUnread 之和。
func (c *Client) TotalUnread() int64 {
	var n int64
	for id, cv := range c.list {
		if cv.NotifyOff {
			n += c.mentions[id]
		} else {
			n += cv.Unread
		}
	}
	return n
}
