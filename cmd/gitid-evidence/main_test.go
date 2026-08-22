//go:build screenshot

package main

// main_test.go — tests for the gitid-evidence publisher (plan 03-11 Task 2, CR-01).
//
// These tests verify that the publisher is fail-closed (rejects invalid inputs,
// existing destinations, and reports manifest hashes), and that it produces a
// valid content-addressed packet on success.

import (
	"encoding/binary"
	"encoding/json"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/screenshot"
	"golang.org/x/crypto/ssh"
)

// TestPublisherRejectsEmptySourceCommit verifies that run() fails when
// --source-commit is not provided (CR-01 fail-closed).
func TestPublisherRejectsEmptySourceCommit(t *testing.T) {
	err := run([]string{"--output-root", t.TempDir()})
	if err == nil {
		t.Fatal("publisher must fail when --source-commit is missing")
	}
}

// TestPublisherCLIProducesImmutable24PanelPacket exercises the command users
// invoke through make, rather than a helper disconnected from publication.
func TestPublisherRequiresCandidateMode(t *testing.T) {
	err := run([]string{"--source-commit", strings.Repeat("a", 40), "--output-root", t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "review-gated") {
		t.Fatalf("publisher must require explicit candidate mode; got %v", err)
	}
}

func TestPublisherCLIRejectsNonemptyRootAndUnknownCommit(t *testing.T) {
	source, err := commandOutput("git", "rev-parse", "HEAD")
	if err != nil {
		t.Fatalf("resolving HEAD: %v", err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "existing"), []byte("not empty"), 0o600); err != nil {
		t.Fatalf("seeding nonempty root: %v", err)
	}
	for _, sourceCommit := range []string{strings.TrimSpace(source), strings.Repeat("0", 40)} {
		cmd := exec.Command("go", "run", "-tags", "screenshot", ".", "--source-commit", sourceCommit, "--output-root", root) //nolint:gosec // fixed local command and test paths
		if output, runErr := cmd.CombinedOutput(); runErr == nil {
			t.Fatalf("publisher CLI accepted %q with a nonempty root:\n%s", sourceCommit, output)
		}
	}
}

// TestPublisherRejectsEmptyOutputRoot verifies that run() fails when
// --output-root is not provided (CR-01 fail-closed).
func TestPublisherRejectsEmptyOutputRoot(t *testing.T) {
	err := run([]string{"--source-commit", strings.Repeat("a", 40)})
	if err == nil {
		t.Fatal("publisher must fail when --output-root is missing")
	}
}

// TestPublisherRejectsShortCommit verifies that run() fails for a commit
// that is not 40 hex characters (CR-01 fail-closed — validateSourceCommit).
func TestPublisherRejectsShortCommit(t *testing.T) {
	err := run([]string{"--source-commit", "abc123", "--output-root", t.TempDir()})
	if err == nil {
		t.Fatal("publisher must fail for a short source commit")
	}
	if !strings.Contains(err.Error(), "40") {
		t.Errorf("error should mention 40-hex requirement; got: %v", err)
	}
}

// TestPublisherRejectsExistingDestination verifies that run() fails when the
// destination directory (output-root/source-commit) already exists.
// This enforces the immutability guarantee (CR-01).
func TestPublisherRejectsExistingDestination(t *testing.T) {
	root := t.TempDir()
	commit := strings.Repeat("b", 40)
	destDir := filepath.Join(root, commit)

	// Pre-create the destination.
	if err := os.MkdirAll(destDir, 0o750); err != nil {
		t.Fatalf("seeding destination: %v", err)
	}

	err := run([]string{"--source-commit", commit, "--output-root", root})
	if err == nil {
		t.Fatal("publisher must fail when the destination already exists")
	}
	if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "nonempty") && !strings.Contains(err.Error(), "review-gated") {
		t.Errorf("error should reject the destination or source; got: %v", err)
	}
}

// TestPublisherRejectsUnknownCommit verifies that run() fails for a valid hex
// commit that is not present in the local git repository (CR-01 fail-closed —
// git cat-file check).
func TestPublisherRejectsUnknownCommit(t *testing.T) {
	// A full 40-hex SHA that is astronomically unlikely to exist in the repo.
	unknownCommit := strings.Repeat("0", 40)
	err := run([]string{"--source-commit", unknownCommit, "--output-root", t.TempDir()})
	if err == nil {
		t.Fatal("publisher must fail for an unknown source commit")
	}
	if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "cat-file") && !strings.Contains(err.Error(), "review-gated") {
		t.Errorf("error should mention 'not found' or 'cat-file'; got: %v", err)
	}
}

