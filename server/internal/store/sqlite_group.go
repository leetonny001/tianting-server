package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

// 群管理的 SQLite 实现。每个方法一个事务：读群（L5：Postgres 上这里 SELECT … FOR UPDATE）→ 权限 / 人数 / 群主判断 → 修改 →
// 提示消息在同一事务分配 seq（群锁 → conv_seq），任何一步失败整体回滚（F-SOC-29 规则 4）。

const groupColumns = `group_id, name, history_visible, created_at, member_count, owner_id, created_by, notice, notice_by, notice_at, max_members`

func scanGroup(r rowScanner) (Group, error) {
	var g Group
	err := r.Scan(&g.GroupID, &g.Name, &g.HistoryVisible, &g.CreatedAt, &g.MemberCount, &g.OwnerID, &g.CreatedBy, &g.Notice, &g.NoticeBy, &g.NoticeAt, &g.MaxMembers)
	return g, err
}

// lockGroup 读出未解散的群；不存在或已解散返回 ErrNotFound。
func lockGroup(ctx context.Context, tx *sql.Tx, groupID string) (Group, error) {
	g, err := scanGroup(tx.QueryRowContext(ctx, `SELECT `+groupColumns+` FROM chat_groups WHERE group_id = ? AND dissolved_at = 0`, groupID))
	if errors.Is(err, sql.ErrNoRows) {
		return Group{}, ErrNotFound
	}
	return g, err
}

func memberRole(ctx context.Context, tx *sql.Tx, groupID, userID string) (string, bool, error) {
	var role string
	err := tx.QueryRowContext(ctx, `SELECT role FROM group_members WHERE group_id = ? AND user_id = ?`, groupID, userID).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return role, err == nil, err
}

// authorize 锁群、确认 actor 是成员，并按权限表判定（op < 0 表示所有成员都可以）。
func authorizeGroup(ctx context.Context, tx *sql.Tx, groupID, actor string, op GroupOp, check bool) (Group, string, error) {
	g, err := lockGroup(ctx, tx, groupID)
	if err != nil {
		return g, "", err
	}
	role, ok, err := memberRole(ctx, tx, groupID, actor)
	if err != nil {
		return g, "", err
	}
	if !ok {
		return g, "", ErrNotGroupMember
	}
	if check && !GroupCan(op, role, g) {
		return g, role, ErrGroupForbidden
	}
	return g, role, nil
}

// failTipWrite 是测试注入点：提示消息写入失败时整个操作回滚（AC-SOC-29-8）。
var failTipWrite func() error

// nameOf 是服务端生成文本用的名字：昵称，没有时 userId，绝不用备注（5.5 节）。
func nameOf(ctx context.Context, tx *sql.Tx, uid string) (string, error) {
	var n string
	err := tx.QueryRowContext(ctx, `SELECT nickname FROM users WHERE user_id = ?`, uid).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && n == "") {
		return uid, nil
	}
	return n, err
}

const maxTipTargets = 20 // F-SOC-29 规则 7

type tipBody struct {
	Tip tipData `json:"tip"`
}

type tipData struct {
	Kind    string   `json:"kind"`
	Actor   string   `json:"actor"`
	Targets []string `json:"targets"`
	From    *string  `json:"from,omitempty"` // ownerChanged：原群主（系统操作时不带）
	Count   int      `json:"count,omitempty"`
	Text    string   `json:"text"`
}

func truncRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// writeTip 在同一事务里分配 seq、写提示消息；from 非空时推进操作人的已读位置（发言即已读）。
func writeTip(ctx context.Context, tx *sql.Tx, groupID, from, kind string, targets []string, extra string, now int64) (*Message, error) {
	if failTipWrite != nil {
		if err := failTipWrite(); err != nil {
			return nil, err
		}
	}
	summary, err := tipSummary(ctx, tx, from, kind, targets, extra)
	if err != nil {
		return nil, err
	}
	shown := targets
	count := 0
	if len(shown) > maxTipTargets {
		count = len(shown)
		shown = shown[:maxTipTargets]
	}
	if shown == nil {
		shown = []string{}
	}
	text := summary
	if kind == "renamed" || kind == "noticeUpdated" {
		text = extra // renamed：新群名；noticeUpdated：公告前 100 码点
	}
	body, _ := json.Marshal(tipBody{Tip: tipData{Kind: kind, Actor: from, Targets: shown, Count: count, Text: text}})
	m := Message{
		MsgID:       uuid.Must(uuid.NewV7()).String(),
		ClientMsgID: "tip-" + uuid.Must(uuid.NewV7()).String(),
		ConvID:      groupConvID(groupID),
		From:        from,
		Type:        "tip",
		Body:        body,
		TS:          now,
	}
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO conv_seq (conv_id, max_seq, last_msg_id, last_from, last_type, last_text, last_ts)
		 VALUES (?, 1, ?, ?, 'tip', ?, ?)
		 ON CONFLICT (conv_id) DO UPDATE SET max_seq = max_seq + 1,
		   last_msg_id = excluded.last_msg_id, last_from = excluded.last_from, last_type = excluded.last_type,
		   last_text = excluded.last_text, last_ts = excluded.last_ts, last_deleted = 0
		 RETURNING max_seq`, m.ConvID, m.MsgID, from, truncRunes(summary, 100), now).Scan(&m.Seq); err != nil {
		return nil, fmt.Errorf("allocate tip seq: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO messages (msg_id, conv_id, seq, from_uid, client_msg_id, type, body, ts) VALUES (?, ?, ?, ?, ?, 'tip', ?, ?)`,
		m.MsgID, m.ConvID, m.Seq, from, m.ClientMsgID, string(body), now); err != nil {
		return nil, fmt.Errorf("insert tip: %w", err)
	}
	if from != "" {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO read_seq (user_id, conv_id, seq) VALUES (?, ?, ?)
			 ON CONFLICT (user_id, conv_id) DO UPDATE SET seq = MAX(seq, excluded.seq)`, from, m.ConvID, m.Seq); err != nil {
			return nil, err
		}
	}
	return &m, nil
}

// tipSummary 生成 lastMsg 用的摘要（只用昵称，5.5 节）。
func tipSummary(ctx context.Context, tx *sql.Tx, from, kind string, targets []string, extra string) (string, error) {
	actor := ""
	if from != "" {
		var err error
		if actor, err = nameOf(ctx, tx, from); err != nil {
			return "", err
		}
	}
	names := make([]string, 0, 3)
	for _, t := range targets[:min(len(targets), 3)] {
		n, err := nameOf(ctx, tx, t)
		if err != nil {
			return "", err
		}
		names = append(names, n)
	}
	list := strings.Join(names, "、")
	if len(targets) > 3 {
		list = fmt.Sprintf("%s等 %d 人", list, len(targets))
	}
	switch kind {
	case "created", "joined":
		return fmt.Sprintf("%s 邀请 %s 加入了群聊", actor, list), nil
	case "ownerChanged":
		if len(targets) == 0 {
			return "群主已退出群聊", nil
		}
		return fmt.Sprintf("%s 已成为群主", list), nil
	case "adminSet":
		return fmt.Sprintf("%s 已成为管理员", list), nil
	case "adminUnset":
		return fmt.Sprintf("%s 不再是管理员", list), nil
	case "renamed":
		return fmt.Sprintf("%s 修改群名为\"%s\"", actor, extra), nil
	case "noticeUpdated":
		return fmt.Sprintf("%s 更新了群公告", actor), nil
	}
	return "[群通知]", nil
}

// checkInvitee 判定被邀请人（F-SOC-21 规则 3）：存在、互为好友、没拉黑邀请人，否则 ErrInviteeNotAllowed（三种情况同码）；群数已满 ErrJoinedGroupLimit。
func checkInvitee(ctx context.Context, tx *sql.Tx, actor, uid string, maxJoined int) error {
	var ok bool
	err := tx.QueryRowContext(ctx, `SELECT
		EXISTS (SELECT 1 FROM users WHERE user_id = ?2)
		AND EXISTS (SELECT 1 FROM friends WHERE owner_id = ?1 AND peer_id = ?2 AND deleted = 0)
		AND EXISTS (SELECT 1 FROM friends WHERE owner_id = ?2 AND peer_id = ?1 AND deleted = 0)
		AND NOT EXISTS (SELECT 1 FROM blocks WHERE owner_id = ?2 AND peer_id = ?1 AND deleted = 0)`, actor, uid).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return ErrInviteeNotAllowed
	}
	return checkJoined(ctx, tx, uid, maxJoined)
}

// checkJoined 是软上限：只在客户端建群 / 邀请时 count(*)，并发时可能少量超出（F-SOC-21 规则 4）。
func checkJoined(ctx context.Context, tx *sql.Tx, uid string, maxJoined int) error {
	var n int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM group_members WHERE user_id = ?`, uid).Scan(&n); err != nil {
		return err
	}
	if n >= maxJoined {
		return ErrJoinedGroupLimit
	}
	return nil
}

