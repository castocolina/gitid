package tui

import (
	"testing"

	"github.com/castocolina/gitid/internal/doctor"
)

// TestSeverityGlyphWarningDistinct asserts that warning uses "!" (D-10, Eval #2)
// and is NOT "✗" (which is reserved for error/critical). This is the
// hard behavioral requirement: warning and error must be visually distinct
// glyphs — "!" vs "✗" — so users can distinguish advisory from actionable severity.
func TestSeverityGlyphWarningDistinct(t *testing.T) {
	warningGlyph := SeverityGlyph(doctor.SeverityWarning, false)
	errorGlyph := SeverityGlyph(doctor.SeverityError, false)

	if warningGlyph == "✗" {
		t.Errorf("SeverityGlyph(Warning) returned %q; must NOT be %q (D-10/Eval#2: warning is '!', error is '✗')",
			warningGlyph, "✗")
	}
	if warningGlyph != "!" {
		t.Errorf("SeverityGlyph(Warning) = %q; want %q (D-10)", warningGlyph, "!")
	}
	if errorGlyph != "✗" {
		t.Errorf("SeverityGlyph(Error) = %q; want %q (D-10)", errorGlyph, "✗")
	}
	// The two must be distinct.
	if warningGlyph == errorGlyph {
		t.Errorf("warning and error glyphs must be distinct; both returned %q", warningGlyph)
	}
}

// TestSeverityGlyphAllLevels verifies all four severity levels return the
// expected glyphs (UTF-8 mode).
func TestSeverityGlyphAllLevels(t *testing.T) {
	tests := []struct {
		severity doctor.Severity
		want     string
	}{
		{doctor.SeverityCritical, "✗"},
		{doctor.SeverityError, "✗"},
		{doctor.SeverityWarning, "!"},
		{doctor.SeverityInfo, "~"},
	}
	for _, tt := range tests {
		got := SeverityGlyph(tt.severity, false)
		if got != tt.want {
			t.Errorf("SeverityGlyph(%v, false) = %q; want %q", tt.severity, got, tt.want)
		}
	}
}

// TestSeverityGlyphASCIIFallbacks verifies the ASCII fallbacks for degraded terminals.
func TestSeverityGlyphASCIIFallbacks(t *testing.T) {
	tests := []struct {
		severity doctor.Severity
		want     string
	}{
		{doctor.SeverityCritical, "FAIL"},
		{doctor.SeverityError, "FAIL"},
		{doctor.SeverityWarning, "!"},
		{doctor.SeverityInfo, "i"},
	}
	for _, tt := range tests {
		got := SeverityGlyph(tt.severity, true)
		if got != tt.want {
			t.Errorf("SeverityGlyph(%v, ascii=true) = %q; want %q", tt.severity, got, tt.want)
		}
	}
}
