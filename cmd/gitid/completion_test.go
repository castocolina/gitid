package main

// completion_test.go covers SHELL-03's three named shells (bash, zsh, fish).
// Cobra ALSO auto-registers a PowerShell completion subcommand once the root
// has subcommands — that fourth shell is left in deliberately (review R-18):
// an undocumented silent omission would be worse than the harmless extra
// shell Cobra gives us for free, so this file documents the choice here
// instead of suppressing the subcommand.
//
// Empirical finding (hypothesis -> test -> implementation, CLAUDE.md): Cobra
// V2's generated bash/zsh/fish completion scripts do NOT statically embed
// every subcommand name as a text literal — verified by running
// `go run ./cmd/gitid completion bash` and grepping its 400+ line output for
// "identity": zero matches. The scripts are DYNAMIC — at Tab-press time they
// shell back out to the compiled binary's `__complete` machinery, which is
// where the actual subcommand list lives. Asserting the identity token
// against the static script text would therefore be asserting something
// that is never true for ANY subcommand, not just this one. The real,
// observable proof that "identity" completes is driving that `__complete`
// machinery directly (TestCompletionDynamicListIncludesIdentity below) —
// confirmed with `go run ./cmd/gitid __complete ""`, which DOES list
// "identity" among the candidates.

import (
	"bytes"
	"strings"
	"testing"
)

// TestCompletionBash verifies that `gitid completion bash` executes without
// error and produces a non-empty script naming "gitid" (CLI-02).
func TestCompletionBash(t *testing.T) {
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"completion", "bash"})
	if err := root.Execute(); err != nil {
		t.Fatalf("completion bash: %v", err)
	}
	assertCompletionScript(t, "bash", buf.String())
}

// TestCompletionZsh verifies that `gitid completion zsh` executes without
// error and produces a non-empty script naming "gitid" (CLI-02).
func TestCompletionZsh(t *testing.T) {
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"completion", "zsh"})
	if err := root.Execute(); err != nil {
		t.Fatalf("completion zsh: %v", err)
	}
	assertCompletionScript(t, "zsh", buf.String())
}

// TestCompletionFish verifies that `gitid completion fish` executes without
// error and produces a non-empty script naming "gitid" (CLI-02). Fish syntax
// correctness is manual-only (fish may be absent in CI); this test asserts
// non-empty output and "gitid" presence only.
func TestCompletionFish(t *testing.T) {
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"completion", "fish"})
	if err := root.Execute(); err != nil {
		t.Fatalf("completion fish: %v", err)
	}
	assertCompletionScript(t, "fish", buf.String())
}

// TestCompletionDynamicListIncludesIdentity drives Cobra's `__complete`
// machinery directly — the SAME mechanism every generated bash/zsh/fish
// script shells back out to at Tab-press time (see the file-level comment
// above) — and asserts "identity" appears among the top-level completion
// candidates (D-01/SHELL-03).
func TestCompletionDynamicListIncludesIdentity(t *testing.T) {
	root := newRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"__complete", ""})
	if err := root.Execute(); err != nil {
		t.Fatalf("__complete: %v", err)
	}
	if !strings.Contains(buf.String(), "identity") {
		t.Errorf("dynamic completion candidates do not include 'identity'; got:\n%s", buf.String())
	}
}

// assertCompletionScript asserts script is non-empty and contains "gitid".
func assertCompletionScript(t *testing.T, shell, script string) {
	t.Helper()
	if strings.TrimSpace(script) == "" {
		t.Fatalf("completion %s produced an empty script", shell)
	}
	if !strings.Contains(script, "gitid") {
		t.Errorf("completion %s output does not contain 'gitid'; got:\n%s", shell, script)
	}
}
