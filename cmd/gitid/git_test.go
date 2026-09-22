package main

// git_test.go covers plan 07-05's frozen CLI contract: the four command
// paths, shared-ceremony call sites, R-5 token-only argument validation,
// adaptive-depth table, and the explicit set-versus-clear fallback contract.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
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
	skipUnlessUnreadableFilesEnforced(t)
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

// goStringLiterals parses path (a Go source file) and returns every string
// literal found in the AST, unquoted.
func goStringLiterals(t *testing.T, path string) []string {
	t.Helper()
	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}

	var result []string
	ast.Inspect(astFile, func(node ast.Node) bool {
		if basicLit, ok := node.(*ast.BasicLit); ok && basicLit.Kind == token.STRING {
			unquoted, uerr := strconv.Unquote(basicLit.Value)
			if uerr == nil {
				result = append(result, unquoted)
			}
		}
		return true
	})
	return result
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

	// Resolve the policy and verify advisory uses policy-derived values
	policy, ok := globalgit.PolicyFor("merge.conflictstyle")
	if !ok {
		t.Fatal("merge.conflictstyle policy not found")
	}
	if policy.Gate != globalgit.GateHard {
		t.Fatalf("merge.conflictstyle gate = %v, want GateHard", policy.Gate)
	}
	if policy.Fallback == "" {
		t.Fatal("merge.conflictstyle has empty Fallback")
	}
	if policy.Recommended == "" {
		t.Fatal("merge.conflictstyle has empty Recommended")
	}

	// Advisory must contain policy.Key, quoted Fallback, and quoted Recommended
	if !strings.Contains(joined, policy.Key) {
		t.Errorf("advisory missing policy key %q", policy.Key)
	}
	quotedFallback := strconv.Quote(policy.Fallback)
	quotedRecommended := strconv.Quote(policy.Recommended)
	if !strings.Contains(joined, quotedFallback) {
		t.Errorf("advisory missing quoted fallback %s (fallback value = %q)\n%s", quotedFallback, policy.Fallback, joined)
	}
	if !strings.Contains(joined, quotedRecommended) {
		t.Errorf("advisory missing quoted recommended %s (recommended value = %q)\n%s", quotedRecommended, policy.Recommended, joined)
	}

	// Advisory must contain the wording produced by the ceremony (not derived in CLI layer)
	if !strings.Contains(joined, "below the git version gate") {
		t.Errorf("advisory must say 'below the git version gate' (ceremony wording):\n%s", joined)
	}

	// git.go must NOT contain "below the git version gate" as a literal string
	gitFilePath := filepath.Join(testRepoRoot(t), "cmd", "gitid", "git.go")
	gitLiterals := goStringLiterals(t, gitFilePath)
	for _, lit := range gitLiterals {
		if strings.Contains(lit, "below the git version gate") {
			t.Error("git.go CLI layer must not contain 'below the git version gate' literal — the advisory must originate in the ceremony")
		}
	}

	// lifecycle.go must NOT have a literal equal to policy.Fallback
	lifecycleFilePath := filepath.Join(testRepoRoot(t), "cmd", "gitid", "lifecycle.go")
	lifecycleLiterals := goStringLiterals(t, lifecycleFilePath)
	for _, lit := range lifecycleLiterals {
		if lit == policy.Fallback {
			t.Errorf("lifecycle.go must not hardcode policy fallback %q — must derive it from globalgit.PolicyFor (WR-09)", policy.Fallback)
		}
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

// ---------------------------------------------------------------------------
// Task 2: the four versioned JSON envelopes, the exit-status table, and the
// exact-key-set / enum contracts pinned in the shape of the SSH noun's tests.
// ---------------------------------------------------------------------------

func TestGitJSONOptionsListExactKeySetAndEnums(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// The probe (cliGitOptionStates -> globalgit.Statuses) shells out with a
	// working directory rooted at ~/.gitconfig.d — see
	// TestGitOptionsApplyDryRunTouchesNothingAndSkipsConfirm.
	if err := os.MkdirAll(filepath.Join(home, ".gitconfig.d"), 0o700); err != nil {
		t.Fatalf("mkdir .gitconfig.d: %v", err)
	}
	b := newBackendForHome(home)
	recs, err := gitOptionRecords(b)
	if err != nil {
		t.Fatalf("gitOptionRecords: %v", err)
	}
	var buf bytes.Buffer
	if rerr := renderGitOptionsList(&buf, false, true, recs); rerr != nil {
		t.Fatalf("renderGitOptionsList: %v", rerr)
	}
	raw := buf.Bytes()

	keys, kerr := jsonObjectKeys(raw)
	if kerr != nil {
		t.Fatalf("top-level keys: %v\n%s", kerr, raw)
	}
	assertExactKeys(t, keys, gitOptionsDocKeys)

	var doc gitOptionsDocument
	if uerr := json.Unmarshal(raw, &doc); uerr != nil {
		t.Fatalf("unmarshal: %v", uerr)
	}
	if doc.Schema != gitOptionsSchema {
		t.Errorf("schema = %q, want %q", doc.Schema, gitOptionsSchema)
	}
	if len(doc.Options) != len(globalgit.Policy) {
		t.Fatalf("options = %d, want %d (policy declaration order)", len(doc.Options), len(globalgit.Policy))
	}
	for i, rec := range doc.Options {
		if rec.Key != globalgit.Policy[i].Key {
			t.Errorf("options[%d].key = %q, want %q", i, rec.Key, globalgit.Policy[i].Key)
		}
		recRaw, merr := json.Marshal(rec)
		if merr != nil {
			t.Fatalf("marshal option: %v", merr)
		}
		recKeys, rerr := jsonObjectKeys(recRaw)
		if rerr != nil {
			t.Fatalf("option keys: %v", rerr)
		}
		assertExactKeys(t, recKeys, gitOptionRecordKeys)
		assertEnumMember(t, "state", rec.State, gitStateEnum)
	}
}

// TestGitJSONStateEnumsAreRenderLayerTaxonomy asserts the JSON state enum is
// EXACTLY the taxonomy the render layer can emit — every
// GlobalGitOptionState maps to a documented member, and every documented
// member is reachable from the render layer. A future screen state that leaks
// into the JSON without a documented member fails here, as does a documented
// member the render layer can never produce.
func TestGitJSONStateEnumsAreRenderLayerTaxonomy(t *testing.T) {
	byRender := map[tuikit.GlobalGitOptionState]string{
		tuikit.GlobalGitNeedsAction:   "needs-action",
		tuikit.GlobalGitAlreadySet:    "already-set",
		tuikit.GlobalGitSetButDiffers: "differs",
		tuikit.GlobalGitNotApplicable: "not-applicable",
	}
	reachable := map[string]bool{}
	for state, want := range byRender {
		got := gitRowStateName(tuikit.GlobalGitOptionView{Key: "init.defaultBranch", State: state})
		if got != want {
			t.Errorf("gitRowStateName(%v) = %q, want %q", state, got, want)
		}
		assertEnumMember(t, "state", got, gitStateEnum)
		reachable[got] = true
	}
	probeErr := gitRowStateName(tuikit.GlobalGitOptionView{Key: "init.defaultBranch", ProbeError: "git probe failed"})
	if probeErr != "probe-error" {
		t.Errorf("probe-error state = %q, want probe-error", probeErr)
	}
	assertEnumMember(t, "state", probeErr, gitStateEnum)
	reachable[probeErr] = true
	for _, member := range gitStateEnum {
		if !reachable[member] {
			t.Errorf("documented state enum member %q is not reachable from the render layer", member)
		}
	}
}

func TestGitJSONEnvelopesCarrySchemaIdentifiers(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".gitconfig.d"), 0o700); err != nil {
		t.Fatalf("mkdir .gitconfig.d: %v", err)
	}
	seamGuard(t)
	cliGlobalGitApplyInto = func(_ *realBackend, _ []string, _ lifecyclePolicy) (lifecycleResult, error) {
		return lifecycleResult{Backups: []string{filepath.Join(home, ".gitconfig")}}, nil
	}

	rawApply, code := captureGitApplyJSON(t, []string{"init.defaultBranch"}, gitApplyFlags{Yes: true, JSON: true}, false, false)
	if code != 0 {
		t.Fatalf("apply exit = %d, want 0", code)
	}
	rawFallbackSet, code := captureGitFallbackSetJSON(t, gitFallbackSetFlags{Name: "Pat", nameWasSet: true, Yes: true, JSON: true}, false, false)
	if code != 0 {
		t.Fatalf("fallback set exit = %d, want 0", code)
	}
	rawFallbackShow, code := captureGitFallbackShowJSON(t)
	if code != 0 {
		t.Fatalf("fallback show exit = %d, want 0", code)
	}

	cases := []struct {
		name   string
		schema string
		raw    []byte
	}{
		{"options", gitOptionsSchema, captureGitOptionsListJSON(t, home)},
		{"apply", gitApplySchema, rawApply},
		{"fallback", gitFallbackSchema, rawFallbackShow},
		{"fallbackset", gitFallbackSetSchema, rawFallbackSet},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got struct {
				Schema string `json:"schema"`
			}
			if uerr := json.Unmarshal(tc.raw, &got); uerr != nil {
				t.Fatalf("unmarshal: %v\n%s", uerr, tc.raw)
			}
			if got.Schema != tc.schema {
				t.Errorf("schema = %q, want %q", got.Schema, tc.schema)
			}
		})
	}
}

