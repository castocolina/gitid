package checks

import "github.com/castocolina/gitid/internal/doctor"

// CheckCoherence checks that every IdentityFile resolves, every includeIf
// points to an existing fragment, IdentitiesOnly yes is present, and signing
// identities have an allowed_signers line.
// STUB — overwritten by Plan 03 (Wave 2).
func CheckCoherence(deps doctor.Deps) []doctor.Finding {
	_ = deps
	return nil
}
