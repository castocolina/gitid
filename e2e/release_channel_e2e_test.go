//go:build e2e

package e2e

// release_channel_e2e_test.go — D-20 (10-CONTEXT.md addendum, 2026-09-05):
// real e2e coverage for scripts/install.sh's GITID_CHANNEL resolution and
// the usable-/dev/tty interactive menu, proving every new path against a
// real fixture server and (for the interactive case) a real pseudo-terminal
// — never merely asserted. Reuses this package's existing stampedArtifacts/
// populateReleaseDownloadDir/e2eEnv helpers (release_e2e_test.go,
// harness_test.go) rather than duplicating the release-build machinery.

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"
)

// releaseEntry is the subset of GitHub's releases-API JSON shape install.sh
// (and this file's fixture server) actually need: tag_name plus a
// human-readable marker of whether the entry is a nightly build.
type releaseEntry struct {
	TagName    string `json:"tag_name"`
	Prerelease bool   `json:"prerelease"`
}

// startReleasesAPIFixtureServer serves the GitHub releases-API response
// shape (`GET /repos/${REPO}/releases`, newest-first array) at BOTH the
// download/redirect paths (startGitHubRedirectFixtureServer's existing
// behavior, reused here so GITID_INSTALL_BASE_URL and
// GITID_INSTALL_API_BASE_URL can point at the SAME fixture server, matching
// the D-20 "one fixture server serves both path shapes" design) and the new
// `/repos/${REPO}/releases` path. artifactsDir must already contain the
// renamed archives/checksums for EVERY tag in entries (see
// retagArtifactsDir) — this function copies them into the download layout
// for each tag.
func startReleasesAPIFixtureServer(t *testing.T, artifactsDirs map[string]string, entries []releaseEntry) *fixtureServer {
	t.Helper()
	root := t.TempDir()
	for tag, dir := range artifactsDirs {
		populateReleaseDownloadDir(t, root, dir, tag)
	}

	body, err := json.Marshal(entries)
	if err != nil {
		t.Fatalf("startReleasesAPIFixtureServer: marshal entries: %v", err)
	}

	// The FIRST non-prerelease entry is what a real GitHub /releases/latest
	// redirect would resolve to — mirrors startGitHubRedirectFixtureServer's
	// existing single-tag redirect, generalized to pick from entries.
	var latestStableTag string
	for _, e := range entries {
		if !e.Prerelease {
			latestStableTag = e.TagName
			break
		}
	}

	latestPath := "/" + releaseRepoPath + "/releases/latest"
	apiPath := "/repos/" + releaseRepoPath + "/releases"
	fileServer := http.FileServer(http.Dir(root))
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case apiPath:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(body)
		case latestPath:
			if latestStableTag == "" {
				http.NotFound(w, r)
				return
			}
			http.Redirect(w, r, "/"+releaseRepoPath+"/releases/tag/"+latestStableTag, http.StatusFound)
		default:
			if strings.HasPrefix(r.URL.Path, "/"+releaseRepoPath+"/releases/tag/") {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("<html><body>release tag page (fixture)</body></html>"))
				return
			}
			fileServer.ServeHTTP(w, r)
		}
	})
	return startRecordingHandler(t, inner)
}

