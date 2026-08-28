package uploader

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// ---- Fake helpers ----------------------------------------------------------

// notFoundError mimics exec.ErrNotFound for LookPath stubs.
type notFoundError struct{ name string }

func (e *notFoundError) Error() string { return e.name + ": executable file not found in $PATH" }

// lookPathOnly returns a Deps.LookPath that finds the named binary at fakeDir.
func fakeLookPath(found map[string]string) func(string) (string, error) {
	return func(name string) (string, error) {
		if p, ok := found[name]; ok {
			return p, nil
		}
		return "", &notFoundError{name: name}
	}
}

// recordingRunCmd returns a RunCmd that records every call and returns the
// configured exit code. exitCode 0 => success; non-zero => error.
type runCall struct {
	name string
	args []string
}

func recordingRunCmd(exitCode int, stdout string) (func(string, ...string) (string, int, error), *[]runCall) {
	calls := &[]runCall{}
	fn := func(name string, args ...string) (string, int, error) {
		*calls = append(*calls, runCall{name: name, args: args})
		if exitCode != 0 {
			return stdout, exitCode, errors.New("exit status 1")
		}
		return stdout, 0, nil
	}
	return fn, calls
}

// ---- TestDetect ------------------------------------------------------------

// TestDetect_GHAuthenticated verifies that when gh is on PATH and auth status
// returns exit 0, Detect returns (ToolGH, path, AuthAuthenticated).
func TestDetect_GHAuthenticated(t *testing.T) {
	runCmd, calls := recordingRunCmd(0, "")
	deps := Deps{
		LookPath: fakeLookPath(map[string]string{"gh": "/fake/gh"}),
		RunCmd:   runCmd,
	}

	tool, path, status := Detect(deps)

	if tool != ToolGH {
		t.Errorf("tool: got %d want ToolGH(%d)", tool, ToolGH)
	}
	if path != "/fake/gh" {
		t.Errorf("path: got %q want /fake/gh", path)
	}
	if status != AuthAuthenticated {
		t.Errorf("status: got %d want AuthAuthenticated(%d)", status, AuthAuthenticated)
	}
	if len(*calls) != 1 {
		t.Fatalf("RunCmd call count: got %d want 1", len(*calls))
	}
	if (*calls)[0].name != "/fake/gh" || (*calls)[0].args[0] != "auth" {
		t.Errorf("RunCmd called with wrong args: %+v", (*calls)[0])
	}
}

// TestDetect_GHNotLoggedIn verifies that when gh is on PATH but auth status
// returns a non-zero exit, Detect returns (ToolGH, path, AuthNotLoggedIn).
func TestDetect_GHNotLoggedIn(t *testing.T) {
	runCmd, _ := recordingRunCmd(1, "")
	deps := Deps{
		LookPath: fakeLookPath(map[string]string{"gh": "/fake/gh"}),
		RunCmd:   runCmd,
	}

	tool, path, status := Detect(deps)

	if tool != ToolGH {
		t.Errorf("tool: got %d want ToolGH(%d)", tool, ToolGH)
	}
	if path != "/fake/gh" {
		t.Errorf("path: got %q want /fake/gh", path)
	}
	if status != AuthNotLoggedIn {
		t.Errorf("status: got %d want AuthNotLoggedIn(%d)", status, AuthNotLoggedIn)
	}
}

// TestDetect_GLAbAuthenticated verifies that when gh is absent but glab is on
// PATH and authenticated, Detect returns (ToolGLab, path, AuthAuthenticated).
func TestDetect_GLabAuthenticated(t *testing.T) {
	runCmd, _ := recordingRunCmd(0, "")
	deps := Deps{
		LookPath: fakeLookPath(map[string]string{"glab": "/fake/glab"}),
		RunCmd:   runCmd,
	}

	tool, path, status := Detect(deps)

	if tool != ToolGLab {
		t.Errorf("tool: got %d want ToolGLab(%d)", tool, ToolGLab)
	}
	if path != "/fake/glab" {
		t.Errorf("path: got %q want /fake/glab", path)
	}
	if status != AuthAuthenticated {
		t.Errorf("status: got %d want AuthAuthenticated(%d)", status, AuthAuthenticated)
	}
}

