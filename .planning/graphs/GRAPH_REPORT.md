# Graph Report - ssh-git-config  (2026-09-05)

## Corpus Check
- 339 files · ~814,554 words
- Verdict: corpus is large enough that graph structure adds value.

## Summary
- 6033 nodes · 22589 edges · 193 communities (174 shown, 3 thin omitted)
- Extraction: 78% EXTRACTED · 22% INFERRED · 0% AMBIGUOUS · INFERRED: 4930 edges (avg confidence: 0.85)
- Token cost: 0 input · 0 output

## Graph Freshness
- Built from commit: `d2a13bf4`
- Run `git rev-parse HEAD` and compare to check if the graph is stale.
- Run `graphify update .` after code changes (no API cost).

## Community Hubs (Navigation)
- gate_visual_regression_test.go
- stripANSI
- identities.go
- press
- cliTestCmd
- testing.T
- createflow_packet_test.go
- Write
- seedDeleteFixture
- validation.go
- realBackend
- mustSee
- globalssh/validate_test.go
- runFix
- identities_test.go
- App
- globalssh_test.go
- identity_manager_pty_e2e_test.go
- RequiredScreenSpecs
- CaptureTUI
- identitiesModel
- newBackendForHome
- store.go
- ExtractRegion
- .commitGitArtifacts
- DefaultGitignorePatterns
- DemoFinding
- EnsureGlobalGit
- github.com/spf13/cobra.Command
- Backend
- upload_real_account_gitlab_e2e_test.go
- ListBlocks
- shadow_test.go
- assertUnchanged
- identity_cli_e2e_test.go
- GenerateMaterial
- harness_test.go
- doctor_pty_e2e_test.go
- ValidateResolvedConfig
- ExistingKey
- gitid/identity_test.go
- Account
- migrate_test.go
- Statuses
- global_git_pty_e2e_test.go
- gitIgnoreModel
- EnsureCustomGitKey
- wiring_storage_test.go
- mouse_test.go
- buildIdentityDeps
- DeriveCloneInput
- newRootCmd
- Seed
- .displayPath
- delete_test.go
- uploader_test.go
- authorresolve.go
- upload_real_account_e2e_test.go
- CLAUDE.md
- gitid-evidence/main.go
- DemoState
- EnsureGlobals
- state_test.go
- ceremonyModel
- archive_test.go
- identity/identity_test.go
- seedSSHDir
- AllowedSignersLine
- EnsureIncludeLine
- fileExists
- PolicyFor
- wiring.go
- git_configuration_pty_e2e_test.go
- CheckCoherence
- gitconfig/reader_test.go
- gitconfig/baseline.go
- modes_test.go
- gitid Agent Instructions
- ssh.go
- SandboxHome
- tester_test.go
- Reconstruct
- wiring_test.go
- RepairKey
- .planUpload
- CheckBaseline
- SSH/Git Identity Manager — Product Requirements Document (PRD)
- ptySession
- migrate.go
- effectiveProbe
- adopter_test.go
- DetectOverlaps
- realBackend
- Requirements Description
- shadow.go
- signing_test.go
- Execution Phases
- stubBackend
- capabilities.go
- ExitCode
- RenderHostBlock
- Rotate
- Deps
- CreateInput
- io.Writer
- globalgit/version_test.go
- writeJSON
- ScreenSpec
- scan.go
- Tool
- repoRoot
- identityVerb
- OptionRow
- frame.go
- adopt.go
- release_e2e_test.go
- globalgit/policy_test.go
- CurrentOS
- PlanFor
- unlockStoreForIdentity
- buildDeleteDeps
- RenderIncludeIf
- tester.go
- resolveFromBuildInfo
- readRepoFile
- ParseManagedIncludeIf
- .runGlobalSSHApply
- ScanDirectives
- CheckDeps
- update_test.go
- lifecycle_fallbackauthor_test.go
- BuildInventory
- globals.go
- CheckPermissions
- gitid-frame-promote/main.go
- expandTildeForHome
- OptionPolicy
- CheckOrphans
- identity/provisional_test.go
- ExactTextViewport
- resolveHomeForCLI
- gitid git JSON schema
- createInputFromCreateFlags
- reserved_test.go
- startStoragePTY
- global_ssh_cli_e2e_test.go
- gitid
- Match
- EnsureGitFallbackAuthor
- gitid ssh JSON schema
- AllSetKeys
- WriteFragment
- AllDirectives
- DeleteDeps
- TestGitJSONOptionsListExactKeySetAndEnums
- BinaryInstallInfo
- globalssh/probe.go
- gitid CLI parity matrix
- SSHVersion
- github.com/castocolina/gitid
- TestKeyTypeMapping
- install.sh
- e2e-shard.sh
- Catalog
- doctor_screen_test.go
- rewrite.go
- RedactCLIOutput
- CheckRedundancy
- determinism_test.go
- fragment_signing_test.go
- ParseManagedHosts
- .resolveKeyPath
- rewrite_test.go
- validation_test.go
- CaptureHTML
- globalgit/isolation_contract_test.go
- perAliasFromContent

## God Nodes (most connected - your core abstractions)
1. `newBackendForHome()` - 287 edges
2. `press()` - 260 edges
3. `NewApp()` - 182 edges
4. `appView()` - 169 edges
5. `realBackend` - 148 edges
6. `identModel()` - 143 edges
7. `SandboxHome()` - 139 edges
8. `mustSee()` - 128 edges
9. `pressSeq()` - 116 edges
10. `stripANSI()` - 112 edges

## Surprising Connections (you probably didn't know these)
- `healthDocument` --references--> `DemoFinding`  [EXTRACTED]
  cmd/gitid/health.go → internal/tuikit/store.go
- `gitignorePairFinding()` --calls--> `fixExcludesfile()`  [INFERRED]
  internal/doctor/checks/baseline.go → cmd/gitid/wiring.go
- `TestGlobalSSHToggleAndClickRespectSelectability()` --calls--> `assertUnchanged()`  [INFERRED]
  internal/tuikit/globalssh_test.go → cmd/gitid/wiring_test.go
- `runCommitCreate()` --references--> `WizardCommitMsg`  [EXTRACTED]
  cmd/gitid/wiring_test.go → internal/tuikit/views.go
- `main()` --calls--> `NewApp()`  [EXTRACTED]
  cmd/gitid-dummy/main.go → internal/tuikit/app.go

## Import Cycles
- None detected.

## Communities (193 total, 3 thin omitted)

### Community 0 - "gate_visual_regression_test.go"
Cohesion: 0.11
Nodes (82): assertAllComparableEqualRegionsAreMutationSensitive(), assertFrameProvenanceMatches(), buildDoctorCaptures(), buildUploadCaptures(), deterministicGitIdentityFixture(), deterministicGitIgnoreFixture(), deterministicGlobalGitFixture(), deterministicGlobalSSHFixture() (+74 more)

### Community 1 - "stripANSI"
Cohesion: 0.06
Nodes (66): asyncCeremony(), destructiveCeremony(), plainCeremony(), TestCeremonyAsyncConfirmShowsNoReceiptUntilSuccess(), TestCeremonyAsyncFailureIsVisibleAndRetryable(), TestCeremonyDestructiveAffirmativeNeverDefaultFocused(), TestCeremonyDestructiveConfirmWordCanContainViewportKey(), TestCeremonyDestructiveGatesOnTypedWord() (+58 more)

### Community 2 - "identities.go"
Cohesion: 0.04
Nodes (58): AlgorithmCatalogEntry, algoDisabled(), atoiSafe(), blockedForwardNote(), collisionTargetFor(), compactIncludeIfPreview(), containsString(), deleteCeremonyFor() (+50 more)

### Community 3 - "press"
Cohesion: 0.05
Nodes (140): NewApp(), NewAppPrefilled(), appView(), press(), TestHelpOverlayShowsFullLegend(), TestNewAppPrefilledNilBehavesLikeNewApp(), TestNewAppPrefilledOpensWizardPrefilled(), TestNewAppPrefilledReuseKeyPopulatesPicker() (+132 more)

### Community 4 - "cliTestCmd"
Cohesion: 0.10
Nodes (55): assertGitApplyEnvelope(), assertGitFallbackSetEnvelope(), captureGitApplyJSON(), captureGitFallbackSetJSON(), captureGitFallbackShowJSON(), captureGitOptionsListJSON(), gitEnvHome(), TestGitApplyAdvisoryExitZeroByDefaultNonZeroWithOptIn() (+47 more)

### Community 5 - "testing.T"
Cohesion: 0.04
Nodes (100): TestDemoAppConstructsAndRenders(), testing.T, TestRunUploadForIdentityOmitsForNonGatedHost(), TestRunUploadForIdentityQuotesTitle(), repoRootForAllowlistTest(), TestNoBackendAllowlist(), TestWritableToHostStar(), TestProviderMarker_RoundTripStable() (+92 more)

