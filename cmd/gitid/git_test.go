package main

// git_test.go covers plan 07-05's frozen CLI contract: the four command
// paths, shared-ceremony call sites, R-5 token-only argument validation,
// adaptive-depth table, and the explicit set-versus-clear fallback contract.

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/castocolina/gitid/internal/gitconfig"
	"github.com/castocolina/gitid/internal/globalgit"
	"github.com/castocolina/gitid/internal/tuikit"
)

func TestGitCmdTreeExactlyFourFrozenPaths(t *testing.T) {
	root := newRootCmd()
	want := []string{
		"gitid git options list",
		"gitid git options apply",
		"gitid git fallback show",
		"gitid git fallback set",
	}
	got := runnableGitPaths(root)
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("git command tree = %v, want exactly %v", got, want)
	}
}

func runnableGitPaths(root *cobra.Command) []string {
	git, _, err := root.Find([]string{"git"})
	if err != nil {
		return nil
	}
	var out []string
	var walk func(*cobra.Command)
	walk = func(c *cobra.Command) {
		if c.RunE != nil && c != git {
			out = append(out, c.CommandPath())
		}
		for _, child := range c.Commands() {
			walk(child)
		}
	}
	walk(git)
	return out
}

func TestGitNounGroupHelpListsRealVerbs(t *testing.T) {
	root := newRootCmd()
	cmd, _, err := root.Find([]string{"git"})
	if err != nil {
		t.Fatalf("Find(git): %v", err)
	}
	if cmd.RunE != nil {
		if rerr := cmd.RunE(cmd, nil); rerr != nil && strings.Contains(rerr.Error(), "arrives in") {
			t.Fatalf("git noun still returns the reserved-placeholder error: %v", rerr)
		}
	}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	if herr := cmd.Help(); herr != nil {
		t.Fatalf("git Help: %v", herr)
	}
	help := buf.String()
	for _, want := range []string{"options", "fallback"} {
		if !strings.Contains(help, want) {
			t.Errorf("git help missing sub-noun %q:\n%s", want, help)
		}
	}
}

func TestGitCmdFlagParsing(t *testing.T) {
	root := newRootCmd()
	cases := []struct {
		path  []string
		flags []string
	}{
		{[]string{"git", "options", "list"}, []string{"json"}},
		{[]string{"git", "options", "apply"}, []string{"dry-run", "yes", "fail-on-advisory", "json"}},
		{[]string{"git", "fallback", "show"}, []string{"json"}},
		{[]string{"git", "fallback", "set"}, []string{"name", "email", "clear-name", "clear-email", "dry-run", "yes", "json"}},
	}
	for _, tc := range cases {
		cmd, _, err := root.Find(tc.path)
		if err != nil {
			t.Fatalf("Find(%v): %v", tc.path, err)
		}
		for _, name := range tc.flags {
			if cmd.Flags().Lookup(name) == nil {
				t.Errorf("%s missing flag --%s", strings.Join(tc.path, " "), name)
			}
		}
	}
}

