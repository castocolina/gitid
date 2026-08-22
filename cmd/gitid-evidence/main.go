//go:build screenshot

// gitid-evidence publishes a content-addressed Phase 3 visual evidence packet.
package main

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/castocolina/gitid/internal/screenshot"
	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

const fixedCaptureTime = "2026-08-21T00:00:00Z"

var captureTimestampPattern = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}[:\-]\d{2}[:\-]\d{2}Z`)

const captureFakeSSHDirName = "fake-ssh"

const captureStageDirName = ".gitid-stage"

const captureManualFixtureName = ".k"

// captureManualReusePrivateFixture is a synthetic, unencrypted OpenSSH key
// solely for capture. It is never a user or account key and is written only
// inside the disposable capture HOME.
const captureManualReusePrivateFixture = `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
QyNTUxOQAAACBGqltrXnIBHpUMIZORe8hgRuyXOPXneb3MgRgsB/+v1wAAAJikcF0YpHBd
GAAAAAtzc2gtZWQyNTUxOQAAACBGqltrXnIBHpUMIZORe8hgRuyXOPXneb3MgRgsB/+v1w
AAAECt0dTv0hd6+apv0NY5AjiiypMrXUpmt0F4jnMQMoREfUaqW2tecgEelQwhk5F7yGBG
7Jc49ed5vcyBGCwH/6/XAAAAFGdpdGlkLWNhcHR1cmUtbWFudWFsAQ==
-----END OPENSSH PRIVATE KEY-----
`

const captureManualReusePublicFixture = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIEaqW2tecgEelQwhk5F7yGBG7Jc49ed5vcyBGCwH/6/X gitid-capture-manual\n"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "gitid-evidence: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) > 0 && args[0] == "finalize" {
		return finalize(args[1:])
	}
	if len(args) > 0 && args[0] == "--candidate-process" {
		sourceCommit, outputPath, err := parseArgs(args[1:], true)
		if err != nil {
			return err
		}
		return generateCandidateOnce(sourceCommit, outputPath)
	}
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
	if !candidate {
		return fmt.Errorf("final publication is review-gated; generate a candidate with --candidate and finalize only after two independent reviews")
	}
	return generateCandidate(sourceCommit, outputPath)
}

func finalize(args []string) error {
	var sourceCommit, candidateDir, outputRoot string
	var reviewDirs []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--source-commit":
			i++
			if i < len(args) {
				sourceCommit = args[i]
			}
		case "--candidate-dir":
			i++
			if i < len(args) {
				candidateDir = args[i]
			}
		case "--output-root":
			i++
			if i < len(args) {
				outputRoot = args[i]
			}
		case "--review-dir":
			i++
			if i < len(args) {
				reviewDirs = append(reviewDirs, args[i])
			}
		default:
			return fmt.Errorf("unknown finalize argument %q", args[i])
		}
	}
	if err := validateSourceCommitFormat(sourceCommit); err != nil || candidateDir == "" || outputRoot == "" || len(reviewDirs) != 2 {
		return fmt.Errorf("finalize requires --source-commit <full-40-hex-sha> --candidate-dir <dir> --review-dir <dir> --review-dir <dir> --output-root <dir>")
	}
	repoRoot, err := repositoryRoot()
	if err != nil {
		return err
	}
	if err := validateCurrentSource(repoRoot, sourceCommit); err != nil {
		return err
	}
	candidate, err := screenshot.ValidateCandidate(candidateDir)
	if err != nil {
		return fmt.Errorf("validating candidate: %w", err)
	}
	if candidate.SourceCommit != sourceCommit {
		return fmt.Errorf("candidate source commit %q does not match requested source %q", candidate.SourceCommit, sourceCommit)
	}
	reviews := make([]screenshot.ReviewInput, 0, 2)
	for _, dir := range reviewDirs {
		review, err := loadReviewInput(dir)
		if err != nil {
			return err
		}
		reviews = append(reviews, review)
	}
	if err := prepareOutputRoot(outputRoot); err != nil {
		return err
	}
	final, err := screenshot.FinalizeCandidate(candidateDir, filepath.Join(outputRoot, sourceCommit), reviews)
	if err != nil {
		return err
	}
	fmt.Printf("gitid-evidence: finalized immutable packet at %s\n", filepath.Join(outputRoot, sourceCommit))
	fmt.Printf("gitid-evidence: manifest SHA-256 = %s\n", final.ManifestSHA256)
	return nil
}

func loadReviewInput(dir string) (screenshot.ReviewInput, error) {
	metadata, err := os.ReadFile(filepath.Join(dir, "metadata.json")) //nolint:gosec // explicit local review directory
	if err != nil {
		return screenshot.ReviewInput{}, fmt.Errorf("reading review metadata: %w", err)
	}
	var review screenshot.ReviewInput
	decoder := json.NewDecoder(bytes.NewReader(metadata))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&review); err != nil {
		return screenshot.ReviewInput{}, fmt.Errorf("decoding review metadata: %w", err)
	}
	for _, asset := range []struct {
		name string
		into *[]byte
	}{{"prompt.txt", &review.Prompt}, {"raw-stdout.txt", &review.Stdout}, {"raw-stderr.txt", &review.Stderr}, {"verdict.json", &review.Verdict}} {
		data, err := os.ReadFile(filepath.Join(dir, asset.name)) //nolint:gosec // explicit local review directory
		if err != nil {
			return screenshot.ReviewInput{}, fmt.Errorf("reading review %s: %w", asset.name, err)
		}
		*asset.into = data
	}
	return review, nil
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

func prepareOutputRoot(root string) error {
	info, err := os.Stat(root)
	if err == nil {
		if !info.IsDir() {
			return fmt.Errorf("output root %q is not a directory", root)
		}
		// Allow the output root to already contain prior publication
		// directories (previous source SHAs). The caller checks that the
		// specific source-SHA subdirectory does not yet exist.
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
	cmd := exec.Command(self, "--candidate-process", "--source-commit", sourceCommit, "--output-dir", outputDir) //nolint:gosec // all args are locally validated paths and full commit SHA
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "GITID_EVIDENCE_REPO_ROOT="+repoRoot)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("candidate process failed: %w\n%s", err, output)
	}
	return nil
}

func generateCandidate(sourceCommit, outputDir string) error {
	repoRoot, err := repositoryRoot()
	if err != nil {
		return err
	}
	if _, err := os.Stat(outputDir); err == nil {
		return fmt.Errorf("candidate output %q already exists", outputDir)
	}
	secondRoot, err := os.MkdirTemp(filepath.Dir(outputDir), ".gitid-evidence-candidate-")
	if err != nil {
		return fmt.Errorf("creating second candidate directory: %w", err)
	}
	defer os.RemoveAll(secondRoot)
	second := filepath.Join(secondRoot, "candidate")
	if err := runCandidateProcess(repoRoot, sourceCommit, outputDir); err != nil {
		return err
	}
	if err := runCandidateProcess(repoRoot, sourceCommit, second); err != nil {
		return err
	}
	if err := compareInventories(outputDir, second); err != nil {
		return fmt.Errorf("two-process candidate determinism check failed: %w", err)
	}
	return nil
}

func generateCandidateOnce(sourceCommit, outputDir string) error {
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

	workspace, err := captureWorkspace(sourceCommit)
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
	// Build provenance content from the capture inputs. EVIDENCE.json records
	// the exact argv, tool versions, source commit, and per-screen routes.
	// REGION-DIFFS.json records live-vs-approved-tui PNG SHA comparisons.
	// Both are declared as manifest members; REVIEW-PROVENANCE.json is added
	// later by the finalization step after independent reviews complete.
	evidence := buildEvidenceJSON(sourceCommit, capture, panels)
	// REGION-DIFFS.json compares live vs approved-tui text captures per ScreenSpec.
	// The text is already available from the panel captures above.
	liveTextMap := panelsToTextMap(live)
	approvedTUITextMap := panelsToTextMap(approvedTUI)
	regionRecords, err := screenshot.BuildRegionDiffs(sourceCommit, liveTextMap, approvedTUITextMap, screenshot.RequiredScreenSpecs())
	if err != nil {
		return err
	}
	regionDiffs := screenshot.BuildRegionDiffsJSON(sourceCommit, regionRecords)

	result, err := screenshot.GenerateVisualPacket(screenshot.PacketOptions{
		SourceCommit:   sourceCommit,
		ApprovalCommit: screenshot.PacketApprovalCommit,
		LiveBackendRef: sourceCommit,
		OutputDir:      outputDir,
		Clock: func() time.Time {
			return mustParseTime(fixedCaptureTime)
		},
		ProvenanceFiles: map[string][]byte{
			"EVIDENCE.json":     evidence,
			"REGION-DIFFS.json": regionDiffs,
		},
	}, panels, capture)
	if err != nil {
		return err
	}
	candidateManifest := filepath.Join(filepath.Dir(result.ManifestPath), "CANDIDATE-MANIFEST.json")
	if err := os.Rename(result.ManifestPath, candidateManifest); err != nil {
		return fmt.Errorf("renaming candidate manifest: %w", err)
	}
	if _, err := screenshot.ValidateCandidate(filepath.Dir(candidateManifest)); err != nil {
		return fmt.Errorf("validating generated candidate: %w", err)
	}
	return nil
}

// buildEvidenceJSON builds the EVIDENCE.json content for a packet.
// This records the exact argv, tool versions, source hash, approval hash,
// and per-screen routes used to produce the evidence (D-22, D-24).
func buildEvidenceJSON(sourceCommit string, capture screenshot.PacketCapture, panels []screenshot.VisualPanel) []byte {
	type ScreenEvidence struct {
		ScreenID string `json:"screen_id"`
		Surface  string `json:"surface"`
		Route    string `json:"route,omitempty"`
	}
	type Evidence struct {
		Version        string                  `json:"version"`
		SourceCommit   string                  `json:"source_commit"`
		ApprovalCommit string                  `json:"approval_commit"`
		CapturedAt     string                  `json:"captured_at"`
		Commands       []string                `json:"commands"`
		ToolVersions   []screenshot.PacketTool `json:"tool_versions"`
		Geometry       string                  `json:"geometry"`
		FontSHA256     string                  `json:"font_sha256"`
		Theme          string                  `json:"theme"`
		Screens        []ScreenEvidence        `json:"screens"`
	}

	routes := screenshot.ApprovedHTMLRoutes()
	screens := make([]ScreenEvidence, 0, len(panels))
	for _, p := range panels {
		route := ""
		if p.Surface == "approved-html" {
			route = routes[p.ScreenID]
		}
		screens = append(screens, ScreenEvidence{
			ScreenID: p.ScreenID,
			Surface:  p.Surface,
			Route:    route,
		})
	}

	ev := Evidence{
		Version:        "03-12.1",
		SourceCommit:   sourceCommit,
		ApprovalCommit: screenshot.PacketApprovalCommit,
		CapturedAt:     fixedCaptureTime,
		Commands:       capture.Commands,
		ToolVersions:   capture.ToolVersions,
		Geometry:       capture.Geometry,
		FontSHA256:     capture.FontSHA256,
		Theme:          capture.Theme,
		Screens:        screens,
	}
	data, err := json.MarshalIndent(ev, "", "  ")
	if err != nil {
		panic("buildEvidenceJSON: " + err.Error())
	}
	return data
}

// panelsToTextMap converts a slice of VisualPanels to a screen-ID→text map
// for region diff generation.
func panelsToTextMap(panels []screenshot.VisualPanel) map[string]string {
	m := make(map[string]string, len(panels))
	for _, p := range panels {
		m[p.ScreenID] = p.Text
	}
	return m
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
	specs := screenshot.RequiredScreenSpecs()
	panels := make([]screenshot.VisualPanel, 0, len(specs))
	sshDir := ""
	var err error
	if fakeSSH {
		sshDir, err = writeFakeSSH(workspace)
		if err != nil {
			return nil, err
		}
	}
	for _, spec := range specs {
		if !screenshot.ScreenAppliesToSurface(spec, surface) {
			continue
		}
		id := spec.ScreenID
		home := captureHomePath(workspace, id)
		if err := os.MkdirAll(home, 0o700); err != nil {
			return nil, fmt.Errorf("creating sandbox HOME: %w", err)
		}
		var raw string
		text, err := captureTUIScreen(bin, home, sshDir, id, fakeSSH, &raw)
		if err != nil {
			return nil, fmt.Errorf("capturing %s/%s: %w", surface, id, err)
		}
		text = normalizeCaptureText(text, home, workspace)
		raw = normalizeCaptureText(raw, home, workspace)
		if err := screenshot.ValidateCapturedState(spec, text); err != nil {
			return nil, err
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
		panels = append(panels, screenshot.VisualPanel{Surface: surface, ScreenID: id, Text: text, RawText: raw, PNGPath: result.PNGPath})
	}
	return panels, nil
}

func normalizeCaptureText(text, home, workspace string) string {
	text = strings.ReplaceAll(text, home, "<home>")
	text = strings.ReplaceAll(text, workspace, "<workspace>")
	return captureTimestampPattern.ReplaceAllString(text, "<timestamp>")
}

func captureHomePath(workspace, screenID string) string {
	return filepath.Join(workspace, "home-"+screenID)
}

func captureTUIScreen(bin, home, fakeSSH, id string, autoStage2 bool, rawOutput *string) (string, error) {
	manualKeyPath := ""
	if id == "reuse-manual-resolved" && fakeSSH != "" {
		var err error
		manualKeyPath, err = seedManualReuseKey(home)
		if err != nil {
			return "", err
		}
	}
	cmd := exec.Command(bin) //nolint:gosec // binary is built by this process from a fixed repository path
	cmd.Env = captureCommandEnvironment(home, fakeSSH)
	// For stage-1 captures: set GITID_BARRIER_FILE so the fake SSH's -G call
	// blocks until the capture script signals stage-1 is done.
	var barrierFile string
	if fakeSSH != "" && (id == "test-stage1-direct" || id == "test-stage1-pass" || id == "test-stage1-command-output") {
		barrierFile = filepath.Join(home, ".gitid-stage1-barrier")
		cmd.Env = append(cmd.Env, "GITID_BARRIER_FILE="+barrierFile)
	}
	if fakeSSH != "" {
		mode := "pass"
		if id == "test-stage1-direct" || id == "test-reachable-not-uploaded" {
			mode = "denied"
		} else if id == "test-hard-failure-retry" {
			mode = "timeout"
		}
		cmd.Env = append(cmd.Env, "GITID_FAKE_SSH_MODE="+mode)
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
		if err := selectReuse(session); err != nil {
			return "", err
		}
	case "reuse-manual-path", "reuse-manual-resolved":
		if err := session.tabs(4); err != nil {
			return "", err
		}
		// Select Reuse, then the trailing manual row and its input. This is a
		// real resolved-key state, never a Generate frame relabeled by ID.
		if err := selectReuse(session); err != nil {
			return "", err
		}
		if err := session.tabs(1); err != nil {
			return "", err
		}
		if err := session.send([]byte("\x1b[C")); err != nil {
			return "", err
		}
		if err := session.tabs(1); err != nil {
			return "", err
		}
		if _, err := session.waitFor("▸ Key path", 8*time.Second); err != nil {
			return "", fmt.Errorf("waiting for manual key-path focus: %w", err)
		}
		if id == "reuse-manual-resolved" && manualKeyPath != "" {
			if err := session.send([]byte("~/.k")); err != nil {
				return "", err
			}
			if _, err := session.waitFor("ssh-ed25519", 8*time.Second); err != nil {
				return "", fmt.Errorf("waiting for resolved manual key marker: %w", err)
			}
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
	case "test-stage1-direct", "test-stage1-pass", "test-stage1-command-output":
		// Run stage 1 and capture BEFORE stage 2 completes — the genuine
		// testRunning2 state (stage-1 result visible + "… running ssh…").
		// The prior runStages() waited for "identityfile" (stage-2 done),
		// making both stage captures byte-identical (UI-REVIEW Critical Pillar 2).
		if err := runStage1Only(session); err != nil {
			return "", err
		}
		if id == "test-stage1-command-output" {
			if err := focusAndNavigateViewport(session, []string{"Stage 1 command:", "Stage 1 output:", "Hi user!"}, false); err != nil {
				return "", err
			}
		}
	case "test-stage2-by-alias", "test-stage2-command-output", "test-stage2-resolution-user-host-port", "test-stage2-resolution-identities-key", "test-reachable-not-uploaded":
		// Run both stages and capture AFTER stage 2 completes (testStage2 state).
		if err := runStages(session, autoStage2); err != nil {
			return "", err
		}
		markers := map[string][]string{
			"test-stage2-command-output":            {"Stage 2 command:", "Stage 2 output:", "Hi user!"},
			"test-stage2-resolution-user-host-port": {"user git", "hostname ssh.github.com", "port 443"},
			"test-stage2-resolution-identities-key": {"identitiesonly yes", "identityfile"},
		}
		if required := markers[id]; len(required) > 0 {
			if err := focusAndNavigateViewport(session, required, false); err != nil {
				return "", err
			}
		}
	case "test-hard-failure-retry":
		if err := runFailure(session); err != nil {
			return "", err
		}
	case "git-form-demo", "confirm-write", "confirm-summary-key-path", "confirm-managed-block":
		if err := runStages(session, autoStage2); err != nil {
			return "", err
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
		if id == "confirm-summary-key-path" || id == "confirm-managed-block" {
			if err := session.send([]byte("v")); err != nil {
				return "", err
			}
			if _, err := session.waitFor("Exact change focused", 8*time.Second); err != nil {
				return "", err
			}
		}
		if id == "confirm-summary-key-path" {
			if err := navigateFocusedViewport(session, []string{"key ~/.ssh/id_ed25519_acme"}, true); err != nil {
				return "", err
			}
		}
		if id == "confirm-managed-block" {
			if err := navigateFocusedViewport(session, []string{
				"# BEGIN gitid managed: acme", "Host acme.github.com", "Hostname ssh.github.com", "Port 443", "User git",
				"IdentityFile ~/.ssh/id_ed25519_acme", "IdentitiesOnly yes", "# END gitid managed: acme",
			}, false); err != nil {
				return "", err
			}
		}
	default:
		return "", fmt.Errorf("unknown capture screen %q", id)
	}
	time.Sleep(150 * time.Millisecond)
	text := strings.TrimSpace(session.snapshot())
	if text == "" {
		return "", fmt.Errorf("captured empty terminal frame")
	}
	// Preserve the raw terminal bytes that produced this exact frame. Stage-two
	// completion may render path-dependent proof after the barrier is released.
	*rawOutput = session.transcript()
	if strings.TrimSpace(*rawOutput) == "" {
		return "", fmt.Errorf("captured empty raw PTY transcript")
	}
	// Release the stage-2 barrier AFTER taking the snapshot, so the stage-2
	// ssh -G call can complete and the process exits cleanly.
	if barrierFile != "" {
		if err := os.WriteFile(barrierFile, []byte("go"), 0o644); err != nil { //nolint:gosec // barrier semaphore, sandbox path
			// Non-fatal: the process will eventually time out anyway.
			_ = err
		}
		// Give stage-2 time to complete so the process exits cleanly.
		time.Sleep(500 * time.Millisecond)
	}
	return text + "\n", nil
}

func captureCommandEnvironment(home, fakeSSH string) []string {
	env := append(os.Environ(),
		"HOME="+home,
		"TERM=xterm-256color",
		"GITID_STAGE_DIR="+filepath.Join(home, captureStageDirName),
	)
	if fakeSSH != "" {
		env = append(env, "PATH="+fakeSSH+string(filepath.ListSeparator)+os.Getenv("PATH"))
	}
	return env
}

func captureWorkspace(sourceCommit string) (string, error) {
	workspace := filepath.Join(os.TempDir(), "gitid-evidence-capture-"+sourceCommit)
	if err := os.Mkdir(workspace, 0o700); err != nil {
		return "", err
	}
	return workspace, nil
}

func selectReuse(session *capturePTY) error {
	if err := session.send([]byte("\x1b[C")); err != nil {
		return err
	}
	if _, err := session.waitFor("● Reuse an", 8*time.Second); err != nil {
		return fmt.Errorf("waiting for selected reuse state: %w", err)
	}
	return nil
}

func focusAndNavigateViewport(session *capturePTY, markers []string, allowHorizontal bool) error {
	if err := session.send([]byte("v")); err != nil {
		return err
	}
	if _, err := session.waitFor("viewport focused", 8*time.Second); err != nil {
		return fmt.Errorf("focusing proof viewport: %w", err)
	}
	return navigateFocusedViewport(session, markers, allowHorizontal)
}

func navigateFocusedViewport(session *capturePTY, markers []string, allowHorizontal bool) error {
	containsAll := func(frame string) bool {
		for _, marker := range markers {
			if !strings.Contains(frame, marker) {
				return false
			}
		}
		return true
	}
	reset := func(key []byte, count int) error {
		for range count {
			if err := session.send(key); err != nil {
				return err
			}
			time.Sleep(15 * time.Millisecond)
		}
		return nil
	}

	// Start from a deterministic origin using the controls advertised by the
	// focused viewport. This is raw PTY input, not model-internal mutation.
	if err := session.send([]byte("\x1b[6~")); err != nil { // PgDn
		return err
	}
	if err := reset([]byte("\x1b[5~"), 12); err != nil { // PgUp
		return err
	}
	if err := session.send([]byte("\x1b[C")); err != nil { // Right
		return err
	}
	if err := reset([]byte("\x1b[D"), 32); err != nil { // Left
		return err
	}
	time.Sleep(40 * time.Millisecond)
	for range 16 {
		if containsAll(session.snapshot()) {
			return nil
		}
		if allowHorizontal {
			for range 32 {
				if err := session.send([]byte("\x1b[C")); err != nil { // Right
					return err
				}
				time.Sleep(10 * time.Millisecond)
				if containsAll(session.snapshot()) {
					return nil
				}
			}
			if err := reset([]byte("\x1b[D"), 32); err != nil {
				return err
			}
		}
		if err := session.send([]byte("\x1b[6~")); err != nil { // PgDn
			return err
		}
		time.Sleep(40 * time.Millisecond)
	}
	return fmt.Errorf("terminal never displayed exact markers %q after viewport navigation:\n%s", markers, session.snapshot())
}

func runFailure(session *capturePTY) error {
	if err := session.send([]byte("\r")); err != nil {
		return err
	}
	if _, err := session.waitFor("Step 2/4", 8*time.Second); err != nil {
		return err
	}
	if err := session.send([]byte("\r")); err != nil {
		return err
	}
	_, err := session.waitFor("Retry (Enter)", 10*time.Second)
	return err
}

// seedManualReuseKey writes known fixture material under the capture HOME. It
// only proves the resolved-manual-key UI state and never reads an account key
// or the user's real SSH directory.
func seedManualReuseKey(home string) (string, error) {
	if err := os.MkdirAll(home, 0o700); err != nil {
		return "", fmt.Errorf("creating sandbox HOME: %w", err)
	}
	path := filepath.Join(home, captureManualFixtureName)
	if err := os.WriteFile(path, []byte(captureManualReusePrivateFixture), 0o600); err != nil { //nolint:gosec // known fixture material in disposable capture HOME
		return "", fmt.Errorf("writing sandbox manual private fixture: %w", err)
	}
	if err := os.WriteFile(path+".pub", []byte(captureManualReusePublicFixture), 0o644); err != nil { //nolint:gosec // public fixture material in disposable capture HOME
		return "", fmt.Errorf("writing sandbox manual public fixture: %w", err)
	}
	return path, nil
}

// runStage1Only navigates to the test screen, runs stage-1, and returns with
// the session in testRunning2 state — stage-1 result visible, stage-2 blocked.
//
// The D-22 barrier fake SSH blocks stage-2's -G call until a semaphore file is
// created, giving us a window to capture the stage-1-only state. The semaphore
// file is created AFTER we capture the snapshot in captureTUIScreen.
func runStage1Only(session *capturePTY) error {
	// Navigate step 0 → step 1.
	if err := session.send([]byte("\r")); err != nil {
		return err
	}
	if _, err := session.waitFor("Step 2/4", 8*time.Second); err != nil {
		return err
	}
	// Press Enter to run stage 1 (testIdle → testRunning1 → stage-1 result).
	if err := session.send([]byte("\r")); err != nil {
		return err
	}
	// Wait for stage-1 output to appear. With the barrier fake SSH, stage-2's
	// -G call is blocked so the session stays in testRunning2 state showing
	// stage-1 outcome + "… running ssh…" indefinitely.
	if _, err := session.waitFor("running ssh", 10*time.Second); err != nil {
		// If "running ssh" didn't appear, stage-2 may have already completed.
		// Check for Hi user! or ! Reachable as a fallback.
		if _, err2 := session.waitFor("Hi user!", 2*time.Second); err2 != nil {
			if _, err3 := session.waitFor("Reachable", 2*time.Second); err3 != nil {
				return fmt.Errorf("runStage1Only: stage-1 result did not appear: %w", err)
			}
		}
	}
	return nil
}

func runStages(session *capturePTY, autoStage2 bool) error {
	if err := session.send([]byte("\r")); err != nil {
		return err
	}
	if _, err := session.waitFor("Step 2/4", 8*time.Second); err != nil {
		return err
	}
	if err := session.send([]byte("\r")); err != nil {
		return err
	}
	if !autoStage2 {
		if _, err := session.waitFor("Run stage 2", 8*time.Second); err != nil {
			return err
		}
		if err := session.send([]byte("\r")); err != nil {
			return err
		}
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
	// Use the canonical approved HTML routes from the screenshot package —
	// this is the single source of truth for which route corresponds to each
	// logical screen ID. Prior versions had wrong mappings (UI-REVIEW HIGH:
	// reuse-manual-path→ssh-form-blank-prefix, mouse-focused-field→ssh-form-empty,
	// git-form-demo→create-flow/backup-notice).
	approvedRoutes := screenshot.ApprovedHTMLRoutes()
	panels := make([]screenshot.VisualPanel, 0, len(approvedRoutes))
	for _, spec := range screenshot.RequiredScreenSpecs() {
		if !spec.ApplicableApprovedHTML {
			continue
		}
		id := spec.ScreenID
		route, ok := approvedRoutes[id]
		if !ok {
			return nil, fmt.Errorf("no approved HTML route for screen %q", id)
		}
		// Strip the leading "/" for the URLFragment and RequiredText.
		routePath := strings.TrimPrefix(route, "/")
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
			URLFragment:       "#/" + routePath,
			RequiredText:      routePath,
		})
		if err != nil {
			return nil, fmt.Errorf("capturing approved HTML %s: %w", id, err)
		}
		panels = append(panels, screenshot.VisualPanel{
			Surface:  "approved-html",
			ScreenID: id,
			Text:     result.BodyText,
			RawText:  result.BodyText,
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
	cmd := exec.Command("git", "diff", "--quiet", sourceCommit, "--", ".", ":(exclude).planning") //nolint:gosec // sourceCommit is full hex; fixed pathspec
	cmd.Dir = repoRoot
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("source tree differs from %s outside preserved .planning artifacts", sourceCommit)
	}
	status, err := commandOutputIn(repoRoot, "git", "status", "--porcelain", "--", ".", ":(exclude).planning")
	if err != nil {
		return fmt.Errorf("checking source tree status: %w", err)
	}
	if strings.TrimSpace(status) != "" {
		return fmt.Errorf("source tree has uncommitted or untracked files outside preserved .planning artifacts")
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
	a, err := screenshot.ValidateCandidate(first)
	if err != nil {
		return fmt.Errorf("validating first candidate: %w", err)
	}
	b, err := screenshot.ValidateCandidate(second)
	if err != nil {
		return fmt.Errorf("validating second candidate: %w", err)
	}
	if !reflect.DeepEqual(canonicalSemanticPacket(a), canonicalSemanticPacket(b)) {
		return fmt.Errorf("canonical semantic evidence differs")
	}
	return nil
}

// canonicalSemanticPacket preserves the registry-derived frame inventory,
// captured text and raw-transcript hashes, region records, and candidate
// metadata. PNG hashes are intentionally blank: each candidate is separately
// required to contain complete, valid PNGs, but renderer bytes are not UX
// semantics under ONESHOT Rule 11 and L16.
func canonicalSemanticPacket(pkt screenshot.Packet) screenshot.Packet {
	pkt.ManifestSHA256 = ""
	for i := range pkt.Members {
		if strings.HasSuffix(pkt.Members[i].Path, ".png") {
			pkt.Members[i].SHA256 = ""
		}
	}
	return pkt
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
	raw  bytes.Buffer
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
				_, _ = s.raw.Write(buf[:n])
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

func (s *capturePTY) transcript() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.raw.String()
}

func (s *capturePTY) close() {
	done := make(chan struct{})
	go func() {
		_ = s.cmd.Wait()
		close(done)
	}()
	_, _ = s.ptmx.Write([]byte("\x03"))
	select {
	case <-done:
		_ = s.ptmx.Close()
		return
	case <-time.After(time.Second):
		_ = s.cmd.Process.Kill()
	}
	_ = s.ptmx.Close()
	select {
	case <-done:
	case <-time.After(time.Second):
	}
}

// writeFakeSSH creates a fake SSH binary for evidence capture. The script
// supports two modes:
//   - Normal (no semaphore): responds immediately to -T and -G calls.
//   - Barrier (semaphore env var set): blocks -G (stage-2 resolution) calls
//     until the semaphore file exists, giving the capture script time to
//     snapshot the stage-1 intermediate state before stage-2 completes.
//
// The semaphore file path is passed via GITID_BARRIER_FILE env var.
func writeFakeSSH(workspace string) (string, error) {
	dir := filepath.Join(workspace, captureFakeSSHDirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	const script = `#!/bin/sh
