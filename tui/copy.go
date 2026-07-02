package tui

// copy.go — Copy-public-key modal (Plan 05).
//
// copyPubkeyModel displays the public key, copies it to the clipboard, and shows
// provider-specific upload instructions. The private key is NEVER copied —
// it is displayed as a faint read-only path only (D-13, T-05.6-14, locked).
//
// Plan 07 extension: upload-assist section (AUTOUP-01, UI-SPEC §4a).
// When gh or glab is detected and authenticated, a per-key upload prompt section
// is shown below the manual instructions. The section is non-blocking (D-11):
// 's' skips any key, Esc closes the modal at any time.
//
// Security invariants (D-13, locked):
// The ONLY value passed to clipboard.Copy is the .pub line (pubLine field).
// The private key path (privKeyPath) is displayed as faint text only.
// Only the .pub path is passed to uploader.UploadKey (T-05.7-07-04 mitigate).
// TestCopyNeverTouchesPrivateKey asserts these invariants by construction.

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/castocolina/gitid/internal/clipboard"
	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/upload"
	"github.com/castocolina/gitid/internal/uploader"
)

// uploadKeyRequest represents a single per-key upload prompt (auth or signing).
type uploadKeyRequest struct {
	title   string // "gitid: <name>" — the --title argument for gh/glab
	keyType string // uploader.KeyAuthentication or uploader.KeySigning
}

// uploadKeyResult carries the outcome of a single key upload attempt.
type uploadKeyResult struct {
	keyType string
	output  string
	err     error
}

// copyPubkeyModel is the copy-public-key modal sub-model.
//
// It displays the public key (truncated), copies it to the system clipboard,
// and shows provider-specific upload instructions. The private key path is
// displayed as faint text only — it is NEVER passed to clipboard.Copy.
//
// Plan 07 extension: upload-assist fields for AUTOUP-01 (gh/glab detect+prompt).
type copyPubkeyModel struct {
	// pubLine is the SSH public key line (.pub content). This is the ONLY value
	// ever sent to clipboard.Copy (D-13 security invariant).
	pubLine string

	// privKeyPath is the private key file path. Displayed as faint text only.
	// NEVER passed to clipboard.Copy or any write operation.
	privKeyPath string

	// provider is the hosting provider (github.com, gitlab.com, etc.) used to
	// select the correct upload.Instructions template.
	provider string

	// copied is true after init() dispatched the clipboard copy cmd.
	copied bool

	// copyErr is non-nil when the clipboard copy failed. The modal degrades
	// gracefully: shows the key for manual copy (CLIP-02).
	copyErr error

	// --- AUTOUP-01 upload-assist fields (Plan 07) ---

	// uploadDetected is true after uploader.Detect ran on modal open.
	// Controls whether the upload-assist section is shown.
	uploadDetected bool

	// uploadTool is the detected tool (gh or glab). Only valid when
	// uploadDetected=true and uploadAuthStatus != AuthToolNotFound.
	uploadTool uploader.Tool

	// uploadToolPath is the resolved binary path (e.g. /usr/local/bin/gh).
	// Only valid when uploadDetected=true and uploadAuthStatus != AuthToolNotFound.
	uploadToolPath string

	// uploadAuthStatus is the authentication state of the detected tool.
	// AuthToolNotFound means the upload-assist section is omitted entirely (D-11).
	uploadAuthStatus uploader.AuthStatus

	// uploadPending is the queue of per-key upload prompts (auth, then signing).
	// Each entry is answered by Enter (upload) or 's' (skip).
	uploadPending []uploadKeyRequest

	// uploadResults accumulates the result of each upload attempt for display.
	uploadResults []uploadKeyResult

	deps tuiDeps
}

// newCopyPubkeyModel constructs a copyPubkeyModel for the given identity.
// pubLine is the .pub content (the ONLY value that will be copied to clipboard).
// privKeyPath is the private key path — displayed as faint text, never copied.
func newCopyPubkeyModel(pubLine, privKeyPath, provider string, deps tuiDeps) copyPubkeyModel {
	if provider == "" {
		provider = "github.com"
	}
	return copyPubkeyModel{
		pubLine:     pubLine,
		privKeyPath: privKeyPath,
		provider:    provider,
		deps:        deps,
	}
}

