# Graph Report - ssh-git-config  (2026-09-01)

## Corpus Check
- 316 files · ~706,572 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 5464 nodes · 20206 edges · 176 communities (157 shown, 3 thin omitted)
- Extraction: 79% EXTRACTED · 21% INFERRED · 0% AMBIGUOUS · INFERRED: 4192 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `3e305db0`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- newBackendForHome
- testing.T
- identities.go
- identities_test.go
- charm.land/bubbletea/v2.Cmd
- stripANSI
- createflow_packet_test.go
- press
- lifecycle_test.go
- ParseManagedHosts
- realBackend
- mustSee
- Seed
- NewApp
- identModel
- gate_visual_regression_test.go
- GenerateMaterial
- identity_manager_pty_e2e_test.go
- BuildRegionDiffs
- seedDeleteFixture
- identitiesModel
- cliTestCmd
- FixtureBackend
- ExtractRegion
- ceremonyModel
- Write
- gitconfig/baseline.go
- DemoFinding
- wiring.go
- createflow.go
- upload_real_account_gitlab_e2e_test.go
- runUploadSpec
- shadow_test.go
- Finding
- identity_cli_e2e_test.go
- identity_manager_upload_test.go
- harness_test.go
- App
- tester.go
- ExistingKey
- ListBlocks
- ReplaceProvisionalBlock
- migrate_test.go
- gitIgnoreModel
- global_git_pty_e2e_test.go
- CaptureCreateFlowScreens
- uploader.go
- wiring_storage_test.go
- ptySession
- RenderHostBlock
- DeriveCloneInput
- store.go
- git.go
- Match
- delete_test.go
- uploader_test.go
- PolicyFor
- upload_real_account_e2e_test.go
- CLAUDE.md
- gitid-evidence/main.go
- DemoState
- EnsureGlobals
- state_test.go
- CreateSpec
- archive_test.go
- identity/identity_test.go
- .view
- validation_test.go
- EnsureIncludeLine
- PlanDelete
- PolicyFor
- keyscan_test.go
- git_configuration_pty_e2e_test.go
- CheckCoherence
- gitconfig/reader_test.go
- EnsureGlobalGit
- modes_test.go
- gitid Agent Instructions
- ssh.go
- SandboxHome
- CheckBaseline
- Reconstruct
- RequiredScreenSpecs
- Account
- .planUpload
- adopt.go
- SSH/Git Identity Manager — Product Requirements Document (PRD)
- startGlobalSSHPTY
- migrate.go
- effectiveProbe
- adopter_test.go
- io.Writer
- realBackend
- Requirements Description
- Statuses
- signing_test.go
- Execution Phases
- BuildInventory
- fix_test.go
- gitid-evidence/main_test.go
- createInputFromCreateFlags
- Rotate
- Deps
- CreateInput
- newWizardPrefilled
- globalgit/version_test.go
- GitSpec
- platform.go
- CheckOrphans
- DetectOverlaps
- github.com/spf13/cobra.Command
- identityVerb
- scan.go
- capabilities.go
- resolveHomeForCLI
- captureTUIScreen
- .GlobalGitOptionStates
- WriteFragment
- Catalog
- lifecycle_fallbackauthor_test.go
- authorresolve.go
- reserved_test.go
- .runGlobalSSHApply
- CheckRedundancy
- runIdentityKeyVerb
- ShortSandboxHome
- EnsureGitFallbackAuthor
- ScanDirectives
- CheckDeps
- shadow.go
- RedactCLIOutput
- displayMessages
- identity/provisional_test.go
- CheckPermissions
- gitid-frame-promote/main.go
- update_test.go
- ExactTextViewport
- writeFakeSSH
- determinism_test.go
- ReusableKeyView
- CaptureTUI
- gitid git JSON schema
- OptionPolicy
- deps/deps.go
- fragment_signing_test.go
- CaptureHTML
- gitid
- TestGitJSONOptionsListExactKeySetAndEnums
- DeleteDeps
- gitid ssh JSON schema
- SSHVersion
- BundleFor
- globalssh.go
- Copy
- globalgit/isolation_contract_test.go
- BinaryInstallInfo
- TestKeyTypeMapping
- gitid CLI parity matrix
- missingFileError
- github.com/castocolina/gitid

## God Nodes (most connected - your core abstractions)
1. `newBackendForHome()` - 259 edges
2. `press()` - 211 edges
3. `NewApp()` - 156 edges
4. `identModel()` - 142 edges
5. `realBackend` - 141 edges
6. `appView()` - 133 edges
7. `mustSee()` - 119 edges
8. `SandboxHome()` - 119 edges
9. `DemoState` - 113 edges
10. `stripANSI()` - 104 edges

## Surprising Connections (you probably didn't know these)
- `gitignorePairFinding()` --calls--> `fixExcludesfile()`  [INFERRED]
  internal/doctor/checks/baseline.go → cmd/gitid/wiring.go
- `TestGlobalSSHToggleAndClickRespectSelectability()` --calls--> `assertUnchanged()`  [INFERRED]
  internal/tuikit/globalssh_test.go → cmd/gitid/wiring_test.go
- `runCommitCreate()` --references--> `WizardCommitMsg`  [EXTRACTED]
  cmd/gitid/wiring_test.go → internal/tuikit/views.go
- `main()` --calls--> `NewApp()`  [EXTRACTED]
  cmd/gitid-dummy/main.go → internal/tuikit/app.go
- `TestDemoAppConstructsAndRenders()` --calls--> `NewApp()`  [EXTRACTED]
  cmd/gitid-dummy/main_test.go → internal/tuikit/app.go

## Import Cycles
- None detected.

## Communities (176 total, 3 thin omitted)

### Community 0 - "newBackendForHome"
Cohesion: 0.04
Nodes (174): TestBaselineGitignoreFixPreservesOtherBaselineSettings(), TestJournalRecordCreatedDirDisjointness(), TestJournalRestoreRemovesCreatedPathsProvesR208(), TestRotateAndRepairWatchPathsIncludeSSHConfigPath(), TestRunGlobalSSHApplyInconclusiveSimulationPermitsWrite(), buildUploaderDeps(), doctorFindings(), newBackendForHome() (+166 more)

### Community 1 - "testing.T"
Cohesion: 0.03
Nodes (121): assertCompletionScript(), TestCompletionBash(), TestCompletionDynamicListIncludesIdentity(), TestCompletionFish(), TestCompletionZsh(), TestGitCmdFlagParsing(), TestGitCompletionStillGenerates(), TestGitNounGroupHelpListsRealVerbs() (+113 more)

### Community 2 - "identities.go"
Cohesion: 0.04
Nodes (71): charm.land/bubbles/v2/textinput.Model, sectionHeader(), anchoredLabelMatch(), atoiSafe(), baselineStrip(), baselineStripCompact(), blockedForwardNote(), ceremonyFooterActions() (+63 more)

### Community 3 - "identities_test.go"
Cohesion: 0.05
Nodes (116): stubDefaultDeletePlan(), TestReservedFooterHonestInKeyConsumingStates(), TestWizardGitButtonsArrowNavigatesWizardSteps(), clearFieldRaw(), clearPrefixRaw(), clonedWizardAtGitStep(), completeCommit(), completeStage() (+108 more)

### Community 4 - "charm.land/bubbletea/v2.Cmd"
Cohesion: 0.03
Nodes (51): printApplyDryRun(), charm.land/bubbletea/v2.Cmd, NoopIdentityPlanner, findStubRow(), fixtureGlobalGitOptionViews(), fixtureSSHStorageView(), stubNameTaken(), TestGlobalGitPolicyBackedRowIsSelectable() (+43 more)

