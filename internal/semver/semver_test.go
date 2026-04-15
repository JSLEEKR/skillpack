package semver

import "testing"

func TestNormalize(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"1.2.3", "v1.2.3"},
		{"v1.2.3", "v1.2.3"},
		{"  v1.2.3 ", "v1.2.3"},
		{"1.2.3-rc.1", "v1.2.3-rc.1"},
		{"", ""},
		{"not-a-version", ""},
		{"1.2", "v1.2"}, // x/mod/semver tolerates a missing patch
	}
	for _, tc := range tests {
		if got := Normalize(tc.in); got != tc.want {
			t.Errorf("Normalize(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestDisplay(t *testing.T) {
	if got := Display("v1.2.3"); got != "1.2.3" {
		t.Errorf("Display(v1.2.3) = %q", got)
	}
	if got := Display("1.2.3"); got != "1.2.3" {
		t.Errorf("Display(1.2.3) = %q", got)
	}
	if got := Display("garbage"); got != "garbage" {
		t.Errorf("Display(garbage) should pass through, got %q", got)
	}
}

func TestIsValid(t *testing.T) {
	cases := map[string]bool{
		"1.2.3":      true,
		"v1.2.3":     true,
		"1.0.0-rc.1": true,
		"":           false,
		"abc":        false,
	}
	for in, want := range cases {
		if got := IsValid(in); got != want {
			t.Errorf("IsValid(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestCompare(t *testing.T) {
	if Compare("1.2.3", "1.2.4") >= 0 {
		t.Errorf("1.2.3 should be < 1.2.4")
	}
	if Compare("v2.0.0", "1.999.999") <= 0 {
		t.Errorf("2.0.0 should be > 1.999.999")
	}
	if Compare("1.2.3", "v1.2.3") != 0 {
		t.Errorf("v-prefix should not affect equality")
	}
}

func TestMajorMinor(t *testing.T) {
	if got := Major("1.2.3"); got != "1" {
		t.Errorf("Major = %q", got)
	}
	if got := MajorMinor("1.2.3"); got != "1.2" {
		t.Errorf("MajorMinor = %q", got)
	}
	if got := Major("garbage"); got != "" {
		t.Errorf("Major(garbage) = %q", got)
	}
}

func TestMatchAny(t *testing.T) {
	for _, c := range []string{"*", "", "latest", "any"} {
		ok, err := Match("1.2.3", c)
		if err != nil || !ok {
			t.Errorf("Match(1.2.3, %q) = %v, %v", c, ok, err)
		}
	}
}

func TestMatchExact(t *testing.T) {
	ok, _ := Match("1.2.3", "1.2.3")
	if !ok {
		t.Errorf("exact match failed")
	}
	ok, _ = Match("1.2.3", "1.2.4")
	if ok {
		t.Errorf("exact mismatch should fail")
	}
	ok, _ = Match("1.2.3", "=1.2.3")
	if !ok {
		t.Errorf("=1.2.3 should match 1.2.3")
	}
}

func TestMatchOps(t *testing.T) {
	tests := []struct {
		v, c string
		want bool
	}{
		{"1.2.3", ">=1.2.3", true},
		{"1.2.3", ">=1.2.4", false},
		{"1.2.4", ">1.2.3", true},
		{"1.2.3", ">1.2.3", false},
		{"1.2.3", "<=1.2.3", true},
		{"1.2.3", "<=1.2.2", false},
		{"1.2.2", "<1.2.3", true},
		{"1.2.3", "<1.2.3", false},
	}
	for _, tc := range tests {
		got, err := Match(tc.v, tc.c)
		if err != nil {
			t.Errorf("Match(%q, %q) err: %v", tc.v, tc.c, err)
		}
		if got != tc.want {
			t.Errorf("Match(%q, %q) = %v, want %v", tc.v, tc.c, got, tc.want)
		}
	}
}

func TestMatchCaret(t *testing.T) {
	tests := []struct {
		v, c string
		want bool
	}{
		{"1.2.3", "^1.2.3", true},
		{"1.2.4", "^1.2.3", true},
		{"1.9.9", "^1.2.3", true},
		{"2.0.0", "^1.2.3", false},
		{"1.2.2", "^1.2.3", false},
		// 0.x special case
		{"0.2.3", "^0.2.3", true},
		{"0.2.4", "^0.2.3", true},
		{"0.3.0", "^0.2.3", false},
		// 0.0.x special case (M1 regression): matches npm/cargo/semver.org —
		// ^0.0.x is a PATCH-only bound because any 0.0 bump is breaking.
		{"0.0.1", "^0.0.1", true},
		{"0.0.2", "^0.0.1", false}, // before the fix, this was true (too loose)
		{"0.1.0", "^0.0.1", false},
		{"0.0.3", "^0.0.3", true},
		{"0.0.4", "^0.0.3", false},
	}
	for _, tc := range tests {
		got, err := Match(tc.v, tc.c)
		if err != nil {
			t.Errorf("Match(%q, %q) err: %v", tc.v, tc.c, err)
		}
		if got != tc.want {
			t.Errorf("Match(%q, %q) = %v, want %v", tc.v, tc.c, got, tc.want)
		}
	}
}

func TestMatchTilde(t *testing.T) {
	tests := []struct {
		v, c string
		want bool
	}{
		{"1.2.3", "~1.2.3", true},
		{"1.2.4", "~1.2.3", true},
		{"1.2.99", "~1.2.3", true},
		{"1.3.0", "~1.2.3", false},
		{"1.2.2", "~1.2.3", false},
	}
	for _, tc := range tests {
		got, _ := Match(tc.v, tc.c)
		if got != tc.want {
			t.Errorf("Match(%q, %q) = %v, want %v", tc.v, tc.c, got, tc.want)
		}
	}
}

func TestMatchXRange(t *testing.T) {
	tests := []struct {
		v, c string
		want bool
	}{
		{"1.2.3", "1.x", true},
		{"1.99.99", "1.x", true},
		{"2.0.0", "1.x", false},
		{"1.2.3", "1.2.x", true},
		{"1.2.99", "1.2.x", true},
		{"1.3.0", "1.2.x", false},
		{"1.2.3", "1.2.X", true}, // capital X
	}
	for _, tc := range tests {
		got, err := Match(tc.v, tc.c)
		if err != nil {
			t.Errorf("Match(%q, %q) err: %v", tc.v, tc.c, err)
		}
		if got != tc.want {
			t.Errorf("Match(%q, %q) = %v, want %v", tc.v, tc.c, got, tc.want)
		}
	}
}

func TestMatchInvalidVersion(t *testing.T) {
	_, err := Match("garbage", "^1.0.0")
	if err == nil {
		t.Errorf("expected error on invalid version")
	}
}

func TestMatchInvalidConstraint(t *testing.T) {
	_, err := Match("1.0.0", "^garbage")
	if err == nil {
		t.Errorf("expected error on invalid constraint")
	}
}

func TestBestMatch(t *testing.T) {
	cands := []string{"1.0.0", "1.2.0", "1.2.3", "2.0.0", "0.9.0"}
	got, err := BestMatch(cands, "^1.0.0")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "1.2.3" {
		t.Errorf("BestMatch = %q, want 1.2.3", got)
	}
}

func TestBestMatchNone(t *testing.T) {
	got, err := BestMatch([]string{"1.0.0", "1.1.0"}, "^2.0.0")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "" {
		t.Errorf("BestMatch = %q, want empty", got)
	}
}

func TestBestMatchVPrefixCandidates(t *testing.T) {
	// Mixed v-prefix should still work.
	cands := []string{"v1.0.0", "1.2.3", "v1.5.0"}
	got, _ := BestMatch(cands, "^1.0.0")
	if got != "v1.5.0" && got != "1.5.0" {
		t.Errorf("BestMatch = %q, want highest v1.5.0", got)
	}
}

func TestComparePanicsOnInvalid(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Compare should panic on invalid input")
		}
	}()
	Compare("garbage", "1.0.0")
}
