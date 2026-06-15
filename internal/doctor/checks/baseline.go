// Package checks implements the per-family health check functions for
// gitid doctor. Each family lives in its own file and is overwritten in
// place by Wave 2 plans without redeclaration.
package checks

import "github.com/castocolina/gitid/internal/doctor"

// CheckBaseline checks the four Phase 3.1 baseline invariants: excludesfile
// wiring, baseline [include] resolves, ignorecase drift, and curated excludes.
// STUB — overwritten by Plan 02 (Wave 2).
func CheckBaseline(deps doctor.Deps) []doctor.Finding {
	_ = deps
	return nil
}