### Community 5 - "stripANSI"
Cohesion: 0.04
Nodes (90): charm.land/lipgloss/v2.Style, image/color.Color, padRight(), fixtureGlobalSSHOptionViews(), TestOptionRowNowValueClipsWithEllipsis(), dimPane(), fitLine(), footerActionAt() (+82 more)

### Community 6 - "createflow_packet_test.go"
Cohesion: 0.06
Nodes (87): canonicalSemanticPacket(), loadReviewInput(), makeCandidate(), TestCanonicalManifestIgnoresRawPTYTranscriptVariation(), writeValidPNG(), writeCanonicalManifest(), image/color.RGBA, approvedHTMLRoutesInternal() (+79 more)

### Community 7 - "press"
Cohesion: 0.07
Nodes (86): press(), regionFlat(), gignApp(), gignModel(), TestGitIgnoreActivateStoresState(), TestGitIgnoreApplyOpensCeremonyWithPlan(), TestGitIgnoreApplyPlanErrorFailsClosed(), TestGitIgnoreBrowseFooterUsesEditorCopy() (+78 more)

### Community 8 - "lifecycle_test.go"
Cohesion: 0.05
Nodes (75): managedFixturePaths(), TestIdentityDeleteUnauthorizedNoYesLeavesBytesIdentical(), TestIdentityDeleteWithYesCompletesAndDryRunWritesNothing(), TestIdentityKeyVerbUnauthorizedNoYesLeavesBytesIdentical(), TestRunGitFallbackAuthorApply_DeclinedConfirmation(), TestRunGitFallbackAuthorApply_DryRun(), TestRunGitFallbackAuthorApply_EmptyPairNoop(), TestRunGitFallbackAuthorApply_InjectedFailureRestores() (+67 more)

### Community 9 - "ParseManagedHosts"
Cohesion: 0.05
Nodes (63): perAliasFromContent(), TestPerAliasConformance(), TestPerAliasConformanceEmpty(), AllHostStanzas(), extractProviderMarker(), HostBlockFacts, SSHHostInfo, hostLineNumber() (+55 more)

### Community 10 - "realBackend"
Cohesion: 0.06
Nodes (13): applyConvergenceAlarms(), buildDeleteDeps(), buildIdentityDeps(), countForeignProviderRefs(), realBackend, nameFromAlias(), planTokenFor(), providerDisplayName() (+5 more)

### Community 11 - "mustSee"
Cohesion: 0.16
Nodes (71): clickLabelRow(), delayedResolutionSSHDir(), newRealCreateFlowCmd(), openCreateWizard(), quitCleanly(), requireFocusedProof(), seedEncryptedKeyFixture(), tabKeys() (+63 more)

### Community 12 - "Seed"
Cohesion: 0.06
Nodes (69): go/ast.Expr, newScreens(), TestNewScreensExhaustiveSwitchOverTabID(), Seed(), newGlobalGitModel(), TestGitFallbackInputsSeededFromStateView(), TestGlobalGitActivateReturnsNilCmd(), TestGlobalGitBaselineCeremonyOmitsCrossWarningWhenFallbackIsComplete() (+61 more)

### Community 13 - "NewApp"
Cohesion: 0.07
Nodes (68): NewApp(), NewAppPrefilled(), appView(), TestHelpOverlayShowsFullLegend(), TestNewAppPrefilledNilBehavesLikeNewApp(), TestNewAppPrefilledOpensWizardPrefilled(), TestNewAppPrefilledReuseKeyPopulatesPicker(), TestNewAppRendersTheFrame() (+60 more)

### Community 14 - "identModel"
Cohesion: 0.07
Nodes (66): TestCeremonyArrowsMoveButtonFocusNonDestructive(), TestCeremonyDestructiveArrowsStayOnTypedInput(), TestCeremonyTabRingAndEnterActivatesFocused(), TestCloneFocusRingInputToButton(), TestDeleteScopeRingTabAndArrows(), TestEditSSHFocusRingReachesRewriteButton(), TestMouseCeremonyButtonsCancelConfirmDone(), TestMouseCloneButtonClones() (+58 more)

### Community 15 - "gate_visual_regression_test.go"
Cohesion: 0.14
Nodes (67): assertAllComparableEqualRegionsAreMutationSensitive(), buildHealthFixerCaptures(), buildUploadCaptures(), deterministicGitIdentityFixture(), deterministicGitIgnoreFixture(), deterministicGlobalGitFixture(), deterministicGlobalSSHFixture(), deterministicHealthFixerFixture() (+59 more)

### Community 16 - "GenerateMaterial"
Cohesion: 0.06
Nodes (57): TestSmokeNetworkConnectivity(), doctorAddWiring(), golang.org/x/crypto/ssh.PublicKey, DerivePublicKey(), TestDerivePublicKeyMissingFile(), TestDerivePublicKeyRejectsGarbage(), TestDerivePublicKeyRoundTrips(), generateEd25519() (+49 more)

### Community 17 - "identity_manager_pty_e2e_test.go"
Cohesion: 0.09
Nodes (61): compareCreateFlowCheckpoint(), loadCreateFlowAllowlist(), createFlowAllowlistEntry, errorRecorder, fakeErrorRecorder, repoRoot(), allIdentManagerRegions(), assertManifestFields() (+53 more)

### Community 18 - "BuildRegionDiffs"
Cohesion: 0.06
Nodes (64): TestNegativeControl_GlobalGitMidByteTruncationHashStable(), TestNegativeControl_HealthFixerUnclassifiedDifference(), TestNegativeControl_MissingGitScreenState(), TestNegativeControl_MissingGlobalSSHState(), TestNegativeControl_MissingIdentityManagerState(), TestNegativeControl_UploadVisualUnclassifiedDifference(), gitIgnoreVisualSpecs(), gitScreenSpecs() (+56 more)

### Community 19 - "seedDeleteFixture"
Cohesion: 0.06
Nodes (64): confirmDelete(), runIdentityDelete(), checkParityMatrix(), namedCommandPaths(), parityToolingExcluded(), parseParityMatrix(), passStageMsg(), reservedNoun() (+56 more)

### Community 20 - "identitiesModel"
Cohesion: 0.10
Nodes (25): charm.land/bubbletea/v2.KeyMsg, mustKey(), synthKey(), blockLine(), hitNeedle(), hitAnyFieldRow(), hitFieldRow(), hitStrategyRow() (+17 more)

### Community 21 - "cliTestCmd"
Cohesion: 0.09
Nodes (58): assertGitApplyEnvelope(), assertGitFallbackSetEnvelope(), captureGitApplyJSON(), captureGitFallbackSetJSON(), captureGitFallbackShowJSON(), captureGitOptionsListJSON(), gitEnvHome(), runnableGitPaths() (+50 more)

### Community 22 - "FixtureBackend"
Cohesion: 0.05
Nodes (13): findFixtureRow(), fixtureNameTaken(), fixtureStageCmd(), FixtureBackend, identityManagerSSHHost(), providerHostFromAlias(), NoopGitFallbackAuthorPlanner, ValidationError (+5 more)

### Community 23 - "ExtractRegion"
Cohesion: 0.10
Nodes (55): markerInPane(), normalizeCapturedStateText(), ansiOffsetToRaw(), extractActionMenuRows(), extractBackupPathList(), extractBreadcrumb(), extractConfirmationPreview(), extractConfirmWarningBlock() (+47 more)

### Community 24 - "ceremonyModel"
Cohesion: 0.07
Nodes (43): ceremonyClickKey(), newCeremony(), renderReceiptList(), asyncCeremony(), destructiveCeremony(), plainCeremony(), TestCeremonyAsyncConfirmShowsNoReceiptUntilSuccess(), TestCeremonyAsyncFailureIsVisibleAndRetryable() (+35 more)

