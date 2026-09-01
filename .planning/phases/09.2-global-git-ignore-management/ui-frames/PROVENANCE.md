# Phase 9.2 Global Git Ignore — PTY frame evidence

These are the Phase 09.2-03 fresh approved captures for the Global Git Ignore screen — the
six registered base states and four unregistered error/refusal states, captured at the fixed
100x30 geometry, driven by real PTY sessions with raw keystrokes against the compiled
`gitid` binary (never a unit/wiring-test substitute — ONESHOT.md rule 8).

Re-promoted after the 09.2-UI-REVIEW.md fix pass (breadcrumb duplication removed, frozen
malformed-file/sentinel-rejection copy routed through `internal/tuikit/design.go` constants,
sentinel-rejection glyph added) — every one of the ten frames changed at least in its top
breadcrumb rows, so every hash below was recomputed from a fresh `tmp/ui-frames/` capture.

## Group 1: Registered Base States (automated visual-regression gateway)

| State ID | Frame | Producing test | Group | Source commit | SHA-256 |
|---|---|---|---|---|---|
| gign-after-reset | gign-after-reset.txt | TestGitIgnore_RealPTYEditThenResetDiscardsEdit | registered | 8cbd0beed1765a61bbbb4e5128d9ebcabf0b25a5 | d9cb425504a7bbc0d27617d69f52e30076f13c39a04b373dab6a9521682957c4 |
| gign-editing | gign-editing.txt | TestGitIgnore_RealPTYShortcutLettersAreTypedNotTriggered | registered | 8cbd0beed1765a61bbbb4e5128d9ebcabf0b25a5 | a7077fb64388ec6a06ed07e79a9f1db2cad3b62ee32393c815d8948a0c4d9e4f |
| gign-existing-block | gign-existing-block.txt | TestGitIgnore_RealPTYShowsExistingManagedBlock | registered | 8cbd0beed1765a61bbbb4e5128d9ebcabf0b25a5 | fb918715159d3112f34221340a5d8f3aaf95222dbbb13588b4ab95d9cffa27b7 |
| gign-receipt | gign-receipt.txt | TestGitIgnore_RealPTYConfirmWritesBlockAndBacksUp | registered | 8cbd0beed1765a61bbbb4e5128d9ebcabf0b25a5 | 418c5e4ecc2d59c234df001e01a1d59858e7c36c83431dfdad0d4804bbed11a1 |
| gign-review-ceremony | gign-review-ceremony.txt | TestGitIgnore_RealPTYCancelWritesNothing | registered | 8cbd0beed1765a61bbbb4e5128d9ebcabf0b25a5 | 9de5e5366f1494fe33f00a3f200550ae8522b55e11ab662f72284eaae6b217db |
| gign-seeded-defaults | gign-seeded-defaults.txt | TestGitIgnore_RealPTYSeedsFromDefaultsWhenAbsent | registered | 8cbd0beed1765a61bbbb4e5128d9ebcabf0b25a5 | 2bc89d46d9c3affdf9f268b06dff81821c8d5e267013a2af89513dc18a0c8912 |

## Group 2: Unregistered Error/Refusal States (evidence only)

| State ID | Frame | Producing test | Group | Source commit | SHA-256 |
|---|---|---|---|---|---|
| gign-error-changed-since-preview | gign-error-changed-since-preview.txt | TestGitIgnore_RealPTYChangedSincePreview | unregistered-error | 8cbd0beed1765a61bbbb4e5128d9ebcabf0b25a5 | 1015e8f46c3addfb098c7fcadda41a45465e13774c2a001a0c74dc5e01d723c6 |
| gign-error-malformed-file | gign-error-malformed-file.txt | TestGitIgnore_RealPTYRefusesMalformedFile | unregistered-error | 8cbd0beed1765a61bbbb4e5128d9ebcabf0b25a5 | 2842f57e7f5be07eeb4718ee3f4b576591d185b78717f3f2b0ed800fb9185a67 |
| gign-error-no-baseline-block | gign-error-no-baseline-block.txt | TestGitIgnore_RealPTYRefusesWhenNoBaselineBlock | unregistered-error | 8cbd0beed1765a61bbbb4e5128d9ebcabf0b25a5 | 3dbd891bb30215d0838333971de38c4cf4c5a4dd71f55d900169fc0e293d8b8e |
| gign-error-sentinel-rejected | gign-error-sentinel-rejected.txt | TestGitIgnore_RealPTYTypedSentinelIsRefused | unregistered-error | 8cbd0beed1765a61bbbb4e5128d9ebcabf0b25a5 | 44f1954204d844c45bccf52dd3bae26cdbba515cb2782862f15e7c7bffe07dd0 |
