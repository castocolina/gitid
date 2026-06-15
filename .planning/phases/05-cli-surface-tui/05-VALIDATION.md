---
phase: 5
slug: cli-surface-tui
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-12
---

# Phase 5 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Derived from `05-RESEARCH.md` §"Validation Architecture".

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` package (stdlib) + `go test` |
| **Config file** | none — standard `go test ./...` |
| **Quick run command** | `go test ./tui/... ./cmd/gitid/... -count=1` |
| **Full suite command** | `make test` → `go test -race ./...` |
| **Estimated runtime** | ~30 seconds (quick) / ~60 seconds (full -race) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./tui/... ./cmd/gitid/... -count=1 -run <TestForThisTask>`
- **After every plan wave:** Run `go test -race ./...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** ~30 seconds

---

## Per-Task Verification Map

> Task IDs are assigned by the planner. Behaviors below map requirement → test;
> the planner threads each into a concrete `{N}-PP-TT` task.

| Behavior | Requirement | Test Type | Automated Command | File Exists | Status |
|----------|-------------|-----------|-------------------|-------------|--------|
| Top-level `rotate`/`copy`/`host add` registered + reachable | CLI-01 | unit | `go test ./cmd/gitid/... -run TestNewRootCmd` | ❌ W0 | ⬜ pending |
| `identity copy <name>` registered under identity group | CLI-01 | unit | `go test ./cmd/gitid/... -run TestIdentityCopy` | ❌ W0 | ⬜ pending |
| `copy <name>` reads pub file, copies to clipboard, prints upload instructions | CLI-01 / CLIP-02 / UP-02 | unit | `go test ./cmd/gitid/... -run TestRunCopy` | ❌ W0 | ⬜ pending |
| `gitid completion bash` non-empty script containing "gitid" | CLI-02 | unit | `go test ./cmd/gitid/... -run TestCompletionBash` | ❌ W0 | ⬜ pending |
| `gitid completion zsh` non-empty valid zsh script | CLI-02 | unit | `go test ./cmd/gitid/... -run TestCompletionZsh` | ❌ W0 | ⬜ pending |
| `gitid completion fish` non-empty output | CLI-02 | unit | `go test ./cmd/gitid/... -run TestCompletionFish` | ❌ W0 | ⬜ pending |
| Dashboard `Init()` returns Batch of per-family `tea.Cmd`s | TUI-01 | unit | `go test ./tui/... -run TestDashboardInit` | ❌ W0 | ⬜ pending |
| `familyResultMsg` → state transitions loading→loaded | TUI-01 | unit | `go test ./tui/... -run TestDashboardFamilyResult` | ❌ W0 | ⬜ pending |
| `r` triggers refresh (runID increments, families reset) | TUI-01 | unit | `go test ./tui/... -run TestDashboardRefresh` | ❌ W0 | ⬜ pending |
| Stale results (old runID) ignored after refresh | TUI-01 | unit | `go test ./tui/... -run TestDashboardStaleResult` | ❌ W0 | ⬜ pending |
| `Enter` from dashboard pushes IdentityList screen | TUI-02 | unit | `go test ./tui/... -run TestDashboardEnterNavigates` | ❌ W0 | ⬜ pending |
| `Esc` from IdentityList pops to dashboard | TUI-02 | unit | `go test ./tui/... -run TestIdentityListEscPops` | ❌ W0 | ⬜ pending |
| `Enter` on identity item pushes IdentityDetail | TUI-02 | unit | `go test ./tui/... -run TestIdentityListEnterPushesDetail` | ❌ W0 | ⬜ pending |
| `a` from IdentityList pushes CreateForm | TUI-02 | unit | `go test ./tui/... -run TestIdentityListAddKey` | ❌ W0 | ⬜ pending |
| `e` from IdentityDetail pushes UpdateForm | TUI-02 | unit | `go test ./tui/... -run TestIdentityDetailEditKey` | ❌ W0 | ⬜ pending |
| `Tab`/`Shift+Tab` advance/retreat form focus | TUI-02 | unit | `go test ./tui/... -run TestFormTabNavigation` | ❌ W0 | ⬜ pending |
| ProveBeforeWrite: confirm only active after both phases pass | TUI-02 / D-04 | unit | `go test ./tui/... -run TestProveScreenConfirmGate` | ❌ W0 | ⬜ pending |
| ProveBeforeWrite: phase-1 failure disables confirm, shows error | TUI-02 / D-04 | unit | `go test ./tui/... -run TestProveScreenPhase1Failure` | ❌ W0 | ⬜ pending |
| `q`/`ctrl+c` returns `tea.Quit()` from any screen | TUI-01 | unit | `go test ./tui/... -run TestQuitFromAnyScreen` | ❌ W0 | ⬜ pending |
| Non-TTY no-args: exits 1, prints usage hint | TUI-01 | unit | `go test ./cmd/gitid/... -run TestNoArgsNonTTY` | ❌ W0 | ⬜ pending |
| `WindowSizeMsg` propagated to dashboard/list/prove screens | TUI-01 | unit | `go test ./tui/... -run TestWindowSizePropagation` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] Framework install: `go get charm.land/bubbletea/v2@v2.0.7 charm.land/lipgloss/v2@v2.0.3 charm.land/bubbles/v2@v2.1.0`
- [ ] `tui/model_test.go` — TUI-01/TUI-02 root model tests
- [ ] `tui/dashboard_test.go` — TUI-01 dashboard tests
- [ ] `tui/navigation_test.go` — TUI-02 stack navigation tests
- [ ] `tui/form_test.go` — TUI-02 form model tests
- [ ] `tui/prove_test.go` — TUI-02 prove-before-write tests
- [ ] `cmd/gitid/copy_test.go` — CLI-01 copy command tests
- [ ] `cmd/gitid/completion_test.go` — CLI-02 completion tests

*RED-stub convention (STATE.md): each new TUI file gets a RED stub first — `Init()` returns `nil`, `Update()` returns `m, nil`, `View()` returns `tea.NewView("")` — using `_`-param signatures so the lint-gated hook passes while the test fails genuinely.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Interactive TUI navigation feel (rendering, color on a real TTY) | TUI-01/TUI-02 | Bubble Tea program needs a real terminal; unit tests cover state transitions, not pixel rendering | Run `gitid` (no args) in a real terminal; confirm dashboard loads, arrow/vim keys navigate, Esc pops, `r` refreshes |
| `fish` completion syntax validity | CLI-02 | `fish` not installed in dev env (research §Environment Availability) | On a machine with fish: `gitid completion fish | fish -c 'source -'` |
| End-to-end in-app Create with real keygen + ssh test | TUI-02 / D-04 | Prove-before-write touches real `ssh`/keygen; unit tests use fakes via identity.Deps | Create an identity through the TUI form against a real provider; confirm two-phase test output + backup + confirm before write |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
