//go:build e2e

package e2e

// global_git_cli_e2e_test.go drives the compiled gitid binary's git verbs
// headlessly (no PTY, plan 07-05 Task 3): frozen JSON envelopes, options
// apply + refusal + idempotence + dry run, the fallback author set/clear
// cycle, and a NON-VACUOUS below-gate advisory case against the fake git shim
// (plan 07-04's FakeGitShimDir). Assertions are made on the FILES as well as
// the envelopes — the suite never trusts the tool's own report of what it did.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/castocolina/gitid/internal/globalgit"
)

type gitOptionsDoc struct {
	Schema  string            `json:"schema"`
	Options []gitOptionRecord `json:"options"`
}

type gitOptionRecord struct {
	Key              string `json:"key"`
	Token            string `json:"token"`
	CurrentValue     string `json:"current_value"`
	Provenance       string `json:"provenance"`
	RecommendedValue string `json:"recommended_value"`
	State            string `json:"state"`
	ProbeError       string `json:"probe_error"`
}

type gitApplyDoc struct {
	Schema     string   `json:"schema"`
	DryRun     bool     `json:"dry_run"`
	Applied    []string `json:"applied"`
	Declined   []string `json:"declined"`
	TargetPath string   `json:"target_path"`
	Backups    []string `json:"backups"`
	Restored   []string `json:"restored"`
	Advisories []string `json:"advisories"`
	Error      string   `json:"error"`
	ExitCode   int      `json:"exit_code"`
}