// retagArtifactsDir copies stampedArtifacts's real, host-built archive (and
// rewrites the checksums manifest) into a fresh directory as if they had
// been published under newTag instead of art.tag — install.sh derives its
// asset/checksums filenames from the CHOSEN tag's version_num
// (`${tag#v}`), so a fixture exercising tag resolution for a tag other than
// the one stampedArtifacts actually built under must present renamed (but
// still real, still correctly-hashed) files.
func retagArtifactsDir(t *testing.T, art releaseArtifacts, newTag string) string {
	t.Helper()
	newVersion := strings.TrimPrefix(newTag, "v")
	dir := t.TempDir()

	oldChecksumsName := checksumsName(art.version)
	newChecksumsName := checksumsName(newVersion)
	oldChecksumsBody, err := os.ReadFile(filepath.Join(art.distDir, oldChecksumsName)) //nolint:gosec
	if err != nil {
		t.Fatalf("retagArtifactsDir: read checksums: %v", err)
	}
	newChecksumsBody := string(oldChecksumsBody)

	for _, p := range publishedPlatforms {
		oldName := archiveName(art.version, p.os, p.arch)
		newName := archiveName(newVersion, p.os, p.arch)
		oldPath := filepath.Join(art.distDir, oldName)
		if _, statErr := os.Stat(oldPath); statErr != nil {
			continue // not every platform archive need exist on every test host
		}
		body, readErr := os.ReadFile(oldPath) //nolint:gosec
		if readErr != nil {
			t.Fatalf("retagArtifactsDir: read %s: %v", oldName, readErr)
		}
		if writeErr := os.WriteFile(filepath.Join(dir, newName), body, 0o644); writeErr != nil { //nolint:gosec
			t.Fatalf("retagArtifactsDir: write %s: %v", newName, writeErr)
		}
		newChecksumsBody = strings.ReplaceAll(newChecksumsBody, oldName, newName)
	}
	if err := os.WriteFile(filepath.Join(dir, newChecksumsName), []byte(newChecksumsBody), 0o644); err != nil { //nolint:gosec
		t.Fatalf("retagArtifactsDir: write checksums: %v", err)
	}
	return dir
}

// runInstallScriptWithEnv is a lighter-weight sibling of
// runInstallScriptFull (release_e2e_test.go) that accepts arbitrary extra
// env vars instead of a fixed parameter list — needed here for
// GITID_CHANNEL / GITID_INSTALL_API_BASE_URL / GITID_ASSUME_HEADLESS, none
// of which the original helper's signature anticipated.
func runInstallScriptWithEnv(t *testing.T, home string, extraEnv []string) scriptResult {
	t.Helper()
	env, _ := e2eEnv(t, home)
	env = append(env, extraEnv...)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second*ciTimeoutMultiplier())
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/sh", "scripts/install.sh")
	cmd.Dir = repoRoot(t)
	cmd.Env = env
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return scriptResult{stdout: stdout.String(), stderr: stderr.String(), err: err}
}

func TestInstallScript_ChannelNightlyResolvesLatestNightly(t *testing.T) {
	art := stampedArtifacts(t)
	stableTag := art.tag
	nightlyTag := "v0.0.0-nightly.20260101120000.abc1234"
	nightlyDir := retagArtifactsDir(t, art, nightlyTag)

	fx := startReleasesAPIFixtureServer(t, map[string]string{
		stableTag:  art.distDir,
		nightlyTag: nightlyDir,
	}, []releaseEntry{
		{TagName: nightlyTag, Prerelease: true}, // newest-first: nightly first
		{TagName: stableTag, Prerelease: false},
	})

	home := t.TempDir()
	res := runInstallScriptWithEnv(t, home, []string{
		"GITID_INSTALL_BASE_URL=" + fx.srv.URL,
		"GITID_INSTALL_API_BASE_URL=" + fx.srv.URL,
		"GITID_CHANNEL=nightly",
	})
	if res.err != nil {
		t.Fatalf("install.sh (GITID_CHANNEL=nightly) failed: %v\nstdout=%s\nstderr=%s", res.err, res.stdout, res.stderr)
	}
	installed := filepath.Join(home, ".local", "bin", "gitid")
	if _, statErr := os.Stat(installed); statErr != nil {
		t.Fatalf("expected %s to exist after nightly install: %v", installed, statErr)
	}
	nightlyArchive := hostArchiveName(t, strings.TrimPrefix(nightlyTag, "v"))
	want := gitidMemberSHA256(t, filepath.Join(nightlyDir, nightlyArchive))
	got := fileSHA256(t, installed)
	if got != want {
		t.Fatalf("installed binary SHA-256 %s does not match the NIGHTLY archive's gitid member %s — resolved the wrong tag", got, want)
	}
}

