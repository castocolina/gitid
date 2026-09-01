package gitconfig

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/castocolina/gitid/internal/filewriter"
)

// ManagedBlockShape is the healthy-file classification InspectManagedBlockFile
// returns: no block, or exactly one complete block of the requested name.
type ManagedBlockShape struct {
	Managed bool
	Body    string
}

// ManagedBlockErrorReason classifies a malformed managed-block shape into one
// of the reason categories the Global Git Ignore screen's frozen copy uses
// (09.2-UI-SPEC.md's "Malformed file refusal" row). This package stays
// UI-free — it never spells out user-facing wording, only the category and
// the offending line, so a caller can translate without parsing a sentence.
type ManagedBlockErrorReason int

const (
	// ManagedBlockReasonUnclosedMarker covers an opening marker with no
	// matching closing marker (orphan BEGIN, or a second BEGIN of the same
	// name appearing before the first is closed).
	ManagedBlockReasonUnclosedMarker ManagedBlockErrorReason = iota
	// ManagedBlockReasonMismatchedMarker covers a closing marker that
	// doesn't match its opening marker (a standalone END with no open
	// marker, or an END whose name disagrees with the BEGIN it closes).
	ManagedBlockReasonMismatchedMarker
	// ManagedBlockReasonDuplicateBlock covers two complete blocks of the
	// same name in one file.
	ManagedBlockReasonDuplicateBlock
)

// ManagedBlockError is a structured error from InspectManagedBlockFile (and
// its InspectGitignoreFile wrapper). Error() preserves the original
// developer-facing diagnostic text for logs and tests; Line and Reason let a
// UI layer translate the error into its own frozen, user-facing copy instead
// of rendering this sentence verbatim.
type ManagedBlockError struct {
	Line   int
	Reason ManagedBlockErrorReason
	text   string
}

func (e *ManagedBlockError) Error() string { return e.text }

func newManagedBlockError(line int, reason ManagedBlockErrorReason, format string, args ...any) *ManagedBlockError {
	return &ManagedBlockError{Line: line, Reason: reason, text: fmt.Sprintf(format, args...)}
}

// SentinelLineError is NormalizeGitignoreLines's structured error for a
// user-typed or pasted line that collides with a managed-block sentinel.
// Error() preserves the original diagnostic text; Line lets a UI layer
// translate the error into its own frozen, user-facing copy.
type SentinelLineError struct {
	Line int
	text string
}

func (e *SentinelLineError) Error() string { return e.text }

// BaselineState holds the reconstructed managed baseline across all three
// managed surfaces. It is a value type (no pointer), following the FragmentInfo
// / IncludeIfInfo precedent in reader.go.
type BaselineState struct {
	// Installed is true when both the include block in ~/.gitconfig AND the
	// baseline block in ~/.gitconfig.d/00-baseline exist.
	Installed bool
	// Incomplete is true when some-but-not-all required artifacts are present.
	Incomplete bool
	// Missing lists the artifact description(s) that are absent (for show output).
	Missing []string
	// BaselineKeys maps lowercased section.key to the value found in the
	// managed baseline block body (e.g. "core.ignorecase" → "false").
	BaselineKeys map[string]string
	// URLRewrites is the list of active HTTPS→SSH mappings from the
	// url-rewrites managed block, in file order.
	URLRewrites []URLRewrite
	// GitignorePatterns is the list of non-empty pattern lines from the
	// managed gitignore block, in file order.
	GitignorePatterns []string
}

// RemoveURLRewritesBlock removes the "url-rewrites" managed block from
// baselineFilePath independently, leaving all other content (including the
// "baseline" block and any foreign content) intact (D-07). Idempotent when the
// block is absent. Returns the backup path from filewriter.Write.
func RemoveURLRewritesBlock(baselineFilePath string) (backupPath string, err error) {
	existing, readErr := os.ReadFile(baselineFilePath) //nolint:gosec // baselineFilePath is a trusted gitid-managed path (G304)
	if os.IsNotExist(readErr) {
		return "", nil // nothing to remove — idempotent no-op
	}
	if readErr != nil {
		return "", fmt.Errorf("reading %s: %w", baselineFilePath, readErr)
	}

	composed := filewriter.RemoveBlock(existing, "url-rewrites")
	bp, writeErr := filewriter.Write(baselineFilePath, composed, gitconfigMode)
	if writeErr != nil {
		return "", fmt.Errorf("writing %s after url-rewrites removal: %w", baselineFilePath, writeErr)
	}
	return bp, nil
}

