//go:build e2e

package e2e

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	e2eStampVersion = "9.9.9-e2e"
	e2eStampCommit  = "deadbee"
	e2eStampDate    = "2001-02-03"

	// releaseRepoPath mirrors install.sh's own REPO constant — the fixture
	// servers below root their download layout under this exact path so a
	// request for "/${REPO}/releases/download/<tag>/<asset>" resolves
	// identically whether GITHUB_ORIGIN is the real github.com or a test
	// fixture server (REVIEW C-2: a pure origin substitution, never a
	// simplified/bypassed test-only path shape).
	releaseRepoPath = "castocolina/gitid"
)

// publishedPlatforms is the D-12 four-target build matrix. Archive/checksums
// filenames are built from these plus a discovered version string — never
// hardcoded literals (REVIEW C-2's "real filename" requirement).
var publishedPlatforms = []struct{ os, arch string }{
	{"darwin", "amd64"},
	{"darwin", "arm64"},
	{"linux", "amd64"},
	{"linux", "arm64"},
}

// archiveName reproduces goreleaser's default archive name_template
// ({{.ProjectName}}_{{.Version}}_{{.Os}}_{{.Arch}}, D-07) — the SAME
// construction install.sh's own asset-name logic must independently arrive
// at from a resolved tag.
func archiveName(version, osTag, archTag string) string {
	return fmt.Sprintf("gitid_%s_%s_%s.tar.gz", version, osTag, archTag)
}

// checksumsName reproduces goreleaser's default checksum name_template
// ({{.ProjectName}}_{{.Version}}_checksums.txt, D-15).
func checksumsName(version string) string {
	return fmt.Sprintf("gitid_%s_checksums.txt", version)
}

// downloadPath reproduces the real GitHub Releases URL shape
// "/${REPO}/releases/download/${tag}/${name}" both the fixture servers and
// install.sh itself construct.
func downloadPath(tag, name string) string {
	return "/" + releaseRepoPath + "/releases/download/" + tag + "/" + name
}

// releaseArtifacts is what a single `make release-snapshot` run produced:
// the dist/ directory plus goreleaser's OWN computed version string
// (discovered from the real output, never assumed) and the synthetic
// "v"-prefixed tag string install.sh's GITID_VERSION pin and the fixture
// servers below key their download paths on.
type releaseArtifacts struct {
	distDir string
	version string
	tag     string
}

var (
	releaseOnce sync.Once
	releaseInfo releaseArtifacts
	releaseErr  error
)

// e2eStampLine is the D-11 four-part --version stamp
// ("<version> (<commit>, <date>, <goos>/<goarch>)") a make-release-snapshot
// stamped build produces for the given platform. Plan 10-01 added the
// "<goos>/<goarch>" suffix to the format; the prior raw-binary-era literal
// this test file used lacked it (a known-broken assertion this rewrite
// fixes, per 10-05's own required_reading note).
func e2eStampLine(goos, goarch string) string {
	return fmt.Sprintf("gitid version %s (%s, %s, %s/%s)", e2eStampVersion, e2eStampCommit, e2eStampDate, goos, goarch)
}

// stampedArtifacts runs the REAL `make release-snapshot` once per test
// package run (D-05: the same goreleaser config the phase's release.yml
// invokes) and discovers the actual produced archive/checksums version from
// dist/ — never a hardcoded raw-binary-era literal (REVIEW C-2).
func stampedArtifacts(t *testing.T) releaseArtifacts {
	t.Helper()
	releaseOnce.Do(func() {
		root := repoRoot(t)
		ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second*ciTimeoutMultiplier())
		defer cancel()
		cmd := exec.CommandContext(ctx, "make", "release-snapshot",
			"VERSION="+e2eStampVersion,
			"COMMIT="+e2eStampCommit,
			"DATE="+e2eStampDate,
		)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "HOME="+realHome)
		out, err := cmd.CombinedOutput()
		if err != nil {
			releaseErr = fmt.Errorf("make release-snapshot: %w\n%s", err, out)
			return
		}
		distDir := filepath.Join(root, "dist")
		version, verr := discoverReleaseVersion(distDir)
		if verr != nil {
			releaseErr = verr
			return
		}
		releaseInfo = releaseArtifacts{distDir: distDir, version: version, tag: "v" + version}
	})
	if releaseErr != nil {
		t.Fatal(releaseErr)
	}
	return releaseInfo
}

// discoverReleaseVersion extracts goreleaser's own computed version string
// from the ONE checksums manifest `make release-snapshot` produced in
// distDir — the version embedded in every archive/checksums filename this
// test suite must match, discovered from the real artifact rather than
// guessed.
func discoverReleaseVersion(distDir string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(distDir, "gitid_*_checksums.txt"))
	if err != nil {
		return "", fmt.Errorf("discoverReleaseVersion: glob %s: %w", distDir, err)
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("discoverReleaseVersion: want exactly 1 checksums manifest in %s, got %d: %v", distDir, len(matches), matches)
	}
	base := filepath.Base(matches[0])
	version := strings.TrimSuffix(strings.TrimPrefix(base, "gitid_"), "_checksums.txt")
	if version == "" || version == base {
		return "", fmt.Errorf("discoverReleaseVersion: could not extract version from %q", base)
	}
	return version, nil
}

// hostArchiveName returns the archive filename for the host's own
// GOOS/GOARCH, skipping the test when the host falls outside the published
// matrix (D-12).
func hostArchiveName(t *testing.T, version string) string {
	t.Helper()
	for _, p := range publishedPlatforms {
		if p.os == runtime.GOOS && p.arch == runtime.GOARCH {
			return archiveName(version, p.os, p.arch)
		}
	}
	t.Skipf("host %s/%s is outside the published matrix", runtime.GOOS, runtime.GOARCH)
	return ""
}

