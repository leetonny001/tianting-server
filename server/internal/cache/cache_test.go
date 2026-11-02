package cache

import "testing"

func TestLocalInvalidate(t *testing.T) {
	l := NewLocal()
	var got []string
	l.Subscribe("group", func(k string) { got = append(got, "a:"+k) })
	l.Subscribe("group", func(k string) { got = append(got, "b:"+k) })
	l.Subscribe("user", func(k string) { got = append(got, "user:"+k) })
	l.Invalidate("group", "g1")
	if len(got) != 2 || got[0] != "a:g1" || got[1] != "b:g1" {
		t.Fatalf("got %v", got)
	}
	l.Invalidate("nobody", "x") // 没有订阅者也不报错
}