// init dispatches the initial clipboard copy cmd and runs upload detection.
// The clipboard copy always fires. Upload detection runs via deps.uploader if
// the seam is non-nil (AuthToolNotFound = silent skip per D-11).
func (m copyPubkeyModel) init() (copyPubkeyModel, tea.Cmd) {
	m.copied = true
	cmds := []tea.Cmd{runClipboardCopyCmd(m.pubLine)}

	// Run upload-assist detection if the uploader seam is wired.
	if m.deps.uploader.LookPath != nil {
		tool, toolPath, status := uploader.Detect(m.deps.uploader)
		m.uploadDetected = true
		m.uploadTool = tool
		m.uploadToolPath = toolPath
		m.uploadAuthStatus = status

		// Pre-build the pending upload queue when authenticated (per-key confirm, D-12).
		if status == uploader.AuthAuthenticated {
			identityName := identityNameFromPath(m.privKeyPath)
			title := "gitid: " + identityName
			m.uploadPending = []uploadKeyRequest{
				{title: title, keyType: uploader.KeyAuthentication},
				{title: title, keyType: uploader.KeySigning},
			}
		}
	}

	// Security invariant: copy ONLY m.pubLine — never m.privKeyPath.
	return m, tea.Batch(cmds...)
}

// identityNameFromPath extracts the identity name from a key path.
// E.g. "~/.ssh/gitid_personal" → "personal".
func identityNameFromPath(privKeyPath string) string {
	// Find the last "/" separator.
	lastSlash := strings.LastIndex(privKeyPath, "/")
	base := privKeyPath
	if lastSlash >= 0 {
		base = privKeyPath[lastSlash+1:]
	}
	// Strip "gitid_" prefix.
	const prefix = "gitid_"
	if strings.HasPrefix(base, prefix) {
		return base[len(prefix):]
	}
	return base
}

// uploadKeyResultMsg carries the outcome of a single key upload via gh/glab.
type uploadKeyResultMsg struct {
	keyType string // KeyAuthentication or KeySigning
	output  string
	err     error
}

// update handles messages for the copy modal.
func (m copyPubkeyModel) update(msg tea.Msg) (copyPubkeyModel, tea.Cmd) {
	switch msg := msg.(type) {

	case clipboardResultMsg:
		m.copyErr = msg.err
		return m, nil

	case uploadKeyResultMsg:
		// S1016: uploadKeyResultMsg and uploadKeyResult share the same fields;
		// convert directly rather than constructing a new literal.
		m.uploadResults = append(m.uploadResults, uploadKeyResult(msg))
		// Remove the first pending item (the one just completed/skipped).
		if len(m.uploadPending) > 0 {
			m.uploadPending = m.uploadPending[1:]
		}
		return m, nil

	case tea.KeyPressMsg:
		key := msg.String()
		switch key {
		case "c":
			// Copy again — same pubLine only, never the private key.
			return m, runClipboardCopyCmd(m.pubLine)
		case "s":
			// Skip the current upload prompt (non-blocking per D-11).
			if len(m.uploadPending) > 0 {
				skipped := m.uploadPending[0]
				m.uploadPending = m.uploadPending[1:]
				m.uploadResults = append(m.uploadResults, uploadKeyResult{
					keyType: skipped.keyType,
					output:  "skipped",
					err:     nil,
				})
			}
			return m, nil
		case "enter", "u":
			// Upload the current pending key via gh/glab (per-key confirm, D-12).
			if len(m.uploadPending) > 0 && m.uploadDetected &&
				m.uploadAuthStatus == uploader.AuthAuthenticated {
				req := m.uploadPending[0]
				// Security invariant: only the .pub path is uploaded (T-05.7-07-04).
				return m, runUploadKeyCmd(m.uploadTool, m.uploadToolPath, m.privKeyPath, req.title, req.keyType, m.deps)
			}
			return m, nil
		case "esc":
			return m, clearModalCmd()
		}
	}

	return m, nil
}

