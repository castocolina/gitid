package globalssh

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// resolutionDependentKeys are the four rows that consult the generic
// resolution / isolated probes. Named here so tests assert by this map, not
// by "all rows minus two". WR-10: test-only (referenced only from this
// file), so it lives with the test that uses it rather than in production
// classify.go.
func resolutionDependentKeys() []string {
	return []string{keyStrictHostKey, keyForwardAgent, keyHashKnownHosts, keyAddKeysToAgent}
}

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

func TestStatusesClassifiesAllPolicyRows(t *testing.T) {
	plain := strings.ReplaceAll(cannedResolved, "hashknownhosts no", "hashknownhosts yes")
	deps := depsForOut(plain, cannedResolved, "~/.ssh/config",
		[]byte("HashKnownHosts yes\n"), "/etc/ssh/ssh_config", nil, nil)
	statuses := statusByKey(t, Statuses(deps))
	if statuses["HashKnownHosts"].State != StateAlreadySet {
		t.Errorf("HashKnownHosts state = %v, want StateAlreadySet", statuses["HashKnownHosts"].State)
	}
	if statuses["ForwardAgent"].State != StateAlreadySet {
		t.Errorf("ForwardAgent state = %v, want StateAlreadySet for its safe default", statuses["ForwardAgent"].State)
	}
	if statuses["IdentitiesOnly"].State != StateNotApplicable || statuses["IdentitiesOnly"].NotApplicableReason != ReasonNothingToVerify {
		t.Errorf("IdentitiesOnly = (%v, %v), want not applicable with nothing to verify", statuses["IdentitiesOnly"].State, statuses["IdentitiesOnly"].NotApplicableReason)
	}
}

// TestStatusesProbeErrorCarriesMessage proves the inconclusive rows carry the
// concrete probe error text (the probe-error note the pane renders), not an
// invented constant.
func TestStatusesProbeErrorCarriesMessage(t *testing.T) {
	deps := depsForOut("", "", "~/.ssh/config", nil, "/etc/ssh/ssh_config", nil, nil)
	deps.RunSSHG = func(context.Context, ...string) (string, error) { return "", errors.New("ssh: boom") }
	statuses := statusByKey(t, Statuses(deps))
	for _, key := range []string{"StrictHostKeyChecking", "ForwardAgent", "HashKnownHosts", "AddKeysToAgent"} {
		if statuses[key].ProbeError != "ssh: boom" {
			t.Errorf("%s ProbeError = %q, want the concrete probe error", key, statuses[key].ProbeError)
		}
	}
}

// TestStatusesProbeErrorDegradesToNotApplicable is the WR-14 regression: a
// row whose Source is SourceInconclusive (a probe it depends on failed) must
// render as StateNotApplicable/ReasonProbeFailed, never StateNeedsAction.
// Before the fix, the inconclusive branches left CurrentValue empty and fell
// through to stateFor's effective=="" case, which returns StateNeedsAction —
// a machine with no ssh on PATH rendered every resolution-dependent row as a
// selectable "needs action" checkbox despite the probe having failed.
func TestStatusesProbeErrorDegradesToNotApplicable(t *testing.T) {
	deps := depsForOut("", "", "~/.ssh/config", nil, "/etc/ssh/ssh_config", nil, nil)
	deps.RunSSHG = func(context.Context, ...string) (string, error) { return "", errors.New("ssh: boom") }
	statuses := statusByKey(t, Statuses(deps))
	for _, key := range resolutionDependentKeys() {
		st := statuses[key]
		if st.Source != SourceInconclusive {
			t.Fatalf("%s Source = %v, want SourceInconclusive (test setup)", key, st.Source)
		}
		if st.State != StateNotApplicable {
			t.Errorf("%s State = %v, want StateNotApplicable; WR-14 regressed", key, st.State)
		}
		if st.NotApplicableReason != ReasonProbeFailed {
			t.Errorf("%s NotApplicableReason = %v, want ReasonProbeFailed; WR-14 regressed", key, st.NotApplicableReason)
		}
		if st.ProbeError == "" {
			t.Errorf("%s ProbeError is empty; the advisory explanation must survive the degrade", key)
		}
	}
}

