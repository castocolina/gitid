package tuikit

// backend_stub_test.go supplies the test-only tuikit.Backend every internal
// test constructs its App with.
//
// These tests are `package tuikit` — they reach unexported screens, models
// and helpers, so they cannot move to an external test package. They also
// cannot import internal/dummytui for its FixtureBackend: dummytui imports
// tuikit, so that would be an import cycle. The seam therefore gets a
// SECOND implementation here, in a _test.go file (which never appears in
// `go list -deps ./internal/tuikit`, so the no-backend import-graph gate is
// unaffected).
//
// stubBackend reproduces the FIXTURE behavior these tests were written
// against, value for value: the same eight identity rows, the same five
// health findings and attribution, the same create-flow strings. It is a
// deliberate copy — the alternative (importing the fixtures) is the import
// cycle above. dummytui's own TestFixtureConsistency keeps the fixture side
// coherent; the assertions in this package keep this side coherent, and the
// e2e PTY walk drives the REAL wiring (tuikit.NewApp + dummytui's
// FixtureBackend) end to end, so a divergence between the two cannot hide.

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/sshconfig"
)

// ---------------------------------------------------------------------------
// Fixture data (mirrors internal/dummytui/data.go).
// ---------------------------------------------------------------------------

// stubIdentityRow is the fixture row shape (dummytui.IdentityManagerRow).
type stubIdentityRow struct {
	Name            string
	State           string
	SSHHost         string
	KeyPath         string
	GitFragmentPath string
	Note            string
}

// stubIdentityRows mirrors dummytui.IdentityManagerRows — one row per MGR-02
// state label, so a populated list demonstrates every label at once.
var stubIdentityRows = []stubIdentityRow{
	{Name: "personal", State: "complete", SSHHost: "personal.github.com", KeyPath: "~/.ssh/id_ed25519_personal", GitFragmentPath: "~/.gitconfig.d/personal", Note: "SSH Host block and Git fragment both present."},
	{Name: "work", State: "incomplete", SSHHost: "work.github.com", KeyPath: "~/.ssh/id_ed25519_work", Note: "SSH Host block present; no Git identity configured for this alias."},
	{Name: "opensource", State: "git-only", GitFragmentPath: "~/.gitconfig.d/opensource", Note: "Git identity relies on the global SSH config; no own Host block."},
	{Name: "archived", State: "key-unused", KeyPath: "~/.ssh/id_ed25519_archived", Note: "Key file exists on disk but no identity references it."},
	{Name: "staging", State: "key-used-ssh-only", SSHHost: "staging.github.com", KeyPath: "~/.ssh/id_ed25519_staging", Note: "Key referenced by a Host block; not wired for Git commit signing."},
	{Name: "clientA", State: "key-used-both", SSHHost: "clienta.github.com", KeyPath: "~/.ssh/id_ed25519_clientA", GitFragmentPath: "~/.gitconfig.d/clientA", Note: "Key wired for both SSH auth and Git commit signing."},
	{Name: "clientB", State: "key-missing", SSHHost: "clientb.github.com", KeyPath: "~/.ssh/id_ed25519_clientB", Note: "Host block references a key file that is absent from disk."},
	{Name: "legacy", State: "fragment-path-missing", SSHHost: "legacy.github.com", GitFragmentPath: "~/.gitconfig.d/legacy", Note: "includeIf points at a Git fragment file that does not exist."},
}

// stubHealthFindings mirrors dummytui.HealthFindings — byte-identical
// ids/sections/severities/copy, covering all four severity levels.
var stubHealthFindings = []HealthFinding{
	{
		ID: "ssh-key-perms-archived", Section: "SSH", Severity: SeverityCritical, Family: "Permissions",
		Title:        "Private key is world-readable",
		Explanation:  "~/.ssh/id_ed25519_archived is mode 0644 -- gitid-managed keys must be 0600. Any other account on this machine can read the key material.",
		SuggestedFix: "chmod 0600 ~/.ssh/id_ed25519_archived.",
		Fixable:      true,
	},
	{
		ID: "ssh-identitiesonly-contradiction", Section: "SSH", Severity: SeverityError, Family: "Coherence",
		Title:        "IdentitiesOnly no contradicts an explicit IdentityFile",
		Explanation:  "Host clientb.github.com sets IdentitiesOnly no while also naming IdentityFile ~/.ssh/id_ed25519_clientB -- ssh may still offer every other key it knows before falling back to the one explicitly configured (HLTH-04).",
		SuggestedFix: "Set IdentitiesOnly yes on the clientb.github.com Host block.",
		Fixable:      true,
	},
	{
		ID: "git-includeif-missing-fragment", Section: "Git", Severity: SeverityError, Family: "Orphans",
		Title:        "includeIf targets a missing fragment",
		Explanation:  "[includeIf \"gitdir:~/legacy/\"] in ~/.gitconfig points at ~/.gitconfig.d/legacy, which does not exist on disk -- commits made under ~/legacy/ silently fall back to your global git identity instead of \"legacy\" (HLTH-04).",
		SuggestedFix: "Restore ~/.gitconfig.d/legacy, or repoint the includeIf.",
		Fixable:      true,
	},
	{
		ID: "ssh-duplicate-host-star", Section: "SSH", Severity: SeverityWarning, Family: "Redundancy",
		Title:        "Duplicate Host * stanza",
		Explanation:  "~/.ssh/config defines Host * twice -- line 4 and line 41. The second stanza silently overrides directives set by the first (HLTH-03).",
		SuggestedFix: "Merge the two Host * stanzas into one.",
		Fixable:      true,
	},
	{
		ID: "git-opensource-no-host-block", Section: "Git", Severity: SeverityInfo, Family: "Overlap",
		Title:       "opensource has no dedicated SSH Host block",
		Explanation: "The \"opensource\" Git identity resolves correctly via its includeIf, but relies entirely on the global SSH config -- there is no gitid-managed Host block scoping which key ssh offers for it. Informational only.",
	},
}

// stubFindingIdentity attributes each seeded finding to the identity it is
// about; ssh-duplicate-host-star stays global (no entry).
var stubFindingIdentity = map[string]string{
	"ssh-key-perms-archived":           "archived",
	"ssh-identitiesonly-contradiction": "clientB",
	"git-includeif-missing-fragment":   "legacy",
	"git-opensource-no-host-block":     "opensource",
}

// Create-flow fixture literals (mirroring dummytui's CreateFlow*/GitScreen*
// constants the pre-extraction tests asserted against).
const (
	// CreateFlowTestTmpConfig is the throwaway temp config connectivity
	// tests run against — the live ~/.ssh/config is untouched.
	CreateFlowTestTmpConfig = "/tmp/gitid-test-a1b2c3.config"
	// CreateFlowTestStage1Command is the stage-1 direct connectivity test
	// for the canonical "personal" fixture identity.
	CreateFlowTestStage1Command = "ssh -T -F " + CreateFlowTestTmpConfig + " -p 443 -i ~/.ssh/id_ed25519_personal git@ssh.github.com"

	// gitScreenFragmentFile is the canonical fixture fragment path.
	gitScreenFragmentFile = "~/.gitconfig.d/personal"
	// gitScreenIncludeIfGitdirLine is the gitdir-match includeIf block.
	gitScreenIncludeIfGitdirLine = `[includeIf "gitdir:~/personal/"]
    path = ` + gitScreenFragmentFile
	// gitScreenIncludeIfHasconfigLine is the hasconfig-match alternative.
	gitScreenIncludeIfHasconfigLine = `[includeIf "hasconfig:remote.*.url:git@personal.github.com:*/**"]
    path = ` + gitScreenFragmentFile
	// gitScreenMatchStrategyDefault is the default includeIf match strategy.
	gitScreenMatchStrategyDefault = "gitdir"
)

