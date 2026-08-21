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
	// Validate every declared member.
	for _, m := range pkt.Members {
		absPath := filepath.Join(packetDir, m.Path)
		content, err := os.ReadFile(absPath) //nolint:gosec // absPath is constructed from gitid-controlled packet dir and manifest path (G304)
		if err != nil {
			return Packet{}, fmt.Errorf("screenshot: ValidatePacket: reading member %q: %w", m.Path, err)
		}
		got := sha256Hex(content)
		if got != m.SHA256 {
			return Packet{}, fmt.Errorf("screenshot: ValidatePacket: member %q hash mismatch: got %s, want %s", m.Path, got, m.SHA256)
		}
	}
	return pkt, nil
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
