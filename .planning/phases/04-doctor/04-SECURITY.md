---
phase: 04-doctor
status: secured
asvs_level: 1
audited: 2026-06-12
threats_total: 24
threats_closed: 21
threats_accepted: 3
threats_open: 0
warnings: 1
register_authored_at_plan_time: true
open_blockers: 0
warning_followup: "WARNING-01 — D-01 (no filewriter import in internal/doctor) holds today but has no automated gate; add a depguard rule in .golangci.yml denying internal/filewriter under internal/doctor/**"
---

# SECURITY.md — Phase 04 (doctor) Audit

**Phase:** 04 — doctor
**ASVS Level:** 1
**Audit Date:** 2026-06-12
**Auditor:** gsd-security-auditor (claude-sonnet-4-6)
**block_on:** high (BLOCKER = OPEN threat with mitigate disposition whose code evidence is absent)

---

## Audit Summary

**Threats Closed:** 21/24
**Threats Open (BLOCKER):** 0
**Accepted Risks Documented:** 3
**Unregistered Flags:** 1 (WARNING — new direct dependency golang.org/x/term, plan-disclosed)

The D-01 trust-boundary invariant (internal/doctor imports no filewriter) is **currently true** in the code but the declared "CI grep gate" automation does not exist as an enforceable check in Makefile, .golangci.yml, or pre-commit hooks. This is recorded as a WARNING, not a BLOCKER, because: (a) the property holds today as verified by grep, and (b) the gate is claimed as a mitigation component in T-04-03 and T-04-21.

---

## Threat Verification Table

