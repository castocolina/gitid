package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestCompletionBash verifies that `gitid completion bash` executes without
// error and produces a non-empty script containing "gitid" (CLI-02).
func TestCompletionBash(t *testing.T) {
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"completion", "bash"})
	if err := root.Execute(); err != nil {
		t.Fatalf("completion bash: %v", err)
	}
	if !strings.Contains(buf.String(), "gitid") {
		t.Errorf("completion bash output does not contain 'gitid'; got:\n%s", buf.String())
	}
}

// TestCompletionZsh verifies that `gitid completion zsh` executes without
// error and produces a non-empty script containing "gitid" (CLI-02).
func TestCompletionZsh(t *testing.T) {
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"completion", "zsh"})
	if err := root.Execute(); err != nil {
		t.Fatalf("completion zsh: %v", err)
	}
	if !strings.Contains(buf.String(), "gitid") {
		t.Errorf("completion zsh output does not contain 'gitid'; got:\n%s", buf.String())
	}
}

// TestCompletionFish verifies that `gitid completion fish` executes without
// error and produces a non-empty script containing "gitid" (CLI-02).
// Fish syntax correctness is manual-only (fish may be absent in CI); this
// test asserts non-empty output and "gitid" presence only.
func TestCompletionFish(t *testing.T) {
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"completion", "fish"})
	if err := root.Execute(); err != nil {
		t.Fatalf("completion fish: %v", err)
	}
	if !strings.Contains(buf.String(), "gitid") {
		t.Errorf("completion fish output does not contain 'gitid'; got:\n%s", buf.String())
	}
}
