package gitconfig

import (
	"fmt"
	"strings"

	"github.com/castocolina/gitid/internal/filewriter"
)

// GlobalGitBlockName is the sentinel suffix of the gitid-managed block that
// holds the global-git options in the include'd baseline file. The full
// sentinel lines are:
//
//	# BEGIN gitid managed: global-git
//	# END gitid managed: global-git
//
// This is the frozen name from internal/tuikit/design.go's GlobalGitSentinelBegin.
const GlobalGitBlockName = "global-git"

// LegacyGlobalGitBlockName is the pre-Phase-7 sentinel key an earlier gitid
// (POC era) wrote under. EnsureGlobalGit ADOPTS a machine still carrying a
// block under this name — the body's key/value pairs survive the rename, the
// position in the file is preserved, and the legacy block is removed in the
// same compose, so no machine ever ends up carrying both names (D-01,
// 07-CONTEXT.md). Both names are registered in IsReservedBlockName so the
// doctor's destructive fix path can never delete a block gitid immediately
// rewrites (project learning L4, T-07-03).
const LegacyGlobalGitBlockName = "baseline"

// GlobalGitSentinelBegin is the opening sentinel line of the global-git block.
// It mirrors internal/tuikit/design.go's GlobalGitSentinelBegin constant and
// is kept here as the gitconfig package's authority so write tests can assert
// the produced bytes match the fixture without importing tuikit.
const GlobalGitSentinelBegin = "# BEGIN gitid managed: " + GlobalGitBlockName

// GlobalGitSentinelEnd is the closing sentinel line of the global-git block.
const GlobalGitSentinelEnd = "# END gitid managed: " + GlobalGitBlockName

// globalGitExcludesKey is the one key EnsureGlobalGit never emits and
// actively drops from adopted bodies. D-11 gives Phase 8 both halves of the
// global-gitignore setting together (the key AND the pattern file); writing
// only the key here would produce a dangling pointer that git silently
// tolerates — an invisibly broken half-state.
const globalGitExcludesKey = "core.excludesfile"

// globalGitSectionOrder is the canonical section order the merged body renders
// in. It matches GlobalGitFullManagedBlockText's declaration order in
// internal/tuikit/design.go. Plan 07-03 fills all sections; plan 07-01 uses
// only [init]. A section is emitted whenever the merged result has at least
// one key in it — an empty section is never written.
var globalGitSectionOrder = []string{
	"init",
	"core",
	"push",
	"pull",
	"fetch",
	"color",
	"merge",
	"diff",
	"alias",
}

