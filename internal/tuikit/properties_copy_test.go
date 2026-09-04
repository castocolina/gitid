package tuikit

import (
	"fmt"
	"testing"
)

// TestFrozenPropertiesCopy is the AUTHORITATIVE, byte-exact contract for
// every Global SSH "All directives" sub-tab (PROP-01 / 09.5-UI-SPEC.md
// Copywriting Contract) frozen copy constant declared in design.go's
// "Phase 9.5 properties browser copy" section, PLUS plan 09.5-02's Global
// Git "Set keys" sub-tab constants (PROP-02) — both sub-tabs share this ONE
// table rather than a second copy test, per the doc comment on
// PropsGitSubTabLabel etc. `make gate-copy-freeze` is a SECONDARY
// source-presence guard (a comment or dead declaration would satisfy a plain
// grep) — THIS test is what actually pins the value, mirroring
// TestFrozenGitIgnoreCopy's precedent. For the format-string constants
// (PropsMatchCountFmt, PropsSSHNoFilterMatchFmt, PropsGitNoFilterMatchFmt)
// the assertion covers the FORMATTED result of a representative call, so the
// placeholder positions are pinned too, not just the surrounding words.
func TestFrozenPropertiesCopy(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"PropsSSHSubTabLabel", PropsSSHSubTabLabel, "All directives"},
		{"PropsFilterPlaceholder", PropsFilterPlaceholder, "Type to filter…"},
		{"PropsMatchCountFmt(23, 91)", fmt.Sprintf(PropsMatchCountFmt, 23, 91), "23 of 91 shown"},
		{"PropsSSHProbeFailedHeading", PropsSSHProbeFailedHeading, "! The SSH configuration could not be resolved."},
		{"PropsSSHProbeFailedBody", PropsSSHProbeFailedBody, "ssh -G could not be run against this host — re-enter the screen to retry."},
		{"PropsSSHNoFilterMatchFmt(...)", fmt.Sprintf(PropsSSHNoFilterMatchFmt, "stricthost"), `No directives match "stricthost".`},
		{"PropsCrossReferenceNote", PropsCrossReferenceNote, "Also tracked as a recommended option — see the Options tab for gitid's guidance."},
		{"PropsSSHSourceLine", PropsSSHSourceLine, "Resolved via ssh -G — reflects Include/Match precedence already applied."},
		{"PropsGitSubTabLabel", PropsGitSubTabLabel, "Set keys"},
		{"PropsGitProbeFailedHeading", PropsGitProbeFailedHeading, "! Git config could not be read."},
		{"PropsGitProbeFailedBody", PropsGitProbeFailedBody, "git config --list --show-origin failed — re-enter the screen to retry."},
		{"PropsGitNoKeysSet", PropsGitNoKeysSet, "No git config keys are set yet."},
		{"PropsGitNoFilterMatchFmt(...)", fmt.Sprintf(PropsGitNoFilterMatchFmt, "stricthost"), `No keys match "stricthost".`},
		{"PropsGitMultiValuedNoteFmt(3)", fmt.Sprintf(PropsGitMultiValuedNoteFmt, 3), "3 values are set for this key — showing the last one applied."},
		{"PropsAddCustomKeyLabel", PropsAddCustomKeyLabel, "Add custom key"},
		{"PropsGitCustomCeremonyHeadingFmt(...)", fmt.Sprintf(PropsGitCustomCeremonyHeadingFmt, "~/.gitconfig.d/00-baseline"), "Write custom Git key to ~/.gitconfig.d/00-baseline"},
		{"PropsGitCustomReceiptFmt(...)", fmt.Sprintf(PropsGitCustomReceiptFmt, "core.pager", "less -FRX"), "core.pager = less -FRX written."},
		{"PropsGitKeyInvalidFmt(...)", fmt.Sprintf(PropsGitKeyInvalidFmt, "no dot in key"), "That key/value can't be written: no dot in key"},
		{"PropsAddCustomDirectiveLabel", PropsAddCustomDirectiveLabel, "Add custom directive"},
		{"PropsSSHNameCheckFmt(...)", fmt.Sprintf(PropsSSHNameCheckFmt, "TCPKeepAlive"), "Checking 'TCPKeepAlive' against OpenSSH's known-directive list…"},
		{"PropsSSHUnknownDirectiveFmt(...)", fmt.Sprintf(PropsSSHUnknownDirectiveFmt, "NotARealDirective"), "'NotARealDirective' is not a recognized SSH directive — nothing was written."},
		{"PropsSSHProofRejectedFmt(...)", fmt.Sprintf(PropsSSHProofRejectedFmt, "Bad configuration option value"), "ssh -G rejected this value: Bad configuration option value — nothing was written."},
		{"PropsSSHCustomCeremonyHeadingFmt(...)", fmt.Sprintf(PropsSSHCustomCeremonyHeadingFmt, "~/.ssh/config"), "Write custom SSH directive to ~/.ssh/config"},
		{"PropsSSHCustomReceiptFmt(...)", fmt.Sprintf(PropsSSHCustomReceiptFmt, "TCPKeepAlive", "yes"), "TCPKeepAlive yes written."},
		{"PropsSSHPreexistingConfigErrorFmt(...)", fmt.Sprintf(PropsSSHPreexistingConfigErrorFmt, "SomeOtherDirective"), "'SomeOtherDirective' already has a problem in your current configuration — unrelated to what you just entered. Nothing was written."},
		{"PropsSSHNothingWrittenNote", PropsSSHNothingWrittenNote, "Nothing was written."},
		{"PropsSSHRecognizedDirectiveFmt(...)", fmt.Sprintf(PropsSSHRecognizedDirectiveFmt, "TCPKeepAlive"), "✓ 'TCPKeepAlive' is a recognized SSH directive."},
	}

	const wantCount = 27
	if len(cases) != wantCount {
		t.Fatalf("TestFrozenPropertiesCopy covers %d constants, want %d — a row was forgotten or double-counted", len(cases), wantCount)
	}

	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q, want the frozen 09.5-UI-SPEC.md Copywriting Contract value %q", c.name, c.got, c.want)
		}
	}
}
