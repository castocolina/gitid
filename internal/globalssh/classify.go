package globalssh

import (
	"strings"

	"github.com/castocolina/gitid/internal/sshconfig"
)

// SourceClass is where a probed value provably comes from. A claim is made
// only when the evidence supports it (06-REVIEWS.md's HIGH provenance
// finding): inferring /etc/ssh/ssh_config from the "effective but not in our
// file and not the baseline" three-way signature is unsound, because a user
// Include, a Match block, an environment-supplied option or a vendor drop-in
// produces exactly the same signature. The system file is named only when an
// explicit parse of that file found the directive.
type SourceClass int

const (
	// SourceGitidParsed means the value was found by the directive scan of the
	// per-user config gitid reads — a value gitid parsed itself, with a known
	// file and line.
	SourceGitidParsed SourceClass = iota
	// SourceOutsideGitid means the value is effective on the machine, absent
	// from everything gitid parsed, and not corroborated by the system-file
	// parse (including when that read failed). It is reported with the hedged
	// "somewhere gitid does not read" form, never asserted to a specific file.
	SourceOutsideGitid
	// SourceSystemFile means the value is effective, differs from the
	// baseline, and an explicit parse of the system ssh_config found the
	// directive there — the only case gitid may name that file.
	SourceSystemFile
	// SourceBaseline means the value is effective (or absent) and equal to the
	// value `ssh -G` resolves with no user configuration participating (the
	// isolated-config baseline), and absent from everything gitid parsed.
	SourceBaseline
	// SourceInconclusive means at least one probe did not answer, so no
	// provenance claim is possible; ProbeError carries that probe's error.
	SourceInconclusive
)

// OptionState is the row state the Options sub-tab renders. Plan 06-01
// computes a correct state only for HashKnownHosts; the full three/four-state
// model is plan 06-03's (D-12's "set, differs" word state and D-11's
// not-applicable row).
type OptionState int

const (
	// StateNeedsAction is the ZERO value: the option is unset, or set to a
	// value that differs from the recommendation, so applying it is meaningful.
	StateNeedsAction OptionState = iota
	// StateAlreadySet means the effective value equals the recommendation.
	StateAlreadySet
	// StateDiffers means the option is explicitly set to a non-recommended
	// value — a deliberate choice that is still flagged (D-12 word state,
	// 06-03).
	StateDiffers
	// StateNotApplicable means the option does not exist on this platform
	// (e.g. UseKeychain off macOS — D-11 row state, 06-03).
	StateNotApplicable
)

// OptionStatus is one policy row's machine-readable answer: the effective
// value, the provable source class (plus the exact file and line when gitid
// can name them), the D-10 recommendation facts, the row state, and a
// probe-error note (non-empty exactly when Source is SourceInconclusive).
type OptionStatus struct {
	Key              string
	CurrentValue     string
	RecommendedValue string
	Risk             string
	Source           SourceClass
	SourceFile       string
	SourceLine       int
	State            OptionState
	ProbeError       string
}

// Statuses probes the machine for every D-10 policy row and classifies each
// value's provenance per D-01, with the review's correction — a claim is made
// only when the evidence supports it:
//
//   - found by the gitid-visible file scan       -> SourceGitidParsed, with the
//     file and line the scan reported;
//   - absent from that scan, effective and equal
//     to the isolated-config baseline            -> SourceBaseline;
//   - absent, effective, different from baseline,
//     AND found by a system-config parse         -> SourceSystemFile, with that
//     file and line;
//   - absent, effective, different from baseline,
//     and NOT corroborated (including when the
//     system-config read failed)                 -> SourceOutsideGitid;
//   - any probe error                             -> SourceInconclusive with
//     ProbeError populated, for every row the scan did not already claim
//     (the scan's own read failure degrades to "cannot name", not to a wrong
//     name).
//
// Statuses returns a slice with exactly len(Policy) entries and NEVER returns
// an error: this is the resolution of 06-UI-SPEC.md's unresolved
// probe-failure question, chosen fail-open because GSSH-01's advisory posture
// extends to the detection layer — a probe failure must degrade to a
// per-option advisory note, never an error screen or an empty pane.
func Statuses(deps Deps) []OptionStatus {
	// A fileHits failure never empties the pane: an empty hits map just means
	// "cannot name" and the rows still classify from the probes.
	hits, _ := fileHits(deps)

	eff, effErr := effective(deps)
	base, baseErr := baseline(deps)
	probeErr := firstErr(effErr, baseErr)

	sysHits := make(map[string]sshconfig.DirectiveHit)
	sysPath, sysContent, sysErr := deps.ReadSystemConfig()
	if sysErr == nil {
		for _, hit := range sshconfig.ScanDirectives(sysContent, sysPath, policyKeys()) {
			sysHits[hit.Key] = hit
		}
	}

	out := make([]OptionStatus, 0, len(Policy))
	for _, p := range Policy {
		st := OptionStatus{
			Key:              p.Key,
			RecommendedValue: p.Recommended,
			Risk:             p.Risk,
			State:            StateNeedsAction,
		}
		lk := strings.ToLower(p.Key)
		if hit, ok := hits[p.Key]; ok {
			st.Source = SourceGitidParsed
			st.CurrentValue = hit.Value
			st.SourceFile = hit.SourcePath
			st.SourceLine = hit.Line
		} else if probeErr != nil {
			st.Source = SourceInconclusive
			st.ProbeError = probeErr.Error()
		} else {
			effVal := eff[lk]
			baseVal := base[lk]
			sysHit, sysHitOK := sysHits[p.Key]
			switch {
			case effVal == baseVal:
				// present-and-equal AND absent-from-both collapse to the same
				// provable claim: the option is not set in any file gitid
				// parses, and the reported value is the value the isolated
				// config probe returned — never an invented constant.
				st.Source = SourceBaseline
				st.CurrentValue = baseVal
			case sysHitOK:
				st.Source = SourceSystemFile
				st.SourceFile = sysHit.SourcePath
				st.SourceLine = sysHit.Line
				st.CurrentValue = effVal
			default:
				st.Source = SourceOutsideGitid
				st.CurrentValue = effVal
			}
		}
		if p.Key == "HashKnownHosts" {
			// Plan 06-01 owns a correct row STATE only for the tracer option;
			// the remaining five rows carry their probed values with the
			// needs-action zero value until 06-03 ships the full state model.
			if st.CurrentValue == p.Recommended {
				st.State = StateAlreadySet
			} else {
				st.State = StateNeedsAction
			}
		}
		out = append(out, st)
	}
	return out
}

// firstErr returns the first non-nil error of the given set, or nil.
// Statuses uses it so the ProbeError note can name the first probe that
// refused to answer.
func firstErr(errs ...error) error {
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}