// ReadBaselineState reconstructs the managed baseline state from the three
// disk files with no sidecar DB (IDENT-07 model, SC-5). Missing files are
// treated as empty (no error). It checks:
//   - gitconfigPath for the "baseline-include" block
//   - baselineFilePath for the "baseline" and "url-rewrites" blocks
//   - gitignorePath for the "gitignore" block
//
// Installed=true only when both the include block and the baseline block exist.
// Incomplete=true when some-but-not-all artifacts are present.
func ReadBaselineState(gitconfigPath, baselineFilePath, gitignorePath string) (BaselineState, error) {
	// Read each file; missing files are treated as empty bytes (not an error).
	gitconfigContent := readFileSilent(gitconfigPath)
	baselineContent := readFileSilent(baselineFilePath)
	gitignoreContent := readFileSilent(gitignorePath)

	// Extract managed blocks from each file.
	gitconfigBlocks := indexBlocks(filewriter.ListBlocks(gitconfigContent))
	baselineBlocks := indexBlocks(filewriter.ListBlocks(baselineContent))
	gitignoreBlocks := indexBlocks(filewriter.ListBlocks(gitignoreContent))

	_, includeBlockExists := gitconfigBlocks["baseline-include"]
	_, baselineBlockExists := baselineBlocks["baseline"]

	var state BaselineState

	// Determine installed / incomplete state.
	switch {
	case includeBlockExists && baselineBlockExists:
		state.Installed = true
	case !includeBlockExists && !baselineBlockExists:
		// Not installed — return zero state.
		return state, nil
	default:
		// Some-but-not-all artifacts are present.
		state.Incomplete = true
		if !includeBlockExists {
			state.Missing = append(state.Missing, "include block in "+gitconfigPath)
		}
		if !baselineBlockExists {
			state.Missing = append(state.Missing, baselineFilePath)
		}
		return state, nil
	}

	// Parse baseline keys from the baseline block body.
	if b, ok := baselineBlocks["baseline"]; ok {
		state.BaselineKeys = parseGitconfigBlockBody(b.Body)
	}

	// Parse url-rewrites from the url-rewrites block body.
	if b, ok := baselineBlocks["url-rewrites"]; ok {
		state.URLRewrites = parseURLRewritesBlockBody(b.Body)
	}

	// Parse gitignore patterns from the gitignore block body.
	if b, ok := gitignoreBlocks["gitignore"]; ok {
		state.GitignorePatterns = parseGitignoreBlockBody(b.Body)
	}

	return state, nil
}

// readFileSilent reads a file, returning nil bytes (not an error) when the
// file does not exist. Other errors are silently treated as empty too — the
// caller checks block presence to determine state.
func readFileSilent(path string) []byte {
	content, err := os.ReadFile(path) //nolint:gosec // path is a trusted gitid-managed path (G304)
	if err != nil {
		return nil
	}
	return content
}

// indexBlocks converts a NamedBlock slice into a map keyed by block name.
func indexBlocks(blocks []filewriter.NamedBlock) map[string]filewriter.NamedBlock {
	m := make(map[string]filewriter.NamedBlock, len(blocks))
	for _, b := range blocks {
		m[b.Name] = b
	}
	return m
}

// parseGitconfigBlockBody parses a baseline block body (tab-indented gitconfig
// format) into a lowercase section.key→value map using a simple line scanner.
// The section header tracks current context; key=value pairs are accumulated
// under "section.key".
//
// Section headers are only recognised at indent level zero (no leading tab) so
// that an alias value like `!f() { x = y; }; f` is never mis-parsed as a
// section. Key–value splitting uses the first "=" only (strings.Index) so
// values that contain " = " (e.g. complex aliases) are preserved verbatim.
func parseGitconfigBlockBody(body string) map[string]string {
	result := make(map[string]string)
	var section string
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		// Section header: must NOT start with a tab (not an indented key line)
		// and must be bracketed, e.g. [core] or [alias].
		if !strings.HasPrefix(line, "\t") && strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			section = strings.ToLower(trimmed[1 : len(trimmed)-1])
			continue
		}
		// Key–value: split on the first "=" only so values containing " = " are
		// preserved (e.g. alias.lg with a complex format string).
		if eq := strings.Index(trimmed, "="); eq != -1 && section != "" {
			key := strings.ToLower(section + "." + strings.TrimSpace(trimmed[:eq]))
			result[key] = strings.TrimSpace(trimmed[eq+1:])
		}
	}
	return result
}

