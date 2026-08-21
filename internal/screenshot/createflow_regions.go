//go:build screenshot

package screenshot

// createflow_regions.go defines named screen regions for the region-scoped
// visual-regression gate (plan 03-09, Task 1, CR-10 fix).
//
// Motivation: the original gate (03-06 Task 2) exempted every screen at
// whole-screen granularity — meaning an unrelated change to the header,
// breadcrumb, form field labels, wizard stepper, keybar, or any other
// non-divergent region would pass silently. CR-10 requires that ONLY the
// specific documented sub-region (e.g. the Host-block preview box, the
// connectivity output line) may differ; all remaining semantic regions must
// be byte-identical.
//
// Region extraction is pure string-scanning over the rendered View().Content
// output: no ANSI stripping (regions must match the styled output), no
// external tools.

import (
	"strings"
)

// RegionName identifies a named sub-region of a create-flow wizard screen.
// Each region corresponds to a structurally distinct area the visual gate
// can compare independently.
type RegionName string

const (
	// RegionHeader is the first rendered line — the nav-tab bar at the top
	// of the shell frame (" gitid  [1] Identities · [2] Global SSH · …").
	RegionHeader RegionName = "header"

	// RegionBreadcrumb is the second rendered line — the path inside the
	// active pane ("Identities › New identity › SSH details").
	RegionBreadcrumb RegionName = "breadcrumb"

	// RegionWizardStepper is the "Step N/4 · SSH details ● ○ ○ ○" line plus
	// the chord-hint line immediately below it. Both are part of the wizard
	// chrome and must be byte-identical between real and dummy.
	RegionWizardStepper RegionName = "wizard-stepper"

	// RegionFormFields is the pane-right body section that contains the four
	// approved SSH form fields (Alias prefix, SSH Host, Real hostname, Port)
	// and the Key row. It spans from the first "│  " field row through the
	// last "│" field row, before the Host-block preview box begins.
	RegionFormFields RegionName = "form-fields"

	// RegionHostPreview is the Host-block preview box bounded by the dashed
	// border lines ("╭╌…╮" … "╰╌…╯"). This is the region whose indent
	// format differs between the real backend (2-space + provider marker)
	// and the dummy (4-space markerless) — documented as T-03-HOSTBLOCK.
	RegionHostPreview RegionName = "host-preview"

	// RegionConnectivityOutput is the test-outcome section present on
	// test-stage1-direct and test-stage2-by-alias screens — the lines
	// showing the exact command run and its output. This is the D-02
	// divergence region (yellow warning vs. success/failure glyph text).
	RegionConnectivityOutput RegionName = "connectivity-output"

	// RegionContinueDisabledReason is the line immediately below the
	// "[ Continue ]" button that carries the disabled reason. On git-form-demo
	// this is the D-19 divergence: real binary shows "— Git configuration
	// arrives with the next build"; dummy shows "— needs user.name + a valid email".
	RegionContinueDisabledReason RegionName = "continue-disabled-reason"

	// RegionKeybar is the last 2–3 non-empty lines — the keybar/footer area
	// below the form that shows Tab/↑↓/Enter/Esc affordances.
	RegionKeybar RegionName = "keybar"

	// RegionReusePickerEntries is the Key picker candidate list (the rows
	// between "● Generate a new key   ○ Reuse an existing key" toggle and
	// the Host-block preview box). On reuse-key-vs-generate, this contains
	// dynamically-scanned key entries that differ between backends.
	RegionReusePickerEntries RegionName = "reuse-picker-entries"

	// RegionKeySection is the Key row and its sub-rows (the generate/reuse
	// toggle and the algorithm catalog or reuse picker entries). This region
	// differs between real and dummy in two known ways:
	//   1. Algorithm catalog ordering (real = live-probed; dummy = fixture order)
	//   2. Reuse picker entries (real = scanned temp-HOME keys; dummy = fixture list)
	// The form labels above the Key row (Alias prefix/SSH Host/Real hostname/Port)
	// are covered by RegionFormFields which stops before the Key row.
	RegionKeySection RegionName = "key-section"

	// RegionHeaderStatus is the right portion of the header line (line 1) that
	// shows the identity count and health summary ("0 ids · ✓ ok" for the real
	// backend; "8 ids · ! 1 ✗ 3" for the pre-seeded dummy). The nav-tabs
	// portion of the header is stable and covered by RegionHeader's ANSI-stripped
	// nav-tabs comparison (always identical). This region is allowlisted with the
	// same T-03-SIDEBAR justification as the sidebar entries.
	RegionHeaderStatus RegionName = "header-status"

	// RegionSidebar is the pane-left area (to the left of the "│" separator)
	// showing the identity list, identity count, and health summary. The real
	// backend starts with 0 identities; the dummy's FixtureBackend carries 8
	// pre-seeded fixture identities — the sidebar content is legitimately
	// different by construction and is not a design divergence.
	// NOTE: this divergence is NOT one of the documented D-02/D-16/D-19
	// design decisions; it is an inherent structural difference between a
	// fresh-HOME real backend and a pre-seeded fixture backend. It is
	// allowlisted as "sidebar-state" with an explanation.
	RegionSidebar RegionName = "sidebar"
)

