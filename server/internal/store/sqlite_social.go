package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// 社交关系的 SQLite 实现（PRD-social M3，BE-101、BE-106～BE-115）。
//
// 加锁（8.3 节）：SQLite 单连接，事务天然串行，lockUsers 只负责建出 user_social 行并按字节序记录顺序；
// Postgres 实现在同样的位置用 SELECT … FOR UPDATE。所有修改都经 socialTx：每个用户在一个事务里只分配一个 ver。

type socialTx struct {
	ctx   context.Context
	tx    *sql.Tx
	now   int64
	vers  map[string]int64 // 本事务已分配的 ver
	locks []lockEvent      // 本事务的加锁序列（加锁顺序断言）
}

// 加锁对象的种类，按 8.3 节全局顺序：群 → 用户 → conv_seq。
const (
	lockKindGroup = 1
	lockKindUser  = 2
	lockKindConv  = 3
)

type lockEvent struct {
	kind int
	id   string
}

// onLockOrderViolation 不为 nil 时，事务的加锁序列违反 8.3 节全局顺序就回调（测试注入；生产为 nil，不做任何事）。
// SQLite 单连接下顺序写错不会死锁，只有这个断言能发现；Postgres 上同样的错误会变成死锁。
var onLockOrderViolation func(seq string)

// lock 记录一次加锁并检查顺序：种类不能倒退；同一种类内 id 严格递增（字节序）。
func (t *socialTx) lock(kind int, id string) {
	if n := len(t.locks); n > 0 && onLockOrderViolation != nil {
		last := t.locks[n-1]
		if kind < last.kind || kind == last.kind && id <= last.id {
			onLockOrderViolation(fmt.Sprintf("%v then {%d %s}", t.locks, kind, id))
		}
	}
	t.locks = append(t.locks, lockEvent{kind, id})
}

func (s *SQLite) beginSocial(ctx context.Context, now int64) (*socialTx, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &socialTx{ctx: ctx, tx: tx, now: now, vers: map[string]int64{}}, nil
}

func (t *socialTx) exec(q string, args ...any) (sql.Result, error) {
	return t.tx.ExecContext(t.ctx, q, args...)
}

func (t *socialTx) row(q string, args ...any) *sql.Row {
	return t.tx.QueryRowContext(t.ctx, q, args...)
}

// lockUsers 按 userId 字节序锁住用户的 user_social 行（行不存在先建）。调用方只传存在的用户（L2：不为不存在的用户建行）。
func (t *socialTx) lockUsers(ids ...string) error {
	ids = slices.Clone(ids)
	slices.Sort(ids)
	for _, id := range slices.Compact(ids) {
		t.lock(lockKindUser, id)
		if _, err := t.exec(`INSERT INTO user_social (user_id, updated_at) VALUES (?, ?) ON CONFLICT DO NOTHING`, id, t.now); err != nil {
			return err
		}
	}
	return nil
}

// verFor 返回本事务给 uid 分配的 ver，第一次调用时分配（ver = ver + 1，L1）。同一事务对同一用户只分配一次。
func (t *socialTx) verFor(uid string) (int64, error) {
	if v, ok := t.vers[uid]; ok {
		return v, nil
	}
	var v int64
	if err := t.row(`UPDATE user_social SET ver = ver + 1, updated_at = ? WHERE user_id = ? RETURNING ver`, t.now, uid).Scan(&v); err != nil {
		return 0, fmt.Errorf("allocate ver for %s: %w", uid, err)
	}
	t.vers[uid] = v
	return v, nil
}

func (t *socialTx) userExists(uid string) (bool, error) {
	var one int
	err := t.row(`SELECT 1 FROM users WHERE user_id = ?`, uid).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (t *socialTx) state(uid string) (SocialState, error) {
	return readState(t.ctx, t.tx, uid)
}

type querier interface {
	QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row
	QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
}

func readState(ctx context.Context, q querier, uid string) (SocialState, error) {
	var st SocialState
	var searchable sql.NullInt64
	err := q.QueryRowContext(ctx,
		`SELECT ver, purged_ver, request_seen_ver, searchable, friend_count, block_count FROM user_social WHERE user_id = ?`, uid).
		Scan(&st.Ver, &st.PurgedVer, &st.RequestSeenVer, &searchable, &st.FriendCount, &st.BlockCount)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return st, err
	}
	if searchable.Valid {
		b := searchable.Int64 != 0
		st.Searchable = &b
	}
	err = q.QueryRowContext(ctx, `SELECT official FROM users WHERE user_id = ?`, uid).Scan(&st.Official)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return st, err
	}
	return st, nil
}

// edge 返回 owner → peer 的好友边是否存在（非墓碑）。
func (t *socialTx) edge(owner, peer string) (bool, error) {
	var deleted bool
	err := t.row(`SELECT deleted FROM friends WHERE owner_id = ? AND peer_id = ?`, owner, peer).Scan(&deleted)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil && !deleted, err
}

func (t *socialTx) blocked(owner, peer string) (bool, error) {
	var deleted bool
	err := t.row(`SELECT deleted FROM blocks WHERE owner_id = ? AND peer_id = ?`, owner, peer).Scan(&deleted)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil && !deleted, err
}

func (t *socialTx) friendCount(uid string) (int, error) {
	var n int
	err := t.row(`SELECT friend_count FROM user_social WHERE user_id = ?`, uid).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return n, err
}

// addEdge 建立（或恢复）owner → peer 的边；已存在时不改动，返回 false。
func (t *socialTx) addEdge(owner, peer, remark, source string) (bool, error) {
	if ok, err := t.edge(owner, peer); err != nil || ok {
		return false, err
	}
	ver, err := t.verFor(owner)
	if err != nil {
		return false, err
	}
	if _, err := t.exec(
		`INSERT INTO friends (owner_id, peer_id, remark, source, created_at, updated_at, deleted, ver) VALUES (?, ?, ?, ?, ?, ?, 0, ?)
		 ON CONFLICT (owner_id, peer_id) DO UPDATE SET remark = excluded.remark, source = excluded.source,
		   created_at = excluded.created_at, updated_at = excluded.updated_at, deleted = 0, ver = excluded.ver`,
		owner, peer, remark, source, t.now, t.now, ver); err != nil {
		return false, err
	}
	_, err = t.exec(`UPDATE user_social SET friend_count = friend_count + 1 WHERE user_id = ?`, owner)
	return err == nil, err
}

