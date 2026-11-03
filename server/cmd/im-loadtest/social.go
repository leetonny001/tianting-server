package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"time"

	"tianting/server/internal/protocol"
)

// PRD-social 的性能场景（BE-121）。用测试模式启动服务端，只为用 /admin/test/social/seed 造数据（不经限流）。

// scenarioSocialFull：TC-SOC-06-18（AC-SOC-06-12）全量轮 limit=500，单页 P99 ≤ 100ms、整轮 ≤ 1 秒；3 轮取中位数。
func scenarioSocialFull(cfg config) result {
	r := result{Case: "TC-SOC-06-18", Title: "全量同步（5000 好友 + 1000 黑名单 + 200 申请）", Target: "limit=500 单页 P99 ≤ 100ms，整轮 ≤ 1 秒（3 轮中位数）"}
	s := newServer(cfg.bin, cfg.dir, "social-full")
	s.env = []string{"IM_TEST_MODE=1"}
	if err := s.start(); err != nil {
		r.note("启动失败：%v", err)
		return r
	}
	defer s.stop()
	reqUsers := ids("r", 200, 3)
	if err := s.importUsers(append([]string{"alice"}, reqUsers...)); err != nil {
		r.note("准备用户失败：%v", err)
		return r
	}
	var friends, blocks, reqs []map[string]any
	for _, id := range ids("f", 5000, 4) {
		friends = append(friends, map[string]any{"owner": "alice", "peer": id})
	}
	for _, id := range ids("b", 1000, 4) {
		blocks = append(blocks, map[string]any{"owner": "alice", "peer": id})
	}
	for _, id := range reqUsers {
		reqs = append(reqs, map[string]any{"from": id, "to": "alice"})
	}
	if _, err := s.admin(http.MethodPost, "/admin/test/social/seed", map[string]any{"friends": friends, "blocks": blocks, "requests": reqs}); err != nil {
		r.note("造数据失败：%v", err)
		return r
	}
	tokens, err := s.tokens([]string{"alice"})
	if err != nil {
		r.note("签发 token 失败：%v", err)
		return r
	}
	ctx := context.Background()
	c, err := dial(ctx, s.wsURL(), "alice", tokens["alice"])
	if err != nil {
		r.note("连接失败：%v", err)
		return r
	}
	defer c.close()

	var pages latencies
	var rounds []time.Duration
	items := 0
	for round := range 3 {
		start := time.Now()
		data := map[string]any{"sinceVer": 0, "limit": 500}
		n := 0
		for {
			t := time.Now()
			f, err := c.call(ctx, protocol.CmdSocialSync, data)
			if err == nil && f.Error != nil {
				err = f.Error
			}
			if err != nil {
				r.note("第 %d 轮同步失败：%v", round+1, err)
				return r
			}
			pages.add(time.Since(t))
			var p protocol.SocialSyncAck
			json.Unmarshal(f.Data, &p)
			n += len(p.Friends) + len(p.Requests) + len(p.Blocks)
			if !p.HasMore {
				break
			}
			data = map[string]any{"cursor": p.Cursor, "limit": 500}
		}
		rounds = append(rounds, time.Since(start))
		items = n
	}
	slices.Sort(rounds)
	sum := pages.summary()
	r.row("每轮项数", "%d（期望 6200）", items)
	r.row("单页耗时", "P50 %s，P99 %s，最大 %s（%d 页）", ms(sum.P50), ms(sum.P99), ms(sum.Max), sum.N)
	r.row("整轮耗时", "%s / %s / %s（中位数 %s）", ms(rounds[0]), ms(rounds[1]), ms(rounds[2]), ms(rounds[1]))
	r.Pass = items == 6200 && sum.P99 <= 100*time.Millisecond && rounds[1] <= time.Second
	return r
}

// scenarioC2CFriends：好友制下私聊发送（多了一次关系查询）的 ack 延迟 P99 比开放模式增加 ≤ 2ms（PRD-social 1.3 节）。
func scenarioC2CFriends(cfg config) result {
	r := result{Case: "PRD-social 1.3", Title: "好友制私聊发送不回归", Target: "好友制 ack P99 − 开放模式 ack P99 ≤ 2ms（各 300 条，交替各 3 次取中位数）"}
	run := func(name string, env []string, befriend bool) (summary, error) {
		s := newServer(cfg.bin, cfg.dir, name)
		s.env = env
		if err := s.start(); err != nil {
			return summary{}, err
		}
		defer s.stop()
		tokens, err := setupUsers(s, []string{"alice", "bob"})
		if err != nil {
			return summary{}, err
		}
		if befriend {
			edges := []map[string]any{{"owner": "alice", "peer": "bob"}, {"owner": "bob", "peer": "alice"}}
			if _, err := s.admin(http.MethodPost, "/admin/test/social/seed", map[string]any{"friends": edges}); err != nil {
				return summary{}, err
			}
		}
		ctx := context.Background()
		c, err := dial(ctx, s.wsURL(), "alice", tokens["alice"])
		if err != nil {
			return summary{}, err
		}
		defer c.close()
		var lat latencies
		for i := range 300 {
			_, d, err := c.sendText(ctx, protocol.ConvC2C, "bob", fmt.Sprintf("cf-%s-%04d", name, i), 0)
			if err != nil {
				return summary{}, fmt.Errorf("send %d: %w", i, err)
			}
			lat.add(d)
			time.Sleep(55 * time.Millisecond) // 单用户 20 条/秒限流
		}
		return lat.summary(), nil
	}
	// 两种模式交替各跑 3 次，比较 P99 的中位数（C-48：3 次取中位数，减少 fsync 抖动的影响）。
	var opens, friendsRuns []summary
	for i := range 3 {
		o, err := run(fmt.Sprintf("c2c-open-%d", i), []string{"IM_TEST_MODE=1"}, false)
		if err != nil {
			r.note("开放模式失败：%v", err)
			return r
		}
		f, err := run(fmt.Sprintf("c2c-friends-%d", i), []string{"IM_TEST_MODE=1", `IM_SOCIAL_DEFAULTS={"mode":"friends"}`}, true)
		if err != nil {
			r.note("好友制失败：%v", err)
			return r
		}
		opens, friendsRuns = append(opens, o), append(friendsRuns, f)
	}
	median := func(ss []summary) (p50, p99 time.Duration, all string) {
		var a, b []time.Duration
		for _, s := range ss {
			a, b = append(a, s.P50), append(b, s.P99)
			all += ms(s.P99) + " "
		}
		slices.Sort(a)
		slices.Sort(b)
		return a[1], b[1], all
	}
	o50, o99, oAll := median(opens)
	f50, f99, fAll := median(friendsRuns)
	r.row("开放模式 ack（中位数）", "P50 %s，P99 %s（3 次 P99：%s）", ms(o50), ms(o99), oAll)
	r.row("好友制 ack（中位数）", "P50 %s，P99 %s（3 次 P99：%s）", ms(f50), ms(f99), fAll)
	r.row("P99 增加", "%s", ms(f99-o99))
	r.Pass = f99-o99 <= 2*time.Millisecond
	return r
}
