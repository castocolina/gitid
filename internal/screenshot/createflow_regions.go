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

	// RegionConfirmationPreview is the complete focusable review text in a
	// create ceremony, including summary, key path, and managed-block sentinels.
	RegionConfirmationPreview RegionName = "confirmation-preview"

	// RegionGitFormFields is the Configure-Git form's user.name/user.email
	// rows plus the compact gpg.format/signingkey/gpgsign/Force-SSH metadata
	// line (04-04-PLAN.md Task 3 — Phase 4 git-screen registration). Spans
	// from "user.name" up to (not including) the "Match strategy" row.
	RegionGitFormFields RegionName = "git-form-fields"

	// RegionGitStrategy is the git-screen's match-strategy header, its three
	// always-rendered option rows, and the conditional gitdir-path row. Spans
	// from "Match strategy" up to (not including) the fragment preview block.
	RegionGitStrategy RegionName = "git-strategy"

	// RegionGitPreview is the git-screen's fragment-file and includeIf-block
	// preview boxes. Spans from the first preview title through the
	// "Write it" button row.
	RegionGitPreview RegionName = "git-preview"

	// RegionGitCeremony is the Configure-Git write-ceremony pane's content —
	// from its heading ("Write Git identity for …") or its receipt heading
	// (the "… configured" result message) through the end of the frame.
	RegionGitCeremony RegionName = "git-ceremony"

	// RegionDetailSSHSection is the identity detail pane's "SSH — shown
	// first, always" section (FIELDS.md detail-ssh-first's ssh_section
	// field) — from that heading through the blank line before "Git".
	RegionDetailSSHSection RegionName = "identity-detail-ssh"

	// RegionDetailGitSection is the identity detail pane's "Git" section
	// (FIELDS.md's git_section_absent_note field, MGR-03) — from the "Git"
	// heading through the blank line before the global-baseline strip.
	RegionDetailGitSection RegionName = "identity-detail-git"

	// RegionDetailFindingsSection is the identity detail pane's
	// "Findings (N) …" section (FIELDS.md's per_identity_health field,
	// MGR-07) — from that heading to the end of the frame.
	RegionDetailFindingsSection RegionName = "identity-detail-findings"

	// RegionActionMenuRows is the action-menu's four approved rows
	// (FIELDS.md action-menu state) — from the "Actions — <name>" heading
	// to the end of the frame.
	RegionActionMenuRows RegionName = "identity-action-menu"

	// RegionDeleteChoiceOptions is the delete-choice screen's two scope
	// options (FIELDS.md delete-choice state) — from the
	// `Delete "<name>" — choose scope` heading to the end of the frame.
	RegionDeleteChoiceOptions RegionName = "identity-delete-choice"

	// RegionConfirmWarningBlock is the confirm-destructive/backup-notice
	// ceremony's warning + hint + scan-preview text (FIELDS.md
	// confirm_warning field) — from the ceremony heading ("Delete
	// EVERYTHING for …") through the "Exact change" preview title, so it
	// captures the D-11/D-12/D-13 additions without the dynamic diff body.
	RegionConfirmWarningBlock RegionName = "identity-confirm-warning"

	// RegionBackupPathList is every "Backup → "/"Backed up → " line on a
	// ceremony pane (FIELDS.md backup-notice's path fields) — matched by
	// line prefix wherever it appears in the frame.
	RegionBackupPathList RegionName = "identity-backup-paths"

	// RegionGSSOptionsBrowse is the Global SSH Options sub-tab's whole
	// master-detail body (06-07-PLAN.md Task 1, Phase 6 registration): the
	// option-row master list (left of the │ divider) AND its live detail
	// pane (right of the │), from the first pane row through the last one.
	// The sub-tab strip line above it is identical chrome on both surfaces.
	// Anchor: the detail pane's always-rendered advisory note
	// ("~ Recommended, not required") inside a │-pane line — present only
	// on the Options sub-tab, never on Storage or the ceremonies, so the
	// region is empty on every other screen (CR-10: no cross-screen
	// contamination of the comparison inventory).
	RegionGSSOptionsBrowse RegionName = "gss-options-browse"

	// RegionGSSStorageBrowse is the Global SSH Storage & preview sub-tab's
	// whole master-detail body: the STORE-01 left pane and the resulting-
	// config preview boxes (right of the │), from the first pane row that
	// introduces a preview ("Resulting config") through the last │ line.
	// Anchor: the preview label "Resulting config" inside a │-pane line —
	// rendered by BOTH the sentinel and Include layouts, never by the
	// Options sub-tab or the ceremonies.
	RegionGSSStorageBrowse RegionName = "gss-storage-browse"

	// RegionGSSApplyCeremony is the Global SSH apply ceremony's preview
	// body (06-07 registration): from the D-07 heading
	// ("Write Host * managed block to …") through the confirm/cancel
	// button row. The ceremony renders full-width (no │ divider), so
	// rightPane returns raw lines; the heading anchor is unique to this
	// ceremony on either surface.
	RegionGSSApplyCeremony RegionName = "gss-apply-ceremony"

	// RegionGSSApplyHeading is the apply ceremony's resolved-target heading
	// line ONLY (06-07 registration, T-06-CEREMONYTARGET): the D-07 heading
	// whose shared prefix is "Write Host * managed block to " and whose tail
	// is the RESOLVED storage target — a distinct region from the ceremony
	// body so the target-file divergence gets its own narrow disposition.
	RegionGSSApplyHeading RegionName = "gss-apply-heading"

	// RegionGSSStorageCeremony is the Global SSH storage-migration
	// ceremony's preview body (06-07 registration): from the STORE-03
	// heading ("Migrate SSH storage layout → …") through the confirm/cancel
	// button row. Full-width like the apply ceremony; anchor unique to this
	// ceremony on either surface.
	RegionGSSStorageCeremony RegionName = "gss-storage-ceremony"

	// RegionGGitOptionsBrowse is the Global Git Options master-detail body
	// (07-06 registration): the option-row master list (left of the │ divider)
	// AND its live detail pane (right of the │), from the first pane row
	// through the last one. Anchor: the detail pane's always-rendered advisory
	// note ("~ Recommended, not required") inside a │-pane line — present only
	// on the Options sub-tab, never on the ceremonies, so the region is empty
	// on every other screen (CR-10: no cross-screen contamination).
	RegionGGitOptionsBrowse RegionName = "ggit-options-browse"

	// RegionGGitApplyCeremony is the Global Git apply ceremony's preview body
	// (07-06 registration): from the D-07 heading ("Write global-git managed
	// block to …") through the confirm/cancel button row. The ceremony
	// renders full-width (no │ divider), so rightPane returns raw lines; the
	// heading anchor is unique to this ceremony on either surface.
	RegionGGitApplyCeremony RegionName = "ggit-apply-ceremony"

	// RegionGGitApplyHeading is the apply ceremony's resolved-target heading
	// line ONLY (07-06 registration): the D-07 heading whose shared prefix is
	// "Write global-git managed block to " and whose tail is the RESOLVED
	// baseline target — a distinct region so the target-file divergence gets
	// its own narrow disposition.
	RegionGGitApplyHeading RegionName = "ggit-apply-heading"

	// RegionHealthBody is the Health tab's master-detail body (08-08
	// registration): the findings list (left of the │ divider) AND its
	// always-visible inline detail pane (right of the │, Known Divergence
	// #1's finding-detail state). Anchor: the "Health" breadcrumb line —
	// unique to this tab, never rendered on Fixer or any other tab.
	RegionHealthBody RegionName = "health-body"

	// RegionFixerBody is the Fixer tab's master-detail body in list mode
	// (08-08 registration): the fixable-findings list plus its detail pane
	// (the "f · Fix this…" affordance and the batch-fix note). Anchor: the
	// "Fixer" breadcrumb line with NO trailing " › " crumb — present only in
	// list mode, never while a fix ceremony is open.
	RegionFixerBody RegionName = "fixer-body"

	// RegionFixerCeremony is the Fixer tab's fix ceremony (08-08
	// registration, Known Divergence #2's compressed 2-state ceremony):
	// from the "Fix: <title>" heading through the confirm/cancel button
	// row. Full-width (no │ divider) like the other ceremony regions.
	RegionFixerCeremony RegionName = "fixer-ceremony"

	// RegionUploadSection is the D-08/UP-02/UP-03 upload beat's own content
	// (09-07-PLAN.md Task 2): the "Running: <command>" announce lines, the
	// ✓/✗ per-registration result rows, and the manual-fallback heading and
	// instructions. It appears on TWO surfaces — the create-flow wizard's
	// step-2 "Test connection" pane (rendered via wizardModel's
	// renderTestScreen) and the identity-manager's register-key pane
	// (renderRegisterKey) — both call the SAME shared renderUploadSection
	// helper (internal/tuikit/identities.go), so one extractor covers both.
	//
	// Anchor: the first right-of-"│" line containing one of the upload
	// beat's own distinctive markers ("gh ssh-key add"/"glab ssh-key add",
	// "Register with " (the manual-fallback prompt heading), "Auto-
	// registration" (the declined/unavailable fallback heading), "key
	// registered", "registration failed", or "already registered"). These
	// markers are deliberately NOT the generic "Running"/"Reachable"/
	// "authenticated" words RegionConnectivityOutput already anchors on
	// (extractConnectivityOutput, above) — reusing those would make
	// RegionUploadSection silently overlap the SSH connectivity-test
	// region on the SAME screen (both regions can render on the wizard's
	// step-2 pane). When the match lands on the wrapped command line, the
	// extraction rewinds one line to also include its own immediately
	// preceding lone "Running:" label line, so the frozen label survives in
	// the extracted text. The region ends at the first Host-block/Stage-1
	// preview box ("╭╌") or the end of the frame, whichever comes first —
	// verified against Task 1's real captured frames
	// (.planning/phases/09-upload-credentials-assist/ui-frames/
	// create-flow-upload-autonomous-github.txt and
	// identity-manager-register-key-modal-runs.txt) before finalizing.
	RegionUploadSection RegionName = "upload-section"
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
	case RegionConfirmationPreview:
		return extractConfirmationPreview(lines)
	case RegionGitFormFields:
		return extractGitFormFields(lines)
	case RegionGitStrategy:
		return extractGitStrategy(lines)
	case RegionGitPreview:
		return extractGitPreview(lines)
	case RegionGitCeremony:
		return extractGitCeremony(lines)
	case RegionDetailSSHSection:
		return extractDetailSSHSection(lines)
	case RegionDetailGitSection:
		return extractDetailGitSection(lines)
	case RegionDetailFindingsSection:
		return extractDetailFindingsSection(lines)
	case RegionActionMenuRows:
		return extractActionMenuRows(lines)
	case RegionDeleteChoiceOptions:
		return extractDeleteChoiceOptions(lines)
	case RegionConfirmWarningBlock:
		return extractConfirmWarningBlock(lines)
	case RegionBackupPathList:
		return extractBackupPathList(lines)
	case RegionGSSOptionsBrowse:
		return extractGSSOptionsBrowse(lines)
	case RegionGSSStorageBrowse:
		return extractGSSStorageBrowse(lines)
	case RegionGSSApplyCeremony:
		return extractGSSApplyCeremony(lines)
	case RegionGSSApplyHeading:
		return extractGSSApplyHeading(lines)
	case RegionGSSStorageCeremony:
		return extractGSSStorageCeremony(lines)
	case RegionGGitOptionsBrowse:
		return extractGGitOptionsBrowse(lines)
	case RegionGGitApplyCeremony:
		return extractGGitApplyCeremony(lines)
	case RegionGGitApplyHeading:
		return extractGGitApplyHeading(lines)
	case RegionHealthBody:
		return extractHealthBody(lines)
	case RegionFixerBody:
		return extractFixerBody(lines)
	case RegionFixerCeremony:
		return extractFixerCeremony(lines)
	case RegionUploadSection:
		return extractUploadSection(lines)
	}
	return ""
}

