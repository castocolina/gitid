package dummytui

// fixturebackend.go is the dummy half of the D-17 extraction: the
// tuikit.Backend implementation the LIVE DESIGN DEMO (cmd/gitid-dummy) is
// built on.
//
// internal/tuikit owns the approved, frozen render stack and never reads or
// writes a file. Everything it needs from the outside world arrives through
// a tuikit.Backend. FixtureBackend answers every one of those calls from
// data.go's recipe fixtures, in memory — so the demo keeps rendering
// byte-identically to the pre-extraction dummy while the real binary
// (cmd/gitid) injects a Backend that talks to the user's actual machine.
//
// Every value returned below is the VERBATIM behavior the pre-extraction
// dummy had inline (internal/dummytui/store.go's Seed/Reduce and
// identities.go's providerDefaults/hostBlockText/stage1Cmd/stage2Cmd/
// reviewCeremony). The demo copy is frozen: nothing here may be reworded.

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/tuikit"
)

// fixtureStageDelay is the brief "running ssh…" tick the demo shows before a
// test stage answers — the pre-extraction runStageCmd delay, unchanged.
const fixtureStageDelay = 350 * time.Millisecond

// findingIdentityAttribution maps seeded finding ids to the identity each
// finding is about — the Go mirror of store.ts's findingIdentity map.
// ssh-duplicate-host-star stays global (no entry).
var findingIdentityAttribution = map[string]string{
	"ssh-key-perms-archived":           "archived",
	"ssh-identitiesonly-contradiction": "clientB",
	"git-includeif-missing-fragment":   "legacy",
	"git-opensource-no-host-block":     "opensource",
}

// FixtureBackend is the in-memory tuikit.Backend the design demo runs on.
// It holds no state of its own: the App owns the DemoState and hands it back
// on every Persist, so the whole demo stays a pure (state, action) → state
// reduction over data.go's fixtures.
type FixtureBackend struct {
	tuikit.NoopIdentityPlanner
	tuikit.NoopGitFallbackAuthorPlanner
}

// NewFixtureBackend returns the demo's fixture Backend. cmd/gitid-dummy is
// its only production caller.
func NewFixtureBackend() FixtureBackend { return FixtureBackend{} }

// compile-time proof the fixture really satisfies the seam (the project's
// injected-seam wiring blindspot: a seam that only "looks" wired is worse
// than no seam at all).
var _ tuikit.Backend = FixtureBackend{}
var _ tuikit.IdentityPlanner = FixtureBackend{}
var _ tuikit.SSHStoragePlanner = FixtureBackend{}
var _ tuikit.GlobalGitPlanner = FixtureBackend{}
var _ tuikit.GitFallbackAuthorPlanner = FixtureBackend{}

// ---------------------------------------------------------------------------
// Data
// ---------------------------------------------------------------------------

