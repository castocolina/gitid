package checks

import "github.com/castocolina/gitid/internal/doctor"

// CheckSigning checks gpg.format=ssh, allowed_signers path, and the
// git-version gate for hasconfig: usage (D-20).
// STUB — overwritten by Plan 04 (Wave 2).
func CheckSigning(deps doctor.Deps) []doctor.Finding {
	_ = deps
	return nil
}

// CheckAgent checks whether the ssh-agent is reachable and each gitid-managed
// key is loaded in the running agent.
// STUB — overwritten by Plan 04 (Wave 2).
func CheckAgent(deps doctor.Deps) []doctor.Finding {
	_ = deps
	return nil
}