### Community 6 - "createflow_packet_test.go"
Cohesion: 0.05
Nodes (101): canonicalSemanticPacket(), compareInventories(), generateCandidate(), loadReviewInput(), runCandidateProcess(), TestCanonicalManifestIgnoresRawPTYTranscriptVariation(), writeCanonicalManifest(), approvedHTMLRoutesInternal() (+93 more)

### Community 7 - "Write"
Cohesion: 0.06
Nodes (27): cohFileInfo, fakeFileInfo, orphFileInfo, writeFile2(), os.FileMode, time.Time, TestCopyFile(), atomicReplace() (+19 more)

### Community 8 - "seedDeleteFixture"
Cohesion: 0.06
Nodes (64): customDirectiveVerifyAdvisories(), archivedPrivatePath(), archiveEntriesOrZero(), containsStage(), fakeResolveDeps(), forEachVerb(), realBackend, groupHermeticBackend() (+56 more)

### Community 9 - "validation.go"
Cohesion: 0.13
Nodes (19): isValidationError(), TestValidateHostBlockAcceptsSafeIdentityFilePaths(), TestValidateHostBlockIdentityFileRejectedAtRenderBoundary(), TestValidateHostBlockRejectsUnsafeIdentityFileTokens(), TestGlobalsSharedMatcherNotReimplemented(), aliasCollides(), AliasCollision(), globMatch() (+11 more)

### Community 10 - "realBackend"
Cohesion: 0.06
Nodes (16): displayMessages(), displayPaths(), applyConvergenceAlarms(), atoiOr(), realBackend, TestToTestResultViewMapsEveryOutcome(), toTestResultView(), DefaultPort() (+8 more)

### Community 11 - "mustSee"
Cohesion: 0.16
Nodes (77): delayedResolutionSSHDir(), newRealCreateFlowCmd(), openCreateWizard(), quitCleanly(), requireFocusedProof(), seedEncryptedKeyFixture(), tabKeys(), TestCreateFlow_AlgorithmAvailability() (+69 more)

### Community 12 - "globalssh/validate_test.go"
Cohesion: 0.09
Nodes (42): DirectiveProof, fakeCombinedRunner, fakeStdoutRunner, Deps, hasHostStarLine(), IsStructuralDirectiveName(), offendingDirectiveName(), ProveCustomDirective() (+34 more)

### Community 13 - "runFix"
Cohesion: 0.20
Nodes (17): TestDoctorAliasFixWithoutYesPrompts(), confirmFix(), firstFixable(), newFixCmd(), runFix(), runFixCommand(), scanForFix(), seedInstalledBaseline() (+9 more)

### Community 14 - "identities_test.go"
Cohesion: 0.04
Nodes (162): stubDefaultDeletePlan(), TestCeremonyArrowsMoveButtonFocusNonDestructive(), TestCeremonyDestructiveArrowsStayOnTypedInput(), TestCeremonyTabRingAndEnterActivatesFocused(), TestCloneFocusRingInputToButton(), TestDeleteScopeRingTabAndArrows(), TestEditSSHFocusRingReachesRewriteButton(), TestMouseCeremonyButtonsCancelConfirmDone() (+154 more)

### Community 15 - "App"
Cohesion: 0.05
Nodes (77): NewAppOnGlobalGit(), TestNewAppOnGlobalGitOpensEmptySelection(), pressKey(), newIdentitiesModel(), openKeyCeremonyAtReview(), pressAndRun(), TestConfigureGitNameEmailStrategyReduceExactCommittedSpec(), TestConfigureGitReducesExactCommittedSpec() (+69 more)

### Community 16 - "globalssh_test.go"
Cohesion: 0.06
Nodes (71): NewAppOnGlobalSSH(), TestNewAppOnGlobalSSHOpensEmptyOptionsAndStorage(), TestOptionRowNowValueClipsWithEllipsis(), deliverMsg(), TestGlobalSSHCeremonyPreviewUsesUnchangedDefaultBudget(), newGlobalSSHModel(), notApplicableSentence(), optionRow() (+63 more)

### Community 17 - "identity_manager_pty_e2e_test.go"
Cohesion: 0.11
Nodes (54): compareCreateFlowCheckpoint(), loadCreateFlowAllowlist(), createFlowAllowlistEntry, errorRecorder, fakeErrorRecorder, FakeGHTrackAddedKeys(), allIdentManagerRegions(), assertManifestFields() (+46 more)

### Community 18 - "RequiredScreenSpecs"
Cohesion: 0.08
Nodes (48): main(), TestCandidateUsesOnlyTUISurfaces(), TestCaptureTUIScreenReuseSelection(), predicateMatches(), TestDoctorHTMLNonApplicabilityPerSpec(), TestGlobalGitHTMLNonApplicabilityPerSpec(), TestGlobalSSHRegistryFrameCountIncrease(), TestNegativeControl_CrossRegistryLeakage() (+40 more)

### Community 19 - "CaptureTUI"
Cohesion: 0.22
Nodes (8): charm.land/bubbletea/v2.View, TestCaptureTUI(), CaptureTUI(), finalizePNG(), writeGoldenTempFile(), fixtureModel, Result, TUIOptions

### Community 20 - "identitiesModel"
Cohesion: 0.09
Nodes (27): charm.land/bubbletea/v2.KeyMsg, charm.land/bubbletea/v2.Msg, mustKey(), synthKey(), ceremonyClickKey(), blockLine(), hitNeedle(), actionMenuLabels() (+19 more)

### Community 21 - "newBackendForHome"
Cohesion: 0.04
Nodes (64): TestCloneCeremonyInputsFingerprintsMatchTestStage(), TestCustomGitKeyPlanDoesNotPromiseABackupForAWriteTheWriterWillSkip(), TestCustomGitKeyPlanShowsRealDiff(), TestRunCustomGitKeyWriteIsIdempotent(), TestRunCustomGitKeyWriteLandsBothWrites(), extractManagedBlock(), readBaselineConflictstyle(), runRealGitConfig() (+56 more)

### Community 22 - "store.go"
Cohesion: 0.04
Nodes (36): storageToWire(), wireToStorage(), go/ast.Expr, fixtureSSHStorageView(), mutuallyExclusiveSSHStoragePlanFn(), AllActions(), cloneState(), CountFindings() (+28 more)

### Community 23 - "ExtractRegion"
Cohesion: 0.07
Nodes (71): markerInPane(), normalizeCapturedStateText(), ansiOffsetToRaw(), applyHeadingRegion(), bodyFromAnchorToEnd(), ceremonyBodyAfter(), extractActionMenuRows(), extractBackupPathList() (+63 more)

### Community 24 - ".commitGitArtifacts"
Cohesion: 0.21
Nodes (6): containedRegularPath(), matchesFor(), TestMatchesForUsesExactSSHHostAndGitDir(), gitDirSnapshot, gitFileSnapshot, mutationJournal

### Community 25 - "DefaultGitignorePatterns"
Cohesion: 0.13
Nodes (34): commitGitIgnore(), seedGitIgnoreFile(), seedManagedBaseline(), snapshotHomeRecursive(), TestGlobalGitIgnoreApplyPlanNoBaselineBlock(), TestGlobalGitIgnoreApplyPlanSentinelErrorIsTranslated(), TestGlobalGitIgnoreApplyPlanUnsetIsTwoTarget(), TestGlobalGitIgnoreApplyPlanWiredIsSingleTarget() (+26 more)

### Community 26 - "DemoFinding"
Cohesion: 0.13
Nodes (21): IdentityManagerRow, HealthFindingByID(), TestFixtureBackendSatisfiesIdentityPlannerThroughNoop(), TestFixtureConsistency(), HealthFinding, exactFinding(), groupFindings(), orderedFindings() (+13 more)

### Community 27 - "EnsureGlobalGit"
Cohesion: 0.14
Nodes (26): sectionKeyValue, belongsToKnownSection(), EnsureGlobalGit(), existingGlobalGitBody(), keysForSection(), parseGlobalGitBody(), renderGlobalGitBody(), fullTableSelection() (+18 more)

### Community 28 - "github.com/spf13/cobra.Command"
Cohesion: 0.11
Nodes (43): fillGitApplyFromResult(), fillGitFallbackSetFromResult(), finishGitApply(), finishGitFallbackSet(), gitApplyTokenHelp(), gitApplyTokens(), gitOptionRecords(), gitRowStateName() (+35 more)

### Community 29 - "Backend"
Cohesion: 0.13
Nodes (52): charm.land/bubbletea/v2.Model, anyView(), CaptureDoctorScreens(), CaptureGitIgnoreScreens(), CaptureGitScreenScreens(), CaptureGlobalGitScreens(), CaptureGlobalSSHScreens(), CaptureIdentityManagerScreens() (+44 more)

