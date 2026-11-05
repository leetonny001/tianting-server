package sessions

import "testing"

func TestMaskIP(t *testing.T) {
	for in, want := range map[string]string{
		"10.1.2.3":             "10.1.*.*",
		"::ffff:10.1.2.3":      "10.1.*.*",
		"2001:db8:1:2:3:4:5:6": "2001:db8:1:*:*:*:*:*",
		"2001:0db8::1":         "2001:db8:0:*:*:*:*:*",
		"fe80::1%en0":          "fe80:0:0:*:*:*:*:*",
		"::1":                  "0:0:0:*:*:*:*:*",
		"":                     "*",
		"not-an-ip":            "*",
		"127.0.0.1:1234":       "*",
	} {
		if got := MaskIP(in); got != want {
			t.Errorf("MaskIP(%q) = %q, want %q", in, got, want)
		}
	}
}

type fakeSource []Conn

func (f fakeSource) ListUserConns(string) []Conn    { return f }
func (f fakeSource) ConnCount(string) int           { return len(f) }
func (f fakeSource) PlatformCounts() map[string]int { return nil }

func TestQuerySortsAndMarksPartial(t *testing.T) {
	r := Query(fakeSource{{ConnID: "b", ConnectedAt: 2}, {ConnID: "c", ConnectedAt: 1}, {ConnID: "a", ConnectedAt: 2}}, "u")
	var got []string
	for _, c := range r.Sessions {
		got = append(got, c.ConnID)
	}
	if len(got) != 3 || got[0] != "c" || got[1] != "a" || got[2] != "b" {
		t.Errorf("order = %v", got)
	}
	if !r.Partial || r.Scope != ScopeLocalNode {
		t.Errorf("partial = %v scope = %q", r.Partial, r.Scope)
	}
	if empty := Query(fakeSource(nil), "u"); empty.Sessions == nil {
		t.Error("empty result must marshal to [] not null")
	}
}
