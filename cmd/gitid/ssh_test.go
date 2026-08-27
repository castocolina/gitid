package main

// ssh_test.go covers plan 06-06's frozen CLI contract: the four command
// paths, the four JSON envelopes' exact key sets and enum values, the
// exit-status table (process status equals envelope exit_code), the
// adaptive-depth table, refusals, dry-run no-write, and by-construction
// ceremony invocation.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/castocolina/gitid/internal/globalssh"
	"github.com/castocolina/gitid/internal/platform"
	"github.com/castocolina/gitid/internal/tuikit"
)

func TestSSHCmdTreeExactlyFourFrozenPaths(t *testing.T) {
	root := newRootCmd()
	want := []string{
		"gitid ssh options list",
		"gitid ssh options apply",
		"gitid ssh storage show",
		"gitid ssh storage migrate",
	}
	got := runnableSSHPaths(root)
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ssh command tree = %v, want exactly %v", got, want)
	}
}

func runnableSSHPaths(root *cobra.Command) []string {
	ssh, _, err := root.Find([]string{"ssh"})
	if err != nil {
		return nil
	}
	var out []string
	var walk func(*cobra.Command)
	walk = func(c *cobra.Command) {
		if c.RunE != nil && c != ssh {
			out = append(out, c.CommandPath())
		}
		for _, child := range c.Commands() {
			walk(child)
		}
	}
	walk(ssh)
	return out
}

func TestSSHNounGroupHelpListsRealVerbs(t *testing.T) {
	root := newRootCmd()
	cmd, _, err := root.Find([]string{"ssh"})
	if err != nil {
		t.Fatalf("Find(ssh): %v", err)
	}
	if cmd.RunE != nil {
		if rerr := cmd.RunE(cmd, nil); rerr != nil && strings.Contains(rerr.Error(), "arrives in") {
			t.Fatalf("ssh noun still returns the reserved-placeholder error: %v", rerr)
		}
	}
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	if herr := cmd.Help(); herr != nil {
		t.Fatalf("ssh Help: %v", herr)
	}
	help := buf.String()
	for _, want := range []string{"options", "storage"} {
		if !strings.Contains(help, want) {
			t.Errorf("ssh help missing sub-noun %q:\n%s", want, help)
		}
	}
}

