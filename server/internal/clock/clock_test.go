package clock

import (
	"testing"
	"time"
)

func TestFreezeAndUnfreeze(t *testing.T) {
	c := New()
	if d := time.Since(c.Now()); d < 0 || d > time.Second {
		t.Fatalf("default clock is not real time: %v", d)
	}
	at := time.UnixMilli(1_700_000_000_000)
	c.Freeze(at)
	time.Sleep(5 * time.Millisecond)
	if !c.Now().Equal(at) {
		t.Fatalf("frozen clock moved: %v", c.Now())
	}
	c.Unfreeze()
	if time.Since(c.Now()) > time.Second {
		t.Fatal("unfreeze did not restore real time")
	}
}
