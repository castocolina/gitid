package checks

import (
	"fmt"
	"os"
	"strings"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/sshconfig"
)

// CheckFiles reports SSH and Git configuration files that cannot be parsed.
func CheckFiles(deps doctor.Deps) []doctor.Finding {
	var findings []doctor.Finding

	if err := checkSSHConfig(deps); err != nil {
		findings = append(findings, filesFinding(deps, deps.SSHConfigPath, "SSH", err))
	}
	for _, path := range gitConfigPaths(deps) {
		if err := checkGitConfig(deps, path); err != nil {
			findings = append(findings, filesFinding(deps, path, "Git", err))
		}
	}
	return findings
}

// checkSSHConfig reports a parse error only when ~/.ssh/config EXISTS and
// fails to parse. A file that does not exist yet is a normal, healthy
// first-run state (nothing has been set up), never a "critical, checks
// paused" condition — CheckOrphans/CheckCoherence/etc. already produce
// correctly-scoped zero findings for zero identities, and this project's
// own established convention (CheckBaseline) reports an ENTIRELY-missing
// artifact as an actionable ERROR with a real fix, never CRITICAL with
// nothing to do but wait. Conflating "not started yet" with "corrupted"
// here would reintroduce the false-positive-loop class this wave exists to
// close, at first-run — the single most common state any user hits.
func checkSSHConfig(deps doctor.Deps) error {
	if deps.ReadFile == nil {
		return nil
	}
	content, err := deps.ReadFile(deps.SSHConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	_, err = sshconfig.Parse(content)
	return err
}

func gitConfigPaths(deps doctor.Deps) []string {
	return deps.GitConfigPaths
}

// checkGitConfig mirrors checkSSHConfig's existence-vs-corruption
// distinction: a git config file (the base ~/.gitconfig or a fragment)
// that does not exist yet is never a parse failure — only a file that
// exists but fails git's own parse is.
func checkGitConfig(deps doctor.Deps, path string) error {
	if deps.Stat != nil {
		if _, err := deps.Stat(path); err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
	}
	if deps.RunGitConfigGet == nil {
		return nil
	}
	_, err := deps.RunGitConfigGet(path, "--list")
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "exit status 1") && !strings.Contains(err.Error(), "exit status 128") {
		return nil
	}
	return err
}

func filesFinding(deps doctor.Deps, path, target string, err error) doctor.Finding {
	snippet := ""
	if deps.ReadFile != nil {
		if content, readErr := deps.ReadFile(path); readErr == nil {
			snippet = strings.TrimSpace(string(content))
		}
	}
	raw := err.Error()
	return doctor.Finding{
		Family:       doctor.FamilyFiles,
		Severity:     doctor.SeverityCritical,
		Title:        fmt.Sprintf("%s configuration cannot be parsed", target),
		Explanation:  fmt.Sprintf("%s: %s. Checks for this section are paused until it parses again.", path, raw),
		SuggestedFix: "Correct the parse error, then run Health again.",
		Target:       target,
		ParseError:   &doctor.ParseError{File: path, Raw: raw, Snippet: snippet},
	}
}