// extractGitid pulls ONLY the "gitid" tar member out of archivePath into
// destDir — mirroring install.sh's own extraction discipline (never a
// trusted glob/arbitrary archive entry) so test assertions about "what got
// installed" are provably derived from the real archive contents, not a
// hand-rolled fixture.
func extractGitid(t *testing.T, archivePath, destDir string) {
	t.Helper()
	f, err := os.Open(archivePath) //nolint:gosec
	if err != nil {
		t.Fatalf("extractGitid: open %s: %v", archivePath, err)
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("extractGitid: gzip %s: %v", archivePath, err)
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			t.Fatalf("extractGitid: %s: no gitid member found", archivePath)
		}
		if err != nil {
			t.Fatalf("extractGitid: %s: %v", archivePath, err)
		}
		if hdr.Name != "gitid" {
			continue
		}
		out, err := os.OpenFile(filepath.Join(destDir, "gitid"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755) //nolint:gosec
		if err != nil {
			t.Fatalf("extractGitid: create extracted gitid: %v", err)
		}
		if _, err := io.Copy(out, tr); err != nil { //nolint:gosec
			_ = out.Close()
			t.Fatalf("extractGitid: copy: %v", err)
		}
		if err := out.Close(); err != nil {
			t.Fatalf("extractGitid: close: %v", err)
		}
		return
	}
}

// gitidMemberSHA256 is the SHA-256 of the "gitid" binary INSIDE archivePath
// — what install.sh's own extraction step should produce, distinct from the
// archive's own SHA-256 (which also covers LICENSE/README).
func gitidMemberSHA256(t *testing.T, archivePath string) string {
	t.Helper()
	tmp := t.TempDir()
	extractGitid(t, archivePath, tmp)
	return fileSHA256(t, filepath.Join(tmp, "gitid"))
}

// newMutableReleaseCopy copies the full real release artifact set (all 4
// archives + the checksums manifest) into a fresh temp dir a test can then
// mutate (flip a byte, drop a manifest line) before serving it — keeping
// the mutation on a throwaway copy, never the shared dist/ output other
// tests in this package also read.
func newMutableReleaseCopy(t *testing.T, art releaseArtifacts) string {
	t.Helper()
	tmp := t.TempDir()
	names := make([]string, 0, len(publishedPlatforms)+1)
	for _, p := range publishedPlatforms {
		names = append(names, archiveName(art.version, p.os, p.arch))
	}
	names = append(names, checksumsName(art.version))
	copyDir(t, art.distDir, tmp, names...)
	return tmp
}

// populateReleaseDownloadDir copies every archive/checksums file out of
// artifactsDir into root, rooted at the REAL GitHub Releases path shape
// "<root>/${REPO}/releases/download/<tag>/" — the shared layout both fixture
// server constructors below build on.
func populateReleaseDownloadDir(t *testing.T, root, artifactsDir, tag string) {
	t.Helper()
	downloadDir := filepath.Join(root, filepath.FromSlash(releaseRepoPath), "releases", "download", tag)
	if err := os.MkdirAll(downloadDir, 0o755); err != nil {
		t.Fatalf("populateReleaseDownloadDir: mkdir %s: %v", downloadDir, err)
	}
	entries, err := os.ReadDir(artifactsDir)
	if err != nil {
		t.Fatalf("populateReleaseDownloadDir: reading %s: %v", artifactsDir, err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".tar.gz") && !strings.HasSuffix(name, "_checksums.txt") {
			continue
		}
		names = append(names, name)
	}
	copyDir(t, artifactsDir, downloadDir, names...)
}

type fixtureServer struct {
	srv   *httptest.Server
	mu    sync.Mutex
	paths []string
}

func (s *fixtureServer) Paths() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, len(s.paths))
	copy(out, s.paths)
	return out
}

func startFixtureServer(t *testing.T, root string) *fixtureServer {
	t.Helper()
	fs := &fixtureServer{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fs.mu.Lock()
		fs.paths = append(fs.paths, r.URL.Path)
		fs.mu.Unlock()
		http.FileServer(http.Dir(root)).ServeHTTP(w, r)
	})
	fs.srv = httptest.NewServer(handler)
	t.Cleanup(fs.srv.Close)
	return fs
}

func startRecordingHandler(t *testing.T, inner http.Handler) *fixtureServer {
	t.Helper()
	fsrv := &fixtureServer{}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fsrv.mu.Lock()
		fsrv.paths = append(fsrv.paths, r.URL.Path)
		fsrv.mu.Unlock()
		inner.ServeHTTP(w, r)
	})
	fsrv.srv = httptest.NewServer(handler)
	t.Cleanup(fsrv.srv.Close)
	return fsrv
}

// startReleaseFixtureServer mimics GitHub's REAL /releases/download/<tag>/<asset>
// URL shape (REVIEW C-2): every request install.sh makes against it exercises
// the SAME versioned asset-name and versioned-checksums-filename construction
// code a real pinned install against github.com would.
func startReleaseFixtureServer(t *testing.T, artifactsDir, tag string) *fixtureServer {
	t.Helper()
	root := t.TempDir()
	populateReleaseDownloadDir(t, root, artifactsDir, tag)
	return startFixtureServer(t, root)
}

