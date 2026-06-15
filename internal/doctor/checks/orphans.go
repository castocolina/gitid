package checks

import "github.com/castocolina/gitid/internal/doctor"

// CheckOrphans checks for artifacts on disk with no owning managed block —
// distinctly from coherence gaps (D-10).
// STUB — overwritten by Plan 03 (Wave 2).
func CheckOrphans(deps doctor.Deps) []doctor.Finding {
	_ = deps
	return nil
}
