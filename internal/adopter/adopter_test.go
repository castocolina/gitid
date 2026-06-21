package adopter_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/castocolina/gitid/internal/adopter"
	"github.com/castocolina/gitid/internal/gitconfig"
)

// --- fakes ---

// fakeAdopterDeps builds a test Deps wired with controllable fakes.
type fakeAdopterDeps struct {
	copyFileCalled bool
	copyFileSrc    string
	copyFileDst    string
	copyFileErr    error

	writeIncludeIfCalled bool
	writeIncludeIfID     string
	writeIncludeIfFrag   string
	writeIncludeIfMatch  []gitconfig.Match
	writeIncludeIfBackup string
	writeIncludeIfErr    error

	removeCallCount int
}

func (f *fakeAdopterDeps) build() adopter.Deps {
	return adopter.Deps{
		ReadFile: func(_ string) ([]byte, error) {
			return nil, errors.New("not wired")
		},
		WriteFile: func(_ string, _ []byte, _ os.FileMode) (string, error) {
			return "", errors.New("not wired")
		},
		CopyFile: func(src, dst string) error {
			f.copyFileCalled = true
			f.copyFileSrc = src
			f.copyFileDst = dst
			return f.copyFileErr
		},
		BackupAndRemove: func(_ string) (string, error) {
			// Count calls so tests can assert no removal happened (D-05).
			f.removeCallCount++
			return "", nil
		},
		WriteIncludeIf: func(id, fragPath string, matches []gitconfig.Match) (string, error) {
			f.writeIncludeIfCalled = true
			f.writeIncludeIfID = id
			f.writeIncludeIfFrag = fragPath
			f.writeIncludeIfMatch = matches
			return f.writeIncludeIfBackup, f.writeIncludeIfErr
		},
		ReadFragment: func(_ string) (gitconfig.FragmentInfo, error) {
			return gitconfig.FragmentInfo{Missing: true}, nil
		},
		ListCandidates: func(_ string) ([]string, error) {
			return nil, nil
		},
	}
}

// --- Adopt tests ---

// TestAdoptMigrate verifies the migrate path:
//   - calls CopyFile(src, ~/.gitconfig.d/<name>)
//   - calls WriteIncludeIf(name, dst, matches)
//   - sets MigratedPath
//   - does NOT call BackupAndRemove (D-05 never-delete invariant)
func TestAdoptMigrate(t *testing.T) {
	fake := &fakeAdopterDeps{writeIncludeIfBackup: "backup.bak"}
	deps := fake.build()

	home := t.TempDir()
	gitconfigPath := filepath.Join(home, ".gitconfig")
	sourcePath := filepath.Join(home, ".gitconfig_work")
	matches := []gitconfig.Match{
		{Kind: gitconfig.MatchGitdir, Value: "~/git/work/"},
	}

	result, err := adopter.Adopt(sourcePath, "work", gitconfigPath, adopter.AdoptMigrate, matches, deps)
	if err != nil {
		t.Fatalf("Adopt(migrate) returned error: %v", err)
	}

	// CopyFile must be called with the source and the expected destination.
	if !fake.copyFileCalled {
		t.Error("CopyFile was not called")
	}
	wantDest := filepath.Join(home, ".gitconfig.d", "work")
	if fake.copyFileSrc != sourcePath {
		t.Errorf("CopyFile src: got %q want %q", fake.copyFileSrc, sourcePath)
	}
	if fake.copyFileDst != wantDest {
		t.Errorf("CopyFile dst: got %q want %q", fake.copyFileDst, wantDest)
	}

	// WriteIncludeIf must be called with the destination (not the source).
	if !fake.writeIncludeIfCalled {
		t.Error("WriteIncludeIf was not called")
	}
	if fake.writeIncludeIfID != "work" {
		t.Errorf("WriteIncludeIf id: got %q want %q", fake.writeIncludeIfID, "work")
	}
	if fake.writeIncludeIfFrag != wantDest {
		t.Errorf("WriteIncludeIf fragPath: got %q want %q", fake.writeIncludeIfFrag, wantDest)
	}

	// MigratedPath must be the destination.
	if result.MigratedPath != wantDest {
		t.Errorf("MigratedPath: got %q want %q", result.MigratedPath, wantDest)
	}

	// BackupPaths must include the backup returned by WriteIncludeIf.
	if len(result.BackupPaths) != 1 || result.BackupPaths[0] != "backup.bak" {
		t.Errorf("BackupPaths: got %v want [backup.bak]", result.BackupPaths)
	}

	// NEVER-DELETE invariant: BackupAndRemove must NOT be called (D-05).
	if fake.removeCallCount != 0 {
		t.Errorf("BackupAndRemove was called %d times; must be 0 inside Adopt (D-05)", fake.removeCallCount)
	}
}

