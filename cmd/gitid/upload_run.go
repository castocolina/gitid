package main

// upload_run.go is the ONE upload orchestration both the TUI backend
// (realBackend.RunUpload) and every CLI upload-capable verb call. Plan
// 09-02/09-03/09-04 built the decision logic — eligibility, staging-derived
// public-key path, inventory, missing-type diff, per-type upload,
// classification, D-17 confirmation with one bounded retry, manual-fallback
// selection — inside realBackend.RunUpload's tea.Cmd closure. 09-05-PLAN.md's
// Task 1 relocates it here so the CLI can reach it without a second copy: a
// second copy is exactly how the missing-type diff, the classifier mapping,
// or the confirmation retry would drift between the TUI and the CLI
// (cmd/gitid/identity.go's own header comment (17-27) explains why the
// two-commands-from-one-spec pattern exists for the SAME reason).
//
// The decision logic is split into two phases, planUpload and executeUpload,
// rather than one flat function, because the TUI's D-02/R7 announce-before-
// run beat needs an OBSERVABLE gap between "here is the command about to
// run" and "here is what happened" — a single synchronous function cannot
// produce that gap, since a caller only sees its return value once
// everything has already happened. planUpload performs every READ-ONLY
// decision step (eligibility, staging, the pre-upload inventory dedupe read)
// and returns either a terminal View (nothing left to do — omitted,
// disabled, staging failed, or already complete) or an uploadPlan carrying
// the commands about to run. executeUpload performs the actual upload and
// the D-17 confirmation. runUploadFor is exactly planUpload-then-
// executeUpload with no gap — the CLI's single-shot entry point, since a
// CLI process has no live rendering loop to show an intermediate state to.
// realBackend.RunUpload (wiring.go) composes the SAME two functions with the
// gap R7 requires, by returning UploadStartedMsg with a FollowUp that calls
// executeUpload once the model has rendered the announce. There is exactly
// one implementation of the decision logic in this file; there are two
// different ways of SEQUENCING it for two different callers.
//
// uploadRequest is the smallest struct that serves both callers without
// either converting through the other's type: the TUI's tuikit.CreateSpec
// carries wizard-only fields (Algorithm, Port, ReuseKeyPath) an existing,
// already-committed identity does not have, and the CLI's identity.Account
// carries persistence-layer fields (KeyPath, FragmentPath, Matches) the
// upload decision never needs. uploadRequestFromSpec and
// uploadRequestForAccount are the two thin adapters.

import (
	"fmt"
	"io"

	"github.com/castocolina/gitid/internal/identity"
	"github.com/castocolina/gitid/internal/tuikit"
	"github.com/castocolina/gitid/internal/upload"
	"github.com/castocolina/gitid/internal/uploader"
)

// uploadRequest is the provider-agnostic input every upload decision needs:
// which identity, which hostname (the D-13 provider gate reads this), and
// the path to a REAL, already-on-disk public key file.
type uploadRequest struct {
	Identity string
	Hostname string
	PubPath  string
}

// uploadRequestForAccount adapts an already-committed identity.Account (the
// CLI's shape: register-key, and the derived-autonomous-upload step on
// create/clone/rotate/new-key) into an uploadRequest. identity.Account's
// PubPath is the DISPLAY form (may carry a literal "~/" prefix, mirroring
// identity_clone.go's own src.KeyPath = expandTildeForHome(...) call for
// the same reason) — home expands it to a real, absolute path before any
// file read is attempted.
func uploadRequestForAccount(acct identity.Account, home string) uploadRequest {
	return uploadRequest{Identity: acct.Name, Hostname: acct.Hostname, PubPath: expandTildeForHome(acct.PubPath, home)}
}

