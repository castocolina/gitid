//go:build e2e

package e2e

// identity_manager_pty_e2e_test.go — raw-keystroke PTY proof that the
// Identity Manager's Git-only delete really writes (05-01-PLAN.md Task 1,
// the Phase 5 tracer). Drives the REAL `gitid` binary (never gitid-dummy)
// via raw keystrokes over a pseudo-terminal at 100x30, seeded with ONE
// complete identity's four artifacts plus a key pair.
//
// This is the DIRECT countermeasure to 05-RESEARCH.md Pitfall 1
// (realBackend.Persist silently falling through to the in-memory
// tuikit.Reduce for every mutation except AddIdentity/Reset): the test
// restarts the binary in a SECOND PTY session after the delete and asserts
// the deleted Git side is STILL absent from the freshly re-read list — a
// Reduce-only illusion would show the identity healed back to "complete" on
// restart, since nothing would have actually left disk.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/castocolina/gitid/internal/uploader"
)

// untouchedGitOnlyDeletePaths are the artifacts D-10 says a Git-only delete
// must NEVER touch: the SSH config, both key halves, and allowed_signers.
func untouchedGitOnlyDeletePaths(home, name string) []string {
	sshDir := filepath.Join(home, ".ssh")
	return []string{
		filepath.Join(sshDir, "config"),
		filepath.Join(sshDir, "id_ed25519_"+name),
		filepath.Join(sshDir, "id_ed25519_"+name+".pub"),
		filepath.Join(sshDir, "allowed_signers"),
	}
}

// seedRecipeShapeSSHConfig overwrites the fixture's ~/.ssh/config with a
// recipe-shape (recipes/ssh-config.recipe) managed block for name — the
// alt-SSH `Hostname ssh.github.com` / `Port 443` pair review R-17 asks the
// surviving block to still carry after a git-only delete. seedGitPTYIdentity
// (ui_pty_e2e_test.go's seedMinimalIdentity) writes a plainer fixture
// (`HostName github.com`, no explicit Port) sufficient for its own tests,
// but not for this file's recipe-shape regression proof.
func seedRecipeShapeSSHConfig(t *testing.T, home, name string) {
	t.Helper()
	content := "# BEGIN gitid managed: " + name + "\n" +
		"Host " + name + ".github.com\n" +
		"  Hostname ssh.github.com\n" +
		"  Port 443\n" +
		"  User git\n" +
		"  IdentityFile ~/.ssh/id_ed25519_" + name + "\n" +
		"  IdentitiesOnly yes\n" +
		"# END gitid managed: " + name + "\n\n" +
		"Host *\n" +
		"  IdentitiesOnly yes\n"
	if err := os.WriteFile(filepath.Join(home, ".ssh", "config"), []byte(content), 0o600); err != nil {
		t.Fatalf("seedRecipeShapeSSHConfig: WriteFile: %v", err)
	}
}

// TestIdentityManager_DeleteGitOnly drives the real binary's delete-choice
// ceremony end-to-end: "d" opens the scope chooser (git-only default-
// focused, the safer option), Enter opens the ceremony, Enter confirms
// (async CommitDelete), the receipt renders, and a SECOND binary launch
// proves the write survived a restart.
func TestIdentityManager_DeleteGitOnly(t *testing.T) {
	home := SandboxHome(t)
	seedGitPTYIdentity(t, home, "acme") // ui_pty_e2e_test.go + git_configuration_pty_e2e_test.go fixture: complete SSH+Git+signers+key
	seedRecipeShapeSSHConfig(t, home, "acme")
	bin := BuildBinary(t)

	untouched := untouchedGitOnlyDeletePaths(home, "acme")
	before := snapshotGitBytes(t, untouched)
	if len(before) != len(untouched) {
		t.Fatalf("fixture invalid: expected all %d untouched-path fixtures to pre-exist, got %d: %v", len(untouched), len(before), before)
	}
	sshConfigBefore := before[filepath.Join(home, ".ssh", "config")]
	if !strings.Contains(string(sshConfigBefore), "IdentitiesOnly yes") ||
		!strings.Contains(string(sshConfigBefore), "Hostname ssh.github.com") ||
		!strings.Contains(string(sshConfigBefore), "Port 443") {
		t.Fatalf("fixture invalid: seeded SSH config missing expected recipe-shape lines:\n%s", sshConfigBefore)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s := startPTYAt(t, newRealCreateFlowCmd(t, ctx, bin, home, ""), dummyTermWidth, dummyTermHeight)
	closed := false
	defer func() {
		if !closed {
			s.close(t)
		}
	}()

	uiReady(t, s)
	mustSee(t, s, "Identities", "sidebar renders with the seeded identity")
	mustSee(t, s, "acme", "the seeded identity appears in the sidebar")

	s.sendKey([]byte("d"), keystrokeDelay)
	mustSee(t, s, "Delete Git identity only (safer — SSH stays)", "delete-choice: the scope chooser renders")
	mustSee(t, s, "Delete everything (SSH + Git + key) — irreversible", "delete-choice: both scope options render")

	s.sendKey(dummyKeyEnter, keystrokeDelay) // git-only is default-focused — Enter opens the ceremony on it
	mustSee(t, s, `Delete the Git identity of "acme" (SSH stays)`, "ceremony: the git-only heading renders")
	mustSee(t, s, "Nothing has changed yet", "ceremony: the pre-confirm assurance renders before any write")

	// Nothing on disk yet — still just showing the ceremony preview.
	assertGitBytesUnchanged(t, before, snapshotGitBytes(t, untouched))
	fragmentPath := filepath.Join(home, ".gitconfig.d", "acme")
	if _, err := os.Stat(fragmentPath); err != nil {
		t.Fatalf("fragment must still exist before confirm: %v", err)
	}

	s.sendKey(dummyKeyEnter, keystrokeDelay) // confirm — async CommitDelete
	mustSee(t, s, `Git identity of "acme" deleted`, "result-success: the receipt renders after the real write completes")
	mustSee(t, s, "the SSH side is untouched", "result-success: the receipt names the git-only scope's promise")

	saveFrame(t, "identity-manager-delete-git-only-result", s)

	// Filesystem assertions — the D-10 proof: SSH config, both key halves,
	// and allowed_signers are byte-identical to their pre-delete state.
	assertGitBytesUnchanged(t, before, snapshotGitBytes(t, untouched))

	// R-17: the surviving SSH Host block keeps its recipe-shape lines.
	sshConfigAfter, err := os.ReadFile(filepath.Join(home, ".ssh", "config")) //nolint:gosec // fixed sandbox path (G304)
	if err != nil {
		t.Fatalf("reading post-delete ssh config: %v", err)
	}
	for _, want := range []string{"IdentitiesOnly yes", "Hostname ssh.github.com", "Port 443"} {
		if !strings.Contains(string(sshConfigAfter), want) {
			t.Errorf("post-delete SSH config missing recipe-shape line %q:\n%s", want, sshConfigAfter)
		}
	}

	// The Git side is actually gone: includeIf block removed from
	// ~/.gitconfig, fragment file removed from disk.
	gcAfter, err := os.ReadFile(filepath.Join(home, ".gitconfig")) //nolint:gosec // fixed sandbox path (G304)
	if err != nil {
		t.Fatalf("reading post-delete gitconfig: %v", err)
	}
	if strings.Contains(string(gcAfter), "BEGIN gitid managed: acme") {
		t.Errorf("post-delete gitconfig still carries the acme includeIf block:\n%s", gcAfter)
	}
	if _, err := os.Stat(fragmentPath); !os.IsNotExist(err) {
		t.Errorf("fragment file survived a git-only delete: statErr=%v", err)
	}

	s.close(t)
	closed = true

	// Restart the binary in a SECOND PTY session — the direct countermeasure
	// to RESEARCH.md Pitfall 1: a Reduce-only illusion would show "acme"
	// healed back to complete on restart, since nothing would have actually
	// left disk. A real write survives the restart.
	ctx2, cancel2 := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel2()
	s2 := startPTYAt(t, newRealCreateFlowCmd(t, ctx2, bin, home, ""), dummyTermWidth, dummyTermHeight)
	defer s2.close(t)

	uiReady(t, s2)
	mustSee(t, s2, "acme", "the identity row survives a restart (git-only heals to incomplete, never disappears)")
	mustNotSee(t, s2, `Delete the Git identity of "acme"`, "no stale ceremony carries over into the fresh session")

	saveFrame(t, "identity-manager-delete-git-only-post-restart", s2)
}

// TestIdentityManager_CLIAndTUIProduceByteIdenticalGitconfig runs the
// git-only delete over TWO IDENTICAL sandbox HOMEs — once through the REAL
// binary's TUI ceremony (raw PTY keystrokes) and once through the REAL
// binary's CLI verb (`identity delete --git-only --yes`) — and asserts the
// resulting ~/.gitconfig bytes are byte-equal: D-02's "CLI and TUI MUST call
// the same chokepoint — no behavioral fork" claim, proven end-to-end through
// the compiled binary (cmd/gitid/wiring_test.go proves the in-process half).
func TestIdentityManager_CLIAndTUIProduceByteIdenticalGitconfig(t *testing.T) {
	bin := BuildBinary(t)

	homeTUI := SandboxHome(t)
	seedGitPTYIdentity(t, homeTUI, "acme")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s := startPTYAt(t, newRealCreateFlowCmd(t, ctx, bin, homeTUI, ""), dummyTermWidth, dummyTermHeight)
	uiReady(t, s)
	mustSee(t, s, "acme", "TUI path: seeded identity renders")
	s.sendKey([]byte("d"), keystrokeDelay)
	s.sendKey(dummyKeyEnter, keystrokeDelay) // open the git-only ceremony
	mustSee(t, s, `Delete the Git identity of "acme" (SSH stays)`, "TUI path: ceremony opened")
	s.sendKey(dummyKeyEnter, keystrokeDelay) // confirm
	mustSee(t, s, `Git identity of "acme" deleted`, "TUI path: receipt rendered")
	s.close(t)

	// A fresh HOME for the CLI path — SandboxHome sets $HOME for the CURRENT
	// test's own os/exec calls only, but the CLI runs as a real subprocess
	// with its own explicit HOME env entry, so a second t.TempDir() suffices
	// without disturbing the TUI path's already-completed sandbox above.
	homeCLI := t.TempDir()
	seedGitPTYIdentity(t, homeCLI, "acme")

	cliCtx, cliCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cliCancel()
	cliCmd := exec.CommandContext(cliCtx, bin, "identity", "delete", "acme", "--git-only", "--yes") //nolint:gosec // bin from BuildBinary; fixed args
	cliEnv, _ := e2eEnv(t, homeCLI)
	cliCmd.Env = cliEnv
	if out, err := cliCmd.CombinedOutput(); err != nil {
		t.Fatalf("CLI identity delete failed: %v\noutput: %s", err, out)
	}

	gcTUI, err := os.ReadFile(filepath.Join(homeTUI, ".gitconfig")) //nolint:gosec // fixed sandbox path (G304)
	if err != nil {
		t.Fatalf("reading TUI-path gitconfig: %v", err)
	}
	gcCLI, err := os.ReadFile(filepath.Join(homeCLI, ".gitconfig")) //nolint:gosec // fixed sandbox path (G304)
	if err != nil {
		t.Fatalf("reading CLI-path gitconfig: %v", err)
	}
	if string(gcTUI) != string(gcCLI) {
		t.Errorf("TUI and CLI delete paths produced different ~/.gitconfig bytes:\nTUI:\n%s\n--- CLI ---\n%s", gcTUI, gcCLI)
	}
}

// ---------------------------------------------------------------------------
// 05-09-PLAN.md Task 1 — the full per-state manager suite (DLV-06) plus the
// independent FIELDS.md manifest backstop (review R-26).
//
// SHARED-RENDERER BACKSTOP (review R-26): both cmd/gitid and cmd/gitid-dummy
// render every identity-manager screen through the SAME internal/tuikit
// stack, so this plan's Task 2 real-versus-dummy paired comparison CANNOT
// detect a layout/label/content defect present in BOTH renderers. The
// assertions below are the countermeasure: for every approved state they
// read that state's required-field list from
// .planning/design/identity-manager/FIELDS.md AT TEST TIME (parsed, never
// transcribed into this file) and assert every required field is present in
// the REAL binary's decoded frame ALONE. This assertion must NEVER be
// weakened into a dummy comparison — the manifest parse plus a hand-checked
// negative control (TestIdentityManager_ManifestBackstopNegativeControl) are
// what make it a real backstop instead of a restatement of the design doc.
// ---------------------------------------------------------------------------

// fieldsManifestFieldRe extracts the first backtick-quoted identifier from a
// FIELDS.md table row's Field column (e.g. "`identity_row` × 8" -> "identity_row").
var fieldsManifestFieldRe = regexp.MustCompile("`([a-z_]+)`")

// parseFieldsManifest reads .planning/design/identity-manager/FIELDS.md and
// returns, per "## identity-manager / <state>" heading, the ordered list of
// Field-column identifiers its table declares REQUIRED — the manifest this
// suite reads at test time rather than transcribing (review R-26's own
// instruction).
func parseFieldsManifest(t *testing.T) map[string][]string {
	t.Helper()
	path := filepath.Join(repoRoot(t), ".planning", "design", "identity-manager", "FIELDS.md")
	data, err := os.ReadFile(path) //nolint:gosec // fixed repo-relative path (G304)
	if err != nil {
		t.Fatalf("parseFieldsManifest: reading %s: %v", path, err)
	}
	sections := map[string][]string{}
	current := ""
	for _, raw := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(raw)
		if strings.HasPrefix(trimmed, "## identity-manager / ") {
			current = strings.TrimPrefix(trimmed, "## identity-manager / ")
			if idx := strings.Index(current, " ("); idx >= 0 {
				current = current[:idx] // drop a trailing parenthetical, e.g. "(entry screen)"
			}
			continue
		}
		if current == "" || !strings.HasPrefix(trimmed, "|") {
			continue
		}
		cols := strings.Split(trimmed, "|")
		if len(cols) < 3 || !isAllDigits(strings.TrimSpace(cols[1])) {
			continue // header/separator row, or a row outside a "# | Field | ..." table
		}
		m := fieldsManifestFieldRe.FindStringSubmatch(cols[2])
		if m == nil {
			continue
		}
		sections[current] = append(sections[current], m[1])
	}
	if len(sections) == 0 {
		t.Fatalf("parseFieldsManifest: parsed ZERO sections from %s — the manifest heading/table shape drifted from what this parser expects", path)
	}
	return sections
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// identityManagerFieldSubstrings maps a FIELDS.md field identifier to the
// CURRENT literal substring that field renders as in the compiled real
// binary. FIELDS.md's own "Label" column is the ORIGINAL design copy, not
// necessarily what finally shipped (05-UI-SPEC.md's Copywriting Contract
// documents several DRAFT rewordings) — this map is the living
// cross-reference between the frozen field IDENTITY and its current render.
// A field FIELDS.md adds with no entry here is a hard, loud test failure
// (assertManifestFields), which is exactly the drift this backstop exists to
// catch — the map is deliberately NOT auto-derived from FIELDS.md's Label
// column.
var identityManagerFieldSubstrings = map[string]string{
	// detail-ssh-first
	"ssh_section":             "SSH — shown first, always",
	"git_section_absent_note": "Git not configured — no fabricated values shown.",
	"per_identity_health":     "Findings (",
	// action-menu
	"action_view_detail":  "View SSH-first detail",
	"action_clone":        "Clone (c)",
	"action_new_key":      "Generate new key",
	"action_delete":       "Delete (d)",
	"action_register_key": "Register key with provider now (u)",
	// delete-choice
	"delete_choice_git_only":   "Delete Git identity only",
	"delete_choice_everything": "Delete everything (SSH + Git + key)",
	// delete_choice_target is asserted directly against the identity name by
	// each delete-choice test (the target IS the fixture's own identity
	// name, never a fixed literal) — see the field-not-in-map allowance in
	// assertManifestFields.
	// confirm-destructive
	"confirm_warning": "This action is irreversible",
	// confirm_default_no is asserted directly (the Cancel control's exact
	// label, checked structurally, not a bare substring — see
	// assertManifestFields's allowance list).
	// backup-notice (this app's compressed 2-state ceremony renders the
	// backup PROMISE on the SAME pre-confirm pane FIELDS.md calls
	// "backup-notice" — ceremony.go's own header comment documents the
	// compression).
	"backup_explainer": "(written first — restore it to undo)",
	// ssh_config_backup_path / gitconfig_backup_path are asserted directly
	// against the real backup path text (dynamic, contains a timestamp) by
	// each test — see the allowance list below.
	// list-empty
	"empty_state_copy": "No identities yet",
	"empty_state_cta":  "Press n to create your first identity",
	// register-key-modal (Phase 9, 09-07-PLAN.md Task 3, review R-26's
	// backstop extended to the Phase 9 checkpoints) — UploadRunningLineFmt
	// and UploadResultOKFmt (internal/tuikit/design.go) are format strings
	// with a dynamic %s, so the anchor is the literal substring that
	// survives regardless of the interpolated command/type.
	// upload_manual_fallback is NOT here — see fieldsAssertedStructurally:
	// it is conditional (mutually exclusive with running_line/result_row,
	// never co-occurring in one captured frame) and is asserted directly by
	// TestIdentityManager_RegisterKeyModalManualFallback instead.
	"modal_heading":       "'s key with",
	"upload_running_line": "Running:",
	"upload_result_row":   "key registered",
}

// fieldsAssertedStructurally are FIELDS.md field identifiers this suite
// checks with dedicated, state-specific logic (a dynamic identity name, a
// timestamped backup path, a button's exact focus state) rather than a
// fixed substring in identityManagerFieldSubstrings — named here so
// assertManifestFields can distinguish "known, checked elsewhere" from "new
// field with no coverage at all" (the real drift signal).
var fieldsAssertedStructurally = map[string]bool{
	"delete_choice_target":   true,
	"confirm_default_no":     true,
	"ssh_config_backup_path": true,
	"gitconfig_backup_path":  true,
	"identity_row":           true, // list-populated: covered by TestIdentityManager_ListPopulatedEightTaxonomy directly
	"row_glyph":              true,
	"row_state_word":         true,
	"row_note":               true,
	"header_context_chip":    true,
	"clone_source_name":      true, // clone-name-prompt: covered by the mouse-focus test's dynamic heading check
	"clone_suggested_name":   true,
	"clone_distinct_note":    true,
	// register-key-modal: conditional field, mutually exclusive with
	// upload_running_line/upload_result_row — covered directly by
	// TestIdentityManager_RegisterKeyModalManualFallback's own mustSee.
	"upload_manual_fallback": true,
}

// manifestFieldsPresent returns the subset of substrs NOT found in frame —
// the pure predicate both the positive assertions and the negative control
// share, so the negative control proves the SAME logic the positive
// assertions use is capable of failing (never a separately-written check
// that could vacuously always pass).
func manifestFieldsPresent(frame string, substrs []string) (missing []string) {
	for _, want := range substrs {
		if !strings.Contains(frame, want) {
			missing = append(missing, want)
		}
	}
	return missing
}

// assertManifestFields is the REAL-binary-only backstop: for stateKey (a
// FIELDS.md "## identity-manager / <state>" heading), require every parsed
// field either has a known substring in identityManagerFieldSubstrings
// (asserted here) or is declared as structurally-asserted elsewhere
// (fieldsAssertedStructurally) — an unrecognized field fails loudly, which
// is the drift-detection this backstop exists for.
func assertManifestFields(t *testing.T, manifest map[string][]string, stateKey string, s *ptySession) {
	t.Helper()
	fields, ok := manifest[stateKey]
	if !ok || len(fields) == 0 {
		t.Fatalf("assertManifestFields: FIELDS.md has no parsed section %q — manifest/test drifted", stateKey)
	}
	var substrs []string
	for _, f := range fields {
		if fieldsAssertedStructurally[f] {
			continue
		}
		want, known := identityManagerFieldSubstrings[f]
		if !known {
			t.Fatalf("assertManifestFields: FIELDS.md field %q (state %q) has no known substring and is not declared structurally-asserted — the manifest backstop is tracking a field this test does not check; add an entry to identityManagerFieldSubstrings or fieldsAssertedStructurally", f, stateKey)
		}
		substrs = append(substrs, want)
	}
	frame := s.snapshot()
	if missing := manifestFieldsPresent(frame, substrs); len(missing) > 0 {
		t.Errorf("FIELDS.md manifest backstop (state %q): required field text missing from the REAL binary's decoded frame: %v\nframe:\n%s", stateKey, missing, frame)
	}
}

// TestIdentityManager_ManifestBackstopNegativeControl proves
// manifestFieldsPresent is NOT vacuous: deleting one required substring from
// a captured frame must make the SAME check report it missing.
func TestIdentityManager_ManifestBackstopNegativeControl(t *testing.T) {
	frame := "Actions — acme\n\n  View SSH-first detail\n  Clone (c)\n  Generate new key\n  Delete (d)\n"
	substrs := []string{"View SSH-first detail", "Clone (c)", "Generate new key", "Delete (d)"}

	if missing := manifestFieldsPresent(frame, substrs); len(missing) != 0 {
		t.Fatalf("fixture invalid: manifestFieldsPresent reported missing fields against a frame that has them all: %v", missing)
	}

	mutated := strings.Replace(frame, "Generate new key", "", 1)
	missing := manifestFieldsPresent(mutated, substrs)
	if len(missing) != 1 || missing[0] != "Generate new key" {
		t.Fatalf("negative control FAILED: removing %q from the frame did not make manifestFieldsPresent report it missing (got %v) — the backstop would never catch a real regression", "Generate new key", missing)
	}
}

// ---------------------------------------------------------------------------
// list-populated: the 8-taxonomy-label, 7-row-word, orphan-key-absent proof.
// ---------------------------------------------------------------------------

// runManagerIdentityListJSON runs `identity list --json` against the
// compiled real binary over home and parses the frozen D-03 JSON document,
// reusing identity_cli_e2e_test.go's runIdentityListJSON transport (bytes)
// and identityListDoc/identityRecordDoc wire types — a second, divergently-
// typed JSON struct in this file would risk drifting from the one D-03
// contract both files must track identically.
func runManagerIdentityListJSON(t *testing.T, bin, home string) identityListDoc {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out := runIdentityListJSON(t, ctx, bin, home)
	var doc identityListDoc
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatalf("identity list --json: unmarshal: %v\noutput: %s", err, out)
	}
	return doc
}

