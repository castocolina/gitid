// Package dummytui is the LIVE interactive Go TUI design demo of gitid
// (DLV-05/DLV-02) — a Bubble Tea v2 app (cmd/gitid-dummy) mirroring the
// interactive web demo at .planning/design/mockup-src/src/demo/ 1:1 per
// 02-REDESIGN-SPEC.md: numbered header nav tabs with a live health chip,
// live master-detail Identities with the 4-pane-state create wizard,
// Global SSH (Options + STORE-01 Storage sub-tabs), Global Git, and a
// Doctor that absorbs the Fixer (FIX-02) — all driving a pure reducer
// (store.go) over dummy, in-memory state. NO backend package is imported
// and nothing on disk is ever read or written; every "write" is a reducer
// transition staged through the shared 2-state mutation ceremony.
//
// data.go is the recipe-accurate fixture source — a Go mirror of
// .planning/design/mockup-src/src/data/recipeFixtures.ts, itself derived
// from recipes/ssh-config.recipe and recipes/gitconfig.recipe (the North
// Star; ed25519 keys, not the gists' RSA, per the recipes' own
// "structure, not key type" caveat). The live demo seeds exclusively from
// these values so both media render the same canonical configuration.
// See .planning/design/REFERENCE-INDEX.md for the full reference map.
package dummytui
