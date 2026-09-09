package globalssh

import "testing"

func TestWritableToHostStar(t *testing.T) {
	for _, p := range Policy {
		want := p.Key != "IdentitiesOnly"
		if got := p.WritableToHostStar(); got != want {
			t.Errorf("%s WritableToHostStar() = %v, want %v", p.Key, got, want)
		}
	}
}

// TestValueKindNonZero (RED first, plan 09.6-01 Task 1 behavior Test 1):
// every row in globalssh.Policy carries a non-zero value kind — a row added
// later that forgets to declare one fails this test by construction.
func TestValueKindNonZero(t *testing.T) {
	for _, p := range Policy {
		if p.Kind == "" {
			t.Errorf("%s Kind is empty", p.Key)
		}
	}
}

// TestValueKindEnumHasValues (RED first, plan 09.6-01 Task 1 behavior Test 3):
// every enum-kind row carries at least two values AND its own Recommended
// appears among them; every non-enum row carries an empty value list.
func TestValueKindEnumHasValues(t *testing.T) {
	for _, p := range Policy {
		if p.Kind == OptionValueKindEnum {
			if len(p.Values) < 2 {
				t.Errorf("%s enum-kind but only %d values declared", p.Key, len(p.Values))
			}
			found := false
			for _, v := range p.Values {
				if v == p.Recommended {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%s Recommended %q not in Values %v", p.Key, p.Recommended, p.Values)
			}
		} else {
			if len(p.Values) != 0 {
				t.Errorf("%s non-enum but has Values %v", p.Key, p.Values)
			}
		}
	}
}

// TestValueKindPlatformIndependentUseKeychain (RED first, plan 09.6-01 Task 1 behavior Test 3b, PD44):
// the SSH row whose policy entry carries the darwin platform restriction
// (internal/globalssh/policy.go:37) classifies as apply-or-not on EVERY
// platform, asserted by reading the kind straight off the policy table with
// no platform predicate consulted and with no build tag or runtime OS branch
// in the test.
func TestValueKindPlatformIndependentUseKeychain(t *testing.T) {
	p, ok := PolicyFor("UseKeychain")
	if !ok {
		t.Fatal("UseKeychain not found in Policy")
	}
	if p.Platform != "darwin" {
		t.Fatalf("UseKeychain Platform=%q, want darwin", p.Platform)
	}
	if p.Kind != OptionValueKindToggle {
		t.Errorf("UseKeychain Kind=%q, want toggle (platform-independent classification)", p.Kind)
	}
}