func TestGitWriteVerbsCallSharedCeremonyByConstruction(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// See TestGitOptionsApplyDryRunTouchesNothingAndSkipsConfirm's comment:
	// the probe requires ~/.gitconfig.d to pre-exist.
	if err := os.MkdirAll(filepath.Join(home, ".gitconfig.d"), 0o700); err != nil {
		t.Fatalf("mkdir .gitconfig.d: %v", err)
	}
	seamGuard(t)

	var applyCalls int
	cliGlobalGitApplyInto = func(_ *realBackend, keys []string, p lifecyclePolicy) (lifecycleResult, error) {
		applyCalls++
		if len(keys) != 1 || keys[0] != "init.defaultBranch" {
			t.Errorf("apply keys = %v, want [init.defaultBranch]", keys)
		}
		if p.DryRun {
			t.Error("headless --yes apply must not set DryRun")
		}
		return lifecycleResult{Backups: []string{"bak"}}, nil
	}
	cmd, _, _ := cliTestCmd()
	if err := runGitOptionsApply(cmd, []string{"init.defaultBranch"}, gitApplyFlags{Yes: true}, false, false); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if applyCalls != 1 {
		t.Errorf("apply ceremony invoked %d times, want 1", applyCalls)
	}

	var fallbackCalls int
	cliGitFallbackAuthorApplyInto = func(_ *realBackend, name, email string, _ lifecyclePolicy) (lifecycleResult, error) {
		fallbackCalls++
		if name != "Pat" || email != "" {
			t.Errorf("fallback pair = (%q, %q), want (Pat, empty)", name, email)
		}
		return lifecycleResult{Backups: []string{"bak"}}, nil
	}
	cmd, _, _ = cliTestCmd()
	if err := runGitFallbackSet(cmd, gitFallbackSetFlags{Name: "Pat", nameWasSet: true, Yes: true}, false, false); err != nil {
		t.Fatalf("fallback set: %v", err)
	}
	if fallbackCalls != 1 {
		t.Errorf("fallback ceremony invoked %d times, want 1", fallbackCalls)
	}

	src, err := os.ReadFile(filepath.Join(testRepoRoot(t), "cmd", "gitid", "git.go")) //nolint:gosec // repository source
	if err != nil {
		t.Fatalf("read git.go: %v", err)
	}
	body := string(src)
	if !strings.Contains(body, "cliGlobalGitApplyInto") || !strings.Contains(body, "b.runGlobalGitApply") {
		t.Error("git.go must call runGlobalGitApply through cliGlobalGitApplyInto")
	}
	if !strings.Contains(body, "cliGitFallbackAuthorApplyInto") || !strings.Contains(body, "b.runGitFallbackAuthorApply") {
		t.Error("git.go must call runGitFallbackAuthorApply through cliGitFallbackAuthorApplyInto")
	}
	if strings.Contains(body, "CommitGlobalGit") || strings.Contains(body, "CommitGitFallbackAuthor") {
		t.Error("CLI verbs must not invoke a tea.Cmd commit seam")
	}
	if strings.Contains(body, "filewriter.Write") || strings.Contains(body, "os.WriteFile") {
		t.Error("CLI verbs must contain no direct file-writing call")
	}
}

func TestGitWriteVerbsUseSharedDepthHelpersByConstruction(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(testRepoRoot(t), "cmd", "gitid", "git.go")) //nolint:gosec // repository source
	if err != nil {
		t.Fatalf("read git.go: %v", err)
	}
	body := string(src)
	if !strings.Contains(body, "depthResolver{") {
		t.Error("git.go must resolve adaptive depth through depthResolver")
	}
	if !strings.Contains(body, "confirmationPolicyFrom(") {
		t.Error("git.go must map --yes through confirmationPolicyFrom")
	}
	for _, fn := range []string{"func runGitOptionsApply", "func runGitFallbackSet"} {
		start := strings.Index(body, fn)
		if start < 0 {
			t.Errorf("git.go missing %s", fn)
			continue
		}
		rest := body[start:]
		end := strings.Index(rest[1:], "\nfunc ")
		if end < 0 {
			end = len(rest)
		} else {
			end++
		}
		fnBody := rest[:end]
		if strings.Contains(fnBody, "term.IsTerminal") || strings.Contains(fnBody, "termIsStdinTTY") || strings.Contains(fnBody, "termIsStdoutTTY") {
			t.Errorf("%s must not read terminal descriptors inline; inject the two booleans", fn)
		}
	}
}

func TestGitOptionsApplyRefusesUnknownTokenByName(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	before := snapshotPaths(t, []string{filepath.Join(home, ".gitconfig")})
	cmd, _, _ := cliTestCmd()
	err := runGitOptionsApply(cmd, []string{"TotallyFake"}, gitApplyFlags{Yes: true}, false, false)
	if exitStatusOf(err) != 1 {
		t.Fatalf("unknown token exit = %v, want 1", err)
	}
	if err == nil || !strings.Contains(err.Error(), "TotallyFake") {
		t.Errorf("error = %v, want it to name the unknown token", err)
	}
	assertUnchanged(t, before, snapshotPaths(t, []string{filepath.Join(home, ".gitconfig")}))
}