// uploadRequestFromSpec adapts the TUI wizard's tuikit.CreateSpec into an
// uploadRequest, performing the ONE piece of real work an account-shaped
// request never needs: staging the key if it was just generated (never yet
// written to its FinalPubPath, per CR-02/CR-08 — the real ~/.ssh stays
// untouched until the wizard's confirmed commit) and, for a freshly
// generated key, writing a temp .pub sibling next to the already-staged
// temp private key so there is a real file to read (mirrors TestStage1's
// own TempPrivatePath pattern; see wiring.go's prior RunUpload for the
// original, now-relocated version of this logic).
func (b *realBackend) uploadRequestFromSpec(spec tuikit.CreateSpec) (uploadRequest, error) {
	in := b.createInputFromSpec(spec)
	staged, err := b.stagedKeyFor(in, spec.ReuseKeyPath)
	if err != nil {
		return uploadRequest{}, err
	}
	pubPath := staged.FinalPubPath
	if staged.PrivPEM != nil {
		tempPub := staged.TempPrivatePath + ".pub"
		if !b.deps.PubExists(tempPub) {
			if werr := b.deps.WritePub(tempPub, staged.PubLine); werr != nil {
				return uploadRequest{}, werr
			}
		}
		pubPath = tempPub
	}
	return uploadRequest{Identity: spec.Identity, Hostname: spec.Hostname, PubPath: pubPath}, nil
}

// uploadPlan carries everything executeUpload needs, computed once by
// planUpload so the CLI's single-shot runUploadFor and the TUI's two-message
// RunUpload never issue the pre-upload inventory read twice.
type uploadPlan struct {
	tool          uploader.Tool
	toolPath      string
	pubPath       string
	pubLine       string
	wanted        []uploader.Registration
	existing      []uploader.ExistingKey
	degraded      bool
	canonicalHost string
	providerName  string
	reqs          []uploader.RegistrationRequest
	commands      []string
}

// planUpload performs every read-only decision step: the D-13 provider
// gate, tool detection, reading the public key, the D-15 pre-upload
// inventory dedupe read, and the missing-type diff. It returns a non-nil
// terminal view when there is nothing left to execute (omitted, disabled,
// a staging/read failure, or the identity is already fully registered);
// otherwise it returns a plan and a nil terminal view.
func (b *realBackend) planUpload(req uploadRequest) (plan uploadPlan, terminal *tuikit.UploadRunView) {
	// D-13: Omitted (provider not gated) and Disabled (no matching CLI on
	// PATH) both skip autonomy entirely. Unauth is deliberately NOT resolved
	// here a second time — see the original RunUpload's own comment,
	// preserved verbatim in intent: the checked path (TUI) or the caller's
	// own explicit invocation (CLI register-key, or default-on write verbs)
	// reaches here without a second auth probe; an unauthenticated attempt
	// simply fails naturally inside UploadKeys and is classified below.
	provider, canonicalHost := uploader.ProviderForHostname(req.Hostname)
	if provider == "" {
		view := tuikit.UploadRunView{Skipped: true}
		return uploadPlan{}, &view
	}
	providerName := providerDisplayName(provider)
	tool, toolPath, status := uploader.DetectFor(provider, b.uploaderDeps)
	if status == uploader.AuthToolNotFound {
		view := tuikit.UploadRunView{Skipped: true, ManualFallback: upload.Instructions(req.Hostname), ProviderName: providerName}
		return uploadPlan{}, &view
	}

	pubBytes, err := b.uploaderDeps.ReadFile(req.PubPath)
	if err != nil {
		view := uploadFailureView(err.Error(), req.Hostname)
		return uploadPlan{}, &view
	}
	pubLine := string(pubBytes)
	wanted := desiredRegistrations(tool)
	b.setUploadPhase(uploadPhaseDedupe)
	existing, ierr := uploader.Inventory(tool, toolPath, b.uploaderDeps)
	degraded := ierr != nil
	missing := wanted
	if !degraded {
		missing = uploader.MissingRegistrations(existing, pubLine, wanted)
	}
	if len(missing) == 0 {
		view := tuikit.UploadRunView{AlreadyComplete: true, ProviderName: providerName}
		return uploadPlan{}, &view
	}

	title := uploader.KeyTitle(req.Identity, shortHostname())
	reqs := uploader.RegistrationRequestsWithTitle(title, missing...)
	commands := make([]string, 0, len(reqs))
	for _, r := range reqs {
		keyType, typeErr := r.Registration.KeyTypeFor(tool)
		if typeErr != nil {
			view := uploadFailureView(typeErr.Error(), req.Hostname)
			return uploadPlan{}, &view
		}
		commands = append(commands, uploader.CommandPreview(tool, toolPath, req.PubPath, r.Title, keyType))
	}

	return uploadPlan{
		tool: tool, toolPath: toolPath, pubPath: req.PubPath, pubLine: pubLine,
		wanted: wanted, existing: existing, degraded: degraded,
		canonicalHost: canonicalHost, providerName: providerName,
		reqs: reqs, commands: commands,
	}, nil
}

