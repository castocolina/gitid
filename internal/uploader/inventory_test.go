package uploader

import (
	"errors"
	"fmt"
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
	if !reflect.DeepEqual(calls, [][]string{{"api", "--paginate", "user/keys"}, {"api", "--paginate", "user/ssh_signing_keys"}}) {
		t.Fatalf("calls=%v", calls)
	}
}

// TestInventoryGHConcatenatesPaginatedPages is the WR-01 regression: `gh api
// --paginate` writes one JSON array per page back-to-back with no
// separator, and every key across every page must end up in the flat
// result — not just the first page's 30-key REST default.
func TestInventoryGHConcatenatesPaginatedPages(t *testing.T) {
	pageOne := `[{"id":1,"title":"a","key":"ssh-ed25519 AAAA a"}]`
	pageTwo := `[{"id":2,"title":"b","key":"ssh-ed25519 AAAA b"}]`
	got, err := Inventory(ToolGH, "gh", Deps{RunCmd: func(_ string, args ...string) (string, int, error) {
		if len(args) > 0 && args[len(args)-1] == "user/keys" {
			return pageOne + pageTwo, 0, nil
		}
		return "[]", 0, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "1" || got[1].ID != "2" {
		t.Fatalf("got=%+v, want both concatenated pages' entries", got)
	}
}

// TestInventoryGLabReadsListOnce is the WR-10 single-page shape: a page
// short of glabPerPage (30) stops the loop after exactly one call, and the
// call carries the verified --page/--per-page pagination flags.
func TestInventoryGLabReadsListOnce(t *testing.T) {
	calls := 0
	got, err := Inventory(ToolGLab, "glab", Deps{RunCmd: func(_ string, args ...string) (string, int, error) {
		calls++
		if !reflect.DeepEqual(args, []string{"ssh-key", "list", "-F", "json", "--per-page", "30", "--page", "1"}) {
			t.Errorf("args=%v", args)
		}
		return `[{"id":2,"title":"both","key":"ssh-ed25519 AAAA both"}]`, 0, nil
	}})
	if err != nil || calls != 1 || got[0].Registration != RegistrationCombined {
		t.Fatalf("got=%+v err=%v calls=%d", got, err, calls)
	}
}

// TestInventoryGLabPaginatesUntilAShortPage is the WR-10 regression: an
// account with exactly glabPerPage (30) keys on the first page must not be
// silently truncated — Inventory must request page 2 and concatenate it,
// stopping only once a page returns fewer than glabPerPage records.
func TestInventoryGLabPaginatesUntilAShortPage(t *testing.T) {
	fullPage := make([]string, glabPerPage)
	for i := range fullPage {
		fullPage[i] = fmt.Sprintf(`{"id":%d,"title":"k%d","key":"ssh-ed25519 AAAA%d"}`, i+1, i+1, i+1)
	}
	pageOneJSON := "[" + strings.Join(fullPage, ",") + "]"
	pageTwoJSON := `[{"id":31,"title":"k31","key":"ssh-ed25519 AAAA31"}]`

	var calls [][]string
	got, err := Inventory(ToolGLab, "glab", Deps{RunCmd: func(_ string, args ...string) (string, int, error) {
		calls = append(calls, append([]string(nil), args...))
		if len(calls) == 1 {
			return pageOneJSON, 0, nil
		}
		return pageTwoJSON, 0, nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 {
		t.Fatalf("calls=%v, want exactly 2 (page 1 full, page 2 short stops the loop)", calls)
	}
	if !reflect.DeepEqual(calls[0], []string{"ssh-key", "list", "-F", "json", "--per-page", "30", "--page", "1"}) {
		t.Errorf("first call args=%v", calls[0])
	}
	if !reflect.DeepEqual(calls[1], []string{"ssh-key", "list", "-F", "json", "--per-page", "30", "--page", "2"}) {
		t.Errorf("second call args=%v", calls[1])
	}
	if len(got) != glabPerPage+1 || got[glabPerPage].ID != "31" {
		t.Fatalf("got %d entries, want %d (page 1 + page 2's single entry, ID 31 last)", len(got), glabPerPage+1)
	}
}

// TestInventoryGLabTerminatesWhenProviderIgnoresPageCap is the WR-07 (review
// iteration 3) regression: a fake RunCmd that returns a full page
// unconditionally (mirroring a glab that silently ignores --page) must not
// hang glabInventory forever -- the loop must bail out after glabMaxPages
// and return a non-gating error instead of spinning.
func TestInventoryGLabTerminatesWhenProviderIgnoresPageCap(t *testing.T) {
	fullPage := make([]string, glabPerPage)
	for i := range fullPage {
		fullPage[i] = fmt.Sprintf(`{"id":%d,"title":"k%d","key":"ssh-ed25519 AAAA%d"}`, i+1, i+1, i+1)
	}
	pageJSON := "[" + strings.Join(fullPage, ",") + "]"

	calls := 0
	got, err := Inventory(ToolGLab, "glab", Deps{RunCmd: func(_ string, _ ...string) (string, int, error) {
		calls++
		return pageJSON, 0, nil // always a full page — never a short one
	}})
	if got != nil {
		t.Errorf("got=%v, want nil once the page cap is hit", got)
	}
	if err == nil {
		t.Fatal("want a non-nil error once glabMaxPages is exceeded, got nil (the call never terminated on its own)")
	}
	if !strings.Contains(err.Error(), "did not terminate") {
		t.Errorf("err=%q, want it to name the page-cap termination reason", err.Error())
	}
	if calls != glabMaxPages {
		t.Errorf("calls=%d, want exactly glabMaxPages (%d) — the loop must stop AT the cap, not run one extra call past it", calls, glabMaxPages)
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

// TestInventorySkipsStderrBannerPrecedingSuccessfulJSON is the WR-12
// (review iteration 3) regression: RunCmd returns CombinedOutput
// (stdout+stderr merged), so a stderr diagnostic on a SUCCESSFUL call (a gh
// deprecation notice, glab's "Using host ..." banner, a corporate proxy
// message) used to corrupt the whole JSON decode and turn a healthy
// inventory into a false D-15 degradation. A leading non-JSON line must be
// skipped so the real payload behind it still decodes.
func TestInventorySkipsStderrBannerPrecedingSuccessfulJSON(t *testing.T) {
	banner := "Using host glab.example.com\n" + `[{"id":1,"title":"a","key":"ssh-ed25519 AAAA a"}]`
	got, err := Inventory(ToolGLab, "glab", Deps{RunCmd: func(string, ...string) (string, int, error) { return banner, 0, nil }})
	if err != nil {
		t.Fatalf("a leading stderr banner must not fail a healthy inventory read: %v", err)
	}
	if len(got) != 1 || got[0].ID != "1" {
		t.Fatalf("got=%+v, want the real payload behind the banner decoded", got)
	}
}

// TestInventoryDiscardedPrefixReachesTheParseError is the negative-control
// sibling: when the payload behind a banner is STILL malformed (a genuine
// parse failure, not just noise), the error must name the discarded prefix
// as diagnostic context instead of a bare "invalid character" with no clue
// what preceded it.
func TestInventoryDiscardedPrefixReachesTheParseError(t *testing.T) {
	banner := "gh: a deprecation notice on stderr\n{not valid json"
	_, err := Inventory(ToolGH, "gh", Deps{RunCmd: func(_ string, args ...string) (string, int, error) {
		if len(args) > 0 && args[len(args)-1] == "user/keys" {
			return banner, 0, nil
		}
		return "[]", 0, nil
	}})
	if err == nil {
		t.Fatal("a genuinely malformed payload must still fail")
	}
	if !strings.Contains(err.Error(), "deprecation notice") {
		t.Errorf("err=%q, want the discarded banner text carried into the error for diagnostic context", err.Error())
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

// TestOldKeyCandidatesRefusesOnBlankCurrentBlob is the WR-01 regression,
// fail-open direction 1: a blank currentBlob (the account's .pub read
// succeeded but returned empty/truncated content) must never be treated as
// "match nothing, exclude nothing" — that silently re-opens CR-01, since the
// sole surviving title match would then be the just-registered NEW key,
// returned as if it were "the unambiguous old key".
func TestOldKeyCandidatesRefusesOnBlankCurrentBlob(t *testing.T) {
	existing := []ExistingKey{
		{ID: "1", Title: "gitid: acme @ mbp", Key: "ssh-ed25519 AAAAnew", Registration: RegistrationAuthentication},
	}
	got := OldKeyCandidates(existing, "gitid: acme @ mbp", "")
	if got != nil {
		t.Fatalf("got=%+v, want nil: a blank currentBlob means we cannot prove which record is the new key", got)
	}
}

// TestOldKeyCandidatesRefusesOnBlankRecordBlob is the WR-01 regression,
// fail-open direction 2: every rec.Key normalizing to "" (a provider/CLI
// JSON field rename or API version that omits "key") must never pass the
// "exactly one distinct blob" ambiguity check — an all-blank set used to
// look like a single distinct blob and return the just-registered key as
// safe to delete.
func TestOldKeyCandidatesRefusesOnBlankRecordBlob(t *testing.T) {
	existing := []ExistingKey{
		{ID: "1", Title: "gitid: acme @ mbp", Key: "", Registration: RegistrationAuthentication},
	}
	got := OldKeyCandidates(existing, "gitid: acme @ mbp", NormalizeKeyBlob("ssh-ed25519 AAAAnew"))
	if got != nil {
		t.Fatalf("got=%+v, want nil: a record whose key blob cannot be read makes the whole set unsafe to act on", got)
	}
}