// TestInstallScript_ChannelUnsetPreservesExistingHeadlessBehavior is the
// must_have regression guard: with GITID_CHANNEL unset and no usable tty
// (GITID_ASSUME_HEADLESS=1, deterministic in CI regardless of the real
// /dev/tty state), install.sh must behave EXACTLY like the pre-D-20 script
// — same /releases/latest redirect resolution, same installed binary.
func TestInstallScript_ChannelUnsetPreservesExistingHeadlessBehavior(t *testing.T) {
	art := stampedArtifacts(t)
	fx := startReleasesAPIFixtureServer(t, map[string]string{art.tag: art.distDir}, []releaseEntry{
		{TagName: art.tag, Prerelease: false},
	})

	home := t.TempDir()
	res := runInstallScriptWithEnv(t, home, []string{
		"GITID_INSTALL_BASE_URL=" + fx.srv.URL,
		"GITID_INSTALL_API_BASE_URL=" + fx.srv.URL,
		"GITID_ASSUME_HEADLESS=1",
	})
	if res.err != nil {
		t.Fatalf("install.sh (channel unset, headless) failed: %v\nstdout=%s\nstderr=%s", res.err, res.stdout, res.stderr)
	}
	installed := filepath.Join(home, ".local", "bin", "gitid")
	hostArchive := hostArchiveName(t, art.version)
	want := gitidMemberSHA256(t, filepath.Join(art.distDir, hostArchive))
	got := fileSHA256(t, installed)
	if got != want {
		t.Fatalf("installed binary SHA-256 %s does not match the STABLE archive's gitid member %s — headless default resolution regressed", got, want)
	}
}

// TestInstallScript_UnusableTTYFallsBackToHeadless proves the OTHER half of
// the same regression guard from the angle of the probe itself: even when
// GITID_CHANNEL/GITID_VERSION are BOTH unset (the condition that would
// otherwise trigger the interactive menu), GITID_ASSUME_HEADLESS=1 forces
// the headless branch deterministically, with no menu printed and no hang.
func TestInstallScript_UnusableTTYFallsBackToHeadless(t *testing.T) {
	art := stampedArtifacts(t)
	fx := startReleasesAPIFixtureServer(t, map[string]string{art.tag: art.distDir}, []releaseEntry{
		{TagName: art.tag, Prerelease: false},
	})

	home := t.TempDir()
	res := runInstallScriptWithEnv(t, home, []string{
		"GITID_INSTALL_BASE_URL=" + fx.srv.URL,
		"GITID_INSTALL_API_BASE_URL=" + fx.srv.URL,
		"GITID_ASSUME_HEADLESS=1",
	})
	if res.err != nil {
		t.Fatalf("install.sh (assume-headless) failed: %v\nstdout=%s\nstderr=%s", res.err, res.stdout, res.stderr)
	}
	if strings.Contains(res.stdout, "select a release") || strings.Contains(res.stderr, "select a release") {
		t.Fatalf("headless run unexpectedly printed the interactive menu:\nstdout=%s\nstderr=%s", res.stdout, res.stderr)
	}
	installed := filepath.Join(home, ".local", "bin", "gitid")
	if _, statErr := os.Stat(installed); statErr != nil {
		t.Fatalf("expected %s to exist: %v", installed, statErr)
	}
}

