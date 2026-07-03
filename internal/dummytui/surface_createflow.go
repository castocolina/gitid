package dummytui

import (
	"fmt"

	lipgloss "charm.land/lipgloss/v2"
)

// surface_createflow.go registers the create-flow surface (02-UX-DIRECTION.md
// §4.1, the PILOT surface) as a KEYLESS modal flow launched FROM
// identity-manager via the target-owned LaunchFrom/LaunchKey binding (review
// C3): this file alone wires the launch point — no edit to data.go or
// model.go is needed. LaunchKey "n" is allocated to create-flow in
// doc.go/02-UX-DIRECTION.md §2's key-allocation table (the single
// authority); the registration-time collision guard in registry.go rejects
// any future surface that tries to claim it.
//
// The twelve screens below mirror, byte-for-byte on labels/copy/defaults,
// the /mui mockup built in Task 1
// (.planning/design/mockup-src/src/routes/create-flow/*.route.tsx) and the
// literal recipe copy in src/data/recipeFixtures.ts — every recipe-critical
// value (Port 443, IdentitiesOnly yes, the ssh -G IdentityFile proof, the
// timestamped backup path) is kept as a byte-visible Go string constant
// here, not derived, so it stays a static, diff-able contract (matching
// recipeFixtures.ts's own "written as a literal" precedent). NO backend
// import — only bubbletea/lipgloss (DLV-05 no-backend ALLOWLIST).
//
// Each screen's Render also embeds its manifest.json "signature" — a
// screen-specific unique marker distinct from the "<surface>/<screen>"
// breadcrumb — so design_capture_test.go's TUI subtest and the PTY dummy-nav
// e2e can both assert a capture landed on the RIGHT screen, never a
// same-shaped-but-wrong-state false positive (review HIGH-3c, T-02-FP).

// Recipe-accurate literal copy (recipes/ssh-config.recipe via
// src/data/recipeFixtures.ts — the North Star; structure matches, algorithm
// is ed25519 not the gists' RSA per the recipes' own "structure, not key
// type" caveat). Identity alias "personal" / host "personal.github.com"
// matches the recipe's own worked example and recipeFixtures.ts.
const (
	cfAliasPrefix     = "personal"
	cfSSHHost         = "personal.github.com"
	cfRealHostname    = "ssh.github.com"
	cfPort            = "443"
	cfIdentityFile    = "~/.ssh/id_ed25519_personal"
	cfBlankPrefixHost = "github.com"

	cfSSHHostBlock = `Host personal.github.com
    Hostname ssh.github.com
    Port 443
    User git
    IdentityFile ~/.ssh/id_ed25519_personal
    IdentitiesOnly yes`

	cfMacGlobalsBlock = `IgnoreUnknown UseKeychain

Host *
    UseKeychain yes
    AddKeysToAgent yes`

	cfTestTmpConfig     = "/tmp/gitid-test-a1b2c3.config"
	cfTestStage1Command = "ssh -T -F " + cfTestTmpConfig + " -p " + cfPort + " -i " + cfIdentityFile + " git@" + cfRealHostname
	cfTestStage1Output  = "Hi personal! You've successfully authenticated, but GitHub does not provide shell access."
	cfTestStage2Command = "ssh -G " + cfSSHHost + " -F " + cfTestTmpConfig + " | grep identityfile"
	cfTestStage2Output  = "identityfile " + cfIdentityFile
	cfTestFailCommand   = cfTestStage1Command
	cfTestFailOutput    = "git@" + cfRealHostname + ": Permission denied (publickey)."

	cfTargetFile = "~/.ssh/config"
	cfBackupPath = "~/.ssh/config.backup.2026-07-03T03-59-12Z"

	cfSentinelBegin = "# BEGIN gitid managed: personal"
	cfSentinelEnd   = "# END gitid managed: personal"
)

var cfManagedBlockText = cfSentinelBegin + "\n" + cfSSHHostBlock + "\n" + cfSentinelEnd

