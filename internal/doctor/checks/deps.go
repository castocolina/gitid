package checks

import "github.com/castocolina/gitid/internal/doctor"

// CheckDeps checks whether the required and optional external tools are present
// on PATH and returns findings with per-OS install hints for missing tools.
// STUB — overwritten by Plan 02 (Wave 2).
func CheckDeps(deps doctor.Deps) []doctor.Finding {
	_ = deps
	return nil
}