// TestInstallScript_UsableTTYShowsInteractiveMenu drives scripts/install.sh
// under a REAL pseudo-terminal (github.com/creack/pty, already a module
// dependency via this package's other PTY e2e suites) with its stdin
// attached to the pty's slave end and GITID_ASSUME_HEADLESS unset — proving
// the `{ true < /dev/tty; } 2>/dev/null` open-probe genuinely succeeds
// under a real controlling terminal and the numbered menu actually renders,
// then feeds a numeric choice through the pty's master end and asserts the
// chosen release was installed.
func TestInstallScript_UsableTTYShowsInteractiveMenu(t *testing.T) {
	art := stampedArtifacts(t)
	nightlyTag := "v0.0.0-nightly.20260101120000.abc1234"
	nightlyDir := retagArtifactsDir(t, art, nightlyTag)
	fx := startReleasesAPIFixtureServer(t, map[string]string{
		nightlyTag: nightlyDir,
		art.tag:    art.distDir,
	}, []releaseEntry{
		{TagName: nightlyTag, Prerelease: true}, // index 1 in the printed menu
		{TagName: art.tag, Prerelease: false},   // index 2
	})

	home := t.TempDir()
	env, _ := e2eEnv(t, home)
	env = append(env,
		"GITID_INSTALL_BASE_URL="+fx.srv.URL,
		"GITID_INSTALL_API_BASE_URL="+fx.srv.URL,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second*ciTimeoutMultiplier())
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/sh", "scripts/install.sh")
	cmd.Dir = repoRoot(t)
	cmd.Env = env

	ptmx, err := pty.Start(cmd)
	if err != nil {
		t.Fatalf("pty.Start: %v", err)
	}
	defer func() { _ = ptmx.Close() }()

	// captureBuf is safe for concurrent read (by the poll loop below) and
	// write (by the reader goroutine) — a plain strings.Builder is not.
	var mu sync.Mutex
	var captureBuf strings.Builder
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		buf := make([]byte, 65536)
		for {
			n, rerr := ptmx.Read(buf)
			if n > 0 {
				mu.Lock()
				captureBuf.Write(buf[:n])
				mu.Unlock()
			}
			if rerr != nil {
				return
			}
		}
	}()
	snapshot := func() string {
		mu.Lock()
		defer mu.Unlock()
		return captureBuf.String()
	}

	// Poll for the actual prompt text instead of a fixed sleep (code-review
	// finding: a blind sleep is a flaky-test generator on a loaded/slow CI
	// runner). "Enter choice" is the exact prompt install.sh prints right
	// before its `read -r choice < /dev/tty` — the earliest safe moment to
	// write a reply.
	deadline := time.Now().Add(30 * time.Second * ciTimeoutMultiplier())
	for !strings.Contains(snapshot(), "Enter choice") {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for the interactive menu prompt; captured so far:\n%s", snapshot())
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, err := ptmx.Write([]byte("1\n")); err != nil {
		t.Fatalf("write menu choice to pty: %v", err)
	}

	waitErr := cmd.Wait()
	<-readDone
	output := snapshot()
	if waitErr != nil {
		t.Fatalf("install.sh (interactive pty) failed: %v\noutput=%s", waitErr, output)
	}
	if !strings.Contains(output, "select a release") {
		t.Fatalf("expected the interactive menu prompt in pty output, got:\n%s", output)
	}
	if !strings.Contains(output, nightlyTag) {
		t.Fatalf("expected the menu to list the nightly tag %q, got:\n%s", nightlyTag, output)
	}
	installed := filepath.Join(home, ".local", "bin", "gitid")
	if _, statErr := os.Stat(installed); statErr != nil {
		t.Fatalf("expected %s to exist after interactive install, output:\n%s\nstat err: %v", installed, output, statErr)
	}
	nightlyArchive := hostArchiveName(t, strings.TrimPrefix(nightlyTag, "v"))
	want := gitidMemberSHA256(t, filepath.Join(nightlyDir, nightlyArchive))
	got := fileSHA256(t, installed)
	if got != want {
		t.Fatalf("installed binary SHA-256 %s does not match the menu's choice-1 (nightly) archive %s — wrong entry installed", got, want)
	}
}

// TestInstallScript_InvalidChannelRefuses is a code-review regression guard:
// an unrecognized GITID_CHANNEL value (a typo like "Nightly", or any string
// other than "stable"/"nightly") must be refused loudly, never silently
// treated as the default/stable channel.
func TestInstallScript_InvalidChannelRefuses(t *testing.T) {
	art := stampedArtifacts(t)
	fx := startReleasesAPIFixtureServer(t, map[string]string{art.tag: art.distDir}, []releaseEntry{
		{TagName: art.tag, Prerelease: false},
	})

	home := t.TempDir()
	res := runInstallScriptWithEnv(t, home, []string{
		"GITID_INSTALL_BASE_URL=" + fx.srv.URL,
		"GITID_INSTALL_API_BASE_URL=" + fx.srv.URL,
		"GITID_ASSUME_HEADLESS=1",
		"GITID_CHANNEL=Nightly",
	})
	if res.err == nil {
		t.Fatalf("expected install.sh to refuse an unrecognized GITID_CHANNEL, but it succeeded:\nstdout=%s\nstderr=%s", res.stdout, res.stderr)
	}
	if !strings.Contains(res.stderr, "unrecognized GITID_CHANNEL") {
		t.Fatalf("expected an 'unrecognized GITID_CHANNEL' refusal message, got stderr:\n%s", res.stderr)
	}
	installed := filepath.Join(home, ".local", "bin", "gitid")
	if _, statErr := os.Stat(installed); statErr == nil {
		t.Fatal("install.sh installed a binary despite an invalid GITID_CHANNEL — it must refuse before any install")
	}
}