// TestStatusesConfigReadErrorDegradesToNotApplicable proves the same WR-14
// fix for the config-read-dependent rows (UseKeychain, IdentitiesOnly),
// which consult deps.ReadConfig rather than the resolution probe.
func TestStatusesConfigReadErrorDegradesToNotApplicable(t *testing.T) {
	deps := depsForOut(cannedResolved, cannedResolved, "~/.ssh/config", nil, "/etc/ssh/ssh_config", nil, nil)
	deps.GOOS = "darwin"
	deps.ReadConfig = func() (string, []byte, error) { return "", nil, errors.New("config: boom") }
	statuses := statusByKey(t, Statuses(deps))
	for _, key := range []string{"UseKeychain", "IdentitiesOnly"} {
		st := statuses[key]
		if st.Source != SourceInconclusive {
			t.Fatalf("%s Source = %v, want SourceInconclusive (test setup)", key, st.Source)
		}
		if st.State != StateNotApplicable {
			t.Errorf("%s State = %v, want StateNotApplicable; WR-14 regressed", key, st.State)
		}
		if st.NotApplicableReason != ReasonProbeFailed {
			t.Errorf("%s NotApplicableReason = %v, want ReasonProbeFailed; WR-14 regressed", key, st.NotApplicableReason)
		}
	}
}

// TestStatusesReadsConfigOnce is the WR-15 regression: Statuses must call
// deps.ReadConfig() exactly ONCE and derive both the directive hits AND the
// perAliasFromContent input from that single read. Before the fix, two
// independent goroutines each called deps.ReadConfig() concurrently — a
// write landing between the two reads could leave the hits (from the first
// call) and the per-alias conformance count (from the second call, whose
// error was silently discarded) computed against two DIFFERENT snapshots of
// the file.
func TestStatusesReadsConfigOnce(t *testing.T) {
	var calls int32
	deps := depsForOut(cannedResolved, cannedResolved, "~/.ssh/config", nil, "/etc/ssh/ssh_config", nil, nil)
	deps.ReadConfig = func() (string, []byte, error) {
		atomic.AddInt32(&calls, 1)
		return "~/.ssh/config", []byte("StrictHostKeyChecking accept-new\n"), nil
	}
	Statuses(deps)
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("deps.ReadConfig called %d times, want exactly 1; WR-15 regressed", got)
	}
}

// TestStatusesConfigReadErrorNeverLeavesStaleHits proves the single-read
// fix's error-propagation half: when deps.ReadConfig fails, hits AND the
// per-alias conformance count must BOTH degrade together (SourceInconclusive
// for the config-dependent rows), never partially succeed from a second,
// separately-erroring read.
func TestStatusesConfigReadErrorNeverLeavesStaleHits(t *testing.T) {
	deps := depsForOut(cannedResolved, cannedResolved, "~/.ssh/config", nil, "/etc/ssh/ssh_config", nil, nil)
	deps.GOOS = "darwin"
	deps.ReadConfig = func() (string, []byte, error) {
		return "", nil, errors.New("config: boom")
	}
	statuses := statusByKey(t, Statuses(deps))
	for _, key := range []string{"UseKeychain", "IdentitiesOnly"} {
		if statuses[key].Source != SourceInconclusive {
			t.Errorf("%s Source = %v, want SourceInconclusive", key, statuses[key].Source)
		}
		if statuses[key].ProbeError == "" {
			t.Errorf("%s ProbeError is empty, want the config read error", key)
		}
	}
}

func TestStateFor(t *testing.T) {
	forwardAgent, ok := PolicyFor("ForwardAgent")
	if !ok {
		t.Fatal("ForwardAgent policy missing")
	}
	useKeychain, ok := PolicyFor("UseKeychain")
	if !ok {
		t.Fatal("UseKeychain policy missing")
	}

	cases := []struct {
		name      string
		policy    OptionPolicy
		effective string
		source    SourceClass
		goos      string
		wantState OptionState
		wantWhy   NotApplicableReason
	}{
		{"missing value", forwardAgent, "", SourceBaseline, "linux", StateNeedsAction, ReasonNone},
		{"safe default matches", forwardAgent, "no", SourceBaseline, "linux", StateAlreadySet, ReasonNone},
		{"different baseline", forwardAgent, "yes", SourceBaseline, "linux", StateNeedsAction, ReasonNone},
		{"explicit different", forwardAgent, "yes", SourceGitidParsed, "linux", StateDiffers, ReasonNone},
		{"case insensitive", forwardAgent, "NO", SourceOutsideGitid, "linux", StateAlreadySet, ReasonNone},
		{"platform gate first", useKeychain, "yes", SourceGitidParsed, "linux", StateNotApplicable, ReasonPlatform},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotState, gotWhy := stateFor(tc.policy, tc.effective, tc.source, tc.goos)
			if gotState != tc.wantState || gotWhy != tc.wantWhy {
				t.Fatalf("stateFor() = (%v, %v), want (%v, %v)", gotState, gotWhy, tc.wantState, tc.wantWhy)
			}
		})
	}
}