if [ "$1" = "-Q" ] && [ "$2" = "key" ]; then
  printf 'ssh-ed25519\nssh-rsa\necdsa-sha2-nistp256\n'
  exit 0
fi
config_path=""
next_is_config=0
is_resolution=0
for arg in "$@"; do
  if [ "$next_is_config" = "1" ]; then config_path="$arg"; next_is_config=0; continue; fi
  if [ "$arg" = "-F" ]; then next_is_config=1; continue; fi
  if [ "$arg" = "-G" ]; then is_resolution=1; fi
done
if [ "$is_resolution" = "1" ]; then
  # If a barrier file is configured, block until it exists (max 10s).
  if [ -n "$GITID_BARRIER_FILE" ]; then
    i=0
    while [ ! -f "$GITID_BARRIER_FILE" ] && [ "$i" -lt 100 ]; do
      sleep 0.1
      i=$((i+1))
    done
  fi
  user=$(awk '$1 == "User" { print $2; exit }' "$config_path")
  hostname=$(awk '$1 == "Hostname" { print $2; exit }' "$config_path")
  port=$(awk '$1 == "Port" { print $2; exit }' "$config_path")
  identitiesonly=$(awk '$1 == "IdentitiesOnly" { print $2; exit }' "$config_path")
  identityfile=$(awk '$1 == "IdentityFile" { print $2; exit }' "$config_path")
  printf 'user %s\nhostname %s\nport %s\nidentitiesonly %s\nidentityfile %s\n' "$user" "$hostname" "$port" "$identitiesonly" "$identityfile"
  exit 0
fi
if [ "$GITID_FAKE_SSH_MODE" = "timeout" ]; then
  echo "connect to host ssh.github.com port 443: Connection timed out" >&2
  exit 255
fi
if [ "$GITID_FAKE_SSH_MODE" = "denied" ]; then
  echo "git@ssh.github.com: Permission denied (publickey)." >&2
  exit 1
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
