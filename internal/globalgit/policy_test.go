package globalgit

import (
	"regexp"
	"strings"
	"testing"
)

// TestPolicyFor_KnownKey exercises the case-insensitive display-key lookup.
func TestPolicyFor_KnownKey(t *testing.T) {
	p, ok := PolicyFor("init.defaultBranch")
	if !ok {
		t.Fatal("PolicyFor(init.defaultBranch) returned ok=false")
	}
	if p.Recommended != "main" {
		t.Errorf("Recommended = %q, want %q", p.Recommended, "main")
	}
	if p.MinVersion != "2.28" {
		t.Errorf("MinVersion = %q, want %q", p.MinVersion, "2.28")
	}
	if p.Gate != GateInformational {
		t.Errorf("Gate = %v, want GateInformational", p.Gate)
	}
}

func TestPolicyFor_CaseInsensitive(t *testing.T) {
	_, ok := PolicyFor("INIT.DEFAULTBRANCH")
	if !ok {
		t.Error("PolicyFor should match case-insensitively")
	}
}

func TestPolicyFor_UnknownKey(t *testing.T) {
	_, ok := PolicyFor("unknown.key.that.does.not.exist")
	if ok {
		t.Error("PolicyFor should return ok=false for unknown key")
	}
}

// TestPolicyForMember_ScalarRow asserts PolicyForMember matches a scalar
// row's member key (which, for a scalar row, equals the display key) — CR-01
// round 3: a custom git key is always a single dotted key, so it can only
// ever collide with a MEMBER key, never a bundle row's prose display key.
func TestPolicyForMember_ScalarRow(t *testing.T) {
	p, ok := PolicyForMember("init.defaultBranch")
	if !ok {
		t.Fatal("PolicyForMember(init.defaultBranch) returned ok=false")
	}
	if p.Key != "init.defaultBranch" {
		t.Errorf("row.Key = %q, want %q", p.Key, "init.defaultBranch")
	}
}

// TestPolicyForMember_BundleRow asserts PolicyForMember matches a MEMBER key
// of a bundle row even though PolicyFor(the same string) would fail — the
// bundle row's DISPLAY key is prose ("core.autocrlf / core.eol"), not any
// single member key.
func TestPolicyForMember_BundleRow(t *testing.T) {
	p, ok := PolicyForMember("core.autocrlf")
	if !ok {
		t.Fatal("PolicyForMember(core.autocrlf) returned ok=false")
	}
	if p.Key != "core.autocrlf / core.eol" {
		t.Errorf("row.Key = %q, want the bundle row's display key", p.Key)
	}
	if _, ok := PolicyFor("core.autocrlf"); ok {
		t.Fatal("test invariant broken: PolicyFor(core.autocrlf) must NOT match — it only matches DISPLAY keys")
	}

	if _, ok := PolicyForMember("alias.st"); !ok {
		t.Error("PolicyForMember(alias.st) should match the alias bundle row's member key")
	}
}

// TestPolicyForMember_CaseInsensitive mirrors PolicyFor's own case rule.
func TestPolicyForMember_CaseInsensitive(t *testing.T) {
	if _, ok := PolicyForMember("INIT.DEFAULTBRANCH"); !ok {
		t.Error("PolicyForMember should match case-insensitively")
	}
}

// TestPolicyForMember_UnknownKey asserts a key not managed by any row returns
// ok=false, so a genuinely free-form custom key is never wrongly blocked.
func TestPolicyForMember_UnknownKey(t *testing.T) {
	if _, ok := PolicyForMember("http.sslVerify"); ok {
		t.Error("PolicyForMember should return ok=false for a key no row manages")
	}
}

// TestPolicyOrderMatchesAuthorityPinnedTable asserts the ordered row list
// equals the 07-03-PLAN.md <authority> block's D-08 pinned display order, row
// for row.
func TestPolicyOrderMatchesAuthorityPinnedTable(t *testing.T) {
	want := []string{
		"init.defaultBranch",
		"core.ignorecase",
		"core.autocrlf / core.eol",
		"user.email (global fallback)",
		"user.useConfigOnly",
		"push.autoSetupRemote",
		"pull.rebase",
		"fetch.prune",
		"alias (8 shortcuts)",
		"color (ui/branch/diff/status)",
		"merge.conflictstyle",
		"diff.colorMoved",
	}
	if len(Policy) != len(want) {
		t.Fatalf("policy rows = %d, want %d", len(Policy), len(want))
	}
	for i, key := range want {
		if Policy[i].Key != key {
			t.Errorf("row %d key = %q, want %q", i, Policy[i].Key, key)
		}
	}
}

