// Package dummytui implements the navigation-only Go TUI "dummy" for gitid's
// Phase 2 design checkpoint (DLV-05 front half, DLV-02 cross-media parity).
//
// # DLV-05 no-backend ALLOWLIST rule
//
// internal/dummytui and cmd/gitid-dummy are a physically separate binary from
// the shipped cmd/gitid product. They MUST NOT import any first-party
// github.com/castocolina/gitid/... package other than internal/dummytui
// itself (and its own subpackages, if any). No internal/identity, keygen,
// sshconfig, gitconfig, filewriter, tester, doctor, adopter, uploader,
// repoclone — nothing. This is enforced as an ALLOWLIST (strictly stronger
// than a denylist) by nobackend_test.go, which shells out to
// `go list -deps ./cmd/gitid-dummy/... ./internal/dummytui/...` and fails on
// ANY first-party package other than exactly internal/dummytui and
// cmd/gitid-dummy. An allowlist catches a NEW or RENAMED backend package by
// construction; a denylist (an explicit list of forbidden packages) would
// need to be updated every time a new backend package is added — the
// allowlist form never goes stale.
//
// The dummy renders static/scripted view state only: hardcoded fixture data
// in data.go, no filesystem reads of ~/.ssh or ~/.gitconfig, no exec.Command
// against ssh/git, no network calls. It exists purely to prove full
// navigability of the shell + all seven surfaces' screens before any backend
// logic is written (DLV-05's per-surface order: HTML mockup -> TUI dummy ->
// user approval -> backend).
//
// # Modal-launch contract
//
// The five primary surfaces (identity-manager, global-ssh, global-git,
// health, fixer) are reachable via number keys 1-5 (SurfaceDef.ActivationKey).
// The two modal-flow surfaces (create-flow, git-screen, added by later
// fan-out plans) are KEYLESS: they set SurfaceDef.ActivationKey to "" and
// instead declare SurfaceDef.LaunchFrom (the source surface ID) and
// SurfaceDef.LaunchKey (the key, pressed while LaunchFrom is the active
// surface, that launches them). This is TARGET-OWNED: the modal surface's own
// file names the source surface + key that launches it, so a fan-out plan
// wires its own launch binding without ever editing the source surface's
// file, model.go, or data.go.
//
// route() (registry.go) resolves a key press in this order (registry.go's
// pure reducer): while a modal is active (navState.modalStack is non-empty),
// number keys 1-5 are ignored and Esc pops the top modal frame; while no
// modal is active, a key resolves via the DEFINED PRECEDENCE below.
//
// # Key-allocation table (single authority)
//
// This is the key-allocation table. It mirrors 02-UX-DIRECTION.md section 2
// verbatim. Every surface
// MUST allocate its ActivationKey, its keyless LaunchKey, and its
// intra-surface ScreenDef.Keys against THIS table — never independently. All
// keys below are pressed while identity-manager is the active surface, so
// they must be mutually distinct and distinct from the reserved keys.
//
//	Key | Owner            | Kind                          | Meaning
//	----|-------------------|-------------------------------|------------------------------
//	1   | identity-manager  | ActivationKey (number-key)    | Identities / home
//	2   | global-ssh        | ActivationKey                 | Global SSH options
//	3   | global-git        | ActivationKey                 | Global Git options
//	4   | health            | ActivationKey                 | Health
//	5   | fixer             | ActivationKey                 | Fixer
//	n   | create-flow       | LaunchKey (LaunchFrom=identity-manager) | launch new-identity modal
//	g   | git-screen        | LaunchKey (LaunchFrom=identity-manager) | launch git-config modal
//	a   | identity-manager  | intra-surface ScreenDef.Keys  | -> action-menu
//	c   | identity-manager  | intra-surface ScreenDef.Keys  | -> clone-name-prompt
//	d   | identity-manager  | intra-surface ScreenDef.Keys  | -> delete-choice
//	Enter | (all)           | reserved                      | activate / open detail / confirm
//	Esc | (all)             | reserved                      | back / cancel / pop modal
//	q ? / j k (arrows) | (all) | reserved                   | quit / help / filter / move
//
// route()'s PRECEDENCE (deterministic, this is a determinism backstop —
// registration-time collision guards below are the real enforcement):
//  1. an intra-surface transition via the active screen's ScreenDef.Keys
//  2. a keyless surface's LaunchKey whose LaunchFrom == the active surface
//  3. a number-key ActivationKey view-switch (1-5)
//
// Register and RegisterOrReplace (registry.go) run a REGISTRATION-TIME
// LaunchKey collision guard that rejects (panics on) any registration that
// would let two of the above claim the SAME key on the SAME source surface —
// so a create-flow/git-screen LaunchKey clashing with identity-manager's own
// a/c/d ScreenDef.Keys fails loudly at registration, never as a confusing
// 02-11 PTY e2e failure. Adding a surface or transition: claim a free key in
// 02-UX-DIRECTION.md section 2 FIRST, then mirror it here.
package dummytui