// InitialState builds the initial demo state from data.go's fixtures — the
// Go mirror of store.ts's initialDemoState (the pre-extraction Seed()).
// Rows with a Git fragment get the same derived author values the web seed
// uses.
func (FixtureBackend) InitialState() tuikit.DemoState {
	identities := make([]tuikit.DemoIdentity, 0, len(IdentityManagerRows))
	for _, row := range IdentityManagerRows {
		id := tuikit.DemoIdentity{
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
	findings := make([]tuikit.DemoFinding, 0, len(HealthFindings))
	for _, f := range HealthFindings {
		findings = append(findings, tuikit.DemoFinding{
			HealthFinding: f,
			Identity:      findingIdentityAttribution[f.ID],
		})
	}
	return tuikit.DemoState{
		Identities: identities,
		Findings:   findings,
		SSHStorage: tuikit.StorageSentinel,
	}
}

// DemoBanner reports whether a tab must render the D-16 "still demo data"
// banner. The dummy returns false for EVERY tab — the whole dummy IS demo
// data, so a per-tab banner would be noise (and would change the frozen
// render). Only the real binary raises it.
func (FixtureBackend) DemoBanner(tuikit.TabID) bool { return false }

// Persist applies one committed Action in memory. Reduce answers every
// transition except Reset, which only a Backend can answer (it means
// "whatever initial is for you") — here, the fixtures again.
func (b FixtureBackend) Persist(state tuikit.DemoState, action tuikit.Action) tuikit.DemoState {
	if _, isReset := action.(tuikit.Reset); isReset {
		return b.InitialState()
	}
	return tuikit.Reduce(state, action)
}

// PersistError always returns nil — Reduce is an in-memory transition and
// never fails.
func (FixtureBackend) PersistError() error { return nil }

// ---------------------------------------------------------------------------
// Create-flow effects
// ---------------------------------------------------------------------------

// AlgorithmCatalog is the KEY-01 catalog the wizard's SSH step offers. The
// dummy returns the full frozen catalog; the real binary may return a
// machine-probed subset of the same shape.
func (FixtureBackend) AlgorithmCatalog() []tuikit.AlgorithmCatalogEntry {
	return tuikit.AlgorithmCatalog
}

// ProviderDefaults mirrors Identities.tsx's providerDefaults: github.com
// gets the port-443 alt-SSH endpoint, anything else defaults to itself:22.
func (FixtureBackend) ProviderDefaults(provider string) (hostname, port string) {
	if provider == "github.com" {
		return "ssh.github.com", "443"
	}
	if provider == "" {
		return "github.com", "22"
	}
	return provider, "22"
}

// DefaultMatchStrategy is the includeIf match strategy a new identity starts
// on (GITUI-03; "gitdir" per recipes/).
func (FixtureBackend) DefaultMatchStrategy() string { return GitScreenMatchStrategyDefault }

// ValidateHostBlock validates the four SSH form values before they are
// interpolated into an OpenSSH Host block.
func (FixtureBackend) ValidateHostBlock(_, _, _, _ string) *tuikit.ValidationError {
	return nil
}

// HostBlockPreview renders the managed Host block for spec — the ONE source
// of the block shape, shared by the wizard preview/ceremony and the
// edit-SSH preview/ceremony, and the same text "written" on confirm.
func (FixtureBackend) HostBlockPreview(spec tuikit.CreateSpec) string {
	return "Host " + spec.Alias + "\n    Hostname " + spec.Hostname +
		"\n    Port " + spec.Port + "\n    User git\n    IdentityFile " + spec.KeyPath +
		"\n    IdentitiesOnly yes"
}

// GitFragmentPreview is the ~/.gitconfig.d/<identity> fragment content for
// spec. user.signingkey is a PATH to the public half, never key material.
func (FixtureBackend) GitFragmentPreview(spec tuikit.GitSpec) string {
	return "[user]\n    name = " + spec.Name + "\n    email = " + spec.Email +
		"\n    signingkey = " + spec.KeyPath + ".pub\n\n[gpg]\n    format = ssh\n\n[commit]\n    gpgsign = true"
}

// IncludeIfPreview is the ~/.gitconfig includeIf block for spec's match
// strategy, aliased to spec.Identity.
//
// CR-14 (was WR-31): the gitdir-match condition substitutes spec.GitDir —
// the SAME resolved value gitForm.gitDirFor derives and every other widget
// on the frame renders — not a frozen "~/personal/" literal predating D-02's
// "~/git/<identity>/" derivation. Without this, the dummy and real previews
// can never agree about the gitdir, which is exactly the divergence CR-10's
// scoped predicate is supposed to narrowly authorize (and nothing wider).
func (FixtureBackend) IncludeIfPreview(spec tuikit.GitSpec) string {
	preview := GitScreenMatchStrategyPreview[spec.Strategy]
	gitDir := spec.GitDir
	if gitDir == "" {
		gitDir = "~/git/" + spec.Identity + "/"
	}
	preview = strings.ReplaceAll(preview, `gitdir:~/personal/`, "gitdir:"+gitDir)
	return strings.ReplaceAll(preview, "personal", spec.Identity)
}

// AliasCollision reports whether alias is already claimed by an existing
// identity in the demo fixtures — the D-09 collision check the wizard gates
// step 1 on. The dummy answers from the in-memory rows; the real binary reads
// the user's actual Host blocks.
func (FixtureBackend) AliasCollision(alias string) (bool, error) {
	for _, row := range IdentityManagerRows {
		if row.SSHHost == alias {
			return true, nil
		}
	}
	return false, nil
}

// ScanReusableKeys lists the keys the D-10 picker offers for reuse. The dummy
// derives them from the SAME fixture rows the Identity Manager renders, so
// the picker's "in use by: <identity> (<provider>)" labels (D-12) are
// traceably the same data — never a second, divergent list. Nothing is read
// from disk.
func (FixtureBackend) ScanReusableKeys() []tuikit.ReusableKeyView {
	var out []tuikit.ReusableKeyView
	for _, row := range IdentityManagerRows {
		if row.KeyPath == "" {
			continue
		}
		inUseBy := row.Name + " (" + providerHostFromAlias(row.SSHHost) + ")"
		if row.State == "key-unused" {
			inUseBy = "" // the fixture's deliberately unreferenced key
		}
		out = append(out, tuikit.ReusableKeyView{
			Path:        row.KeyPath,
			Algorithm:   "ed25519",
			Fingerprint: GitScreenAllowedSignersKeyMaterial,
			HasPub:      row.State != "key-missing",
			InUseBy:     inUseBy,
		})
	}
	return out
}

// providerHostFromAlias mirrors the composition root's D-20 "provider is
// read off the Host suffix" reduction (personal.github.com -> github.com)
// so the dummy's InUseBy label matches the real Backend's shape exactly —
// pure string derivation, not a second provider TABLE.
func providerHostFromAlias(alias string) string {
	parts := strings.Split(strings.TrimSpace(alias), ".")
	if len(parts) <= 2 {
		return alias
	}
	return strings.Join(parts[len(parts)-2:], ".")
}

// ManualReusePath resolves the D-10 picker's manual-path row against the
// SAME fixture rows ScanReusableKeys offers, so the demo's manual entry is
// traceable data too, never a divergent shape. An unrecognized path reports
// the same "not found" story a real symlink/parse rejection would.
func (b FixtureBackend) ManualReusePath(path string) (tuikit.ReusableKeyView, error) {
	for _, v := range b.ScanReusableKeys() {
		if v.Path == path {
			return v, nil
		}
	}
	return tuikit.ReusableKeyView{}, fmt.Errorf("no such key: %s", path)
}

// TestConfigPath is the throwaway config both test stages run against, so
// the live ~/.ssh/config stays untouched until the final confirm.
func (FixtureBackend) TestConfigPath() string { return CreateFlowTestTmpConfig }

// Stage1Command is the stage-1 direct test command (TEST-01), with the
// CONSISTENT flag order pinned by data.go's CreateFlowTestStage1Command.
func (b FixtureBackend) Stage1Command(spec tuikit.CreateSpec) string {
	return "ssh -T -F " + CreateFlowTestTmpConfig + " -p " + spec.Port +
		" -i " + spec.KeyPath + " git@" + spec.Hostname
}

// Stage2Command is the stage-2 by-alias test (TEST-02) — no -i BY DESIGN:
// the config must supply the key, which is exactly what this stage proves.
func (b FixtureBackend) Stage2Command(spec tuikit.CreateSpec) string {
	return "ssh -G -F " + CreateFlowTestTmpConfig + " " + spec.Alias + " | grep identityfile"
}

// TestStage1 "runs" stage 1: a brief tick, then the fixture outcome. The
// D-16 demo control (spec.SimulateFailure) previews the error path — the
// provider answered but rejected the key, which is the D-02 reachable /
// not-uploaded state, never a hard failure.
func (b FixtureBackend) TestStage1(spec tuikit.CreateSpec) tea.Cmd {
	result := tuikit.TestResultView{
		Outcome: tuikit.TestOutcomePass,
		Command: b.Stage1Command(spec),
		Detail:  "Hi " + spec.Identity + "! You've successfully authenticated, but GitHub does not provide shell access.",
	}
	if spec.SimulateFailure {
		result.Outcome = tuikit.TestOutcomeReachableNotUploaded
		result.Detail = "git@" + spec.Hostname + ": Permission denied (publickey)."
	}
	return fixtureStageCmd(1, result)
}

// TestStage2 "runs" stage 2: the by-alias resolution proof. It is only ever
// reached once stage 1 passed, so the fixture always resolves.
func (b FixtureBackend) TestStage2(spec tuikit.CreateSpec) tea.Cmd {
	return fixtureStageCmd(2, tuikit.TestResultView{
		Outcome: tuikit.TestOutcomePass,
		Command: b.Stage2Command(spec),
		Detail:  "identityfile " + spec.KeyPath,
	})
}

// fixtureStageCmd schedules a stage completion after the demo's running tick
// — the pre-extraction runStageCmd, now carrying the stage's result.
func fixtureStageCmd(stage int, result tuikit.TestResultView) tea.Cmd {
	return tea.Tick(fixtureStageDelay, func(time.Time) tea.Msg {
		return tuikit.WizardStageMsg{Stage: stage, Result: result}
	})
}

// ResolvedStorageTarget is the file gitid's managed blocks actually land in
// for state's STORE-01 layout — ~/.ssh/config under the sentinel layout, the
// gitid-owned Include'd file otherwise (D-05/D-06).
func (FixtureBackend) ResolvedStorageTarget(state tuikit.DemoState) string {
	if state.SSHStorage == tuikit.StorageInclude {
		return "~/.ssh/config.d/gitid.config"
	}
	return "~/.ssh/config"
}

// CreateWritePlan is what a committed create will touch: the target files
// and the timestamped backups taken FIRST (TEST-03). git is nil when the
// user skipped the wizard's Git step.
func (FixtureBackend) CreateWritePlan(spec tuikit.CreateSpec, git *tuikit.GitSpec) tuikit.WritePlanView {
	if git == nil {
		return tuikit.WritePlanView{
			Targets: []string{"~/.ssh/config"},
			Backups: []string{tuikit.NewBackupPath("~/.ssh/config")},
		}
	}
	return tuikit.WritePlanView{
		Targets: []string{"~/.ssh/config", "~/.gitconfig.d/" + spec.Identity, "~/.gitconfig", "~/.ssh/allowed_signers"},
		Backups: []string{tuikit.NewBackupPath("~/.ssh/config"), tuikit.NewBackupPath("~/.gitconfig")},
	}
}

// GitWritePlan mirrors CreateWritePlan's declared-fixture shape for the
// standalone Configure-Git ceremony (CR-12's Backend.GitWritePlan seam). The
// dummy's own CommitGit never touches HOME and reports no real backups
// (FixtureBackend.CommitGit returns an empty GitCommitMsg), so the ceremony
// keeps whatever this declared plan states throughout state A AND the
// receipt — this is the SAME static 2-entry backup list
// (CTX-D-12/result-success:git-ceremony's allowlisted divergence already
// documents that the dummy's declared list never names a fragment backup,
// unlike the real binary's complete receipt).
func (FixtureBackend) GitWritePlan(spec tuikit.GitSpec) tuikit.WritePlanView {
	return tuikit.WritePlanView{
		Targets: []string{"~/.gitconfig.d/" + spec.Identity, "~/.gitconfig", "~/.ssh/allowed_signers"},
		Backups: []string{tuikit.NewBackupPath("~/.gitconfig"), tuikit.NewBackupPath("~/.ssh/allowed_signers")},
	}
}

// CopyPublicKey is the D-03 clipboard copy offered on the ReachableNotUploaded
// warning path. The dummy touches NOTHING — it says so explicitly in the
// receipt, because only the Backend knows whether a real clipboard was written.
func (FixtureBackend) CopyPublicKey(string) (string, error) {
	return "Public key copied to clipboard (demo).", nil
}

// FixPlanFor delegates UNCHANGED to the frozen free tuikit.PlanFor switch
// (08-02-PLAN.md Task 2) — the dummy's fix-plan text is Phase-2 approved
// design fixture data, never real file content.
func (FixtureBackend) FixPlanFor(finding tuikit.DemoFinding) tuikit.FixPlan {
	return tuikit.PlanFor(finding)
}

// GitStepDisabledReason implements D-19: the dummy keeps the UNCHANGED
// form-validity gate and its own frozen reason ("— needs user.name + a
// valid email", owned by internal/tuikit) — Phase 3 does not touch the
// demo's Git-step behavior, only the real binary's (cmd/gitid).
func (FixtureBackend) GitStepDisabledReason() (string, bool) {
	return "", false
}

// CommitCreate confirms a create in the demo. The dummy has no real files to
// write, so it reports success with the same mock backups CreateWritePlan
// advertises — keeping the demo's render identical to the pre-extraction flow.
func (b FixtureBackend) CommitCreate(id tuikit.DemoIdentity) tea.Cmd {
	plan := b.CreateWritePlan(tuikit.CreateSpec{Identity: id.Name}, nil)
	return func() tea.Msg {
		return tuikit.WizardCommitMsg{Backups: plan.Backups}
	}
}

// CommitGit keeps the approved dummy flow in memory. It must never touch HOME;
// success is only the fixture receipt advertised by the design backend.
func (FixtureBackend) CommitGit(tuikit.GitSpec) tea.Cmd {
	return func() tea.Msg { return tuikit.GitCommitMsg{} }
}

// CommitDelete keeps the approved dummy flow in memory — it never touches
// HOME, reporting success after the same brief tick the other async fixture
// commands use, with the existing fixture backup constants (NewBackupPath)
// so the dummy binary keeps compiling and behaving byte-identically to
// before this seam existed.
func (FixtureBackend) CommitDelete(_ string, scope string) tea.Cmd {
	backup := tuikit.NewBackupPath("~/.gitconfig")
	if scope == "everything" {
		backup = tuikit.NewBackupPath("~/.ssh/config")
	}
	return tea.Tick(fixtureStageDelay, func(time.Time) tea.Msg {
		return tuikit.DeleteCommitMsg{Backups: []string{backup}}
	})
}

// CommitRotate returns the dummy's successful rotation result.
func (FixtureBackend) CommitRotate(name string) tea.Cmd {
	return tea.Tick(fixtureStageDelay, func(time.Time) tea.Msg {
		return tuikit.KeyCommitMsg{
			Mode:            tuikit.KeyCeremonyModeRotate,
			Backups:         []string{tuikit.NewBackupPath("~/.ssh/config")},
			ArchivedKeyPath: "~/.ssh/gitid-archive/id_ed25519_" + name + ".old",
		}
	})
}

// CommitNewKey returns the dummy's successful repair result.
func (FixtureBackend) CommitNewKey(string) tea.Cmd {
	return tea.Tick(fixtureStageDelay, func(time.Time) tea.Msg {
		return tuikit.KeyCommitMsg{
			Mode:    tuikit.KeyCeremonyModeRepair,
			Backups: []string{tuikit.NewBackupPath("~/.ssh/config")},
		}
	})
}

// ---------------------------------------------------------------------------
// Upload / Credentials Assist (Phase 9, plan 09-02 tracer).
//
// The demo must render IDENTICALLY on every machine, or the visual-
// regression gate becomes noise — so UploadEligibility never probes a real
// PATH. Its fixed GitHub, GitLab, and omitted answers demonstrate three
// states; disabled is exercised by tests and the no-CLI PTY case. The real
// binary's machine-dependent answer is a classified divergence plan 09-07
// registers, not a bug in this fixture.
//
// UploadInstructions returns fixed prose rather than importing
// internal/upload — dummytui's ALLOWLIST (nobackend_test.go) forbids
// importing ANY first-party backend package, uploader/upload included.
// ---------------------------------------------------------------------------

// UploadEligibility answers the frozen demo shape described above.
func (FixtureBackend) UploadEligibility(hostname string) tea.Cmd {
	return func() tea.Msg {
		switch {
		case strings.Contains(hostname, "github"):
			return tuikit.UploadEligibilityMsg{Hostname: hostname, View: tuikit.UploadEligibilityView{
				State: tuikit.UploadEligibilityReady, ProviderName: "GitHub", ToolName: "gh", Hostname: "github.com",
			}}
		case strings.Contains(hostname, "gitlab"):
			return tuikit.UploadEligibilityMsg{Hostname: hostname, View: tuikit.UploadEligibilityView{
				State: tuikit.UploadEligibilityUnauth, ProviderName: "GitLab", ToolName: "glab", Hostname: "gitlab.com",
			}}
		default:
			return tuikit.UploadEligibilityMsg{Hostname: hostname, View: tuikit.UploadEligibilityView{State: tuikit.UploadEligibilityOmitted}}
		}
	}
}

// RunUpload "runs" the autonomous upload beat: a brief tick (mirroring
// TestStage1/TestStage2's fixtureStageDelay), then a fixed result driven by
// the hostname — never a real gh/glab lookup — so the demo shows every
// D-16 result shape deterministically: github shows both registrations
// uploaded, gitlab shows one combined row, and a seeded "partial." fixture
// host shows a real partial-failure pair with the signing remediation
// reason, exercising the same render branch the real scope-error case does.
func (b FixtureBackend) RunUpload(spec tuikit.CreateSpec) tea.Cmd {
	title := fmt.Sprintf(tuikit.UploadKeyTitleFmt, spec.Identity, "demo-machine")
	// WR-05: the D-07 title always contains spaces ("gitid: <identity> @
	// <machine>"), so an unquoted preview pasted into a shell would run a
	// DIFFERENT command than the one gitid actually runs — the exact bug
	// WR-18 fixed for the real backend's uploader.CommandPreview/previewLine
	// (internal/uploader/uploader.go). dummytui cannot import internal/uploader
	// (the nobackend allowlist), so the already-quoted shape is hardcoded here
	// instead of shared, mirroring what previewLine emits for this argv.
	authCmd := fmt.Sprintf("/usr/local/bin/gh ssh-key add %s.pub --title '%s' --type authentication", spec.KeyPath, title)
	signCmd := fmt.Sprintf("/usr/local/bin/gh ssh-key add %s.pub --title '%s' --type signing", spec.KeyPath, title)
	glabCmd := fmt.Sprintf("/usr/local/bin/glab ssh-key add %s.pub -t '%s' --usage-type auth_and_signing", spec.KeyPath, title)

	var view tuikit.UploadRunView
	switch {
	case strings.Contains(spec.Hostname, "partial"):
		view = tuikit.UploadRunView{Rows: []tuikit.UploadResultRow{
			{Registration: tuikit.UploadRegistrationAuthentication, Label: tuikit.UploadRegistrationLabelAuth, Command: authCmd, Outcome: tuikit.UploadRowUploaded},
			{Registration: tuikit.UploadRegistrationSigning, Label: tuikit.UploadRegistrationLabelSigning, Command: signCmd, Outcome: tuikit.UploadRowFailed,
				Reason: fmt.Sprintf(tuikit.UploadScopeRemediationSigningFmt, "github.com")},
		}}
	case strings.Contains(spec.Hostname, "gitlab"):
		view = tuikit.UploadRunView{Rows: []tuikit.UploadResultRow{
			{Registration: tuikit.UploadRegistrationCombined, Label: tuikit.UploadRegistrationLabelCombined, Command: glabCmd, Outcome: tuikit.UploadRowUploaded},
		}}
	default:
		view = tuikit.UploadRunView{Rows: []tuikit.UploadResultRow{
			{Registration: tuikit.UploadRegistrationAuthentication, Label: tuikit.UploadRegistrationLabelAuth, Command: authCmd, Outcome: tuikit.UploadRowUploaded},
			{Registration: tuikit.UploadRegistrationSigning, Label: tuikit.UploadRegistrationLabelSigning, Command: signCmd, Outcome: tuikit.UploadRowUploaded},
		}}
	}
	return tea.Tick(fixtureStageDelay, func(time.Time) tea.Msg {
		return tuikit.UploadRunMsg{Name: spec.Identity, View: view}
	})
}

// RotateDeleteOffer answers the D-04 demo shape deterministically: a github
// fixture identity gets an available offer naming a demo old key, everything
// else (gitlab, unknown, non-gated) is unavailable with a reason.
func (FixtureBackend) RotateDeleteOffer(name string) tea.Cmd {
	host := identityManagerSSHHost(name)
	return func() tea.Msg {
		switch {
		case strings.Contains(host, "github"):
			// WR-05: this fixture used to advertise the fixed, namespace-
			// blind "gh ssh-key delete <id> --yes" argv CR-02 established as
			// WRONG (GitHub's authentication and signing key registrations
			// are separate REST resources with independently colliding ID
			// spaces — a delete must be scoped per Registration). The dummy
			// backend is this project's design/screenshot source of truth,
			// so it must model the SAME shapes the real backend now
			// produces: KeyID is the opaque per-registration JSON payload
			// encodeDeleteCandidates emits (cmd/gitid/upload_run.go),
			// KeyDetail carries the reviewed IDs plus a blob suffix
			// (deleteCandidatesDetail), and ManualCommand previews BOTH
			// registration-scoped delete commands, one per line
			// (deleteCandidatesManualCommand / uploader.DeleteCommandPreview)
			// — a rotated GitHub key always carries two registrations.
			return tuikit.RotateDeleteOfferMsg{Name: name, View: tuikit.RotateDeleteOfferView{
				Available:    true,
				ProviderName: "GitHub",
				IdentityName: name,
				MachineName:  "demo-machine",
				KeyTitle:     fmt.Sprintf(tuikit.UploadKeyTitleFmt, name, "demo-machine"),
				KeyID:        `[{"id":"1234567","registration":0},{"id":"7654321","registration":1}]`,
				KeyDetail:    "ID 1234567, 7654321 — …AAAIDEMOKEY",
				ManualCommand: "/usr/local/bin/gh api -X DELETE user/keys/1234567\n" +
					"/usr/local/bin/gh api -X DELETE user/ssh_signing_keys/7654321",
			}}
		default:
			return tuikit.RotateDeleteOfferMsg{Name: name, View: tuikit.RotateDeleteOfferView{Unavailable: "no matching old key found on this machine"}}
		}
	}
}

// CommitRotateDeleteOldKey "deletes" the demo old key deterministically —
// always succeeds, mirroring RunUploadForIdentity's brief-tick demo shape.
func (FixtureBackend) CommitRotateDeleteOldKey(string, string) tea.Cmd {
	return func() tea.Msg {
		return tuikit.RotateDeleteCommitMsg{}
	}
}

// UploadInstructions returns fixed demo prose — the real backend's
// byte-identical internal/upload.Instructions(provider) is out of reach
// here by the ALLOWLIST rule above.
func (FixtureBackend) UploadInstructions(provider string) string {
	return "Upload your public key to " + provider + " manually — see its SSH key settings page."
}

// identityManagerSSHHost resolves name's fixture SSHHost from
// IdentityManagerRows, or "" if name is unknown — the same lookup
// RegisterKeyPlan and RunUploadForIdentity both need.
func identityManagerSSHHost(name string) string {
	for _, row := range IdentityManagerRows {
		if row.Name == name {
			return row.SSHHost
		}
	}
	return ""
}

// RegisterKeyPlan answers the D-08 register-key pane's eligibility probe
// with the same frozen demo shape UploadEligibility uses, keyed by the
// identity's fixture SSHHost rather than a form's live hostname.
func (FixtureBackend) RegisterKeyPlan(name string) tea.Cmd {
	host := identityManagerSSHHost(name)
	return func() tea.Msg {
		switch {
		case strings.Contains(host, "github"):
			return tuikit.RegisterKeyPlanMsg{Name: name, View: tuikit.UploadEligibilityView{
				State: tuikit.UploadEligibilityReady, ProviderName: "GitHub", ToolName: "gh", Hostname: "github.com",
			}}
		case strings.Contains(host, "gitlab"):
			return tuikit.RegisterKeyPlanMsg{Name: name, View: tuikit.UploadEligibilityView{
				State: tuikit.UploadEligibilityUnauth, ProviderName: "GitLab", ToolName: "glab", Hostname: "gitlab.com",
			}}
		default:
			return tuikit.RegisterKeyPlanMsg{Name: name, View: tuikit.UploadEligibilityView{State: tuikit.UploadEligibilityOmitted}}
		}
	}
}

// RunUploadForIdentity "runs" the D-08 pane's upload beat for the named
// identity's own key, mirroring RunUpload's brief-tick, hostname-driven
// demo shape.
func (b FixtureBackend) RunUploadForIdentity(name string) tea.Cmd {
	host := identityManagerSSHHost(name)
	title := fmt.Sprintf(tuikit.UploadKeyTitleFmt, name, "demo-machine")
	keyPath := "~/.ssh/id_ed25519_" + name
	authCmd := fmt.Sprintf("/usr/local/bin/gh ssh-key add %s.pub --title %s --type authentication", keyPath, title)
	signCmd := fmt.Sprintf("/usr/local/bin/gh ssh-key add %s.pub --title %s --type signing", keyPath, title)
	glabCmd := fmt.Sprintf("/usr/local/bin/glab ssh-key add %s.pub -t %s --usage-type auth_and_signing", keyPath, title)

	var view tuikit.UploadRunView
	if strings.Contains(host, "gitlab") {
		view = tuikit.UploadRunView{Rows: []tuikit.UploadResultRow{
			{Registration: tuikit.UploadRegistrationCombined, Label: tuikit.UploadRegistrationLabelCombined, Command: glabCmd, Outcome: tuikit.UploadRowUploaded},
		}}
	} else {
		view = tuikit.UploadRunView{Rows: []tuikit.UploadResultRow{
			{Registration: tuikit.UploadRegistrationAuthentication, Label: tuikit.UploadRegistrationLabelAuth, Command: authCmd, Outcome: tuikit.UploadRowUploaded},
			{Registration: tuikit.UploadRegistrationSigning, Label: tuikit.UploadRegistrationLabelSigning, Command: signCmd, Outcome: tuikit.UploadRowUploaded},
		}}
	}
	return tea.Tick(fixtureStageDelay, func(time.Time) tea.Msg {
		return tuikit.UploadRunMsg{Name: name, View: view}
	})
}

// ---------------------------------------------------------------------------
// Global SSH options (plan 06-01) — the Options sub-tab seam, projected from
// the frozen fixture so the demo's rendering stays byte-identical.
// ---------------------------------------------------------------------------

// GlobalSSHOptionStates projects the frozen GlobalSSHOptions fixture into the
// live view shape: Current/Recommended/Risk/OneLiner are the fixture's own
// values, and the NeedsAction flag becomes the row State. The provenance is a
// fixture-friendly label — the real backend names the actual file and line;
// the demo never probes a machine.
func (FixtureBackend) GlobalSSHOptionStates() ([]tuikit.GlobalSSHOptionView, error) {
	out := make([]tuikit.GlobalSSHOptionView, 0, len(tuikit.GlobalSSHOptions))
	for _, o := range tuikit.GlobalSSHOptions {
		explanation := o.OneLiner
		if o.Key == "IdentitiesOnly" {
			explanation = tuikit.GlobalSSHDetailExplanation
		}
		state := tuikit.GlobalSSHAlreadySet
		if o.NeedsAction {
			state = tuikit.GlobalSSHNeedsAction
		}
		out = append(out, tuikit.GlobalSSHOptionView{
			Key:                o.Key,
			CurrentValue:       o.Current,
			Provenance:         "fixture value — the demo does not probe this machine",
			Recommended:        o.Recommended,
			Risk:               o.Risk,
			OneLiner:           o.OneLiner,
			Explanation:        explanation,
			State:              state,
			WritableToHostStar: o.Key != "IdentitiesOnly",
		})
	}
	return out, nil
}

// GlobalSSHApplyPlan is deliberately EMPTY for the demo: the ceremony falls
// back to its own fixture-derived target/backup/preview (the frozen Phase-2
// shape), and the demo never computes a real diff. The REAL backend returns
// the resolved storage target, promised backups and a genuine diff.
func (FixtureBackend) GlobalSSHApplyPlan([]string) (tuikit.GlobalSSHApplyPlanView, error) {
	return tuikit.GlobalSSHApplyPlanView{}, nil
}

// CommitGlobalSSH keeps the approved dummy apply flow in memory — it never
// touches HOME, reporting the fixture receipt after the same brief tick the
// other async fixture commands use.
func (FixtureBackend) CommitGlobalSSH([]string) tea.Cmd {
	backup := tuikit.NewBackupPath("~/.ssh/config")
	return tea.Tick(fixtureStageDelay, func(time.Time) tea.Msg {
		return tuikit.GlobalSSHCommitMsg{Backups: []string{backup}}
	})
}

// ---------------------------------------------------------------------------
// GlobalGitPlanner (plan 07-01) — the demo satisfies the seam from frozen
// fixtures. cmd/gitid-dummy renders its Global Git screen byte-identically
// to today through this seam; cmd/gitid injects a real GlobalGitPlanner.
// ---------------------------------------------------------------------------

// GlobalGitOptionStates projects the frozen GlobalGitOptions fixture into the
// live view shape: Current/Recommended/OneLiner are the fixture's own values,
// and the NeedsAction flag becomes the row State. The provenance is a
// fixture-friendly label — the real backend names the actual file; the demo
// never probes a machine.
func (FixtureBackend) GlobalGitOptionStates() ([]tuikit.GlobalGitOptionView, error) {
	out := make([]tuikit.GlobalGitOptionView, 0, len(tuikit.GlobalGitOptions))
	for _, o := range tuikit.GlobalGitOptions {
		state := tuikit.GlobalGitAlreadySet
		if o.NeedsAction {
			state = tuikit.GlobalGitNeedsAction
		}
		out = append(out, tuikit.GlobalGitOptionView{
			Key:               o.Key,
			CurrentValue:      o.Current,
			Provenance:        "fixture value — the demo does not probe this machine",
			Recommended:       o.Recommended,
			OneLiner:          o.OneLiner,
			State:             state,
			PolicyBacked:      o.Key != tuikit.GlobalGitEmailFallbackKey,
			HasWritableMember: o.Key != tuikit.GlobalGitEmailFallbackKey,
		})
	}
	return out, nil
}

// GlobalGitApplyPlan is deliberately EMPTY for the demo: the ceremony falls
// back to its own fixture-derived target/backup/preview (the frozen Phase-2
// shape), and the demo never computes a real diff. The REAL backend returns
// the resolved baseline target, promised backups and a genuine diff.
func (FixtureBackend) GlobalGitApplyPlan([]string) (tuikit.GlobalGitApplyPlanView, error) {
	return tuikit.GlobalGitApplyPlanView{}, nil
}

// CommitGlobalGit keeps the approved dummy apply flow in memory — it never
// touches HOME, reporting the fixture receipt after the same brief tick the
// other async fixture commands use.
func (FixtureBackend) CommitGlobalGit([]string) tea.Cmd {
	backup := tuikit.NewBackupPath("~/.gitconfig.d/00-baseline")
	return tea.Tick(fixtureStageDelay, func(time.Time) tea.Msg {
		return tuikit.GlobalGitCommitMsg{Backups: []string{backup}}
	})
}

// GitFallbackAuthorState returns an empty pair — the demo's fallback
// fields start unset, matching the recipes default.
func (FixtureBackend) GitFallbackAuthorState() (tuikit.GitFallbackAuthorView, error) {
	return tuikit.GitFallbackAuthorView{}, nil
}

// GitFallbackAuthorPlan is the demo apply preview: the frozen ~/.gitconfig
// target, a promised backup, and a locally-composed pair snapshot.
func (FixtureBackend) GitFallbackAuthorPlan(name, email string) (tuikit.GitFallbackAuthorPlanView, error) {
	preview := "+ [user]\n"
	if name != "" {
		preview += "+     name = " + name + "  " + tuikit.GlobalGitEmailDiffAnnotation + "\n"
	}
	if email != "" {
		preview += "+     email = " + email + "  " + tuikit.GlobalGitEmailDiffAnnotation + "\n"
	}
	return tuikit.GitFallbackAuthorPlanView{
		Targets: []string{"~/.gitconfig"},
		Backups: []string{tuikit.NewBackupPath("~/.gitconfig")},
		Diff:    strings.TrimRight(preview, "\n"),
		Removal: name == "" && email == "",
	}, nil
}

// CommitGitFallbackAuthor keeps the approved dummy apply flow in memory —
// it never touches HOME, reporting the fixture receipt after the same
// brief tick the other async fixture commands use.
func (FixtureBackend) CommitGitFallbackAuthor(string, string) tea.Cmd {
	backup := tuikit.NewBackupPath("~/.gitconfig")
	return tea.Tick(fixtureStageDelay, func(time.Time) tea.Msg {
		return tuikit.GitFallbackAuthorCommitMsg{Backups: []string{backup}}
	})
}

// SSHStorageMigrationPlan returns the frozen STORE-01 fixture previews so the
// dummy's Storage sub-tab render stays byte-identical to before this plan.
func (FixtureBackend) SSHStorageMigrationPlan(layout tuikit.SSHStorageLayout) (tuikit.SSHStorageMigrationView, error) {
	s := FixtureBackend{}.InitialState()
	toInclude := layout == tuikit.StorageInclude
	headingTail := "sentinel blocks in ~/.ssh/config"
	diff := "+ gitid blocks written back, sentinel-delimited, into ~/.ssh/config\n- Include ~/.ssh/config.d/gitid.config (line removed)\n- ~/.ssh/config.d/gitid.config (file retired)\n  everything outside gitid blocks: untouched"
	if toInclude {
		headingTail = "Include’d gitid.config"
		diff = "+ Include ~/.ssh/config.d/gitid.config   (near the top of ~/.ssh/config)\n+ ~/.ssh/config.d/gitid.config (all gitid blocks move here)\n- # BEGIN/END gitid managed blocks removed from ~/.ssh/config\n  everything outside gitid blocks: untouched"
	}
	return tuikit.SSHStorageMigrationView{
		CurrentLayout:   s.SSHStorage,
		TargetLayout:    layout,
		Heading:         "Migrate SSH storage layout → " + headingTail,
		Targets:         []string{"~/.ssh/config", "~/.ssh/config.d/gitid.config"},
		Backups:         []string{tuikit.NewBackupPath("~/.ssh/config")},
		Diff:            diff,
		MainPreview:     tuikit.IncludePreviewMain,
		OwnedPreview:    tuikit.IncludePreviewOwned(s),
		SentinelPreview: tuikit.SentinelPreview(s),
		PlanToken:       "fixture-token-" + string(layout),
	}, nil
}

// CommitSSHStorage keeps the approved dummy migrate flow in memory — it never
// touches HOME, reporting the fixture receipt after the same brief tick the
// other async fixture commands use.
func (FixtureBackend) CommitSSHStorage(tuikit.SSHStorageLayout, string) tea.Cmd {
	backup := tuikit.NewBackupPath("~/.ssh/config")
	return tea.Tick(fixtureStageDelay, func(time.Time) tea.Msg {
		return tuikit.SSHStorageCommitMsg{Backups: []string{backup}}
	})
}

// ---------------------------------------------------------------------------
// Clone (D-14/D-15/D-16/D-17, MGR-04) — plan 05-05.
// ---------------------------------------------------------------------------

// fixtureCloneSuffix mirrors identity.CloneSuffix's value — the dummy has no
// backend package to import (tuikit's own import-allowlist rule extends to
// its Backend implementers), so this is a deliberate copy of the frozen
// literal, not a shared constant.
const fixtureCloneSuffix = "-clone"

// fixtureCopiedFieldGitName/Email mirror identity.CopiedFieldGitName/Email's
// values exactly — the D-14 review-flag field identifiers.
const (
	fixtureCopiedFieldGitName  = "user.name"
	fixtureCopiedFieldGitEmail = "user.email"
)

// fixtureNameTaken is the dummy's D-17 taken-name check over the fixture
// rows, case-insensitive (SSH host patterns are matched case-insensitively).
func fixtureNameTaken(candidate string) bool {
	for _, row := range IdentityManagerRows {
		if strings.EqualFold(row.Name, candidate) {
			return true
		}
	}
	return false
}

// findFixtureRow resolves a fixture row by name.
func findFixtureRow(name string) (IdentityManagerRow, bool) {
	for _, row := range IdentityManagerRows {
		if row.Name == name {
			return row, true
		}
	}
	return IdentityManagerRow{}, false
}

// SuggestCloneName mirrors identity.SuggestCloneName's D-17 silent-bump
// semantics over the fixture rows.
func (FixtureBackend) SuggestCloneName(source string) string {
	base := source + fixtureCloneSuffix
	if !fixtureNameTaken(base) {
		return base
	}
	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s-%d", base, n)
		if !fixtureNameTaken(candidate) {
			return candidate
		}
	}
}

// KeyActionFor answers from fixture rows: key-missing routes to repair,
// every other classified state routes to rotate.
func (FixtureBackend) KeyActionFor(name string) (string, error) {
	row, ok := findFixtureRow(name)
	if !ok {
		return "", fmt.Errorf("unknown identity %q", name)
	}
	if row.State == "key-missing" {
		return tuikit.KeyCeremonyModeRepair, nil
	}
	return tuikit.KeyCeremonyModeRotate, nil
}

// KeyCeremonyPlan returns fixture facts for the rotate or repair ceremony.
func (FixtureBackend) KeyCeremonyPlan(name, mode string) (tuikit.KeyCeremonyView, error) {
	row, ok := findFixtureRow(name)
	if !ok {
		return tuikit.KeyCeremonyView{}, fmt.Errorf("unknown identity %q", name)
	}
	keyPath := row.KeyPath
	if keyPath == "" {
		keyPath = "~/.ssh/id_ed25519_" + name
	}
	provider := strings.TrimPrefix(row.SSHHost, name+".")
	if provider == "" {
		provider = "github.com"
	}
	plan := tuikit.KeyCeremonyView{
		Mode:         mode,
		IdentityName: name,
		ProviderHost: provider,
		KeyPath:      keyPath,
		PubKeyPath:   keyPath + ".pub",
		Targets:      []string{"~/.ssh/config", "~/.ssh/allowed_signers", keyPath},
		Backups:      []string{tuikit.NewBackupPath("~/.ssh/config")},
	}
	if mode == tuikit.KeyCeremonyModeRotate {
		plan.ArchivedKeyPath = "~/.ssh/gitid-archive/id_ed25519_" + name + ".old"
	}
	return plan, nil
}

// DeletePlan answers from fixture rows so the demo delete screens render a
// real plan value instead of UI-invented paths.
func (FixtureBackend) DeletePlan(name, scope string) (tuikit.DeletePlanView, error) {
	row, _ := findFixtureRow(name)
	fragment := row.GitFragmentPath
	if fragment == "" {
		fragment = "~/.gitconfig.d/" + name
	}
	keyPath := row.KeyPath
	if keyPath == "" {
		keyPath = "~/.ssh/id_ed25519_" + name
	}
	plan := tuikit.DeletePlanView{Name: name, Scope: scope, Backups: []string{tuikit.NewBackupPath("~/.gitconfig")}}
	plan.Targets = []tuikit.DeleteTargetView{
		{File: "~/.gitconfig", Block: name, Label: "Git includeIf block"},
		{File: fragment, Block: "", Label: "Git fragment file"},
	}
	if scope != "everything" {
		return plan, nil
	}
	plan.Targets = append(plan.Targets,
		tuikit.DeleteTargetView{File: "~/.ssh/config", Block: name, Label: "SSH Host block"},
		tuikit.DeleteTargetView{File: "~/.ssh/allowed_signers", Block: name, Label: "allowed_signers entry"},
		tuikit.DeleteTargetView{File: keyPath, Block: "", Label: "Key pair"},
	)
	plan.Backups = []string{tuikit.NewBackupPath("~/.ssh/config"), tuikit.NewBackupPath("~/.gitconfig")}
	return plan, nil
}

// ClonePrefill mirrors identity.DeriveCloneInput's D-14 copy/re-derive split
// over the fixture rows: user.name/user.email are copied verbatim from the
// source (when the source has a Git identity at all) and reported in
// CopiedFields; everything else is re-derived from cloneName.
func (b FixtureBackend) ClonePrefill(source, cloneName string, reuseSourceKey bool) (tuikit.ClonePrefillView, error) {
	cloneName = strings.TrimSpace(cloneName)
	if cloneName == "" {
		return tuikit.ClonePrefillView{}, fmt.Errorf("clone name is required")
	}
	if strings.EqualFold(cloneName, source) {
		return tuikit.ClonePrefillView{}, fmt.Errorf("clone name must differ from the source name")
	}
	if fixtureNameTaken(cloneName) {
		return tuikit.ClonePrefillView{}, fmt.Errorf("%q is already in use", cloneName)
	}
	src, ok := findFixtureRow(source)
	if !ok {
		return tuikit.ClonePrefillView{}, fmt.Errorf("clone: source identity %q not found", source)
	}
	provider := "github.com"
	if src.SSHHost != "" {
		provider = providerHostFromAlias(src.SSHHost)
	}
	hostname, port := b.ProviderDefaults(provider)
	gitName, gitEmail := "", ""
	if src.GitFragmentPath != "" {
		gitName = src.Name + " identity"
		gitEmail = "you@" + src.Name + ".example"
	}
	view := tuikit.ClonePrefillView{
		SourceName:    source,
		CloneName:     cloneName,
		AliasPrefix:   cloneName,
		Hostname:      hostname,
		Port:          port,
		GitName:       gitName,
		GitEmail:      gitEmail,
		MatchStrategy: GitScreenMatchStrategyDefault,
		GitDir:        "~/git/" + cloneName + "/",
		CopiedFields:  []string{fixtureCopiedFieldGitName, fixtureCopiedFieldGitEmail},
	}
	if reuseSourceKey {
		view.ReuseKeyPath = src.KeyPath
	}
	return view, nil
}
