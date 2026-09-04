package sshconfig

import (
	"bytes"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/filewriter"
)

// globalBody extracts the GlobalBlockName block body from content.
func globalBody(t *testing.T, content []byte) string {
	t.Helper()
	for _, b := range filewriter.ListBlocks(content) {
		if b.Name == GlobalBlockName {
			return b.Body
		}
	}
	t.Fatalf("no %s block found in:\n%s", GlobalBlockName, content)
	return ""
}

// TestEnsureGlobalsGuardFirstOnEveryPlatform pins D-11: the guard directive is
// the FIRST non-sentinel line of the rendered body on darwin AND on linux, and
// its index is strictly less than the `Host *` line and any `UseKeychain` line
// (the lexical-before requirement that keeps a config synced from a mac from
// hard-erroring on linux).
func TestEnsureGlobalsGuardFirstOnEveryPlatform(t *testing.T) {
	for _, goos := range []string{"darwin", "linux"} {
		got, err := EnsureGlobals(nil, nil, goos)
		if err != nil {
			t.Fatalf("EnsureGlobals(%s): %v", goos, err)
		}
		body := globalBody(t, got)
		lines := strings.Split(body, "\n")
		if len(lines) == 0 || strings.TrimSpace(lines[0]) != "IgnoreUnknown UseKeychain" {
			t.Errorf("%s: first body line = %q, want the guard directive", goos, lines[0])
		}
		guardIdx := strings.Index(body, "IgnoreUnknown UseKeychain")
		hostIdx := strings.Index(body, "Host *")
		useIdx := strings.Index(body, "UseKeychain yes")
		if guardIdx == -1 || hostIdx == -1 {
			t.Fatalf("%s: guard or Host * missing; body:\n%s", goos, body)
		}
		if guardIdx >= hostIdx || (useIdx != -1 && guardIdx >= useIdx) {
			t.Errorf("%s: guard must be lexically before Host * and any UseKeychain line; body:\n%s", goos, body)
		}
	}
}