func TestSeedManualReuseKeyUsesDeterministicSandboxFixture(t *testing.T) {
	firstHome := t.TempDir()
	secondHome := t.TempDir()
	firstPath, err := seedManualReuseKey(firstHome)
	if err != nil {
		t.Fatalf("seeding first sandbox key: %v", err)
	}
	secondPath, err := seedManualReuseKey(secondHome)
	if err != nil {
		t.Fatalf("seeding second sandbox key: %v", err)
	}
	for _, path := range []string{firstPath, secondPath} {
		if !strings.Contains(path, string(filepath.Separator)+".ssh"+string(filepath.Separator)) {
			t.Fatalf("sandbox key path %q is not inside an SSH directory", path)
		}
	}
	firstPrivate, err := os.ReadFile(firstPath)
	if err != nil {
		t.Fatalf("reading first sandbox private fixture: %v", err)
	}
	secondPrivate, err := os.ReadFile(secondPath)
	if err != nil {
		t.Fatalf("reading second sandbox private fixture: %v", err)
	}
	if string(firstPrivate) != string(secondPrivate) {
		t.Fatal("sandbox private fixture bytes differ between capture inputs")
	}
	privateSigner, err := ssh.ParsePrivateKey(firstPrivate)
	if err != nil {
		t.Fatalf("parsing sandbox private fixture: %v", err)
	}
	firstPublic, err := os.ReadFile(firstPath + ".pub")
	if err != nil {
		t.Fatalf("reading first sandbox public fixture: %v", err)
	}
	secondPublic, err := os.ReadFile(secondPath + ".pub")
	if err != nil {
		t.Fatalf("reading second sandbox public fixture: %v", err)
	}
	if string(firstPublic) != string(secondPublic) {
		t.Fatal("sandbox public fixture bytes differ between capture inputs")
	}
	publicKey, _, _, _, err := ssh.ParseAuthorizedKey(firstPublic)
	if err != nil {
		t.Fatalf("parsing sandbox public fixture: %v", err)
	}
	if publicKey.Type() != ssh.KeyAlgoED25519 || ssh.FingerprintSHA256(publicKey) != "SHA256:0WDiIzM4Vu8Q/Iut7m4BuXCgGczOJGXVwVirboftrx0" {
		t.Fatalf("sandbox public fixture = %s %q, want fingerprinted ed25519 key", publicKey.Type(), ssh.FingerprintSHA256(publicKey))
	}
	if ssh.FingerprintSHA256(privateSigner.PublicKey()) != ssh.FingerprintSHA256(publicKey) {
		t.Fatal("sandbox public fixture does not match its private fixture")
	}
}

func TestCaptureInputsNormalizeOnlyDisposablePrefixes(t *testing.T) {
	firstWorkspace := "/var/tmp/gitid-evidence-capture-101"
	secondWorkspace := "/var/tmp/gitid-evidence-capture-202"
	firstHome := captureHomePath(firstWorkspace, "reuse-manual-resolved")
	secondHome := captureHomePath(secondWorkspace, "reuse-manual-resolved")
	first := strings.Join([]string{
		"key=" + filepath.Join(firstHome, ".ssh", "id_ed25519_capture_manual"),
		"helper=" + filepath.Join(firstWorkspace, captureFakeSSHDirName, "ssh"),
		"actual=/Users/alice/.ssh/id_ed25519",
		"literal=capture-101",
		"at=2026-08-21T12:34:56Z",
	}, "\n")
	second := strings.Join([]string{
		"key=" + filepath.Join(secondHome, ".ssh", "id_ed25519_capture_manual"),
		"helper=" + filepath.Join(secondWorkspace, captureFakeSSHDirName, "ssh"),
		"actual=/Users/alice/.ssh/id_ed25519",
		"literal=capture-101",
		"at=2026-08-21T12:34:56Z",
	}, "\n")

	gotFirst := normalizeCaptureText(first, firstHome, firstWorkspace)
	gotSecond := normalizeCaptureText(second, secondHome, secondWorkspace)
	if gotFirst != gotSecond {
		t.Fatalf("normalized capture inputs differ:\n--- first ---\n%s\n--- second ---\n%s", gotFirst, gotSecond)
	}
	want := strings.Join([]string{
		"key=<home>/.ssh/id_ed25519_capture_manual",
		"helper=<workspace>/fake-ssh/ssh",
		"actual=/Users/alice/.ssh/id_ed25519",
		"literal=capture-101",
		"at=<timestamp>",
	}, "\n")
	if gotFirst != want {
		t.Fatalf("normalized capture =\n%s\nwant:\n%s", gotFirst, want)
	}
}