// TestPolicyMemberKeyCounts asserts the per-row member-key counts: one for
// each scalar row, two for the line-endings row, eight for the alias row,
// four for the color row, zero for the fallback-author row.
func TestPolicyMemberKeyCounts(t *testing.T) {
	want := []struct {
		key   string
		count int
	}{
		{"init.defaultBranch", 1},
		{"core.ignorecase", 1},
		{"core.autocrlf / core.eol", 2},
		{"user.email (global fallback)", 0},
		{"user.useConfigOnly", 1},
		{"push.autoSetupRemote", 1},
		{"pull.rebase", 1},
		{"fetch.prune", 1},
		{"alias (8 shortcuts)", 8},
		{"color (ui/branch/diff/status)", 4},
		{"merge.conflictstyle", 1},
		{"diff.colorMoved", 1},
	}
	for _, tc := range want {
		p, ok := PolicyFor(tc.key)
		if !ok {
			t.Fatalf("policy row %q missing", tc.key)
		}
		if got := len(p.Members); got != tc.count {
			t.Errorf("row %q member count = %d, want %d", tc.key, got, tc.count)
		}
	}
}

// TestPolicyMergeConflictstyleIsOnlyHardGate asserts merge.conflictstyle is the
// ONLY row in the table whose gate kind is hard, and that it declares its
// fallback value.
func TestPolicyMergeConflictstyleIsOnlyHardGate(t *testing.T) {
	var hard []OptionPolicy
	for _, p := range Policy {
		if p.Gate == GateHard {
			hard = append(hard, p)
		}
	}
	if len(hard) != 1 {
		t.Fatalf("hard-gated rows = %d, want exactly 1", len(hard))
	}
	if hard[0].Key != "merge.conflictstyle" {
		t.Errorf("hard-gated row = %q, want merge.conflictstyle", hard[0].Key)
	}
	if hard[0].Fallback != "diff3" {
		t.Errorf("hard-gated fallback = %q, want diff3", hard[0].Fallback)
	}
}

// TestPolicyTokensUniqueShellSafeAndPinned asserts every row's CLI token is
// unique, contains no space, slash or shell metacharacter, that the full set
// equals the <authority> table's second column, and that the fallback-author
// row carries none.
func TestPolicyTokensUniqueShellSafeAndPinned(t *testing.T) {
	pinned := map[string]bool{
		"init.defaultBranch":   true,
		"core.ignorecase":      true,
		"core.lineEndings":     true,
		"user.useConfigOnly":   true,
		"push.autoSetupRemote": true,
		"pull.rebase":          true,
		"fetch.prune":          true,
		"alias":                true,
		"color":                true,
		"merge.conflictstyle":  true,
		"diff.colorMoved":      true,
	}
	shellSafe := regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	seen := map[string]string{} // token -> row key
	for _, p := range Policy {
		if p.Key == "user.email (global fallback)" {
			if p.Token != "" {
				t.Errorf("fallback-author row must carry no token, got %q", p.Token)
			}
			continue
		}
		if p.Token == "" {
			t.Errorf("row %q must carry a non-empty CLI token", p.Key)
		}
		if !shellSafe.MatchString(p.Token) {
			t.Errorf("row %q token %q is not shell-safe (space/slash/metacharacter)", p.Key, p.Token)
		}
		if owner, dup := seen[p.Token]; dup {
			t.Errorf("token %q duplicated on rows %q and %q", p.Token, owner, p.Key)
		}
		seen[p.Token] = p.Key
		if !pinned[p.Token] {
			t.Errorf("row %q token %q is not in the pinned <authority> set", p.Key, p.Token)
		}
	}
	if len(seen) != len(pinned) {
		t.Errorf("policy token set = %d, want %d (the <authority> table's second column)", len(seen), len(pinned))
	}
}

// TestPolicyLineEndingsQuirkRecorded asserts the eol-ignored-while-autocrlf-is-
// input quirk is stated on the policy, not only in copy — a reviewer must see
// that gitid knows it writes a key with no immediate effect, and why.
func TestPolicyLineEndingsQuirkRecorded(t *testing.T) {
	p, ok := PolicyFor("core.autocrlf / core.eol")
	if !ok {
		t.Fatal("line-endings row missing")
	}
	eol, ok := p.memberFor("core.eol")
	if !ok {
		t.Fatal("core.eol member missing")
	}
	if eol.Note == "" || !strings.Contains(strings.ToLower(eol.Note), "ignored") {
		t.Errorf("core.eol must carry a note documenting that git ignores it while autocrlf=input; got %q", eol.Note)
	}
	if p.Token != "core.lineEndings" {
		t.Errorf("line-endings token = %q, want core.lineEndings (a member key like core.eol must never be the token)", p.Token)
	}
}

