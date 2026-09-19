# Phase 9.6 Global Git/SSH options consistency approved PTY frames

These are the Phase 9.6 real-PTY captures for the renamed Other keys
sub-tab, the differs-row glyph, and the browse states whose rendered copy
changed during the consistency fixes. Every frame is captured at the fixed
100x30 geometry, driven by a real PTY session with raw keystrokes against the
compiled `gitid` binary (never a unit/wiring-test substitute — ONESHOT.md rule 8).

Promoted by `go run ./cmd/gitid-frame-promote` — never hand-copied. Re-run that
command after any PTY test change; it overwrites this table and every frame in
this directory from a fresh `tmp/ui-frames/` capture.

## Provenance

| State ID | Frame | Producing test | Shim mode | Geometry | Source commit | SHA-256 |
|---|---|---|---|---|---|---|
| global-git-browse | global-git-browse.txt | TestGlobalGit_RealPTYBrowse | real git, no shim | 100x30 | cb35f84edd84eafd75a0a6fe892aa278d69ab3c0 | 06d1851ae826355a5f526856e06df6c40563f88d1f8273cf77dcd069c0331970 |
| global-git-browse-bottom | global-git-browse-bottom.txt | TestGlobalGit_RealPTYBrowse | real git, no shim | 100x30 | cb35f84edd84eafd75a0a6fe892aa278d69ab3c0 | 52b5b0aa65aebca14700bb6138bfafe54b02ced20314c3d0bb1319f690c6cf27 |
| global-git-differs | global-git-differs.txt | TestGlobalGit_RealPTYDiffersRow | real git, no shim | 100x30 | cb35f84edd84eafd75a0a6fe892aa278d69ab3c0 | b3dd20145fd7e09bd556f2364bf5d0a40581717400b271ef0b2cba8f3d1bc816 |
| global-git-other-keys-back-to-options | global-git-set-keys-back-to-options.txt | TestGlobalGit_RealPTYSetKeysBrowse | real git, no shim | 100x30 | cb35f84edd84eafd75a0a6fe892aa278d69ab3c0 | fa05e3a42c2df07fd4d41b93ee1c917f27d573127fd76e1ad7d0b722efa5c757 |
| global-git-other-keys-browse | global-git-set-keys-browse.txt | TestGlobalGit_RealPTYSetKeysBrowse | real git, no shim | 100x30 | cb35f84edd84eafd75a0a6fe892aa278d69ab3c0 | 66fb027bfca69a543ce1568445351ba6e962e48f6c8e09edb46093c1a3113487 |
| global-git-other-keys-filter-narrowed | global-git-set-keys-filter-narrowed.txt | TestGlobalGit_RealPTYSetKeysFilter | real git, no shim | 100x30 | cb35f84edd84eafd75a0a6fe892aa278d69ab3c0 | 037b638d66634481faf6e6aa61f3ffebfd721ce46da5dd1d429d77570b377a10 |
| global-git-other-keys-probe-failure | global-git-set-keys-probe-failure.txt | TestGlobalGit_RealPTYSetKeysProbeFailure | fake git 2.50.0 (config probe broken) | 100x30 | cb35f84edd84eafd75a0a6fe892aa278d69ab3c0 | d63939ad1f6ca69f0904c0d6a26c7fe4f002432c2f114d20e072a68bb017e799 |
