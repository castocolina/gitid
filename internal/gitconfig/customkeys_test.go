package gitconfig

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/filewriter"
)

// TestSplitGitKeyTwoSegments verifies a plain two-segment key splits into a
// section and variable with no subsection.
func TestSplitGitKeyTwoSegments(t *testing.T) {
	section, subsection, variable, err := SplitGitKey("core.pager")
	if err != nil {
		t.Fatalf("SplitGitKey(core.pager): unexpected error: %v", err)
	}
	if section != "core" {
		t.Errorf("section: got %q want %q", section, "core")
	}
	if subsection != "" {
		t.Errorf("subsection: got %q want empty", subsection)
	}
	if variable != "pager" {
		t.Errorf("variable: got %q want %q", variable, "pager")
	}
}

// TestSplitGitKeyThreeSegments verifies a three-segment key splits into
// section, subsection, and variable.
func TestSplitGitKeyThreeSegments(t *testing.T) {
	section, subsection, variable, err := SplitGitKey("mytool.sub.key")
	if err != nil {
		t.Fatalf("SplitGitKey(mytool.sub.key): unexpected error: %v", err)
	}
	if section != "mytool" {
		t.Errorf("section: got %q want %q", section, "mytool")
	}
	if subsection != "sub" {
		t.Errorf("subsection: got %q want %q", subsection, "sub")
	}
	if variable != "key" {
		t.Errorf("variable: got %q want %q", variable, "key")
	}
}

// TestSplitGitKeySubsectionContainingDots verifies the LAST-dot rule (D-G):
// a subsection that itself contains dots (a URL) is not mis-split by a
// naive split-on-every-dot approach.
func TestSplitGitKeySubsectionContainingDots(t *testing.T) {
	section, subsection, variable, err := SplitGitKey("http.https://example.com.sslVerify")
	if err != nil {
		t.Fatalf("SplitGitKey: unexpected error: %v", err)
	}
	if section != "http" {
		t.Errorf("section: got %q want %q", section, "http")
	}
	if subsection != "https://example.com" {
		t.Errorf("subsection: got %q want %q", subsection, "https://example.com")
	}
	if variable != "sslVerify" {
		t.Errorf("variable: got %q want %q", variable, "sslVerify")
	}
}

// TestSplitGitKeyRejectsMalformedKeys verifies every documented malformed
// shape returns a named error rather than silently mis-filing the key
// (RESEARCH Pitfall 3).
func TestSplitGitKeyRejectsMalformedKeys(t *testing.T) {
	cases := []struct {
		name string
		key  string
	}{
		{"no dot at all", "coreonly"},
		{"empty section", ".pager"},
		{"empty variable", "core."},
		{"variable starting with digit", "core.9pager"},
		{"variable containing underscore", "core.pa_ger"},
		{"variable containing space", "core.pa ger"},
		{"section containing whitespace", "co re.pager"},
		{"section containing bracket", "co]re.pager"},
		{"subsection containing double quote", `mytool.su"b.key`},
		{"subsection containing backslash", `mytool.su\b.key`},
		{"subsection containing newline", "mytool.su\nb.key"},
		{"subsection containing NBSP (CR-01)", "http.a\u00a0b.sslVerify"},
		{"subsection containing TAB (CR-01)", "http.a\tb.sslVerify"},
		{"subsection containing ZWSP (CR-01)", "http.a\u200bb.sslVerify"},
		{"subsection containing DEL (CR-01)", "http.a\x7fb.sslVerify"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, err := SplitGitKey(tc.key)
			if err == nil {
				t.Fatalf("SplitGitKey(%q): expected error, got nil", tc.key)
			}
		})
	}
}