// startGitHubRedirectFixtureServer additionally mimics GitHub's REAL
// /releases/latest 302-redirect-to-/releases/tag/<tag> shape (REVIEW C-2) —
// the one dedicated seam for install.sh's redirect-resolution/tag-stripping
// logic, which every OTHER test in this file bypasses via an explicit
// GITID_VERSION pin.
func startGitHubRedirectFixtureServer(t *testing.T, artifactsDir, tag string) *fixtureServer {
	t.Helper()
	root := t.TempDir()
	populateReleaseDownloadDir(t, root, artifactsDir, tag)

	latestPath := "/" + releaseRepoPath + "/releases/latest"
	tagPagePath := "/" + releaseRepoPath + "/releases/tag/" + tag
	fileServer := http.FileServer(http.Dir(root))
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case latestPath:
			http.Redirect(w, r, tagPagePath, http.StatusFound)
		case tagPagePath:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<html><body>release tag page (fixture)</body></html>"))
		default:
			fileServer.ServeHTTP(w, r)
		}
	})
	return startRecordingHandler(t, inner)
}

type scriptResult struct {
	stdout string
	stderr string
	err    error
}

// runInstallScript runs scripts/install.sh pinned to an explicit release
// tag (GITID_VERSION) — the real, already-specified pinned-install
// production path (D-14), exercised by every rewritten
// TestInstallScript_* case so each one drives install.sh's REAL versioned
// asset-name AND versioned-checksums-filename construction code (REVIEW
// C-2), never a simplified/bypassed one.
func runInstallScript(t *testing.T, home, path, baseURL, version string) scriptResult {
	t.Helper()
	return runInstallScriptFull(t, home, path, baseURL, version, "", false)
}

func runInstallScriptPiped(t *testing.T, home, path, baseURL, version string) scriptResult {
	t.Helper()
	return runInstallScriptFull(t, home, path, baseURL, version, "", true)
}

// runInstallScriptFull is the one place that builds install.sh's child
// environment (via e2eEnv, R1) and optionally sets GITID_VERSION /
// GITID_INSTALL_DIR — leaving either empty omits the env var entirely so a
// test can exercise the script's own unset-default behavior (the redirect
// test leaves version empty; every other test pins it).
func runInstallScriptFull(t *testing.T, home, path, baseURL, version, installDir string, piped bool) scriptResult {
	t.Helper()
	prefixes, replacePath := installPathParts(path)
	env, _ := e2eEnv(t, home, prefixes...)
	env = append(env, "GITID_INSTALL_BASE_URL="+baseURL)
	if version != "" {
		env = append(env, "GITID_VERSION="+version)
	}
	if installDir != "" {
		env = append(env, "GITID_INSTALL_DIR="+installDir)
	}
	if replacePath != "" {
		env = append(env, "PATH="+replacePath)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second*ciTimeoutMultiplier())
	defer cancel()
	var cmd *exec.Cmd
	if piped {
		cmd = exec.CommandContext(ctx, "/bin/sh", "-c", "cat scripts/install.sh | /bin/sh")
	} else {
		cmd = exec.CommandContext(ctx, "/bin/sh", "scripts/install.sh")
	}
	cmd.Dir = repoRoot(t)
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return scriptResult{stdout: stdout.String(), stderr: stderr.String(), err: err}
}

func installPathParts(path string) (prefixes []string, replacePath string) {
	ambient := os.Getenv("PATH")
	if path == "" || path == ambient {
		return nil, ""
	}
	suffix := string(os.PathListSeparator) + ambient
	if strings.HasSuffix(path, suffix) {
		prefix := strings.TrimSuffix(path, suffix)
		for _, part := range filepath.SplitList(prefix) {
			if part != "" {
				prefixes = append(prefixes, part)
			}
		}
		return prefixes, ""
	}
	return nil, path
}

func TestRelease_SnapshotBuildProducesArchivesForEveryTarget(t *testing.T) {
	art := stampedArtifacts(t)
	for _, p := range publishedPlatforms {
		name := archiveName(art.version, p.os, p.arch)
		path := filepath.Join(art.distDir, name)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("%s missing: %v", name, err)
		}
		if info.Size() == 0 {
			t.Fatalf("%s is empty", name)
		}
	}
	hostArchive := hostArchiveName(t, art.version)
	extractDir := t.TempDir()
	extractGitid(t, filepath.Join(art.distDir, hostArchive), extractDir)
	binPath := filepath.Join(extractDir, "gitid")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second*ciTimeoutMultiplier())
	defer cancel()
	cmd := exec.CommandContext(ctx, binPath, "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s --version: %v\n%s", hostArchive, err, out)
	}
	got := strings.TrimSpace(string(out))
	want := e2eStampLine(runtime.GOOS, runtime.GOARCH)
	if got != want {
		t.Fatalf("%s --version = %q, want %q", hostArchive, got, want)
	}
}

func TestRelease_UnstampedBuildKeepsDevDefaults(t *testing.T) {
	root := repoRoot(t)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second*ciTimeoutMultiplier())
	defer cancel()
	cmd := exec.CommandContext(ctx, "make", "build")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "HOME="+realHome)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("make build: %v\n%s", err, out)
	}
	verCtx, verCancel := context.WithTimeout(context.Background(), 10*time.Second*ciTimeoutMultiplier())
	defer verCancel()
	ver := exec.CommandContext(verCtx, filepath.Join(root, "bin", "gitid"), "--version")
	out, err := ver.CombinedOutput()
	if err != nil {
		t.Fatalf("bin/gitid --version: %v\n%s", err, out)
	}
	got := strings.TrimSpace(string(out))
	// VERSION now defaults to a live `git describe --tags --match "v*"`
	// value (Phase 10, D-10), non-deterministic across commits/tags — assert
	// the SHAPE instead of an exact literal. REVIEW C-1: anchor the version
	// field's first character to [^v] so a leading-`v` regression (the
	// Makefile's `patsubst v%,%,...` strip silently not taking effect) fails
	// this test instead of passing under a loose `\S+` match.
	want := regexp.MustCompile(`^gitid version [^v]\S*? \(none, unknown, (darwin|linux)/(amd64|arm64)\)$`)
	if !want.MatchString(got) {
		t.Fatalf("unstamped --version = %q, want match of %s", got, want.String())
	}
}

