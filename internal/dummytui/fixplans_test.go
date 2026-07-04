package dummytui

import (
	"strings"
	"testing"
)

// seededFinding returns the seeded DemoFinding with the given id.
func seededFinding(t *testing.T, id string) DemoFinding {
	t.Helper()
	for _, f := range Seed().Findings {
		if f.ID == id {
			return f
		}
	}
	t.Fatalf("no seeded finding %q", id)
	return DemoFinding{}
}

func TestPlanForKeyPerms(t *testing.T) {
	plan := PlanFor(seededFinding(t, "ssh-key-perms-archived"))
	if plan.File != "~/.ssh/id_ed25519_archived" {
		t.Errorf("file = %q", plan.File)
	}
	if !strings.Contains(plan.Diff, "- mode 0644 (world-readable)") || !strings.Contains(plan.Diff, "+ mode 0600 (owner only)") {
		t.Errorf("diff = %q", plan.Diff)
	}
	if plan.Destructive != nil {
		t.Error("chmod fix is not destructive")
	}
	if plan.Result != "chmod 0600 ~/.ssh/id_ed25519_archived applied." {
		t.Errorf("result = %q", plan.Result)
	}
}

func TestPlanForContradictionIsDestructiveAndReusesFixtureDiff(t *testing.T) {
	plan := PlanFor(seededFinding(t, "ssh-identitiesonly-contradiction"))
	if plan.File != "~/.ssh/config" {
		t.Errorf("file = %q", plan.File)
	}
	// The diff reuses data.go's FixerFixPreviewLines verbatim.
	if plan.Diff != strings.Join(FixerFixPreviewLines, "\n") {
		t.Errorf("diff must be FixerFixPreviewLines verbatim; got %q", plan.Diff)
	}
	if plan.Destructive == nil {
		t.Fatal("the in-place rewrite fix must be destructive")
	}
	if plan.Destructive.ConfirmWord != "clientb.github.com" {
		t.Errorf("confirm word = %q, want the Host name", plan.Destructive.ConfirmWord)
	}
	if !strings.Contains(plan.Destructive.Warning, "cannot be undone without restoring the backup") {
		t.Errorf("warning = %q", plan.Destructive.Warning)
	}
	if plan.Result != "IdentitiesOnly set to yes on Host clientb.github.com in ~/.ssh/config." {
		t.Errorf("result = %q", plan.Result)
	}
}

func TestPlanForMissingFragment(t *testing.T) {
	plan := PlanFor(seededFinding(t, "git-includeif-missing-fragment"))
	if plan.File != "~/.gitconfig.d/legacy" {
		t.Errorf("file = %q", plan.File)
	}
	if !strings.Contains(plan.Diff, "fragment restored from template") {
		t.Errorf("diff = %q", plan.Diff)
	}
	if !strings.Contains(plan.Result, `"legacy" is complete`) {
		t.Errorf("result = %q", plan.Result)
	}
}

func TestPlanForDuplicateHostStar(t *testing.T) {
	plan := PlanFor(seededFinding(t, "ssh-duplicate-host-star"))
	if plan.File != "~/.ssh/config" {
		t.Errorf("file = %q", plan.File)
	}
	if plan.Result != "The two Host * stanzas were merged into one." {
		t.Errorf("result = %q", plan.Result)
	}
}

func TestPlanForDefaultFallback(t *testing.T) {
	plan := PlanFor(DemoFinding{HealthFinding: HealthFinding{ID: "unknown", SuggestedFix: "do the thing"}})
	if plan.File != "~/.ssh/config" || plan.Diff != "+ do the thing" || plan.Result != "Fix applied." {
		t.Errorf("default plan = %+v", plan)
	}
}
