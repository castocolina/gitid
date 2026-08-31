# Phase 9 upload-surface approved PTY frames

These are D-09's fresh approved captures for exactly the amended upload
screens — the Phase 9 visual-regression baseline every gate-visual-regression
run compares the real binary against. Every frame is captured at the fixed
100x30 geometry, driven by a real PTY session with raw keystrokes against the
compiled `gitid` binary (never a unit/wiring-test substitute — ONESHOT.md rule 8).

Promoted by `go run ./cmd/gitid-frame-promote` — never hand-copied. Re-run that
command after any PTY test change; it overwrites this table and every frame in
this directory from a fresh `tmp/ui-frames/` capture.

## Provenance

| State ID | Frame | Producing test | Shim mode | Geometry | Source commit | SHA-256 |
|---|---|---|---|---|---|---|
| register-key-modal | identity-manager-register-key-modal-runs.txt | TestIdentityManager_RegisterKeyModalRuns | gh ok | 100x30 | d7ad2eeb61e364b7e624712dfc72f5cf6b8b1c4b | ee58bdbfca188d76deb5d38e257715cdf8ff26c6500aeec953546643ec380ba9 |
| register-key-modal-manual-fallback | identity-manager-register-key-modal-manual-fallback.txt | TestIdentityManager_RegisterKeyModalManualFallback | gh auth-fail | 100x30 | d7ad2eeb61e364b7e624712dfc72f5cf6b8b1c4b | 21d6c6ce5d4d997cbc23465094b9e477473ffa4191111a9f86dc60f53c484a57 |
| register-key-modal-u-key | identity-manager-register-key-modal-u-key.txt | TestIdentityManager_RegisterKeyModalOpensWithU | gh ok | 100x30 | d7ad2eeb61e364b7e624712dfc72f5cf6b8b1c4b | dbea6a08cd708207c792f1193de399f523d532d83bbae1373473cbd0b8e26589 |
| rotate-delete-offer-absent | identity-manager-rotate-delete-offer-absent.txt | TestIdentityManager_RotateDeleteOfferAbsentWhenInventoryFails | gh inventory-fail | 100x30 | d7ad2eeb61e364b7e624712dfc72f5cf6b8b1c4b | c97fff611819d343f51bd6d2d4ba6409a9e0ac623fa993e98d44f54e71e4cbe4 |
| rotate-delete-offer-default | identity-manager-rotate-delete-offer-default.txt | TestIdentityManager_RotateDeleteOfferDefaultsToLeave | gh delete-ok + inventory | 100x30 | d7ad2eeb61e364b7e624712dfc72f5cf6b8b1c4b | 9b95f3d52b207dc59d39683a26e680707169bad06bdae9ca3111c7514edf3b7d |
| rotate-delete-offer-delete | identity-manager-rotate-delete-offer-delete.txt | TestIdentityManager_RotateDeleteOfferDeletesOnExplicitChoice | gh delete-ok + inventory | 100x30 | d7ad2eeb61e364b7e624712dfc72f5cf6b8b1c4b | 68ce74e8d57888cc1a62bae7d714a224d5a909bb6d9c42fe407da83319d86344 |
| upload-already-complete | create-flow-upload-already-complete.txt | TestCreateFlow_UploadAlreadyCompleteCollapsesToOneLine | gh inventory-both | 100x30 | d7ad2eeb61e364b7e624712dfc72f5cf6b8b1c4b | cc7a3ccc804f02187faadc0bddab4b3074180035a29a2bf6749471d0ddc390d4 |
| upload-checkbox-disabled | create-flow-upload-checkbox-disabled.txt | TestCreateFlow_UploadCheckboxDisabledState | deny shim (no gh) | 100x30 | d7ad2eeb61e364b7e624712dfc72f5cf6b8b1c4b | caa4e9698828e1705437d5cf63114ad862ee39394eb05106fe3023f6a3fca323 |
| upload-checkbox-ready | create-flow-upload-autonomous-github.txt | TestCreateFlow_UploadAutonomousGitHubTracer | gh ok | 100x30 | d7ad2eeb61e364b7e624712dfc72f5cf6b8b1c4b | ca2cb60c3bc5d4c595331da465e3fd5d728bc94a3a1765a2acb0bf769b4d754a |
| upload-checkbox-tab-and-click | create-flow-upload-checkbox-tab-and-click.txt | TestCreateFlow_UploadCheckboxTabAndClickReachable | gh ok | 100x30 | d7ad2eeb61e364b7e624712dfc72f5cf6b8b1c4b | 8d6dc1d34a19088a2a524d372c10dab0dcbf034bf0490de56f8e0a3bf33ce26f |
| upload-checkbox-unauth | create-flow-upload-checkbox-unauth.txt | TestCreateFlow_UploadCheckboxUnauthState | gh auth-fail | 100x30 | d7ad2eeb61e364b7e624712dfc72f5cf6b8b1c4b | caa4e9698828e1705437d5cf63114ad862ee39394eb05106fe3023f6a3fca323 |
| upload-manual-fallback | create-flow-upload-partial-scope.txt | TestCreateFlow_UploadPartialScopeShowsBothRows | gh scope-fail-signing | 100x30 | d7ad2eeb61e364b7e624712dfc72f5cf6b8b1c4b | 3344c0d6bba07081df4d0f0ef4ff7b91c7ac0f4e0523a5bf33a0616fe61b9ff9 |
| upload-omitted | create-flow-reachable-not-uploaded-evidence.txt | TestCreateFlow_ReachableNotUploadedEvidence | fake ssh denied, no gh | 100x30 | d7ad2eeb61e364b7e624712dfc72f5cf6b8b1c4b | 61cb9bcf5352c0cd62419075518bcc43a4996ff1570f807b336f28365cff56a2 |
