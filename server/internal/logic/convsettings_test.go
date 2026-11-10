package logic

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"tianting/server/internal/cache"
	"tianting/server/internal/clock"
	"tianting/server/internal/metrics"
	"tianting/server/internal/protocol"
	"tianting/server/internal/ratelimit"
	"tianting/server/internal/store"
)

// C5 的 logic 单测（BE-483 第 4 条）：会话设置失效广播的发布端，用记录型 Invalidator 断言。

type recordingInv struct {
	*cache.Local
	mu  sync.Mutex
	got []string // topic:key
}

func (r *recordingInv) Invalidate(topic, key string) {
	r.mu.Lock()
	r.got = append(r.got, topic+":"+key)
	r.mu.Unlock()
	r.Local.Invalidate(topic, key)
}

func (r *recordingInv) take() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.got
	r.got = nil
	return out
}

func (r *recordingInv) published(uid string) bool {
	return slices.Contains(r.take(), TopicConvSettings+":"+uid)
}

type nopRouter struct{}

func (nopRouter) DeliverToUser(string, []byte, string) int          { return 0 }
func (nopRouter) DeliverToUserExceptSID(string, []byte, string) int { return 0 }
func (nopRouter) KickUser(string, string)                           {}
func (nopRouter) SuspendUser(string, *int64)                        {}
func (nopRouter) KickSession(string, string, string, *int64)        {}
func (nopRouter) KickStaleGen(string, string, int64)                {}
func (nopRouter) KickUserExcept(string, string, string, *int64)     {}

type testSession struct{ uid, conn string }

func (s testSession) ConnID() string { return s.conn }
func (s testSession) UserID() string { return s.uid }
func (s testSession) SID() string    { return "" }
func (s testSession) Gen() int64     { return 0 }

func newCVSService(t *testing.T, cfg ConvSettingsConfig) (*Service, store.Store, *recordingInv) {
	t.Helper()
	st, err := store.OpenSQLite(filepath.Join(t.TempDir(), "im.db"), store.SQLiteOptions{AutoMigrate: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	clk := clock.New()
	clk.Freeze(time.UnixMilli(1_800_000_000_000))
	inv := &recordingInv{Local: cache.NewLocal()}
	s := New(st, nopRouter{}, clk, nil, ratelimit.New(1000, 1000, clk), inv, metrics.New(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	s.ConfigureConvSettings(cfg)
	return s, st, inv
}

func call(t *testing.T, s *Service, uid, cmd string, data any) *protocol.OutFrame {
	t.Helper()
	raw, _ := json.Marshal(data)
	return s.Handle(context.Background(), testSession{uid: uid, conn: uid + "-c1"}, protocol.InFrame{ID: "r1", Cmd: cmd, Data: raw})
}

// 退群、被移出、解散的清理提交后发布 convSettings（key 是 userId）：主动退群、群主移出、客户端解散、后台移除和解散（NotifyConvRemoved）。
func TestConvSettingsInvalidationOnLeave(t *testing.T) {
	s, st, inv := newCVSService(t, ConvSettingsConfig{Enabled: true})
	ctx := context.Background()
	if err := st.UpsertUsers(ctx, []store.User{{UserID: "o"}, {UserID: "a"}, {UserID: "b"}, {UserID: "c"}}); err != nil {
		t.Fatal(err)
	}
	if err := st.CreateGroup(ctx, store.Group{GroupID: "g1", Name: "g1", CreatedAt: 1}, []string{"a", "b"}); err != nil {
		t.Fatal(err)
	}
	inv.take()
	if f := call(t, s, "a", protocol.CmdGroupLeave, map[string]any{"groupId": "g1"}); f.Error != nil {
		t.Fatalf("leave: %+v", f.Error)
	}
	if !inv.published("a") {
		t.Error("groupLeave did not publish convSettings")
	}
	s.NotifyConvRemoved("group:g1", []string{"b", "c"}, protocol.RemovedByAdmin) // 后台移除、解散、群主移出都走这里
	got := inv.take()
	if !slices.Contains(got, TopicConvSettings+":b") || !slices.Contains(got, TopicConvSettings+":c") {
		t.Errorf("NotifyConvRemoved published %v", got)
	}
}

// convSet (and convDelete, BE-485) publish convSettings after the commit; rejected requests do not.
func TestConvSettingsInvalidationOnCommands(t *testing.T) {
	s, st, inv := newCVSService(t, ConvSettingsConfig{Enabled: true, RatePerSec: 1000})
	ctx := context.Background()
	if _, _, err := st.SaveMessage(ctx, store.Message{MsgID: "m1", ClientMsgID: "c1", ConvID: "c2c:a:b", From: "a", Type: "text",
		Body: []byte(`{"text":"hi"}`), TS: 1, ConvUsers: []string{"a", "b"}}); err != nil {
		t.Fatal(err)
	}
	inv.take()
	if f := call(t, s, "a", protocol.CmdConvSet, map[string]any{"convId": "c2c:b:x", "pinned": true}); f.Error == nil || f.Error.Code != protocol.CodeNotConvParticipant {
		t.Fatalf("non-participant: %+v", f.Error)
	}
	if got := inv.take(); len(got) != 0 {
		t.Errorf("rejected convSet published %v", got)
	}
	if f := call(t, s, "a", protocol.CmdConvSet, map[string]any{"convId": "c2c:a:b", "notifyOff": true}); f.Error != nil {
		t.Fatalf("convSet: %+v", f.Error)
	}
	if !inv.published("a") {
		t.Error("convSet did not publish convSettings")
	}
}