### Community 25 - "Write"
Cohesion: 0.06
Nodes (20): cohFileInfo, fakeFileInfo, orphFileInfo, containedRegularPath(), gitDirSnapshot, gitFileSnapshot, mutationJournal, os.FileMode (+12 more)

### Community 26 - "gitconfig/baseline.go"
Cohesion: 0.08
Nodes (46): gitIgnoreMalformedReasonText(), BaselineConfig, ManagedBlockError, SentinelLineError, URLRewrite, TestFixtureBackendGlobalGitIgnore(), TestRunUploadForIdentityOmitsForNonGatedHost(), TestRunUploadForIdentityQuotesTitle() (+38 more)

### Community 27 - "DemoFinding"
Cohesion: 0.08
Nodes (38): findingsForIdentity(), findingsForIdentityPairs(), healthFinish(), newHealthCmd(), printHealthFindings(), runHealth(), suppressParseErrorFindingPairs(), suppressParseErrorFindings() (+30 more)

### Community 28 - "wiring.go"
Cohesion: 0.08
Nodes (33): cloneCeremonyInputs(), realBackend, orDefault(), buildAuthorResolutionCheck(), buildDoctorDeps(), containsLine(), expandTildeForHome(), fileExists() (+25 more)

### Community 29 - "createflow.go"
Cohesion: 0.15
Nodes (48): charm.land/bubbletea/v2.Model, anyView(), CaptureGitIgnoreScreens(), CaptureGitScreenScreens(), CaptureGlobalGitScreens(), CaptureGlobalSSHScreens(), CaptureHealthFixerScreens(), CaptureIdentityManagerScreens() (+40 more)

### Community 30 - "upload_real_account_gitlab_e2e_test.go"
Cohesion: 0.10
Nodes (42): fakeGLabInventoryRecord, glabArgvRecorder, glabTokenSelf, outstandingGLabRemoteKeys, recordedGLabArgv, recordedGLabRemoteKey, assertNoGLabTokenRevealingFlag(), countUnscopedGLab() (+34 more)

### Community 31 - "runUploadSpec"
Cohesion: 0.09
Nodes (44): encodeDeleteCandidates(), printUploadOutcome(), TestPlanUploadReadFileFailureRedactsHomePath(), TestPrintUploadOutcomeMatchesTheWizardSection(), TestPrintUploadOutcomeRendersEachSection(), TestPrintUploadOutcomeUsesOnlyFrozenCopy(), TestRunUploadForIsTheOnlyOrchestration(), TestSelfHostedCreateNeverPrintsTheNoUploadFlagNote() (+36 more)

### Community 32 - "shadow_test.go"
Cohesion: 0.19
Nodes (43): BuildGraph(), Simulate(), badOutput(), buildDeps(), deriveAnswerFromConfig(), Deps, managedBlockFor(), recommendedOutput() (+35 more)

### Community 33 - "Finding"
Cohesion: 0.09
Nodes (39): confirmFix(), firstFixable(), newFixCmd(), runFix(), runFixCommand(), scanForFix(), findingStableID(), runDoctorAndConvert() (+31 more)

### Community 34 - "identity_cli_e2e_test.go"
Cohesion: 0.12
Nodes (41): seedGitPTYIdentity(), runHealthCLI(), TestHealthFixCLIParity(), healthDoc, archivePrivateFor(), assertManagedArtifactsEqual(), assertReferentialCoherence(), assertReferentialCoherenceFails() (+33 more)

### Community 35 - "identity_manager_upload_test.go"
Cohesion: 0.11
Nodes (41): stubDefaultKeyCeremonyPlan(), frameBodyRows(), openKeyCeremonyAtReview(), pressAndRun(), TestKeyCeremonyCommitReducesOnlyAfterSuccess(), TestKeyCeremonyMaximalFixtureFitsFrame(), TestKeyCeremonyRepairOmitsGraceAndArchive(), TestKeyCeremonyRotateRendersGraceAndArchive() (+33 more)

### Community 36 - "harness_test.go"
Cohesion: 0.12
Nodes (39): TestCreateFlow_ExistingPTYCannotReachRealProviderCLI(), TestDebugCaps_RealWiring(), e2eT, ambientPathSentinelHit(), e2eEnv(), envValue(), FakeGitDir(), FakeGitShimDir() (+31 more)

### Community 37 - "App"
Cohesion: 0.09
Nodes (27): charm.land/bubbletea/v2.MouseClickMsg, charm.land/bubbletea/v2.View, NewAppOnGlobalGit(), NewAppOnGlobalSSH(), TestNewAppOnGlobalGitOpensEmptySelection(), TestNewAppOnGlobalSSHOpensEmptyOptionsAndStorage(), TestMouseDoctorFixThisButtonAndCeremonyCancel(), docModel() (+19 more)

### Community 38 - "tester.go"
Cohesion: 0.10
Nodes (38): UpdateDeps, UpdateResult, expandTilde(), readPubLine(), TestReadPubLine_ExpandsTilde(), Update(), ClassifyPreWrite(), TestResolvedViaCommandMatchesResolvedViaArgv() (+30 more)

### Community 39 - "ExistingKey"
Cohesion: 0.11
Nodes (37): deleteCandidatesDetail(), decodeProviderKeyPages(), DeleteRecordedKey(), FindByTitle(), glabInventory(), Deps, ExistingKey, HasRegistration() (+29 more)

### Community 40 - "ListBlocks"
Cohesion: 0.09
Nodes (35): NamedBlock, TestInsertBlockAfter_CRLFAnchor(), TestInsertBlockAfter_MissingAnchor(), TestInsertBlockAfter_PlacesImmediatelyAfterAnchor(), TestInsertBlockAfter_UpdateInPlace(), InsertBlockAfter(), TestListBlocks_CRLFNormalized(), TestListBlocks_Empty() (+27 more)

### Community 41 - "ReplaceProvisionalBlock"
Cohesion: 0.11
Nodes (36): removeBlockWith(), ListProvisionalBlocks(), RemoveProvisionalBlock(), ReplaceProvisionalBlock(), TestListProvisionalBlocks_Empty(), TestListProvisionalBlocks_MutualExclusion(), TestListProvisionalBlocks_ReturnsProvisional(), TestProvisionalRoundTrip_WriteListRemove() (+28 more)

### Community 42 - "migrate_test.go"
Cohesion: 0.23
Nodes (39): Migrate(), RealMigrateDeps(), assertGlobalsBlockLast(), containsBlockName(), fakeDepsForMutate(), migrateFixture(), mustReadFile(), parseIdentityFiles() (+31 more)

### Community 43 - "gitIgnoreModel"
Cohesion: 0.08
Nodes (21): IdentityManagerRow, charm.land/bubbles/v2/textarea.Model, HealthFindingByID(), TestFixtureBackendSatisfiesIdentityPlannerThroughNoop(), TestFixtureConsistency(), maxInt(), GitIgnoreReceiptWrongTarget(), GitIgnoreWiringPointsElsewhere() (+13 more)

### Community 44 - "global_git_pty_e2e_test.go"
Cohesion: 0.16
Nodes (36): gitApplyDoc, gitFallbackDoc, gitFallbackSetDoc, gitOptionRecord, gitOptionsDoc, containsSubstring(), runGitCLI(), TestGlobalGitCLI_BelowGateAdvisoryExitCodes() (+28 more)

### Community 45 - "CaptureCreateFlowScreens"
Cohesion: 0.10
Nodes (34): main(), TestDemoAppConstructsAndRenders(), predicateMatches(), NewFixtureBackend(), CaptureCreateFlowScreens(), CompareTextCaptures(), TestCompareTextCaptures_DifferentReturnError(), TestCompareTextCaptures_IdenticalReturnsNil() (+26 more)