func TestGitOptionsApplyRefusesMemberKeysNamingOwningToken(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	type memberCase struct {
		member string
		token  string
	}
	var cases []memberCase
	for _, p := range globalgit.Policy {
		if p.Token == "" {
			continue
		}
		for _, m := range p.Members {
			if m.Key == p.Token {
				continue
			}
			cases = append(cases, memberCase{member: m.Key, token: p.Token})
		}
	}
	cases = append(cases, memberCase{member: "init.defaultbranch", token: "init.defaultBranch"})
	if len(cases) == 0 {
		t.Fatal("policy produced no member-key cases")
	}
	for _, tc := range cases {
		t.Run(tc.member, func(t *testing.T) {
			cmd, _, _ := cliTestCmd()
			err := runGitOptionsApply(cmd, []string{tc.member}, gitApplyFlags{Yes: true}, false, false)
			if exitStatusOf(err) != 1 {
				t.Fatalf("member key %q exit = %v, want 1", tc.member, err)
			}
			if err == nil || !strings.Contains(err.Error(), tc.token) {
				t.Errorf("error = %v, want it to name owning token %q", err, tc.token)
			}
			if err != nil && strings.Contains(err.Error(), "unknown") && !strings.Contains(err.Error(), tc.token) {
				t.Errorf("refusal must name the owning token, not only 'unknown': %v", err)
			}
		})
	}
}

func TestGitOptionsApplyAcceptedTokensEqualPolicyTokens(t *testing.T) {
	got := gitApplyTokens()
	var want []string
	for _, p := range globalgit.Policy {
		if p.Token != "" {
			want = append(want, p.Token)
		}
	}
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("accepted tokens = %v, want policy tokens %v", got, want)
	}
}

func TestGitOptionsApplyHelpListsTokensIncludingLineEndings(t *testing.T) {
	root := newRootCmd()
	cmd, _, err := root.Find([]string{"git", "options", "apply"})
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	if herr := cmd.Help(); herr != nil {
		t.Fatalf("Help: %v", herr)
	}
	help := buf.String()
	for _, tok := range gitApplyTokens() {
		if !strings.Contains(help, tok) {
			t.Errorf("apply help missing token %q:\n%s", tok, help)
		}
	}
	if !strings.Contains(help, "core.lineEndings") {
		t.Errorf("apply help must list the one-word line-endings token:\n%s", help)
	}
}

func TestGitOptionsApplyRefusesNonSelectableStates(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seamGuard(t)

	cases := []struct {
		name  string
		state tuikit.GlobalGitOptionState
		word  string
		view  tuikit.GlobalGitOptionView
	}{
		{
			name:  "already-set",
			state: tuikit.GlobalGitAlreadySet,
			word:  "already-set",
			view: tuikit.GlobalGitOptionView{
				Key: "init.defaultBranch", State: tuikit.GlobalGitAlreadySet,
				PolicyBacked: true, HasWritableMember: true,
			},
		},
		{
			name:  "differs",
			state: tuikit.GlobalGitSetButDiffers,
			word:  "differs",
			view: tuikit.GlobalGitOptionView{
				Key: "init.defaultBranch", State: tuikit.GlobalGitSetButDiffers,
				PolicyBacked: true, HasWritableMember: true,
			},
		},
		{
			name:  "not-applicable",
			state: tuikit.GlobalGitNotApplicable,
			word:  "not-applicable",
			view: tuikit.GlobalGitOptionView{
				Key: "init.defaultBranch", State: tuikit.GlobalGitNotApplicable,
				PolicyBacked: true, HasWritableMember: true,
			},
		},
		{
			name:  "probe-error",
			state: tuikit.GlobalGitNeedsAction,
			word:  "probe",
			view: tuikit.GlobalGitOptionView{
				Key: "init.defaultBranch", State: tuikit.GlobalGitNeedsAction,
				PolicyBacked: true, HasWritableMember: true, ProbeError: "git probe failed",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cliGitOptionStates = func(_ *realBackend) ([]tuikit.GlobalGitOptionView, error) {
				return []tuikit.GlobalGitOptionView{tc.view}, nil
			}
			cmd, _, _ := cliTestCmd()
			err := runGitOptionsApply(cmd, []string{"init.defaultBranch"}, gitApplyFlags{Yes: true}, false, false)
			if exitStatusOf(err) != 1 {
				t.Fatalf("non-selectable %s exit = %v, want 1", tc.name, err)
			}
			if err == nil || !strings.Contains(err.Error(), tc.word) {
				t.Errorf("error = %v, want it to name state %q", err, tc.word)
			}
		})
	}
}