// Screen-specific signatures — MUST stay byte-identical to
// .planning/design/create-flow/manifest.json's "signature" field per screen
// (review HIGH-3c: a screen-specific marker, never a generic reused string).
const (
	sigAlgoCatalog        = "SIG-ALGO-CATALOG-ED25519-DEFAULT"
	sigSSHFormEmpty       = "SIG-SSH-FORM-EMPTY"
	sigSSHFormFilled      = "SIG-SSH-FORM-FILLED-LIVE-PREVIEW"
	sigSSHFormBlankPrefix = "SIG-SSH-FORM-BLANK-PREFIX-WYSIWYG"
	sigReuseKeyVsGenerate = "SIG-REUSE-KEY-VS-GENERATE"
	sigMacosGlobalsBlock  = "SIG-MACOS-GLOBALS-BLOCK"
	sigTestStage1Direct   = "SIG-TEST-STAGE1-DIRECT"
	sigTestStage2ByAlias  = "SIG-TEST-STAGE2-BY-ALIAS"
	sigTestFail           = "SIG-TEST-FAIL"
	sigConfirmWrite       = "SIG-CONFIRM-WRITE"
	sigBackupNotice       = "SIG-BACKUP-NOTICE"
	sigResultSuccess      = "SIG-RESULT-SUCCESS"
)

// Local styles (D-02: no backend imports, so no dependency on tui/styles.go
// — a small self-contained palette mirroring 02-UX-DIRECTION.md §2's color
// semantics table: healthy=green+word, warning=yellow+word,
// error=red+word, never color alone).
var (
	styleCFHeading = lipgloss.NewStyle().Bold(true)
	styleCFDim     = lipgloss.NewStyle().Faint(true)
	styleCFSuccess = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	styleCFWarning = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
	styleCFError   = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	styleCFTitle   = lipgloss.NewStyle().Bold(true)
)

// cfAlgoEntry mirrors recipeFixtures.ts's AlgorithmCatalogEntry shape
// (KEY-01's top-5 catalog; ed25519 is best/default, KEY-03's macOS/Linux
// local-availability notes for the other four).
type cfAlgoEntry struct {
	id, security, macos, linux string
	recommended                bool
}

var cfAlgorithmCatalog = []cfAlgoEntry{
	{
		id:          "ed25519",
		recommended: true,
		security:    "Modern EdDSA curve — small keys, fast, constant-time (timing-attack resistant). The recommended default.",
		macos:       "Native (LibreSSL) — always available",
		linux:       "Native (OpenSSL) — always available",
	},
	{
		id:       "ed25519-sk",
		security: "Hardware-backed: private key material never leaves the security key; requires a physical touch to sign.",
		macos:    "Needs libfido2 + a FIDO2 security key",
		linux:    "Needs libfido2 + a FIDO2 security key",
	},
	{
		id:       "rsa-4096",
		security: "Strong at 4096 bits; widely compatible, larger keys and slower signing than ed25519.",
		macos:    "Native — always available",
		linux:    "Native — always available",
	},
	{
		id:       "ecdsa-p256",
		security: "Compact NIST P-256 curve; smaller than RSA.",
		macos:    "Native — always available",
		linux:    "Native — always available",
	},
	{
		id:       "ecdsa-sk",
		security: "Hardware-backed ECDSA variant of ed25519-sk; physical security-key touch required.",
		macos:    "Needs libfido2 + a FIDO2 security key",
		linux:    "Needs libfido2 + a FIDO2 security key",
	},
}

