package main

import (
	"testing"

	"github.com/castocolina/gitid/internal/globalgit"
	"github.com/castocolina/gitid/internal/tuikit"
)

func TestSSHExplicitValuesUsesRequestedEnum(t *testing.T) {
	got, err := sshExplicitValues(
		[]string{"StrictHostKeyChecking"},
		[]tuikit.OverrideRequest{{Key: "StrictHostKeyChecking", RequestedValue: "no"}},
	)
	if err != nil {
		t.Fatalf("sshExplicitValues: %v", err)
	}
	if got["StrictHostKeyChecking"] != "no" {
		t.Fatalf("StrictHostKeyChecking = %q, want requested override %q", got["StrictHostKeyChecking"], "no")
	}
}

func TestSSHExplicitValuesRejectsUnknownEnum(t *testing.T) {
	_, err := sshExplicitValues(
		[]string{"StrictHostKeyChecking"},
		[]tuikit.OverrideRequest{{Key: "StrictHostKeyChecking", RequestedValue: "maybe"}},
	)
	if err == nil {
		t.Fatal("invalid enum override must be refused")
	}
}

func TestGitExplicitValuesUsesRequestedText(t *testing.T) {
	got, err := gitExplicitValues(
		[]string{"init.defaultBranch"},
		[]tuikit.OverrideRequest{{Key: "init.defaultBranch", RequestedValue: "develop"}},
		func(globalgit.OptionPolicy) globalgit.GateOutcome { return globalgit.GateMet },
	)
	if err != nil {
		t.Fatalf("gitExplicitValues: %v", err)
	}
	if got["init.defaultBranch"] != "develop" {
		t.Fatalf("init.defaultBranch = %q, want requested override %q", got["init.defaultBranch"], "develop")
	}
}