// removeEdge 把 owner → peer 的边写成墓碑（备注清空）；没有边时返回 false。
func (t *socialTx) removeEdge(owner, peer string) (bool, error) {
	if ok, err := t.edge(owner, peer); err != nil || !ok {
		return false, err
	}
	ver, err := t.verFor(owner)
	if err != nil {
		return false, err
	}
	if _, err := t.exec(`UPDATE friends SET deleted = 1, remark = '', updated_at = ?, ver = ? WHERE owner_id = ? AND peer_id = ?`,
		t.now, ver, owner, peer); err != nil {
		return false, err
	}
	_, err = t.exec(`UPDATE user_social SET friend_count = friend_count - 1 WHERE user_id = ?`, owner)
	return err == nil, err
}

// acceptRequests 把 owner 一侧与 peer 之间所有未同意的申请行置为 accepted。
func (t *socialTx) acceptRequests(owner, peer string) error {
	var n int
	if err := t.row(`SELECT count(*) FROM friend_requests WHERE owner_id = ? AND peer_id = ? AND status <> 'accepted'`, owner, peer).Scan(&n); err != nil || n == 0 {
		return err
	}
	ver, err := t.verFor(owner)
	if err != nil {
		return err
	}
	_, err = t.exec(`UPDATE friend_requests SET status = 'accepted', updated_at = ?, ver = ? WHERE owner_id = ? AND peer_id = ? AND status <> 'accepted'`,
		t.now, ver, owner, peer)
	return err
}

// upsertRequest 新建或合并一条申请行（F-SOC-02 规则 3）：状态回到 pending，附言 / 来源 / 预设备注覆盖，applyCount + 1。
func (t *socialTx) upsertRequest(owner, peer, dir, message, source, sourceRef, preset string, expireAt int64) error {
	ver, err := t.verFor(owner)
	if err != nil {
		return err
	}
	_, err = t.exec(
		`INSERT INTO friend_requests (owner_id, peer_id, direction, status, message, source, source_ref, preset_remark, apply_count, created_at, updated_at, expire_at, ver)
		 VALUES (?, ?, ?, 'pending', ?, ?, ?, ?, 1, ?, ?, ?, ?)
		 ON CONFLICT (owner_id, peer_id, direction) DO UPDATE SET status = 'pending', message = excluded.message,
		   source = excluded.source, source_ref = excluded.source_ref, preset_remark = excluded.preset_remark,
		   apply_count = friend_requests.apply_count + 1, updated_at = excluded.updated_at, expire_at = excluded.expire_at, ver = excluded.ver`,
		owner, peer, dir, message, source, sourceRef, preset, t.now, t.now, expireAt, ver)
	return err
}

func (t *socialTx) request(owner, peer, dir string) (RequestRow, bool, error) {
	r, err := scanRequest(t.row(`SELECT `+requestColumns+` FROM friend_requests WHERE owner_id = ? AND peer_id = ? AND direction = ?`, owner, peer, dir))
	if errors.Is(err, sql.ErrNoRows) {
		return RequestRow{}, false, nil
	}
	return r, err == nil, err
}

// finish 提交事务，返回 me 的结果：变化时带上我本次改动的全部行。
func (t *socialTx) finish(me string) (SocialChange, error) {
	c := SocialChange{Bumped: t.vers}
	if v, ok := t.vers[me]; ok {
		c.Ver, c.Changed = v, true
		items, err := itemsAtVer(t.ctx, t.tx, me, v)
		if err != nil {
			return SocialChange{}, err
		}
		c.Items = items
	} else {
		st, err := t.state(me)
		if err != nil {
			return SocialChange{}, err
		}
		c.Ver = st.Ver
	}
	return c, t.tx.Commit()
}

func itemsAtVer(ctx context.Context, q querier, uid string, ver int64) (SocialItems, error) {
	var it SocialItems
	var err error
	if it.Friends, err = queryFriends(ctx, q, `WHERE owner_id = ? AND ver = ? ORDER BY peer_id`, uid, ver); err != nil {
		return it, err
	}
	if it.Requests, err = queryRequests(ctx, q, `WHERE owner_id = ? AND ver = ? ORDER BY peer_id, direction`, uid, ver); err != nil {
		return it, err
	}
	it.Blocks, err = queryBlocks(ctx, q, `WHERE owner_id = ? AND ver = ? ORDER BY peer_id`, uid, ver)
	return it, err
}

const (
	friendColumns  = `owner_id, peer_id, remark, source, created_at, updated_at, deleted, ver`
	requestColumns = `owner_id, peer_id, direction, status, message, source, source_ref, preset_remark, apply_count, created_at, updated_at, expire_at, ver`
	blockColumns   = `owner_id, peer_id, created_at, updated_at, deleted, ver`
)

func scanRequest(r rowScanner) (RequestRow, error) {
	var x RequestRow
	err := r.Scan(&x.Owner, &x.Peer, &x.Direction, &x.Status, &x.Message, &x.Source, &x.SourceRef, &x.PresetRemark,
		&x.ApplyCount, &x.CreatedAt, &x.UpdatedAt, &x.ExpireAt, &x.Ver)
	return x, err
}