func init() {
	Register(SurfaceDef{
		ID:         "create-flow",
		Title:      "Create Identity",
		LaunchFrom: "identity-manager",
		LaunchKey:  "n",
		Screens: []ScreenDef{
			{ID: "algo-catalog", Keys: map[string]string{"c": "ssh-form-empty"}, Render: renderAlgoCatalog},
			{ID: "ssh-form-empty", Keys: map[string]string{"b": "ssh-form-blank-prefix", "f": "ssh-form-filled"}, Render: renderSSHFormEmpty},
			{ID: "ssh-form-blank-prefix", Render: renderSSHFormBlankPrefix},
			{ID: "ssh-form-filled", Keys: map[string]string{"r": "reuse-key-vs-generate", "m": "macos-globals-block", "t": "test-stage1-direct"}, Render: renderSSHFormFilled},
			{ID: "reuse-key-vs-generate", Render: renderReuseKeyVsGenerate},
			{ID: "macos-globals-block", Render: renderMacosGlobalsBlock},
			{ID: "test-stage1-direct", Keys: map[string]string{"a": "test-stage2-by-alias"}, Render: renderTestStage1Direct},
			{ID: "test-stage2-by-alias", Keys: map[string]string{"w": "confirm-write", "x": "test-fail"}, Render: renderTestStage2ByAlias},
			{ID: "test-fail", Render: renderTestFail},
			{ID: "confirm-write", Keys: map[string]string{"y": "backup-notice"}, Render: renderConfirmWrite},
			{ID: "backup-notice", Keys: map[string]string{"z": "result-success"}, Render: renderBackupNotice},
			{ID: "result-success", Render: renderResultSuccess},
		},
	})
}

// cfBody joins the heading, body lines, and the trailing signature marker
// into one screen body string — every render func below funnels through
// this so the signature is always present, in the same place, deterministically.
func cfBody(heading, sig string, lines ...string) string {
	all := make([]string, 0, len(lines)+2)
	all = append(all, styleCFHeading.Render(heading))
	all = append(all, lines...)
	all = append(all, "", styleCFDim.Render("["+sig+"]"))
	return lipgloss.JoinVertical(lipgloss.Left, all...)
}

func renderAlgoCatalog() string {
	var lines []string
	for _, a := range cfAlgorithmCatalog {
		label := a.id
		if a.recommended {
			label = styleCFSuccess.Render(label + " ✓ best / default")
		} else {
			label = styleCFTitle.Render(label)
		}
		lines = append(lines,
			label,
			"  "+a.security,
			"  macOS: "+a.macos+"   Linux: "+a.linux,
		)
	}
	return cfBody("1. Choose a key algorithm", sigAlgoCatalog, lines...)
}

func renderSSHFormEmpty() string {
	return cfBody("2. SSH connection details", sigSSHFormEmpty,
		"Alias prefix:   (empty)",
		"SSH Host:       (empty — auto-joins once Alias prefix is set)",
		"Real hostname:  (empty)",
		"Port:           "+cfPort+" (default)",
		"",
		"Live Host block preview: (fill in the fields to see the resulting Host block)",
	)
}

func renderSSHFormFilled() string {
	return cfBody("2. SSH connection details", sigSSHFormFilled,
		"Alias prefix:   "+cfAliasPrefix,
		"SSH Host:       "+cfSSHHost+" (auto-joined, editable)",
		"Real hostname:  "+cfRealHostname,
		"Port:           "+cfPort,
		"",
		styleCFDim.Render("Live Host block preview:"),
		cfSSHHostBlock,
	)
}

func renderSSHFormBlankPrefix() string {
	return cfBody("2. SSH connection details — blank prefix (WYSIWYG)", sigSSHFormBlankPrefix,
		"Alias prefix:   (blank)",
		"SSH Host:       "+cfBlankPrefixHost+" (provider host verbatim, no invented suffix)",
		"",
		"Resulting Host line: Host "+cfBlankPrefixHost,
	)
}

func renderReuseKeyVsGenerate() string {
	return cfBody("3. Key source", sigReuseKeyVsGenerate,
		styleCFSuccess.Render("Generate a new key")+" — gitid generates a fresh "+cfIdentityFile+" using the chosen algorithm.",
		"Reuse an existing key — point this identity at a key file that already exists on disk.",
	)
}