func TestGitJSONEnvelopesExactKeySets(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".gitconfig.d"), 0o700); err != nil {
		t.Fatalf("mkdir .gitconfig.d: %v", err)
	}
	seamGuard(t)
	cliGlobalGitApplyInto = func(_ *realBackend, _ []string, _ lifecyclePolicy) (lifecycleResult, error) {
		return lifecycleResult{Backups: []string{filepath.Join(home, ".gitconfig")}, Advisories: []string{"advisory: below the version gate"}}, nil
	}

	t.Run("options", func(t *testing.T) {
		assertExactKeysFrom(t, captureGitOptionsListJSON(t, home), gitOptionsDocKeys)
	})
	t.Run("apply", func(t *testing.T) {
		raw, _ := captureGitApplyJSON(t, []string{"init.defaultBranch"}, gitApplyFlags{Yes: true, JSON: true}, false, false)
		assertExactKeysFrom(t, raw, gitApplyDocKeys)
	})
	t.Run("fallback", func(t *testing.T) {
		raw, _ := captureGitFallbackShowJSON(t)
		assertExactKeysFrom(t, raw, gitFallbackDocKeys)
	})
	t.Run("fallbackset", func(t *testing.T) {
		raw, _ := captureGitFallbackSetJSON(t, gitFallbackSetFlags{Name: "Pat", nameWasSet: true, Yes: true, JSON: true}, false, false)
		assertExactKeysFrom(t, raw, gitFallbackSetDocKeys)
	})
}

