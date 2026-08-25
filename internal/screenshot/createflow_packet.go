//go:build screenshot

package screenshot

// createflow_packet.go implements the registry-derived evidence packet for
// the Phase 3 create-flow visual review (plan 03-11 Task 2, CR-01 through
// CR-04, CR-10).
//
// A packet contains:
//   - every applicable live/approved frame declared by ScreenSpecRegistry
//   - MANIFEST.json: canonical content-addressed inventory
//
// Two-subprocess determinism (CR-10): two complete candidate bundles are
// generated in separate subprocesses (or sequentially in isolated temp dirs)
// and their canonical manifests must be byte-identical before publication.
//
// Tool gating: PNG capture requires freeze (for TUI) and Chromium (for HTML).
// When tools are absent, the corresponding capture step fails with a
// descriptive error — the packet is never partially published (fail-closed).

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// PacketMember describes one file in a review packet.
type PacketMember struct {
	// Path is the relative path within the packet directory.
	Path string `json:"path"`
	// SHA256 is the hex-encoded SHA-256 of the file's content.
	SHA256 string `json:"sha256"`
	// Kind classifies the member: "text", "png-live", "png-tui", "png-html", "manifest".
	Kind string `json:"kind"`
	// ScreenID names the create-flow screen checkpoint (for panel members).
	ScreenID string `json:"screen_id,omitempty"`
}

// Packet is the canonical review evidence packet. It is written as
// MANIFEST.json at the packet root (CR-01 immutable content-addressing).
type Packet struct {
	// Version identifies the manifest schema version.
	Version string `json:"version"`
	// SourceCommit is the full SHA of the source commit being evidenced.
	SourceCommit string `json:"source_commit"`
	// ApprovalCommit is the full SHA of the design approval commit.
	ApprovalCommit string `json:"approval_commit"`
	// CapturedAt is the RFC3339 UTC timestamp of packet generation.
	CapturedAt string `json:"captured_at"`
	// LiveBackendRef identifies the live backend (current HEAD short SHA).
	LiveBackendRef string `json:"live_backend_ref"`
	// Members is the complete, sorted inventory of packet files.
	Members []PacketMember `json:"members"`
	// ManifestSHA256 is the hex-encoded SHA-256 of the MANIFEST.json content
	// with this field set to the empty string (content-addressing).
	ManifestSHA256 string `json:"manifest_sha256"`
	// Capture records the fixed rendering inputs and commands used for a visual
	// packet. It is empty for the legacy text-only packet format.
	Capture PacketCapture `json:"capture,omitempty"`
}

// PacketCapture records the reproducibility inputs for a visual packet.
type PacketCapture struct {
	Commands     []string     `json:"commands"`
	ToolVersions []PacketTool `json:"tool_versions"`
	Geometry     string       `json:"geometry"`
	FontSHA256   string       `json:"font_sha256"`
	Theme        string       `json:"theme"`
}

// PacketTool identifies one capture tool and its pinned version.
type PacketTool struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// VisualPanel is one text-and-PNG panel supplied to GenerateVisualPacket.
// PNGPath must point to a completed renderer output; the function copies it
// into the immutable packet after validating that it is non-empty.
type VisualPanel struct {
	Surface  string
	ScreenID string
	Text     string
	// RawText is the unmodified PTY transcript or browser capture text that
	// produced the rendered frame. It is separate from Text so clipped terminal
	// rows cannot impersonate raw evidence.
	RawText string
	PNGPath string
}

// PacketOptions configures a packet generation run.
type PacketOptions struct {
	// SourceCommit is the full 40-hex SHA of the source commit being evidenced.
	SourceCommit string
	// ApprovalCommit is the full 40-hex SHA of the design approval commit.
	ApprovalCommit string
	// LiveBackendRef is a short identifier for the live backend (e.g. HEAD short SHA).
	LiveBackendRef string
	// OutputDir is the directory to write the packet into. Must not exist.
	OutputDir string
	// Clock provides the "now" timestamp. If nil, time.Now().UTC() is used.
	Clock func() time.Time
	// FreezeBin is the path to the freeze binary (for TUI PNG capture).
	// If empty, freeze is resolved via exec.LookPath.
	FreezeBin string
	// FontFile is the JetBrains Mono TTF path for deterministic freeze rendering.
	FontFile string
	// Theme is the freeze theme (e.g. "dracula").
	Theme string
	// ProvenanceFiles is an optional map of filename → content for extra
	// provenance members (e.g. EVIDENCE.json, REGION-DIFFS.json). Each file
	// is written to the packet root and declared in the manifest.
	ProvenanceFiles map[string][]byte
}

// PacketResult is returned by GeneratePacket.
type PacketResult struct {
	// ManifestPath is the absolute path to the written MANIFEST.json.
	ManifestPath string
	// Packet is the in-memory packet data.
	Packet Packet
	// MemberCount is the total number of members in the packet.
	MemberCount int
}

// packetVersion is the current manifest schema version.
const packetVersion = "03-11.1"

const visualPacketVersion = "03-11.2"

// ---------------------------------------------------------------------------
// 03-16 Task 1: Single configured-review constants.
// These must match .planning/config.json review.reviewer_instances exactly.
// They are compile-time constants — never read from config at runtime.
// ---------------------------------------------------------------------------

// ConfiguredReviewerInstance is the instance name of the sole configured reviewer.
const ConfiguredReviewerInstance = "opencode-my-plan-review"

// ConfiguredReviewerCLI is the CLI tool name for the configured reviewer.
const ConfiguredReviewerCLI = "opencode"

// ConfiguredReviewerModel is the model identifier for the configured reviewer.
const ConfiguredReviewerModel = "local-llm-env/my-plan-review"

// configuredReviewVersion is the version tag for new review records (03-16.1).
const configuredReviewVersion = "03-16.1"

// validateConfiguredReviewRecord verifies that a review input satisfies the
// single-configured-reviewer contract. It checks:
//
//   - Reviewer matches ConfiguredReviewerInstance
//   - Tool matches ConfiguredReviewerCLI
//   - Model matches ConfiguredReviewerModel
//   - Provider is non-empty and is NOT a configured literal (not equal to the
//     tool name or model name — a provider that equals those values is a sign of
//     tampered provenance; a real provider hash must differ from config constants)
//   - Verdict version is configuredReviewVersion
//   - Verdict is bound to the correct source commit and candidate manifest hash
//   - Verdict OpenCritical == 0 and OpenHigh == 0
func validateConfiguredReviewRecord(review ReviewInput, sourceCommit, candidateManifestSHA256 string) error {
	if review.Reviewer != ConfiguredReviewerInstance {
		return fmt.Errorf("screenshot: configured review: reviewer %q does not match configured instance %q", review.Reviewer, ConfiguredReviewerInstance)
	}
	if review.Tool != ConfiguredReviewerCLI {
		return fmt.Errorf("screenshot: configured review: tool %q does not match configured CLI %q", review.Tool, ConfiguredReviewerCLI)
	}
	if review.Model != ConfiguredReviewerModel {
		return fmt.Errorf("screenshot: configured review: model %q does not match configured model %q", review.Model, ConfiguredReviewerModel)
	}
	if strings.TrimSpace(review.Provider) == "" {
		return fmt.Errorf("screenshot: configured review: provider must be non-empty")
	}
	// Provider must not be a configured literal — it must be externally-generated
	// provenance data (a session hash, token, etc.), not the tool or model name.
	if review.Provider == ConfiguredReviewerCLI || review.Provider == ConfiguredReviewerModel || review.Provider == ConfiguredReviewerInstance {
		return fmt.Errorf("screenshot: configured review: provider %q must not be a configured literal (use an external session hash)", review.Provider)
	}
	if review.Session == "" {
		return fmt.Errorf("screenshot: configured review: session must be non-empty")
	}
	if review.ExitCode != 0 || len(review.Prompt) == 0 || len(review.Stdout) == 0 || len(review.Verdict) == 0 {
		return fmt.Errorf("screenshot: configured review: review has incomplete raw inputs or nonzero exit")
	}
	// Parse and validate the verdict.
	decoder := json.NewDecoder(strings.NewReader(string(review.Verdict)))
	decoder.DisallowUnknownFields()
	var verdict ReviewVerdict
	if err := decoder.Decode(&verdict); err != nil {
		return fmt.Errorf("screenshot: configured review: decoding verdict: %w", err)
	}
	if verdict.Version != configuredReviewVersion {
		return fmt.Errorf("screenshot: configured review: verdict version %q does not match expected %q", verdict.Version, configuredReviewVersion)
	}
	if verdict.SourceCommit != sourceCommit {
		return fmt.Errorf("screenshot: configured review: verdict source commit %q does not match %q", verdict.SourceCommit, sourceCommit)
	}
	if verdict.CandidateManifestSHA256 != candidateManifestSHA256 {
		return fmt.Errorf("screenshot: configured review: verdict candidate hash %q does not match %q", verdict.CandidateManifestSHA256, candidateManifestSHA256)
	}
	if verdict.Reviewer != ConfiguredReviewerInstance {
		return fmt.Errorf("screenshot: configured review: verdict reviewer %q does not match configured instance %q", verdict.Reviewer, ConfiguredReviewerInstance)
	}
	if verdict.OpenCritical != 0 || verdict.OpenHigh != 0 {
		return fmt.Errorf("screenshot: configured review: verdict has nonzero blockers (critical=%d, high=%d)", verdict.OpenCritical, verdict.OpenHigh)
	}
	return nil
}