// TestAdoptReferenceInPlace verifies the reference path:
//   - does NOT call CopyFile
//   - calls WriteIncludeIf with the original source path
//   - MigratedPath is empty
//   - does NOT call BackupAndRemove (D-05)
func TestAdoptReferenceInPlace(t *testing.T) {
	fake := &fakeAdopterDeps{writeIncludeIfBackup: ""}
	deps := fake.build()

	home := t.TempDir()
	gitconfigPath := filepath.Join(home, ".gitconfig")
	sourcePath := filepath.Join(home, ".gitconfig_personal")
	matches := []gitconfig.Match{
		{Kind: gitconfig.MatchHasconfig, Value: "remote.*.url:git@github.com:personal/**"},
	}

	result, err := adopter.Adopt(sourcePath, "personal", gitconfigPath, adopter.AdoptReferenceInPlace, matches, deps)
	if err != nil {
		t.Fatalf("Adopt(reference-in-place) returned error: %v", err)
	}

	// CopyFile must NOT be called.
	if fake.copyFileCalled {
		t.Error("CopyFile was called for reference-in-place — must not copy")
	}

	// WriteIncludeIf must point at the ORIGINAL source path.
	if !fake.writeIncludeIfCalled {
		t.Error("WriteIncludeIf was not called")
	}
	if fake.writeIncludeIfFrag != sourcePath {
		t.Errorf("WriteIncludeIf fragPath: got %q want %q (original source)", fake.writeIncludeIfFrag, sourcePath)
	}

	// MigratedPath must be empty (no copy took place).
	if result.MigratedPath != "" {
		t.Errorf("MigratedPath: got %q want empty for reference-in-place", result.MigratedPath)
	}

	// NEVER-DELETE invariant.
	if fake.removeCallCount != 0 {
		t.Errorf("BackupAndRemove was called %d times; must be 0 inside Adopt (D-05)", fake.removeCallCount)
	}
}

// TestAdoptBackupPathsCollected verifies that every non-empty backupPath returned
// by WriteIncludeIf is collected in AdoptResult.BackupPaths.
func TestAdoptBackupPathsCollected(t *testing.T) {
	fake := &fakeAdopterDeps{writeIncludeIfBackup: "/home/user/.gitconfig.bak.20240101-000000"}
	deps := fake.build()

	home := t.TempDir()
	result, err := adopter.Adopt(
		filepath.Join(home, ".gitconfig_work"),
		"work",
		filepath.Join(home, ".gitconfig"),
		adopter.AdoptReferenceInPlace,
		nil,
		deps,
	)
	if err != nil {
		t.Fatalf("Adopt returned error: %v", err)
	}
	if len(result.BackupPaths) != 1 {
		t.Fatalf("BackupPaths length: got %d want 1", len(result.BackupPaths))
	}
	if result.BackupPaths[0] != fake.writeIncludeIfBackup {
		t.Errorf("BackupPaths[0]: got %q want %q", result.BackupPaths[0], fake.writeIncludeIfBackup)
	}
}

