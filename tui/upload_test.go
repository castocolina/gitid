package tui

// upload_test.go — Tests for the gh/glab upload-assist section in copyPubkeyModel.
// Task 1 of Plan 05.7-07: TDD RED tests for AUTOUP-01.
//
// Test coverage:
//   - upload section rendered when AuthAuthenticated
//   - upload section shows not-logged-in notice when AuthNotLoggedIn
//   - upload section omitted when AuthToolNotFound
//   - 's' skips, Esc still closes (non-blocking)

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/uploader"
)

// makeTestCopyModelWithUpload builds a copyPubkeyModel with upload-assist fields set.
func makeTestCopyModelWithUpload(status uploader.AuthStatus) copyPubkeyModel {
	deps := fakeWriteTUIDeps(nil)
	m := newCopyPubkeyModel("ssh-ed25519 AAAA test@gitid", "~/.ssh/key", "github.com", deps)
	m.uploadDetected = true
	m.uploadAuthStatus = status
	m.uploadTool = uploader.ToolGH
	m.uploadToolPath = "/usr/local/bin/gh"
	return m
}

// TestCopyUploadAssistAuthenticated verifies that when AuthAuthenticated the upload-assist
// section renders per-key prompts (Upload auth key, Upload signing key).
// Requirement: AUTOUP-01 / UI-SPEC §4a.
func TestCopyUploadAssistAuthenticated(t *testing.T) {
	m := makeTestCopyModelWithUpload(uploader.AuthAuthenticated)
	view := m.view(80)

	// Must show the upload section header.
	if !strings.Contains(view, "Assisted upload") && !strings.Contains(view, "upload") {
		t.Errorf("authenticated state must show upload-assist section; view: %q", truncateString(view, 400))
	}
	// Must show per-key prompts.
	if !strings.Contains(strings.ToLower(view), "auth") {
		t.Errorf("authenticated upload-assist must show auth key upload prompt; view: %q", truncateString(view, 400))
	}
}

// TestCopyUploadAssistNotLoggedIn verifies that AuthNotLoggedIn shows the not-authenticated
// notice (no upload prompts).
func TestCopyUploadAssistNotLoggedIn(t *testing.T) {
	m := makeTestCopyModelWithUpload(uploader.AuthNotLoggedIn)
	view := m.view(80)

	// Must show a not-authenticated notice.
	if !strings.Contains(view, "not authenticated") && !strings.Contains(view, "auth login") {
		t.Errorf("not-logged-in state must show authentication notice; view: %q", truncateString(view, 400))
	}
	// Must NOT show per-key upload prompts (auth is required first).
	if strings.Contains(view, "Upload auth key?") || strings.Contains(view, "Upload signing key?") {
		t.Error("not-logged-in state must NOT show per-key upload prompts")
	}
}

// TestCopyUploadAssistToolNotFound verifies that AuthToolNotFound omits the upload
// section entirely (silent-skip per D-11).
func TestCopyUploadAssistToolNotFound(t *testing.T) {
	m := newCopyPubkeyModel("ssh-ed25519 AAAA test@gitid", "~/.ssh/key", "github.com", tuiDeps{})
	m.uploadDetected = true
	m.uploadAuthStatus = uploader.AuthToolNotFound
	// Tool not found: section must be omitted.

	view := m.view(80)

	// The upload-assist section must be absent.
	if strings.Contains(view, "Assisted upload") {
		t.Error("AuthToolNotFound must omit the upload-assist section (D-11 silent-skip)")
	}
	// Regular manual instructions should still be present.
	if !strings.Contains(view, "Copy Public Key") {
		t.Errorf("copy modal must still render its title when tool not found; view: %q", truncateString(view, 400))
	}
}

// TestCopyUploadAssistSKeySkipsNonBlocking verifies that pressing 's' skips the
// upload prompt without blocking Esc (non-blocking per D-11).
func TestCopyUploadAssistSKeySkipsNonBlocking(t *testing.T) {
	m := makeTestCopyModelWithUpload(uploader.AuthAuthenticated)
	m.uploadPending = []uploadKeyRequest{
		{title: "gitid: personal", keyType: uploader.KeyAuthentication},
		{title: "gitid: personal", keyType: uploader.KeySigning},
	}

	// Press 's' to skip the first key.
	m2, _ := m.update(newKeyMsg('s'))

	// After 's', must be able to press Esc and close the modal.
	_, escCmd := m2.update(newKeyMsg(rune(tea.KeyEscape)))
	if escCmd == nil {
		t.Error("Esc must still dispatch clearModalCmd after 's' skip (non-blocking)")
		return
	}
	msg := escCmd()
	if _, ok := msg.(clearModalMsg); !ok {
		t.Errorf("Esc after upload skip must emit clearModalMsg; got %T", msg)
	}
}