// TestEnsureGlobalsIdempotent proves the managed block is stable: applying the
// same inputs twice yields byte-identical content the second time.
func TestEnsureGlobalsIdempotent(t *testing.T) {
	seed := []byte(managedTestBlock("global-ssh", "Host *\n  UseKeychain yes\n  AddKeysToAgent yes\n"))
	first, err := EnsureGlobals(seed, nil, "darwin")
	if err != nil {
		t.Fatalf("first EnsureGlobals: %v", err)
	}
	second, err := EnsureGlobals(first, nil, "darwin")
	if err != nil {
		t.Fatalf("second EnsureGlobals: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Errorf("non-idempotent globals render; first:\n%s\nsecond:\n%s", first, second)
	}
}

// TestEnsureGlobalsPreservesDarwinKeyOnLinux pins the platform contract: a
// darwin-only key already present in the body is PRESERVED when running on
// linux — dropping it would silently degrade a config synced from a mac.
func TestEnsureGlobalsPreservesDarwinKeyOnLinux(t *testing.T) {
	seed := []byte(managedTestBlock("global-ssh", "Host *\n  UseKeychain yes\n  AddKeysToAgent yes\n"))
	got, err := EnsureGlobals(seed, nil, "linux")
	if err != nil {
		t.Fatalf("EnsureGlobals(linux): %v", err)
	}
	body := globalBody(t, got)
	for _, want := range []string{"UseKeychain yes", "AddKeysToAgent yes"} {
		if !strings.Contains(body, want) {
			t.Errorf("darwin-only key %q lost on linux; body:\n%s", want, body)
		}
	}
}

// TestEnsureGlobalsExistingValueBeatsDefault pins D-06's key-union rule: a key
// already present in the body is preserved when the platform default would
// have supplied a DIFFERENT value.
func TestEnsureGlobalsExistingValueBeatsDefault(t *testing.T) {
	seed := []byte(managedTestBlock("global-ssh", "Host *\n  UseKeychain no\n"))
	got, err := EnsureGlobals(seed, nil, "darwin")
	if err != nil {
		t.Fatalf("EnsureGlobals: %v", err)
	}
	body := globalBody(t, got)
	if !strings.Contains(body, "UseKeychain no") {
		t.Errorf("pre-existing UseKeychain no was overwritten by the darwin default; body:\n%s", body)
	}
	if strings.Contains(body, "UseKeychain yes") {
		t.Errorf("platform default must not override an existing value; body:\n%s", body)
	}
}

// TestEnsureGlobalsExplicitOverlayAlwaysWins pins the fix direction: a key
// named in the explicit overlay wins even when the body already carries a
// different value.
func TestEnsureGlobalsExplicitOverlayAlwaysWins(t *testing.T) {
	seed := []byte(managedTestBlock("global-ssh", "Host *\n  HashKnownHosts no\n"))
	got, err := EnsureGlobals(seed, map[string]string{"HashKnownHosts": "yes"}, "linux")
	if err != nil {
		t.Fatalf("EnsureGlobals: %v", err)
	}
	body := globalBody(t, got)
	if !strings.Contains(body, "HashKnownHosts yes") {
		t.Errorf("explicit overlay did not win; body:\n%s", body)
	}
	if strings.Contains(body, "HashKnownHosts no") {
		t.Errorf("the old non-recommended value survived the explicit overlay; body:\n%s", body)
	}
}

// TestRenderGlobalBodyWithOverlayMatchesEnsureGlobalsExactly is the WR-08/
// WR-09 (09.5-REVIEW.md round 3) parity regression: RenderGlobalBodyWithOverlay
// is the extracted "render what the real write will produce" step
// EnsureGlobals itself uses — a STAGED proof that calls this function must
// see byte-identical text to what the confirmed write actually composes, for
// the SAME existing body, explicit overlay, and goos. Proven across a
// scalar, a bundle-shaped overlay, and darwin's platform-default injection.
func TestRenderGlobalBodyWithOverlayMatchesEnsureGlobalsExactly(t *testing.T) {
	cases := []struct {
		name     string
		existing string
		explicit map[string]string
		goos     string
	}{
		{"empty body, linux, no overlay", "", nil, "linux"},
		{"empty body, darwin (platform defaults apply)", "", nil, "darwin"},
		{"existing scalar replaced by overlay", "Host *\n  StrictHostKeyChecking accept-new\n", map[string]string{"StrictHostKeyChecking": "yes"}, "linux"},
		{"unrecognised custom directive appended", "Host *\n  ForwardAgent no\n", map[string]string{"StreamLocalBindMask": "0177"}, "linux"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seed := []byte(managedTestBlock("global-ssh", tc.existing))
			composed, err := EnsureGlobals(seed, tc.explicit, tc.goos)
			if err != nil {
				t.Fatalf("EnsureGlobals: %v", err)
			}
			want := globalBody(t, composed)

			// filewriter.ReplaceBlock trims the trailing newline when it
			// stores a block's body (an unrelated, separate concern from
			// render fidelity — every OTHER block-rendering package in this
			// project trims the same way, e.g. gitconfig.RenderCustomKeysBlock),
			// so RenderGlobalBodyWithOverlay's raw renderGlobalBody output
			// carries one trailing '\n' that globalBody's EXTRACTED body
			// never does. Trimmed here for a fair comparison — the ONE
			// insignificant difference this asymmetry produces is harmless
			// for a staged ssh -G probe either way.
			got := strings.TrimRight(RenderGlobalBodyWithOverlay(tc.existing, tc.explicit, tc.goos), "\n")
			if got != want {
				t.Errorf("RenderGlobalBodyWithOverlay diverges from EnsureGlobals's own composed body:\ngot:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

// TestEnsureGlobalsAdoptsLegacyBlock pins D-08 adoption: a legacy-named block
// carrying a directive is renamed to GlobalBlockName, its body survives, and
// no block remains under the legacy name after the same write.
func TestEnsureGlobalsAdoptsLegacyBlock(t *testing.T) {
	seed := []byte(managedTestBlock("_global", "Host *\n  HashKnownHosts yes\n"))
	got, err := EnsureGlobals(seed, nil, "linux")
	if err != nil {
		t.Fatalf("EnsureGlobals: %v", err)
	}
	body := globalBody(t, got)
	if !strings.Contains(body, "HashKnownHosts yes") {
		t.Errorf("adopted legacy body directive lost; body:\n%s", body)
	}
	for _, b := range filewriter.ListBlocks(got) {
		if b.Name == LegacyGlobalBlockName {
			t.Errorf("legacy block still present after adoption:\n%s", got)
		}
	}
}

// TestEnsureGlobalsLeavesForeignHostStarAlone pins the byte-for-byte safety
// property (T-06-01): a hand-written `Host *` stanza OUTSIDE every gitid
// sentinel is untouched.
func TestEnsureGlobalsLeavesForeignHostStarAlone(t *testing.T) {
	foreign := "Host *\n  Compression yes\n"
	seed := []byte(foreign)
	got, err := EnsureGlobals(seed, nil, "linux")
	if err != nil {
		t.Fatalf("EnsureGlobals: %v", err)
	}
	if !strings.Contains(string(got), foreign) {
		t.Errorf("hand-written foreign stanza changed; got:\n%s", got)
	}
}

// TestEnsureGlobalsAppendsUnrecognisedKey pins the never-drop contract: a
// hand-added directive inside the managed block that is not in the canonical
// order is preserved and appended after the ordered keys.
func TestEnsureGlobalsAppendsUnrecognisedKey(t *testing.T) {
	seed := []byte(managedTestBlock("global-ssh", "Host *\n  ServerAliveInterval 60\n"))
	got, err := EnsureGlobals(seed, nil, "darwin")
	if err != nil {
		t.Fatalf("EnsureGlobals: %v", err)
	}
	body := globalBody(t, got)
	if !strings.Contains(body, "ServerAliveInterval 60") {
		t.Errorf("unrecognised pre-existing key dropped; body:\n%s", body)
	}
	ordered := strings.Index(body, "AddKeysToAgent")
	unrecognised := strings.Index(body, "ServerAliveInterval")
	if unrecognised <= ordered {
		t.Errorf("unrecognised key must be appended AFTER ordered keys (AddKeysToAgent at %d, ServerAliveInterval at %d); body:\n%s", ordered, unrecognised, body)
	}
}

// TestEnsureGlobalsPreservesMultiTokenValues is the CR-02 regression: a
// hand-added directive whose value is more than one whitespace-separated
// token must round-trip intact — before the fix, parseGlobalBody kept only
// fields[1] and silently deleted every token after it (the common
// 1Password/Secretive macOS IdentityAgent setup, SendEnv globs, ProxyCommand
// arguments, and CanonicalDomains lists all lost data on every write).
func TestEnsureGlobalsPreservesMultiTokenValues(t *testing.T) {
	cases := []struct {
		name string
		line string
	}{
		{"IdentityAgent", `IdentityAgent "~/Library/Group Containers/2BUA8C4S2C.com.agilebits/Library/Application Support/1Password/agent.sock"`},
		{"SendEnv", "SendEnv LANG LC_*"},
		{"ProxyCommand", "ProxyCommand ssh -W %h:%p bastion"},
		{"CanonicalDomains", "CanonicalDomains example.com internal.example.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seed := []byte(managedTestBlock("global-ssh", "Host *\n  "+tc.line+"\n"))
			got, err := EnsureGlobals(seed, nil, "linux")
			if err != nil {
				t.Fatalf("EnsureGlobals: %v", err)
			}
			body := globalBody(t, got)
			if !strings.Contains(body, tc.line) {
				t.Errorf("multi-token value truncated; want line %q intact in body:\n%s", tc.line, body)
			}
		})
	}
}

// TestEnsureGlobalsPreservesInteriorWhitespaceOnASecondWrite is the CR-02
// round-2 regression: TestEnsureGlobalsPreservesMultiTokenValues above only
// proves single-space-separated values survive ONE write — every case there
// is a Fields-then-Join round trip that happens to be a no-op for
// single-space input, so it cannot catch a whitespace-collapse bug.
// parseGlobalBody used to reconstruct a directive's value via
// strings.Join(strings.Fields(trimmed)[1:], " "), which collapses every run
// of internal whitespace to a single space — invisible on write 1 (the
// value is written as typed and proven correct against a real ssh -G), but
// destructive on write 2: the SECOND EnsureGlobals call that touches this
// block (any curated Global-SSH apply, any later custom directive, any
// repair) parses the FIRST write's rendered line back through
// parseGlobalBody and re-renders it — silently rewriting a TAB-separated
// argument list or a double-spaced quoted path to a DIFFERENT value the
// user never confirmed. This asserts the value is BYTE-IDENTICAL across two
// consecutive EnsureGlobals calls, not merely present as a substring.
func TestEnsureGlobalsPreservesInteriorWhitespaceOnASecondWrite(t *testing.T) {
	cases := []struct {
		name string
		line string
	}{
		{"double space in a quoted path", `IdentityFile "/pa  th/id"`},
		{"tab-separated ProxyCommand arguments", "ProxyCommand /usr/bin/nc\t-X 5 %h %p"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seed := []byte(managedTestBlock("global-ssh", "Host *\n  "+tc.line+"\n"))

			// Write 1: the candidate is composed for the first time.
			afterFirst, err := EnsureGlobals(seed, nil, "linux")
			if err != nil {
				t.Fatalf("first EnsureGlobals: %v", err)
			}
			firstBody := globalBody(t, afterFirst)
			if !strings.Contains(firstBody, tc.line) {
				t.Fatalf("write 1: value not intact; want line %q in body:\n%s", tc.line, firstBody)
			}

			// Write 2: an UNRELATED subsequent write to the same block (here,
			// the same explicit map again, mirroring a later curated
			// Global-SSH apply or custom directive touching the block) must
			// not silently collapse the interior whitespace this second
			// parse-then-render pass reads back.
			afterSecond, err := EnsureGlobals(afterFirst, nil, "linux")
			if err != nil {
				t.Fatalf("second EnsureGlobals: %v", err)
			}
			secondBody := globalBody(t, afterSecond)
			if !strings.Contains(secondBody, tc.line) {
				t.Errorf("write 2: interior whitespace was collapsed; want line %q intact, got body:\n%s", tc.line, secondBody)
			}
			if !bytes.Equal(afterFirst, afterSecond) {
				t.Errorf("EnsureGlobals is not idempotent across writes when the value has interior whitespace:\nfirst:\n%s\nsecond:\n%s", afterFirst, afterSecond)
			}
		})
	}
}

// TestEnsureGlobalsPreservesCommentLines is the comment half of the CR-02
// fix: a `#` comment line inside the managed block must survive round-trip
// rather than being silently dropped.
func TestEnsureGlobalsPreservesCommentLines(t *testing.T) {
	seed := []byte(managedTestBlock("global-ssh", "Host *\n  # keep this note\n  HashKnownHosts yes\n"))
	got, err := EnsureGlobals(seed, nil, "linux")
	if err != nil {
		t.Fatalf("EnsureGlobals: %v", err)
	}
	body := globalBody(t, got)
	if !strings.Contains(body, "# keep this note") {
		t.Errorf("comment line dropped from managed block; body:\n%s", body)
	}

	// Idempotency: the comment must survive a second pass too, not just be
	// silently re-appended a second time or duplicated.
	second, err := EnsureGlobals(got, nil, "linux")
	if err != nil {
		t.Fatalf("second EnsureGlobals: %v", err)
	}
	if !bytes.Equal(got, second) {
		t.Errorf("comment preservation is not idempotent; first:\n%s\nsecond:\n%s", got, second)
	}
}

// TestEnsureGlobalsUnconditionalGuardOnEmptyConfig pins the empty-config
// case: on linux an empty config still renders the guard + the Host * stanza
// (the block itself is never dropped).
func TestEnsureGlobalsUnconditionalGuardOnEmptyConfig(t *testing.T) {
	got, err := EnsureGlobals(nil, nil, "linux")
	if err != nil {
		t.Fatalf("EnsureGlobals: %v", err)
	}
	body := globalBody(t, got)
	if !strings.Contains(body, "Host *") {
		t.Errorf("empty linux config must still carry the Host * stanza; body:\n%s", body)
	}
}

// TestEnsureGlobalsRepositionsAdoptedLegacyBlockAfterIdentity pins D-09:
// a legacy-named globals block that sat BEFORE an identity Host block is
// adopted AND moved so the new-named globals block is the last gitid-managed
// block. filewriter.ReplaceBlock would otherwise leave a wrong-position
// block in place (or, for a first-write of GlobalBlockName, append — either
// way the post-condition is "last", never "wherever it happened to sit").
func TestEnsureGlobalsRepositionsAdoptedLegacyBlockAfterIdentity(t *testing.T) {
	identity := managedTestBlock("personal", "Host personal.github.com\n  Hostname ssh.github.com\n")
	legacy := managedTestBlock("_global", "Host *\n  HashKnownHosts yes\n")
	got, err := EnsureGlobals([]byte(legacy+identity), nil, "linux")
	if err != nil {
		t.Fatalf("EnsureGlobals: %v", err)
	}
	assertGlobalsLastAfter(t, got, "personal")
	if !strings.Contains(globalBody(t, got), "HashKnownHosts yes") {
		t.Errorf("adopted directive lost during reposition; got:\n%s", got)
	}
}

// TestEnsureGlobalsRepositionsExistingBlockAfterIdentity is the in-place
// case D-09 cannot leave to ReplaceBlock: a GlobalBlockName block that
// already sits BEFORE an identity must be removed and re-appended so it
// lands last. This is also the create-after-fix shape — Write appends the
// new identity after the existing globals, then EnsureGlobals must move
// the globals block back to the end.
func TestEnsureGlobalsRepositionsExistingBlockAfterIdentity(t *testing.T) {
	identity := managedTestBlock("personal", "Host personal.github.com\n  Hostname ssh.github.com\n")
	globals := managedTestBlock("global-ssh", "Host *\n  HashKnownHosts no\n")
	got, err := EnsureGlobals([]byte(globals+identity), map[string]string{"HashKnownHosts": "yes"}, "linux")
	if err != nil {
		t.Fatalf("EnsureGlobals: %v", err)
	}
	assertGlobalsLastAfter(t, got, "personal")
	if !strings.Contains(globalBody(t, got), "HashKnownHosts yes") {
		t.Errorf("explicit overlay lost during reposition; got:\n%s", got)
	}
	second, err := EnsureGlobals(got, map[string]string{"HashKnownHosts": "yes"}, "linux")
	if err != nil {
		t.Fatalf("second EnsureGlobals: %v", err)
	}
	if !bytes.Equal(got, second) {
		t.Errorf("repositioned globals render is not idempotent;\nfirst:\n%s\nsecond:\n%s", got, second)
	}
}

// assertGlobalsLastAfter fails if the globals begin-sentinel is missing, if
// the named identity begin-sentinel is missing, or if the globals block does
// not start after the identity block.
func assertGlobalsLastAfter(t *testing.T, content []byte, identityName string) {
	t.Helper()
	text := string(content)
	gOff := strings.Index(text, filewriter.BeginPrefix+GlobalBlockName+"\n")
	iOff := strings.Index(text, filewriter.BeginPrefix+identityName+"\n")
	if gOff < 0 || iOff < 0 {
		t.Fatalf("missing sentinels (globals=%d identity=%d) in:\n%s", gOff, iOff, content)
	}
	if gOff < iOff {
		t.Errorf("globals begin-sentinel at %d precedes identity %q at %d; want globals LAST:\n%s", gOff, identityName, iOff, content)
	}
	blocks := filewriter.ListBlocks(content)
	if len(blocks) == 0 || blocks[len(blocks)-1].Name != GlobalBlockName {
		t.Errorf("last gitid-managed block = %q, want %s:\n%s", lastBlockName(blocks), GlobalBlockName, content)
	}
}

func lastBlockName(blocks []filewriter.NamedBlock) string {
	if len(blocks) == 0 {
		return ""
	}
	return blocks[len(blocks)-1].Name
}

// managedTestBlock wraps body in the managed sentinels for name (a package
// local mirror of the composition EnsureGlobals itself produces).
func managedTestBlock(name, body string) string {
	return "# BEGIN gitid managed: " + name + "\n" + strings.TrimRight(body, "\n") + "\n# END gitid managed: " + name + "\n"
}