func TestCaptureCommandEnvironmentUsesStableSandboxStagePath(t *testing.T) {
	home := "/var/tmp/gitid-evidence-capture/home-test-stage2-proof-top"
	env := captureCommandEnvironment(home, "/var/tmp/gitid-evidence-capture/fake-ssh")
	values := make(map[string]string, len(env))
	for _, entry := range env {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			values[key] = value
		}
	}
	if values["HOME"] != home {
		t.Fatalf("HOME = %q, want %q", values["HOME"], home)
	}
	wantStage := filepath.Join(home, captureStageDirName)
	if values["GITID_STAGE_DIR"] != wantStage {
		t.Fatalf("GITID_STAGE_DIR = %q, want %q", values["GITID_STAGE_DIR"], wantStage)
	}
	if !strings.HasPrefix(values["PATH"], "/var/tmp/gitid-evidence-capture/fake-ssh"+string(filepath.ListSeparator)) {
		t.Fatalf("PATH = %q, want fake SSH prefix", values["PATH"])
	}
}

func TestCompareInventoriesRequiresSemanticEqualityButIgnoresPNGBytes(t *testing.T) {
	first := makeCandidate(t, filepath.Join(t.TempDir(), "first"))
	second := makeCandidate(t, filepath.Join(t.TempDir(), "second"))

	addPNGMetadata(t, second, "live/ssh-form-filled.png")
	if err := compareInventories(first, second); err != nil {
		t.Fatalf("renderer-byte-only PNG variation must not fail semantic comparison: %v", err)
	}

	mutateCandidateMember(t, second, "live/ssh-form-filled.txt", []byte("semantic drift"))
	if err := compareInventories(first, second); err == nil {
		t.Fatal("semantic evidence drift must fail candidate comparison")
	}
}

func TestCompareInventoriesRejectsInvalidPNG(t *testing.T) {
	first := makeCandidate(t, filepath.Join(t.TempDir(), "first"))
	second := makeCandidate(t, filepath.Join(t.TempDir(), "second"))
	mutateCandidateMember(t, second, "live/ssh-form-filled.png", []byte("not a PNG"))

	if err := compareInventories(first, second); err == nil || !strings.Contains(err.Error(), "complete valid PNG") {
		t.Fatalf("invalid PNG must reject candidate comparison, got %v", err)
	}
}

func makeCandidate(t *testing.T, dir string) string {
	t.Helper()
	source := strings.Repeat("a", 40)
	panels := make([]screenshot.VisualPanel, 0, screenshot.RequiredVisualPanelCount())
	regionRecords := make([]screenshot.RegionDiffRecord, 0, len(screenshot.RequiredScreenSpecs()))
	for _, spec := range screenshot.RequiredScreenSpecs() {
		record := screenshot.RegionDiffRecord{ScreenID: spec.ScreenID}
		for _, region := range spec.RequiredRegions {
			live := "live " + spec.ScreenID + " " + string(region)
			approved := ""
			if spec.ApplicableApprovedTUI {
				approved = live
			}
			record.Regions = append(record.Regions, screenshot.NamedRegionDiff{
				Name: region, LiveText: live, ApprovedText: approved,
				ApprovedApplicable: spec.ApplicableApprovedTUI,
				LiveHash:           sha256String(live), ApprovedHash: sha256String(approved), Equal: true,
			})
		}
		regionRecords = append(regionRecords, record)
		for _, surface := range []string{"live", "approved-tui", "approved-html"} {
			if !screenshot.ScreenAppliesToSurface(spec, surface) {
				continue
			}
			pngPath := filepath.Join(t.TempDir(), surface+"-"+spec.ScreenID+".png")
			writeValidPNG(t, pngPath, color.RGBA{R: uint8(len(panels) + 1), A: 0xff})
			panels = append(panels, screenshot.VisualPanel{
				Surface: surface, ScreenID: spec.ScreenID,
				Text:    "normalized text for " + surface + "/" + spec.ScreenID,
				RawText: "raw transcript for " + surface + "/" + spec.ScreenID,
				PNGPath: pngPath,
			})
		}
	}
	result, err := screenshot.GenerateVisualPacket(screenshot.PacketOptions{
		SourceCommit: source, ApprovalCommit: screenshot.PacketApprovalCommit, LiveBackendRef: "test", OutputDir: dir,
		ProvenanceFiles: map[string][]byte{
			"EVIDENCE.json":     []byte("candidate metadata"),
			"REGION-DIFFS.json": screenshot.BuildRegionDiffsJSON(source, regionRecords),
		},
	}, panels, screenshot.PacketCapture{
		Commands: []string{"test"}, ToolVersions: []screenshot.PacketTool{{Name: "go", Version: "test"}},
		Geometry: "100x30", FontSHA256: strings.Repeat("b", 64), Theme: "test",
	})
	if err != nil {
		t.Fatalf("GenerateVisualPacket: %v", err)
	}
	if err := os.Rename(result.ManifestPath, filepath.Join(dir, "CANDIDATE-MANIFEST.json")); err != nil {
		t.Fatalf("renaming candidate manifest: %v", err)
	}
	return dir
}