// executeUpload runs plan's registrations and the D-17 post-upload
// confirmation, returning the completed view. It is safe to call from
// either a synchronous CLI path or a tea.Cmd goroutine; it performs no
// locking of its own beyond what confirmUpload already does.
func (b *realBackend) executeUpload(hostname string, plan uploadPlan) (view tuikit.UploadRunView) {
	defer func() {
		if recovered := recover(); recovered != nil {
			view = uploadFailureView("gitid internal defect: "+uploader.RedactCLIOutput(fmt.Sprint(recovered), b.home, 58), hostname)
		}
	}()
	results := uploader.UploadKeys(plan.tool, plan.toolPath, plan.pubPath, plan.reqs, b.uploaderDeps)
	byRegistration := make(map[uploader.Registration]tuikit.UploadResultRow, len(results))
	for _, result := range results {
		byRegistration[result.Registration] = toUploadResultRow(plan.tool, result, plan.canonicalHost, b.home)
	}
	rows := make([]tuikit.UploadResultRow, 0, len(plan.wanted))
	for _, registration := range plan.wanted {
		if row, ok := byRegistration[registration]; ok {
			rows = append(rows, row)
			continue
		}
		if !plan.degraded && uploader.HasRegistration(plan.existing, plan.pubLine, registration) {
			rows = append(rows, toUploadResultRow(plan.tool, uploader.RegistrationResult{Registration: registration, Outcome: uploader.OutcomeAlreadyPresent}, plan.canonicalHost, b.home))
		}
	}
	// D-17: the ssh -T half of D-17's verification is the wizard's EXISTING
	// stage-1/stage-2 gate (TUI) or the write verb's own post-write re-test
	// (CLI rotate/new-key); no new probe belongs here, only the inventory
	// confirmation ssh -T structurally cannot see (the signing registration
	// has no ssh -T-observable side effect).
	rows, confirmDegraded := b.confirmUpload(plan.tool, plan.toolPath, plan.pubLine, plan.canonicalHost, plan.wanted, rows)

	view = tuikit.UploadRunView{Rows: rows, InventoryDegraded: plan.degraded || confirmDegraded, ProviderName: plan.providerName}
	allFailed := len(results) > 0
	for _, row := range results {
		if row.Outcome != uploader.OutcomeFailed {
			allFailed = false
		}
	}
	if allFailed {
		view.ManualFallback = upload.Instructions(hostname)
	}
	return view
}

// runUploadFor is the CLI's single-shot upload entry point: plan, then (if
// there is anything to do) execute, synchronously. This is the function a
// script-facing verb calls; it never returns a tea.Cmd or a tea.Msg.
func (b *realBackend) runUploadFor(req uploadRequest) (view tuikit.UploadRunView) {
	defer func() {
		if recovered := recover(); recovered != nil {
			view = uploadFailureView("gitid internal defect: "+uploader.RedactCLIOutput(fmt.Sprint(recovered), b.home, 58), req.Hostname)
		}
	}()
	plan, terminal := b.planUpload(req)
	if terminal != nil {
		return *terminal
	}
	return b.executeUpload(req.Hostname, plan)
}

