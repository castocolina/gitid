package globalssh

import (
	"strings"
	"sync"

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

// OptionState is the row state the Options sub-tab renders. The four visual
// states are the whole vocabulary: needs-action, already-set, set-but-differs
// and not-applicable (D-12, D-11). Attribution is carried separately on
// OptionStatus.Source so a value gitid did not parse is never worded as the
// user's choice.
type OptionState int

const (
	// StateNeedsAction is the ZERO value: the option is unset, or the
	// recommendation is still open because a baseline value does not match it.
	StateNeedsAction OptionState = iota
	// StateAlreadySet means the effective value equals the recommendation.
	StateAlreadySet
	// StateDiffers means the option is set somewhere to a non-recommended
	// value — a deliberate or external choice that is still flagged (D-12).
	StateDiffers
	// StateNotApplicable means the option does not apply on this machine
	// (wrong platform, OpenSSH too old or unverified, or nothing to verify).
	StateNotApplicable
)

// NotApplicableReason distinguishes the four situations that share the
// not-applicable visual state so each one can keep its own words.
type NotApplicableReason int

const (
	// ReasonNone is the zero value: the row is applicable.
	ReasonNone NotApplicableReason = iota
	// ReasonPlatform means the option does not exist on this OS (D-11).
	ReasonPlatform
	// ReasonVersionTooOld means the recommended value needs a newer OpenSSH (D-13).
	ReasonVersionTooOld
	// ReasonVersionUnverified means gitid could not read the OpenSSH version (D-13).
	ReasonVersionUnverified
	// ReasonNothingToVerify means IdentitiesOnly has no managed Host blocks to check.
	ReasonNothingToVerify
)

// OptionStatus is one policy row's machine-readable answer: the effective
// value, the provable source class (plus the exact file and line when gitid
// can name them), the D-10 recommendation facts, the row state, the
// not-applicable reason, and a probe-error note populated only from sources
// that row consulted.
type OptionStatus struct {
	Key                 string
	CurrentValue        string
	RecommendedValue    string
	Risk                string
	Source              SourceClass
	SourceFile          string
	SourceLine          int
	State               OptionState
	NotApplicableReason NotApplicableReason
	ProbeError          string
}

// stateFor classifies one policy row from its effective value, source class
// and platform. Branch order is load-bearing (06-REVIEWS.md cycle-2 HIGH):
//
//  1. policy Platform is darwin and goos is not darwin → StateNotApplicable
//     with ReasonPlatform (D-11). A value read on the wrong platform is
//     meaningless, so this runs before any comparison.
//  2. there is no value at all, from any source → StateNeedsAction.
//  3. value equals p.Recommended (case-insensitive) → StateAlreadySet,
//     regardless of src. State is a property of the VALUE; OptionStatus.Source
//     answers attribution and must not be consulted here. An already-set row
//     whose source is the baseline class is already-set because OpenSSH's own
//     default happens to be safe, not because anyone configured it.
//  4. src is the baseline class (unequal, and set in no file gitid or ssh
//     reads) → StateNeedsAction. The recommendation is still open and there
//     is nothing of the user's to overwrite.
//  5. value present, unequal, and set somewhere → StateDiffers (D-12).
//
// The visual state in step 5 is the same whether the value came from the
// user's own file or from somewhere outside it, because the UI vocabulary is
// four states. Attribution must not collapse: an explicitly present value is
// not evidence that the USER placed it there, and the word "your choice" is
// only correct for the gitid-parsed class.
//
// D-14 forbids any persisted decline record. These states are DERIVED from
// the machine on every read; re-showing a declined recommendation is
// intentional and acceptable because the copy respects the choice rather
// than nagging. Do not add a suppression file here.
func stateFor(p OptionPolicy, effective string, src SourceClass, goos string) (OptionState, NotApplicableReason) {
	if p.Platform == "darwin" && goos != "darwin" {
		return StateNotApplicable, ReasonPlatform
	}
	if effective == "" {
		return StateNeedsAction, ReasonNone
	}
	if strings.EqualFold(effective, p.Recommended) {
		return StateAlreadySet, ReasonNone
	}
	if src == SourceBaseline {
		return StateNeedsAction, ReasonNone
	}
	return StateDiffers, ReasonNone
}

// Evidence-source dependency map (06-REVIEWS.md HIGH on probe-error handling).
// A failure of a source a row never read is not evidence about that row:
//
//   - UseKeychain depends on the config read alone (D-02: Apple's OpenSSH
//     ssh -G never emits a usekeychain line even when the directive is set).
//   - IdentitiesOnly depends on the config read alone (per-alias conformance).
//   - StrictHostKeyChecking, ForwardAgent, HashKnownHosts, AddKeysToAgent
//     depend on the resolution probe and, for the baseline class only, on the
//     isolated probe.
const (
	keyUseKeychain    = "UseKeychain"
	keyIdentitiesOnly = "IdentitiesOnly"
	keyStrictHostKey  = "StrictHostKeyChecking"
	keyForwardAgent   = "ForwardAgent"
	keyHashKnownHosts = "HashKnownHosts"
	keyAddKeysToAgent = "AddKeysToAgent"
)

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
//   - a probe error                             -> SourceInconclusive with
//     ProbeError populated, ONLY for rows that
//     consulted the source that failed.
//
// The three independent evidence sources — the resolution probe, the isolated
// baseline probe, and the config read — run concurrently under their own
// bounded timeouts so three three-second budgets cannot compose into a
// nine-second screen activation.
//
// Statuses returns a slice with exactly len(Policy) entries and NEVER returns
// an error: this is the resolution of 06-UI-SPEC.md's unresolved
// probe-failure question, chosen fail-open because GSSH-01's advisory posture
// extends to the detection layer — a probe failure must degrade to a
// per-option advisory note, never an error screen or an empty pane.
func Statuses(deps Deps) []OptionStatus {
	var (
		hits       map[string]sshconfig.DirectiveHit
		config     []byte
		configErr  error
		eff        map[string]string
		effErr     error
		base       map[string]string
		baseErr    error
		sysPath    string
		sysContent []byte
		sysErr     error
		wg         sync.WaitGroup
	)
	wg.Add(5)
	go func() { defer wg.Done(); hits, configErr = fileHits(deps) }()
	go func() { defer wg.Done(); _, config, _ = deps.ReadConfig() }()
	go func() { defer wg.Done(); eff, effErr = effective(deps) }()
	go func() { defer wg.Done(); base, baseErr = baseline(deps) }()
	go func() { defer wg.Done(); sysPath, sysContent, sysErr = deps.ReadSystemConfig() }()
	wg.Wait()

	sysHits := make(map[string]sshconfig.DirectiveHit)
	if sysErr == nil {
		for _, hit := range sshconfig.ScanDirectives(sysContent, sysPath, policyKeys()) {
			sysHits[hit.Key] = hit
		}
	}

	conforming, total := 0, 0
	if configErr == nil {
		conforming, total, _, _ = perAliasFromContent(config)
	}
	resolutionErr := firstErr(effErr, baseErr)

	out := make([]OptionStatus, 0, len(Policy))
	for _, p := range Policy {
		st := OptionStatus{
			Key:              p.Key,
			RecommendedValue: p.Recommended,
			Risk:             p.Risk,
			State:            StateNeedsAction,
		}
		switch p.Key {
		case keyUseKeychain:
			// File-parse only (D-02). VERIFIED platform fact: ssh -G never
			// emits a usekeychain line even when the directive is set, so a
			// class derived from ssh -G cannot corroborate this row.
			if configErr != nil {
				st.Source = SourceInconclusive
				st.ProbeError = configErr.Error()
			} else if hit, ok := hits[p.Key]; ok {
				st.Source = SourceGitidParsed
				st.CurrentValue = hit.Value
				st.SourceFile = hit.SourcePath
				st.SourceLine = hit.Line
			} else {
				st.Source = SourceBaseline
			}
		case keyIdentitiesOnly:
			if configErr != nil {
				st.Source = SourceInconclusive
				st.ProbeError = configErr.Error()
			} else if total == 0 {
				st.Source = SourceBaseline
				st.State = StateNotApplicable
				st.NotApplicableReason = ReasonNothingToVerify
				out = append(out, st)
				continue
			} else if conforming == total {
				st.Source = SourceGitidParsed
				st.CurrentValue = p.Recommended
			} else {
				st.Source = SourceGitidParsed
				st.CurrentValue = "no"
				st.State = StateNeedsAction
				out = append(out, st)
				continue
			}
		default:
			if resolutionErr != nil {
				st.Source = SourceInconclusive
				st.ProbeError = resolutionErr.Error()
			} else if hit, ok := hits[p.Key]; ok {
				st.Source = SourceGitidParsed
				st.CurrentValue = hit.Value
				st.SourceFile = hit.SourcePath
				st.SourceLine = hit.Line
			} else {
				lk := strings.ToLower(p.Key)
				effVal := eff[lk]
				baseVal := base[lk]
				sysHit, sysHitOK := sysHits[p.Key]
				switch {
				case effVal == baseVal:
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
		}
		st.State, st.NotApplicableReason = stateFor(p, st.CurrentValue, st.Source, deps.GOOS)
		out = append(out, st)
	}
	return out
}

// firstErr returns the first non-nil error of the given set, or nil.
// Statuses uses it so the ProbeError note can name the first probe that
// refused to answer.
func firstErr(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
