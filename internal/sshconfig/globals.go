package sshconfig

import (
	"fmt"
	"strings"

	"github.com/castocolina/gitid/internal/filewriter"
	"github.com/castocolina/gitid/internal/platform"
)

// GlobalBlockName is the sentinel key of gitid's single `Host *` managed
// block (D-06, D-08). Every write flow that may touch the wildcard stanza —
// create, rotate, repair, and the global-SSH fix ceremony — reaches it
// through EnsureGlobals, the ONE owner of the block. Keying it separately
// from per-identity blocks lets the writer rewrite it idempotently and always
// keep it LAST, after every specific host block, so first-match-wins
// resolution keeps the aliases authoritative (Pitfall 5 / T-02-15).
const GlobalBlockName = "global-ssh"

// LegacyGlobalBlockName is the pre-D-08 sentinel key an earlier gitid wrote
// under. EnsureGlobals ADOPTS a machine still carrying a block under this
// name — the body's key/value pairs survive the rename — and removes the
// legacy block in the same write, so no machine ever ends up carrying both
// names (D-08). Plan 06-02 discharges the registry consolidation: BOTH names
// join IsReservedBlockName and the migration classification there.
const LegacyGlobalBlockName = "_global"

// GlobalHostStarOrder is the canonical key order the merged `Host *` body
// renders in. It mirrors the D-10 recommendation table's declaration order
// (which must equal tuikit.GlobalSSHOptions' fixture order — pinned by a test
// in internal/globalssh); a hand-added directive inside the block that is NOT
// in this list is appended after the ordered keys so it is never dropped.
var GlobalHostStarOrder = []string{
	"StrictHostKeyChecking",
	"ForwardAgent",
	"HashKnownHosts",
	"IdentitiesOnly",
	"AddKeysToAgent",
	"UseKeychain",
}

// EnsureGlobals is the ONE entry point that owns the gitid `Host *` managed
// block (D-06). It reads the current body from the block named GlobalBlockName
// (or, when absent, from LegacyGlobalBlockName), merges its key/value pairs
// with the darwin-only platform defaults (for ABSENT keys only — an existing
// value always wins) and the caller's explicit overlay (unconditional — the
// user's confirmed global-SSH fixes always win), renders the canonical block,
// and composes it into existing, removing any legacy-named block in the same
// write.
//
// The rendered body is canonical (D-11, recipes/ssh-config.recipe's own
// first-directive shape):
//
//	IgnoreUnknown UseKeychain
//
//	Host *
//	  <merged keys, in GlobalHostStarOrder, then any unrecognised key>
//
// PLATFORM CONTRACT (the retired renderer documented the opposite — pinned
// here after the review found it undocumented):
//   - the `IgnoreUnknown UseKeychain` guard directive is emitted
//     UNCONDITIONALLY on every platform, and first, because it is inert where
//     the guarded directive is absent and is the only thing that keeps a
//     configuration synced from a mac from hard-erroring on linux (D-11);
//   - a darwin-only key already present in an existing body is PRESERVED on
//     linux, never dropped — dropping it would silently degrade a synced
//     configuration;
//   - darwin-only DEFAULTS (UseKeychain yes, AddKeysToAgent yes) are supplied
//     only when platform.SupportsUseKeychain(goos) is true;
//   - the function never returns content in which a previously non-empty
//     globals block became empty or absent.
//
// PLACEMENT DIVERGENCE (D-09, documented where the code makes the choice):
// recipes/ssh-config.recipe shows the wildcard stanza at the TOP of its
// example file, but there the shape is inert — no key in that `Host *`
// collides with a host block. gitid's `Host *` carries keys that DO interact
// with per-alias values, so first-match-wins makes LAST position the only
// correct choice. Do not "fix" this back toward the recipe; the ordering is
// asserted by tests, not incidental.
//
// The composed bytes (block plus every byte of foreign content) are validated
// with a second Parse pass before returning, so a render that would not
// round-trip is rejected rather than persisted (T-06-06).
func EnsureGlobals(existing []byte, explicit map[string]string, goos string) ([]byte, error) {
	merged := parseGlobalBody(existingGlobalBody(existing))

	// Overlay platform defaults for ABSENT keys only — existing values always
	// win (D-06).
	if platform.SupportsUseKeychain(goos) {
		if _, ok := merged.lookup("UseKeychain"); !ok {
			merged.set("UseKeychain", "yes")
		}
		if _, ok := merged.lookup("AddKeysToAgent"); !ok {
			merged.set("AddKeysToAgent", "yes")
		}
	}
	// Overlay the explicit fixes unconditionally — these are the user's
	// confirmed global-SSH fixes and they always win (D-16).
	for k, v := range explicit {
		merged.set(k, v)
	}

	rendered := renderGlobalBody(merged)
	composed := filewriter.ReplaceBlock(existing, GlobalBlockName, rendered)
	composed = filewriter.RemoveBlock(composed, LegacyGlobalBlockName)
	composed = ensureGlobalsLast(composed, rendered)

	// Round-trip safety: parse -> compose -> parse stability, exactly as Write
	// already requires of its own composition.
	if _, perr := Parse(composed); perr != nil {
		return nil, fmt.Errorf("sshconfig: composed globals block is not parseable, refusing to write: %w", perr)
	}
	return composed, nil
}

