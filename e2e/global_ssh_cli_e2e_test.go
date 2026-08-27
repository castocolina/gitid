//go:build e2e

package e2e

// global_ssh_cli_e2e_test.go drives the compiled gitid binary's ssh verbs
// headlessly (no PTY): frozen JSON envelopes, apply + idempotence + backups,
// and the advisory exit-status contract.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type sshOptionsDoc struct {
	Schema  string            `json:"schema"`
	Options []sshOptionRecord `json:"options"`
}

type sshOptionRecord struct {
	Key                 string `json:"key"`
	CurrentValue        string `json:"current_value"`
	RecommendedValue    string `json:"recommended_value"`
	Risk                string `json:"risk"`
	Scope               string `json:"scope"`
	State               string `json:"state"`
	Source              string `json:"source"`
	SourceFile          string `json:"source_file"`
	SourceLine          int    `json:"source_line"`
	NotApplicableReason string `json:"not_applicable_reason"`
	VersionNote         string `json:"version_note"`
	ProbeError          string `json:"probe_error"`
}

type sshStorageDoc struct {
	Schema             string `json:"schema"`
	Layout             string `json:"layout"`
	TargetPath         string `json:"target_path"`
	MainConfigPath     string `json:"main_config_path"`
	IncludeLinePresent bool   `json:"include_line_present"`
}

type sshApplyDoc struct {
	Schema                 string   `json:"schema"`
	DryRun                 bool     `json:"dry_run"`
	Applied                []string `json:"applied"`
	Declined               []string `json:"declined"`
	TargetPath             string   `json:"target_path"`
	Backups                []string `json:"backups"`
	Restored               []string `json:"restored"`
	Advisories             []string `json:"advisories"`
	SimulationInconclusive bool     `json:"simulation_inconclusive"`
	SimulationNote         string   `json:"simulation_note"`
	Error                  string   `json:"error"`
	ExitCode               int      `json:"exit_code"`
}

func runSSHCLI(t *testing.T, ctx context.Context, bin, home, fakeSSH string, args ...string) (stdout []byte, code int) {
	t.Helper()
	var out, errb bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, args...) //nolint:gosec // bin from BuildBinary; fixed test literals
	cmd.Stdout = &out
	cmd.Stderr = &errb
	env := append(os.Environ(), "HOME="+home)
	if fakeSSH != "" {
		env = append(env, "PATH="+fakeSSH+":"+os.Getenv("PATH"))
	}
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

func countBackups(t *testing.T, home string) int {
	t.Helper()
	n := 0
	err := filepath.Walk(home, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.Contains(filepath.Base(path), ".bak.") {
			n++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking home for backups: %v", err)
	}
	return n
}

func TestGlobalSSHCLI_ListShowApplyIdempotent(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)
	fake := FakeSSHDir(t, "globalssh")
	seedGlobalSSHHome(t, home, "none")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	listOut, listCode := runSSHCLI(t, ctx, bin, home, fake, "ssh", "options", "list", "--json")
	if listCode != 0 {
		t.Fatalf("options list --json exit = %d\n%s", listCode, listOut)
	}
	var list sshOptionsDoc
	if err := json.Unmarshal(listOut, &list); err != nil {
		t.Fatalf("list unmarshal: %v\n%s", err, listOut)
	}
	if list.Schema != "gitid.ssh.options/v1" {
		t.Errorf("list schema = %q", list.Schema)
	}
	wantKeys := []string{"StrictHostKeyChecking", "ForwardAgent", "HashKnownHosts", "IdentitiesOnly", "AddKeysToAgent", "UseKeychain"}
	if len(list.Options) != len(wantKeys) {
		t.Fatalf("options = %d, want %d", len(list.Options), len(wantKeys))
	}
	for i, k := range wantKeys {
		if list.Options[i].Key != k {
			t.Errorf("options[%d].key = %q, want %q", i, list.Options[i].Key, k)
		}
	}

	showOut, showCode := runSSHCLI(t, ctx, bin, home, fake, "ssh", "storage", "show", "--json")
	if showCode != 0 {
		t.Fatalf("storage show --json exit = %d\n%s", showCode, showOut)
	}
	var stor sshStorageDoc
	if err := json.Unmarshal(showOut, &stor); err != nil {
		t.Fatalf("show unmarshal: %v\n%s", err, showOut)
	}
	if stor.Schema != "gitid.ssh.storage/v1" {
		t.Errorf("storage schema = %q", stor.Schema)
	}
	if stor.Layout != "include" {
		t.Errorf("layout = %q, want include", stor.Layout)
	}

	target := filepath.Join(home, ".ssh", "config.d", "gitid.config")
	apply1, code1 := runSSHCLI(t, ctx, bin, home, fake, "ssh", "options", "apply", "HashKnownHosts", "--yes", "--json")
	if code1 != 0 {
		t.Fatalf("first apply exit = %d\n%s", code1, apply1)
	}
	var env1 sshApplyDoc
	if err := json.Unmarshal(apply1, &env1); err != nil {
		t.Fatalf("apply unmarshal: %v\n%s", err, apply1)
	}
	if env1.Schema != "gitid.ssh.apply/v1" || env1.ExitCode != 0 {
		t.Errorf("apply envelope schema=%q exit_code=%d", env1.Schema, env1.ExitCode)
	}
	after1, err := os.ReadFile(target) //nolint:gosec // sandbox path
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if !bytes.Contains(after1, []byte("HashKnownHosts yes")) {
		t.Errorf("managed block missing HashKnownHosts yes:\n%s", after1)
	}
	if countBackups(t, home) < 1 {
		t.Fatal("first apply must take a timestamped backup")
	}

	apply2, code2 := runSSHCLI(t, ctx, bin, home, fake, "ssh", "options", "apply", "HashKnownHosts", "--yes", "--json")
	if code2 != 0 {
		t.Fatalf("second apply exit = %d\n%s", code2, apply2)
	}
	after2, err := os.ReadFile(target) //nolint:gosec // sandbox path
	if err != nil {
		t.Fatalf("reread target: %v", err)
	}
	if !bytes.Equal(after1, after2) {
		t.Errorf("second apply mutated config:\nfirst:\n%s\nsecond:\n%s", after1, after2)
	}
	if countBackups(t, home) < 2 {
		t.Fatal("second apply must record a second timestamped backup")
	}
}

