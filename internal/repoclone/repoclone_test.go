package repoclone

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---- Task 1: ProviderFromURL ----

func TestProviderFromURL(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		rawURL   string
		wantHost string
		wantErr  error
	}{
		{
			name:     "https github",
			rawURL:   "https://github.com/org/repo.git",
			wantHost: "github.com",
		},
		{
			name:     "https gitlab custom",
			rawURL:   "https://gitlab.example.com/org/repo",
			wantHost: "gitlab.example.com",
		},
		{
			name:     "scp git@github",
			rawURL:   "git@github.com:org/repo.git",
			wantHost: "github.com",
		},
		{
			name:     "scp git@gitlab custom",
			rawURL:   "git@gitlab.example.com:org/repo.git",
			wantHost: "gitlab.example.com",
		},
		{
			name:     "ssh scheme with port",
			rawURL:   "ssh://git@gitlab.example.com:443/org/repo",
			wantHost: "gitlab.example.com",
		},
		{
			name:    "garbage string",
			rawURL:  "not-a-url",
			wantErr: ErrUnknownProvider,
		},
		{
			name:    "empty string",
			rawURL:  "",
			wantErr: ErrUnknownProvider,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ProviderFromURL(tc.rawURL)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("ProviderFromURL(%q): got err %v, want %v", tc.rawURL, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ProviderFromURL(%q): unexpected error: %v", tc.rawURL, err)
			}
			if got != tc.wantHost {
				t.Errorf("ProviderFromURL(%q): got %q, want %q", tc.rawURL, got, tc.wantHost)
			}
		})
	}
}

// ---- Task 1: RewriteToAlias ----

func TestRewriteToAlias(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		rawURL  string
		alias   string
		want    string
		wantErr bool
	}{
		{
			name:   "https github with .git suffix",
			rawURL: "https://github.com/org/repo.git",
			alias:  "personal.github.com",
			want:   "git@personal.github.com:org/repo.git",
		},
		{
			name:   "https github without .git suffix",
			rawURL: "https://github.com/org/repo",
			alias:  "personal.github.com",
			want:   "git@personal.github.com:org/repo",
		},
		{
			name:   "scp form with .git suffix",
			rawURL: "git@github.com:org/repo.git",
			alias:  "personal.github.com",
			want:   "git@personal.github.com:org/repo.git",
		},
		{
			name:   "scp form without .git suffix",
			rawURL: "git@github.com:org/repo",
			alias:  "personal.github.com",
			want:   "git@personal.github.com:org/repo",
		},
		{
			name:   "ssh scheme with port",
			rawURL: "ssh://git@gitlab.example.com:443/org/repo",
			alias:  "corp.gitlab.example.com",
			want:   "git@corp.gitlab.example.com:org/repo",
		},
		{
			name:    "garbage URL returns error",
			rawURL:  "not-a-url",
			alias:   "personal.github.com",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := RewriteToAlias(tc.rawURL, tc.alias)
			if tc.wantErr {
				if err == nil {
					t.Errorf("RewriteToAlias(%q, %q): expected error, got nil (result=%q)", tc.rawURL, tc.alias, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("RewriteToAlias(%q, %q): unexpected error: %v", tc.rawURL, tc.alias, err)
			}
			if got != tc.want {
				t.Errorf("RewriteToAlias(%q, %q): got %q, want %q", tc.rawURL, tc.alias, got, tc.want)
			}
			// Recipe form: must NOT contain a scheme like "https://" or "git://"
			if strings.Contains(got, "://") {
				t.Errorf("RewriteToAlias result contains scheme: %q", got)
			}
		})
	}
}

// ---- Task 1: DestPath ----

func TestDestPath(t *testing.T) {
	t.Parallel()
	home := "/home/testuser"
	baseDir := filepath.Join(home, "git")

	cases := []struct {
		name    string
		baseDir string
		client  string
		rawURL  string
		want    string
		wantErr bool
	}{
		{
			name:    "https github",
			baseDir: baseDir,
			client:  "personal",
			rawURL:  "https://github.com/org/myrepo.git",
			want:    filepath.Join(baseDir, "personal", "myrepo"),
		},
		{
			name:    "scp form",
			baseDir: baseDir,
			client:  "work",
			rawURL:  "git@github.com:org/workrepo.git",
			want:    filepath.Join(baseDir, "work", "workrepo"),
		},
		{
			name:    "ssh scheme",
			baseDir: baseDir,
			client:  "personal",
			rawURL:  "ssh://git@gitlab.example.com:443/org/myrepo",
			want:    filepath.Join(baseDir, "personal", "myrepo"),
		},
		{
			name:    "garbage URL returns error",
			baseDir: baseDir,
			client:  "personal",
			rawURL:  "not-a-url",
			wantErr: true,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := DestPath(tc.baseDir, tc.client, tc.rawURL)
			if tc.wantErr {
				if err == nil {
					t.Errorf("DestPath: expected error, got nil (result=%q)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("DestPath: unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("DestPath: got %q, want %q", got, tc.want)
			}
		})
	}
}

// ---- Task 2: Clone seam — dest-outside-base guard ----

func TestClone_DestOutsideBase(t *testing.T) {
	t.Parallel()
	const fakeHome = "/home/testuser"
	statCalled := false

	deps := Deps{
		UserHomeDir: func() (string, error) { return fakeHome, nil },
		Stat: func(_ string) (os.FileInfo, error) {
			statCalled = true
			return nil, os.ErrNotExist
		},
		Clone: func(_, _ string) ([]string, error) {
			t.Error("Clone func must NOT be called when dest is outside base")
			return nil, nil
		},
		Pull: func(_ string) ([]string, error) { return nil, nil },
	}

	cases := []struct {
		name string
		dest string
	}{
		{"absolute path outside base", "/etc/x"},
		{"path traversal via ..", filepath.Join(fakeHome, "git", "..", "..", "etc")},
		{"sibling of home", filepath.Join(fakeHome, "other")},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			statCalled = false
			_, err := Clone("https://example.com/org/repo.git", tc.dest, deps)
			if !errors.Is(err, ErrDestOutsideBase) {
				t.Errorf("Clone(%q): got err=%v, want ErrDestOutsideBase", tc.dest, err)
			}
			if statCalled {
				t.Error("Stat must NOT be called when dest is outside base (guard fires first)")
			}
		})
	}
}

