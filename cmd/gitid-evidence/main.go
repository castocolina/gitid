//go:build screenshot

// gitid-evidence publishes a content-addressed Phase 3 visual evidence packet.
package main

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"

	"github.com/castocolina/gitid/internal/screenshot"
)

const fixedCaptureTime = "2026-08-21T00:00:00Z"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "gitid-evidence: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	candidate := false
	filtered := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--candidate" {
			candidate = true
			continue
		}
		filtered = append(filtered, arg)
	}
	sourceCommit, outputPath, err := parseArgs(filtered, candidate)
	if err != nil {
		return err
	}
	if candidate {
		return generateCandidate(sourceCommit, outputPath)
	}
	return publish(sourceCommit, outputPath)
}

func parseArgs(args []string, candidate bool) (sourceCommit, outputPath string, err error) {
	pathFlag := "--output-root"
	if candidate {
		pathFlag = "--output-dir"
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--source-commit", "-source-commit":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("--source-commit requires a value")
			}
			i++
			sourceCommit = args[i]
		case pathFlag:
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("%s requires a value", pathFlag)
			}
			i++
			outputPath = args[i]
		default:
			return "", "", fmt.Errorf("unknown argument %q", args[i])
		}
	}
	if sourceCommit == "" {
		return "", "", fmt.Errorf("--source-commit <full-40-hex-sha> is required")
	}
	if outputPath == "" {
		return "", "", fmt.Errorf("%s is required", pathFlag)
	}
	if err := validateSourceCommitFormat(sourceCommit); err != nil {
		return "", "", err
	}
	return sourceCommit, outputPath, nil
}

func publish(sourceCommit, outputRoot string) error {
	repoRoot, err := repositoryRoot()
	if err != nil {
		return err
	}
	if err := validateCurrentSource(repoRoot, sourceCommit); err != nil {
		return err
	}
	if err := prepareOutputRoot(outputRoot); err != nil {
		return err
	}
	destDir := filepath.Join(outputRoot, sourceCommit)
	if _, err := os.Stat(destDir); err == nil {
		return fmt.Errorf("destination %q already exists — refusing to overwrite", destDir)
	}

	parent := filepath.Dir(outputRoot)
	candidateRoots := make([]string, 0, 2)
	defer func() {
		for _, root := range candidateRoots {
			_ = os.RemoveAll(root)
		}
	}()
	for range 2 {
		root, err := os.MkdirTemp(parent, ".gitid-evidence-candidate-")
		if err != nil {
			return fmt.Errorf("creating candidate root: %w", err)
		}
		candidateRoots = append(candidateRoots, root)
	}
	candidateDirs := []string{
		filepath.Join(candidateRoots[0], sourceCommit),
		filepath.Join(candidateRoots[1], sourceCommit),
	}
	for _, candidateDir := range candidateDirs {
		if err := runCandidateProcess(repoRoot, sourceCommit, candidateDir); err != nil {
			return err
		}
		if _, err := screenshot.ValidatePacket(candidateDir); err != nil {
			return fmt.Errorf("validating candidate %q: %w", candidateDir, err)
		}
	}
	if err := compareInventories(candidateDirs[0], candidateDirs[1]); err != nil {
		return fmt.Errorf("two-process determinism check failed: %w", err)
	}
	if err := os.Rename(candidateDirs[0], destDir); err != nil {
		return fmt.Errorf("atomically publishing %q: %w", destDir, err)
	}
	if err := syncDir(outputRoot); err != nil {
		return fmt.Errorf("syncing published packet directory: %w", err)
	}
	pkt, err := screenshot.ValidatePacket(destDir)
	if err != nil {
		return fmt.Errorf("validating published packet: %w", err)
	}
	fmt.Printf("gitid-evidence: published %d PNG panels to %s\n", screenshot.ValidatePanelCount, destDir)
	fmt.Printf("gitid-evidence: manifest SHA-256 = %s\n", pkt.ManifestSHA256)
	return nil
}

func prepareOutputRoot(root string) error {
	info, err := os.Stat(root)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("output root %q is not a directory", root)
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			return fmt.Errorf("reading output root %q: %w", root, err)
		}
		if len(entries) != 0 {
			return fmt.Errorf("output root %q is nonempty — refusing to publish into it", root)
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("checking output root %q: %w", root, err)
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		return fmt.Errorf("creating output root %q: %w", root, err)
	}
	return nil
}

