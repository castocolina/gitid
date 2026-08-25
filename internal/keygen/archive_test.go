package keygen

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFixtureKey seeds a minimal private+public key pair at privPath/pubPath.
func writeFixtureKey(t *testing.T, privPath, pubPath string) {
	t.Helper()
	if err := os.WriteFile(privPath, []byte("fake-private-key-material\n"), 0o600); err != nil { //nolint:gosec // hermetic t.TempDir() fixture
		t.Fatalf("seeding private key: %v", err)
	}
	if err := os.WriteFile(pubPath, []byte("ssh-ed25519 AAAAFAKE work@gitid\n"), 0o644); err != nil { //nolint:gosec // hermetic t.TempDir() fixture
		t.Fatalf("seeding public key: %v", err)
	}
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected %s to exist: %v", path, err)
	}
}

func assertFileAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected %s to be absent, stat err = %v", path, err)
	}
}

func assertNoArchiveEntries(t *testing.T, archiveDir string) {
	t.Helper()
	entries, err := os.ReadDir(archiveDir)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		t.Fatalf("reading archive dir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected zero archive entries, got %v", entries)
	}
}

// TestCopyKeyPairToArchive_ModesAndNaming asserts the D-06 mode contract
// (0700 dir, 0600 archived private key, 0644 archived public key) and the
// "<basename>.<stamp>" / "<basename>.pub.<stamp>" naming.
func TestCopyKeyPairToArchive_ModesAndNaming(t *testing.T) {
	archiveDir := filepath.Join(t.TempDir(), "gitid-archive")
	srcDir := t.TempDir()
	privPath := filepath.Join(srcDir, "id_ed25519_work")
	pubPath := privPath + ".pub"
	writeFixtureKey(t, privPath, pubPath)

	pair, err := CopyKeyPairToArchive(archiveDir, privPath, pubPath, "170000000000000001", IgnoreCreated)
	if err != nil {
		t.Fatalf("CopyKeyPairToArchive: %v", err)
	}

	dirInfo, err := os.Stat(archiveDir)
	if err != nil {
		t.Fatalf("stat archive dir: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0o700 {
		t.Errorf("archive dir mode = %o, want 0700", perm)
	}

	privInfo, err := os.Stat(pair.PrivatePath)
	if err != nil {
		t.Fatalf("stat archived priv: %v", err)
	}
	if perm := privInfo.Mode().Perm(); perm != 0o600 {
		t.Errorf("archived priv mode = %o, want 0600", perm)
	}
	wantPriv := filepath.Join(archiveDir, "id_ed25519_work.170000000000000001")
	if pair.PrivatePath != wantPriv {
		t.Errorf("PrivatePath = %q, want %q", pair.PrivatePath, wantPriv)
	}

	pubInfo, err := os.Stat(pair.PublicPath)
	if err != nil {
		t.Fatalf("stat archived pub: %v", err)
	}
	if perm := pubInfo.Mode().Perm(); perm != 0o644 {
		t.Errorf("archived pub mode = %o, want 0644", perm)
	}
	wantPub := filepath.Join(archiveDir, "id_ed25519_work.pub.170000000000000001")
	if pair.PublicPath != wantPub {
		t.Errorf("PublicPath = %q, want %q", pair.PublicPath, wantPub)
	}
}

// TestCopyKeyPairToArchive_TightensExistingWiderDirectory proves an
// already-existing loosely-permissioned archive directory is chmod'd back
// to 0700 (review R-20).
func TestCopyKeyPairToArchive_TightensExistingWiderDirectory(t *testing.T) {
	archiveDir := t.TempDir()
	if err := os.Chmod(archiveDir, 0o777); err != nil { //nolint:gosec // deliberately loose mode so the copier must tighten it to 0700 (G302)
		t.Fatalf("chmod archive dir wide: %v", err)
	}
	srcDir := t.TempDir()
	privPath := filepath.Join(srcDir, "id_ed25519_work")
	pubPath := privPath + ".pub"
	writeFixtureKey(t, privPath, pubPath)

	if _, err := CopyKeyPairToArchive(archiveDir, privPath, pubPath, "170000000000000002", IgnoreCreated); err != nil {
		t.Fatalf("CopyKeyPairToArchive: %v", err)
	}

	info, err := os.Stat(archiveDir)
	if err != nil {
		t.Fatalf("stat archive dir: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("archive dir mode = %o, want 0700 after tighten", perm)
	}
}

// TestCopyKeyPairToArchive_RejectsSymlinkArchiveDir proves a symlinked
// archive directory is rejected before any chmod or copy (review R-20): no
// file is copied, and the symlink's TARGET mode is left unchanged (never
// chmod'd through the link).
func TestCopyKeyPairToArchive_RejectsSymlinkArchiveDir(t *testing.T) {
	realDir := t.TempDir()
	if err := os.Chmod(realDir, 0o755); err != nil { //nolint:gosec // test fixture: a distinguishable non-default mode to prove it stays unchanged (G302)
		t.Fatalf("chmod real dir: %v", err)
	}
	symlinkPath := filepath.Join(t.TempDir(), "gitid-archive-link")
	if err := os.Symlink(realDir, symlinkPath); err != nil {
		t.Fatalf("creating symlink: %v", err)
	}
	srcDir := t.TempDir()
	privPath := filepath.Join(srcDir, "id_ed25519_work")
	pubPath := privPath + ".pub"
	writeFixtureKey(t, privPath, pubPath)

	_, err := CopyKeyPairToArchive(symlinkPath, privPath, pubPath, "170000000000000003", IgnoreCreated)
	if err == nil {
		t.Fatal("expected an error when the archive directory path is a symlink")
	}
	entries, rerr := os.ReadDir(realDir)
	if rerr != nil {
		t.Fatalf("reading symlink target: %v", rerr)
	}
	if len(entries) != 0 {
		t.Errorf("expected no files copied into the symlink target, got %v", entries)
	}
	info, serr := os.Lstat(realDir)
	if serr != nil {
		t.Fatalf("lstat symlink target: %v", serr)
	}
	if perm := info.Mode().Perm(); perm != 0o755 {
		t.Errorf("symlink target mode changed: got %o, want 0755 (unchanged)", perm)
	}
}

// TestCopyKeyPairToArchive_LeavesSourcesUntouched proves CopyKeyPairToArchive
// never removes its sources — the copy-only half of review R-03's split.
func TestCopyKeyPairToArchive_LeavesSourcesUntouched(t *testing.T) {
	archiveDir := t.TempDir()
	srcDir := t.TempDir()
	privPath := filepath.Join(srcDir, "id_ed25519_work")
	pubPath := privPath + ".pub"
	writeFixtureKey(t, privPath, pubPath)

	if _, err := CopyKeyPairToArchive(archiveDir, privPath, pubPath, "170000000000000004", IgnoreCreated); err != nil {
		t.Fatalf("CopyKeyPairToArchive: %v", err)
	}
	assertFileExists(t, privPath)
	assertFileExists(t, pubPath)
}

// TestMoveKeyPairToArchive_RemovesSources proves MoveKeyPairToArchive
// removes both sources on success — the move half of review R-03's split.
func TestMoveKeyPairToArchive_RemovesSources(t *testing.T) {
	archiveDir := t.TempDir()
	srcDir := t.TempDir()
	privPath := filepath.Join(srcDir, "id_ed25519_work")
	pubPath := privPath + ".pub"
	writeFixtureKey(t, privPath, pubPath)

	if _, err := MoveKeyPairToArchive(archiveDir, privPath, pubPath, "170000000000000005", os.Remove, IgnoreCreated); err != nil {
		t.Fatalf("MoveKeyPairToArchive: %v", err)
	}
	assertFileAbsent(t, privPath)
	assertFileAbsent(t, pubPath)
}

// TestMoveKeyPairToArchive_SecondSourceRemovalFailure drives a deterministic
// failure removing the SECOND source (the public key) and asserts: the
// archive copies exist, the error wraps ErrArchiveIncomplete and names the
// surviving source, the returned ArchivedPair carries both archive paths,
// and — the primitive's half of review R3-01 — BOTH onCreated calls
// happened before the first remove call, proven by one shared ordered log.
func TestMoveKeyPairToArchive_SecondSourceRemovalFailure(t *testing.T) {
	archiveDir := t.TempDir()
	srcDir := t.TempDir()
	privPath := filepath.Join(srcDir, "id_ed25519_work")
	pubPath := privPath + ".pub"
	writeFixtureKey(t, privPath, pubPath)

	var log []string
	onCreated := func(path string) error {
		log = append(log, "created:"+filepath.Base(path))
		return nil
	}
	remove := func(path string) error {
		log = append(log, "remove:"+filepath.Base(path))
		if path == pubPath {
			return errors.New("boom: permission denied")
		}
		return os.Remove(path)
	}

	pair, err := MoveKeyPairToArchive(archiveDir, privPath, pubPath, "170000000000000006", remove, onCreated)
	if err == nil {
		t.Fatal("expected an error when the second source removal fails")
	}
	if !errors.Is(err, ErrArchiveIncomplete) {
		t.Errorf("error = %v, want it to wrap ErrArchiveIncomplete", err)
	}
	if !strings.Contains(err.Error(), pubPath) {
		t.Errorf("error = %v, want it to name the surviving source %q", err, pubPath)
	}
	if pair.PrivatePath == "" || pair.PublicPath == "" {
		t.Fatalf("expected both archive paths in the returned pair, got %+v", pair)
	}
	assertFileExists(t, pair.PrivatePath)
	assertFileExists(t, pair.PublicPath)
	assertFileExists(t, pubPath)  // removal failed — source survives
	assertFileAbsent(t, privPath) // removal succeeded — source is gone

	firstRemoveIdx, lastCreatedIdx, createdCount := -1, -1, 0
	for i, e := range log {
		if strings.HasPrefix(e, "created:") {
			lastCreatedIdx = i
			createdCount++
		}
		if strings.HasPrefix(e, "remove:") && firstRemoveIdx == -1 {
			firstRemoveIdx = i
		}
	}
	if firstRemoveIdx == -1 || lastCreatedIdx == -1 || lastCreatedIdx > firstRemoveIdx {
		t.Errorf("expected both onCreated calls before the first remove call; log = %v", log)
	}
	if createdCount != 2 {
		t.Errorf("expected exactly 2 onCreated calls, got %d; log = %v", createdCount, log)
	}
}

// TestMoveKeyPairToArchive_ObserverErrorAbortsBeforeRemoval proves an
// onCreated that errors on the SECOND copy aborts the archive: no archive
// copy remains, both sources are untouched, and remove is NEVER called — a
// journal that cannot record the path must stop the destructive half, not
// merely be surprised by it afterwards (review R3-01).
func TestMoveKeyPairToArchive_ObserverErrorAbortsBeforeRemoval(t *testing.T) {
	archiveDir := t.TempDir()
	srcDir := t.TempDir()
	privPath := filepath.Join(srcDir, "id_ed25519_work")
	pubPath := privPath + ".pub"
	writeFixtureKey(t, privPath, pubPath)

	callCount := 0
	onCreated := func(string) error {
		callCount++
		if callCount == 2 {
			return errors.New("journal write failed")
		}
		return nil
	}
	removeCalled := false
	remove := func(path string) error {
		removeCalled = true
		return os.Remove(path)
	}

	_, err := MoveKeyPairToArchive(archiveDir, privPath, pubPath, "170000000000000007", remove, onCreated)
	if err == nil {
		t.Fatal("expected a non-nil error when onCreated rejects the second copy")
	}
	if removeCalled {
		t.Error("remove must never be called when the observer aborts the archive")
	}
	assertFileExists(t, privPath)
	assertFileExists(t, pubPath)
	assertNoArchiveEntries(t, archiveDir)
}

// TestArchivers_NilObserverRefused proves both CopyKeyPairToArchive and
// MoveKeyPairToArchive refuse a nil onCreated with ErrNoCreatedObserver
// before any copy or removal — archiving without a way to undo it is the
// defect (review R3-01).
func TestArchivers_NilObserverRefused(t *testing.T) {
	t.Run("copy", func(t *testing.T) {
		archiveDir := t.TempDir()
		srcDir := t.TempDir()
		privPath := filepath.Join(srcDir, "id_ed25519_work")
		pubPath := privPath + ".pub"
		writeFixtureKey(t, privPath, pubPath)

		_, err := CopyKeyPairToArchive(archiveDir, privPath, pubPath, "170000000000000008", nil)
		if !errors.Is(err, ErrNoCreatedObserver) {
			t.Errorf("error = %v, want ErrNoCreatedObserver", err)
		}
		assertNoArchiveEntries(t, archiveDir)
		assertFileExists(t, privPath)
		assertFileExists(t, pubPath)
	})

	t.Run("move", func(t *testing.T) {
		archiveDir := t.TempDir()
		srcDir := t.TempDir()
		privPath := filepath.Join(srcDir, "id_ed25519_work")
		pubPath := privPath + ".pub"
		writeFixtureKey(t, privPath, pubPath)

		removeCalled := false
		remove := func(path string) error { removeCalled = true; return os.Remove(path) }
		_, err := MoveKeyPairToArchive(archiveDir, privPath, pubPath, "170000000000000009", remove, nil)
		if !errors.Is(err, ErrNoCreatedObserver) {
			t.Errorf("error = %v, want ErrNoCreatedObserver", err)
		}
		if removeCalled {
			t.Error("remove must never be called when onCreated is nil")
		}
		assertNoArchiveEntries(t, archiveDir)
		assertFileExists(t, privPath)
		assertFileExists(t, pubPath)
	})
}

// TestCopyKeyPairToArchive_DestinationCollisionIsHardError proves an
// existing destination archive path is a hard error: the primitive's half
// of the collision contract (review R2-05). No source is touched and the
// pre-existing destination is byte-unchanged. The composition-root retry
// that consumes this error is proven in plan 05-03 Task 2.
func TestCopyKeyPairToArchive_DestinationCollisionIsHardError(t *testing.T) {
	archiveDir := t.TempDir()
	srcDir := t.TempDir()
	privPath := filepath.Join(srcDir, "id_ed25519_work")
	pubPath := privPath + ".pub"
	writeFixtureKey(t, privPath, pubPath)

	const stamp = "170000000000000010"
	preexisting := filepath.Join(archiveDir, "id_ed25519_work."+stamp)
	preexistingContent := []byte("do-not-overwrite")
	if err := os.WriteFile(preexisting, preexistingContent, 0o600); err != nil { //nolint:gosec // hermetic t.TempDir() fixture
		t.Fatalf("seeding pre-existing destination: %v", err)
	}

	_, err := CopyKeyPairToArchive(archiveDir, privPath, pubPath, stamp, IgnoreCreated)
	if err == nil {
		t.Fatal("expected an error when the destination already exists")
	}
	assertFileExists(t, privPath)
	assertFileExists(t, pubPath)
	got, rerr := os.ReadFile(preexisting) //nolint:gosec // hermetic t.TempDir() fixture
	if rerr != nil {
		t.Fatalf("reading pre-existing destination: %v", rerr)
	}
	if string(got) != string(preexistingContent) {
		t.Errorf("pre-existing destination was modified; got %q, want %q", got, preexistingContent)
	}
}

// TestCopyKeyPairToArchive_AbsentPublicHalf proves archiving a pair with an
// empty pubPath succeeds and reports an empty ArchivedPair.PublicPath.
func TestCopyKeyPairToArchive_AbsentPublicHalf(t *testing.T) {
	archiveDir := t.TempDir()
	srcDir := t.TempDir()
	privPath := filepath.Join(srcDir, "id_ed25519_work")
	if err := os.WriteFile(privPath, []byte("priv-material\n"), 0o600); err != nil { //nolint:gosec // hermetic t.TempDir() fixture
		t.Fatalf("seeding priv key: %v", err)
	}

	pair, err := CopyKeyPairToArchive(archiveDir, privPath, "", "170000000000000011", IgnoreCreated)
	if err != nil {
		t.Fatalf("CopyKeyPairToArchive: %v", err)
	}
	if pair.PublicPath != "" {
		t.Errorf("PublicPath = %q, want empty", pair.PublicPath)
	}
	if pair.PrivatePath == "" {
		t.Error("PrivatePath must be non-empty")
	}
}

// TestRemoveArchivedPair_OutsideDirRejected proves RemoveArchivedPair
// refuses a path not contained in archiveDir (review R-10) and leaves it
// untouched.
func TestRemoveArchivedPair_OutsideDirRejected(t *testing.T) {
	archiveDir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "not-archived")
	if err := os.WriteFile(outside, []byte("x"), 0o600); err != nil { //nolint:gosec // hermetic t.TempDir() fixture
		t.Fatalf("seeding outside file: %v", err)
	}

	err := RemoveArchivedPair(archiveDir, ArchivedPair{PrivatePath: outside})
	if err == nil {
		t.Fatal("expected an error removing a path outside the archive directory")
	}
	assertFileExists(t, outside)
}

// TestRemoveArchivedPair_AbsentPathTolerated proves RemoveArchivedPair
// tolerates an already-absent archive path.
func TestRemoveArchivedPair_AbsentPathTolerated(t *testing.T) {
	archiveDir := t.TempDir()
	absent := filepath.Join(archiveDir, "already-gone")

	if err := RemoveArchivedPair(archiveDir, ArchivedPair{PrivatePath: absent}); err != nil {
		t.Errorf("expected an already-absent archive path to be tolerated, got: %v", err)
	}
}

// TestRemoveArchivedPair_RemovesExactPaths proves RemoveArchivedPair removes
// exactly the paths carried in the pair.
func TestRemoveArchivedPair_RemovesExactPaths(t *testing.T) {
	archiveDir := t.TempDir()
	priv := filepath.Join(archiveDir, "id_ed25519_work.170000000000000012")
	pub := filepath.Join(archiveDir, "id_ed25519_work.pub.170000000000000012")
	if err := os.WriteFile(priv, []byte("priv"), 0o600); err != nil { //nolint:gosec // hermetic t.TempDir() fixture
		t.Fatalf("seeding priv: %v", err)
	}
	if err := os.WriteFile(pub, []byte("pub"), 0o644); err != nil { //nolint:gosec // hermetic t.TempDir() fixture
		t.Fatalf("seeding pub: %v", err)
	}

	if err := RemoveArchivedPair(archiveDir, ArchivedPair{PrivatePath: priv, PublicPath: pub}); err != nil {
		t.Fatalf("RemoveArchivedPair: %v", err)
	}
	assertFileAbsent(t, priv)
	assertFileAbsent(t, pub)
}

// TestCopyExclusive_RefusesExistingDestination pins copyExclusive's one
// load-bearing property directly (review R2-07): an existing destination is
// a hard error, never an overwrite. The mirrored assertion lives in
// internal/filewriter's own tests (TestCopyFileExclusiveRefusesExistingDestination)
// so the two implementations of the idiom cannot drift on this property.
func TestCopyExclusive_RefusesExistingDestination(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	if err := os.WriteFile(src, []byte("source-bytes"), 0o600); err != nil { //nolint:gosec // hermetic t.TempDir() fixture
		t.Fatalf("seeding src: %v", err)
	}
	dst := filepath.Join(dir, "dst")
	preexisting := []byte("do-not-overwrite")
	if err := os.WriteFile(dst, preexisting, 0o600); err != nil { //nolint:gosec // hermetic t.TempDir() fixture
		t.Fatalf("seeding dst: %v", err)
	}

	err := copyExclusive(src, dst, 0o600)
	if err == nil || !os.IsExist(err) {
		t.Fatalf("copyExclusive with an existing destination = %v, want an os.IsExist error", err)
	}
	got, rerr := os.ReadFile(dst) //nolint:gosec // hermetic t.TempDir() fixture
	if rerr != nil {
		t.Fatalf("reading dst: %v", rerr)
	}
	if string(got) != string(preexisting) {
		t.Errorf("dst content changed; got %q, want %q", got, preexisting)
	}
}