### Community 30 - "upload_real_account_gitlab_e2e_test.go"
Cohesion: 0.11
Nodes (38): fakeGLabInventoryRecord, glabArgvRecorder, glabTokenSelf, outstandingGLabRemoteKeys, recordedGLabArgv, recordedGLabRemoteKey, assertNoGLabTokenRevealingFlag(), countUnscopedGLab() (+30 more)

### Community 31 - "ListBlocks"
Cohesion: 0.09
Nodes (35): TestInsertBlockAfter_CRLFAnchor(), TestInsertBlockAfter_MissingAnchor(), TestInsertBlockAfter_PlacesImmediatelyAfterAnchor(), TestInsertBlockAfter_UpdateInPlace(), InsertBlockAfter(), TestListBlocks_CRLFNormalized(), TestListBlocks_Empty(), TestListBlocks_ForeignContentPreservedInOutput() (+27 more)

### Community 32 - "shadow_test.go"
Cohesion: 0.19
Nodes (43): BuildGraph(), Simulate(), badOutput(), buildDeps(), deriveAnswerFromConfig(), Deps, managedBlockFor(), recommendedOutput() (+35 more)

### Community 33 - "assertUnchanged"
Cohesion: 0.10
Nodes (41): managedFixturePaths(), TestIdentityDeleteUnauthorizedNoYesLeavesBytesIdentical(), TestIdentityDeleteWithYesCompletesAndDryRunWritesNothing(), TestIdentityKeyVerbUnauthorizedNoYesLeavesBytesIdentical(), TestCustomGitKeyLifecycleStagesRow(), TestCustomGitKeyPlanRejectsMalformedKeyBeforeAnyWrite(), TestCustomGitKeyPlanRejectsPolicyManagedKey(), TestRunCustomGitKeyWriteDryRunNeverConfirms() (+33 more)

### Community 34 - "identity_cli_e2e_test.go"
Cohesion: 0.12
Nodes (41): seedGitPTYIdentity(), runHealthCLI(), TestHealthFixCLIParity(), healthDoc, archivePrivateFor(), assertManagedArtifactsEqual(), assertReferentialCoherence(), assertReferentialCoherenceFails() (+33 more)

### Community 35 - "GenerateMaterial"
Cohesion: 0.06
Nodes (60): TestSmokeNetworkConnectivity(), golang.org/x/crypto/ssh.PublicKey, DerivePublicKey(), TestDerivePublicKeyMissingFile(), TestDerivePublicKeyRejectsGarbage(), TestDerivePublicKeyRoundTrips(), generateEd25519(), GenerateMaterial() (+52 more)

### Community 36 - "harness_test.go"
Cohesion: 0.13
Nodes (42): TestCreateFlow_ExistingPTYCannotReachRealProviderCLI(), seedDoctorBatch(), e2eT, ambientPathSentinelHit(), e2eEnv(), envValue(), FakeGHInventoryFile(), FakeGitDir() (+34 more)

### Community 37 - "doctor_pty_e2e_test.go"
Cohesion: 0.27
Nodes (17): confirmNonDestructiveFix(), makeImmutable(), openDoctor(), seedDoctorFlagship(), seedDoctorGreen(), startDoctorPTY(), startDoctorPTYWithEnv(), startEphemeralSSHAgent() (+9 more)

### Community 38 - "ValidateResolvedConfig"
Cohesion: 0.12
Nodes (21): TestResolvedViaGCommandExists(), TestStage2FieldValidationFailsOnMismatch(), TestStage2ProofCarriesBothCommandsAndOutputs(), TestStage2ValidationAcceptsCorrectFields(), TestStage2ValidationRejectsEmptyResolutionOutput(), TestStage2ValidationRejectsWrongIdentityFileOrder(), TestValidateResolvedConfigExists(), functionBody() (+13 more)

### Community 39 - "ExistingKey"
Cohesion: 0.11
Nodes (39): deleteCandidatesDetail(), deleteCandidatesManualCommand(), encodeDeleteCandidates(), providerToolName(), encoding/json.Number, decodeProviderKeyPages(), DeleteRecordedKey(), FindByTitle() (+31 more)

### Community 40 - "gitid/identity_test.go"
Cohesion: 0.06
Nodes (61): TestGitOptionsApplyBelowGateAdvisoryOriginatesFromCeremony(), TestGitWriteVerbsUseSharedDepthHelpersByConstruction(), runIdentityClone(), confirmDelete(), realBackend, printDeleteDryRun(), renderDeletePlan(), runIdentityDelete() (+53 more)

### Community 41 - "Account"
Cohesion: 0.18
Nodes (29): realBackend, DeleteTarget, PlanDeps, DeleteScope, deleteTargets(), DeletePlan, nonEmptyStrings(), PlanDelete() (+21 more)

### Community 42 - "migrate_test.go"
Cohesion: 0.23
Nodes (38): Migrate(), RealMigrateDeps(), assertGlobalsBlockLast(), containsBlockName(), fakeDepsForMutate(), migrateFixture(), mustReadFile(), parseIdentityFiles() (+30 more)

### Community 43 - "Statuses"
Cohesion: 0.16
Nodes (31): firstErr(), Deps, NotApplicableReason, OptionState, OptionStatus, SourceClass, OptionPolicy, stateFor() (+23 more)

### Community 44 - "global_git_pty_e2e_test.go"
Cohesion: 0.14
Nodes (44): gitApplyDoc, gitFallbackDoc, gitFallbackSetDoc, gitOptionRecord, gitOptionsDoc, containsSubstring(), runGitCLI(), TestGlobalGitCLI_BelowGateAdvisoryExitCodes() (+36 more)

### Community 45 - "gitIgnoreModel"
Cohesion: 0.10
Nodes (14): charm.land/bubbles/v2/textarea.Model, maxInt(), GitIgnoreReceiptWrongTarget(), GitIgnoreWiringPointsElsewhere(), ValidationError, ManagedBlockSentinels(), TestFrozenGitIgnoreCopy(), ensureGitIgnoreGlyph() (+6 more)

### Community 46 - "EnsureCustomGitKey"
Cohesion: 0.15
Nodes (26): CustomKey, EnsureCustomGitKey(), GitKeysEqual(), gitKeysEqual(), ParseCustomKeysBlock(), RenderCustomKeysBlock(), SplitGitKey(), TestEnsureCustomGitKeyDropsAnUnrenderableEntryInsteadOfFailingTheWholeWrite() (+18 more)

### Community 47 - "wiring_storage_test.go"
Cohesion: 0.19
Nodes (35): assertNoDualPresence(), backendWithFakeSSH(), buildFakeSSHForMigration(), realBackend, hasBlock(), runStorageCommit(), seedMigrateHome(), seedMigrateHomeInclude() (+27 more)

### Community 48 - "mouse_test.go"
Cohesion: 0.19
Nodes (13): TestSubTabStripClickHitTestMatchesRenderedSpans(), sidebarWidth(), clickAt(), gitModelOf(), gssModelOf(), TestMouseClickBetweenHeaderTargetsIsInert(), TestMouseGlobalGitOptionRowSelects(), TestMouseGlobalSSHSubTabsAndOptionRows() (+5 more)

### Community 49 - "buildIdentityDeps"
Cohesion: 0.10
Nodes (38): buildIdentityDeps(), NamedBlock, listBlocksWith(), ListProvisionalBlocks(), RemoveProvisionalBlock(), ReplaceProvisionalBlock(), TestListProvisionalBlocks_Empty(), TestListProvisionalBlocks_MutualExclusion() (+30 more)

### Community 50 - "DeriveCloneInput"
Cohesion: 0.14
Nodes (32): CloneNotices, CloneTargets, FieldError, DeriveCloneInput(), deriveCloneMatches(), GitdirMatch(), HasconfigMatch(), nameTaken() (+24 more)

### Community 51 - "newRootCmd"
Cohesion: 0.07
Nodes (40): assertCompletionScript(), TestCompletionBash(), TestCompletionDynamicListIncludesIdentity(), TestCompletionFish(), TestCompletionZsh(), TestDoctorAliasFixForwardsFlagsUnchanged(), TestDoctorAliasMatchesHealthAndIsHidden(), runnableGitPaths() (+32 more)

### Community 52 - "Seed"
Cohesion: 0.11
Nodes (43): Seed(), newGlobalGitModel(), TestCustomKeyCeremonyConfirmUsesTheSubmittedSnapshotNotTheLiveInputFields(), TestCustomKeyStaleCommitMessageIsNotMisattributed(), TestGitAbandonedFailedCommitStillProducesNote(), TestGitFallbackAbandonedApplyStillDispatchesReducerAction(), TestGitFallbackInputsSeededFromStateView(), TestGitRowBudgetHeightFloorsAtMinFrameHeight() (+35 more)