// parseURLRewritesBlockBody parses a url-rewrites block body into a slice of
// URLRewrite pairs. It looks for [url "git@..."] section headers followed by
// `insteadOf = https://...` key lines.
func parseURLRewritesBlockBody(body string) []URLRewrite {
	var rewrites []URLRewrite
	var currentSSH string
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		// [url "git@github.com:"] — extract the SSH prefix
		if strings.HasPrefix(trimmed, `[url "`) && strings.HasSuffix(trimmed, `"]`) {
			currentSSH = trimmed[len(`[url "`) : len(trimmed)-len(`"]`)]
			continue
		}
		// \tinsteadOf = https://github.com/
		kv := strings.SplitN(trimmed, " = ", 2)
		if len(kv) == 2 && strings.ToLower(kv[0]) == "insteadof" && currentSSH != "" {
			rewrites = append(rewrites, URLRewrite{
				HTTPSPrefix: strings.TrimSpace(kv[1]),
				SSHPrefix:   currentSSH,
			})
			currentSSH = ""
		}
	}
	return rewrites
}

// parseGitignoreBlockBody parses a gitignore block body into a slice of
// non-empty, non-comment pattern lines.
func parseGitignoreBlockBody(body string) []string {
	var patterns []string
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			patterns = append(patterns, trimmed)
		}
	}
	return patterns
}

// BaselineConfig holds the Tier-2 (optional) fields for the baseline block.
// Tier-1 keys (ignorecase, excludesfile, push.autoSetupRemote, pull.rebase,
// fetch.prune, color.ui) are unconditional and never gated.
type BaselineConfig struct {
	// AutoCRLF controls whether core.autocrlf = input is written (Tier-2).
	AutoCRLF bool
	// Pager is the value for core.pager (Tier-2). Empty string omits the key.
	Pager string
	// ExtraColors controls whether color.branch/diff/status=auto are written.
	ExtraColors bool
	// DiffColorMoved controls whether diff.colorMoved=zebra is written (Tier-2).
	DiffColorMoved bool
	// MergeConflictStyle is the value for merge.conflictstyle (e.g. "zdiff3").
	// Empty string omits the [merge] section entirely (C4 git-version gate).
	MergeConflictStyle string
	// InitDefaultBranch is the value for init.defaultBranch (Tier-2).
	// Empty string omits the [init] section.
	InitDefaultBranch string
	// IncludeAliases controls whether the [alias] section is written (Tier-2).
	IncludeAliases bool
}

// DefaultBaselineConfig returns a BaselineConfig with all Tier-2 options
// enabled and set to the values from the gist reference (D-04).
func DefaultBaselineConfig() BaselineConfig {
	return BaselineConfig{
		AutoCRLF:           true,
		Pager:              "less -FRX",
		ExtraColors:        true,
		DiffColorMoved:     true,
		MergeConflictStyle: "zdiff3",
		InitDefaultBranch:  "main",
		IncludeAliases:     true,
	}
}

// URLRewrite is one HTTPS→SSH insteadOf mapping. HTTPSPrefix is the HTTPS URL
// prefix (e.g. "https://github.com/") and SSHPrefix is the replacement SSH
// target (e.g. "git@github.com:").
type URLRewrite struct {
	HTTPSPrefix string
	SSHPrefix   string
}

// DefaultURLRewrites returns the three big-three HTTPS→SSH mappings (D-05/D-06).
// Order is fixed: github.com, gitlab.com, bitbucket.org (determinism, Pitfall D).
func DefaultURLRewrites() []URLRewrite {
	return []URLRewrite{
		{HTTPSPrefix: "https://github.com/", SSHPrefix: "git@github.com:"},
		{HTTPSPrefix: "https://gitlab.com/", SSHPrefix: "git@gitlab.com:"},
		{HTTPSPrefix: "https://bitbucket.org/", SSHPrefix: "git@bitbucket.org:"},
	}
}

