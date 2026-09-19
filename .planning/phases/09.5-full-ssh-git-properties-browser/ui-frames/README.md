# Phase 9.5 Global SSH/Git properties-browser approved PTY frames

These are PROP-01..04's approved captures for the "All directives" browser,
the custom-directive entry flow, the "Other keys" browser (with its net-new
sub-tab strip), and the custom Git key entry flow. Every frame is captured at the fixed
100x30 geometry, driven by a real PTY session with raw keystrokes against the
compiled `gitid` binary (never a unit/wiring-test substitute — ONESHOT.md rule 8).

Promoted by `go run ./cmd/gitid-frame-promote` — never hand-copied. Re-run that
command after any PTY test change; it overwrites this table and every frame in
this directory from a fresh `tmp/ui-frames/` capture.

## Provenance

| State ID | Frame | Producing test | Shim mode | Geometry | Source commit | SHA-256 |
|---|---|---|---|---|---|---|
| global-git-custom-key-malformed | global-git-custom-key-malformed.txt | TestGlobalGit_RealPTYCustomKeyRejectsMalformedKey | real git, no shim | 100x30 | cb35f84edd84eafd75a0a6fe892aa278d69ab3c0 | 6496e6dc03c4cd9d8097188ab90fe9bfbbef203a1f38cd6ae1e0393e3c47af01 |
| global-git-custom-key-write | global-git-custom-key-write.txt | TestGlobalGit_RealPTYCustomKeyWrite | real git, no shim | 100x30 | cb35f84edd84eafd75a0a6fe892aa278d69ab3c0 | 58310dc2f5ae5e4e5e2b2533d8a2563d301d66fc33d5ac08522903dff8d80f28 |
| global-git-set-keys-back-to-options | global-git-set-keys-back-to-options.txt | TestGlobalGit_RealPTYSetKeysBrowse | real git, no shim | 100x30 | cb35f84edd84eafd75a0a6fe892aa278d69ab3c0 | fa05e3a42c2df07fd4d41b93ee1c917f27d573127fd76e1ad7d0b722efa5c757 |
| global-git-set-keys-browse | global-git-set-keys-browse.txt | TestGlobalGit_RealPTYSetKeysBrowse | real git, no shim | 100x30 | cb35f84edd84eafd75a0a6fe892aa278d69ab3c0 | 66fb027bfca69a543ce1568445351ba6e962e48f6c8e09edb46093c1a3113487 |
| global-git-set-keys-filter-digit-captured | global-git-set-keys-filter-digit-captured.txt | TestGlobalGit_RealPTYSetKeysFilter | real git, no shim | 100x30 | cb35f84edd84eafd75a0a6fe892aa278d69ab3c0 | 8138aef504fd92e9a9cde513d3f454c9a396d401ef7f57e4a4d324f66359b5ab |
| global-git-set-keys-filter-narrowed | global-git-set-keys-filter-narrowed.txt | TestGlobalGit_RealPTYSetKeysFilter | real git, no shim | 100x30 | cb35f84edd84eafd75a0a6fe892aa278d69ab3c0 | 037b638d66634481faf6e6aa61f3ffebfd721ce46da5dd1d429d77570b377a10 |
| global-git-set-keys-probe-failure | global-git-set-keys-probe-failure.txt | TestGlobalGit_RealPTYSetKeysProbeFailure | fake git 2.50.0 (config probe broken) | 100x30 | cb35f84edd84eafd75a0a6fe892aa278d69ab3c0 | d63939ad1f6ca69f0904c0d6a26c7fe4f002432c2f114d20e072a68bb017e799 |
| global-git-strip-click-to-options | global-git-strip-click-to-options.txt | TestGlobalGit_RealPTYSubTabStripClick | real git, no shim | 100x30 | cb35f84edd84eafd75a0a6fe892aa278d69ab3c0 | fa05e3a42c2df07fd4d41b93ee1c917f27d573127fd76e1ad7d0b722efa5c757 |
| global-git-strip-click-to-set-keys | global-git-strip-click-to-set-keys.txt | TestGlobalGit_RealPTYSubTabStripClick | real git, no shim | 100x30 | cb35f84edd84eafd75a0a6fe892aa278d69ab3c0 | 50344b908fc8d3a31853381c91974f634535c40dcee1f01949e7fc58b014bbe6 |
| global-ssh-all-directives-browse | global-ssh-all-directives-browse.txt | TestGlobalSSH_RealPTYAllDirectivesBrowse | fake ssh: globalssh | 100x30 | cb35f84edd84eafd75a0a6fe892aa278d69ab3c0 | 5b18b4c6f6989a00714cd56b9df110f755676436e7ae7f335c7ca7ea2e9360de |
| global-ssh-all-directives-filter-digit-captured | global-ssh-all-directives-filter-digit-captured.txt | TestGlobalSSH_RealPTYAllDirectivesFilter | fake ssh: globalssh | 100x30 | cb35f84edd84eafd75a0a6fe892aa278d69ab3c0 | 3caf6582461aab45640b4b30bdc66fac0f82062183f93db690c2dd3c1604148c |
| global-ssh-all-directives-filter-narrowed | global-ssh-all-directives-filter-narrowed.txt | TestGlobalSSH_RealPTYAllDirectivesFilter | fake ssh: globalssh | 100x30 | cb35f84edd84eafd75a0a6fe892aa278d69ab3c0 | c3ab6910ca5c25bd3b423b7de24c8e6e540ad29f3231c13cf1a5a990194bc7c4 |
| global-ssh-all-directives-label-click | global-ssh-all-directives-label-click.txt | TestGlobalSSH_RealPTYAllDirectivesLabelMouseClick | fake ssh: globalssh | 100x30 | cb35f84edd84eafd75a0a6fe892aa278d69ab3c0 | 68fb6ce290ee9ca8252053a3e8e3574a42e44c66750ff88312a5654ed2290277 |
| global-ssh-all-directives-probe-failure | global-ssh-all-directives-probe-failure.txt | TestGlobalSSH_RealPTYAllDirectivesProbeFailure | fake ssh: globalssh-probe-unresolvable | 100x30 | cb35f84edd84eafd75a0a6fe892aa278d69ab3c0 | e775bd4eff93cbca7f2003ddc47c36ba9fbd88e0bf7f1db7ed0c31a0b98d8b3e |
| global-ssh-custom-directive-ceremony | global-ssh-custom-directive-ceremony.txt | TestGlobalSSH_RealPTYCustomDirectiveWrite | fake ssh: globalssh | 100x30 | cb35f84edd84eafd75a0a6fe892aa278d69ab3c0 | 00d76f0539054ecd3785d21dcefe8f7afdcc80ff0eaa06e9537b4facaac84dac |
| global-ssh-custom-directive-rejected-name | global-ssh-custom-directive-rejected-name.txt | TestGlobalSSH_RealPTYCustomDirectiveRejectedNameNeverWrites | fake ssh: globalssh | 100x30 | cb35f84edd84eafd75a0a6fe892aa278d69ab3c0 | 7c717a133ea020bf7288c7ca4706188d1ce2de3b0b321badde3116b9babbf757 |
| global-ssh-custom-directive-write-receipt | global-ssh-custom-directive-write-receipt.txt | TestGlobalSSH_RealPTYCustomDirectiveWrite | fake ssh: globalssh | 100x30 | cb35f84edd84eafd75a0a6fe892aa278d69ab3c0 | d9f9ab60b99cf4b7665aa0a2a9d4caa7e1b40132b660e6b221694d2eaa20ac96 |