### Community 46 - "uploader.go"
Cohesion: 0.13
Nodes (34): desiredRegistrations(), toUploadResultRow(), entriesWithScope(), exactScopedEntry(), registrationLabel(), deleteArgs(), DeleteCommandPreview(), DeleteKey() (+26 more)

### Community 47 - "wiring_storage_test.go"
Cohesion: 0.19
Nodes (35): assertNoDualPresence(), backendWithFakeSSH(), buildFakeSSHForMigration(), realBackend, hasBlock(), runStorageCommit(), seedMigrateHome(), seedMigrateHomeInclude() (+27 more)

### Community 48 - "ptySession"
Cohesion: 0.13
Nodes (30): newDummyCreateFlowCmd(), writeFileT(), confirmNonDestructiveFix(), makeImmutable(), openFixer(), openHealth(), seedHealthFixerBatch(), seedHealthFixerFlagship() (+22 more)

### Community 49 - "RenderHostBlock"
Cohesion: 0.10
Nodes (30): ioDiscard, Backup(), TestBackupLeavesOriginalUnchanged(), TestBackupMissingFileReturnsEmpty(), TestCopyFileExclusiveRefusesExistingDestination(), TestEnsureDir(), TestWriteBacksUpExistingTarget(), TestWriteBackupNamesAreCollisionProof() (+22 more)

### Community 50 - "DeriveCloneInput"
Cohesion: 0.14
Nodes (32): CloneNotices, CloneTargets, FieldError, DeriveCloneInput(), deriveCloneMatches(), GitdirMatch(), HasconfigMatch(), nameTaken() (+24 more)

### Community 51 - "store.go"
Cohesion: 0.06
Nodes (21): cloneState(), CountFindings(), AddIdentity, FixFinding, HealthRollup(), recomputeAfterGit(), ApplyGitBaseline, ApplyGitGlobalEmail (+13 more)

### Community 52 - "git.go"
Cohesion: 0.13
Nodes (34): fillGitApplyFromResult(), fillGitFallbackSetFromResult(), finishGitApply(), finishGitFallbackSet(), gitApplyTokenHelp(), gitApplyTokens(), gitOptionRecords(), gitRowStateName() (+26 more)

### Community 53 - "Match"
Cohesion: 0.12
Nodes (31): MatchKind, TestIncludeIfGitdir_ResolvesViaRealGit(), TestParseManagedIncludeIfExcludesProviderRewrite(), Match, ProviderRewriteBlockName(), RemoveProviderRewrite(), renderBlockBody(), RenderCheckedIncludeIf() (+23 more)

### Community 54 - "delete_test.go"
Cohesion: 0.23
Nodes (34): deleteCallLog, Delete(), baseDeleteAccount(), containsBlock(), containsLine(), containsStr(), fatalOnInvokeSSHDeps(), gcFixtureWithBlocks() (+26 more)

### Community 55 - "uploader_test.go"
Cohesion: 0.09
Nodes (32): AuthCheck(), DetectFor(), assertArgs(), recordingRunCmd(), TestAuthCheck_Authenticated(), TestAuthCheck_NotLoggedIn(), TestAuthCheckPassesHostname(), TestCommandPreviewQuotesTheD07TitleForSafeCopyPaste() (+24 more)

### Community 56 - "PolicyFor"
Cohesion: 0.10
Nodes (28): sshNAReasonWire(), sshOptionRecords(), sshSourceWire(), sshStateWire(), toGlobalSSHOptionState(), OptionPolicy, VersionOutcome, firstErr() (+20 more)

### Community 57 - "upload_real_account_e2e_test.go"
Cohesion: 0.14
Nodes (28): argvRecorder, outstandingRemoteKeys, recordedArgv, recordedRemoteKey, ambientEnvMap(), countUnscoped(), deleteRecordedRemoteKey(), drainOutstandingRemoteKeys() (+20 more)

### Community 58 - "CLAUDE.md"
Cohesion: 0.06
Nodes (31): 1. `~/.ssh/config` parsing: kevinburke/ssh_config, 2. `~/.gitconfig` parsing and writing, 3. Ed25519 key generation + OpenSSH formatting + `allowed_signers`, 4. Cobra + shell completion, 5. Quality toolchain, Alternatives Considered, Area-by-Area Rationale, BEGIN gitid managed: <identity-name> (+23 more)

### Community 59 - "gitid-evidence/main.go"
Cohesion: 0.13
Nodes (31): buildEvidenceJSON(), captureApprovedHTMLPanels(), captureApprovedTUIPanels(), captureLivePanels(), captureTUIPanels(), captureWorkspace(), commandOutput(), commandOutputIn() (+23 more)

### Community 60 - "DemoState"
Cohesion: 0.16
Nodes (10): charm.land/bubbletea/v2.Msg, findingsBanner(), gssOptionsTopLines(), pendingOptions(), firstBackup(), succeededOutcome(), DemoState, appliedOption (+2 more)

### Community 61 - "EnsureGlobals"
Cohesion: 0.16
Nodes (28): EnsureGlobals(), ensureGlobalsLast(), existingGlobalBody(), globalsBlockIsLast(), newGlobalMap(), parseGlobalBody(), renderGlobalBody(), assertGlobalsLastAfter() (+20 more)

### Community 62 - "state_test.go"
Cohesion: 0.12
Nodes (28): collapseState(), classifyCase, KeyAction, Severity, Inventory, Classify(), ClassifyState(), crossReferenceUnusedKeys() (+20 more)

### Community 63 - "CreateSpec"
Cohesion: 0.11
Nodes (9): atoiOr(), toTestResultView(), DefaultPort(), WizardStageMsg, stubStageCmd(), CreateSpec, TestOutcome, TestResultView (+1 more)

### Community 64 - "archive_test.go"
Cohesion: 0.15
Nodes (26): archiveDestPath(), copyExclusive(), CopyKeyPairToArchive(), CreatedFunc, MoveKeyPairToArchive(), prepareArchiveDir(), RemoveArchivedPair(), assertFileAbsent() (+18 more)

### Community 65 - "identity/identity_test.go"
Cohesion: 0.19
Nodes (30): orderRecorder, Create(), Deps, callLog, newFakeDeps(), newOrderRecordingDeps(), newSplitDeps(), sampleInput() (+22 more)

### Community 66 - ".view"
Cohesion: 0.15
Nodes (17): toGlobalGitNotApplicableReason(), fallbackCurrentLabel(), gitCueLine(), gitNeedsAttention(), gitRowForScreenRow(), gitVisibleRowCount(), globalGitNotApplicableSentence(), globalGitRowLine2() (+9 more)

### Community 67 - "validation_test.go"
Cohesion: 0.11
Nodes (26): TestGlobalsSharedMatcherNotReimplemented(), aliasCollides(), AliasCollision(), globMatch(), ValidationError, HostLineMatches(), HostMatch(), HostPatternsMatch() (+18 more)

### Community 68 - "EnsureIncludeLine"
Cohesion: 0.12
Nodes (28): github.com/kevinburke/ssh_config.Config, EnsureIncludeDir(), EnsureIncludeLine(), IsGlobalBlockName(), IsReservedBlockName(), ManagedBlockNames(), ReservedPaths(), containsName() (+20 more)

### Community 69 - "PlanDelete"
Cohesion: 0.18
Nodes (29): DeleteTarget, PlanDeps, DeleteScope, deleteTargets(), DeletePlan, nonEmptyStrings(), PlanDelete(), providerRewriteDeleteTarget() (+21 more)