// gitScreenMatchStrategyPreview keys the live includeIf preview by match
// strategy ("both" = two blocks, OR semantics).
var gitScreenMatchStrategyPreview = map[string]string{
	"gitdir":    gitScreenIncludeIfGitdirLine,
	"hasconfig": gitScreenIncludeIfHasconfigLine,
	"both":      gitScreenIncludeIfGitdirLine + "\n\n" + gitScreenIncludeIfHasconfigLine,
}

// Seed is the fixture initial state these tests assert against — the same
// value stubBackend.InitialState returns, exposed as a helper because many
// tests reduce over a seeded state directly.
func Seed() DemoState { return stubBackend{}.InitialState() }

// ---------------------------------------------------------------------------
// The stub Backend.
// ---------------------------------------------------------------------------

// stubBackend is the test-only Backend: the fixture behavior, in memory.
//
// gitStepAlwaysDisabled/gitStepReason drive GitStepDisabledReason() (D-19).
// The zero value — every existing `stubBackend{}` call site — keeps the
// UNCHANGED dummy-style form-validity gate; only a test that explicitly sets
// gitStepAlwaysDisabled simulates the real binary's unconditional disable.
type stubBackend struct {
	NoopIdentityPlanner
	NoopGlobalSSHPlanner
	// NoopGlobalGitPlanner is intentionally NOT embedded: the stub provides
	// fixture-driven implementations of GlobalGitPlanner below, exactly
	// mirroring how it handles GlobalSSHPlanner. A missing real implementation
	// must be a compile error, not a silent sentinel.
	NoopGlobalSSHOverridePlanner
	NoopGlobalGitOverridePlanner
	NoopSSHStoragePlanner
	// NoopSSHPropertiesBrowser is embedded for the SAME reason
	// NoopGlobalSSHPlanner is above: AllSSHDirectives is overridden directly
	// below (an empty, successful default rather than the Noop sentinel —
	// most tests never touch the properties sub-tab and should not have to
	// opt out of a canned error), so the embed is a compile-time safety net
	// only, never actually reached.
	NoopSSHPropertiesBrowser
	// NoopGitPropertiesBrowser is embedded for the SAME reason
	// NoopSSHPropertiesBrowser is above: AllGitSetKeys is overridden directly
	// below (an empty, successful default rather than the Noop sentinel —
	// most tests never touch the Set-keys sub-tab and should not have to
	// opt out of a canned error), so the embed is a compile-time safety net
	// only, never actually reached.
	NoopGitPropertiesBrowser
	// NoopGitCustomKeyPlanner is intentionally NOT embedded: the stub
	// provides fixture-driven implementations of GitCustomKeyPlanner below,
	// exactly mirroring how it handles GlobalGitPlanner. A missing real
	// implementation must be a compile error, not a silent sentinel.
	gitStepAlwaysDisabled bool
	gitStepReason         string
	keyActionErr          error
	deletePlanErr         error
	deletePlanFn          func(name, scope string) (DeletePlanView, error)
	keyCeremonyPlanErr    error
	keyCeremonyPlanFn     func(name, mode string) (KeyCeremonyView, error)
	keyCommit             KeyCommitMsg
	// Global-SSH seam overrides (zero values keep the fixture projection from
	// the frozen GlobalSSHOptions below).
	sshOptions     []GlobalSSHOptionView
	sshOptionsErr  error
	sshApplyPlan   GlobalSSHApplyPlanView
	sshApplyPlanFn func(keys []string) (GlobalSSHApplyPlanView, error)
	sshCommitMsg   GlobalSSHCommitMsg
	// SSH properties (plan 09.5-01) seam overrides — zero values keep an
	// empty, successful default (see the NoopSSHPropertiesBrowser doc
	// comment above for why this default is a success, not the Noop error).
	sshDirectives    []SSHDirectiveView
	sshDirectivesErr error
	// Git properties (plan 09.5-02) seam overrides — zero values keep an
	// empty, successful default (see the NoopGitPropertiesBrowser doc
	// comment above for why this default is a success, not the Noop error).
	gitSetKeys    []GitSetKeyView
	gitSetKeysErr error
	// Global-Git seam overrides (zero values keep the fixture projection from
	// fixtureGlobalGitOptionViews() below — mirrors the SSH seam pattern).
	gitOptions     []GlobalGitOptionView
	gitOptionsErr  error
	gitApplyPlan   GlobalGitApplyPlanView
	gitApplyPlanFn func(keys []string) (GlobalGitApplyPlanView, error)
	gitCommitMsg   GlobalGitCommitMsg
	// Global-SSH override seam overrides (plan 09.6-01) — test hooks for the
	// new staged-override pathway. Do NOT embed NoopGlobalSSHOverridePlanner —
	// a missing real implementation must be a compile error.
	sshOverridePlan     GlobalSSHApplyPlanView
	sshOverridePlanFn   func(keys []string, overrides []OverrideRequest) (GlobalSSHApplyPlanView, error)
	sshOverrideCommit   GlobalSSHCommitMsg
	sshOverrideCommitFn func(keys []string, overrides []OverrideRequest) tea.Cmd
	// Global-Git override seam overrides (plan 09.6-01) — test hooks for the
	// new staged-override pathway. Do NOT embed NoopGlobalGitOverridePlanner —
	// a missing real implementation must be a compile error.
	gitOverridePlan     GlobalGitApplyPlanView
	gitOverridePlanFn   func(keys []string, overrides []OverrideRequest) (GlobalGitApplyPlanView, error)
	gitOverrideCommit   GlobalGitCommitMsg
	gitOverrideCommitFn func(keys []string, overrides []OverrideRequest) tea.Cmd
	// Custom-key seam overrides (plan 09.5-03) — zero values keep the
	// ceremony's target/backup fallback, mirroring the gitApplyPlan/
	// gitCommitMsg pattern immediately above.
	customKeyPlan   GitCustomKeyPlanView
	customKeyPlanFn func(key, value string) (GitCustomKeyPlanView, error)
	customKeyCommit GitCustomKeyCommitMsg
	customKeyFn     func(key, value string) tea.Cmd
	// Custom-directive seam overrides (plan 09.5-04) — mirrors the
	// customKey* fields immediately above, plus a THIRD (validate) seam for
	// the un-skippable stage-2 proof. Do NOT embed
	// NoopSSHCustomDirectivePlanner — a missing real implementation must be
	// a compile error, not a silent sentinel.
	sshDirectiveValidateFn func(name, value string) tea.Cmd
	sshDirectivePlan       SSHCustomDirectivePlanView
	sshDirectivePlanFn     func(name, value string) (SSHCustomDirectivePlanView, error)
	sshDirectiveCommit     SSHCustomDirectiveCommitMsg
	sshDirectiveCommitFn   func(name, value string) tea.Cmd
	// Fallback-author seam overrides (zero values keep empty/unset fields
	// so existing tests stay green). Do NOT embed
	// NoopGitFallbackAuthorPlanner — a missing real implementation must
	// be a compile error, not a silent sentinel.
	fallbackState     GitFallbackAuthorView
	fallbackStateErr  error
	fallbackPlan      GitFallbackAuthorPlanView
	fallbackPlanFn    func(name, email string) (GitFallbackAuthorPlanView, error)
	fallbackCommitMsg GitFallbackAuthorCommitMsg
	fallbackCommitFn  func(name, email string) tea.Cmd
	gitCommitFn       func(keys []string) tea.Cmd
	// Global-gitignore seam overrides (zero values keep a wired-at-managed
	// empty view so existing tests stay green).
	gignState       GlobalGitIgnoreView
	gignStateErr    error
	gignStateFn     func() (GlobalGitIgnoreView, error)
	gignApplyPlan   GlobalGitIgnoreApplyPlanView
	gignApplyPlanFn func(content string) (GlobalGitIgnoreApplyPlanView, error)
	gignCommitMsg   GlobalGitIgnoreCommitMsg
	gignCommitFn    func(content, planToken string) tea.Cmd
	// Storage-migration seam overrides (zero values keep the fixture
	// preview helpers so existing Storage sub-tab tests stay green).
	sshStorageView   SSHStorageMigrationView
	sshStorageErr    error
	sshStoragePlanFn func(layout SSHStorageLayout) (SSHStorageMigrationView, error)
	sshStorageCommit SSHStorageCommitMsg
	// storageCall records the last CommitSSHStorage arguments when non-nil
	// (a pointer field so a value-receiver stub can still write through).
	storageCall *storageCommitCall
	// fixPersistErr, when non-nil, makes Persist(FixFinding{ID: fixFailID})
	// return state UNCHANGED (mirroring Wave 2's D-10 auto-restore: a
	// failed fix's own file is restored, so a rescan reproduces the same
	// finding) — 08-06-PLAN.md Task 3's TestBatchWalkHalt fixture.
	fixPersistErr error
	fixFailID     string
	// lastPersistErr is a pointer box (a value-receiver Persist still
	// writes through it, mirroring storageCall above) recording whether
	// the MOST RECENT Persist call was the configured fix failure — every
	// other action, including a SUCCEEDING fix, must report nil here, or
	// App.checkFixBatchHalt would wrongly halt on every subsequent
	// dispatch once fixPersistErr is set once.
	lastPersistErr *error
	// D-04 delete-offer overrides (Task 3, 09-06-PLAN.md): rotateDeleteOfferFn
	// overrides RotateDeleteOffer's default github/not-github answer for a
	// test that needs a specific shape (a different-machine title match
	// miss, an inventory failure, a repair-mode absence). rotateDeleteCalls
	// is a pointer box (mirroring storageCall above) recording every keyID
	// CommitRotateDeleteOldKey was called with, in order, so a retry test
	// can assert the SAME id was used with no intervening inventory read.
	// rotateDeleteCommitErr makes the commit fail deterministically.
	rotateDeleteOfferFn   func(name string) tea.Cmd
	rotateDeleteCalls     *[]string
	rotateDeleteCommitErr string
}