| Threat ID | Category | Disposition | Status | Evidence |
|-----------|----------|-------------|--------|----------|
| T-04-01 | InfoDisclosure | mitigate | CLOSED | cmd/gitid/doctor.go:83,89 — `//nolint:gosec // sshConfigPath/gitconfigPath is a gitid-managed path (G304)`; perms.go reads mode via `deps.Stat`, never key content |
| T-04-02 | Tampering | mitigate | CLOSED | checks/perms.go:15-19 — KEY-02 constants (0700/0600/0644/0600/0644); checkPath uses tighten-only predicate `got &^ want != 0` (flags only when actual mode is looser than target; a 0400 key vs 0600 target is not flagged); fix mode is `got & want` (never adds a bit the file lacked); `deps.FixPerm(p, s)` where s == got&want — closes over-tightened (e.g. 0400 key) edge case found in Phase-4 code review |
| T-04-03 | Tampering | mitigate | WARNING | Property holds: `grep -rn '"github.com/castocolina/gitid/internal/filewriter"' internal/doctor/` → EMPTY (verified). **But**: no automated CI grep gate exists in Makefile, .golangci.yml, or pre-commit hooks — the declared gate is manual-only. See OPEN_WARNINGS. |
| T-04-04 | Tampering | accept | CLOSED | Accepted risk documented below. ANSI guard: doctor.go:101 `isTerminalOutput(os.Stdout)` checks NO_COLOR env first (line 586-588), then ModeCharDevice |
| T-04-05 | InfoDisclosure | mitigate | CLOSED | gitconfig/baseline.go:71,146,228 — `//nolint:gosec // ... trusted gitid-managed path (G304)`; ReadBaselineState reads config values (excludesfile path, ignorecase), never private key material |
| T-04-06 | Tampering | mitigate | CLOSED | internal/deps/deps.go:38 — `exec.LookPath(name)` (no shell); install hints are printed strings in doctor.go via `d.InstallHint(t.name, currentOS)`, never executed (checks/deps.go:47-48) |
| T-04-07 | Tampering | mitigate | CLOSED | checks/deps.go import block: `doctor`, `fmt` only — no filewriter. checks/baseline.go import block: `doctor`, `gitconfig`, `strings` — no filewriter |
| T-04-08 | InfoDisclosure | mitigate | CLOSED | coherence.go:187 reads AllowedSignersPath (public signers file), coherence.go:252 reads acct.PubPath (`.pub` file) — emails, pub key lines, gpg.format values only; no private key path read |
| T-04-09 | Tampering | mitigate | CLOSED | orphans.go:109-110 — `referencedKeys := sliceToSet(deps.AllSSHHostIdentityFiles)`; AllSSHHostIdentityFiles populated by `sshconfig.ParseAllHostIdentityFiles(sshBytes)` (cmd/gitid/doctor.go:198) which parses all Host blocks; unused-key Fix is `nil` (orphans.go:135) |
| T-04-10 | Spoofing | mitigate | CLOSED | coherence.go:293-329 — `findSignerLine` two-pass WR-01 scan: exact `principal == email` check returns immediately; case-fold match recorded but scan continues; after all lines, exact match wins (line 309: `if principal == email { return true, principal }`). Byte-exact == enforced |
| T-04-11 | Tampering | mitigate | CLOSED | orphans.go:38 comment confirms no known_hosts; `grep -rn 'known_hosts' internal/doctor/` → only comment text, no read call; no filewriter import in orphans.go (import block: `fmt`, `os`, `doctor`) |
| T-04-12 | Tampering | mitigate | CLOSED | cmd/gitid/doctor.go:131,150 — `exec.Command("ssh-add", "-l")` and `exec.Command("ssh-keygen", "-lf", path)` with `//nolint:gosec // arg-slice form, no shell; fixed args (G204)` and `//nolint:gosec // arg-slice form, no shell; path is trusted gitid-managed .pub (G204/G304)` |
| T-04-13 | InfoDisclosure | mitigate | CLOSED | signing.go:65-71 — `isKeyLoaded` calls `runFp(pubKeyPath)` (ssh-keygen -lf on .pub file), extracts SHA256 token, compares against agent output. Private key never read or logged. CheckSigning reads only `acct.Matches` and `gitconfig.MatchHasconfig` |
| T-04-14 | DoS | accept | CLOSED | Accepted risk documented below |
| T-04-15 | Tampering | mitigate | CLOSED | signing.go import block: `fmt`, `strings`, `doctor`, `gitconfig` — no filewriter, no os.Chmod. All agent findings use `Fix: nil` (signing.go:100,133) |
| T-04-16 | Tampering | mitigate | CLOSED | cmd/gitid/doctor.go:270-276 — RemoveBlock closure: `filewriter.RemoveBlock(content, name)` then `filewriter.Write(path, removed, mode)`. Backup+atomic write confirmed: filewriter.go:36-38 creates `.bak.<timestamp>`, then temp+rename pattern. AddWiring delegates to sshconfig.Write/keygen.WriteAllowedSigners/gitconfig.WriteBaselineInclude — all route through filewriter |
| T-04-17 | Tampering | mitigate | CLOSED | filewriter/block.go:52-57 — `RemoveBlock` returns input unchanged when block absent (idempotent). filewriter/block.go:175-220 — `ReplaceBlock` yields byte-identical output on repeated calls. filewriter.go:36 — per-write timestamped backup |
| T-04-18 | EoP | mitigate | CLOSED | cmd/gitid/doctor.go:41-43 — `--yes` without `--fix` returns error immediately. applyFixes gate (lines 488-494): when `fix==false` and TTY, presents top-level confirm default N. confirm() in add.go:512 — `[y/N]` prompt, only `"y"` or `"yes"` accepted. No path applies fixes without either explicit `--fix` or user confirmation |
| T-04-19 | Tampering | mitigate | CLOSED | cmd/gitid/doctor.go:251-254 — FixPerm chmods to caller-supplied mode; no-widening guarantee enforced upstream: checks/perms.go checkPath passes `got & want` (tighten-only safe target) so FixPerm never receives a mode wider than the file's original permissions. WR-02: RemoveBlock mode derived from path (0644 for allowed_signers, 0600 for config files) at doctor.go:272-275 — unchanged. Over-tightened (0400 key) edge case closed: 0400 &^ 0600 == 0 → no finding, no chmod |
| T-04-20 | Tampering | mitigate | CLOSED | Fix closure paths sourced exclusively from buildDoctorDeps computed paths (doctor.go:165-201: `filepath.Join(home, ...)`) — never from free-form user input. AddWiring `path` param in each fix closure captures `sshConfigPath`, `allowedSignersPath`, or `gitconfigPath` (coherence.go:125-134, 264-265; baseline.go:59-65) |
| T-04-21 | EoP | mitigate | WARNING | Fix closures verified in cmd/gitid/doctor.go `buildDoctorDeps`. No filewriter import in internal/doctor (grep-verified). **But**: "grep gate" is manual, not automated in Makefile/CI/pre-commit. Same gap as T-04-03. See OPEN_WARNINGS |
| T-04-22 | Tampering | mitigate | CLOSED | cmd/gitid/doctor.go:216-217 — `RunSSHAdd: runSSHAdd` and `RunSSHKeygenFingerprint: runSSHKeygenFingerprint` wired in `buildDoctorDeps`. Both helpers use `exec.Command` arg-slice form (lines 131, 150) with G204 annotations. Not nil (DOC-GAP-02 closed) |
| T-04-23 | InfoDisclosure | mitigate | CLOSED | cmd/gitid/doctor.go:116 — `if len(fixable) > 0 && (fix || isTerminalInput(os.Stdin))` — fix gate skipped entirely when `fix==false` and stdin is not a TTY. isTerminalInput (line 602-604) uses `term.IsTerminal` |
| T-04-24 | DoS | accept | CLOSED | Accepted risk documented below |
| T-04-SC | Supply chain | mitigate | CLOSED | go.mod contains only 5 direct dependencies (atotto/clipboard, kevinburke/ssh_config, spf13/cobra, x/crypto, x/term) + 3 indirect (mousetrap, pflag, x/sys). golang.org/x/term v0.44.0 promoted from transitive to direct — disclosed in 04-07-SUMMARY.md as the only new dependency. No npm/pip/cargo/go packages added. |