func (s *SQLite) CreateClientGroup(ctx context.Context, p CreateClientGroupParams) (string, bool, *Message, string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", false, nil, "", err
	}
	defer tx.Rollback()
	var existing string
	err = tx.QueryRowContext(ctx, `SELECT group_id FROM client_group_req WHERE user_id = ? AND client_req_id = ?`, p.UserID, p.ClientReqID).Scan(&existing)
	if err == nil {
		return existing, true, nil, "", tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", false, nil, "", err
	}
	if p.QuotaDenied {
		return "", false, nil, "", ErrCreateQuota
	}
	if err := checkJoined(ctx, tx, p.UserID, p.MaxJoined); err != nil {
		return "", false, nil, p.UserID, err
	}
	for _, m := range p.Members {
		if err := checkInvitee(ctx, tx, p.UserID, m, p.MaxJoined); err != nil {
			return "", false, nil, m, err
		}
	}
	name := p.Name
	if name == "" { // 前 3 个成员（含自己，按 members 顺序）的昵称，超过 50 码点截断
		parts := make([]string, 0, 3)
		for _, u := range append([]string{p.UserID}, p.Members...)[:min(len(p.Members)+1, 3)] {
			n, err := nameOf(ctx, tx, u)
			if err != nil {
				return "", false, nil, "", err
			}
			parts = append(parts, n)
		}
		name = truncRunes(strings.Join(parts, "、"), 50)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO chat_groups (group_id, name, history_visible, created_at, owner_id, created_by, max_members, member_count)
		 VALUES (?, ?, 0, ?, ?, ?, ?, ?)`, p.GroupID, name, p.Now, p.UserID, p.UserID, p.MaxMembers, len(p.Members)+1); err != nil {
		return "", false, nil, "", err
	}
	if _, err := insertMembers(ctx, tx, p.GroupID, append([]string{p.UserID}, p.Members...), p.Now); err != nil {
		return "", false, nil, "", err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE group_members SET role = 'owner', role_at = ? WHERE group_id = ? AND user_id = ?`, p.Now, p.GroupID, p.UserID); err != nil {
		return "", false, nil, "", err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO client_group_req (user_id, client_req_id, group_id, created_at) VALUES (?, ?, ?, ?)`,
		p.UserID, p.ClientReqID, p.GroupID, p.Now); err != nil {
		return "", false, nil, "", err
	}
	tip, err := writeTip(ctx, tx, p.GroupID, p.UserID, "created", p.Members, "", p.Now)
	if err != nil {
		return "", false, nil, "", err
	}
	return p.GroupID, false, tip, "", tx.Commit()
}

func (s *SQLite) InviteMembers(ctx context.Context, groupID, actor string, members []string, maxJoined int, now int64) ([]string, []InviteFailure, *Message, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, nil, err
	}
	defer tx.Rollback()
	g, _, err := authorizeGroup(ctx, tx, groupID, actor, OpInvite, true)
	if err != nil {
		return nil, nil, nil, err
	}
	var toAdd []string
	var failed []InviteFailure
	for _, m := range members {
		if _, ok, err := memberRole(ctx, tx, groupID, m); err != nil {
			return nil, nil, nil, err
		} else if ok {
			continue // 已经是成员：不进 added、不进 failed，不重置 joinSeq（C-36）
		}
		switch err := checkInvitee(ctx, tx, actor, m, maxJoined); {
		case err == nil:
			toAdd = append(toAdd, m)
		case errors.Is(err, ErrInviteeNotAllowed), errors.Is(err, ErrJoinedGroupLimit):
			failed = append(failed, InviteFailure{m, err})
		default:
			return nil, nil, nil, err
		}
	}
	if len(toAdd) == 0 {
		return nil, failed, nil, tx.Commit()
	}
	// 先按当前 maxSeq 记 joinSeq，再分配提示消息的 seq：新成员 minSeq 处能看到自己的入群提示，未读 1（AC-SOC-22-5）。
	added, err := insertMembers(ctx, tx, groupID, toAdd, now)
	if err != nil {
		return nil, nil, nil, err
	}
	if g.MemberCount+len(added) > g.MaxMembers {
		return nil, nil, nil, ErrLimitExceeded // 回滚：一人都不加
	}
	if err := updateMemberCount(ctx, tx, groupID); err != nil {
		return nil, nil, nil, err
	}
	tip, err := writeTip(ctx, tx, groupID, actor, "joined", added, "", now)
	if err != nil {
		return nil, nil, nil, err
	}
	return added, failed, tip, tx.Commit()
}

func (s *SQLite) KickMembers(ctx context.Context, groupID, actor string, members []string) ([]string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	_, role, err := authorizeGroup(ctx, tx, groupID, actor, OpKick, true)
	if err != nil {
		return nil, err
	}
	var removed []string
	for _, m := range members {
		r, ok, err := memberRole(ctx, tx, groupID, m)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		if r == RoleOwner || (r == RoleAdmin && role != RoleOwner) { // 不能移出群主；管理员只能移出成员
			return nil, ErrGroupForbidden
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM group_members WHERE group_id = ? AND user_id = ?`, groupID, m); err != nil {
			return nil, err
		}
		if err := deleteGroupMutesTx(ctx, tx, groupID, m); err != nil {
			return nil, err
		}
		if err := deleteConvSettingsTx(ctx, tx, groupID, m); err != nil {
			return nil, err
		}
		removed = append(removed, m)
	}
	if err := updateMemberCount(ctx, tx, groupID); err != nil {
		return nil, err
	}
	return removed, tx.Commit()
}