// ---- Task 2: Clone seam — dest-exists guard ----

func TestClone_DestExists(t *testing.T) {
	t.Parallel()
	const fakeHome = "/home/testuser"
	cloneCalled := false

	deps := Deps{
		UserHomeDir: func() (string, error) { return fakeHome, nil },
		// Stat returns a non-nil FileInfo (dest exists)
		Stat: func(_ string) (os.FileInfo, error) { return fakeFileInfo{}, nil },
		Clone: func(_, _ string) ([]string, error) {
			cloneCalled = true
			return nil, nil
		},
		Pull: func(_ string) ([]string, error) { return nil, nil },
	}

	destPath := filepath.Join(fakeHome, "git", "personal", "myrepo")
	_, err := Clone("https://github.com/org/myrepo.git", destPath, deps)
	if !errors.Is(err, ErrDestExists) {
		t.Errorf("Clone: got err=%v, want ErrDestExists", err)
	}
	if cloneCalled {
		t.Error("Clone func must NOT be called when dest already exists")
	}
}

// ---- Task 2: Clone seam — success path (dest does not exist) ----

func TestClone_Success(t *testing.T) {
	t.Parallel()
	const fakeHome = "/home/testuser"
	var capturedURL, capturedDest string

	deps := Deps{
		UserHomeDir: func() (string, error) { return fakeHome, nil },
		Stat:        func(_ string) (os.FileInfo, error) { return nil, os.ErrNotExist },
		Clone: func(u, d string) ([]string, error) {
			capturedURL = u
			capturedDest = d
			return []string{"Cloning into 'myrepo'..."}, nil
		},
		Pull: func(_ string) ([]string, error) { return nil, nil },
	}

	destPath := filepath.Join(fakeHome, "git", "personal", "myrepo")
	lines, err := Clone("https://github.com/org/myrepo.git", destPath, deps)
	if err != nil {
		t.Fatalf("Clone: unexpected error: %v", err)
	}
	if len(lines) == 0 {
		t.Error("Clone: expected output lines, got none")
	}
	if capturedURL != "https://github.com/org/myrepo.git" {
		t.Errorf("Clone: passed wrong URL to clone func: %q", capturedURL)
	}
	if capturedDest != destPath {
		t.Errorf("Clone: passed wrong dest to clone func: %q", capturedDest)
	}
}

// ---- Task 2: Pull seam ----

func TestPull_DelegatesToDeps(t *testing.T) {
	t.Parallel()
	var capturedDest string

	deps := Deps{
		UserHomeDir: func() (string, error) { return "/home/testuser", nil },
		Stat:        func(_ string) (os.FileInfo, error) { return nil, os.ErrNotExist },
		Clone:       func(_, _ string) ([]string, error) { return nil, nil },
		Pull: func(d string) ([]string, error) {
			capturedDest = d
			return []string{"Already up to date."}, nil
		},
	}

	dest := "/home/testuser/git/personal/myrepo"
	lines, err := Pull(dest, deps)
	if err != nil {
		t.Fatalf("Pull: unexpected error: %v", err)
	}
	if len(lines) == 0 {
		t.Error("Pull: expected output lines, got none")
	}
	if capturedDest != dest {
		t.Errorf("Pull: passed wrong dest: %q", capturedDest)
	}
}

// ---- fakeFileInfo satisfies os.FileInfo for tests ----

type fakeFileInfo struct{}

func (fakeFileInfo) Name() string       { return "fake" }
func (fakeFileInfo) Size() int64        { return 0 }
func (fakeFileInfo) Mode() os.FileMode  { return 0 }
func (fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (fakeFileInfo) IsDir() bool        { return false }
func (fakeFileInfo) Sys() interface{}   { return nil }
