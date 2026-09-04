package gitconfig

import (
	"fmt"
	"regexp"
	"strings"

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

// forbiddenSubsectionCharRE matches a double quote, backslash, or newline/
// carriage-return character anywhere in a candidate subsection — any of
// these would corrupt the `[section "subsection"]` header this package
// renders. This is a KEY-SYNTAX guard (RESEARCH Pitfall 3), distinct from
// the VALUE injection guard (validateValue, fragment.go) — the value's
// injection guard has exactly one call site, named in this file's own
// doc comments; this regex is not a second copy of it.
var forbiddenSubsectionCharRE = regexp.MustCompile(`["\\\n\r]`)

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
	if forbiddenCustomValueCharRE.MatchString(value) {
		return fmt.Errorf("gitconfig: %s value %q must not contain a double quote, backslash, '#' or ';' — git would fail to parse or silently truncate the resulting file", key, value)
	}
	if value != strings.TrimSpace(value) {
		return fmt.Errorf("gitconfig: %s value %q must not have leading or trailing whitespace — git strips it on read", key, value)
	}
	return nil
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
// must not contain a double quote, backslash, or newline/carriage return —
// any of those would corrupt the `[section "subsection"]` header this
// package renders. Every rejection names the offending part so a silent
// mis-file (RESEARCH Pitfall 3) is never mistaken for success.
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
	if forbiddenSubsectionCharRE.MatchString(subsection) {
		return "", "", "", fmt.Errorf("gitconfig: subsection %q must not contain a double quote, backslash, or newline", subsection)
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
			fmt.Fprintf(&b, "[%s %q]\n", section, subsection)
		} else {
			fmt.Fprintf(&b, "[%s]\n", section)
		}
		fmt.Fprintf(&b, "\t%s = %s\n", variable, e.Value)
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

// EnsureCustomGitKey upserts key=value into the custom-git-keys managed
// block, mirroring EnsureGlobalGit's shape. The key and value are validated
// FIRST — SplitGitKey for the key's syntax (D-G), validateCustomValue (this
// file, CR-01) for the value's combined injection guard (validateValue,
// RESEARCH Pitfall 4) AND render-syntax guard (a quote/backslash/#/; would
// make the rendered line unparseable or silently truncated by git, and
// leading/trailing whitespace would be silently stripped) — so a malformed
// key or an unrenderable value is rejected before any text is composed. The
// current block is then parsed, the entry
// is upserted case-insensitively by key (git lower-cases keys in --list
// output), and the merged entries are rendered and composed through
// filewriter.ReplaceBlock — the ONE managed-block chokepoint. Foreign
// content outside the block, and every other managed block in existing
// (including the curated global-git block), is preserved verbatim by that
// chokepoint; this function does not re-implement that guarantee.
//
// Re-calling with an unchanged key/value renders byte-identical output to
// existing's current custom-git-keys block, so a caller's byte-equality
// check can skip the write and take no backup (SC-1) — EnsureCustomGitKey
// itself does not perform that check; matching EnsureGlobalGit's contract,
// the caller owns it.
func EnsureCustomGitKey(existing []byte, key, value string) ([]byte, error) {
	if _, _, _, err := SplitGitKey(key); err != nil {
		return nil, fmt.Errorf("gitconfig: EnsureCustomGitKey: %w", err)
	}
	if err := validateCustomValue(key, value); err != nil {
		return nil, fmt.Errorf("gitconfig: EnsureCustomGitKey: %w", err)
	}

	entries := ParseCustomKeysBlock(existing)
	upserted := false
	for i, e := range entries {
		if strings.EqualFold(e.Key, key) {
			entries[i].Value = value
			upserted = true
			break
		}
	}
	if !upserted {
		entries = append(entries, CustomKey{Key: key, Value: value})
	}

	body, err := RenderCustomKeysBlock(entries)
	if err != nil {
		return nil, fmt.Errorf("gitconfig: EnsureCustomGitKey: %w", err)
	}

	return filewriter.ReplaceBlock(existing, CustomGitKeysBlockName, body), nil
}
