# 03-09 Visual-Regression Review Packet

Assembled by plan 03-09 Task 2 executor (DLV-04.2/D-25).  
Approval-bearing commit: `3c3130e` (docs(02-12): record DLV-08 design approval — 2026-07-06 by Pepe)

## Contents

### Live-TUI contact sheet panels (`panel-pngs/`)

8 PNG panels — the REAL `cmd/gitid` binary's create-flow screens, captured
offline via the `offlineCaptureBackend` deterministic SSH seam. Each panel:
- Rendered at 100×30 terminal geometry (D-04)
- JetBrains Mono / dracula theme
- SHA-256 recorded in `EVIDENCE.json`

Screens: ssh-form-filled, reuse-key-vs-generate, reuse-manual-path,
mouse-focused-field, test-stage1-direct, test-stage2-by-alias,
git-form-demo, confirm-write.

### Approved-TUI reference panels (`approved-tui-panels/`)

8 PNG panels — `cmd/gitid-dummy`'s FixtureBackend screens (the approved
Phase-2 design contract). These represent what the gate compares against.
Same geometry, font, and theme as the live-TUI panels.

### Evidence (`EVIDENCE.json`)

Records: generated timestamp, source commit (HEAD at generation time),
geometry, font, theme, and SHA-256 hashes for each live-TUI panel.
The approved-TUI panels have stable deterministic hashes (dummy fixture
data is fixed; no temp-dir randomness).

### Approved-HTML contact sheet

**Not included**: requires the web mockup SPA to be served (Vite dev server
or `python3 -m http.server 8747 --directory .planning/design/mockup-src/dist`)
and headless Chromium (requires `make setup-env && make screenshot-html`).
The SPA's create-flow static reference routes (`/ref/create-flow/<screen>`)
provide equivalent evidence. Reviewers may use `make demo-web` to serve the
interactive demo and inspect screens live.

## Expected Divergences

The region-scoped gate in `cmd/gitid/gate_visual_regression_test.go` enforces
these documented divergences (from `visual-divergence-allowlist.txt`):

| Code | Region | Screens | Reason |
|------|--------|---------|--------|
| T-03-SIDEBAR | sidebar, header-status | all 8 | Fresh-HOME real backend (0 ids) vs. 8 pre-seeded fixture identities |
| T-03-KEYSECTION | key-section | 4 SSH-form screens | Algorithm catalog order (live-probed vs. fixture order) OR reuse picker entries (scanned key vs. fixture list) |
| T-03-HOSTBLOCK | host-preview | 5 SSH-form + confirm screens | Real: 2-space + provider marker; dummy: 4-space markerless (SSHUI-03 WYSIWYG) |
| D-02 | connectivity-output | test-stage1-direct, test-stage2-by-alias | Connectivity text differs by construction; yellow-not-red D-02 contract proven by PTY e2e |
| D-19 | continue-disabled-reason | git-form-demo | Real: "— Git configuration arrives with the next build"; dummy: "— needs user.name + a valid email" |

**Non-exempt regions (must be byte-identical)**:
header (nav-tabs), breadcrumb, wizard-stepper, form-fields (labels/hints),
keybar (affordance footer). Any drift in these regions fails the gate.

## Review Instructions

Reviewers are asked to:

1. **Inspect appearance**: Do the live-TUI panels match the approved-TUI panels
   in layout, field order, focus markers, preview borders, buttons, and footer?
2. **Four-field form**: Is the SSH form exactly Alias prefix → SSH Host →
   Real hostname → Port (no Provider field, no extra fields)?
3. **Labels and options**: Are field labels, hints, algorithm options, and
   button labels identical to the approved design?
4. **Focus**: Is focus correctly rendered (▸ marker, bracket contour) per D1?
5. **Warnings**: Is D-02's "! Reachable — key not uploaded yet" correctly yellow
   (not red) on test-stage screens?
6. **Exact commands**: Do the test-stage screens show the complete SSH command?
7. **Safety ceremony**: Does confirm-write show the backup path / target file preview?
8. **Recipe Host preview**: Does the SSH Host-block preview match recipe structure
   (alias, `Hostname`, `Port 443`, `User git`, `IdentityFile`, `IdentitiesOnly yes`)?

## Allowlist Text Diffs

The full text diffs (ANSI-stripped) are produced by running:

```sh
go test -tags screenshot -run TestGateVisualRegression -v ./cmd/gitid/
```

Each allowlisted region is logged with its D-XX reference and reason.
Unallowlisted differences fail the gate outright.

## Provenance

- Approval commit: `3c3130e` (DLV-08 approval, 2026-07-06 by Pepe)
- Live-TUI panels generated from: HEAD at time of `make gate-visual-regression`
- Region-scoped gate schema: `screen:region:predicate:D-XX:reason` (5 fields)
- Gate validation: unknown screens/regions, duplicates, blank reasons, missing
  D-XX references, and stale entries all fail
- Non-exempt coverage: ≥1 byte-identical region per screen required