### Community 53 - ".displayPath"
Cohesion: 0.22
Nodes (13): fixExcludesfile(), gitIgnorePreimageToken(), patchExcludesfileInBaselineBody(), TestGitIgnorePreimageTokenIsDomainSeparated(), translateGitIgnoreErr(), ManagedBlockShape, InspectGitignoreFile(), InspectManagedBlockFile() (+5 more)

### Community 54 - "delete_test.go"
Cohesion: 0.24
Nodes (33): deleteCallLog, Delete(), baseDeleteAccount(), containsBlock(), containsLine(), containsStr(), fatalOnInvokeSSHDeps(), gcFixtureWithBlocks() (+25 more)

### Community 55 - "uploader_test.go"
Cohesion: 0.07
Nodes (52): providerDisplayName(), sync.Mutex, AuthCheck(), buildArgs(), CommandPreview(), DetectFor(), RegistrationRequest, RegistrationResult (+44 more)

### Community 56 - "authorresolve.go"
Cohesion: 0.22
Nodes (16): AuthorKeyResolution, DirectoryResolution, MatchedOutcome, getAuthorKey(), Deps, AuthorResolution, isGitConfigUnset(), parseShowOriginGet() (+8 more)

### Community 57 - "upload_real_account_e2e_test.go"
Cohesion: 0.14
Nodes (30): argvRecorder, outstandingRemoteKeys, recordedArgv, recordedRemoteKey, ambientEnvMap(), countUnscoped(), deleteRecordedRemoteKey(), drainOutstandingRemoteKeys() (+22 more)

### Community 58 - "CLAUDE.md"
Cohesion: 0.06
Nodes (31): 1. `~/.ssh/config` parsing: kevinburke/ssh_config, 2. `~/.gitconfig` parsing and writing, 3. Ed25519 key generation + OpenSSH formatting + `allowed_signers`, 4. Cobra + shell completion, 5. Quality toolchain, Alternatives Considered, Area-by-Area Rationale, BEGIN gitid managed: <identity-name> (+23 more)

### Community 59 - "gitid-evidence/main.go"
Cohesion: 0.06
Nodes (71): buildEvidenceJSON(), captureApprovedHTMLPanels(), captureApprovedTUIPanels(), captureCommandEnvironment(), captureHomePath(), captureLivePanels(), captureTUIPanels(), captureTUIScreen() (+63 more)

### Community 60 - "DemoState"
Cohesion: 0.04
Nodes (71): charm.land/bubbles/v2/textinput.Model, TestDummyStorageSubTabGoldenText(), padRight(), fixtureGlobalGitOptionViews(), fitPane(), frameBodyRows(), joinMasterDetail(), masterListWidth() (+63 more)

### Community 61 - "EnsureGlobals"
Cohesion: 0.26
Nodes (22): EnsureGlobals(), assertGlobalsLastAfter(), globalBody(), lastBlockName(), managedTestBlock(), TestEnsureGlobalsAdoptsLegacyBlock(), TestEnsureGlobalsAppendsUnrecognisedKey(), TestEnsureGlobalsExistingValueBeatsDefault() (+14 more)

### Community 62 - "state_test.go"
Cohesion: 0.11
Nodes (28): collapseState(), classifyCase, KeyAction, Severity, Inventory, Classify(), ClassifyState(), crossReferenceUnusedKeys() (+20 more)

### Community 63 - "ceremonyModel"
Cohesion: 0.08
Nodes (17): findStubRow(), stubDefaultKeyCeremonyPlan(), stubNameTaken(), newCeremony(), renderReceiptList(), archivePathForKeyCeremony(), keyCeremonyFor(), GitCustomKeyPlanView (+9 more)

### Community 64 - "archive_test.go"
Cohesion: 0.17
Nodes (26): archiveDestPath(), copyExclusive(), CopyKeyPairToArchive(), CreatedFunc, MoveKeyPairToArchive(), prepareArchiveDir(), RemoveArchivedPair(), assertFileAbsent() (+18 more)

### Community 65 - "identity/identity_test.go"
Cohesion: 0.19
Nodes (30): orderRecorder, Create(), assertOrder(), Deps, callLog, newFakeDeps(), newOrderRecordingDeps(), newSplitDeps() (+22 more)

### Community 66 - "seedSSHDir"
Cohesion: 0.08
Nodes (64): TestBaselineGitignoreFixPreservesOtherBaselineSettings(), TestJournalRecordCreatedDirDisjointness(), TestJournalRestoreRemovesCreatedPathsProvesR208(), TestJournalRestoreRestoresWatchedBytesAndModeProvesR21(), TestRotateAndRepairWatchPathsIncludeSSHConfigPath(), doctorFindings(), newMutationJournal(), managedBlock() (+56 more)

### Community 67 - "AllowedSignersLine"
Cohesion: 0.19
Nodes (25): doctorAddWiring(), AllowedSignersLine(), AppendAllowedSigners(), mustLine(), pubLine(), readFile(), TestAllowedSignersLine(), TestAllowedSignersLine_RejectsCommaPrincipalInjection() (+17 more)

### Community 68 - "EnsureIncludeLine"
Cohesion: 0.12
Nodes (25): github.com/kevinburke/ssh_config.Config, EnsureIncludeDir(), EnsureIncludeLine(), IsReservedBlockName(), ReservedPaths(), containsName(), mustRead(), seedIncludeLayout() (+17 more)

### Community 69 - "fileExists"
Cohesion: 0.12
Nodes (16): TestRunCustomSSHDirectiveWriteFloorsTheIncludeLineWhenNeeded(), TestRunGlobalSSHApplyFreshHomeRemovesCreatedConfigOnFailure(), containsLine(), fileExists(), globalsBodyText(), globalsTextDiff(), planTokenFor(), splitLines() (+8 more)

### Community 70 - "PolicyFor"
Cohesion: 0.27
Nodes (17): TestClassify_BundleNeedsActionWhenOneUnset(), Classify(), TestClassify_AlreadySet_FromUnset(), TestClassify_AttributionByOriginPathNotMembership(), TestClassify_BundleAggregate(), TestClassify_BundleAllSetAndEqualIsAlreadySet(), TestClassify_BundleAllSetButSomeDifferIsSetButDiffers(), TestClassify_NeedsAction_WhenUnsetEverywhere() (+9 more)

### Community 71 - "wiring.go"
Cohesion: 0.11
Nodes (16): buildDoctorDeps(), bundleAggregateCell(), bundlePerKeyNotes(), filterReservedDoctorKeyPaths(), findingStableID(), globalGitVersionNote(), injectRepairFailures(), injectRotateFailures() (+8 more)

### Community 72 - "git_configuration_pty_e2e_test.go"
Cohesion: 0.17
Nodes (29): allGitScreenRegions(), assertGitBytesUnchanged(), compareGitScreenCheckpoint(), extractGitScreenCeremony(), extractGitScreenFormFields(), extractGitScreenHeaderStatus(), extractGitScreenPreview(), extractGitScreenRegion() (+21 more)

### Community 73 - "CheckCoherence"
Cohesion: 0.29
Nodes (29): CheckCoherence(), boolPtr(), cohContains(), cohStat(), cohTitles(), makeAccount(), signerLineFor(), TestCheckCoherenceAuthorResolution() (+21 more)

### Community 74 - "gitconfig/reader_test.go"
Cohesion: 0.12
Nodes (28): TestBaselineGitignoreFixViaCLI(), TestRunCustomGitKeyWriteVerifyIsolatedFromAmbientBrokenRepo(), conditionToMatch(), IncludeIfInfo, nonRepoIsolationEnv(), parseIncludeIfBody(), ReadFragment(), RemoveAllowedSignersBlock() (+20 more)

### Community 75 - "gitconfig/baseline.go"
Cohesion: 0.12
Nodes (30): gitIgnoreMalformedReasonText(), BaselineConfig, ManagedBlockError, SentinelLineError, URLRewrite, DefaultBaselineConfig(), DefaultURLRewrites(), BaselineState (+22 more)

### Community 76 - "modes_test.go"
Cohesion: 0.19
Nodes (25): modeLog, TestAddAccountNoPersistKey(), TestReuseNoPersistKey(), AddAccount(), ensurePubReadOnly(), fragmentPathFor(), Deps, Reuse() (+17 more)

### Community 77 - "gitid Agent Instructions"
Cohesion: 0.07
Nodes (23): Code Exploration, Commands, Detailed Guidance, gitid Agent Instructions, Non-Negotiable Rules, Required Start, UI Reference, Avoid (+15 more)

### Community 78 - "ssh.go"
Cohesion: 0.12
Nodes (28): buildSSHStorageDocument(), fillApplyFromResult(), fillMigrateFromResult(), realBackend, newApplyEnvelope(), newMigrateEnvelope(), newSSHCmd(), newSSHOptionsCmd() (+20 more)