// #nosec G202 -- where 都是代码里的常量片段，取值全部走占位符
func queryFriends(ctx context.Context, q querier, where string, args ...any) ([]FriendRow, error) {
	rows, err := q.QueryContext(ctx, `SELECT `+friendColumns+` FROM friends `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FriendRow
	for rows.Next() {
		var f FriendRow
		if err := rows.Scan(&f.Owner, &f.Peer, &f.Remark, &f.Source, &f.CreatedAt, &f.UpdatedAt, &f.Deleted, &f.Ver); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// #nosec G202 -- 同上
func queryRequests(ctx context.Context, q querier, where string, args ...any) ([]RequestRow, error) {
	rows, err := q.QueryContext(ctx, `SELECT `+requestColumns+` FROM friend_requests `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RequestRow
	for rows.Next() {
		r, err := scanRequest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// #nosec G202 -- 同上
func queryBlocks(ctx context.Context, q querier, where string, args ...any) ([]BlockRow, error) {
	rows, err := q.QueryContext(ctx, `SELECT `+blockColumns+` FROM blocks `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BlockRow
	for rows.Next() {
		var b BlockRow
		if err := rows.Scan(&b.Owner, &b.Peer, &b.CreatedAt, &b.UpdatedAt, &b.Deleted, &b.Ver); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// ---- 配置 ----

func (s *SQLite) SocialSettingsJSON(ctx context.Context) (string, bool, error) {
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = 'social'`).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return v, err == nil, err
}

func (s *SQLite) InitSocialSettings(ctx context.Context, value string, now int64) (bool, error) {
	res, err := s.db.ExecContext(ctx, `INSERT INTO app_settings (key, value, updated_at) VALUES ('social', ?, ?) ON CONFLICT DO NOTHING`, value, now)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (s *SQLite) UpdateSocialSettings(ctx context.Context, now int64, fn func(old string) (string, error)) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var old string
	if err := tx.QueryRowContext(ctx, `SELECT value FROM app_settings WHERE key = 'social'`).Scan(&old); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	v, err := fn(old)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO app_settings (key, value, updated_at) VALUES ('social', ?, ?)
		 ON CONFLICT (key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`, v, now); err != nil {
		return err
	}
	return tx.Commit()
}

// ---- 好友申请（F-SOC-02 规则 2 判定表第 3～9 行；第 1、2 行在 logic） ----

func (s *SQLite) ApplyFriend(ctx context.Context, p ApplyFriendParams) (string, SocialChange, error) {
	t, err := s.beginSocial(ctx, p.Now)
	if err != nil {
		return "", SocialChange{}, err
	}
	defer t.tx.Rollback()

	me, peer := p.From, p.To
	peerExists, err := t.userExists(peer)
	if err != nil {
		return "", SocialChange{}, err
	}
	// L2：判定之前先锁双方；对方不存在只锁自己。
	if peerExists {
		err = t.lockUsers(me, peer)
	} else {
		err = t.lockUsers(me)
	}
	if err != nil {
		return "", SocialChange{}, err
	}

	// 3：我把对方拉黑了。
	if b, err := t.blocked(me, peer); err != nil {
		return "", SocialChange{}, err
	} else if b {
		return "", SocialChange{}, ErrIBlockedPeer
	}
	mine, err := t.edge(me, peer)
	if err != nil {
		return "", SocialChange{}, err
	}
	theirs, err := t.edge(peer, me)
	if err != nil {
		return "", SocialChange{}, err
	}
	// 4：已互为好友。
	if mine && theirs {
		c, err := t.finish(me)
		return ReqAccepted, c, err
	}
	// 5：我的好友已满。
	if n, err := t.friendCount(me); err != nil {
		return "", SocialChange{}, err
	} else if n >= p.MaxFriends {
		return "", SocialChange{}, ErrFriendLimit
	}
	// 6：对方不存在或把我拉黑了：只写我的 out 行，对方零写入。
	peerBlocksMe := false
	if peerExists {
		if peerBlocksMe, err = t.blocked(peer, me); err != nil {
			return "", SocialChange{}, err
		}
	}
	if !peerExists || peerBlocksMe {
		if err := t.upsertRequest(me, peer, DirOut, p.Message, p.Source, p.SourceRef, p.Remark, p.ExpireAt); err != nil {
			return "", SocialChange{}, err
		}
		c, err := t.finish(me)
		return ReqPending, c, err
	}
	// 7：对方仍有我的边（我之前单向删了对方）：恢复我的边，对方无感知。
	if theirs {
		if _, err := t.addEdge(me, peer, p.Remark, p.Source); err != nil {
			return "", SocialChange{}, err
		}
		if err := t.acceptRequests(me, peer); err != nil {
			return "", SocialChange{}, err
		}
		c, err := t.finish(me)
		return ReqAccepted, c, err
	}
	// 8：我这一侧有一条来自对方的、未过期的 in 行：自动成为好友。
	in, ok, err := t.request(me, peer, DirIn)
	if err != nil {
		return "", SocialChange{}, err
	}
	if ok && (in.Status == ReqPending || in.Status == ReqRejected) && in.ExpireAt > p.Now {
		out, _, err := t.request(peer, me, DirOut)
		if err != nil {
			return "", SocialChange{}, err
		}
		if err := t.befriend(me, peer, p.Remark, p.Source, out.PresetRemark, out.Source, p.MaxFriends); err != nil {
			return "", SocialChange{}, err
		}
		c, err := t.finish(me)
		return ReqAccepted, c, err
	}
	// 9：新建或合并申请。source=group 时在锁内校验双方都在群里，不满足时 in 行记 other（out 行原样）。
	inSource, inRef := p.Source, p.SourceRef
	if p.Source == "group" {
		var n int
		if err := t.row(`SELECT count(*) FROM group_members gm JOIN chat_groups g ON g.group_id = gm.group_id
			WHERE gm.group_id = ? AND gm.user_id IN (?, ?) AND g.dissolved_at = 0`, p.SourceRef, me, peer).Scan(&n); err != nil {
			return "", SocialChange{}, err
		}
		if n != 2 {
			inSource, inRef = "other", ""
		}
	}
	if err := t.upsertRequest(me, peer, DirOut, p.Message, p.Source, p.SourceRef, p.Remark, p.ExpireAt); err != nil {
		return "", SocialChange{}, err
	}
	if err := t.upsertRequest(peer, me, DirIn, p.Message, inSource, inRef, "", p.ExpireAt); err != nil {
		return "", SocialChange{}, err
	}
	c, err := t.finish(me)
	return ReqPending, c, err
}

// befriend 建立 me ⇄ peer 的双向边并把双方之间所有申请行置为 accepted。
// 对方好友已满（且对方还没有我的边）返回 ErrPeerFriendLimit，不改数据（调用方回滚）。
func (t *socialTx) befriend(me, peer, myRemark, mySource, peerRemark, peerSource string, maxFriends int) error {
	theirs, err := t.edge(peer, me)
	if err != nil {
		return err
	}
	if !theirs {
		n, err := t.friendCount(peer)
		if err != nil {
			return err
		}
		if n >= maxFriends {
			return ErrPeerFriendLimit
		}
	}
	if _, err := t.addEdge(me, peer, myRemark, mySource); err != nil {
		return err
	}
	if _, err := t.addEdge(peer, me, peerRemark, peerSource); err != nil {
		return err
	}
	if err := t.acceptRequests(me, peer); err != nil {
		return err
	}
	return t.acceptRequests(peer, me)
}

// ---- 处理申请、删除、备注（F-SOC-03、F-SOC-05） ----

func (s *SQLite) RespondFriend(ctx context.Context, p RespondFriendParams) (SocialChange, error) {
	t, err := s.beginSocial(ctx, p.Now)
	if err != nil {
		return SocialChange{}, err
	}
	defer t.tx.Rollback()

	fromExists, err := t.userExists(p.From)
	if err != nil {
		return SocialChange{}, err
	}
	if fromExists {
		err = t.lockUsers(p.Me, p.From)
	} else {
		err = t.lockUsers(p.Me)
	}
	if err != nil {
		return SocialChange{}, err
	}
	in, ok, err := t.request(p.Me, p.From, DirIn)
	if err != nil {
		return SocialChange{}, err
	}
	if !ok {
		return SocialChange{}, ErrRequestNotFound
	}
	if in.Status == ReqAccepted {
		if !p.Accept {
			return SocialChange{}, ErrRequestHandled
		}
		return t.finish(p.Me) // 幂等：changed=false
	}
	if in.ExpireAt <= p.Now {
		return SocialChange{}, ErrRequestNotFound
	}
	if !p.Accept {
		if in.Status == ReqRejected {
			return t.finish(p.Me)
		}
		ver, err := t.verFor(p.Me)
		if err != nil {
			return SocialChange{}, err
		}
		if _, err := t.exec(`UPDATE friend_requests SET status = 'rejected', updated_at = ?, ver = ? WHERE owner_id = ? AND peer_id = ? AND direction = 'in'`,
			p.Now, ver, p.Me, p.From); err != nil {
			return SocialChange{}, err
		}
		return t.finish(p.Me)
	}
	// 同意：L3，4132 / 4133 读加锁后的计数。我已经有对方的边时不占新名额。
	if mine, err := t.edge(p.Me, p.From); err != nil {
		return SocialChange{}, err
	} else if !mine {
		n, err := t.friendCount(p.Me)
		if err != nil {
			return SocialChange{}, err
		}
		if n >= p.MaxFriends {
			return SocialChange{}, ErrFriendLimit
		}
	}
	out, _, err := t.request(p.From, p.Me, DirOut)
	if err != nil {
		return SocialChange{}, err
	}
	if err := t.befriend(p.Me, p.From, p.Remark, in.Source, out.PresetRemark, out.Source, p.MaxFriends); err != nil {
		return SocialChange{}, err
	}
	return t.finish(p.Me)
}

func (s *SQLite) DeleteFriend(ctx context.Context, me, peer string, now int64) (SocialChange, error) {
	t, err := s.beginSocial(ctx, now)
	if err != nil {
		return SocialChange{}, err
	}
	defer t.tx.Rollback()
	if err := t.lockUsers(me); err != nil {
		return SocialChange{}, err
	}
	if _, err := t.removeEdge(me, peer); err != nil {
		return SocialChange{}, err
	}
	return t.finish(me)
}

func (s *SQLite) SetRemark(ctx context.Context, me, peer, remark string, now int64) (SocialChange, error) {
	t, err := s.beginSocial(ctx, now)
	if err != nil {
		return SocialChange{}, err
	}
	defer t.tx.Rollback()
	if err := t.lockUsers(me); err != nil {
		return SocialChange{}, err
	}
	var cur string
	var deleted bool
	err = t.row(`SELECT remark, deleted FROM friends WHERE owner_id = ? AND peer_id = ?`, me, peer).Scan(&cur, &deleted)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && deleted) {
		return SocialChange{}, ErrNotFriend
	}
	if err != nil {
		return SocialChange{}, err
	}
	if cur != remark {
		ver, err := t.verFor(me)
		if err != nil {
			return SocialChange{}, err
		}
		if _, err := t.exec(`UPDATE friends SET remark = ?, updated_at = ?, ver = ? WHERE owner_id = ? AND peer_id = ?`, remark, now, ver, me, peer); err != nil {
			return SocialChange{}, err
		}
	}
	return t.finish(me)
}

// ---- 拉黑（F-SOC-07，L4：只锁自己） ----

func (s *SQLite) Block(ctx context.Context, me, peer string, now int64, maxBlocks int) (SocialChange, error) {
	t, err := s.beginSocial(ctx, now)
	if err != nil {
		return SocialChange{}, err
	}
	defer t.tx.Rollback()
	if err := t.lockUsers(me); err != nil {
		return SocialChange{}, err
	}
	if b, err := t.blocked(me, peer); err != nil || b {
		if err != nil {
			return SocialChange{}, err
		}
		return t.finish(me)
	}
	st, err := t.state(me)
	if err != nil {
		return SocialChange{}, err
	}
	if st.BlockCount >= maxBlocks {
		return SocialChange{}, ErrBlockLimit
	}
	ver, err := t.verFor(me)
	if err != nil {
		return SocialChange{}, err
	}
	if _, err := t.exec(
		`INSERT INTO blocks (owner_id, peer_id, created_at, updated_at, deleted, ver) VALUES (?, ?, ?, ?, 0, ?)
		 ON CONFLICT (owner_id, peer_id) DO UPDATE SET created_at = excluded.created_at, updated_at = excluded.updated_at, deleted = 0, ver = excluded.ver`,
		me, peer, now, now, ver); err != nil {
		return SocialChange{}, err
	}
	if _, err := t.exec(`UPDATE user_social SET block_count = block_count + 1 WHERE user_id = ?`, me); err != nil {
		return SocialChange{}, err
	}
	// 我一侧来自对方的待处理申请置为 rejected（同一个 ver）；对方的 out 行不变。
	if _, err := t.exec(`UPDATE friend_requests SET status = 'rejected', updated_at = ?, ver = ?
		WHERE owner_id = ? AND peer_id = ? AND direction = 'in' AND status = 'pending' AND expire_at > ?`, now, ver, me, peer, now); err != nil {
		return SocialChange{}, err
	}
	return t.finish(me)
}

func (s *SQLite) Unblock(ctx context.Context, me, peer string, now int64) (SocialChange, error) {
	t, err := s.beginSocial(ctx, now)
	if err != nil {
		return SocialChange{}, err
	}
	defer t.tx.Rollback()
	if err := t.lockUsers(me); err != nil {
		return SocialChange{}, err
	}
	if b, err := t.blocked(me, peer); err != nil || !b {
		if err != nil {
			return SocialChange{}, err
		}
		return t.finish(me)
	}
	ver, err := t.verFor(me)
	if err != nil {
		return SocialChange{}, err
	}
	if _, err := t.exec(`UPDATE blocks SET deleted = 1, updated_at = ?, ver = ? WHERE owner_id = ? AND peer_id = ?`, now, ver, me, peer); err != nil {
		return SocialChange{}, err
	}
	if _, err := t.exec(`UPDATE user_social SET block_count = block_count - 1 WHERE user_id = ?`, me); err != nil {
		return SocialChange{}, err
	}
	return t.finish(me)
}

// ---- 红点、隐私 ----

func (s *SQLite) SetRequestSeen(ctx context.Context, me string, ver, now int64) (int64, SocialChange, error) {
	t, err := s.beginSocial(ctx, now)
	if err != nil {
		return 0, SocialChange{}, err
	}
	defer t.tx.Rollback()
	if err := t.lockUsers(me); err != nil {
		return 0, SocialChange{}, err
	}
	st, err := t.state(me)
	if err != nil {
		return 0, SocialChange{}, err
	}
	seen := max(st.RequestSeenVer, min(ver, st.Ver))
	if seen != st.RequestSeenVer {
		if _, err := t.verFor(me); err != nil {
			return 0, SocialChange{}, err
		}
		if _, err := t.exec(`UPDATE user_social SET request_seen_ver = ? WHERE user_id = ?`, seen, me); err != nil {
			return 0, SocialChange{}, err
		}
	}
	c, err := t.finish(me)
	return seen, c, err
}

func (s *SQLite) SetSearchable(ctx context.Context, me string, searchable, def bool, now int64) (SocialChange, error) {
	t, err := s.beginSocial(ctx, now)
	if err != nil {
		return SocialChange{}, err
	}
	defer t.tx.Rollback()
	if err := t.lockUsers(me); err != nil {
		return SocialChange{}, err
	}
	st, err := t.state(me)
	if err != nil {
		return SocialChange{}, err
	}
	cur := def
	if st.Searchable != nil {
		cur = *st.Searchable
	}
	if cur != searchable {
		if _, err := t.verFor(me); err != nil {
			return SocialChange{}, err
		}
		if _, err := t.exec(`UPDATE user_social SET searchable = ? WHERE user_id = ?`, searchable, me); err != nil {
			return SocialChange{}, err
		}
	}
	return t.finish(me)
}

func (s *SQLite) SocialState(ctx context.Context, uid string) (SocialState, error) {
	return readState(ctx, s.db, uid)
}

// ---- 同步（F-SOC-06） ----

func (s *SQLite) SocialPage(ctx context.Context, q SocialPageQuery) (SocialPage, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return SocialPage{}, err
	}
	defer tx.Rollback()
	st, err := readState(ctx, tx, q.UID)
	if err != nil {
		return SocialPage{}, err
	}
	page := SocialPage{State: st, Full: q.Full, SinceVer: q.SinceVer, UntilVer: q.UntilVer}
	if q.First {
		page.Full = q.SinceVer <= 0 || q.SinceVer > st.Ver || q.SinceVer < st.PurgedVer
		page.UntilVer = st.Ver
	} else if !q.Full && st.PurgedVer > q.SinceVer {
		page.Restart = true // 本轮中途执行过清理
		return page, nil
	}
	lower := page.SinceVer
	if page.Full {
		lower = -1
	}
	n := q.Limit + 1

	// 三张表各取 limit+1 行，按排序键归并（8.6 节；行值比较可迁移到 Postgres）。
	fw, fa := afterCond(q.After, ItemFriend, "peer_id")
	fq := `WHERE owner_id = ? AND ver > ? AND ver <= ?` + fw
	if page.Full {
		fq += ` AND deleted = 0`
	}
	friends, err := queryFriends(ctx, tx, fq+` ORDER BY ver, peer_id LIMIT ?`, append(append([]any{q.UID, lower, page.UntilVer}, fa...), n)...)
	if err != nil {
		return SocialPage{}, err
	}
	rw, ra := afterCond(q.After, ItemRequest, "peer_id, direction")
	rq := `WHERE owner_id = ? AND ver > ? AND ver <= ?` + rw
	rargs := append([]any{q.UID, lower, page.UntilVer}, ra...)
	if page.Full {
		rq += ` AND updated_at >= ?`
		rargs = append(rargs, q.RequestCutoff)
	}
	requests, err := queryRequests(ctx, tx, rq+` ORDER BY ver, peer_id, direction LIMIT ?`, append(rargs, n)...)
	if err != nil {
		return SocialPage{}, err
	}
	bw, ba := afterCond(q.After, ItemBlock, "peer_id")
	bq := `WHERE owner_id = ? AND ver > ? AND ver <= ?` + bw
	if page.Full {
		bq += ` AND deleted = 0`
	}
	blocks, err := queryBlocks(ctx, tx, bq+` ORDER BY ver, peer_id LIMIT ?`, append(append([]any{q.UID, lower, page.UntilVer}, ba...), n)...)
	if err != nil {
		return SocialPage{}, err
	}

	type entry struct {
		key SortKey
		idx int
	}
	all := make([]entry, 0, len(friends)+len(requests)+len(blocks))
	for i, f := range friends {
		all = append(all, entry{SortKey{f.Ver, ItemFriend, f.Peer, ""}, i})
	}
	for i, r := range requests {
		all = append(all, entry{SortKey{r.Ver, ItemRequest, r.Peer, r.Direction}, i})
	}
	for i, b := range blocks {
		all = append(all, entry{SortKey{b.Ver, ItemBlock, b.Peer, ""}, i})
	}
	slices.SortFunc(all, func(a, b entry) int { return CompareSortKey(a.key, b.key) })
	if len(all) > q.Limit {
		page.HasMore = true
		all = all[:q.Limit]
	}
	for _, e := range all {
		switch e.key.Type {
		case ItemFriend:
			page.Items.Friends = append(page.Items.Friends, friends[e.idx])
		case ItemRequest:
			page.Items.Requests = append(page.Items.Requests, requests[e.idx])
		default:
			page.Items.Blocks = append(page.Items.Blocks, blocks[e.idx])
		}
	}
	if len(all) > 0 {
		k := all[len(all)-1].key
		page.Last = &k
	}
	return page, tx.Commit()
}

// CompareSortKey 按 (ver, 类型, 对方 userId, 方向) 比较。
func CompareSortKey(a, b SortKey) int {
	switch {
	case a.Ver != b.Ver:
		return cmpInt(a.Ver, b.Ver)
	case a.Type != b.Type:
		return cmpInt(int64(a.Type), int64(b.Type))
	case a.Peer != b.Peer:
		return strings.Compare(a.Peer, b.Peer)
	}
	return strings.Compare(a.Dir, b.Dir)
}

func cmpInt(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// afterCond 返回"排序键严格大于 after"在类型 typ 的表上的条件。
func afterCond(after *SortKey, typ int, peerCols string) (string, []any) {
	switch {
	case after == nil:
		return "", nil
	case after.Type < typ:
		return ` AND ver >= ?`, []any{after.Ver}
	case after.Type > typ:
		return ` AND ver > ?`, []any{after.Ver}
	case typ == ItemRequest:
		return ` AND (ver, ` + peerCols + `) > (?, ?, ?)`, []any{after.Ver, after.Peer, after.Dir}
	default:
		return ` AND (ver, ` + peerCols + `) > (?, ?)`, []any{after.Ver, after.Peer}
	}
}

// ---- 清理任务（F-SOC-06 规则 5） ----

func (s *SQLite) PurgeSocial(ctx context.Context, before int64) (int, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT owner_id FROM friends WHERE deleted = 1 AND updated_at < ?
		 UNION SELECT owner_id FROM blocks WHERE deleted = 1 AND updated_at < ?
		 UNION SELECT owner_id FROM friend_requests WHERE updated_at < ?`, before, before, before)
	if err != nil {
		return 0, err
	}
	var owners []string
	for rows.Next() {
		var o string
		if err := rows.Scan(&o); err != nil {
			rows.Close()
			return 0, err
		}
		owners = append(owners, o)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	total := 0
	for _, o := range owners {
		n, err := s.purgeOwner(ctx, o, before)
		if err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}

func (s *SQLite) purgeOwner(ctx context.Context, owner string, before int64) (int, error) {
	t, err := s.beginSocial(ctx, before)
	if err != nil {
		return 0, err
	}
	defer t.tx.Rollback()
	if err := t.lockUsers(owner); err != nil {
		return 0, err
	}
	var maxVer int64
	if err := t.row(`SELECT max(v) FROM (
		SELECT COALESCE(max(ver), 0) AS v FROM friends WHERE owner_id = ? AND deleted = 1 AND updated_at < ?
		UNION ALL SELECT COALESCE(max(ver), 0) FROM blocks WHERE owner_id = ? AND deleted = 1 AND updated_at < ?
		UNION ALL SELECT COALESCE(max(ver), 0) FROM friend_requests WHERE owner_id = ? AND updated_at < ?)`,
		owner, before, owner, before, owner, before).Scan(&maxVer); err != nil {
		return 0, err
	}
	n := 0
	for _, q := range []string{
		`DELETE FROM friends WHERE owner_id = ? AND deleted = 1 AND updated_at < ?`,
		`DELETE FROM blocks WHERE owner_id = ? AND deleted = 1 AND updated_at < ?`,
		`DELETE FROM friend_requests WHERE owner_id = ? AND updated_at < ?`,
	} {
		res, err := t.exec(q, owner, before)
		if err != nil {
			return 0, err
		}
		k, _ := res.RowsAffected()
		n += int(k)
	}
	if _, err := t.exec(`UPDATE user_social SET purged_ver = max(purged_ver, ?) WHERE user_id = ?`, maxVer, owner); err != nil {
		return 0, err
	}
	return n, t.tx.Commit()
}

// ---- 发送路径、搜索、资料 ----

func (s *SQLite) Relation(ctx context.Context, me, peer, convID string) (Relation, error) {
	var r Relation
	err := s.db.QueryRowContext(ctx, `SELECT
		EXISTS (SELECT 1 FROM users WHERE user_id = ?2 AND deleted_at = 0),
		EXISTS (SELECT 1 FROM friends WHERE owner_id = ?1 AND peer_id = ?2 AND deleted = 0),
		EXISTS (SELECT 1 FROM friends WHERE owner_id = ?2 AND peer_id = ?1 AND deleted = 0),
		EXISTS (SELECT 1 FROM blocks WHERE owner_id = ?2 AND peer_id = ?1 AND deleted = 0),
		COALESCE((SELECT official FROM users WHERE user_id = ?1), 0),
		COALESCE((SELECT official FROM users WHERE user_id = ?2), 0),
		COALESCE((SELECT max_seq FROM conv_seq WHERE conv_id = ?3), 0) > 0,
		COALESCE((SELECT sent_count FROM user_conv WHERE user_id = ?1 AND conv_id = ?3), 0),
		COALESCE((SELECT sent_count FROM user_conv WHERE user_id = ?2 AND conv_id = ?3), 0)`, me, peer, convID).
		Scan(&r.PeerExists, &r.MeToPeer, &r.PeerToMe, &r.PeerBlocksMe, &r.MeOfficial, &r.PeerOfficial, &r.HasMessages, &r.MySent, &r.PeerSent)
	return r, err
}

func sentCount(ctx context.Context, tx *sql.Tx, uid, convID string) (int, error) {
	var n int
	err := tx.QueryRowContext(ctx, `SELECT sent_count FROM user_conv WHERE user_id = ? AND conv_id = ?`, uid, convID).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return n, err
}

func (s *SQLite) SearchUser(ctx context.Context, me, target string, now int64) (User, bool, *bool, SearchRelation, error) {
	// 无论用户是否存在都执行同一条查询（F-SOC-09 规则 3：响应时间不可区分）。
	var u User
	var exists bool
	var searchable sql.NullInt64
	var rel SearchRelation
	err := s.db.QueryRowContext(ctx, `SELECT
		EXISTS (SELECT 1 FROM users WHERE user_id = ?2),
		COALESCE((SELECT nickname FROM users WHERE user_id = ?2), ''),
		COALESCE((SELECT avatar FROM users WHERE user_id = ?2), ''),
		COALESCE((SELECT official FROM users WHERE user_id = ?2), 0),
		(SELECT searchable FROM user_social WHERE user_id = ?2),
		EXISTS (SELECT 1 FROM blocks WHERE owner_id = ?1 AND peer_id = ?2 AND deleted = 0),
		EXISTS (SELECT 1 FROM friends WHERE owner_id = ?1 AND peer_id = ?2 AND deleted = 0),
		EXISTS (SELECT 1 FROM friend_requests WHERE owner_id = ?1 AND peer_id = ?2 AND direction = 'in' AND status = 'pending' AND expire_at > ?3),
		EXISTS (SELECT 1 FROM friend_requests WHERE owner_id = ?1 AND peer_id = ?2 AND direction = 'out' AND status = 'pending' AND expire_at > ?3)`,
		me, target, now).
		Scan(&exists, &u.Nickname, &u.Avatar, &u.Official, &searchable, &rel.IBlocked, &rel.IFriend, &rel.PendingIn, &rel.PendingOut)
	if err != nil {
		return User{}, false, nil, rel, err
	}
	u.UserID = target
	var sp *bool
	if searchable.Valid {
		b := searchable.Int64 != 0
		sp = &b
	}
	return u, exists, sp, rel, nil
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?, ", n), ", ")
}

func (s *SQLite) VisibleUsers(ctx context.Context, uid string, ids []string, cutoff int64) ([]User, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	args := make([]any, 0, len(ids)+6)
	for _, id := range ids {
		args = append(args, id)
	}
	args = append(args, uid, uid, uid, uid, uid, cutoff)
	// #nosec G202 -- 只拼接占位符
	rows, err := s.db.QueryContext(ctx, `SELECT u.user_id, u.nickname, u.avatar, u.created_at, u.official FROM users u
		WHERE u.user_id IN (`+placeholders(len(ids))+`) AND (
		  u.user_id = ?
		  OR EXISTS (SELECT 1 FROM friends f WHERE f.owner_id = ? AND f.peer_id = u.user_id AND f.deleted = 0)
		  OR EXISTS (SELECT 1 FROM user_conv uc WHERE uc.user_id = ? AND uc.peer_id = u.user_id)
		  OR EXISTS (SELECT 1 FROM group_members a JOIN group_members b ON b.group_id = a.group_id
		             JOIN chat_groups g ON g.group_id = a.group_id
		             WHERE a.user_id = ? AND b.user_id = u.user_id AND g.dissolved_at = 0)
		  OR EXISTS (SELECT 1 FROM friend_requests r WHERE r.owner_id = ? AND r.peer_id = u.user_id AND r.direction = 'in' AND r.updated_at >= ?)
		) ORDER BY u.user_id`, args...)
	if err != nil {
		return nil, err
	}
	return scanUsers(rows)
}

func scanUsers(rows *sql.Rows) ([]User, error) {
	defer rows.Close()
	var out []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.UserID, &u.Nickname, &u.Avatar, &u.CreatedAt, &u.Official); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *SQLite) Profiles(ctx context.Context, ids []string) (map[string]User, error) {
	out := make(map[string]User, len(ids))
	ids = slices.Compact(slices.Sorted(slices.Values(ids)))
	for start := 0; start < len(ids); start += 500 {
		chunk := ids[start:min(start+500, len(ids))]
		args := make([]any, len(chunk))
		for i, id := range chunk {
			args[i] = id
		}
		// #nosec G202 -- 只拼接占位符
		rows, err := s.db.QueryContext(ctx, `SELECT user_id, nickname, avatar, created_at, official FROM users WHERE user_id IN (`+placeholders(len(chunk))+`)`, args...)
		if err != nil {
			return nil, err
		}
		users, err := scanUsers(rows)
		if err != nil {
			return nil, err
		}
		for _, u := range users {
			out[u.UserID] = u
		}
	}
	return out, nil
}

func (s *SQLite) MyGroups(ctx context.Context, uid, after string, limit int) ([]MyGroup, bool, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT g.group_id, g.name, g.member_count, gm.joined_at, gm.role FROM group_members gm JOIN chat_groups g ON g.group_id = gm.group_id
		 WHERE gm.user_id = ? AND gm.group_id > ? AND g.dissolved_at = 0 ORDER BY gm.group_id LIMIT ?`, uid, after, limit+1)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	var out []MyGroup
	for rows.Next() {
		var g MyGroup
		if err := rows.Scan(&g.GroupID, &g.Name, &g.MemberCount, &g.JoinedAt, &g.MyRole); err != nil {
			return nil, false, err
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	more := len(out) > limit
	if more {
		out = out[:limit]
	}
	return out, more, nil
}

// ---- 管理接口（F-SOC-12、F-SOC-13） ----

func (s *SQLite) ImportFriends(ctx context.Context, pairs []FriendPair, now int64, maxFriends int) (map[string]int64, string, error) {
	t, err := s.beginSocial(ctx, now)
	if err != nil {
		return nil, "", err
	}
	defer t.tx.Rollback()
	users := pairUsers(pairs)
	for _, u := range users { // 按输入顺序报告第一个不存在的用户
		ok, err := t.userExists(u)
		if err != nil {
			return nil, "", err
		}
		if !ok {
			return nil, u, ErrUserMissing
		}
	}
	// L3：先收集全部用户、去重、按字节序一次性加锁。
	if err := t.lockUsers(users...); err != nil {
		return nil, "", err
	}
	grew := map[string]bool{}
	for _, p := range pairs {
		for _, side := range [][2]string{{p.A, p.B}, {p.B, p.A}} {
			owner, peer := side[0], side[1]
			remark := p.RemarkA
			if owner == p.B {
				remark = p.RemarkB
			}
			ok, err := t.edge(owner, peer)
			if err != nil {
				return nil, "", err
			}
			if !ok {
				if _, err := t.addEdge(owner, peer, deref(remark), "admin"); err != nil {
					return nil, "", err
				}
				grew[owner] = true
				continue
			}
			if remark == nil {
				continue
			}
			var cur string
			if err := t.row(`SELECT remark FROM friends WHERE owner_id = ? AND peer_id = ?`, owner, peer).Scan(&cur); err != nil {
				return nil, "", err
			}
			if cur != *remark {
				ver, err := t.verFor(owner)
				if err != nil {
					return nil, "", err
				}
				if _, err := t.exec(`UPDATE friends SET remark = ?, updated_at = ?, ver = ? WHERE owner_id = ? AND peer_id = ?`, *remark, now, ver, owner, peer); err != nil {
					return nil, "", err
				}
			}
		}
		if err := t.acceptRequests(p.A, p.B); err != nil {
			return nil, "", err
		}
		if err := t.acceptRequests(p.B, p.A); err != nil {
			return nil, "", err
		}
	}
	sorted := slices.Sorted(slices.Values(users))
	for _, u := range sorted {
		if !grew[u] {
			continue
		}
		n, err := t.friendCount(u)
		if err != nil {
			return nil, "", err
		}
		if n > maxFriends {
			return nil, u, ErrImportFriendCap
		}
	}
	return t.vers, "", t.tx.Commit()
}

func (s *SQLite) RemoveFriends(ctx context.Context, pairs []FriendPair, now int64) (map[string]int64, error) {
	t, err := s.beginSocial(ctx, now)
	if err != nil {
		return nil, err
	}
	defer t.tx.Rollback()
	var existing []string
	for _, u := range pairUsers(pairs) {
		ok, err := t.userExists(u)
		if err != nil {
			return nil, err
		}
		if ok {
			existing = append(existing, u)
		}
	}
	if err := t.lockUsers(existing...); err != nil {
		return nil, err
	}
	for _, p := range pairs {
		for _, side := range [][2]string{{p.A, p.B}, {p.B, p.A}} {
			if _, err := t.removeEdge(side[0], side[1]); err != nil {
				return nil, err
			}
		}
	}
	return t.vers, t.tx.Commit()
}

func pairUsers(pairs []FriendPair) []string {
	var out []string
	seen := map[string]bool{}
	for _, p := range pairs {
		for _, u := range []string{p.A, p.B} {
			if !seen[u] {
				seen[u] = true
				out = append(out, u)
			}
		}
	}
	return out
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (s *SQLite) ListFriends(ctx context.Context, uid, after string, limit int) ([]AdminFriend, bool, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT peer_id, source, created_at FROM friends WHERE owner_id = ? AND deleted = 0 AND peer_id > ? ORDER BY peer_id LIMIT ?`, uid, after, limit+1)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	var out []AdminFriend
	for rows.Next() {
		var f AdminFriend
		if err := rows.Scan(&f.UserID, &f.Source, &f.CreatedAt); err != nil {
			return nil, false, err
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	more := len(out) > limit
	if more {
		out = out[:limit]
	}
	return out, more, nil
}

func (s *SQLite) SetOfficial(ctx context.Context, uid string, official bool) error {
	res, err := s.db.ExecContext(ctx, `UPDATE users SET official = ? WHERE user_id = ?`, official, uid)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- 测试数据与计数器修复 ----

func (s *SQLite) SeedSocial(ctx context.Context, sd SocialSeed) (map[string]int64, error) {
	now := sd.UpdatedAt
	t, err := s.beginSocial(ctx, now)
	if err != nil {
		return nil, err
	}
	defer t.tx.Rollback()
	var owners []string
	for _, e := range sd.Friends {
		owners = append(owners, e.Owner)
	}
	for _, e := range sd.Blocks {
		owners = append(owners, e.Owner)
	}
	for _, r := range sd.Requests {
		owners = append(owners, r.From, r.To)
	}
	if err := t.lockUsers(owners...); err != nil {
		return nil, err
	}
	for _, e := range sd.Friends {
		if _, err := t.addEdge(e.Owner, e.Peer, "", "other"); err != nil {
			return nil, err
		}
	}
	for _, e := range sd.Blocks {
		if b, err := t.blocked(e.Owner, e.Peer); err != nil {
			return nil, err
		} else if b {
			continue
		}
		ver, err := t.verFor(e.Owner)
		if err != nil {
			return nil, err
		}
		if _, err := t.exec(`INSERT INTO blocks (owner_id, peer_id, created_at, updated_at, deleted, ver) VALUES (?, ?, ?, ?, 0, ?)
			ON CONFLICT (owner_id, peer_id) DO UPDATE SET updated_at = excluded.updated_at, deleted = 0, ver = excluded.ver`,
			e.Owner, e.Peer, now, now, ver); err != nil {
			return nil, err
		}
		if _, err := t.exec(`UPDATE user_social SET block_count = block_count + 1 WHERE user_id = ?`, e.Owner); err != nil {
			return nil, err
		}
	}
	for _, r := range sd.Requests {
		exp := r.ExpireAt
		if exp == 0 {
			exp = now + 7*24*3600*1000
		}
		src := r.Source
		if src == "" {
			src = "other"
		}
		if err := t.upsertRequest(r.From, r.To, DirOut, r.Message, src, "", "", exp); err != nil {
			return nil, err
		}
		if err := t.upsertRequest(r.To, r.From, DirIn, r.Message, src, "", "", exp); err != nil {
			return nil, err
		}
		if r.Status != "" && r.Status != ReqPending {
			if _, err := t.exec(`UPDATE friend_requests SET status = ? WHERE (owner_id = ? AND peer_id = ?) OR (owner_id = ? AND peer_id = ?)`,
				r.Status, r.From, r.To, r.To, r.From); err != nil {
				return nil, err
			}
		}
	}
	for _, m := range sd.GroupMembers {
		if _, err := t.exec(`INSERT INTO chat_groups (group_id, name, created_at) VALUES (?, ?, ?) ON CONFLICT DO NOTHING`, m.GroupID, m.GroupID, now); err != nil {
			return nil, err
		}
		if _, err := insertMembers(ctx, t.tx, m.GroupID, []string{m.UserID}, now); err != nil {
			return nil, err
		}
		if err := updateMemberCount(ctx, t.tx, m.GroupID); err != nil {
			return nil, err
		}
	}
	return t.vers, t.tx.Commit()
}

func (s *SQLite) RepairCounters(ctx context.Context) (int, error) {
	total := 0
	for _, q := range []string{
		`INSERT INTO user_social (user_id) SELECT DISTINCT owner_id FROM friends WHERE true ON CONFLICT DO NOTHING`,
		`INSERT INTO user_social (user_id) SELECT DISTINCT owner_id FROM blocks WHERE true ON CONFLICT DO NOTHING`,
	} {
		if _, err := s.db.ExecContext(ctx, q); err != nil {
			return total, err
		}
	}
	for _, q := range []string{
		`UPDATE user_social SET friend_count = (SELECT count(*) FROM friends f WHERE f.owner_id = user_social.user_id AND f.deleted = 0)
		 WHERE friend_count <> (SELECT count(*) FROM friends f WHERE f.owner_id = user_social.user_id AND f.deleted = 0)`,
		`UPDATE user_social SET block_count = (SELECT count(*) FROM blocks b WHERE b.owner_id = user_social.user_id AND b.deleted = 0)
		 WHERE block_count <> (SELECT count(*) FROM blocks b WHERE b.owner_id = user_social.user_id AND b.deleted = 0)`,
		`UPDATE chat_groups SET member_count = (SELECT count(*) FROM group_members gm WHERE gm.group_id = chat_groups.group_id)
		 WHERE member_count <> (SELECT count(*) FROM group_members gm WHERE gm.group_id = chat_groups.group_id)`,
		`UPDATE user_conv SET sent_count = (SELECT count(*) FROM messages m WHERE m.conv_id = user_conv.conv_id AND m.from_uid = user_conv.user_id)
		 WHERE sent_count <> (SELECT count(*) FROM messages m WHERE m.conv_id = user_conv.conv_id AND m.from_uid = user_conv.user_id)`,
	} {
		res, err := s.db.ExecContext(ctx, q)
		if err != nil {
			return total, err
		}
		n, _ := res.RowsAffected()
		total += int(n)
	}
	return total, nil
}

func (s *SQLite) CounterDrift(ctx context.Context, sample int) (int, error) {
	var total int
	for _, q := range []string{
		`SELECT count(*) FROM (SELECT * FROM user_social ORDER BY random() LIMIT ?) u
		 WHERE friend_count <> (SELECT count(*) FROM friends f WHERE f.owner_id = u.user_id AND f.deleted = 0)
		    OR block_count <> (SELECT count(*) FROM blocks b WHERE b.owner_id = u.user_id AND b.deleted = 0)`,
		`SELECT count(*) FROM (SELECT * FROM chat_groups ORDER BY random() LIMIT ?) g
		 WHERE member_count <> (SELECT count(*) FROM group_members gm WHERE gm.group_id = g.group_id)`,
		`SELECT count(*) FROM (SELECT * FROM user_conv ORDER BY random() LIMIT ?) uc
		 WHERE sent_count <> (SELECT count(*) FROM messages m WHERE m.conv_id = uc.conv_id AND m.from_uid = uc.user_id)`,
	} {
		var n int
		if err := s.db.QueryRowContext(ctx, q, sample).Scan(&n); err != nil {
			return total, err
		}
		total += n
	}
	return total, nil
}