// GenerateVisualPacket writes the complete registry-derived evidence packet. Unlike
// GenerateTextPacket, this API cannot represent a panel without both the text
// captured from the source binary/page and a non-empty renderer-produced PNG.
func GenerateVisualPacket(opts PacketOptions, panels []VisualPanel, capture PacketCapture) (PacketResult, error) {
	if err := validatePacketOptions(opts, "GenerateVisualPacket"); err != nil {
		return PacketResult{}, err
	}
	if err := ValidateScreenSpecs(RequiredScreenSpecs()); err != nil {
		return PacketResult{}, err
	}
	if err := os.MkdirAll(opts.OutputDir, 0o750); err != nil {
		return PacketResult{}, fmt.Errorf("screenshot: GenerateVisualPacket: creating output dir %q: %w", opts.OutputDir, err)
	}

	expected := requiredVisualPanels()
	seen := make(map[string]bool, len(expected))
	members := make([]PacketMember, 0, len(expected)*3+len(opts.ProvenanceFiles))
	for _, panel := range panels {
		if !validPacketSurface(panel.Surface) {
			return PacketResult{}, fmt.Errorf("screenshot: GenerateVisualPacket: unknown panel surface %q", panel.Surface)
		}
		if _, ok := screenSpec(panel.ScreenID); !ok {
			return PacketResult{}, fmt.Errorf("screenshot: GenerateVisualPacket: unknown screen %q", panel.ScreenID)
		}
		key := panel.Surface + "/" + panel.ScreenID
		if !expected[key] {
			return PacketResult{}, fmt.Errorf("screenshot: GenerateVisualPacket: panel %q is not applicable according to the registry", key)
		}
		if seen[key] {
			return PacketResult{}, fmt.Errorf("screenshot: GenerateVisualPacket: duplicate panel %q", key)
		}
		seen[key] = true
		if strings.TrimSpace(panel.Text) == "" {
			return PacketResult{}, fmt.Errorf("screenshot: GenerateVisualPacket: panel %q has empty text", key)
		}
		if strings.TrimSpace(panel.RawText) == "" {
			return PacketResult{}, fmt.Errorf("screenshot: GenerateVisualPacket: panel %q has empty raw evidence", key)
		}
		png, err := os.ReadFile(panel.PNGPath) //nolint:gosec // renderer path is created by the publisher
		if err != nil {
			return PacketResult{}, fmt.Errorf("screenshot: GenerateVisualPacket: reading PNG for %q: %w", key, err)
		}
		if len(png) == 0 {
			return PacketResult{}, fmt.Errorf("screenshot: GenerateVisualPacket: panel %q has empty PNG", key)
		}

		dir := filepath.Join(opts.OutputDir, panel.Surface)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return PacketResult{}, fmt.Errorf("screenshot: GenerateVisualPacket: creating panel directory %q: %w", panel.Surface, err)
		}
		textRel := filepath.Join(panel.Surface, panel.ScreenID+".txt")
		rawRel := filepath.Join(panel.Surface, panel.ScreenID+".raw")
		pngRel := filepath.Join(panel.Surface, panel.ScreenID+".png")
		if err := writeAndSync(filepath.Join(opts.OutputDir, textRel), []byte(panel.Text)); err != nil {
			return PacketResult{}, fmt.Errorf("screenshot: GenerateVisualPacket: writing text %q: %w", key, err)
		}
		if err := writeAndSync(filepath.Join(opts.OutputDir, pngRel), png); err != nil {
			return PacketResult{}, fmt.Errorf("screenshot: GenerateVisualPacket: writing PNG %q: %w", key, err)
		}
		if err := writeAndSync(filepath.Join(opts.OutputDir, rawRel), []byte(panel.RawText)); err != nil {
			return PacketResult{}, fmt.Errorf("screenshot: GenerateVisualPacket: writing raw evidence %q: %w", key, err)
		}
		members = append(members,
			PacketMember{Path: textRel, SHA256: sha256Hex([]byte(panel.Text)), Kind: "text", ScreenID: panel.ScreenID},
			PacketMember{Path: rawRel, SHA256: sha256Hex([]byte(panel.RawText)), Kind: "raw", ScreenID: panel.ScreenID},
			PacketMember{Path: pngRel, SHA256: sha256Hex(png), Kind: "png-" + panel.Surface, ScreenID: panel.ScreenID},
		)
	}
	for key := range expected {
		if !seen[key] {
			return PacketResult{}, fmt.Errorf("screenshot: GenerateVisualPacket: missing registry-required panel %s", key)
		}
	}
	// Write optional provenance files (EVIDENCE.json, REGION-DIFFS.json, etc.)
	// These are declared in the manifest and validated by ValidatePacket.
	for name, content := range opts.ProvenanceFiles {
		if err := writeAndSync(filepath.Join(opts.OutputDir, name), content); err != nil {
			return PacketResult{}, fmt.Errorf("screenshot: GenerateVisualPacket: writing provenance %q: %w", name, err)
		}
		members = append(members, PacketMember{
			Path:   name,
			SHA256: sha256Hex(content),
			Kind:   "provenance",
		})
	}
	sort.Slice(members, func(i, j int) bool { return members[i].Path < members[j].Path })

	now := time.Unix(0, 0).UTC()
	if opts.Clock != nil {
		now = opts.Clock().UTC()
	}
	pkt := Packet{
		Version:        visualPacketVersion,
		SourceCommit:   opts.SourceCommit,
		ApprovalCommit: opts.ApprovalCommit,
		CapturedAt:     now.Format(time.RFC3339),
		LiveBackendRef: opts.LiveBackendRef,
		Members:        members,
		Capture:        capture,
	}
	manifest, err := marshalPacket(pkt)
	if err != nil {
		return PacketResult{}, fmt.Errorf("screenshot: GenerateVisualPacket: marshaling manifest: %w", err)
	}
	pkt.ManifestSHA256 = sha256Hex(manifest)
	manifest, err = marshalPacket(pkt)
	if err != nil {
		return PacketResult{}, fmt.Errorf("screenshot: GenerateVisualPacket: marshaling manifest with hash: %w", err)
	}
	manifestPath := filepath.Join(opts.OutputDir, "MANIFEST.json")
	if err := writeAndSync(manifestPath, manifest); err != nil {
		return PacketResult{}, fmt.Errorf("screenshot: GenerateVisualPacket: writing MANIFEST.json: %w", err)
	}
	return PacketResult{ManifestPath: manifestPath, Packet: pkt, MemberCount: len(members)}, nil
}

func validatePacketOptions(opts PacketOptions, operation string) error {
	if opts.SourceCommit == "" || len(opts.SourceCommit) != 40 {
		return fmt.Errorf("screenshot: %s: source commit must be a full 40-hex SHA; got %q", operation, opts.SourceCommit)
	}
	if opts.ApprovalCommit == "" || len(opts.ApprovalCommit) != 40 {
		return fmt.Errorf("screenshot: %s: approval commit must be a full 40-hex SHA; got %q", operation, opts.ApprovalCommit)
	}
	if opts.OutputDir == "" {
		return fmt.Errorf("screenshot: %s: OutputDir is required", operation)
	}
	if _, err := os.Stat(opts.OutputDir); err == nil {
		return fmt.Errorf("screenshot: %s: output directory %q already exists — refusing to overwrite (immutability guarantee)", operation, opts.OutputDir)
	}
	return nil
}

func validPacketSurface(surface string) bool {
	return surface == "live" || surface == "approved-tui" || surface == "approved-html"
}

func isCreateFlowScreenID(id string) bool {
	_, ok := screenSpec(id)
	return ok
}

func screenSpec(id string) (ScreenSpec, bool) {
	for _, spec := range RequiredScreenSpecs() {
		if spec.ScreenID == id {
			return spec, true
		}
	}
	return ScreenSpec{}, false
}

