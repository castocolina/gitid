package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/doctor"
	"github.com/castocolina/gitid/internal/tuikit"
)

func TestDoctorAliasMatchesHealthAndIsHidden(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	var health, doctor, help bytes.Buffer
	var healthCode, doctorCode int
	for _, tc := range []struct {
		args []string
		out  *bytes.Buffer
	}{
		{[]string{"health"}, &health},
		{[]string{"doctor"}, &doctor},
		{[]string{"--help"}, &help},
	} {
		root := newRootCmd()
		root.SetArgs(tc.args)
		root.SetOut(tc.out)
		code := exitStatusOf(root.Execute())
		switch tc.out {
		case &health:
			healthCode = code
		case &doctor:
			doctorCode = code
		}
	}
	if healthCode != doctorCode {
		t.Errorf("doctor exit code = %d, want health exit code %d", doctorCode, healthCode)
	}
	if health.String() != doctor.String() {
		t.Errorf("doctor output differs from health:\nhealth: %q\ndoctor: %q", health.String(), doctor.String())
	}
	if strings.Contains(help.String(), "doctor") {
		t.Errorf("hidden doctor appeared in root help:\n%s", help.String())
	}
}

func TestDoctorAliasFixForwardsFlagsUnchanged(t *testing.T) {
	home := t.TempDir()
	configPath := seedFlagshipFixture(t, home)
	seedInstalledBaseline(t, home)
	t.Setenv("HOME", home)

	var output bytes.Buffer
	root := newRootCmd()
	root.SetArgs([]string{"doctor", "--fix", "--yes"})
	root.SetOut(&output)
	if code := exitStatusOf(root.Execute()); code != 1 {
		t.Fatalf("doctor --fix --yes exit = %d, want 1\n%s", code, output.String())
	}
	if got := readFile(t, configPath); !strings.Contains(got, "IdentitiesOnly yes # deliberately loose") {
		t.Fatalf("doctor --fix --yes did not apply flagship fix:\n%s", got)
	}
}

func TestDoctorAliasFixWithoutYesPrompts(t *testing.T) {
	var prompted, applied bool
	withFixScanOverride(t, func(string) ([]doctor.Finding, []tuikit.DemoFinding) {
		if applied {
			return nil, nil
		}
		return syntheticFinding(doctor.Finding{Family: doctor.FamilyCoherence, Severity: doctor.SeverityError, Title: "prompted fix", Fix: &doctor.FixDescriptor{Summary: "prompted fix", Fn: func() error { applied = true; return nil }}})
	})
	root := newRootCmd()
	var output bytes.Buffer
	root.SetArgs([]string{"doctor", "--fix"})
	root.SetIn(strings.NewReader("n\n"))
	root.SetOut(&output)
	if code := exitStatusOf(root.Execute()); code != 2 {
		t.Fatalf("doctor --fix exit = %d, want 2", code)
	}
	prompted = strings.Contains(output.String(), "Apply \"prompted fix\"? [y/N]")
	if !prompted || applied {
		t.Errorf("doctor --fix prompt/applied = %t/%t, output:\n%s", prompted, applied, output.String())
	}
}
