package upload_test

import (
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/upload"
)

func TestInstructions(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		contains []string
		nonEmpty bool
	}{
		{
			name:     "github contains settings/ssh/new and two registrations",
			provider: "github",
			contains: []string{
				"github.com/settings/ssh/new",
				"Authentication key",
				"Signing key",
			},
		},
		{
			name:     "github uppercase normalized",
			provider: "GitHub",
			contains: []string{
				"github.com/settings/ssh/new",
			},
		},
		{
			name:     "gitlab contains user_settings/ssh_keys",
			provider: "gitlab",
			contains: []string{
				"user_settings/ssh_keys",
			},
		},
		{
			name:     "bitbucket default branch is non-empty",
			provider: "bitbucket",
			nonEmpty: true,
		},
		{
			name:     "empty provider default is non-empty",
			provider: "",
			nonEmpty: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := upload.Instructions(tc.provider)
			if tc.nonEmpty && got == "" {
				t.Errorf("Instructions(%q) returned empty string", tc.provider)
			}
			for _, want := range tc.contains {
				if !strings.Contains(got, want) {
					t.Errorf("Instructions(%q) does not contain %q\ngot: %s", tc.provider, want, got)
				}
			}
		})
	}
}
