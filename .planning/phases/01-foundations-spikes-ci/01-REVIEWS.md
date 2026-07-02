---
phase: 1
reviewers: [codex]
reviewed_at: 2026-07-02T17:34:56Z
plans_reviewed: [01-01-PLAN.md, 01-02-PLAN.md, 01-03-PLAN.md, 01-04-PLAN.md, 01-05-PLAN.md, 01-06-PLAN.md, 01-07-PLAN.md]
reviewer_model: codex (codex-cli 0.142.5, default model)
---

# Cross-AI Plan Review — Phase 1: Foundations, Spikes & CI

> Single external reviewer this run: **Codex**. Independent adversarial review; it re-verified GitHub hosted-runner labels, `freeze` v0.2.2, and `go-rod` v0.116.2 against live sources. Overall verdict: **MEDIUM-HIGH risk — approve after tightening.**

## Codex Review

## Summary

The plans are unusually thorough and mostly well aligned with Phase 1’s “non-UI foundations, test-proven” goal. The strongest parts are the TDD framing, explicit real `ssh -G` checks, reserved-block guard, dependency graph isolation for screenshot tooling, and corrected CI runner labels. That said, I would not approve these unchanged: several plans over-specify implementation before proving repo compatibility, some wave-parallelism assumptions are unsafe, and the riskiest area, SSH Include adoption/migration, is underdefined for real OpenSSH behavior, path expansion, duplicate/ordered Include directives, and rollback semantics. CI and screenshot tooling also carry more reproducibility risk than the plans admit. I verified current GitHub runner docs: `macos-15-intel` and `macos-15` are valid labels today, with `macos-latest` also listed for arm64 macOS runners; GitHub explicitly warns `*-latest` may not be the newest OS, so the pinned `macos-15` choice is defensible. Sources: GitHub runner docs, freeze releases, and pkg.go.dev for rod.  

## Strengths

- The phase decomposition is coherent: platform probes, keygen registry, SSH storage, identity taxonomy, screenshot tooling, debug surface, then CI.

- The plans repeatedly enforce the right invariant for this product: no mutation without backup, idempotent managed blocks, parse validation, and real `ssh -G` proof.

- The FIDO2 token correction is important and correctly called out. Mapping human names like `ed25519-sk` to real OpenSSH tokens like `sk-ssh-ed25519@openssh.com` avoids a likely silent availability bug.

- The `internal/adopter` trap is recognized. Keeping SSH Include adoption in `internal/sshconfig` avoids contaminating SSH config behavior with gitconfig `includeIf` assumptions.

- The reserved `ssh-include` block guard is correctly paired with STORE-01 instead of deferred. That is exactly the kind of bug that otherwise creates doctor/fixer loops.

- The screenshot plan correctly distinguishes make/CI-callable tooling from agent-only browser tooling.

- CI runner research is materially improved versus the stale context. GitHub’s hosted-runner docs currently list `macos-15-intel` for Intel macOS and `macos-15` for arm64 macOS runners, so the plan’s labels are plausible and current. GitHub also notes `*-latest` labels are not guaranteed to mean newest OS, supporting a pinned label strategy.

## Concerns

- **HIGH: 01-03 migration rollback is not specified strongly enough.**  
  Task 3 says “failure mid-way does not corrupt the source” because `filewriter.Write` is atomic per file. That is not enough for a two-file migration touching `~/.ssh/config` and `~/.ssh/config.d/gitid.config`. Atomicity per file does not give transactionality across files. A failure after writing one file but before writing the other can leave duplicated blocks, missing blocks, or an Include line pointing at stale content.

- **HIGH: 01-03 Include adoption semantics are underspecified.**  
  “Detect existing external Include directive and adopt its path” is too vague for real configs. OpenSSH supports multiple `Include` directives, globs, quoted paths, absolute paths, `~` paths, and ordering-sensitive first-match behavior. The plan does not define how to choose among multiple Includes, how to handle globs resolving to many files, how to avoid adopting a broad user-managed glob accidentally, or how to preserve first-match semantics when moving blocks.