// TestPolicyScalarDefaultsNamedWhenUnsetDiffers asserts every scalar row whose
// recommendation differs from git's behavior when unset carries a non-empty
// built-in-default string on its member, so an unset row can name it (D-03).
func TestPolicyScalarDefaultsNamedWhenUnsetDiffers(t *testing.T) {
	// key -> the git behaviour gitid's recommendation fights when unset.
	expectDefault := map[string]string{
		"init.defaultBranch":   "master",
		"core.ignorecase":      "true",
		"push.autoSetupRemote": "false",
		"pull.rebase":          "false",
		"fetch.prune":          "false",
		"user.useConfigOnly":   "false",
		"merge.conflictstyle":  "merge",
		"diff.colorMoved":      "no",
	}
	for _, p := range Policy {
		if len(p.Members) != 1 {
			continue
		}
		m := p.Members[0]
		want, differs := expectDefault[p.Key]
		memberDiffers := !strings.EqualFold(m.Recommended, want)
		if differs && memberDiffers && m.GitDefault == "" {
			t.Errorf("row %q recommends %q but git's unset default is %q — the member must carry that GitDefault so an unset row can name it", p.Key, m.Recommended, want)
		}
		if m.GitDefault != "" && m.GitDefault == m.Recommended {
			t.Errorf("row %q GitDefault %q equals the recommendation — naming it as a default would be meaningless", p.Key, m.GitDefault)
		}
	}
}

// TestPolicyAliasesVerbatimFromRecipe asserts the eight aliases carry the
// recipe's values byte-identical, including the full lg format string — never
// paraphrased (D-08 North Star).
func TestPolicyAliasesVerbatimFromRecipe(t *testing.T) {
	p, ok := PolicyFor("alias (8 shortcuts)")
	if !ok {
		t.Fatal("alias row missing")
	}
	want := map[string]string{
		"alias.st":      "status",
		"alias.co":      "checkout",
		"alias.br":      "branch",
		"alias.ci":      "commit",
		"alias.df":      "diff",
		"alias.lg":      "log --graph --pretty=format:'%Cred%h%Creset -%C(yellow)%d%Creset %s %Cgreen(%cr) %C(bold blue)<%an>%Creset' --abbrev-commit",
		"alias.unstage": "reset HEAD --",
		"alias.last":    "log -1 HEAD",
	}
	if len(p.Members) != len(want) {
		t.Fatalf("alias members = %d, want %d", len(p.Members), len(want))
	}
	for _, m := range p.Members {
		w, ok := want[m.Key]
		if !ok {
			t.Errorf("unexpected alias member %q", m.Key)
			continue
		}
		if m.Recommended != w {
			t.Errorf("alias %s = %q, want recipe value %q", m.Key, m.Recommended, w)
		}
	}
}

// TestPolicyColorKeysVerbatimFromRecipe asserts the four color keys carry
// auto each, matching the recipe's ~/.gitconfig_default example.
func TestPolicyColorKeysVerbatimFromRecipe(t *testing.T) {
	p, ok := PolicyFor("color (ui/branch/diff/status)")
	if !ok {
		t.Fatal("color row missing")
	}
	if len(p.Members) != 4 {
		t.Fatalf("color members = %d, want 4", len(p.Members))
	}
	for _, m := range p.Members {
		if m.Recommended != "auto" {
			t.Errorf("color member %s = %q, want recipe value auto", m.Key, m.Recommended)
		}
	}
}

// TestPolicyMemberForCaseInsensitive asserts member lookup is case-insensitive
// like the row lookup.
func TestPolicyMemberForCaseInsensitive(t *testing.T) {
	p, _ := PolicyFor("merge.conflictstyle")
	if _, ok := p.memberFor("MERGE.CONFLICTSTYLE"); !ok {
		t.Error("memberFor must match case-insensitively (git lower-cases keys)")
	}
	if _, ok := p.memberFor("merge.doesnotexist"); ok {
		t.Error("memberFor must refuse unknown member keys")
	}
}