// DefaultGitignorePatterns returns the curated gitignore seed list with comment headers,
// grouped by category for readability. This slice contains both pattern entries and comment
// lines (those starting with "#" after trimming whitespace). The byte-stability contract
// (Pitfall D) demands fixed order. For comparison against what ReadBaselineState returns,
// callers must use DefaultGitignoreEntries, which is the comment-free and blank-free view.
//
// The first six patterns (.DS_Store, Thumbs.db, *.log, *.bak, *.tmp, *.swp) are SC-2-locked;
// the remaining are planner discretion (D-Claude). Project-scoped build output (such as
// distribution or build directories) is deliberately NOT seeded — a global ignore that hides
// those would silently un-track them in repositories that legitimately commit them; the
// content is user-editable now, so anyone who wants them adds them on the screen.
//
// Scratch-directory entries (tmp/, .tmp/) are a deliberate, narrower exception to the
// above rule: the user asked for global scratch-directory ignoring by name in GIGN-01,
// and a directory of that name is conventionally machine-local scratch, whereas a
// distribution or build directory is routinely committed.
func DefaultGitignorePatterns() []string {
	return []string{
		"# OS artifacts",
		".DS_Store",
		"Thumbs.db",
		"desktop.ini",
		"# Editors and IDEs",
		".idea/",
		".vscode/",
		"*.swp",
		"*.swo",
		"*~",
		"# Logs, temp and scratch",
		"*.log",
		"*.bak",
		"*.tmp",
		"tmp/",
		".tmp/",
		"# Environment files (committed examples stay tracked)",
		".env",
		".env.*",
		"!.env.example",
		"# Python",
		"__pycache__/",
		"*.pyc",
		".venv/",
		"venv/",
		"# Node",
		"node_modules/",
		"# Tooling caches",
		".direnv/",
		".pytest_cache/",
		".mypy_cache/",
		".ruff_cache/",
	}
}

// DefaultGitignoreEntries returns the ordered subset of DefaultGitignorePatterns
// with blank lines and comment lines (those whose trimmed form starts with "#") removed.
// This is the view callers must use when comparing against what ReadBaselineState's
// GitignorePatterns returns — the two use identical filtering logic (parseGitignoreBlockBody),
// so they can never disagree about the rendered content.
func DefaultGitignoreEntries() []string {
	patterns := DefaultGitignorePatterns()
	var entries []string
	for _, p := range patterns {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			entries = append(entries, trimmed)
		}
	}
	return entries
}