- **HIGH: 01-04 state taxonomy may be conceptually overloaded.**  
  MGR-02 mixes identity states (`complete`, `incomplete`, `git-only`, `fragment-path-missing`) with key states (`key-unused`, `key-used-ssh-only`, `key-used-both`, `key-missing`). Forcing `ClassifyState(acct, keyExists, keyUsedInSSH, keyUsedInGit) State` to return exactly one state per `Account` may collapse distinct facts. Example: an identity can be `fragment-path-missing` and reference a missing key. A key can be unused without there being an identity account to classify. This needs a richer model or clear precedence rules.

- **HIGH: 01-06 depends on 01-04, but 01-04 may not produce enough data for real identity state output.**  
  The debug command is expected to print each identity’s computed state, but 01-04 only defines a pure classifier taking injected booleans. No plan owns the real aggregation layer that gathers key existence, SSH host usage, Git signing usage, fragment existence, and unused key inventory from parsed config. Without that, 01-06 risks either faking states or rebuilding nontrivial logic in `cmd/gitid`.

- **MEDIUM: Wave 1 has hidden ordering conflicts despite `depends_on: []`.**  
  01-01 and 01-02 are declared parallel, but 01-02’s catalog availability uses probe token concepts from 01-01. It avoids importing `platform`, but the naming/types still need coordination. 01-05 and 01-07 both modify `Makefile`, and 01-07 depends on 01-05, which is good, but 01-05 itself is marked Wave 1 while being `autonomous: false`; that can block the whole wave and delay Wave 2.

- **MEDIUM: 01-05 introduces a second human checkpoint in a phase whose product process says one checkpoint.**  
  The supply-chain legitimacy gate is reasonable, but it conflicts with the stated “single human checkpoint” ethos. If this is allowed as an engineering security exception, say that explicitly. Otherwise, replace it with a documented pinned-dependency review and automated checksum/provenance checks.

- **MEDIUM: screenshot determinism is still weaker than the plan claims.**  
  A vendored font helps, but OS font rasterization, DPI, antialiasing, terminal theme defaults, freeze version, and image metadata can still differ. The plan says “deterministic” but only verifies “non-empty PNG.” That is not enough groundwork for later visual regression.

- **MEDIUM: go-rod/Chromium reproducibility is under-proven.**  
  `go-rod/rod@v0.116.2` exists, but pkg.go.dev marks it “not latest” and published in 2024. That is not disqualifying, but it makes “current maintained choice” weaker. Auto-downloaded Chromium in CI is also a supply-chain and flakiness risk unless the exact browser revision, cache path, and offline/failure behavior are pinned and tested.

- **MEDIUM: 01-07 runs full e2e and screenshot/browser provisioning on all macOS runners without cost/time fallback.**  
  This is quality-positive but likely slow and brittle. It may also hit macOS arm64 limitations or third-party action compatibility issues; GitHub documents arm64 macOS runner limitations for some capabilities. The plan should define what is required on every PR versus scheduled/main.

- **MEDIUM: 01-01 `ProbeSSHVersion() (string, error)` conflicts with task behavior.**  
  The artifact says `ProbeSSHVersion() (string, error)`, but behavior expects parsed OpenSSH version + SSL flavor. Returning only `string` invites either lossy formatting or duplicated parsing downstream. Make it return a struct.

- **MEDIUM: `ssh-add -l` probing can hang or behave inconsistently.**  
  Agent probing should set a timeout and treat common exit codes distinctly: no agent, no identities, agent locked/unavailable. “HasSSHAgent bool” may be too coarse for troubleshooting.

- **MEDIUM: libfido2 detection is probably too platform-specific to be one bool.**  
  `ssh -Q key` may list `sk-*` support even when runtime middleware is unusable. Presence of `libfido2`, `ssh-sk-helper`, Homebrew dylibs, and OpenSSH provider path are different facts. A single `HasLibfido2` can mislead KEY-03 hints.

- **LOW: plan 01-02 stubs for future algorithms can leak into UX semantics.**  
  “Registered but not implemented” is useful internally, but be careful not to let registry presence imply generation support. Tests should assert creation paths cannot select unimplemented algorithms as if available.

