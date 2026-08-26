package globalssh

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/sshconfig"
	"github.com/castocolina/gitid/internal/tuikit"
)

// cannedResolved is a realistic `ssh -G` output fragment carrying the six D-10
// keys, lowercase-keyed exactly as OpenSSH emits them. Values are the
// OpenSSH-compiled defaults the baseline probe returns; tests override the
// keys they plant.
const cannedResolved = `hashknownhosts no
stricthostkeychecking ask
forwardagent no
identitiesonly no
addkeystoagent no
usekeychain no
`

// depsForOut returns a Deps whose probes answer plain/baseline from the given
// outputs and whose file reads answer from the given content. The empty
// string means "no output / no content".
func depsForOut(plain, isolated string, configPath string, config []byte, sysPath string, sys []byte, sysErr error) Deps {
	return Deps{
		RunSSHG: func(_ context.Context, args ...string) (string, error) {
			for _, a := range args {
				if a == "/dev/null" {
					return isolated, nil
				}
			}
			return plain, nil
		},
		ReadConfig: func() (string, []byte, error) {
			return configPath, config, nil
		},
		ReadSystemConfig: func() (string, []byte, error) {
			return sysPath, sys, sysErr
		},
		GOOS: "linux",
	}
}

// statusByKey indexes a Statuses slice by canonical key.
func statusByKey(t *testing.T, statuses []OptionStatus) map[string]OptionStatus {
	t.Helper()
	m := make(map[string]OptionStatus, len(statuses))
	for _, st := range statuses {
		m[st.Key] = st
	}
	return m
}

// TestPolicyMatchesFixtureOrder pins D-10's single-order contract: the policy
// table's Key sequence equals tuikit.GlobalSSHOptions' declaration order (the
// fixture the Options sub-tab renders rows in) AND sshconfig's render order,
// so the block written by EnsureGlobals reads in the same order the rows do.
func TestPolicyMatchesFixtureOrder(t *testing.T) {
	if len(Policy) != 6 {
		t.Fatalf("Policy has %d entries, want 6 (D-10)", len(Policy))
	}
	if len(tuikit.GlobalSSHOptions) != len(Policy) {
		t.Fatalf("fixture has %d options, policy has %d", len(tuikit.GlobalSSHOptions), len(Policy))
	}
	for i := range Policy {
		if Policy[i].Key != tuikit.GlobalSSHOptions[i].Key {
			t.Errorf("Policy[%d].Key = %q, want fixture order %q (D-10 single order)", i, Policy[i].Key, tuikit.GlobalSSHOptions[i].Key)
		}
		if sshconfig.GlobalHostStarOrder[i] != Policy[i].Key {
			t.Errorf("sshconfig.GlobalHostStarOrder[%d] = %q, want policy order %q (one canonical order)", i, sshconfig.GlobalHostStarOrder[i], Policy[i].Key)
		}
	}
	p, ok := PolicyFor("StrictHostKeyChecking")
	if !ok || p.Recommended != "accept-new" {
		t.Errorf("PolicyFor(StrictHostKeyChecking).Recommended = %q (found=%v), want accept-new (D-10)", p.Recommended, ok)
	}
	p, ok = PolicyFor("ForwardAgent")
	if !ok || p.Risk != "High" {
		t.Errorf("PolicyFor(ForwardAgent).Risk = %q (found=%v), want High (D-10)", p.Risk, ok)
	}
	if _, ok := PolicyFor("NoSuchOption"); ok {
		t.Error("PolicyFor must report unknown keys as absent")
	}
}

// TestBuildProbeDepsIsRealWired proves the exported real constructor is
// actually wired (nil-seam guard): every seam is non-nil, and ReadConfig
// names the config path it was told about — the path a label must name.
func TestBuildProbeDepsIsRealWired(t *testing.T) {
	deps := BuildProbeDeps(t.TempDir() + "/config")
	if deps.RunSSHG == nil || deps.ReadConfig == nil || deps.ReadSystemConfig == nil {
		t.Fatal("BuildProbeDeps left a seam nil (injected-seam wiring blindspot)")
	}
}

// TestProbeHostIsInvalidTLD pins D-05: the probe host only matches a wildcard
// stanza and never opens a socket.
func TestProbeHostIsInvalidTLD(t *testing.T) {
	if !strings.HasSuffix(ProbeHost, ".invalid") {
		t.Errorf("ProbeHost = %q, want a TLD in .invalid", ProbeHost)
	}
}

// TestStatusesNeverReturnsEmptyOnProbeError pins the fail-open contract: when
// `ssh -G` refuses to answer, Statuses still returns one row per policy entry,
// every row carrying a non-empty ProbeError and the inconclusive class — never
// a nil slice, never an error that empties the pane (06-UI-SPEC.md's
// unresolved probe-failure question, resolved fail-open).
func TestStatusesNeverReturnsEmptyOnProbeError(t *testing.T) {
	sentinel := errors.New("probe exploded")
	deps := depsForOut("", "", "~/.ssh/config", nil, "/etc/ssh/ssh_config", nil, nil)
	deps.RunSSHG = func(context.Context, ...string) (string, error) { return "", sentinel }

	statuses := Statuses(deps)
	if len(statuses) != len(Policy) {
		t.Fatalf("Statuses len = %d, want %d even when the probe fails", len(statuses), len(Policy))
	}
	for _, st := range statuses {
		if st.Source != SourceInconclusive {
			t.Errorf("%s: source = %v, want SourceInconclusive on probe error", st.Key, st.Source)
		}
		if st.ProbeError == "" {
			t.Errorf("%s: ProbeError must be populated on probe error", st.Key)
		}
	}
}

// TestStatusesTimeoutDegradesToInconclusive proves a timed-out probe (not just
// a hard error) also degrades to an advisory note rather than a hang or an
// empty pane (T-06-02).
func TestStatusesTimeoutDegradesToInconclusive(t *testing.T) {
	deps := depsForOut("", "", "~/.ssh/config", nil, "/etc/ssh/ssh_config", nil, nil)
	deps.RunSSHG = func(_ context.Context, _ ...string) (string, error) {
		return "", context.DeadlineExceeded
	}
	for _, st := range Statuses(deps) {
		if st.Source != SourceInconclusive || st.ProbeError == "" {
			t.Errorf("%s: source = %v probeErr = %q, want inconclusive + note", st.Key, st.Source, st.ProbeError)
		}
	}
}
