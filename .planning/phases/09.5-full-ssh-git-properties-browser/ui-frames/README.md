# Phase 9.5 Global SSH/Git properties-browser approved PTY frames

These are PROP-01..04's approved captures for the "All directives" browser,
the custom-directive entry flow, the "Set keys" browser (with its net-new
sub-tab strip), and the custom Git key entry flow. Every frame is captured at the fixed
100x30 geometry, driven by a real PTY session with raw keystrokes against the
compiled `gitid` binary (never a unit/wiring-test substitute — ONESHOT.md rule 8).

Promoted by `go run ./cmd/gitid-frame-promote` — never hand-copied. Re-run that
command after any PTY test change; it overwrites this table and every frame in
this directory from a fresh `tmp/ui-frames/` capture.

## Provenance

| State ID | Frame | Producing test | Shim mode | Geometry | Source commit | SHA-256 |
|---|---|---|---|---|---|---|
| global-git-custom-key-malformed | global-git-custom-key-malformed.txt | TestGlobalGit_RealPTYCustomKeyRejectsMalformedKey | real git, no shim | 100x30 | 3fc9ec3e29819394f137acccec5450e857323131 | 23915cc4cb6ca59a07a45af6bd9ca4f14e2a5aae0372e4716525459801033f06 |
| global-git-custom-key-write | global-git-custom-key-write.txt | TestGlobalGit_RealPTYCustomKeyWrite | real git, no shim | 100x30 | 3fc9ec3e29819394f137acccec5450e857323131 | 753cb38ba9ac3e8eefc5a87cb27d9f658613efc6cc6c0e45beedb99c8d3b7f1b |
| global-git-set-keys-back-to-options | global-git-set-keys-back-to-options.txt | TestGlobalGit_RealPTYSetKeysBrowse | real git, no shim | 100x30 | 3fc9ec3e29819394f137acccec5450e857323131 | deba8cf183faac44ea02c23ce71ed77bd0a41fe2b0d4b927ca952af43d3b23be |
| global-git-set-keys-browse | global-git-set-keys-browse.txt | TestGlobalGit_RealPTYSetKeysBrowse | real git, no shim | 100x30 | 3fc9ec3e29819394f137acccec5450e857323131 | dbf055d2e02adc5c0d3b162b47ec826f2bf733417aab17a7f039bdebbfc91aae |
| global-git-set-keys-filter-digit-captured | global-git-set-keys-filter-digit-captured.txt | TestGlobalGit_RealPTYSetKeysFilter | real git, no shim | 100x30 | 3fc9ec3e29819394f137acccec5450e857323131 | ac1716f9f81e8a06e8e69de419e3f03bf3d24fe7cda7c47b4f7ccd7a2f732fe3 |
| global-git-set-keys-filter-narrowed | global-git-set-keys-filter-narrowed.txt | TestGlobalGit_RealPTYSetKeysFilter | real git, no shim | 100x30 | 3fc9ec3e29819394f137acccec5450e857323131 | 23d8889a37e15ea201bfd5bb82a0c4fcc5b36ff0b6f7489589b02c494fadd7e5 |
| global-git-set-keys-probe-failure | global-git-set-keys-probe-failure.txt | TestGlobalGit_RealPTYSetKeysProbeFailure | fake git 2.50.0 (config probe broken) | 100x30 | 3fc9ec3e29819394f137acccec5450e857323131 | d3ddb0ce8e1d5dc416d5bd2c5eab529c6d20e85bd9634747b84330ad8fd86ed9 |
| global-git-strip-click-to-options | global-git-strip-click-to-options.txt | TestGlobalGit_RealPTYSubTabStripClick | real git, no shim | 100x30 | 3fc9ec3e29819394f137acccec5450e857323131 | 8930c48271ce04f8160520e8f01f25e791d2e9be2ec470cf36dcbeb84e9b73da |
| global-git-strip-click-to-set-keys | global-git-strip-click-to-set-keys.txt | TestGlobalGit_RealPTYSubTabStripClick | real git, no shim | 100x30 | 3fc9ec3e29819394f137acccec5450e857323131 | f165d6ffd2b90c25da4fbec9e419e73c016c24fb076fb1a1d4e1089c7b3f5887 |
| global-ssh-all-directives-browse | global-ssh-all-directives-browse.txt | TestGlobalSSH_RealPTYAllDirectivesBrowse | fake ssh: globalssh | 100x30 | 3fc9ec3e29819394f137acccec5450e857323131 | 33fdf67bf630c8e2f8839442c1f56fa08a2b112cc55b2366bc20e2143d85ace2 |
| global-ssh-all-directives-filter-digit-captured | global-ssh-all-directives-filter-digit-captured.txt | TestGlobalSSH_RealPTYAllDirectivesFilter | fake ssh: globalssh | 100x30 | 3fc9ec3e29819394f137acccec5450e857323131 | 65613ec542b5050cae7c95c098de26bf346733f9141235650a4eee17563e1e19 |
| global-ssh-all-directives-filter-narrowed | global-ssh-all-directives-filter-narrowed.txt | TestGlobalSSH_RealPTYAllDirectivesFilter | fake ssh: globalssh | 100x30 | 3fc9ec3e29819394f137acccec5450e857323131 | 3e8d76a3698e4315645fe0d6d9f1b407aecf1b38f797f65c8d8f0897147fd509 |
| global-ssh-all-directives-label-click | global-ssh-all-directives-label-click.txt | TestGlobalSSH_RealPTYAllDirectivesLabelMouseClick | fake ssh: globalssh | 100x30 | 3fc9ec3e29819394f137acccec5450e857323131 | 33fdf67bf630c8e2f8839442c1f56fa08a2b112cc55b2366bc20e2143d85ace2 |
| global-ssh-all-directives-probe-failure | global-ssh-all-directives-probe-failure.txt | TestGlobalSSH_RealPTYAllDirectivesProbeFailure | fake ssh: globalssh-probe-unresolvable | 100x30 | 3fc9ec3e29819394f137acccec5450e857323131 | f59d3dc6462cb23e12147d75d47176b357175f66a7836b0d2fd935198106ca9a |
| global-ssh-custom-directive-ceremony | global-ssh-custom-directive-ceremony.txt | TestGlobalSSH_RealPTYCustomDirectiveWrite | fake ssh: globalssh | 100x30 | 3fc9ec3e29819394f137acccec5450e857323131 | cd14fd03f935d70102be3868b6bda72b9e88592503a48dbc4ed9922fe29dddf6 |
| global-ssh-custom-directive-rejected-name | global-ssh-custom-directive-rejected-name.txt | TestGlobalSSH_RealPTYCustomDirectiveRejectedNameNeverWrites | fake ssh: globalssh | 100x30 | 3fc9ec3e29819394f137acccec5450e857323131 | f58351c162b2e578850f74f7c36f0287d2c3c15fd6771485195fb3b065cdd6ea |
| global-ssh-custom-directive-write-receipt | global-ssh-custom-directive-write-receipt.txt | TestGlobalSSH_RealPTYCustomDirectiveWrite | fake ssh: globalssh | 100x30 | 3fc9ec3e29819394f137acccec5450e857323131 | 03dcde1d79802a6e23ebc238d001cb06b8d4893fb26fee1120305227531d160f |
