package tuikit

import (
	"fmt"
	"testing"
)

// TestFrozenPropertiesCopy is the AUTHORITATIVE, byte-exact contract for
// every Global SSH "All directives" sub-tab (PROP-01 / 09.5-UI-SPEC.md
// Copywriting Contract) frozen copy constant declared in design.go's
// "Phase 9.5 properties browser copy" section. `make gate-copy-freeze` is a
// SECONDARY source-presence guard (a comment or dead declaration would
// satisfy a plain grep) — THIS test is what actually pins the value, mirror-
// ing TestFrozenGitIgnoreCopy's precedent. For the two format-string
// constants (PropsMatchCountFmt, PropsSSHNoFilterMatchFmt) the assertion
// covers the FORMATTED result of a representative call, so the placeholder
// positions are pinned too, not just the surrounding words.
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
	}

	const wantCount = 8
	if len(cases) != wantCount {
		t.Fatalf("TestFrozenPropertiesCopy covers %d constants, want %d — a row was forgotten or double-counted", len(cases), wantCount)
	}

	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q, want the frozen 09.5-UI-SPEC.md Copywriting Contract value %q", c.name, c.got, c.want)
		}
	}
}