// storageCommitCall is the last CommitSSHStorage (layout, token) pair a
// stub recorded — pointed at from stubBackend so value-receiver methods
// can write through.
type storageCommitCall struct {
	layout SSHStorageLayout
	token  string
}

var _ Backend = stubBackend{}
var _ IdentityPlanner = stubBackend{}

// GitStepDisabledReason implements the D-19 seam for tests. See the struct
// doc comment above for the zero-value (dummy-style) default.
func (b stubBackend) GitStepDisabledReason() (string, bool) {
	return b.gitStepReason, b.gitStepAlwaysDisabled
}

func (stubBackend) InitialState() DemoState {
	identities := make([]DemoIdentity, 0, len(stubIdentityRows))
	for _, row := range stubIdentityRows {
		id := DemoIdentity{
			Name:            row.Name,
			State:           row.State,
			SSHHost:         row.SSHHost,
			KeyPath:         row.KeyPath,
			GitFragmentPath: row.GitFragmentPath,
			Note:            row.Note,
		}
		if row.SSHHost != "" {
			id.Hostname = "ssh.github.com"
			id.Port = 443
		}
		if row.GitFragmentPath != "" {
			id.GitName = row.Name + " identity"
			id.GitEmail = "you@" + row.Name + ".example"
			if row.KeyPath != "" {
				id.SigningKeyPath = row.KeyPath + ".pub"
			}
		}
		identities = append(identities, id)
	}
	findings := make([]DemoFinding, 0, len(stubHealthFindings))
	for _, f := range stubHealthFindings {
		findings = append(findings, DemoFinding{HealthFinding: f, Identity: stubFindingIdentity[f.ID]})
	}
	return DemoState{Identities: identities, Findings: findings, SSHStorage: StorageSentinel}
}

func (stubBackend) DemoBanner(TabID) bool { return false }

func (stubBackend) FixPlanFor(finding DemoFinding) FixPlan { return PlanFor(finding) }

func (b stubBackend) Persist(state DemoState, action Action) DemoState {
	if fx, isFix := action.(FixFinding); isFix && b.fixPersistErr != nil && fx.ID == b.fixFailID {
		if b.lastPersistErr != nil {
			*b.lastPersistErr = b.fixPersistErr
		}
		return state // unchanged -- mirrors D-10 auto-restore: nothing converges
	}
	if b.lastPersistErr != nil {
		*b.lastPersistErr = nil
	}
	if _, isReset := action.(Reset); isReset {
		return b.InitialState()
	}
	return Reduce(state, action)
}

// PersistError reports the error the MOST RECENT Persist call failed with —
// see lastPersistErr's doc comment for why this must track only the last
// call, not fixPersistErr unconditionally.
func (b stubBackend) PersistError() error {
	if b.lastPersistErr == nil {
		return nil
	}
	return *b.lastPersistErr
}

func (stubBackend) AlgorithmCatalog() []AlgorithmCatalogEntry { return AlgorithmCatalog }

func (stubBackend) ProviderDefaults(provider string) (hostname, port string) {
	if provider == "github.com" {
		return "ssh.github.com", "443"
	}
	if provider == "" {
		return "github.com", "22"
	}
	return provider, "22"
}