// TestInstallScript_MenuExcludesOldAssetShapeReleases is a regression guard
// for a real cross-AI code-review finding (2026-09-05): the interactive
// menu must not list a release that never published the new (D-07/D-14)
// versioned-checksums asset shape this installer expects — selecting one
// would 404. oldShapeTag is deliberately given a releases-API entry but NO
// download-directory contents at all (simulating a pre-migration,
// old-raw-binary-shape release, which this fixture's file server then 404s
// on), so a correct install.sh must silently skip it from the menu.
func TestInstallScript_MenuExcludesOldAssetShapeReleases(t *testing.T) {
	art := stampedArtifacts(t)
	oldShapeTag := "v0.1.0-rc.1" // present in the release list, but no assets exist for it in this fixture
	nightlyTag := "v0.0.0-nightly.20260101120000.abc1234"
	nightlyDir := retagArtifactsDir(t, art, nightlyTag)
	fx := startReleasesAPIFixtureServer(t, map[string]string{
		nightlyTag: nightlyDir,
		art.tag:    art.distDir,
		// oldShapeTag intentionally omitted from artifactsDirs — no
		// download directory is populated for it, so any request against
		// its asset paths 404s via the fixture's underlying file server.
	}, []releaseEntry{
		{TagName: oldShapeTag, Prerelease: false}, // newest per this fixture's ordering, but asset-shape-unavailable
		{TagName: nightlyTag, Prerelease: true},
		{TagName: art.tag, Prerelease: false},
	})

	home := t.TempDir()
	env, _ := e2eEnv(t, home)
	env = append(env,
		"GITID_INSTALL_BASE_URL="+fx.srv.URL,
		"GITID_INSTALL_API_BASE_URL="+fx.srv.URL,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second*ciTimeoutMultiplier())
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/sh", "scripts/install.sh")
	cmd.Dir = repoRoot(t)
	cmd.Env = env

	ptmx, err := pty.Start(cmd)
	if err != nil {
		t.Fatalf("pty.Start: %v", err)
	}
	defer func() { _ = ptmx.Close() }()

	var mu sync.Mutex
	var captureBuf strings.Builder
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		buf := make([]byte, 65536)
		for {
			n, rerr := ptmx.Read(buf)
			if n > 0 {
				mu.Lock()
				captureBuf.Write(buf[:n])
				mu.Unlock()
			}
			if rerr != nil {
				return
			}
		}
	}()
	snapshot := func() string {
		mu.Lock()
		defer mu.Unlock()
		return captureBuf.String()
	}

	deadline := time.Now().Add(30 * time.Second * ciTimeoutMultiplier())
	for !strings.Contains(snapshot(), "Enter choice") {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for the interactive menu prompt; captured so far:\n%s", snapshot())
		}
		time.Sleep(20 * time.Millisecond)
	}
	menuText := snapshot()
	if strings.Contains(menuText, oldShapeTag) {
		t.Fatalf("menu listed %q, a release with no published new-shape assets — it must be filtered out:\n%s", oldShapeTag, menuText)
	}
	if !strings.Contains(menuText, nightlyTag) || !strings.Contains(menuText, art.tag) {
		t.Fatalf("menu is missing an installable release (nightly=%q stable=%q):\n%s", nightlyTag, art.tag, menuText)
	}

	if _, err := ptmx.Write([]byte("1\n")); err != nil {
		t.Fatalf("write menu choice to pty: %v", err)
	}
	waitErr := cmd.Wait()
	<-readDone
	if waitErr != nil {
		t.Fatalf("install.sh (interactive pty) failed: %v\noutput=%s", waitErr, snapshot())
	}
	installed := filepath.Join(home, ".local", "bin", "gitid")
	if _, statErr := os.Stat(installed); statErr != nil {
		t.Fatalf("expected %s to exist after interactive install: %v", installed, statErr)
	}
}