type gitFallbackDoc struct {
	Schema      string `json:"schema"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	NameStatus  string `json:"name_status"`
	EmailStatus string `json:"email_status"`
}

type gitFallbackSetDoc struct {
	Schema       string   `json:"schema"`
	DryRun       bool     `json:"dry_run"`
	SetName      bool     `json:"set_name"`
	SetEmail     bool     `json:"set_email"`
	ClearedName  bool     `json:"cleared_name"`
	ClearedEmail bool     `json:"cleared_email"`
	TargetPath   string   `json:"target_path"`
	Backups      []string `json:"backups"`
	Restored     []string `json:"restored"`
	Advisories   []string `json:"advisories"`
	Error        string   `json:"error"`
	ExitCode     int      `json:"exit_code"`
}

// runGitCLI invokes the compiled binary headlessly against a sandbox HOME.
// fakeGit, when non-empty, is a FakeGitShimDir directory prepended to PATH so
// the child's git probes resolve the shim (the below-gate version override).
func runGitCLI(t *testing.T, ctx context.Context, bin, home, fakeGit string, args ...string) (stdout []byte, code int) {
	t.Helper()
	var out, errb bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, args...) //nolint:gosec // bin from BuildBinary; fixed test literals
	cmd.Stdout = &out
	cmd.Stderr = &errb
	env, _ := e2eEnv(t, home, fakeGit)
	cmd.Env = env
	err := cmd.Run()
	if err == nil {
		return out.Bytes(), 0
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return out.Bytes(), ee.ExitCode()
	}
	t.Fatalf("gitid %s: %v\nstdout:\n%s\nstderr:\n%s", strings.Join(args, " "), err, out.String(), errb.String())
	return nil, -1
}

// unmarshalSingleJSON parses raw as EXACTLY ONE well-formed JSON document —
// a second value (or trailing garbage) is a failure. Every command in this
// suite routes its output through here.
func unmarshalSingleJSON(t *testing.T, raw []byte, v interface{}) {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(raw))
	if err := dec.Decode(v); err != nil {
		t.Fatalf("first JSON value: %v\n%s", err, raw)
	}
	var extra interface{}
	if err := dec.Decode(&extra); err != io.EOF {
		t.Fatalf("expected exactly one JSON document, got trailing content: %v\n%s", err, raw)
	}
}

func TestGlobalGitCLI_ListApplyIdempotent(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)
	// A pre-existing main config so the floor-include write has a real backup
	// target, while leaving every baseline row unset (needs-action).
	main, baseline := seedGlobalGitHome(t, home, "[user]\n\tname = Pat\n\temail = pat@example.com\n", "")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	listOut, listCode := runGitCLI(t, ctx, bin, home, "", "git", "options", "list", "--json")
	if listCode != 0 {
		t.Fatalf("options list --json exit = %d\n%s", listCode, listOut)
	}
	var list gitOptionsDoc
	unmarshalSingleJSON(t, listOut, &list)
	if list.Schema != "gitid.git.options/v1" {
		t.Errorf("list schema = %q", list.Schema)
	}
	if len(list.Options) != len(globalgit.Policy) {
		t.Fatalf("options = %d, want %d (one row per policy row)", len(list.Options), len(globalgit.Policy))
	}
	for i, rec := range list.Options {
		if rec.Key != globalgit.Policy[i].Key {
			t.Errorf("options[%d].key = %q, want %q", i, rec.Key, globalgit.Policy[i].Key)
		}
		if rec.Token != globalgit.Policy[i].Token {
			t.Errorf("options[%d].token = %q, want %q", i, rec.Token, globalgit.Policy[i].Token)
		}
	}

	apply1, code1 := runGitCLI(t, ctx, bin, home, "", "git", "options", "apply", "init.defaultBranch", "--yes", "--json")
	if code1 != 0 {
		t.Fatalf("first apply exit = %d\n%s", code1, apply1)
	}
	var env1 gitApplyDoc
	unmarshalSingleJSON(t, apply1, &env1)
	if env1.Schema != "gitid.git.apply/v1" || env1.ExitCode != 0 {
		t.Errorf("apply envelope schema=%q exit_code=%d", env1.Schema, env1.ExitCode)
	}
	if len(env1.Applied) != 1 || env1.Applied[0] != "init.defaultBranch" {
		t.Errorf("applied = %v, want [init.defaultBranch]", env1.Applied)
	}
	after1, err := os.ReadFile(baseline) //nolint:gosec // sandbox path
	if err != nil {
		t.Fatalf("read baseline: %v", err)
	}
	if !bytes.Contains(after1, []byte("defaultBranch = main")) {
		t.Errorf("baseline missing defaultBranch = main:\n%s", after1)
	}

	listOut2, listCode2 := runGitCLI(t, ctx, bin, home, "", "git", "options", "list", "--json")
	if listCode2 != 0 {
		t.Fatalf("second list exit = %d\n%s", listCode2, listOut2)
	}
	var list2 gitOptionsDoc
	unmarshalSingleJSON(t, listOut2, &list2)
	if len(list2.Options) != len(globalgit.Policy) {
		t.Fatalf("second list options = %d, want %d", len(list2.Options), len(globalgit.Policy))
	}
	var appliedRec *gitOptionRecord
	for i := range list2.Options {
		if list2.Options[i].Key == "init.defaultBranch" {
			appliedRec = &list2.Options[i]
		}
	}
	if appliedRec == nil {
		t.Fatal("second list missing init.defaultBranch row")
	}
	if appliedRec.State != "already-set" {
		t.Errorf("applied row state = %q, want already-set", appliedRec.State)
	}
	if !strings.Contains(appliedRec.Provenance, "set by gitid") {
		t.Errorf("applied row provenance = %q, want it to name gitid", appliedRec.Provenance)
	}

	// Second apply: the row is now already-set, so the frozen CLI contract
	// REFUSES it (07-05 Task 1: a row gitid provably cannot change is refused
	// naming its state) — the file must stay byte-identical.
	apply2, code2 := runGitCLI(t, ctx, bin, home, "", "git", "options", "apply", "init.defaultBranch", "--yes", "--json")
	if code2 != 1 {
		t.Fatalf("second apply exit = %d, want 1 (already-set refusal)\n%s", code2, apply2)
	}
	var env2 gitApplyDoc
	unmarshalSingleJSON(t, apply2, &env2)
	if env2.ExitCode != code2 {
		t.Errorf("second apply envelope exit_code %d != process status %d", env2.ExitCode, code2)
	}
	if !strings.Contains(env2.Error, "already-set") {
		t.Errorf("second apply error = %q, want it to name already-set", env2.Error)
	}
	after2, err := os.ReadFile(baseline) //nolint:gosec // sandbox path
	if err != nil {
		t.Fatalf("reread baseline: %v", err)
	}
	if !bytes.Equal(after1, after2) {
		t.Errorf("second apply mutated config:\nfirst:\n%s\nsecond:\n%s", after1, after2)
	}

	// The floor-include write also touched the main config on apply 1; the
	// refusal on apply 2 must leave that untouched too.
	mainAfter, err := os.ReadFile(main) //nolint:gosec // sandbox path
	if err != nil {
		t.Fatalf("read main config: %v", err)
	}
	if !bytes.Contains(mainAfter, []byte("[include]")) {
		t.Errorf("main config missing the floor include after apply:\n%s", mainAfter)
	}
}

func TestGlobalGitCLI_ConflictingValueRefused(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)
	seedGlobalGitHome(t, home, "[init]\n\tdefaultBranch = trunk\n", "")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	out, code := runGitCLI(t, ctx, bin, home, "", "git", "options", "apply", "init.defaultBranch", "--yes", "--json")
	if code != 1 {
		t.Fatalf("conflicting-value apply exit = %d, want 1\n%s", code, out)
	}
	var env gitApplyDoc
	unmarshalSingleJSON(t, out, &env)
	if env.ExitCode != code {
		t.Errorf("envelope exit_code %d != process status %d", env.ExitCode, code)
	}
	if env.Error == "" {
		t.Error("refusal envelope must populate error")
	}
	if !strings.Contains(env.Error, "differs") {
		t.Errorf("refusal error = %q, want it to name the differs state", env.Error)
	}
	baseline := filepath.Join(home, ".gitconfig.d", "00-baseline")
	if _, err := os.Stat(baseline); err == nil {
		t.Errorf("refused apply must not create the baseline file")
	}
}

func TestGlobalGitCLI_DryRunNoWrite(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)
	main, baseline := seedGlobalGitHome(t, home, "[user]\n\tname = Pat\n", "")
	paths := []string{main, baseline}
	before := snapshotGitBytes(t, paths)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, code := runGitCLI(t, ctx, bin, home, "", "git", "options", "apply", "init.defaultBranch", "--dry-run", "--json")
	if code != 0 {
		t.Fatalf("dry-run exit = %d\n%s", code, out)
	}
	var env gitApplyDoc
	unmarshalSingleJSON(t, out, &env)
	if !env.DryRun {
		t.Error("dry_run marker must be true")
	}
	if env.ExitCode != code {
		t.Errorf("envelope exit_code %d != process status %d", env.ExitCode, code)
	}
	assertGitBytesUnchanged(t, before, snapshotGitBytes(t, paths))
	if countBackups(t, home) != 0 {
		t.Error("dry-run must not take a backup")
	}
}

func TestGlobalGitCLI_FallbackShowSetClear(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)
	seedGlobalGitHome(t, home, "", "")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	show1, code1 := runGitCLI(t, ctx, bin, home, "", "git", "fallback", "show", "--json")
	if code1 != 0 {
		t.Fatalf("fresh fallback show exit = %d\n%s", code1, show1)
	}
	var fb1 gitFallbackDoc
	unmarshalSingleJSON(t, show1, &fb1)
	if fb1.Schema != "gitid.git.fallback/v1" {
		t.Errorf("fallback schema = %q", fb1.Schema)
	}
	if fb1.NameStatus != "unset" || fb1.EmailStatus != "unset" {
		t.Errorf("fresh-home statuses = (%q, %q), want (unset, unset)", fb1.NameStatus, fb1.EmailStatus)
	}

	setOut, setCode := runGitCLI(t, ctx, bin, home, "", "git", "fallback", "set", "--name", "Pat Example", "--yes", "--json")
	if setCode != 0 {
		t.Fatalf("fallback set exit = %d\n%s", setCode, setOut)
	}
	var setDoc gitFallbackSetDoc
	unmarshalSingleJSON(t, setOut, &setDoc)
	if setDoc.Schema != "gitid.git.fallbackset/v1" || setDoc.ExitCode != 0 {
		t.Errorf("fallback-set envelope schema=%q exit_code=%d", setDoc.Schema, setDoc.ExitCode)
	}
	if !setDoc.SetName || setDoc.SetEmail || setDoc.ClearedName || setDoc.ClearedEmail {
		t.Errorf("set envelope flags = (set_name=%v set_email=%v cleared_name=%v cleared_email=%v), want only set_name", setDoc.SetName, setDoc.SetEmail, setDoc.ClearedName, setDoc.ClearedEmail)
	}

	show2, code2 := runGitCLI(t, ctx, bin, home, "", "git", "fallback", "show", "--json")
	if code2 != 0 {
		t.Fatalf("post-set show exit = %d\n%s", code2, show2)
	}
	var fb2 gitFallbackDoc
	unmarshalSingleJSON(t, show2, &fb2)
	if fb2.Name != "Pat Example" || fb2.NameStatus != "set" {
		t.Errorf("after set: name = %q status = %q, want Pat Example / set", fb2.Name, fb2.NameStatus)
	}
	if fb2.EmailStatus != "unset" {
		t.Errorf("after set-name-only: email status = %q, want unset", fb2.EmailStatus)
	}
	gitconfigPath := filepath.Join(home, ".gitconfig")
	if body := readFileE2E(t, gitconfigPath); !strings.Contains(body, globalGitAuthorMarker) {
		t.Fatalf("managed fallback block missing after set:\n%s", body)
	}

	clearOut, clearCode := runGitCLI(t, ctx, bin, home, "", "git", "fallback", "set", "--clear-name", "--yes", "--json")
	if clearCode != 0 {
		t.Fatalf("fallback clear exit = %d\n%s", clearCode, clearOut)
	}
	var clearDoc gitFallbackSetDoc
	unmarshalSingleJSON(t, clearOut, &clearDoc)
	if !clearDoc.ClearedName {
		t.Error("clear envelope must record cleared_name")
	}

	show3, code3 := runGitCLI(t, ctx, bin, home, "", "git", "fallback", "show", "--json")
	if code3 != 0 {
		t.Fatalf("post-clear show exit = %d\n%s", code3, show3)
	}
	var fb3 gitFallbackDoc
	unmarshalSingleJSON(t, show3, &fb3)
	if fb3.NameStatus != "unset" || fb3.EmailStatus != "unset" {
		t.Errorf("post-clear statuses = (%q, %q), want (unset, unset)", fb3.NameStatus, fb3.EmailStatus)
	}
	if body := readFileE2E(t, gitconfigPath); strings.Contains(body, globalGitAuthorMarker) {
		t.Fatalf("managed fallback block must be ABSENT after clear:\n%s", body)
	}
}

func TestGlobalGitCLI_BelowGateAdvisoryExitCodes(t *testing.T) {
	// 06-06's recorded trap: an advisory fixture that never produces the
	// condition it claims to test passes vacuously. Here the condition is the
	// hard-gate substitution, and it only exists when the shim's version
	// override REACHES the binary — so a dry run against the SAME sandbox is
	// run FIRST and asserted to carry the substitution advisory. If the shim
	// never reached the binary, the real machine's git (>= 2.35) would meet
	// the gate, no advisory would appear, and the dry run would fail.
	shim := FakeGitShimDir(t, "2.34.1", "")

	home := SandboxHome(t)
	bin := BuildBinary(t)
	_, baseline := seedGlobalGitHome(t, home, "", "")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	dry, dryCode := runGitCLI(t, ctx, bin, home, shim, "git", "options", "apply", "merge.conflictstyle", "--dry-run", "--json")
	if dryCode != 0 {
		t.Fatalf("below-gate dry run exit = %d\n%s", dryCode, dry)
	}
	var dryDoc gitApplyDoc
	unmarshalSingleJSON(t, dry, &dryDoc)
	if !containsSubstring(dryDoc.Advisories, "below the git version gate") {
		t.Fatalf("shim override did not produce the below-gate condition; the case would be vacuous\n%s", dry)
	}

	apply, code := runGitCLI(t, ctx, bin, home, shim, "git", "options", "apply", "merge.conflictstyle", "--yes", "--json")
	if code != 0 {
		t.Fatalf("below-gate apply default exit = %d, want 0\n%s", code, apply)
	}
	var env gitApplyDoc
	unmarshalSingleJSON(t, apply, &env)
	if env.ExitCode != code {
		t.Errorf("envelope exit_code %d != process status %d", env.ExitCode, code)
	}
	if !containsSubstring(env.Advisories, "below the git version gate") {
		t.Errorf("below-gate apply must emit the substitution advisory; advisories = %v", env.Advisories)
	}
	if !containsSubstring(env.Advisories, "diff3") {
		t.Errorf("substitution advisory must name the written fallback value; advisories = %v", env.Advisories)
	}
	content := readFileE2E(t, baseline)
	if !strings.Contains(content, "conflictstyle = diff3") || strings.Contains(content, "conflictstyle = zdiff3") {
		t.Fatalf("below-gate write did not use the fallback value diff3:\n%s", content)
	}
	t.Logf("CAPTURED-BELOW-GATE-APPLY-ENVELOPE\n%s\nEND-CAPTURED", apply)

	home2 := SandboxHome(t)
	seedGlobalGitHome(t, home2, "", "")
	out3, code3 := runGitCLI(t, ctx, bin, home2, shim, "git", "options", "apply", "merge.conflictstyle", "--yes", "--fail-on-advisory", "--json")
	if code3 != 3 {
		t.Fatalf("--fail-on-advisory exit = %d, want 3\n%s", code3, out3)
	}
	var env3 gitApplyDoc
	unmarshalSingleJSON(t, out3, &env3)
	if env3.ExitCode != 3 {
		t.Errorf("envelope exit_code = %d, want 3", env3.ExitCode)
	}
}

func containsSubstring(items []string, needle string) bool {
	for _, it := range items {
		if strings.Contains(it, needle) {
			return true
		}
	}
	return false
}