// RenderBaselineBlock renders the baseline gitconfig block body with fixed
// section ordering ([core],[push],[pull],[fetch],[color],[diff],[merge],[init],
// [alias]) and tab-prefixed keys — matching the SC-1 idempotency contract and
// RESEARCH Example 1. Calling it twice with the same cfg yields identical bytes.
//
// Tier-1 keys (ignorecase, excludesfile, push.autoSetupRemote, pull.rebase,
// fetch.prune, color.ui) are always written. Tier-2 keys are gated on cfg
// fields. No user section is ever emitted (D-04b). core.editor is never
// seeded (D-12).
//
// Any user-supplied string in cfg (Pager, MergeConflictStyle, InitDefaultBranch)
// is validated with validateValue before render; an invalid value returns an error
// so callers can surface it cleanly instead of crashing on panic (WR-03).
func RenderBaselineBlock(cfg BaselineConfig) (string, error) {
	// Validate user-supplied Tier-2 strings before rendering (V5 injection guard).
	if cfg.Pager != "" {
		if err := validateValue("core.pager", cfg.Pager); err != nil {
			return "", fmt.Errorf("gitconfig: RenderBaselineBlock: %w", err)
		}
	}
	if cfg.MergeConflictStyle != "" {
		if err := validateValue("merge.conflictstyle", cfg.MergeConflictStyle); err != nil {
			return "", fmt.Errorf("gitconfig: RenderBaselineBlock: %w", err)
		}
	}
	if cfg.InitDefaultBranch != "" {
		if err := validateValue("init.defaultBranch", cfg.InitDefaultBranch); err != nil {
			return "", fmt.Errorf("gitconfig: RenderBaselineBlock: %w", err)
		}
	}

	var b strings.Builder

	// [core] — Tier-1 keys always first, Tier-2 (autocrlf, pager) conditional.
	fmt.Fprintf(&b, "[core]\n")
	fmt.Fprintf(&b, "\tignorecase = false\n")
	fmt.Fprintf(&b, "\texcludesfile = ~/.gitignore_global\n")
	if cfg.AutoCRLF {
		fmt.Fprintf(&b, "\tautocrlf = input\n")
	}
	if cfg.Pager != "" {
		fmt.Fprintf(&b, "\tpager = %s\n", cfg.Pager)
	}

	// [push] — Tier-1
	fmt.Fprintf(&b, "[push]\n")
	fmt.Fprintf(&b, "\tautoSetupRemote = true\n")

	// [pull] — Tier-1
	fmt.Fprintf(&b, "[pull]\n")
	fmt.Fprintf(&b, "\trebase = true\n")

	// [fetch] — Tier-1
	fmt.Fprintf(&b, "[fetch]\n")
	fmt.Fprintf(&b, "\tprune = true\n")

	// [color] — ui is Tier-1; branch/diff/status are Tier-2
	fmt.Fprintf(&b, "[color]\n")
	fmt.Fprintf(&b, "\tui = auto\n")
	if cfg.ExtraColors {
		fmt.Fprintf(&b, "\tbranch = auto\n")
		fmt.Fprintf(&b, "\tdiff = auto\n")
		fmt.Fprintf(&b, "\tstatus = auto\n")
	}

	// [diff] — Tier-2
	if cfg.DiffColorMoved {
		fmt.Fprintf(&b, "[diff]\n")
		fmt.Fprintf(&b, "\tcolorMoved = zebra\n")
	}

	// [merge] — Tier-2; omit entirely when MergeConflictStyle is empty (C4 gate)
	if cfg.MergeConflictStyle != "" {
		fmt.Fprintf(&b, "[merge]\n")
		fmt.Fprintf(&b, "\tconflictstyle = %s\n", cfg.MergeConflictStyle)
	}

	// [init] — Tier-2
	if cfg.InitDefaultBranch != "" {
		fmt.Fprintf(&b, "[init]\n")
		fmt.Fprintf(&b, "\tdefaultBranch = %s\n", cfg.InitDefaultBranch)
	}

	// [alias] — Tier-2; fixed alias order from D-04a / gist reference.
	if cfg.IncludeAliases {
		fmt.Fprintf(&b, "[alias]\n")
		fmt.Fprintf(&b, "\tst = status\n")
		fmt.Fprintf(&b, "\tco = checkout\n")
		fmt.Fprintf(&b, "\tbr = branch\n")
		fmt.Fprintf(&b, "\tci = commit\n")
		fmt.Fprintf(&b, "\tdf = diff\n")
		fmt.Fprintf(&b, "\tlg = log --graph --pretty=format:'%%Cred%%h%%Creset -%%C(yellow)%%d%%Creset %%s %%Cgreen(%%cr) %%C(bold blue)<%%an>%%Creset' --abbrev-commit\n")
		fmt.Fprintf(&b, "\tunstage = reset HEAD --\n")
		fmt.Fprintf(&b, "\tlast = log -1 HEAD\n")
	}

	return strings.TrimRight(b.String(), "\n"), nil
}

