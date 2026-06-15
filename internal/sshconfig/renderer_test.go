package sshconfig

import (
	"strings"
	"testing"
)

// indexOf returns the byte index of substr in s, or -1 when absent. It is a
// thin helper so ordering assertions read clearly.
func indexOf(s, substr string) int {
	return strings.Index(s, substr)
}

// TestRenderHostBlock asserts SSH-01: the rendered Host block contains, in
// order, the five required directives for an aliased identity.
func TestRenderHostBlock(t *testing.T) {
	got := RenderHostBlock("work.github.com", "ssh.github.com", 443, "~/.ssh/id_ed25519_work")

	// Each required directive must appear, in this exact relative order.
	wantOrder := []string{
		"Host work.github.com",
		"Hostname ssh.github.com",
		"Port 443",
		"User git",
		"IdentityFile ~/.ssh/id_ed25519_work",
		"IdentitiesOnly yes",
	}

	prev := -1
	for _, directive := range wantOrder {
		idx := indexOf(got, directive)
		if idx == -1 {
			t.Fatalf("RenderHostBlock missing directive %q; got:\n%s", directive, got)
		}
		if idx <= prev {
			t.Fatalf("RenderHostBlock directive %q out of order; got:\n%s", directive, got)
		}
		prev = idx
	}
}

// TestRenderGlobalBlockDarwin asserts SSH-03 + Pitfall 4: on macOS the Host *
// block emits IgnoreUnknown UseKeychain before UseKeychain yes before
// AddKeysToAgent yes.
func TestRenderGlobalBlockDarwin(t *testing.T) {
	got := RenderGlobalBlock("darwin")

	if !strings.Contains(got, "Host *") {
		t.Fatalf("RenderGlobalBlock(darwin) missing 'Host *'; got:\n%s", got)
	}

	ignoreIdx := indexOf(got, "IgnoreUnknown UseKeychain")
	useIdx := indexOf(got, "UseKeychain yes")
	addIdx := indexOf(got, "AddKeysToAgent yes")

	if ignoreIdx == -1 || useIdx == -1 || addIdx == -1 {
		t.Fatalf("RenderGlobalBlock(darwin) missing keychain directives; got:\n%s", got)
	}
	if ignoreIdx >= useIdx || useIdx >= addIdx {
		t.Fatalf("RenderGlobalBlock(darwin) directive order wrong (want IgnoreUnknown < UseKeychain < AddKeysToAgent); got:\n%s", got)
	}
}

// TestRenderGlobalBlockLinux asserts SSH-03: Linux gets no UseKeychain block at
// all (empty string), since the directive is Apple-only.
func TestRenderGlobalBlockLinux(t *testing.T) {
	got := RenderGlobalBlock("linux")
	if got != "" {
		t.Fatalf("RenderGlobalBlock(linux) want empty string, got:\n%s", got)
	}
}

// TestGlobalBlockOrderedLast asserts Pitfall 5 / T-02-15: when a host block and
// the global block are composed, 'Host *' must come AFTER the specific host so
// first-match-wins does not let the wildcard override the alias.
func TestGlobalBlockOrderedLast(t *testing.T) {
	host := RenderHostBlock("work.github.com", "ssh.github.com", 443, "~/.ssh/id_ed25519_work")
	global := RenderGlobalBlock("darwin")

	composed := host + "\n" + global

	hostIdx := indexOf(composed, "Host work.github.com")
	wildcardIdx := indexOf(composed, "Host *")

	if hostIdx == -1 || wildcardIdx == -1 {
		t.Fatalf("composed config missing a host marker; got:\n%s", composed)
	}
	if wildcardIdx < hostIdx {
		t.Fatalf("'Host *' must be ordered after specific host; got:\n%s", composed)
	}
}
