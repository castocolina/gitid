package globalssh

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/castocolina/gitid/internal/platform"
)

// VersionOutcome is how a policy's recommended value relates to the
// machine's OpenSSH version (D-13).
type VersionOutcome int

const (
	// VersionAvailable means the parsed version is at or above the policy minimum.
	VersionAvailable VersionOutcome = iota
	// VersionTooOld means the parsed version is below the policy minimum.
	VersionTooOld
	// VersionUnverified means gitid could not establish the machine's OpenSSH
	// version. D-13 exists to gate accept-new on OpenSSH at least 7.6;
	// treating an unreadable ssh -V as proof of compliance would write a
	// directive whose compatibility gitid never established. The row still
	// renders and still explains itself — the advisory posture is preserved
	// for the EXPLANATION — but the WRITE is withheld until the machine can
	// be identified. Plan 06-06's CLI reports the same refusal with an error
	// naming `ssh -V` as the command to check.
	VersionUnverified
)

// VersionNotePrefix is the dynamic version line's prefix. It MUST stay out of
// the copy-freeze list because the rest of the line changes with the machine.
const VersionNotePrefix = "Your OpenSSH:"

// VersionNoteUnverified is the frozen single line for an unreadable version.
const VersionNoteUnverified = "OpenSSH version could not be read; run ssh -V to check compatibility"

// VersionGate compares the parsed OpenSSH version against p.MinOpenSSH using
// numeric component comparison, not string ordering, so 7.10 is greater than
// 7.6. It reuses platform.SSHVersion — it does not re-parse ssh -V itself.
func VersionGate(v platform.SSHVersion, p OptionPolicy) (VersionOutcome, string) {
	if p.MinOpenSSH == "" {
		return VersionAvailable, ""
	}
	if strings.TrimSpace(v.OpenSSHVersion) == "" {
		return VersionUnverified, VersionNoteUnverified
	}
	if versionLess(v.OpenSSHVersion, p.MinOpenSSH) {
		return VersionTooOld, fmt.Sprintf("%s %s — accept-new needs OpenSSH %s+, upgrade to use it", VersionNotePrefix, v.OpenSSHVersion, p.MinOpenSSH)
	}
	return VersionAvailable, fmt.Sprintf("%s %s — accept-new is available", VersionNotePrefix, v.OpenSSHVersion)
}

func versionLess(got, minimum string) bool {
	gMaj, gMin := versionParts(got)
	mMaj, mMin := versionParts(minimum)
	if gMaj != mMaj {
		return gMaj < mMaj
	}
	return gMin < mMin
}

func versionParts(v string) (major, minor int) {
	trimmed := v
	if i := strings.IndexAny(trimmed, "pP"); i >= 0 {
		trimmed = trimmed[:i]
	}
	parts := strings.Split(trimmed, ".")
	if len(parts) > 0 {
		major, _ = strconv.Atoi(parts[0])
	}
	if len(parts) > 1 {
		minor, _ = strconv.Atoi(parts[1])
	}
	return major, minor
}
