package hub

import (
	"strconv"
	"sync"
	"testing"
)

type fakeClient struct {
	id     string
	uid    string
	mu     sync.Mutex
	got    [][]byte
	fail   bool
	kicked string
}

func (f *fakeClient) ConnID() string { return f.id }
func (f *fakeClient) UserID() string { return f.uid }
func (f *fakeClient) Send(b []byte) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail {
		return false
	}
	f.got = append(f.got, b)
	return true
}
func (f *fakeClient) Kick(reason string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.kicked = reason
}
func (f *fakeClient) count() int { f.mu.Lock(); defer f.mu.Unlock(); return len(f.got) }

func TestSendToUserReachesAllDevicesExceptSender(t *testing.T) {
	h := New()
	phone := &fakeClient{id: "1", uid: "u1"}
	pc := &fakeClient{id: "2", uid: "u1"}
	other := &fakeClient{id: "3", uid: "u2"}
	h.Register(phone, 0)
	h.Register(pc, 0)
	h.Register(other, 0)

	if n := h.SendToUser("u1", []byte("x"), "1"); n != 1 {
		t.Fatalf("delivered %d, want 1", n)
	}
	if phone.count() != 0 || pc.count() != 1 || other.count() != 0 {
		t.Errorf("phone=%d pc=%d other=%d", phone.count(), pc.count(), other.count())
	}
	if n := h.SendToUser("u1", []byte("y"), ""); n != 2 {
		t.Errorf("delivered %d, want 2", n)
	}
}

func TestUnregisterAndFailedSend(t *testing.T) {
	h := New()
	a := &fakeClient{id: "1", uid: "u1"}
	b := &fakeClient{id: "2", uid: "u1", fail: true}
	h.Register(a, 0)
	h.Register(b, 0)

	if n := h.SendToUser("u1", []byte("x"), ""); n != 1 {
		t.Errorf("delivered %d, want 1 (failed send not counted)", n)
	}
	h.Unregister(a)
	h.Unregister(b)
	if h.ConnCount("u1") != 0 {
		t.Error("user should be offline")
	}
	if n := h.SendToUser("u1", []byte("x"), ""); n != 0 {
		t.Errorf("offline user got %d deliveries", n)
	}
}

func TestConcurrentAccess(t *testing.T) {
	h := New()
	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := &fakeClient{id: strconv.Itoa(i + 1), uid: "u"}
			h.Register(c, 0)
			h.SendToUser("u", []byte("x"), c.id)
			h.Unregister(c)
		}()
	}
	wg.Wait()
	if h.ConnCount("u") != 0 {
		t.Error("all connections should be gone")
	}
}

func TestKickUser(t *testing.T) {
	h := New()
	a := &fakeClient{id: "1", uid: "u1"}
	b := &fakeClient{id: "2", uid: "u1"}
	other := &fakeClient{id: "3", uid: "u2"}
	h.Register(a, 0)
	h.Register(b, 0)
	h.Register(other, 0)
	if n := h.KickLocal("u1", "banned"); n != 2 {
		t.Fatalf("kicked %d, want 2", n)
	}
	if a.kicked != "banned" || b.kicked != "banned" || other.kicked != "" {
		t.Errorf("a=%q b=%q other=%q", a.kicked, b.kicked, other.kicked)
	}
}

// ARCH S6：超过上限时移出并返回最早的连接；之后不再收到投递。
func TestRegisterEvictsOldest(t *testing.T) {
	h := New()
	cs := make([]*fakeClient, 4)
	for i := range cs {
		cs[i] = &fakeClient{id: strconv.Itoa(i), uid: "u"}
	}
	for _, c := range cs[:3] {
		if ev := h.Register(c, 3); len(ev) != 0 {
			t.Fatalf("under limit evicted %v", ev)
		}
	}
	ev := h.Register(cs[3], 3)
	if len(ev) != 1 || ev[0] != cs[0] {
		t.Fatalf("evicted %v, want the oldest", ev)
	}
	if h.ConnCount("u") != 3 {
		t.Errorf("count = %d", h.ConnCount("u"))
	}
	h.SendToUser("u", []byte("x"), "")
	if cs[0].count() != 0 || cs[3].count() != 1 {
		t.Error("evicted connection must not receive frames")
	}
	h.Unregister(cs[0]) // 被踢的连接关闭时还会调用 Unregister，不能影响其他连接
	if h.ConnCount("u") != 3 {
		t.Errorf("count after late unregister = %d", h.ConnCount("u"))
	}
}

func TestKickAll(t *testing.T) {
	h := New()
	a, b := &fakeClient{id: "1", uid: "u1"}, &fakeClient{id: "2", uid: "u2"}
	h.Register(a, 0)
	h.Register(b, 0)
	if n := h.KickAll("serverShutdown"); n != 2 || a.kicked != "serverShutdown" || b.kicked != "serverShutdown" {
		t.Fatalf("n=%d a=%q b=%q", n, a.kicked, b.kicked)
	}
}