func TestRelease_ChecksumsManifestMatchesArchives(t *testing.T) {
	art := stampedArtifacts(t)
	manifestName := checksumsName(art.version)
	raw, err := os.ReadFile(filepath.Join(art.distDir, manifestName)) //nolint:gosec
	if err != nil {
		t.Fatalf("reading %s: %v", manifestName, err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("%s has %d lines, want 4:\n%s", manifestName, len(lines), raw)
	}
	pat := regexp.MustCompile(`^[0-9a-f]{64}  gitid_` + regexp.QuoteMeta(art.version) + `_(darwin|linux)_(amd64|arm64)\.tar\.gz$`)
	seen := map[string]string{}
	for _, line := range lines {
		if !pat.MatchString(line) {
			t.Fatalf("%s line %q does not match anchored two-space asset pattern", manifestName, line)
		}
		hash, name, ok := strings.Cut(line, "  ")
		if !ok {
			t.Fatalf("%s line %q missing two-space separator", manifestName, line)
		}
		if _, dup := seen[name]; dup {
			t.Fatalf("duplicate asset %q in %s", name, manifestName)
		}
		seen[name] = hash
		body, err := os.ReadFile(filepath.Join(art.distDir, name)) //nolint:gosec
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		sum := sha256.Sum256(body)
		got := hex.EncodeToString(sum[:])
		if got != hash {
			t.Fatalf("%s: recomputed sha256 %s != manifest %s", name, got, hash)
		}
	}
	for _, p := range publishedPlatforms {
		name := archiveName(art.version, p.os, p.arch)
		if _, ok := seen[name]; !ok {
			t.Fatalf("%s missing published asset %q", manifestName, name)
		}
	}
	if len(seen) != 4 {
		t.Fatalf("%s named %d assets, want 4", manifestName, len(seen))
	}
}

func TestInstallScript_InstallsVerifiedHostBinary(t *testing.T) {
	art := stampedArtifacts(t)
	hostArchive := hostArchiveName(t, art.version)
	home := SandboxHome(t)
	fsrv := startReleaseFixtureServer(t, art.distDir, art.tag)
	res := runInstallScript(t, home, os.Getenv("PATH"), fsrv.srv.URL, art.tag)
	if res.err != nil {
		t.Fatalf("install.sh: %v\nstdout:\n%s\nstderr:\n%s", res.err, res.stdout, res.stderr)
	}
	if !strings.Contains(res.stdout, "  verified: SHA-256 ") {
		t.Fatalf("stdout missing verified line:\n%s", res.stdout)
	}
	installed := filepath.Join(home, ".local", "bin", "gitid")
	if !strings.Contains(res.stdout, "  installed: "+installed) {
		t.Fatalf("stdout missing installed line for %s:\n%s", installed, res.stdout)
	}
	if !strings.Contains(res.stdout, "  PATH: ") {
		t.Fatalf("stdout missing PATH line:\n%s", res.stdout)
	}
	info, err := os.Stat(installed)
	if err != nil {
		t.Fatalf("installed binary missing: %v", err)
	}
	if info.Mode()&0o777 != 0o755 {
		t.Fatalf("installed mode = %o, want 0755", info.Mode()&0o777)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second*ciTimeoutMultiplier())
	defer cancel()
	out, err := exec.CommandContext(ctx, installed, "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("installed --version: %v\n%s", err, out)
	}
	wantVersionLine := e2eStampLine(runtime.GOOS, runtime.GOARCH)
	if got := strings.TrimSpace(string(out)); got != wantVersionLine {
		t.Fatalf("installed --version = %q, want %q", got, wantVersionLine)
	}
	gotPaths := fsrv.Paths()
	wantAsset := downloadPath(art.tag, hostArchive)
	wantSums := downloadPath(art.tag, checksumsName(art.version))
	if len(gotPaths) != 2 {
		t.Fatalf("fixture requests = %v, want exactly [%s %s]", gotPaths, wantAsset, wantSums)
	}
	seen := map[string]bool{}
	for _, p := range gotPaths {
		seen[p] = true
	}
	if !seen[wantAsset] || !seen[wantSums] {
		t.Fatalf("fixture requests = %v, want %s and %s", gotPaths, wantAsset, wantSums)
	}
}

func TestInstallScript_ChecksumMismatchRefuses(t *testing.T) {
	art := stampedArtifacts(t)
	hostArchive := hostArchiveName(t, art.version)
	tmp := newMutableReleaseCopy(t, art)
	path := filepath.Join(tmp, hostArchive)
	body, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		t.Fatalf("reading host archive: %v", err)
	}
	body[0] ^= 0xff
	if err := os.WriteFile(path, body, 0o644); err != nil { //nolint:gosec
		t.Fatalf("writing flipped archive: %v", err)
	}
	home := SandboxHome(t)
	fsrv := startReleaseFixtureServer(t, tmp, art.tag)
	res := runInstallScript(t, home, os.Getenv("PATH"), fsrv.srv.URL, art.tag)
	if res.err == nil {
		t.Fatalf("install.sh succeeded on checksum mismatch; stdout:\n%s", res.stdout)
	}
	if !strings.Contains(res.stderr, "checksum verification FAILED") {
		t.Fatalf("stderr missing checksum refusal:\n%s", res.stderr)
	}
	if _, err := os.Stat(filepath.Join(home, ".local", "bin", "gitid")); !os.IsNotExist(err) {
		t.Fatalf("refused install still left ~/.local/bin/gitid (err=%v)", err)
	}
}