// TestDetect_NeitherPresent verifies that when neither gh nor glab is on PATH,
// Detect returns ("", "", AuthToolNotFound).
func TestDetect_NeitherPresent(t *testing.T) {
	runCmd, calls := recordingRunCmd(0, "")
	deps := Deps{
		LookPath: fakeLookPath(map[string]string{}),
		RunCmd:   runCmd,
	}

	tool, path, status := Detect(deps)

	if status != AuthToolNotFound {
		t.Errorf("status: got %d want AuthToolNotFound(%d)", status, AuthToolNotFound)
	}
	if path != "" {
		t.Errorf("path: got %q want empty", path)
	}
	if tool != 0 {
		t.Errorf("tool: got %d want 0", tool)
	}
	if len(*calls) != 0 {
		t.Errorf("RunCmd should not be called when no tool found; got %d calls", len(*calls))
	}
}

// TestDetect_GHPreferredOverGLab verifies that when both gh and glab are on
// PATH and authenticated, Detect returns gh (deterministic order).
func TestDetect_GHPreferredOverGLab(t *testing.T) {
	runCmd, _ := recordingRunCmd(0, "")
	deps := Deps{
		LookPath: fakeLookPath(map[string]string{
			"gh":   "/fake/gh",
			"glab": "/fake/glab",
		}),
		RunCmd: runCmd,
	}

	tool, path, status := Detect(deps)

	if tool != ToolGH {
		t.Errorf("tool: got %d want ToolGH(%d) — gh must be preferred over glab", tool, ToolGH)
	}
	if path != "/fake/gh" {
		t.Errorf("path: got %q want /fake/gh", path)
	}
	if status != AuthAuthenticated {
		t.Errorf("status: got %d want AuthAuthenticated(%d)", status, AuthAuthenticated)
	}
}

// TestDetect_AuthToolNotFound is the legacy stub test preserved for regression:
// when LookPath finds nothing, status must be AuthToolNotFound.
func TestDetect_AuthToolNotFound(t *testing.T) {
	deps := Deps{
		LookPath: func(_ string) (string, error) {
			return "", &notFoundError{name: "gh"}
		},
		RunCmd: func(_ string, _ ...string) (string, int, error) {
			return "", 0, nil
		},
	}

	_, _, status := Detect(deps)
	if status != AuthToolNotFound {
		t.Errorf("Detect: got status %d want AuthToolNotFound(%d)", status, AuthToolNotFound)
	}
}

// ---- TestAuthCheck ---------------------------------------------------------

// TestAuthCheck_Authenticated verifies that exit 0 returns AuthAuthenticated.
func TestAuthCheck_Authenticated(t *testing.T) {
	runCmd, _ := recordingRunCmd(0, "")
	deps := Deps{RunCmd: runCmd}

	if got := AuthCheck("/fake/gh", deps, "github.com"); got != AuthAuthenticated {
		t.Errorf("AuthCheck exit 0: got %d want AuthAuthenticated(%d)", got, AuthAuthenticated)
	}
}

// TestAuthCheck_NotLoggedIn verifies that a non-zero exit returns AuthNotLoggedIn.
func TestAuthCheck_NotLoggedIn(t *testing.T) {
	runCmd, _ := recordingRunCmd(1, "")
	deps := Deps{RunCmd: runCmd}

	if got := AuthCheck("/fake/gh", deps, "github.com"); got != AuthNotLoggedIn {
		t.Errorf("AuthCheck exit 1: got %d want AuthNotLoggedIn(%d)", got, AuthNotLoggedIn)
	}
}

// TestAuthCheckPassesHostname asserts the exact argv AuthCheck sends —
// "auth status --hostname <canonicalHost>", NEVER a bare "auth status"
// (D-14: a bare auth status checks ALL hosts, wrong in both directions).
func TestAuthCheckPassesHostname(t *testing.T) {
	runCmd, calls := recordingRunCmd(0, "")
	deps := Deps{RunCmd: runCmd}

	AuthCheck("/fake/gh", deps, "github.com")

	if len(*calls) != 1 {
		t.Fatalf("RunCmd call count: got %d want 1", len(*calls))
	}
	want := []string{"auth", "status", "--hostname", "github.com"}
	assertArgs(t, (*calls)[0].name, (*calls)[0].args, "/fake/gh", want)
}

// ---- TestProviderForHostname -----------------------------------------------