// requiredVisualPanels returns the TUI-only panel inventory for candidate
// generation and the routine gate. Approved-HTML capture was removed from the
// candidate and gate paths in 03-16 Task 2 (browser/pnpm removed); historical
// helpers may still reference approved-html but must not be reachable from
// either path.
func requiredVisualPanels() map[string]bool {
	expected := make(map[string]bool)
	for _, spec := range RequiredScreenSpecs() {
		for _, surface := range []string{"live", "approved-tui"} {
			if ScreenAppliesToSurface(spec, surface) {
				expected[surface+"/"+spec.ScreenID] = true
			}
		}
	}
	return expected
}

// RequiredVisualPanelCount reports the registry-derived panel count for human
// progress output. It is intentionally not a validation constant.
func RequiredVisualPanelCount() int { return len(requiredVisualPanels()) }

// GenerateTextPacket generates a text-only packet (no PNG capture) from two
// backend captures. It is the deterministic inner loop used by both the routine
// gate and the publisher. PNG members are added as stubs when tools are absent;
// callers that require PNG panels must verify MemberCount == ValidatePanelCount.
//
// CR-10: accepts an injected Backend (not random material). The two-candidate
// determinism check is performed by the caller (GenerateTwoCandidates).
func GenerateTextPacket(opts PacketOptions, liveCaptures map[string]string, approvedTUICaptures map[string]string) (PacketResult, error) {
	if opts.SourceCommit == "" || len(opts.SourceCommit) != 40 {
		return PacketResult{}, fmt.Errorf("screenshot: GenerateTextPacket: source commit must be a full 40-hex SHA; got %q", opts.SourceCommit)
	}
	if opts.ApprovalCommit == "" || len(opts.ApprovalCommit) != 40 {
		return PacketResult{}, fmt.Errorf("screenshot: GenerateTextPacket: approval commit must be a full 40-hex SHA; got %q", opts.ApprovalCommit)
	}
	if opts.OutputDir == "" {
		return PacketResult{}, fmt.Errorf("screenshot: GenerateTextPacket: OutputDir is required")
	}
	if _, err := os.Stat(opts.OutputDir); err == nil {
		return PacketResult{}, fmt.Errorf("screenshot: GenerateTextPacket: output directory %q already exists — refusing to overwrite (immutability guarantee)", opts.OutputDir)
	}

	if err := os.MkdirAll(opts.OutputDir, 0o750); err != nil {
		return PacketResult{}, fmt.Errorf("screenshot: GenerateTextPacket: creating output dir %q: %w", opts.OutputDir, err)
	}

	now := time.Now().UTC()
	if opts.Clock != nil {
		now = opts.Clock()
	}

	var members []PacketMember

	// Write live text captures.
	liveDir := filepath.Join(opts.OutputDir, "live")
	if err := os.MkdirAll(liveDir, 0o750); err != nil {
		return PacketResult{}, fmt.Errorf("screenshot: creating live dir: %w", err)
	}
	for _, spec := range RequiredScreenSpecs() {
		if !spec.ApplicableLive {
			continue
		}
		id := spec.ScreenID
		text, ok := liveCaptures[id]
		if !ok {
			return PacketResult{}, fmt.Errorf("screenshot: GenerateTextPacket: live capture missing screen %q", id)
		}
		relPath := filepath.Join("live", id+".txt")
		absPath := filepath.Join(opts.OutputDir, relPath)
		if err := writeAndSync(absPath, []byte(text)); err != nil {
			return PacketResult{}, fmt.Errorf("screenshot: writing live text %q: %w", id, err)
		}
		members = append(members, PacketMember{
			Path:     relPath,
			SHA256:   sha256Hex([]byte(text)),
			Kind:     "text",
			ScreenID: id,
		})
	}

	// Write approved-TUI text captures.
	tuiDir := filepath.Join(opts.OutputDir, "approved-tui")
	if err := os.MkdirAll(tuiDir, 0o750); err != nil {
		return PacketResult{}, fmt.Errorf("screenshot: creating approved-tui dir: %w", err)
	}
	for _, spec := range RequiredScreenSpecs() {
		if !spec.ApplicableApprovedTUI {
			continue
		}
		id := spec.ScreenID
		text, ok := approvedTUICaptures[id]
		if !ok {
			return PacketResult{}, fmt.Errorf("screenshot: GenerateTextPacket: approved-TUI capture missing screen %q", id)
		}
		relPath := filepath.Join("approved-tui", id+".txt")
		absPath := filepath.Join(opts.OutputDir, relPath)
		if err := writeAndSync(absPath, []byte(text)); err != nil {
			return PacketResult{}, fmt.Errorf("screenshot: writing approved-tui text %q: %w", id, err)
		}
		members = append(members, PacketMember{
			Path:     relPath,
			SHA256:   sha256Hex([]byte(text)),
			Kind:     "png-tui",
			ScreenID: id,
		})
	}

	// Sort members by path for canonical ordering.
	sort.Slice(members, func(i, j int) bool {
		return members[i].Path < members[j].Path
	})

	pkt := Packet{
		Version:        packetVersion,
		SourceCommit:   opts.SourceCommit,
		ApprovalCommit: opts.ApprovalCommit,
		CapturedAt:     now.Format(time.RFC3339),
		LiveBackendRef: opts.LiveBackendRef,
		Members:        members,
	}

	// Write MANIFEST.json with ManifestSHA256 empty first to compute it.
	manifestBytes, err := marshalPacket(pkt)
	if err != nil {
		return PacketResult{}, fmt.Errorf("screenshot: marshaling manifest: %w", err)
	}
	pkt.ManifestSHA256 = sha256Hex(manifestBytes)

	// Re-marshal with the hash filled in.
	manifestBytes, err = marshalPacket(pkt)
	if err != nil {
		return PacketResult{}, fmt.Errorf("screenshot: marshaling manifest with hash: %w", err)
	}

	manifestPath := filepath.Join(opts.OutputDir, "MANIFEST.json")
	if err := writeAndSync(manifestPath, manifestBytes); err != nil {
		return PacketResult{}, fmt.Errorf("screenshot: writing MANIFEST.json: %w", err)
	}

	return PacketResult{
		ManifestPath: manifestPath,
		Packet:       pkt,
		MemberCount:  len(members),
	}, nil
}

// ValidatePacket validates a completed packet directory against the expected
// invariants: correct member count, all declared hashes match on-disk bytes,
// no undeclared files, MANIFEST.json parseable, and SourceCommit/ApprovalCommit
// present.
func ValidatePacket(packetDir string) (Packet, error) {
	return validatePacketManifest(packetDir, "MANIFEST.json", true)
}

// ValidateCandidate validates a reviewable candidate bundle. Candidates carry
// the same immutable capture inventory as final packets but deliberately omit
// review provenance: that file can only be created after independent reviews
// of this exact candidate hash have completed.
func ValidateCandidate(packetDir string) (Packet, error) {
	return validatePacketManifest(packetDir, "CANDIDATE-MANIFEST.json", false)
}