// tableBackend is a stubBackend whose ProviderDefaults answers the FULL
// known-provider table the REAL binary answers with (identity.DefaultHostname /
// identity.DefaultPort — the recipes/ alt-SSH endpoints on 443, unknown hosts
// on 22). The dummy deliberately keeps its github-only fixture table, so this
// is the seam's OTHER shape: tests that exercise D-20/D-21 provider reactivity
// drive the form through this one.
//
// It MIRRORS the real table; the real table itself is pinned by
// cmd/gitid/wiring_test.go's TestProviderDefaults, which asserts the same four
// rows against identity.DefaultHostname directly.
type tableBackend struct{ stubBackend }

func (tableBackend) ProviderDefaults(provider string) (hostname, port string) {
	switch provider {
	case "":
		return "github.com", "22"
	case "github.com":
		return "ssh.github.com", "443"
	case "gitlab.com":
		return "altssh.gitlab.com", "443"
	case "bitbucket.org":
		return "altssh.bitbucket.org", "443"
	default:
		return provider, "22"
	}
}

func (stubBackend) DefaultMatchStrategy() string { return gitScreenMatchStrategyDefault }

func (stubBackend) ValidateHostBlock(alias, hostname, port, identityFile string) *ValidationError {
	if err := sshconfig.ValidateHostBlock(alias, hostname, port, identityFile); err != nil {
		if validationErr, ok := err.(*sshconfig.ValidationError); ok {
			return &ValidationError{Field: validationErr.Field, Message: validationErr.Message}
		}
		return &ValidationError{Message: err.Error()}
	}
	return nil
}

func (stubBackend) HostBlockPreview(spec CreateSpec) string {
	return "Host " + spec.Alias + "\n    Hostname " + spec.Hostname +
		"\n    Port " + spec.Port + "\n    User git\n    IdentityFile " + spec.KeyPath +
		"\n    IdentitiesOnly yes"
}

func (stubBackend) GitFragmentPreview(spec GitSpec) string {
	return "[user]\n    name = " + spec.Name + "\n    email = " + spec.Email +
		"\n    signingkey = " + spec.KeyPath + ".pub\n\n[gpg]\n    format = ssh\n\n[commit]\n    gpgsign = true"
}

// IncludeIfPreview mirrors FixtureBackend's CR-14 fix: the gitdir-match
// condition substitutes spec.GitDir, not a frozen "~/personal/" literal.
func (stubBackend) IncludeIfPreview(spec GitSpec) string {
	preview := gitScreenMatchStrategyPreview[spec.Strategy]
	gitDir := spec.GitDir
	if gitDir == "" {
		gitDir = "~/git/" + spec.Identity + "/"
	}
	preview = strings.ReplaceAll(preview, `gitdir:~/personal/`, "gitdir:"+gitDir)
	return strings.ReplaceAll(preview, "personal", spec.Identity)
}

func (stubBackend) AliasCollision(alias string) (bool, error) {
	for _, row := range stubIdentityRows {
		if row.SSHHost == alias {
			return true, nil
		}
	}
	return false, nil
}

// stubManualReusePath is the fixture path stubBackend.ManualReusePath
// resolves — every other path reports the same "not found" story a real
// symlink/parse rejection would (identities_test.go exercises both).
const stubManualReusePath = "/manual/id_ed25519_manual"

func (stubBackend) ScanReusableKeys() []ReusableKeyView {
	var out []ReusableKeyView
	for _, row := range stubIdentityRows {
		if row.KeyPath == "" {
			continue
		}
		inUseBy := row.Name + " (" + hostSuffix(row.SSHHost) + ")"
		if row.State == "key-unused" {
			inUseBy = ""
		}
		// clientA deliberately carries an algorithm outside the D-13
		// catalog-token set (backend_stub_test.go's `internal/keygen`-free
		// mirror of an OpenSSH wire type gitid's own generate path never
		// produces) so the picker's non-catalog informational note has a
		// fixture row to render against.
		algo := "ssh-ed25519"
		if row.Name == "clientA" {
			algo = "ssh-dss"
		}
		out = append(out, ReusableKeyView{
			Path:        row.KeyPath,
			Algorithm:   algo,
			Fingerprint: "SHA256:stub-" + row.Name,
			HasPub:      row.State != "key-missing",
			Encrypted:   row.Name == "staging",
			InUseBy:     inUseBy,
		})
	}
	return out
}

// ManualReusePath resolves the D-10 picker's manual-path row against the ONE
// fixture path stubManualReusePath — an unrecognized path errors, mirroring
// the real Backend's symlink/parse-rejection contract without touching disk.
func (stubBackend) ManualReusePath(path string) (ReusableKeyView, error) {
	if path == stubManualReusePath {
		return ReusableKeyView{
			Path: path, Algorithm: "ssh-ed25519", Fingerprint: "SHA256:stub-manual",
		}, nil
	}
	return ReusableKeyView{}, fmt.Errorf("no such key: %s", path)
}

func (stubBackend) TestConfigPath() string { return CreateFlowTestTmpConfig }

func (stubBackend) Stage1Command(spec CreateSpec) string {
	return "ssh -T -F " + CreateFlowTestTmpConfig + " -p " + spec.Port +
		" -i " + spec.KeyPath + " git@" + spec.Hostname
}

func (stubBackend) Stage2Command(spec CreateSpec) string {
	return "ssh -G -F " + CreateFlowTestTmpConfig + " " + spec.Alias + " | grep identityfile"
}

// stage1Result is the outcome stage 1 answers with for spec. The D-16 demo
// control previews the error path: the provider ANSWERED but rejected the
// key, which is the D-02 reachable/not-uploaded state, never a hard failure.
// Tests drive the wizard through this same function (completeStage), so the
// message they inject is exactly what TestStage1 would deliver.
func (b stubBackend) stage1Result(spec CreateSpec) TestResultView {
	result := TestResultView{
		Outcome: TestOutcomePass,
		Command: b.Stage1Command(spec),
		Detail:  "Hi " + spec.Identity + "! You've successfully authenticated, but GitHub does not provide shell access.",
	}
	if spec.SimulateFailure {
		result.Outcome = TestOutcomeReachableNotUploaded
		result.Detail = "git@" + spec.Hostname + ": Permission denied (publickey)."
	}
	return result
}

// stage2Result is the by-alias resolution proof. Stage 2 is only reachable
// once stage 1 passed, so the fixture always resolves.
func (b stubBackend) stage2Result(spec CreateSpec) TestResultView {
	return TestResultView{
		Outcome: TestOutcomePass,
		Command: b.Stage2Command(spec),
		Detail:  "identityfile " + spec.KeyPath,
	}
}

func (b stubBackend) TestStage1(spec CreateSpec) tea.Cmd {
	return stubStageCmd(1, b.stage1Result(spec))
}

func (b stubBackend) TestStage2(spec CreateSpec) tea.Cmd {
	return stubStageCmd(2, b.stage2Result(spec))
}

// stubStageCmd mirrors the dummy's 350ms "running ssh…" tick.
func stubStageCmd(stage int, result TestResultView) tea.Cmd {
	return tea.Tick(350*time.Millisecond, func(time.Time) tea.Msg {
		return WizardStageMsg{Stage: stage, Result: result}
	})
}