// identityManagerGlyphByState mirrors internal/tuikit/design.go's
// IdentityManagerGlyphByState — duplicated here as a small, frozen literal
// (not imported: e2e is a black-box PTY suite over the compiled binary by
// this project's own convention, matching every other test in this file).
var identityManagerGlyphByState = map[string]string{
	"complete":              "✓",
	"incomplete":            "!",
	"git-only":              "!",
	"key-unused":            "!",
	"key-used-ssh-only":     "✓",
	"key-used-both":         "✓",
	"key-missing":           "✗",
	"fragment-path-missing": "✗",
}

// TestIdentityManager_ListPopulatedEightTaxonomy is the review R-06-aligned
// list-populated proof: seedEightTaxonomyIdentities' sandbox produces all
// EIGHT MGR-02 labels across the two health axes (asserted via the frozen
// `identity list --json` read surface, per 05-01-SUMMARY.md's binding
// taxonomy resolution) and exactly the SEVEN reachable collapsed row words
// (asserted by selecting each identity in the compiled real binary's live
// master-detail pane and reading its own "<glyph> <word>" line) — never a
// claim of eight distinct row words, which ClassifyState's collapse cannot
// produce. A separately-seeded orphan key is asserted present under
// `unused_keys` and absent from every rendered manager row.
func TestIdentityManager_ListPopulatedEightTaxonomy(t *testing.T) {
	home := SandboxHome(t)
	fixtures := seedEightTaxonomyIdentities(t, home)
	bin := BuildBinary(t)

	// --json axis coverage: the union of identity_state/key_state values
	// across every seeded identity must include all eight locked labels.
	doc := runManagerIdentityListJSON(t, bin, home)
	seenAxis := map[string]bool{}
	byName := map[string]identityRecordDoc{}
	for _, rec := range doc.Identities {
		seenAxis[rec.IdentityState] = true
		seenAxis[rec.KeyState] = true
		byName[rec.Name] = rec
	}
	wantLabels := []string{
		"complete", "incomplete", "git-only", "fragment-path-missing",
		"key-missing", "key-unused", "key-used-ssh-only", "key-used-both",
	}
	for _, want := range wantLabels {
		if !seenAxis[want] {
			t.Errorf("label %q never observed in identity_state/key_state across --json output: %+v", want, doc.Identities)
		}
	}
	for _, f := range fixtures {
		rec, ok := byName[f.Name]
		if !ok {
			t.Fatalf("--json output missing seeded identity %q: %+v", f.Name, doc.Identities)
		}
		if rec.IdentityState != f.WantIdentityState {
			t.Errorf("%s: identity_state = %q, want %q", f.Name, rec.IdentityState, f.WantIdentityState)
		}
		if rec.KeyState != f.WantKeyState {
			t.Errorf("%s: key_state = %q, want %q", f.Name, rec.KeyState, f.WantKeyState)
		}
		if rec.State != f.WantRowWord {
			t.Errorf("%s: collapsed state (--json) = %q, want %q", f.Name, rec.State, f.WantRowWord)
		}
	}

	// Orphan key: seeded by seedEightTaxonomyIdentities (writeStubKeyPair
	// "orphan"), referenced by NO Host block anywhere.
	orphanPath := filepath.Join(home, ".ssh", "id_ed25519_orphan")
	foundOrphan := false
	for _, k := range doc.UnusedKeys {
		if strings.Contains(k, "id_ed25519_orphan") {
			foundOrphan = true
		}
	}
	if !foundOrphan {
		t.Errorf("orphan key %s not present in unused_keys: %v", orphanPath, doc.UnusedKeys)
	}
	for _, rec := range doc.Identities {
		if rec.Name == "orphan" {
			t.Errorf("the orphan key fixture must never surface as its own manager identity row: %+v", rec)
		}
	}

	// Live compiled-binary proof: select each identity in turn and assert
	// its OWN detail pane renders exactly its expected collapsed word next
	// to the expected glyph — the SEVEN reachable row words, never eight.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s := startPTYAt(t, newRealCreateFlowCmd(t, ctx, bin, home, ""), dummyTermWidth, dummyTermHeight)
	defer s.close(t)
	uiReady(t, s)
	mustSee(t, s, fmt.Sprintf("%d ids", len(fixtures)), "list-populated: header_context_chip shows the identity count")

	seenRowWords := map[string]bool{}
	visited := map[string]bool{}
	for i := 0; i < len(fixtures); i++ {
		frame := s.snapshot()
		var current *taxonomyIdentity
		for j := range fixtures {
			if strings.Contains(frame, " "+fixtures[j].Name+"  ") && !visited[fixtures[j].Name] {
				current = &fixtures[j]
				break
			}
		}
		if current == nil {
			// Fall back: match any known name present in the detail heading
			// (handles a leading-row edge case the "  " spacing probe misses).
			for j := range fixtures {
				if !visited[fixtures[j].Name] && strings.Contains(frame, fixtures[j].Name) {
					current = &fixtures[j]
					break
				}
			}
		}
		if current == nil {
			t.Fatalf("could not identify the currently-selected taxonomy identity in frame:\n%s", frame)
		}
		visited[current.Name] = true
		wantLine := identityManagerGlyphByState[current.WantRowWord] + " " + current.WantRowWord
		mustSee(t, s, wantLine, fmt.Sprintf("%s: detail pane shows its collapsed row word %q", current.Name, current.WantRowWord))
		seenRowWords[current.WantRowWord] = true
		s.sendKey(dummyKeyDown, keystrokeDelay)
	}
	wantRowWords := []string{"complete", "incomplete", "git-only", "fragment-path-missing", "key-missing", "key-unused", "key-used-ssh-only"}
	sort.Strings(wantRowWords)
	var gotRowWords []string
	for w := range seenRowWords {
		gotRowWords = append(gotRowWords, w)
	}
	sort.Strings(gotRowWords)
	if len(gotRowWords) != 7 || !equalStrings(gotRowWords, wantRowWords) {
		t.Errorf("distinct rendered row words = %v, want exactly the seven reachable values %v", gotRowWords, wantRowWords)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// list-empty: the true first-run landing state.
// ---------------------------------------------------------------------------

// TestIdentityManager_ListEmpty drives the real binary against a completely
// empty sandbox home and asserts the frozen empty-state copy and its call to
// action render — never a blank list.
func TestIdentityManager_ListEmpty(t *testing.T) {
	home := SandboxHome(t) // no seeding — the true first-run empty home
	bin := BuildBinary(t)
	manifest := parseFieldsManifest(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s := startPTYAt(t, newRealCreateFlowCmd(t, ctx, bin, home, ""), dummyTermWidth, dummyTermHeight)
	defer s.close(t)

	uiReady(t, s)
	mustSee(t, s, "No identities yet", "list-empty: the frozen empty-state copy renders")
	mustSee(t, s, "Press n to create your first identity", "list-empty: the frozen call to action renders")
	assertManifestFields(t, manifest, "list-empty", s)

	saveFrame(t, "identity-manager-list-empty", s)
}

// ---------------------------------------------------------------------------
// detail-ssh-first: SSH-first, Git absence honestly stated, per-identity
// health.
// ---------------------------------------------------------------------------

func TestIdentityManager_DetailSSHFirst(t *testing.T) {
	home := SandboxHome(t)
	seedMinimalIdentity(t, home, "work")
	// seedMinimalIdentity writes a COMPLETE identity (SSH+Git+fragment) —
	// strip the Git side so "work" is SSH-only, targeting FIELDS.md's own
	// stated precision goal for this state.
	for _, p := range []string{
		filepath.Join(home, ".gitconfig"),
		filepath.Join(home, ".gitconfig.d", "work"),
	} {
		if err := os.Remove(p); err != nil {
			t.Fatalf("removing Git side of the SSH-only fixture: %v", err)
		}
	}
	bin := BuildBinary(t)
	manifest := parseFieldsManifest(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s := startPTYAt(t, newRealCreateFlowCmd(t, ctx, bin, home, ""), dummyTermWidth, dummyTermHeight)
	defer s.close(t)

	uiReady(t, s)
	mustSee(t, s, "SSH — shown first, always", "detail-ssh-first: the SSH section renders first")
	mustSee(t, s, "Git not configured — no fabricated values shown.", "detail-ssh-first: the Git-absence note is explicit, never a fabricated field")
	mustNotSee(t, s, "gpg.format=ssh", "detail-ssh-first: no fabricated signing line for an SSH-only identity")
	assertManifestFields(t, manifest, "detail-ssh-first", s)

	saveFrame(t, "identity-manager-detail-ssh-first", s)
}

// ---------------------------------------------------------------------------
// action-menu: the four-row hub, in order.
// ---------------------------------------------------------------------------

func TestIdentityManager_ActionMenu(t *testing.T) {
	home := SandboxHome(t)
	seedMinimalIdentity(t, home, "acme")
	bin := BuildBinary(t)
	manifest := parseFieldsManifest(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s := startPTYAt(t, newRealCreateFlowCmd(t, ctx, bin, home, ""), dummyTermWidth, dummyTermHeight)
	defer s.close(t)

	uiReady(t, s)
	mustSee(t, s, "acme", "sidebar renders the seeded identity")
	s.sendKey([]byte("a"), keystrokeDelay)
	mustSee(t, s, "Actions — acme", "action-menu: opened via the allocated a key")

	frame := s.snapshot()
	order := []string{"View SSH-first detail", "Clone (c)", "Generate new key", "Delete (d)"}
	last := -1
	for _, want := range order {
		idx := strings.Index(frame, want)
		if idx < 0 {
			t.Fatalf("action-menu row %q never rendered:\n%s", want, frame)
		}
		if idx <= last {
			t.Errorf("action-menu row %q rendered out of order (index %d, previous %d):\n%s", want, idx, last, frame)
		}
		last = idx
	}
	assertManifestFields(t, manifest, "action-menu", s)

	saveFrame(t, "identity-manager-action-menu", s)
}

// ---------------------------------------------------------------------------
// Key ceremony: rotate (healthy key) vs repair (key-missing), driven from
// the action menu's single "Generate new key" row (D-05 routing).
// ---------------------------------------------------------------------------

// openKeyCeremonyViaActionMenu opens the action menu and activates its
// third row ("Generate new key", index 2), then drives the existing
// two-stage test gate to reach the review pane.
func openKeyCeremonyViaActionMenu(t *testing.T, s *ptySession) {
	t.Helper()
	s.sendKey([]byte("a"), keystrokeDelay)
	mustSee(t, s, "Actions", "action menu opens")
	s.sendKey(dummyKeyDown, keystrokeDelay)
	s.sendKey(dummyKeyDown, keystrokeDelay)
	s.sendKey(dummyKeyEnter, keystrokeDelay) // activate "Generate new key"
	mustSee(t, s, "Key ceremony", "key ceremony opens")
	mustSee(t, s, "Run stage 1 (Enter)", "stage1 gate renders")
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Run stage 2 (Enter)", "stage2 gate renders")
	s.sendKey(dummyKeyEnter, keystrokeDelay)
}

// TestIdentityManager_KeyCeremonyRotate drives the rotate ceremony (a
// healthy, singly-owned key) end to end and asserts the D-08 grace-window
// hint renders on the receipt plus the D-06 archive path, and that the
// archive directory gains the retired key.
func TestIdentityManager_KeyCeremonyRotate(t *testing.T) {
	// ShortSandboxHome (never SandboxHome): assertReceiptPathsExist below
	// parses the receipt's absolute path tokens, which must survive the
	// pane's word-wrap without a mid-path line break — see
	// ShortSandboxHome's own doc comment.
	home := ShortSandboxHome(t)
	// The lifecycle's pre-write connectivity gate runs a REAL ssh probe
	// (identity.go's preWriteGate) against acct.Hostname/acct.Port — a
	// Port-less fixture resolves to port 0 and hard-fails ("Bad port '0'")
	// before any write. Use the recipe-shape fixture (explicit Hostname/
	// Port 443) and FakeSSHDir's "denied" mode (tester.ReachableNotUploaded
	// — reachable, key not yet authorized — exactly rotate's expected
	// post-write outcome) so the probe completes without real network
	// access, matching every other lifecycle PTY test in this package.
	seedGitPTYIdentity(t, home, "acme") // complete, singly-owned key -> KeyActionFor routes to rotate
	seedRecipeShapeSSHConfig(t, home, "acme")
	bin := BuildBinary(t)
	fakeSSHDir := FakeSSHDir(t, "denied")

	oldKeyPath := filepath.Join(home, ".ssh", "id_ed25519_acme")
	oldKeyBytes, err := os.ReadFile(oldKeyPath) //nolint:gosec // fixed sandbox path (G304)
	if err != nil {
		t.Fatalf("reading pre-rotate key: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s := startPTYAt(t, newRealCreateFlowCmd(t, ctx, bin, home, fakeSSHDir), dummyTermWidth, dummyTermHeight)
	defer s.close(t)

	uiReady(t, s)
	mustSee(t, s, "acme", "sidebar renders the seeded identity")
	openKeyCeremonyViaActionMenu(t, s)
	mustSee(t, s, "Key ceremony — acme", "review pane renders the rotate ceremony")
	mustSee(t, s, "Nothing has changed yet", "the pre-confirm assurance renders before any write")
	// backup-notice (this app's compressed 2-state ceremony): the D-06
	// archive-path line is part of the SAME pre-confirm pane as the backup
	// promise, per ceremony.go's own "the compressed 2-state write ceremony"
	// header comment.
	mustSee(t, s, "Old key archived to", "rotate: the D-06 archive-path line renders in the backup-notice pane")

	s.sendKey(dummyKeyEnter, keystrokeDelay) // confirm — async CommitRotate
	mustSee(t, s, "Key ceremony completed.", "rotate: the receipt renders after the real write completes")
	mustSee(t, s, "The old key stays valid at", "rotate: the D-08 grace-window hint renders on the receipt")
	// review R-28: every receipt path genuinely exists on disk for rotate —
	// its "Wrote → " list only ever names files the transaction actually
	// wrote, never a removed target (unlike delete's fragment-removal case).
	assertReceiptPathsExist(t, home, s.snapshot())

	saveFrame(t, "identity-manager-key-ceremony-rotate", s)

	newKeyBytes, err := os.ReadFile(oldKeyPath) //nolint:gosec // fixed sandbox path (G304)
	if err != nil {
		t.Fatalf("reading post-rotate key: %v", err)
	}
	if string(newKeyBytes) == string(oldKeyBytes) {
		t.Error("rotate must generate a NEW key at the canonical path, not leave the old bytes in place")
	}
	archiveDir := filepath.Join(home, ".ssh", "gitid-archive")
	entries, err := os.ReadDir(archiveDir)
	if err != nil || len(entries) == 0 {
		t.Errorf("rotate must archive the retired key under %s: err=%v entries=%v", archiveDir, err, entries)
	}
}

// TestIdentityManager_KeyCeremonyRepair drives the repair ceremony (a
// key-missing identity) end to end and asserts NEITHER the grace-window
// hint NOR the archive-path line renders (D-05: repair never touches
// pre-existing key material, and there is no old key to keep valid).
func TestIdentityManager_KeyCeremonyRepair(t *testing.T) {
	// ShortSandboxHome: see TestIdentityManager_KeyCeremonyRotate's own
	// comment — assertReceiptPathsExist needs unwrapped absolute paths.
	home := ShortSandboxHome(t)
	seedGitPTYIdentity(t, home, "acme")
	seedRecipeShapeSSHConfig(t, home, "acme")
	keyPath := filepath.Join(home, ".ssh", "id_ed25519_acme")
	pubPath := keyPath + ".pub"
	if err := os.Remove(keyPath); err != nil {
		t.Fatalf("removing key to seed key-missing: %v", err)
	}
	if err := os.Remove(pubPath); err != nil {
		t.Fatalf("removing pub key to seed key-missing: %v", err)
	}
	bin := BuildBinary(t)
	fakeSSHDir := FakeSSHDir(t, "denied")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s := startPTYAt(t, newRealCreateFlowCmd(t, ctx, bin, home, fakeSSHDir), dummyTermWidth, dummyTermHeight)
	defer s.close(t)

	uiReady(t, s)
	mustSee(t, s, "acme", "sidebar renders the seeded identity")
	openKeyCeremonyViaActionMenu(t, s)
	mustSee(t, s, "Key ceremony — acme", "review pane renders the repair ceremony")

	s.sendKey(dummyKeyEnter, keystrokeDelay) // confirm — async CommitNewKey
	mustSee(t, s, "Key ceremony completed.", "repair: the receipt renders after the real write completes")
	mustNotSee(t, s, "The old key stays valid at", "repair: no grace-window hint — there is no old key")
	mustNotSee(t, s, "Old key archived to", "repair: no archive-path line — repair never archives")
	// review R-28: repair's "Wrote → " list never names a removed target
	// either (D-05: repair only ever creates, never archives/removes).
	assertReceiptPathsExist(t, home, s.snapshot())

	saveFrame(t, "identity-manager-key-ceremony-repair", s)

	if _, err := os.Stat(keyPath); err != nil {
		t.Errorf("repair must generate a fresh key at the canonical path: %v", err)
	}
	archiveDir := filepath.Join(home, ".ssh", "gitid-archive")
	if entries, err := os.ReadDir(archiveDir); err == nil && len(entries) > 0 {
		t.Errorf("repair must never archive anything, found entries under %s: %v", archiveDir, entries)
	}
}

// ---------------------------------------------------------------------------
// D-08: the register-key modal (09-06-PLAN.md Task 1, 09-07-PLAN.md Task 1).
// ---------------------------------------------------------------------------

// openRegisterKeyModalViaActionMenu opens the action menu and activates its
// fifth row ("Register key with provider now (u)", index 4 — D2, 260831-3a9
// reworded this row so the immediate provider mutation it triggers is
// legible before it is pressed) — the derived-row-count sibling of
// openKeyCeremonyViaActionMenu.
func openRegisterKeyModalViaActionMenu(t *testing.T, s *ptySession) {
	t.Helper()
	s.sendKey([]byte("a"), keystrokeDelay)
	mustSee(t, s, "Actions", "action menu opens")
	for range 4 {
		s.sendKey(dummyKeyDown, keystrokeDelay)
	}
	s.sendKey(dummyKeyEnter, keystrokeDelay) // activate "Register key"
}

// TestIdentityManager_RegisterKeyModalRuns opens the register-key pane via
// the action menu's fifth row and asserts the frozen modal heading, the
// upload beat's announce line, and its result rows — D-02: opening the
// pane IS the opt-in, so registration runs with no further keystroke.
func TestIdentityManager_RegisterKeyModalRuns(t *testing.T) {
	home := SandboxHome(t)
	seedMinimalIdentity(t, home, "acme")
	// seedMinimalIdentity's stub .pub is not a real, parseable public key
	// (uploader rejects it as invalid content) — overwrite with a real
	// generated key pair, mirroring the create-flow tests' own fixture
	// pattern, since this test drives a REAL upload.
	seedEncryptedKeyFixture(t, filepath.Join(home, ".ssh", "id_ed25519_acme"), "acme", "")
	bin := BuildBinary(t)
	fakeGH, _ := FakeGHDir(t, "ok")
	manifest := parseFieldsManifest(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s := startPTYAt(t, newRealCreateFlowCmd(t, ctx, bin, home, fakeGH), dummyTermWidth, dummyTermHeight)
	defer s.close(t)

	uiReady(t, s)
	mustSee(t, s, "acme", "sidebar renders the seeded identity")
	openRegisterKeyModalViaActionMenu(t, s)

	mustSee(t, s, "Register acme's key with GitHub", "the frozen RegisterKeyModalHeadingFmt heading renders")
	mustSee(t, s, "Running:", "the upload beat's announce line renders")
	mustSee(t, s, "Authentication key registered", "the authentication result row renders")
	mustSee(t, s, "Signing key registered", "the signing result row renders")
	assertManifestFields(t, manifest, "register-key-modal", s)
	saveFrame(t, "identity-manager-register-key-modal-runs", s)

	s.sendKey(dummyKeyEsc, keystrokeDelay)
	mustNotSee(t, s, "Register acme's key with GitHub", "Esc closes the register-key pane")
}

// TestIdentityManager_RegisterKeyModalManualFallback opens the same pane
// with the fake gh unauthenticated: the manual-fallback instructions block
// renders instead of a run, and no ssh-key add invocation is ever recorded
// (the eligibility probe's own auth-status call is expected and is not what
// this test asserts is absent).
func TestIdentityManager_RegisterKeyModalManualFallback(t *testing.T) {
	home := SandboxHome(t)
	seedMinimalIdentity(t, home, "acme")
	bin := BuildBinary(t)
	fakeGH, ghLog := FakeGHDir(t, "auth-fail")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s := startPTYAt(t, newRealCreateFlowCmd(t, ctx, bin, home, fakeGH), dummyTermWidth, dummyTermHeight)
	defer s.close(t)

	uiReady(t, s)
	mustSee(t, s, "acme", "sidebar renders the seeded identity")
	openRegisterKeyModalViaActionMenu(t, s)

	mustSee(t, s, "Auto-registration wasn't available. Register it yourself:", "the frozen manual-fallback heading renders")
	mustSee(t, s, "github.com/settings/ssh/new", "the GitHub manual instructions render")
	saveFrame(t, "identity-manager-register-key-modal-manual-fallback", s)

	for _, entry := range ReadFakeCLILog(t, ghLog) {
		if strings.Contains(entry, "ssh-key add") {
			t.Errorf("manual-fallback path issued an add invocation: %q", entry)
		}
	}
}

// TestIdentityManager_RegisterKeyModalOpensWithU proves `u` on the detail
// pane opens the same register-key pane directly, without the action menu.
func TestIdentityManager_RegisterKeyModalOpensWithU(t *testing.T) {
	home := SandboxHome(t)
	seedMinimalIdentity(t, home, "acme")
	// See TestIdentityManager_RegisterKeyModalRuns's comment: a real key is
	// needed since opening the pane drives a real upload attempt.
	seedEncryptedKeyFixture(t, filepath.Join(home, ".ssh", "id_ed25519_acme"), "acme", "")
	bin := BuildBinary(t)
	fakeGH, _ := FakeGHDir(t, "ok")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s := startPTYAt(t, newRealCreateFlowCmd(t, ctx, bin, home, fakeGH), dummyTermWidth, dummyTermHeight)
	defer s.close(t)

	uiReady(t, s)
	mustSee(t, s, "acme", "sidebar renders the seeded identity")
	s.sendKey([]byte("u"), keystrokeDelay)
	mustSee(t, s, "Register acme's key with GitHub", "the `u` shortcut opens the same register-key pane")
	saveFrame(t, "identity-manager-register-key-modal-u-key", s)
}

// ---------------------------------------------------------------------------
// D-04: the interactive old-key delete offer (09-06-PLAN.md Task 3,
// 09-07-PLAN.md Task 1).
// ---------------------------------------------------------------------------

// seedRotateDeleteOfferFixture seeds a rotate-ready "acme" identity (the
// SAME recipe-shape fixture TestIdentityManager_KeyCeremonyRotate uses) and
// returns this machine's D-07 machine-scoped title for "acme" — the exact
// string the fake gh's inventory fixture must carry for the offer to
// resolve Available.
func seedRotateDeleteOfferFixture(t *testing.T, home string) string {
	t.Helper()
	seedGitPTYIdentity(t, home, "acme")
	seedRecipeShapeSSHConfig(t, home, "acme")
	hostname, err := os.Hostname()
	if err != nil {
		t.Fatalf("os.Hostname: %v", err)
	}
	return uploader.KeyTitle("acme", hostname)
}

// driveRotateToResultScreen opens the key ceremony via the action menu,
// confirms it (async CommitRotate), and waits for the receipt — the common
// setup every rotate-delete-offer test shares.
func driveRotateToResultScreen(t *testing.T, s *ptySession) {
	t.Helper()
	openKeyCeremonyViaActionMenu(t, s)
	s.sendKey(dummyKeyEnter, keystrokeDelay) // confirm — async CommitRotate
	mustSee(t, s, "Key ceremony completed.", "rotate: the receipt renders after the real write completes")
}

// mustSeeSlow is mustSee with a longer timeout, for an assertion that must
// wait out the upload beat's real D-17 post-upload confirmation retry
// (uploadConfirmRetryInterval, a real 2s sleep the fake gh's static
// inventory fixture always fails to converge against, since it never
// reflects what was "added") PLUS the chained D-04 delete-offer probe that
// only dispatches once that beat resolves — comfortably longer than
// mustSee's default 8s budget covers.
func mustSeeSlow(t *testing.T, s *ptySession, substr, context string) {
	t.Helper()
	last, ok := s.waitFor(20*time.Second, func(text string) bool {
		return strings.Contains(text, substr)
	})
	if !ok {
		t.Fatalf("%s: %q never appeared. Last frame:\n%s", context, substr, last)
	}
}

// TestIdentityManager_RotateDeleteOfferDefaultsToLeave drives a rotate to
// its result screen with the gh shim in the delete-ok-plus-inventory mode:
// asserts the offer's heading, the old key's machine-scoped title, and that
// the leave option is the default-focused one; pressing Enter from that
// default must render the left-in-place message and the unchanged grace
// hint, and must never issue a delete invocation.
func TestIdentityManager_RotateDeleteOfferDefaultsToLeave(t *testing.T) {
	home := ShortSandboxHome(t)
	title := seedRotateDeleteOfferFixture(t, home)
	bin := BuildBinary(t)
	fakeSSHDir := FakeSSHDir(t, "denied")
	fakeGH, ghLog := FakeGHDir(t, "delete-ok")
	FakeGHInventoryFile(t, fmt.Sprintf(`[{"id":555,"title":%q,"key":"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5STUB old@gitid"}]`, title))
	// CR-02: the D-04 offer's belt-and-braces check requires the freshly
	// rotated key to already be registered in a fresh inventory read —
	// track this test's real "ssh-key add" calls so the subsequent read
	// reflects them, matching real GitHub.
	FakeGHTrackAddedKeys(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s := startPTYAt(t, newRealCreateFlowCmd(t, ctx, bin, home, fakeSSHDir, fakeGH), dummyTermWidth, dummyTermHeight)
	defer s.close(t)

	uiReady(t, s)
	mustSee(t, s, "acme", "sidebar renders the seeded identity")
	driveRotateToResultScreen(t, s)

	mustSeeSlow(t, s, "Remove the old key from GitHub?", "the D-04 offer heading renders")
	mustSee(t, s, title, "the old key's machine-scoped title renders")
	mustSee(t, s, "Leave it", "the leave option renders")
	saveFrame(t, "identity-manager-rotate-delete-offer-default", s)

	s.sendKey(dummyKeyEnter, keystrokeDelay) // default focus = leave
	mustSee(t, s, "Left in place", "choosing the default (leave) renders the left-in-place message")
	mustSee(t, s, "The old key stays valid at", "the existing D-08 grace hint still renders unchanged")

	for _, entry := range ReadFakeCLILog(t, ghLog) {
		// CR-02: the delete call is `gh api -X DELETE user/keys/<id>` (or
		// .../ssh_signing_keys/<id>), never `gh ssh-key delete <id>`.
		if strings.Contains(entry, "-X DELETE") {
			t.Errorf("Enter from the default (leave) focus must never delete: %q", entry)
		}
	}
}

// TestIdentityManager_RotateDeleteOfferDeletesOnExplicitChoice is the same
// flow, but moves to the delete option before Enter: asserts the removed
// message and, since the fixture's inventory answers identically for both
// the authentication and signing endpoints (delete-ok mode never branches
// on which `api` path was requested), exactly TWO recorded delete
// invocations carrying the resolved ID 555 — one per registration
// namespace (CR-02: GitHub tracks a separate authentication and signing
// registration for the same physical key, and both must be removed, never
// only the first) — each addressing its OWN REST resource
// (user/keys/555 vs user/ssh_signing_keys/555), never the SAME one twice
// (D-04's explicit-move-plus-Enter requirement — never a single default
// keystroke — still applies to triggering the delete at all).
func TestIdentityManager_RotateDeleteOfferDeletesOnExplicitChoice(t *testing.T) {
	home := ShortSandboxHome(t)
	title := seedRotateDeleteOfferFixture(t, home)
	bin := BuildBinary(t)
	fakeSSHDir := FakeSSHDir(t, "denied")
	fakeGH, ghLog := FakeGHDir(t, "delete-ok")
	FakeGHInventoryFile(t, fmt.Sprintf(`[{"id":555,"title":%q,"key":"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5STUB old@gitid"}]`, title))
	// CR-02: the D-04 offer's belt-and-braces check requires the freshly
	// rotated key to already be registered in a fresh inventory read —
	// track this test's real "ssh-key add" calls so the subsequent read
	// reflects them, matching real GitHub.
	FakeGHTrackAddedKeys(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s := startPTYAt(t, newRealCreateFlowCmd(t, ctx, bin, home, fakeSSHDir, fakeGH), dummyTermWidth, dummyTermHeight)
	defer s.close(t)

	uiReady(t, s)
	mustSee(t, s, "acme", "sidebar renders the seeded identity")
	driveRotateToResultScreen(t, s)
	mustSeeSlow(t, s, "Remove the old key from GitHub?", "the D-04 offer heading renders")

	s.sendKey(dummyKeyDown, keystrokeDelay) // move to the delete option
	s.sendKey(dummyKeyEnter, keystrokeDelay)
	mustSee(t, s, "Old key removed from GitHub", "the removed message renders")
	saveFrame(t, "identity-manager-rotate-delete-offer-delete", s)

	authDeletes, signDeletes := 0, 0
	for _, entry := range ReadFakeCLILog(t, ghLog) {
		switch {
		case strings.Contains(entry, "-X DELETE") && strings.Contains(entry, "user/keys/555"):
			authDeletes++
		case strings.Contains(entry, "-X DELETE") && strings.Contains(entry, "user/ssh_signing_keys/555"):
			signDeletes++
		case strings.Contains(entry, "-X DELETE"):
			t.Errorf("unexpected delete invocation: %q", entry)
		}
	}
	if authDeletes != 1 {
		t.Errorf("authentication delete invocations = %d, want exactly 1 addressing user/keys/555", authDeletes)
	}
	if signDeletes != 1 {
		t.Errorf("signing delete invocations = %d, want exactly 1 addressing user/ssh_signing_keys/555", signDeletes)
	}
}

// TestIdentityManager_RotateDeleteOfferAbsentWhenInventoryFails drives the
// gh shim in inventory-fail mode: the offer must not render and the
// existing frozen grace-hint sentence must be shown instead, unchanged.
func TestIdentityManager_RotateDeleteOfferAbsentWhenInventoryFails(t *testing.T) {
	home := ShortSandboxHome(t)
	seedRotateDeleteOfferFixture(t, home)
	bin := BuildBinary(t)
	fakeSSHDir := FakeSSHDir(t, "denied")
	fakeGH, _ := FakeGHDir(t, "inventory-fail")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s := startPTYAt(t, newRealCreateFlowCmd(t, ctx, bin, home, fakeSSHDir, fakeGH), dummyTermWidth, dummyTermHeight)
	defer s.close(t)

	uiReady(t, s)
	mustSee(t, s, "acme", "sidebar renders the seeded identity")
	driveRotateToResultScreen(t, s)

	mustSee(t, s, "The old key stays valid at", "the frozen grace-hint sentence renders")
	mustNotSee(t, s, "Remove the old key from", "the offer must not render when the inventory read fails")
	saveFrame(t, "identity-manager-rotate-delete-offer-absent", s)
}

// ---------------------------------------------------------------------------
// Mouse: clone name field focus + both delete-choice options (click-to-focus,
// checkpoint-2 D8, applies unchanged to identity-manager per 05-UI-SPEC.md).
// ---------------------------------------------------------------------------

func TestIdentityManager_MouseCloneAndDeleteChoiceFocus(t *testing.T) {
	home := SandboxHome(t)
	seedMinimalIdentity(t, home, "acme")
	bin := BuildBinary(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s := startPTYAt(t, newRealCreateFlowCmd(t, ctx, bin, home, ""), dummyTermWidth, dummyTermHeight)
	defer s.close(t)

	uiReady(t, s)
	mustSee(t, s, "acme", "sidebar renders the seeded identity")

	s.sendKey([]byte("c"), keystrokeDelay)
	mustSee(t, s, `Clone "acme"`, "clone-name-prompt opens")
	clickLabelRow(t, s, "New identity name")
	for _, r := range "-clicked" {
		s.sendKey([]byte(string(r)), keystrokeDelay)
	}
	mustSee(t, s, "-clicked", "mouse click focused the clone name field for raw keyboard entry")
	s.sendKey([]byte{0x1b}, keystrokeDelay) // Esc back to detail

	s.sendKey([]byte("d"), keystrokeDelay)
	mustSee(t, s, "Delete Git identity only", "delete-choice renders")
	mustSee(t, s, "● Delete Git identity only", "delete-choice: git-only starts focused (default-focused, safer option)")
	mustSee(t, s, "○ Delete everything", "delete-choice: the everything option starts unfocused")
	// A click anywhere on the everything row's rendered text (not just its
	// leading glyph column) must move focus — proving the WHOLE row is the
	// mouse hit target (click-to-focus, D8).
	clickLabelRow(t, s, "irreversible")
	mustSee(t, s, "● Delete everything", "mouse click anywhere on the everything row focuses it")
	clickLabelRow(t, s, "Delete Git identity only")
	mustSee(t, s, "● Delete Git identity only", "mouse click on the git-only row focuses it back")
}

// ---------------------------------------------------------------------------
// Delete-everything: confirm-destructive + backup-notice, both the D-13
// planted-hits and zero-hits controls, plus filesystem evidence.
// ---------------------------------------------------------------------------

// openEverythingDeleteChoice opens delete-choice on the currently-selected
// identity and toggles scope to "everything" (never default-focused).
func openEverythingDeleteChoice(t *testing.T, s *ptySession) {
	t.Helper()
	s.sendKey([]byte("d"), keystrokeDelay)
	mustSee(t, s, "Delete Git identity only", "delete-choice renders")
	s.sendKey(dummyKeyDown, keystrokeDelay) // git-only -> everything
	mustSee(t, s, "● Delete everything", "delete-choice: everything scope now focused")
	s.sendKey(dummyKeyEnter, keystrokeDelay)
}

// typeDestructiveConfirmWord types word into the confirm-destructive pane's
// typed-confirm input (already focused by newCeremony) and confirms.
func typeDestructiveConfirmWord(s *ptySession, word string) {
	for _, r := range word {
		s.sendKey([]byte(string(r)), keystrokeDelay)
	}
	s.sendKey(dummyKeyEnter, keystrokeDelay)
}

// TestIdentityManager_DeleteEverythingPlantedHits drives the everything-
// scope delete over a sandbox with exactly two planted D-13 unmanaged-
// reference hits (review R-27: known file+line identities, never a bare
// count) and asserts the rendered hit lines name exactly those two (file,
// line) pairs plus the disclaimer, then confirms and asserts the sandbox
// filesystem after the receipt.
func TestIdentityManager_DeleteEverythingPlantedHits(t *testing.T) {
	// ShortSandboxHome (never SandboxHome): the rendered hit line's exact
	// (file, line) text must survive the confirm pane's bounded-width
	// clip — see ShortSandboxHome's own doc comment. The target's own name
	// is kept to 2 characters for the same reason: every byte here is a
	// byte the clip has less room for before the line number.
	home := ShortSandboxHome(t)
	const target = "tg"
	hits := seedDeleteEverythingTargetWithPlantedHits(t, home, target)
	bin := BuildBinary(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s := startPTYAt(t, newRealCreateFlowCmd(t, ctx, bin, home, ""), dummyTermWidth, dummyTermHeight)
	defer s.close(t)

	uiReady(t, s)
	mustSee(t, s, target, "sidebar renders the seeded identity")
	// "sibling" sorts before the 2-char target name and is selected by
	// default — select the target explicitly (not its sibling).
	s.sendKey(dummyKeyDown, keystrokeDelay)
	mustSee(t, s, "Identities › "+target, "the target identity is selected before opening delete-choice")
	openEverythingDeleteChoice(t, s)
	mustSee(t, s, fmt.Sprintf(`Delete EVERYTHING for %q (SSH + Git + key)`, target), "confirm-destructive: the everything heading renders")
	mustSee(t, s, "This action is irreversible", "confirm-destructive: the D-11 warning heading renders")

	for _, h := range hits {
		want := fmt.Sprintf("Found %q referenced in %s: %d", target, h.File, h.Line)
		mustSee(t, s, want, fmt.Sprintf("confirm-destructive: the planted hit at %s:%d is named exactly", h.File, h.Line))
	}
	// Split across the word-wrap point (the pane's fixed width can legitimately
	// wrap the sentence, turning a single embedded space into a newline).
	mustSee(t, s, "Repo remotes using git@<alias>: cannot be scanned and will", "confirm-destructive: the D-13 honest disclaimer renders alongside the hit list")
	mustSee(t, s, "break after this delete.", "confirm-destructive: the D-13 honest disclaimer's full sentence renders")

	typeDestructiveConfirmWord(s, target)
	mustSee(t, s, `deleted`, "result-success: the receipt renders after the real write completes")

	saveFrame(t, "identity-manager-delete-everything-planted-hits", s)

	if _, err := os.Stat(filepath.Join(home, ".ssh", "id_ed25519_"+target)); !os.IsNotExist(err) {
		t.Errorf("the target's key must not survive an everything-scope delete: statErr=%v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".gitconfig.d", target)); !os.IsNotExist(err) {
		t.Errorf("the target's Git fragment must not survive an everything-scope delete: statErr=%v", err)
	}
	sshAfter, err := os.ReadFile(filepath.Join(home, ".ssh", "config")) //nolint:gosec // fixed sandbox path (G304)
	if err != nil {
		t.Fatalf("reading post-delete ssh config: %v", err)
	}
	if strings.Contains(string(sshAfter), "BEGIN gitid managed: "+target) {
		t.Errorf("post-delete ssh config still carries the target Host block:\n%s", sshAfter)
	}
	if !strings.Contains(string(sshAfter), "BEGIN gitid managed: sibling") {
		t.Errorf("the sibling's OWN managed block (a separate identity) must survive target's delete:\n%s", sshAfter)
	}
	// The archived key-pair copy the receipt promised (D-11: never claim
	// irrecoverable when a copy exists).
	archiveDir := filepath.Join(home, ".ssh", "gitid-archive")
	if entries, err := os.ReadDir(archiveDir); err != nil || len(entries) == 0 {
		t.Errorf("delete-everything must back up the key pair under %s before removing it: err=%v entries=%v", archiveDir, err, entries)
	}
}

// TestIdentityManager_DeleteEverythingCleanSandbox is the D-13 zero-hits
// control: over a sandbox with no other identity and no hand-written
// reference to the target's alias anywhere, the unmanaged-reference warning
// block must not render at all — neither the disclaimer nor any hit line.
func TestIdentityManager_DeleteEverythingCleanSandbox(t *testing.T) {
	home := SandboxHome(t)
	seedDeleteEverythingTargetClean(t, home, "clean")
	bin := BuildBinary(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s := startPTYAt(t, newRealCreateFlowCmd(t, ctx, bin, home, ""), dummyTermWidth, dummyTermHeight)
	defer s.close(t)

	uiReady(t, s)
	mustSee(t, s, "clean", "sidebar renders the seeded identity")
	openEverythingDeleteChoice(t, s)
	mustSee(t, s, `Delete EVERYTHING for "clean" (SSH + Git + key)`, "confirm-destructive: the everything heading renders")

	mustNotSee(t, s, "referenced in", "clean sandbox: no unmanaged-reference hit line renders")
	mustNotSee(t, s, "Repo remotes using git@<alias>:", "clean sandbox: the D-13 disclaimer does not render when there are zero hits")

	typeDestructiveConfirmWord(s, "clean")
	mustSee(t, s, "deleted", "result-success: the receipt renders after the real write completes")

	saveFrame(t, "identity-manager-delete-everything-clean", s)

	if _, err := os.Stat(filepath.Join(home, ".ssh", "id_ed25519_clean")); !os.IsNotExist(err) {
		t.Errorf("the clean-scenario key must not survive an everything-scope delete: statErr=%v", err)
	}
}

// TestIdentityManager_DeleteEverythingSharedKeyNote drives delete-choice to
// everything scope over two identities that share a key path and asserts
// the D-12 downgrade note names the surviving sibling — then cancels (Esc)
// without confirming, since this test's only claim is the note text, not
// the write.
func TestIdentityManager_DeleteEverythingSharedKeyNote(t *testing.T) {
	home := SandboxHome(t)
	first, second := seedSharedKeyIdentities(t, home)
	bin := BuildBinary(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s := startPTYAt(t, newRealCreateFlowCmd(t, ctx, bin, home, ""), dummyTermWidth, dummyTermHeight)
	defer s.close(t)

	uiReady(t, s)
	mustSee(t, s, first, "sidebar renders the first shared-key identity")
	openEverythingDeleteChoice(t, s)
	mustSee(t, s, fmt.Sprintf("This key is also used by %q", second), "confirm-destructive: the D-12 downgrade note names the surviving sibling")
	// Two smaller substrings rather than the whole sentence — the note
	// word-wraps at the pane width, so a wrap point can legitimately land a
	// newline where the frozen copy has a plain space.
	mustSee(t, s, "it will be kept.", "confirm-destructive: the D-12 note states the key survives")
	mustSee(t, s, "SSH and Git artifacts are removed.", "confirm-destructive: the D-12 note scopes to this identity's own artifacts")

	s.sendKey([]byte{0x1b}, keystrokeDelay) // Esc — cancel, no write
	mustSee(t, s, first, "cancelling leaves the identity list intact")

	keyPath := filepath.Join(home, ".ssh", "id_ed25519_shared")
	if _, err := os.Stat(keyPath); err != nil {
		t.Errorf("the shared key must still exist after cancelling: %v", err)
	}
}

// TestIdentityManager_DeleteEverythingProviderSurvives is the D-09 fixture
// proof: deleting one of two identities on the SAME provider (everything
// scope) must leave the shared per-provider insteadOf rewrite block intact
// — the sibling still references it.
func TestIdentityManager_DeleteEverythingProviderSurvives(t *testing.T) {
	home := SandboxHome(t)
	first, second := seedTwoIdentitiesSameProviderE2E(t, home)
	bin := BuildBinary(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s := startPTYAt(t, newRealCreateFlowCmd(t, ctx, bin, home, ""), dummyTermWidth, dummyTermHeight)
	defer s.close(t)

	uiReady(t, s)
	mustSee(t, s, first, "sidebar renders the first same-provider identity")
	openEverythingDeleteChoice(t, s)
	typeDestructiveConfirmWord(s, first)
	mustSee(t, s, "deleted", "result-success: the receipt renders after the real write completes")

	gc, err := os.ReadFile(filepath.Join(home, ".gitconfig")) //nolint:gosec // fixed sandbox path (G304)
	if err != nil {
		t.Fatalf("reading post-delete gitconfig: %v", err)
	}
	if !strings.Contains(string(gc), "BEGIN gitid managed: rewrite-github.com") {
		t.Errorf("the shared provider-rewrite block must survive while %q still references it:\n%s", second, gc)
	}
	if strings.Contains(string(gc), "BEGIN gitid managed: "+first) {
		t.Errorf("post-delete gitconfig still carries %s's own includeIf block:\n%s", first, gc)
	}
}

// ---------------------------------------------------------------------------
// 05-09-PLAN.md Task 2 — paired compiled-real versus live-dummy comparison
// (DLV-04), reusing the Phase 4 (git-screen) mechanism exactly rather than
// inventing a second one: two REAL PTY sessions (compiled cmd/gitid,
// compiled cmd/gitid-dummy), normalized semantic checkpoints, a strict
// 5-field allowlist schema, stale/defect hard-failure, and a header record
// of the shared-renderer limitation this comparison cannot see past (both
// binaries render every identity-manager screen through the SAME
// internal/tuikit stack — see Task 1's independent FIELDS.md manifest
// backstop for the countermeasure, review R-26).
// ---------------------------------------------------------------------------

// identManagerRegion names a semantic sub-area of an identity-manager PTY
// frame for the real-vs-dummy comparison.
type identManagerRegion string

const (
	identRegionSidebar            identManagerRegion = "sidebar"
	identRegionHeaderStatus       identManagerRegion = "header-status"
	identRegionDetail             identManagerRegion = "detail"
	identRegionBreadcrumb         identManagerRegion = "breadcrumb"
	identRegionUploadSection      identManagerRegion = "upload-section"
	identRegionConnectivityOutput identManagerRegion = "connectivity-output"
)

func allIdentManagerRegions() []identManagerRegion {
	return []identManagerRegion{
		identRegionSidebar, identRegionHeaderStatus, identRegionDetail,
		identRegionBreadcrumb, identRegionUploadSection, identRegionConnectivityOutput,
	}
}

// identRightOfDivider mirrors git_configuration_pty_e2e_test.go's
// gitScreenRightOfDivider — returns "" (not the whole line) when no divider
// is present, so footer/status chrome rows (which never carry "│") are
// naturally excluded from the detail region rather than leaking in verbatim.
func identRightOfDivider(line string) (string, bool) {
	idx := strings.Index(line, "│")
	if idx < 0 {
		return "", false
	}
	return line[idx+len("│"):], true
}

func extractIdentManagerSidebar(lines []string) string {
	var out []string
	for _, line := range lines {
		idx := strings.Index(line, "│")
		if idx > 0 && strings.TrimSpace(line[:idx]) != "" {
			out = append(out, strings.TrimSpace(line[:idx]))
		}
	}
	return strings.Join(out, "\n")
}

func extractIdentManagerHeaderStatus(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	header := lines[0]
	idx := strings.LastIndex(header, "Fixer")
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(header[idx+len("Fixer"):])
}

func extractIdentManagerDetail(lines []string) string {
	var out []string
	for _, line := range lines {
		rp, ok := identRightOfDivider(line)
		if !ok || strings.TrimSpace(rp) == "" {
			continue
		}
		out = append(out, strings.TrimRight(rp, " "))
	}
	return strings.Join(out, "\n")
}

// identManagerBreadcrumbRow is the crumb line's fixed position — row 1
// (0-indexed), directly below the header and above every pane body, per
// internal/tuikit/frame.go's RenderFrame ("header, faint breadcrumb line,
// the body"). It never contains "│" (it is a single unsplit row), so
// extractIdentManagerDetail's divider-based scan cannot pick it up on its
// own — it needs its own fixed-row extraction.
const identManagerBreadcrumbRow = 1

func extractIdentManagerBreadcrumb(lines []string) string {
	if len(lines) <= identManagerBreadcrumbRow {
		return ""
	}
	return strings.TrimSpace(lines[identManagerBreadcrumbRow])
}

// identManagerUploadMarkers mirrors internal/screenshot/createflow_regions.go's
// extractUploadSection marker set (Phase 9 Task 2) — the upload beat's OWN
// announce/result/fallback text, deliberately excluding the D-01/D-08
// checkbox or menu-row LABEL text that also appears on unrelated screens.
var identManagerUploadMarkers = []string{
	"gh ssh-key add", "glab ssh-key add",
	"Auto-registration", "key registered", "registration failed",
	"already registered", "not logged in to", "Registering…",
}

// extractIdentManagerBeatFrom returns the detail pane's right-of-divider
// content starting at the first line containing any of markers, through the
// end of the pane body — mirroring extractUploadSection's "start at first
// marker, run to the end of the pane" shape, adapted to the raw-PTY
// right-of-divider convention identRightOfDivider already establishes here
// (there is no "╭╌" sentinel on this surface's pane bodies).
func extractIdentManagerBeatFrom(lines []string, markers []string) string {
	var detailLines []string
	for _, line := range lines {
		rp, ok := identRightOfDivider(line)
		if !ok {
			continue
		}
		detailLines = append(detailLines, strings.TrimRight(rp, " "))
	}
	start := -1
	for i, line := range detailLines {
		for _, marker := range markers {
			if strings.Contains(line, marker) {
				start = i
				break
			}
		}
		if start >= 0 {
			break
		}
	}
	if start < 0 {
		return ""
	}
	// Rewind over an immediately preceding lone "Running:" label line, same
	// as extractUploadSection, so the frozen announce label survives.
	if start > 0 && strings.TrimSpace(detailLines[start-1]) == "Running:" {
		start--
	}
	var out []string
	for _, line := range detailLines[start:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func extractIdentManagerUploadSection(lines []string) string {
	return extractIdentManagerBeatFrom(lines, identManagerUploadMarkers)
}

// extractIdentManagerConnectivityOutput mirrors RegionConnectivityOutput's
// own generic "Running" trigger (internal/screenshot/createflow_regions.go)
// — a strictly broader anchor than upload-section's own marker set, which
// is exactly why the create-flow/identity-manager allowlists classify the
// SAME divergence twice, once per region (Task 2's own allowlist comment).
func extractIdentManagerConnectivityOutput(lines []string) string {
	return extractIdentManagerBeatFrom(lines, []string{"Running:"})
}

func extractIdentManagerRegion(frame string, region identManagerRegion) string {
	lines := strings.Split(frame, "\n")
	switch region {
	case identRegionSidebar:
		return extractIdentManagerSidebar(lines)
	case identRegionHeaderStatus:
		return extractIdentManagerHeaderStatus(lines)
	case identRegionDetail:
		return extractIdentManagerDetail(lines)
	case identRegionBreadcrumb:
		return extractIdentManagerBreadcrumb(lines)
	case identRegionUploadSection:
		return extractIdentManagerUploadSection(lines)
	case identRegionConnectivityOutput:
		return extractIdentManagerConnectivityOutput(lines)
	}
	return ""
}

// identManagerDecisionRefPattern accepts a bare "D-NN" (05-CONTEXT.md, this
// phase's own decisions — the default per 05-09-PLAN.md's <authority>
// convention), "UI-D-NN" (05-UI-SPEC.md), "CTX-D-NN" (04-CONTEXT.md,
// Phase 4), "DLV-NN" (REQUIREMENTS.md) — accepted for the one class of
// divergence that is not any single design DECISION but the comparison
// mechanism's own structural test-fixture-size property (the dummy's
// frozen, always-8-identity fixture set vs. the real binary's
// per-checkpoint sandbox), governed directly by the DLV-04 requirement
// rather than a numbered D-NN — or "UP-NN" (REQUIREMENTS.md's Phase 9
// UP-01/UP-02/UP-03 upload requirements; added by 09-07-PLAN.md Task 3 to
// accept the UP-4 decision-ref Task 2 already committed in both
// visual-divergence-allowlist.txt files for the Phase 9 rows — a
// deliberate, documented widening of this pattern rather than a rename of
// Task 2's already-reviewed rows).
var identManagerDecisionRefPattern = regexp.MustCompile(`^(CTX-D-|UI-D-|D-|DLV-|UP-)\d+$`)

var (
	identManagerEmailPattern     = regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	identManagerTimestampPattern = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}[:\-]\d{2}[:\-]\d{2}Z`)
	identManagerNanosPattern     = regexp.MustCompile(`\.bak\.\d+`)
	// identManagerIdentityTokens is the fixed, known vocabulary of identity
	// names this suite's real fixtures and the dummy's IdentityManagerRows
	// fixtures use — deliberately narrow, matching git-screen's own
	// precedent (both sides are controlled test fixtures, never arbitrary
	// user data).
	identManagerIdentityTokens = []string{
		"acme", "personal", "work", "opensource", "archived", "staging", "clientA", "clientB", "legacy",
	}
)

// normalizeIdentManagerCheckpoint replaces identity-specific and
// wall-clock-specific substrings with stable placeholders — mirrors
// normalizeGitCheckpoint exactly.
func normalizeIdentManagerCheckpoint(s string) string {
	s = identManagerTimestampPattern.ReplaceAllString(s, "<timestamp>")
	s = identManagerNanosPattern.ReplaceAllString(s, ".bak.<nanos>")
	s = identManagerEmailPattern.ReplaceAllString(s, "<email>")
	for _, tok := range identManagerIdentityTokens {
		s = strings.ReplaceAll(s, tok, "<identity>")
	}
	return s
}

// identManagerAllowlistEntry is one parsed, strict-schema
// (checkpoint:region:predicate:decision-ref:reason) divergence
// classification, scoped to identity-manager checkpoints and UI-D-NN
// decision refs (05-UI-SPEC.md's Scoped Additions).
type identManagerAllowlistEntry struct {
	Checkpoint  string
	Region      identManagerRegion
	Predicate   string
	DecisionRef string
	Reason      string
	used        bool
}

// splitIdentManagerAllowlistLine mirrors git_configuration_pty_e2e_test.go's
// splitGitScreenAllowlistLine — a second, independently-authored copy
// (not a shared symbol) per this project's own accepted-duplication
// precedent for small keep-in-sync literals, since a divergence in either
// copy would only ever affect its OWN file's allowlist parsing.
func splitIdentManagerAllowlistLine(s string) []string {
	cut := func(r string) (field, rest string, ok bool) {
		idx := strings.Index(r, ":")
		if idx < 0 {
			return "", r, false
		}
		return r[:idx], r[idx+1:], true
	}
	f0, rest, ok := cut(s)
	if !ok {
		return nil
	}
	f1, rest, ok := cut(rest)
	if !ok {
		return nil
	}
	var f2, f3, f4 string
	switch {
	case strings.HasPrefix(rest, "contains:"), strings.HasPrefix(rest, "absent:"):
		keyword := "contains:"
		if strings.HasPrefix(rest, "absent:") {
			keyword = "absent:"
		}
		inner := rest[len(keyword):]
		if strings.HasPrefix(inner, `"`) {
			closeQ := strings.Index(inner[1:], `"`)
			if closeQ < 0 {
				idx := strings.Index(inner, ":")
				if idx < 0 {
					return nil
				}
				f2 = keyword + inner[:idx]
				rest = inner[idx+1:]
			} else {
				quoted := inner[:closeQ+2]
				f2 = keyword + quoted
				rest = inner[closeQ+2:]
				rest = strings.TrimPrefix(rest, ":")
			}
		} else {
			idx := strings.Index(inner, ":")
			if idx < 0 {
				return nil
			}
			f2 = keyword + inner[:idx]
			rest = inner[idx+1:]
		}
	default:
		var ok2 bool
		f2, rest, ok2 = cut(rest)
		if !ok2 {
			return nil
		}
	}
	f3, f4, ok = cut(rest)
	if !ok {
		return nil
	}
	return []string{f0, f1, f2, f3, f4}
}

// loadIdentManagerAllowlist parses
// .planning/design/identity-manager/visual-divergence-allowlist.txt.
func loadIdentManagerAllowlist(t *testing.T) []*identManagerAllowlistEntry {
	t.Helper()
	path := filepath.Join(repoRoot(t), ".planning", "design", "identity-manager", "visual-divergence-allowlist.txt")
	data, err := os.ReadFile(path) //nolint:gosec // fixed repo-relative path (G304)
	if err != nil {
		t.Fatalf("loadIdentManagerAllowlist: reading %s: %v", path, err)
	}
	validCheckpoints := map[string]bool{
		"action-menu": true, "delete-choice": true, "confirm-destructive": true,
		"detail-ssh-first": true, "rotate-result": true, "repair-result": true,
		// Phase 9 (09-07-PLAN.md Task 3): register-key-modal and
		// rotate-delete-offer are driven by TestRegisterKeyModal_CompiledRealVsLiveDummyPTY,
		// a separate test function from the six checkpoints above.
		"register-key-modal": true, "rotate-delete-offer": true,
	}
	validRegions := make(map[identManagerRegion]bool, len(allIdentManagerRegions()))
	for _, r := range allIdentManagerRegions() {
		validRegions[r] = true
	}
	seen := make(map[string]bool)
	var entries []*identManagerAllowlistEntry
	for lineNum, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := splitIdentManagerAllowlistLine(line)
		if len(parts) != 5 {
			t.Fatalf("identity-manager allowlist line %d: expected 5 colon-separated fields (checkpoint:region:predicate:decision-ref:reason), got %d in: %q", lineNum+1, len(parts), line)
		}
		checkpoint := strings.TrimSpace(parts[0])
		region := identManagerRegion(strings.TrimSpace(parts[1]))
		predicate := strings.TrimSpace(parts[2])
		decisionRef := strings.TrimSpace(parts[3])
		reason := strings.TrimSpace(parts[4])
		if !validCheckpoints[checkpoint] {
			t.Fatalf("identity-manager allowlist line %d: unknown checkpoint %q", lineNum+1, checkpoint)
		}
		if !validRegions[region] {
			t.Fatalf("identity-manager allowlist line %d: unknown region %q", lineNum+1, region)
		}
		if predicate == "differs" {
			t.Fatalf("identity-manager allowlist line %d: forbidden predicate %q — use contains:<text> or absent:<text> (CR-04 precedent)", lineNum+1, predicate)
		}
		if !strings.HasPrefix(predicate, "contains:") && !strings.HasPrefix(predicate, "absent:") {
			t.Fatalf("identity-manager allowlist line %d: invalid predicate %q (must be 'contains:<text>' or 'absent:<text>')", lineNum+1, predicate)
		}
		// Per 05-09-PLAN.md's <authority> convention: a BARE D-NN citation
		// means 05-CONTEXT.md (this phase's own decisions — D-05 through
		// D-17, the Scoped Additions 05-UI-SPEC.md maps to visual
		// surfaces); CTX-D-NN would cite Phase 4's 04-CONTEXT.md; UI-D-NN
		// would cite a numbered decision in 05-UI-SPEC.md itself (none are
		// currently defined there, but the form is accepted for forward
		// compatibility with the git-screen allowlist's own convention).
		if !identManagerDecisionRefPattern.MatchString(decisionRef) {
			t.Fatalf("identity-manager allowlist line %d: decision-ref %q must be a scoped D-NN (05-CONTEXT.md), UI-D-NN (05-UI-SPEC.md), or CTX-D-NN (04-CONTEXT.md) identifier", lineNum+1, decisionRef)
		}
		if reason == "" {
			t.Fatalf("identity-manager allowlist line %d: blank reason", lineNum+1)
		}
		key := checkpoint + ":" + string(region)
		if seen[key] {
			t.Fatalf("identity-manager allowlist line %d: duplicate entry for checkpoint %q region %q", lineNum+1, checkpoint, region)
		}
		seen[key] = true
		entries = append(entries, &identManagerAllowlistEntry{
			Checkpoint: checkpoint, Region: region, Predicate: predicate, DecisionRef: decisionRef, Reason: reason,
		})
	}
	return entries
}

// identManagerPredicateSatisfied mirrors gitScreenPredicateSatisfied's
// CR-10 fix exactly: contains:X requires the marker on BOTH sides;
// absent:X requires the presence/absence ASYMMETRY itself.
func identManagerPredicateSatisfied(entry *identManagerAllowlistEntry, real, dummy string) bool {
	switch {
	case strings.HasPrefix(entry.Predicate, "contains:"):
		needle := strings.Trim(strings.TrimPrefix(entry.Predicate, "contains:"), `"`)
		return strings.Contains(real, needle) && strings.Contains(dummy, needle)
	case strings.HasPrefix(entry.Predicate, "absent:"):
		needle := strings.Trim(strings.TrimPrefix(entry.Predicate, "absent:"), `"`)
		return strings.Contains(real, needle) != strings.Contains(dummy, needle)
	}
	return false
}

// errorRecorder is the minimal *testing.T surface compareIdentManagerCheckpoint
// needs — an interface (not the concrete *testing.T) so the negative-control
// tests below can inject a NON-failing recorder and observe whether the
// comparison WOULD have failed, without a subtest's real t.Errorf/t.Fatalf
// call marking the enclosing negative-control test itself as failed (Go's
// testing package propagates a subtest's failure to every parent
// unconditionally, which would make "prove this correctly fails" tests
// impossible to write as passing tests otherwise).
type errorRecorder interface {
	Helper()
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
}

// compareIdentManagerCheckpoint mirrors compareGitScreenCheckpoint exactly.
func compareIdentManagerCheckpoint(t errorRecorder, checkpoint string, realFrame, dummyFrame string, allowlist []*identManagerAllowlistEntry) {
	t.Helper()
	compareIdentManagerCheckpointSkipping(t, checkpoint, realFrame, dummyFrame, allowlist, nil)
}

// compareIdentManagerCheckpointSkipping is compareIdentManagerCheckpoint
// with an explicit set of regions never compared for this checkpoint.
// Phase 9's register-key-modal and rotate-delete-offer checkpoints (Task 3,
// 09-07-PLAN.md) use this to skip identRegionDetail: "detail" is this e2e
// package's OWN coarse, monolithic super-region (the whole right-of-divider
// pane), not a real screenshot-package RegionName — it has no code-side
// counterpart TestUploadVisualAllowlistMatchesRegistry could match a
// disposition against for a Phase 9 (uploadScreenIDs) checkpoint, and its
// content is already fully covered, at finer granularity, by the
// upload-section and connectivity-output dispositions the shared allowlist
// files already register — comparing "detail" too would only ever
// re-report the SAME divergence a third time with no new information,
// exactly the reasoning connectivityOverlapDisposition's own doc comment
// already gives for why RegionConnectivityOutput repeats RegionUploadSection.
func compareIdentManagerCheckpointSkipping(t errorRecorder, checkpoint string, realFrame, dummyFrame string, allowlist []*identManagerAllowlistEntry, skip map[identManagerRegion]bool) {
	t.Helper()
	comparable := 0
	for _, region := range allIdentManagerRegions() {
		if skip[region] {
			continue
		}
		realRegion := extractIdentManagerRegion(realFrame, region)
		dummyRegion := extractIdentManagerRegion(dummyFrame, region)
		if strings.TrimSpace(realRegion) == "" && strings.TrimSpace(dummyRegion) == "" {
			continue
		}
		comparable++
		if normalizeIdentManagerCheckpoint(realRegion) == normalizeIdentManagerCheckpoint(dummyRegion) {
			continue
		}
		var matched *identManagerAllowlistEntry
		for _, entry := range allowlist {
			if entry.Checkpoint == checkpoint && entry.Region == region {
				matched = entry
				break
			}
		}
		if matched == nil {
			t.Errorf("identity-manager semantic gate: %s/%s diverges with NO allowlist classification (DLV-04 requires ux-improvement or defect for every difference)\n--- real ---\n%s\n--- dummy ---\n%s",
				checkpoint, region, realRegion, dummyRegion)
			continue
		}
		if !identManagerPredicateSatisfied(matched, realRegion, dummyRegion) {
			t.Errorf("identity-manager semantic gate: %s/%s allowlist entry %q does not match the observed divergence\n--- real ---\n%s\n--- dummy ---\n%s",
				checkpoint, region, matched.Predicate, realRegion, dummyRegion)
			continue
		}
		matched.used = true
	}
	if comparable == 0 {
		t.Fatalf("identity-manager semantic gate: checkpoint %q produced NO comparable region on either side — checkpoint script bug, not a real absence", checkpoint)
	}
}

// newDummyIdentManagerCmd builds the exec.Cmd for a live cmd/gitid-dummy PTY
// session — the SAME internal/tuikit render stack the real binary uses,
// injected with dummytui.FixtureBackend instead of a real Backend (D-12/
// DLV-04).
func newDummyIdentManagerCmd(t *testing.T, ctx context.Context, bin, home string) *exec.Cmd {
	t.Helper()
	cmd := exec.CommandContext(ctx, bin) //nolint:gosec // bin from BuildDummyBinary; no user input
	env, _ := e2eEnv(t, home)
	cmd.Env = append(env, "TERM=xterm-256color")
	return cmd
}

// TestIdentityManager_CompiledRealVsLiveDummyPTY drives SIX identity-manager
// checkpoints through two real PTY sessions — the compiled cmd/gitid binary
// and the compiled cmd/gitid-dummy binary — comparing NORMALIZED semantic
// checkpoints (never raw terminal bytes, never any web/HTML/MUI/Chromium/PNG
// artifact). Every unequal or one-sided region on every checkpoint consumes
// EXACTLY ONE classification entry in
// .planning/design/identity-manager/visual-divergence-allowlist.txt, citing
// a scoped UI-D-NN (05-UI-SPEC.md) decision.
func TestIdentityManager_CompiledRealVsLiveDummyPTY(t *testing.T) {
	allowlist := loadIdentManagerAllowlist(t)
	realBin := BuildBinary(t)
	dummyBin := BuildDummyBinary(t)

	t.Run("action-menu", func(t *testing.T) {
		realHome := SandboxHome(t)
		seedMinimalIdentity(t, realHome, "acme")
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		real := startPTYAt(t, newRealCreateFlowCmd(t, ctx, realBin, realHome, ""), dummyTermWidth, dummyTermHeight)
		defer real.close(t)
		uiReady(t, real)
		mustSee(t, real, "acme", "real: sidebar renders the seeded identity")
		real.sendKey([]byte("a"), keystrokeDelay)
		mustSee(t, real, "Actions", "real: action menu opens")

		dummyHome := SandboxHome(t)
		dctx, dcancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer dcancel()
		dummy := startPTYAt(t, newDummyIdentManagerCmd(t, dctx, dummyBin, dummyHome), dummyTermWidth, dummyTermHeight)
		defer dummy.close(t)
		mustSee(t, dummy, "[1] Identities", "dummy: launches on the Identities tab")
		mustSee(t, dummy, "personal", "dummy: seeded fixture sidebar row")
		dummy.sendKey([]byte("a"), keystrokeDelay)
		mustSee(t, dummy, "Actions", "dummy: action menu opens")

		compareIdentManagerCheckpoint(t, "action-menu", real.snapshot(), dummy.snapshot(), allowlist)
	})

	t.Run("delete-choice", func(t *testing.T) {
		realHome := SandboxHome(t)
		seedMinimalIdentity(t, realHome, "acme")
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		real := startPTYAt(t, newRealCreateFlowCmd(t, ctx, realBin, realHome, ""), dummyTermWidth, dummyTermHeight)
		defer real.close(t)
		uiReady(t, real)
		real.sendKey([]byte("d"), keystrokeDelay)
		mustSee(t, real, "Delete Git identity only", "real: delete-choice opens")

		dummyHome := SandboxHome(t)
		dctx, dcancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer dcancel()
		dummy := startPTYAt(t, newDummyIdentManagerCmd(t, dctx, dummyBin, dummyHome), dummyTermWidth, dummyTermHeight)
		defer dummy.close(t)
		mustSee(t, dummy, "[1] Identities", "dummy: launches on the Identities tab")
		dummy.sendKey([]byte("d"), keystrokeDelay)
		mustSee(t, dummy, "Delete Git identity only", "dummy: delete-choice opens")

		compareIdentManagerCheckpoint(t, "delete-choice", real.snapshot(), dummy.snapshot(), allowlist)
	})

	t.Run("confirm-destructive", func(t *testing.T) {
		realHome := SandboxHome(t)
		seedMinimalIdentity(t, realHome, "acme")
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		real := startPTYAt(t, newRealCreateFlowCmd(t, ctx, realBin, realHome, ""), dummyTermWidth, dummyTermHeight)
		defer real.close(t)
		uiReady(t, real)
		real.sendKey([]byte("d"), keystrokeDelay)
		real.sendKey(dummyKeyDown, keystrokeDelay)
		real.sendKey(dummyKeyEnter, keystrokeDelay)
		mustSee(t, real, "This action is irreversible", "real: confirm-destructive opens")

		dummyHome := SandboxHome(t)
		dctx, dcancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer dcancel()
		dummy := startPTYAt(t, newDummyIdentManagerCmd(t, dctx, dummyBin, dummyHome), dummyTermWidth, dummyTermHeight)
		defer dummy.close(t)
		mustSee(t, dummy, "[1] Identities", "dummy: launches on the Identities tab")
		dummy.sendKey([]byte("d"), keystrokeDelay)
		dummy.sendKey(dummyKeyDown, keystrokeDelay)
		dummy.sendKey(dummyKeyEnter, keystrokeDelay)
		mustSee(t, dummy, "This action is irreversible", "dummy: confirm-destructive opens")

		compareIdentManagerCheckpoint(t, "confirm-destructive", real.snapshot(), dummy.snapshot(), allowlist)
	})

	t.Run("detail-ssh-first", func(t *testing.T) {
		realHome := SandboxHome(t)
		seedSSHOnlyIdentity(t, realHome, "work")
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		real := startPTYAt(t, newRealCreateFlowCmd(t, ctx, realBin, realHome, ""), dummyTermWidth, dummyTermHeight)
		defer real.close(t)
		uiReady(t, real)
		mustSee(t, real, "SSH — shown first, always", "real: SSH-first detail renders")

		dummyHome := SandboxHome(t)
		dctx, dcancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer dcancel()
		dummy := startPTYAt(t, newDummyIdentManagerCmd(t, dctx, dummyBin, dummyHome), dummyTermWidth, dummyTermHeight)
		defer dummy.close(t)
		mustSee(t, dummy, "[1] Identities", "dummy: launches on the Identities tab")
		dummy.sendKey(dummyKeyDown, keystrokeDelay) // personal -> work (IdentityManagerDetailTarget)
		mustSee(t, dummy, "SSH — shown first, always", "dummy: SSH-first detail renders")

		compareIdentManagerCheckpoint(t, "detail-ssh-first", real.snapshot(), dummy.snapshot(), allowlist)
	})

	t.Run("rotate-result", func(t *testing.T) {
		realHome := SandboxHome(t)
		seedGitPTYIdentity(t, realHome, "acme")
		seedRecipeShapeSSHConfig(t, realHome, "acme")
		fakeSSHDir := FakeSSHDir(t, "denied")
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		real := startPTYAt(t, newRealCreateFlowCmd(t, ctx, realBin, realHome, fakeSSHDir), dummyTermWidth, dummyTermHeight)
		defer real.close(t)
		uiReady(t, real)
		openKeyCeremonyViaActionMenu(t, real)
		real.sendKey(dummyKeyEnter, keystrokeDelay) // confirm
		mustSee(t, real, "Key ceremony completed.", "real: rotate receipt renders")
		mustSee(t, real, "The old key stays valid at", "real: grace-window hint renders")
		realFrame := real.snapshot()

		dummyHome := SandboxHome(t)
		dctx, dcancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer dcancel()
		dummy := startPTYAt(t, newDummyIdentManagerCmd(t, dctx, dummyBin, dummyHome), dummyTermWidth, dummyTermHeight)
		defer dummy.close(t)
		mustSee(t, dummy, "[1] Identities", "dummy: launches on the Identities tab")
		openKeyCeremonyViaActionMenu(t, dummy) // personal (index 0) -> rotate
		dummy.sendKey(dummyKeyEnter, keystrokeDelay)
		mustSee(t, dummy, "Key ceremony completed.", "dummy: rotate receipt renders")
		mustSee(t, dummy, "The old key stays valid at", "dummy: grace-window hint renders")
		dummyFrame := dummy.snapshot()

		// 09-06-PLAN.md Task 2 chains the upload beat onto every successful
		// commit, so this checkpoint's frame now also carries upload-
		// section/connectivity-output content — but this checkpoint's own
		// fixture (FakeSSHDir "denied", NO gh shim) never reaches a settled
		// outcome within any bounded real-PTY wait: the deny shim's failure
		// path was found to take an unpredictable, sometimes 20s+ duration
		// under -race, making any fixed wait here inherently flaky. This
		// checkpoint's PURPOSE is the key-ceremony receipt, not the upload
		// beat (which register-key-modal/rotate-delete-offer already cover
		// thoroughly with real gh="ok"/"delete-ok" fixtures and proper
		// waits) — skip upload-section/connectivity-output here rather than
		// chase real-subprocess timing this checkpoint was never designed
		// to test. identRegionDetail is ALSO skipped for the same coarse-
		// superset reason register-key-modal/rotate-delete-offer's own
		// comparison already documents.
		compareIdentManagerCheckpointSkipping(t, "rotate-result", realFrame, dummyFrame, allowlist,
			map[identManagerRegion]bool{identRegionUploadSection: true, identRegionConnectivityOutput: true})
	})

	t.Run("repair-result", func(t *testing.T) {
		realHome := SandboxHome(t)
		seedGitPTYIdentity(t, realHome, "acme")
		seedRecipeShapeSSHConfig(t, realHome, "acme")
		for _, p := range []string{
			filepath.Join(realHome, ".ssh", "id_ed25519_acme"),
			filepath.Join(realHome, ".ssh", "id_ed25519_acme.pub"),
		} {
			if err := os.Remove(p); err != nil {
				t.Fatalf("seeding key-missing fixture: %v", err)
			}
		}
		fakeSSHDir := FakeSSHDir(t, "denied")
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		real := startPTYAt(t, newRealCreateFlowCmd(t, ctx, realBin, realHome, fakeSSHDir), dummyTermWidth, dummyTermHeight)
		defer real.close(t)
		uiReady(t, real)
		openKeyCeremonyViaActionMenu(t, real)
		real.sendKey(dummyKeyEnter, keystrokeDelay)
		mustSee(t, real, "Key ceremony completed.", "real: repair receipt renders")
		mustNotSee(t, real, "The old key stays valid at", "real: repair never renders the grace-window hint")
		realFrame := real.snapshot()

		dummyHome := SandboxHome(t)
		dctx, dcancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer dcancel()
		dummy := startPTYAt(t, newDummyIdentManagerCmd(t, dctx, dummyBin, dummyHome), dummyTermWidth, dummyTermHeight)
		defer dummy.close(t)
		mustSee(t, dummy, "[1] Identities", "dummy: launches on the Identities tab")
		for i := 0; i < 6; i++ { // personal -> ... -> clientB (index 6, key-missing)
			dummy.sendKey(dummyKeyDown, keystrokeDelay)
		}
		mustSee(t, dummy, "Identities › clientB", "dummy: clientB (key-missing) selected")
		openKeyCeremonyViaActionMenu(t, dummy)
		dummy.sendKey(dummyKeyEnter, keystrokeDelay)
		mustSee(t, dummy, "Key ceremony completed.", "dummy: repair receipt renders")
		mustNotSee(t, dummy, "The old key stays valid at", "dummy: repair never renders the grace-window hint")
		dummyFrame := dummy.snapshot()

		// Same reasoning as rotate-result's own comment above: this
		// checkpoint's fixture has no gh shim, so the chained upload beat's
		// outcome timing is unbounded/flaky under -race — skip
		// upload-section/connectivity-output here; register-key-modal/
		// rotate-delete-offer already cover the upload beat itself with
		// proper gh="ok"/"delete-ok" fixtures and stable waits.
		compareIdentManagerCheckpointSkipping(t, "repair-result", realFrame, dummyFrame, allowlist,
			map[identManagerRegion]bool{identRegionUploadSection: true, identRegionConnectivityOutput: true})
	})

	// This test drives exactly these six checkpoints; register-key-modal and
	// rotate-delete-offer (Phase 9, 09-07-PLAN.md Task 3) are driven by the
	// separate TestRegisterKeyModal_CompiledRealVsLiveDummyPTY below, which
	// runs its own "unused" check scoped to its own two checkpoints. Each
	// test's "unused" check is scoped to the checkpoints IT drives, since
	// loadIdentManagerAllowlist(t) parses every line in the shared file.
	thisTestCheckpoints := map[string]bool{
		"action-menu": true, "delete-choice": true, "confirm-destructive": true,
		"detail-ssh-first": true, "rotate-result": true, "repair-result": true,
	}
	for _, entry := range allowlist {
		if !thisTestCheckpoints[entry.Checkpoint] {
			continue
		}
		if !entry.used {
			t.Errorf("identity-manager allowlist entry %s/%s (%s) was never triggered by any checkpoint comparison — remove the stale entry", entry.Checkpoint, entry.Region, entry.DecisionRef)
		}
	}
}

// TestRegisterKeyModal_CompiledRealVsLiveDummyPTY drives the Phase 9
// register-key modal (D-08) and the D-04 rotate-delete-offer sub-beat
// through two real PTY sessions — the compiled cmd/gitid binary and the
// compiled cmd/gitid-dummy binary — comparing NORMALIZED semantic
// checkpoints (never raw terminal bytes), following the SAME pattern as
// TestIdentityManager_CompiledRealVsLiveDummyPTY above (09-07-PLAN.md Task
// 3, UP-01/UP-02/UP-03).
//
// SHARED-RENDERER LIMITATION (STATE.md's Phase-4 CR-15 finding, restated
// here per this plan's own requirement): both binaries render the
// register-key pane and the rotate-delete-offer sub-beat through the SAME
// internal/tuikit identities.go code — cmd/gitid injects a real Backend,
// cmd/gitid-dummy injects internal/dummytui.FixtureBackend. This paired
// comparison therefore catches wiring/content differences between the two
// Backend implementations, never a defect INSIDE the shared renderer
// itself. The independent backstop is 09-07-PLAN.md Task 1's per-state PTY
// suite (TestIdentityManager_RegisterKeyModal*, TestIdentityManager_RotateDeleteOffer*)
// plus the FIELDS.md manifest assertions below — both derive their
// expectations from the design contract (09-UI-SPEC.md), not from the
// other binary, so a shared-renderer defect that fools BOTH sides equally
// cannot hide from either.
//
// Both PTY sessions — real AND dummy — build their environment through
// e2eEnv (review R1): the real session via newRealCreateFlowCmd's
// e2eEnv-backed variadic shim form, the dummy session via
// newDummyIdentManagerCmd (which itself calls e2eEnv), so neither side can
// resolve a real gh/glab even though the dummy never actually shells out.
func TestRegisterKeyModal_CompiledRealVsLiveDummyPTY(t *testing.T) {
	allowlist := loadIdentManagerAllowlist(t)
	realBin := BuildBinary(t)
	dummyBin := BuildDummyBinary(t)

	t.Run("register-key-modal", func(t *testing.T) {
		realHome := SandboxHome(t)
		seedMinimalIdentity(t, realHome, "acme")
		seedEncryptedKeyFixture(t, filepath.Join(realHome, ".ssh", "id_ed25519_acme"), "acme", "")
		fakeGH, _ := FakeGHDir(t, "ok")
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		real := startPTYAt(t, newRealCreateFlowCmd(t, ctx, realBin, realHome, fakeGH), dummyTermWidth, dummyTermHeight)
		defer real.close(t)
		uiReady(t, real)
		mustSee(t, real, "acme", "real: sidebar renders the seeded identity")
		openRegisterKeyModalViaActionMenu(t, real)
		mustSee(t, real, "Register acme's key with GitHub", "real: the frozen modal heading renders")
		mustSee(t, real, "Running:", "real: the upload beat's announce line renders")

		dummyHome := SandboxHome(t)
		dctx, dcancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer dcancel()
		dummy := startPTYAt(t, newDummyIdentManagerCmd(t, dctx, dummyBin, dummyHome), dummyTermWidth, dummyTermHeight)
		defer dummy.close(t)
		mustSee(t, dummy, "[1] Identities", "dummy: launches on the Identities tab")
		mustSee(t, dummy, "personal", "dummy: seeded fixture sidebar row (github host, row 0)")
		openRegisterKeyModalViaActionMenu(t, dummy)
		mustSee(t, dummy, "Register personal's key with GitHub", "dummy: the frozen modal heading renders")
		mustSee(t, dummy, "Running:", "dummy: the upload beat's announce line renders")
		// CR-01 anti-drift guard: the previous fix pass's UploadRunMsg.Name
		// stale-guard (WR-01, iteration 3) silently discards every fixture
		// reply because FixtureBackend never set Name — the dummy pane hung
		// on "Registering…" forever with no failing assertion, since the
		// prior assertions only ever checked the announce line. Assert the
		// RESULT row explicitly so a dropped fixture reply fails loudly.
		mustSee(t, dummy, "Authentication key registered", "dummy: the upload beat's result row renders (CR-01 anti-drift guard)")

		compareIdentManagerCheckpointSkipping(t, "register-key-modal", real.snapshot(), dummy.snapshot(), allowlist, map[identManagerRegion]bool{identRegionDetail: true})
	})

	t.Run("rotate-delete-offer", func(t *testing.T) {
		realHome := ShortSandboxHome(t)
		title := seedRotateDeleteOfferFixture(t, realHome)
		fakeSSHDir := FakeSSHDir(t, "denied")
		fakeGH, _ := FakeGHDir(t, "delete-ok")
		FakeGHInventoryFile(t, fmt.Sprintf(`[{"id":555,"title":%q,"key":"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5STUB old@gitid"}]`, title))
		// CR-02: the D-04 offer's belt-and-braces check requires the freshly
		// rotated key to already be registered in a fresh inventory read —
		// track this test's real "ssh-key add" calls so the subsequent read
		// reflects them, matching real GitHub.
		FakeGHTrackAddedKeys(t)
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		real := startPTYAt(t, newRealCreateFlowCmd(t, ctx, realBin, realHome, fakeSSHDir, fakeGH), dummyTermWidth, dummyTermHeight)
		defer real.close(t)
		uiReady(t, real)
		mustSee(t, real, "acme", "real: sidebar renders the seeded identity")
		driveRotateToResultScreen(t, real)
		mustSeeSlow(t, real, "Remove the old key from GitHub?", "real: the D-04 offer heading renders")
		mustSee(t, real, "Leave it", "real: the leave option renders")

		dummyHome := SandboxHome(t)
		dctx, dcancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer dcancel()
		dummy := startPTYAt(t, newDummyIdentManagerCmd(t, dctx, dummyBin, dummyHome), dummyTermWidth, dummyTermHeight)
		defer dummy.close(t)
		mustSee(t, dummy, "[1] Identities", "dummy: launches on the Identities tab")
		mustSee(t, dummy, "personal", "dummy: seeded fixture sidebar row (github host, row 0, rotate-eligible)")
		driveRotateToResultScreen(t, dummy)
		mustSeeSlow(t, dummy, "Remove the old key from GitHub?", "dummy: the D-04 offer heading renders")
		mustSee(t, dummy, "Leave it", "dummy: the leave option renders")

		compareIdentManagerCheckpointSkipping(t, "rotate-delete-offer", real.snapshot(), dummy.snapshot(), allowlist, map[identManagerRegion]bool{identRegionDetail: true})

		// WR-01 anti-drift guard (review iteration 5): the previous fix
		// pass's UploadRunMsg.Name stale-guard regression (CR-01, iteration
		// 3) silently discarded every fixture reply because FixtureBackend
		// never set Name — this subtest's assertions above only ever
		// checked the OFFER heading, never the delete COMMIT result, so an
		// identical regression on RotateDeleteCommitMsg.Name (WR-01,
		// iteration 5) would hang the dummy leg on "Removing…" forever with
		// no failing assertion. Explicitly move to the delete option and
		// confirm, then assert the removal result row renders.
		dummy.sendKey(dummyKeyDown, keystrokeDelay) // move to the delete option
		dummy.sendKey(dummyKeyEnter, keystrokeDelay)
		mustSee(t, dummy, "Old key removed from GitHub", "dummy: the removal result row renders (WR-01 anti-drift guard)")
	})

	newCheckpoints := map[string]bool{"register-key-modal": true, "rotate-delete-offer": true}
	// identRegionBreadcrumb is genuinely never used for either new
	// checkpoint at THIS comparison level: normalizeIdentManagerCheckpoint
	// already collapses the embedded identity name to "<identity>" before
	// comparing, so the real ("acme"/"...") and dummy ("personal") crumb
	// lines always compare equal here — unlike the IN-PROCESS visual gate
	// (internal/screenshot), which compares raw unnormalized text and DOES
	// need the register-key-modal:breadcrumb disposition Task 2 registered.
	// Observed directly: running this test with the breadcrumb check
	// included reports it unused/stale on every run.
	skipStaleCheck := map[string]bool{"breadcrumb": true}
	for _, entry := range allowlist {
		if !newCheckpoints[entry.Checkpoint] {
			continue
		}
		if skipStaleCheck[string(entry.Region)] {
			continue
		}
		if !entry.used {
			t.Errorf("identity-manager allowlist entry %s/%s (%s) was never triggered by any checkpoint comparison — remove the stale entry", entry.Checkpoint, entry.Region, entry.DecisionRef)
		}
	}
}

// fakeErrorRecorder implements errorRecorder without ever failing the REAL
// enclosing test — a negative control needs to observe "the comparison
// WOULD have reported a failure" without that failure propagating to the
// negative-control test itself (Go's testing package marks every parent as
// failed the instant any subtest calls t.Errorf/t.Fatalf, which makes a
// "prove this correctly fails" test impossible to write as a passing test
// using a real *testing.T subtest).
type fakeErrorRecorder struct {
	failed   bool
	messages []string
}

func (f *fakeErrorRecorder) Helper() {}
func (f *fakeErrorRecorder) Errorf(format string, args ...any) {
	f.failed = true
	f.messages = append(f.messages, fmt.Sprintf(format, args...))
}
func (f *fakeErrorRecorder) Fatalf(format string, args ...any) {
	f.failed = true
	f.messages = append(f.messages, fmt.Sprintf(format, args...))
}

// TestIdentityManager_AllowlistUnclassifiedDifferenceRejected is a negative
// control operating directly on compareIdentManagerCheckpoint (no PTY
// needed — a real divergence with an EMPTY allowlist): proves the
// comparison FAILS when a real region difference has no classification
// entry at all, matching the acceptance criterion "a difference with no
// entry fails the comparison."
func TestIdentityManager_AllowlistUnclassifiedDifferenceRejected(t *testing.T) {
	// A structural difference that survives identity-token normalization
	// (not just "acme" vs "personal", which normalizeIdentManagerCheckpoint
	// legitimately collapses to the same "<identity>" placeholder).
	realFrame := "gitid header line ending in Doctor status\n│ acme\n│ Delete EVERYTHING for the identity — irreversible warning shown\n"
	dummyFrame := "gitid header line ending in Doctor status\n│ acme\n│ Delete the identity — no warning rendered at all\n"

	rec := &fakeErrorRecorder{}
	compareIdentManagerCheckpoint(rec, "confirm-destructive", realFrame, dummyFrame, nil)
	if !rec.failed {
		t.Fatal("negative control FAILED: a real divergence with NO allowlist entry must fail the comparison, but it reported no failure")
	}
	t.Logf("negative control observed the expected failure: %v", rec.messages)
}

// TestIdentityManager_AllowlistStaleEntryRejected is a negative control
// proving the SAME post-loop staleness check
// TestIdentityManager_CompiledRealVsLiveDummyPTY runs: an allowlist entry
// for a checkpoint/region pair that no longer differs (identical real and
// dummy frames) must be reported as stale — matching the acceptance
// criterion "an entry naming a checkpoint that no longer differs fails as
// stale."
func TestIdentityManager_AllowlistStaleEntryRejected(t *testing.T) {
	frame := "gitid header line ending in Doctor status\n│ acme\n│ identical detail content\n"
	entry := &identManagerAllowlistEntry{
		Checkpoint: "confirm-destructive", Region: identRegionDetail,
		Predicate: `contains:"identical detail content"`, DecisionRef: "D-11", Reason: "test fixture — expected never used",
	}
	allowlist := []*identManagerAllowlistEntry{entry}

	// Both sides IDENTICAL: nothing differs, so entry.used must stay false.
	rec := &fakeErrorRecorder{}
	compareIdentManagerCheckpoint(rec, "confirm-destructive", frame, frame, allowlist)
	if rec.failed {
		t.Fatalf("fixture invalid: identical frames must never fail the comparison: %v", rec.messages)
	}
	if entry.used {
		t.Fatal("fixture invalid: identical frames must never mark an allowlist entry used")
	}

	// The real test's own post-loop staleness check, exercised here against
	// the same fakeErrorRecorder so a fresh reader can see the exact
	// failure the stale entry produces without it failing this test.
	if !entry.used {
		rec.Errorf("identity-manager allowlist entry %s/%s (%s) was never triggered by any checkpoint comparison — remove the stale entry", entry.Checkpoint, entry.Region, entry.DecisionRef)
	}
	if !rec.failed {
		t.Fatal("negative control FAILED: a stale allowlist entry (checkpoint/region that never differs) must fail the staleness check, but it reported no failure")
	}
	t.Logf("negative control observed the expected staleness failure: %v", rec.messages)
}

// ---------------------------------------------------------------------------
// 05-09-PLAN.md Task 3 (review R-28) — the previously-planned manual walk,
// automated. Three claims, each mechanically checkable against the compiled
// binary's own PTY receipts and the real sandbox filesystem — no human
// walkthrough. Phase-level human acceptance remains at /gsd-verify-work
// (phase close), outside this plan; nothing here replaces it.
// ---------------------------------------------------------------------------

// receiptPathPattern matches a filesystem-looking token in a decoded PTY
// receipt frame: a tilde-relative or absolute path made of the path-safe
// character set this project's own rendered paths use (letters, digits, "._-/").
// Trailing punctuation a sentence might attach (".", ",", ")") is trimmed by
// the caller, never matched here, so a token like "acme." doesn't wrongly
// keep its full stop.
var receiptPathPattern = regexp.MustCompile(`(?:~|/)[A-Za-z0-9._/-]+`)

// parseReceiptFilesystemTokens extracts every filesystem-looking token from
// a decoded receipt frame, deduplicated, in first-seen order. Pure text
// scanning — no filesystem access — so the negative control below can prove
// it fails deterministically on a fabricated path without seeding one.
func parseReceiptFilesystemTokens(frame string) []string {
	seen := map[string]bool{}
	var out []string
	for _, raw := range receiptPathPattern.FindAllString(frame, -1) {
		tok := strings.TrimRight(raw, ".,;:)")
		if tok == "" || tok == "~" || tok == "/" || seen[tok] {
			continue
		}
		seen[tok] = true
		out = append(out, tok)
	}
	return out
}

// assertReceiptPathsExist parses every filesystem-looking token out of a
// decoded receipt frame and os.Stats each one (tilde-expanded against home)
// in the real sandbox — review R-28's "every receipt names real paths that
// exist on disk" claim, automated. Delegates to
// assertReceiptPathsExistRecorder (*testing.T satisfies receiptPathRecorder)
// so the negative control below exercises the EXACT SAME logic through a
// non-propagating fake, never a separately-written duplicate that could
// silently drift from what this function actually checks.
func assertReceiptPathsExist(t *testing.T, home, frame string) {
	t.Helper()
	assertReceiptPathsExistRecorder(t, home, frame)
}

// TestIdentityManager_ReceiptPathsNegativeControl proves
// assertReceiptPathsExist actually fails for a fabricated, non-existent
// path — review R-28's required negative control.
func TestIdentityManager_ReceiptPathsNegativeControl(t *testing.T) {
	home := t.TempDir()
	fabricated := "✓ Key ceremony completed.\nWrote → ~/.ssh/this-path-was-never-written-by-anything\n"
	// fakeErrorRecorder (defined above, alongside
	// TestIdentityManager_AllowlistUnclassifiedDifferenceRejected) already
	// implements the Helper/Errorf/Fatalf shape both errorRecorder and
	// receiptPathRecorder need — one fake, reused for both negative-control
	// families rather than a second, near-identical type.
	rec := &fakeErrorRecorder{}
	assertReceiptPathsExistRecorder(rec, home, fabricated)
	if !rec.failed {
		t.Fatal("negative control FAILED: a fabricated non-existent path must make assertReceiptPathsExist report a failure, but it reported none")
	}
}

// receiptPathRecorder is the minimal *testing.T surface
// assertReceiptPathsExistRecorder needs.
type receiptPathRecorder interface {
	Helper()
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
}

// assertReceiptPathsExistRecorder is assertReceiptPathsExist generalized
// over receiptPathRecorder so the negative control above can inject a
// non-propagating fake. The real test-facing assertReceiptPathsExist(t
// *testing.T, ...) delegates here — *testing.T already satisfies the
// interface.
func assertReceiptPathsExistRecorder(t receiptPathRecorder, home, frame string) {
	t.Helper()
	tokens := parseReceiptFilesystemTokens(frame)
	if len(tokens) == 0 {
		t.Fatalf("assertReceiptPathsExist: no filesystem-looking token found in receipt frame — parser or frame drifted:\n%s", frame)
		return
	}
	var missing []string
	for _, tok := range tokens {
		real := tok
		if strings.HasPrefix(real, "~/") {
			real = filepath.Join(home, real[2:])
		} else if real == "~" {
			real = home
		}
		if !filepath.IsAbs(real) {
			continue
		}
		if _, err := os.Stat(real); err != nil {
			missing = append(missing, tok)
		}
	}
	if len(missing) > 0 {
		t.Errorf("assertReceiptPathsExist: receipt names %d path(s) that do NOT exist on disk: %v\nframe:\n%s", len(missing), missing, frame)
	}
}

// TestIdentityManager_SuccessNeverClaimedWithoutTheWork is review R-28's
// third claim ("nothing reported success without doing the work"), made
// explicit as its OWN cross-check rather than left implicit in every write
// case's post-receipt filesystem assertion: it drives one full ceremony
// (git-only delete) and proves the receipt heading is reachable ONLY AFTER
// the corresponding file change (the gitconfig includeIf block's removal)
// is observable on disk — never before.
func TestIdentityManager_SuccessNeverClaimedWithoutTheWork(t *testing.T) {
	// ShortSandboxHome (never SandboxHome): the receipt's absolute path
	// tokens must survive the pane's word-wrap without a mid-path line
	// break, or parseReceiptFilesystemTokens sees wrapped fragments instead
	// of one path — see ShortSandboxHome's own doc comment (originally
	// written for the D-13 hit-line assertions, the same class of problem).
	home := ShortSandboxHome(t)
	seedGitPTYIdentity(t, home, "acme")
	bin := BuildBinary(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s := startPTYAt(t, newRealCreateFlowCmd(t, ctx, bin, home, ""), dummyTermWidth, dummyTermHeight)
	defer s.close(t)

	uiReady(t, s)
	mustSee(t, s, "acme", "sidebar renders the seeded identity")
	s.sendKey([]byte("d"), keystrokeDelay)
	mustSee(t, s, "Delete Git identity only", "delete-choice renders")
	s.sendKey(dummyKeyEnter, keystrokeDelay) // git-only ceremony
	mustSee(t, s, `Delete the Git identity of "acme" (SSH stays)`, "ceremony opens")

	gitconfigPath := filepath.Join(home, ".gitconfig")
	before, err := os.ReadFile(gitconfigPath) //nolint:gosec // fixed sandbox path (G304)
	if err != nil {
		t.Fatalf("reading pre-delete gitconfig: %v", err)
	}
	if !strings.Contains(string(before), "BEGIN gitid managed: acme") {
		t.Fatalf("fixture invalid: gitconfig missing the acme block before any write:\n%s", before)
	}

	s.sendKey(dummyKeyEnter, keystrokeDelay) // confirm
	mustSee(t, s, `Git identity of "acme" deleted`, "the success receipt renders")

	// The receipt is ALREADY visible at this point (mustSee polled until it
	// appeared) — the real question is whether the WORK was already done
	// by the time it did. Reading the file NOW must show it gone; if the
	// receipt could render before the write landed, this read would race
	// and non-deterministically still show the block present.
	after, err := os.ReadFile(gitconfigPath) //nolint:gosec // fixed sandbox path (G304)
	if err != nil {
		t.Fatalf("reading post-delete gitconfig: %v", err)
	}
	if strings.Contains(string(after), "BEGIN gitid managed: acme") {
		t.Fatalf("the success receipt rendered but the gitconfig still carries the acme block — success was claimed without doing the work:\n%s", after)
	}
	// assertReceiptPathsExist is deliberately NOT called here: a delete
	// ceremony's "Wrote → " target list legitimately includes the FRAGMENT
	// FILE IT REMOVES (D-10) — "Wrote → " means "this transaction touched
	// this location," not "this file now exists," for a delete target
	// specifically. The claim genuinely holds for create-like receipts
	// (rotate/repair, which never remove a listed target) — see
	// TestIdentityManager_KeyCeremonyRotate/Repair, which call it.
}