// ---------------------------------------------------------------------------
// Git-screen regions (04-04-PLAN.md Task 3). The git-form/ceremony pane
// shares the SAME master/detail "│" divider as create-flow's wizard, so
// rightPane's ANSI-aware split reuses unchanged. These extractors use the
// same structural-marker approach as
// e2e/git_configuration_pty_e2e_test.go's ANSI-free PTY-frame extractors
// (Task 2), operating here on ANSI-PRESERVING captured text.
// ---------------------------------------------------------------------------

// extractGitFormFields returns the git-form's user.name/user.email rows plus
// the compact gpg.format/signingkey/gpgsign/Force-SSH metadata line.
func extractGitFormFields(lines []string) string {
	var out []string
	capturing := false
	for _, line := range lines {
		rp := rightPane(line)
		rpPlain := stripANSI(rp)
		if strings.Contains(rpPlain, "Match strategy") {
			break
		}
		if strings.Contains(rpPlain, "user.name") {
			capturing = true
		}
		if capturing {
			out = append(out, rp)
		}
	}
	return strings.Join(out, "\n")
}

// extractGitStrategy returns the match-strategy header, its three
// always-rendered option rows, and the conditional gitdir-path row.
func extractGitStrategy(lines []string) string {
	var out []string
	capturing := false
	for _, line := range lines {
		rp := rightPane(line)
		rpPlain := stripANSI(rp)
		if strings.Contains(rpPlain, "fragment file") {
			break
		}
		if strings.Contains(rpPlain, "Match strategy") {
			capturing = true
		}
		if capturing {
			out = append(out, rp)
		}
	}
	return strings.Join(out, "\n")
}