func renderMacosGlobalsBlock() string {
	return cfBody("macOS Keychain globals", sigMacosGlobalsBlock,
		"A one-time global Host * block stores key passphrases in the macOS Keychain and",
		"auto-adds new keys to the agent. IgnoreUnknown UseKeychain makes this a documented",
		"no-op on Linux.",
		"",
		cfMacGlobalsBlock,
	)
}

func renderTestStage1Direct() string {
	return cfBody("4. Test connectivity — stage 1: direct (provider URL, no alias)", sigTestStage1Direct,
		"$ "+cfTestStage1Command,
		styleCFSuccess.Render("✓ "+cfTestStage1Output),
		"",
		styleCFDim.Render("Runs against a throwaway temp config — live "+cfTargetFile+" is untouched."),
	)
}

func renderTestStage2ByAlias() string {
	return cfBody("4. Test connectivity — stage 2: by alias (ssh -G proof)", sigTestStage2ByAlias,
		"$ "+cfTestStage2Command,
		styleCFSuccess.Render("✓ "+cfTestStage2Output),
		"",
		styleCFDim.Render("ssh -G resolves the effective config for the alias, proving IdentitiesOnly yes"),
		styleCFDim.Render("and this key are what will actually be used — still against the temp config."),
	)
}

func renderTestFail() string {
	return cfBody("4. Test connectivity — failed", sigTestFail,
		"$ "+cfTestFailCommand,
		styleCFError.Render("✗ "+cfTestFailOutput),
		"",
		styleCFError.Render("The key was not accepted. Nothing was written — only the throwaway temp"),
		styleCFError.Render("config was exercised. Go back and check the key, alias, or algorithm."),
	)
}

func renderConfirmWrite() string {
	return cfBody("5. Confirm write", sigConfirmWrite,
		styleCFWarning.Render("! Nothing has changed yet — review below, then confirm."),
		"",
		fmt.Sprintf("Will write to %s:", cfTargetFile),
		cfManagedBlockText,
		"",
		styleCFDim.Render("gitid only owns the block between the sentinels — everything else is preserved verbatim."),
	)
}

func renderBackupNotice() string {
	return cfBody("6. Backup created", sigBackupNotice,
		styleCFSuccess.Render("✓ "+cfBackupPath),
		"",
		styleCFDim.Render("A full copy of your previous config was saved before any change was applied —"),
		styleCFDim.Render("this backup path is the undo story."),
	)
}

func renderResultSuccess() string {
	msg := fmt.Sprintf(`Identity "%s" created — %s now resolves to %s.`, cfAliasPrefix, cfSSHHost, cfIdentityFile)
	return cfBody("✓ Identity created", sigResultSuccess,
		styleCFSuccess.Render("✓ "+msg),
		"Written to "+cfTargetFile+".",
		"",
		styleCFDim.Render("To restore by hand, the backup is at "+cfBackupPath+"."),
	)
}

// cfSignatureByScreen is a lookup table screen-ID -> signature, mirroring
// manifest.json — used by surface_createflow_test.go.
var cfSignatureByScreen = map[string]string{
	"algo-catalog":          sigAlgoCatalog,
	"ssh-form-empty":        sigSSHFormEmpty,
	"ssh-form-filled":       sigSSHFormFilled,
	"ssh-form-blank-prefix": sigSSHFormBlankPrefix,
	"reuse-key-vs-generate": sigReuseKeyVsGenerate,
	"macos-globals-block":   sigMacosGlobalsBlock,
	"test-stage1-direct":    sigTestStage1Direct,
	"test-stage2-by-alias":  sigTestStage2ByAlias,
	"test-fail":             sigTestFail,
	"confirm-write":         sigConfirmWrite,
	"backup-notice":         sigBackupNotice,
	"result-success":        sigResultSuccess,
}