// TestProviderForHostnameMatchesHostBoundaries is R2's regression test: a
// future strings.Contains reimplementation of the provider gate must fail
// this table. Positive cases cover the main domain and real subdomains
// (including this project's own alt-SSH aliases); negative cases cover
// every host that merely CONTAINS a provider's name without being a genuine
// subdomain of it.
func TestProviderForHostnameMatchesHostBoundaries(t *testing.T) {
	cases := []struct {
		host, wantProvider, wantCanonical string
	}{
		{"github.com", "github", "github.com"},
		{"ssh.github.com", "github", "github.com"},
		{"personal.github.com", "github", "github.com"},
		{"GitHub.com", "github", "github.com"},
		{"github.com.", "github", "github.com"},
		{"github.com:443", "github", "github.com"},
		{"gitlab.com", "gitlab", "gitlab.com"},
		{"altssh.gitlab.com", "gitlab", "gitlab.com"},
		{"github.example.com", "", ""},
		{"notgithub.com", "", ""},
		{"mygithub.com", "", ""},
		{"github.com.evil.net", "", ""},
		{"gitlab.example.org", "", ""},
		{"git.internal.example", "", ""},
	}
	for _, c := range cases {
		gotProvider, gotCanonical := ProviderForHostname(c.host)
		if gotProvider != c.wantProvider || gotCanonical != c.wantCanonical {
			t.Errorf("ProviderForHostname(%q) = (%q, %q), want (%q, %q)",
				c.host, gotProvider, gotCanonical, c.wantProvider, c.wantCanonical)
		}
		if c.wantProvider != "" && c.host != c.wantCanonical {
			// Every subdomain/alias case must differ from its canonical
			// answer — only the bare canonical domain itself may equal it.
			if gotCanonical == c.host {
				t.Errorf("ProviderForHostname(%q) canonical host echoed the input; want the canonical %q", c.host, c.wantCanonical)
			}
		}
	}
}

// ---- TestDetectFor ----------------------------------------------------------

// TestDetectForNeverCrossRoutes is D-11's never-cross-route regression: a
// fake reporting glab present+authenticated and gh absent must still make
// DetectFor("github", ...) answer AuthToolNotFound, and glab must never
// appear in the recorded LookPath calls.
func TestDetectForNeverCrossRoutes(t *testing.T) {
	var lookedUp []string
	deps := Deps{
		LookPath: func(name string) (string, error) {
			lookedUp = append(lookedUp, name)
			if name == "glab" {
				return "/fake/glab", nil
			}
			return "", &notFoundError{name: name}
		},
		RunCmd: func(_ string, _ ...string) (string, int, error) { return "", 0, nil },
	}

	tool, path, status := DetectFor("github", deps)
	if status != AuthToolNotFound {
		t.Errorf("DetectFor(github): status = %d, want AuthToolNotFound(%d)", status, AuthToolNotFound)
	}
	if tool != 0 || path != "" {
		t.Errorf("DetectFor(github): tool=%d path=%q, want zero values on not-found", tool, path)
	}
	for _, name := range lookedUp {
		if name == "glab" {
			t.Fatalf("DetectFor(github) must never LookPath(glab); recorded calls: %v", lookedUp)
		}
	}
}

// TestDetectForUnknownProviderNeverProbes verifies that an unknown/empty
// provider key never calls LookPath at all — the omitted decision is pure.
func TestDetectForUnknownProviderNeverProbes(t *testing.T) {
	var lookedUp []string
	deps := Deps{
		LookPath: func(name string) (string, error) {
			lookedUp = append(lookedUp, name)
			return "/fake/" + name, nil
		},
		RunCmd: func(_ string, _ ...string) (string, int, error) { return "", 0, nil },
	}

	_, _, status := DetectFor("", deps)
	if status != AuthToolNotFound {
		t.Errorf("DetectFor(\"\"): status = %d, want AuthToolNotFound(%d)", status, AuthToolNotFound)
	}
	if len(lookedUp) != 0 {
		t.Errorf("DetectFor(\"\") must not call LookPath; recorded calls: %v", lookedUp)
	}
}

// ---- TestUploadKey ---------------------------------------------------------

// TestUploadKey_GHAuthentication verifies the exact arg slice for gh auth upload.
func TestUploadKey_GHAuthentication(t *testing.T) {
	runCmd, calls := recordingRunCmd(0, "Added SSH key.")
	deps := Deps{RunCmd: runCmd}

	out, err := UploadKey(ToolGH, "/fake/gh", "~/.ssh/id_ed25519.pub", "gitid: personal", KeyAuthentication, deps)

	if err != nil {
		t.Fatalf("UploadKey: unexpected error: %v", err)
	}
	if out != "Added SSH key." {
		t.Errorf("output: got %q want %q", out, "Added SSH key.")
	}
	if len(*calls) != 1 {
		t.Fatalf("RunCmd call count: got %d want 1", len(*calls))
	}
	c := (*calls)[0]
	want := []string{"ssh-key", "add", "~/.ssh/id_ed25519.pub", "--title", "gitid: personal", "--type", "authentication"}
	assertArgs(t, c.name, c.args, "/fake/gh", want)
}