// EnsureGlobalGit is the ONE owner of the global-git managed block in the
// include'd baseline file (D-01, 07-CONTEXT.md). It merges, then composes:
//
//  1. Reads the existing block body (from GlobalGitBlockName OR
//     LegacyGlobalGitBlockName — adoption path) and parses it into a
//     key→value map.
//  2. Overlays the confirmed selection on top, so selected keys win and keys
//     already in the block survive untouched (R-2 additive merge).
//  3. Renders the merged map into the frozen section order, emitting a section
//     only when the merged result has at least one key in it.
//  4. Replaces (or creates) the GlobalGitBlockName block; removes any
//     LegacyGlobalGitBlockName block so no machine ends up carrying both.
//
// WHY the merge is additive: under this phase's selection rules a key gitid
// already applied classifies as already-set and is therefore NEVER in a
// selection. A selection-driven overwrite (without the prior merge) would
// delete every previously applied setting on every apply — silently — because
// the selection only contains keys that were absent or wrong. The additive
// merge is what makes EnsureGlobalGit safe to call repeatedly on a real machine
// (R-2, 07-01-PLAN.md).
//
// The ONE key the merge subtracts is core.excludesfile: it is dropped from an
// adopted body rather than preserved, because D-11 rejects a dangling excludes
// pointer written without its pattern file (Phase 8 owns both halves together).
// Every other non-policy key found in an adopted body is preserved — the
// deferred pager setting is the live example.
//
// EnsureGlobalGit returns an error if selected contains core.excludesfile
// (Phase 8's owner), so the call site gets a named refusal rather than a
// silent drop.
func EnsureGlobalGit(existing []byte, selected map[string]string) ([]byte, error) {
	// Guard: reject core.excludesfile in the selection (D-11 owner: Phase 8).
	for k := range selected {
		if strings.EqualFold(k, globalGitExcludesKey) {
			return nil, fmt.Errorf("gitconfig: EnsureGlobalGit: %q is not selectable — Phase 8 owns writing it together with the gitignore pattern file (D-11)", globalGitExcludesKey)
		}
	}

	// Validate all selected values before composing anything (T-07-04).
	for k, v := range selected {
		if err := validateValue(k, v); err != nil {
			return nil, fmt.Errorf("gitconfig: EnsureGlobalGit: %w", err)
		}
	}

	// Step 1: extract the existing block body — from the current name or the
	// legacy name (adoption path). Prefer the current name.
	existingBody := existingGlobalGitBody(existing)

	// Also validate values carried over from the existing block (an adopted
	// legacy block is user-reachable content that could contain injected values).
	mergedRaw := parseGlobalGitBody(existingBody)
	for k, v := range mergedRaw {
		if err := validateValue(k, v); err != nil {
			// A legacy block with a bad value is silently dropped rather than
			// blocking the compose — the user's file existed before gitid and
			// we must not refuse to write just because it had a quirky value.
			delete(mergedRaw, k)
		}
	}

	// Drop core.excludesfile from the adopted body (D-11.2).
	delete(mergedRaw, globalGitExcludesKey)
	delete(mergedRaw, strings.ToLower(globalGitExcludesKey))

	// Step 2: overlay the confirmed selection — selected keys always win.
	for k, v := range selected {
		mergedRaw[strings.ToLower(k)] = v
	}

	// Step 3: render in section order.
	body := renderGlobalGitBody(mergedRaw)

	// Step 4: compose — replace the current-name block and remove the legacy
	// block so no machine ends up carrying both.
	composed := filewriter.ReplaceBlock(existing, GlobalGitBlockName, body)
	composed = filewriter.RemoveBlock(composed, LegacyGlobalGitBlockName)

	return composed, nil
}

// existingGlobalGitBody extracts the body of the managed block from existing
// content, preferring the current name over the legacy name (adoption path).
func existingGlobalGitBody(existing []byte) string {
	if existing == nil {
		return ""
	}
	for _, b := range filewriter.ListBlocks(existing) {
		if b.Name == GlobalGitBlockName {
			return b.Body
		}
	}
	// Fall back to the legacy name (adoption path).
	for _, b := range filewriter.ListBlocks(existing) {
		if b.Name == LegacyGlobalGitBlockName {
			return b.Body
		}
	}
	return ""
}

// parseGlobalGitBody parses a managed block body (tab-indented gitconfig
// format) into a lowercase section.key→value map. It reuses the same logic as
// parseGitconfigBlockBody in baseline.go. Returns an empty map on empty input.
func parseGlobalGitBody(body string) map[string]string {
	return parseGitconfigBlockBody(body)
}

