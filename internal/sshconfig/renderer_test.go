package sshconfig

import (
	"strings"
	"testing"
	"unicode"
)

// ---------------------------------------------------------------------------
// CR-08: RenderCheckedHostBlock — authoritative validated render boundary
// ---------------------------------------------------------------------------

// TestRenderCheckedHostBlockRejectsUnicodeSpace proves that
// RenderCheckedHostBlock rejects IdentityFile values containing Unicode
// whitespace characters above the ASCII range (CR-08). The unchecked
// RenderHostBlock has no such gate; the checked variant must.
func TestRenderCheckedHostBlockRejectsUnicodeSpace(t *testing.T) {
	// U+00A0 NO-BREAK SPACE is a valid Unicode whitespace character
	// (unicode.IsSpace returns true) but passes the C1-control check used
	// by ValidateHostBlock — this is the gap CR-08 requires closing.
	unicodeSpacePath := "/home/user/path\u00A0key"
	_, err := RenderCheckedHostBlock("alias", "hostname", 443, unicodeSpacePath, "")
	if err == nil {
		t.Fatalf("RenderCheckedHostBlock must reject IdentityFile with Unicode space U+00A0; path=%q", unicodeSpacePath)
	}
	if !strings.Contains(err.Error(), "unicode") && !strings.Contains(err.Error(), "whitespace") && !strings.Contains(err.Error(), "space") {
		t.Errorf("error should mention unicode/whitespace; got: %v", err)
	}
}

// TestRenderCheckedHostBlockRejectsAllUnicodeIsSpaceChars verifies that
// RenderCheckedHostBlock rejects every character for which unicode.IsSpace
// returns true in an IdentityFile value (CR-08 full Unicode whitespace gate).
func TestRenderCheckedHostBlockRejectsAllUnicodeIsSpaceChars(t *testing.T) {
	// Characters unicode.IsSpace returns true for that are not ASCII
	unicodeSpaces := []rune{
		'\u00A0', // NO-BREAK SPACE
		'\u1680', // OGHAM SPACE MARK
		'\u2000', // EN QUAD
		'\u2009', // THIN SPACE
		'\u200A', // HAIR SPACE
		'\u202F', // NARROW NO-BREAK SPACE
		'\u205F', // MEDIUM MATHEMATICAL SPACE
		'\u3000', // IDEOGRAPHIC SPACE
	}
	for _, r := range unicodeSpaces {
		path := "/home/user/path" + string(r) + "key"
		if !unicode.IsSpace(r) {
			t.Skipf("rune U+%04X is not unicode.IsSpace — test fixture needs updating", r)
		}
		_, err := RenderCheckedHostBlock("alias", "hostname", 443, path, "")
		if err == nil {
			t.Errorf("RenderCheckedHostBlock accepted IdentityFile with unicode.IsSpace rune U+%04X; path=%q", r, path)
		}
	}
}

// TestRenderCheckedHostBlockRejectsUnicodeControlChars verifies that
// RenderCheckedHostBlock rejects Unicode control characters (CR-08).
func TestRenderCheckedHostBlockRejectsUnicodeControlChars(t *testing.T) {
	// U+0085 NEXT LINE — a valid Unicode control character not caught by the C1 range check
	// (the C1 range is 0x80-0x9F; 0x85 is in that range and IS caught — use U+200B instead)
	zeroWidthSpace := "/home/user/path\u200Bkey" // U+200B ZERO WIDTH SPACE — unicode.IsSpace is false but unicode.IsControl is false too; use an actual control
	_ = zeroWidthSpace
	// Use a known Unicode control: U+0085 NEXT LINE (NEL), which is unicode.IsControl
	nelPath := "/home/user/path\u0085key"
	_, err := RenderCheckedHostBlock("alias", "hostname", 443, nelPath, "")
	if err == nil {
		t.Errorf("RenderCheckedHostBlock must reject IdentityFile with U+0085 NEL (Unicode control); path=%q", nelPath)
	}
}

// TestRenderCheckedHostBlockPassesSafeIdentityFile verifies that a valid path
// passes the checked renderer and returns the expected block (CR-08 positive).
func TestRenderCheckedHostBlockPassesSafeIdentityFile(t *testing.T) {
	block, err := RenderCheckedHostBlock("work.github.com", "ssh.github.com", 443, "~/.ssh/id_ed25519_work", "github.com")
	if err != nil {
		t.Fatalf("RenderCheckedHostBlock with valid inputs returned error: %v", err)
	}
	for _, want := range []string{
		"Host work.github.com",
		"Hostname ssh.github.com",
		"Port 443",
		"User git",
		"IdentityFile ~/.ssh/id_ed25519_work",
		"IdentitiesOnly yes",
		"# gitid: provider=github.com",
	} {
		if !strings.Contains(block, want) {
			t.Errorf("RenderCheckedHostBlock missing %q; block:\n%s", want, block)
		}
	}
}

// indexOf returns the byte index of substr in s, or -1 when absent. It is a
// thin helper so ordering assertions read clearly.
func indexOf(s, substr string) int {
	return strings.Index(s, substr)
}