// extractGitPreview returns the fragment-file and includeIf-block preview
// boxes — from the first preview title up to the "Write it" button row.
func extractGitPreview(lines []string) string {
	var out []string
	capturing := false
	for _, line := range lines {
		rp := rightPane(line)
		rpPlain := stripANSI(rp)
		if strings.Contains(rpPlain, "Write it") {
			break
		}
		if strings.Contains(rpPlain, "fragment file") || strings.Contains(rpPlain, "includeIf block") {
			capturing = true
		}
		if capturing {
			out = append(out, rp)
		}
	}
	return strings.Join(out, "\n")
}

// extractGitCeremony returns the Configure-Git write-ceremony pane's content
// — from its heading ("Write Git identity for …") or its receipt heading
// (the "… configured — applies via" result message) through the end of the
// frame.
func extractGitCeremony(lines []string) string {
	rp := make([]string, len(lines))
	for i, line := range lines {
		rp[i] = stripANSI(rightPane(line))
	}
	// containsGitCeremonyMarker reports whether s (already whitespace-
	// collapsed) contains either receipt/heading marker phrase. WR-10:
	// "configured — applies via" (the exact receipt heading built by
	// identities.go's gitCeremonyFor: `Git identity "<name>" configured —
	// applies via the <strategy> strategy.`) is the actual marker — a bare
	// "configured" false-positives on any OTHER line that happens to
	// mention it, e.g. the sidebar note "no Git identity configured for
	// this alias", or a future "Not configured" status.
	containsGitCeremonyMarker := func(s string) bool {
		return strings.Contains(s, "Write Git identity") || strings.Contains(s, "configured — applies via")
	}
	start := -1
	for i := range lines {
		// WR-26: check the CURRENT row alone FIRST. The marker phrase is a
		// contiguous run checked against a SINGLE rendered row of a
		// width-constrained detail pane; when it is fully intact on row i
		// (the common, unwrapped case), row i alone already contains it —
		// resolving start here, not one row early. WR-22's original fix
		// checked the 2-row window (rp[i]+" "+rp[i+1]) FIRST: when the phrase
		// is fully intact on row k, the window at i=k-1 already contains it
		// too (rp[k] is embedded at the window's tail), so the loop broke one
		// row too early — absorbing an extra, arbitrary, potentially
		// nondeterministic row into RegionGitCeremony.
		if containsGitCeremonyMarker(strings.Join(strings.Fields(rp[i]), " ")) {
			start = i
			break
		}
		// WR-22: the phrase wraps as soon as the identity name is long
		// enough, splitting at either of its two internal spaces, silently
		// degrading the region to empty (a vacuous pass, not a caught
		// divergence) if only a same-row check ran. Only fall through to the
		// 2-row window when the phrase is NOT already on this row alone —
		// i.e. only when it genuinely spans the row boundary.
		if i+1 < len(lines) {
			// WR-26 (second half): a naive window check here reintroduces the
			// SAME off-by-one it is meant to fix, just shifted by one
			// iteration — if row i+1 alone already contains the full marker
			// (e.g. row i is some unrelated content and row i+1 is the
			// complete, unwrapped heading), the concatenated window ALSO
			// contains it as a substring, even though nothing actually spans
			// the boundary. Skip the window match in that case: the marker
			// belongs to row i+1, and iteration i+1's own row-alone check
			// above will correctly set start there.
			nextAlone := strings.Join(strings.Fields(rp[i+1]), " ")
			if containsGitCeremonyMarker(nextAlone) {
				continue
			}
			window := strings.Join(strings.Fields(rp[i]+" "+rp[i+1]), " ")
			if containsGitCeremonyMarker(window) {
				start = i
				break
			}
		}
	}
	if start < 0 {
		return ""
	}
	var out []string
	for _, line := range lines[start:] {
		out = append(out, rightPane(line))
	}
	return strings.Join(out, "\n")
}