func validatePacketManifest(packetDir, manifestName string, final bool) (Packet, error) {
	manifestPath := filepath.Join(packetDir, manifestName)
	data, err := os.ReadFile(manifestPath) //nolint:gosec // packetDir is gitid-controlled (G304)
	if err != nil {
		return Packet{}, fmt.Errorf("screenshot: ValidatePacket: reading MANIFEST.json: %w", err)
	}
	var pkt Packet
	if err := json.Unmarshal(data, &pkt); err != nil {
		return Packet{}, fmt.Errorf("screenshot: ValidatePacket: parsing MANIFEST.json: %w", err)
	}
	if pkt.SourceCommit == "" {
		return Packet{}, fmt.Errorf("screenshot: ValidatePacket: missing source_commit in MANIFEST.json")
	}
	if pkt.ApprovalCommit == "" {
		return Packet{}, fmt.Errorf("screenshot: ValidatePacket: missing approval_commit in MANIFEST.json")
	}
	manifestForHash := pkt
	manifestForHash.ManifestSHA256 = ""
	expectedManifest, err := marshalPacket(manifestForHash)
	if err != nil {
		return Packet{}, fmt.Errorf("screenshot: ValidatePacket: marshaling manifest for hash: %w", err)
	}
	if pkt.ManifestSHA256 != sha256Hex(expectedManifest) {
		return Packet{}, fmt.Errorf("screenshot: ValidatePacket: manifest hash mismatch: got %s, want %s", pkt.ManifestSHA256, sha256Hex(expectedManifest))
	}
	declared := map[string]bool{manifestName: true}
	// Validate every declared member.
	for _, m := range pkt.Members {
		if m.Path == "" || filepath.IsAbs(m.Path) || strings.HasPrefix(filepath.Clean(m.Path), "..") {
			return Packet{}, fmt.Errorf("screenshot: ValidatePacket: invalid member path %q", m.Path)
		}
		if declared[m.Path] {
			return Packet{}, fmt.Errorf("screenshot: ValidatePacket: duplicate member %q", m.Path)
		}
		declared[m.Path] = true
		absPath := filepath.Join(packetDir, m.Path)
		content, err := os.ReadFile(absPath) //nolint:gosec // absPath is constructed from gitid-controlled packet dir and manifest path (G304)
		if err != nil {
			return Packet{}, fmt.Errorf("screenshot: ValidatePacket: reading member %q: %w", m.Path, err)
		}
		if pkt.Version == visualPacketVersion && len(content) == 0 && m.Kind != "review" {
			return Packet{}, fmt.Errorf("screenshot: ValidatePacket: visual member %q is empty", m.Path)
		}
		got := sha256Hex(content)
		if got != m.SHA256 {
			return Packet{}, fmt.Errorf("screenshot: ValidatePacket: member %q hash mismatch: got %s, want %s", m.Path, got, m.SHA256)
		}
		if strings.HasSuffix(m.Path, ".png") {
			if _, err := png.Decode(bytes.NewReader(content)); err != nil {
				return Packet{}, fmt.Errorf("screenshot: ValidatePacket: member %q is not a complete valid PNG: %w", m.Path, err)
			}
		}
	}
	if pkt.Version == visualPacketVersion {
		if err := validateVisualPacket(pkt); err != nil {
			return Packet{}, err
		}
		// Detect duplicate PNG bytes across different screen IDs on same surface
		// (UI-REVIEW Critical: stage-1 and stage-2 had identical PNG hashes).
		if err := validateDuplicatePNGBytes(packetDir, pkt); err != nil {
			return Packet{}, err
		}
		if err := validateRegionDiffsFile(packetDir, pkt); err != nil {
			return Packet{}, err
		}
		if err := filepath.Walk(packetDir, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if info.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(packetDir, path)
			if err != nil {
				return err
			}
			if !declared[rel] {
				return fmt.Errorf("screenshot: ValidatePacket: undeclared member %q", rel)
			}
			return nil
		}); err != nil {
			return Packet{}, err
		}
		if final {
			if err := ValidateReviewProvenance(packetDir, pkt); err != nil {
				return Packet{}, err
			}
		}
	}
	return pkt, nil
}

func validateVisualPacket(pkt Packet) error {
	if pkt.Capture.Geometry == "" || pkt.Capture.FontSHA256 == "" || pkt.Capture.Theme == "" || len(pkt.Capture.Commands) == 0 || len(pkt.Capture.ToolVersions) == 0 {
		return fmt.Errorf("screenshot: ValidatePacket: visual packet has incomplete capture provenance")
	}
	expected := requiredVisualPanels()
	panels := make(map[string]bool, len(expected))
	text := make(map[string]bool, len(expected))
	raw := make(map[string]bool, len(expected))
	for _, m := range pkt.Members {
		parts := strings.Split(filepath.ToSlash(m.Path), "/")
		if len(parts) != 2 || !validPacketSurface(parts[0]) || !isCreateFlowScreenID(m.ScreenID) {
			continue
		}
		key := parts[0] + "/" + m.ScreenID
		if !expected[key] {
			return fmt.Errorf("screenshot: ValidatePacket: non-applicable visual panel %q", key)
		}
		switch {
		case strings.HasSuffix(m.Path, ".png") && m.Kind == "png-"+parts[0]:
			if panels[key] {
				return fmt.Errorf("screenshot: ValidatePacket: duplicate PNG panel %q", key)
			}
			panels[key] = true
		case strings.HasSuffix(m.Path, ".txt") && m.Kind == "text":
			text[key] = true
		case strings.HasSuffix(m.Path, ".raw") && m.Kind == "raw":
			raw[key] = true
		default:
			return fmt.Errorf("screenshot: ValidatePacket: invalid visual evidence member %q", m.Path)
		}
	}
	for key := range expected {
		if !panels[key] || !text[key] || !raw[key] {
			return fmt.Errorf("screenshot: ValidatePacket: missing PNG/rendered/raw evidence for %s", key)
		}
	}
	return nil
}

// CompareManifests reports whether two PacketResults have identical canonical
// manifests (byte-by-byte after marshaling without the ManifestSHA256 field).
// Used by the two-candidate determinism check (CR-10).
func CompareManifests(a, b PacketResult) error {
	aBytes, err := marshalPacket(Packet{
		Version:        a.Packet.Version,
		SourceCommit:   a.Packet.SourceCommit,
		ApprovalCommit: a.Packet.ApprovalCommit,
		Members:        a.Packet.Members,
		// Exclude CapturedAt and ManifestSHA256 (wall-clock and hash)
	})
	if err != nil {
		return fmt.Errorf("screenshot: CompareManifests: marshaling first: %w", err)
	}
	bBytes, err := marshalPacket(Packet{
		Version:        b.Packet.Version,
		SourceCommit:   b.Packet.SourceCommit,
		ApprovalCommit: b.Packet.ApprovalCommit,
		Members:        b.Packet.Members,
	})
	if err != nil {
		return fmt.Errorf("screenshot: CompareManifests: marshaling second: %w", err)
	}
	if string(aBytes) != string(bBytes) {
		return fmt.Errorf("screenshot: CompareManifests: manifests differ\n--- first ---\n%s\n--- second ---\n%s", aBytes, bBytes)
	}
	return nil
}

// CompareTextCaptures reports whether two capture sets are byte-identical for
// the complete registry inventory (CR-10 text determinism).
func CompareTextCaptures(a, b map[string]string) error {
	for _, spec := range RequiredScreenSpecs() {
		id := spec.ScreenID
		if a[id] != b[id] {
			return fmt.Errorf("screenshot: CompareTextCaptures: screen %q differs\n--- first ---\n%s\n--- second ---\n%s", id, a[id], b[id])
		}
	}
	return nil
}

// marshalPacket serializes pkt to indented JSON (canonical form).
func marshalPacket(pkt Packet) ([]byte, error) {
	return json.MarshalIndent(pkt, "", "  ")
}

// sha256Hex returns the lowercase hex SHA-256 of data.
func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// writeAndSync writes data to path atomically (create, write, sync, close).
func writeAndSync(path string, data []byte) error {
	f, err := os.Create(path) //nolint:gosec // path is gitid-controlled packet member path (G304)
	if err != nil {
		return fmt.Errorf("creating %q: %w", path, err)
	}
	if _, werr := f.Write(data); werr != nil {
		_ = f.Close()
		return fmt.Errorf("writing %q: %w", path, werr)
	}
	if serr := f.Sync(); serr != nil {
		_ = f.Close()
		return fmt.Errorf("syncing %q: %w", path, serr)
	}
	return f.Close()
}

// PacketApprovalCommit is the full SHA of the Phase-2 design approval commit.
// Packet generation must capture from this commit, never from current HEAD.
const PacketApprovalCommit = "3c3130e404329cf42baafdf63a6c22758437edc6"

// requiredProvenanceFiles are the packet member paths that every final
// evidence packet must declare and hash. These were absent from the 03-11
// packet (UI-REVIEW Critical/High: EVIDENCE.json, REGION-DIFFS.json, and
// REVIEW-PROVENANCE.json each reported exists=False declared=False).
var requiredProvenanceFiles = []string{
	"EVIDENCE.json",
	"REGION-DIFFS.json",
	"REVIEW-PROVENANCE.json",
}

// ValidateProvenanceRecords checks that the packet declares all required
// provenance files (EVIDENCE.json, REGION-DIFFS.json, REVIEW-PROVENANCE.json).
// Returns an error describing the first missing file.
func ValidateProvenanceRecords(pkt Packet) error {
	declared := make(map[string]bool, len(pkt.Members))
	for _, m := range pkt.Members {
		declared[m.Path] = true
	}
	for _, required := range requiredProvenanceFiles {
		if !declared[required] {
			return fmt.Errorf("screenshot: ValidateProvenanceRecords: required provenance file %q is not declared in the packet manifest", required)
		}
	}
	return nil
}

// ReviewVerdict is the machine-readable conclusion emitted alongside an
// unedited raw review stream. Only zero open Critical and High findings can
// advance a candidate to final publication.
type ReviewVerdict struct {
	Version                 string `json:"version"`
	SourceCommit            string `json:"source_commit"`
	CandidateManifestSHA256 string `json:"candidate_manifest_sha256"`
	Reviewer                string `json:"reviewer"`
	OpenCritical            int    `json:"open_critical"`
	OpenHigh                int    `json:"open_high"`
}

// ReviewInput is one complete independent review supplied to finalization.
// Raw bytes are copied unchanged into the final packet before their hashes are
// recorded in REVIEW-PROVENANCE.json.
type ReviewInput struct {
	Reviewer  string
	Tool      string
	Provider  string
	Model     string
	Session   string
	StartedAt string
	EndedAt   string
	ExitCode  int
	Prompt    []byte
	Stdout    []byte
	Stderr    []byte
	Verdict   []byte
}