func TestInstallScript_PathHintWhenNotOnPath(t *testing.T) {
	art := stampedArtifacts(t)
	_ = hostArchiveName(t, art.version)
	home := SandboxHome(t)
	fsrv := startReleaseFixtureServer(t, art.distDir, art.tag)
	res := runInstallScript(t, home, os.Getenv("PATH"), fsrv.srv.URL, art.tag)
	if res.err != nil {
		t.Fatalf("install.sh: %v\nstdout:\n%s\nstderr:\n%s", res.err, res.stdout, res.stderr)
	}
	hintDir := filepath.Join(home, ".local", "bin")
	want := `  PATH: ` + hintDir + ` is NOT on your PATH — add to shell: export PATH="$PATH:` + hintDir + `"`
	if !strings.Contains(res.stdout, want) {
		t.Fatalf("stdout missing PATH hint %q:\n%s", want, res.stdout)
	}
}

func TestInstallScript_PathOkWhenOnPath(t *testing.T) {
	art := stampedArtifacts(t)
	_ = hostArchiveName(t, art.version)
	home := SandboxHome(t)
	hintDir := filepath.Join(home, ".local", "bin")
	fsrv := startReleaseFixtureServer(t, art.distDir, art.tag)
	res := runInstallScript(t, home, hintDir+string(os.PathListSeparator)+os.Getenv("PATH"), fsrv.srv.URL, art.tag)
	if res.err != nil {
		t.Fatalf("install.sh: %v\nstdout:\n%s\nstderr:\n%s", res.err, res.stdout, res.stderr)
	}
	want := "  PATH: OK (gitid is on PATH)"
	if !strings.Contains(res.stdout, want) {
		t.Fatalf("stdout missing PATH OK line:\n%s", res.stdout)
	}
}

func fakeUnameDir(t *testing.T, sysname, machine string) string {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\n" +
		"case \"$1\" in\n" +
		"  -s) printf '%s\\n' '" + sysname + "' ;;\n" +
		"  -m) printf '%s\\n' '" + machine + "' ;;\n" +
		"  *) printf '%s\\n' '" + sysname + "' ;;\n" +
		"esac\n"
	path := filepath.Join(dir, "uname")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil { //nolint:gosec
		t.Fatalf("writing fake uname: %v", err)
	}
	return dir
}

