package checks_test

import (
	"errors"
	"os"
	"testing"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/doctor/checks"
)

func TestCheckFilesReportsCriticalGitParseFailure(t *testing.T) {
	fragment := "/home/u/.gitconfig.d/work"
	findings := checks.CheckFiles(doctor.Deps{
		GitConfigPaths: []string{fragment},
		Stat:           orphStat(fragment),
		RunGitConfigGet: func(file, key string) (string, error) {
			if file != fragment || key != "--list" {
				t.Fatalf("git config probe = (%q, %q)", file, key)
			}
			return "", errors.New("exit status 128")
		},
	})
	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(findings))
	}
	got := findings[0]
	if got.Family != doctor.FamilyFiles || got.Severity != doctor.SeverityCritical || got.Target != "Git" {
		t.Fatalf("finding = %+v, want critical Files/Git", got)
	}
}

// TestCheckFilesToleratesMissingSSHConfig proves a MISSING ~/.ssh/config —
// the normal first-run state, before any identity has ever been created —
// produces NO finding, never a critical "checks paused" alarm. Conflating
// "not started yet" with "corrupted" would reintroduce, at the single most
// common state any user hits, exactly the false-positive-loop class this
// wave (08-03) exists to close (a deliberate, documented divergence from
// the plan's own literal "missing or failing to parse" phrasing — see
// 08-03-SUMMARY.md).
func TestCheckFilesToleratesMissingSSHConfig(t *testing.T) {
	findings := checks.CheckFiles(doctor.Deps{
		SSHConfigPath: "/home/u/.ssh/config",
		ReadFile: func(string) ([]byte, error) {
			return nil, os.ErrNotExist
		},
	})
	if len(findings) != 0 {
		t.Fatalf("findings = %+v, want 0 (a missing config is a normal first-run state, not a parse error)", findings)
	}
}

// TestCheckFilesReportsGenuineSSHReadFailure proves a NON-missing read
// failure (the file exists but cannot be read — e.g. a permissions error,
// or here a stand-in for "exists but ReadFile could not return usable
// bytes") still produces the critical Files/SSH finding — the existence-
// vs-corruption distinction added above only tolerates os.ErrNotExist,
// never every error.
func TestCheckFilesReportsGenuineSSHReadFailure(t *testing.T) {
	findings := checks.CheckFiles(doctor.Deps{
		SSHConfigPath: "/home/u/.ssh/config",
		ReadFile: func(string) ([]byte, error) {
			return nil, errors.New("permission denied")
		},
	})
	if len(findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(findings))
	}
	got := findings[0]
	if got.Family != doctor.FamilyFiles || got.Severity != doctor.SeverityCritical || got.Target != "SSH" {
		t.Fatalf("finding = %+v, want critical Files/SSH", got)
	}
}

// TestCheckFilesToleratesMissingGitConfigFragment mirrors the SSH-side
// tolerance for the Git side: a fragment that does not exist yet is never
// a parse failure.
func TestCheckFilesToleratesMissingGitConfigFragment(t *testing.T) {
	fragment := "/home/u/.gitconfig.d/work"
	findings := checks.CheckFiles(doctor.Deps{
		GitConfigPaths: []string{fragment},
		Stat: func(string) (os.FileInfo, error) {
			return nil, os.ErrNotExist
		},
	})
	if len(findings) != 0 {
		t.Fatalf("findings = %+v, want 0 (a missing fragment is a normal state, not a parse error)", findings)
	}
}
