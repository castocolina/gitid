package sshconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// handWrittenFixture is a real hand-written ~/.ssh/config: a managed block
// gitid owns, plus a hand-written Host stanza with an inline comment and
// tab-indented directives — the surgical rewrite must touch only the
// IdentitiesOnly line and leave every other byte, including the comment and
// the managed block, untouched.
const handWrittenFixture = `# my own notes
Host clientb.github.com
	# personal note on this stanza
	HostName ssh.github.com
	IdentitiesOnly no # deliberately loose
	IdentityFile ~/.ssh/id_ed25519_clientb
	Port 22

# BEGIN gitid managed: acme
Host acme.github.com
	HostName ssh.github.com
	Port 443
	IdentitiesOnly yes
	IdentityFile ~/.ssh/id_ed25519_acme
# END gitid managed: acme
`

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("seeding fixture config: %v", err)
	}
	return path
}

func TestRewriteHostDirective_ChangesOnlyTheTargetLine(t *testing.T) {
	path := writeTempConfig(t, handWrittenFixture)

	if _, err := RewriteHostDirective(path, "clientb.github.com", "IdentitiesOnly", "yes"); err != nil {
		t.Fatalf("RewriteHostDirective: %v", err)
	}

	got, err := os.ReadFile(path) //nolint:gosec // test fixture path
	if err != nil {
		t.Fatalf("reading rewritten config: %v", err)
	}

	want := strings.Replace(handWrittenFixture,
		"IdentitiesOnly no # deliberately loose",
		"IdentitiesOnly yes # deliberately loose", 1)

	if string(got) != want {
		t.Fatalf("rewritten config changed more than the target line.\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRewriteHostDirective_PreservesIndentAndComment(t *testing.T) {
	path := writeTempConfig(t, handWrittenFixture)

	if _, err := RewriteHostDirective(path, "clientb.github.com", "IdentitiesOnly", "yes"); err != nil {
		t.Fatalf("RewriteHostDirective: %v", err)
	}
	got, _ := os.ReadFile(path) //nolint:gosec // test fixture path
	if !strings.Contains(string(got), "\tIdentitiesOnly yes # deliberately loose\n") {
		t.Fatalf("rewritten line lost its tab indentation or trailing comment:\n%s", got)
	}
}

func TestRewriteHostDirective_ManagedBlockUntouched(t *testing.T) {
	path := writeTempConfig(t, handWrittenFixture)

	if _, err := RewriteHostDirective(path, "clientb.github.com", "IdentitiesOnly", "yes"); err != nil {
		t.Fatalf("RewriteHostDirective: %v", err)
	}
	got, _ := os.ReadFile(path) //nolint:gosec // test fixture path
	if !strings.Contains(string(got), "# BEGIN gitid managed: acme") ||
		!strings.Contains(string(got), "IdentitiesOnly yes\n\tIdentityFile ~/.ssh/id_ed25519_acme") {
		t.Fatalf("the gitid-managed block was altered:\n%s", got)
	}
}

func TestRewriteHostDirective_CreatesTimestampedBackup(t *testing.T) {
	path := writeTempConfig(t, handWrittenFixture)

	backupPath, err := RewriteHostDirective(path, "clientb.github.com", "IdentitiesOnly", "yes")
	if err != nil {
		t.Fatalf("RewriteHostDirective: %v", err)
	}
	if backupPath == "" {
		t.Fatal("backupPath must be non-empty")
	}
	backup, err := os.ReadFile(backupPath) //nolint:gosec // test fixture path
	if err != nil {
		t.Fatalf("reading backup: %v", err)
	}
	if string(backup) != handWrittenFixture {
		t.Fatal("backup does not contain the pre-rewrite content")
	}
}

func TestRewriteHostDirective_StanzaNotFound(t *testing.T) {
	path := writeTempConfig(t, handWrittenFixture)
	if _, err := RewriteHostDirective(path, "nonexistent.github.com", "IdentitiesOnly", "yes"); err == nil {
		t.Fatal("want an error when the Host stanza is not found")
	}
}

func TestRewriteHostDirective_DirectiveNotFound(t *testing.T) {
	path := writeTempConfig(t, handWrittenFixture)
	if _, err := RewriteHostDirective(path, "clientb.github.com", "ProxyCommand", "foo"); err == nil {
		t.Fatal("want an error when the directive is not present in the stanza")
	}
}

func TestRewriteHostDirective_RejectsNewlineInValue(t *testing.T) {
	path := writeTempConfig(t, handWrittenFixture)
	if _, err := RewriteHostDirective(path, "clientb.github.com", "IdentitiesOnly", "yes\nHost evil.example.com"); err == nil {
		t.Fatal("want an error for a value containing a newline (CR-18 class injection)")
	}
	got, _ := os.ReadFile(path) //nolint:gosec // test fixture path
	if string(got) != handWrittenFixture {
		t.Fatal("a rejected value must never partially write")
	}
}

func TestRewriteHostDirective_RejectsControlByteInValue(t *testing.T) {
	path := writeTempConfig(t, handWrittenFixture)
	if _, err := RewriteHostDirective(path, "clientb.github.com", "IdentitiesOnly", "yes\x00"); err == nil {
		t.Fatal("want an error for a value containing a NUL byte")
	}
}

func TestRewriteHostDirective_RejectsInvalidHostPattern(t *testing.T) {
	path := writeTempConfig(t, handWrittenFixture)
	if _, err := RewriteHostDirective(path, "not a valid host; rm -rf", "IdentitiesOnly", "yes"); err == nil {
		t.Fatal("want an error for an invalid host pattern")
	}
}

func TestApplyVerifiedHostDirective_SucceedsAndVerifies(t *testing.T) {
	path := writeTempConfig(t, handWrittenFixture)

	if _, err := ApplyVerifiedHostDirective(path, "clientb.github.com", "IdentitiesOnly", "yes"); err != nil {
		t.Fatalf("ApplyVerifiedHostDirective: %v", err)
	}
	got, _ := os.ReadFile(path) //nolint:gosec // test fixture path
	if !strings.Contains(string(got), "IdentitiesOnly yes # deliberately loose") {
		t.Fatalf("rewrite did not apply:\n%s", got)
	}
}

// TestApplyVerifiedHostDirective_RestoresOnVerificationFailure proves the D-10
// auto-restore path: corrupting the just-written file (simulating a
// verification mismatch) via postRewriteHook must restore the ORIGINAL
// pre-fix content and report failure — never a false success.
func TestApplyVerifiedHostDirective_RestoresOnVerificationFailure(t *testing.T) {
	path := writeTempConfig(t, handWrittenFixture)

	old := postRewriteHook
	defer func() { postRewriteHook = old }()
	postRewriteHook = func(configPath string) {
		// Corrupt the file so the parse->render->re-parse stability check
		// fails: a NUL byte breaks Decode outright.
		_ = os.WriteFile(configPath, []byte("Host clientb.github.com\n\tIdentityFile x\x00y\n"), 0o600)
	}

	_, err := ApplyVerifiedHostDirective(path, "clientb.github.com", "IdentitiesOnly", "yes")
	if err == nil {
		t.Fatal("want an error when the post-rewrite verification fails")
	}
	got, rerr := os.ReadFile(path) //nolint:gosec // test fixture path
	if rerr != nil {
		t.Fatalf("reading restored config: %v", rerr)
	}
	if string(got) != handWrittenFixture {
		t.Fatalf("file was not restored to its original content after a verification failure:\n%s", got)
	}
}

func TestDiffHostDirective_RendersBeforeAfter(t *testing.T) {
	diff, err := DiffHostDirective([]byte(handWrittenFixture), "clientb.github.com", "IdentitiesOnly", "yes")
	if err != nil {
		t.Fatalf("DiffHostDirective: %v", err)
	}
	if !strings.Contains(diff, "- \tIdentitiesOnly no # deliberately loose") {
		t.Errorf("diff missing the removed line:\n%s", diff)
	}
	if !strings.Contains(diff, "+ \tIdentitiesOnly yes # deliberately loose") {
		t.Errorf("diff missing the added line:\n%s", diff)
	}
	if !strings.Contains(diff, "HostName ssh.github.com") {
		t.Errorf("diff missing unchanged context lines:\n%s", diff)
	}
}

func TestDiffHostDirective_StanzaNotFound(t *testing.T) {
	if _, err := DiffHostDirective([]byte(handWrittenFixture), "nonexistent.github.com", "IdentitiesOnly", "yes"); err == nil {
		t.Fatal("want an error when the stanza is not found")
	}
}

func TestParseAllHostBlocks_HandWrittenAndManaged(t *testing.T) {
	blocks := ParseAllHostBlocks([]byte(handWrittenFixture))
	if len(blocks) != 2 {
		t.Fatalf("ParseAllHostBlocks() = %d blocks, want 2: %+v", len(blocks), blocks)
	}
	var handWritten, managed *HostBlockFacts
	for i := range blocks {
		switch blocks[i].Pattern {
		case "clientb.github.com":
			handWritten = &blocks[i]
		case "acme.github.com":
			managed = &blocks[i]
		}
	}
	if handWritten == nil {
		t.Fatal("hand-written stanza not found")
	}
	if handWritten.ManagedBlockName != "" {
		t.Errorf("hand-written stanza ManagedBlockName = %q, want empty", handWritten.ManagedBlockName)
	}
	if handWritten.IdentitiesOnly == nil || *handWritten.IdentitiesOnly {
		t.Errorf("hand-written stanza IdentitiesOnly = %v, want explicit false", handWritten.IdentitiesOnly)
	}
	if handWritten.IdentityFile != "~/.ssh/id_ed25519_clientb" {
		t.Errorf("hand-written stanza IdentityFile = %q", handWritten.IdentityFile)
	}
	if handWritten.LineNumber == 0 {
		t.Error("hand-written stanza LineNumber must be non-zero (the IdentitiesOnly directive line)")
	}

	if managed == nil {
		t.Fatal("managed stanza not found")
	}
	if managed.ManagedBlockName != "acme" {
		t.Errorf("managed stanza ManagedBlockName = %q, want %q", managed.ManagedBlockName, "acme")
	}
}

func TestParseAllHostBlocks_UnsetIdentitiesOnlyIsNilNeverFalse(t *testing.T) {
	blocks := ParseAllHostBlocks([]byte("Host bare.example.com\n\tIdentityFile ~/.ssh/id_ed25519_bare\n"))
	if len(blocks) != 1 {
		t.Fatalf("ParseAllHostBlocks() = %d blocks, want 1", len(blocks))
	}
	if blocks[0].IdentitiesOnly != nil {
		t.Errorf("IdentitiesOnly = %v, want nil (unset must never be conflated with explicit false)", blocks[0].IdentitiesOnly)
	}
}

func TestParseAllHostBlocks_EmptyContentNoImplicitHostStar(t *testing.T) {
	blocks := ParseAllHostBlocks([]byte(""))
	if len(blocks) != 0 {
		t.Fatalf("ParseAllHostBlocks(empty) = %d blocks, want 0 (implicit Host * must be skipped): %+v", len(blocks), blocks)
	}
}
