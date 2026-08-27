package globalgit

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/castocolina/gitid/internal/deps"
)

// This file mirrors internal/globalssh/version.go in shape: the gate outcome
// enum, a numeric version comparison, and the one place a machine property may
// change something gitid WRITES. On the git side that substitution happens for
// exactly one row — merge.conflictstyle (D-08's only hard gate).

// VersionNotePrefix is the dynamic version line's prefix. It MUST stay out of
// the copy-freeze list because the rest of the line changes with the machine
// (the Phase 6 D-13 precedent).
const VersionNotePrefix = "Your git:"

// VersionNoteUnreadable is the frozen single line for an unreadable version.
const VersionNoteUnreadable = "git version could not be read — the fallback value will be written for version-gated rows"

// GateOutcome is how the machine's git version relates to a row's minimum.
type GateOutcome int

const (
	// GateMet means the parsed version is at or above the row minimum.
	GateMet GateOutcome = iota
	// GateBelow means the parsed version is below the row minimum.
	GateBelow
	// GateUnreadable means gitid could not establish the machine's git
	// version. For the HARD gate this is deliberately treated as below-gate:
	// the fallback value is accepted by every git that accepts the
	// recommendation, so the conservative direction only costs a better
	// conflict display while the optimistic direction could cost a failing
	// merge. This is the git-side analog of Phase 6 D-13's refusal to write
	// accept-new on an unverified OpenSSH, reached from the same reasoning.
	GateUnreadable
)

// VersionGate compares an already-read git version against p.MinVersion using
// numeric component comparison (never string ordering, so 2.47 is greater than
// 2.35), mirroring globalssh.VersionGate. An empty version is GateUnreadable
// with the frozen note. A row with no gate is always GateMet with no note.
func VersionGate(version string, p OptionPolicy) (GateOutcome, string) {
	if p.MinVersion == "" {
		return GateMet, ""
	}
	trimmed := strings.TrimSpace(version)
	if trimmed == "" {
		return GateUnreadable, VersionNoteUnreadable
	}
	gated := p.Key + " " + p.Recommended
	if versionLess(trimmed, p.MinVersion) {
		return GateBelow, fmt.Sprintf("%s %s — %s requires git %s+", VersionNotePrefix, trimmed, gated, p.MinVersion)
	}
	return GateMet, fmt.Sprintf("%s %s — %s is available", VersionNotePrefix, trimmed, gated)
}

// WriteValueFor is the ONE place that decides the VERSION-GATED written value.
// For a row with no gate and for a row with an informational gate it returns
// the recommendation unchanged — old git silently ignores an unknown KEY, so
// writing it is harmless no matter the machine's version. For the hard-gated
// row it returns the recommendation when the gate is met and the row's
// declared fallback value when it is not (below OR unreadable), because old
// git ERRORS at merge time on an unknown VALUE.
//
// key is a MEMBER config key of row, matched case-insensitively. Unrecognised
// keys return "" — the call site iterates the row's own members, so a silent
// empty write here is a bug the composer's tests catch, not a feature.
func WriteValueFor(row OptionPolicy, key string, gate GateOutcome) string {
	for _, member := range row.Members {
		if !strings.EqualFold(member.Key, key) {
			continue
		}
		if row.Gate == GateHard && gate != GateMet {
			return row.Fallback
		}
		return member.Recommended
	}
	return ""
}

// RealGateForRow resolves the machine's git version and returns the gate
// outcome for p. The version comes from deps.GitVersion — the single
// git-version probe in the module, the same source deps.GitVersionAtLeast
// already reads (D-08 mandates reusing that probe, not adding a second one).
// An unreadable version is GateUnreadable (see GateUnreadable's doc comment
// for why the hard gate may never treat it as met).
func RealGateForRow(p OptionPolicy) (GateOutcome, string) {
	v, err := deps.GitVersion()
	if err != nil {
		return GateUnreadable, VersionNoteUnreadable
	}
	return VersionGate(v, p)
}

func versionLess(got, minimum string) bool {
	gMaj, gMin := versionParts(got)
	mMaj, mMin := versionParts(minimum)
	if gMaj != mMaj {
		return gMaj < mMaj
	}
	return gMin < mMin
}

func versionParts(v string) (major, minor int) {
	trimmed := v
	for _, p := range []string{"p", "P", ".g"} {
		if i := strings.Index(trimmed, p); i >= 0 && i > 0 {
			trimmed = trimmed[:i]
			break
		}
	}
	parts := strings.Split(trimmed, ".")
	if len(parts) > 0 {
		major, _ = strconv.Atoi(parts[0])
	}
	if len(parts) > 1 {
		minor, _ = strconv.Atoi(parts[1])
	}
	return major, minor
}
