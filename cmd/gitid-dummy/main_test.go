package main

import (
	"os"
	"strings"
	"testing"
)

// TestRunNonTTYRefusesToLaunch verifies that run() refuses to start the
// interactive Bubble Tea program when stdout is not a terminal (the `go test`
// environment is never a TTY), returning exit code 1 with an explanatory
// message on the error writer instead of blocking on a real TUI.
//
// Besides guarding that behaviour, this test exists so cmd/gitid-dummy is not a
// test-less main package: `go test -coverprofile ./...` collects coverage
// in-process for packages that have tests, avoiding the external `covdata`
// tool path a test-less main package would otherwise trigger.
func TestRunNonTTYRefusesToLaunch(t *testing.T) {
	errFile, err := os.CreateTemp(t.TempDir(), "stderr-*")
	if err != nil {
		t.Fatalf("creating temp error writer: %v", err)
	}
	t.Cleanup(func() { _ = errFile.Close() })

	// In the test process, os.Stdout is not a terminal, so run() must take the
	// non-TTY branch. The first argument is unused by run(); pass os.Stdout for
	// signature clarity.
	code := run(os.Stdout, errFile)
	if code != 1 {
		t.Errorf("run() with a non-TTY stdout = %d, want 1", code)
	}

	if _, err := errFile.Seek(0, 0); err != nil {
		t.Fatalf("seeking error writer: %v", err)
	}
	buf := make([]byte, 1024)
	n, _ := errFile.Read(buf)
	if got := string(buf[:n]); !strings.Contains(got, "not a terminal") {
		t.Errorf("run() non-TTY message = %q, want it to mention 'not a terminal'", got)
	}
}
