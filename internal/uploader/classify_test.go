package uploader

import (
	"errors"
	"strings"
	"testing"
)

func TestClassifyScopeFailuresByScopeIdentifier(t *testing.T) {
	for _, tc := range []struct {
		output string
		want   FailureKind
	}{{"reworded admin:public_key issue", FailureScopeAuth}, {"changed admin:ssh_signing_key issue", FailureScopeSigning}} {
		if got := ClassifyUploadFailure(ToolGH, RegistrationAuthentication, RegistrationResult{Outcome: OutcomeFailed, Output: tc.output}); got != tc.want {
			t.Errorf("got %v want %v", got, tc.want)
		}
	}
}

// TestClassifyNotAuthenticatedForBothProviders is the WR-03 regression:
// ClassifyUploadFailure must resolve FailureNotAuthenticated for either
// provider's "not logged in"/"not authenticated" CLI output, so
// toUploadResultRow's consumer switch has something real to map to
// remediation copy instead of falling through to the raw CLI line.
func TestClassifyNotAuthenticatedForBothProviders(t *testing.T) {
	for _, tc := range []struct {
		tool   Tool
		output string
	}{
		{ToolGH, "error: not logged in to github.com"},
		{ToolGLab, "error: not authenticated to gitlab.com"},
	} {
		got := ClassifyUploadFailure(tc.tool, RegistrationAuthentication, RegistrationResult{Outcome: OutcomeFailed, Output: tc.output})
		if got != FailureNotAuthenticated {
			t.Errorf("tool=%v output=%q: got %v, want FailureNotAuthenticated", tc.tool, tc.output, got)
		}
	}
}
func TestClassifyGLabAlreadyTakenIsAConflictNotSuccess(t *testing.T) {
	res := RegistrationResult{Outcome: OutcomeFailed, Output: "fingerprint already taken"}
	if got := ClassifyUploadFailure(ToolGLab, RegistrationCombined, res); got != FailureCrossAccountConflict || ClassifyGHDuplicate(res) {
		t.Fatalf("got=%v", got)
	}
}
func TestClassifyGHDuplicateRequiresZeroExit(t *testing.T) {
	if ClassifyGHDuplicate(RegistrationResult{Output: "key already exists", Err: errors.New("failed")}) {
		t.Fatal("failure marked duplicate")
	}
}
func TestRedactCLIOutputRemovesTokenShapes(t *testing.T) {
	gh := "ghp_abcdefghijklmnopqrstuvwx"
	gl := "glpat-abcdefghijklmnopqrstuvwx"
	got := RedactCLIOutput("word "+gh+" "+gl, "", 100)
	if strings.Contains(got, gh) || strings.Contains(got, gl) || !strings.Contains(got, "word") {
		t.Fatal(got)
	}
}

// TestRedactCLIOutputRemovesFineGrainedPAT is the WR-04 regression: the
// classic-token alternation (ghp_/gho_/ghu_/ghs_/ghr_) never matched
// GitHub's fine-grained personal access tokens (github_pat_...) — the most
// common modern token shape — because "gh" followed by "i" in "github_" is
// not in [pousr].
func TestRedactCLIOutputRemovesFineGrainedPAT(t *testing.T) {
	pat := "github_pat_11ABCDEFG0abcdefghijklmnop_abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXY"
	got := RedactCLIOutput("word "+pat, "", 200)
	if strings.Contains(got, pat) || !strings.Contains(got, "word") {
		t.Fatalf("fine-grained PAT was not redacted: %q", got)
	}
}
func TestRedactCLIOutputReplacesHomePath(t *testing.T) {
	if got := RedactCLIOutput("/Users/me/file", "/Users/me", 100); strings.Contains(got, "/Users/me") {
		t.Fatal(got)
	}
}
func TestRedactCLIOutputIsSingleLineAndBounded(t *testing.T) {
	got := RedactCLIOutput(strings.Repeat("x", 20)+"\nsecond", "", 5)
	if strings.Contains(got, "\n") || len([]rune(got)) > 5 {
		t.Fatal(got)
	}
}
