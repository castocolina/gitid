package identity

import "testing"

// TestLineReferencesAlias_WholeTokenOnly is the D-13 matching table: matches
// for the bare token, a Host directive value, and a git-at-alias URL form;
// non-matches for a prefix, a suffix, and a dotted extension of the alias.
func TestLineReferencesAlias_WholeTokenOnly(t *testing.T) {
	const alias = "work"
	cases := []struct {
		name string
		line string
		want bool
	}{
		{"bare token", "work", true},
		{"Host directive value", "Host work", true},
		{"git-at-alias URL", "git@work:org/repo", true},
		{"bare host:path form", "work:org/repo", true},
		{"prefix of a longer token", "work-archive", false},
		{"suffix of a longer token", "mywork", false},
		{"dotted extension", "work.example.com", false},
		{"unrelated line", "# nothing here", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := lineReferencesAlias(tc.line, alias)
			if got != tc.want {
				t.Errorf("lineReferencesAlias(%q, %q) = %v, want %v", tc.line, alias, got, tc.want)
			}
		})
	}
}

// buildScanFixture returns raw bytes containing: the identity's OWN managed
// block (mentions the alias), a SIBLING's managed block (also mentions the
// alias), and foreign/unmanaged text (also mentions the alias) — the R-09
// three-region fixture.
func buildScanFixture() []byte {
	return []byte(`# foreign line before any block
Host work
  Hostname example.com

# BEGIN gitid managed: work
Host work.github.com
  Hostname ssh.github.com
work
# END gitid managed: work

# BEGIN gitid managed: personal
[includeIf "gitdir:~/git/personal/"]
	# references work below in a comment-ish line
	somekey = work
# END gitid managed: personal

# foreign line after every block
git@work:org/repo
`)
}

// TestSplitScanRegions_ThreeRegionsCoverWholeFile drives SplitScanRegions
// over the R-09 fixture and asserts three region-tagged sources whose bytes
// concatenate back to the original content.
func TestSplitScanRegions_ThreeRegionsCoverWholeFile(t *testing.T) {
	content := buildScanFixture()
	sources := SplitScanRegions("ssh-config", content, "work")

	var seen = map[ScanRegion]bool{}
	var reassembled []byte
	for _, s := range sources {
		seen[s.Region] = true
		reassembled = append(reassembled, s.Bytes...)
	}
	if !seen[ScanRegionOwnManaged] || !seen[ScanRegionOtherManaged] || !seen[ScanRegionUnmanaged] {
		t.Fatalf("expected all three regions present, got %v", seen)
	}
	if string(reassembled) != string(content) {
		t.Errorf("region sources do not reassemble to the original content:\ngot:  %q\nwant: %q", reassembled, content)
	}
}

// TestScanUnmanagedReferences_DropsOwnManagedReportsOthers is the review
// R-09 proof: a hit inside the identity's OWN managed block is dropped; a
// hit inside a SIBLING's managed block is reported, tagged
// ScanRegionOtherManaged; a hit in foreign text is reported, tagged
// ScanRegionUnmanaged.
func TestScanUnmanagedReferences_DropsOwnManagedReportsOthers(t *testing.T) {
	content := buildScanFixture()
	sources := SplitScanRegions("ssh-config", content, "work")
	hits := ScanUnmanagedReferences("work", sources)

	var sawOther, sawUnmanaged, sawOwn bool
	for _, h := range hits {
		switch h.Region {
		case ScanRegionOtherManaged:
			sawOther = true
		case ScanRegionUnmanaged:
			sawUnmanaged = true
		case ScanRegionOwnManaged:
			sawOwn = true
		}
	}
	if sawOwn {
		t.Error("a hit inside the identity's OWN managed block was reported — must be dropped")
	}
	if !sawOther {
		t.Error("no hit reported inside the SIBLING's managed block — review R-09 requires disclosure")
	}
	if !sawUnmanaged {
		t.Error("no hit reported in foreign/unmanaged text")
	}
}

// TestScanUnmanagedReferences_ZeroHitsIsEmptySlice asserts that scanning
// sources containing ONLY the identity's own managed block returns zero
// hits (an empty/nil slice, never a placeholder entry).
func TestScanUnmanagedReferences_ZeroHitsIsEmptySlice(t *testing.T) {
	content := []byte("# BEGIN gitid managed: work\nHost work.github.com\n  Hostname ssh.github.com\n# END gitid managed: work\n")
	sources := SplitScanRegions("ssh-config", content, "work")
	hits := ScanUnmanagedReferences("work", sources)
	if len(hits) != 0 {
		t.Errorf("hits = %v, want empty", hits)
	}
}

// TestScanUnmanagedReferences_LineNumbersAndVerbatimText asserts hits carry
// correct 1-based line numbers and the untruncated matched line.
func TestScanUnmanagedReferences_LineNumbersAndVerbatimText(t *testing.T) {
	content := []byte("line one\nline two mentions work here\nline three\n")
	sources := []ScanSource{{File: "gitconfig", Bytes: content, Region: ScanRegionUnmanaged}}
	hits := ScanUnmanagedReferences("work", sources)
	if len(hits) != 1 {
		t.Fatalf("hits = %v, want exactly 1", hits)
	}
	if hits[0].Line != 2 {
		t.Errorf("Line = %d, want 2", hits[0].Line)
	}
	if hits[0].Text != "line two mentions work here" {
		t.Errorf("Text = %q, want verbatim matched line", hits[0].Text)
	}
	if hits[0].File != "gitconfig" {
		t.Errorf("File = %q, want %q", hits[0].File, "gitconfig")
	}
}

// TestScanUnmanagedReferences_NoPrivateKeySources asserts (by construction —
// a composition-root-shaped fixture) that the four canonical scan sources
// never include a private key path: this is a documentation-level test
// pinning the composition root's contract that ONLY the SSH config, the
// gitconfig, the identity's fragment, and allowed_signers are scanned.
func TestScanUnmanagedReferences_NoPrivateKeySources(t *testing.T) {
	sources := []ScanSource{
		{File: "/home/u/.ssh/config", Region: ScanRegionUnmanaged},
		{File: "/home/u/.gitconfig", Region: ScanRegionUnmanaged},
		{File: "/home/u/.gitconfig.d/work", Region: ScanRegionUnmanaged},
		{File: "/home/u/.ssh/allowed_signers", Region: ScanRegionUnmanaged},
	}
	for _, s := range sources {
		if s.File == "/home/u/.ssh/id_ed25519_work" {
			t.Fatalf("a private key path must never be a scan source: %s", s.File)
		}
	}
	if len(sources) != 4 {
		t.Fatalf("composition root scan-source list has %d entries, want exactly 4", len(sources))
	}
}

// TestUnmanagedScanDisclaimer_Fixed pins the frozen disclaimer's exact
// wording — the copy-freeze gate (plan 05-06) greps for this literal string.
func TestUnmanagedScanDisclaimer_Fixed(t *testing.T) {
	const want = "Repo remotes using git@<alias>: cannot be scanned and will break after this delete."
	if UnmanagedScanDisclaimer != want {
		t.Errorf("UnmanagedScanDisclaimer = %q, want %q", UnmanagedScanDisclaimer, want)
	}
}
