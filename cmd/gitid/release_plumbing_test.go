package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func workflowPath(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", ".github", "workflows", "ci.yml")
}

func makefilePath(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "Makefile")
}

func readRepoFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		t.Fatalf("cannot read %s: %v", path, err)
	}
	return string(raw)
}

func jobBlock(t *testing.T, src, name string) string {
	t.Helper()
	lines := strings.Split(src, "\n")
	start := -1
	want := "  " + name + ":"
	for i, line := range lines {
		if strings.TrimRight(line, " \t") == want {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatalf("job %q not found", name)
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		line := lines[i]
		if strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "   ") && strings.HasSuffix(strings.TrimRight(line, " \t"), ":") {
			end = i
			break
		}
	}
	return strings.Join(lines[start:end], "\n")
}

func TestWorkflowPinsEveryActionToACommitSHA(t *testing.T) {
	src := readRepoFile(t, workflowPath(t))
	re := regexp.MustCompile(`^uses:\s+[^@]+@[0-9a-f]{40} # v`)
	var uses int
	for i, line := range strings.Split(src, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "uses: ") {
			continue
		}
		uses++
		if !re.MatchString(trimmed) {
			t.Errorf("line %d: uses: is not pinned to a 40-hex SHA with # v comment: %s", i+1, trimmed)
		}
	}
	if uses == 0 {
		t.Fatal("no uses: lines found")
	}
}

func TestWorkflowTopLevelPermissionsStayReadOnly(t *testing.T) {
	src := readRepoFile(t, workflowPath(t))
	lines := strings.Split(src, "\n")
	count := 0
	for i, line := range lines {
		if line == "permissions:" {
			count++
			if i+1 >= len(lines) || lines[i+1] != "  contents: read" {
				t.Fatalf("top-level permissions: is not followed by contents: read")
			}
		}
	}
	if count != 1 {
		t.Fatalf("found %d column-0 permissions: keys, want 1", count)
	}
}

// TestLdflagsSymbolsAreDeclaredInVersionPackage locks the Makefile's -X path
// to internal/version's exact unexported var names (Phase 10, D-09): the
// three linker-injectable identifiers moved from cmd/gitid/main.go into
// internal/version/version.go, and the Makefile's LDFLAGS retargeted from
// `-X main.<name>=` to `-X github.com/castocolina/gitid/internal/version.
// <name>=` accordingly. Renamed from TestLdflagsSymbolsAreDeclaredInMain.
func TestLdflagsSymbolsAreDeclaredInVersionPackage(t *testing.T) {
	makefile := readRepoFile(t, makefilePath(t))
	versionSrc := readRepoFile(t, filepath.Join("..", "..", "internal", "version", "version.go"))
	re := regexp.MustCompile(`-X github\.com/castocolina/gitid/internal/version\.([A-Za-z_][A-Za-z0-9_]*)=`)
	matches := re.FindAllStringSubmatch(makefile, -1)
	if len(matches) == 0 {
		t.Fatal("Makefile LDFLAGS has no -X github.com/castocolina/gitid/internal/version.<name>=")
	}
	seen := map[string]bool{}
	for _, m := range matches {
		name := m[1]
		if seen[name] {
			continue
		}
		seen[name] = true
		// No `= ` requirement: internal/version's vars are declared with NO
		// literal default (a grouped `var (name string)` block, D-09) so a
		// non-empty compiled-in value never masks the debug.ReadBuildInfo()
		// fallback — just assert the name is declared at package level.
		decl := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(name) + `\s`)
		if !decl.MatchString(versionSrc) {
			t.Errorf("-X github.com/castocolina/gitid/internal/version.%s= has no matching package-level %s declaration in internal/version/version.go", name, name)
		}
	}
}

func TestBuildCrossStampsEveryTarget(t *testing.T) {
	makefile := readRepoFile(t, makefilePath(t))
	idx := strings.Index(makefile, "\nbuild-cross:")
	if idx < 0 {
		t.Fatal("Makefile missing build-cross target")
	}
	rest := makefile[idx+1:]
	end := strings.Index(rest, "\n## ")
	if end < 0 {
		end = len(rest)
	}
	body := rest[:end]
	var builds int
	for _, line := range strings.Split(body, "\n") {
		if !strings.Contains(line, "go build") {
			continue
		}
		builds++
		if !strings.Contains(line, `-ldflags "$(LDFLAGS)"`) {
			t.Errorf("build-cross go build missing -ldflags \"$(LDFLAGS)\": %s", strings.TrimSpace(line))
		}
	}
	if builds < 4 {
		t.Fatalf("build-cross has %d go build lines, want at least 4", builds)
	}
}

// TestSetupEnvTargetsNeverInstallUnpinnedGosec is the missing regression
// guard round 3's IN-01/IN-03 findings named: `setup-env-release`'s
// standalone `gosec@latest` install was removed as dead weight (round 2
// WR-01, round 3 commit) — golangci-lint's own embedded gosec linter
// (.golangci.yml) is the real coverage `make lint` uses — but nothing
// asserted this, which is exactly how the identical line survived
// untouched in the full `setup-env` target until round 3 caught it. This
// test extracts each target's own recipe body (bounded by the next `\n## `
// doc-comment header, mirroring TestBuildCrossStampsEveryTarget's own
// extraction idiom) and fails if either recipe still shells out to a
// standalone `gosec` binary.
func TestSetupEnvTargetsNeverInstallUnpinnedGosec(t *testing.T) {
	makefile := readRepoFile(t, makefilePath(t))
	for _, target := range []string{"setup-env", "setup-env-release"} {
		marker := "\n" + target + ":\n"
		idx := strings.Index(makefile, marker)
		if idx < 0 {
			t.Fatalf("Makefile missing target %q", target)
		}
		rest := makefile[idx+len(marker):]
		end := strings.Index(rest, "\n## ")
		if end < 0 {
			end = len(rest)
		}
		body := rest[:end]
		if strings.Contains(body, "cmd/gosec") {
			t.Errorf("target %q installs a standalone gosec binary — golangci-lint's embedded gosec linter is the real coverage `make lint` uses; a standalone install is unpinned (@latest) dead weight (REVIEW round-2/round-3 WR-01):\n%s", target, body)
		}
	}
}