func runCandidateProcess(repoRoot, sourceCommit, outputDir string) error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locating evidence executable: %w", err)
	}
	cmd := exec.Command(self, "--candidate", "--source-commit", sourceCommit, "--output-dir", outputDir) //nolint:gosec // all args are locally validated paths and full commit SHA
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "GITID_EVIDENCE_REPO_ROOT="+repoRoot)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("candidate process failed: %w\n%s", err, output)
	}
	return nil
}

func generateCandidate(sourceCommit, outputDir string) error {
	repoRoot := os.Getenv("GITID_EVIDENCE_REPO_ROOT")
	if repoRoot == "" {
		var err error
		repoRoot, err = repositoryRoot()
		if err != nil {
			return err
		}
	}
	if err := validateCurrentSource(repoRoot, sourceCommit); err != nil {
		return err
	}
	if _, err := os.Stat(outputDir); err == nil {
		return fmt.Errorf("candidate output %q already exists", outputDir)
	}

	fontFile := filepath.Join(repoRoot, ".planning", "design", "fonts", "JetBrainsMono-Regular.ttf")
	font, err := os.ReadFile(fontFile) //nolint:gosec // repository-owned fixed asset
	if err != nil {
		return fmt.Errorf("reading vendored capture font: %w", err)
	}
	freeze, err := resolveFreeze()
	if err != nil {
		return fmt.Errorf("freeze is required for evidence capture: %w", err)
	}
	freezeVersion, err := commandOutput(freeze, "--version")
	if err != nil {
		return fmt.Errorf("reading freeze version: %w", err)
	}
	goVersion, err := commandOutput("go", "version")
	if err != nil {
		return fmt.Errorf("reading Go version: %w", err)
	}
	pnpmVersion, err := commandOutput("pnpm", "--version")
	if err != nil {
		return fmt.Errorf("reading pnpm version: %w", err)
	}

	workspace, err := os.MkdirTemp("", "gitid-evidence-capture-")
	if err != nil {
		return fmt.Errorf("creating capture workspace: %w", err)
	}
	defer os.RemoveAll(workspace)
	renderDir := filepath.Join(workspace, "rendered")
	live, err := captureLivePanels(repoRoot, workspace, renderDir, freeze, fontFile)
	if err != nil {
		return err
	}
	approvalDir, err := exportApprovalSource(repoRoot, workspace)
	if err != nil {
		return err
	}
	approvedTUI, err := captureApprovedTUIPanels(approvalDir, workspace, renderDir, freeze, fontFile)
	if err != nil {
		return err
	}
	approvedHTML, err := captureApprovedHTMLPanels(repoRoot, approvalDir, workspace, renderDir)
	if err != nil {
		return err
	}
	panels := append(live, approvedTUI...)
	panels = append(panels, approvedHTML...)

	capture := screenshot.PacketCapture{
		Commands: []string{
			"git archive " + screenshot.PacketApprovalCommit,
			"go build -o <live-gitid> ./cmd/gitid",
			"<live-gitid> (PTY, HOME=<sandbox>, PATH=<fake-ssh>)",
			"go build -o <approved-gitid-dummy> ./cmd/gitid-dummy",
			"<approved-gitid-dummy> (PTY)",
			"pnpm exec vite build --outDir <approval-dist>",
			"freeze <capture.txt> -o <panel.png> --font.file JetBrainsMono-Regular.ttf --theme dracula",
		},
		ToolVersions: []screenshot.PacketTool{
			{Name: "freeze", Version: strings.TrimSpace(freezeVersion)},
			{Name: "go", Version: strings.TrimSpace(goVersion)},
			{Name: "pnpm", Version: strings.TrimSpace(pnpmVersion)},
			{Name: "chromium-revision", Version: fmt.Sprintf("%d", screenshot.ChromiumRevision)},
		},
		Geometry:   "TUI=100x30;HTML=1280x800@1/light",
		FontSHA256: sha256Hex(font),
		Theme:      "dracula",
	}
	result, err := screenshot.GenerateVisualPacket(screenshot.PacketOptions{
		SourceCommit:   sourceCommit,
		ApprovalCommit: screenshot.PacketApprovalCommit,
		LiveBackendRef: sourceCommit,
		OutputDir:      outputDir,
		Clock: func() time.Time {
			return mustParseTime(fixedCaptureTime)
		},
	}, panels, capture)
	if err != nil {
		return err
	}
	if _, err := screenshot.ValidatePacket(filepath.Dir(result.ManifestPath)); err != nil {
		return fmt.Errorf("validating generated candidate: %w", err)
	}
	return nil
}

