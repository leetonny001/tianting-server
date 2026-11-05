// Package delivery 定义"把帧送到用户的在线连接"这件事的接口（ARCH 2.2）。
//
// 业务代码（logic、admin）只依赖这里的接口，不直接接触在线表：
// 单节点时由本机 hub 实现；集群时换成"查在线路由 + 经消息总线转发到用户所在节点"的实现，业务代码不用改。
// 投递是尽力而为的：丢了一条，客户端会靠 seq 断档 + sync 补齐，正确性不依赖投递成功。
package delivery

// Session 是业务代码能看到的一条连接：它属于哪个用户、全局唯一的连接 ID。
// 连接 ID 带节点前缀（如 "n1a2b3-17"），集群里不会重复。
type Session interface {
	ConnID() string
	UserID() string
}

// Router 把帧投递给用户在所有节点上的连接。两个方法都不能阻塞调用方。
type Router interface {
	// DeliverToUser 投递给用户的所有连接，跳过 exceptConn（空字符串表示不跳过）。
	DeliverToUser(userID string, frame []byte, exceptConn string)
	// KickUser 给用户的所有连接下发 kicked 并关闭。
	KickUser(userID, reason string)
	// SuspendUser 给用户的所有连接下发 kicked {reason: suspended, until}（永久为 nil）并关闭（封禁，F-ADM-13 规则 2）。
	SuspendUser(userID string, until *int64)
}

// Broadcaster 是可选能力：投递给全部在线连接（socialSettingsChanged，尽力而为）。
// 单节点由 hub 实现；集群时经消息总线发给每个节点。
type Broadcaster interface {
	DeliverToAll(frame []byte)
}
