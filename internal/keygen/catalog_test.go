package keygen

import (
	"strings"
	"testing"
)

// TestCatalog_HasFiveEntries asserts Catalog returns exactly the 5 KEY-01
// top-5 algorithms.
func TestCatalog_HasFiveEntries(t *testing.T) {
	cat := Catalog()
	if len(cat) != 5 {
		t.Fatalf("len(Catalog()) = %d, want 5", len(cat))
	}
	wantNames := map[string]bool{
		"ed25519":    true,
		"ed25519-sk": true,
		"rsa-4096":   true,
		"ecdsa-p256": true,
		"ecdsa-sk":   true,
	}
	for _, a := range cat {
		if !wantNames[a.Name] {
			t.Errorf("Catalog() contains unexpected entry %q", a.Name)
		}
		delete(wantNames, a.Name)
	}
	if len(wantNames) != 0 {
		t.Errorf("Catalog() is missing entries: %v", wantNames)
	}
}

// TestCatalog_ExactlyOneDefault asserts exactly one entry is Default==true
// and it is ed25519 (D-06).
func TestCatalog_ExactlyOneDefault(t *testing.T) {
	cat := Catalog()
	var defaults []string
	for _, a := range cat {
		if a.Default {
			defaults = append(defaults, a.Name)
		}
	}
	if len(defaults) != 1 {
		t.Fatalf("Catalog() has %d Default=true entries (%v), want exactly 1", len(defaults), defaults)
	}
	if defaults[0] != "ed25519" {
		t.Errorf("Catalog() default = %q, want ed25519", defaults[0])
	}
}

// TestCatalog_ImplementedFlags asserts ed25519 and rsa-4096 are
// Implemented==true and the other three (the registry's not-yet-implemented
// stubs) are Implemented==false — matching the registry.go dispatch table.
func TestCatalog_ImplementedFlags(t *testing.T) {
	wantImplemented := map[string]bool{
		"ed25519":    true,
		"rsa-4096":   true,
		"ecdsa-p256": false,
		"ed25519-sk": false,
		"ecdsa-sk":   false,
	}
	for _, a := range Catalog() {
		want, ok := wantImplemented[a.Name]
		if !ok {
			t.Fatalf("unexpected catalog entry %q", a.Name)
		}
		if a.Implemented != want {
			t.Errorf("Catalog() entry %q Implemented = %v, want %v", a.Name, a.Implemented, want)
		}
	}
}

// TestCatalog_EntriesCarryMetadata asserts every entry carries a query
// token, a security note, and per-OS availability/variant notes — the fields
// KEY-01 requires, independent of final ordering/copy (deferred to Phase 2
// design, D-06).
func TestCatalog_EntriesCarryMetadata(t *testing.T) {
	for _, a := range Catalog() {
		if a.QueryToken == "" {
			t.Errorf("Catalog() entry %q has empty QueryToken", a.Name)
		}
		if a.Security == "" {
			t.Errorf("Catalog() entry %q has empty Security note", a.Name)
		}
		if a.DarwinNote == "" {
			t.Errorf("Catalog() entry %q has empty DarwinNote", a.Name)
		}
		if a.LinuxNote == "" {
			t.Errorf("Catalog() entry %q has empty LinuxNote", a.Name)
		}
	}
}

// TestCatalog_QueryTokensMatchProtocolIdentifiers asserts the -sk entries use
// the real `ssh -Q key` protocol tokens (RESEARCH Pitfall 2: the human name
// "ed25519-sk" is NOT the wire token; it is "sk-ssh-ed25519@openssh.com").
func TestCatalog_QueryTokensMatchProtocolIdentifiers(t *testing.T) {
	wantToken := map[string]string{
		"ed25519":    "ssh-ed25519",
		"rsa-4096":   "ssh-rsa",
		"ecdsa-p256": "ecdsa-sha2-nistp256",
		"ed25519-sk": "sk-ssh-ed25519@openssh.com",
		"ecdsa-sk":   "sk-ecdsa-sha2-nistp256@openssh.com",
	}
	for _, a := range Catalog() {
		if got, want := a.QueryToken, wantToken[a.Name]; got != want {
			t.Errorf("Catalog() entry %q QueryToken = %q, want %q", a.Name, got, want)
		}
	}
}