func minimalToolPath(t *testing.T, tools []string) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range tools {
		resolved, err := exec.LookPath(name)
		if err != nil {
			t.Skipf("required tool %q is unavailable on this host", name)
		}
		if err := os.Symlink(resolved, filepath.Join(dir, name)); err != nil {
			t.Fatalf("symlink %s: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "sha256sum")); err == nil {
		t.Fatal("curated PATH unexpectedly contains sha256sum")
	}
	return dir
}

func fileSHA256(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func assertNoGitidUnder(t *testing.T, root string) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Name() == "gitid" {
			return fmt.Errorf("leaked gitid at %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestInstallScript_OSArchMatrix(t *testing.T) {
	art := stampedArtifacts(t)
	cases := []struct {
		sysname, machine, osTag, archTag string
	}{
		{"Darwin", "x86_64", "darwin", "amd64"},
		{"Darwin", "amd64", "darwin", "amd64"},
		{"Darwin", "arm64", "darwin", "arm64"},
		{"Darwin", "aarch64", "darwin", "arm64"},
		{"Linux", "x86_64", "linux", "amd64"},
		{"Linux", "amd64", "linux", "amd64"},
		{"Linux", "arm64", "linux", "arm64"},
		{"Linux", "aarch64", "linux", "arm64"},
	}
	for _, tc := range cases {
		t.Run(tc.sysname+"/"+tc.machine, func(t *testing.T) {
			home := SandboxHome(t)
			fsrv := startReleaseFixtureServer(t, art.distDir, art.tag)
			unameDir := fakeUnameDir(t, tc.sysname, tc.machine)
			path := unameDir + string(os.PathListSeparator) + os.Getenv("PATH")
			res := runInstallScript(t, home, path, fsrv.srv.URL, art.tag)
			if res.err != nil {
				t.Fatalf("install.sh: %v\nstdout:\n%s\nstderr:\n%s", res.err, res.stdout, res.stderr)
			}
			wantArchive := archiveName(art.version, tc.osTag, tc.archTag)
			wantAsset := downloadPath(art.tag, wantArchive)
			seen := map[string]bool{}
			for _, p := range fsrv.Paths() {
				seen[p] = true
			}
			if !seen[wantAsset] {
				t.Fatalf("fixture requests = %v, want %s", fsrv.Paths(), wantAsset)
			}
			installed := filepath.Join(home, ".local", "bin", "gitid")
			got := fileSHA256(t, installed)
			want := gitidMemberSHA256(t, filepath.Join(art.distDir, wantArchive))
			if got != want {
				t.Fatalf("installed SHA-256 %s != archive %s member %s", got, wantArchive, want)
			}
		})
	}
}

func TestInstallScript_UnsupportedOSRefuses(t *testing.T) {
	art := stampedArtifacts(t)
	home := SandboxHome(t)
	fsrv := startReleaseFixtureServer(t, art.distDir, art.tag)
	unameDir := fakeUnameDir(t, "Plan9", "amd64")
	path := unameDir + string(os.PathListSeparator) + os.Getenv("PATH")
	res := runInstallScript(t, home, path, fsrv.srv.URL, art.tag)
	if res.err == nil {
		t.Fatalf("install.sh succeeded for Plan9; stdout:\n%s", res.stdout)
	}
	if !strings.Contains(res.stderr, "unsupported operating system: Plan9") {
		t.Fatalf("stderr missing OS refusal:\n%s", res.stderr)
	}
	if got := fsrv.Paths(); len(got) != 0 {
		t.Fatalf("fixture recorded %v, want zero requests before download", got)
	}
	if _, err := os.Stat(filepath.Join(home, ".local", "bin", "gitid")); !os.IsNotExist(err) {
		t.Fatalf("refused OS still installed gitid (err=%v)", err)
	}
}

func TestInstallScript_UnsupportedArchRefuses(t *testing.T) {
	art := stampedArtifacts(t)
	for _, arch := range []string{"i386", "ppc64"} {
		t.Run(arch, func(t *testing.T) {
			home := SandboxHome(t)
			fsrv := startReleaseFixtureServer(t, art.distDir, art.tag)
			unameDir := fakeUnameDir(t, "Linux", arch)
			path := unameDir + string(os.PathListSeparator) + os.Getenv("PATH")
			res := runInstallScript(t, home, path, fsrv.srv.URL, art.tag)
			if res.err == nil {
				t.Fatalf("install.sh succeeded for arch %s; stdout:\n%s", arch, res.stdout)
			}
			needle := "unsupported architecture: " + arch
			if !strings.Contains(res.stderr, needle) {
				t.Fatalf("stderr missing %q:\n%s", needle, res.stderr)
			}
			if got := fsrv.Paths(); len(got) != 0 {
				t.Fatalf("fixture recorded %v, want zero requests before download", got)
			}
			if _, err := os.Stat(filepath.Join(home, ".local", "bin", "gitid")); !os.IsNotExist(err) {
				t.Fatalf("refused arch still installed gitid (err=%v)", err)
			}
		})
	}
}

func TestInstallScript_MissingChecksumEntryRefuses(t *testing.T) {
	art := stampedArtifacts(t)
	hostArchive := hostArchiveName(t, art.version)
	tmp := newMutableReleaseCopy(t, art)
	manifestPath := filepath.Join(tmp, checksumsName(art.version))
	raw, err := os.ReadFile(manifestPath) //nolint:gosec
	if err != nil {
		t.Fatalf("reading checksums manifest: %v", err)
	}
	var kept []string
	for _, line := range strings.Split(strings.TrimRight(string(raw), "\n"), "\n") {
		if strings.HasSuffix(line, "  "+hostArchive) {
			continue
		}
		kept = append(kept, line)
	}
	if err := os.WriteFile(manifestPath, []byte(strings.Join(kept, "\n")+"\n"), 0o644); err != nil { //nolint:gosec
		t.Fatalf("writing filtered checksums manifest: %v", err)
	}
	home := SandboxHome(t)
	fsrv := startReleaseFixtureServer(t, tmp, art.tag)
	res := runInstallScript(t, home, os.Getenv("PATH"), fsrv.srv.URL, art.tag)
	if res.err == nil {
		t.Fatalf("install.sh succeeded with missing checksum entry; stdout:\n%s", res.stdout)
	}
	if !strings.Contains(res.stderr, "has no entry for ") || !strings.Contains(res.stderr, hostArchive) {
		t.Fatalf("stderr missing missing-entry refusal for %s:\n%s", hostArchive, res.stderr)
	}
	if _, err := os.Stat(filepath.Join(home, ".local", "bin", "gitid")); !os.IsNotExist(err) {
		t.Fatalf("missing-entry refusal still installed gitid (err=%v)", err)
	}
}

func TestInstallScript_DownloadFailureRefuses(t *testing.T) {
	art := stampedArtifacts(t)
	hostArchive := hostArchiveName(t, art.version)
	manifestName := checksumsName(art.version)
	cases := []struct {
		name string
		miss string
	}{
		{"asset-404", downloadPath(art.tag, hostArchive)},
		{"checksums-404", downloadPath(art.tag, manifestName)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := SandboxHome(t)
			root := t.TempDir()
			populateReleaseDownloadDir(t, root, art.distDir, art.tag)
			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == tc.miss {
					http.NotFound(w, r)
					return
				}
				http.FileServer(http.Dir(root)).ServeHTTP(w, r)
			})
			fsrv := startRecordingHandler(t, inner)
			res := runInstallScript(t, home, os.Getenv("PATH"), fsrv.srv.URL, art.tag)
			if res.err == nil {
				t.Fatalf("install.sh succeeded on 404 %s; stdout:\n%s", tc.miss, res.stdout)
			}
			if !strings.Contains(res.stderr, "download failed: ") {
				t.Fatalf("stderr missing download failure:\n%s", res.stderr)
			}
			if _, err := os.Stat(filepath.Join(home, ".local", "bin", "gitid")); !os.IsNotExist(err) {
				t.Fatalf("download failure still installed gitid (err=%v)", err)
			}
		})
	}
}

func TestInstallScript_OverwritesExistingInstall(t *testing.T) {
	art := stampedArtifacts(t)
	hostArchive := hostArchiveName(t, art.version)
	home := SandboxHome(t)
	destDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatalf("mkdir install dir: %v", err)
	}
	dest := filepath.Join(destDir, "gitid")
	if err := os.WriteFile(dest, []byte("not-gitid"), 0o644); err != nil { //nolint:gosec
		t.Fatalf("seeding existing install: %v", err)
	}
	fsrv := startReleaseFixtureServer(t, art.distDir, art.tag)
	res := runInstallScript(t, home, os.Getenv("PATH"), fsrv.srv.URL, art.tag)
	if res.err != nil {
		t.Fatalf("install.sh: %v\nstdout:\n%s\nstderr:\n%s", res.err, res.stdout, res.stderr)
	}
	got := fileSHA256(t, dest)
	want := gitidMemberSHA256(t, filepath.Join(art.distDir, hostArchive))
	if got != want {
		t.Fatalf("overwritten install SHA-256 %s != archive member %s", got, want)
	}
	info, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("stat overwritten install: %v", err)
	}
	if info.Mode()&0o777 != 0o755 {
		t.Fatalf("overwritten mode = %o, want 0755", info.Mode()&0o777)
	}
}