### Community 79 - "SandboxHome"
Cohesion: 0.30
Nodes (27): appendGitIgnoreLine(), applyGitIgnoreAndConfirm(), assertGitIgnoreFilesUnchanged(), captureGitIgnoreFrame(), newGitIgnoreCmd(), seedEditableGitIgnoreHome(), seedGitIgnoreHome(), snapshotGitIgnoreFiles() (+19 more)

### Community 80 - "tester_test.go"
Cohesion: 0.15
Nodes (20): ClassifyPreWrite(), preWriteArgs(), PreWriteCommand(), preWriteWith(), fakeSSHForResolvedOutput(), TestClassifyPreWrite_FailureConnectionRefused(), TestClassifyPreWrite_FailureDNSAndTimeout(), TestClassifyPreWrite_IgnoresExitCode() (+12 more)

### Community 81 - "Reconstruct"
Cohesion: 0.15
Nodes (29): DefaultHostname(), TestDefaultHostname(), ProviderHostForSSHHostname(), ProviderKeyForHost(), ProviderRefCount(), Reconstruct(), RewriteProviderKey(), buildGCBlock() (+21 more)

### Community 82 - "wiring_test.go"
Cohesion: 0.05
Nodes (79): TestFixCmdDryRunWritesNothing(), TestBuildTUIDepsWiresSSHCustomDirectivePlanner(), appliedGlobalSSHKeys(), buildBackend(), buildUploaderDeps(), countBackupSiblings(), countPhase(), fakeUploaderRunUploadDeps() (+71 more)

### Community 83 - "RepairKey"
Cohesion: 0.24
Nodes (21): repairLog, Deps, keyDirFor(), repairInput(), RepairKey(), RepairKeyPath(), callLog, Deps (+13 more)

### Community 84 - ".planUpload"
Cohesion: 0.10
Nodes (23): decodeDeleteCandidates(), realBackend, printUploadOutcome(), registrationOf(), TestPlanUploadReadFileFailureRedactsHomePath(), TestPrintUploadOutcomeMatchesTheWizardSection(), TestPrintUploadOutcomeRendersEachSection(), TestPrintUploadOutcomeUsesOnlyFrozenCopy() (+15 more)

### Community 85 - "CheckBaseline"
Cohesion: 0.21
Nodes (25): blockIsPopulated(), CheckBaseline(), excludesFileExists(), pointsToManagedTarget(), setDiffersFindings(), fakeBaselineDeps(), fullyInstalledState(), TestBaselineAllPass() (+17 more)

### Community 86 - "SSH/Git Identity Manager — Product Requirements Document (PRD)"
Cohesion: 0.07
Nodes (26): 1. Background, 2. Domain model, 3. Source of truth & safe writes, 4.1 Identity / Account / Credential CRUD, 4.2 Two-phase test flow (input & output shown), 4.3 Clipboard, 4.4 Upload instructions, 4.5 Doctor (health checks) (+18 more)

### Community 87 - "ptySession"
Cohesion: 0.12
Nodes (43): clickLabelRow(), newDummyCreateFlowCmd(), mustSeeTimeout(), assertOptionColumnsAligned(), captureGlobalSSHFrame(), clickOptionToggle(), globalSSHSubTabStrip(), mustNotContainGlobalSSH() (+35 more)

### Community 88 - "migrate.go"
Cohesion: 0.17
Nodes (26): IsGlobalBlockName(), blockBodyMap(), buildMigrationDiff(), callStep(), checkDigestMatch(), composeDestination(), composeSource(), contentDigest() (+18 more)

### Community 89 - "effectiveProbe"
Cohesion: 0.13
Nodes (25): buildAuthorResolutionCheck(), missingFileError, os/exec.ExitError, TestGlobalgitProbeZLayout_RealBinary(), BuildProbeDeps(), effectiveProbe(), Deps, inFileProbe() (+17 more)

### Community 90 - "adopter_test.go"
Cohesion: 0.16
Nodes (22): AdoptMethod, AdoptResult, fakeAdopterDeps, Adopt(), Deps, ListCandidates(), ListCandidatesFromHome(), MatchIdentityName() (+14 more)

### Community 91 - "DetectOverlaps"
Cohesion: 0.22
Nodes (18): OverlapPair, CheckOverlap(), classifyGitdirOverlap(), DetectOverlaps(), extractGitHost(), hasconfigOverlaps(), normGitdir(), makeGitdirAccount() (+10 more)

### Community 92 - "realBackend"
Cohesion: 0.15
Nodes (10): collectCreateBackups(), fallbackAuthorPreview(), findGitWorkTree(), realBackend, isGitWorkTree(), originNamesFile(), collectDeleteBackups(), injectDeleteFailures() (+2 more)

### Community 93 - "Requirements Description"
Cohesion: 0.08
Nodes (24): Acceptance Criteria, Assumptions carried (confirm if wrong) — the 100→110 polish, Background, Constraints, Design Decisions, Detailed Requirements, Edge Cases, Execution Phases (+16 more)

### Community 94 - "shadow.go"
Cohesion: 0.20
Nodes (17): buildGlobalSSHShadowCheck(), TestGlobalSSHShadowCheckRealWiring_FreshHome(), GraphFile, ShadowFinding, SimulationGraph, findIncludeLine(), ShadowResult, isInsideManagedBlock() (+9 more)

### Community 95 - "signing_test.go"
Cohesion: 0.17
Nodes (22): agentState, os.FileInfo, CheckAgent(), CheckSigning(), classifyAgentState(), extractFingerprint(), isKeyLoaded(), fakeStatFails() (+14 more)

### Community 96 - "Execution Phases"
Cohesion: 0.08
Nodes (23): Acceptance Criteria, Background, Constraints, Design Decisions, Detailed Requirements, Execution Phases, Feature Overview, Functional (+15 more)

### Community 97 - "stubBackend"
Cohesion: 0.02
Nodes (69): printGitApplyDryRun(), printGitFallbackDryRun(), printApplyDryRun(), toGlobalSSHOptionState(), charm.land/bubbletea/v2.Cmd, findFixtureRow(), fixtureNameTaken(), fixtureStageCmd() (+61 more)

### Community 98 - "capabilities.go"
Cohesion: 0.21
Nodes (13): BuildProbeDeps(), Capabilities, Deps, Probe(), probeAgent(), probeFIDO(), probeKeychain(), TestCapabilities() (+5 more)

### Community 99 - "ExitCode"
Cohesion: 0.17
Nodes (16): ExitCode(), Families(), TestCheckRedundancySignature(), TestExitCodeClean(), TestExitCodeCritical(), TestExitCodeCriticalBothTiers(), TestExitCodeError(), TestExitCodeHighestWins() (+8 more)

### Community 100 - "RenderHostBlock"
Cohesion: 0.15
Nodes (20): ioDiscard, TestMultiIdentityCoexistence(), TestParseRoundTripStable(), TestWriteBackupOnPreexisting(), TestWriteGlobalBlockOrderedLast(), TestWriteIdempotent(), TestWritePreservesForeignContent(), RenderCheckedHostBlock() (+12 more)

### Community 101 - "Rotate"
Cohesion: 0.22
Nodes (21): rotateArchivePaths(), rotateLog, algoFromKeyPath(), Deps, RotateResult, Rotate(), rotateInput(), callLog (+13 more)

### Community 102 - "Deps"
Cohesion: 0.12
Nodes (34): CheckFn, Family, FixRewrite, ParseError, gitignorePairFinding(), allowedSignersMissingFindingWithFix(), buildSignersFix(), checkAuthorResolution() (+26 more)

### Community 103 - "CreateInput"
Cohesion: 0.31
Nodes (21): KeyResult, signersWriter, CreateInput, CreateResult, Deps, StagedKey, mergeCreateResults(), PersistAll() (+13 more)

### Community 104 - "io.Writer"
Cohesion: 0.26
Nodes (18): fp(), joinStrings(), newDebugCapsCmd(), newDebugCmd(), osNote(), printCapabilities(), printCatalog(), printInventory() (+10 more)

### Community 105 - "globalgit/version_test.go"
Cohesion: 0.19
Nodes (19): globalGitGateOutcome(), GateOutcome, OptionPolicy, RealGateForRow(), conflictstyleRow(), OptionPolicy, TestVersionGateNamesTheGatedOptionNotAHardcodedLiteral(), TestVersionGateNoMinimum() (+11 more)

### Community 106 - "writeJSON"
Cohesion: 0.25
Nodes (18): buildIdentityRecords(), matchStrategyFor(), newIdentityListVerb(), newIdentityShowVerb(), renderIdentityList(), renderIdentityShow(), toIdentityRecord(), unclassifiedIdentityRecord() (+10 more)

