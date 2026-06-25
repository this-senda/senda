package app

import "testing"

func TestIsNewer(t *testing.T) {
	cases := []struct {
		current, latest string
		want            bool
	}{
		{"1.2.3", "1.3.0", true},       // minor bump
		{"1.2.3", "1.2.4", true},       // patch bump
		{"1.2.3", "2.0.0", true},       // major bump
		{"1.2.3", "1.2.3", false},      // same
		{"1.3.0", "1.2.9", false},      // older release out (shouldn't downgrade)
		{"v1.2.3", "v1.3.0", true},     // tolerant of leading v on both
		{"1.2.3", "1.2.3-rc.1", false}, // prerelease is lower than release
		{"1.2.3-rc.1", "1.2.3", true},  // release supersedes its prerelease
		{"dev", "1.2.3", false},        // dev build never reports an update
		{"1.2.3", "garbage", false},    // unparseable latest
	}
	for _, c := range cases {
		if got := isNewer(c.current, c.latest); got != c.want {
			t.Errorf("isNewer(%q, %q) = %v, want %v", c.current, c.latest, got, c.want)
		}
	}
}