// TestAdoptInvalidIdentityName verifies that Adopt rejects an invalid
// identityName (containing a newline) before calling any seam (T-05.7-02-01).
func TestAdoptInvalidIdentityName(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{name: "newline", input: "bad\nname"},
		{name: "empty", input: ""},
		{name: "spaces", input: "with spaces"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeAdopterDeps{}
			deps := fake.build()

			home := t.TempDir()
			_, err := adopter.Adopt(
				filepath.Join(home, ".gitconfig_x"),
				tc.input,
				filepath.Join(home, ".gitconfig"),
				adopter.AdoptMigrate,
				nil,
				deps,
			)
			if err == nil {
				t.Errorf("Adopt(%q) expected error, got nil", tc.input)
			}
			// No seam must have been called.
			if fake.copyFileCalled {
				t.Error("CopyFile was called despite invalid identityName")
			}
			if fake.writeIncludeIfCalled {
				t.Error("WriteIncludeIf was called despite invalid identityName")
			}
			if fake.removeCallCount != 0 {
				t.Error("BackupAndRemove was called despite invalid identityName")
			}
		})
	}
}

// TestAdoptMatchesPassedVerbatim verifies that the matches slice is forwarded
// to WriteIncludeIf unchanged (no mutation by Adopt).
func TestAdoptMatchesPassedVerbatim(t *testing.T) {
	fake := &fakeAdopterDeps{}
	deps := fake.build()

	home := t.TempDir()
	matches := []gitconfig.Match{
		{Kind: gitconfig.MatchGitdir, Value: "~/git/work/"},
		{Kind: gitconfig.MatchHasconfig, Value: "remote.*.url:git@github.com:work/**"},
	}

	_, err := adopter.Adopt(
		filepath.Join(home, ".gitconfig_work"),
		"work",
		filepath.Join(home, ".gitconfig"),
		adopter.AdoptMigrate,
		matches,
		deps,
	)
	if err != nil {
		t.Fatalf("Adopt returned error: %v", err)
	}

	if len(fake.writeIncludeIfMatch) != len(matches) {
		t.Fatalf("WriteIncludeIf matches length: got %d want %d", len(fake.writeIncludeIfMatch), len(matches))
	}
	for i, m := range matches {
		if fake.writeIncludeIfMatch[i] != m {
			t.Errorf("WriteIncludeIf matches[%d]: got %v want %v", i, fake.writeIncludeIfMatch[i], m)
		}
	}
}

// TestAdoptCopyFileErrorAbortsBeforeWriteIncludeIf verifies that a CopyFile
// error aborts without calling WriteIncludeIf (no partial includeIf written).
func TestAdoptCopyFileErrorAbortsBeforeWriteIncludeIf(t *testing.T) {
	fake := &fakeAdopterDeps{copyFileErr: errors.New("disk full")}
	deps := fake.build()

	home := t.TempDir()
	_, err := adopter.Adopt(
		filepath.Join(home, ".gitconfig_work"),
		"work",
		filepath.Join(home, ".gitconfig"),
		adopter.AdoptMigrate,
		nil,
		deps,
	)
	if err == nil {
		t.Fatal("expected error from CopyFile failure, got nil")
	}
	if fake.writeIncludeIfCalled {
		t.Error("WriteIncludeIf was called after CopyFile failure — must abort before")
	}
}

// --- ListCandidates tests ---