### Community 70 - "PolicyFor"
Cohesion: 0.13
Nodes (29): TestClassify_BundleNeedsActionWhenOneUnset(), Classify(), TestClassify_AlreadySet_FromUnset(), TestClassify_AttributionByOriginPathNotMembership(), TestClassify_BundleAggregate(), TestClassify_BundleAllSetAndEqualIsAlreadySet(), TestClassify_BundleAllSetButSomeDifferIsSetButDiffers(), TestClassify_NeedsAction_WhenUnsetEverywhere() (+21 more)

### Community 71 - "keyscan_test.go"
Cohesion: 0.17
Nodes (27): fileExists(), fingerprintFromEncryptedPrivate(), ReusableKey, isPassphraseMissing(), pubMetadata(), scanKeyCandidate(), ScanManualKey(), ScanReusableKeys() (+19 more)

### Community 72 - "git_configuration_pty_e2e_test.go"
Cohesion: 0.17
Nodes (29): allGitScreenRegions(), assertGitBytesUnchanged(), compareGitScreenCheckpoint(), extractGitScreenCeremony(), extractGitScreenFormFields(), extractGitScreenHeaderStatus(), extractGitScreenPreview(), extractGitScreenRegion() (+21 more)

### Community 73 - "CheckCoherence"
Cohesion: 0.29
Nodes (29): CheckCoherence(), boolPtr(), cohContains(), cohStat(), cohTitles(), makeAccount(), signerLineFor(), TestCheckCoherenceAuthorResolution() (+21 more)

### Community 74 - "gitconfig/reader_test.go"
Cohesion: 0.12
Nodes (28): TestIsReservedBlockName_GitFallbackAuthor(), TestIsReservedBlockName_GlobalGit(), TestIsReservedBlockName_PlainIdentity(), conditionToMatch(), IncludeIfInfo, IsReservedBlockName(), parseIncludeIfBody(), ParseManagedIncludeIf() (+20 more)

### Community 75 - "EnsureGlobalGit"
Cohesion: 0.13
Nodes (27): sectionKeyValue, belongsToKnownSection(), ComposeBaselineInclude(), EnsureGlobalGit(), existingGlobalGitBody(), keysForSection(), renderGlobalGitBody(), fullTableSelection() (+19 more)

### Community 76 - "modes_test.go"
Cohesion: 0.17
Nodes (27): modeLog, assertOrder(), TestAddAccountNoPersistKey(), TestReuseNoPersistKey(), AddAccount(), ensurePubReadOnly(), fragmentPathFor(), Deps (+19 more)

### Community 77 - "gitid Agent Instructions"
Cohesion: 0.07
Nodes (23): Code Exploration, Commands, Detailed Guidance, gitid Agent Instructions, Non-Negotiable Rules, Required Start, UI Reference, Avoid (+15 more)

### Community 78 - "ssh.go"
Cohesion: 0.13
Nodes (24): buildSSHStorageDocument(), finishApply(), finishMigrate(), realBackend, newApplyEnvelope(), newMigrateEnvelope(), newSSHOptionsListVerb(), newSSHStorageShowVerb() (+16 more)

### Community 79 - "SandboxHome"
Cohesion: 0.30
Nodes (27): appendGitIgnoreLine(), applyGitIgnoreAndConfirm(), assertGitIgnoreFilesUnchanged(), captureGitIgnoreFrame(), newGitIgnoreCmd(), seedEditableGitIgnoreHome(), seedGitIgnoreHome(), snapshotGitIgnoreFiles() (+19 more)

### Community 80 - "CheckBaseline"
Cohesion: 0.20
Nodes (26): blockIsPopulated(), CheckBaseline(), excludesFileExists(), gitignorePairFinding(), pointsToManagedTarget(), setDiffersFindings(), fakeBaselineDeps(), fullyInstalledState() (+18 more)

### Community 81 - "Reconstruct"
Cohesion: 0.17
Nodes (26): nameUnion(), ProviderHostForSSHHostname(), ProviderKeyForHost(), ProviderRefCount(), Reconstruct(), buildGCBlock(), buildSSHBlock(), TestProviderHostForSSHHostname() (+18 more)

### Community 82 - "RequiredScreenSpecs"
Cohesion: 0.09
Nodes (27): TestCaptureTUIScreenReuseSelection(), globalSSHPTYFrameDir(), isGlobalSSHScopedRef(), namedPTYFrame(), readUploadFrameProvenanceStateIDs(), TestGlobalGitHTMLNonApplicabilityPerSpec(), TestGlobalSSHHTMLNonApplicabilityPerSpec(), TestGlobalSSHNonApplicabilityNamesExistingPTYFrame() (+19 more)

### Community 83 - "Account"
Cohesion: 0.21
Nodes (23): realBackend, repairLog, Account, Deps, keyDirFor(), repairInput(), RepairKey(), RepairKeyPath() (+15 more)

### Community 84 - ".planUpload"
Cohesion: 0.13
Nodes (18): decodeDeleteCandidates(), deleteCandidatesManualCommand(), realBackend, registrationOf(), uploadFailureView(), deleteCandidate, uploadPlan, uploadRequest (+10 more)

### Community 85 - "adopt.go"
Cohesion: 0.18
Nodes (25): resolveGlobalSSHTargetPath(), Adopt(), candidateTarget(), containsPath(), DetectInclude(), expandIncludePath(), IncludeDirective, includeDirectiveArgs() (+17 more)

### Community 86 - "SSH/Git Identity Manager — Product Requirements Document (PRD)"
Cohesion: 0.07
Nodes (26): 1. Background, 2. Domain model, 3. Source of truth & safe writes, 4.1 Identity / Account / Credential CRUD, 4.2 Two-phase test flow (input & output shown), 4.3 Clipboard, 4.4 Upload instructions, 4.5 Doctor (health checks) (+18 more)

### Community 87 - "startGlobalSSHPTY"
Cohesion: 0.21
Nodes (24): countBackups(), runSSHCLI(), TestGlobalSSHCLI_AdvisoryExitCodes(), TestGlobalSSHCLI_DryRunNoWrite(), TestGlobalSSHCLI_ListShowApplyIdempotent(), captureGlobalSSHFrame(), globalSSHSubTabStrip(), mustNotContainGlobalSSH() (+16 more)

### Community 88 - "migrate.go"
Cohesion: 0.19
Nodes (22): blockBodyMap(), buildMigrationDiff(), callStep(), checkDigestMatch(), composeDestination(), composeSource(), contentDigest(), equalStringSlices() (+14 more)

### Community 89 - "effectiveProbe"
Cohesion: 0.17
Nodes (24): EffectiveEntry, os/exec.ExitError, BuildProbeDeps(), effectiveProbe(), Deps, inFileProbe(), isExitErr(), isMissingFileErr() (+16 more)

### Community 90 - "adopter_test.go"
Cohesion: 0.16
Nodes (22): AdoptMethod, AdoptResult, fakeAdopterDeps, Adopt(), Deps, ListCandidates(), ListCandidatesFromHome(), MatchIdentityName() (+14 more)

### Community 91 - "io.Writer"
Cohesion: 0.19
Nodes (22): fp(), joinStrings(), newDebugCapsCmd(), newDebugCmd(), osNote(), printCapabilities(), printCatalog(), printInventory() (+14 more)

### Community 92 - "realBackend"
Cohesion: 0.18
Nodes (8): collectCreateBackups(), fallbackAuthorPreview(), findGitWorkTree(), realBackend, isGitWorkTree(), originNamesFile(), confirmationMode, lifecyclePolicy

### Community 93 - "Requirements Description"
Cohesion: 0.08
Nodes (24): Acceptance Criteria, Assumptions carried (confirm if wrong) — the 100→110 polish, Background, Constraints, Design Decisions, Detailed Requirements, Edge Cases, Execution Phases (+16 more)

