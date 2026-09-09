package globalgit_test

import (
	"testing"

	"github.com/castocolina/gitid/internal/globalgit"
)

// TestWriteRequestedValueForToggle tests toggle-kind rows.
func TestWriteRequestedValueForToggle(t *testing.T) {
	tests := []struct {
		name      string
		requested string
		want      string
	}{
		{"lowercase true", "true", "true"},
		{"uppercase TRUE", "TRUE", "true"},
		{"lowercase false", "false", "false"},
		{"uppercase FALSE", "FALSE", "false"},
		{"mixed case True", "True", "true"},
		{"invalid value", "maybe", ""},
		{"empty string", "", ""},
		{"with spaces", "  true  ", "true"},
	}

	row := globalgit.OptionPolicy{
		Kind: globalgit.OptionValueKindToggle,
		Members: []globalgit.MemberPolicy{
			{Key: "test.key", Recommended: "true"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := globalgit.WriteRequestedValueFor(row, "test.key", tt.requested, globalgit.GateMet)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestWriteRequestedValueForEnum tests enum-kind rows.
func TestWriteRequestedValueForEnum(t *testing.T) {
	row := globalgit.OptionPolicy{
		Kind:   globalgit.OptionValueKindEnum,
		Values: []string{"merge", "diff3", "zdiff3"},
		Members: []globalgit.MemberPolicy{
			{Key: "merge.conflictstyle", Recommended: "diff3"},
		},
	}

	tests := []struct {
		name      string
		requested string
		want      string
	}{
		{"valid value 1", "merge", "merge"},
		{"valid value 2", "diff3", "diff3"},
		{"valid value 3", "zdiff3", "zdiff3"},
		{"invalid value", "other", ""},
		{"case sensitive", "DIFF3", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := globalgit.WriteRequestedValueFor(row, "merge.conflictstyle", tt.requested, globalgit.GateMet)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestWriteRequestedValueForText tests text-kind rows.
func TestWriteRequestedValueForText(t *testing.T) {
	row := globalgit.OptionPolicy{
		Kind: globalgit.OptionValueKindText,
		Members: []globalgit.MemberPolicy{
			{Key: "init.defaultBranch", Recommended: "main"},
		},
	}

	tests := []struct {
		name      string
		requested string
		want      string
	}{
		{"valid branch main", "main", "main"},
		{"valid branch develop", "develop", "develop"},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := globalgit.WriteRequestedValueFor(row, "init.defaultBranch", tt.requested, globalgit.GateMet)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestWriteRequestedValueForHardGateBelow tests hard-gate fallback behavior.
func TestWriteRequestedValueForHardGateBelow(t *testing.T) {
	row := globalgit.OptionPolicy{
		Gate:       globalgit.GateHard,
		Kind:       globalgit.OptionValueKindEnum,
		Fallback:   "diff3",
		Values:     []string{"merge", "diff3", "zdiff3"},
		Members: []globalgit.MemberPolicy{
			{Key: "merge.conflictstyle", Recommended: "zdiff3"},
		},
	}

	// Below gate: should return fallback regardless of requested value
	got := globalgit.WriteRequestedValueFor(row, "merge.conflictstyle", "zdiff3", globalgit.GateBelow)
	if got != "diff3" {
		t.Errorf("got %q, want fallback 'diff3'", got)
	}

	// Gate met: should return requested value
	got = globalgit.WriteRequestedValueFor(row, "merge.conflictstyle", "zdiff3", globalgit.GateMet)
	if got != "zdiff3" {
		t.Errorf("got %q, want requested 'zdiff3'", got)
	}
}

// TestWriteRequestedValueForUnknownKey tests unknown key handling.
func TestWriteRequestedValueForUnknownKey(t *testing.T) {
	row := globalgit.OptionPolicy{
		Kind: globalgit.OptionValueKindText,
		Members: []globalgit.MemberPolicy{
			{Key: "init.defaultBranch", Recommended: "main"},
		},
	}

	got := globalgit.WriteRequestedValueFor(row, "unknown.key", "value", globalgit.GateMet)
	if got != "" {
		t.Errorf("got %q, want empty string for unknown key", got)
	}
}
