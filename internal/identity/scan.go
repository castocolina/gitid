// Package identity — scan.go implements the D-13 advisory unmanaged-reference
// scan: before a delete-everything, warn the user about every OTHER place
// (outside the identity's own managed blocks) that still mentions the
// identity's alias. The scan is advisory only — see ScanUnmanagedReferences'
// doc comment — it never blocks or alters the delete.
package identity

import "strings"

// ScanRegion labels WHERE inside a scanned file a hit was found. D-13 says
// "unmanaged regions", but a reference inside a SIBLING identity's own
// managed block is a real operational consequence of the delete and must be
// disclosed, not silently dropped alongside the identity's own block
// (review R-09) — hence three regions, not two.
type ScanRegion string

const (
	// ScanRegionOwnManaged is bytes inside the identity BEING DELETED's own
	// managed block(s). ScanUnmanagedReferences DROPS hits here — they are
	// exactly what the delete removes, so reporting them would be noise.
	ScanRegionOwnManaged ScanRegion = "own-managed"
	// ScanRegionOtherManaged is bytes inside ANOTHER gitid-managed
	// identity's block. Reported and tagged, so the user sees which sibling
	// still references the alias.
	ScanRegionOtherManaged ScanRegion = "other-managed"
	// ScanRegionUnmanaged is foreign text outside every managed block.
	// Reported and tagged.
	ScanRegionUnmanaged ScanRegion = "unmanaged"
)

// UnmanagedHit is one D-13 scan match: alias appeared as a whole token in
// file at line (1-based), inside region, and Text carries the matched line
// verbatim (untruncated — the domain never truncates; the render layer
// applies the existing preview-block clip convention, per 05-UI-SPEC.md's
// unresolved truncation-width question, resolved by plan 05-04 this way and
// implemented by plan 05-06).
type UnmanagedHit struct {
	File   string
	Line   int
	Region ScanRegion
	Text   string
}

// ScanSource is one region-tagged chunk of a scanned file's bytes.
// SplitScanRegions is the one place that produces these from a whole file's
// content; a caller MAY also construct ScanSource values directly (e.g. a
// test, or a composition root scanning a file with no managed blocks at
// all — a single ScanRegionUnmanaged source covering the whole file).
type ScanSource struct {
	File   string
	Bytes  []byte
	Region ScanRegion
}

// UnmanagedScanDisclaimer is the frozen, D-13-mandated disclaimer: the scan
// cannot see repository remotes (they live in each repo's own .git/config,
// never in the four files gitid scans), so a `git@<alias>:...` remote will
// break after the delete regardless of what the scan finds. This is the
// SINGLE definition of that sentence for the whole phase (review R-27):
// plan 05-06 registers THIS constant in the copy-freeze gate and renders it
// by reference — it must never declare a second copy of the wording in
// internal/tuikit. Wording matches 05-UI-SPEC.md's Copywriting Contract
// verbatim; "<alias>" is a literal placeholder token the render layer
// substitutes with the real alias, never re-worded here.
const UnmanagedScanDisclaimer = "Repo remotes using git@<alias>: cannot be scanned and will break after this delete."

// SplitScanRegions splits file's content into region-tagged ScanSource
// chunks: bytes inside ownIdentity's own gitid-managed block(s)
// (ScanRegionOwnManaged), bytes inside any OTHER gitid-managed block
// (ScanRegionOtherManaged, tagged with that block's own name via a
// per-source note is NOT carried — the region tag alone is what
// ScanUnmanagedReferences consults), and everything else
// (ScanRegionUnmanaged). Sources are returned in file order and their Bytes
// concatenate back to content exactly (CRLF normalized to LF first, mirroring
// filewriter.ListBlocks' own normalization) — this is the ONE place the
// region-split rule lives, consumed by both the composition root and tests
// (review R-09).
func SplitScanRegions(file string, content []byte, ownIdentity string) []ScanSource {
	normalised := strings.ReplaceAll(string(content), "\r\n", "\n")
	lines := strings.SplitAfter(normalised, "\n")

	var sources []ScanSource
	var cur strings.Builder
	curRegion := ScanRegionUnmanaged
	inBlock := false
	currentName := ""

	flush := func() {
		if cur.Len() == 0 {
			return
		}
		sources = append(sources, ScanSource{File: file, Bytes: []byte(cur.String()), Region: curRegion})
		cur.Reset()
	}

	for _, line := range lines {
		trimmed := strings.TrimRight(line, "\n\r")
		switch {
		case !inBlock && strings.HasPrefix(trimmed, beginPrefixForScan):
			// A new managed block begins: flush whatever unmanaged/other
			// region preceded it, then start accumulating the block's own
			// region (own vs other, decided by name).
			flush()
			currentName = strings.TrimPrefix(trimmed, beginPrefixForScan)
			curRegion = regionForBlockName(currentName, ownIdentity)
			inBlock = true
			cur.WriteString(line)
		case inBlock && strings.HasPrefix(trimmed, endPrefixForScan) && strings.TrimPrefix(trimmed, endPrefixForScan) == currentName:
			cur.WriteString(line)
			flush()
			inBlock = false
			currentName = ""
			curRegion = ScanRegionUnmanaged
		default:
			cur.WriteString(line)
		}
	}
	flush()
	return sources
}