// afterOwnerGone 在群主已经被删除之后处理（F-SOC-23 规则 4、5）：自建群转给最早的管理员 / 最早入群的成员，
// 没有人了就解散；托管群回到无群主。
func afterOwnerGone(ctx context.Context, tx *sql.Tx, g Group, now int64) (LeaveResult, error) {
	if g.Managed() {
		if _, err := tx.ExecContext(ctx, `UPDATE chat_groups SET owner_id = '' WHERE group_id = ?`, g.GroupID); err != nil {
			return LeaveResult{}, err
		}
		tip, err := writeTip(ctx, tx, g.GroupID, "", "ownerChanged", nil, "", now)
		return LeaveResult{Tip: tip}, err
	}
	var next string
	err := tx.QueryRowContext(ctx,
		`SELECT user_id FROM group_members WHERE group_id = ?
		 ORDER BY role = 'admin' DESC, CASE WHEN role = 'admin' THEN role_at ELSE joined_at END, user_id LIMIT 1`, g.GroupID).Scan(&next)
	if errors.Is(err, sql.ErrNoRows) {
		return LeaveResult{Dissolved: true}, dissolveTx(ctx, tx, g.GroupID, now)
	}
	if err != nil {
		return LeaveResult{}, err
	}
	if err := setOwnerTx(ctx, tx, g.GroupID, next, now); err != nil {
		return LeaveResult{}, err
	}
	tip, err := writeTip(ctx, tx, g.GroupID, "", "ownerChanged", []string{next}, "", now)
	return LeaveResult{Tip: tip}, err
}

func setOwnerTx(ctx context.Context, tx *sql.Tx, groupID, owner string, now int64) error {
	if _, err := tx.ExecContext(ctx, `UPDATE group_members SET role = 'member', role_at = 0 WHERE group_id = ? AND role = 'owner'`, groupID); err != nil {
		return err
	}
	if owner != "" {
		if _, err := tx.ExecContext(ctx, `UPDATE group_members SET role = 'owner', role_at = ? WHERE group_id = ? AND user_id = ?`, now, groupID, owner); err != nil {
			return err
		}
	}
	_, err := tx.ExecContext(ctx, `UPDATE chat_groups SET owner_id = ? WHERE group_id = ?`, owner, groupID)
	return err
}

