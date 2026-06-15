package gitconfig

import (
	"os"
	"os/exec"
	"strings"

	"github.com/castocolina/gitid/internal/filewriter"
)

// IncludeIfInfo holds reconstruction data from one gitid managed includeIf block.
type IncludeIfInfo struct {
	FragmentPath string  // last `path =` value seen (all matches share one path)
	Matches      []Match // all [includeIf "..."] conditions in the block
}

// FragmentInfo holds the user identity fields from a per-identity fragment.
type FragmentInfo struct {
	GitName    string
	GitEmail   string
	SigningKey string // the .pub PATH value of user.signingkey (stored as literal — Pitfall E)
	GPGFormat  string // "ssh" when signing is enabled
	CommitSign bool   // true when commit.gpgsign = true
	Missing    bool   // true when the fragment file does not exist
}

// ParseManagedIncludeIf extracts all gitid-managed includeIf blocks from the
// bytes of ~/.gitconfig, keyed by identity name (D-01).
func ParseManagedIncludeIf(content []byte) map[string]IncludeIfInfo {
	blocks := filewriter.ListBlocks(content)
	result := make(map[string]IncludeIfInfo, len(blocks))
	for _, b := range blocks {
		result[b.Name] = parseIncludeIfBody(b.Body)
	}
	return result
}

// parseIncludeIfBody parses the raw body of a gitid managed includeIf block
// into an IncludeIfInfo. The format is gitid-controlled so a simple line
// scanner (no library) is used.
func parseIncludeIfBody(body string) IncludeIfInfo {
	var info IncludeIfInfo
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		// [includeIf "gitdir:~/git/work/"] or [includeIf "hasconfig:..."]
		if strings.HasPrefix(line, `[includeIf "`) && strings.HasSuffix(line, `"]`) {
			cond := line[len(`[includeIf "`) : len(line)-2]
			m := conditionToMatch(cond)
			info.Matches = append(info.Matches, m)
		}
		// \tpath = ~/.gitconfig.d/work
		if strings.HasPrefix(line, "path = ") {
			info.FragmentPath = strings.TrimPrefix(line, "path = ")
		}
	}
	return info
}

// conditionToMatch converts an includeIf condition string into a Match value.
func conditionToMatch(cond string) Match {
	if strings.HasPrefix(cond, "gitdir:") {
		return Match{Kind: MatchGitdir, Value: strings.TrimPrefix(cond, "gitdir:")}
	}
	return Match{Kind: MatchHasconfig, Value: strings.TrimPrefix(cond, "hasconfig:")}
}

// ReadFragment runs `git config --file <fragPath> --list` and parses the
// output into a FragmentInfo. When the file is absent, Missing is set true
// and the other fields are zero (best-effort D-02). The signingkey path is
// stored as the literal string returned by git (Pitfall E: no tilde expansion,
// no filepath.Abs).
func ReadFragment(fragPath string) (FragmentInfo, error) {
	if _, statErr := os.Stat(fragPath); os.IsNotExist(statErr) {
		return FragmentInfo{Missing: true}, nil
	}
	cmd := exec.Command("git", "config", "--file", fragPath, "--list") //nolint:gosec // arg-slice form, no shell; fragPath is a trusted gitid-managed path (G204)
	out, err := cmd.Output()
	if err != nil {
		// Treat unreadable fragment as missing (best-effort D-02).
		return FragmentInfo{Missing: true}, nil
	}
	var info FragmentInfo
	for _, line := range strings.Split(string(out), "\n") {
		kv := strings.SplitN(line, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch strings.ToLower(kv[0]) {
		case "user.name":
			info.GitName = kv[1]
		case "user.email":
			info.GitEmail = kv[1]
		case "user.signingkey":
			info.SigningKey = kv[1] // stored as literal — Pitfall E
		case "gpg.format":
			info.GPGFormat = kv[1]
		case "commit.gpgsign":
			info.CommitSign = strings.EqualFold(kv[1], "true")
		}
	}
	return info, nil
}

// RemoveAllowedSignersLine rewrites path with the line for identityEmail
// removed. A line matches only when its FIRST whitespace-delimited field (the
// allowed_signers PRINCIPAL) equals identityEmail EXACTLY — substring matching
// is unsafe because emails can share a common prefix (e.g. removing
// "alice@corp.com" must not strip "alice@corp.com.attacker.example", CR-01) —
// AND the line still carries namespaces="git" (T-03-01, Pitfall D). Blank lines
// and comments are preserved untouched. Backs up via filewriter.Write at mode
// 0600 (T-03-05: private material). Idempotent when no matching line exists.
// Missing file returns ("", nil).
func RemoveAllowedSignersLine(path, identityEmail string) (backupPath string, err error) {
	existing, readErr := os.ReadFile(path) //nolint:gosec // path is a trusted gitid-managed allowed_signers path
	if os.IsNotExist(readErr) {
		return "", nil // missing file — idempotent
	}
	if readErr != nil {
		return "", readErr
	}
	var kept []string
	for _, line := range strings.Split(string(existing), "\n") {
		// Exact first-field principal match: split on whitespace and compare the
		// PRINCIPAL token exactly, AND require namespaces="git" on the same line.
		// Blank/comment lines have no matching first field and are preserved.
		fields := strings.Fields(line)
		if len(fields) >= 1 && fields[0] == identityEmail && strings.Contains(line, `namespaces="git"`) {
			continue // remove this exact-principal line
		}
		kept = append(kept, line)
	}
	result := strings.Join(kept, "\n")
	return filewriter.Write(path, []byte(result), 0o600)
}
