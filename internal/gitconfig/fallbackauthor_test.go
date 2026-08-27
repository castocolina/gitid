package gitconfig

import (
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/filewriter"
)

func recipeShapedGitconfig() []byte {
	includeBody := "[include]\n\tpath = ~/.gitconfig.d/00-baseline"
	workBody := "[includeIf \"gitdir:~/git/work/\"]\n\tpath = ~/.gitconfig.d/work"
	personalBody := "[includeIf \"gitdir:~/git/personal/\"]\n\tpath = ~/.gitconfig.d/personal"
	return []byte(
		filewriter.BeginPrefix + BaselineIncludeBlockName + "\n" + includeBody + "\n" +
			filewriter.EndPrefix + BaselineIncludeBlockName + "\n" +
			filewriter.BeginPrefix + "work\n" + workBody + "\n" + filewriter.EndPrefix + "work\n" +
			filewriter.BeginPrefix + "personal\n" + personalBody + "\n" + filewriter.EndPrefix + "personal\n",
	)
}

func fallbackBody(content []byte) string {
	for _, b := range filewriter.ListBlocks(content) {
		if b.Name == GitFallbackAuthorBlockName {
			return b.Body
		}
	}
	return ""
}

func TestEnsureGitFallbackAuthor_BothHalves(t *testing.T) {
	result, err := EnsureGitFallbackAuthor(recipeShapedGitconfig(), "Pat Example", "pat@example.com")
	if err != nil {
		t.Fatalf("EnsureGitFallbackAuthor: %v", err)
	}
	body := fallbackBody(result)
	if !strings.Contains(body, "name = Pat Example") {
		t.Errorf("both-halves body missing name:\n%s", body)
	}
	if !strings.Contains(body, "email = pat@example.com") {
		t.Errorf("both-halves body missing email:\n%s", body)
	}
}

func TestEnsureGitFallbackAuthor_NameOnly(t *testing.T) {
	result, err := EnsureGitFallbackAuthor(recipeShapedGitconfig(), "Pat Example", "")
	if err != nil {
		t.Fatalf("EnsureGitFallbackAuthor: %v", err)
	}
	body := fallbackBody(result)
	if !strings.Contains(body, "name = Pat Example") {
		t.Errorf("name-only body missing name:\n%s", body)
	}
	if strings.Contains(body, "email") {
		t.Errorf("name-only body must omit the email key entirely, got:\n%s", body)
	}
}

func TestEnsureGitFallbackAuthor_EmailOnly(t *testing.T) {
	result, err := EnsureGitFallbackAuthor(recipeShapedGitconfig(), "", "pat@example.com")
	if err != nil {
		t.Fatalf("EnsureGitFallbackAuthor: %v", err)
	}
	body := fallbackBody(result)
	if !strings.Contains(body, "email = pat@example.com") {
		t.Errorf("email-only body missing email:\n%s", body)
	}
	if strings.Contains(body, "name") {
		t.Errorf("email-only body must omit the name key entirely, got:\n%s", body)
	}
}

func TestEnsureGitFallbackAuthor_BothEmptyRemoves(t *testing.T) {
	seeded, err := EnsureGitFallbackAuthor(recipeShapedGitconfig(), "Pat Example", "pat@example.com")
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	result, err := EnsureGitFallbackAuthor(seeded, "", "")
	if err != nil {
		t.Fatalf("both-empty: %v", err)
	}
	s := string(result)
	if strings.Contains(s, GitFallbackAuthorBlockName) {
		t.Errorf("both-empty must leave no block of that name, got:\n%s", s)
	}
	if strings.Contains(s, filewriter.BeginPrefix+GitFallbackAuthorBlockName) ||
		strings.Contains(s, filewriter.EndPrefix+GitFallbackAuthorBlockName) {
		t.Errorf("both-empty left orphaned sentinel markers:\n%s", s)
	}
}