func TestGitOptionsApplyDryRunTouchesNothingAndSkipsConfirm(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// The probe (cliGitOptionStates -> globalgit.Statuses) shells out with a
	// working directory rooted at ~/.gitconfig.d and fails outright ("chdir:
	// no such file or directory") if it is absent — every real entry point
	// (e2e's seedGlobalGitHome, the TUI's own activate()) always has this
	// directory pre-created before the first probe runs.
	if err := os.MkdirAll(filepath.Join(home, ".gitconfig.d"), 0o700); err != nil {
		t.Fatalf("mkdir .gitconfig.d: %v", err)
	}
	gitconfigPath := filepath.Join(home, ".gitconfig")
	original := []byte("[user]\n\tname = leftover\n")
	if err := os.WriteFile(gitconfigPath, original, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	seamGuard(t)
	var confirmCalls int
	cliGlobalGitApplyInto = func(b *realBackend, keys []string, p lifecyclePolicy) (lifecycleResult, error) {
		if !p.DryRun {
			t.Error("dry-run must set DryRun on the lifecycle policy")
		}
		if p.Prompt != nil {
			confirmCalls++
		}
		return b.runGlobalGitApply(keys, p)
	}
	cmd, out, _ := cliTestCmd()
	err := runGitOptionsApply(cmd, []string{"init.defaultBranch"}, gitApplyFlags{DryRun: true}, false, false)
	if err != nil {
		t.Fatalf("dry-run must exit zero: %v", err)
	}
	after, rerr := os.ReadFile(gitconfigPath) //nolint:gosec // test sandbox path
	if rerr != nil {
		t.Fatalf("reread: %v", rerr)
	}
	if !bytes.Equal(after, original) {
		t.Errorf("dry-run mutated config:\nbefore=%q\nafter=%q", original, after)
	}
	if confirmCalls != 0 {
		t.Errorf("dry-run consulted the confirmation prompt %d times", confirmCalls)
	}
	if !strings.Contains(out.String(), "dry run") {
		t.Errorf("dry-run output missing preview:\n%s", out.String())
	}
}

func TestGitOptionsApplyOffTerminalWithoutYesRefuses(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// See TestGitOptionsApplyDryRunTouchesNothingAndSkipsConfirm's comment:
	// the probe requires ~/.gitconfig.d to pre-exist.
	if err := os.MkdirAll(filepath.Join(home, ".gitconfig.d"), 0o700); err != nil {
		t.Fatalf("mkdir .gitconfig.d: %v", err)
	}
	seamGuard(t)
	var calls int
	cliGlobalGitApplyInto = func(_ *realBackend, _ []string, _ lifecyclePolicy) (lifecycleResult, error) {
		calls++
		return lifecycleResult{}, nil
	}
	cmd, _, _ := cliTestCmd()
	err := runGitOptionsApply(cmd, []string{"init.defaultBranch"}, gitApplyFlags{}, false, false)
	if exitStatusOf(err) != 1 {
		t.Fatalf("unauthorized apply exit = %v, want 1", err)
	}
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Errorf("error = %v, want it to name --yes", err)
	}
	if calls != 0 {
		t.Errorf("lifecycle invoked %d times on unauthorized apply, want 0", calls)
	}
}

func TestGitFallbackSetFlagRefusals(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	t.Run("empty name", func(t *testing.T) {
		cmd, _, _ := cliTestCmd()
		err := runGitFallbackSet(cmd, gitFallbackSetFlags{Name: "", nameWasSet: true, Yes: true}, false, false)
		if exitStatusOf(err) != 1 {
			t.Fatalf("empty name exit = %v, want 1", err)
		}
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), "empty") {
			t.Errorf("error = %v, want it to name empty", err)
		}
	})
	t.Run("empty email", func(t *testing.T) {
		cmd, _, _ := cliTestCmd()
		err := runGitFallbackSet(cmd, gitFallbackSetFlags{Email: "", emailWasSet: true, Yes: true}, false, false)
		if exitStatusOf(err) != 1 {
			t.Fatalf("empty email exit = %v, want 1", err)
		}
		if err == nil || !strings.Contains(strings.ToLower(err.Error()), "empty") {
			t.Errorf("error = %v, want it to name empty", err)
		}
	})
	t.Run("set and clear name", func(t *testing.T) {
		cmd, _, _ := cliTestCmd()
		err := runGitFallbackSet(cmd, gitFallbackSetFlags{Name: "Pat", nameWasSet: true, ClearName: true, Yes: true}, false, false)
		if exitStatusOf(err) != 1 {
			t.Fatalf("set+clear name exit = %v, want 1", err)
		}
	})
	t.Run("set and clear email", func(t *testing.T) {
		cmd, _, _ := cliTestCmd()
		err := runGitFallbackSet(cmd, gitFallbackSetFlags{Email: "p@e.com", emailWasSet: true, ClearEmail: true, Yes: true}, false, false)
		if exitStatusOf(err) != 1 {
			t.Fatalf("set+clear email exit = %v, want 1", err)
		}
	})
	t.Run("no flags", func(t *testing.T) {
		cmd, _, _ := cliTestCmd()
		err := runGitFallbackSet(cmd, gitFallbackSetFlags{Yes: true}, false, false)
		if exitStatusOf(err) != 1 {
			t.Fatalf("no-flags exit = %v, want 1", err)
		}
		if err == nil || !strings.Contains(err.Error(), "--name") || !strings.Contains(err.Error(), "--email") {
			t.Errorf("error = %v, want it to name the missing flags", err)
		}
	})
}

