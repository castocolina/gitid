package gitconfig_test

import (
	"testing"

	"github.com/castocolina/gitid/internal/gitconfig"
)

// TestValidateDefaultBranch tests the git ref-name validator.
func TestValidateDefaultBranch(t *testing.T) {
	tests := []struct {
		name      string
		branchName string
		wantErr   bool
	}{
		{"valid main", "main", false},
		{"valid develop", "develop", false},
		{"valid with slash", "feature/my-feature", false},
		{"valid with multiple slashes", "release/v1/beta", false},
		{"empty", "", true},
		{"starts with dash", "-main", true},
		{"starts with dot", ".main", true},
		{"ends with dot", "main.", true},
		{"ends with .lock", "main.lock", true},
		{"contains ..", "fea..ture", true},
		{"contains //", "fea//ture", true},
		{"all dots", "..", true},
		{"single dot", ".", true},
		{"reserved tilde", "fea~ture", true},
		{"reserved caret", "fea^ture", true},
		{"reserved colon", "fea:ture", true},
		{"reserved question", "fea?ture", true},
		{"reserved asterisk", "fea*ture", true},
		{"reserved bracket", "fea[ture", true},
		{"reserved backslash", "fea\\ture", true},
		{"at symbol", "@", true},
		{"at brace", "@{foo}", true},
		{"only dots component", "feat/./main", true},
		{"empty component", "feat//main", true},
		{"whitespace", "feat ure", true},
		{"tab", "feat\ture", true},
		{"newline", "feat\nure", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := gitconfig.ValidateDefaultBranch(tt.branchName)
			if (err != nil) != tt.wantErr {
				t.Errorf("got err %v, want err %v", err != nil, tt.wantErr)
			}
		})
	}
}