func resolveFreeze() (string, error) {
	gopath, err := commandOutput("go", "env", "GOPATH")
	if err == nil {
		candidate := filepath.Join(strings.TrimSpace(gopath), "bin", "freeze")
		if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	freeze, lookErr := exec.LookPath("freeze")
	if lookErr != nil {
		return "", fmt.Errorf("freeze not found at $(go env GOPATH)/bin/freeze or on PATH: %w", lookErr)
	}
	return freeze, nil
}

func captureLivePanels(repoRoot, workspace, renderDir, freeze, fontFile string) ([]screenshot.VisualPanel, error) {
	bin := filepath.Join(workspace, "live-gitid")
	if err := runCommand(repoRoot, "go", "build", "-o", bin, "./cmd/gitid"); err != nil {
		return nil, fmt.Errorf("building current real gitid binary: %w", err)
	}
	return captureTUIPanels("live", bin, workspace, renderDir, freeze, fontFile, true)
}

func captureApprovedTUIPanels(approvalDir, workspace, renderDir, freeze, fontFile string) ([]screenshot.VisualPanel, error) {
	bin := filepath.Join(workspace, "approved-gitid-dummy")
	if err := runCommand(approvalDir, "go", "build", "-o", bin, "./cmd/gitid-dummy"); err != nil {
		return nil, fmt.Errorf("building approved-commit TUI: %w", err)
	}
	return captureTUIPanels("approved-tui", bin, workspace, renderDir, freeze, fontFile, false)
}

func captureTUIPanels(surface, bin, workspace, renderDir, freeze, fontFile string, fakeSSH bool) ([]screenshot.VisualPanel, error) {
	panels := make([]screenshot.VisualPanel, 0, len(screenshot.CreateFlowScreenIDs))
	for _, id := range screenshot.CreateFlowScreenIDs {
		home, err := os.MkdirTemp(workspace, "home-")
		if err != nil {
			return nil, fmt.Errorf("creating sandbox HOME: %w", err)
		}
		var sshDir string
		if fakeSSH {
			sshDir, err = writeFakeSSH(workspace)
			if err != nil {
				return nil, err
			}
		}
		text, err := captureTUIScreen(bin, home, sshDir, id)
		if err != nil {
			return nil, fmt.Errorf("capturing %s/%s: %w", surface, id, err)
		}
		result, err := screenshot.CaptureTUI(text, screenshot.TUIOptions{
			FreezeBin: freeze,
			FontFile:  fontFile,
			Theme:     "dracula",
			Width:     screenshot.CaptureWidth,
			Height:    screenshot.CaptureHeight,
			OutDir:    filepath.Join(renderDir, surface),
			Name:      id,
		})
		if err != nil {
			return nil, fmt.Errorf("rendering %s/%s with freeze: %w", surface, id, err)
		}
		panels = append(panels, screenshot.VisualPanel{Surface: surface, ScreenID: id, Text: text, PNGPath: result.PNGPath})
	}
	return panels, nil
}

func captureTUIScreen(bin, home, fakeSSH, id string) (string, error) {
	cmd := exec.Command(bin) //nolint:gosec // binary is built by this process from a fixed repository path
	cmd.Env = append(os.Environ(), "HOME="+home, "TERM=xterm-256color")
	if fakeSSH != "" {
		cmd.Env = append(cmd.Env, "PATH="+fakeSSH+":"+os.Getenv("PATH"), "GITID_FAKE_SSH_MODE=pass")
	}
	session, err := startCapturePTY(cmd)
	if err != nil {
		return "", err
	}
	defer session.close()
	if _, err := session.waitFor("[1] Identities", 8*time.Second); err != nil {
		return "", err
	}
	if err := session.send([]byte("n")); err != nil {
		return "", err
	}
	if _, err := session.waitFor("Step 1/4", 8*time.Second); err != nil {
		return "", err
	}
	switch id {
	case "ssh-form-filled":
	case "reuse-key-vs-generate":
		if err := session.tabs(4); err != nil {
			return "", err
		}
		if err := session.send([]byte("\x1b[C")); err != nil {
			return "", err
		}
	case "reuse-manual-path":
		if err := session.tabs(4); err != nil {
			return "", err
		}
		if err := session.send([]byte("\x1b[C\x1b[D")); err != nil {
			return "", err
		}
	case "mouse-focused-field":
		if err := session.tabs(3); err != nil {
			return "", err
		}
		frame := session.snapshot()
		row := terminalRow(frame, "Port")
		if row == 0 {
			return "", fmt.Errorf("locating Port row for mouse capture")
		}
		if err := session.send([]byte(fmt.Sprintf("\x1b[<0;10;%dM\x1b[<0;10;%dm", row, row))); err != nil {
			return "", err
		}
	case "test-stage1-direct", "test-stage2-by-alias", "git-form-demo", "confirm-write":
		if err := runStages(session); err != nil {
			return "", err
		}
		if id == "test-stage1-direct" || id == "test-stage2-by-alias" {
			break
		}
		if err := session.send([]byte("\r")); err != nil {
			return "", err
		}
		if _, err := session.waitFor("Step 3/4", 8*time.Second); err != nil {
			return "", err
		}
		if id == "git-form-demo" {
			break
		}
		if err := session.tabs(4); err != nil {
			return "", err
		}
		if err := session.send([]byte("\r")); err != nil {
			return "", err
		}
		if _, err := session.waitFor("Create identity", 8*time.Second); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unknown capture screen %q", id)
	}
	time.Sleep(150 * time.Millisecond)
	text := strings.TrimSpace(session.snapshot())
	if text == "" {
		return "", fmt.Errorf("captured empty terminal frame")
	}
	return text + "\n", nil
}

func runStages(session *capturePTY) error {
	if err := session.send([]byte("\r")); err != nil {
		return err
	}
	if _, err := session.waitFor("Step 2/4", 8*time.Second); err != nil {
		return err
	}
	if err := session.send([]byte("\r")); err != nil {
		return err
	}
	if _, err := session.waitFor("identityfile", 8*time.Second); err != nil {
		return err
	}
	return nil
}

func captureApprovedHTMLPanels(repoRoot, approvalDir, workspace, renderDir string) ([]screenshot.VisualPanel, error) {
	mockup := filepath.Join(approvalDir, ".planning", "design", "mockup-src")
	nodeModules := filepath.Join(repoRoot, ".planning", "design", "mockup-src", "node_modules")
	if _, err := os.Stat(nodeModules); err != nil {
		return nil, fmt.Errorf("approved HTML capture requires installed mockup node_modules: %w", err)
	}
	if err := os.Symlink(nodeModules, filepath.Join(mockup, "node_modules")); err != nil {
		return nil, fmt.Errorf("linking approved mockup dependencies: %w", err)
	}
	dist := filepath.Join(workspace, "approval-dist")
	if err := runCommand(mockup, "pnpm", "exec", "vite", "build", "--outDir", dist); err != nil {
		return nil, fmt.Errorf("building approved HTML source: %w", err)
	}
	routes := map[string]string{
		"ssh-form-filled":       "ssh-form-filled",
		"reuse-key-vs-generate": "reuse-key-vs-generate",
		"reuse-manual-path":     "ssh-form-blank-prefix",
		"mouse-focused-field":   "ssh-form-empty",
		"test-stage1-direct":    "test-stage1-direct",
		"test-stage2-by-alias":  "test-stage2-by-alias",
		"git-form-demo":         "backup-notice",
		"confirm-write":         "confirm-write",
	}
	panels := make([]screenshot.VisualPanel, 0, len(routes))
	for _, id := range screenshot.CreateFlowScreenIDs {
		route := routes[id]
		required := "create-flow/" + route
		result, err := screenshot.CaptureHTML(screenshot.HTMLOptions{
			FixturePath:       filepath.Join(dist, "index.html"),
			OutDir:            filepath.Join(renderDir, "approved-html"),
			Name:              id,
			ViewportWidth:     1280,
			ViewportHeight:    800,
			DeviceScaleFactor: 1,
			ColorScheme:       "light",
			Timeout:           60 * time.Second,
			AllowDownload:     false,
			URLFragment:       "#/create-flow/" + route,
			RequiredText:      required,
		})
		if err != nil {
			return nil, fmt.Errorf("capturing approved HTML %s: %w", id, err)
		}
		panels = append(panels, screenshot.VisualPanel{
			Surface:  "approved-html",
			ScreenID: id,
			Text:     "approval route: " + required + "\nsource: " + screenshot.PacketApprovalCommit + "\n",
			PNGPath:  result.PNGPath,
		})
	}
	return panels, nil
}

func exportApprovalSource(repoRoot, workspace string) (string, error) {
	_ = repoRoot
	archive, err := exec.Command("git", "archive", screenshot.PacketApprovalCommit).Output() //nolint:gosec // approval SHA is a fixed full literal
	if err != nil {
		return "", fmt.Errorf("exporting approval commit %s: %w", screenshot.PacketApprovalCommit, err)
	}
	dir := filepath.Join(workspace, "approval-source")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}
	tr := tar.NewReader(bytes.NewReader(archive))
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("reading approval archive: %w", err)
		}
		if filepath.IsAbs(header.Name) || strings.HasPrefix(filepath.Clean(header.Name), "..") {
			return "", fmt.Errorf("approval archive contains unsafe path %q", header.Name)
		}
		path := filepath.Join(dir, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, os.FileMode(header.Mode)); err != nil {
				return "", err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
				return "", err
			}
			file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode)) //nolint:gosec // archive path was checked above
			if err != nil {
				return "", err
			}
			_, copyErr := io.Copy(file, tr)
			closeErr := file.Close()
			if copyErr != nil {
				return "", copyErr
			}
			if closeErr != nil {
				return "", closeErr
			}
		}
	}
	return dir, nil
}