func TestEnsureGitFallbackAuthor_Idempotent(t *testing.T) {
	first, err := EnsureGitFallbackAuthor(recipeShapedGitconfig(), "Pat Example", "pat@example.com")
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := EnsureGitFallbackAuthor(first, "Pat Example", "pat@example.com")
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if string(first) != string(second) {
		t.Errorf("idempotency FAILED:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestReadGitFallbackAuthor_RoundTrip(t *testing.T) {
	cases := []struct {
		name, email string
	}{
		{"Pat Example", "pat@example.com"},
		{"Pat Example", ""},
		{"", "pat@example.com"},
		{"", ""},
	}
	for _, tc := range cases {
		composed, err := EnsureGitFallbackAuthor(recipeShapedGitconfig(), tc.name, tc.email)
		if err != nil {
			t.Fatalf("compose %q/%q: %v", tc.name, tc.email, err)
		}
		gotName, gotEmail := ReadGitFallbackAuthor(composed)
		if gotName != tc.name || gotEmail != tc.email {
			t.Errorf("round-trip %q/%q: got %q/%q", tc.name, tc.email, gotName, gotEmail)
		}
	}
}

func TestEnsureGitFallbackAuthor_RejectsNewline(t *testing.T) {
	before := recipeShapedGitconfig()
	_, err := EnsureGitFallbackAuthor(before, "Pat\n[user]", "pat@example.com")
	if err == nil {
		t.Fatal("expected newline-bearing name to be rejected before compose")
	}
	if !strings.Contains(err.Error(), "newline") {
		t.Errorf("error should mention newlines, got: %v", err)
	}
}

func TestEnsureGitFallbackAuthor_PlacementBetweenIncludeAndIncludeIf(t *testing.T) {
	existing := recipeShapedGitconfig()
	result, err := EnsureGitFallbackAuthor(existing, "Pat Example", "pat@example.com")
	if err != nil {
		t.Fatalf("EnsureGitFallbackAuthor: %v", err)
	}
	s := string(result)

	includeEnd := filewriter.EndPrefix + BaselineIncludeBlockName
	includeEndOff := strings.Index(s, includeEnd)
	if includeEndOff < 0 {
		t.Fatal("floor include end-marker missing after compose")
	}
	fallbackBegin := filewriter.BeginPrefix + GitFallbackAuthorBlockName
	fallbackOff := strings.Index(s, fallbackBegin)
	if fallbackOff < 0 {
		t.Fatal("fallback begin-marker missing after compose")
	}
	includeIfOff := strings.Index(s, `[includeIf`)
	if includeIfOff < 0 {
		t.Fatal("recipe-shaped fixture must contain an includeIf section")
	}
	if fallbackOff <= includeEndOff {
		t.Errorf("fallback begin (%d) is not after include end (%d)", fallbackOff, includeEndOff)
	}
	if fallbackOff >= includeIfOff {
		t.Errorf("fallback begin (%d) is not before first includeIf (%d)\n%s", fallbackOff, includeIfOff, s)
	}
}

func TestIsReservedBlockName_GitFallbackAuthor(t *testing.T) {
	if !IsReservedBlockName(GitFallbackAuthorBlockName) {
		t.Errorf("GitFallbackAuthorBlockName %q should be reserved", GitFallbackAuthorBlockName)
	}
}

func TestParseManagedIncludeIf_ExcludesReservedFallbackAuthor(t *testing.T) {
	content, err := EnsureGitFallbackAuthor(recipeShapedGitconfig(), "Pat Example", "pat@example.com")
	if err != nil {
		t.Fatalf("compose: %v", err)
	}
	got := ParseManagedIncludeIf(content)
	if _, ok := got[GitFallbackAuthorBlockName]; ok {
		t.Errorf("reserved %q must be excluded from identity discovery", GitFallbackAuthorBlockName)
	}
	if _, ok := got["work"]; !ok {
		t.Error("real identity 'work' should still be present")
	}
}