// TestRenderHostBlock asserts SSH-01: the rendered Host block contains, in
// order, the five required directives for an aliased identity.
func TestRenderHostBlock(t *testing.T) {
	got := RenderHostBlock("work.github.com", "ssh.github.com", 443, "~/.ssh/id_ed25519_work", "")

	// Each required directive must appear, in this exact relative order.
	wantOrder := []string{
		"Host work.github.com",
		"Hostname ssh.github.com",
		"Port 443",
		"User git",
		"IdentityFile ~/.ssh/id_ed25519_work",
		"IdentitiesOnly yes",
	}

	prev := -1
	for _, directive := range wantOrder {
		idx := indexOf(got, directive)
		if idx == -1 {
			t.Fatalf("RenderHostBlock missing directive %q; got:\n%s", directive, got)
		}
		if idx <= prev {
			t.Fatalf("RenderHostBlock directive %q out of order; got:\n%s", directive, got)
		}
		prev = idx
	}
}

// TestRenderGlobalBlockDarwin asserts SSH-03 + Pitfall 4: on macOS the Host *
// block emits IgnoreUnknown UseKeychain before UseKeychain yes before
// AddKeysToAgent yes.
func TestRenderGlobalBlockDarwin(t *testing.T) {
	got := RenderGlobalBlock("darwin")

	if !strings.Contains(got, "Host *") {
		t.Fatalf("RenderGlobalBlock(darwin) missing 'Host *'; got:\n%s", got)
	}

	ignoreIdx := indexOf(got, "IgnoreUnknown UseKeychain")
	useIdx := indexOf(got, "UseKeychain yes")
	addIdx := indexOf(got, "AddKeysToAgent yes")

	if ignoreIdx == -1 || useIdx == -1 || addIdx == -1 {
		t.Fatalf("RenderGlobalBlock(darwin) missing keychain directives; got:\n%s", got)
	}
	if ignoreIdx >= useIdx || useIdx >= addIdx {
		t.Fatalf("RenderGlobalBlock(darwin) directive order wrong (want IgnoreUnknown < UseKeychain < AddKeysToAgent); got:\n%s", got)
	}
}

// TestRenderGlobalBlockLinux asserts SSH-03: Linux gets no UseKeychain block at
// all (empty string), since the directive is Apple-only.
func TestRenderGlobalBlockLinux(t *testing.T) {
	got := RenderGlobalBlock("linux")
	if got != "" {
		t.Fatalf("RenderGlobalBlock(linux) want empty string, got:\n%s", got)
	}
}

// TestRenderHostBlock_WithProvider asserts D-11: when a non-empty provider is
// given, RenderHostBlock appends "# gitid: provider=<p>" as the last line of
// the Host stanza body (after IdentitiesOnly yes).
func TestRenderHostBlock_WithProvider(t *testing.T) {
	got := RenderHostBlock("work.github.com", "ssh.github.com", 443, "~/.ssh/id_ed25519_work", "github")

	const marker = "# gitid: provider=github"
	if !strings.Contains(got, marker) {
		t.Fatalf("RenderHostBlock with provider missing marker %q; got:\n%s", marker, got)
	}

	// Marker must come AFTER "IdentitiesOnly yes" (Pitfall 2 / RESEARCH.md).
	idxIdentities := indexOf(got, "IdentitiesOnly yes")
	idxMarker := indexOf(got, marker)
	if idxIdentities == -1 {
		t.Fatalf("RenderHostBlock missing IdentitiesOnly yes; got:\n%s", got)
	}
	if idxMarker <= idxIdentities {
		t.Fatalf("provider marker must come AFTER IdentitiesOnly yes; got:\n%s", got)
	}
}

// TestRenderHostBlock_EmptyProvider asserts D-11: when provider is empty, no
// marker line is emitted (legacy / no-provider callers unaffected).
func TestRenderHostBlock_EmptyProvider(t *testing.T) {
	got := RenderHostBlock("work.github.com", "ssh.github.com", 443, "~/.ssh/id_ed25519_work", "")
	if strings.Contains(got, "# gitid: provider=") {
		t.Fatalf("RenderHostBlock with empty provider must emit no marker; got:\n%s", got)
	}
}

// TestRenderHostBlock_NewlineInProviderPanics asserts RESEARCH Security Domain /
// T-05.5-04: a newline-bearing provider value panics with a programming-error
// message (defense-in-depth against comment injection).
func TestRenderHostBlock_NewlineInProviderPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("RenderHostBlock with newline in provider must panic, but did not")
		}
	}()
	RenderHostBlock("work.github.com", "ssh.github.com", 443, "~/.ssh/id_ed25519_work", "a\nb")
}

// TestGlobalBlockOrderedLast asserts Pitfall 5 / T-02-15: when a host block and
// the global block are composed, 'Host *' must come AFTER the specific host so
// first-match-wins does not let the wildcard override the alias.
func TestGlobalBlockOrderedLast(t *testing.T) {
	host := RenderHostBlock("work.github.com", "ssh.github.com", 443, "~/.ssh/id_ed25519_work", "")
	global := RenderGlobalBlock("darwin")

	composed := host + "\n" + global

	hostIdx := indexOf(composed, "Host work.github.com")
	wildcardIdx := indexOf(composed, "Host *")

	if hostIdx == -1 || wildcardIdx == -1 {
		t.Fatalf("composed config missing a host marker; got:\n%s", composed)
	}
	if wildcardIdx < hostIdx {
		t.Fatalf("'Host *' must be ordered after specific host; got:\n%s", composed)
	}
}