func extractConfirmationPreview(lines []string) string {
	var out []string
	inCeremony := false
	for _, line := range lines {
		plain := stripANSI(line)
		if strings.Contains(plain, "Exact change") {
			inCeremony = true
		}
		if inCeremony && (strings.Contains(plain, "Exact change") || strings.Contains(plain, "# BEGIN gitid managed:") || strings.Contains(plain, "# END gitid managed:") || strings.Contains(plain, "SSH:")) {
			out = append(out, rightPane(line))
		}
	}
	return strings.Join(out, "\n")
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
	// status starts after the last nav-tab "Fixer " entry + trailing spaces
	// (08-01-PLAN.md Task 1's TabID 4->5 split retired the single "Doctor"
	// tab in favor of "Health"/"Fixer" — "Fixer" is now the last nav tab).
	markerIdx := strings.LastIndex(plain, "Fixer ")
	if markerIdx < 0 {
		return header
	}
	after := plain[markerIdx+len("Fixer "):]
	statusIdx := strings.IndexFunc(after, func(r rune) bool { return r != ' ' })
	if statusIdx < 0 {
		// no status — return full header
		return header
	}
	// truncate at the status start (in ANSI-preserved raw line)
	rawEnd := ansiOffsetToRaw(header, markerIdx+len("Fixer ")+statusIdx)
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
	return line[rawIdx+len("│"):] // skip the complete UTF-8 pane separator
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
		// "ssh -" (a flag-prefixed invocation, e.g. "ssh -T -F ...") is the
		// actual test-stage command marker. A bare "ssh " false-positives on
		// unrelated content that merely mentions ssh as a substring — e.g.
		// the git-screen form's "gpg.format=ssh " metadata line (04-04-PLAN.md
		// Task 3 discovery: this pre-existing over-broad marker made
		// extractConnectivityOutput swallow the REST of a git-form-demo
		// capture, which has no "Esc returns" line to close the region,
		// producing spurious real-vs-dummy divergence on an unrelated
		// screen/region pair).
		if !inOutput && (strings.Contains(rpPlain, "ssh -") ||
			strings.Contains(rpPlain, "Running") ||
			strings.Contains(rpPlain, "Reachable") ||
			strings.Contains(rpPlain, "Permission denied") ||
			strings.Contains(rpPlain, "authenticated") ||
			strings.Contains(rpPlain, "The connection failed")) {
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
	// "· [N] Fixer " or "Fixer " occurrence and take what follows ("Fixer" is
	// the last nav tab since 08-01-PLAN.md Task 1's TabID 4->5 split retired
	// the single "Doctor" tab).
	markerIdx := strings.LastIndex(plain, "Fixer ")
	if markerIdx < 0 {
		return header
	}
	// find the non-space content after "Fixer " (account for padding)
	after := plain[markerIdx+len("Fixer "):]
	statusIdx := strings.IndexFunc(after, func(r rune) bool { return r != ' ' })
	if statusIdx < 0 {
		return ""
	}
	// map back to ANSI-preserved position
	rawStart := ansiOffsetToRaw(header, markerIdx+len("Fixer ")+statusIdx)
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

// identRightOfDivider returns the (ANSI-preserved) content right of the "│"
// master/detail separator on one line, or "" (never the whole line) when no
// divider is present — mirroring e2e/identity_manager_pty_e2e_test.go's own
// identRightOfDivider so footer/status chrome (which never carries "│")
// never leaks into an extracted identity-manager region.
func identRightOfDivider(line string) string {
	idx := strings.Index(stripANSI(line), "│")
	if idx < 0 {
		return ""
	}
	raw := ansiOffsetToRaw(line, idx)
	if raw < 0 || raw+len("│") > len(line) {
		return ""
	}
	return line[raw+len("│"):]
}

// extractIdentityRightOfDividerBetween returns the right-of-divider content
// from the first line whose plain text contains startMarker through (not
// including) the first SUBSEQUENT line whose plain text contains
// endMarker — or through the end of the frame when endMarker is "" or never
// found. Shared by every identity-manager section extractor below (the
// SAME "from heading to next heading" shape createflow.go's own
// extractGitScreenFormFields/Strategy/Preview already use for git-screen).
func extractIdentityRightOfDividerBetween(lines []string, startMarker, endMarker string) string {
	var out []string
	capturing := false
	for _, line := range lines {
		rp := identRightOfDivider(line)
		plain := stripANSI(rp)
		if capturing && endMarker != "" && strings.Contains(plain, endMarker) {
			break
		}
		if !capturing && strings.Contains(plain, startMarker) {
			capturing = true
		}
		if capturing {
			out = append(out, rp)
		}
	}
	return strings.Join(out, "\n")
}

// extractDetailSSHSection returns the "SSH — shown first, always" section.
func extractDetailSSHSection(lines []string) string {
	return extractIdentityRightOfDividerBetween(lines, "SSH — shown first, always", "Git")
}

// extractDetailGitSection returns the "Git" section (MGR-03's honest
// absence-note field) — bounded by the "Global baseline" strip that always
// follows it.
func extractDetailGitSection(lines []string) string {
	// A plain Contains("Git") start marker is FAR too generic (it collides
	// with "Configure Git", "Write Git identity for", git-screen frames,
	// etc. — found empirically when this region's diff fired on the
	// UNRELATED create-flow "test-stage1-direct" screen). renderDetail's
	// section heading is a WHOLE, bare "Git" line (sectionHeader("Git")) —
	// require the right-of-divider content's TRIMMED plain text to equal
	// "Git" exactly, never a substring match.
	var out []string
	capturing := false
	for _, line := range lines {
		rp := identRightOfDivider(line)
		plain := strings.TrimSpace(stripANSI(rp))
		if capturing && strings.Contains(plain, "Global baseline") {
			break
		}
		if !capturing && plain == "Git" {
			capturing = true
		}
		if capturing {
			out = append(out, rp)
		}
	}
	return strings.Join(out, "\n")
}

// extractDetailFindingsSection returns the "Findings (N) …" section (MGR-07).
func extractDetailFindingsSection(lines []string) string {
	return extractIdentityRightOfDividerBetween(lines, "Findings (", "")
}

// extractActionMenuRows returns the action-menu's four rows.
func extractActionMenuRows(lines []string) string {
	return extractIdentityRightOfDividerBetween(lines, "Actions — ", "")
}

// extractDeleteChoiceOptions returns the delete-choice screen's two options.
func extractDeleteChoiceOptions(lines []string) string {
	return extractIdentityRightOfDividerBetween(lines, "— choose scope", "")
}

// extractConfirmWarningBlock returns the confirm-destructive ceremony's
// warning/hint/scan-preview text, stopping before the dynamic diff preview.
func extractConfirmWarningBlock(lines []string) string {
	return extractIdentityRightOfDividerBetween(lines, "Delete EVERYTHING for", "Exact change")
}

// extractBackupPathList returns every "Backup → "/"Backed up → " line,
// wherever it appears (both the pre-confirm promise and the post-confirm
// receipt use these exact prefixes — ceremony.go's view()).
func extractBackupPathList(lines []string) string {
	// Scoped to frames that also carry the confirm-destructive heading
	// ("Delete EVERYTHING for") — a bare substring search for "Backup → "
	// alone collides with EVERY OTHER ceremony pane in the registry
	// (create-flow's confirm-write, git-screen's review-readonly/result-
	// success — all share ceremony.go's identical backup-line rendering),
	// found empirically when this region fired on the unrelated create-flow
	// "confirm-write" screen. This region is currently used by ONLY the
	// identity-manager confirm-destructive checkpoint; if a future
	// checkpoint needs it too, widen the anchor list rather than dropping
	// it back to a bare substring search.
	hasAnchor := false
	for _, line := range lines {
		if strings.Contains(stripANSI(identRightOfDivider(line)), "Delete EVERYTHING for") {
			hasAnchor = true
			break
		}
	}
	if !hasAnchor {
		return ""
	}
	var out []string
	for _, line := range lines {
		rp := identRightOfDivider(line)
		plain := stripANSI(rp)
		if strings.Contains(plain, "Backup → ") || strings.Contains(plain, "Backed up → ") {
			out = append(out, rp)
		}
	}
	return strings.Join(out, "\n")
}

// gssPaneBodyAfter returns every line from the first line containing marker
// inside a │-pane line through the last │-containing line of the frame — the
// structural body boundary of both Global SSH master-detail sub-tabs
// (sub-tab strip and chrome sit above the first pane row; the status line
// and the two footer keybar rows carry no │). Returns "" when the anchor is
// absent so an unrelated surface never contributes content to this region
// (CR-10 cross-screen contamination guard).
func gssPaneBodyAfter(lines []string, marker string) string {
	start := -1
	for i, line := range lines {
		if !strings.Contains(stripANSI(line), "│") {
			continue
		}
		rp := stripANSI(rightPane(line))
		if strings.Contains(rp, marker) {
			start = i
			break
		}
	}
	if start < 0 {
		return ""
	}
	var out []string
	for _, line := range lines[start:] {
		if !strings.Contains(stripANSI(line), "│") {
			break
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// gssCeremonyBodyAfter returns every line from the first line containing
// heading through the line containing endMarker — the full-width ceremony
// body (no │ divider), bounded below by its own confirm/cancel button row so
// the status line and footer keybar never leak in. Returns "" when the
// heading is absent.
func gssCeremonyBodyAfter(lines []string, heading, endMarker string) string {
	start := -1
	for i, line := range lines {
		if strings.Contains(stripANSI(line), heading) {
			start = i
			break
		}
	}
	if start < 0 {
		return ""
	}
	var out []string
	for i, line := range lines[start:] {
		if i > 0 && strings.Contains(stripANSI(line), endMarker) {
			break
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// extractGSSOptionsBrowse returns the Options sub-tab master-detail body.
func extractGSSOptionsBrowse(lines []string) string {
	// Anchor on the "Global SSH › Options" breadcrumb, not on the detail
	// pane's advisory text. "Recommended, not required" was the original
	// anchor (it survives scroll/focus changes since it's boilerplate on
	// every advisory row), but 07-06 discovered it is NOT Global-SSH-
	// specific: plan 07-03's D-02 decision deliberately reuses the SAME
	// frozen sentence for Global Git's own advisory rows, so once Global
	// Git entered the registry this extractor started ALSO matching content
	// inside Global Git frames — 07-06's own <authority> block's CR-10
	// cross-screen-contamination hazard, materializing on the OTHER side
	// from where 07-06 first fixed it (extractGGitOptionsBrowse). The
	// breadcrumb is unique to this screen and present regardless of scroll
	// or focus state, exactly mirroring extractGGitOptionsBrowse's own fix.
	crumbIdx := -1
	for i, line := range lines {
		if strings.Contains(stripANSI(line), "Global SSH › Options") {
			crumbIdx = i
			break
		}
	}
	if crumbIdx < 0 {
		return ""
	}
	start := -1
	for i := crumbIdx + 1; i < len(lines); i++ {
		if strings.Contains(stripANSI(lines[i]), "│") {
			start = i
			break
		}
	}
	if start < 0 {
		return ""
	}
	var out []string
	for _, line := range lines[start:] {
		if !strings.Contains(stripANSI(line), "│") {
			break
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// extractGSSStorageBrowse returns the Storage & preview sub-tab master-detail body.
func extractGSSStorageBrowse(lines []string) string {
	return gssPaneBodyAfter(lines, "Resulting config")
}

// extractGSSApplyCeremony returns the apply ceremony's preview body.
func extractGSSApplyCeremony(lines []string) string {
	return gssCeremonyBodyAfter(lines, "Write Host * managed block to", "Apply selected (Enter)")
}

// extractGSSApplyHeading returns only the apply ceremony's D-07 resolved-
// target heading line(s) — the line(s) carrying "Write Host * managed block
// to", truncated to the heading paragraph so the body diff never leaks in.
func extractGSSApplyHeading(lines []string) string {
	var out []string
	for i, line := range lines {
		plain := stripANSI(line)
		if !strings.Contains(plain, "Write Host * managed block to") {
			continue
		}
		out = append(out, line)
		// WR-12: absorb every immediately-following wrapped continuation row
		// up to (but not including) the "Touches" row — a resolved target
		// long enough to wrap (the common case for a sandbox HOME like
		// /tmp/h4042748673/.ssh/config.d/gitid.config) spans MORE than one
		// row past the heading. The previous version appended nothing here
		// and always returned after the FIRST line, silently dropping the
		// wrapped tail from the comparison.
		for j := i + 1; j < len(lines) && !strings.Contains(stripANSI(lines[j]), "Touches"); j++ {
			out = append(out, lines[j])
		}
		break
	}
	return strings.Join(out, "\n")
}

// extractGSSStorageCeremony returns the storage-migration ceremony's preview body.
func extractGSSStorageCeremony(lines []string) string {
	return gssCeremonyBodyAfter(lines, "Migrate SSH storage layout", "Migrate (Enter)")
}

// ggitCeremonyBodyAfter returns every line from the first line containing
// heading through the line containing endMarker — the full-width ceremony
// body (no │ divider), bounded below by its own confirm/cancel button row.
// Returns "" when the heading is absent.
func ggitCeremonyBodyAfter(lines []string, heading, endMarker string) string {
	start := -1
	for i, line := range lines {
		if strings.Contains(stripANSI(line), heading) {
			start = i
			break
		}
	}
	if start < 0 {
		return ""
	}
	var out []string
	for i, line := range lines[start:] {
		if i > 0 && strings.Contains(stripANSI(line), endMarker) {
			break
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// extractGGitOptionsBrowse returns the Options sub-tab master-detail body.
func extractGGitOptionsBrowse(lines []string) string {
	// Anchor on the "Global Git › Options" breadcrumb, not on any row's own
	// text. A ROW-specific marker (the original approach anchored on
	// "init.defaultBranch", the first row's key) only survives while that
	// row's OWN detail happens to render on the right — the moment focus
	// scrolls past it (07-04's scroll cue state moves focus to a LATER
	// row), the right pane shows THAT row's detail instead and the marker
	// never matches, silently returning an empty region. The breadcrumb is
	// unique to this screen (never appears on Global SSH, avoiding CR-10's
	// cross-screen contamination hazard the "Recommended, not required"
	// anchor originally had) and is present on every Global Git Options
	// state regardless of scroll position or which row is focused.
	crumbIdx := -1
	for i, line := range lines {
		if strings.Contains(stripANSI(line), "Global Git › Options") {
			crumbIdx = i
			break
		}
	}
	if crumbIdx < 0 {
		return ""
	}
	// The master-detail body's first │-divided row may be preceded by a
	// findings-banner line (no │) — skip forward to the first │ line.
	start := -1
	for i := crumbIdx + 1; i < len(lines); i++ {
		if strings.Contains(stripANSI(lines[i]), "│") {
			start = i
			break
		}
	}
	if start < 0 {
		return ""
	}
	var out []string
	for _, line := range lines[start:] {
		if !strings.Contains(stripANSI(line), "│") {
			break
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// extractGGitApplyCeremony returns the apply ceremony's preview body.
func extractGGitApplyCeremony(lines []string) string {
	return ggitCeremonyBodyAfter(lines, "Write global-git managed block to", "Apply selected (Enter)")
}

// extractGGitApplyHeading returns only the apply ceremony's D-07 resolved-
// target heading line(s) — the line(s) carrying "Write global-git managed block
// to", truncated to the heading paragraph so the body diff never leaks in.
func extractGGitApplyHeading(lines []string) string {
	var out []string
	for i, line := range lines {
		plain := stripANSI(line)
		if !strings.Contains(plain, "Write global-git managed block to") {
			continue
		}
		out = append(out, line)
		// WR-12: absorb every immediately-following wrapped continuation row
		// up to (but not including) the "Touches" row — a resolved target
		// long enough to wrap spans MORE than one row past the heading.
		for j := i + 1; j < len(lines) && !strings.Contains(stripANSI(lines[j]), "Touches"); j++ {
			out = append(out, lines[j])
		}
		break
	}
	return strings.Join(out, "\n")
}

// extractHealthBody returns the Health tab's master-detail body: the
// findings list plus its inline detail pane. Anchored on the "Health"
// breadcrumb line (exact match after stripping ANSI/whitespace — the
// breadcrumb is its own dedicated line, never mixed with finding text), then
// scans forward to the first │-divided line and collects every consecutive
// │ line after it (08-08 registration).
func extractHealthBody(lines []string) string {
	crumbIdx := -1
	for i, line := range lines {
		if strings.TrimSpace(stripANSI(line)) == "Health" {
			crumbIdx = i
			break
		}
	}
	if crumbIdx < 0 {
		return ""
	}
	start := -1
	for i := crumbIdx + 1; i < len(lines); i++ {
		if strings.Contains(stripANSI(lines[i]), "│") {
			start = i
			break
		}
	}
	if start < 0 {
		return ""
	}
	var out []string
	for _, line := range lines[start:] {
		if !strings.Contains(stripANSI(line), "│") {
			break
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// extractFixerBody returns the Fixer tab's master-detail body in LIST mode
// only: anchored on the "Fixer" breadcrumb with no trailing " › " crumb (a
// ceremony's breadcrumb is "Fixer › Fix › <title>", which this exact match
// deliberately excludes — the ceremony body is RegionFixerCeremony's own
// region, never this one).
func extractFixerBody(lines []string) string {
	crumbIdx := -1
	for i, line := range lines {
		if strings.TrimSpace(stripANSI(line)) == "Fixer" {
			crumbIdx = i
			break
		}
	}
	if crumbIdx < 0 {
		return ""
	}
	start := -1
	for i := crumbIdx + 1; i < len(lines); i++ {
		if strings.Contains(stripANSI(lines[i]), "│") {
			start = i
			break
		}
	}
	if start < 0 {
		return ""
	}
	var out []string
	for _, line := range lines[start:] {
		if !strings.Contains(stripANSI(line), "│") {
			break
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// extractFixerCeremony returns the fix ceremony's body: from the "Fix: "
// heading (fixCeremonyFor's own Heading prefix, constant across every
// finding) through the "Cancel (Esc)" button label (ceremony.go's
// cancelLabel(), rendered once per ceremony regardless of state A/B).
func extractFixerCeremony(lines []string) string {
	return ggitCeremonyBodyAfter(lines, "Fix: ", "Cancel (Esc)")
}

// extractUploadSection returns the upload beat's own content (see
// RegionUploadSection's doc comment for the anchor rationale and the
// overlap-avoidance reasoning versus RegionConnectivityOutput).
func extractUploadSection(lines []string) string {
	// Deliberately excludes "Register with " (the D-01 checkbox's OWN label
	// text, rendered on step 0 whenever a gated host resolves) — that text
	// is part of the ordinary form body on EVERY create-flow step-0 screen
	// (ssh-form-filled, reuse-key-vs-generate, ...), not just Phase 9's own
	// specs; anchoring on it here would make RegionUploadSection collide
	// with every pre-existing step-0 spec that has no disposition for it.
	// RegionUploadSection is scoped to the upload BEAT's own content — the
	// announce/result/fallback text that appears only once the beat has
	// actually run or been explicitly declined.
	uploadMarkers := []string{
		"gh ssh-key add", "glab ssh-key add",
		"Auto-registration", "key registered", "registration failed",
		"already registered", "not logged in to",
	}
	start := -1
	for i, line := range lines {
		plain := stripANSI(rightPane(line))
		for _, marker := range uploadMarkers {
			if strings.Contains(plain, marker) {
				start = i
				break
			}
		}
		if start >= 0 {
			break
		}
	}
	if start < 0 {
		return ""
	}
	// Rewind over an immediately preceding lone "Running:" label line so the
	// frozen announce label survives in the extracted text.
	if start > 0 && strings.TrimSpace(stripANSI(rightPane(lines[start-1]))) == "Running:" {
		start--
	}
	var out []string
	for _, line := range lines[start:] {
		plain := stripANSI(line)
		if strings.Contains(plain, "╭╌") {
			break
		}
		out = append(out, rightPane(line))
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
		RegionKeybar,
		RegionReusePickerEntries,
		RegionSidebar,
		RegionConfirmationPreview,
		RegionGitFormFields,
		RegionGitStrategy,
		RegionGitPreview,
		RegionGitCeremony,
		RegionDetailSSHSection,
		RegionDetailGitSection,
		RegionDetailFindingsSection,
		RegionActionMenuRows,
		RegionDeleteChoiceOptions,
		RegionConfirmWarningBlock,
		RegionBackupPathList,
		RegionGSSOptionsBrowse,
		RegionGSSStorageBrowse,
		RegionGSSApplyCeremony,
		RegionGSSApplyHeading,
		RegionGSSStorageCeremony,
		RegionGGitOptionsBrowse,
		RegionGGitApplyCeremony,
		RegionGGitApplyHeading,
		RegionHealthBody,
		RegionFixerBody,
		RegionFixerCeremony,
		RegionUploadSection,
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