func TestGitJSONFallbackShowExactKeySetAndUnsetStatuses(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	raw, code := captureGitFallbackShowJSON(t)
	if code != 0 {
		t.Fatalf("fresh-home show exit = %d, want 0", code)
	}
	assertExactKeysFrom(t, raw, gitFallbackDocKeys)
	var doc gitFallbackDocument
	if uerr := json.Unmarshal(raw, &doc); uerr != nil {
		t.Fatalf("unmarshal: %v\n%s", uerr, raw)
	}
	if doc.Schema != gitFallbackSchema {
		t.Errorf("schema = %q, want %q", doc.Schema, gitFallbackSchema)
	}
	if doc.NameStatus != "unset" || doc.EmailStatus != "unset" {
		t.Errorf("fresh-home statuses = (%q, %q), want (unset, unset)", doc.NameStatus, doc.EmailStatus)
	}
}

func TestGitJSONApplyAndFallbackSetEnvelopesOnEveryPath(t *testing.T) {
	t.Run("apply success", func(t *testing.T) {
		gitEnvHome(t)
		raw, code := captureGitApplyJSON(t, []string{"init.defaultBranch"}, gitApplyFlags{Yes: true, JSON: true}, false, false)
		assertGitApplyEnvelope(t, raw, code, false)
		if code != 0 {
			t.Errorf("success exit = %d, want 0", code)
		}
	})
	t.Run("apply refusal unknown token", func(t *testing.T) {
		gitEnvHome(t)
		raw, code := captureGitApplyJSON(t, []string{"TotallyFake"}, gitApplyFlags{Yes: true, JSON: true}, false, false)
		assertGitApplyEnvelope(t, raw, code, false)
		if code != 1 {
			t.Errorf("unknown token exit = %d, want 1", code)
		}
		var doc gitApplyDocument
		if uerr := json.Unmarshal(raw, &doc); uerr != nil {
			t.Fatalf("unmarshal: %v", uerr)
		}
		if doc.Error == "" {
			t.Error("refusal envelope must populate error")
		}
	})
	t.Run("apply rolled-back failure", func(t *testing.T) {
		gitEnvHome(t)
		seamGuard(t)
		cliGlobalGitApplyInto = func(_ *realBackend, _ []string, _ lifecyclePolicy) (lifecycleResult, error) {
			return lifecycleResult{Restored: []string{"~/.gitconfig.d/00-baseline: restored"}}, fmt.Errorf("injected write failure")
		}
		raw, code := captureGitApplyJSON(t, []string{"init.defaultBranch"}, gitApplyFlags{Yes: true, JSON: true}, false, false)
		assertGitApplyEnvelope(t, raw, code, false)
		if code != 2 {
			t.Errorf("rolled-back apply exit = %d, want 2", code)
		}
		var doc gitApplyDocument
		if uerr := json.Unmarshal(raw, &doc); uerr != nil {
			t.Fatalf("unmarshal: %v", uerr)
		}
		if len(doc.Restored) == 0 {
			t.Error("rolled-back apply envelope must report its restored paths")
		}
	})
	t.Run("fallback set refusal no flags", func(t *testing.T) {
		gitEnvHome(t)
		raw, code := captureGitFallbackSetJSON(t, gitFallbackSetFlags{Yes: true, JSON: true}, false, false)
		assertGitFallbackSetEnvelope(t, raw, code, false)
		if code != 1 {
			t.Errorf("no-flags exit = %d, want 1", code)
		}
		var doc gitFallbackSetDocument
		if uerr := json.Unmarshal(raw, &doc); uerr != nil {
			t.Fatalf("unmarshal: %v", uerr)
		}
		if doc.Error == "" {
			t.Error("refusal envelope must populate error")
		}
	})
	t.Run("fallback set rolled-back failure", func(t *testing.T) {
		gitEnvHome(t)
		seamGuard(t)
		cliGitFallbackAuthorApplyInto = func(_ *realBackend, _, _ string, _ lifecyclePolicy) (lifecycleResult, error) {
			return lifecycleResult{Restored: []string{"~/.gitconfig: restored"}}, fmt.Errorf("injected write failure")
		}
		raw, code := captureGitFallbackSetJSON(t, gitFallbackSetFlags{Name: "Pat", nameWasSet: true, Yes: true, JSON: true}, false, false)
		assertGitFallbackSetEnvelope(t, raw, code, false)
		if code != 2 {
			t.Errorf("rolled-back fallback set exit = %d, want 2", code)
		}
	})
}

func gitEnvHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".gitconfig.d"), 0o700); err != nil {
		t.Fatalf("mkdir .gitconfig.d: %v", err)
	}
}

func TestGitApplyAdvisoryExitZeroByDefaultNonZeroWithOptIn(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".gitconfig.d"), 0o700); err != nil {
		t.Fatalf("mkdir .gitconfig.d: %v", err)
	}
	seamGuard(t)
	cliGlobalGitApplyInto = func(_ *realBackend, _ []string, _ lifecyclePolicy) (lifecycleResult, error) {
		return lifecycleResult{Backups: []string{"b"}, Advisories: []string{"advisory: below the version gate"}}, nil
	}

	_, codeDefault := captureGitApplyJSON(t, []string{"init.defaultBranch"}, gitApplyFlags{Yes: true, JSON: true}, false, false)
	if codeDefault != 0 {
		t.Errorf("advisory success default exit = %d, want 0", codeDefault)
	}
	_, codeOptIn := captureGitApplyJSON(t, []string{"init.defaultBranch"}, gitApplyFlags{Yes: true, JSON: true, FailOnAdvisory: true}, false, false)
	if codeOptIn != 3 {
		t.Errorf("advisory success --fail-on-advisory exit = %d, want 3", codeOptIn)
	}
}

func TestGitApplyDryRunExitsZeroEvenWithAdvisoryOptIn(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".gitconfig.d"), 0o700); err != nil {
		t.Fatalf("mkdir .gitconfig.d: %v", err)
	}
	seamGuard(t)
	cliGlobalGitApplyInto = func(_ *realBackend, _ []string, _ lifecyclePolicy) (lifecycleResult, error) {
		return lifecycleResult{Advisories: []string{"advisory: below the version gate"}}, nil
	}
	raw, code := captureGitApplyJSON(t, []string{"init.defaultBranch"}, gitApplyFlags{DryRun: true, JSON: true, FailOnAdvisory: true}, false, false)
	assertGitApplyEnvelope(t, raw, code, true)
	if code != 0 {
		t.Errorf("dry run with advisory opt-in exit = %d, want 0", code)
	}
}

