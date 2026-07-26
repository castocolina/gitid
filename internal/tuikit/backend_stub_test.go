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
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
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
		SuggestedFix: "chmod 0600 ~/.ssh/id_ed25519_archived -- available on the Fixer screen.",
	},
	{
		ID: "ssh-identitiesonly-contradiction", Section: "SSH", Severity: SeverityError, Family: "Coherence",
		Title:        "IdentitiesOnly no contradicts an explicit IdentityFile",
		Explanation:  "Host clientb.github.com sets IdentitiesOnly no while also naming IdentityFile ~/.ssh/id_ed25519_clientB -- ssh may still offer every other key it knows before falling back to the one explicitly configured (HLTH-04).",
		SuggestedFix: "Set IdentitiesOnly yes on the clientb.github.com Host block -- available on the Fixer screen.",
	},
	{
		ID: "git-includeif-missing-fragment", Section: "Git", Severity: SeverityError, Family: "Orphans",
		Title:        "includeIf targets a missing fragment",
		Explanation:  "[includeIf \"gitdir:~/legacy/\"] in ~/.gitconfig points at ~/.gitconfig.d/legacy, which does not exist on disk -- commits made under ~/legacy/ silently fall back to your global git identity instead of \"legacy\" (HLTH-04).",
		SuggestedFix: "Restore ~/.gitconfig.d/legacy, or repoint the includeIf -- available on the Fixer screen.",
	},
	{
		ID: "ssh-duplicate-host-star", Section: "SSH", Severity: SeverityWarning, Family: "Redundancy",
		Title:        "Duplicate Host * stanza",
		Explanation:  "~/.ssh/config defines Host * twice -- line 4 and line 41. The second stanza silently overrides directives set by the first (HLTH-03).",
		SuggestedFix: "Merge the two Host * stanzas into one -- available on the Fixer screen.",
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
type stubBackend struct{}

var _ Backend = stubBackend{}

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
		if row.GitFragmentPath != "" {
			id.GitName = row.Name + " identity"
			id.GitEmail = "you@" + row.Name + ".example"
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

func (b stubBackend) Persist(state DemoState, action Action) DemoState {
	if _, isReset := action.(Reset); isReset {
		return b.InitialState()
	}
	return Reduce(state, action)
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

func (stubBackend) DefaultMatchStrategy() string { return gitScreenMatchStrategyDefault }

func (stubBackend) HostBlockPreview(spec CreateSpec) string {
	return "Host " + spec.Alias + "\n    Hostname " + spec.Hostname +
		"\n    Port " + spec.Port + "\n    User git\n    IdentityFile " + spec.KeyPath +
		"\n    IdentitiesOnly yes"
}

func (stubBackend) GitFragmentPreview(spec GitSpec) string {
	return "[user]\n    name = " + spec.Name + "\n    email = " + spec.Email +
		"\n    signingkey = " + spec.KeyPath + ".pub\n\n[gpg]\n    format = ssh\n\n[commit]\n    gpgsign = true"
}

func (stubBackend) IncludeIfPreview(spec GitSpec) string {
	return strings.ReplaceAll(gitScreenMatchStrategyPreview[spec.Strategy], "personal", spec.Identity)
}

func (stubBackend) AliasCollision(state DemoState, identity string) bool {
	return hasIdentityNamed(state, identity)
}

func (stubBackend) ScanReusableKeys() []ReusableKeyView {
	var out []ReusableKeyView
	for _, row := range stubIdentityRows {
		if row.KeyPath == "" {
			continue
		}
		inUseBy := row.Name
		if row.State == "key-unused" {
			inUseBy = ""
		}
		out = append(out, ReusableKeyView{
			Path:      row.KeyPath,
			Algorithm: "ed25519",
			HasPub:    row.State != "key-missing",
			InUseBy:   inUseBy,
		})
	}
	return out
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

func (stubBackend) CopyPublicKey(string) (string, error) {
	return "Public key copied to clipboard (demo).", nil
}