// TestRenderCustomKeysBlockEmitsSubsectionHeaders verifies a two-segment
// key renders a plain [section] header and a three-segment key renders a
// [section "subsection"] header, and that two entries in the same section
// each render their own header (git merges repeated headers at read time).
func TestRenderCustomKeysBlockEmitsSubsectionHeaders(t *testing.T) {
	entries := []CustomKey{
		{Key: "core.pager", Value: "less -FRX"},
		{Key: "mytool.sub.key", Value: "value1"},
		{Key: "core.editor", Value: "vim"},
	}
	body, err := RenderCustomKeysBlock(entries)
	if err != nil {
		t.Fatalf("RenderCustomKeysBlock: unexpected error: %v", err)
	}
	if !strings.Contains(body, "[core]\n\tpager = less -FRX") {
		t.Errorf("expected [core] header with pager line, got:\n%s", body)
	}
	if !strings.Contains(body, `[mytool "sub"]`+"\n\tkey = value1") {
		t.Errorf("expected [mytool \"sub\"] header with key line, got:\n%s", body)
	}
	if !strings.Contains(body, "[core]\n\teditor = vim") {
		t.Errorf("expected a second [core] header for the editor entry, got:\n%s", body)
	}
	if strings.Count(body, "[core]") != 2 {
		t.Errorf("expected two separate [core] headers (one per core entry), got:\n%s", body)
	}
}

// TestEnsureCustomGitKeyRejectsInjectionValues verifies validateValue is on
// the free-form value path — a newline, a carriage return, or a "[remote"
// token is refused (RESEARCH Pitfall 4).
func TestEnsureCustomGitKeyRejectsInjectionValues(t *testing.T) {
	cases := []struct {
		name  string
		value string
	}{
		{"newline", "good\n[remote \"evil\"]"},
		{"carriage return", "good\r\nbad"},
		{"remote token", "value [remote \"evil\"] injected"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := EnsureCustomGitKey(nil, "core.pager", tc.value)
			if err == nil {
				t.Fatalf("EnsureCustomGitKey with %s value: expected error, got nil", tc.name)
			}
		})
	}
}

// TestEnsureCustomGitKeyRejectsUnparseableGitSyntaxValues verifies CR-01: a
// value containing a double quote or backslash makes git's own config-file
// grammar unparseable (a quote opens a quoted region, a backslash starts an
// escape sequence), and a value containing '#' or ';' is silently truncated
// by git as a comment start. All four must be rejected by EnsureCustomGitKey
// BEFORE any text is composed — this is a render-syntax guard distinct from
// validateValue's injection guard (newline / "[remote").
func TestEnsureCustomGitKeyRejectsUnparseableGitSyntaxValues(t *testing.T) {
	cases := []struct {
		name  string
		value string
	}{
		{"double quote", `foo"bar`},
		{"backslash", `C:\path\to`},
		{"hash comment start", "foo #bar"},
		{"semicolon comment start", "foo ;bar"},
		{"leading whitespace", " foo"},
		{"trailing whitespace", "foo "},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := EnsureCustomGitKey(nil, "core.pager", tc.value)
			if err == nil {
				t.Fatalf("EnsureCustomGitKey with %s value %q: expected error, got nil", tc.name, tc.value)
			}
		})
	}
}

// TestRenderCustomKeysBlockRejectsUnparseableGitSyntaxValues verifies the
// same CR-01 guard applies at the render call site directly (RenderCustomKeysBlock),
// not just through the EnsureCustomGitKey upsert path.
func TestRenderCustomKeysBlockRejectsUnparseableGitSyntaxValues(t *testing.T) {
	entries := []CustomKey{{Key: "core.pager", Value: `foo"bar`}}
	if _, err := RenderCustomKeysBlock(entries); err == nil {
		t.Fatal("RenderCustomKeysBlock with a double-quote-bearing value: expected error, got nil")
	}
}