// ensureGlobalsLast is the D-09 post-condition: the globals block's start
// offset must be greater than the start offset of every other gitid-managed
// block. ReplaceBlock updates an existing block IN PLACE, so an adopted
// legacy block (or a create that appended an identity after a prior globals
// write) can sit in the wrong position; when that happens, remove and
// re-append so the block lands last. Do not rely on ReplaceBlock to preserve
// last-position — that is incidental, not the invariant.
func ensureGlobalsLast(content []byte, body string) []byte {
	if !globalsBlockIsLast(content) {
		content = filewriter.RemoveBlock(content, GlobalBlockName)
		content = filewriter.ReplaceBlock(content, GlobalBlockName, body)
	}
	return content
}

// globalsBlockIsLast reports whether the GlobalBlockName block starts after
// every other gitid-managed block in content. A missing globals block is
// treated as "not last" so a caller that expected one to exist still
// re-appends.
func globalsBlockIsLast(content []byte) bool {
	globalsOff := -1
	otherOff := -1
	offset := 0
	for _, line := range strings.SplitAfter(string(content), "\n") {
		trimmed := strings.TrimRight(line, "\n\r")
		if strings.HasPrefix(trimmed, filewriter.BeginPrefix) {
			name := strings.TrimPrefix(trimmed, filewriter.BeginPrefix)
			if name == GlobalBlockName {
				globalsOff = offset
			} else {
				otherOff = offset
			}
		}
		offset += len(line)
	}
	if globalsOff < 0 {
		return false
	}
	return globalsOff > otherOff
}

// existingGlobalBody returns the current block body under GlobalBlockName, or
// — when that block is absent — under LegacyGlobalBlockName. An empty string
// means "no block yet".
func existingGlobalBody(content []byte) string {
	blocks := filewriter.ListBlocks(content)
	for _, b := range blocks {
		if b.Name == GlobalBlockName {
			return b.Body
		}
	}
	for _, b := range blocks {
		if b.Name == LegacyGlobalBlockName {
			return b.Body
		}
	}
	return ""
}

// globalKV is one parsed directive inside the managed block: the original key
// spelling plus its value.
type globalKV struct {
	key   string
	value string
}

// globalMap is an ordered, case-insensitive key/value table. Order preserves
// first-seen position; a repeated key is updated in place (last value wins), so
// the position of a hand-added key never jumps.
type globalMap struct {
	pairs []globalKV
	index map[string]int
}

func newGlobalMap() *globalMap {
	return &globalMap{index: make(map[string]int)}
}

func (g *globalMap) set(key, value string) {
	lk := strings.ToLower(key)
	if idx, ok := g.index[lk]; ok {
		g.pairs[idx].value = value
		return
	}
	g.index[lk] = len(g.pairs)
	g.pairs = append(g.pairs, globalKV{key: key, value: value})
}

func (g *globalMap) lookup(key string) (string, bool) {
	idx, ok := g.index[strings.ToLower(key)]
	if !ok {
		return "", false
	}
	return g.pairs[idx].value, true
}

// parseGlobalBody parses a managed-block body into an ordered key/value map,
// tolerating both the two-space hostIndent and the four-space form, and
// ignoring the `Host *` and `IgnoreUnknown` lines — they are structure,
// re-rendered below.
func parseGlobalBody(body string) *globalMap {
	m := newGlobalMap()
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 2 {
			continue
		}
		if strings.EqualFold(fields[0], "Host") || strings.EqualFold(fields[0], "IgnoreUnknown") {
			continue
		}
		m.set(fields[0], fields[1])
	}
	return m
}

// renderGlobalBody renders the canonical block body: the guard directive as
// the FIRST line, a blank line, `Host *`, then the merged keys indented with
// hostIndent — ordered by GlobalHostStarOrder with any unrecognised
// pre-existing key appended after them, so a hand-added directive inside the
// block is never dropped.
func renderGlobalBody(m *globalMap) string {
	known := make(map[string]bool, len(GlobalHostStarOrder))
	for _, k := range GlobalHostStarOrder {
		known[strings.ToLower(k)] = true
	}
	var b strings.Builder
	b.WriteString("IgnoreUnknown UseKeychain\n\nHost *\n")
	for _, k := range GlobalHostStarOrder {
		if v, ok := m.lookup(k); ok {
			fmt.Fprintf(&b, "%s%s %s\n", hostIndent, k, v)
		}
	}
	for _, kv := range m.pairs {
		if known[strings.ToLower(kv.key)] {
			continue
		}
		fmt.Fprintf(&b, "%s%s %s\n", hostIndent, kv.key, kv.value)
	}
	return b.String()
}