- **LOW: 01-06 private-key leakage test is too narrow.**  
  Asserting absence of `"PRIVATE KEY"` catches obvious leakage, not all sensitive paths. It should also avoid printing passphrase fields, private-key paths in unintended contexts, and full environment dumps.

## Suggestions

- Add a formal migration transaction model to 01-03: preflight, write target temp, validate target parse and `ssh -G`, write source temp, validate final state, then commit. Return a recovery plan and backups for both files. Add tests for injected failure after each write step.

- Define Include adoption selection rules before implementation:
  - adopt only a path containing gitid sentinels, or only after explicit caller choice;
  - reject broad globs unless the target file can be unambiguously selected;
  - preserve Include order;
  - handle multiple Include directives deterministically;
  - expand `~/.ssh` and absolute paths consistently;
  - never adopt relative paths outside the verified `~/.ssh`-relative rule.

- Replace `ClassifyState(...) State` with a report model:
  `IdentityHealth{IdentityState, KeyState, Problems []Problem}` or similar. Keep the 8 labels, but do not force mutually exclusive facts unless MGR-02 truly means that. At minimum, define precedence rules and add tests for overlapping failures.

- Add a 01-04 or 01-06 task for the real “state inventory builder”: parse managed SSH blocks, parse git includes/fragments, stat keys/fragments, compute key usage in SSH and signing, detect unused keys, then call the pure classifier/report builder.

- Change 01-01 APIs to structured results:
  `SSHVersion{OpenSSHVersion, SSLFlavor, SSLVersion, Raw string}` and `Capabilities{KeyTypes []string, Algorithms []string, Agent AgentStatus, FIDO FIDOStatus, Keychain KeychainStatus}`. Booleans will be too weak for doctor/fixer copy later.

- Add timeouts to all external probe commands using `exec.CommandContext`. This matters for `ssh-add`, `ssh-keygen`, and browser tooling.

- In 01-05, require deterministic screenshot metadata checks beyond non-empty PNG: fixed viewport, fixed device scale factor, fixed color scheme, fixed font, no timestamp metadata, and a golden hash or perceptual-diff baseline for the trivial fixture.

- Consider isolating screenshot tooling under `tools/screenshot` with its own `go.mod`. Build tags keep it out of the binary, but the main module’s `go.sum` and dependency audit still become noisier.

- For CI, split jobs:
  - fast PR gate: `setup-env`, `test`, `lint`, maybe e2e on Ubuntu + one macOS;
  - full cross-runner gate: push/main or required pre-release branch;
  - build-cross can run once on Linux unless native behavior is being tested.
  If the product requires all three on every PR, document the cost as intentional.

- Pin GitHub Actions by SHA, not only version tags, if the security posture is serious. The plan says “explicit tags”; tags are mutable in principle.

- Add permissions blocks to `.github/workflows/ci.yml`, e.g. `contents: read`, and avoid secrets entirely in Phase 1.

- Add tests for permission bits on created `~/.ssh/config.d` directory and Include file, not just key files. STORE-01 needs directory mode and config file mode proven.

## Risk Assessment

Overall risk: **MEDIUM-HIGH**.

The architectural direction is sound, but the highest-risk part of the product is safe mutation of SSH/Git artifacts, and the STORE plan still treats multi-file migration too casually. Screenshot and CI tooling are also likely to expose flakiness once run on hosted macOS and Linux. The plans can probably achieve the five Phase 1 success criteria after tightening, but as written some criteria are only nominally satisfied: identity taxonomy lacks a real inventory builder, screenshot determinism is asserted more than proven, and migration reversibility is tested but not transactionally designed.

Sources checked: GitHub’s hosted runner reference currently lists `ubuntu-latest`, `macos-15-intel`, and `macos-15` labels, and warns that `*-latest` labels are not necessarily the newest OS; freeze v0.2.2 exists in Charmbracelet releases; pkg.go.dev lists `github.com/go-rod/rod` v0.116.2 and its repository.

---

## Consensus Summary

Only one external reviewer (Codex) ran this pass, so "consensus" reflects Codex's
prioritization plus where it agrees/disagrees with the internal gsd-plan-checker
(which returned VERIFICATION PASSED, 10/10). The two are complementary: the checker
verified structural completeness and requirement coverage; Codex probed *semantic
depth* and found the plans under-specify the hardest runtime behaviors.