---

## Accepted Risks

| Threat ID | Category | Acceptance Rationale |
|-----------|----------|----------------------|
| T-04-04 | Tampering (ANSI injection) | ANSI/color enabled only when `isTerminalOutput(os.Stdout)` returns true AND `NO_COLOR` is not set. Piped output is always plain text. The only ANSI escape sequences used are SGR codes for color/bold (doctor.go:451-456). Accepting residual risk that a crafted identity name with embedded ANSI in a terminal context could be cosmetically confusing; no code execution risk exists because ANSI SGR codes carry no executable payload. |
| T-04-14 | DoS (ssh-add hang) | `ssh-add -l` with a stale SSH_AUTH_SOCK returns promptly with exit 2; classifyAgentState maps this to `agentUnreachable` (signing.go:35). No timeout wrapper exists. Accept: the risk is bounded — ssh-add does not block indefinitely on a dead socket in practice; it returns quickly with an error. Adding a timeout wrapper is deferred to a future hardening phase. |
| T-04-24 | DoS (ssh-add hang) | Duplicate of T-04-14; explicitly accepted by the threat register. Same rationale. |

---

## Open Warnings (not BLOCKERs)

### WARNING-01: CI Grep Gate for D-01 Is Manual Only (T-04-03, T-04-21)

**Threats affected:** T-04-03, T-04-21

**What is claimed:** Both threats state "CI grep gate asserts no filewriter import in internal/doctor" as a component of the mitigation.

**What exists:** The isolation property is **currently true** — `grep -rn '"github.com/castocolina/gitid/internal/filewriter"' internal/doctor/` returns empty. However, no automated enforcement exists:

- Makefile: no grep gate target
- .golangci.yml: no depguard or importcheck rule prohibiting filewriter imports in internal/doctor
- .pre-commit-config.yaml: no hook for import isolation
- .github/: does not exist (no CI pipeline)

**Risk:** A future contributor could import filewriter into internal/doctor and the violation would not be caught automatically before merge.

**Recommended remediation (implementation team, not this audit):** Add a `depguard` rule in `.golangci.yml` denying `github.com/castocolina/gitid/internal/filewriter` in `internal/doctor/**`, or add a `make lint-gate` target with a `grep` assertion.

---

## Unregistered Threat Flags (from SUMMARY.md)

### 04-07-SUMMARY.md

| Flag | Description | Mapping | Status |
|------|-------------|---------|--------|
| `threat_flag: cmd-injection-mitigated` | Two new external process invocations (ssh-add, ssh-keygen) use arg-slice form only; no shell expansion possible | Maps to T-04-22 | Verified CLOSED — T-04-22 entry above |

**New direct dependency:** `golang.org/x/term v0.44.0` added in 04-07 (promoted from transitive). Disclosed in 04-07-SUMMARY.md. No new attack surface introduced — the package is used solely for `term.IsTerminal(fd)` in `isTerminalInput`. Mapped to T-04-SC which is CLOSED.

### 04-06-SUMMARY.md

No new threat flags declared. Executor confirmed T-04-16/T-04-17/T-04-19/T-04-21 hold after gap closure.

---

## Phase Verdict

**SECURED with warnings.**

All 21 `mitigate` threats are CLOSED (property holds in code). The 3 `accept` threats are documented. The declared "CI grep gate" for D-01 is absent as automation (WARNING-01) but the isolation property it was meant to enforce is currently true. No BLOCKER conditions exist. Phase 04 may ship; WARNING-01 should be addressed before the next phase that extends `internal/doctor`.
