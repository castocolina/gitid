package sshconfig

import "testing"

// TestScanDirectivesReportsKnownLineAndHostPattern seeds a directive at a
// known line inside a Host stanza and asserts the exact 1-based line number
// and the enclosing host pattern come back (the provenance-label data the
// retired AllHostStanzas shape could not provide).
func TestScanDirectivesReportsKnownLineAndHostPattern(t *testing.T) {
	content := []byte("Host personal.github.com\n  Hostname ssh.github.com\n  HashKnownHosts yes\n  IdentitiesOnly yes\n")
	hits := ScanDirectives(content, "~/.ssh/config", []string{"HashKnownHosts", "IdentitiesOnly"})
	if len(hits) != 2 {
		t.Fatalf("hits = %d, want 2", len(hits))
	}
	hk, id := hits[0], hits[1]
	if hk.Key != "HashKnownHosts" || hk.Value != "yes" || hk.Line != 3 || hk.HostPattern != "personal.github.com" {
		t.Errorf("HashKnownHosts hit = %+v, want key/value/line 3/Host personal.github.com", hk)
	}
	if id.Key != "IdentitiesOnly" || id.Line != 4 || id.HostPattern != "personal.github.com" {
		t.Errorf("IdentitiesOnly hit = %+v, want line 4 / Host personal.github.com", id)
	}
	if hk.SourcePath != "~/.ssh/config" {
		t.Errorf("SourcePath = %q, want the caller-supplied path", hk.SourcePath)
	}
}

// TestScanDirectivesTopLevelAndHostStar covers the wildcard stanza: a
// directive inside `Host *` must report `*` as its HostPattern, and a
// directive OUTSIDE any Host stanza (a top-level global) must report the empty
// pattern.
func TestScanDirectivesTopLevelAndHostStar(t *testing.T) {
	content := []byte("HashKnownHosts yes\n\nHost *\n  UseKeychain yes\n")
	hits := ScanDirectives(content, "p", []string{"HashKnownHosts", "UseKeychain"})
	if len(hits) != 2 {
		t.Fatalf("hits = %d, want 2", len(hits))
	}
	if hits[0].HostPattern != "" {
		t.Errorf("top-level directive HostPattern = %q, want empty (not stanza-nested)", hits[0].HostPattern)
	}
	if hits[0].Line != 1 {
		t.Errorf("top-level directive line = %d, want 1", hits[0].Line)
	}
	if hits[1].HostPattern != "*" {
		t.Errorf("Host * directive HostPattern = %q, want *", hits[1].HostPattern)
	}
}

// TestScanDirectivesAbsentReturnsEmpty preserves the honest scope contract: an
// absent key returns an empty slice (which callers must read as "cannot name",
// never "does not exist").
func TestScanDirectivesAbsentReturnsEmpty(t *testing.T) {
	content := []byte("Host x\n  HashKnownHosts yes\n")
	if hits := ScanDirectives(content, "p", []string{"ForwardAgent"}); len(hits) != 0 {
		t.Errorf("absent key returned %d hits, want 0", len(hits))
	}
}

// TestScanDirectivesCaseInsensitiveKeys strikes the canonical-spelling match:
// a lowercase directive in the file still reports the caller's canonical key
// casing.
func TestScanDirectivesCaseInsensitiveKeys(t *testing.T) {
	content := []byte("hashknownhosts yes\n")
	hits := ScanDirectives(content, "p", []string{"HashKnownHosts"})
	if len(hits) != 1 || hits[0].Key != "HashKnownHosts" {
		t.Fatalf("hits = %+v, want one hit with canonical key HashKnownHosts", hits)
	}
}
