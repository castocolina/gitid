package gitconfig

import (
	"fmt"
	"strings"

	"github.com/castocolina/gitid/internal/filewriter"
)

// GitFallbackAuthorBlockName is the sentinel suffix of the gitid-managed
// fallback-author block that sits in ~/.gitconfig immediately after the floor
// [include] block (D-05). The full sentinel lines are:
//
//	# BEGIN gitid managed: global-git-author
//	# END gitid managed: global-git-author
//
// The name is a sibling of GlobalGitBlockName. A user is free to name an
// identity "global-git-author"; the real guard is the reserved registry in
// IsReservedBlockName. A registered name is one the doctor's orphans check
// never treats as an identity, whatever a user calls their own. The orphans
// fixture is the thing that proves it (project learning L4, T-07-13).
const GitFallbackAuthorBlockName = "global-git-author"

// EnsureGitFallbackAuthor is the ONE owner of the fallback-author managed
// block in ~/.gitconfig (D-04, D-05). Both halves empty removes the block via
// RemoveBlock and returns; otherwise it composes a [user] section containing
// only the non-empty halves and places it via InsertBlockAfter anchored on
// BaselineIncludeBlockName. Every value passes validateValue first. Never
// emit a key with an empty value — that is the D-04 distinction between
// "unset" and "set to nothing", and git treats them differently.
//
// The caller must guarantee the floor include anchor is present in existing
// before this runs (R-4). InsertBlockAfter itself does not create anchors;
// cmd/gitid's runGitFallbackAuthorApply composes ComposeBaselineInclude into
// the same byte stream first.
func EnsureGitFallbackAuthor(existing []byte, name, email string) ([]byte, error) {
	if err := validateValue("user.name", name); err != nil {
		return nil, fmt.Errorf("gitconfig: EnsureGitFallbackAuthor: %w", err)
	}
	// A non-empty email gets the SAME stricter shape check a per-identity
	// email gets (code review finding: this write site previously only
	// applied the generic validateValue, weaker than ValidateEmail's
	// additional "@"-presence and comma/space rejection — a defense-in-
	// depth gap given this exact value class is the one CR-18 hardened
	// elsewhere in this package). Empty is the D-04 "unset" signal and
	// skips the shape check, matching every caller's own empty-is-valid
	// convention.
	if email != "" {
		if err := ValidateEmail(email); err != nil {
			return nil, fmt.Errorf("gitconfig: EnsureGitFallbackAuthor: %w", err)
		}
	}

	if name == "" && email == "" {
		return filewriter.RemoveBlock(existing, GitFallbackAuthorBlockName), nil
	}

	var b strings.Builder
	b.WriteString("[user]\n")
	if name != "" {
		fmt.Fprintf(&b, "\tname = %s\n", name)
	}
	if email != "" {
		fmt.Fprintf(&b, "\temail = %s\n", email)
	}
	body := strings.TrimRight(b.String(), "\n")

	out, err := filewriter.InsertBlockAfter(existing, BaselineIncludeBlockName, GitFallbackAuthorBlockName, body)
	if err != nil {
		return nil, fmt.Errorf("gitconfig: EnsureGitFallbackAuthor: %w", err)
	}
	return out, nil
}

// ReadGitFallbackAuthor parses the fallback-author block body back into the
// pair, returning empty strings for absent keys and for an absent block. The
// TUI seeds its two fields from this, which is what makes the apply rule
// truthful (D-04 / 07-UI-SPEC.md partial-apply resolution).
func ReadGitFallbackAuthor(existing []byte) (name, email string) {
	for _, b := range filewriter.ListBlocks(existing) {
		if b.Name != GitFallbackAuthorBlockName {
			continue
		}
		parsed := parseGitconfigBlockBody(b.Body)
		return parsed["user.name"], parsed["user.email"]
	}
	return "", ""
}