// TestEnsureCustomGitKeyPreservesForeignContentAndPriorKeys verifies adding
// a second custom key keeps the first, keeps the curated global-git block
// byte-identical, and keeps any non-gitid content in the file verbatim.
func TestEnsureCustomGitKeyPreservesForeignContentAndPriorKeys(t *testing.T) {
	foreign := "[user]\n\tname = Foreign User\n"
	existing := []byte(foreign +
		"# BEGIN gitid managed: global-git\n[core]\n\tignorecase = false\n# END gitid managed: global-git\n")

	afterFirst, err := EnsureCustomGitKey(existing, "core.pager", "less -FRX")
	if err != nil {
		t.Fatalf("first EnsureCustomGitKey: unexpected error: %v", err)
	}

	afterSecond, err := EnsureCustomGitKey(afterFirst, "mytool.sub.key", "value1")
	if err != nil {
		t.Fatalf("second EnsureCustomGitKey: unexpected error: %v", err)
	}

	result := string(afterSecond)
	if !strings.Contains(result, foreign) {
		t.Errorf("foreign content not preserved verbatim, got:\n%s", result)
	}
	if !strings.Contains(result, "# BEGIN gitid managed: global-git\n[core]\n\tignorecase = false\n# END gitid managed: global-git") {
		t.Errorf("curated global-git block not preserved byte-identically, got:\n%s", result)
	}
	if !strings.Contains(result, "core.pager") && !strings.Contains(result, "pager = less -FRX") {
		t.Errorf("first custom key (core.pager) not preserved, got:\n%s", result)
	}
	if !strings.Contains(result, "key = value1") {
		t.Errorf("second custom key (mytool.sub.key) not present, got:\n%s", result)
	}
}

// TestEnsureCustomGitKeyUpsertsSameKey verifies writing the same key with a
// new value replaces it rather than appending a duplicate, case-insensitively.
func TestEnsureCustomGitKeyUpsertsSameKey(t *testing.T) {
	afterFirst, err := EnsureCustomGitKey(nil, "core.pager", "less -FRX")
	if err != nil {
		t.Fatalf("first EnsureCustomGitKey: unexpected error: %v", err)
	}

	// Re-write with different casing on the key and a new value.
	afterSecond, err := EnsureCustomGitKey(afterFirst, "Core.Pager", "cat")
	if err != nil {
		t.Fatalf("second EnsureCustomGitKey: unexpected error: %v", err)
	}

	result := string(afterSecond)
	if strings.Count(result, "pager = ") != 1 {
		t.Fatalf("expected exactly one pager entry after upsert, got:\n%s", result)
	}
	if !strings.Contains(result, "pager = cat") {
		t.Errorf("expected upserted value 'cat', got:\n%s", result)
	}
	if strings.Contains(result, "less -FRX") {
		t.Errorf("old value should have been replaced, got:\n%s", result)
	}
}

// TestEnsureCustomGitKeyTreatsDifferentlyCasedSubsectionsAsDistinctKeys is
// the WR-05 regression: git config sections and variables are
// case-INSENSITIVE, but SUBSECTIONS are case-SENSITIVE (git-config(1),
// CONFIGURATION FILE) — so "http.https://Example.com.sslVerify" and
// "http.https://example.com.sslVerify" are two genuinely DIFFERENT git
// keys. EnsureCustomGitKey's upsert must not merge them.
func TestEnsureCustomGitKeyTreatsDifferentlyCasedSubsectionsAsDistinctKeys(t *testing.T) {
	afterFirst, err := EnsureCustomGitKey(nil, "http.https://Example.com.sslVerify", "true")
	if err != nil {
		t.Fatalf("first EnsureCustomGitKey: unexpected error: %v", err)
	}
	afterSecond, err := EnsureCustomGitKey(afterFirst, "http.https://example.com.sslVerify", "false")
	if err != nil {
		t.Fatalf("second EnsureCustomGitKey: unexpected error: %v", err)
	}

	result := string(afterSecond)
	if !strings.Contains(result, `"https://Example.com"`) {
		t.Errorf("first (differently-cased) subsection was overwritten instead of preserved, got:\n%s", result)
	}
	if !strings.Contains(result, `"https://example.com"`) {
		t.Errorf("second subsection missing, got:\n%s", result)
	}
	if strings.Count(result, "sslVerify") != 2 {
		t.Errorf("expected TWO distinct sslVerify entries (case-sensitive subsections), got:\n%s", result)
	}
}

