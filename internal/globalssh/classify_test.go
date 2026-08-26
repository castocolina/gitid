package globalssh

import (
	"context"
	"errors"
	"testing"
)

// TestStatuses is the table-driven provenance classifier suite (D-01 with the
// review's correction — a claim is made only when the evidence supports it).
// Each case pins the returned SourceClass constant and, where the class names
// a file, the file and line.
func TestStatuses(t *testing.T) {
	const cfgPath = "~/.ssh/config"
	const sysPath = "/etc/ssh/ssh_config"

	cases := []struct {
		name       string
		plain      string
		isolated   string
		config     string // per-user config content planted at cfgPath
		sys        string // system config content planted at sysPath
		sysErr     error
		wantSource SourceClass
		wantValue  string
		wantFile   string
		wantLine   int
	}{
		{
			name:       "gitid-parsed",
			plain:      "hashknownhosts yes\n",
			isolated:   cannedResolved,
			config:     "Host foo\n  HashKnownHosts yes\n",
			sys:        "",
			wantSource: SourceGitidParsed,
			wantValue:  "yes",
			wantFile:   cfgPath,
			wantLine:   2,
		},
		{
			name:       "system-file-corroborated",
			plain:      "hashknownhosts yes\n",
			isolated:   cannedResolved, // baseline: hashknownhosts no
			config:     "",
			sys:        "# system default\nHashKnownHosts yes\n",
			wantSource: SourceSystemFile,
			wantValue:  "yes",
			wantFile:   sysPath,
			wantLine:   2,
		},
		{
			// The review's HIGH provenance finding, pinned: an UNREADABLE
			// system config must yield the hedged outside class, NEVER
			// SourceSystemFile — the label may not name a file it could not
			// prove the directive came from.
			name:       "system-file-unreadable",
			plain:      "hashknownhosts yes\n",
			isolated:   cannedResolved,
			config:     "",
			sys:        "",
			sysErr:     errors.New("permission denied"),
			wantSource: SourceOutsideGitid,
			wantValue:  "yes",
		},
		{
			name:       "baseline",
			plain:      cannedResolved,
			isolated:   cannedResolved,
			config:     "",
			sys:        "",
			wantSource: SourceBaseline,
			wantValue:  "no",
		},
		{
			name:       "outside-gitid-not-corroborated",
			plain:      "hashknownhosts yes\n",
			isolated:   cannedResolved,
			config:     "",
			sys:        "# nothing here\n",
			wantSource: SourceOutsideGitid,
			wantValue:  "yes",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			deps := depsForOut(tc.plain, tc.isolated, cfgPath, []byte(tc.config), sysPath, []byte(tc.sys), tc.sysErr)
			statuses := statusByKey(t, Statuses(deps))
			st, ok := statuses["HashKnownHosts"]
			if !ok {
				t.Fatalf("Statuses missing HashKnownHosts row (%d rows)", len(statuses))
			}
			if st.Source != tc.wantSource {
				t.Errorf("source = %v, want %v", st.Source, tc.wantSource)
			}
			if st.CurrentValue != tc.wantValue {
				t.Errorf("current value = %q, want %q", st.CurrentValue, tc.wantValue)
			}
			if st.SourceFile != tc.wantFile {
				t.Errorf("source file = %q, want %q", st.SourceFile, tc.wantFile)
			}
			if st.SourceLine != tc.wantLine {
				t.Errorf("source line = %d, want %d", st.SourceLine, tc.wantLine)
			}
			if st.ProbeError != "" {
				t.Errorf("ProbeError = %q, want empty for a non-inconclusive class", st.ProbeError)
			}
		})
	}
}

// TestStatusesHashKnownHostsState pins the tracer row's correct State: only
// HashKnownHosts carries an already-set/needs-action State in this plan; the
// other five rows keep the needs-action zero value (06-03 owns the full model).
func TestStatusesHashKnownHostsState(t *testing.T) {
	deps := depsForOut("hashknownhosts yes\n", cannedResolved, "~/.ssh/config",
		[]byte("HashKnownHosts yes\n"), "/etc/ssh/ssh_config", nil, nil)
	for _, st := range Statuses(deps) {
		if st.Key == "HashKnownHosts" && st.State != StateAlreadySet {
			t.Errorf("HashKnownHosts state = %v, want StateAlreadySet", st.State)
		}
		if st.Key != "HashKnownHosts" && st.State != StateNeedsAction {
			t.Errorf("%s state = %v, want the needs-action zero value in 06-01", st.Key, st.State)
		}
	}
}

// TestStatusesProbeErrorCarriesMessage proves the inconclusive rows carry the
// concrete probe error text (the probe-error note the pane renders), not an
// invented constant.
func TestStatusesProbeErrorCarriesMessage(t *testing.T) {
	deps := depsForOut("", "", "~/.ssh/config", nil, "/etc/ssh/ssh_config", nil, nil)
	deps.RunSSHG = func(context.Context, ...string) (string, error) { return "", errors.New("ssh: boom") }
	for _, st := range Statuses(deps) {
		if st.ProbeError != "ssh: boom" {
			t.Errorf("%s ProbeError = %q, want the concrete probe error", st.Key, st.ProbeError)
		}
	}
}
