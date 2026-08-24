//go:build e2e

package e2e

import (
	"context"
	"testing"
	"time"
)

func TestGitConfiguration_RealPTYStateInventory(t *testing.T) {
	home := SandboxHome(t)
	seedMinimalIdentity(t, home, "acme")
	bin := BuildBinary(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	s := startPTYAt(t, newRealCreateFlowCmd(ctx, bin, home, ""), dummyTermWidth, dummyTermHeight)
	defer s.close(t)

	uiReady(t, s)
	s.sendKey([]byte("g"), keystrokeDelay)
	mustSee(t, s, "Git identity — acme (editing existing fragment)", "standalone edit opens the compiled real Git form")
	mustSee(t, s, "user.name", "git-form-filled exposes user.name")
	mustSee(t, s, "user.email", "git-form-filled exposes user.email")
	t.Fatal("RED: complete the Phase 4 Git PTY state inventory")
}
