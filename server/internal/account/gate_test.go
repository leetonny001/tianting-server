package account

import (
	"context"
	"errors"
	"testing"
	"time"
)

func fullGate(t *testing.T) (*Gate, *Faults) {
	t.Helper()
	f := NewFaults()
	f.SetArgonBlock(true)
	t.Cleanup(func() { f.SetArgonBlock(false) })
	g := NewGate(Config{}, f)
	for range 4 {
		go func() {
			if free, err := g.Acquire(context.Background(), GateGeneral); err == nil {
				free()
			}
		}()
	}
	deadline := time.Now().Add(2 * time.Second)
	for f.ArgonBlocked() < 4 {
		if time.Now().After(deadline) {
			t.Fatalf("only %d blocked", f.ArgonBlocked())
		}
		time.Sleep(time.Millisecond)
	}
	return g, f
}

// TC-ACC-01-16（AC-ACC-01-16，闸门部分）：匿名请求占满 4 个位时，再来的匿名请求约 1 秒后 ErrGateBusy，改密用专用位不受影响。
// token/refresh、logout 不进闸门，接口部分随 BE-415、BE-417。
func TestTC_ACC_01_16_GateFull(t *testing.T) {
	g, _ := fullGate(t)
	start := time.Now()
	if _, err := g.Acquire(context.Background(), GateGeneral); !errors.Is(err, ErrGateBusy) {
		t.Fatalf("err = %v", err)
	}
	if d := time.Since(start); d < 900*time.Millisecond || d > 1500*time.Millisecond {
		t.Errorf("queued %v, want about 1s", d)
	}
	start = time.Now()
	free, err := g.Acquire(context.Background(), GatePassword)
	if err != nil {
		t.Fatalf("password slot: %v", err)
	}
	defer free()
	if d := time.Since(start); d > 200*time.Millisecond {
		t.Errorf("password slot took %v", d)
	}
}

// TC-ACC-05-17（AC-ACC-05-18，闸门部分）、TC-ACC-04-23（AC-ACC-04-24，闸门部分）：recover 用自己的专用位，
// 匿名占满普通闸门、改密占住改密专用位都不影响。恢复码命中才用专用位的判断随 BE-418。
func TestTC_ACC_05_17_RecoverSlotIndependent(t *testing.T) {
	g, _ := fullGate(t)
	pw, err := g.Acquire(context.Background(), GatePassword)
	if err != nil {
		t.Fatal(err)
	}
	defer pw()
	start := time.Now()
	free, err := g.AcquireRecover(context.Background(), "u1", 1)
	if err != nil {
		t.Fatalf("recover slot: %v", err)
	}
	if d := time.Since(start); d > 200*time.Millisecond {
		t.Errorf("recover slot took %v", d)
	}
	free()
}

// 专用位各 1 个（第 4.1 节，同时最多 6 个）；recover 排队时限 5 秒可配，超时 ErrGateBusy。
func TestGateDedicatedSlotsAreSingle(t *testing.T) {
	g := NewGate(Config{GateQueue: 50 * time.Millisecond, RecoverQueue: 50 * time.Millisecond}, nil)
	if cap(g.general) != 4 || cap(g.password) != 1 || g.recover.slots != 1 {
		t.Fatalf("slots %d %d %d", cap(g.general), cap(g.password), g.recover.slots)
	}
	free, err := g.Acquire(context.Background(), GatePassword)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.Acquire(context.Background(), GatePassword); !errors.Is(err, ErrGateBusy) {
		t.Errorf("password slot: err = %v", err)
	}
	free()
	free, err = g.AcquireRecover(context.Background(), "u1", 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.AcquireRecover(context.Background(), "u2", 1); !errors.Is(err, ErrGateBusy) {
		t.Errorf("recover slot: err = %v", err)
	}
	free()
}

func TestDefaultConfigMatchesPRD(t *testing.T) {
	c := Config{}.WithDefaults()
	if c.RegisterMode != ModeInvite || c.AccessTTL != 24*time.Hour || c.RefreshTTL != 90*24*time.Hour || c.RefreshTTLWeb != 24*time.Hour ||
		c.RefreshAbsolute != 365*24*time.Hour || c.ArgonSlots != 4 || c.RecoverSlots != 1 || c.GateQueue != time.Second || c.RecoverQueue != 5*time.Second {
		t.Errorf("%+v", c)
	}
	if c.Argon != (ArgonParams{MemoryKiB: 64 * 1024, Time: 3, Threads: 1}) {
		t.Errorf("argon %+v", c.Argon)
	}
}

func TestHashRoundTrip(t *testing.T) {
	light := ArgonParams{MemoryKiB: 64, Time: 1, Threads: 1}
	h, err := hashPassword(light, "pw-密码")
	if err != nil {
		t.Fatal(err)
	}
	if ok, stale, err := verifyPassword(light, h, "pw-密码"); !ok || stale || err != nil {
		t.Errorf("ok=%v stale=%v err=%v", ok, stale, err)
	}
	if ok, _, _ := verifyPassword(light, h, "pw-密码x"); ok {
		t.Error("wrong password must fail")
	}
	if _, stale, _ := verifyPassword(ArgonParams{MemoryKiB: 128, Time: 1, Threads: 1}, h, "pw-密码"); !stale {
		t.Error("old parameters must be stale")
	}
	if _, _, err := verifyPassword(light, "plain", "x"); err == nil {
		t.Error("a non argon2 value must be an error")
	}
}
