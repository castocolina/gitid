package globalgit

import (
	"strings"
	"testing"
)

func conflictstyleRow(t *testing.T) OptionPolicy {
	t.Helper()
	p, ok := PolicyFor("merge.conflictstyle")
	if !ok {
		t.Fatal("merge.conflictstyle policy missing")
	}
	return p
}

// TestWriteValueForNoGateReturnsRecommendation asserts rows without a gate
// return the member recommendation unchanged whatever the gate outcome.
func TestWriteValueForNoGateReturnsRecommendation(t *testing.T) {
	row, _ := PolicyFor("core.ignorecase")
	for _, gate := range []GateOutcome{GateMet, GateBelow, GateUnreadable} {
		if got := WriteValueFor(row, "core.ignorecase", gate); got != "false" {
			t.Errorf("WriteValueFor(no-gate, %v) = %q, want false", gate, got)
		}
	}
}

// TestWriteValueForInformationalGateNotMetReturnsRecommendation asserts an
// informational-gated row's value is unchanged even when the gate is NOT met —
// old git ignores an unknown KEY, so writing is harmless (D-08).
func TestWriteValueForInformationalGateNotMetReturnsRecommendation(t *testing.T) {
	row, _ := PolicyFor("init.defaultBranch")
	for _, gate := range []GateOutcome{GateBelow, GateUnreadable} {
		if got := WriteValueFor(row, "init.defaultBranch", gate); got != "main" {
			t.Errorf("WriteValueFor(informational, %v) = %q, want main", gate, got)
		}
	}
}

// TestWriteValueForHardGateMetReturnsRecommendation asserts the hard-gated row
// writes zdiff3 when the gate IS met.
func TestWriteValueForHardGateMetReturnsRecommendation(t *testing.T) {
	row := conflictstyleRow(t)
	if got := WriteValueFor(row, "merge.conflictstyle", GateMet); got != "zdiff3" {
		t.Errorf("WriteValueFor(hard, met) = %q, want zdiff3", got)
	}
}

// TestWriteValueForHardGateNotMetReturnsFallback asserts the hard-gated row
// writes the declared fallback when the gate is NOT met.
func TestWriteValueForHardGateNotMetReturnsFallback(t *testing.T) {
	row := conflictstyleRow(t)
	for _, gate := range []GateOutcome{GateBelow, GateUnreadable} {
		if got := WriteValueFor(row, "merge.conflictstyle", gate); got != "diff3" {
			t.Errorf("WriteValueFor(hard, %v) = %q, want diff3", gate, got)
		}
	}
}

// TestWriteValueForUnknownMemberKey returns the empty sentinel for a member
// key no row manages.
func TestWriteValueForUnknownMemberKey(t *testing.T) {
	row := conflictstyleRow(t)
	if got := WriteValueFor(row, "alias.st", GateMet); got != "" {
		t.Errorf("WriteValueFor(unknown member) = %q, want empty sentinel", got)
	}
}

// TestWriteValueForBundleReturnsEachMember asserts a bundle row resolves each
// of its member keys to that member's recommendation.
func TestWriteValueForBundleReturnsEachMember(t *testing.T) {
	row, _ := PolicyFor("color (ui/branch/diff/status)")
	for _, m := range row.Members {
		if got := WriteValueFor(row, m.Key, GateMet); got != "auto" {
			t.Errorf("WriteValueFor(%s) = %q, want auto", m.Key, got)
		}
	}
}

func TestVersionGateNumericCompare(t *testing.T) {
	p := conflictstyleRow(t)
	cases := []struct {
		name    string
		version string
		want    GateOutcome
	}{
		{"below", "2.34", GateBelow},
		{"equal", "2.35", GateMet},
		{"above", "2.47.1", GateMet},
		{"two-digit minor beats one-digit", "2.40", GateMet},
		{"suffix after patch", "2.35.8 (Apple Git-154)", GateMet},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outcome, note := VersionGate(tc.version, p)
			if outcome != tc.want {
				t.Fatalf("VersionGate(%q) outcome = %v, want %v", tc.version, outcome, tc.want)
			}
			if note == "" {
				t.Fatal("version note must be non-empty when a minimum is set")
			}
		})
	}
}

// TestVersionGateUnreadable pins the unreadable handling: empty version →
// GateUnreadable with the frozen note, and the hard gate MUST NOT treat it as
// met (D-08: old git errors on the unknown zdiff3 VALUE at merge time).
func TestVersionGateUnreadable(t *testing.T) {
	p := conflictstyleRow(t)
	outcome, note := VersionGate("", p)
	if outcome != GateUnreadable {
		t.Fatalf("empty version = %v, want GateUnreadable", outcome)
	}
	if !strings.Contains(note, VersionNoteUnreadable) {
		t.Fatalf("unreadable note = %q, want the frozen VersionNoteUnreadable", note)
	}
}

// TestVersionGateNoMinimum returns GateMet with an empty note when there is no
// gate.
func TestVersionGateNoMinimum(t *testing.T) {
	row, _ := PolicyFor("pull.rebase")
	outcome, note := VersionGate("anything", row)
	if outcome != GateMet || note != "" {
		t.Fatalf("no-minimum VersionGate() = (%v, %q), want available with empty note", outcome, note)
	}
}

// TestVersionGateNamesTheGatedOptionNotAHardcodedLiteral is the WR-09
// regression applied to the git side: the note must name THAT row's
// key/recommended value, never a hardcoded zdiff3 literal.
func TestVersionGateNamesTheGatedOptionNotAHardcodedLiteral(t *testing.T) {
	synthetic := OptionPolicy{
		Key:         "init.defaultBranch",
		Recommended: "main",
		MinVersion:  "9.9",
		Gate:        GateInformational,
	}
	_, note := VersionGate("2.28", synthetic)
	if !strings.Contains(note, "init.defaultBranch main") {
		t.Errorf("note = %q, want it to name the gated option/value, not a hardcoded literal", note)
	}
	if strings.Contains(note, "zdiff3") {
		t.Errorf("note = %q, WR-09 regressed: hardcoded zdiff3 literal appeared for a different row", note)
	}
}