func TestStateForEqualityWinsForEverySource(t *testing.T) {
	p, ok := PolicyFor("ForwardAgent")
	if !ok {
		t.Fatal("ForwardAgent policy missing")
	}
	for _, source := range []SourceClass{SourceGitidParsed, SourceOutsideGitid, SourceSystemFile, SourceBaseline, SourceInconclusive} {
		state, reason := stateFor(p, "no", source, "linux")
		if state != StateAlreadySet || reason != ReasonNone {
			t.Errorf("source %v: stateFor() = (%v, %v), want (StateAlreadySet, ReasonNone)", source, state, reason)
		}
	}
}

func TestStatusesDiffersKeepsAttributionSeparate(t *testing.T) {
	p, ok := PolicyFor("ForwardAgent")
	if !ok {
		t.Fatal("ForwardAgent policy missing")
	}
	userState, _ := stateFor(p, "yes", SourceGitidParsed, "linux")
	outsideState, _ := stateFor(p, "yes", SourceOutsideGitid, "linux")
	if userState != StateDiffers || outsideState != StateDiffers {
		t.Fatalf("unequal sources produced %v and %v, want StateDiffers for both", userState, outsideState)
	}
}

func TestStatusesUseKeychainIgnoresResolutionOutput(t *testing.T) {
	deps := depsForOut("usekeychain no\n", cannedResolved, "~/.ssh/config", []byte("UseKeychain yes\n"), "/etc/ssh/ssh_config", nil, nil)
	deps.GOOS = "darwin"
	status := statusByKey(t, Statuses(deps))["UseKeychain"]
	if status.CurrentValue != "yes" || status.Source != SourceGitidParsed || status.State != StateAlreadySet {
		t.Fatalf("UseKeychain = %+v, want parsed file value yes and already set", status)
	}
}

func TestStatusesResolutionProbeErrorLeavesFileRowsUnchanged(t *testing.T) {
	config := []byte("UseKeychain yes\n# BEGIN gitid managed: personal\nHost personal.github.com\n  IdentitiesOnly yes\n# END gitid managed: personal\n")
	okDeps := depsForOut(cannedResolved, cannedResolved, "~/.ssh/config", config, "/etc/ssh/ssh_config", nil, nil)
	okDeps.GOOS = "darwin"
	ok := statusByKey(t, Statuses(okDeps))
	failDeps := depsForOut(cannedResolved, cannedResolved, "~/.ssh/config", config, "/etc/ssh/ssh_config", nil, nil)
	failDeps.GOOS = "darwin"
	failDeps.RunSSHG = func(context.Context, ...string) (string, error) { return "", errors.New("ssh: boom") }
	fail := statusByKey(t, Statuses(failDeps))
	for _, key := range []string{"UseKeychain", "IdentitiesOnly"} {
		if fail[key].State != ok[key].State || fail[key].ProbeError != ok[key].ProbeError {
			t.Errorf("%s changed after resolution probe failure: ok=%+v fail=%+v", key, ok[key], fail[key])
		}
	}
	if ok["UseKeychain"].State != StateAlreadySet && ok["IdentitiesOnly"].State != StateAlreadySet {
		t.Fatal("fixture must keep at least one file-derived row already set")
	}
	// WR-14: a resolution probe failure degrades these rows to
	// not-applicable/probe-failed (never needs-action — that would render a
	// selectable checkbox inviting the user to "fix" something gitid could
	// not read) while still carrying the ProbeError explanation.
	for _, key := range resolutionDependentKeys() {
		if fail[key].State != StateNotApplicable || fail[key].NotApplicableReason != ReasonProbeFailed || fail[key].ProbeError == "" {
			t.Errorf("%s = %+v, want not-applicable/probe-failed with probe error", key, fail[key])
		}
		if fail[key].State == StateAlreadySet {
			t.Errorf("%s must not promote to already-set on a failed resolution probe", key)
		}
	}
}

