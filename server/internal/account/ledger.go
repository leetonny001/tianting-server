package account

import (
	"context"
	"fmt"
	"slices"
	"time"
)

// refresh 密钥台账（F-ACC-03 规则 3，RS5-04）：退役条件是 IM_REFRESH_ABSOLUTE（默认 365 天），不是 JWT 的 30 天。
// 提前删除密钥会让持它签发的 refresh token 全部变成 invalid，等于全员下线，所以配置检查要拦。

// LedgerProblems 列出配置里没有、但台账显示还不能退役的密钥。泄露应急用 `im-server refresh-keys forget <kid>` 明确放弃。
func (s *Service) LedgerProblems(ctx context.Context) ([]string, error) {
	entries, err := s.Store.RefreshKeyLedger(ctx)
	if err != nil {
		return nil, err
	}
	now := s.Clock.Now()
	var out []string
	for _, e := range entries {
		if e.Forgotten != 0 || slices.ContainsFunc(s.cfg.RefreshKeys, func(k RefreshKey) bool { return k.ID == e.KID }) {
			continue
		}
		if e.RetiredAt == 0 {
			out = append(out, fmt.Sprintf("refresh key %q is the last active key in the ledger but is no longer configured", e.KID))
			continue
		}
		if until := time.UnixMilli(e.RetiredAt).Add(s.cfg.RefreshAbsolute); now.Before(until) {
			out = append(out, fmt.Sprintf("refresh key %q was retired less than IM_REFRESH_ABSOLUTE ago and must stay configured until %s", e.KID, until.UTC().Format(time.DateOnly)))
		}
	}
	return out, nil
}

// SyncLedger 把现在的配置写进台账（启动时调用）。
func (s *Service) SyncLedger(ctx context.Context) error {
	if len(s.cfg.RefreshKeys) == 0 {
		return nil
	}
	ids := make([]string, 0, len(s.cfg.RefreshKeys))
	for _, k := range s.cfg.RefreshKeys {
		ids = append(ids, k.ID)
	}
	return s.Store.SyncRefreshKeyLedger(ctx, ids, s.cfg.RefreshActive, s.Clock.Now().UnixMilli())
}