func (stubBackend) ResolvedStorageTarget(state DemoState) string {
	if state.SSHStorage == StorageInclude {
		return "~/.ssh/config.d/gitid.config"
	}
	return "~/.ssh/config"
}

func (stubBackend) CreateWritePlan(spec CreateSpec, git *GitSpec) WritePlanView {
	if git == nil {
		return WritePlanView{
			Targets: []string{"~/.ssh/config"},
			Backups: []string{NewBackupPath("~/.ssh/config")},
		}
	}
	return WritePlanView{
		Targets: []string{"~/.ssh/config", "~/.gitconfig.d/" + spec.Identity, "~/.gitconfig", "~/.ssh/allowed_signers"},
		Backups: []string{NewBackupPath("~/.ssh/config"), NewBackupPath("~/.gitconfig")},
	}
}

// GitWritePlan mirrors CreateWritePlan's stub shape for the standalone
// Configure-Git ceremony (CR-12's Backend.GitWritePlan seam). The stub has
// no real filesystem to probe, so it returns the same declared 2-backup
// shape gitCeremonyFor used to hardcode before CR-12 — sufficient for
// internal/tuikit's own unit tests, which assert on ceremony Preview/note
// text, never on Targets/Backups counts.
func (stubBackend) GitWritePlan(spec GitSpec) WritePlanView {
	return WritePlanView{
		Targets: []string{"~/.gitconfig.d/" + spec.Identity, "~/.gitconfig", "~/.ssh/allowed_signers"},
		Backups: []string{NewBackupPath("~/.gitconfig"), NewBackupPath("~/.ssh/allowed_signers")},
	}
}

func (stubBackend) CopyPublicKey(string) (string, error) {
	return "Public key copied to clipboard (demo).", nil
}

// CommitCreate implements the async create seam for tests. The stub has no
// real filesystem effects, so it returns an immediate success with a fixture
// backup path.
func (stubBackend) CommitCreate(_ DemoIdentity) tea.Cmd {
	return func() tea.Msg {
		return WizardCommitMsg{Backups: []string{NewBackupPath("~/.ssh/config")}}
	}
}

// CommitDelete preserves the test backend's zero-value, no-filesystem
// behavior — an immediate success with no backups.
func (stubBackend) CommitDelete(string, string) tea.Cmd {
	return func() tea.Msg { return DeleteCommitMsg{} }
}

// CommitGit preserves the test backend's zero-value, no-filesystem behavior.
func (stubBackend) CommitGit(GitSpec) tea.Cmd {
	return func() tea.Msg { return GitCommitMsg{} }
}

// ---------------------------------------------------------------------------
// Upload / Credentials Assist (Phase 9, plan 09-02 tracer) — the stub answers
// Ready for any hostname whose provider resolves to "github" (matching the
// wizard's default acme.github.com fixture) and Omitted otherwise. No real
// subprocess is ever involved — deterministic, in-memory, mirroring the
// dummy's own frozen-choice fixture convention.
// ---------------------------------------------------------------------------

func (stubBackend) UploadEligibility(hostname string) tea.Cmd {
	return func() tea.Msg {
		if strings.Contains(hostname, "github") {
			return UploadEligibilityMsg{Hostname: hostname, View: UploadEligibilityView{
				State: UploadEligibilityReady, ProviderName: "GitHub", ToolName: "gh", Hostname: "github.com",
			}}
		}
		return UploadEligibilityMsg{Hostname: hostname, View: UploadEligibilityView{State: UploadEligibilityOmitted}}
	}
}

func (b stubBackend) RunUpload(spec CreateSpec) tea.Cmd {
	return func() tea.Msg {
		return UploadRunMsg{Name: spec.Identity, View: UploadRunView{Rows: []UploadResultRow{{
			Registration: UploadRegistrationAuthentication,
			Label:        UploadRegistrationLabelAuth,
			Command:      "gh ssh-key add " + spec.KeyPath + ".pub --title gitid: " + spec.Identity + " --type authentication",
			Outcome:      UploadRowUploaded,
		}}}}
	}
}

func (stubBackend) UploadInstructions(provider string) string {
	return "Upload your public key to " + provider + " manually."
}

func (stubBackend) RegisterKeyPlan(name string) tea.Cmd {
	return func() tea.Msg {
		host := ""
		for _, row := range stubIdentityRows {
			if row.Name == name {
				host = row.SSHHost
				break
			}
		}
		if strings.Contains(host, "github") {
			return RegisterKeyPlanMsg{Name: name, View: UploadEligibilityView{
				State: UploadEligibilityReady, ProviderName: "GitHub", ToolName: "gh", Hostname: "github.com",
			}}
		}
		return RegisterKeyPlanMsg{Name: name, View: UploadEligibilityView{State: UploadEligibilityOmitted}}
	}
}

// RotateDeleteOffer answers Available for any github fixture identity,
// Unavailable otherwise — test cases override with a custom stubBackend
// field when they need a specific shape (a different-machine title, an
// inventory failure, etc.) via rotateDeleteOfferFn.
func (b stubBackend) RotateDeleteOffer(name string) tea.Cmd {
	if b.rotateDeleteOfferFn != nil {
		return b.rotateDeleteOfferFn(name)
	}
	return func() tea.Msg {
		host := ""
		for _, row := range stubIdentityRows {
			if row.Name == name {
				host = row.SSHHost
				break
			}
		}
		if strings.Contains(host, "github") {
			return RotateDeleteOfferMsg{Name: name, View: RotateDeleteOfferView{
				Available: true, ProviderName: "GitHub",
				IdentityName: name, MachineName: "this-machine",
				KeyTitle: "gitid: " + name + " @ this-machine", KeyID: "999",
				ManualCommand: "gh ssh-key delete 999 --yes",
			}}
		}
		return RotateDeleteOfferMsg{Name: name, View: RotateDeleteOfferView{Unavailable: "no matching old key found on this machine"}}
	}
}

// CommitRotateDeleteOldKey succeeds deterministically unless a test injects
// rotateDeleteCommitErr, recording the (name, keyID) pair it was called
// with so a test can assert the retry semantics (R12) without a second
// inventory read having happened.
func (b stubBackend) CommitRotateDeleteOldKey(name, keyID string) tea.Cmd {
	return func() tea.Msg {
		if b.rotateDeleteCalls != nil {
			*b.rotateDeleteCalls = append(*b.rotateDeleteCalls, keyID)
		}
		if b.rotateDeleteCommitErr != "" {
			// WR-09: the stub does not model multi-candidate partial
			// success/failure, so RemainingKeyID mirrors keyID unchanged --
			// the correct simulation of "nothing succeeded, retry the same
			// target", matching this stub's existing single-candidate tests.
			return RotateDeleteCommitMsg{Name: name, Err: b.rotateDeleteCommitErr, RemainingKeyID: keyID}
		}
		return RotateDeleteCommitMsg{Name: name}
	}
}