func TestGlobalSSHCLI_AdvisoryExitCodes(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)
	fake := FakeSSHDir(t, "globalssh")
	seedGlobalSSHHome(t, home, "above")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	out, code := runSSHCLI(t, ctx, bin, home, fake, "ssh", "options", "apply", "StrictHostKeyChecking", "--yes", "--json")
	if code != 0 {
		t.Fatalf("advisory apply default exit = %d, want 0\n%s", code, out)
	}
	var env sshApplyDoc
	if err := json.Unmarshal(out, &env); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if env.ExitCode != code {
		t.Errorf("envelope exit_code %d != process %d", env.ExitCode, code)
	}
	if len(env.Advisories) == 0 {
		t.Fatalf("shadowed apply must carry advisories; envelope: %+v", env)
	}

	home2 := SandboxHome(t)
	seedGlobalSSHHome(t, home2, "above")
	out3, code3 := runSSHCLI(t, ctx, bin, home2, fake, "ssh", "options", "apply", "StrictHostKeyChecking", "--yes", "--fail-on-advisory", "--json")
	if code3 != 3 {
		t.Fatalf("--fail-on-advisory exit = %d, want 3\n%s", code3, out3)
	}
	var env3 sshApplyDoc
	if err := json.Unmarshal(out3, &env3); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out3)
	}
	if env3.ExitCode != 3 {
		t.Errorf("envelope exit_code = %d, want 3", env3.ExitCode)
	}
}

func TestGlobalSSHCLI_DryRunNoWrite(t *testing.T) {
	home := SandboxHome(t)
	bin := BuildBinary(t)
	fake := FakeSSHDir(t, "globalssh")
	seedGlobalSSHHome(t, home, "none")
	target := filepath.Join(home, ".ssh", "config.d", "gitid.config")
	before, err := os.ReadFile(target) //nolint:gosec // sandbox path
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, code := runSSHCLI(t, ctx, bin, home, fake, "ssh", "options", "apply", "HashKnownHosts", "--dry-run", "--json")
	if code != 0 {
		t.Fatalf("dry-run exit = %d\n%s", code, out)
	}
	after, err := os.ReadFile(target) //nolint:gosec // sandbox path
	if err != nil {
		t.Fatalf("reread: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Error("dry-run mutated the configuration")
	}
	var env sshApplyDoc
	if uerr := json.Unmarshal(out, &env); uerr != nil {
		t.Fatalf("unmarshal: %v\n%s", uerr, out)
	}
	if !env.DryRun {
		t.Error("dry_run marker must be true")
	}
	if countBackups(t, home) != 0 {
		t.Error("dry-run must not take a backup")
	}
}