func TestGitFallbackSetOneHalfLeavesTheOtherUnchanged(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	b := newBackendForHome(home)
	if _, err := b.runGitFallbackAuthorApply("Pat Example", "pat@example.com", lifecyclePolicy{Confirm: confirmationAlreadyObtained}); err != nil {
		t.Fatalf("seed fallback: %v", err)
	}
	cmd, _, _ := cliTestCmd()
	if err := runGitFallbackSet(cmd, gitFallbackSetFlags{Name: "New Name", nameWasSet: true, Yes: true}, false, false); err != nil {
		t.Fatalf("set name only: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(home, ".gitconfig")) //nolint:gosec // test sandbox path
	if err != nil {
		t.Fatalf("read gitconfig: %v", err)
	}
	name, email := gitconfig.ReadGitFallbackAuthor(raw)
	if name != "New Name" {
		t.Errorf("name = %q, want New Name", name)
	}
	if email != "pat@example.com" {
		t.Errorf("email = %q, want unchanged pat@example.com", email)
	}
}

func TestGitDepthResolverTable(t *testing.T) {
	type combo struct {
		complete, stdin, stdout bool
		want                    resolveOutcome
	}
	combos := []combo{
		{true, false, false, resolveHeadless},
		{true, true, false, resolveHeadless},
		{true, false, true, resolveHeadless},
		{true, true, true, resolveHeadless},
		{false, true, true, resolvePrefilledTUI},
		{false, false, false, resolveMissingFlags},
		{false, true, false, resolveMissingFlags},
		{false, false, true, resolveMissingFlags},
	}
	verbs := []struct {
		name     string
		required string
	}{
		{"options apply", gitApplyRequiredKey},
		{"fallback set", gitFallbackRequired},
	}
	for _, v := range verbs {
		for _, c := range combos {
			name := fmt.Sprintf("%s/complete=%v/in=%v/out=%v", v.name, c.complete, c.stdin, c.stdout)
			t.Run(name, func(t *testing.T) {
				got, missing := depthResolver{
					required:  []string{v.required},
					supplied:  map[string]bool{v.required: c.complete},
					stdinTTY:  c.stdin,
					stdoutTTY: c.stdout,
				}.resolve()
				if got != c.want {
					t.Errorf("outcome = %v, want %v", got, c.want)
				}
				if c.want == resolveMissingFlags {
					if len(missing) != 1 || missing[0] != v.required {
						t.Errorf("missing = %v, want [%s]", missing, v.required)
					}
				}
			})
		}
	}
}

func TestGitIncompleteBothTTYsOpensEmptyTUI(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seamGuard(t)

	t.Run("apply opens Global Git empty", func(t *testing.T) {
		var launched bool
		gitTUILaunch = func(b *realBackend) error {
			launched = true
			app := tuikit.NewAppOnGlobalGit(b)
			if app.ActiveTab() != tuikit.TabGlobalGit {
				t.Errorf("tab = %v, want TabGlobalGit", app.ActiveTab())
			}
			if chosen := app.GlobalGitUIState(); chosen != 0 {
				t.Errorf("option selection = %d, want empty", chosen)
			}
			return nil
		}
		cmd, _, _ := cliTestCmd()
		if err := runGitOptionsApply(cmd, nil, gitApplyFlags{}, true, true); err != nil {
			t.Fatalf("TUI fallback: %v", err)
		}
		if !launched {
			t.Fatal("incomplete apply with both TTYs must launch the TUI")
		}
	})

	t.Run("fallback set opens Global Git empty", func(t *testing.T) {
		var launched bool
		gitTUILaunch = func(b *realBackend) error {
			launched = true
			app := tuikit.NewAppOnGlobalGit(b)
			if app.ActiveTab() != tuikit.TabGlobalGit {
				t.Errorf("tab = %v, want TabGlobalGit", app.ActiveTab())
			}
			if chosen := app.GlobalGitUIState(); chosen != 0 {
				t.Errorf("option selection = %d, want empty", chosen)
			}
			return nil
		}
		cmd, _, _ := cliTestCmd()
		if err := runGitFallbackSet(cmd, gitFallbackSetFlags{}, true, true); err != nil {
			t.Fatalf("TUI fallback: %v", err)
		}
		if !launched {
			t.Fatal("incomplete fallback set with both TTYs must launch the TUI")
		}
	})
}

func TestGitFallbackShowMissingFileReportsUnset(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cmd, out, _ := cliTestCmd()
	err := runGitFallbackShow(cmd, false)
	if err != nil {
		t.Fatalf("missing-file show must exit zero: %v", err)
	}
	body := out.String()
	if !strings.Contains(body, "unset") {
		t.Errorf("show output must report unset halves:\n%s", body)
	}
}

func TestGitFallbackShowUnreadableFileIsRefusal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, ".gitconfig")
	if err := os.WriteFile(path, []byte("[user]\n\tname = x\n"), 0o000); err != nil {
		t.Fatalf("write: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
	cmd, _, _ := cliTestCmd()
	err := runGitFallbackShow(cmd, false)
	if exitStatusOf(err) != 1 {
		t.Fatalf("unreadable file exit = %v, want 1", err)
	}
	if err == nil || !strings.Contains(err.Error(), path) && !strings.Contains(err.Error(), ".gitconfig") {
		t.Errorf("error = %v, want it to name the file", err)
	}
}

func TestGitIncompleteWithoutTerminalNamesMissingArgument(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cmd, _, _ := cliTestCmd()
	err := runGitOptionsApply(cmd, nil, gitApplyFlags{}, false, false)
	if exitStatusOf(err) != 1 {
		t.Fatalf("incomplete apply exit = %v, want 1", err)
	}
	if err == nil || !strings.Contains(err.Error(), gitApplyRequiredKey) {
		t.Errorf("error = %v, want it to name %s", err, gitApplyRequiredKey)
	}

	cmd, _, _ = cliTestCmd()
	err = runGitFallbackSet(cmd, gitFallbackSetFlags{}, false, false)
	if exitStatusOf(err) != 1 {
		t.Fatalf("incomplete fallback set exit = %v, want 1", err)
	}
	if err == nil || !strings.Contains(err.Error(), gitFallbackRequired) {
		t.Errorf("error = %v, want it to name %s", err, gitFallbackRequired)
	}
}

func TestGitOptionsApplyBelowGateAdvisoryOriginatesFromCeremony(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	b := newBackendForHome(home)
	b.gitGate = func() (globalgit.GateOutcome, string) { return globalgit.GateBelow, "" }
	res, err := b.runGlobalGitApply([]string{"merge.conflictstyle"}, lifecyclePolicy{Confirm: confirmationAlreadyObtained})
	if err != nil {
		t.Fatalf("runGlobalGitApply: %v", err)
	}
	if len(res.Advisories) == 0 {
		t.Fatal("below-gate apply must produce an advisory from the ceremony")
	}
	joined := strings.Join(res.Advisories, "\n")
	if !strings.Contains(joined, "diff3") {
		t.Errorf("advisory must name the written fallback value:\n%s", joined)
	}

	src, rerr := os.ReadFile(filepath.Join(testRepoRoot(t), "cmd", "gitid", "lifecycle.go")) //nolint:gosec // repository source
	if rerr != nil {
		t.Fatalf("read lifecycle.go: %v", rerr)
	}
	if !strings.Contains(string(src), "diff3") {
		t.Error("the substitution advisory must originate in the ceremony, not the CLI layer")
	}
}

func TestGitCompletionStillGenerates(t *testing.T) {
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"__complete", "git", ""})
	if err := root.Execute(); err != nil {
		t.Fatalf("__complete git: %v", err)
	}
	got := buf.String()
	for _, want := range []string{"options", "fallback"} {
		if !strings.Contains(got, want) {
			t.Errorf("git completion missing %q:\n%s", want, got)
		}
	}
}