// TestEnsureCustomGitKeyIsIdempotent verifies re-composing with an unchanged
// key/value produces bytes identical to the input (SC-1) — the caller's
// byte-equality check can then skip the write and take no backup.
func TestEnsureCustomGitKeyIsIdempotent(t *testing.T) {
	afterFirst, err := EnsureCustomGitKey(nil, "core.pager", "less -FRX")
	if err != nil {
		t.Fatalf("first EnsureCustomGitKey: unexpected error: %v", err)
	}

	afterSecond, err := EnsureCustomGitKey(afterFirst, "core.pager", "less -FRX")
	if err != nil {
		t.Fatalf("second EnsureCustomGitKey: unexpected error: %v", err)
	}

	if !bytes.Equal(afterFirst, afterSecond) {
		t.Errorf("EnsureCustomGitKey is not idempotent:\nfirst:\n%s\nsecond:\n%s", afterFirst, afterSecond)
	}
}

// TestCustomGitKeysBlockNameIsReserved verifies IsReservedBlockName(CustomGitKeysBlockName)
// is true and ParseManagedIncludeIf does not report the block as an identity.
func TestCustomGitKeysBlockNameIsReserved(t *testing.T) {
	if !IsReservedBlockName(CustomGitKeysBlockName) {
		t.Errorf("CustomGitKeysBlockName %q should be reserved", CustomGitKeysBlockName)
	}

	content := []byte("# BEGIN gitid managed: custom-git-keys\n[core]\n\tpager = less\n# END gitid managed: custom-git-keys\n")
	identities := ParseManagedIncludeIf(content)
	if _, ok := identities[CustomGitKeysBlockName]; ok {
		t.Errorf("ParseManagedIncludeIf must exclude the reserved custom-git-keys block, got: %v", identities)
	}
}

// TestGitReadsBackWhatRenderCustomKeysBlockWrote proves the section-splitting
// rule against the REAL git binary, not a golden literal gitid wrote itself
// (CLAUDE.md's hypothesis -> test -> implementation working method). It
// composes a block for a two-segment key AND a three-segment key with a
// dotted subsection, writes it to a temp file, and asserts `git config
// --file <path> --get <key>` returns each value.
func TestGitReadsBackWhatRenderCustomKeysBlockWrote(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}

	entries := []CustomKey{
		{Key: "core.pager", Value: "less -FRX"},
		{Key: "http.https://example.com.sslVerify", Value: "false"},
	}
	body, err := RenderCustomKeysBlock(entries)
	if err != nil {
		t.Fatalf("RenderCustomKeysBlock: unexpected error: %v", err)
	}

	composed := filewriter.ReplaceBlock(nil, CustomGitKeysBlockName, body)

	dir := t.TempDir()
	path := filepath.Join(dir, "gitconfig-custom-keys")
	if err := os.WriteFile(path, composed, 0o600); err != nil {
		t.Fatalf("writing temp gitconfig: %v", err)
	}

	got, err := exec.Command("git", "config", "--file", path, "--get", "core.pager").Output() //nolint:gosec // path is a t.TempDir()-derived test fixture path
	if err != nil {
		t.Fatalf("git config --get core.pager: %v", err)
	}
	if strings.TrimSpace(string(got)) != "less -FRX" {
		t.Errorf("core.pager: got %q want %q", strings.TrimSpace(string(got)), "less -FRX")
	}

	got, err = exec.Command("git", "config", "--file", path, "--get", "http.https://example.com.sslVerify").Output() //nolint:gosec // path is a t.TempDir()-derived test fixture path
	if err != nil {
		t.Fatalf("git config --get http.https://example.com.sslVerify: %v", err)
	}
	if strings.TrimSpace(string(got)) != "false" {
		t.Errorf("http.https://example.com.sslVerify: got %q want %q", strings.TrimSpace(string(got)), "false")
	}
}
