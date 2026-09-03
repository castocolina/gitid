// Package tuikit is gitid's shared, backend-free TUI presentation layer —
// the ONE source of truth for the approved, frozen design (D-17). It holds
// the Bubble Tea v2 root model, the semantic Theme, the shell frame
// (numbered header nav with a live health chip, breadcrumb/active-area,
// contextual footer), the five primary screens (1 Identities · 2 Global
// SSH · 3 Global Git · 4 Doctor · 5 Global Git Ignore), the create
// wizard, and the shared 2-state mutation ceremony.
//
// BOTH binaries render through this package:
//
//	cmd/gitid-dummy — the live design demo, injecting internal/dummytui's
//	                  recipe fixtures (DLV-05/DLV-02)
//	cmd/gitid       — the real product, injecting the composition root in
//	                  cmd/gitid/wiring.go
//
// They differ ONLY in the tuikit.Backend they inject (backend.go). Because
// the design lives here once, the demo cannot drift from the product.
//
// NO first-party backend package is imported here, ever — not identity,
// tester, sshconfig, keygen, filewriter, doctor, adopter, platform,
// clipboard or uploader — and nothing in this package reads or writes a
// file, shells out, or touches the network. Every value that comes from
// the user's machine arrives through the injected Backend, expressed in
// the tuikit-local view DTOs of views.go. The contract is enforced
// mechanically by internal/dummytui's TestNoBackendAllowlist (go list
// -deps against a three-member allowlist) and by the Makefile's
// gate-no-backend-files target. If a screen appears to need a backend
// type, the answer is a new DTO in views.go plus a conversion in
// cmd/gitid/wiring.go — never a new allowlist entry.
//
// The design in this package is FROZEN: it was approved at the Phase 2
// checkpoint and is pinned by the golden render tests alongside it and by
// the dummy's PTY end-to-end walk. See
// .planning/phases/02-design-all-mockups-checkpoint-1/02-STYLE-SPEC.md for
// the theme role table and the arrow-key precedence rules, and
// .planning/design/REFERENCE-INDEX.md for the full reference map.
package tuikit
