package checks

import (
	"testing"

	"github.com/castocolina/gitid/internal/doctor"
)

// userRedundantConfig is the user's exact reported SSH config scenario that
// triggered UAT G-4: a root-level IgnoreUnknown UseKeychain directive, a
// hand-written "Host *" stanza with UseKeychain, and gitid's managed _global
// "Host *" block (UseKeychain / AddKeysToAgent / IgnoreUnknown). This produces
// redundant Host * stanzas and duplicate global UseKeychain + IgnoreUnknown.
const userRedundantConfig = `
IgnoreUnknown UseKeychain

# User's hand-written global block
Host *
  UseKeychain yes

# BEGIN gitid managed: _global
Host *
  IgnoreUnknown UseKeychain
  UseKeychain yes
  AddKeysToAgent yes
# END gitid managed: _global
`

// legitimateSingleManagedBlock is the canonical gitid-managed _global block
// alone (IgnoreUnknown then UseKeychain once each, single Host *) — must NOT
// produce any redundancy findings (T-05.7-11-03 false-positive guard).
const legitimateSingleManagedBlock = `
# BEGIN gitid managed: _global
Host *
  IgnoreUnknown UseKeychain
  UseKeychain yes
  AddKeysToAgent yes
# END gitid managed: _global
`

// twoHostStarsConfig has two "Host *" stanzas but no duplicated individual directives.
const twoHostStarsConfig = `
Host *
  AddKeysToAgent yes

Host github.com
  HostName ssh.github.com
  Port 443

# BEGIN gitid managed: _global
Host *
  IgnoreUnknown UseKeychain
  UseKeychain yes
  AddKeysToAgent yes
# END gitid managed: _global
`

// cleanConfig has a single Host * block with no duplicate global directives.
const cleanConfig = `
Host github.com
  HostName ssh.github.com
  Port 443
  User git

# BEGIN gitid managed: _global
Host *
  IgnoreUnknown UseKeychain
  UseKeychain yes
  AddKeysToAgent yes
# END gitid managed: _global
`

// makeRedundancyDeps builds a doctor.Deps whose ReadFile returns the given
// content when called with any path (content is fixed for each test case).
func makeRedundancyDeps(content string) doctor.Deps {
	return doctor.Deps{
		SSHConfigPath: "/fake/.ssh/config",
		ReadFile: func(_ string) ([]byte, error) {
			return []byte(content), nil
		},
	}
}

// TestCheckRedundancy_CleanConfig asserts that a config with a single Host *
// block and no duplicate directives produces no findings (true negative).
func TestCheckRedundancy_CleanConfig(t *testing.T) {
	deps := makeRedundancyDeps(cleanConfig)
	findings := CheckRedundancy(deps)
	if len(findings) != 0 {
		t.Errorf("clean config must produce no findings; got %d: %+v", len(findings), findings)
	}
}

// TestCheckRedundancy_LegitimateBlock asserts that the canonical single managed
// _global block alone does not trigger false-positive redundancy findings
// (T-05.7-11-03: IgnoreUnknown then UseKeychain once each is valid).
func TestCheckRedundancy_LegitimateBlock(t *testing.T) {
	deps := makeRedundancyDeps(legitimateSingleManagedBlock)
	findings := CheckRedundancy(deps)
	if len(findings) != 0 {
		t.Errorf("legitimate single managed block must produce no findings; got %d: %+v", len(findings), findings)
	}
}

// TestCheckRedundancy_TwoHostStars asserts that a config with two "Host *"
// stanzas produces at least one finding with the correct family, severity, and
// nil Fix (T-05.7-11-01 advisory-only guarantee).
func TestCheckRedundancy_TwoHostStars(t *testing.T) {
	deps := makeRedundancyDeps(twoHostStarsConfig)
	findings := CheckRedundancy(deps)
	if len(findings) == 0 {
		t.Fatal("two Host * stanzas must produce at least one finding")
	}

	// Find the Host * finding.
	var hostStarFinding *doctor.Finding
	for i := range findings {
		if findings[i].Family == doctor.FamilyRedundancy {
			hostStarFinding = &findings[i]
			break
		}
	}
	if hostStarFinding == nil {
		t.Fatal("must have at least one FamilyRedundancy finding")
	}
	if hostStarFinding.Severity != doctor.SeverityWarning {
		t.Errorf("finding severity must be SeverityWarning; got %v", hostStarFinding.Severity)
	}
	if hostStarFinding.Fix != nil {
		t.Error("Fix must be nil (advisory only, T-05.7-11-01)")
	}
}

// TestCheckRedundancy_UserScenario asserts that the user's exact reported SSH
// config (UAT G-4) produces findings for multiple Host * stanzas AND duplicate
// global directives, all with SeverityWarning and nil Fix.
func TestCheckRedundancy_UserScenario(t *testing.T) {
	deps := makeRedundancyDeps(userRedundantConfig)
	findings := CheckRedundancy(deps)
	if len(findings) == 0 {
		t.Fatal("user redundant config must produce at least one finding (UAT G-4)")
	}

	for _, f := range findings {
		// All findings must be in the FamilyRedundancy family.
		if f.Family != doctor.FamilyRedundancy {
			t.Errorf("finding family must be FamilyRedundancy; got %q", f.Family)
		}
		// All findings must be SeverityWarning — the hard advisory-only requirement.
		if f.Severity != doctor.SeverityWarning {
			t.Errorf("finding severity must be SeverityWarning (T-05.7-11-01); got %v", f.Severity)
		}
		// Fix must be nil — no destructive auto-fix (T-05.7-11-02).
		if f.Fix != nil {
			t.Errorf("Fix must be nil (report-only); got non-nil Fix for finding %q", f.Title)
		}
	}

	// The exit code for redundancy-only findings must be 1 (warning tier).
	// It must NEVER escalate to 2 or 3 (T-05.7-11-01).
	code := doctor.ExitCode(findings)
	if code != 1 {
		t.Errorf("exit code for redundancy findings must be 1 (warning); got %d", code)
	}
}

// TestCheckRedundancy_ExitCodeWarning asserts that redundancy-only findings
// never produce exit code 2 or 3 (the hard non-blocking guarantee).
func TestCheckRedundancy_ExitCodeWarning(t *testing.T) {
	deps := makeRedundancyDeps(userRedundantConfig)
	findings := CheckRedundancy(deps)
	if len(findings) == 0 {
		t.Fatal("expected at least one finding for the redundant config")
	}
	code := doctor.ExitCode(findings)
	if code != 1 {
		t.Errorf("exit code for redundancy-only findings must be 1; got %d (T-05.7-11-01)", code)
	}
}

// TestCheckRedundancy_EmptyConfig asserts that an empty config produces no
// findings and no error (best-effort, matches CheckOverlap behavior).
func TestCheckRedundancy_EmptyConfig(t *testing.T) {
	deps := makeRedundancyDeps("")
	findings := CheckRedundancy(deps)
	if len(findings) != 0 {
		t.Errorf("empty config must produce no findings; got %d", len(findings))
	}
}

// TestCheckRedundancy_FixAlwaysNil asserts that every finding returned by
// CheckRedundancy has Fix == nil (report-only; no destructive auto-fix).
func TestCheckRedundancy_FixAlwaysNil(t *testing.T) {
	for _, content := range []string{userRedundantConfig, twoHostStarsConfig} {
		deps := makeRedundancyDeps(content)
		for _, f := range CheckRedundancy(deps) {
			if f.Fix != nil {
				t.Errorf("every redundancy finding must have Fix==nil; got non-nil for %q", f.Title)
			}
		}
	}
}