### Community 94 - "Statuses"
Cohesion: 0.23
Nodes (23): Deps, Statuses(), resolutionDependentKeys(), TestStatuses(), TestStatusesClassifiesAllPolicyRows(), TestStatusesConcurrentLatency(), TestStatusesConfigReadErrorDegradesToNotApplicable(), TestStatusesConfigReadErrorIsIsolated() (+15 more)

### Community 95 - "signing_test.go"
Cohesion: 0.17
Nodes (22): agentState, os.FileInfo, CheckAgent(), CheckSigning(), classifyAgentState(), extractFingerprint(), isKeyLoaded(), fakeStatFails() (+14 more)

### Community 96 - "Execution Phases"
Cohesion: 0.08
Nodes (23): Acceptance Criteria, Background, Constraints, Design Decisions, Detailed Requirements, Execution Phases, Feature Overview, Functional (+15 more)

### Community 97 - "BuildInventory"
Cohesion: 0.18
Nodes (22): FragmentInfo, BuildInventory(), BuildInventoryDeps(), filterReservedKeyPaths(), InventoryDeps, InventoryDepsForHome(), listKeyFilesReal(), listKeyFilesRealForHome() (+14 more)

### Community 98 - "fix_test.go"
Cohesion: 0.14
Nodes (21): TestDoctorAliasFixForwardsFlagsUnchanged(), TestDoctorAliasFixWithoutYesPrompts(), TestDoctorAliasMatchesHealthAndIsHidden(), seedInstalledBaseline(), syntheticFinding(), TestBaselineGitignoreFixViaCLI(), TestFixCmdDryRunNoFixableFindings(), TestFixCmdDryRunWritesNothing() (+13 more)

### Community 99 - "gitid-evidence/main_test.go"
Cohesion: 0.14
Nodes (22): captureCommandEnvironment(), compareInventories(), run(), seedManualReuseKey(), sha256Hex(), addPNGMetadata(), mutateCandidateMember(), pngChunk() (+14 more)

### Community 100 - "createInputFromCreateFlags"
Cohesion: 0.17
Nodes (20): confirmYes(), createInputFromCreateFlags(), createPrefillFromFlags(), createPreviewLine(), realBackend, indentBlock(), runCreateCeremony(), runCreateDryRun() (+12 more)

### Community 101 - "Rotate"
Cohesion: 0.22
Nodes (21): rotateArchivePaths(), rotateLog, algoFromKeyPath(), Deps, RotateResult, Rotate(), rotateInput(), callLog (+13 more)

### Community 102 - "Deps"
Cohesion: 0.17
Nodes (20): CheckFn, allowedSignersMissingFindingWithFix(), buildSignersFix(), checkAuthorResolution(), checkDirectiveAboveManagedBlock(), checkHandWrittenIdentitiesOnly(), checkShadowedGlobalOptions(), coherenceForAccount() (+12 more)

### Community 103 - "CreateInput"
Cohesion: 0.31
Nodes (21): KeyResult, signersWriter, CreateInput, CreateResult, Deps, StagedKey, mergeCreateResults(), PersistAll() (+13 more)

### Community 104 - "newWizardPrefilled"
Cohesion: 0.11
Nodes (21): newGitForm(), newSSHForm(), newTextInput(), newWizard(), newWizardBase(), newWizardPrefilled(), providerFromHostname(), TestClonePrefilledWizardTestPhaseMatchesFreshWizard() (+13 more)

### Community 105 - "globalgit/version_test.go"
Cohesion: 0.19
Nodes (19): globalGitGateOutcome(), GateOutcome, OptionPolicy, RealGateForRow(), conflictstyleRow(), OptionPolicy, TestVersionGateNamesTheGatedOptionNotAHardcodedLiteral(), TestVersionGateNoMinimum() (+11 more)

### Community 106 - "GitSpec"
Cohesion: 0.12
Nodes (6): matchesFor(), GitSpec, WritePlanView, fixedGitWritePlanBackend, GitOriginal, sentinelIncludeIfBackend

### Community 107 - "platform.go"
Cohesion: 0.17
Nodes (18): toAlgorithmCatalogEntry(), clipboardInstallHint(), CurrentOS(), gitInstallHint(), InstallHint(), libfido2InstallHint(), normalizeTool(), opensshInstallHint() (+10 more)

### Community 108 - "CheckOrphans"
Cohesion: 0.32
Nodes (19): CheckOrphans(), incompleteIdentityNames(), sliceToSet(), orphContains(), orphStat(), orphTitles(), TestMissingFragmentNoDuplicate(), TestOrphanAliasHostNoInclude() (+11 more)

### Community 109 - "DetectOverlaps"
Cohesion: 0.22
Nodes (18): OverlapPair, CheckOverlap(), classifyGitdirOverlap(), DetectOverlaps(), extractGitHost(), hasconfigOverlaps(), normGitdir(), makeGitdirAccount() (+10 more)

### Community 110 - "github.com/spf13/cobra.Command"
Cohesion: 0.18
Nodes (18): newGitCmd(), newGitFallbackCmd(), newGitFallbackShowVerb(), newGitOptionsCmd(), runGitFallbackShow(), confirmationPolicyFrom(), newIdentityCmd(), newVerbCmd() (+10 more)

### Community 111 - "identityVerb"
Cohesion: 0.25
Nodes (19): newGitFallbackSetVerb(), newGitOptionsApplyVerb(), newIdentityCloneVerb(), runIdentityClone(), termIsStdinTTY(), termIsStdoutTTY(), isTTY(), newIdentityCreateVerb() (+11 more)

### Community 112 - "scan.go"
Cohesion: 0.21
Nodes (17): ScanRegion, UnmanagedHit, ScanSource, lineReferencesAlias(), regionForBlockName(), ScanUnmanagedReferences(), splitKeepLines(), SplitScanRegions() (+9 more)

### Community 113 - "capabilities.go"
Cohesion: 0.21
Nodes (13): BuildProbeDeps(), Capabilities, Deps, Probe(), probeAgent(), probeFIDO(), probeKeychain(), TestCapabilities() (+5 more)

### Community 114 - "resolveHomeForCLI"
Cohesion: 0.28
Nodes (16): newDoctorAliasCmd(), buildIdentityRecords(), matchStrategyFor(), newIdentityListVerb(), newIdentityShowVerb(), renderIdentityList(), renderIdentityShow(), resolveHomeForCLI() (+8 more)

### Community 115 - "captureTUIScreen"
Cohesion: 0.29
Nodes (12): captureTUIScreen(), focusAndNavigateViewport(), navigateFocusedViewport(), runFailure(), runStage1Only(), runStages(), selectReuse(), startCapturePTY() (+4 more)

### Community 116 - ".GlobalGitOptionStates"
Cohesion: 0.21
Nodes (16): bundleAggregateCell(), bundlePerKeyNotes(), globalGitVersionNote(), toGlobalGitOptionState(), SourceClass, classifyOne(), ClassifyWithErrors(), Deps (+8 more)

### Community 117 - "WriteFragment"
Cohesion: 0.25
Nodes (16): gitConfigSet(), gitConfigUnsetAll(), SetAllowedSignersFile(), gitGet(), TestSetAllowedSignersFile(), TestSetAllowedSignersFile_LeadingDashValueIsLiteral(), TestWriteFragment_CreatesParentDir(), TestWriteFragment_LeadingDashValueIsLiteralNotGitOption() (+8 more)

### Community 118 - "Catalog"
Cohesion: 0.25
Nodes (16): Catalog(), Generatable(), AlgoInfo, isHardwareBacked(), ResolveAvailability(), TestCatalog_EntriesCarryMetadata(), TestCatalog_ExactlyOneDefault(), TestCatalog_HasFiveEntries() (+8 more)