// RenderURLRewritesBlock renders the url-rewrites block body for the given
// insteadOf mappings. The section order matches the input slice order (callers
// use DefaultURLRewrites for the canonical big-three order). Each URL/SSH prefix
// pair is validated with validateValue before render to guard against newline
// injection. An empty rewrites slice returns ("", nil). An invalid value returns
// ("", error) so callers can surface it cleanly instead of crashing on panic (WR-03).
func RenderURLRewritesBlock(rewrites []URLRewrite) (string, error) {
	if len(rewrites) == 0 {
		return "", nil
	}

	var b strings.Builder
	for _, r := range rewrites {
		// Validate user-supplied URL strings (V5 injection guard).
		if err := validateValue("url.insteadOf.httpsPrefix", r.HTTPSPrefix); err != nil {
			return "", fmt.Errorf("gitconfig: RenderURLRewritesBlock: %w", err)
		}
		if err := validateValue("url.insteadOf.sshPrefix", r.SSHPrefix); err != nil {
			return "", fmt.Errorf("gitconfig: RenderURLRewritesBlock: %w", err)
		}
		fmt.Fprintf(&b, "[url %q]\n", r.SSHPrefix)
		fmt.Fprintf(&b, "\tinsteadOf = %s\n", r.HTTPSPrefix)
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

// RenderGitignoreBlock renders the gitignore block body — one pattern per line
// in the fixed order of the patterns slice. An empty slice returns an empty
// string. The order must match DefaultGitignorePatterns for SC-2 compliance
// and byte-stability (Pitfall D).
func RenderGitignoreBlock(patterns []string) string {
	if len(patterns) == 0 {
		return ""
	}
	var b strings.Builder
	for _, p := range patterns {
		fmt.Fprintf(&b, "%s\n", p)
	}
	return strings.TrimRight(b.String(), "\n")
}

// WriteBaselineFile composes the baseline and url-rewrites managed blocks into
// baselineFilePath through the filewriter chokepoint. It creates the file's
// parent directory (mode 0700) if it does not already exist, consistent with
// WriteFragment's EnsureDir pattern. The baseline block is always written; the
// url-rewrites block is written when len(rewrites) > 0 and removed otherwise
// (D-07 independent toggling). Foreign content outside either block is preserved
// verbatim (D-02). It returns the backup path (empty when the file is new).
// When the composed content is byte-identical to the existing file, the write is
// skipped and an empty backup path is returned (SC-1 idempotency).
func WriteBaselineFile(baselineFilePath string, cfg BaselineConfig, rewrites []URLRewrite) (string, error) {
	if err := filewriter.EnsureDir(filepath.Dir(baselineFilePath), 0o700); err != nil {
		return "", fmt.Errorf("ensuring baseline dir: %w", err)
	}

	existing, err := os.ReadFile(baselineFilePath) //nolint:gosec // baselineFilePath is a trusted gitid-managed path
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("reading %s: %w", baselineFilePath, err)
	}

	baselineBlock, err := RenderBaselineBlock(cfg)
	if err != nil {
		return "", fmt.Errorf("rendering baseline block: %w", err)
	}
	composed := filewriter.ReplaceBlock(existing, "baseline", baselineBlock)
	if len(rewrites) > 0 {
		rewritesBlock, rerr := RenderURLRewritesBlock(rewrites)
		if rerr != nil {
			return "", fmt.Errorf("rendering url-rewrites block: %w", rerr)
		}
		composed = filewriter.ReplaceBlock(composed, "url-rewrites", rewritesBlock)
	} else {
		composed = filewriter.RemoveBlock(composed, "url-rewrites")
	}

	// SC-1 idempotency: skip write (and backup) when content is unchanged.
	if bytes.Equal(composed, existing) {
		return "", nil
	}

	backupPath, err := filewriter.Write(baselineFilePath, composed, gitconfigMode)
	if err != nil {
		return "", fmt.Errorf("writing baseline block to %s: %w", baselineFilePath, err)
	}
	return backupPath, nil
}

// NormalizeGitignoreLines splits content on newlines (tolerating carriage
// returns), right-trims each line, drops the trailing run of empty lines, and
// keeps interior blanks and comment lines. A trimmed line that starts with a
// managed-block sentinel prefix filewriter owns is refused with an error that
// names the offending line number — a nested sentinel would corrupt block
// parsing (T-09.2-01).
func NormalizeGitignoreLines(content string) ([]string, error) {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	raw := strings.Split(normalized, "\n")
	lines := make([]string, 0, len(raw))
	for i, line := range raw {
		trimmed := strings.TrimRight(line, " \t")
		if strings.HasPrefix(trimmed, filewriter.BeginPrefix) || strings.HasPrefix(trimmed, filewriter.EndPrefix) {
			lineNo := i + 1
			return nil, &SentinelLineError{
				Line: lineNo,
				text: fmt.Sprintf("line %d looks like a gitid managed-block sentinel and cannot be part of gitignore content", lineNo),
			}
		}
		lines = append(lines, trimmed)
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines, nil
}

// ComposeGlobalGitignore is the ONE place the gitignore managed block is
// composed. Any caller that needs to show a candidate file must call this and
// nothing else, so the review preview and the commit can never diverge.
func ComposeGlobalGitignore(existing []byte, patterns []string) []byte {
	return filewriter.ReplaceBlock(existing, "gitignore", RenderGitignoreBlock(patterns))
}

// InspectManagedBlockFile scans content for the managed BEGIN and END sentinels
// of blockName and classifies the file before anything treats it as readable or
// writable. Healthy cases (no block; exactly one complete block of that name)
// return the block's body and a Managed flag. Each of the five malformed
// shapes — orphan BEGIN, standalone END, nested BEGIN, mismatched END name,
// and a second complete block of the requested name — returns an error naming
// the offending line number and telling the user the file must be repaired by
// hand. Blocks of OTHER names are foreign content and are neither counted nor
// validated.
//
// Each returned *ManagedBlockError also carries a ManagedBlockErrorReason so
// a caller can translate the error into user-facing copy without parsing the
// diagnostic sentence. The reason categories are coarser than the five
// shapes above: orphan BEGIN and nested BEGIN both classify as
// ManagedBlockReasonUnclosedMarker (an END naming a DIFFERENT block while
// ours is open is treated as foreign and skipped, so ours surfaces as
// unclosed at end-of-scan rather than as a name mismatch); standalone END
// and an END naming OUR block while a different block is open both classify
// as ManagedBlockReasonMismatchedMarker; two complete blocks classifies as
// ManagedBlockReasonDuplicateBlock.
//
// The function is name-parameterized rather than gitignore-specific because
// the same write-divergence hazard exists on the baseline fragment: indexBlocks
// keeps the LAST duplicate while replaceBlockWith claims the FIRST. Failing
// closed on the file's SHAPE before either the read or the write is the fix;
// replaceBlockWith's selection rule is the shared core behind every write path
// and is out of this phase's scope to flip.
func InspectManagedBlockFile(content []byte, blockName string) (ManagedBlockShape, error) {
	normalized := bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))
	lines := strings.Split(string(normalized), "\n")
	if len(lines) == 1 && lines[0] == "" && len(content) == 0 {
		return ManagedBlockShape{}, nil
	}

	openAt := -1
	openName := ""
	var found *ManagedBlockShape

	for i, line := range lines {
		trimmed := strings.TrimRight(line, "\r")
		lineNo := i + 1
		switch {
		case strings.HasPrefix(trimmed, filewriter.BeginPrefix):
			name := strings.TrimPrefix(trimmed, filewriter.BeginPrefix)
			if openAt != -1 {
				if name == blockName && openName == blockName {
					return ManagedBlockShape{}, newManagedBlockError(lineNo, ManagedBlockReasonUnclosedMarker,
						"line %d: nested BEGIN sentinel — repair the file by hand before gitid will touch it", lineNo)
				}
				continue
			}
			openAt = i
			openName = name
		case strings.HasPrefix(trimmed, filewriter.EndPrefix):
			name := strings.TrimPrefix(trimmed, filewriter.EndPrefix)
			if openAt == -1 {
				if name == blockName {
					return ManagedBlockShape{}, newManagedBlockError(lineNo, ManagedBlockReasonMismatchedMarker,
						"line %d: standalone END sentinel — repair the file by hand before gitid will touch it", lineNo)
				}
				continue
			}
			if name != openName {
				// name != openName makes "openName == blockName && name ==
				// blockName" impossible (that would require name ==
				// openName) — that branch was dead code and has been
				// removed (09.2-REVIEW.md WR-01). The two shapes that
				// remain: an END naming some OTHER block while ours is
				// still open is foreign and is skipped, leaving ours to
				// surface as an orphan/unclosed BEGIN at end-of-scan; an
				// END naming OUR block while a DIFFERENT block is open is
				// the one reachable "closing marker that doesn't match its
				// opening marker" shape.
				if openName == blockName {
					continue
				}
				if name == blockName {
					return ManagedBlockShape{}, newManagedBlockError(lineNo, ManagedBlockReasonMismatchedMarker,
						"line %d: END sentinel name does not match its open BEGIN — repair the file by hand before gitid will touch it", lineNo)
				}
				continue
			}
			if openName == blockName {
				if found != nil {
					return ManagedBlockShape{}, newManagedBlockError(lineNo, ManagedBlockReasonDuplicateBlock,
						"line %d: two complete %q blocks in one file — repair the file by hand before gitid will touch it", lineNo, blockName)
				}
				body := strings.Join(lines[openAt+1:i], "\n")
				body = strings.TrimRight(body, "\n")
				found = &ManagedBlockShape{Managed: true, Body: body}
			}
			openAt = -1
			openName = ""
		}
	}
	if openAt != -1 && openName == blockName {
		lineNo := openAt + 1
		return ManagedBlockShape{}, newManagedBlockError(lineNo, ManagedBlockReasonUnclosedMarker,
			"line %d: orphan BEGIN sentinel with no END — repair the file by hand before gitid will touch it", lineNo)
	}
	if found == nil {
		return ManagedBlockShape{}, nil
	}
	return *found, nil
}