func (s *SQLite) LeaveGroup(ctx context.Context, groupID, actor string, now int64) (LeaveResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return LeaveResult{}, err
	}
	defer tx.Rollback()
	g, role, err := authorizeGroup(ctx, tx, groupID, actor, 0, false)
	if err != nil {
		return LeaveResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM group_members WHERE group_id = ? AND user_id = ?`, groupID, actor); err != nil {
		return LeaveResult{}, err
	}
	if err := deleteGroupMutesTx(ctx, tx, groupID, actor); err != nil {
		return LeaveResult{}, err
	}
	if err := deleteConvSettingsTx(ctx, tx, groupID, actor); err != nil {
		return LeaveResult{}, err
	}
	if err := updateMemberCount(ctx, tx, groupID); err != nil {
		return LeaveResult{}, err
	}
	var res LeaveResult
	if role == RoleOwner {
		if res, err = afterOwnerGone(ctx, tx, g, now); err != nil {
			return LeaveResult{}, err
		}
	} else if !g.Managed() && g.MemberCount <= 1 {
		res.Dissolved = true
		if err := dissolveTx(ctx, tx, groupID, now); err != nil {
			return LeaveResult{}, err
		}
	}
	return res, tx.Commit()
}

func (s *SQLite) TransferOwner(ctx context.Context, groupID, actor, to string, now int64) (*Message, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, _, err := authorizeGroup(ctx, tx, groupID, actor, OpManage, true); err != nil {
		return nil, err
	}
	if _, ok, err := memberRole(ctx, tx, groupID, to); err != nil {
		return nil, err
	} else if !ok {
		return nil, ErrTargetNotMember
	}
	if err := setOwnerTx(ctx, tx, groupID, to, now); err != nil {
		return nil, err
	}
	tip, err := writeTip(ctx, tx, groupID, actor, "ownerChanged", []string{to}, "", now)
	if err != nil {
		return nil, err
	}
	return tip, tx.Commit()
}

func (s *SQLite) SetGroupRole(ctx context.Context, groupID, actor, target string, admin bool, now int64) (*Message, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, _, err := authorizeGroup(ctx, tx, groupID, actor, OpManage, true); err != nil {
		return nil, err
	}
	role, ok, err := memberRole(ctx, tx, groupID, target)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrTargetNotMember
	}
	if (role == RoleAdmin) == admin {
		return nil, tx.Commit() // 幂等：不生成提示消息
	}
	next, kind := RoleMember, "adminUnset"
	at := int64(0)
	if admin {
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM group_members WHERE group_id = ? AND role = 'admin'`, groupID).Scan(&n); err != nil {
			return nil, err
		}
		if n >= MaxGroupAdmins {
			return nil, ErrAdminLimit
		}
		next, kind, at = RoleAdmin, "adminSet", now
	}
	if _, err := tx.ExecContext(ctx, `UPDATE group_members SET role = ?, role_at = ? WHERE group_id = ? AND user_id = ?`, next, at, groupID, target); err != nil {
		return nil, err
	}
	tip, err := writeTip(ctx, tx, groupID, actor, kind, []string{target}, "", now)
	if err != nil {
		return nil, err
	}
	return tip, tx.Commit()
}