// ExtractRegion returns the sub-string of screen that corresponds to region.
// All regions are extracted by structural markers in the rendered TUI text —
// no ANSI stripping is performed so the extracted text includes color codes
// exactly as rendered. If the region cannot be located, an empty string is
// returned.
func ExtractRegion(screen string, region RegionName) string {
	lines := splitLines(screen)
	switch region {
	case RegionHeader:
		return extractHeader(lines)
	case RegionBreadcrumb:
		return extractBreadcrumb(lines)
	case RegionWizardStepper:
		return extractWizardStepper(lines)
	case RegionFormFields:
		return extractFormFields(lines)
	case RegionHostPreview:
		return extractHostPreview(lines)
	case RegionConnectivityOutput:
		return extractConnectivityOutput(lines)
	case RegionContinueDisabledReason:
		return extractContinueDisabledReason(lines)
	case RegionKeybar:
		return extractKeybar(lines)
	case RegionReusePickerEntries:
		return extractReusePickerEntries(lines)
	case RegionSidebar:
		return extractSidebar(lines)
	case RegionHeaderStatus:
		return extractHeaderStatus(lines)
	case RegionKeySection:
		return extractKeySection(lines)
	}
	return ""
}

// extractHeader returns the nav-tabs portion of the first rendered line —
// specifically, everything up to (but not including) the status summary
// ("N ids · ✓ ok" / "N ids · ! M ✗ K"). The status is covered by
// RegionHeaderStatus. This makes RegionHeader byte-identical between real
// and dummy backends (the nav tabs are pure chrome, not backend state).
func extractHeader(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	header := lines[0]
	plain := stripANSI(header)
	// status starts after the last nav-tab "Doctor " entry + trailing spaces
	markerIdx := strings.LastIndex(plain, "Doctor ")
	if markerIdx < 0 {
		return header
	}
	after := plain[markerIdx+len("Doctor "):]
	statusIdx := strings.IndexFunc(after, func(r rune) bool { return r != ' ' })
	if statusIdx < 0 {
		// no status — return full header
		return header
	}
	// truncate at the status start (in ANSI-preserved raw line)
	rawEnd := ansiOffsetToRaw(header, markerIdx+len("Doctor ")+statusIdx)
	if rawEnd < 0 || rawEnd >= len(header) {
		return header
	}
	// Trim trailing whitespace to normalize padding differences caused by
	// different status string lengths — the nav-tabs content is the same.
	return strings.TrimRight(header[:rawEnd], " \t")
}

// extractBreadcrumb returns the second rendered line (the breadcrumb/path).
func extractBreadcrumb(lines []string) string {
	if len(lines) < 2 {
		return ""
	}
	return lines[1]
}

// extractWizardStepper returns the right-pane "Step N/4 · …" line and the
// chord-hint line immediately below it. Only the right-pane portion (after │)
// is included to avoid sidebar content contamination.
func extractWizardStepper(lines []string) string {
	var out []string
	for i, line := range lines {
		if strings.Contains(stripANSI(line), "Step ") {
			out = append(out, rightPane(line))
			if i+1 < len(lines) {
				out = append(out, rightPane(lines[i+1]))
			}
			break
		}
	}
	return strings.Join(out, "\n")
}

// rightPane returns the portion of a rendered TUI line that is to the RIGHT
// of the "│" pane separator. If no separator is found, the whole line is
// returned (e.g. header and keybar lines span the full width).
func rightPane(line string) string {
	plain := stripANSI(line)
	idx := strings.Index(plain, "│")
	if idx < 0 {
		return line
	}
	// find the same character position in the original (with ANSI codes)
	// by scanning the ANSI-stripped offset back to the raw line
	rawIdx := ansiOffsetToRaw(line, idx)
	if rawIdx < 0 || rawIdx >= len(line) {
		return line
	}
	return line[rawIdx+1:] // skip the │ itself
}

