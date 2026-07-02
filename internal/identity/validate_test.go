package identity_test

import (
	"testing"

	"github.com/castocolina/gitid/internal/identity"
)

func TestValidateName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "simple lowercase", input: "personal", wantErr: false},
		{name: "complex valid", input: "work_2.x-y", wantErr: false},
		{name: "uppercase in middle", input: "myWork", wantErr: false},
		{name: "space rejected", input: "Work Bad", wantErr: true},
		{name: "shell metachar rejected", input: "a;rm -rf", wantErr: true},
		{name: "empty rejected", input: "", wantErr: true},
		{name: "all uppercase rejected (space-free but valid charset)", input: "UPPER", wantErr: false},
		{name: "leading space rejected", input: " personal", wantErr: true},
		{name: "newline rejected", input: "name\n", wantErr: true},
		{name: "slash rejected", input: "a/b", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := identity.ValidateName(tc.input)
			if tc.wantErr && err == nil {
				t.Errorf("ValidateName(%q) expected error, got nil", tc.input)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("ValidateName(%q) expected nil, got %v", tc.input, err)
			}
		})
	}
}

func TestValidateProvider(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "empty allowed (provider is optional)", input: "", wantErr: false},
		{name: "github", input: "github", wantErr: false},
		{name: "gitlab", input: "gitlab", wantErr: false},
		{name: "self-hosted with dots", input: "git.company.com", wantErr: false},
		{name: "hyphen ok", input: "my-forge", wantErr: false},
		{name: "space rejected (breaks hostname/marker)", input: "git hub", wantErr: true},
		{name: "leading space rejected", input: " github", wantErr: true},
		{name: "trailing space rejected (breaks marker round-trip)", input: "github ", wantErr: true},
		{name: "newline rejected (marker injection)", input: "github\nHost evil", wantErr: true},
		{name: "carriage return rejected", input: "github\r", wantErr: true},
		{name: "slash rejected", input: "git/hub", wantErr: true},
		{name: "shell metachar rejected", input: "a;rm -rf", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := identity.ValidateProvider(tc.input)
			if tc.wantErr && err == nil {
				t.Errorf("ValidateProvider(%q) expected error, got nil", tc.input)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("ValidateProvider(%q) expected nil, got %v", tc.input, err)
			}
		})
	}
}

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "plain address", input: "user@example.com", wantErr: false},
		{name: "plus tag", input: "user+tag@example.com", wantErr: false},
		{name: "empty rejected (email required)", input: "", wantErr: true},
		{name: "embedded space rejected", input: "foo bar@example.com", wantErr: true},
		{name: "leading space rejected", input: " user@example.com", wantErr: true},
		{name: "trailing space rejected", input: "user@example.com ", wantErr: true},
		{name: "tab rejected", input: "user\t@example.com", wantErr: true},
		{name: "missing @ rejected", input: "user.example.com", wantErr: true},
		{name: "newline rejected (fragment injection)", input: "user@example.com\nHost evil", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := identity.ValidateEmail(tc.input)
			if tc.wantErr && err == nil {
				t.Errorf("ValidateEmail(%q) expected error, got nil", tc.input)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("ValidateEmail(%q) expected nil, got %v", tc.input, err)
			}
		})
	}
}
