package gitconfig

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/castocolina/gitid/internal/filewriter"
)

// CustomGitKeysBlockName is the sentinel name of the gitid-managed block that
// holds every free-form git config key a user has added through the Global
// Git "Set keys" sub-tab (D-03, PROP-03). The full sentinel lines are:
//
//	# BEGIN gitid managed: custom-git-keys
//	# END gitid managed: custom-git-keys
//
// It is deliberately a SEPARATE block from GlobalGitBlockName ("global-git"):
// that block's renderer (RenderBaselineBlock) emits a FIXED, hardcoded
// sequence of section headers, while a free-form custom key may name ANY
// section (including one with a subsection) — a shape the curated renderer
// has no mechanism to emit (D-F, 09.5-03-PLAN.md decision record).
//
// It is registered in IsReservedBlockName for the same reason
// GitFallbackAuthorBlockName is: an unregistered name is one the doctor's
// orphans fix DELETES out from under the next write, fighting the write path
// in a destructive false-positive loop (project learning L4).
const CustomGitKeysBlockName = "custom-git-keys"

// gitConfigNameRE matches a single git config section or variable name
// segment per git's own documented rule (man git-config, CONFIGURATION FILE):
// it must start with an alphabetic character and contain only alphanumeric
// characters or '-'.
var gitConfigNameRE = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9-]*$`)

// validateSubsection rejects any rune the header renderer cannot represent
// losslessly (CR-01 round 2). The renderer escapes ONLY `"` and `\` — the two
// characters git's own subsection grammar recognises as escapes — so any
// other non-printable rune (TAB, DEL, NBSP U+00A0, ZWSP, or any other rune
// `unicode.IsPrint` rejects) must be refused here too, not left to reach the
// renderer. Before this fix the renderer used %q (strconv.Quote), which
// escapes EVERY non-printable rune with a `\xNN`/`\uNNNN` sequence git's
// grammar does not understand — those unknown escapes collapse to their bare
// character on read, silently mis-filing the key under a different name and,
// because ParseCustomKeysBlock reads the mangled subsection back verbatim
// (no unescaping), permanently bricking every future write to this block
// (see this file's package-level CR-01 round-2 notes). This is a KEY-SYNTAX
// guard (RESEARCH Pitfall 3), distinct from the VALUE injection guard
// (validateValue, fragment.go) — the value's injection guard has exactly one
// call site, named in this file's own doc comments; this is not a second
// copy of it.
// subsectionEscaper escapes exactly the two characters git's subsection
// grammar recognises as escapes (`\"` and `\\`) — nothing else. validateSubsection
// rejects every other unescapable rune before this escaper ever runs, so the
// escaped text this produces is always losslessly reversible by git's own
// parser (and by ParseCustomKeysBlock's plain strings.Trim(..., `"`), which
// performs no unescaping — see CR-01 round-2 notes above validateSubsection).
var subsectionEscaper = strings.NewReplacer(`\`, `\\`, `"`, `\"`)

func validateSubsection(sub string) error {
	for _, r := range sub {
		if r == '"' || r == '\\' || !unicode.IsPrint(r) {
			return fmt.Errorf(
				"gitconfig: subsection %q contains %q, which git's subsection grammar cannot "+
					"represent — the key would be silently written under a different name", sub, r)
		}
	}
	return nil
}

// forbiddenCustomValueCharRE matches characters that are structural in git's
// config-file grammar and would make the rendered line unparseable (a double
// quote opens a quoted region; a backslash starts an escape sequence) or
// silently truncate the value ('#' and ';' start a comment). This is a
// VALUE-RENDER guard (CR-01), distinct from validateValue's injection guard
// (newline / "[remote"): validateValue alone is not sufficient for a
// free-form custom key, whose value is rendered UNQUOTED into the file.
var forbiddenCustomValueCharRE = regexp.MustCompile(`["\\#;]`)

// validateCustomValue applies validateValue's existing injection guard and
// then CR-01's render-syntax guard: a value containing '"', '\', '#', or ';'
// would make the rendered `variable = value` line either fail to parse
// (quote/backslash) or be silently truncated by git as a comment (#/;).
// Leading/trailing whitespace is rejected too, since git strips it on read —
// the receipt would then be lying about what was actually written (WR-06).
func validateCustomValue(key, value string) error {
	if err := validateValue(key, value); err != nil {
		return err
	}
	// WR-09 (09.5-REVIEW.md round 2): reject an empty value explicitly,
	// matching globalssh.ValidateDirectiveValue's own explicit reasoning for
	// the SSH sibling — "an empty submission must not silently write a
	// whitespace-only line and take a backup for nothing". Before this fix
	// an empty value was accepted (it contains none of
	// forbiddenCustomValueCharRE's characters and TrimSpace("") == ""), so
	// EnsureCustomGitKey would compose `variable = ` (a trailing space, no
	// value), take a backup, and render a receipt reading "key =  written."
	if value == "" {
		return fmt.Errorf("gitconfig: %s value must not be empty — an empty submission would silently write a whitespace-only line and take a backup for nothing", key)
	}
	if forbiddenCustomValueCharRE.MatchString(value) {
		return fmt.Errorf("gitconfig: %s value %q must not contain a double quote, backslash, '#' or ';' — git would fail to parse or silently truncate the resulting file", key, value)
	}
	if value != strings.TrimSpace(value) {
		return fmt.Errorf("gitconfig: %s value %q must not have leading or trailing whitespace — git strips it on read", key, value)
	}
	return nil
}

// ValidateCustomKeyValue is the exported wrapper around validateCustomValue
// (WR-07). runCustomSSHDirectiveWrite validates both the name and the value
// at its plan stage, before any write; runCustomGitKeyWrite's plan stage
// validated only the key (via SplitGitKey), leaving a malformed value
// undetected until EnsureCustomGitKey at the write stage — by which point
// Write 1 (the [include] floor into ~/.gitconfig) may already have executed
// and taken a timestamped backup, forcing a rollback of a write that should
// never have been attempted. Callers outside this package that need to
// validate a custom git value before composing anything (the plan stage,
// the TUI preview) call this rather than duplicating validateCustomValue's
// character/whitespace rules.
func ValidateCustomKeyValue(key, value string) error {
	return validateCustomValue(key, value)
}

// CustomKey is one free-form git config key=value pair accumulated in the
// custom-git-keys managed block.
type CustomKey struct {
	Key   string
	Value string
}

// SplitGitKey splits a free-form git config key into its section, optional
// subsection, and variable parts, implementing D-G's last-dot rule: the
// variable is everything after the LAST dot; the section path (everything
// before it) is then split on its FIRST dot into section + optional
// subsection, since a subsection may itself legitimately contain dots (e.g.
// "http.https://example.com.sslVerify" -> section "http", subsection
// "https://example.com", variable "sslVerify").
//
// The section and variable must each match gitConfigNameRE. The subsection
// is checked by validateSubsection: it must not contain a double quote,
// backslash, or any non-printable rune (control character, TAB, DEL, NBSP,
// ZWSP, …) — any of those would corrupt the `[section "subsection"]` header
// this package renders, or (CR-01 round 2) render as an escape sequence
// git's own subsection grammar cannot represent. Every rejection names the
// offending part so a silent mis-file (RESEARCH Pitfall 3) is never mistaken
// for success.
func SplitGitKey(key string) (section, subsection, variable string, err error) {
	lastDot := strings.LastIndexByte(key, '.')
	if lastDot == -1 {
		return "", "", "", fmt.Errorf("gitconfig: %q has no dot — expected section.variable or section.subsection.variable", key)
	}

	sectionPath := key[:lastDot]
	variable = key[lastDot+1:]
	if !gitConfigNameRE.MatchString(variable) {
		return "", "", "", fmt.Errorf("gitconfig: variable name %q is invalid — must start with a letter and contain only letters, digits, or '-'", variable)
	}

	if firstDot := strings.IndexByte(sectionPath, '.'); firstDot == -1 {
		section = sectionPath
	} else {
		section = sectionPath[:firstDot]
		subsection = sectionPath[firstDot+1:]
	}
	if !gitConfigNameRE.MatchString(section) {
		return "", "", "", fmt.Errorf("gitconfig: section name %q is invalid — must start with a letter and contain only letters, digits, or '-'", section)
	}
	if err := validateSubsection(subsection); err != nil {
		return "", "", "", err
	}

	return section, subsection, variable, nil
}

// ParseCustomKeysBlock reads the current custom-git-keys block body (if any)
// back into a []CustomKey slice, in file order. It parses only the shape
// RenderCustomKeysBlock writes — a `[section]` or `[section "subsection"]`
// header followed by tab-indented `variable = value` lines — since this
// package owns both the read and write side of this block. Returns nil when
// the block is absent.
func ParseCustomKeysBlock(existing []byte) []CustomKey {
	var body string
	found := false
	for _, b := range filewriter.ListBlocks(existing) {
		if b.Name == CustomGitKeysBlockName {
			body = b.Body
			found = true
			break
		}
	}
	if !found {
		return nil
	}

	var entries []CustomKey
	var section, subsection string
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// Section header: not indented, bracketed.
		if !strings.HasPrefix(line, "\t") && strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			header := trimmed[1 : len(trimmed)-1]
			if idx := strings.IndexByte(header, ' '); idx != -1 {
				section = header[:idx]
				subsection = strings.Trim(strings.TrimSpace(header[idx+1:]), `"`)
			} else {
				section = header
				subsection = ""
			}
			continue
		}
		// Key-value: split on the first "=" only.
		if eq := strings.Index(trimmed, "="); eq != -1 && section != "" {
			variable := strings.TrimSpace(trimmed[:eq])
			value := strings.TrimSpace(trimmed[eq+1:])
			fullKey := section
			if subsection != "" {
				fullKey += "." + subsection
			}
			fullKey += "." + variable
			entries = append(entries, CustomKey{Key: fullKey, Value: value})
		}
	}
	return entries
}