// printUploadOutcome renders view as plain text, byte-identical in wording
// to the wizard's own upload section: every string emitted here is a
// reference to an internal/tuikit frozen Upload* constant, never CLI-local
// wording (09-UI-SPEC.md's shown==run contract extended to the CLI surface).
// It writes nothing at all for the omitted state (view is the zero value).
func printUploadOutcome(w io.Writer, view tuikit.UploadRunView) {
	if view.InventoryDegraded {
		fmt.Fprintln(w, fmt.Sprintf(tuikit.UploadInventoryDegradedFmt, view.ProviderName)) //nolint:errcheck // best-effort stdout
	}
	if view.Skipped {
		fmt.Fprintln(w, tuikit.UploadSkippedByFlagNote) //nolint:errcheck // best-effort stdout
	}
	for _, row := range view.Rows {
		if row.Command != "" {
			fmt.Fprintln(w, fmt.Sprintf(tuikit.UploadRunningLineFmt, row.Command)) //nolint:errcheck // best-effort stdout
		}
	}
	if view.AlreadyComplete {
		fmt.Fprintln(w, fmt.Sprintf(tuikit.UploadAlreadyCompleteFmt, view.ProviderName)) //nolint:errcheck // best-effort stdout
	}
	for _, row := range view.Rows {
		switch row.Outcome {
		case tuikit.UploadRowUploaded:
			fmt.Fprintln(w, fmt.Sprintf(tuikit.UploadResultOKFmt, row.Label)) //nolint:errcheck // best-effort stdout
		case tuikit.UploadRowAlreadyPresent:
			fmt.Fprintln(w, fmt.Sprintf(tuikit.UploadResultSkippedFmt, row.Label)) //nolint:errcheck // best-effort stdout
		case tuikit.UploadRowFailed:
			fmt.Fprintln(w, fmt.Sprintf(tuikit.UploadResultFailedFmt, row.Label, row.Reason)) //nolint:errcheck // best-effort stdout
		}
	}
	if view.ManualFallback != "" {
		fmt.Fprintln(w, tuikit.UploadManualHeading) //nolint:errcheck // best-effort stdout
		fmt.Fprint(w, view.ManualFallback)          //nolint:errcheck // best-effort stdout
	}
}

// uploadPhaseDedupe / uploadPhaseConfirmation are the R8 phase markers
// realBackend.uploadPhase is set to immediately before each of this file's
// two uploader.Inventory call sites — planUpload's pre-upload missing-type
// diff and confirmUpload's D-17 post-upload confirmation read(s),
// respectively. A test fake's RunCmd closure reads currentUploadPhase() to
// attribute each recorded invocation to the correct phase, so "exactly one
// confirmation retry" can be proven without conflating it with the
// unrelated dedupe read.
const (
	uploadPhaseDedupe       = "dedupe"
	uploadPhaseConfirmation = "confirmation"
)

// uploadUnconfirmedReasonFmt marks a row whose registration the provider
// accepted (Uploaded/AlreadyPresent) but whose post-upload confirmation
// read still could not see after the one bounded retry (D-17). This is
// NOT a failure: turning it into one would misreport a successful upload
// as rejected. It stays informational text on the SAME UploadRowUploaded/
// UploadRowAlreadyPresent outcome — no fourth UploadRowOutcome value exists
// for this case.
const uploadUnconfirmedReasonFmt = "accepted but not yet visible in %s's inventory — this can lag briefly after upload; re-run gitid's test to confirm"

