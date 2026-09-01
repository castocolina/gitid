# Phase 9.2 Global Git Ignore — PTY frame evidence

These are the Phase 09.2-03 fresh approved captures for the Global Git Ignore screen — the
six registered base states and four unregistered error/refusal states, captured at the fixed
100x30 geometry, driven by real PTY sessions with raw keystrokes against the compiled
`gitid` binary (never a unit/wiring-test substitute — ONESHOT.md rule 8).

Promoted by `go run` — never hand-copied. Re-run after any PTY test change; the promotion
overwrites this table and every frame in this directory from a fresh `tmp/ui-frames/` capture.

## Group 1: Registered Base States (automated visual-regression gateway)

| State ID | Frame | Producing test | Group | Source commit | SHA-256 |
|---|---|---|---|---|---|
| gign-after-reset | gign-after-reset.txt | TestGitIgnore_RealPTYEditThenResetDiscardsEdit | registered | 5a41c88d5202b68327f9b8d19c218913caa8ad9e | c864337e6c2a53a0deb47045e60aaeeb9add90b74ec3c7bc0738f36e15d90288 |
| gign-editing | gign-editing.txt | TestGitIgnore_RealPTYShortcutLettersAreTypedNotTriggered | registered | 5a41c88d5202b68327f9b8d19c218913caa8ad9e | 05454f8aafeb5af7b5c5a414632cdbb6fa7501c38f1f8a44593d4f7ac6b835e5 |
| gign-existing-block | gign-existing-block.txt | TestGitIgnore_RealPTYShowsExistingManagedBlock | registered | 5a41c88d5202b68327f9b8d19c218913caa8ad9e | acbc244317c7a9e9a4b6dc25e74706fc1af0fcde6abfbb7a6d71bf9de248e52a |
| gign-receipt | gign-receipt.txt | TestGitIgnore_RealPTYConfirmWritesBlockAndBacksUp | registered | 5a41c88d5202b68327f9b8d19c218913caa8ad9e | 56e53b09a9fdf6f57fcad0153409e8c2125454546db29a4afc4da68970c09802 |
| gign-review-ceremony | gign-review-ceremony.txt | TestGitIgnore_RealPTYCancelWritesNothing | registered | 5a41c88d5202b68327f9b8d19c218913caa8ad9e | da8e896c787149f010c300243ed2a54beff4906660c3f885f237c2944de839d3 |
| gign-seeded-defaults | gign-seeded-defaults.txt | TestGitIgnore_RealPTYSeedsFromDefaultsWhenAbsent | registered | 5a41c88d5202b68327f9b8d19c218913caa8ad9e | 273e18fac7cea4387e1080de914a9d1cf3f8800e396aaa07591e419517a04a0a |

## Group 2: Unregistered Error/Refusal States (evidence only)

| State ID | Frame | Producing test | Group | Source commit | SHA-256 |
|---|---|---|---|---|---|
| gign-error-changed-since-preview | gign-error-changed-since-preview.txt | TestGitIgnore_RealPTYChangedSincePreview | unregistered-error | 5a41c88d5202b68327f9b8d19c218913caa8ad9e | 791f484295030223a8da8230f2d9e6345e91052470b892e51afd0762698c99cb |
| gign-error-malformed-file | gign-error-malformed-file.txt | TestGitIgnore_RealPTYRefusesMalformedFile | unregistered-error | 5a41c88d5202b68327f9b8d19c218913caa8ad9e | 7cea48226e2caf047e07e694058fd2589289b8d5353a7b53a3f498a5931aaf9b |
| gign-error-no-baseline-block | gign-error-no-baseline-block.txt | TestGitIgnore_RealPTYRefusesWhenNoBaselineBlock | unregistered-error | 5a41c88d5202b68327f9b8d19c218913caa8ad9e | 3065de23cb62820b7f0020f7940fa3937d917875aedb6e333c67bff86ddc577a |
| gign-error-sentinel-rejected | gign-error-sentinel-rejected.txt | TestGitIgnore_RealPTYTypedSentinelIsRefused | unregistered-error | 5a41c88d5202b68327f9b8d19c218913caa8ad9e | 88ec50d83807ae5f4366d2ed26fa91863ae1281edae34a1e961a30093ac74a69 |