// dissolveTx 是解散的实现（F-SOC-26 规则 2）：DissolveGroup、客户端解散、最后一人退群共用。
func dissolveTx(ctx context.Context, tx *sql.Tx, groupID string, now int64) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM group_members WHERE group_id = ?`, groupID); err != nil {
		return err
	}
	if err := deleteGroupMutesTx(ctx, tx, groupID, ""); err != nil {
		return err
	}
	if err := deleteConvSettingsTx(ctx, tx, groupID, ""); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `UPDATE chat_groups SET dissolved_at = ?, member_count = 0 WHERE group_id = ?`, max(now, 1), groupID)
	return err
}

func (s *SQLite) ClientDissolveGroup(ctx context.Context, groupID, actor string, now int64) ([]string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, _, err := authorizeGroup(ctx, tx, groupID, actor, OpDissolve, true); err != nil {
		return nil, err
	}
	members, err := dissolveMembersTx(ctx, tx, groupID, now)
	if err != nil {
		return nil, err
	}
	return members, tx.Commit()
}

func (s *SQLite) UpdateGroupProfile(ctx context.Context, groupID, actor string, name, notice *string, now int64) ([]Message, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, _, err := authorizeGroup(ctx, tx, groupID, actor, OpUpdate, true); err != nil {
		return nil, err
	}
	var tips []Message
	if name != nil {
		if _, err := tx.ExecContext(ctx, `UPDATE chat_groups SET name = ? WHERE group_id = ?`, *name, groupID); err != nil {
			return nil, err
		}
		tip, err := writeTip(ctx, tx, groupID, actor, "renamed", nil, *name, now)
		if err != nil {
			return nil, err
		}
		tips = append(tips, *tip)
	}
	if notice != nil {
		if _, err := tx.ExecContext(ctx, `UPDATE chat_groups SET notice = ?, notice_by = ?, notice_at = ? WHERE group_id = ?`, *notice, actor, now, groupID); err != nil {
			return nil, err
		}
		tip, err := writeTip(ctx, tx, groupID, actor, "noticeUpdated", nil, truncRunes(*notice, 100), now)
		if err != nil {
			return nil, err
		}
		tips = append(tips, *tip)
	}
	return tips, tx.Commit()
}

func (s *SQLite) ListGroupMembers(ctx context.Context, groupID string, after *MemberKey, limit int) ([]GroupMemberRow, *MemberKey, error) {
	return s.ListGroupMembersMatching(ctx, groupID, "", after, limit)
}

func (s *SQLite) ListGroupMembersMatching(ctx context.Context, groupID, keyword string, after *MemberKey, limit int) ([]GroupMemberRow, *MemberKey, error) {
	var conds []string
	args := []any{groupID}
	if keyword != "" {
		// 前缀范围查询，和管理端搜索同一个写法（userId 区分大小写，昵称用 nickname_fold）。
		fold := asciiFold(keyword)
		conds = append(conds, `((user_id >= ? AND user_id < ?) OR (nickname_fold >= ? AND nickname_fold < ?))`)
		args = append(args, keyword, prefixEnd(keyword), fold, prefixEnd(fold))
	}
	if after != nil {
		conds = append(conds, `(rnk, t, user_id) > (?, ?, ?)`)
		args = append(args, after.Rank, after.T, after.User)
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}
	args = append(args, limit+1)
	// #nosec G202 -- where 是常量片段，取值全部走占位符
	rows, err := s.db.QueryContext(ctx, `SELECT user_id, role, joined_at, role_at, nickname, avatar, official, rnk, t FROM (
		SELECT gm.user_id, gm.role, gm.joined_at, gm.role_at,
		       COALESCE(u.nickname, '') AS nickname, COALESCE(u.avatar, '') AS avatar, COALESCE(u.official, 0) AS official,
		       COALESCE(u.nickname_fold, '') AS nickname_fold,
		       CASE gm.role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 ELSE 2 END AS rnk,
		       CASE gm.role WHEN 'admin' THEN gm.role_at ELSE gm.joined_at END AS t
		FROM group_members gm LEFT JOIN users u ON u.user_id = gm.user_id WHERE gm.group_id = ?
	)`+where+` ORDER BY rnk, t, user_id LIMIT ?`, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var out []GroupMemberRow
	var keys []MemberKey
	for rows.Next() {
		var r GroupMemberRow
		var k MemberKey
		if err := rows.Scan(&r.UserID, &r.Role, &r.JoinedAt, &r.RoleAt, &r.Nickname, &r.Avatar, &r.Official, &k.Rank, &k.T); err != nil {
			return nil, nil, err
		}
		k.User = r.UserID
		out = append(out, r)
		keys = append(keys, k)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	if len(out) > limit {
		next := keys[limit-1]
		return out[:limit], &next, nil
	}
	return out, nil, nil
}

func (s *SQLite) SetGroupOwner(ctx context.Context, groupID, owner string, now int64) (*Message, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	g, err := lockGroup(ctx, tx, groupID)
	if err != nil {
		return nil, err
	}
	if owner != "" {
		if _, ok, err := memberRole(ctx, tx, groupID, owner); err != nil {
			return nil, err
		} else if !ok {
			return nil, ErrTargetNotMember
		}
	}
	if g.OwnerID == owner {
		return nil, tx.Commit()
	}
	if err := setOwnerTx(ctx, tx, groupID, owner, now); err != nil {
		return nil, err
	}
	var targets []string
	if owner != "" {
		targets = []string{owner}
	}
	tip, err := writeTip(ctx, tx, groupID, "", "ownerChanged", targets, "", now)
	if err != nil {
		return nil, err
	}
	return tip, tx.Commit()
}

func (s *SQLite) PurgeGroupReqs(ctx context.Context, before int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM client_group_req WHERE created_at < ?`, before)
	return err
}
