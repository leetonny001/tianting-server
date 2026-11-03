package social

import (
	"context"
	"testing"
	"time"

	"tianting/server/internal/cache"
	"tianting/server/internal/clock"
)

func TestDefaultsAreValid(t *testing.T) {
	if err := Defaults().Validate(); err != nil {
		t.Fatal(err)
	}
}

// AC-SOC-01-3：超范围、未知字段、null、mode 非法都整次不生效。
func TestPatch(t *testing.T) {
	base := Defaults()
	got, err := Patch(base, []byte(`{"mode":"friends","strangerMsgLimit":3}`))
	if err != nil || got.Mode != ModeFriends || got.StrangerMsgLimit != 3 || got.MaxFriends != 2000 {
		t.Fatalf("patch = %+v, %v", got, err)
	}
	for _, bad := range []string{`{"mode":"friends","maxFriends":99}`, `{"strangerMsgLimit":-1}`, `{"foo":1}`, `{"mode":"x"}`,
		`{"mode":null}`, `[]`, `null`, `{"newPeerRejectDailyLimit":0}`, `{"maxFriends":1.5}`} {
		if out, err := Patch(base, []byte(bad)); err == nil || out != base {
			t.Errorf("%s: err=%v out=%+v", bad, err, out)
		}
	}
}

type fakeStore struct{ v string }

func (f *fakeStore) SocialSettingsJSON(context.Context) (string, bool, error) { return f.v, f.v != "", nil }

// F-SOC-01 规则 3：失效通知立即生效；漏掉通知时 60 秒 TTL 兜底。
func TestCacheInvalidationAndTTL(t *testing.T) {
	clk := clock.New()
	clk.Freeze(time.UnixMilli(1_800_000_000_000))
	st := &fakeStore{}
	inv := cache.NewLocal()
	c := NewCache(st, clk, inv)
	ctx := context.Background()
	if s, _ := c.Get(ctx); s.Mode != ModeOpen {
		t.Fatal("default mode")
	}
	st.v = `{"mode":"friends"}`
	if s, _ := c.Get(ctx); s.Mode != ModeOpen {
		t.Fatal("should still be cached")
	}
	inv.Invalidate(TopicSettings, "")
	if s, _ := c.Get(ctx); s.Mode != ModeFriends {
		t.Fatal("invalidation should take effect immediately")
	}
	st.v = `{"mode":"open"}`
	clk.Freeze(clk.Now().Add(59 * time.Second))
	if s, _ := c.Get(ctx); s.Mode != ModeFriends {
		t.Fatal("59s: still cached")
	}
	clk.Freeze(clk.Now().Add(2 * time.Second))
	if s, _ := c.Get(ctx); s.Mode != ModeOpen {
		t.Fatal("61s: TTL should expire")
	}
}
