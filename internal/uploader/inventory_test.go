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
		name string
		want []string
	}{
		{ToolGH, "gh", []string{"ssh-key", "delete", "12", "--yes"}},
		{ToolGLab, "glab", []string{"ssh-key", "delete", "12"}},
	} {
		var call []string
		_, err := DeleteKey(tc.tool, tc.name, "12", Deps{RunCmd: func(_ string, args ...string) (string, int, error) { call = args; return "ok\n", 0, nil }})
		if err != nil || !reflect.DeepEqual(call, tc.want) || DeleteCommandPreview(tc.tool, tc.name, "12") != tc.name+" "+strings.Join(call, " ") {
			t.Fatalf("tool=%v call=%v err=%v", tc.tool, call, err)
		}
	}
}
func TestDeleteKeyRejectsAnIDThatIsNotAnID(t *testing.T) {
	for _, id := range []string{"", "1 2", "-1", "title"} {
		calls := 0
		_, err := DeleteKey(ToolGH, "gh", id, Deps{RunCmd: func(string, ...string) (string, int, error) { calls++; return "", 0, nil }})
		if err == nil || calls != 0 {
			t.Fatalf("id=%q err=%v calls=%d", id, err, calls)
		}
	}
}
func TestDeleteRecordedKeyPassesTheRecordedID(t *testing.T) {
	var call []string
	_, _ = DeleteRecordedKey(ToolGH, "gh", ExistingKey{ID: "42", Title: "kept"}, Deps{RunCmd: func(_ string, a ...string) (string, int, error) { call = a; return "", 0, nil }})
	if call[2] != "42" {
		t.Fatal(call)
	}
}
func TestFindByTitleIsExact(t *testing.T) {
	_, ok := FindByTitle([]ExistingKey{{Title: "gitid: x @ one"}}, "gitid: x @ two")
	if ok {
		t.Fatal("loose title match")
	}
}
