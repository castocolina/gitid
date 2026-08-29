package uploader

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/keygen"
)

// ---- Fake helpers ----------------------------------------------------------

// notFoundError mimics exec.ErrNotFound for LookPath stubs.
type notFoundError struct{ name string }

func (e *notFoundError) Error() string { return e.name + ": executable file not found in $PATH" }

// recordingRunCmd returns a RunCmd that records every call and returns the
// configured exit code. exitCode 0 => success; non-zero => error.
type runCall struct {
	name string
	args []string
}

const validPublicKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIE4NwfYkJBFiDeHTmC4hLTspz93ciIR3ViPhg1gA4uER test"

func testReadFile(string) ([]byte, error) { return []byte(validPublicKey), nil }

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
// DetectFor("github", ...) answer found=false, and glab must never appear
// in the recorded LookPath calls.
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

	tool, path, found := DetectFor("github", deps)
	if found {
		t.Errorf("DetectFor(github): found = %v, want false", found)
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

	_, _, found := DetectFor("", deps)
	if found {
		t.Errorf("DetectFor(\"\"): found = %v, want false", found)
	}
	if len(lookedUp) != 0 {
		t.Errorf("DetectFor(\"\") must not call LookPath; recorded calls: %v", lookedUp)
	}
}

// TestDetectForReportsFoundWithoutProbingAuth is the WR-15 regression:
// DetectFor's third return value must be a plain found bool, never an
// AuthStatus borrowed from a probe it never actually ran — a found tool
// answers found=true regardless of whatever RunCmd would say about its
// auth state, because DetectFor must never call RunCmd at all.
func TestDetectForReportsFoundWithoutProbingAuth(t *testing.T) {
	ranRunCmd := false
	deps := Deps{
		LookPath: func(string) (string, error) { return "/fake/gh", nil },
		RunCmd:   func(string, ...string) (string, int, error) { ranRunCmd = true; return "", 1, nil },
	}
	tool, path, found := DetectFor("github", deps)
	if !found {
		t.Fatal("found = false, want true — the tool IS on PATH")
	}
	if tool != ToolGH || path != "/fake/gh" {
		t.Errorf("tool=%d path=%q, want ToolGH and /fake/gh", tool, path)
	}
	if ranRunCmd {
		t.Error("DetectFor called RunCmd — it must never probe authentication, only PATH presence")
	}
}

// ---- TestUploadKey ---------------------------------------------------------

func TestUploadKey_GHAuthentication(t *testing.T) {
	runCmd, calls := recordingRunCmd(0, "Added SSH key.")
	result := uploadKey(ToolGH, "/fake/gh", "key.pub", RegistrationRequest{Registration: RegistrationAuthentication, Title: "gitid: personal"}, Deps{RunCmd: runCmd, ReadFile: testReadFile})
	if result.Outcome != OutcomeUploaded || result.Err != nil {
		t.Fatalf("result = %+v", result)
	}
	assertArgs(t, (*calls)[0].name, (*calls)[0].args, "/fake/gh", []string{"ssh-key", "add", "key.pub", "--title", "gitid: personal", "--type", "authentication"})
}

func TestUploadKey_GLab(t *testing.T) {
	runCmd, calls := recordingRunCmd(0, "Added SSH key.")
	result := uploadKey(ToolGLab, "/fake/glab", "key.pub", RegistrationRequest{Registration: RegistrationCombined, Title: "gitid: work"}, Deps{RunCmd: runCmd, ReadFile: testReadFile})
	if result.Outcome != OutcomeUploaded {
		t.Fatalf("result = %+v", result)
	}
	assertArgs(t, (*calls)[0].name, (*calls)[0].args, "/fake/glab", []string{"ssh-key", "add", "key.pub", "-t", "gitid: work", "--usage-type", "auth_and_signing"})
}

func TestUploadKey_ErrorSurfacesOutput(t *testing.T) {
	runCmd, _ := recordingRunCmd(1, "error: not authenticated")
	result := uploadKey(ToolGH, "/fake/gh", "key.pub", RegistrationRequest{Registration: RegistrationAuthentication}, Deps{RunCmd: runCmd, ReadFile: testReadFile})
	if result.Err == nil || result.Output != "error: not authenticated" {
		t.Fatalf("result = %+v", result)
	}
}