func (b stubBackend) RunUploadForIdentity(name string) tea.Cmd {
	return func() tea.Msg {
		return UploadRunMsg{Name: name, View: UploadRunView{Rows: []UploadResultRow{{
			Registration: UploadRegistrationAuthentication,
			Label:        UploadRegistrationLabelAuth,
			Command:      "gh ssh-key add ~/.ssh/id_ed25519_" + name + ".pub --title gitid: " + name + " --type authentication",
			Outcome:      UploadRowUploaded,
		}}}}
	}
}

// ---------------------------------------------------------------------------
// Global SSH (plan 06-01) — the Options-sub-tab seam.
// ---------------------------------------------------------------------------

// fixtureGlobalSSHOptionViews projects the frozen GlobalSSHOptions fixture
// into the live view shape — the stub's zero value, mirroring the dummy's
// projection so the render stays fixture-identical unless a test overrides.
func fixtureGlobalSSHOptionViews() []GlobalSSHOptionView {
	out := make([]GlobalSSHOptionView, 0, len(GlobalSSHOptions))
	for _, o := range GlobalSSHOptions {
		explanation := o.OneLiner
		if o.Key == "IdentitiesOnly" {
			explanation = GlobalSSHDetailExplanation
		}
		state := GlobalSSHAlreadySet
		if o.NeedsAction {
			state = GlobalSSHNeedsAction
		}
		out = append(out, GlobalSSHOptionView{
			Key:                o.Key,
			CurrentValue:       o.Current,
			Provenance:         "fixture value — the test backend does not probe a machine",
			Recommended:        o.Recommended,
			Risk:               o.Risk,
			OneLiner:           o.OneLiner,
			Explanation:        explanation,
			State:              state,
			WritableToHostStar: o.Key != "IdentitiesOnly",
		})
	}
	return out
}

// GlobalSSHOptionStates returns the test override when set, otherwise the
// fixture projection.
func (b stubBackend) GlobalSSHOptionStates() ([]GlobalSSHOptionView, error) {
	if b.sshOptionsErr != nil {
		return nil, b.sshOptionsErr
	}
	if b.sshOptions != nil {
		return b.sshOptions, nil
	}
	return fixtureGlobalSSHOptionViews(), nil
}

// GlobalSSHApplyPlan returns the test override when set; the zero value keeps
// the ceremony's layout-aware fixture fallback.
func (b stubBackend) GlobalSSHApplyPlan(keys []string) (GlobalSSHApplyPlanView, error) {
	if b.sshApplyPlanFn != nil {
		return b.sshApplyPlanFn(keys)
	}
	return b.sshApplyPlan, nil
}

// CommitGlobalSSH delivers the test override's commit message immediately.
func (b stubBackend) CommitGlobalSSH([]string) tea.Cmd {
	return func() tea.Msg { return b.sshCommitMsg }
}

// AllSSHDirectives returns the test override when set, otherwise an empty,
// successful slice — see the NoopSSHPropertiesBrowser doc comment above for
// why this default is a success rather than the Noop sentinel.
func (b stubBackend) AllSSHDirectives() ([]SSHDirectiveView, error) {
	if b.sshDirectivesErr != nil {
		return nil, b.sshDirectivesErr
	}
	return b.sshDirectives, nil
}

// AllGitSetKeys returns the test override when set, otherwise an empty,
// successful slice — see the NoopGitPropertiesBrowser doc comment above for
// why this default is a success rather than the Noop sentinel.
func (b stubBackend) AllGitSetKeys() ([]GitSetKeyView, error) {
	if b.gitSetKeysErr != nil {
		return nil, b.gitSetKeysErr
	}
	return b.gitSetKeys, nil
}

// fixtureSSHStorageView returns the frozen STORE-01 previews so a zero-value
// stub keeps existing Storage sub-tab tests byte-identical.
func fixtureSSHStorageView(layout SSHStorageLayout) SSHStorageMigrationView {
	s := Seed()
	toInclude := layout == StorageInclude
	headingTail := "sentinel blocks in ~/.ssh/config"
	diff := "+ gitid blocks written back, sentinel-delimited, into ~/.ssh/config\n- Include ~/.ssh/config.d/gitid.config (line removed)\n- ~/.ssh/config.d/gitid.config (file retired)\n  everything outside gitid blocks: untouched"
	if toInclude {
		headingTail = "Include’d gitid.config"
		diff = "+ Include ~/.ssh/config.d/gitid.config   (near the top of ~/.ssh/config)\n+ ~/.ssh/config.d/gitid.config (all gitid blocks move here)\n- # BEGIN/END gitid managed blocks removed from ~/.ssh/config\n  everything outside gitid blocks: untouched"
	}
	return SSHStorageMigrationView{
		CurrentLayout:   s.SSHStorage,
		TargetLayout:    layout,
		Heading:         "Migrate SSH storage layout → " + headingTail,
		Targets:         []string{"~/.ssh/config", "~/.ssh/config.d/gitid.config"},
		Backups:         []string{NewBackupPath("~/.ssh/config")},
		Diff:            diff,
		MainPreview:     IncludePreviewMain,
		OwnedPreview:    IncludePreviewOwned(s),
		SentinelPreview: SentinelPreview(s),
		PlanToken:       "fixture-token-" + string(layout),
	}
}

// SSHStorageMigrationPlan returns the test override when set, otherwise the
// fixture preview so existing Storage sub-tab tests stay green.
func (b stubBackend) SSHStorageMigrationPlan(layout SSHStorageLayout) (SSHStorageMigrationView, error) {
	if b.sshStorageErr != nil {
		return SSHStorageMigrationView{}, b.sshStorageErr
	}
	if b.sshStoragePlanFn != nil {
		return b.sshStoragePlanFn(layout)
	}
	if b.sshStorageView.PlanToken != "" || b.sshStorageView.SentinelPreview != "" || b.sshStorageView.MainPreview != "" {
		return b.sshStorageView, nil
	}
	return fixtureSSHStorageView(layout), nil
}

// CommitSSHStorage records the token the model passed (when storageCall is
// set) and delivers the override commit message immediately.
func (b stubBackend) CommitSSHStorage(layout SSHStorageLayout, planToken string) tea.Cmd {
	if b.storageCall != nil {
		b.storageCall.layout = layout
		b.storageCall.token = planToken
	}
	return func() tea.Msg { return b.sshStorageCommit }
}

// ---------------------------------------------------------------------------
// Global Git (plan 07-01) — the Options pane seam.
// ---------------------------------------------------------------------------

// fixtureGlobalGitOptionViews projects the frozen GlobalGitOptions fixture
// into the live view shape — the stub's zero value, mirroring the dummy's
// projection in fixturebackend.go so the render stays fixture-identical
// unless a test overrides. Rows travel through the seam, never around it —
// this is the only place fixture values enter the globalGitModel.
func fixtureGlobalGitOptionViews() []GlobalGitOptionView {
	out := make([]GlobalGitOptionView, 0, len(GlobalGitOptions))
	for _, o := range GlobalGitOptions {
		state := GlobalGitAlreadySet
		if o.NeedsAction {
			state = GlobalGitNeedsAction
		}
		out = append(out, GlobalGitOptionView{
			Key:               o.Key,
			CurrentValue:      o.Current,
			Provenance:        "fixture value — the test backend does not probe a machine",
			Recommended:       o.Recommended,
			OneLiner:          o.OneLiner,
			State:             state,
			PolicyBacked:      o.Key != GlobalGitEmailFallbackKey,
			HasWritableMember: o.Key != GlobalGitEmailFallbackKey,
			AttributedToUser:  false,
		})
	}
	return out
}

