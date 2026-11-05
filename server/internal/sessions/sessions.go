// Package sessions 是"某个用户当前有哪些连接"的唯一查询入口：管理接口和运维接口都调用 Query，
// 字段和脱敏规则因此只有一份（PRD-admin F-ADM-14 规则 2，评审 X-05）。
package sessions

import (
	"cmp"
	"fmt"
	"net/netip"
	"slices"
	"strings"
)

// Conn 是一条连接的元数据。IP 只有脱敏后的值：完整 IP 不进连接元数据。
type Conn struct {
	ConnID      string `json:"connId"`
	Platform    string `json:"platform"`
	ConnectedAt int64  `json:"connectedAt"`
	Node        string `json:"node"`
	IP          string `json:"ip"`
	QueueLen    int    `json:"queueLen"`
}

// Source 是连接的来源（本节点的 hub；B6 之后由 Presence 汇总各节点）。
type Source interface {
	ListUserConns(userID string) []Conn
	ConnCount(userID string) int
	// PlatformCounts 返回按平台的连接数，没有连接的平台不出现（数据概览，F-ADM-70）。
	PlatformCounts() map[string]int
}

// ScopeLocalNode 表示结果只覆盖接收请求的节点（B6 之前，F-ADM-14 规则 5）。
const ScopeLocalNode = "localNode"

// Result 是查询结果。Partial=true 时不能据此得出"不在线"的结论。
type Result struct {
	Sessions []Conn `json:"sessions"`
	Partial  bool   `json:"partial"`
	Scope    string `json:"scope"`
}

// Query 返回用户的连接，按建立时间排序。
func Query(src Source, userID string) Result {
	conns := src.ListUserConns(userID)
	if conns == nil {
		conns = []Conn{}
	}
	slices.SortFunc(conns, func(a, b Conn) int {
		return cmp.Or(cmp.Compare(a.ConnectedAt, b.ConnectedAt), cmp.Compare(a.ConnID, b.ConnID))
	})
	return Result{Sessions: conns, Partial: true, Scope: ScopeLocalNode}
}

// MaskIP 脱敏来源 IP：IPv4 保留前 2 段（10.1.*.*），IPv6 保留前 3 组（48 位），其余为 *。解析不了时返回 *。
func MaskIP(ip string) string {
	a, err := netip.ParseAddr(ip)
	if err != nil {
		return "*"
	}
	a = a.Unmap().WithZone("")
	if a.Is4() {
		b := a.As4()
		return fmt.Sprintf("%d.%d.*.*", b[0], b[1])
	}
	b := a.As16()
	groups := make([]string, 8)
	for i := range groups {
		groups[i] = "*"
		if i < 3 {
			groups[i] = fmt.Sprintf("%x", uint16(b[2*i])<<8|uint16(b[2*i+1]))
		}
	}
	return strings.Join(groups, ":")
}
