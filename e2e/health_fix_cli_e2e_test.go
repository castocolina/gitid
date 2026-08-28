//go:build e2e

package e2e

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

type healthDoc struct {
	Schema   string `json:"schema"`
	Findings []struct {
		Identity string `json:"identity"`
		Title    string `json:"title"`
	} `json:"findings"`
}

func runHealthCLI(t *testing.T, ctx context.Context, bin, home string, args ...string) ([]byte, int) {
	t.Helper()
	var out, errb bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, args...) //nolint:gosec // bin from BuildBinary; fixed test literals
	cmd.Stdout = &out
	cmd.Stderr = &errb
	cmd.Env = append(os.Environ(), "HOME="+home)
	err := cmd.Run()
	if err == nil {
		return out.Bytes(), 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return out.Bytes(), exitErr.ExitCode()
	}
	t.Fatalf("gitid %s: %v\nstdout:\n%s\nstderr:\n%s", strings.Join(args, " "), err, out.String(), errb.String())
	return nil, -1
}

func TestHealthFixCLIParity(t *testing.T) {
	bin := BuildBinary(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	home := t.TempDir()
	sshDir := filepath.Join(home, ".ssh")
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		t.Fatalf("creating .ssh: %v", err)
	}
	config := filepath.Join(sshDir, "config")
	fixture := "Host work.example.com\n\tHostName example.com\n\tIdentityFile " + filepath.Join(sshDir, "id_work") + "\n\tIdentitiesOnly no\n"
	if err := os.WriteFile(config, []byte(fixture), 0o600); err != nil {
		t.Fatalf("writing fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(sshDir, "id_work"), []byte("not-a-real-key"), 0o600); err != nil {
		t.Fatalf("writing key fixture: %v", err)
	}

	jsonOut, code := runHealthCLI(t, ctx, bin, home, "health", "--json")
	if code != 0 {
		t.Fatalf("health --json exit = %d, want 0\n%s", code, jsonOut)
	}
	var doc healthDoc
	if err := json.Unmarshal(jsonOut, &doc); err != nil {
		t.Fatalf("decoding health envelope: %v\n%s", err, jsonOut)
	}
	if doc.Schema != "gitid.health/v1" || len(doc.Findings) == 0 {
		t.Fatalf("health envelope = %+v", doc)
	}

	scoped, _ := runHealthCLI(t, ctx, bin, home, "health", "--identity", "work", "--json")
	var scopedDoc healthDoc
	if err := json.Unmarshal(scoped, &scopedDoc); err != nil {
		t.Fatalf("decoding scoped health envelope: %v\n%s", err, scoped)
	}
	for _, finding := range scopedDoc.Findings {
		if finding.Identity != "work" {
			t.Errorf("scoped finding has identity %q, want work: %+v", finding.Identity, finding)
		}
	}

	before, err := os.ReadFile(config)
	if err != nil {
		t.Fatalf("reading fixture before dry-run: %v", err)
	}
	if _, code := runHealthCLI(t, ctx, bin, home, "fix", "--dry-run"); code == 0 {
		t.Fatal("fix --dry-run exit = 0 despite outstanding findings")
	}
	afterDryRun, err := os.ReadFile(config)
	if err != nil {
		t.Fatalf("reading fixture after dry-run: %v", err)
	}
	if string(before) != string(afterDryRun) {
		t.Fatal("fix --dry-run wrote the SSH config")
	}

	doctorOut, doctorCode := runHealthCLI(t, ctx, bin, home, "doctor", "--json")
	if string(doctorOut) != string(jsonOut) || doctorCode != code {
		t.Errorf("doctor differs from health: code %d/%d\ndoctor=%s\nhealth=%s", doctorCode, code, doctorOut, jsonOut)
	}
	_, _ = runHealthCLI(t, ctx, bin, home, "doctor", "--fix", "--yes")
	fixed, err := os.ReadFile(config)
	if err != nil {
		t.Fatalf("reading fixture after doctor fix: %v", err)
	}
	if !strings.Contains(string(fixed), "IdentitiesOnly yes") {
		t.Fatalf("doctor --fix --yes did not apply fixer rewrite:\n%s", fixed)
	}
}
