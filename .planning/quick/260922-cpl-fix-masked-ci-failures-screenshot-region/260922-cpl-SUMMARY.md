---
status: complete
---

# 260922-cpl — fix masked CI failures and screenshot region

## Commits

- `948e957` test(gitid): make cmd/gitid tests hermetic for the fedora root container
- `e783171` fix(screenshot): drive gign-receipt through the Confirm-focused apply ceremony
- `0cdfd67` fix(tuikit): keep the full Host-block preview inside the 100x30 step-0 budget

## Root causes

- Host-preview row overflow from `e156586` (09.7-01): three always-on rows pushed `IdentitiesOnly yes` and the preview border off the 100x30 frame (02-STYLE-SPEC §7 no clipping, 09.7 D-08). Recovered exactly 3 rows: duplicate Key header removed, key-source hint compacted to `(←/→)` so the row fits one line, Port hint moved inline onto the Port row. The reuse-picker region is re-anchored on the selected Reuse radio, and a worst-case row-budget test guards the 25-of-25 body-row accounting. Fixes `TestExtractRegion_HostPreview`.
- The gign-receipt screenshot script was stale since quick 260919-jnl made Confirm initially focused.
- Three chmod-0000 tests were bypassed by root in the fedora container; fixed with a probe-based skip.
- `TestPlanUploadReadFileFailureRedactsHomePath` depended on `gh` on PATH; fixed with a `LookPath` stub.

## Verification

Final gate on the committed tree (`0cdfd67`), all green.

`make test` — exit 0. Tail:

```
go test -race -coverprofile=coverage.out ./...
ok  	github.com/castocolina/gitid/cmd/gitid	18.799s	coverage: 78.4% of statements
ok  	github.com/castocolina/gitid/internal/tuikit	(cached)	coverage: 85.7% of statements
go test -tags screenshot -skip 'TestCaptureTUI|TestCaptureHTML|TestProvisionPinnedChromium' ./internal/screenshot/...
ok  	github.com/castocolina/gitid/internal/screenshot	(cached)
```

`go test ./... -race -count=1 2>&1 | grep -E '^(FAIL|--- FAIL)|panic' ; echo done`:

```
done
```

`make lint` — exit 0. Tail:

```
/home/bazzite/go/bin/golangci-lint run --build-tags screenshot ./internal/screenshot/...
0 issues.
/home/bazzite/go/bin/golangci-lint run ./...
0 issues.
```

## Out of scope

- `e2e TestCreateFlow_GitConfigurationDefaultTracer` hangs in `quitCleanly` (`e2e/create_flow_pty_e2e_test.go:96`) past its 10s quit timeout until the 5m test timeout. Identical at HEAD and at base `c799c99`, so it is pre-existing. Command: `go test -tags e2e -timeout 300s -count=1 -run '^TestCreateFlow_GitConfigurationDefaultTracer$' ./e2e/` → `panic: test timed out after 5m0s ... e2e.quitCleanly` at both commits.
- Pre-existing `make gate-visual-regression` ggit-options-list/keybar failure.
- The reuse-key-vs-generate capture lands on the upload checkbox.
- Fedora `make test-e2e` has never been exercised on CI yet.