// GlobalGitOptionStates returns the test override when set, otherwise the
// fixture projection.
func (b stubBackend) GlobalGitOptionStates() ([]GlobalGitOptionView, error) {
	if b.gitOptionsErr != nil {
		return nil, b.gitOptionsErr
	}
	if b.gitOptions != nil {
		return b.gitOptions, nil
	}
	return fixtureGlobalGitOptionViews(), nil
}

// GlobalGitApplyPlan returns the test override when set; the zero value keeps
// the ceremony's target/backup fallback.
func (b stubBackend) GlobalGitApplyPlan(keys []string) (GlobalGitApplyPlanView, error) {
	if b.gitApplyPlanFn != nil {
		return b.gitApplyPlanFn(keys)
	}
	return b.gitApplyPlan, nil
}

// CommitGlobalGit delivers the test override's commit message immediately.
func (b stubBackend) CommitGlobalGit(keys []string) tea.Cmd {
	if b.gitCommitFn != nil {
		return b.gitCommitFn(keys)
	}
	return func() tea.Msg { return b.gitCommitMsg }
}

// CustomGitKeyPlan returns the test override when set; the zero value keeps
// the ceremony's target/backup fallback (plan 09.5-03).
func (b stubBackend) CustomGitKeyPlan(key, value string) (GitCustomKeyPlanView, error) {
	if b.customKeyPlanFn != nil {
		return b.customKeyPlanFn(key, value)
	}
	return b.customKeyPlan, nil
}

// CommitCustomGitKey delivers the test override's commit message immediately.
func (b stubBackend) CommitCustomGitKey(key, value string) tea.Cmd {
	if b.customKeyFn != nil {
		return b.customKeyFn(key, value)
	}
	return func() tea.Msg { return b.customKeyCommit }
}

// ValidateCustomSSHDirective delivers the test override's proof command;
// the zero value delivers a zero-value SSHCustomDirectiveProofMsg (plan
// 09.5-04).
func (b stubBackend) ValidateCustomSSHDirective(name, value string) tea.Cmd {
	if b.sshDirectiveValidateFn != nil {
		return b.sshDirectiveValidateFn(name, value)
	}
	return func() tea.Msg { return SSHCustomDirectiveProofMsg{} }
}

// CustomSSHDirectivePlan returns the test override when set; the zero value
// keeps the ceremony's target/backup fallback (plan 09.5-04).
func (b stubBackend) CustomSSHDirectivePlan(name, value string) (SSHCustomDirectivePlanView, error) {
	if b.sshDirectivePlanFn != nil {
		return b.sshDirectivePlanFn(name, value)
	}
	return b.sshDirectivePlan, nil
}

// CommitCustomSSHDirective delivers the test override's commit message
// immediately.
func (b stubBackend) CommitCustomSSHDirective(name, value string) tea.Cmd {
	if b.sshDirectiveCommitFn != nil {
		return b.sshDirectiveCommitFn(name, value)
	}
	return func() tea.Msg { return b.sshDirectiveCommit }
}

func (b stubBackend) GlobalSSHOverridePlan(keys []string, overrides []OverrideRequest) (GlobalSSHApplyPlanView, error) {
	if b.sshOverridePlanFn != nil {
		return b.sshOverridePlanFn(keys, overrides)
	}
	return b.sshOverridePlan, nil
}

func (b stubBackend) CommitGlobalSSHOverride(keys []string, overrides []OverrideRequest) tea.Cmd {
	if b.sshOverrideCommitFn != nil {
		return b.sshOverrideCommitFn(keys, overrides)
	}
	return func() tea.Msg { return b.sshOverrideCommit }
}

func (b stubBackend) GlobalGitOverridePlan(keys []string, overrides []OverrideRequest) (GlobalGitApplyPlanView, error) {
	if b.gitOverridePlanFn != nil {
		return b.gitOverridePlanFn(keys, overrides)
	}
	return b.gitOverridePlan, nil
}

func (b stubBackend) CommitGlobalGitOverride(keys []string, overrides []OverrideRequest) tea.Cmd {
	if b.gitOverrideCommitFn != nil {
		return b.gitOverrideCommitFn(keys, overrides)
	}
	return func() tea.Msg { return b.gitOverrideCommit }
}

func (b stubBackend) GlobalGitIgnoreState() (GlobalGitIgnoreView, error) {
	if b.gignStateFn != nil {
		return b.gignStateFn()
	}
	if b.gignStateErr != nil {
		return GlobalGitIgnoreView{}, b.gignStateErr
	}
	return b.gignState, nil
}

func (b stubBackend) GlobalGitIgnoreApplyPlan(content string) (GlobalGitIgnoreApplyPlanView, error) {
	if b.gignApplyPlanFn != nil {
		return b.gignApplyPlanFn(content)
	}
	return b.gignApplyPlan, nil
}

func (b stubBackend) CommitGlobalGitIgnore(content, planToken string) tea.Cmd {
	if b.gignCommitFn != nil {
		return b.gignCommitFn(content, planToken)
	}
	return func() tea.Msg { return b.gignCommitMsg }
}

func (b stubBackend) GitFallbackAuthorState() (GitFallbackAuthorView, error) {
	if b.fallbackStateErr != nil {
		return GitFallbackAuthorView{}, b.fallbackStateErr
	}
	return b.fallbackState, nil
}

func (b stubBackend) GitFallbackAuthorPlan(name, email string) (GitFallbackAuthorPlanView, error) {
	if b.fallbackPlanFn != nil {
		return b.fallbackPlanFn(name, email)
	}
	if b.fallbackPlan.Targets != nil || b.fallbackPlan.Diff != "" || b.fallbackPlan.Removal {
		return b.fallbackPlan, nil
	}
	preview := fallbackPreview(name, email)
	if name == "" && email == "" {
		return GitFallbackAuthorPlanView{
			Targets: []string{"~/.gitconfig"},
			Removal: true,
			Diff:    preview,
		}, nil
	}
	return GitFallbackAuthorPlanView{
		Targets: []string{"~/.gitconfig"},
		Backups: []string{NewBackupPath("~/.gitconfig")},
		Diff:    preview,
	}, nil
}

func (b stubBackend) CommitGitFallbackAuthor(name, email string) tea.Cmd {
	if b.fallbackCommitFn != nil {
		return b.fallbackCommitFn(name, email)
	}
	return func() tea.Msg { return b.fallbackCommitMsg }
}

// ---------------------------------------------------------------------------
// Clone (D-14/D-15/D-16/D-17, MGR-04) — plan 05-05.
// ---------------------------------------------------------------------------

// stubCloneSuffix mirrors identity.CloneSuffix's value without importing
// internal/identity (this file's own header comment: the stub is a
// deliberate copy, never an import, to avoid the tuikit<->dummytui cycle —
// same reasoning applies one package further down the stack here).
const stubCloneSuffix = "-clone"

