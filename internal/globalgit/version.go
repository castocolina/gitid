package globalgit

import (
	"fmt"
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

// WriteRequestedValueFor resolves a staged override value for one config key
// on a given row, respecting gates and type validation. It is the value
// resolver WriteValueFor delegates into; WriteValueFor keeps its existing
// signature and call sites unchanged.
//
// For a toggle row, it returns the requested value if it is "true" or "false"
// (case-insensitive), else empty. For an enum row, it returns the requested
// value if it is in the row's declared value set, else empty. For a text row,
// it returns the requested value after passing it through the validator for
// that row (which may reject it). For a bundle row, it always returns the
// row's declared fallback: bundles are edit-ineligible, so a staged override
// targeting a bundle is silently converted to the fallback by the overlay
// builder's bundle refusal gate (PD31), meaning this function is never called
// for a bundle in normal flow.
//
// The gate parameter follows the gate-meeting logic of WriteValueFor: for a
// hard-gated row that did not meet its gate, this function returns the row's
// declared fallback regardless of the requested value, exactly like WriteValueFor
// does. For rows with no gate or an informational gate, the gate outcome is
// not consulted.
func WriteRequestedValueFor(row OptionPolicy, key string, requested string, gate GateOutcome) string {
	// Find the member for this key
	var member *MemberPolicy
	for i := range row.Members {
		if strings.EqualFold(row.Members[i].Key, key) {
			member = &row.Members[i]
			break
		}
	}
	if member == nil {
		return "" // unknown key, let caller handle the refusal
	}

	// Hard-gated rows below their gate always return the fallback,
	// regardless of what was requested (matching WriteValueFor's behavior).
	if row.Gate == GateHard && gate != GateMet {
		return row.Fallback
	}

	// Bundle rows always resolve to their fallback. This should not be
	// reached in normal flow because the overlay builder refuses bundle
	// overrides before calling this function, but we implement it for
	// completeness and safety.
	if row.Kind == OptionValueKindBundle {
		return row.Fallback
	}

	// For toggle rows, validate that the requested value is a boolean string.
	if row.Kind == OptionValueKindToggle {
		lower := strings.ToLower(strings.TrimSpace(requested))
		if lower == "true" || lower == "false" {
			return lower
		}
		return "" // invalid boolean, rejected by value set membership test
	}

	// For enum rows, check membership in the declared value set.
	if row.Kind == OptionValueKindEnum {
		for _, val := range row.Values {
			if val == requested {
				return requested
			}
		}
		return "" // not a member of the value set, refused
	}

	// For text rows, apply the row's validator if one exists.
	// If no validator, accept the value as-is.
	if row.Kind == OptionValueKindText {
		if row.Validator != nil {
			if err := row.Validator(requested); err != nil {
				return "" // validation failed
			}
		}
		return requested
	}

	// Fallback: unknown kind, return empty
	return ""
}

// versionLess compares two git version strings by major.minor, delegating
// to deps.GitVersionParts — the ONE git-version-string parser in the module
// (code review finding: this file previously carried its own divergent
// parser, copied from globalssh's OpenSSH version parser without adapting
// its "p"/"P" suffix-stripping, which is an OpenSSH convention ("9.6p1")
// that git version strings never use — dead weight for this domain, and a
// real risk that GitVersionAtLeast and this gate could disagree about the
// SAME machine on an unusual version token).
func versionLess(got, minimum string) bool {
	gMaj, gMin := deps.GitVersionParts(got)
	mMaj, mMin := deps.GitVersionParts(minimum)
	if gMaj != mMaj {
		return gMaj < mMaj
	}
	return gMin < mMin
}
