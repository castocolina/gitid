//go:build e2e

package e2e

import (
	"bytes"
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
	e2eStampLine    = "gitid version 9.9.9-e2e (deadbee, 2001-02-03)"
)

var publishedAssets = []string{
	"gitid-darwin-amd64",
	"gitid-darwin-arm64",
	"gitid-linux-amd64",
	"gitid-linux-arm64",
}

var (
	stampedOnce   sync.Once
	stampedBinDir string
	stampedErr    error
)

func stampedArtifacts(t *testing.T) string {
	t.Helper()
	stampedOnce.Do(func() {
		root := repoRoot(t)
		ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second*ciTimeoutMultiplier())
		defer cancel()
		cmd := exec.CommandContext(ctx, "make", "checksums",
			"VERSION="+e2eStampVersion,
			"COMMIT="+e2eStampCommit,
			"DATE="+e2eStampDate,
		)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "HOME="+realHome)
		out, err := cmd.CombinedOutput()
		if err != nil {
			stampedErr = fmt.Errorf("make checksums: %w\n%s", err, out)
			return
		}
		stampedBinDir = filepath.Join(root, "bin")
	})
	if stampedErr != nil {
		t.Fatal(stampedErr)
	}
	return stampedBinDir
}

func hostAsset(t *testing.T) string {
	t.Helper()
	name := "gitid-" + runtime.GOOS + "-" + runtime.GOARCH
	for _, asset := range publishedAssets {
		if asset == name {
			return name
		}
	}
	t.Skipf("host %s/%s is outside the published matrix", runtime.GOOS, runtime.GOARCH)
	return ""
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

type scriptResult struct {
	stdout string
	stderr string
	err    error
}

func runInstallScript(t *testing.T, home, path, baseURL string) scriptResult {
	t.Helper()
	return runInstallScriptCmd(t, home, path, baseURL, false)
}

func runInstallScriptCmd(t *testing.T, home, path, baseURL string, piped bool) scriptResult {
	t.Helper()
	prefixes, replacePath := installPathParts(path)
	env, _ := e2eEnv(t, home, prefixes...)
	env = append(env, "GITID_INSTALL_BASE_URL="+baseURL)
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

func TestRelease_BuildCrossStampsEveryTarget(t *testing.T) {
	binDir := stampedArtifacts(t)
	for _, name := range publishedAssets {
		path := filepath.Join(binDir, name)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("%s missing: %v", name, err)
		}
		if info.Size() == 0 {
			t.Fatalf("%s is empty", name)
		}
	}
	host := hostAsset(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second*ciTimeoutMultiplier())
	defer cancel()
	cmd := exec.CommandContext(ctx, filepath.Join(binDir, host), "--version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s --version: %v\n%s", host, err, out)
	}
	got := strings.TrimSpace(string(out))
	if got != e2eStampLine {
		t.Fatalf("%s --version = %q, want %q", host, got, e2eStampLine)
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

func TestRelease_ChecksumsManifestMatchesBinaries(t *testing.T) {
	binDir := stampedArtifacts(t)
	raw, err := os.ReadFile(filepath.Join(binDir, "checksums.txt")) //nolint:gosec
	if err != nil {
		t.Fatalf("reading checksums.txt: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("checksums.txt has %d lines, want 4:\n%s", len(lines), raw)
	}
	pat := regexp.MustCompile(`^[0-9a-f]{64}  gitid-(darwin|linux)-(amd64|arm64)$`)
	seen := map[string]string{}
	for _, line := range lines {
		if !pat.MatchString(line) {
			t.Fatalf("checksums.txt line %q does not match anchored two-space asset pattern", line)
		}
		hash, name, ok := strings.Cut(line, "  ")
		if !ok {
			t.Fatalf("checksums.txt line %q missing two-space separator", line)
		}
		if _, dup := seen[name]; dup {
			t.Fatalf("duplicate asset %q in checksums.txt", name)
		}
		seen[name] = hash
		body, err := os.ReadFile(filepath.Join(binDir, name)) //nolint:gosec
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		sum := sha256.Sum256(body)
		got := hex.EncodeToString(sum[:])
		if got != hash {
			t.Fatalf("%s: recomputed sha256 %s != manifest %s", name, got, hash)
		}
	}
	for _, name := range publishedAssets {
		if _, ok := seen[name]; !ok {
			t.Fatalf("checksums.txt missing published asset %q", name)
		}
	}
	if len(seen) != 4 {
		t.Fatalf("checksums.txt named %d assets, want 4", len(seen))
	}
}

func TestInstallScript_InstallsVerifiedHostBinary(t *testing.T) {
	binDir := stampedArtifacts(t)
	host := hostAsset(t)
	home := SandboxHome(t)
	fs := startFixtureServer(t, binDir)
	res := runInstallScript(t, home, os.Getenv("PATH"), fs.srv.URL)
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
	if got := strings.TrimSpace(string(out)); got != e2eStampLine {
		t.Fatalf("installed --version = %q, want %q", got, e2eStampLine)
	}
	gotPaths := fs.Paths()
	wantAsset := "/" + host
	wantSums := "/checksums.txt"
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
	binDir := stampedArtifacts(t)
	host := hostAsset(t)
	tmp := t.TempDir()
	names := append(append([]string{}, publishedAssets...), "checksums.txt")
	copyDir(t, binDir, tmp, names...)
	path := filepath.Join(tmp, host)
	body, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		t.Fatalf("reading host asset: %v", err)
	}
	body[0] ^= 0xff
	if err := os.WriteFile(path, body, 0o755); err != nil { //nolint:gosec
		t.Fatalf("writing flipped asset: %v", err)
	}
	home := SandboxHome(t)
	fs := startFixtureServer(t, tmp)
	res := runInstallScript(t, home, os.Getenv("PATH"), fs.srv.URL)
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
	binDir := stampedArtifacts(t)
	_ = hostAsset(t)
	home := SandboxHome(t)
	fs := startFixtureServer(t, binDir)
	res := runInstallScript(t, home, os.Getenv("PATH"), fs.srv.URL)
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
	binDir := stampedArtifacts(t)
	_ = hostAsset(t)
	home := SandboxHome(t)
	hintDir := filepath.Join(home, ".local", "bin")
	fs := startFixtureServer(t, binDir)
	res := runInstallScript(t, home, hintDir+string(os.PathListSeparator)+os.Getenv("PATH"), fs.srv.URL)
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

func runInstallScriptPiped(t *testing.T, home, path, baseURL string) scriptResult {
	t.Helper()
	return runInstallScriptCmd(t, home, path, baseURL, true)
}

func TestInstallScript_OSArchMatrix(t *testing.T) {
	binDir := stampedArtifacts(t)
	cases := []struct {
		sysname, machine, asset string
	}{
		{"Darwin", "x86_64", "gitid-darwin-amd64"},
		{"Darwin", "amd64", "gitid-darwin-amd64"},
		{"Darwin", "arm64", "gitid-darwin-arm64"},
		{"Darwin", "aarch64", "gitid-darwin-arm64"},
		{"Linux", "x86_64", "gitid-linux-amd64"},
		{"Linux", "amd64", "gitid-linux-amd64"},
		{"Linux", "arm64", "gitid-linux-arm64"},
		{"Linux", "aarch64", "gitid-linux-arm64"},
	}
	for _, tc := range cases {
		t.Run(tc.sysname+"/"+tc.machine, func(t *testing.T) {
			home := SandboxHome(t)
			fsrv := startFixtureServer(t, binDir)
			unameDir := fakeUnameDir(t, tc.sysname, tc.machine)
			path := unameDir + string(os.PathListSeparator) + os.Getenv("PATH")
			res := runInstallScript(t, home, path, fsrv.srv.URL)
			if res.err != nil {
				t.Fatalf("install.sh: %v\nstdout:\n%s\nstderr:\n%s", res.err, res.stdout, res.stderr)
			}
			wantAsset := "/" + tc.asset
			seen := map[string]bool{}
			for _, p := range fsrv.Paths() {
				seen[p] = true
			}
			if !seen[wantAsset] {
				t.Fatalf("fixture requests = %v, want %s", fsrv.Paths(), wantAsset)
			}
			installed := filepath.Join(home, ".local", "bin", "gitid")
			got := fileSHA256(t, installed)
			want := fileSHA256(t, filepath.Join(binDir, tc.asset))
			if got != want {
				t.Fatalf("installed SHA-256 %s != asset %s %s", got, tc.asset, want)
			}
		})
	}
}

func TestInstallScript_UnsupportedOSRefuses(t *testing.T) {
	binDir := stampedArtifacts(t)
	home := SandboxHome(t)
	fsrv := startFixtureServer(t, binDir)
	unameDir := fakeUnameDir(t, "Plan9", "amd64")
	path := unameDir + string(os.PathListSeparator) + os.Getenv("PATH")
	res := runInstallScript(t, home, path, fsrv.srv.URL)
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
	binDir := stampedArtifacts(t)
	for _, arch := range []string{"i386", "ppc64"} {
		t.Run(arch, func(t *testing.T) {
			home := SandboxHome(t)
			fsrv := startFixtureServer(t, binDir)
			unameDir := fakeUnameDir(t, "Linux", arch)
			path := unameDir + string(os.PathListSeparator) + os.Getenv("PATH")
			res := runInstallScript(t, home, path, fsrv.srv.URL)
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
	binDir := stampedArtifacts(t)
	host := hostAsset(t)
	tmp := t.TempDir()
	names := append(append([]string{}, publishedAssets...), "checksums.txt")
	copyDir(t, binDir, tmp, names...)
	raw, err := os.ReadFile(filepath.Join(tmp, "checksums.txt")) //nolint:gosec
	if err != nil {
		t.Fatalf("reading checksums.txt: %v", err)
	}
	var kept []string
	for _, line := range strings.Split(strings.TrimRight(string(raw), "\n"), "\n") {
		if strings.HasSuffix(line, "  "+host) {
			continue
		}
		kept = append(kept, line)
	}
	if err := os.WriteFile(filepath.Join(tmp, "checksums.txt"), []byte(strings.Join(kept, "\n")+"\n"), 0o644); err != nil { //nolint:gosec
		t.Fatalf("writing filtered checksums.txt: %v", err)
	}
	home := SandboxHome(t)
	fsrv := startFixtureServer(t, tmp)
	res := runInstallScript(t, home, os.Getenv("PATH"), fsrv.srv.URL)
	if res.err == nil {
		t.Fatalf("install.sh succeeded with missing checksum entry; stdout:\n%s", res.stdout)
	}
	if !strings.Contains(res.stderr, "has no entry for ") || !strings.Contains(res.stderr, host) {
		t.Fatalf("stderr missing missing-entry refusal for %s:\n%s", host, res.stderr)
	}
	if _, err := os.Stat(filepath.Join(home, ".local", "bin", "gitid")); !os.IsNotExist(err) {
		t.Fatalf("missing-entry refusal still installed gitid (err=%v)", err)
	}
}

func TestInstallScript_DownloadFailureRefuses(t *testing.T) {
	binDir := stampedArtifacts(t)
	host := hostAsset(t)
	cases := []struct {
		name string
		miss string
	}{
		{"asset-404", "/" + host},
		{"checksums-404", "/checksums.txt"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			home := SandboxHome(t)
			inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == tc.miss {
					http.NotFound(w, r)
					return
				}
				http.FileServer(http.Dir(binDir)).ServeHTTP(w, r)
			})
			fsrv := startRecordingHandler(t, inner)
			res := runInstallScript(t, home, os.Getenv("PATH"), fsrv.srv.URL)
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
	binDir := stampedArtifacts(t)
	host := hostAsset(t)
	home := SandboxHome(t)
	destDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatalf("mkdir install dir: %v", err)
	}
	dest := filepath.Join(destDir, "gitid")
	if err := os.WriteFile(dest, []byte("not-gitid"), 0o644); err != nil { //nolint:gosec
		t.Fatalf("seeding existing install: %v", err)
	}
	fsrv := startFixtureServer(t, binDir)
	res := runInstallScript(t, home, os.Getenv("PATH"), fsrv.srv.URL)
	if res.err != nil {
		t.Fatalf("install.sh: %v\nstdout:\n%s\nstderr:\n%s", res.err, res.stdout, res.stderr)
	}
	got := fileSHA256(t, dest)
	want := fileSHA256(t, filepath.Join(binDir, host))
	if got != want {
		t.Fatalf("overwritten install SHA-256 %s != served asset %s", got, want)
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
	binDir := stampedArtifacts(t)
	_ = hostAsset(t)
	home := SandboxHome(t)
	if _, err := os.Stat(filepath.Join(home, ".local")); !os.IsNotExist(err) {
		t.Fatalf("sandbox HOME already has .local (err=%v)", err)
	}
	fsrv := startFixtureServer(t, binDir)
	res := runInstallScript(t, home, os.Getenv("PATH"), fsrv.srv.URL)
	if res.err != nil {
		t.Fatalf("install.sh: %v\nstdout:\n%s\nstderr:\n%s", res.err, res.stdout, res.stderr)
	}
	if _, err := os.Stat(filepath.Join(home, ".local", "bin", "gitid")); err != nil {
		t.Fatalf("expected created install path: %v", err)
	}
}

func TestInstallScript_PipedIntoShellStillInstalls(t *testing.T) {
	binDir := stampedArtifacts(t)
	host := hostAsset(t)
	home := SandboxHome(t)
	fsrv := startFixtureServer(t, binDir)
	res := runInstallScriptPiped(t, home, os.Getenv("PATH"), fsrv.srv.URL)
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
	want := fileSHA256(t, filepath.Join(binDir, host))
	if got != want {
		t.Fatalf("piped install SHA-256 %s != served asset %s", got, want)
	}
}

func TestInstallScript_FallsBackToShasumWhenSha256sumAbsent(t *testing.T) {
	binDir := stampedArtifacts(t)
	host := hostAsset(t)
	tools := []string{"curl", "mktemp", "grep", "cut", "head", "chmod", "mkdir", "mv", "rm", "uname", "shasum"}
	curated := minimalToolPath(t, tools)
	home := SandboxHome(t)
	fsrv := startFixtureServer(t, binDir)
	res := runInstallScript(t, home, curated, fsrv.srv.URL)
	if res.err != nil {
		t.Fatalf("install.sh without sha256sum: %v\nstdout:\n%s\nstderr:\n%s", res.err, res.stdout, res.stderr)
	}
	installed := filepath.Join(home, ".local", "bin", "gitid")
	got := fileSHA256(t, installed)
	want := fileSHA256(t, filepath.Join(binDir, host))
	if got != want {
		t.Fatalf("shasum-fallback SHA-256 %s != served asset %s", got, want)
	}
	if _, err := os.Stat(filepath.Join(curated, "sha256sum")); !os.IsNotExist(err) {
		t.Fatalf("curated PATH unexpectedly contains sha256sum (err=%v)", err)
	}
	t.Log("curated PATH lacked sha256sum; installer fell back to shasum")
}

func TestInstallScript_LeavesNothingBehindOnRefusal(t *testing.T) {
	binDir := stampedArtifacts(t)
	host := hostAsset(t)
	tmp := t.TempDir()
	names := append(append([]string{}, publishedAssets...), "checksums.txt")
	copyDir(t, binDir, tmp, names...)
	path := filepath.Join(tmp, host)
	body, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		t.Fatalf("reading host asset: %v", err)
	}
	body[0] ^= 0xff
	if err := os.WriteFile(path, body, 0o755); err != nil { //nolint:gosec
		t.Fatalf("writing flipped asset: %v", err)
	}
	home := SandboxHome(t)
	fsrv := startFixtureServer(t, tmp)
	res := runInstallScript(t, home, os.Getenv("PATH"), fsrv.srv.URL)
	if res.err == nil {
		t.Fatalf("install.sh succeeded on checksum mismatch; stdout:\n%s", res.stdout)
	}
	assertNoGitidUnder(t, home)
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
