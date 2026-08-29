package uploader

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestInventoryGHReadsBothEndpointsAndTagsRegistrations(t *testing.T) {
	var calls [][]string
	deps := Deps{RunCmd: func(_ string, args ...string) (string, int, error) {
		calls = append(calls, args)
		if len(calls) == 1 {
			return `[{"id":1,"title":"auth","key":"ssh-ed25519 AAAA auth"}]`, 0, nil
		}
		return `[{"id":2,"title":"sign","key":"ssh-ed25519 AAAA sign"}]`, 0, nil
	}}
	got, err := Inventory(ToolGH, "gh", deps)
	if err != nil || len(got) != 2 || got[0].Registration != RegistrationAuthentication || got[1].Registration != RegistrationSigning {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	if !reflect.DeepEqual(calls, [][]string{{"api", "user/keys"}, {"api", "user/ssh_signing_keys"}}) {
		t.Fatalf("calls=%v", calls)
	}
}

func TestInventoryGLabReadsListOnce(t *testing.T) {
	calls := 0
	got, err := Inventory(ToolGLab, "glab", Deps{RunCmd: func(_ string, args ...string) (string, int, error) {
		calls++
		if !reflect.DeepEqual(args, []string{"ssh-key", "list", "-F", "json"}) {
			t.Errorf("args=%v", args)
		}
		return `[{"id":2,"title":"both","key":"ssh-ed25519 AAAA both"}]`, 0, nil
	}})
	if err != nil || calls != 1 || got[0].Registration != RegistrationCombined {
		t.Fatalf("got=%+v err=%v calls=%d", got, err, calls)
	}
}

func TestInventoryReturnsErrorOnNonZeroExit(t *testing.T) {
	got, err := Inventory(ToolGH, "gh", Deps{RunCmd: func(string, ...string) (string, int, error) { return "", 1, errors.New("bad") }})
	if err == nil || got != nil {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestInventoryReturnsErrorOnUnparseableJSON(t *testing.T) {
	got, err := Inventory(ToolGLab, "glab", Deps{RunCmd: func(string, ...string) (string, int, error) { return "{", 0, nil }})
	if err == nil || got != nil {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestNormalizeKeyBlobDropsTheComment(t *testing.T) {
	if NormalizeKeyBlob("ssh-ed25519 AAAA one") != NormalizeKeyBlob("ssh-ed25519 AAAA two") {
		t.Fatal("comments affected blob")
	}
}
func TestHasRegistrationMatchesOnBlobNotTitle(t *testing.T) {
	if !HasRegistration([]ExistingKey{{Key: "ssh-ed25519 AAAA one", Registration: RegistrationAuthentication}}, "ssh-ed25519 AAAA two", RegistrationAuthentication) {
		t.Fatal("not found")
	}
}
func TestMissingRegistrationsComputesTheGitHubGap(t *testing.T) {
	got := MissingRegistrations([]ExistingKey{{Key: "ssh-ed25519 AAAA", Registration: RegistrationAuthentication}}, "ssh-ed25519 AAAA hi", []Registration{RegistrationAuthentication, RegistrationSigning})
	if !reflect.DeepEqual(got, []Registration{RegistrationSigning}) {
		t.Fatalf("got=%v", got)
	}
}
func TestDeleteKeyUsesAnArgSliceAndPreviewMatches(t *testing.T) {
	for _, tc := range []struct {
		tool Tool
		reg  Registration
		name string
		want []string
	}{
		{ToolGH, RegistrationAuthentication, "gh", []string{"api", "-X", "DELETE", "user/keys/12"}},
		{ToolGH, RegistrationSigning, "gh", []string{"api", "-X", "DELETE", "user/ssh_signing_keys/12"}},
		{ToolGLab, RegistrationCombined, "glab", []string{"ssh-key", "delete", "12"}},
	} {
		var call []string
		_, err := DeleteKey(tc.tool, tc.name, tc.reg, "12", Deps{RunCmd: func(_ string, args ...string) (string, int, error) { call = args; return "ok\n", 0, nil }})
		if err != nil || !reflect.DeepEqual(call, tc.want) || DeleteCommandPreview(tc.tool, tc.reg, tc.name, "12") != tc.name+" "+strings.Join(call, " ") {
			t.Fatalf("tool=%v reg=%v call=%v err=%v", tc.tool, tc.reg, call, err)
		}
	}
}

// TestDeleteKeyGHRegistrationsUseIndependentNamespaces is the CR-02
// regression: GitHub's authentication and signing key IDs are independent,
// freely-colliding integer spaces, so the SAME numeric id must be routed to
// a DIFFERENT REST resource depending on Registration.
func TestDeleteKeyGHRegistrationsUseIndependentNamespaces(t *testing.T) {
	var authCall, signCall []string
	deps := func(dst *[]string) Deps {
		return Deps{RunCmd: func(_ string, args ...string) (string, int, error) { *dst = args; return "", 0, nil }}
	}
	if _, err := DeleteKey(ToolGH, "gh", RegistrationAuthentication, "7", deps(&authCall)); err != nil {
		t.Fatalf("authentication delete: %v", err)
	}
	if _, err := DeleteKey(ToolGH, "gh", RegistrationSigning, "7", deps(&signCall)); err != nil {
		t.Fatalf("signing delete: %v", err)
	}
	if strings.Join(authCall, " ") == strings.Join(signCall, " ") {
		t.Fatalf("authentication and signing deletes of the SAME id issued the SAME argv: %v", authCall)
	}
	if !strings.Contains(strings.Join(authCall, " "), "user/keys/7") {
		t.Errorf("authentication delete argv = %v, want it to address user/keys/7", authCall)
	}
	if !strings.Contains(strings.Join(signCall, " "), "user/ssh_signing_keys/7") {
		t.Errorf("signing delete argv = %v, want it to address user/ssh_signing_keys/7", signCall)
	}
}
func TestDeleteKeyRejectsAnIDThatIsNotAnID(t *testing.T) {
	for _, id := range []string{"", "1 2", "-1", "title"} {
		calls := 0
		_, err := DeleteKey(ToolGH, "gh", RegistrationAuthentication, id, Deps{RunCmd: func(string, ...string) (string, int, error) { calls++; return "", 0, nil }})
		if err == nil || calls != 0 {
			t.Fatalf("id=%q err=%v calls=%d", id, err, calls)
		}
	}
}
func TestDeleteRecordedKeyPassesTheRecordedIDAndRegistration(t *testing.T) {
	var call []string
	_, err := DeleteRecordedKey(ToolGH, "gh", ExistingKey{ID: "42", Title: "kept", Registration: RegistrationSigning}, Deps{RunCmd: func(_ string, a ...string) (string, int, error) { call = a; return "", 0, nil }})
	if err != nil {
		t.Fatal(err)
	}
	argv := strings.Join(call, " ")
	if !strings.Contains(argv, "user/ssh_signing_keys/42") {
		t.Fatalf("call=%v, want it to address user/ssh_signing_keys/42 (the recorded Registration)", call)
	}
}
func TestFindByTitleIsExact(t *testing.T) {
	_, ok := FindByTitle([]ExistingKey{{Title: "gitid: x @ one"}}, "gitid: x @ two")
	if ok {
		t.Fatal("loose title match")
	}
}

// TestOldKeyCandidatesExcludesTheJustRegisteredKey is the CR-01 regression:
// when a title match spans the OLD key and the NEW key the same rotate
// ceremony just uploaded under the identical title, OldKeyCandidates must
// resolve to ONLY the old key by excluding whatever record's blob equals
// currentBlob.
func TestOldKeyCandidatesExcludesTheJustRegisteredKey(t *testing.T) {
	oldKey := "ssh-ed25519 AAAAold"
	newKey := "ssh-ed25519 AAAAnew"
	existing := []ExistingKey{
		{ID: "1", Title: "gitid: acme @ mbp", Key: oldKey, Registration: RegistrationAuthentication},
		{ID: "2", Title: "gitid: acme @ mbp", Key: newKey, Registration: RegistrationAuthentication},
	}
	got := OldKeyCandidates(existing, "gitid: acme @ mbp", NormalizeKeyBlob(newKey))
	if len(got) != 1 || got[0].ID != "1" {
		t.Fatalf("got=%+v, want exactly the old key (ID 1)", got)
	}
}

// TestOldKeyCandidatesReturnsBothRegistrationsOfTheSameOldKey is the CR-02
// happy-path shape: GitHub records a SEPARATE authentication and signing
// entry for the same physical key, both under the same title. Both must
// come back — deleting only one would leave the old key still able to
// authenticate or sign while gitid reports it fully removed.
func TestOldKeyCandidatesReturnsBothRegistrationsOfTheSameOldKey(t *testing.T) {
	oldKey := "ssh-ed25519 AAAAold"
	newKey := "ssh-ed25519 AAAAnew"
	existing := []ExistingKey{
		{ID: "1", Title: "gitid: acme @ mbp", Key: oldKey, Registration: RegistrationAuthentication},
		{ID: "2", Title: "gitid: acme @ mbp", Key: oldKey, Registration: RegistrationSigning},
		{ID: "3", Title: "gitid: acme @ mbp", Key: newKey, Registration: RegistrationAuthentication},
		{ID: "4", Title: "gitid: acme @ mbp", Key: newKey, Registration: RegistrationSigning},
	}
	got := OldKeyCandidates(existing, "gitid: acme @ mbp", NormalizeKeyBlob(newKey))
	if len(got) != 2 {
		t.Fatalf("got=%+v, want both old-key registrations (IDs 1 and 2)", got)
	}
	ids := map[string]bool{got[0].ID: true, got[1].ID: true}
	if !ids["1"] || !ids["2"] {
		t.Fatalf("got=%+v, want IDs 1 and 2", got)
	}
}

// TestOldKeyCandidatesRefusesWhenMoreThanOneDistinctOldKeyRemains is the
// never-guess regression: if title-matching, blob-excluded candidates still
// span more than one DISTINCT key (e.g. a leftover from an earlier
// rotation), OldKeyCandidates must return nil rather than pick one.
func TestOldKeyCandidatesRefusesWhenMoreThanOneDistinctOldKeyRemains(t *testing.T) {
	existing := []ExistingKey{
		{ID: "1", Title: "gitid: acme @ mbp", Key: "ssh-ed25519 AAAAone", Registration: RegistrationAuthentication},
		{ID: "2", Title: "gitid: acme @ mbp", Key: "ssh-ed25519 AAAAtwo", Registration: RegistrationAuthentication},
	}
	got := OldKeyCandidates(existing, "gitid: acme @ mbp", NormalizeKeyBlob("ssh-ed25519 AAAAnew"))
	if got != nil {
		t.Fatalf("got=%+v, want nil (refuse rather than guess between two distinct old keys)", got)
	}
}

// TestOldKeyCandidatesReturnsNilWhenNothingRemains covers the 0-candidate
// side of "0 -> nothing to remove; >1 distinct -> ambiguous, do not guess".
func TestOldKeyCandidatesReturnsNilWhenNothingRemains(t *testing.T) {
	existing := []ExistingKey{
		{ID: "1", Title: "gitid: acme @ mbp", Key: "ssh-ed25519 AAAAnew", Registration: RegistrationAuthentication},
	}
	got := OldKeyCandidates(existing, "gitid: acme @ mbp", NormalizeKeyBlob("ssh-ed25519 AAAAnew"))
	if got != nil {
		t.Fatalf("got=%+v, want nil (nothing left to remove)", got)
	}
}