func TestGitExitCodeEqualsEnvelopeForEveryRow(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(home, ".gitconfig.d"), 0o700); err != nil {
		t.Fatalf("mkdir .gitconfig.d: %v", err)
	}
	seamGuard(t)

	cases := []struct {
		name           string
		res            lifecycleResult
		err            error
		failOnAdvisory bool
		dryRun         bool
		want           int
	}{
		{"success", lifecycleResult{Backups: []string{"b"}}, nil, false, false, 0},
		{"success-with-advisory", lifecycleResult{Backups: []string{"b"}, Advisories: []string{"advisory: below the version gate"}}, nil, false, false, 0},
		{"fail-on-advisory", lifecycleResult{Backups: []string{"b"}, Advisories: []string{"advisory: below the version gate"}}, nil, true, false, 3},
		{"usage-refusal", lifecycleResult{}, fmt.Errorf("unknown token"), false, false, 1},
		{"rolled-back", lifecycleResult{Restored: []string{"r"}}, fmt.Errorf("write failed"), false, false, 2},
		{"dry-run-advisory-stays-zero", lifecycleResult{Advisories: []string{"advisory: below the version gate"}}, nil, true, true, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := gitWriteExitCode(tc.res, tc.err, tc.failOnAdvisory, tc.dryRun)
			if got != tc.want {
				t.Errorf("gitWriteExitCode = %d, want %d", got, tc.want)
			}
			cliGlobalGitApplyInto = func(_ *realBackend, _ []string, _ lifecyclePolicy) (lifecycleResult, error) {
				return tc.res, tc.err
			}
			raw, code := captureGitApplyJSON(t, []string{"init.defaultBranch"}, gitApplyFlags{
				Yes:            true,
				JSON:           true,
				FailOnAdvisory: tc.failOnAdvisory,
				DryRun:         tc.dryRun,
			}, false, false)
			var doc gitApplyDocument
			if uerr := json.Unmarshal(raw, &doc); uerr != nil {
				t.Fatalf("unmarshal: %v\n%s", uerr, raw)
			}
			if doc.ExitCode != code {
				t.Errorf("envelope exit_code %d != process status %d", doc.ExitCode, code)
			}
			if doc.ExitCode != tc.want {
				t.Errorf("envelope exit_code = %d, want %d", doc.ExitCode, tc.want)
			}
		})
	}
}

func captureGitApplyJSON(t *testing.T, tokens []string, flags gitApplyFlags, stdin, stdout bool) ([]byte, int) {
	t.Helper()
	cmd, out, _ := cliTestCmd()
	err := runGitOptionsApply(cmd, tokens, flags, stdin, stdout)
	return out.Bytes(), exitStatusOf(err)
}

func captureGitFallbackSetJSON(t *testing.T, flags gitFallbackSetFlags, stdin, stdout bool) ([]byte, int) {
	t.Helper()
	cmd, out, _ := cliTestCmd()
	err := runGitFallbackSet(cmd, flags, stdin, stdout)
	return out.Bytes(), exitStatusOf(err)
}

func captureGitFallbackShowJSON(t *testing.T) ([]byte, int) {
	t.Helper()
	cmd, out, _ := cliTestCmd()
	err := runGitFallbackShow(cmd, true)
	return out.Bytes(), exitStatusOf(err)
}

func captureGitOptionsListJSON(t *testing.T, home string) []byte {
	t.Helper()
	b := newBackendForHome(home)
	recs, err := gitOptionRecords(b)
	if err != nil {
		t.Fatalf("gitOptionRecords: %v", err)
	}
	var buf bytes.Buffer
	if rerr := renderGitOptionsList(&buf, false, true, recs); rerr != nil {
		t.Fatalf("renderGitOptionsList: %v", rerr)
	}
	return buf.Bytes()
}

func assertGitApplyEnvelope(t *testing.T, raw []byte, processCode int, dryRun bool) {
	t.Helper()
	assertExactKeysFrom(t, raw, gitApplyDocKeys)
	var doc gitApplyDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal apply envelope: %v\n%s", err, raw)
	}
	if doc.Schema != gitApplySchema {
		t.Errorf("schema = %q, want %q", doc.Schema, gitApplySchema)
	}
	if doc.DryRun != dryRun {
		t.Errorf("dry_run = %v, want %v", doc.DryRun, dryRun)
	}
	if doc.ExitCode != processCode {
		t.Errorf("exit_code %d != process status %d", doc.ExitCode, processCode)
	}
	if doc.Advisories == nil {
		t.Error("advisories must be present even when empty")
	}
}

func assertGitFallbackSetEnvelope(t *testing.T, raw []byte, processCode int, dryRun bool) {
	t.Helper()
	assertExactKeysFrom(t, raw, gitFallbackSetDocKeys)
	var doc gitFallbackSetDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal fallback-set envelope: %v\n%s", err, raw)
	}
	if doc.Schema != gitFallbackSetSchema {
		t.Errorf("schema = %q, want %q", doc.Schema, gitFallbackSetSchema)
	}
	if doc.DryRun != dryRun {
		t.Errorf("dry_run = %v, want %v", doc.DryRun, dryRun)
	}
	if doc.ExitCode != processCode {
		t.Errorf("exit_code %d != process status %d", doc.ExitCode, processCode)
	}
	if doc.Advisories == nil {
		t.Error("advisories must be present even when empty")
	}
}
