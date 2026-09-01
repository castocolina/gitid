//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
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
		ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
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
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/bin/sh", "scripts/install.sh")
	cmd.Dir = repoRoot(t)
	cmd.Env = []string{
		"HOME=" + home,
		"PATH=" + path,
		"GITID_INSTALL_BASE_URL=" + baseURL,
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return scriptResult{stdout: stdout.String(), stderr: stderr.String(), err: err}
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "make", "build")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "HOME="+realHome)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("make build: %v\n%s", err, out)
	}
	verCtx, verCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer verCancel()
	ver := exec.CommandContext(verCtx, filepath.Join(root, "bin", "gitid"), "--version")
	out, err := ver.CombinedOutput()
	if err != nil {
		t.Fatalf("bin/gitid --version: %v\n%s", err, out)
	}
	got := strings.TrimSpace(string(out))
	want := "gitid version 0.0.0-dev (none, unknown)"
	if got != want {
		t.Fatalf("unstamped --version = %q, want %q", got, want)
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
