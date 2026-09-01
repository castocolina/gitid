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

func TestWorkflowReleaseJobIsTagScoped(t *testing.T) {
	src := readRepoFile(t, workflowPath(t))
	block := jobBlock(t, src, "release")
	if !strings.Contains(block, "if: startsWith(github.ref, 'refs/tags/v')") {
		t.Fatal("release job missing tag-scoped if:")
	}
	if !strings.Contains(src, `tags: ["v*"]`) && !strings.Contains(src, `tags: ['v*']`) {
		if !regexp.MustCompile(`(?m)^\s+tags:\s*\n\s+- ["']v\*["']`).MatchString(src) &&
			!strings.Contains(src, `"v*"`) {
			t.Fatal(`on: push: is missing a quoted tags glob "v*"`)
		}
	}
}

func TestWorkflowReleaseJobHasScopedWritePermission(t *testing.T) {
	src := readRepoFile(t, workflowPath(t))
	release := jobBlock(t, src, "release")
	if !regexp.MustCompile(`(?m)^\s+permissions:\s*$`).MatchString(release) {
		t.Fatal("release job missing permissions:")
	}
	if !strings.Contains(release, "contents: write") {
		t.Fatal("release job missing contents: write")
	}
	for _, name := range []string{"build-cross", "check", "test-e2e"} {
		block := jobBlock(t, src, name)
		if strings.Contains(block, "permissions:") {
			t.Fatalf("%s job must not declare permissions:", name)
		}
	}
}

func TestWorkflowReleaseJobWaitsForTheGates(t *testing.T) {
	src := readRepoFile(t, workflowPath(t))
	block := jobBlock(t, src, "release")
	// test-e2e was split out of check into its own sharded job at
	// v0.1.0-rc.6 (see ci.yml's header comment); release must wait for
	// all three so a red e2e shard still blocks publication.
	if !strings.Contains(block, "needs: [check, build-cross, test-e2e]") {
		t.Fatal("release job missing needs: [check, build-cross, test-e2e]")
	}
}

func TestWorkflowReleasePublishesExactlyTheFiveAssets(t *testing.T) {
	src := readRepoFile(t, workflowPath(t))
	block := jobBlock(t, src, "release")
	want := []string{
		"bin/gitid-darwin-amd64",
		"bin/gitid-darwin-arm64",
		"bin/gitid-linux-amd64",
		"bin/gitid-linux-arm64",
		"bin/checksums.txt",
	}
	idx := strings.Index(block, "files:")
	if idx < 0 {
		t.Fatal("release job missing files:")
	}
	rest := block[idx:]
	var got []string
	for _, line := range strings.Split(rest, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "bin/") {
			got = append(got, trimmed)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("files: listed %d assets, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("files[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestWorkflowReleaseStepsAreTheExpectedSequence(t *testing.T) {
	src := readRepoFile(t, workflowPath(t))
	block := jobBlock(t, src, "release")
	want := []string{
		"Checkout",
		"Set up Go",
		"Compute release version metadata",
		"make checksums (stamped cross-build + SHA-256 manifest)",
		"Publish GitHub Release",
	}
	var got []string
	for _, line := range strings.Split(block, "\n") {
		trimmed := strings.TrimSpace(line)
		trimmed = strings.TrimPrefix(trimmed, "- ")
		if !strings.HasPrefix(trimmed, "name: ") {
			continue
		}
		name := strings.TrimPrefix(trimmed, "name: ")
		if strings.HasPrefix(name, "release") {
			continue
		}
		got = append(got, name)
	}
	if len(got) != len(want) {
		t.Fatalf("release steps = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("step[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestWorkflowReleaseBuildsThroughTheMakeTarget(t *testing.T) {
	src := readRepoFile(t, workflowPath(t))
	block := jobBlock(t, src, "release")
	if !strings.Contains(block, "make checksums") {
		t.Fatal("release job missing make checksums")
	}
	for _, needle := range []string{"VERSION=", "COMMIT=", "DATE="} {
		if !strings.Contains(block, needle) {
			t.Fatalf("release build step missing %s", needle)
		}
	}
}

func TestWorkflowReleaseDeclaresItsStateAtPublication(t *testing.T) {
	src := readRepoFile(t, workflowPath(t))
	block := jobBlock(t, src, "release")
	if !regexp.MustCompile(`prerelease:\s*\$\{\{\s*steps\.relver\.outputs\.prerelease\s*\}\}`).MatchString(block) {
		t.Fatal("publish step missing prerelease input sourced from relver")
	}
	if !regexp.MustCompile(`make_latest:\s*\$\{\{\s*steps\.relver\.outputs\.make_latest\s*\}\}`).MatchString(block) {
		t.Fatal("publish step missing make_latest input sourced from relver")
	}
}

func TestWorkflowReleaseStateIsNotCorrectedAfterPublication(t *testing.T) {
	src := readRepoFile(t, workflowPath(t))
	block := jobBlock(t, src, "release")
	re := regexp.MustCompile(`(?i)gh\s+release`)
	for i, line := range strings.Split(block, "\n") {
		if re.MatchString(line) {
			t.Fatalf("release job line %d amends the release after publication: %s", i+1, line)
		}
	}
}

func TestWorkflowPrereleaseIsDerivedFromTheTag(t *testing.T) {
	src := readRepoFile(t, workflowPath(t))
	block := jobBlock(t, src, "release")
	if !strings.Contains(block, "id: relver") {
		t.Fatal("missing relver step")
	}
	if !strings.Contains(block, "GITHUB_REF_NAME") {
		t.Fatal("relver step does not read GITHUB_REF_NAME")
	}
	if !strings.Contains(block, `prerelease=`) || !strings.Contains(block, `make_latest=`) {
		t.Fatal("relver step does not write prerelease= and make_latest=")
	}
	if !strings.Contains(block, `"$GITHUB_OUTPUT"`) {
		t.Fatal("relver step does not write to $GITHUB_OUTPUT")
	}
}

func TestLdflagsSymbolsAreDeclaredInMain(t *testing.T) {
	makefile := readRepoFile(t, makefilePath(t))
	mainSrc := readRepoFile(t, filepath.Join("..", "..", "cmd", "gitid", "main.go"))
	re := regexp.MustCompile(`-X main\.([A-Za-z_][A-Za-z0-9_]*)=`)
	matches := re.FindAllStringSubmatch(makefile, -1)
	if len(matches) == 0 {
		t.Fatal("Makefile LDFLAGS has no -X main.<name>=")
	}
	seen := map[string]bool{}
	for _, m := range matches {
		name := m[1]
		if seen[name] {
			continue
		}
		seen[name] = true
		decl := regexp.MustCompile(`(?m)^\s*` + regexp.QuoteMeta(name) + `\s*=\s*`)
		if !decl.MatchString(mainSrc) {
			t.Errorf("-X main.%s= has no matching package-level %s = declaration in cmd/gitid/main.go", name, name)
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
