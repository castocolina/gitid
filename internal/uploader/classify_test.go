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