func validateCurrentSource(repoRoot, sourceCommit string) error {
	if err := validateSourceCommitExists(sourceCommit); err != nil {
		return err
	}
	head, err := commandOutputIn(repoRoot, "git", "rev-parse", "HEAD")
	if err != nil {
		return fmt.Errorf("resolving current HEAD: %w", err)
	}
	if strings.TrimSpace(head) != sourceCommit {
		return fmt.Errorf("source commit %q is not current HEAD %q", sourceCommit, strings.TrimSpace(head))
	}
	cmd := exec.Command("git", "diff", "--quiet", sourceCommit, "--", ".", ":(exclude).planning") //nolint:gosec // sourceCommit is full hex; fixed pathspec
	cmd.Dir = repoRoot
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("source tree differs from %s outside preserved .planning artifacts", sourceCommit)
	}
	return nil
}

func repositoryRoot() (string, error) {
	root, err := commandOutput("git", "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("locating repository root: %w", err)
	}
	return strings.TrimSpace(root), nil
}

func validateSourceCommitFormat(commit string) error {
	if len(commit) != 40 {
		return fmt.Errorf("source commit must be a full 40-hex SHA; got %q (length %d)", commit, len(commit))
	}
	for _, r := range commit {
		if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
			return fmt.Errorf("source commit contains non-hex character %q in %q", r, commit)
		}
	}
	return nil
}

