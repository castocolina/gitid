package dummytui

import (
	"os/exec"
	"strings"
	"testing"
)

// TestNoBackendAllowlist proves DLV-05 (review HIGH-1): the demo import
// graph — internal/dummytui, cmd/gitid-dummy, and the shared render stack
// internal/tuikit — imports NO first-party github.com/castocolina/gitid/...
// package other than exactly those three.
//
// This is an ALLOWLIST, not a denylist — strictly stronger, because it fails
// on ANY new or renamed backend package by construction, rather than
// requiring this test to be kept in sync with an enumerated list of
// forbidden packages (internal/identity, keygen, sshconfig, gitconfig,
// filewriter, tester, doctor, adopter, uploader, ...).
//
// Phase 3 (D-17) added the third member: internal/tuikit is the extracted,
// backend-free render stack both binaries draw through. It talks to the
// outside world ONLY via the tuikit.Backend seam, and speaks view DTOs
// (views.go) rather than backend types. If a later phase needs a backend
// value inside tuikit, the ONLY sanctioned answer is a new tuikit-local view
// DTO plus a conversion in cmd/gitid/wiring.go — NEVER a new allowlist
// member. Widening this allowlist is the thing the test exists to prevent.
func TestNoBackendAllowlist(t *testing.T) {
	const modulePrefix = "github.com/castocolina/gitid/"
	allowed := map[string]bool{
		"github.com/castocolina/gitid/internal/dummytui": true,
		"github.com/castocolina/gitid/cmd/gitid-dummy":   true,
		"github.com/castocolina/gitid/internal/tuikit":   true,
	}

	// #nosec G204 -- arg-slice exec.Command, no shell interpolation; all
	// target patterns are fixed literals, never external/user input.
	cmd := exec.Command("go", "list", "-deps",
		"./cmd/gitid-dummy/...", "./internal/dummytui/...", "./internal/tuikit/...")
	cmd.Dir = repoRootForAllowlistTest(t)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps failed: %v\n%s", err, out)
	}

	var offenders []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, modulePrefix) {
			continue // third-party or stdlib — not first-party, not our concern
		}
		if allowed[line] {
			continue
		}
		offenders = append(offenders, line)
	}

	if len(offenders) > 0 {
		t.Fatalf("demo ALLOWLIST violated — first-party package(s) outside {dummytui, gitid-dummy, tuikit}:\n%s",
			strings.Join(offenders, "\n"))
	}
}

// repoRootForAllowlistTest locates the module root so `go list` resolves the
// ./cmd/gitid-dummy/..., ./internal/dummytui/... and ./internal/tuikit/...
// relative patterns correctly regardless of the working directory `go test`
// was invoked from.
func repoRootForAllowlistTest(t *testing.T) string {
	t.Helper()
	// #nosec G204 -- arg-slice exec.Command, fixed literal args, no shell.
	out, err := exec.Command("go", "env", "GOMOD").Output()
	if err != nil {
		t.Fatalf("go env GOMOD failed: %v", err)
	}
	gomod := strings.TrimSpace(string(out))
	gomod = strings.TrimSuffix(gomod, "go.mod")
	gomod = strings.TrimSuffix(gomod, "/")
	if gomod == "" {
		t.Fatal("go env GOMOD returned an empty path")
	}
	return gomod
}