func TestInstallScript_CreatesInstallDir(t *testing.T) {
	art := stampedArtifacts(t)
	_ = hostArchiveName(t, art.version)
	home := SandboxHome(t)
	if _, err := os.Stat(filepath.Join(home, ".local")); !os.IsNotExist(err) {
		t.Fatalf("sandbox HOME already has .local (err=%v)", err)
	}
	fsrv := startReleaseFixtureServer(t, art.distDir, art.tag)
	res := runInstallScript(t, home, os.Getenv("PATH"), fsrv.srv.URL, art.tag)
	if res.err != nil {
		t.Fatalf("install.sh: %v\nstdout:\n%s\nstderr:\n%s", res.err, res.stdout, res.stderr)
	}
	if _, err := os.Stat(filepath.Join(home, ".local", "bin", "gitid")); err != nil {
		t.Fatalf("expected created install path: %v", err)
	}
}

func TestInstallScript_PipedIntoShellStillInstalls(t *testing.T) {
	art := stampedArtifacts(t)
	hostArchive := hostArchiveName(t, art.version)
	home := SandboxHome(t)
	fsrv := startReleaseFixtureServer(t, art.distDir, art.tag)
	res := runInstallScriptPiped(t, home, os.Getenv("PATH"), fsrv.srv.URL, art.tag)
	if res.err != nil {
		t.Fatalf("piped install.sh: %v\nstdout:\n%s\nstderr:\n%s", res.err, res.stdout, res.stderr)
	}
	if !strings.Contains(res.stdout, "  verified: SHA-256 ") {
		t.Fatalf("stdout missing verified line:\n%s", res.stdout)
	}
	installed := filepath.Join(home, ".local", "bin", "gitid")
	if !strings.Contains(res.stdout, "  installed: "+installed) {
		t.Fatalf("stdout missing installed line for %s:\n%s", installed, res.stdout)
	}
	info, err := os.Stat(installed)
	if err != nil {
		t.Fatalf("installed binary missing: %v", err)
	}
	if info.Mode()&0o777 != 0o755 {
		t.Fatalf("installed mode = %o, want 0755", info.Mode()&0o777)
	}
	got := fileSHA256(t, installed)
	want := gitidMemberSHA256(t, filepath.Join(art.distDir, hostArchive))
	if got != want {
		t.Fatalf("piped install SHA-256 %s != archive member %s", got, want)
	}
}

func TestInstallScript_FallsBackToShasumWhenSha256sumAbsent(t *testing.T) {
	art := stampedArtifacts(t)
	hostArchive := hostArchiveName(t, art.version)
	tools := []string{"curl", "mktemp", "grep", "cut", "head", "chmod", "mkdir", "mv", "rm", "uname", "tar", "shasum"}
	curated := minimalToolPath(t, tools)
	home := SandboxHome(t)
	fsrv := startReleaseFixtureServer(t, art.distDir, art.tag)
	res := runInstallScript(t, home, curated, fsrv.srv.URL, art.tag)
	if res.err != nil {
		t.Fatalf("install.sh without sha256sum: %v\nstdout:\n%s\nstderr:\n%s", res.err, res.stdout, res.stderr)
	}
	installed := filepath.Join(home, ".local", "bin", "gitid")
	got := fileSHA256(t, installed)
	want := gitidMemberSHA256(t, filepath.Join(art.distDir, hostArchive))
	if got != want {
		t.Fatalf("shasum-fallback SHA-256 %s != archive member %s", got, want)
	}
	if _, err := os.Stat(filepath.Join(curated, "sha256sum")); !os.IsNotExist(err) {
		t.Fatalf("curated PATH unexpectedly contains sha256sum (err=%v)", err)
	}
	t.Log("curated PATH lacked sha256sum; installer fell back to shasum")
}

func TestInstallScript_LeavesNothingBehindOnRefusal(t *testing.T) {
	art := stampedArtifacts(t)
	hostArchive := hostArchiveName(t, art.version)
	tmp := newMutableReleaseCopy(t, art)
	path := filepath.Join(tmp, hostArchive)
	body, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		t.Fatalf("reading host archive: %v", err)
	}
	body[0] ^= 0xff
	if err := os.WriteFile(path, body, 0o644); err != nil { //nolint:gosec
		t.Fatalf("writing flipped archive: %v", err)
	}
	home := SandboxHome(t)
	fsrv := startReleaseFixtureServer(t, tmp, art.tag)
	res := runInstallScript(t, home, os.Getenv("PATH"), fsrv.srv.URL, art.tag)
	if res.err == nil {
		t.Fatalf("install.sh succeeded on checksum mismatch; stdout:\n%s", res.stdout)
	}
	assertNoGitidUnder(t, home)
}