// ansiOffsetToRaw maps an offset in the ANSI-stripped string to the
// corresponding byte offset in the raw (ANSI-containing) string.
func ansiOffsetToRaw(raw string, strippedOffset int) int {
	stripped := 0
	i := 0
	for i < len(raw) {
		if raw[i] == '\x1b' && i+1 < len(raw) && raw[i+1] == '[' {
			j := i + 2
			for j < len(raw) && raw[j] != 'm' {
				j++
			}
			i = j + 1
			continue
		}
		if stripped == strippedOffset {
			return i
		}
		stripped++
		i++
	}
	return -1
}

// extractFormFields returns the right-pane body section containing only the
// four approved SSH form field rows (Alias prefix, SSH Host, Real hostname,
// Port and their hints) — stopping before the Key toggle row. The Key row
// and catalog/picker rows are covered by RegionKeySection instead.
// Only the right-pane portion (after "│") is included.
func extractFormFields(lines []string) string {
	var out []string
	inFields := false
	for _, line := range lines {
		plain := stripANSI(line)
		if !inFields && strings.Contains(plain, "Shift+→") {
			inFields = true
			continue
		}
		// stop at the Key row (the generate/reuse toggle)
		rp := rightPane(line)
		rpPlain := stripANSI(rp)
		if inFields && strings.Contains(rpPlain, "Key") &&
			(strings.Contains(rpPlain, "Generate") || strings.Contains(rpPlain, "Reuse")) {
			break
		}
		// also stop at the preview box
		if inFields && (strings.Contains(plain, "╭╌") || strings.Contains(plain, "Live Host-block preview")) {
			break
		}
		if inFields {
			out = append(out, rp)
		}
	}
	return strings.Join(out, "\n")
}

// extractKeySection returns the right-pane lines covering the Key toggle and
// the algorithm catalog or reuse picker entries, from "Key (←/→ change)" until
// the Host-block preview box.
func extractKeySection(lines []string) string {
	var out []string
	inKey := false
	for _, line := range lines {
		plain := stripANSI(line)
		rp := rightPane(line)
		rpPlain := stripANSI(rp)
		if !inKey && strings.Contains(rpPlain, "Key") &&
			(strings.Contains(rpPlain, "Generate") || strings.Contains(rpPlain, "Reuse")) {
			inKey = true
		}
		if inKey && (strings.Contains(plain, "╭╌") || strings.Contains(plain, "Live Host-block preview")) {
			break
		}
		if inKey {
			out = append(out, rp)
		}
	}
	return strings.Join(out, "\n")
}

// extractHostPreview returns the Host-block preview box contents — the lines
// from "╭╌" through "╰╌" inclusive, BUT ONLY for the SSH Host-block preview
// box (which contains "Host " or "Live Host-block preview"). Stage command
// boxes on test screens ("╭╌ Stage 1 —") and git-fragment preview boxes are
// NOT considered Host-block previews. If no SSH Host-block preview box is
// found, an empty string is returned.
func extractHostPreview(lines []string) string {
	var out []string
	inBox := false
	isHostBox := false
	for _, line := range lines {
		plain := stripANSI(line)
		if !inBox && strings.Contains(plain, "╭╌") {
			// Check if this is the SSH Host-block preview (not a stage/git box)
			if strings.Contains(plain, "Host-block preview") || strings.Contains(plain, "Live Host") {
				inBox = true
				isHostBox = true
			}
		}
		if inBox && isHostBox {
			out = append(out, line)
			if strings.Contains(plain, "╰╌") {
				break
			}
		} else if inBox && !isHostBox {
			// wrong box — skip to its end
			if strings.Contains(plain, "╰╌") {
				inBox = false
			}
		}
	}
	return strings.Join(out, "\n")
}

// extractConnectivityOutput returns the right-pane lines showing the SSH test
// output on test-stage1-direct / test-stage2-by-alias screens.
func extractConnectivityOutput(lines []string) string {
	var out []string
	inOutput := false
	for _, line := range lines {
		plain := stripANSI(line)
		rp := rightPane(line)
		rpPlain := stripANSI(rp)
		if !inOutput && (strings.Contains(rpPlain, "ssh ") ||
			strings.Contains(rpPlain, "Running") ||
			strings.Contains(rpPlain, "Reachable") ||
			strings.Contains(rpPlain, "Permission denied") ||
			strings.Contains(rpPlain, "authenticated")) {
			inOutput = true
		}
		if inOutput && strings.Contains(plain, "Esc returns") {
			break
		}
		if inOutput {
			out = append(out, rp)
		}
	}
	return strings.Join(out, "\n")
}