// InspectGitignoreFile is the gitignore-named convenience wrapper over
// InspectManagedBlockFile. It adds no behavior of its own.
func InspectGitignoreFile(content []byte) (ManagedBlockShape, error) {
	return InspectManagedBlockFile(content, "gitignore")
}

// WriteGlobalGitignore composes the gitignore managed block into gitignorePath
// through the filewriter chokepoint. Foreign content outside the managed block
// is preserved verbatim (D-09). It returns the backup path (empty when the file
// is new). When the composed content is byte-identical to the existing file, the
// write is skipped and an empty backup path is returned (SC-2 idempotency).
// Callers that need to show a candidate file must call ComposeGlobalGitignore
// (the same composition this function uses) and nothing else.
func WriteGlobalGitignore(gitignorePath string, patterns []string) (string, error) {
	existing, err := os.ReadFile(gitignorePath) //nolint:gosec // gitignorePath is a trusted gitid-managed path
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("reading %s: %w", gitignorePath, err)
	}

	composed := ComposeGlobalGitignore(existing, patterns)

	// SC-2 idempotency: skip write (and backup) when content is unchanged.
	if bytes.Equal(composed, existing) {
		return "", nil
	}

	backupPath, err := filewriter.Write(gitignorePath, composed, gitconfigMode)
	if err != nil {
		return "", fmt.Errorf("writing gitignore block to %s: %w", gitignorePath, err)
	}
	return backupPath, nil
}

