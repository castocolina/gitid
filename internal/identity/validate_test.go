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
