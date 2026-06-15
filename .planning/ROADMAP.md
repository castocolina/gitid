**Plans**: 5 plans + 2 gap-closure plans
**UI hint**: yes

Plans:

**Wave 1**

- [x] 04-01-PLAN.md — Foundation slice: Finding/Severity/Family model + doctor.Deps + Run/ExitCode + Permissions family + minimal `gitid doctor` grouped renderer end-to-end (DOC-02, DOC-06, DOC-07)

**Wave 2** *(blocked on Wave 1 — each adds only its own checks/*.go, no shared-file edits)*

- [x] 04-02-PLAN.md — Dependencies + Baseline families: deps.Detect compose + extended platform.InstallHint (git/clipboard), ReadBaselineState fold-in (DOC-01, D-16)
- [x] 04-03-PLAN.md — Coherence + Orphans families: existence/resolution + locked-value carve-outs; block-vs-disk orphans + unused-key (DOC-03, DOC-04)
- [x] 04-04-PLAN.md — Signing + Agent families: ssh-add probe + fingerprint match + git<2.36 hasconfig: gate (DOC-05)

**Wave 3** *(blocked on Waves 1-2 — shares cmd/gitid/doctor.go)*

- [x] 04-05-PLAN.md — Auto-fix slice: D-04 gate/per-finding-confirm/--yes flow + permission batching; fixes routed through filewriter chokepoint (DOC-06)

**Gap closure** *(verification gaps_found 2026-06-12 — DOC-GAP-01/02/03; run `/gsd-execute-phase 04 --gaps-only`)*

- [x] 04-06-PLAN.md — DOC-GAP-01: plumb real RemoveBlock/AddWiring through check Fix.Fn (orphan/coherence/baseline auto-fix was a silent no-op) + WR-02 path-aware mode + WR-01 all-candidate signer scan; real-wiring integration tests (DOC-04, DOC-06)
- [x] 04-07-PLAN.md — DOC-GAP-02 + DOC-GAP-03: wire RunSSHAdd/RunSSHKeygenFingerprint (dead Agent check) + TTY-guard the fix gate + IN-03 tiered exit-code propagation + WR-03 gitconfig 0644 perms (DOC-05, DOC-06)