### Agreed Strengths (checker + Codex)
- Coherent decomposition (probe → keygen → storage → taxonomy → screenshot → debug → CI).
- Correct safety invariant everywhere (backup + idempotent managed block + real `ssh -G` proof).
- FIDO2 token correction (`sk-ssh-ed25519@openssh.com`), `internal/adopter` name-trap avoided,
  reserved-block guard paired with STORE-01, corrected `macos-15` CI labels (Codex independently
  confirmed these labels are current and that pinning over `*-latest` is defensible).

### Agreed Concerns (highest priority — HIGH from Codex, not caught by the structural checker)
1. **[HIGH] 01-03 migration is not transactional across files.** Per-file `filewriter.Write`
   atomicity ≠ cross-file transactionality for a 2-file (`~/.ssh/config` + `config.d/gitid.config`)
   migration. A failure between the two writes can duplicate/lose blocks or leave a stale Include.
   → Add a preflight → temp-write → validate (parse + `ssh -G`) → commit → recovery-plan model,
   with injected-failure tests after each write step.
2. **[HIGH] 01-03 Include adoption semantics underspecified.** Real OpenSSH allows multiple
   `Include` directives, globs, quoted/`~`/absolute paths, ordering-sensitive first-match. The
   plan doesn't define selection among multiple Includes, glob handling, or how to avoid adopting
   a broad user glob. → Define adoption selection rules (adopt only paths with gitid sentinels or
   explicit caller choice; reject broad globs; preserve Include order) before implementation.
3. **[HIGH] 01-04 taxonomy conflates identity-states and key-states into one enum,** forcing a
   single `State` return that can collapse distinct facts (e.g. `fragment-path-missing` AND
   `key-missing` simultaneously). → Return a richer `IdentityHealth{IdentityState, KeyState,
   Problems[]}` (keep the 8 labels) or define explicit precedence + overlap tests.
4. **[HIGH] No plan owns the real state-inventory builder.** 01-04 is a *pure classifier* taking
   injected booleans; 01-06 (debug command) is expected to print real per-identity state but
   nothing gathers key existence / SSH host usage / signing usage / fragment existence / unused-key
   inventory from parsed config. → Add an explicit inventory-builder task (in 01-04 or 01-06) so
   01-06 doesn't fake states or rebuild logic in `cmd/gitid`.

### Notable MEDIUM concerns worth folding in
- **Wave hazard:** 01-05 is `autonomous: false` but sits in Wave 1 — its human gate can stall the
  whole wave and delay Wave 2. Consider moving 01-05 to its own wave or making the gate async.
- **Probe API shape:** `ProbeSSHVersion() (string, error)` and single-bool `HasLibfido2`/`HasSSHAgent`
  are too lossy for later doctor/fixer copy — return structs (`SSHVersion{…}`, `Capabilities{…}`,
  `AgentStatus`, `FIDOStatus`). Add `exec.CommandContext` timeouts to all probes (`ssh-add -l` can hang).
- **Screenshot determinism** is asserted (non-empty PNG) but not proven — pin viewport, device scale,
  color scheme, font, strip timestamp metadata, add a golden hash for the fixture (needed for the
  future visual-regression gate).
- **Second human checkpoint:** 01-05's supply-chain gate is a *second* stop in a "one-checkpoint"
  milestone — either justify it explicitly as a security exception or replace with automated
  checksum/provenance + pinned-version review.
- **CI cost/shape:** split fast PR gate (Ubuntu + one macOS) from full cross-runner gate (push/main);
  pin actions by SHA; add a least-privilege `permissions:` block to ci.yml.
- Prove permission bits on the new `~/.ssh/config.d` **directory** + Include file, not just keys.

### Divergent Views
- Codex is more skeptical than the internal checker on **go-rod** ("not latest", 2024) and on
  screenshot/CI reproducibility — the checker treated tooling choices as settled; Codex wants the
  Chromium revision + cache/offline behavior pinned and tested. Worth a Phase-1 spike to de-risk.
- On CI runners the two AGREE (macos-15 labels correct) — Codex independently corroborated the
  research correction, strengthening confidence in D-12.