// RenderCustomKeysBlock renders entries into the custom-git-keys block body:
// per entry, a section header (`[section]` or `[section "subsection"]`, per
// SplitGitKey's parse of the entry's Key) followed by a tab-indented
// `variable = value` line, matching RenderBaselineBlock's tab-indentation
// convention. Two entries in the same section each render their own header —
// git merges repeated section headers at read time (D-F), so correctness
// does not depend on de-duplicating them here.
//
// An entry whose Key fails SplitGitKey, or whose Value fails validateValue,
// makes the whole render fail with a named error rather than silently
// emitting a partial or corrupted block.
func RenderCustomKeysBlock(entries []CustomKey) (string, error) {
	var b strings.Builder
	for _, e := range entries {
		section, subsection, variable, err := SplitGitKey(e.Key)
		if err != nil {
			return "", fmt.Errorf("gitconfig: RenderCustomKeysBlock: %w", err)
		}
		if err := validateCustomValue(e.Key, e.Value); err != nil {
			return "", fmt.Errorf("gitconfig: RenderCustomKeysBlock: %w", err)
		}
		if subsection != "" {
			// CR-01 round 2: never use %q (strconv.Quote) here — it escapes
			// every rune unicode.IsPrint rejects, using \xNN/\uNNNN
			// sequences git's subsection grammar does not understand (git
			// recognises only \" and \\). validateSubsection above already
			// guarantees subsection contains neither '"' nor '\\' unescaped
			// on its own, so this replacer only ever has to escape those
			// two — it can never introduce an escape ParseCustomKeysBlock's
			// unescape-free read-back cannot undo.
			escaped := subsectionEscaper.Replace(subsection)
			fmt.Fprintf(&b, "[%s \"%s\"]\n", section, escaped)
		} else {
			fmt.Fprintf(&b, "[%s]\n", section)
		}
		fmt.Fprintf(&b, "\t%s = %s\n", variable, e.Value)
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

// gitKeysEqual reports whether two dotted git config keys name the SAME
// key, per git-config(1)'s CONFIGURATION FILE rule: section and variable
// names are case-INSENSITIVE, but a SUBSECTION is case-SENSITIVE (WR-05).
// A naive strings.EqualFold over the whole dotted key is wrong for the
// middle segment: "http.https://Example.com.sslVerify" and
// "http.https://example.com.sslVerify" are two genuinely DIFFERENT git
// keys, not the same key spelled two ways. A key that fails SplitGitKey
// compares unequal to everything (SplitGitKey is re-validated by every
// caller before this function is reached, so this is a defensive default,
// never the deciding branch in practice).
func gitKeysEqual(a, b string) bool {
	secA, subA, varA, errA := SplitGitKey(a)
	secB, subB, varB, errB := SplitGitKey(b)
	if errA != nil || errB != nil {
		return false
	}
	return strings.EqualFold(secA, secB) && subA == subB && strings.EqualFold(varA, varB)
}

// EnsureCustomGitKey upserts key=value into the custom-git-keys managed
// block, mirroring EnsureGlobalGit's shape. The key and value are validated
// FIRST — SplitGitKey for the key's syntax (D-G), validateCustomValue (this
// file, CR-01) for the value's combined injection guard (validateValue,
// RESEARCH Pitfall 4) AND render-syntax guard (a quote/backslash/#/; would
// make the rendered line unparseable or silently truncated by git, and
// leading/trailing whitespace would be silently stripped) — so a malformed
// key or an unrenderable value is rejected before any text is composed. The
// current block is then parsed, the entry is upserted via gitKeysEqual
// (WR-05: section/variable case-insensitive, subsection case-sensitive —
// NOT a blanket case-insensitive fold over the whole dotted key), and the
// merged entries are rendered and composed through filewriter.ReplaceBlock
// — the ONE managed-block chokepoint. Foreign content outside the block,
// and every other managed block in existing (including the curated
// global-git block), is preserved verbatim by that chokepoint; this
// function does not re-implement that guarantee.
//
// Re-calling with an unchanged key/value renders byte-identical output to
// existing's current custom-git-keys block, so a caller's byte-equality
// check can skip the write and take no backup (SC-1) — EnsureCustomGitKey
// itself does not perform that check; matching EnsureGlobalGit's contract,
// the caller owns it.
//
// WR-06 (09.5-REVIEW.md round 2): one entry that cannot round-trip re-render
// (a hand edit, a merge conflict, a stale CR-01-vintage subsection, a future
// format change) is DROPPED and named in the returned skipped slice, rather
// than aborting the entire write — this mirrors EnsureGlobalGit's own
// established policy of dropping a bad adopted value instead of refusing to
// write ("we must not refuse to write just because it had a quirky value").
// The upserted key ITSELF still fails closed: SplitGitKey/validateCustomValue
// run on key/value above, BEFORE any entry is parsed or filtered, so the
// entry this call just upserted can never be the one the filter below drops.
func EnsureCustomGitKey(existing []byte, key, value string) ([]byte, []string, error) {
	if _, _, _, err := SplitGitKey(key); err != nil {
		return nil, nil, fmt.Errorf("gitconfig: EnsureCustomGitKey: %w", err)
	}
	if err := validateCustomValue(key, value); err != nil {
		return nil, nil, fmt.Errorf("gitconfig: EnsureCustomGitKey: %w", err)
	}

	entries := ParseCustomKeysBlock(existing)
	upserted := false
	for i, e := range entries {
		if gitKeysEqual(e.Key, key) {
			entries[i].Value = value
			upserted = true
			break
		}
	}
	if !upserted {
		entries = append(entries, CustomKey{Key: key, Value: value})
	}

	var skipped []string
	renderable := make([]CustomKey, 0, len(entries))
	for _, e := range entries {
		if _, _, _, err := SplitGitKey(e.Key); err != nil {
			skipped = append(skipped, fmt.Sprintf("%s (%v)", e.Key, err))
			continue
		}
		if err := validateCustomValue(e.Key, e.Value); err != nil {
			skipped = append(skipped, fmt.Sprintf("%s (%v)", e.Key, err))
			continue
		}
		renderable = append(renderable, e)
	}

	body, err := RenderCustomKeysBlock(renderable)
	if err != nil {
		// renderable was pre-filtered above, so this should be unreachable —
		// kept as a fail-closed backstop rather than assuming the filter
		// above is exhaustive.
		return nil, nil, fmt.Errorf("gitconfig: EnsureCustomGitKey: %w", err)
	}

	return filewriter.ReplaceBlock(existing, CustomGitKeysBlockName, body), skipped, nil
}