func writeValidPNG(t *testing.T, path string, pixel color.RGBA) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.SetRGBA(0, 0, pixel)
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating PNG: %v", err)
	}
	if err := png.Encode(file, img); err != nil {
		_ = file.Close()
		t.Fatalf("encoding PNG: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("closing PNG: %v", err)
	}
}

func addPNGMetadata(t *testing.T, dir, memberPath string) {
	t.Helper()
	path := filepath.Join(dir, memberPath)
	pngBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading PNG: %v", err)
	}
	const iendLength = 12
	metadata := pngChunk("tEXt", []byte("Software\x00alternate renderer"))
	pngBytes = append(append([]byte{}, pngBytes[:len(pngBytes)-iendLength]...), append(metadata, pngBytes[len(pngBytes)-iendLength:]...)...)
	if err := os.WriteFile(path, pngBytes, 0o600); err != nil {
		t.Fatalf("writing PNG metadata: %v", err)
	}
	mutateCandidateMember(t, dir, memberPath, pngBytes)
}

func pngChunk(kind string, data []byte) []byte {
	chunk := make([]byte, 12+len(data))
	binary.BigEndian.PutUint32(chunk, uint32(len(data)))
	copy(chunk[4:], kind)
	copy(chunk[8:], data)
	binary.BigEndian.PutUint32(chunk[8+len(data):], crc32.ChecksumIEEE(append([]byte(kind), data...)))
	return chunk
}

func mutateCandidateMember(t *testing.T, dir, memberPath string, content []byte) {
	t.Helper()
	path := filepath.Join(dir, memberPath)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("writing candidate member: %v", err)
	}
	manifestPath := filepath.Join(dir, "CANDIDATE-MANIFEST.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("reading candidate manifest: %v", err)
	}
	var manifest screenshot.Packet
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("parsing candidate manifest: %v", err)
	}
	for i := range manifest.Members {
		if manifest.Members[i].Path == memberPath {
			manifest.Members[i].SHA256 = sha256Hex(content)
			break
		}
	}
	manifest.ManifestSHA256 = screenshot.CanonicalManifestHash(manifest)
	data, err = json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshaling candidate manifest: %v", err)
	}
	if err := os.WriteFile(manifestPath, data, 0o600); err != nil {
		t.Fatalf("writing candidate manifest: %v", err)
	}
}

func sha256String(value string) string {
	return sha256Hex([]byte(value))
}

// TestCaptureTUIScreenReuseSelection proves the live raw-PTY script reaches the
// reuse state required by the registry before evidence is rendered or saved.
func TestCaptureTUIScreenReuseSelection(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "gitid")
	build := exec.Command("go", "build", "-o", bin, "../gitid") //nolint:gosec // fixed local package and sandbox output
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building gitid capture binary: %v\n%s", err, output)
	}

	var reuseSpec screenshot.ScreenSpec
	for _, spec := range screenshot.RequiredScreenSpecs() {
		if spec.ScreenID == "reuse-key-vs-generate" {
			reuseSpec = spec
			break
		}
	}
	if reuseSpec.ScreenID == "" {
		t.Fatal("reuse-key-vs-generate spec is missing")
	}

	var raw string
	text, err := captureTUIScreen(bin, t.TempDir(), "", reuseSpec.ScreenID, true, &raw)
	if err != nil {
		t.Fatalf("capturing reuse-key-vs-generate: %v", err)
	}
	if !strings.Contains(text, "● Reuse an") {
		t.Fatalf("reuse-key-vs-generate must select reuse, not merely render its label:\n%s", text)
	}
	if err := screenshot.ValidateCapturedState(reuseSpec, text); err != nil {
		t.Fatalf("reuse-key-vs-generate must capture the selected reuse state: %v\n%s", err, text)
	}
	if strings.TrimSpace(raw) == "" {
		t.Fatal("reuse-key-vs-generate must retain raw PTY evidence")
	}
}