func TestStatusesIdentitiesOnlyNothingToVerify(t *testing.T) {
	deps := depsForOut(cannedResolved, cannedResolved, "~/.ssh/config", nil, "/etc/ssh/ssh_config", nil, nil)
	st := statusByKey(t, Statuses(deps))["IdentitiesOnly"]
	if st.State != StateNotApplicable || st.NotApplicableReason != ReasonNothingToVerify {
		t.Fatalf("IdentitiesOnly = (%v, %v), want not-applicable with nothing to verify", st.State, st.NotApplicableReason)
	}
	if st.State == StateAlreadySet {
		t.Fatal("an empty inventory is not conformance")
	}
}

func TestStatusesIdentitiesOnlyConformance(t *testing.T) {
	all := []byte("# BEGIN gitid managed: personal\nHost personal.github.com\n  IdentitiesOnly yes\n# END gitid managed: personal\n")
	one := []byte("# BEGIN gitid managed: personal\nHost personal.github.com\n  IdentitiesOnly yes\n# END gitid managed: personal\n# BEGIN gitid managed: work\nHost work.github.com\n  Hostname ssh.github.com\n# END gitid managed: work\n")
	allState := statusByKey(t, Statuses(depsForOut(cannedResolved, cannedResolved, "~/.ssh/config", all, "/etc/ssh/ssh_config", nil, nil)))["IdentitiesOnly"]
	if allState.State != StateAlreadySet {
		t.Errorf("all-conforming IdentitiesOnly = %v, want already-set", allState.State)
	}
	oneState := statusByKey(t, Statuses(depsForOut(cannedResolved, cannedResolved, "~/.ssh/config", one, "/etc/ssh/ssh_config", nil, nil)))["IdentitiesOnly"]
	if oneState.State != StateNeedsAction {
		t.Errorf("one-offender IdentitiesOnly = %v, want needs-action", oneState.State)
	}
}

func TestStatusesConfigReadErrorIsIsolated(t *testing.T) {
	deps := depsForOut(cannedResolved, cannedResolved, "~/.ssh/config", nil, "/etc/ssh/ssh_config", nil, nil)
	deps.ReadConfig = func() (string, []byte, error) { return "", nil, errors.New("config denied") }
	statuses := statusByKey(t, Statuses(deps))
	for _, key := range []string{"UseKeychain", "IdentitiesOnly"} {
		if statuses[key].ProbeError != "config denied" {
			t.Errorf("%s ProbeError = %q, want config error", key, statuses[key].ProbeError)
		}
	}
	if statuses["ForwardAgent"].ProbeError != "" || statuses["ForwardAgent"].State != StateAlreadySet {
		t.Errorf("ForwardAgent = %+v, want its resolution-derived safe state", statuses["ForwardAgent"])
	}
}

func TestStatusesDeterministic(t *testing.T) {
	deps := depsForOut(cannedResolved, cannedResolved, "~/.ssh/config", []byte("UseKeychain yes\n"), "/etc/ssh/ssh_config", nil, nil)
	deps.GOOS = "darwin"
	first := Statuses(deps)
	second := Statuses(deps)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("Statuses changed without machine change:\nfirst=%+v\nsecond=%+v", first, second)
	}
}

func TestStatusesConcurrentLatency(t *testing.T) {
	sleep := 90 * time.Millisecond
	var mu sync.Mutex
	calls := 0
	deps := depsForOut(cannedResolved, cannedResolved, "~/.ssh/config", []byte("UseKeychain yes\n"), "/etc/ssh/ssh_config", nil, nil)
	deps.GOOS = "darwin"
	deps.RunSSHG = func(_ context.Context, _ ...string) (string, error) {
		time.Sleep(sleep)
		mu.Lock()
		calls++
		mu.Unlock()
		return cannedResolved, nil
	}
	read := deps.ReadConfig
	deps.ReadConfig = func() (string, []byte, error) { time.Sleep(sleep); return read() }
	sys := deps.ReadSystemConfig
	deps.ReadSystemConfig = func() (string, []byte, error) { time.Sleep(sleep); return sys() }
	started := time.Now()
	_ = Statuses(deps)
	elapsed := time.Since(started)
	if elapsed >= 3*sleep {
		t.Fatalf("Statuses elapsed %s, want concurrent runtime below the 3-probe sum", elapsed)
	}
	if calls != 2 {
		t.Fatalf("RunSSHG called %d times, want 2 concurrent probes", calls)
	}
}