// beginPrefixForScan / endPrefixForScan mirror filewriter.BeginPrefix/
// EndPrefix (this file avoids importing internal/filewriter's ReplaceBlock/
// RemoveBlock machinery for a byte-span-tracking scan that package does not
// expose — only the two sentinel-line prefixes are duplicated, matching the
// project's existing accepted-duplication precedent for small,
// keep-in-sync literals).
const (
	beginPrefixForScan = "# BEGIN gitid managed: "
	endPrefixForScan   = "# END gitid managed: "
)

func regionForBlockName(blockName, ownIdentity string) ScanRegion {
	if blockName == ownIdentity {
		return ScanRegionOwnManaged
	}
	return ScanRegionOtherManaged
}

// ScanUnmanagedReferences scans sources for whole-token references to alias,
// dropping any hit inside ScanRegionOwnManaged (that region is exactly what
// the delete removes) and reporting every hit in ScanRegionOtherManaged or
// ScanRegionUnmanaged, tagged with its region. Matching uses whole-token
// boundaries — never a bare strings.Contains substring match (D-13's
// discretion note and 05-RESEARCH.md's Security Domain table both forbid
// it): a line is split into whitespace-delimited fields, each field is
// trimmed of surrounding quote punctuation, and compared against alias
// either as a bare token, as the host segment of a "git@<alias>:..." form,
// or as the segment before a ":" in a bare "<alias>:..." form. Line numbers
// are 1-based and absolute within each source's File — a running per-file
// line cursor advances across every source sharing that File value, so
// SplitScanRegions' multiple same-file sources still number correctly
// end-to-end.
//
// This is advisory only: it returns hits and NEVER blocks or alters a
// delete, and does not itself return UnmanagedScanDisclaimer — callers
// (DeletePlan) attach that fixed disclaimer alongside the hits.
func ScanUnmanagedReferences(alias string, sources []ScanSource) []UnmanagedHit {
	if alias == "" {
		return nil
	}
	var hits []UnmanagedHit
	lineCursor := make(map[string]int)
	for _, src := range sources {
		for _, line := range splitKeepLines(src.Bytes) {
			lineCursor[src.File]++
			if src.Region == ScanRegionOwnManaged {
				continue
			}
			text := strings.TrimRight(line, "\n\r")
			if lineReferencesAlias(text, alias) {
				hits = append(hits, UnmanagedHit{
					File:   src.File,
					Line:   lineCursor[src.File],
					Region: src.Region,
					Text:   text,
				})
			}
		}
	}
	return hits
}

// splitKeepLines splits b into lines, each retaining its own line ending
// (mirroring strings.SplitAfter), and drops the trailing empty element
// SplitAfter produces when b ends with the separator — so a trailing
// newline never counts as an extra, phantom line.
func splitKeepLines(b []byte) []string {
	if len(b) == 0 {
		return nil
	}
	parts := strings.SplitAfter(string(b), "\n")
	if len(parts) > 0 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}

// lineReferencesAlias reports whether line contains alias as a WHOLE TOKEN —
// never as a prefix/suffix/substring of a longer token.
func lineReferencesAlias(line, alias string) bool {
	if alias == "" {
		return false
	}
	for _, field := range strings.Fields(line) {
		if tokenMatchesAlias(field, alias) {
			return true
		}
	}
	return false
}

// tokenMatchesAlias reports whether one whitespace-delimited field matches
// alias, after trimming the surrounding punctuation OpenSSH Host directives
// and git URLs commonly wrap an alias in. Three shapes are recognized:
//
//	"<alias>"        — a bare token (an OpenSSH Host directive value).
//	"git@<alias>:..." — a git-over-SSH URL's user@host segment.
//	"<alias>:..."     — a bare host:path form (no explicit user@).
func tokenMatchesAlias(field, alias string) bool {
	trimmed := strings.Trim(field, `"'`)

	if strings.HasPrefix(trimmed, "git@") {
		rest := strings.TrimPrefix(trimmed, "git@")
		if idx := strings.IndexByte(rest, ':'); idx >= 0 {
			rest = rest[:idx]
		}
		if rest == alias {
			return true
		}
	}

	if idx := strings.IndexByte(trimmed, ':'); idx >= 0 && trimmed[:idx] == alias {
		return true
	}

	return trimmed == alias
}