// TestInstallScript_GITIDVersionPinsToASpecificVersion (REVIEW C-2, net-new):
// a fixture server exposing ONLY the versioned download path — no
// /releases/latest route registered at all — so if install.sh ever fell
// back to redirect-resolution instead of honoring the explicit GITID_VERSION
// pin, this test would fail on a download error rather than silently
// succeeding via the wrong path.
func TestInstallScript_GITIDVersionPinsToASpecificVersion(t *testing.T) {
	art := stampedArtifacts(t)
	hostArchive := hostArchiveName(t, art.version)
	home := SandboxHome(t)
	fsrv := startReleaseFixtureServer(t, art.distDir, art.tag)
	res := runInstallScript(t, home, os.Getenv("PATH"), fsrv.srv.URL, art.tag)
	if res.err != nil {
		t.Fatalf("install.sh: %v\nstdout:\n%s\nstderr:\n%s", res.err, res.stdout, res.stderr)
	}
	installed := filepath.Join(home, ".local", "bin", "gitid")
	got := fileSHA256(t, installed)
	want := gitidMemberSHA256(t, filepath.Join(art.distDir, hostArchive))
	if got != want {
		t.Fatalf("pinned-version install SHA-256 %s != archive member %s", got, want)
	}
	latestPath := "/" + releaseRepoPath + "/releases/latest"
	seen := map[string]bool{}
	for _, p := range fsrv.Paths() {
		seen[p] = true
	}
	if seen[latestPath] {
		t.Fatalf("GITID_VERSION pin still hit %s — redirect-resolution was not bypassed", latestPath)
	}
	wantAsset := downloadPath(art.tag, hostArchive)
	wantSums := downloadPath(art.tag, checksumsName(art.version))
	if !seen[wantAsset] || !seen[wantSums] {
		t.Fatalf("fixture requests = %v, want %s and %s", fsrv.Paths(), wantAsset, wantSums)
	}
}

// TestInstallScript_CustomInstallDirOverride (REVIEW C-2, net-new): the
// GITID_INSTALL_DIR override relocates the install target away from the
// hardcoded ~/.local/bin default — absent from the Phase 9.3 script.
func TestInstallScript_CustomInstallDirOverride(t *testing.T) {
	art := stampedArtifacts(t)
	hostArchive := hostArchiveName(t, art.version)
	home := SandboxHome(t)
	customDir := filepath.Join(home, "custom-gitid-bin")
	fsrv := startReleaseFixtureServer(t, art.distDir, art.tag)
	res := runInstallScriptFull(t, home, os.Getenv("PATH"), fsrv.srv.URL, art.tag, customDir, false)
	if res.err != nil {
		t.Fatalf("install.sh: %v\nstdout:\n%s\nstderr:\n%s", res.err, res.stdout, res.stderr)
	}
	installed := filepath.Join(customDir, "gitid")
	if !strings.Contains(res.stdout, "  installed: "+installed) {
		t.Fatalf("stdout missing installed line for %s:\n%s", installed, res.stdout)
	}
	got := fileSHA256(t, installed)
	want := gitidMemberSHA256(t, filepath.Join(art.distDir, hostArchive))
	if got != want {
		t.Fatalf("custom-install-dir SHA-256 %s != archive member %s", got, want)
	}
	if _, err := os.Stat(filepath.Join(home, ".local", "bin", "gitid")); !os.IsNotExist(err) {
		t.Fatalf("custom install dir override still populated the default ~/.local/bin (err=%v)", err)
	}
}

// TestInstallScript_ResolvesLatestViaGitHubRedirect (REVIEW C-2, net-new):
// GITID_VERSION is deliberately UNSET. GITID_INSTALL_BASE_URL points at a
// fixture server that mirrors GitHub's real /releases/latest ->
// /releases/tag/<tag> 302 redirect shape — the ONE test in this suite that
// drives install.sh's redirect-resolution/tag-stripping logic.
func TestInstallScript_ResolvesLatestViaGitHubRedirect(t *testing.T) {
	art := stampedArtifacts(t)
	hostArchive := hostArchiveName(t, art.version)
	home := SandboxHome(t)
	fsrv := startGitHubRedirectFixtureServer(t, art.distDir, art.tag)
	res := runInstallScriptFull(t, home, os.Getenv("PATH"), fsrv.srv.URL, "", "", false)
	if res.err != nil {
		t.Fatalf("install.sh: %v\nstdout:\n%s\nstderr:\n%s", res.err, res.stdout, res.stderr)
	}
	installed := filepath.Join(home, ".local", "bin", "gitid")
	got := fileSHA256(t, installed)
	want := gitidMemberSHA256(t, filepath.Join(art.distDir, hostArchive))
	if got != want {
		t.Fatalf("redirect-resolved install SHA-256 %s != archive member %s", got, want)
	}
	latestPath := "/" + releaseRepoPath + "/releases/latest"
	tagPagePath := "/" + releaseRepoPath + "/releases/tag/" + art.tag
	wantAsset := downloadPath(art.tag, hostArchive)
	wantSums := downloadPath(art.tag, checksumsName(art.version))
	gotPaths := fsrv.Paths()
	seen := map[string]bool{}
	for _, p := range gotPaths {
		seen[p] = true
	}
	for _, want := range []string{latestPath, tagPagePath, wantAsset, wantSums} {
		if !seen[want] {
			t.Fatalf("fixture requests = %v, want %s among them", gotPaths, want)
		}
	}
	if len(gotPaths) != 4 {
		t.Fatalf("fixture requests = %v, want exactly 4 (latest redirect, tag page, asset, checksums)", gotPaths)
	}
}

func copyDir(t *testing.T, src, dst string, names ...string) {
	t.Helper()
	for _, name := range names {
		in, err := os.Open(filepath.Join(src, name)) //nolint:gosec
		if err != nil {
			t.Fatalf("open %s: %v", name, err)
		}
		out, err := os.Create(filepath.Join(dst, name)) //nolint:gosec
		if err != nil {
			_ = in.Close()
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := io.Copy(out, in); err != nil {
			_ = in.Close()
			_ = out.Close()
			t.Fatalf("copy %s: %v", name, err)
		}
		_ = in.Close()
		if err := out.Close(); err != nil {
			t.Fatalf("close %s: %v", name, err)
		}
	}
}
