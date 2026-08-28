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

func TestCheckFilesReportsMissingSSHConfig(t *testing.T) {
	findings := checks.CheckFiles(doctor.Deps{
		SSHConfigPath: "/home/u/.ssh/config",
		ReadFile: func(string) ([]byte, error) {
			return nil, os.ErrNotExist
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
