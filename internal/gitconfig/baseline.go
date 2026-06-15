package gitconfig

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/castocolina/gitid/internal/filewriter"
)

// Conflict records one overlap between a user-set gitconfig key and the
// baseline key set. Winner is always "user" under floor ordering (D-10).
type Conflict struct {
	Key           string
	UserValue     string
	BaselineValue string
	Winner        string // always "user" (floor ordering — user keys win)
}

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

// BaselineKeySet returns the canonical set of lowercase section.key → value
// pairs that the baseline block manages. This is the authoritative source used
// by ScanConflicts so the two never drift (C2 algorithm requirement). The map
// contains ONLY Tier-1 unconditional keys; Tier-2 keys (autocrlf, pager, …)
// could be absent depending on cfg — the scan is conservative (reports only
// certain conflicts). Callers that need the full Tier-2 set should call
// BaselineKeySet and augment before passing to ScanConflicts.
func BaselineKeySet() map[string]string {
	return map[string]string{
		"core.ignorecase":      "false",
		"core.excludesfile":    "~/.gitignore_global",
		"push.autosetupremote": "true", // git lower-cases keys in --list output
		"pull.rebase":          "true",
		"fetch.prune":          "true",
		"color.ui":             "auto",
	}
}

// ScanConflicts detects overlaps between user-owned keys in gitconfigPath and
// the provided baselineKeys map. It strips ALL gitid managed blocks first
// (RESEARCH C2 / Pitfall C) so that baseline-written keys are never reported
// as user conflicts. The user-owned portion is written to a temp file and
// parsed via `git config --file --list` (WITHOUT --includes per C6). Each
// overlap is returned as a Conflict with Winner="user" (floor ordering).
// Missing file returns (nil, nil) for the first-run case.
func ScanConflicts(gitconfigPath string, baselineKeys map[string]string) ([]Conflict, error) {
	content, err := os.ReadFile(gitconfigPath) //nolint:gosec // gitconfigPath is a trusted gitid-managed path (G304)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // first-run case: no gitconfig yet
		}
		return nil, fmt.Errorf("scanning conflicts: reading %s: %w", gitconfigPath, err)
	}

	// Strip ALL gitid managed blocks from the content to isolate user-owned
	// keys (RESEARCH C2 / Pitfall C): managed-block keys must never appear
	// as user conflicts.
	userPortion := content
	for _, b := range filewriter.ListBlocks(content) {
		userPortion = filewriter.RemoveBlock(userPortion, b.Name)
	}

	// Write the user-owned portion to a temp file so git can parse it.
	tmp, err := os.CreateTemp("", "gitid-conflict-*.gitconfig")
	if err != nil {
		return nil, fmt.Errorf("scanning conflicts: creating temp file: %w", err)
	}
	if _, err = tmp.Write(userPortion); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return nil, fmt.Errorf("scanning conflicts: writing temp file: %w", err)
	}
	if err = tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return nil, fmt.Errorf("scanning conflicts: closing temp file: %w", err)
	}
	defer os.Remove(tmp.Name()) //nolint:errcheck // best-effort cleanup of short-lived temp

	// Parse user-owned keys: arg-slice form (no shell), WITHOUT --includes (C6),
	// so we only see keys physically in this file.
	cmd := exec.Command("git", "config", "--file", tmp.Name(), "--list") //nolint:gosec // arg-slice form, no shell; trusted temp path (G204)
	out, err := cmd.Output()
	if err != nil {
		// An empty or comment-only file is not an error from git's perspective,
		// but a parse failure on a malformed user gitconfig should be surfaced.
		return nil, fmt.Errorf("scanning conflicts: %w", err)
	}

	// Build a lowercase key→value map from the user-owned portion.
	userKeys := make(map[string]string)
	for _, line := range strings.Split(string(out), "\n") {
		kv := strings.SplitN(line, "=", 2)
		if len(kv) != 2 {
			continue
		}
		userKeys[strings.ToLower(kv[0])] = kv[1]
	}

	// Intersect user keys with the baseline key set; emit a Conflict per overlap.
	var conflicts []Conflict
	for baseKey, baseVal := range baselineKeys {
		if userVal, ok := userKeys[baseKey]; ok {
			conflicts = append(conflicts, Conflict{
				Key:           baseKey,
				UserValue:     userVal,
				BaselineValue: baseVal,
				Winner:        "user", // floor ordering: user keys always win (D-10)
			})
		}
	}
	return conflicts, nil
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
func parseGitconfigBlockBody(body string) map[string]string {
	result := make(map[string]string)
	var section string
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			// Section header: [core] → "core", [alias] → "alias"
			section = strings.ToLower(trimmed[1 : len(trimmed)-1])
			continue
		}
		kv := strings.SplitN(trimmed, " = ", 2)
		if len(kv) == 2 && section != "" {
			key := strings.ToLower(section + "." + strings.TrimSpace(kv[0]))
			result[key] = strings.TrimSpace(kv[1])
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

// DefaultGitignorePatterns returns the curated gitignore seed list (D-08/SC-2).
// The first six are SC-2-locked; the remaining are planner discretion (D-Claude).
// Order is fixed to satisfy the byte-stability contract (Pitfall D).
func DefaultGitignorePatterns() []string {
	return []string{
		".DS_Store",
		"Thumbs.db",
		"*.log",
		"*.bak",
		"*.tmp",
		"*.swp",
		"*.swo",
		".idea/",
		".vscode/",
		"node_modules/",
		"__pycache__/",
		"*.pyc",
		".env",
	}
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
// is validated with validateValue before render.
func RenderBaselineBlock(cfg BaselineConfig) string {
	// Validate user-supplied Tier-2 strings before rendering (V5 injection guard).
	if cfg.Pager != "" {
		if err := validateValue("core.pager", cfg.Pager); err != nil {
			panic(fmt.Sprintf("gitconfig: RenderBaselineBlock: %v", err))
		}
	}
	if cfg.MergeConflictStyle != "" {
		if err := validateValue("merge.conflictstyle", cfg.MergeConflictStyle); err != nil {
			panic(fmt.Sprintf("gitconfig: RenderBaselineBlock: %v", err))
		}
	}
	if cfg.InitDefaultBranch != "" {
		if err := validateValue("init.defaultBranch", cfg.InitDefaultBranch); err != nil {
			panic(fmt.Sprintf("gitconfig: RenderBaselineBlock: %v", err))
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

	return strings.TrimRight(b.String(), "\n")
}

// RenderURLRewritesBlock renders the url-rewrites block body for the given
// insteadOf mappings. The section order matches the input slice order (callers
// use DefaultURLRewrites for the canonical big-three order). Each URL/SSH prefix
// pair is validated with validateValue before render to guard against newline
// injection. An empty rewrites slice returns an empty string.
func RenderURLRewritesBlock(rewrites []URLRewrite) string {
	if len(rewrites) == 0 {
		return ""
	}

	var b strings.Builder
	for _, r := range rewrites {
		// Validate user-supplied URL strings (V5 injection guard).
		if err := validateValue("url.insteadOf.httpsPrefix", r.HTTPSPrefix); err != nil {
			panic(fmt.Sprintf("gitconfig: RenderURLRewritesBlock: %v", err))
		}
		if err := validateValue("url.insteadOf.sshPrefix", r.SSHPrefix); err != nil {
			panic(fmt.Sprintf("gitconfig: RenderURLRewritesBlock: %v", err))
		}
		fmt.Fprintf(&b, "[url %q]\n", r.SSHPrefix)
		fmt.Fprintf(&b, "\tinsteadOf = %s\n", r.HTTPSPrefix)
	}
	return strings.TrimRight(b.String(), "\n")
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

	composed := filewriter.ReplaceBlock(existing, "baseline", RenderBaselineBlock(cfg))
	if len(rewrites) > 0 {
		composed = filewriter.ReplaceBlock(composed, "url-rewrites", RenderURLRewritesBlock(rewrites))
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

// WriteGlobalGitignore composes the gitignore managed block into gitignorePath
// through the filewriter chokepoint. Foreign content outside the managed block
// is preserved verbatim (D-09). It returns the backup path (empty when the file
// is new). When the composed content is byte-identical to the existing file, the
// write is skipped and an empty backup path is returned (SC-2 idempotency).
func WriteGlobalGitignore(gitignorePath string, patterns []string) (string, error) {
	existing, err := os.ReadFile(gitignorePath) //nolint:gosec // gitignorePath is a trusted gitid-managed path
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("reading %s: %w", gitignorePath, err)
	}

	composed := filewriter.ReplaceBlock(existing, "gitignore", RenderGitignoreBlock(patterns))

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
// file (floor model — D-10, RESEARCH C1). The include path is written as a
// fixed literal string with a literal ~ so git expands it at runtime (RESEARCH
// Open Q2). The sentinel name is "baseline-include" (distinct from "baseline").
//
// On first write the block is prepended before all existing content. On
// subsequent writes (re-runs) the block is updated in-place via ReplaceBlock so
// its floor position is preserved. It returns the backup path (empty when the
// file is new).
func WriteBaselineInclude(gitconfigPath, baselineFilePath string) (string, error) {
	// The include body is a fixed constant; baselineFilePath is an in-process
	// constant (never user input), so T-03.1-03 is satisfied — no user string
	// is interpolated into the include body.
	_ = baselineFilePath // used as documentation; literal path written below
	const includeBody = "[include]\n\tpath = ~/.gitconfig.d/00-baseline"

	existing, err := os.ReadFile(gitconfigPath) //nolint:gosec // gitconfigPath is a trusted gitid-managed path
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("reading %s: %w", gitconfigPath, err)
	}

	composed := filewriter.PrependBlockIfNotFound(existing, "baseline-include", includeBody)

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
