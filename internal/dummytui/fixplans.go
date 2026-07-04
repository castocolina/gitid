package dummytui

// fixplans.go is the Go mirror of
// .planning/design/mockup-src/src/demo/fixplans.ts — the exact target
// file, diff, confirm semantics, and result copy for every fixable doctor
// finding. Shared by the Doctor surface and the per-identity findings
// panel (FIX-01/02: the fixer is a consequence of the doctor, reachable
// wherever a finding is shown — never its own view).

import "strings"

// FixDestructive gates a destructive fix behind a typed confirm word.
type FixDestructive struct {
	ConfirmWord string
	Warning     string
}

// FixPlan is one fixable finding's write plan: the target file, the exact
// diff previewed, optional destructive gating, and the result receipt.
type FixPlan struct {
	File        string
	Diff        string
	Destructive *FixDestructive
	Result      string
}

// PlanFor returns the fix plan for finding — mirroring fixplans.ts's
// planFor per finding id. The contradiction fix reuses data.go's
// FixerFixPreviewLines verbatim and is destructive (typed Host name).
func PlanFor(finding DemoFinding) FixPlan {
	switch finding.ID {
	case "ssh-key-perms-archived":
		return FixPlan{
			File:   "~/.ssh/id_ed25519_archived",
			Diff:   "- mode 0644 (world-readable)\n+ mode 0600 (owner only)",
			Result: "chmod 0600 ~/.ssh/id_ed25519_archived applied.",
		}
	case "ssh-identitiesonly-contradiction":
		return FixPlan{
			File: "~/.ssh/config",
			Diff: strings.Join(FixerFixPreviewLines, "\n"),
			Destructive: &FixDestructive{
				ConfirmWord: "clientb.github.com",
				Warning:     `This rewrites a directive already present in your SSH config. Type the Host name "clientb.github.com" to confirm — this cannot be undone without restoring the backup.`,
			},
			Result: "IdentitiesOnly set to yes on Host clientb.github.com in ~/.ssh/config.",
		}
	case "git-includeif-missing-fragment":
		return FixPlan{
			File:   "~/.gitconfig.d/legacy",
			Diff:   "+ ~/.gitconfig.d/legacy (fragment restored from template)\n  [includeIf \"gitdir:~/legacy/\"] → path now resolves",
			Result: `~/.gitconfig.d/legacy restored — the includeIf resolves again; "legacy" is complete.`,
		}
	case "ssh-duplicate-host-star":
		return FixPlan{
			File:   "~/.ssh/config",
			Diff:   "- Host * (line 41 — duplicate stanza removed)\n+ (its directives merged into the Host * at line 4)",
			Result: "The two Host * stanzas were merged into one.",
		}
	default:
		return FixPlan{
			File:   "~/.ssh/config",
			Diff:   "+ " + finding.SuggestedFix,
			Result: "Fix applied.",
		}
	}
}
