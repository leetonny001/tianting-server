// Package store 定义持久化接口。MVP 用 SQLite 实现，后续可换 Postgres。
package store

import (
	"context"
	"encoding/json"
	"errors"
)

// 存储层错误。
var (
	ErrNotFound = errors.New("not found")
	ErrExists   = errors.New("already exists")
)

// Message 是持久化的一条消息。Body 是原样保存的 JSON。
type Message struct {
	MsgID       string
	ClientMsgID string
	ConvID      string
	Seq         int64
	From        string
	Type        string
	Body        json.RawMessage
	TS          int64
}

// Store 是 IM 服务端用到的全部持久化操作。
type Store interface {
	// SaveMessage 为消息分配会话内递增 seq 并保存。
	// 同一发送者重复提交同一 ClientMsgID 时不新建，返回第一次保存的消息且 dup=true。
	SaveMessage(ctx context.Context, m Message) (saved Message, dup bool, err error)
	// ListMessages 返回 seq > afterSeq 的消息（升序，最多 limit 条），以及后面是否还有。
	ListMessages(ctx context.Context, convID string, afterSeq int64, limit int) (msgs []Message, hasMore bool, err error)

	CreateGroup(ctx context.Context, groupID, name string, members []string) error
	AddGroupMembers(ctx context.Context, groupID string, members []string) error
	GroupMembers(ctx context.Context, groupID string) ([]string, error)
	IsGroupMember(ctx context.Context, groupID, userID string) (bool, error)

	// SetReadSeq 记录已读位置，只前进不后退。
	SetReadSeq(ctx context.Context, userID, convID string, seq int64) error
	ReadSeq(ctx context.Context, userID, convID string) (int64, error)

	Close() error
}