func TestUploadShownEqualsRun(t *testing.T) {
	runCmd, calls := recordingRunCmd(0, "")
	preview := CommandPreview(ToolGH, "/fake/gh", "key.pub", "title", KeyAuthentication)
	uploadKey(ToolGH, "/fake/gh", "key.pub", RegistrationRequest{Registration: RegistrationAuthentication, Title: "title"}, Deps{RunCmd: runCmd, ReadFile: testReadFile})
	if preview != strings.Join(append([]string{(*calls)[0].name}, (*calls)[0].args...), " ") {
		t.Fatal("shown command != run command")
	}
}

func TestCommandPreview_UnknownToolReturnsErrorMessage(t *testing.T) {
	if !strings.HasPrefix(CommandPreview(Tool(99), "/fake/tool", "k.pub", "title", "auth"), "(preview unavailable:") {
		t.Fatal("missing error preview")
	}
}

func TestUploadKeyRefusesNonPubPath(t *testing.T) {
	calls := 0
	result := uploadKey(ToolGH, "gh", "private", RegistrationRequest{}, Deps{ReadFile: testReadFile, RunCmd: func(string, ...string) (string, int, error) { calls++; return "", 0, nil }})
	if result.Err == nil || calls != 0 {
		t.Fatalf("result=%+v calls=%d", result, calls)
	}
}