// WriteBaselineInclude prepends a managed [include] block pointing at
// baselineFilePath into gitconfigPath, placing the block at the TOP of the
// file (floor model — D-10, RESEARCH C1). The include path value is written
// verbatim from baselineFilePath — the caller is responsible for passing the
// tilde form (`~/.gitconfig.d/00-baseline`) so that git expands it at runtime.
// The sentinel name is "baseline-include" (distinct from "baseline").
//
// On first write the block is prepended before all existing content. On
// subsequent writes (re-runs) the block is updated in-place via ReplaceBlock so
// its floor position is preserved. It returns the backup path (empty when the
// file is new or when the content is unchanged — idempotent skip).
func WriteBaselineInclude(gitconfigPath, baselineFilePath string) (string, error) {
	existing, err := os.ReadFile(gitconfigPath) //nolint:gosec // gitconfigPath is a trusted gitid-managed path
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("reading %s: %w", gitconfigPath, err)
	}

	// ComposeBaselineInclude is extracted so the global-git apply ceremony can
	// compose and write unconditionally (R-3) while this function keeps the
	// SC-1 idempotent-skip contract its callers (the doctor Baseline check and
	// the cmd-layer wiring dispatcher) depend on — byte for byte.
	composed := ComposeBaselineInclude(existing, baselineFilePath)

	// SC-1 idempotency: skip write (and backup) when content is unchanged.
	if bytes.Equal(composed, existing) {
		return "", nil
	}

	backupPath, err := filewriter.Write(gitconfigPath, composed, gitconfigMode)
	if err != nil {
		return "", fmt.Errorf("writing baseline-include block to %s: %w", gitconfigPath, err)
	}
	return backupPath, nil
}