// TestListCandidatesFindsFragments verifies that ~/.gitconfig_work and
// ~/.gitconfig_personal are returned when both exist, and ~/.gitconfig itself
// is excluded (glob pattern naturally excludes it).
func TestListCandidatesFindsFragments(t *testing.T) {
	home := t.TempDir()

	// Create candidate fragment files.
	for _, name := range []string{".gitconfig_work", ".gitconfig_personal"} {
		if err := os.WriteFile(filepath.Join(home, name), []byte("[user]\n\tname = test\n"), 0o644); err != nil { //nolint:gosec // 0644 matches gitconfig contract; test fixture (G306)
			t.Fatalf("creating %s: %v", name, err)
		}
	}
	// Create ~/.gitconfig itself — must NOT appear in candidates.
	if err := os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[core]\n"), 0o644); err != nil { //nolint:gosec // 0644 matches gitconfig contract; test fixture (G306)
		t.Fatalf("creating .gitconfig: %v", err)
	}

	candidates, err := adopter.ListCandidates(home, nil)
	if err != nil {
		t.Fatalf("ListCandidates returned error: %v", err)
	}

	found := make(map[string]bool)
	for _, c := range candidates {
		found[filepath.Base(c)] = true
	}

	if !found[".gitconfig_work"] {
		t.Error("expected .gitconfig_work in candidates, not found")
	}
	if !found[".gitconfig_personal"] {
		t.Error("expected .gitconfig_personal in candidates, not found")
	}
	if found[".gitconfig"] {
		t.Error(".gitconfig must not appear in candidates")
	}
}

// TestListCandidatesExcludesManagedNames verifies that a name in managedNames
// is excluded from candidates.
func TestListCandidatesExcludesManagedNames(t *testing.T) {
	home := t.TempDir()

	for _, name := range []string{".gitconfig_work", ".gitconfig_personal"} {
		if err := os.WriteFile(filepath.Join(home, name), []byte("[user]\n"), 0o644); err != nil { //nolint:gosec // 0644 matches gitconfig contract; test fixture (G306)
			t.Fatalf("creating %s: %v", name, err)
		}
	}

	candidates, err := adopter.ListCandidates(home, []string{"work"})
	if err != nil {
		t.Fatalf("ListCandidates returned error: %v", err)
	}

	found := make(map[string]bool)
	for _, c := range candidates {
		found[filepath.Base(c)] = true
	}

	if found[".gitconfig_work"] {
		t.Error(".gitconfig_work must be excluded (work is in managedNames)")
	}
	if !found[".gitconfig_personal"] {
		t.Error(".gitconfig_personal must remain as a candidate")
	}
}

// TestListCandidatesEmptyWhenNoneExist verifies that an empty slice (not error)
// is returned when no ~/.gitconfig_* files exist.
func TestListCandidatesEmptyWhenNoneExist(t *testing.T) {
	home := t.TempDir()

	candidates, err := adopter.ListCandidates(home, nil)
	if err != nil {
		t.Fatalf("ListCandidates returned error: %v", err)
	}
	if len(candidates) != 0 {
		t.Errorf("expected empty slice, got %v", candidates)
	}
}

// TestListCandidatesExcludesGitconfigDFiles verifies that files under
// ~/.gitconfig.d/ are excluded from candidates even if they somehow match
// the glob (defensive: the glob shouldn't match them, but the filter guards it).
func TestListCandidatesExcludesGitconfigDFiles(t *testing.T) {
	home := t.TempDir()

	// Create a real candidate.
	if err := os.WriteFile(filepath.Join(home, ".gitconfig_work"), []byte("[user]\n"), 0o644); err != nil { //nolint:gosec // 0644 matches gitconfig contract; test fixture (G306)
		t.Fatalf("creating .gitconfig_work: %v", err)
	}
	// Create ~/.gitconfig.d/personal — should not appear.
	dDir := filepath.Join(home, ".gitconfig.d")
	if err := os.MkdirAll(dDir, 0o750); err != nil { //nolint:gosec // test-only temp dir (G301)
		t.Fatalf("mkdir .gitconfig.d: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dDir, "personal"), []byte("[user]\n"), 0o644); err != nil { //nolint:gosec // 0644 matches gitconfig contract; test fixture (G306)
		t.Fatalf("creating .gitconfig.d/personal: %v", err)
	}

	candidates, err := adopter.ListCandidates(home, nil)
	if err != nil {
		t.Fatalf("ListCandidates returned error: %v", err)
	}

	for _, c := range candidates {
		if filepath.Dir(c) == dDir {
			t.Errorf("candidate %q is inside .gitconfig.d/ — must be excluded", c)
		}
	}
	if len(candidates) != 1 {
		t.Errorf("expected 1 candidate, got %d: %v", len(candidates), candidates)
	}
}

// --- MatchIdentityName tests ---

// TestMatchIdentityNameBySuffix verifies that a path matching a known name
// by filename suffix returns that name.
func TestMatchIdentityNameBySuffix(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".gitconfig_work")
	// File must exist for Lstat to succeed (and not be a symlink).
	if err := os.WriteFile(path, []byte("[user]\n"), 0o644); err != nil { //nolint:gosec // 0644 matches gitconfig contract; test fixture (G306)
		t.Fatalf("creating fragment: %v", err)
	}

	name, err := adopter.MatchIdentityName(
		path,
		[]string{"work", "personal"},
		nil,
		nil,
	)
	if err != nil {
		t.Fatalf("MatchIdentityName returned error: %v", err)
	}
	if name != "work" {
		t.Errorf("got %q want %q", name, "work")
	}
}