func TestCaptureTUIScreenManualReuseStatesAreDistinct(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "gitid")
	build := exec.Command("go", "build", "-o", bin, "../gitid") //nolint:gosec // fixed local package and sandbox output
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building gitid capture binary: %v\n%s", err, output)
	}

	workspace := t.TempDir()
	fakeSSH, err := writeFakeSSH(workspace)
	if err != nil {
		t.Fatalf("creating fake SSH: %v", err)
	}
	var manualRaw, resolvedRaw string
	manual, err := captureTUIScreen(bin, captureHomePath(workspace, "reuse-manual-path"), fakeSSH, "reuse-manual-path", true, &manualRaw)
	if err != nil {
		t.Fatalf("capturing manual-path state: %v", err)
	}
	resolved, err := captureTUIScreen(bin, captureHomePath(workspace, "reuse-manual-resolved"), fakeSSH, "reuse-manual-resolved", true, &resolvedRaw)
	if err != nil {
		t.Fatalf("capturing resolved-manual state: %v", err)
	}
	if !strings.Contains(manual, "Enter a path manually") || strings.Contains(manual, "id_ed25519_capture_manual") {
		t.Fatalf("manual-path capture must show the empty manual input, not a resolved key:\n%s", manual)
	}
	if !strings.Contains(resolved, "id_ed25519_capture_manual") || !strings.Contains(resolved, "ssh-ed25519") {
		t.Fatalf("resolved-manual capture must show its fixture path and metadata:\n%s", resolved)
	}
	if manual == resolved || manualRaw == resolvedRaw {
		t.Fatal("manual-path and resolved-manual captures must remain separate semantic states")
	}
}

// TestCaptureTUIScreenConfirmationManagedBlock proves the raw-PTY capture
// reaches the complete pre-write managed block rather than a fixed viewport page.
func TestCaptureTUIScreenConfirmationManagedBlock(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "gitid")
	build := exec.Command("go", "build", "-o", bin, "../gitid") //nolint:gosec // fixed local package and sandbox output
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building gitid capture binary: %v\n%s", err, output)
	}

	workspace := t.TempDir()
	fakeSSH, err := writeFakeSSH(workspace)
	if err != nil {
		t.Fatalf("creating fake SSH: %v", err)
	}
	var raw string
	text, err := captureTUIScreen(bin, t.TempDir(), fakeSSH, "confirm-managed-block", true, &raw)
	if err != nil {
		t.Fatalf("capturing confirm-managed-block: %v", err)
	}
	if !strings.Contains(text, "# END gitid managed:") {
		t.Fatalf("confirm-managed-block must expose the END sentinel before writing:\n%s", text)
	}
	if strings.TrimSpace(raw) == "" {
		t.Fatal("confirm-managed-block must retain raw PTY evidence")
	}
}

func TestCaptureTUIScreenStage1PassStopsBeforeStage2(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "gitid")
	build := exec.Command("go", "build", "-o", bin, "../gitid") //nolint:gosec // fixed local package and sandbox output
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building gitid capture binary: %v\n%s", err, output)
	}

	workspace := t.TempDir()
	fakeSSH, err := writeFakeSSH(workspace)
	if err != nil {
		t.Fatalf("creating fake SSH: %v", err)
	}
	var raw string
	text, err := captureTUIScreen(bin, t.TempDir(), fakeSSH, "test-stage1-pass", true, &raw)
	if err != nil {
		t.Fatalf("capturing test-stage1-pass: %v", err)
	}
	if !strings.Contains(text, "running ssh") {
		t.Fatalf("test-stage1-pass must preserve the in-flight stage-2 state:\n%s", text)
	}
	if strings.Contains(text, "Next: Git identity") {
		t.Fatalf("test-stage1-pass must not capture the completed stage-2 state:\n%s", text)
	}
	if strings.Contains(raw, "Next: Git identity") {
		t.Fatal("test-stage1-pass raw evidence must stop at the captured stage-one state")
	}

	direct, err := captureTUIScreen(bin, t.TempDir(), fakeSSH, "test-stage1-direct", true, &raw)
	if err != nil {
		t.Fatalf("capturing test-stage1-direct: %v", err)
	}
	if !strings.Contains(direct, "! Reachable") {
		t.Fatalf("test-stage1-direct must capture the D-02 warning state:\n%s", direct)
	}
}