### Community 107 - "ScreenSpec"
Cohesion: 0.08
Nodes (50): makeCandidate(), writeValidPNG(), TestGlobalSSHHTMLNonApplicabilityPerSpec(), TestNegativeControl_GlobalGitMidByteTruncationHashStable(), image/color.RGBA, doctorSpecs(), gitIgnoreVisualSpecs(), gitScreenSpecs() (+42 more)

### Community 108 - "scan.go"
Cohesion: 0.20
Nodes (17): ScanRegion, UnmanagedHit, ScanSource, lineReferencesAlias(), regionForBlockName(), ScanUnmanagedReferences(), splitKeepLines(), SplitScanRegions() (+9 more)

### Community 109 - "Tool"
Cohesion: 0.19
Nodes (18): desiredRegistrations(), toUploadResultRow(), glabEntriesWithScope(), glabExactScopedEntry(), glabRegistrationLabel(), deleteArgs(), DeleteCommandPreview(), DeleteKey() (+10 more)

### Community 110 - "repoRoot"
Cohesion: 0.35
Nodes (9): repoRoot(), TestInstall_MakeInstallOutput(), TestRelease_UnstampedBuildKeepsDevDefaults(), goreleaserBinPath(), realGoreleaserConfigWithScratchDist(), runScratchGoreleaserRelease(), TestReleaseHomebrewGate_NotSkippedStillSucceedsLocally(), TestReleaseHomebrewGate_SkippedNeverEntersHomebrewPipe() (+1 more)

### Community 111 - "identityVerb"
Cohesion: 0.08
Nodes (44): newIdentityCloneVerb(), termIsStdinTTY(), termIsStdoutTTY(), isTTY(), outcomeLabel(), newIdentityDeleteVerb(), identityVerbSpecs(), realBackend (+36 more)

### Community 112 - "OptionRow"
Cohesion: 0.18
Nodes (18): BundleResult, EffectiveEntry, SourceClass, BundleFor(), OptionPolicy, TestBundleForAggregate(), TestBundleForEmptyEffective(), TestBundleForNamesOnlyDiffersMembers() (+10 more)

### Community 113 - "frame.go"
Cohesion: 0.06
Nodes (39): charm.land/bubbletea/v2.MouseClickMsg, charm.land/lipgloss/v2.Style, image/color.Color, dimPane(), fitLine(), footerActionAt(), footerFit(), TabID (+31 more)

### Community 114 - "adopt.go"
Cohesion: 0.16
Nodes (26): resolveGlobalSSHTargetPath(), TestResolveGlobalSSHTargetPath_FreshHome(), Adopt(), candidateTarget(), containsPath(), DetectInclude(), expandIncludePath(), IncludeDirective (+18 more)

### Community 115 - "release_e2e_test.go"
Cohesion: 0.13
Nodes (59): fixtureServer, retagArtifactsDir(), runInstallScriptWithEnv(), startReleasesAPIFixtureServer(), TestInstallScript_ChannelNightlyResolvesLatestNightly(), TestInstallScript_ChannelUnsetPreservesExistingHeadlessBehavior(), TestInstallScript_InvalidChannelRefuses(), TestInstallScript_MenuExcludesOldAssetShapeReleases() (+51 more)

### Community 116 - "globalgit/policy_test.go"
Cohesion: 0.13
Nodes (17): PolicyForMember(), TestPolicyAliasesVerbatimFromRecipe(), TestPolicyColorKeysVerbatimFromRecipe(), TestPolicyFor_CaseInsensitive(), TestPolicyFor_KnownKey(), TestPolicyFor_UnknownKey(), TestPolicyForMember_BundleRow(), TestPolicyForMember_CaseInsensitive() (+9 more)

### Community 117 - "CurrentOS"
Cohesion: 0.20
Nodes (16): clipboardInstallHint(), CurrentOS(), gitInstallHint(), InstallHint(), libfido2InstallHint(), normalizeTool(), opensshInstallHint(), parseKeyTypes() (+8 more)

### Community 118 - "PlanFor"
Cohesion: 0.29
Nodes (9): FixPlan, PlanFor(), seededFinding(), TestPlanForContradictionIsDestructiveAndReusesFixtureDiff(), TestPlanForDefaultFallback(), TestPlanForDuplicateHostStar(), TestPlanForKeyPerms(), TestPlanForMissingFragment() (+1 more)

### Community 119 - "unlockStoreForIdentity"
Cohesion: 0.22
Nodes (16): assertFixedDirectivePresent(), realBackend, identityBlockBody(), runCommitCreate(), TestApplyThenCreatePreservesGlobalFix(), TestCombinedTransactionErrorMessageIsDisplayShortened(), TestCombinedTransactionReportsRestorationFailure(), TestCombinedTransactionRetainsBackupsWhenRestorationFails() (+8 more)

### Community 120 - "buildDeleteDeps"
Cohesion: 0.21
Nodes (5): buildDeleteDeps(), countForeignProviderRefs(), nameFromAlias(), AllHostStanzas(), HostStanza

### Community 121 - "RenderIncludeIf"
Cohesion: 0.17
Nodes (18): TestIncludeIfGitdir_ResolvesViaRealGit(), RemoveProviderRewrite(), RenderIncludeIf(), countBackupFiles(), TestIncludeIfRejectsUnsafeInput(), TestIncludeIfStrategies(), TestProviderRewrite(), TestProviderRewriteRejectsUnsafeInput() (+10 more)

### Community 122 - "tester.go"
Cohesion: 0.27
Nodes (14): UpdateDeps, UpdateResult, expandTilde(), readPubLine(), TestReadPubLine_ExpandsTilde(), Update(), execRunner(), Outcome (+6 more)

### Community 123 - "resolveFromBuildInfo"
Cohesion: 0.21
Nodes (12): newVersionCmd(), versionDocument, runtime/debug.BuildInfo, Info, Resolve(), resolveFromBuildInfo(), TestResolve_FallsBackToBuildInfoNeverPanicsAndNonEmptyVersion(), TestResolve_PrefersLdflagsStampWhenPresent() (+4 more)

### Community 124 - "readRepoFile"
Cohesion: 0.11
Nodes (46): TestCIWorkflowNeverInlinesExpressionsIntoRunScripts(), TestFedoraJobCheckoutFetchesFullTagHistory(), TestFedoraJobDeclaresNoPermissions(), TestFedoraJobDnfInstallIsFirstStep(), TestFedoraJobExists(), TestFedoraJobRunsOnlyOnPush(), TestFedoraJobRunsTheFullAutomatedSuite(), TestFedoraJobSafeDirectoryBeforeCheckout() (+38 more)

### Community 125 - "ParseManagedIncludeIf"
Cohesion: 0.18
Nodes (14): TestCustomGitKeysBlockNameIsReserved(), TestIsReservedBlockName_GitFallbackAuthor(), TestIsReservedBlockName_GlobalGit(), TestIsReservedBlockName_PlainIdentity(), IsReservedBlockName(), ParseManagedIncludeIf(), TestParseManagedIncludeIf_Empty(), TestParseManagedIncludeIf_ExcludesReservedBaseline() (+6 more)

### Community 126 - ".runGlobalSSHApply"
Cohesion: 0.14
Nodes (17): TestGlobalSSHFixturePolicyParity(), OptionPolicy, VersionOutcome, TestStateFor(), TestStateForEqualityWinsForEverySource(), TestStatusesDiffersKeepsAttributionSeparate(), PolicyFor(), Deps (+9 more)

### Community 127 - "ScanDirectives"
Cohesion: 0.28
Nodes (11): DirectiveHit, DirectiveSource, ScanDirectives(), ScanDirectivesMulti(), TestScanDirectivesAbsentReturnsEmpty(), TestScanDirectivesCaseInsensitiveKeys(), TestScanDirectivesMultiLineOffset(), TestScanDirectivesMultiOrderAndPath() (+3 more)

### Community 128 - "CheckDeps"
Cohesion: 0.15
Nodes (16): Detect(), found(), GitVersion(), GitVersionAtLeast(), GitVersionParts(), Report, TestDetectSmoke(), TestMissingRequired() (+8 more)

### Community 129 - "update_test.go"
Cohesion: 0.48
Nodes (10): updateCallLog, baseAccount(), newFakeUpdateDeps(), TestUpdate_FragmentOnly(), TestUpdate_NameImmutable(), TestUpdate_SigningOffCallsRemoveAllowedSigners(), TestUpdate_SigningOnCallsWriteAllowedSigners(), TestUpdate_Structural() (+2 more)

### Community 130 - "lifecycle_fallbackauthor_test.go"
Cohesion: 0.21
Nodes (16): extractFuncBody(), fallbackBody(), mustRead(), seedIncludeIf(), TestGitFallbackAuthorLifecycleStagesRow(), TestRunGitFallbackAuthorApply_BothHalves(), TestRunGitFallbackAuthorApply_DistinctFromGlobalGitApply(), TestRunGitFallbackAuthorApply_EmptyPairRemoves() (+8 more)