// TestMatchIdentityNameByEmailFallback verifies that when the filename suffix
// does not match a known name, the fragment's email is used.
func TestMatchIdentityNameByEmailFallback(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".gitconfig_unknown")
	if err := os.WriteFile(path, []byte("[user]\n"), 0o644); err != nil { //nolint:gosec // 0644 matches gitconfig contract; test fixture (G306)
		t.Fatalf("creating fragment: %v", err)
	}

	accountEmails := map[string]string{
		"alice@work.com": "work",
	}
	readFragment := func(_ string) (gitconfig.FragmentInfo, error) {
		return gitconfig.FragmentInfo{GitEmail: "alice@work.com"}, nil
	}

	name, err := adopter.MatchIdentityName(path, []string{"work", "personal"}, accountEmails, readFragment)
	if err != nil {
		t.Fatalf("MatchIdentityName returned error: %v", err)
	}
	if name != "work" {
		t.Errorf("got %q want %q", name, "work")
	}
}

// TestMatchIdentityNameAmbiguous verifies that when neither suffix nor email
// matches, ErrAmbiguousIdentity is returned.
func TestMatchIdentityNameAmbiguous(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, ".gitconfig_unknown")
	if err := os.WriteFile(path, []byte("[user]\n"), 0o644); err != nil { //nolint:gosec // 0644 matches gitconfig contract; test fixture (G306)
		t.Fatalf("creating fragment: %v", err)
	}

	readFragment := func(_ string) (gitconfig.FragmentInfo, error) {
		return gitconfig.FragmentInfo{GitEmail: "nobody@nowhere.com"}, nil
	}

	_, err := adopter.MatchIdentityName(
		path,
		[]string{"work", "personal"},
		map[string]string{"alice@work.com": "work"},
		readFragment,
	)
	if !errors.Is(err, adopter.ErrAmbiguousIdentity) {
		t.Errorf("expected ErrAmbiguousIdentity, got %v", err)
	}
}

// TestMatchIdentityNameSymlinkRejected verifies that a symlink candidate is
// rejected (T-05.7-02-02).
func TestMatchIdentityNameSymlinkRejected(t *testing.T) {
	home := t.TempDir()

	// Create a real file to point the symlink at.
	realFile := filepath.Join(home, ".gitconfig_real")
	if err := os.WriteFile(realFile, []byte("[user]\n"), 0o644); err != nil { //nolint:gosec // 0644 matches gitconfig contract; test fixture (G306)
		t.Fatalf("creating real file: %v", err)
	}
	// Create a symlink that looks like a fragment.
	linkPath := filepath.Join(home, ".gitconfig_work")
	if err := os.Symlink(realFile, linkPath); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}

	_, err := adopter.MatchIdentityName(linkPath, []string{"work"}, nil, nil)
	if err == nil {
		t.Error("expected error for symlink candidate, got nil")
	}
	if errors.Is(err, adopter.ErrAmbiguousIdentity) {
		t.Error("symlink error should not be ErrAmbiguousIdentity — it is a distinct rejection")
	}
}
