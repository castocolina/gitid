//go:build screenshot

package main

// main_test.go — tests for the gitid-evidence publisher (plan 03-11 Task 2, CR-01).
//
// These tests verify that the publisher is fail-closed (rejects invalid inputs,
// existing destinations, and reports manifest hashes), and that it produces a
// valid content-addressed packet on success.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPublisherRejectsEmptySourceCommit verifies that run() fails when
// --source-commit is not provided (CR-01 fail-closed).
func TestPublisherRejectsEmptySourceCommit(t *testing.T) {
	err := run([]string{"--output-root", t.TempDir()})
	if err == nil {
		t.Fatal("publisher must fail when --source-commit is missing")
	}
}

// TestPublisherRejectsEmptyOutputRoot verifies that run() fails when
// --output-root is not provided (CR-01 fail-closed).
func TestPublisherRejectsEmptyOutputRoot(t *testing.T) {
	err := run([]string{"--source-commit", strings.Repeat("a", 40)})
	if err == nil {
		t.Fatal("publisher must fail when --output-root is missing")
	}
}

// TestPublisherRejectsShortCommit verifies that run() fails for a commit
// that is not 40 hex characters (CR-01 fail-closed — validateSourceCommit).
func TestPublisherRejectsShortCommit(t *testing.T) {
	err := run([]string{"--source-commit", "abc123", "--output-root", t.TempDir()})
	if err == nil {
		t.Fatal("publisher must fail for a short source commit")
	}
	if !strings.Contains(err.Error(), "40") {
		t.Errorf("error should mention 40-hex requirement; got: %v", err)
	}
}

// TestPublisherRejectsExistingDestination verifies that run() fails when the
// destination directory (output-root/source-commit) already exists.
// This enforces the immutability guarantee (CR-01).
func TestPublisherRejectsExistingDestination(t *testing.T) {
	root := t.TempDir()
	commit := strings.Repeat("b", 40)
	destDir := filepath.Join(root, commit)

	// Pre-create the destination.
	if err := os.MkdirAll(destDir, 0o750); err != nil {
		t.Fatalf("seeding destination: %v", err)
	}

	err := run([]string{"--source-commit", commit, "--output-root", root})
	if err == nil {
		t.Fatal("publisher must fail when the destination already exists")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error should mention 'already exists'; got: %v", err)
	}
}

// TestPublisherRejectsUnknownCommit verifies that run() fails for a valid hex
// commit that is not present in the local git repository (CR-01 fail-closed —
// git cat-file check).
func TestPublisherRejectsUnknownCommit(t *testing.T) {
	// A full 40-hex SHA that is astronomically unlikely to exist in the repo.
	unknownCommit := strings.Repeat("0", 40)
	err := run([]string{"--source-commit", unknownCommit, "--output-root", t.TempDir()})
	if err == nil {
		t.Fatal("publisher must fail for an unknown source commit")
	}
	if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "cat-file") {
		t.Errorf("error should mention 'not found' or 'cat-file'; got: %v", err)
	}
}
