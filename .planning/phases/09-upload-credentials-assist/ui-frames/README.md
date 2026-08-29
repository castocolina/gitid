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
| register-key-modal | identity-manager-register-key-modal-runs.txt | TestIdentityManager_RegisterKeyModalRuns | gh ok | 100x30 | 2c861fb59b997d5602eb506d992243e9ca719781 | 313e3fc844295cf44d0265c4ef2984bb6e942426de4eaef5ec98ed1bb739e4b9 |
| register-key-modal-manual-fallback | identity-manager-register-key-modal-manual-fallback.txt | TestIdentityManager_RegisterKeyModalManualFallback | gh auth-fail | 100x30 | 2c861fb59b997d5602eb506d992243e9ca719781 | 5f7980df6dee0dc72c0c03243ddcc5e539bdb4f2507777db31e1010431ea810a |
| register-key-modal-u-key | identity-manager-register-key-modal-u-key.txt | TestIdentityManager_RegisterKeyModalOpensWithU | gh ok | 100x30 | 2c861fb59b997d5602eb506d992243e9ca719781 | e1e733137bf0ed13513717986cfbc43793e7ee6be59ad07577afbe4160d571e5 |
| rotate-delete-offer-absent | identity-manager-rotate-delete-offer-absent.txt | TestIdentityManager_RotateDeleteOfferAbsentWhenInventoryFails | gh inventory-fail | 100x30 | 2c861fb59b997d5602eb506d992243e9ca719781 | dce627e84ca148ffc645d07429580483f21f7630551b80507d4649c3c048f863 |
| rotate-delete-offer-default | identity-manager-rotate-delete-offer-default.txt | TestIdentityManager_RotateDeleteOfferDefaultsToLeave | gh delete-ok + inventory | 100x30 | 2c861fb59b997d5602eb506d992243e9ca719781 | cf3cd58f0f2a48fe8f810fab764b333e21a739b6b544aa2de0bdf54d612ea5bd |
| rotate-delete-offer-delete | identity-manager-rotate-delete-offer-delete.txt | TestIdentityManager_RotateDeleteOfferDeletesOnExplicitChoice | gh delete-ok + inventory | 100x30 | 2c861fb59b997d5602eb506d992243e9ca719781 | 813c3869b1239743e32faa7f38fc108668e44e2c931ac0df626b2a5825e4fc3d |
| upload-already-complete | create-flow-upload-already-complete.txt | TestCreateFlow_UploadAlreadyCompleteCollapsesToOneLine | gh inventory-both | 100x30 | 2c861fb59b997d5602eb506d992243e9ca719781 | 188f7d1ffd633bb7c491a8e7e8b5438bfbee0034c786e25e25e766b2f92c637c |
| upload-checkbox-disabled | create-flow-upload-checkbox-disabled.txt | TestCreateFlow_UploadCheckboxDisabledState | deny shim (no gh) | 100x30 | 2c861fb59b997d5602eb506d992243e9ca719781 | 3eb17e5ec77b2b396c173d2ac7dbe7c2a348978aca590fec7366e65222506629 |
| upload-checkbox-ready | create-flow-upload-autonomous-github.txt | TestCreateFlow_UploadAutonomousGitHubTracer | gh ok | 100x30 | 2c861fb59b997d5602eb506d992243e9ca719781 | 3155173ef6b2fa769afac2c891be3e523a3fd1f97ba67d2226d74f182edfe460 |
| upload-checkbox-tab-and-click | create-flow-upload-checkbox-tab-and-click.txt | TestCreateFlow_UploadCheckboxTabAndClickReachable | gh ok | 100x30 | 2c861fb59b997d5602eb506d992243e9ca719781 | 7522f38913b33ff5e24d4228884bc3eead23a29303a1efd04a12bcc9fbc11558 |
| upload-checkbox-unauth | create-flow-upload-checkbox-unauth.txt | TestCreateFlow_UploadCheckboxUnauthState | gh auth-fail | 100x30 | 2c861fb59b997d5602eb506d992243e9ca719781 | 3eb17e5ec77b2b396c173d2ac7dbe7c2a348978aca590fec7366e65222506629 |
| upload-manual-fallback | create-flow-upload-partial-scope.txt | TestCreateFlow_UploadPartialScopeShowsBothRows | gh scope-fail-signing | 100x30 | 2c861fb59b997d5602eb506d992243e9ca719781 | 0968b17d3094322d81584fedeb12b51b82fd0d52ca6108accc65b6eed8f132ac |
| upload-omitted | create-flow-reachable-not-uploaded-evidence.txt | TestCreateFlow_TestStageReachableNotUploaded | fake ssh denied, no gh | 100x30 | 2c861fb59b997d5602eb506d992243e9ca719781 | 152fb6c0380ca2d4b4baad1cb4d78946183ef3bc346c2ae01a51639f092b5737 |
