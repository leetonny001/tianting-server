package store

import "context"

// AdminQueryStore 是管理后台第一批的只读查询（PRD-admin G-ADM-05、09、10、12）。
// 前缀匹配一律用范围查询，不用 LIKE：`_`、`%` 是合法字符，不能当通配符（AC-ADM-10-3）。
type AdminQueryStore interface {
	// SearchUsers 按 userId 前缀（区分大小写）或昵称前缀（ASCII 不区分大小写）搜索，按 userId 升序。
	// Q 为空时不按关键字过滤。next 为空表示没有更多。
	SearchUsers(ctx context.Context, q UserSearch) (users []User, next string, err error)
	// SearchGroups 按 groupId 前缀或群名前缀搜索，按创建时间倒序。Q 为空时列出全部。
	SearchGroups(ctx context.Context, q GroupSearch) (groups []GroupListItem, next *GroupCursor, err error)
	// ListGroupMembersByID 按 userId 升序分页，prefix 是 userId 前缀（区分大小写）。不检查群是否存在。
	ListGroupMembersByID(ctx context.Context, groupID, after, prefix string, limit int) (rows []GroupMemberRow, next string, err error)
	// ListMessageMeta 按 seq 倒序返回元数据，不读取、不返回正文。不受成员关系和 historyVisible 限制。
	ListMessageMeta(ctx context.Context, q MessageMetaQuery) (msgs []MessageMeta, hasMore bool, err error)
	// ListUserGroups 返回用户当前所在、没有解散的群，按 groupId 升序（G-ADM-07）。不检查用户是否存在。
	ListUserGroups(ctx context.Context, userID, after string, limit int) (groups []UserGroupRow, next string, err error)
	// ListUserConvMeta 返回用户的会话，排序和客户端会话列表相同（updatedAt 倒序，同一时间按 convId 升序）（G-ADM-13）。不检查用户是否存在。
	// 调用方不能把 Last.Text 返回出去。
	ListUserConvMeta(ctx context.Context, userID string, after *ConvCursor, limit int) (convs []ConvRow, next *ConvCursor, err error)
	// AdminStats 返回 [from, to) 内新增的用户数、消息数（不含群提示消息），以及没有解散的群总数（G-ADM-17）。
	AdminStats(ctx context.Context, from, to int64) (AdminStats, error)
}

// UserGroupRow 是用户所在的一个群和他在群里的角色、入群时间。
type UserGroupRow struct {
	Group
	Role     string
	JoinedAt int64
}

// ConvCursor 是用户会话列表的游标。
type ConvCursor struct {
	UpdatedAt int64
	ConvID    string
}

// AdminStats 是数据概览里来自数据库的部分。
type AdminStats struct {
	NewUsers int64
	Messages int64
	Groups   int64
}

// UserSearch 是用户搜索条件。After 是上一页最后一个 userId。
type UserSearch struct {
	Q            string
	OfficialOnly bool
	After        string
	Limit        int
}

// GroupCursor 是群列表的游标（created_at 倒序，同一时间按 group_id 倒序）。
type GroupCursor struct {
	CreatedAt int64
	GroupID   string
}

// GroupSearch 是群搜索条件。
type GroupSearch struct {
	Q                string
	IncludeDissolved bool
	After            *GroupCursor
	Limit            int
}

// GroupListItem 是群列表的一项。DissolvedAt 为 0 表示未解散。
type GroupListItem struct {
	Group
	DissolvedAt int64
}

// MessageMetaQuery 是消息元数据查询条件。BeforeSeq 为 0 表示从最新开始；SinceTS、UntilTS 为 0 表示不限（闭区间）。
type MessageMetaQuery struct {
	ConvID    string
	BeforeSeq int64
	From      string
	SinceTS   int64
	UntilTS   int64
	Limit     int
}

// MessageMeta 是一条消息的元数据。故意没有正文字段（AC-ADM-30-7）。BodyLength 是文本的码点数，非文本消息是 body JSON 的码点数。
// FileSize 是图片、文件消息绑定的原文件字节数（不含缩略图、不含文件名，PRD-chat v1.4 第 14.1 节，P-37 ③）；其他消息和已删除的为 0。
type MessageMeta struct {
	Seq        int64
	MsgID      string
	From       string
	Type       string
	BodyLength int
	FileSize   int64
	DeletedAt  int64
	DeletedBy  string
	TS         int64
}

// prefixEnd 返回前缀范围查询的上界（不含）：0xFF 不会出现在 UTF-8 里，比任何以 prefix 开头的字符串都大。
func prefixEnd(prefix string) string { return prefix + "\xff" }

// asciiFold 把 ASCII 字母转成小写，其他字节不变（与 SQLite lower() 一致）。
func asciiFold(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}
