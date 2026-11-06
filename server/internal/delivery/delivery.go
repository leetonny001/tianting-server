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
	// SID、Gen 是这条连接用的 token 的设备会话和代号（PRD-chat 第 11.4 节，RS5-08）。没有 sid 的 token 是空串和 0。
	SID() string
	Gen() int64
}

// Router 把帧投递给用户在所有节点上的连接。两个方法都不能阻塞调用方。
type Router interface {
	// DeliverToUser 投递给用户的所有连接，跳过 exceptConn（空字符串表示不跳过）。返回本节点接受投递的连接数（只用于指标）。
	DeliverToUser(userID string, frame []byte, exceptConn string) int
	// DeliverToUserExceptSID 投递给用户除这条设备会话之外的所有连接（securityNotice 的"其他设备"，发起方可能是 HTTP 请求，没有连接 ID）。
	// exceptSID 为空表示不跳过。返回值同 DeliverToUser。
	DeliverToUserExceptSID(userID string, frame []byte, exceptSID string) int
	// KickUser 给用户的所有连接下发 kicked 并关闭。
	KickUser(userID, reason string)
	// SuspendUser 给用户的所有连接下发 kicked {reason: suspended, until}（永久为 nil）并关闭（封禁，F-ADM-13 规则 2）。
	SuspendUser(userID string, until *int64)
	// KickSession 踢这条设备会话的全部连接（PRD-chat 第 11.4 节）。集群时走总线；userID 只用来找连接。
	KickSession(userID, sid, reason string, recoverBefore *int64)
	// KickStaleGen 踢这条 sid 上代号不是 currentGen 的连接（转正，F-ACC-03 规则 4）：kicked/tokenExpired。
	KickStaleGen(userID, sid string, currentGen int64)
	// KickUserExcept 踢用户除 exceptSID 之外的全部连接（改密、找回、退出其他设备、注销：吊销之后新 sid 可能已经连上）。
	KickUserExcept(userID, exceptSID, reason string, recoverBefore *int64)
}

// Broadcaster 是投递给全部在线连接的能力（socialSettingsChanged 和全员推送，尽力而为）。
// 单节点由 hub 实现；集群时经消息总线发给每个节点。socialSettingsChanged 把它当可选能力；
// 系统推送开启时缺少它就启动失败（PRD-live F-PUSH-04 规则 2）。
type Broadcaster interface {
	// DeliverToAll 不阻塞调用方，返回本节点接受投递的连接数（只用于指标）。
	DeliverToAll(frame []byte) int
}