// TestCatalog_ImplementedFalseEntriesErrorOnGeneration asserts that for every
// catalog entry with Implemented==false, keygen.GenerateMaterial cannot
// select it as if it were generatable — the generation path always errors
// with a not-yet-implemented message (T-01-21).
func TestCatalog_ImplementedFalseEntriesErrorOnGeneration(t *testing.T) {
	for _, a := range Catalog() {
		if a.Implemented {
			continue
		}
		_, err := GenerateMaterial(Params{Algo: a.Name, Identity: "x", Comment: "x@gitid"})
		if err == nil {
			t.Errorf("GenerateMaterial(%q) (Implemented=false) returned nil error, want not-yet-implemented error", a.Name)
			continue
		}
		if !strings.Contains(err.Error(), "not yet implemented") {
			t.Errorf("GenerateMaterial(%q) error = %q, want to contain \"not yet implemented\"", a.Name, err.Error())
		}
	}
}

// TestResolveAvailability_EmptySupportedTokensNoneAvailable asserts that with
// an empty supportedTokens slice, no entry is Available.
func TestResolveAvailability_EmptySupportedTokensNoneAvailable(t *testing.T) {
	resolved := ResolveAvailability(Catalog(), nil, false)
	for _, a := range resolved {
		if a.Available {
			t.Errorf("entry %q Available = true with empty supportedTokens, want false", a.Name)
		}
	}
}

// TestResolveAvailability_MarksAvailableWhenTokenPresent asserts a non-sk
// entry becomes Available when its query token is present in
// supportedTokens, regardless of fidoUsable.
func TestResolveAvailability_MarksAvailableWhenTokenPresent(t *testing.T) {
	resolved := ResolveAvailability(Catalog(), []string{"ssh-ed25519", "ssh-rsa"}, false)
	want := map[string]bool{
		"ed25519":    true,
		"rsa-4096":   true,
		"ecdsa-p256": false,
		"ed25519-sk": false,
		"ecdsa-sk":   false,
	}
	for _, a := range resolved {
		if a.Available != want[a.Name] {
			t.Errorf("entry %q Available = %v, want %v", a.Name, a.Available, want[a.Name])
		}
	}
}

// TestResolveAvailability_SkRequiresFidoUsable asserts a -sk entry is
// Available only when BOTH its token is present AND fidoUsable is true.
func TestResolveAvailability_SkRequiresFidoUsable(t *testing.T) {
	tokens := []string{"sk-ssh-ed25519@openssh.com", "sk-ecdsa-sha2-nistp256@openssh.com"}

	notUsable := ResolveAvailability(Catalog(), tokens, false)
	for _, a := range notUsable {
		if strings.HasSuffix(a.Name, "-sk") && a.Available {
			t.Errorf("entry %q Available = true with fidoUsable=false, want false", a.Name)
		}
	}

	usable := ResolveAvailability(Catalog(), tokens, true)
	for _, a := range usable {
		if strings.HasSuffix(a.Name, "-sk") && !a.Available {
			t.Errorf("entry %q Available = false with token present and fidoUsable=true, want true", a.Name)
		}
	}
}

// TestGeneratable_RequiresBothImplementedAndAvailable asserts Generatable
// requires BOTH Implemented AND Available — a stub whose token happens to be
// present in supportedTokens is still NOT generatable (Codex LOW: registry
// presence never implies generation support).
func TestGeneratable_RequiresBothImplementedAndAvailable(t *testing.T) {
	// Every token present, FIDO usable: everything is Available, but the
	// three stubs must still be non-Generatable.
	allTokens := []string{
		"ssh-ed25519", "ssh-rsa", "ecdsa-sha2-nistp256",
		"sk-ssh-ed25519@openssh.com", "sk-ecdsa-sha2-nistp256@openssh.com",
	}
	resolved := ResolveAvailability(Catalog(), allTokens, true)

	wantGeneratable := map[string]bool{
		"ed25519":    true,
		"rsa-4096":   true,
		"ecdsa-p256": false,
		"ed25519-sk": false,
		"ecdsa-sk":   false,
	}
	for _, a := range resolved {
		if !a.Available {
			t.Fatalf("entry %q Available = false with every token present, want true (test setup invariant)", a.Name)
		}
		if got, want := Generatable(a), wantGeneratable[a.Name]; got != want {
			t.Errorf("Generatable(%q) = %v, want %v (Implemented=%v Available=%v)", a.Name, got, want, a.Implemented, a.Available)
		}
	}
}

// TestGeneratable_ImplementedButUnavailable asserts an Implemented=true
// algorithm is still not Generatable when its token is absent locally.
func TestGeneratable_ImplementedButUnavailable(t *testing.T) {
	resolved := ResolveAvailability(Catalog(), nil, false)
	for _, a := range resolved {
		if a.Name != "ed25519" {
			continue
		}
		if Generatable(a) {
			t.Errorf("Generatable(ed25519) = true with no supported tokens, want false")
		}
	}
}