// TestUploadKey_GHSigning verifies the exact arg slice for gh signing upload.
func TestUploadKey_GHSigning(t *testing.T) {
	runCmd, calls := recordingRunCmd(0, "Added SSH key.")
	deps := Deps{RunCmd: runCmd}

	_, err := UploadKey(ToolGH, "/fake/gh", "~/.ssh/id_ed25519.pub", "gitid: personal", KeySigning, deps)

	if err != nil {
		t.Fatalf("UploadKey: unexpected error: %v", err)
	}
	c := (*calls)[0]
	want := []string{"ssh-key", "add", "~/.ssh/id_ed25519.pub", "--title", "gitid: personal", "--type", "signing"}
	assertArgs(t, c.name, c.args, "/fake/gh", want)
}

// TestUploadKey_GLab verifies the exact arg slice for glab upload.
func TestUploadKey_GLab(t *testing.T) {
	runCmd, calls := recordingRunCmd(0, "Added SSH key.")
	deps := Deps{RunCmd: runCmd}

	_, err := UploadKey(ToolGLab, "/fake/glab", "~/.ssh/id_ed25519.pub", "gitid: work", GLabKeyTypeForAuth, deps)

	if err != nil {
		t.Fatalf("UploadKey: unexpected error: %v", err)
	}
	c := (*calls)[0]
	// glab uses -t (short) for title and --usage-type for role.
	want := []string{"ssh-key", "add", "~/.ssh/id_ed25519.pub", "-t", "gitid: work", "--usage-type", "auth"}
	assertArgs(t, c.name, c.args, "/fake/glab", want)
}

// TestUploadKey_ErrorSurfacesOutput verifies that when RunCmd returns non-zero
// exit, the captured output is included in the error so callers can show a
// manual fallback (D-11 / D-12).
func TestUploadKey_ErrorSurfacesOutput(t *testing.T) {
	runCmd, _ := recordingRunCmd(1, "error: not authenticated")
	deps := Deps{RunCmd: runCmd}

	out, err := UploadKey(ToolGH, "/fake/gh", "~/.ssh/id_ed25519.pub", "gitid: personal", KeyAuthentication, deps)

	if err == nil {
		t.Fatal("UploadKey: expected error on non-zero exit")
	}
	if out != "error: not authenticated" {
		t.Errorf("output: got %q want %q", out, "error: not authenticated")
	}
}

// TestUploadKey_MetacharInPubPath verifies that a pubPath containing shell
// metacharacters is passed as a single unmodified argument (no shell expansion).
// The recorded-fake proves no splitting occurs — the arg slice is exact.
func TestUploadKey_MetacharInPubPath(t *testing.T) {
	runCmd, calls := recordingRunCmd(0, "Added SSH key.")
	deps := Deps{RunCmd: runCmd}

	// pubPath with spaces and shell metacharacters — must arrive intact as one arg.
	pubPath := "~/.ssh/my key $(whoami).pub"
	_, err := UploadKey(ToolGH, "/fake/gh", pubPath, "gitid: personal", KeyAuthentication, deps)

	if err != nil {
		t.Fatalf("UploadKey: unexpected error: %v", err)
	}
	c := (*calls)[0]
	// The third arg must be the full pubPath, unchanged and unsplit.
	if len(c.args) < 3 || c.args[2] != pubPath {
		t.Errorf("pubPath metachar not passed as single arg; args[2]=%q (full args: %v)", safeGet(c.args, 2), c.args)
	}
}

// ---- TestCommandPreview ----------------------------------------------------

// TestCommandPreview_GHEqualsRunCmd verifies that the command string returned
// by CommandPreview encodes the SAME args that UploadKey passes to RunCmd.
// This is the UI-SPEC §4a "shown command == run command" invariant.
func TestCommandPreview_GHEqualsRunCmd(t *testing.T) {
	runCmd, calls := recordingRunCmd(0, "")
	deps := Deps{RunCmd: runCmd}

	pubPath := "~/.ssh/id_ed25519.pub"
	title := "gitid: personal"
	keyType := KeyAuthentication
	toolPath := "/fake/gh"

	preview := CommandPreview(ToolGH, toolPath, pubPath, title, keyType)
	_, _ = UploadKey(ToolGH, toolPath, pubPath, title, keyType, deps)

	if len(*calls) != 1 {
		t.Fatalf("RunCmd call count: got %d want 1", len(*calls))
	}
	c := (*calls)[0]
	// Reconstruct the "run" string from the actual RunCmd call.
	runParts := append([]string{c.name}, c.args...)
	runStr := strings.Join(runParts, " ")

	if preview != runStr {
		t.Errorf("shown command != run command:\n  preview: %q\n  run:     %q", preview, runStr)
	}
}