func validateSourceCommitExists(commit string) error {
	cmd := exec.Command("git", "cat-file", "-e", commit+"^{commit}") //nolint:gosec // commit is hex-validated above
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("source commit %q not found in local repo", commit)
	}
	return nil
}

func commandOutput(command string, args ...string) (string, error) {
	return commandOutputIn("", command, args...)
}

func commandOutputIn(dir, command string, args ...string) (string, error) {
	cmd := exec.Command(command, args...) //nolint:gosec // command and args are fixed publisher inputs
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w\n%s", command, strings.Join(args, " "), err, out)
	}
	return string(out), nil
}

func runCommand(dir, command string, args ...string) error {
	_, err := commandOutputIn(dir, command, args...)
	return err
}

func compareInventories(first, second string) error {
	a, err := inventory(first)
	if err != nil {
		return err
	}
	b, err := inventory(second)
	if err != nil {
		return err
	}
	if len(a) != len(b) {
		return fmt.Errorf("member count differs: %d != %d", len(a), len(b))
	}
	for path, hash := range a {
		if b[path] != hash {
			return fmt.Errorf("member %q differs: %s != %s", path, hash, b[path])
		}
	}
	return nil
}

func inventory(root string) (map[string]string, error) {
	out := make(map[string]string)
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path) //nolint:gosec // path came from a controlled candidate walk
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		out[filepath.ToSlash(rel)] = sha256Hex(data)
		return nil
	})
	return out, err
}

func syncDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close() //nolint:errcheck // sync result is returned below
	return dir.Sync()
}

func mustParseTime(value string) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		panic(err)
	}
	return parsed
}

func sha256Hex(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func terminalRow(frame, label string) int {
	for index, line := range strings.Split(frame, "\n") {
		if strings.Contains(line, label) {
			return index + 1
		}
	}
	return 0
}

type capturePTY struct {
	ptmx *os.File
	cmd  *exec.Cmd
	emu  *vt.Emulator
	mu   sync.Mutex
}

func startCapturePTY(cmd *exec.Cmd) (*capturePTY, error) {
	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: screenshot.CaptureHeight, Cols: screenshot.CaptureWidth}) //nolint:gosec // fixed small geometry
	if err != nil {
		return nil, fmt.Errorf("starting TUI PTY: %w", err)
	}
	s := &capturePTY{ptmx: ptmx, cmd: cmd, emu: vt.NewEmulator(screenshot.CaptureWidth, screenshot.CaptureHeight)}
	go func() {
		buf := make([]byte, 4096)
		for {
			n, readErr := ptmx.Read(buf)
			if n > 0 {
				s.mu.Lock()
				_, _ = s.emu.Write(buf[:n])
				s.mu.Unlock()
			}
			if readErr != nil {
				return
			}
		}
	}()
	go func() {
		response := make([]byte, 256)
		for {
			n, readErr := s.emu.Read(response)
			if n > 0 {
				_, _ = ptmx.Write(response[:n])
			}
			if readErr != nil {
				return
			}
		}
	}()
	return s, nil
}

func (s *capturePTY) send(data []byte) error {
	_, err := s.ptmx.Write(data)
	return err
}

func (s *capturePTY) tabs(count int) error {
	for range count {
		if err := s.send([]byte("\t")); err != nil {
			return err
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}

func (s *capturePTY) waitFor(want string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		frame := s.snapshot()
		if strings.Contains(frame, want) {
			return frame, nil
		}
		time.Sleep(25 * time.Millisecond)
	}
	return s.snapshot(), fmt.Errorf("terminal never displayed %q", want)
}

func (s *capturePTY) snapshot() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.emu.String()
}

func (s *capturePTY) close() {
	_, _ = s.ptmx.Write([]byte("\x03"))
	_ = s.ptmx.Close()
	_ = s.cmd.Wait()
}

func writeFakeSSH(workspace string) (string, error) {
	dir, err := os.MkdirTemp(workspace, "fake-ssh-")
	if err != nil {
		return "", err
	}
	const script = `#!/bin/sh
config_path=""
next_is_config=0
is_resolution=0
for arg in "$@"; do
  if [ "$next_is_config" = "1" ]; then config_path="$arg"; next_is_config=0; continue; fi
  if [ "$arg" = "-F" ]; then next_is_config=1; continue; fi
  if [ "$arg" = "-G" ]; then is_resolution=1; fi
done
if [ "$is_resolution" = "1" ]; then
  user=$(awk '$1 == "User" { print $2; exit }' "$config_path")
  hostname=$(awk '$1 == "Hostname" { print $2; exit }' "$config_path")
  port=$(awk '$1 == "Port" { print $2; exit }' "$config_path")
  identitiesonly=$(awk '$1 == "IdentitiesOnly" { print $2; exit }' "$config_path")
  identityfile=$(awk '$1 == "IdentityFile" { print $2; exit }' "$config_path")
  printf 'user %s\nhostname %s\nport %s\nidentitiesonly %s\nidentityfile %s\n' "$user" "$hostname" "$port" "$identitiesonly" "$identityfile"
  exit 0
fi
echo "Hi user! You've successfully authenticated, but GitHub does not provide shell access."
exit 1
`
	path := filepath.Join(dir, "ssh")
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil { //nolint:gosec // static sandbox helper
		return "", err
	}
	return dir, nil
}