func TestUploadKeyRefusesAPrivateKeyRenamedAsPub(t *testing.T) {
	material, err := keygen.GenerateMaterial(keygen.Params{Algo: "ed25519", Identity: "upload", Comment: "upload@gitid"})
	if err != nil {
		t.Fatalf("GenerateMaterial: %v", err)
	}
	path := filepath.Join(t.TempDir(), "renamed.pub")
	if err := os.WriteFile(path, material.PrivPEM, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	calls := 0
	result := uploadKey(ToolGH, "gh", path, RegistrationRequest{}, Deps{
		ReadFile: os.ReadFile,
		RunCmd:   func(string, ...string) (string, int, error) { calls++; return "", 0, nil },
	})
	if result.Err == nil || !strings.Contains(result.Err.Error(), "private key content") || calls != 0 {
		t.Fatalf("result=%+v calls=%d", result, calls)
	}
}

func TestUploadKeyRefusesUnparseablePublicKeyContent(t *testing.T) {
	calls := 0
	result := uploadKey(ToolGH, "gh", "bad.pub", RegistrationRequest{}, Deps{ReadFile: func(string) ([]byte, error) { return []byte("bad"), nil }, RunCmd: func(string, ...string) (string, int, error) { calls++; return "", 0, nil }})
	if result.Err == nil || calls != 0 {
		t.Fatalf("result=%+v calls=%d", result, calls)
	}
}

func TestUploadKeyAcceptsARealPublicKey(t *testing.T) {
	material, err := keygen.GenerateMaterial(keygen.Params{Algo: "ed25519", Identity: "upload", Comment: "upload@gitid"})
	if err != nil {
		t.Fatalf("GenerateMaterial: %v", err)
	}
	path := filepath.Join(t.TempDir(), "key.pub")
	if err := os.WriteFile(path, []byte(material.PubLine), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	runCmd, calls := recordingRunCmd(0, "")
	result := uploadKey(ToolGH, "gh", path, RegistrationRequest{Registration: RegistrationAuthentication}, Deps{ReadFile: os.ReadFile, RunCmd: runCmd})
	if result.Outcome != OutcomeUploaded || result.Err != nil || len(*calls) != 1 {
		t.Fatalf("result=%+v calls=%+v", result, calls)
	}
}

func TestUploadKeysHonoursPerRegistrationTitles(t *testing.T) {
	runCmd, calls := recordingRunCmd(0, "")
	results := UploadKeys(ToolGH, "gh", "key.pub", []RegistrationRequest{{Registration: RegistrationAuthentication, Title: "auth"}, {Registration: RegistrationSigning, Title: "sign"}}, Deps{ReadFile: testReadFile, RunCmd: runCmd})
	if len(results) != 2 || results[0].Title != "auth" || results[1].Title != "sign" || (*calls)[0].args[4] != "auth" || (*calls)[1].args[4] != "sign" {
		t.Fatalf("results=%+v calls=%+v", results, calls)
	}
}

func TestUploadKeysNeverSubstitutesTheProductTitle(t *testing.T) {
	runCmd, calls := recordingRunCmd(0, "")
	const policyTitle = "gitid-e2e:run-1:signing"
	UploadKeys(ToolGH, "gh", "key.pub", []RegistrationRequest{{Registration: RegistrationAuthentication, Title: policyTitle}}, Deps{ReadFile: testReadFile, RunCmd: runCmd})
	if len(*calls) != 1 || (*calls)[0].args[4] != policyTitle {
		t.Fatalf("calls=%+v", calls)
	}
}

func TestRegistrationRequestsWithTitleAppliesOneTitleToAll(t *testing.T) {
	got := RegistrationRequestsWithTitle("same", RegistrationAuthentication, RegistrationSigning)
	want := []RegistrationRequest{{Registration: RegistrationAuthentication, Title: "same"}, {Registration: RegistrationSigning, Title: "same"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got=%+v want=%+v", got, want)
	}
}

func TestUploadKeysSignatureUsesPerRegistrationRequests(t *testing.T) {
	typ := reflect.TypeOf(UploadKeys)
	requestSlice := reflect.TypeOf([]RegistrationRequest(nil))
	if typ.NumIn() != 5 || typ.In(3) != requestSlice || typ.In(3).Kind() != reflect.Slice {
		t.Fatalf("UploadKeys signature = %v, want fourth parameter []RegistrationRequest", typ)
	}
	for i := 3; i < typ.NumIn(); i++ {
		if typ.In(i).Kind() == reflect.String {
			t.Fatalf("UploadKeys has forbidden batch-wide title parameter at %d", i)
		}
	}
}

func TestUploadKeysAttemptsEveryRegistrationAfterAFailure(t *testing.T) {
	calls := 0
	results := UploadKeys(ToolGH, "gh", "key.pub", []RegistrationRequest{{Registration: RegistrationAuthentication}, {Registration: RegistrationSigning}}, Deps{ReadFile: testReadFile, RunCmd: func(string, ...string) (string, int, error) {
		calls++
		if calls == 1 {
			return "bad", 1, errors.New("bad")
		}
		return "ok", 0, nil
	}})
	if len(results) != 2 || results[0].Outcome != OutcomeFailed || calls != 2 {
		t.Fatalf("results=%+v calls=%d", results, calls)
	}
}

func TestGLabUsesCombinedUsageType(t *testing.T) {
	runCmd, calls := recordingRunCmd(0, "")
	uploadKey(ToolGLab, "glab", "key.pub", RegistrationRequest{Registration: RegistrationCombined}, Deps{ReadFile: testReadFile, RunCmd: runCmd})
	if (*calls)[0].args[6] != GLabKeyTypeAuthAndSigning {
		t.Fatal((*calls)[0].args)
	}
}

func TestKeyTitleIsMachineScoped(t *testing.T) {
	if got := KeyTitle("personal", "ramons-mbp.local"); got != "gitid: personal @ ramons-mbp" {
		t.Fatal(got)
	}
}
func TestTitleMatchesThisMachineRejectsOtherMachines(t *testing.T) {
	if titleMatchesThisMachine("gitid: personal @ other", "personal", "mine.local") {
		t.Fatal("other machine matched")
	}
}

func TestNoSecondSSHKeyAddArgvBuilder(t *testing.T) {
	root := filepath.Join("..", "..")
	dirs := []string{filepath.Join(root, "internal", "uploader"), filepath.Join(root, "internal", "tuikit"), filepath.Join(root, "cmd", "gitid")}
	files := 0
	for _, dir := range dirs {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				return err
			}
			hasSSHKey, hasAdd := false, false
			ast.Inspect(parsed, func(node ast.Node) bool {
				literal, ok := node.(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					return true
				}
				value, unquoteErr := strconv.Unquote(literal.Value)
				if unquoteErr != nil {
					return true
				}
				hasSSHKey = hasSSHKey || value == "ssh-key"
				hasAdd = hasAdd || value == "add"
				return true
			})
			if hasSSHKey && hasAdd {
				files++
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", dir, err)
		}
	}
	if files != 1 {
		t.Fatalf("ssh-key add argv builder files = %d, want 1", files)
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