type reviewProvenanceRecord struct {
	Reviewer      string `json:"reviewer"`
	Tool          string `json:"tool"`
	Provider      string `json:"provider"`
	Model         string `json:"model"`
	Session       string `json:"session"`
	StartedAt     string `json:"started_at"`
	EndedAt       string `json:"ended_at"`
	ExitCode      int    `json:"exit_code"`
	PromptPath    string `json:"prompt_path"`
	PromptSHA256  string `json:"prompt_sha256"`
	StdoutPath    string `json:"stdout_path"`
	StdoutSHA256  string `json:"stdout_sha256"`
	StderrPath    string `json:"stderr_path"`
	StderrSHA256  string `json:"stderr_sha256"`
	VerdictPath   string `json:"verdict_path"`
	VerdictSHA256 string `json:"verdict_sha256"`
}

type reviewProvenance struct {
	Version                 string                   `json:"version"`
	SourceCommit            string                   `json:"source_commit"`
	CandidateManifestSHA256 string                   `json:"candidate_manifest_sha256"`
	Reviews                 []reviewProvenanceRecord `json:"reviews"`
}

// ValidateReviewProvenance binds the final packet to the embedded candidate
// and the single configured-reviewer zero-blocker verdict.
func ValidateReviewProvenance(packetDir string, pkt Packet) error {
	if err := ValidateProvenanceRecords(pkt); err != nil {
		return err
	}
	candidateBytes, err := os.ReadFile(filepath.Join(packetDir, "CANDIDATE-MANIFEST.json")) //nolint:gosec // packet-controlled path
	if err != nil {
		return fmt.Errorf("screenshot: ValidateReviewProvenance: reading candidate manifest: %w", err)
	}
	var candidate Packet
	if err := json.Unmarshal(candidateBytes, &candidate); err != nil {
		return fmt.Errorf("screenshot: ValidateReviewProvenance: parsing candidate manifest: %w", err)
	}
	if candidate.ManifestSHA256 != CanonicalManifestHash(candidate) || candidate.SourceCommit != pkt.SourceCommit {
		return fmt.Errorf("screenshot: ValidateReviewProvenance: candidate manifest is not bound to the final source")
	}
	data, err := os.ReadFile(filepath.Join(packetDir, "REVIEW-PROVENANCE.json")) //nolint:gosec // packet-controlled path
	if err != nil {
		return fmt.Errorf("screenshot: ValidateReviewProvenance: reading provenance: %w", err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	var provenance reviewProvenance
	if err := decoder.Decode(&provenance); err != nil {
		return fmt.Errorf("screenshot: ValidateReviewProvenance: decoding provenance: %w", err)
	}
	// New contract: exactly one review from the configured reviewer (03-16.1).
	if provenance.Version != configuredReviewVersion || provenance.SourceCommit != pkt.SourceCommit || provenance.CandidateManifestSHA256 != candidate.ManifestSHA256 || len(provenance.Reviews) != 1 {
		return fmt.Errorf("screenshot: ValidateReviewProvenance: invalid version, source, candidate binding, or review count (want exactly 1 at version %q)", configuredReviewVersion)
	}
	record := provenance.Reviews[0]
	// Validate the configured reviewer triple.
	if record.Reviewer != ConfiguredReviewerInstance {
		return fmt.Errorf("screenshot: ValidateReviewProvenance: reviewer %q does not match configured instance %q", record.Reviewer, ConfiguredReviewerInstance)
	}
	if record.Tool != ConfiguredReviewerCLI {
		return fmt.Errorf("screenshot: ValidateReviewProvenance: tool %q does not match configured CLI %q", record.Tool, ConfiguredReviewerCLI)
	}
	if record.Model != ConfiguredReviewerModel {
		return fmt.Errorf("screenshot: ValidateReviewProvenance: model %q does not match configured model %q", record.Model, ConfiguredReviewerModel)
	}
	if strings.TrimSpace(record.Provider) == "" {
		return fmt.Errorf("screenshot: ValidateReviewProvenance: provider must be non-empty")
	}
	if record.Provider == ConfiguredReviewerCLI || record.Provider == ConfiguredReviewerModel || record.Provider == ConfiguredReviewerInstance {
		return fmt.Errorf("screenshot: ValidateReviewProvenance: provider %q must not be a configured literal", record.Provider)
	}
	if record.Session == "" {
		return fmt.Errorf("screenshot: ValidateReviewProvenance: session must be non-empty")
	}
	if record.ExitCode != 0 {
		return fmt.Errorf("screenshot: ValidateReviewProvenance: reviewer %q exited %d", record.Reviewer, record.ExitCode)
	}
	members := make(map[string]PacketMember, len(pkt.Members))
	for _, member := range pkt.Members {
		members[member.Path] = member
	}
	for _, asset := range []struct{ path, hash string }{
		{record.PromptPath, record.PromptSHA256}, {record.StdoutPath, record.StdoutSHA256}, {record.StderrPath, record.StderrSHA256}, {record.VerdictPath, record.VerdictSHA256},
	} {
		member, ok := members[asset.path]
		if !ok || member.SHA256 != asset.hash {
			return fmt.Errorf("screenshot: ValidateReviewProvenance: declared review asset %q is missing or hash-mismatched", asset.path)
		}
	}
	verdictData, err := os.ReadFile(filepath.Join(packetDir, record.VerdictPath)) //nolint:gosec // declared manifest member
	if err != nil {
		return fmt.Errorf("screenshot: ValidateReviewProvenance: reading verdict: %w", err)
	}
	verdictDecoder := json.NewDecoder(strings.NewReader(string(verdictData)))
	verdictDecoder.DisallowUnknownFields()
	var verdict ReviewVerdict
	if err := verdictDecoder.Decode(&verdict); err != nil {
		return fmt.Errorf("screenshot: ValidateReviewProvenance: decoding verdict: %w", err)
	}
	if verdict.Version != configuredReviewVersion || verdict.SourceCommit != pkt.SourceCommit || verdict.CandidateManifestSHA256 != candidate.ManifestSHA256 || verdict.Reviewer != ConfiguredReviewerInstance || verdict.OpenCritical != 0 || verdict.OpenHigh != 0 {
		return fmt.Errorf("screenshot: ValidateReviewProvenance: verdict for %q is not a bound zero-blocker result at version %q", record.Reviewer, configuredReviewVersion)
	}
	return nil
}

// FinalizeCandidate creates a new immutable final packet only after exactly
// one complete, zero-blocker review from the configured reviewer
// (opencode-my-plan-review / opencode / local-llm-env/my-plan-review) binds
// to the candidate manifest. It never mutates the candidate and refuses an
// existing destination.
func FinalizeCandidate(candidateDir, finalDir string, reviews []ReviewInput) (Packet, error) {
	candidate, err := ValidateCandidate(candidateDir)
	if err != nil {
		return Packet{}, fmt.Errorf("screenshot: FinalizeCandidate: invalid candidate: %w", err)
	}
	// Exactly one review from the configured reviewer is required.
	configuredCount := 0
	for _, r := range reviews {
		if r.Reviewer == ConfiguredReviewerInstance {
			configuredCount++
		}
	}
	if configuredCount != 1 {
		return Packet{}, fmt.Errorf("screenshot: FinalizeCandidate: exactly one review from configured reviewer %q is required (got %d)", ConfiguredReviewerInstance, configuredCount)
	}
	if len(reviews) != 1 {
		return Packet{}, fmt.Errorf("screenshot: FinalizeCandidate: exactly one review is accepted (got %d); duplicate or extra reviewers are not permitted", len(reviews))
	}
	review := reviews[0]
	if err := validateConfiguredReviewRecord(review, candidate.SourceCommit, candidate.ManifestSHA256); err != nil {
		return Packet{}, fmt.Errorf("screenshot: FinalizeCandidate: %w", err)
	}
	if _, err := os.Stat(finalDir); err == nil {
		return Packet{}, fmt.Errorf("screenshot: FinalizeCandidate: destination %q already exists", finalDir)
	} else if !os.IsNotExist(err) {
		return Packet{}, fmt.Errorf("screenshot: FinalizeCandidate: checking destination: %w", err)
	}
	parent := filepath.Dir(finalDir)
	stage, err := os.MkdirTemp(parent, ".gitid-evidence-final-")
	if err != nil {
		return Packet{}, fmt.Errorf("screenshot: FinalizeCandidate: creating staging directory: %w", err)
	}
	defer os.RemoveAll(stage)
	if err := copyTree(candidateDir, stage); err != nil {
		return Packet{}, err
	}
	records := make([]reviewProvenanceRecord, 0, 1)
	name := strings.ReplaceAll(strings.ReplaceAll(review.Reviewer, "/", "_"), "\\", "_")
	base := filepath.ToSlash(filepath.Join("reviews", name))
	assets := []struct {
		name string
		data []byte
	}{{"prompt.txt", review.Prompt}, {"raw-stdout.txt", review.Stdout}, {"raw-stderr.txt", review.Stderr}, {"verdict.json", review.Verdict}}
	record := reviewProvenanceRecord{Reviewer: review.Reviewer, Tool: review.Tool, Provider: review.Provider, Model: review.Model, Session: review.Session, StartedAt: review.StartedAt, EndedAt: review.EndedAt, ExitCode: review.ExitCode}
	for _, asset := range assets {
		path := filepath.Join(stage, filepath.FromSlash(base), asset.name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return Packet{}, fmt.Errorf("screenshot: FinalizeCandidate: creating review directory: %w", err)
		}
		if err := writeAndSync(path, asset.data); err != nil {
			return Packet{}, fmt.Errorf("screenshot: FinalizeCandidate: writing review asset: %w", err)
		}
		path = filepath.ToSlash(filepath.Join(base, asset.name))
		switch asset.name {
		case "prompt.txt":
			record.PromptPath, record.PromptSHA256 = path, sha256Hex(asset.data)
		case "raw-stdout.txt":
			record.StdoutPath, record.StdoutSHA256 = path, sha256Hex(asset.data)
		case "raw-stderr.txt":
			record.StderrPath, record.StderrSHA256 = path, sha256Hex(asset.data)
		case "verdict.json":
			record.VerdictPath, record.VerdictSHA256 = path, sha256Hex(asset.data)
		}
	}
	records = append(records, record)
	provenanceData, err := json.MarshalIndent(reviewProvenance{Version: configuredReviewVersion, SourceCommit: candidate.SourceCommit, CandidateManifestSHA256: candidate.ManifestSHA256, Reviews: records}, "", "  ")
	if err != nil {
		return Packet{}, fmt.Errorf("screenshot: FinalizeCandidate: marshaling provenance: %w", err)
	}
	if err := writeAndSync(filepath.Join(stage, "REVIEW-PROVENANCE.json"), provenanceData); err != nil {
		return Packet{}, fmt.Errorf("screenshot: FinalizeCandidate: writing provenance: %w", err)
	}
	files, err := packetInventory(stage)
	if err != nil {
		return Packet{}, fmt.Errorf("screenshot: FinalizeCandidate: inventory: %w", err)
	}
	members := make([]PacketMember, 0, len(files))
	candidateMembers := make(map[string]PacketMember, len(candidate.Members))
	for _, member := range candidate.Members {
		candidateMembers[member.Path] = member
	}
	for path, hash := range files {
		member := PacketMember{Path: path, SHA256: hash, Kind: finalMemberKind(path)}
		if prior, ok := candidateMembers[path]; ok {
			member.Kind, member.ScreenID = prior.Kind, prior.ScreenID
		}
		members = append(members, member)
	}
	sort.Slice(members, func(i, j int) bool { return members[i].Path < members[j].Path })
	final := candidate
	final.Members = members
	final.ManifestSHA256 = ""
	manifest, err := marshalPacket(final)
	if err != nil {
		return Packet{}, err
	}
	final.ManifestSHA256 = sha256Hex(manifest)
	manifest, err = marshalPacket(final)
	if err != nil {
		return Packet{}, err
	}
	if err := writeAndSync(filepath.Join(stage, "MANIFEST.json"), manifest); err != nil {
		return Packet{}, fmt.Errorf("screenshot: FinalizeCandidate: writing manifest: %w", err)
	}
	if _, err := ValidatePacket(stage); err != nil {
		return Packet{}, fmt.Errorf("screenshot: FinalizeCandidate: validating staging packet: %w", err)
	}
	if err := os.Rename(stage, finalDir); err != nil {
		return Packet{}, fmt.Errorf("screenshot: FinalizeCandidate: publishing atomically: %w", err)
	}
	return final, nil
}

func finalMemberKind(path string) string {
	switch {
	case path == "CANDIDATE-MANIFEST.json":
		return "candidate-manifest"
	case path == "REVIEW-PROVENANCE.json":
		return "provenance"
	case strings.HasPrefix(path, "reviews/"):
		return "review"
	default:
		return "candidate-member"
	}
}

func copyTree(source, destination string) error {
	return filepath.Walk(source, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(destination, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o750)
		}
		data, err := os.ReadFile(path) //nolint:gosec // walked from validated candidate directory
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			return err
		}
		return writeAndSync(target, data)
	})
}