### Community 131 - "BuildInventory"
Cohesion: 0.18
Nodes (22): FragmentInfo, BuildInventory(), BuildInventoryDeps(), filterReservedKeyPaths(), InventoryDeps, InventoryDepsForHome(), listKeyFilesReal(), listKeyFilesRealForHome() (+14 more)

### Community 132 - "globals.go"
Cohesion: 0.33
Nodes (8): ensureGlobalsLast(), globalsBlockIsLast(), newGlobalMap(), parseGlobalBody(), renderGlobalBody(), RenderGlobalBodyWithOverlay(), globalKV, globalMap

### Community 133 - "CheckPermissions"
Cohesion: 0.35
Nodes (12): CheckPermissions(), containsStr(), findSubstr(), makeMissingStat(), TestCheckPermsCritical(), TestCheckPermsDirError(), TestCheckPermsLooseKeyTightensNotWidens(), TestCheckPermsMissingSkipped() (+4 more)

### Community 134 - "gitid-frame-promote/main.go"
Cohesion: 0.21
Nodes (19): gitHead(), main(), promoteForPhase(), promoteFrames(), repoRoot(), resolvePhase(), resolveProvenanceText(), run() (+11 more)

### Community 135 - "expandTildeForHome"
Cohesion: 0.15
Nodes (22): cloneCeremonyInputs(), realBackend, orDefault(), TestCloneCeremonyInputsGitDirMatchesClonePrefillDerivation(), TestCloneCeremonyInputsMirrorsSourceForceSSH(), TestCloneCeremonyInputsPublicKeyPathNeverBarePubSuffix(), expandTildeForHome(), gitDirFromMatches() (+14 more)

### Community 136 - "OptionPolicy"
Cohesion: 0.33
Nodes (6): validateGitApplyTokens(), GateKind, MemberPolicy, OptionPolicy, PolicyForToken(), TokenOwningMember()

### Community 137 - "CheckOrphans"
Cohesion: 0.31
Nodes (20): CheckOrphans(), incompleteIdentityNames(), sliceToSet(), orphContains(), orphStat(), orphTitles(), TestCustomGitKeysBlockNameIsReserved(), TestMissingFragmentNoDuplicate() (+12 more)

### Community 138 - "identity/provisional_test.go"
Cohesion: 0.35
Nodes (12): provisionalCallArgs, EffectiveAlias(), fakeDepsForProvisional(), callLog, makeCreateInput(), makeStagedKey(), TestDropProvisionalSSH_CallsOnlyDropSeam(), TestEffectiveAlias() (+4 more)

### Community 140 - "resolveHomeForCLI"
Cohesion: 0.13
Nodes (20): newDoctorAliasCmd(), findingsForIdentity(), findingsForIdentityPairs(), healthFinish(), newHealthCmd(), printHealthFindings(), runHealth(), suppressParseErrorFindingPairs() (+12 more)

### Community 141 - "gitid git JSON schema"
Cohesion: 0.22
Nodes (8): Captured example — a below-gate conflict-style apply, Exit-status contract, `gitid git fallback set --json`, `gitid git fallback show --json`, gitid git JSON schema, `gitid git options apply --json`, `gitid git options list --json`, Option object

### Community 142 - "createInputFromCreateFlags"
Cohesion: 0.13
Nodes (26): confirmYes(), createInputFromCreateFlags(), createPrefillFromFlags(), createPreviewLine(), realBackend, indentBlock(), newIdentityCreateVerb(), runCreateCeremony() (+18 more)

### Community 143 - "reserved_test.go"
Cohesion: 0.33
Nodes (16): includeFixture, applyEveryFix(), block(), blockNames(), containsName(), mustReadFile(), orphanDeps(), removeBlock() (+8 more)

### Community 144 - "startStoragePTY"
Cohesion: 0.44
Nodes (13): captureStorageFrame(), FakeMigrateSSHDir(), managedBlockNamesInFile(), seedStorageMigrateHome(), startStoragePTY(), storageSubTabStrip(), TestGlobalSSHStorage_RealPTYBrowse(), TestGlobalSSHStorage_RealPTYChangedSincePreview() (+5 more)

### Community 145 - "global_ssh_cli_e2e_test.go"
Cohesion: 0.27
Nodes (10): countBackups(), runSSHCLI(), TestGlobalSSHCLI_AdvisoryExitCodes(), TestGlobalSSHCLI_DryRunNoWrite(), TestGlobalSSHCLI_ListShowApplyIdempotent(), sshApplyDoc, sshOptionRecord, sshOptionsDoc (+2 more)

### Community 146 - "gitid"
Cohesion: 0.11
Nodes (16): How this ledger is updated, Platform Notes, curl | sh one-liner (`scripts/install.sh`), gitid, `go install`, Homebrew tap, Install, Manual install (inspect first) (+8 more)

### Community 147 - "Match"
Cohesion: 0.40
Nodes (9): MatchKind, Match, renderBlockBody(), RenderCheckedIncludeIf(), safeInline(), TestWriteIncludeIf_IdempotentAndPreservesForeign(), validateIncludeIf(), validSSHHasconfig() (+1 more)

### Community 148 - "EnsureGitFallbackAuthor"
Cohesion: 0.27
Nodes (15): EnsureGitFallbackAuthor(), ReadGitFallbackAuthor(), fallbackBody(), recipeShapedGitconfig(), TestEnsureGitFallbackAuthor_BothEmptyRemoves(), TestEnsureGitFallbackAuthor_BothHalves(), TestEnsureGitFallbackAuthor_EmailOnly(), TestEnsureGitFallbackAuthor_EmptyEmailStillValid() (+7 more)

### Community 149 - "gitid ssh JSON schema"
Cohesion: 0.25
Nodes (7): Exit-status contract, gitid ssh JSON schema, `gitid ssh options apply --json`, `gitid ssh options list --json`, `gitid ssh storage migrate --json`, `gitid ssh storage show --json`, Option object

### Community 150 - "AllSetKeys"
Cohesion: 0.29
Nodes (9): SetKey, AllSetKeys(), Deps, TestAllSetKeysIsSortedByKey(), TestAllSetKeysMarksMultiValuedKeys(), TestAllSetKeysPreservesValuesContainingSeparators(), TestAllSetKeysPropagatesProbeError(), TestAllSetKeysReturnsEverySetKeyWithProvenance() (+1 more)

### Community 151 - "WriteFragment"
Cohesion: 0.25
Nodes (16): gitConfigSet(), gitConfigUnsetAll(), SetAllowedSignersFile(), gitGet(), TestSetAllowedSignersFile(), TestSetAllowedSignersFile_LeadingDashValueIsLiteral(), TestWriteFragment_CreatesParentDir(), TestWriteFragment_LeadingDashValueIsLiteralNotGitOption() (+8 more)

### Community 152 - "AllDirectives"
Cohesion: 0.31
Nodes (8): AllDirectives(), Deps, Directive, TestAllDirectivesIsSortedByKey(), TestAllDirectivesProbesTheWildcardSentinel(), TestAllDirectivesPropagatesProbeError(), TestAllDirectivesReturnsEveryResolvedKey(), TestAllDirectivesSkipsCamelCaseLines()

### Community 153 - "DeleteDeps"
Cohesion: 0.28
Nodes (8): deleteEverything(), DeleteScopeFrom(), DeleteDeps, DeleteResult, SharedKeyOwners(), TestDeleteScopeFrom_KnownAndUnknown(), TestSharedKeyOwners_MultipleSiblingsSortedOrder(), TestSharedKeyOwners_NoOwnersEmpty()

### Community 154 - "TestGitJSONOptionsListExactKeySetAndEnums"
Cohesion: 0.39
Nodes (7): TestGitJSONOptionsListExactKeySetAndEnums(), TestGitJSONStateEnumsAreRenderLayerTaxonomy(), jsonObjectKeys(), assertEnumMember(), assertExactKeys(), TestSSHJSONOptionsListExactKeySetAndEnums(), TestSSHJSONStorageShowExactKeySet()

### Community 155 - "BinaryInstallInfo"
Cohesion: 0.47
Nodes (4): BinaryInstallInfo(), binaryOnPath(), TestBinaryInstallInfo(), TestBinaryOnPath()

### Community 156 - "globalssh/probe.go"
Cohesion: 0.36
Nodes (8): TestIsolatedConfigContract(), baseline(), effective(), Deps, hitsFromContent(), parseResolvedOptions(), policyKeys(), runProbe()

### Community 157 - "gitid CLI parity matrix"
Cohesion: 0.50
Nodes (3): gitid CLI parity matrix, Requirement-keyed outcome matrix, The per-verb `--dry-run` contract (R12-DR)

### Community 158 - "SSHVersion"
Cohesion: 0.48
Nodes (5): SSHVersion, parseSSHVersion(), ProbeSSHVersion(), TestParseSSHVersion(), TestProbeSSHVersionReturnsStruct()

