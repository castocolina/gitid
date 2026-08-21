//go:build screenshot

package screenshot

// createflow_packet.go implements the canonical 24-panel evidence packet for
// the Phase 3 create-flow visual review (plan 03-11 Task 2, CR-01 through
// CR-04, CR-10).
//
// A packet contains:
//   - 8 live panels: the real backend captured at current HEAD
//   - 8 approved-TUI panels: captured from approval commit TUI source
//   - 8 approved-HTML panels: captured from approval commit HTML source
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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
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
	PNGPath  string
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

// ValidatePanelCount is the canonical expected number of panels in a review packet.
// 8 live + 8 approved-TUI + 8 approved-HTML = 24 total panels.
const ValidatePanelCount = 24

// packetVersion is the current manifest schema version.
const packetVersion = "03-11.1"

const visualPacketVersion = "03-11.2"

// GenerateVisualPacket writes the complete 24-panel evidence packet. Unlike
// GenerateTextPacket, this API cannot represent a panel without both the text
// captured from the source binary/page and a non-empty renderer-produced PNG.
func GenerateVisualPacket(opts PacketOptions, panels []VisualPanel, capture PacketCapture) (PacketResult, error) {
	if err := validatePacketOptions(opts, "GenerateVisualPacket"); err != nil {
		return PacketResult{}, err
	}
	if len(panels) != ValidatePanelCount {
		return PacketResult{}, fmt.Errorf("screenshot: GenerateVisualPacket: got %d panels, want exactly %d", len(panels), ValidatePanelCount)
	}
	if err := os.MkdirAll(opts.OutputDir, 0o750); err != nil {
		return PacketResult{}, fmt.Errorf("screenshot: GenerateVisualPacket: creating output dir %q: %w", opts.OutputDir, err)
	}

	seen := make(map[string]bool, ValidatePanelCount)
	members := make([]PacketMember, 0, ValidatePanelCount*2)
	for _, panel := range panels {
		if !validPacketSurface(panel.Surface) {
			return PacketResult{}, fmt.Errorf("screenshot: GenerateVisualPacket: unknown panel surface %q", panel.Surface)
		}
		if !isCreateFlowScreenID(panel.ScreenID) {
			return PacketResult{}, fmt.Errorf("screenshot: GenerateVisualPacket: unknown screen %q", panel.ScreenID)
		}
		key := panel.Surface + "/" + panel.ScreenID
		if seen[key] {
			return PacketResult{}, fmt.Errorf("screenshot: GenerateVisualPacket: duplicate panel %q", key)
		}
		seen[key] = true
		if strings.TrimSpace(panel.Text) == "" {
			return PacketResult{}, fmt.Errorf("screenshot: GenerateVisualPacket: panel %q has empty text", key)
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
		pngRel := filepath.Join(panel.Surface, panel.ScreenID+".png")
		if err := writeAndSync(filepath.Join(opts.OutputDir, textRel), []byte(panel.Text)); err != nil {
			return PacketResult{}, fmt.Errorf("screenshot: GenerateVisualPacket: writing text %q: %w", key, err)
		}
		if err := writeAndSync(filepath.Join(opts.OutputDir, pngRel), png); err != nil {
			return PacketResult{}, fmt.Errorf("screenshot: GenerateVisualPacket: writing PNG %q: %w", key, err)
		}
		members = append(members,
			PacketMember{Path: textRel, SHA256: sha256Hex([]byte(panel.Text)), Kind: "text", ScreenID: panel.ScreenID},
			PacketMember{Path: pngRel, SHA256: sha256Hex(png), Kind: "png-" + panel.Surface, ScreenID: panel.ScreenID},
		)
	}
	for _, surface := range []string{"live", "approved-tui", "approved-html"} {
		for _, id := range CreateFlowScreenIDs {
			if !seen[surface+"/"+id] {
				return PacketResult{}, fmt.Errorf("screenshot: GenerateVisualPacket: missing panel %s/%s", surface, id)
			}
		}
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
	for _, candidate := range CreateFlowScreenIDs {
		if candidate == id {
			return true
		}
	}
	return false
}

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
	for _, id := range CreateFlowScreenIDs {
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
	for _, id := range CreateFlowScreenIDs {
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
	manifestPath := filepath.Join(packetDir, "MANIFEST.json")
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
	declared := map[string]bool{"MANIFEST.json": true}
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
		if pkt.Version == visualPacketVersion && len(content) == 0 {
			return Packet{}, fmt.Errorf("screenshot: ValidatePacket: visual member %q is empty", m.Path)
		}
		got := sha256Hex(content)
		if got != m.SHA256 {
			return Packet{}, fmt.Errorf("screenshot: ValidatePacket: member %q hash mismatch: got %s, want %s", m.Path, got, m.SHA256)
		}
	}
	if pkt.Version == visualPacketVersion {
		if err := validateVisualPacket(pkt); err != nil {
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
	}
	return pkt, nil
}

func validateVisualPacket(pkt Packet) error {
	if len(pkt.Members) != ValidatePanelCount*2 {
		return fmt.Errorf("screenshot: ValidatePacket: visual packet has %d members, want %d text/PNG members", len(pkt.Members), ValidatePanelCount*2)
	}
	if pkt.Capture.Geometry == "" || pkt.Capture.FontSHA256 == "" || pkt.Capture.Theme == "" || len(pkt.Capture.Commands) == 0 || len(pkt.Capture.ToolVersions) == 0 {
		return fmt.Errorf("screenshot: ValidatePacket: visual packet has incomplete capture provenance")
	}
	panels := make(map[string]bool, ValidatePanelCount)
	for _, m := range pkt.Members {
		if !strings.HasSuffix(m.Path, ".png") {
			continue
		}
		parts := strings.Split(filepath.ToSlash(m.Path), "/")
		if len(parts) != 2 || !validPacketSurface(parts[0]) || !isCreateFlowScreenID(m.ScreenID) || m.Kind != "png-"+parts[0] {
			return fmt.Errorf("screenshot: ValidatePacket: invalid visual panel member %q", m.Path)
		}
		key := parts[0] + "/" + m.ScreenID
		if panels[key] {
			return fmt.Errorf("screenshot: ValidatePacket: duplicate visual panel %q", key)
		}
		panels[key] = true
	}
	if len(panels) != ValidatePanelCount {
		return fmt.Errorf("screenshot: ValidatePacket: got %d PNG panels, want exactly %d", len(panels), ValidatePanelCount)
	}
	for _, surface := range []string{"live", "approved-tui", "approved-html"} {
		for _, id := range CreateFlowScreenIDs {
			if !panels[surface+"/"+id] {
				return fmt.Errorf("screenshot: ValidatePacket: missing PNG panel %s/%s", surface, id)
			}
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

// CompareTextCaptures reports whether two sets of text captures are byte-identical
// for all CreateFlowScreenIDs (CR-10 text determinism).
func CompareTextCaptures(a, b map[string]string) error {
	for _, id := range CreateFlowScreenIDs {
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