// extractContinueDisabledReason returns the disabled reason line for the
// [ Continue ] button on the git-form-demo screen (right pane only).
func extractContinueDisabledReason(lines []string) string {
	for i, line := range lines {
		plain := stripANSI(line)
		if strings.Contains(plain, "[ Continue ]") || strings.Contains(plain, "Continue ]") {
			if i+1 < len(lines) {
				return rightPane(lines[i+1])
			}
		}
		rpPlain := strings.TrimSpace(stripANSI(rightPane(line)))
		if strings.HasPrefix(rpPlain, "—") &&
			(strings.Contains(rpPlain, "needs user.name") || strings.Contains(rpPlain, "Git configuration")) {
			return rightPane(line)
		}
	}
	return ""
}

// extractKeybar returns the last 2–3 non-empty lines (the keybar/footer area).
func extractKeybar(lines []string) string {
	var out []string
	for i := len(lines) - 1; i >= 0 && len(out) < 3; i-- {
		if strings.TrimSpace(stripANSI(lines[i])) != "" {
			out = append([]string{lines[i]}, out...)
		}
	}
	return strings.Join(out, "\n")
}

// extractReusePickerEntries returns the Key picker candidate rows on
// reuse-key-vs-generate screens — the dynamically-scanned key entries.
func extractReusePickerEntries(lines []string) string {
	var out []string
	inPicker := false
	for _, line := range lines {
		plain := stripANSI(line)
		// start after the "Reuse an existing key" toggle line
		if !inPicker && strings.Contains(plain, "Reuse an") && strings.Contains(plain, "existing key") {
			inPicker = true
			continue
		}
		// stop at the preview box
		if inPicker && strings.Contains(plain, "╭╌") {
			break
		}
		if inPicker {
			out = append(out, line)
		}
	}
	return strings.Join(out, "\n")
}

// extractHeaderStatus returns the rightmost portion of the header line (line 1)
// that shows the identity count and health summary. The stable nav-tabs portion
// of the header is always byte-identical between real and dummy backends.
func extractHeaderStatus(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	header := lines[0]
	plain := stripANSI(header)
	// The status summary ("N ids · ✓ ok" or "N ids · ! M ✗ K") always appears
	// after a run of spaces following the last nav-tab entry. Find the last
	// "· [N] Doctor " or "Doctor " occurrence and take what follows.
	markerIdx := strings.LastIndex(plain, "Doctor ")
	if markerIdx < 0 {
		return header
	}
	// find the non-space content after "Doctor " (account for padding)
	after := plain[markerIdx+len("Doctor "):]
	statusIdx := strings.IndexFunc(after, func(r rune) bool { return r != ' ' })
	if statusIdx < 0 {
		return ""
	}
	// map back to ANSI-preserved position
	rawStart := ansiOffsetToRaw(header, markerIdx+len("Doctor ")+statusIdx)
	if rawStart < 0 || rawStart >= len(header) {
		return header[markerIdx:]
	}
	return header[rawStart:]
}

// extractSidebar returns the pane-left area (content to the left of the │
// separator). The sidebar shows the identity list, health summary, and count.
func extractSidebar(lines []string) string {
	var out []string
	for _, line := range lines {
		// sidebar content precedes the │ separator; skip lines that are all
		// right-pane content (no left margin before │).
		// A sidebar line is one where there is non-whitespace content before │.
		idx := strings.Index(stripANSI(line), "│")
		if idx > 0 {
			plain := strings.TrimSpace(stripANSI(line[:idx]))
			if plain != "" {
				out = append(out, line[:idx])
			}
		}
	}
	return strings.Join(out, "\n")
}

// AllRegionNames returns all defined RegionNames for allowlist schema validation.
func AllRegionNames() []RegionName {
	return []RegionName{
		RegionHeader,
		RegionHeaderStatus,
		RegionBreadcrumb,
		RegionWizardStepper,
		RegionFormFields,
		RegionKeySection,
		RegionHostPreview,
		RegionConnectivityOutput,
		RegionContinueDisabledReason,
		RegionKeybar,
		RegionReusePickerEntries,
		RegionSidebar,
	}
}

// StripANSIExported is the exported wrapper of stripANSI, used by the
// gate runner in cmd/gitid to produce human-readable diff output without
// color codes obscuring the diff text.
func StripANSIExported(s string) string { return stripANSI(s) }

// stripANSI removes ANSI SGR escape sequences from s for structural
// matching (finding marker text without color interference).
// This is internal to region extraction only — the extracted region
// strings themselves preserve ANSI codes for the gate comparison.
func stripANSI(s string) string {
	var out strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			// skip until 'm'
			j := i + 2
			for j < len(s) && s[j] != 'm' {
				j++
			}
			i = j + 1
			continue
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}