// confirmUpload implements D-17's post-upload verification: one inventory
// read to confirm every registration this run reported as Uploaded or
// AlreadyPresent is actually present, with exactly one bounded retry when
// it is not. It never gates and never retries a FAILING READ itself a
// second time — only a registration that is legitimately still missing
// after a successful read gets the one retry.
func (b *realBackend) confirmUpload(tool uploader.Tool, toolPath, pubLine, providerHost string, wanted []uploader.Registration, rows []tuikit.UploadResultRow) ([]tuikit.UploadResultRow, bool) {
	toConfirm := make(map[uploader.Registration]int, len(wanted))
	for i, registration := range wanted {
		if i >= len(rows) {
			continue
		}
		if rows[i].Outcome == tuikit.UploadRowUploaded || rows[i].Outcome == tuikit.UploadRowAlreadyPresent {
			toConfirm[registration] = i
		}
	}
	if len(toConfirm) == 0 {
		return rows, false
	}

	read := func() (map[uploader.Registration]bool, bool) {
		b.setUploadPhase(uploadPhaseConfirmation)
		existing, err := uploader.Inventory(tool, toolPath, b.uploaderDeps)
		if err != nil {
			return nil, true
		}
		present := make(map[uploader.Registration]bool, len(toConfirm))
		for registration := range toConfirm {
			present[registration] = uploader.HasRegistration(existing, pubLine, registration)
		}
		return present, false
	}

	present, degraded := read()
	if degraded {
		return rows, true
	}
	var stillMissing []uploader.Registration
	for registration, ok := range present {
		if !ok {
			stillMissing = append(stillMissing, registration)
		}
	}
	if len(stillMissing) > 0 {
		b.confirmSleep()
		second, degraded2 := read()
		if degraded2 {
			return rows, true
		}
		for _, registration := range stillMissing {
			present[registration] = second[registration]
		}
	}
	for registration, idx := range toConfirm {
		if !present[registration] {
			rows[idx].Reason = fmt.Sprintf(uploadUnconfirmedReasonFmt, providerHost)
		}
	}
	return rows, false
}

// rotateDeleteOfferFor resolves D-04's interactive old-key delete offer for
// an EXISTING identity via a FRESH provider inventory read — never on the
// render path (R3), and never cached across calls: D-04's Open Question 2 is
// resolved lazily, at result-screen time, because an eager resolution during
// KeyCeremonyPlan would cache an ID captured before the user even confirmed
// the rotate. Matching is by EXACT title equality against THIS machine's
// title (uploader.KeyTitle + uploader.FindByTitle) — never a substring or a
// name-only match — because D-07's whole point is that a rotate on one
// machine must never offer to delete a DIFFERENT machine's still-in-use key.
// Every failure path (unknown identity, non-qualifying provider, tool
// absent/unauthenticated, inventory error, no match) returns an
// Available=false view with a reason; it never returns an error the caller
// must special-case. This lives here, not in wiring.go, so the ONE
// uploader.Inventory call this beat makes stays inside the shared
// decision-logic file the wiring_test.go/upload_run_test.go AST checks
// guard (R3's "decision logic lives only in upload_run.go" rule).
func (b *realBackend) rotateDeleteOfferFor(name string) tuikit.RotateDeleteOfferView {
	acct, ok := b.findAccount(name)
	if !ok {
		return tuikit.RotateDeleteOfferView{Unavailable: fmt.Sprintf("identity %q not found", name)}
	}
	provider, canonicalHost := uploader.ProviderForHostname(acct.Hostname)
	if provider == "" {
		return tuikit.RotateDeleteOfferView{Unavailable: "provider not eligible for autonomous key management"}
	}
	tool, toolPath, status := uploader.DetectFor(provider, b.uploaderDeps)
	if status == uploader.AuthToolNotFound {
		return tuikit.RotateDeleteOfferView{Unavailable: fmt.Sprintf("%s CLI not found on PATH", providerToolName(provider))}
	}
	if uploader.AuthCheck(toolPath, b.uploaderDeps, canonicalHost) != uploader.AuthAuthenticated {
		return tuikit.RotateDeleteOfferView{Unavailable: fmt.Sprintf("not authenticated with %s", providerDisplayName(provider))}
	}
	existing, err := uploader.Inventory(tool, toolPath, b.uploaderDeps)
	if err != nil {
		return tuikit.RotateDeleteOfferView{Unavailable: "could not read the existing key inventory"}
	}
	title := uploader.KeyTitle(name, shortHostname())
	found, ok := uploader.FindByTitle(existing, title)
	if !ok {
		return tuikit.RotateDeleteOfferView{Unavailable: "no matching old key found on this machine"}
	}
	return tuikit.RotateDeleteOfferView{
		Available:     true,
		ProviderName:  providerDisplayName(provider),
		IdentityName:  name,
		MachineName:   shortHostname(),
		KeyTitle:      found.Title,
		KeyID:         found.ID,
		ManualCommand: uploader.DeleteCommandPreview(tool, toolPath, found.ID),
	}
}
