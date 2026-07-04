// Package dummytui holds the shared dummy fixture data for gitid's design
// reference.
//
// The static screen machinery that used to live here (registry, model,
// shell, overlay, one surface file per Phase 2 design surface) rendered a
// navigation-only static Go TUI dummy whose PNG captures formed half of the
// Phase 2 static reference set. That static paradigm was rejected at the
// design checkpoint: the authoritative design reference is now the
// INTERACTIVE web demo at .planning/design/mockup-src/src/demo/, and a live
// executable Go TUI demo will replace the static dummy in a separately
// replanned task. The static screens, the cmd/gitid-dummy binary, and their
// captures were removed (recoverable from git history).
//
// What remains is data.go: the recipe-accurate fixture data — a Go mirror
// of .planning/design/mockup-src/src/data/recipeFixtures.ts, itself derived
// from recipes/ssh-config.recipe and recipes/gitconfig.recipe (the North
// Star; ed25519 keys, not the gists' RSA, per the recipes' own "structure,
// not key type" caveat). The upcoming live Go TUI demo will seed from these
// values so both media keep rendering the same canonical configuration.
// See .planning/design/REFERENCE-INDEX.md for the full reference map.
package dummytui
