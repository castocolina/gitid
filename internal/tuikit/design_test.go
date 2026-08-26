package tuikit

import "testing"

func TestGlobalSSHOptionsFrozenOrder(t *testing.T) {
	want := []string{
		"StrictHostKeyChecking",
		"ForwardAgent",
		"HashKnownHosts",
		"IdentitiesOnly",
		"AddKeysToAgent",
		"UseKeychain",
	}
	if len(GlobalSSHOptions) != len(want) {
		t.Fatalf("GlobalSSHOptions has %d entries, want %d", len(GlobalSSHOptions), len(want))
	}
	for i, key := range want {
		if GlobalSSHOptions[i].Key != key {
			t.Errorf("GlobalSSHOptions[%d].Key = %q, want %q", i, GlobalSSHOptions[i].Key, key)
		}
	}
}