func TestSSHCmdFlagParsing(t *testing.T) {
	root := newRootCmd()
	cases := []struct {
		path  []string
		flags []string
	}{
		{[]string{"ssh", "options", "list"}, []string{"json"}},
		{[]string{"ssh", "options", "apply"}, []string{"dry-run", "yes", "fail-on-advisory", "json"}},
		{[]string{"ssh", "storage", "show"}, []string{"json"}},
		{[]string{"ssh", "storage", "migrate"}, []string{"to", "dry-run", "yes", "json"}},
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

func TestSSHJSONOptionsListExactKeySetAndEnums(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	b := newBackendForHome(home)
	recs, err := sshOptionRecords(b)
	if err != nil {
		t.Fatalf("sshOptionRecords: %v", err)
	}
	var buf bytes.Buffer
	if rerr := renderSSHOptionsList(&buf, false, true, recs); rerr != nil {
		t.Fatalf("renderSSHOptionsList: %v", rerr)
	}
	raw := buf.Bytes()

	keys, kerr := jsonObjectKeys(raw)
	if kerr != nil {
		t.Fatalf("top-level keys: %v\n%s", kerr, raw)
	}
	assertExactKeys(t, keys, sshOptionsDocKeys)

	var doc sshOptionsDocument
	if uerr := json.Unmarshal(raw, &doc); uerr != nil {
		t.Fatalf("unmarshal: %v", uerr)
	}
	if doc.Schema != sshOptionsSchema {
		t.Errorf("schema = %q, want %q", doc.Schema, sshOptionsSchema)
	}
	if len(doc.Options) != len(globalssh.Policy) {
		t.Fatalf("options = %d, want %d (policy declaration order)", len(doc.Options), len(globalssh.Policy))
	}
	for i, rec := range doc.Options {
		if rec.Key != globalssh.Policy[i].Key {
			t.Errorf("options[%d].key = %q, want %q", i, rec.Key, globalssh.Policy[i].Key)
		}
		recRaw, merr := json.Marshal(rec)
		if merr != nil {
			t.Fatalf("marshal option: %v", merr)
		}
		recKeys, rerr := jsonObjectKeys(recRaw)
		if rerr != nil {
			t.Fatalf("option keys: %v", rerr)
		}
		assertExactKeys(t, recKeys, sshOptionRecordKeys)
		assertEnumMember(t, "state", rec.State, sshStateEnum)
		assertEnumMember(t, "source", rec.Source, sshSourceEnum)
		assertEnumMember(t, "risk", rec.Risk, sshRiskEnum)
		assertEnumMember(t, "scope", rec.Scope, sshScopeEnum)
		assertEnumMember(t, "not_applicable_reason", rec.NotApplicableReason, sshNAReasonEnum)
	}
}

func TestSSHJSONStorageShowExactKeySet(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	doc := buildSSHStorageDocument(newBackendForHome(home))
	var buf bytes.Buffer
	if err := writeJSON(&buf, doc); err != nil {
		t.Fatalf("writeJSON: %v", err)
	}
	keys, err := jsonObjectKeys(buf.Bytes())
	if err != nil {
		t.Fatalf("keys: %v", err)
	}
	assertExactKeys(t, keys, sshStorageDocKeys)
	if doc.Schema != sshStorageSchema {
		t.Errorf("schema = %q, want %q", doc.Schema, sshStorageSchema)
	}
	assertEnumMember(t, "layout", doc.Layout, sshLayoutEnum)
}

func TestSSHJSONApplyAndMigrateEnvelopesOnEveryPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	t.Run("apply success", func(t *testing.T) {
		raw, code := captureApplyJSON(t, []string{"HashKnownHosts"}, sshApplyFlags{Yes: true, JSON: true}, false, false)
		assertApplyEnvelope(t, raw, code, false)
	})
	t.Run("apply refusal unknown key", func(t *testing.T) {
		raw, code := captureApplyJSON(t, []string{"NotARealOption"}, sshApplyFlags{Yes: true, JSON: true}, false, false)
		assertApplyEnvelope(t, raw, code, false)
		if code != 1 {
			t.Errorf("unknown key exit = %d, want 1", code)
		}
	})
	t.Run("apply rolled-back failure", func(t *testing.T) {
		seamGuard(t)
		cliGlobalSSHApplyInto = func(_ *realBackend, _ []string, _ lifecyclePolicy) (lifecycleResult, error) {
			return lifecycleResult{Restored: []string{"~/.ssh/config: restored"}}, fmt.Errorf("injected write failure")
		}
		raw, code := captureApplyJSON(t, []string{"HashKnownHosts"}, sshApplyFlags{Yes: true, JSON: true}, false, false)
		assertApplyEnvelope(t, raw, code, false)
		if code != 2 {
			t.Errorf("rolled-back apply exit = %d, want 2", code)
		}
	})
	t.Run("migrate refusal unknown layout", func(t *testing.T) {
		cmd, out, _ := cliTestCmd()
		err := runSSHStorageMigrateVerb(cmd, sshMigrateFlags{To: "cloud", Yes: true, JSON: true}, false, false)
		raw := out.Bytes()
		code := exitStatusOf(err)
		assertMigrateEnvelope(t, raw, code, false)
		if code != 1 {
			t.Errorf("unknown layout exit = %d, want 1", code)
		}
	})
	t.Run("migrate rolled-back failure", func(t *testing.T) {
		seamGuard(t)
		cliSSHStorageMigrateInto = func(_ *realBackend, _ tuikit.SSHStorageLayout, _ lifecyclePolicy) (lifecycleResult, error) {
			return lifecycleResult{Restored: []string{"~/.ssh/config: restored"}}, fmt.Errorf("injected migrate failure")
		}
		cmd, out, _ := cliTestCmd()
		err := runSSHStorageMigrateVerb(cmd, sshMigrateFlags{To: sshLayoutInFile, Yes: true, JSON: true}, false, false)
		assertMigrateEnvelope(t, out.Bytes(), exitStatusOf(err), false)
		if exitStatusOf(err) != 2 {
			t.Errorf("rolled-back migrate exit = %d, want 2", exitStatusOf(err))
		}
	})
}

func TestSSHJSONApplyAdvisoriesPresentEvenWhenEmpty(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seamGuard(t)

	t.Run("empty list on clean success", func(t *testing.T) {
		cliGlobalSSHApplyInto = func(_ *realBackend, _ []string, _ lifecyclePolicy) (lifecycleResult, error) {
			return lifecycleResult{Backups: []string{"/tmp/config.bak"}}, nil
		}
		raw, code := captureApplyJSON(t, []string{"HashKnownHosts"}, sshApplyFlags{Yes: true, JSON: true}, false, false)
		var doc sshApplyDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if doc.Advisories == nil {
			t.Fatal("advisories must be present (empty array), not omitted")
		}
		if len(doc.Advisories) != 0 {
			t.Errorf("advisories = %v, want empty", doc.Advisories)
		}
		if code != 0 {
			t.Errorf("clean success exit = %d, want 0", code)
		}
	})

	t.Run("advisory list on shadowed success", func(t *testing.T) {
		cliGlobalSSHApplyInto = func(_ *realBackend, _ []string, _ lifecyclePolicy) (lifecycleResult, error) {
			return lifecycleResult{
				Backups:    []string{"/tmp/config.bak"},
				Advisories: []string{"advisory: HashKnownHosts was applied but is still shadowed"},
			}, nil
		}
		raw, code := captureApplyJSON(t, []string{"HashKnownHosts"}, sshApplyFlags{Yes: true, JSON: true}, false, false)
		var doc sshApplyDocument
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(doc.Advisories) == 0 {
			t.Fatal("shadowed success must carry the advisory list")
		}
		if code != 0 {
			t.Errorf("advisory success default exit = %d, want 0", code)
		}
	})
}

func TestSSHExitCodeEqualsEnvelopeForEveryRow(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
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
		{"success-with-advisory", lifecycleResult{Backups: []string{"b"}, Advisories: []string{"shadow"}}, nil, false, false, 0},
		{"fail-on-advisory", lifecycleResult{Backups: []string{"b"}, Advisories: []string{"shadow"}}, nil, true, false, 3},
		{"usage-refusal", lifecycleResult{}, fmt.Errorf("unknown"), false, false, 1},
		{"rolled-back", lifecycleResult{Restored: []string{"r"}}, fmt.Errorf("write failed"), false, false, 2},
		{"dry-run-advisory-stays-zero", lifecycleResult{Advisories: []string{"shadow"}}, nil, true, true, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sshWriteExitCode(tc.res, tc.err, tc.failOnAdvisory, tc.dryRun)
			if got != tc.want {
				t.Errorf("sshWriteExitCode = %d, want %d", got, tc.want)
			}
			cliGlobalSSHApplyInto = func(_ *realBackend, _ []string, _ lifecyclePolicy) (lifecycleResult, error) {
				return tc.res, tc.err
			}
			raw, code := captureApplyJSON(t, []string{"HashKnownHosts"}, sshApplyFlags{
				Yes:            true,
				JSON:           true,
				FailOnAdvisory: tc.failOnAdvisory,
				DryRun:         tc.dryRun,
			}, false, false)
			var doc sshApplyDocument
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

func TestSSHApplyDryRunJSONLeavesBytesUnchanged(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	configPath := filepath.Join(sshDir, "config")
	original := []byte("Host example\n  User git\n")
	if err := os.WriteFile(configPath, original, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	cmd, out, _ := cliTestCmd()
	err := runSSHOptionsApply(cmd, []string{"HashKnownHosts"}, sshApplyFlags{DryRun: true, JSON: true}, false, false)
	if exitStatusOf(err) != 0 {
		t.Fatalf("dry-run --json exit = %v, want 0", err)
	}
	after, rerr := os.ReadFile(configPath) //nolint:gosec // test sandbox path
	if rerr != nil {
		t.Fatalf("reread: %v", rerr)
	}
	if !bytes.Equal(after, original) {
		t.Errorf("dry-run mutated config:\nbefore=%q\nafter=%q", original, after)
	}
	var doc sshApplyDocument
	if uerr := json.Unmarshal(out.Bytes(), &doc); uerr != nil {
		t.Fatalf("unmarshal: %v\n%s", uerr, out.Bytes())
	}
	if !doc.DryRun {
		t.Error("dry_run marker must be true")
	}
	assertExactKeysFrom(t, out.Bytes(), sshApplyDocKeys)
}

func TestSSHApplyRefusesPerAliasOption(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cmd, out, _ := cliTestCmd()
	err := runSSHOptionsApply(cmd, []string{"IdentitiesOnly"}, sshApplyFlags{Yes: true, JSON: true}, false, false)
	if exitStatusOf(err) != 1 {
		t.Fatalf("per-alias apply exit = %v, want 1", err)
	}
	if err == nil || !strings.Contains(err.Error(), "IdentitiesOnly") {
		t.Errorf("error = %v, want it to name IdentitiesOnly", err)
	}
	assertApplyEnvelope(t, out.Bytes(), 1, false)
}

func TestSSHApplyRefusesVersionUnverified(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seamGuard(t)
	cliGlobalSSHApplyInto = func(b *realBackend, keys []string, p lifecyclePolicy) (lifecycleResult, error) {
		b.probeSSHVersion = func() (platform.SSHVersion, error) { return platform.SSHVersion{}, nil }
		return b.runGlobalSSHApply(keys, p)
	}
	cmd, out, _ := cliTestCmd()
	err := runSSHOptionsApply(cmd, []string{"StrictHostKeyChecking"}, sshApplyFlags{Yes: true, JSON: true}, false, false)
	if exitStatusOf(err) != 1 {
		t.Fatalf("version-unverified apply exit = %v, want 1", err)
	}
	if err == nil || !strings.Contains(err.Error(), "ssh -V") {
		t.Errorf("error = %v, want it to name ssh -V", err)
	}
	assertApplyEnvelope(t, out.Bytes(), 1, false)
}

func TestSSHApplyRefusesUnknownKey(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cmd, _, _ := cliTestCmd()
	err := runSSHOptionsApply(cmd, []string{"TotallyFake"}, sshApplyFlags{Yes: true}, false, false)
	if exitStatusOf(err) != 1 {
		t.Fatalf("unknown key exit = %v, want 1", err)
	}
	if err == nil || !strings.Contains(err.Error(), "TotallyFake") {
		t.Errorf("error = %v, want it to name the unknown key", err)
	}
}

func TestSSHApplyDryRunPrintsPreviewAndExitsZero(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	configPath := filepath.Join(sshDir, "config")
	original := []byte("Host example\n  User git\n")
	if err := os.WriteFile(configPath, original, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	cmd, out, _ := cliTestCmd()
	err := runSSHOptionsApply(cmd, []string{"HashKnownHosts"}, sshApplyFlags{DryRun: true}, false, false)
	if err != nil {
		t.Fatalf("dry-run must exit zero: %v", err)
	}
	after, rerr := os.ReadFile(configPath) //nolint:gosec // test sandbox path
	if rerr != nil {
		t.Fatalf("reread: %v", rerr)
	}
	if !bytes.Equal(after, original) {
		t.Error("dry-run mutated the configuration file")
	}
	if !strings.Contains(out.String(), "dry run") {
		t.Errorf("dry-run output missing preview:\n%s", out.String())
	}
}

func TestSSHApplyOffTerminalWithoutYesRefuses(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seamGuard(t)
	var calls int
	cliGlobalSSHApplyInto = func(_ *realBackend, _ []string, _ lifecyclePolicy) (lifecycleResult, error) {
		calls++
		return lifecycleResult{}, nil
	}
	cmd, out, _ := cliTestCmd()
	err := runSSHOptionsApply(cmd, []string{"HashKnownHosts"}, sshApplyFlags{JSON: true}, false, false)
	if exitStatusOf(err) != 1 {
		t.Fatalf("unauthorized apply exit = %v, want 1", err)
	}
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Errorf("error = %v, want it to name --yes", err)
	}
	if calls != 0 {
		t.Errorf("lifecycle invoked %d times on unauthorized apply, want 0", calls)
	}
	assertApplyEnvelope(t, out.Bytes(), 1, false)
}

func TestSSHDepthResolverTable(t *testing.T) {
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
		{"options apply", sshApplyRequiredKey},
		{"storage migrate", sshMigrateRequired},
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

func TestSSHIncompleteBothTTYsOpensEmptyTUI(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seamGuard(t)

	t.Run("apply opens Options empty", func(t *testing.T) {
		var launched bool
		var storageTab bool
		sshTUILaunch = func(b *realBackend, storage bool) error {
			launched = true
			storageTab = storage
			app := tuikit.NewAppOnGlobalSSH(b, storage)
			if app.ActiveTab() != tuikit.TabGlobalSSH {
				t.Errorf("tab = %v, want TabGlobalSSH", app.ActiveTab())
			}
			st, chosen, _ := app.GlobalSSHUIState()
			if st {
				t.Error("apply fallback must open the Options sub-tab")
			}
			if chosen != 0 {
				t.Errorf("option selection = %d, want empty", chosen)
			}
			return nil
		}
		cmd, _, _ := cliTestCmd()
		if err := runSSHOptionsApply(cmd, nil, sshApplyFlags{}, true, true); err != nil {
			t.Fatalf("TUI fallback: %v", err)
		}
		if !launched {
			t.Fatal("incomplete apply with both TTYs must launch the TUI")
		}
		if storageTab {
			t.Error("apply fallback must pass storageTab=false")
		}
	})

	t.Run("migrate opens Storage radio on current", func(t *testing.T) {
		var launched bool
		sshTUILaunch = func(b *realBackend, storage bool) error {
			launched = true
			if !storage {
				t.Error("migrate fallback must pass storageTab=true")
			}
			app := tuikit.NewAppOnGlobalSSH(b, storage)
			if app.ActiveTab() != tuikit.TabGlobalSSH {
				t.Errorf("tab = %v, want TabGlobalSSH", app.ActiveTab())
			}
			st, chosen, radio := app.GlobalSSHUIState()
			if !st {
				t.Error("migrate fallback must open the Storage sub-tab")
			}
			if chosen != 0 {
				t.Errorf("option selection = %d, want empty", chosen)
			}
			if radio != tuikit.StorageInclude && radio != tuikit.StorageSentinel {
				t.Errorf("storage radio = %q, want a current-layout sentinel", radio)
			}
			return nil
		}
		cmd, _, _ := cliTestCmd()
		if err := runSSHStorageMigrateVerb(cmd, sshMigrateFlags{}, true, true); err != nil {
			t.Fatalf("TUI fallback: %v", err)
		}
		if !launched {
			t.Fatal("incomplete migrate with both TTYs must launch the TUI")
		}
	})
}

func TestSSHWriteVerbsCallSharedCeremonyByConstruction(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	seamGuard(t)

	var applyCalls int
	cliGlobalSSHApplyInto = func(_ *realBackend, keys []string, p lifecyclePolicy) (lifecycleResult, error) {
		applyCalls++
		if len(keys) != 1 || keys[0] != "HashKnownHosts" {
			t.Errorf("apply keys = %v, want [HashKnownHosts]", keys)
		}
		if p.DryRun {
			t.Error("headless --yes apply must not set DryRun")
		}
		return lifecycleResult{Backups: []string{"bak"}}, nil
	}
	cmd, _, _ := cliTestCmd()
	if err := runSSHOptionsApply(cmd, []string{"HashKnownHosts"}, sshApplyFlags{Yes: true}, false, false); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if applyCalls != 1 {
		t.Errorf("apply ceremony invoked %d times, want 1", applyCalls)
	}

	var migrateCalls int
	cliSSHStorageMigrateInto = func(_ *realBackend, target tuikit.SSHStorageLayout, _ lifecyclePolicy) (lifecycleResult, error) {
		migrateCalls++
		if target != tuikit.StorageSentinel {
			t.Errorf("migrate target = %q, want StorageSentinel", target)
		}
		return lifecycleResult{Backups: []string{"bak"}}, nil
	}
	cmd, _, _ = cliTestCmd()
	if err := runSSHStorageMigrateVerb(cmd, sshMigrateFlags{To: sshLayoutInFile, Yes: true}, false, false); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if migrateCalls != 1 {
		t.Errorf("migrate ceremony invoked %d times, want 1", migrateCalls)
	}

	src, err := os.ReadFile(filepath.Join(testRepoRoot(t), "cmd", "gitid", "ssh.go")) //nolint:gosec // repository source
	if err != nil {
		t.Fatalf("read ssh.go: %v", err)
	}
	body := string(src)
	if !strings.Contains(body, "cliGlobalSSHApplyInto") || !strings.Contains(body, "b.runGlobalSSHApply") {
		t.Error("ssh.go must call runGlobalSSHApply through cliGlobalSSHApplyInto")
	}
	if !strings.Contains(body, "cliSSHStorageMigrateInto") || !strings.Contains(body, "b.runSSHStorageMigrate") {
		t.Error("ssh.go must call runSSHStorageMigrate through cliSSHStorageMigrateInto")
	}
	if strings.Contains(body, "CommitGlobalSSH") || strings.Contains(body, "CommitSSHStorage") {
		t.Error("CLI verbs must not invoke a tea.Cmd commit seam")
	}
}

func TestSSHWriteVerbsUseSharedDepthHelpersByConstruction(t *testing.T) {
	src, err := os.ReadFile(filepath.Join(testRepoRoot(t), "cmd", "gitid", "ssh.go")) //nolint:gosec // repository source
	if err != nil {
		t.Fatalf("read ssh.go: %v", err)
	}
	body := string(src)
	if !strings.Contains(body, "depthResolver{") {
		t.Error("ssh.go must resolve adaptive depth through depthResolver")
	}
	if !strings.Contains(body, "confirmationPolicyFrom(") {
		t.Error("ssh.go must map --yes through confirmationPolicyFrom")
	}
	for _, fn := range []string{"func runSSHOptionsApply", "func runSSHStorageMigrateVerb"} {
		start := strings.Index(body, fn)
		if start < 0 {
			t.Errorf("ssh.go missing %s", fn)
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

func TestSSHCompletionStillGenerates(t *testing.T) {
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"__complete", "ssh", ""})
	if err := root.Execute(); err != nil {
		t.Fatalf("__complete ssh: %v", err)
	}
	got := buf.String()
	for _, want := range []string{"options", "storage"} {
		if !strings.Contains(got, want) {
			t.Errorf("ssh completion missing %q:\n%s", want, got)
		}
	}
}

func captureApplyJSON(t *testing.T, keys []string, flags sshApplyFlags, stdin, stdout bool) ([]byte, int) {
	t.Helper()
	cmd, out, _ := cliTestCmd()
	err := runSSHOptionsApply(cmd, keys, flags, stdin, stdout)
	return out.Bytes(), exitStatusOf(err)
}

func assertApplyEnvelope(t *testing.T, raw []byte, processCode int, dryRun bool) {
	t.Helper()
	assertExactKeysFrom(t, raw, sshApplyDocKeys)
	var doc sshApplyDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal apply envelope: %v\n%s", err, raw)
	}
	if doc.Schema != sshApplySchema {
		t.Errorf("schema = %q, want %q", doc.Schema, sshApplySchema)
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

func assertMigrateEnvelope(t *testing.T, raw []byte, processCode int, dryRun bool) {
	t.Helper()
	assertExactKeysFrom(t, raw, sshMigrateDocKeys)
	var doc sshMigrateDocument
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal migrate envelope: %v\n%s", err, raw)
	}
	if doc.Schema != sshMigrateSchema {
		t.Errorf("schema = %q, want %q", doc.Schema, sshMigrateSchema)
	}
	if doc.DryRun != dryRun {
		t.Errorf("dry_run = %v, want %v", doc.DryRun, dryRun)
	}
	if doc.ExitCode != processCode {
		t.Errorf("exit_code %d != process status %d", doc.ExitCode, processCode)
	}
}

func assertExactKeysFrom(t *testing.T, raw []byte, want []string) {
	t.Helper()
	keys, err := jsonObjectKeys(raw)
	if err != nil {
		t.Fatalf("jsonObjectKeys: %v\n%s", err, raw)
	}
	assertExactKeys(t, keys, want)
}

func assertExactKeys(t *testing.T, got, want []string) {
	t.Helper()
	g, w := append([]string{}, got...), append([]string{}, want...)
	sort.Strings(g)
	sort.Strings(w)
	if !reflect.DeepEqual(g, w) {
		t.Errorf("key set = %v, want exactly %v", g, w)
	}
}

func assertEnumMember(t *testing.T, field, value string, allowed []string) {
	t.Helper()
	for _, a := range allowed {
		if value == a {
			return
		}
	}
	t.Errorf("%s = %q is not a documented enum member %v", field, value, allowed)
}

func TestSSHFinishWrapsNonZero(t *testing.T) {
	if err := sshFinish(0, errors.New("ignored")); err != nil {
		t.Errorf("zero code must return nil, got %v", err)
	}
	err := sshFinish(2, errors.New("rolled back"))
	if exitStatusOf(err) != 2 {
		t.Errorf("exitStatusOf = %d, want 2", exitStatusOf(err))
	}
}
