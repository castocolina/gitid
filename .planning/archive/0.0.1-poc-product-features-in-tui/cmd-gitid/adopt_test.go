package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/adopter"
	"github.com/castocolina/gitid/internal/gitconfig"
)

// fakeMigrateAdoptDeps returns a deps that records calls and simulates success.
type fakeMigrateAdoptDeps struct {
	copyCalled         bool
	copyDst            string
	writeIncludeIfID   string
	writeIncludeIfFrag string
}

func (f *fakeMigrateAdoptDeps) build() adopter.Deps {
	return adopter.Deps{
		ReadFile: func(_ string) ([]byte, error) { return nil, os.ErrNotExist },
		WriteFile: func(_ string, _ []byte, _ os.FileMode) (string, error) {
			return "", nil
		},
		CopyFile: func(_ string, dst string) error {
			f.copyCalled = true
			f.copyDst = dst
			return nil
		},
		BackupAndRemove: func(_ string) (string, error) { return "", nil },
		WriteIncludeIf: func(id, fragPath string, _ []gitconfig.Match) (string, error) {
			f.writeIncludeIfID = id
			f.writeIncludeIfFrag = fragPath
			return "", nil
		},
		ReadFragment:   func(_ string) (gitconfig.FragmentInfo, error) { return gitconfig.FragmentInfo{}, nil },
		ListCandidates: func(_ string) ([]string, error) { return nil, nil },
	}
}

// TestRunAdopt_MigrateMethod verifies that runAdopt calls adopter.Adopt(migrate)
// and prints the result steps when --method migrate is used.
func TestRunAdopt_MigrateMethod(t *testing.T) {
	rec := &fakeMigrateAdoptDeps{}

	home := t.TempDir()
	fragPath := filepath.Join(home, ".gitconfig_work")
	if err := os.WriteFile(fragPath, []byte("[user]\n\tname = Work\n"), 0o644); err != nil { //nolint:gosec // test fixture — gitconfig fragments are 0644 (G306)
		t.Fatalf("seeding fragment: %v", err)
	}
	// Override UserHomeDir so gitconfigPath resolves inside the temp dir.
	t.Setenv("HOME", home)

	var buf bytes.Buffer
	err := runAdopt(&buf, fragPath, "migrate", "work", true, func() (adopter.Deps, error) {
		return rec.build(), nil
	})
	if err != nil {
		t.Fatalf("runAdopt returned error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "work") {
		t.Errorf("expected identity name 'work' in output; got: %s", output)
	}
	if !rec.copyCalled {
		t.Error("expected CopyFile to be called for migrate method")
	}
	if rec.writeIncludeIfID != "work" {
		t.Errorf("WriteIncludeIf called with id %q, want 'work'", rec.writeIncludeIfID)
	}
}

// TestRunAdopt_ReferenceMethod verifies that --method reference skips the copy
// and calls WriteIncludeIf with the original source path.
func TestRunAdopt_ReferenceMethod(t *testing.T) {
	rec := &fakeMigrateAdoptDeps{}

	home := t.TempDir()
	fragPath := filepath.Join(home, ".gitconfig_work")
	if err := os.WriteFile(fragPath, []byte("[user]\n\tname = Work\n"), 0o644); err != nil { //nolint:gosec // test fixture — gitconfig fragments are 0644 (G306)
		t.Fatalf("seeding fragment: %v", err)
	}
	t.Setenv("HOME", home)

	var buf bytes.Buffer
	err := runAdopt(&buf, fragPath, "reference", "work", true, func() (adopter.Deps, error) {
		return rec.build(), nil
	})
	if err != nil {
		t.Fatalf("runAdopt returned error: %v", err)
	}

	if rec.copyCalled {
		t.Error("CopyFile must NOT be called for reference method")
	}
	if rec.writeIncludeIfFrag != fragPath {
		t.Errorf("WriteIncludeIf fragment path = %q, want %q", rec.writeIncludeIfFrag, fragPath)
	}
}

// TestRunAdopt_DeriveNameFromFilename verifies that when --name is absent the
// identity name is derived from the .gitconfig_<suffix> filename.
func TestRunAdopt_DeriveNameFromFilename(t *testing.T) {
	rec := &fakeMigrateAdoptDeps{}

	home := t.TempDir()
	fragPath := filepath.Join(home, ".gitconfig_personal")
	if err := os.WriteFile(fragPath, []byte("[user]\n\tname = P\n"), 0o644); err != nil { //nolint:gosec // test fixture — gitconfig fragments are 0644 (G306)
		t.Fatalf("seeding fragment: %v", err)
	}
	t.Setenv("HOME", home)

	var buf bytes.Buffer
	// No --name flag: name="" so it must derive from filename.
	err := runAdopt(&buf, fragPath, "migrate", "", true, func() (adopter.Deps, error) {
		return rec.build(), nil
	})
	if err != nil {
		t.Fatalf("runAdopt returned error: %v", err)
	}

	if rec.writeIncludeIfID != "personal" {
		t.Errorf("expected name 'personal' derived from filename; got %q", rec.writeIncludeIfID)
	}
}

// TestRunAdopt_NoMatchingName verifies that a fragment whose filename does not
// follow .gitconfig_<name> AND --name is empty returns a clear error.
func TestRunAdopt_NoMatchingName(t *testing.T) {
	home := t.TempDir()
	fragPath := filepath.Join(home, "myconfig")
	if err := os.WriteFile(fragPath, []byte("[user]\n"), 0o644); err != nil { //nolint:gosec // test fixture — gitconfig fragments are 0644 (G306)
		t.Fatalf("seeding fragment: %v", err)
	}
	t.Setenv("HOME", home)

	var buf bytes.Buffer
	err := runAdopt(&buf, fragPath, "migrate", "", true, func() (adopter.Deps, error) {
		return adopter.Deps{}, nil
	})
	if err == nil {
		t.Fatal("expected error when name cannot be derived, got nil")
	}
	if !strings.Contains(err.Error(), "cannot derive identity name") {
		t.Errorf("expected 'cannot derive identity name' in error; got: %v", err)
	}
}