### Community 119 - "lifecycle_fallbackauthor_test.go"
Cohesion: 0.21
Nodes (16): extractFuncBody(), fallbackBody(), mustRead(), seedIncludeIf(), TestGitFallbackAuthorLifecycleStagesRow(), TestRunGitFallbackAuthorApply_BothHalves(), TestRunGitFallbackAuthorApply_DistinctFromGlobalGitApply(), TestRunGitFallbackAuthorApply_EmptyPairRemoves() (+8 more)

### Community 120 - "authorresolve.go"
Cohesion: 0.24
Nodes (15): AuthorKeyResolution, DirectoryResolution, MatchedOutcome, getAuthorKey(), Deps, AuthorResolution, isGitConfigUnset(), parseShowOriginGet() (+7 more)

### Community 121 - "reserved_test.go"
Cohesion: 0.35
Nodes (15): includeFixture, applyEveryFix(), block(), blockNames(), containsName(), mustReadFile(), orphanDeps(), removeBlock() (+7 more)

### Community 122 - ".runGlobalSSHApply"
Cohesion: 0.22
Nodes (13): buildGlobalSSHShadowCheck(), Deps, ShadowFinding, TestIsolatedConfigContract(), baseline(), BuildProbeDeps(), effective(), parseResolvedOptions() (+5 more)

### Community 123 - "CheckRedundancy"
Cohesion: 0.30
Nodes (13): globalScan, canonicalDirectiveName(), CheckRedundancy(), scanGlobalDirectives(), makeRedundancyDeps(), TestCheckRedundancy_AdviceNamesCurrentBlock(), TestCheckRedundancy_CleanConfig(), TestCheckRedundancy_EmptyConfig() (+5 more)

### Community 124 - "runIdentityKeyVerb"
Cohesion: 0.21
Nodes (13): outcomeLabel(), realBackend, keyVerbLabel(), printKeyCeremonyDryRun(), runIdentityKeyVerb(), missingFlagErr(), realBackend, runIdentityRegisterKey() (+5 more)

### Community 125 - "ShortSandboxHome"
Cohesion: 0.42
Nodes (14): captureStorageFrame(), FakeMigrateSSHDir(), managedBlockNamesInFile(), seedStorageMigrateHome(), startStoragePTY(), storageSubTabStrip(), TestGlobalSSHStorage_RealPTYBrowse(), TestGlobalSSHStorage_RealPTYChangedSincePreview() (+6 more)

### Community 126 - "EnsureGitFallbackAuthor"
Cohesion: 0.36
Nodes (14): EnsureGitFallbackAuthor(), fallbackBody(), recipeShapedGitconfig(), TestEnsureGitFallbackAuthor_BothEmptyRemoves(), TestEnsureGitFallbackAuthor_BothHalves(), TestEnsureGitFallbackAuthor_EmailOnly(), TestEnsureGitFallbackAuthor_EmptyEmailStillValid(), TestEnsureGitFallbackAuthor_Idempotent() (+6 more)

### Community 127 - "ScanDirectives"
Cohesion: 0.24
Nodes (13): hitsFromContent(), policyKeys(), DirectiveHit, DirectiveSource, ScanDirectives(), ScanDirectivesMulti(), TestScanDirectivesAbsentReturnsEmpty(), TestScanDirectivesCaseInsensitiveKeys() (+5 more)

### Community 128 - "CheckDeps"
Cohesion: 0.26
Nodes (9): Report, CheckDeps(), fakeDepsDeps(), fakeDetectTools(), TestCheckDeps_AllPresent(), TestCheckDeps_MultipleRequiredMissing(), TestCheckDeps_OptionalClipboardMissing(), TestCheckDeps_RequiredMissing() (+1 more)

### Community 129 - "shadow.go"
Cohesion: 0.27
Nodes (13): GraphFile, SimulationGraph, findIncludeLine(), isInsideManagedBlock(), linearise(), resolveIncludeTarget(), rewriteIncludes(), shadowSourceFor() (+5 more)

### Community 130 - "RedactCLIOutput"
Cohesion: 0.24
Nodes (12): ClassifyGHDuplicate(), ClassifyUploadFailure(), RedactCLIOutput(), TestClassifyGHDuplicateRequiresZeroExit(), TestClassifyGLabAlreadyTakenIsAConflictNotSuccess(), TestClassifyNotAuthenticatedForBothProviders(), TestClassifyScopeFailuresByScopeIdentifier(), TestRedactCLIOutputIsSingleLineAndBounded() (+4 more)

### Community 131 - "displayMessages"
Cohesion: 0.33
Nodes (5): displayMessages(), displayPaths(), fillApplyFromResult(), fillMigrateFromResult(), DeleteScopeFrom()

### Community 132 - "identity/provisional_test.go"
Cohesion: 0.35
Nodes (12): provisionalCallArgs, EffectiveAlias(), fakeDepsForProvisional(), callLog, makeCreateInput(), makeStagedKey(), TestDropProvisionalSSH_CallsOnlyDropSeam(), TestEffectiveAlias() (+4 more)

### Community 133 - "CheckPermissions"
Cohesion: 0.35
Nodes (12): CheckPermissions(), containsStr(), findSubstr(), makeMissingStat(), TestCheckPermsCritical(), TestCheckPermsDirError(), TestCheckPermsLooseKeyTightensNotWidens(), TestCheckPermsMissingSkipped() (+4 more)

### Community 134 - "gitid-frame-promote/main.go"
Cohesion: 0.30
Nodes (10): gitHead(), main(), promoteFrames(), repoRoot(), run(), TestPromoteFramesWritesEveryFrameWhenAllCapturesArePresent(), TestPromoteFramesWritesNothingWhenAnyCaptureIsMissing(), writeFrame() (+2 more)

### Community 135 - "update_test.go"
Cohesion: 0.48
Nodes (10): updateCallLog, baseAccount(), newFakeUpdateDeps(), TestUpdate_FragmentOnly(), TestUpdate_NameImmutable(), TestUpdate_SigningOffCallsRemoveAllowedSigners(), TestUpdate_SigningOnCallsWriteAllowedSigners(), TestUpdate_Structural() (+2 more)

### Community 137 - "writeFakeSSH"
Cohesion: 0.24
Nodes (11): captureHomePath(), normalizeCaptureText(), TestCaptureInputsNormalizeOnlyDisposablePrefixes(), TestCaptureTUIScreenConfirmationKeyPathFrame(), TestCaptureTUIScreenConfirmationManagedBlock(), TestCaptureTUIScreenConfirmationManagedBlockFrame(), TestCaptureTUIScreenExactProofFrames(), TestCaptureTUIScreenManualReuseStatesAreDistinct() (+3 more)

### Community 138 - "determinism_test.go"
Cohesion: 0.36
Nodes (9): HashPNG(), StripPNGMetadata(), buildChunk(), fixturePNG(), TestHashPNG_StableForFixedInput(), TestHashPNG_UnaffectedByTimestampMetadata(), TestStripPNGMetadata_Idempotent(), TestStripPNGMetadata_RejectsBadSignature() (+1 more)

### Community 139 - "ReusableKeyView"
Cohesion: 0.29
Nodes (4): providerFromAlias(), toReusableKeyViews(), nonCatalogAlgorithm(), ReusableKeyView

### Community 140 - "CaptureTUI"
Cohesion: 0.31
Nodes (7): TestCaptureTUI(), CaptureTUI(), finalizePNG(), writeGoldenTempFile(), fixtureModel, Result, TUIOptions