// view renders the copy modal at the given terminal width.
func (m copyPubkeyModel) view(w int) string {
	mw := modalWidth(w)
	var sb strings.Builder

	// Title.
	sb.WriteString(StyleModalTitle.Render("Copy Public Key"))
	sb.WriteString("\n\n")

	// Clipboard status line.
	if m.copied && m.copyErr != nil {
		// Check for the specific ErrNoClipboard case.
		sb.WriteString(SeverityStyle(doctor.SeverityInfo).Render(
			"! clipboard copy failed [info] — key printed above, copy manually.",
		))
	} else if m.copied {
		sb.WriteString(StylePass.Render("Public key copied to clipboard."))
	} else {
		sb.WriteString(StyleBody.Render("Public key:"))
	}
	sb.WriteString("\n\n")

	// Truncated public key + "[c] copy again" hint.
	sb.WriteString(StyleFaint.Render(truncatePubLine(m.pubLine)))
	sb.WriteString("    ")
	sb.WriteString(StyleFaint.Render("[c] copy again"))
	sb.WriteString("\n\n")

	// Upload instructions.
	providerHost := strings.SplitN(m.provider, ":", 2)[0]
	instructions := upload.Instructions(providerHost)
	sb.WriteString(StyleBody.Render(instructions))
	sb.WriteString("\n")

	// Private key path line (faint, display only — NEVER COPIED).
	sb.WriteString(StyleFaint.Render("Private key path: " + m.privKeyPath + "  (never copied)"))
	sb.WriteString("\n")

	// Upload-assist section (AUTOUP-01, UI-SPEC §4a, Plan 07).
	// Shown only when tool was detected. AuthToolNotFound = omit entirely (D-11 silent-skip).
	if m.uploadDetected && m.uploadAuthStatus != uploader.AuthToolNotFound {
		sb.WriteString("\n")
		toolDisplayName := uploader.ToolName(m.uploadTool)
		switch m.uploadAuthStatus {
		case uploader.AuthAuthenticated:
			// Section header.
			sb.WriteString(StyleHeader.Render("── Assisted upload (" + toolDisplayName + " detected) ──"))
			sb.WriteString("\n\n")
			// Show completed results.
			for _, res := range m.uploadResults {
				if res.err != nil {
					sb.WriteString(SeverityStyle(doctor.SeverityError).Render("✗ " + res.keyType + " upload failed"))
					sb.WriteString("\n")
					sb.WriteString(StyleFaint.PaddingLeft(4).Render(res.output))
					sb.WriteString("\n")
				} else if res.output != "skipped" {
					sb.WriteString(StylePass.Render("✓ " + res.keyType + " key uploaded"))
					sb.WriteString("\n")
				} else {
					sb.WriteString(StyleFaint.Render("  " + res.keyType + " key skipped"))
					sb.WriteString("\n")
				}
			}
			// Show next pending prompt.
			if len(m.uploadPending) > 0 {
				req := m.uploadPending[0]
				// Command preview (shown command == run command, UI-SPEC §4a).
				preview := uploader.CommandPreview(m.uploadTool, m.uploadToolPath, m.privKeyPath, req.title, req.keyType)
				sb.WriteString(StyleLabel.Render("Upload " + req.keyType + " key via " + toolDisplayName + ":"))
				sb.WriteString("\n")
				sb.WriteString(StyleFaint.PaddingLeft(4).Render(preview))
				sb.WriteString("\n\n")
				sb.WriteString(StyleBody.Render("Upload " + req.keyType + " key? [Enter to upload / s to skip]"))
				sb.WriteString("\n")
			}

		case uploader.AuthNotLoggedIn:
			// Not-authenticated notice (no upload prompts shown).
			sb.WriteString(StyleHeader.Render("── Assisted upload (" + toolDisplayName + " detected) ──"))
			sb.WriteString("\n\n")
			sb.WriteString(SeverityStyle(doctor.SeverityInfo).Render("~ " + toolDisplayName + " detected but not authenticated. [info]"))
			sb.WriteString("\n")
			sb.WriteString(StyleFaint.Render("  Run '" + toolDisplayName + " auth login' to enable assisted upload."))
			sb.WriteString("\n")
			sb.WriteString(StyleFaint.Render("  Manual instructions above remain the default."))
			sb.WriteString("\n")
		}
	}

	return StyleModal.Width(mw).Render(sb.String())
}

// runClipboardCopyCmd constructs the async tea.Cmd that copies pubLine to the
// system clipboard. The ONLY value passed to clipboard.Copy is pubLine.
// Per PATTERNS § tui/copy.go Pattern H — keep verbatim.
//
// Security invariant (D-13, T-05.6-14): this function receives pubLine only;
// the private key path is never in scope here.
func runClipboardCopyCmd(pubLine string) tea.Cmd {
	return func() tea.Msg {
		err := clipboard.Copy(pubLine)
		return clipboardResultMsg{err: err}
	}
}

// runUploadKeyCmd dispatches the gh/glab key-upload operation through the
// injected deps.uploader seam. NO os/exec in this function (T-05.7-07-01).
//
// Security invariant (T-05.7-07-04): the .pub path is derived from privKeyPath
// by appending ".pub" — the actual public key file, NOT the private key, is
// passed to uploader.UploadKey. The private key is never in scope here.
func runUploadKeyCmd(tool uploader.Tool, toolPath, privKeyPath, title, keyType string, deps tuiDeps) tea.Cmd {
	return func() tea.Msg {
		// Derive .pub path — T-05.7-07-04: only .pub path is ever uploaded.
		pubPath := privKeyPath + ".pub"
		output, err := uploader.UploadKey(tool, toolPath, pubPath, title, keyType, deps.uploader)
		return uploadKeyResultMsg{keyType: keyType, output: output, err: err}
	}
}

// truncatePubLine truncates a public key line to at most 60 characters,
// appending "..." when truncated. Verbatim from PATTERNS § tui/copy.go.
func truncatePubLine(line string) string {
	const maxLen = 60
	if len(line) <= maxLen {
		return line
	}
	return line[:maxLen] + "..."
}
