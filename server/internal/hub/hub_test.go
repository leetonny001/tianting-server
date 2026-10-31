package hub

import (
	"sync"
	"testing"
)

type fakeClient struct {
	id     uint64
	uid    string
	mu     sync.Mutex
	got    [][]byte
	fail   bool
	kicked string
}

func (f *fakeClient) ConnID() uint64 { return f.id }
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
	phone := &fakeClient{id: 1, uid: "u1"}
	pc := &fakeClient{id: 2, uid: "u1"}
	other := &fakeClient{id: 3, uid: "u2"}
	h.Register(phone)
	h.Register(pc)
	h.Register(other)

	if n := h.SendToUser("u1", []byte("x"), 1); n != 1 {
		t.Fatalf("delivered %d, want 1", n)
	}
	if phone.count() != 0 || pc.count() != 1 || other.count() != 0 {
		t.Errorf("phone=%d pc=%d other=%d", phone.count(), pc.count(), other.count())
	}
	if n := h.SendToUser("u1", []byte("y"), 0); n != 2 {
		t.Errorf("delivered %d, want 2", n)
	}
}

func TestUnregisterAndFailedSend(t *testing.T) {
	h := New()
	a := &fakeClient{id: 1, uid: "u1"}
	b := &fakeClient{id: 2, uid: "u1", fail: true}
	h.Register(a)
	h.Register(b)

	if n := h.SendToUser("u1", []byte("x"), 0); n != 1 {
		t.Errorf("delivered %d, want 1 (failed send not counted)", n)
	}
	h.Unregister(a)
	h.Unregister(b)
	if h.ConnCount("u1") != 0 {
		t.Error("user should be offline")
	}
	if n := h.SendToUser("u1", []byte("x"), 0); n != 0 {
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
			c := &fakeClient{id: uint64(i + 1), uid: "u"}
			h.Register(c)
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
	a := &fakeClient{id: 1, uid: "u1"}
	b := &fakeClient{id: 2, uid: "u1"}
	other := &fakeClient{id: 3, uid: "u2"}
	h.Register(a)
	h.Register(b)
	h.Register(other)
	if n := h.KickUser("u1", "banned"); n != 2 {
		t.Fatalf("kicked %d, want 2", n)
	}
	if a.kicked != "banned" || b.kicked != "banned" || other.kicked != "" {
		t.Errorf("a=%q b=%q other=%q", a.kicked, b.kicked, other.kicked)
	}
}