// TestUploadShownEqualsRun is the named UP-02 regression: the space-joined
// tail of CommandPreview's output must equal the recorded RunCmd invocation
// for the same inputs — buildArgs is the ONE function behind both, so
// shown==run is structural rather than merely asserted per test case.
func TestUploadShownEqualsRun(t *testing.T) {
	runCmd, calls := recordingRunCmd(0, "")
	deps := Deps{RunCmd: runCmd}

	pubPath := "~/.ssh/id_ed25519.pub"
	title := "gitid: acme @ my-machine"
	keyType := KeyAuthentication
	toolPath := "/fake/gh"

	preview := CommandPreview(ToolGH, toolPath, pubPath, title, keyType)
	if _, err := UploadKey(ToolGH, toolPath, pubPath, title, keyType, deps); err != nil {
		t.Fatalf("UploadKey: unexpected error: %v", err)
	}

	if len(*calls) != 1 {
		t.Fatalf("RunCmd call count: got %d want 1", len(*calls))
	}
	c := (*calls)[0]
	runStr := strings.Join(append([]string{c.name}, c.args...), " ")
	if preview != runStr {
		t.Errorf("shown command != run command:\n  preview: %q\n  run:     %q", preview, runStr)
	}
}

// TestCommandPreview_GLab verifies the glab form of the preview string.
func TestCommandPreview_GLab(t *testing.T) {
	preview := CommandPreview(ToolGLab, "/fake/glab", "~/.ssh/id_ed25519.pub", "gitid: work", GLabKeyTypeForAuth)
	want := "/fake/glab ssh-key add ~/.ssh/id_ed25519.pub -t gitid: work --usage-type auth"
	if preview != want {
		t.Errorf("preview:\n  got:  %q\n  want: %q", preview, want)
	}
}

// TestCommandPreview_UnknownToolReturnsErrorMessage verifies graceful handling
// of an invalid Tool value.
func TestCommandPreview_UnknownToolReturnsErrorMessage(t *testing.T) {
	preview := CommandPreview(Tool(99), "/fake/tool", "k.pub", "title", "auth")
	if !strings.HasPrefix(preview, "(preview unavailable:") {
		t.Errorf("unexpected preview for unknown tool: %q", preview)
	}
}

// ---- helpers ---------------------------------------------------------------

func assertArgs(t *testing.T, gotName string, gotArgs []string, wantName string, wantArgs []string) {
	t.Helper()
	if gotName != wantName {
		t.Errorf("RunCmd name: got %q want %q", gotName, wantName)
	}
	if len(gotArgs) != len(wantArgs) {
		t.Errorf("RunCmd args len: got %d want %d\n  got:  %v\n  want: %v", len(gotArgs), len(wantArgs), gotArgs, wantArgs)
		return
	}
	for i := range wantArgs {
		if gotArgs[i] != wantArgs[i] {
			t.Errorf("RunCmd args[%d]: got %q want %q", i, gotArgs[i], wantArgs[i])
		}
	}
}

func safeGet(s []string, i int) string {
	if i < len(s) {
		return s[i]
	}
	return "<missing>"
}

// TestProviderForHostnameContainsNoUnanchoredSubstringTest is R2's source-
// level guard: it parses THIS package's own uploader.go, isolates
// ProviderForHostname's function body (and ONLY that body — comment lines
// are stripped by go/parser itself, so prose about the review can never
// satisfy or trip this), and fails if that body calls strings.Contains
// directly. The gate must be built from equality plus a dot-anchored
// suffix test (isMainDomainOrSubdomain), never an unanchored containment
// predicate — a future `strings.Contains(host, "github")` reimplementation
// fails this test even though TestProviderForHostnameMatchesHostBoundaries
// might still pass on an incomplete case table.
func TestProviderForHostnameContainsNoUnanchoredSubstringTest(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "uploader.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing uploader.go: %v", err)
	}

	var fn *ast.FuncDecl
	ast.Inspect(file, func(n ast.Node) bool {
		if f, ok := n.(*ast.FuncDecl); ok && f.Name.Name == "ProviderForHostname" {
			fn = f
			return false
		}
		return true
	})
	if fn == nil || fn.Body == nil {
		t.Fatal("ProviderForHostname not found in uploader.go — has it been renamed?")
	}

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}
		if pkg.Name == "strings" && sel.Sel.Name == "Contains" {
			t.Error("ProviderForHostname's body calls strings.Contains directly — R2 requires equality + a dot-anchored suffix test (isMainDomainOrSubdomain), never an unanchored containment predicate")
		}
		return true
	})
}