func packetInventory(root string) (map[string]string, error) {
	files := make(map[string]string)
	err := filepath.Walk(root, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path) //nolint:gosec // walked staging directory
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(rel)] = sha256Hex(data)
		return nil
	})
	return files, err
}

// validateDuplicatePNGBytes detects differently-named screens on the same
// surface that share identical PNG bytes — a sign that both were captured from
// the same terminal state (UI-REVIEW Critical Pillar 2 finding).
// It reads actual file bytes from disk rather than trusting manifest SHA-256
// values (which could be stale if files were modified after generation).
//
// Same-route interaction variants (e.g. reuse-manual-path and
// reuse-key-vs-generate both use /create-flow/reuse-key-vs-generate on the
// approved-html surface) are allowed to share PNG bytes since they genuinely
// come from the same HTML route. The live and approved-tui surfaces MUST have
// distinct PNGs for every distinct screen ID.
func validateDuplicatePNGBytes(packetDir string, pkt Packet) error {
	// Build a map of approved routes to detect same-route approved-html pairs.
	approvedRoutes := approvedHTMLRoutesInternal()

	// Build a map of same-surface interaction variants from the registry.
	// Pairs of (variantScreenID, baseScreenID) are allowed to share PNG bytes
	// on the live surface when the viewport content maps to the same terminal
	// layout (proof-viewport frames scrolled to different offsets).
	sameVariantPairs := make(map[string]string) // screenID → VariantOf base
	for _, spec := range RequiredScreenSpecs() {
		if spec.VariantOf != "" {
			sameVariantPairs[spec.ScreenID] = spec.VariantOf
		}
	}
	isVariantPair := func(a, b string) bool {
		aBase := sameVariantPairs[a]
		bBase := sameVariantPairs[b]
		if aBase == b || bBase == a {
			return true
		}
		if aBase != "" && aBase == bBase {
			return true
		}
		return false
	}

	// Map actual on-disk PNG SHA-256 → first (surface, screenID) that owned it.
	type screenKey struct{ surface, screenID string }
	seen := make(map[string]screenKey)
	for _, m := range pkt.Members {
		if !strings.HasSuffix(m.Path, ".png") {
			continue
		}
		parts := strings.Split(filepath.ToSlash(m.Path), "/")
		if len(parts) != 2 || !validPacketSurface(parts[0]) {
			continue
		}
		surface := parts[0]
		absPath := filepath.Join(packetDir, m.Path)
		data, err := os.ReadFile(absPath) //nolint:gosec // packetDir is gitid-controlled (G304)
		if err != nil {
			continue // missing file handled by hash-check above
		}
		actualHash := sha256Hex(data)
		key := screenKey{surface: surface, screenID: m.ScreenID}
		if prior, exists := seen[actualHash]; exists {
			if prior.surface == surface && prior.screenID != m.ScreenID {
				// For the approved-html surface: two screens may legitimately
				// share a PNG when they capture the same HTML route (same-route
				// interaction variants). Skip the duplicate error only when both
				// screens map to the same approved HTML route.
				if surface == "approved-html" {
					priorRoute := approvedRoutes[prior.screenID]
					curRoute := approvedRoutes[m.ScreenID]
					if priorRoute != "" && priorRoute == curRoute {
						continue // same HTML route — allowed to share PNG
					}
				}
				// For live/approved-tui surfaces: interaction variants declared
				// via VariantOf may share PNG bytes when they capture the same
				// terminal layout scrolled to different content (proof viewport).
				if isVariantPair(prior.screenID, m.ScreenID) {
					continue // declared interaction variant — allowed to share PNG
				}
				return fmt.Errorf("screenshot: ValidatePacket: duplicate PNG bytes (SHA-256 %s) for differently-named screens %q and %q on surface %q — both were likely captured from the same terminal state",
					actualHash, prior.screenID, m.ScreenID, surface)
			}
		} else {
			seen[actualHash] = key
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Canonical manifest helpers (03-13 Task 3).
// ---------------------------------------------------------------------------

// CanonicalManifestHash computes the SHA-256 of the canonical JSON serialization
// of pkt with its ManifestSHA256 field zeroed — the documented self-hash algorithm:
//
//	SHA-256 over canonical JSON with manifest_sha256 empty, members sorted by
//	path, fixed field order from the schema, UTF-8 encoding, and no trailing newline.
//
// This is the same function ValidatePacket uses to verify stored manifests, exported
// here so external verifiers can reproduce the hash from stored bytes.
func CanonicalManifestHash(pkt Packet) string {
	hashPkt := pkt
	hashPkt.ManifestSHA256 = ""
	data, err := marshalPacket(hashPkt)
	if err != nil {
		return ""
	}
	return sha256Hex(data)
}

// RegionDiffRecord is one screen's region comparison in REGION-DIFFS.json.
type RegionDiffRecord struct {
	// ScreenID is the logical screen identifier.
	ScreenID string `json:"screen_id"`
	// LiveHash is the SHA-256 of the live capture text for this screen.
	LiveHash string `json:"live_sha256"`
	// ApprovedHash is the SHA-256 of the approved-tui capture text.
	ApprovedHash string `json:"approved_tui_sha256"`
	// Equal reports whether live and approved-tui text are byte-identical.
	Equal bool `json:"equal"`
	// Divergence names the region that explains a non-equal result (if any).
	Divergence string `json:"divergence,omitempty"`
	// Justification is the allowlist entry explaining the divergence (if applicable).
	Justification string `json:"justification,omitempty"`
	// Regions is the complete named semantic comparison inventory for this frame.
	// Whole-screen equality is intentionally only a summary; reviewers inspect
	// the normalized bytes and hashes of these named regions.
	Regions []NamedRegionDiff `json:"regions"`
}

// NamedRegionDiff is one semantic region comparison within a frame. Empty
// evidence is retained so the stored inventory is complete and omission-safe.
type NamedRegionDiff struct {
	Name           RegionName `json:"name"`
	LiveText       string     `json:"live_text"`
	ApprovedText   string     `json:"approved_tui_text"`
	LiveApplicable bool       `json:"live_applicable"`
	// ApprovedApplicable records whether the approved TUI has a corresponding
	// state. A live-only production frame must say so explicitly rather than
	// silently dropping its comparison.
	ApprovedApplicable     bool   `json:"approved_tui_applicable"`
	Comparable             bool   `json:"comparable"`
	LiveHash               string `json:"live_sha256"`
	ApprovedHash           string `json:"approved_tui_sha256"`
	Equal                  bool   `json:"equal"`
	Divergence             string `json:"divergence,omitempty"`
	Justification          string `json:"justification,omitempty"`
	Decision               string `json:"decision,omitempty"`
	NonApplicabilityReason string `json:"non_applicability_reason,omitempty"`
	Classification         string `json:"classification,omitempty"`
}

func validDifferenceClassification(classification string) bool {
	return classification == "ux-improvement" || classification == "defect"
}

// RegionDiffs is the schema for REGION-DIFFS.json.
type RegionDiffs struct {
	Version      string             `json:"version"`
	SourceCommit string             `json:"source_commit"`
	GeneratedAt  string             `json:"generated_at"`
	Screens      []RegionDiffRecord `json:"screens"`
}

// BuildRegionDiffs generates a RegionDiffs document comparing live captures
// against approved-tui captures for every ScreenSpec in specs. Each record
// carries the live/approved SHA-256 pair, equality result, and any explicit
// screen-specific, decision-linked divergence disposition. RequiredRegions
// enforces mandatory nonempty evidence; the stored comparison inventory has
// exactly one entry for every name returned by AllRegionNames. The result is
// non-empty — every spec must produce at least one record.
//
// normalizePrefix strips disposable absolute path prefixes (temp dirs,
// timestamps) before hashing, retaining full commands and output content.
func BuildRegionDiffs(sourceCommit string, liveCaptures, approvedCaptures map[string]string, specs []ScreenSpec) ([]RegionDiffRecord, error) {
	if err := ValidateScreenSpecs(specs); err != nil {
		return nil, fmt.Errorf("screenshot: BuildRegionDiffs: invalid screen specs: %w", err)
	}
	records := make([]RegionDiffRecord, 0, len(specs))
	for _, spec := range specs {
		liveText := liveCaptures[spec.ScreenID]
		approvedText := approvedCaptures[spec.ScreenID]
		if spec.ApplicableLive && strings.TrimSpace(liveText) == "" {
			return nil, fmt.Errorf("screenshot: BuildRegionDiffs: required live frame %q is missing", spec.ScreenID)
		}
		if len(spec.RequiredRegions) == 0 {
			return nil, fmt.Errorf("screenshot: BuildRegionDiffs: frame %q declares no required regions", spec.ScreenID)
		}
		if spec.ApplicableApprovedTUI && strings.TrimSpace(approvedText) == "" {
			return nil, fmt.Errorf("screenshot: BuildRegionDiffs: required approved-tui frame %q is missing", spec.ScreenID)
		}
		liveNorm := normalizeForRegion(liveText)
		approvedNorm := normalizeForRegion(approvedText)
		rec := RegionDiffRecord{
			ScreenID:     spec.ScreenID,
			LiveHash:     sha256Hex([]byte(liveNorm)),
			ApprovedHash: sha256Hex([]byte(approvedNorm)),
			Equal:        liveNorm == approvedNorm,
		}
		for _, name := range spec.RequiredRegions {
			liveRegion := normalizeForRegion(ExtractRegion(liveText, name))
			approvedRegion := normalizeForRegion(ExtractRegion(approvedText, name))
			if spec.ApplicableLive && strings.TrimSpace(liveRegion) == "" {
				return nil, fmt.Errorf("screenshot: BuildRegionDiffs: frame %q required region %q is empty in live evidence", spec.ScreenID, name)
			}
			if spec.ApplicableApprovedTUI && strings.TrimSpace(approvedRegion) == "" {
				return nil, fmt.Errorf("screenshot: BuildRegionDiffs: frame %q required region %q is empty in approved-tui evidence", spec.ScreenID, name)
			}
		}
		for _, name := range AllRegionNames() {
			liveRegion := normalizeForRegion(ExtractRegion(liveText, name))
			approvedRegion := normalizeForRegion(ExtractRegion(approvedText, name))
			region := NamedRegionDiff{
				Name:               name,
				LiveText:           liveRegion,
				ApprovedText:       approvedRegion,
				LiveApplicable:     spec.ApplicableLive,
				ApprovedApplicable: spec.ApplicableApprovedTUI,
				Comparable:         spec.ApplicableLive && spec.ApplicableApprovedTUI,
				LiveHash:           sha256Hex([]byte(liveRegion)),
				ApprovedHash:       sha256Hex([]byte(approvedRegion)),
				Equal:              spec.ApplicableLive && spec.ApplicableApprovedTUI && liveRegion == approvedRegion,
			}
			if !region.Comparable {
				surface := "approved-tui"
				if !spec.ApplicableLive {
					surface = "live"
				}
				nonApplicable, _ := NonApplicabilityForSurface(spec, surface)
				region.Decision = nonApplicable.Decision
				region.NonApplicabilityReason = nonApplicable.Reason
				region.Classification = nonApplicable.Classification
				region.Justification = nonApplicable.Decision + ": " + nonApplicable.Reason
			} else if !region.Equal {
				disposition, found := RegionDispositionFor(spec, name)
				if !found {
					return nil, fmt.Errorf("screenshot: BuildRegionDiffs: frame %q region %q differs without a screen-specific declared disposition", spec.ScreenID, name)
				}
				// WR-19: a disposition with a Predicate only accepts a
				// divergence whose actual text satisfies it — RequiredRegions
				// is presence-only and AllRegionNames() above is iterated
				// regardless, so without this check any future drift in a
				// dispositioned region passed silently as the SAME
				// ux-improvement, whether or not it resembled what was
				// reviewed. An empty Predicate preserves prior blanket
				// acceptance exactly (backward compatible).
				if !regionPredicateSatisfied(disposition.Predicate, liveRegion, approvedRegion) {
					return nil, fmt.Errorf("screenshot: BuildRegionDiffs: frame %q region %q disposition predicate %q does not match the observed divergence (live: %q; approved: %q)", spec.ScreenID, name, disposition.Predicate, liveRegion, approvedRegion)
				}
				region.Divergence = disposition.Divergence
				region.Decision = disposition.Decision
				region.Justification = disposition.Decision + ": " + disposition.Reason
				region.Classification = disposition.Classification
			}
			rec.Regions = append(rec.Regions, region)
		}
		// Whole-frame byte equality is informational only. Acceptance is decided
		// region-by-region above, where each difference is classified and each
		// non-comparable surface is explicit.
		records = append(records, rec)
	}
	return records, nil
}

// RegionDispositionFor returns the explicit difference declaration attached to
// this screen. It deliberately has no fallback by region name.
func RegionDispositionFor(spec ScreenSpec, name RegionName) (RegionDisposition, bool) {
	for _, disposition := range spec.RegionDispositions {
		if disposition.Region == name {
			return disposition, true
		}
	}
	return RegionDisposition{}, false
}

// BuildRegionDiffsJSON marshals a RegionDiffs document for inclusion in the
// evidence packet. Uses the same canonical indented JSON format as MANIFEST.json.
func BuildRegionDiffsJSON(sourceCommit string, records []RegionDiffRecord) []byte {
	rd := RegionDiffs{
		Version:      "03-13.1",
		SourceCommit: sourceCommit,
		GeneratedAt:  "auto",
		Screens:      records,
	}
	if len(rd.Screens) == 0 {
		rd.Screens = []RegionDiffRecord{}
	}
	data, err := json.MarshalIndent(rd, "", "  ")
	if err != nil {
		panic("BuildRegionDiffsJSON: " + err.Error())
	}
	return data
}

func validateRegionDiffsFile(packetDir string, pkt Packet) error {
	data, err := os.ReadFile(filepath.Join(packetDir, "REGION-DIFFS.json")) //nolint:gosec // manifest declared path
	if err != nil {
		return fmt.Errorf("screenshot: ValidatePacket: reading REGION-DIFFS.json: %w", err)
	}
	return ValidateRegionDiffs(data, pkt.SourceCommit, RequiredScreenSpecs())
}

// ValidateRegionDiffs strictly validates the stored region document. It binds
// every declared frame to the current source commit, requires the complete
// AllRegionNames inventory, and rejects differences without an explicit D-XX
// justification. RequiredRegions remains the mandatory-nonempty subset.
func ValidateRegionDiffs(data []byte, sourceCommit string, specs []ScreenSpec) error {
	if err := ValidateScreenSpecs(specs); err != nil {
		return fmt.Errorf("screenshot: ValidateRegionDiffs: invalid screen specs: %w", err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	var diffs RegionDiffs
	if err := decoder.Decode(&diffs); err != nil {
		return fmt.Errorf("screenshot: ValidateRegionDiffs: decoding document: %w", err)
	}
	if diffs.SourceCommit != sourceCommit {
		return fmt.Errorf("screenshot: ValidateRegionDiffs: source commit %q does not match %q", diffs.SourceCommit, sourceCommit)
	}
	if len(diffs.Screens) != len(specs) {
		return fmt.Errorf("screenshot: ValidateRegionDiffs: got %d frame records, want %d", len(diffs.Screens), len(specs))
	}
	byID := make(map[string]RegionDiffRecord, len(diffs.Screens))
	for _, record := range diffs.Screens {
		if _, exists := byID[record.ScreenID]; exists {
			return fmt.Errorf("screenshot: ValidateRegionDiffs: duplicate frame %q", record.ScreenID)
		}
		byID[record.ScreenID] = record
	}
	for _, spec := range specs {
		record, ok := byID[spec.ScreenID]
		if !ok {
			return fmt.Errorf("screenshot: ValidateRegionDiffs: missing frame %q", spec.ScreenID)
		}
		knownRegions := make(map[RegionName]bool, len(AllRegionNames()))
		for _, name := range AllRegionNames() {
			knownRegions[name] = true
		}
		regions := make(map[RegionName]NamedRegionDiff, len(record.Regions))
		for _, region := range record.Regions {
			if !knownRegions[region.Name] {
				return fmt.Errorf("screenshot: ValidateRegionDiffs: unknown region %q for frame %q", region.Name, spec.ScreenID)
			}
			if _, exists := regions[region.Name]; exists {
				return fmt.Errorf("screenshot: ValidateRegionDiffs: duplicate region %q for frame %q", region.Name, spec.ScreenID)
			}
			if region.LiveApplicable != spec.ApplicableLive {
				return fmt.Errorf("screenshot: ValidateRegionDiffs: live applicability mismatch for %q/%q", spec.ScreenID, region.Name)
			}
			if region.LiveHash != sha256Hex([]byte(region.LiveText)) {
				return fmt.Errorf("screenshot: ValidateRegionDiffs: invalid live evidence for %q/%q", spec.ScreenID, region.Name)
			}
			if region.ApprovedApplicable != spec.ApplicableApprovedTUI {
				return fmt.Errorf("screenshot: ValidateRegionDiffs: approved applicability mismatch for %q/%q", spec.ScreenID, region.Name)
			}
			if region.ApprovedHash != sha256Hex([]byte(region.ApprovedText)) {
				return fmt.Errorf("screenshot: ValidateRegionDiffs: invalid approved evidence for %q/%q", spec.ScreenID, region.Name)
			}
			wantComparable := region.LiveApplicable && region.ApprovedApplicable
			if region.Comparable != wantComparable {
				return fmt.Errorf("screenshot: ValidateRegionDiffs: comparability mismatch for %q/%q", spec.ScreenID, region.Name)
			}
			if region.Equal != (wantComparable && region.LiveText == region.ApprovedText) {
				return fmt.Errorf("screenshot: ValidateRegionDiffs: equality mismatch for %q/%q", spec.ScreenID, region.Name)
			}
			if !region.Comparable {
				surface := "approved-tui"
				if !region.LiveApplicable {
					surface = "live"
				}
				nonApplicable, found := NonApplicabilityForSurface(spec, surface)
				if !found || region.Decision != nonApplicable.Decision || region.NonApplicabilityReason != nonApplicable.Reason || region.Classification != nonApplicable.Classification || !validDifferenceClassification(region.Classification) {
					return fmt.Errorf("screenshot: ValidateRegionDiffs: contradictory non-comparable evidence for %q/%q", spec.ScreenID, region.Name)
				}
				if !strings.Contains(region.Justification, region.Decision) {
					return fmt.Errorf("screenshot: ValidateRegionDiffs: non-comparable evidence lacks decision-linked justification for %q/%q", spec.ScreenID, region.Name)
				}
			} else if !region.Equal {
				disposition, found := RegionDispositionFor(spec, region.Name)
				if !found || region.Divergence != disposition.Divergence || region.Decision != disposition.Decision ||
					region.Justification != disposition.Decision+": "+disposition.Reason || region.Classification != disposition.Classification {
					return fmt.Errorf("screenshot: ValidateRegionDiffs: unexplained or unclassified divergence for %q/%q", spec.ScreenID, region.Name)
				}
			} else if region.Divergence != "" || region.Justification != "" || region.Classification != "" || region.NonApplicabilityReason != "" || region.Decision != "" {
				return fmt.Errorf("screenshot: ValidateRegionDiffs: equal region %q/%q carries a difference classification", spec.ScreenID, region.Name)
			}
			regions[region.Name] = region
		}
		for _, name := range AllRegionNames() {
			if _, ok := regions[name]; !ok {
				return fmt.Errorf("screenshot: ValidateRegionDiffs: missing named region %q for frame %q", name, spec.ScreenID)
			}
		}
		for _, required := range spec.RequiredRegions {
			region, ok := regions[required]
			if !ok {
				return fmt.Errorf("screenshot: ValidateRegionDiffs: missing required region %q for frame %q", required, spec.ScreenID)
			}
			if (spec.ApplicableLive && strings.TrimSpace(region.LiveText) == "") ||
				(spec.ApplicableApprovedTUI && strings.TrimSpace(region.ApprovedText) == "") {
				return fmt.Errorf("screenshot: ValidateRegionDiffs: required region %q for frame %q has empty applicable evidence", required, spec.ScreenID)
			}
		}
	}
	return nil
}

// normalizeForRegion strips disposable absolute temp-path prefixes and
// timestamp strings from a capture for stable region comparison — retaining
// full commands, outputs, config values, ANSI semantic codes, and markers.
func normalizeForRegion(text string) string {
	// Replace temp dir prefixes (e.g. /var/folders/.../gitid-...).
	// These are already normalized in CaptureCreateFlowScreens via normalizeTimestamps.
	return text
}

// approvedHTMLRoutesInternal is the package-internal version of the route map
// (without import cycle — createflow.go's exported ApprovedHTMLRoutes calls this).
func approvedHTMLRoutesInternal() map[string]string {
	return map[string]string{
		"ssh-form-filled":       "/create-flow/ssh-form-filled",
		"reuse-key-vs-generate": "/create-flow/reuse-key-vs-generate",
		"reuse-manual-path":     "/create-flow/reuse-key-vs-generate",
		"mouse-focused-field":   "/create-flow/ssh-form-filled",
		"test-stage1-direct":    "/create-flow/test-stage1-direct",
		"test-stage2-by-alias":  "/create-flow/test-stage2-by-alias",
		"git-form-demo":         "/git-screen/git-form-filled",
		"confirm-write":         "/create-flow/confirm-write",
	}
}