### Community 176 - "TestKeyTypeMapping"
Cohesion: 0.60
Nodes (3): AlgorithmForToken(), SupportedAlgorithms(), TestKeyTypeMapping()

### Community 177 - "install.sh"
Cohesion: 0.60
Nodes (3): asset_shape_available(), fail(), install.sh script

### Community 191 - "Catalog"
Cohesion: 0.21
Nodes (18): toAlgorithmCatalogEntry(), Catalog(), Generatable(), AlgoInfo, isHardwareBacked(), ResolveAvailability(), TestCatalog_EntriesCarryMetadata(), TestCatalog_ExactlyOneDefault() (+10 more)

### Community 196 - "doctor_screen_test.go"
Cohesion: 0.10
Nodes (40): newScreens(), TestNewScreensExhaustiveSwitchOverTabID(), TestMouseDoctorFixThisButtonAndCeremonyCancel(), fixableFindings(), newDoctorModel(), ceremonyPending(), confirmFix(), selectedFinding() (+32 more)

### Community 207 - "rewrite.go"
Cohesion: 0.25
Nodes (14): ApplyVerifiedHostDirective(), DiffHostDirective(), isStanzaHeader(), locateDirectiveLine(), parseDirectiveLine(), restoreFromBackup(), rewriteDirectiveLine(), rewriteDirectiveLineValue() (+6 more)

### Community 210 - "RedactCLIOutput"
Cohesion: 0.20
Nodes (12): ClassifyGHDuplicate(), ClassifyUploadFailure(), RedactCLIOutput(), TestClassifyGHDuplicateRequiresZeroExit(), TestClassifyGLabAlreadyTakenIsAConflictNotSuccess(), TestClassifyNotAuthenticatedForBothProviders(), TestClassifyScopeFailuresByScopeIdentifier(), TestRedactCLIOutputIsSingleLineAndBounded() (+4 more)

### Community 221 - "CheckRedundancy"
Cohesion: 0.30
Nodes (13): globalScan, canonicalDirectiveName(), CheckRedundancy(), scanGlobalDirectives(), makeRedundancyDeps(), TestCheckRedundancy_AdviceNamesCurrentBlock(), TestCheckRedundancy_CleanConfig(), TestCheckRedundancy_EmptyConfig() (+5 more)

### Community 224 - "determinism_test.go"
Cohesion: 0.36
Nodes (9): HashPNG(), StripPNGMetadata(), buildChunk(), fixturePNG(), TestHashPNG_StableForFixedInput(), TestHashPNG_UnaffectedByTimestampMetadata(), TestStripPNGMetadata_Idempotent(), TestStripPNGMetadata_RejectsBadSignature() (+1 more)

### Community 225 - "fragment_signing_test.go"
Cohesion: 0.50
Nodes (8): assertContains(), assertNotContains(), gitConfigList(), TestWriteFragment_SigningFalse(), TestWriteFragment_SigningToggleOffToOn(), TestWriteFragment_SigningToggleOnToOff(), TestWriteFragment_SigningTrue(), TestWriteFragment_ValidationStillApplied()

### Community 228 - "ParseManagedHosts"
Cohesion: 0.15
Nodes (21): nameUnion(), extractProviderMarker(), HostBlockFacts, SSHHostInfo, hostLineNumber(), managedBlockLineMap(), ParseAllHostBlocks(), parseHostBlockBody() (+13 more)

### Community 230 - ".resolveKeyPath"
Cohesion: 0.16
Nodes (8): providerFromAlias(), TestProviderFromAliasPreservesMultiLabelProvider(), TestToReusableKeyViewsCarriesFlagsAndOwner(), toReusableKeyViews(), Copy(), TestCopyAvailable(), TestCopyPropagatesOtherErrors(), TestCopyUnavailable()

### Community 231 - "rewrite_test.go"
Cohesion: 0.35
Nodes (13): RewriteHostDirective(), TestApplyVerifiedHostDirective_RestoresOnVerificationFailure(), TestApplyVerifiedHostDirective_SucceedsAndVerifies(), TestRewriteHostDirective_ChangesOnlyTheTargetLine(), TestRewriteHostDirective_CreatesTimestampedBackup(), TestRewriteHostDirective_DirectiveNotFound(), TestRewriteHostDirective_ManagedBlockUntouched(), TestRewriteHostDirective_PreservesIndentAndComment() (+5 more)

### Community 232 - "validation_test.go"
Cohesion: 0.23
Nodes (13): containsFold(), TestAliasCollisionCyclicInclude(), TestAliasCollisionExact(), TestAliasCollisionIncludeAware(), TestAliasCollisionMalformedConfig(), TestAliasCollisionMissingFile(), TestAliasCollisionNegated(), TestAliasCollisionWildcard() (+5 more)

### Community 237 - "CaptureHTML"
Cohesion: 0.33
Nodes (7): TestCaptureHTML(), TestCaptureHTML_OfflineFailurePath(), TestProvisionPinnedChromium(), CaptureHTML(), resolveBrowserBinary(), Result, HTMLOptions

### Community 239 - "globalgit/isolation_contract_test.go"
Cohesion: 0.50
Nodes (4): initTestRepo(), TestGlobalgitDepsAllowlist(), TestGlobalgitIsolationContract(), TestGlobalgitPackageNeverWrites()

### Community 240 - "perAliasFromContent"
Cohesion: 0.50
Nodes (3): perAliasFromContent(), TestPerAliasConformance(), TestPerAliasConformanceEmpty()

## Knowledge Gaps
- **152 isolated node(s):** `reusableKeyMat`, `gitFallbackDocument`, `realBackend`, `versionDocument`, `gitOptionRecord` (+147 more)
  These have ≤1 connection - possible missing edges or undocumented components. (Counts symbols only; 348 node(s) total have ≤1 connection when file, concept and rationale nodes are included.)
- **3 thin communities (<3 nodes) omitted from report** — run `graphify query` to explore isolated nodes.

## Suggested Questions
_Questions this graph is uniquely positioned to answer:_

- **Why does `FixtureBackend` connect `stubBackend` to `identities.go`, `gitIgnoreModel`, `frame.go`, `RequiredScreenSpecs`, `store.go`, `PlanFor`, `DemoState`, `ceremonyModel`?**
  _High betweenness centrality (0.026) - this node is a cross-community bridge._
- **Why does `stubBackend` connect `stubBackend` to `identities.go`, `press`, `GenerateMaterial`, `testing.T`, `validation_test.go`, `identities_test.go`, `frame.go`, `DeriveCloneInput`, `store.go`, `PlanFor`, `DemoState`, `state_test.go`, `ceremonyModel`?**
  _High betweenness centrality (0.023) - this node is a cross-community bridge._
- **Why does `realBackend` connect `realBackend` to `Write`, `expandTildeForHome`, `validation.go`, `EnsureGitFallbackAuthor`, `newBackendForHome`, `.commitGitArtifacts`, `DefaultGitignorePatterns`, `DemoFinding`, `SSHVersion`, `upload_real_account_gitlab_e2e_test.go`, `ExistingKey`, `gitIgnoreModel`, `buildIdentityDeps`, `DeriveCloneInput`, `.displayPath`, `uploader_test.go`, `authorresolve.go`, `state_test.go`, `Catalog`, `seedSSHDir`, `EnsureIncludeLine`, `fileExists`, `wiring.go`, `RedactCLIOutput`, `.planUpload`, `ptySession`, `effectiveProbe`, `realBackend`, `stubBackend`, `.resolveKeyPath`, `CreateInput`, `globalgit/version_test.go`, `scan.go`, `frame.go`, `adopt.go`, `PlanFor`, `buildDeleteDeps`?**
  _High betweenness centrality (0.022) - this node is a cross-community bridge._
- **Are the 282 inferred relationships involving `newBackendForHome()` (e.g. with `runFix()` and `buildDoctorCaptures()`) actually correct?**
  _`newBackendForHome()` has 282 INFERRED edges - model-reasoned connections that need verification._
- **Are the 251 inferred relationships involving `press()` (e.g. with `pressKey()` and `TestCeremonyDestructiveArrowsStayOnTypedInput()`) actually correct?**
  _`press()` has 251 INFERRED edges - model-reasoned connections that need verification._
- **Are the 163 inferred relationships involving `NewApp()` (e.g. with `TestHelpOverlayShowsFullLegend()` and `TestNewAppPrefilledNilBehavesLikeNewApp()`) actually correct?**
  _`NewApp()` has 163 INFERRED edges - model-reasoned connections that need verification._
- **Are the 160 inferred relationships involving `appView()` (e.g. with `stripANSI()` and `TestMouseCeremonyButtonsCancelConfirmDone()`) actually correct?**
  _`appView()` has 160 INFERRED edges - model-reasoned connections that need verification._