// stubNameTaken reports whether candidate collides (case-insensitively, SSH
// host patterns are matched case-insensitively) with an existing fixture
// identity name — the stub's D-17 taken-name check.
func stubNameTaken(candidate string) bool {
	for _, row := range stubIdentityRows {
		if strings.EqualFold(row.Name, candidate) {
			return true
		}
	}
	return false
}

// SuggestCloneName mirrors identity.SuggestCloneName's D-17 silent-bump
// semantics over the fixture rows.
func (stubBackend) SuggestCloneName(source string) string {
	base := source + stubCloneSuffix
	if !stubNameTaken(base) {
		return base
	}
	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s-%d", base, n)
		if !stubNameTaken(candidate) {
			return candidate
		}
	}
}

// findStubRow resolves a fixture row by name.
func findStubRow(name string) (stubIdentityRow, bool) {
	for _, row := range stubIdentityRows {
		if row.Name == name {
			return row, true
		}
	}
	return stubIdentityRow{}, false
}

// KeyActionFor answers from fixture rows: key-missing routes to repair,
// every other classified state routes to rotate. A test-injected keyActionErr
// fails closed so the menu cannot open the ceremony on a classification miss.
func (b stubBackend) KeyActionFor(name string) (string, error) {
	if b.keyActionErr != nil {
		return "", b.keyActionErr
	}
	row, ok := findStubRow(name)
	if !ok {
		return "", fmt.Errorf("unknown identity %q", name)
	}
	if row.State == "key-missing" {
		return KeyCeremonyModeRepair, nil
	}
	return KeyCeremonyModeRotate, nil
}

// DeletePlan answers from fixture rows, or a test-injected error/override.
func (b stubBackend) DeletePlan(name, scope string) (DeletePlanView, error) {
	if b.deletePlanErr != nil {
		return DeletePlanView{}, b.deletePlanErr
	}
	if b.deletePlanFn != nil {
		return b.deletePlanFn(name, scope)
	}
	return stubDefaultDeletePlan(name, scope), nil
}

func (b stubBackend) KeyCeremonyPlan(name, mode string) (KeyCeremonyView, error) {
	if b.keyCeremonyPlanErr != nil {
		return KeyCeremonyView{}, b.keyCeremonyPlanErr
	}
	if b.keyCeremonyPlanFn != nil {
		return b.keyCeremonyPlanFn(name, mode)
	}
	return stubDefaultKeyCeremonyPlan(name, mode), nil
}

func (b stubBackend) CommitRotate(string) tea.Cmd {
	return func() tea.Msg {
		result := b.keyCommit
		result.Mode = KeyCeremonyModeRotate
		return result
	}
}

func (b stubBackend) CommitNewKey(string) tea.Cmd {
	return func() tea.Msg {
		result := b.keyCommit
		result.Mode = KeyCeremonyModeRepair
		return result
	}
}

func stubDefaultKeyCeremonyPlan(name, mode string) KeyCeremonyView {
	row, _ := findStubRow(name)
	keyPath := row.KeyPath
	if keyPath == "" {
		keyPath = "~/.ssh/id_ed25519_" + name
	}
	provider := hostSuffix(row.SSHHost)
	if provider == "" {
		provider = defaultProvider
	}
	plan := KeyCeremonyView{
		Mode:         mode,
		IdentityName: name,
		ProviderHost: provider,
		KeyPath:      keyPath,
		PubKeyPath:   keyPath + ".pub",
		Targets:      []string{"~/.ssh/config", "~/.ssh/allowed_signers", keyPath},
		Backups:      []string{NewBackupPath("~/.ssh/config")},
	}
	if mode == KeyCeremonyModeRotate {
		plan.ArchivedKeyPath = "~/.ssh/gitid-archive/id_ed25519_" + name + ".old"
	}
	return plan
}

func stubDefaultDeletePlan(name, scope string) DeletePlanView {
	row, _ := findStubRow(name)
	fragment := row.GitFragmentPath
	if fragment == "" {
		fragment = "~/.gitconfig.d/" + name
	}
	keyPath := row.KeyPath
	if keyPath == "" {
		keyPath = "~/.ssh/id_ed25519_" + name
	}
	plan := DeletePlanView{Name: name, Scope: scope, Backups: []string{NewBackupPath("~/.gitconfig")}}
	plan.Targets = []DeleteTargetView{
		{File: "~/.gitconfig", Block: name, Label: "Git includeIf block"},
		{File: fragment, Block: "", Label: "Git fragment file"},
	}
	if scope != "everything" {
		return plan
	}
	plan.Targets = append(plan.Targets,
		DeleteTargetView{File: "~/.ssh/config", Block: name, Label: "SSH Host block"},
		DeleteTargetView{File: "~/.ssh/allowed_signers", Block: name, Label: "allowed_signers entry"},
		DeleteTargetView{File: keyPath, Block: "", Label: "Key pair"},
	)
	plan.Backups = []string{NewBackupPath("~/.ssh/config"), NewBackupPath("~/.gitconfig")}
	return plan
}

// ClonePrefill mirrors identity.DeriveCloneInput's D-14 copy/re-derive split
// over the fixture rows: user.name/user.email are copied verbatim from the
// source (when the source has a Git identity at all) and reported in
// CopiedFields; everything else is re-derived from cloneName.
func (stubBackend) ClonePrefill(source, cloneName string, reuseSourceKey bool) (ClonePrefillView, error) {
	cloneName = strings.TrimSpace(cloneName)
	if cloneName == "" {
		return ClonePrefillView{}, fmt.Errorf("clone name is required")
	}
	if strings.EqualFold(cloneName, source) {
		return ClonePrefillView{}, fmt.Errorf("clone name must differ from the source name")
	}
	if stubNameTaken(cloneName) {
		return ClonePrefillView{}, fmt.Errorf("%q is already in use", cloneName)
	}
	src, ok := findStubRow(source)
	if !ok {
		return ClonePrefillView{}, fmt.Errorf("clone: source identity %q not found", source)
	}
	provider := defaultProvider
	if src.SSHHost != "" {
		provider = hostSuffix(src.SSHHost)
	}
	hostname, port := stubBackend{}.ProviderDefaults(provider)
	gitName, gitEmail := "", ""
	if src.GitFragmentPath != "" {
		gitName = src.Name + " identity"
		gitEmail = "you@" + src.Name + ".example"
	}
	view := ClonePrefillView{
		SourceName:    source,
		CloneName:     cloneName,
		AliasPrefix:   cloneName,
		Hostname:      hostname,
		Port:          port,
		GitName:       gitName,
		GitEmail:      gitEmail,
		MatchStrategy: gitScreenMatchStrategyDefault,
		GitDir:        "~/git/" + cloneName + "/",
		CopiedFields:  []string{copiedFieldGitName, copiedFieldGitEmail},
	}
	if reuseSourceKey {
		view.ReuseKeyPath = src.KeyPath
	}
	return view, nil
}
