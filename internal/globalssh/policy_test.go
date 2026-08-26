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