### Community 141 - "gitid git JSON schema"
Cohesion: 0.22
Nodes (8): Captured example — a below-gate conflict-style apply, Exit-status contract, `gitid git fallback set --json`, `gitid git fallback show --json`, gitid git JSON schema, `gitid git options apply --json`, `gitid git options list --json`, Option object

### Community 142 - "OptionPolicy"
Cohesion: 0.36
Nodes (5): GateKind, MemberPolicy, OptionPolicy, PolicyForToken(), TokenOwningMember()

### Community 143 - "deps/deps.go"
Cohesion: 0.31
Nodes (7): Detect(), found(), GitVersion(), GitVersionAtLeast(), GitVersionParts(), TestDetectSmoke(), TestMissingRequired()

### Community 144 - "fragment_signing_test.go"
Cohesion: 0.50
Nodes (8): assertContains(), assertNotContains(), gitConfigList(), TestWriteFragment_SigningFalse(), TestWriteFragment_SigningToggleOffToOn(), TestWriteFragment_SigningToggleOnToOff(), TestWriteFragment_SigningTrue(), TestWriteFragment_ValidationStillApplied()

### Community 145 - "CaptureHTML"
Cohesion: 0.33
Nodes (7): TestCaptureHTML(), TestCaptureHTML_OfflineFailurePath(), TestProvisionPinnedChromium(), CaptureHTML(), resolveBrowserBinary(), Result, HTMLOptions

### Community 146 - "gitid"
Cohesion: 0.22
Nodes (7): gitid, Quick Start, Status, What gitid manages (objective), Caveat: structure, not key type, recipes/ — the target shape gitid manages, What the recipes establish (the wiring gitid must reproduce)

### Community 147 - "TestGitJSONOptionsListExactKeySetAndEnums"
Cohesion: 0.39
Nodes (7): TestGitJSONOptionsListExactKeySetAndEnums(), TestGitJSONStateEnumsAreRenderLayerTaxonomy(), jsonObjectKeys(), assertEnumMember(), assertExactKeys(), TestSSHJSONOptionsListExactKeySetAndEnums(), TestSSHJSONStorageShowExactKeySet()

### Community 148 - "DeleteDeps"
Cohesion: 0.32
Nodes (7): collectDeleteBackups(), deleteEverything(), DeleteDeps, DeleteResult, SharedKeyOwners(), TestSharedKeyOwners_MultipleSiblingsSortedOrder(), TestSharedKeyOwners_NoOwnersEmpty()

### Community 149 - "gitid ssh JSON schema"
Cohesion: 0.25
Nodes (7): Exit-status contract, gitid ssh JSON schema, `gitid ssh options apply --json`, `gitid ssh options list --json`, `gitid ssh storage migrate --json`, `gitid ssh storage show --json`, Option object

### Community 150 - "SSHVersion"
Cohesion: 0.39
Nodes (5): SSHVersion, parseSSHVersion(), ProbeSSHVersion(), TestParseSSHVersion(), TestProbeSSHVersionReturnsStruct()

### Community 151 - "BundleFor"
Cohesion: 0.36
Nodes (6): BundleResult, BundleFor(), OptionPolicy, TestBundleForAggregate(), TestBundleForEmptyEffective(), TestBundleForNamesOnlyDiffersMembers()

### Community 152 - "globalssh.go"
Cohesion: 0.43
Nodes (6): TestDummyStorageSubTabGoldenText(), IncludePreviewOwned(), managedHostStar(), SentinelPreview(), gssMode, gssSubTab

### Community 153 - "Copy"
Cohesion: 0.47
Nodes (4): Copy(), TestCopyAvailable(), TestCopyPropagatesOtherErrors(), TestCopyUnavailable()

### Community 154 - "globalgit/isolation_contract_test.go"
Cohesion: 0.40
Nodes (5): initTestRepo(), TestGlobalgitDepsAllowlist(), TestGlobalgitIsolationContract(), TestGlobalgitPackageNeverWrites(), TestGlobalgitProbeZLayout_RealBinary()

### Community 155 - "BinaryInstallInfo"
Cohesion: 0.47
Nodes (4): BinaryInstallInfo(), binaryOnPath(), TestBinaryInstallInfo(), TestBinaryOnPath()

### Community 156 - "TestKeyTypeMapping"
Cohesion: 0.60
Nodes (3): AlgorithmForToken(), SupportedAlgorithms(), TestKeyTypeMapping()

### Community 157 - "gitid CLI parity matrix"
Cohesion: 0.50
Nodes (3): gitid CLI parity matrix, Requirement-keyed outcome matrix, The per-verb `--dry-run` contract (R12-DR)

## Knowledge Gaps
- **143 isolated node(s):** `reusableKeyMat`, `gitFallbackDocument`, `realBackend`, `gitOptionRecord`, `gitApplyDoc` (+138 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 329 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **3 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `realBackend` connect `realBackend` to `newBackendForHome`, `displayMessages`, `charm.land/bubbletea/v2.Cmd`, `stripANSI`, `ParseManagedHosts`, `ReusableKeyView`, `SSHVersion`, `FixtureBackend`, `Write`, `gitconfig/baseline.go`, `wiring.go`, `upload_real_account_gitlab_e2e_test.go`, `ptySession`, `PolicyFor`, `upload_real_account_e2e_test.go`, `CreateSpec`, `archive_test.go`, `validation_test.go`, `.planUpload`, `effectiveProbe`, `CreateInput`, `globalgit/version_test.go`, `GitSpec`, `platform.go`, `.GlobalGitOptionStates`, `authorresolve.go`?**
  _High betweenness centrality (0.062) - this node is a cross-community bridge._
- **Why does `newBackendForHome()` connect `newBackendForHome` to `testing.T`, `lifecycle_test.go`, `realBackend`, `gate_visual_regression_test.go`, `BuildRegionDiffs`, `TestGitJSONOptionsListExactKeySetAndEnums`, `seedDeleteFixture`, `cliTestCmd`, `DemoFinding`, `wiring.go`, `runUploadSpec`, `Finding`, `wiring_storage_test.go`, `git.go`, `ssh.go`, `fix_test.go`, `createInputFromCreateFlags`, `github.com/spf13/cobra.Command`, `identityVerb`, `resolveHomeForCLI`, `lifecycle_fallbackauthor_test.go`, `runIdentityKeyVerb`?**
  _High betweenness centrality (0.033) - this node is a cross-community bridge._
- **Why does `EnsureGlobalGit()` connect `EnsureGlobalGit` to `ListBlocks`, `realBackend`, `WriteFragment`, `gitconfig/baseline.go`, `wiring.go`?**
  _High betweenness centrality (0.028) - this node is a cross-community bridge._
- **Are the 254 inferred relationships involving `newBackendForHome()` (e.g. with `runFix()` and `buildHealthFixerCaptures()`) actually correct?**
  _`newBackendForHome()` has 254 INFERRED edges - model-reasoned connections that need verification._
- **Are the 202 inferred relationships involving `press()` (e.g. with `pressKey()` and `TestCeremonyDestructiveArrowsStayOnTypedInput()`) actually correct?**
  _`press()` has 202 INFERRED edges - model-reasoned connections that need verification._
- **Are the 137 inferred relationships involving `NewApp()` (e.g. with `TestHelpOverlayShowsFullLegend()` and `TestNewAppPrefilledNilBehavesLikeNewApp()`) actually correct?**
  _`NewApp()` has 137 INFERRED edges - model-reasoned connections that need verification._
- **Are the 69 inferred relationships involving `identModel()` (e.g. with `TestCeremonyArrowsMoveButtonFocusNonDestructive()` and `TestCeremonyDestructiveArrowsStayOnTypedInput()`) actually correct?**
  _`identModel()` has 69 INFERRED edges - model-reasoned connections that need verification._