// renderGlobalGitBody renders the merged key→value map into the frozen section
// order. Sections with no keys are omitted. The key→value map uses lowercase
// "section.key" form (as git --list emits). The rendered output uses
// tab-indented key = value lines inside bracketed section headers.
//
// Keys that belong to a known section are grouped there; unknown keys are
// appended to an [extras] section (placeholder for future policy additions).
func renderGlobalGitBody(merged map[string]string) string {
	if len(merged) == 0 {
		return ""
	}

	var b strings.Builder

	// Render each known section in order.
	for _, section := range globalGitSectionOrder {
		sectionKeys := keysForSection(merged, section)
		if len(sectionKeys) == 0 {
			continue
		}
		fmt.Fprintf(&b, "[%s]\n", section)
		for _, kv := range sectionKeys {
			fmt.Fprintf(&b, "\t%s = %s\n", kv.displayKey, kv.value)
		}
	}

	// Append any keys that don't belong to a known section (future-proofing).
	for fullKey, value := range merged {
		if !belongsToKnownSection(fullKey) {
			parts := strings.SplitN(fullKey, ".", 2)
			if len(parts) == 2 {
				fmt.Fprintf(&b, "[%s]\n\t%s = %s\n", parts[0], parts[1], value)
			}
		}
	}

	return strings.TrimRight(b.String(), "\n")
}

// sectionKeyValue is a (displayKey, value) pair for rendering.
type sectionKeyValue struct {
	displayKey string // the key without the section prefix (e.g. "defaultBranch")
	value      string
}

// keysForSection returns all (displayKey, value) pairs from merged that belong
// to section, in a deterministic order matching the frozen block text.
func keysForSection(merged map[string]string, section string) []sectionKeyValue {
	var keys []sectionKeyValue
	// Use the frozen section key order from GlobalGitFullManagedBlockText.
	// This is the order plan 07-01 uses; plan 07-03 fills it out to all keys.
	for _, canonical := range canonicalSectionKeys[section] {
		fullKey := strings.ToLower(section + "." + canonical)
		if value, ok := merged[fullKey]; ok {
			keys = append(keys, sectionKeyValue{displayKey: canonical, value: value})
		}
	}
	// Append any extra keys in this section not in the canonical list.
	prefix := section + "."
	for fullKey, value := range merged {
		if strings.HasPrefix(fullKey, prefix) {
			subKey := fullKey[len(prefix):]
			found := false
			for _, ck := range canonicalSectionKeys[section] {
				if strings.EqualFold(ck, subKey) {
					found = true
					break
				}
			}
			if !found {
				keys = append(keys, sectionKeyValue{displayKey: subKey, value: value})
			}
		}
	}
	return keys
}

// canonicalSectionKeys defines the canonical key order within each section,
// matching the frozen GlobalGitFullManagedBlockText in internal/tuikit/design.go.
// Plan 07-03 fills all sections; plan 07-01 only needs [init].
var canonicalSectionKeys = map[string][]string{
	"init":  {"defaultBranch"},
	"core":  {"ignorecase", "autocrlf", "eol", "pager"},
	"push":  {"autoSetupRemote"},
	"pull":  {"rebase"},
	"fetch": {"prune"},
	"color": {"ui", "branch", "diff", "status"},
	"merge": {"conflictstyle"},
	"diff":  {"colorMoved"},
	"alias": {"st", "co", "br", "ci", "df", "lg", "unstage", "last"},
}

// belongsToKnownSection returns true when fullKey belongs to one of the
// sections in globalGitSectionOrder.
func belongsToKnownSection(fullKey string) bool {
	for _, section := range globalGitSectionOrder {
		if strings.HasPrefix(fullKey, section+".") {
			return true
		}
	}
	return false
}

// ComposeBaselineInclude returns existing with the gitid managed [include]
// block pointing at baselineFilePath prepended at the top when absent, or
// updated in place when present (preserving its floor position). This is the
// composition half extracted from WriteBaselineInclude so the global-git apply
// ceremony can compose and write unconditionally (R-3), while WriteBaselineInclude's
// existing callers (the doctor Baseline check and the cmd-layer wiring
// dispatcher) keep the idempotent-skip contract they depend on, byte for byte.
func ComposeBaselineInclude(existing []byte, baselineFilePath string) []byte {
	includeBody := "[include]\n\tpath = " + baselineFilePath
	return filewriter.PrependBlockIfNotFound(existing, BaselineIncludeBlockName, includeBody)
}
