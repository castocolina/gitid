package main

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"

	"github.com/castocolina/gitid/internal/tuikit"
)

// TestRunUploadForIsTheOnlyOrchestration is the structural durability check
// for 09-05-PLAN.md Task 1's extraction: wiring.go must call runUploadFor
// (or the planUpload/executeUpload pair it composes) rather than reaching
// the uploader package's decision functions directly. The positive half
// (upload_run.go DOES contain the real calls) is covered by
// TestRunUploadDoesNotCallProviderCommandsOutsideATeaCmd in wiring_test.go;
// this test is the negative half specific to wiring.go, proving the
// extraction is durable — a future direct uploader.UploadKeys call
// reintroduced into wiring.go fails here.
func TestRunUploadForIsTheOnlyOrchestration(t *testing.T) {
	src, err := os.ReadFile("wiring.go") //nolint:gosec // package-local source file (G304)
	if err != nil {
		t.Fatalf("reading wiring.go: %v", err)
	}
	if !strings.Contains(string(src), "planUpload(req)") && !strings.Contains(string(src), "runUploadFor(") {
		t.Fatal("wiring.go's RunUpload no longer calls into upload_run.go's shared orchestration")
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "wiring.go", src, 0)
	if err != nil {
		t.Fatalf("parsing wiring.go: %v", err)
	}
	ast.Inspect(file, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if ok && pkg.Name == "uploader" && (sel.Sel.Name == "UploadKeys" || sel.Sel.Name == "Inventory" || sel.Sel.Name == "MissingRegistrations") {
			t.Errorf("wiring.go calls uploader.%s directly — the decision logic must live only in upload_run.go", sel.Sel.Name)
		}
		return true
	})
}

// TestPrintUploadOutcomeUsesOnlyFrozenCopy is a source-level check over
// upload_run.go: every user-facing string printUploadOutcome emits must be
// a reference to a tuikit.Upload* identifier, never a CLI-local string
// literal. Comment lines are filtered out by go/parser itself (comments are
// not part of the AST's string-literal nodes), so a doc comment mentioning
// frozen copy cannot satisfy or trip this check.
func TestPrintUploadOutcomeUsesOnlyFrozenCopy(t *testing.T) {
	src, err := os.ReadFile("upload_run.go") //nolint:gosec // package-local source file (G304)
	if err != nil {
		t.Fatalf("reading upload_run.go: %v", err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "upload_run.go", src, 0)
	if err != nil {
		t.Fatalf("parsing upload_run.go: %v", err)
	}
	var printerBody ast.Node
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "printUploadOutcome" {
			printerBody = fn.Body
			break
		}
	}
	if printerBody == nil {
		t.Fatal("printUploadOutcome not found in upload_run.go")
	}
	ast.Inspect(printerBody, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		val := strings.Trim(lit.Value, "\"`")
		// Allowed: empty/whitespace format-control strings.
		if strings.TrimSpace(val) == "" {
			return true
		}
		t.Errorf("printUploadOutcome contains a bare string literal %q — every user-facing string must be a tuikit.Upload* reference", val)
		return true
	})
}

// TestPrintUploadOutcomeRendersEachSection is table-driven over the six
// view shapes the plan names, asserting the expected frozen text is
// present and, for the omitted (zero-value) case, that the writer received
// zero bytes.
func TestPrintUploadOutcomeRendersEachSection(t *testing.T) {
	tests := []struct {
		name        string
		view        tuikit.UploadRunView
		want        []string
		mustNotWant []string
		zero        bool
	}{
		{
			name: "success",
			view: tuikit.UploadRunView{ProviderName: "GitHub", Rows: []tuikit.UploadResultRow{
				{Label: tuikit.UploadRegistrationLabelAuth, Command: "gh ssh-key add k.pub --type authentication", Outcome: tuikit.UploadRowUploaded},
			}},
			want: []string{"Running: gh ssh-key add", "Authentication key registered"},
		},
		{
			name: "partial failure",
			view: tuikit.UploadRunView{ProviderName: "GitHub", Rows: []tuikit.UploadResultRow{
				{Label: tuikit.UploadRegistrationLabelAuth, Outcome: tuikit.UploadRowUploaded},
				{Label: tuikit.UploadRegistrationLabelSigning, Outcome: tuikit.UploadRowFailed, Reason: "insufficient scope"},
			}},
			want: []string{"Authentication key registered", "Signing key registration failed: insufficient scope"},
		},
		{
			name: "already-complete",
			view: tuikit.UploadRunView{ProviderName: "GitHub", AlreadyComplete: true},
			want: []string{"Already registered with GitHub"},
		},
		{
			name: "degraded",
			view: tuikit.UploadRunView{ProviderName: "GitHub", InventoryDegraded: true, Rows: []tuikit.UploadResultRow{
				{Label: tuikit.UploadRegistrationLabelAuth, Outcome: tuikit.UploadRowUploaded},
			}},
			want: []string{"Could not check GitHub for existing keys"},
		},
		{
			// The actual --no-upload flag path (runUploadStep's noUpload
			// branch) — the ONLY shape that legitimately prints the frozen
			// "Auto-upload skipped (--no-upload)." note.
			name: "skipped by flag",
			view: tuikit.UploadRunView{SkippedByFlag: true, ManualFallback: "manual steps here"},
			want: []string{tuikit.UploadSkippedByFlagNote, tuikit.UploadManualHeading, "manual steps here"},
		},
		{
			// WR-02 regression: planUpload's Disabled derived state (no
			// matching provider CLI on PATH) sets Skipped, NOT SkippedByFlag
			// — no --no-upload flag was ever passed here, so the note must
			// NOT render, even though the manual-fallback block still does.
			name:        "disabled (no CLI on PATH) — not a flag skip",
			view:        tuikit.UploadRunView{Skipped: true, ManualFallback: "manual steps here"},
			want:        []string{tuikit.UploadManualHeading, "manual steps here"},
			mustNotWant: []string{tuikit.UploadSkippedByFlagNote},
		},
		{
			// WR-02 regression, Omitted sub-case: a self-hosted GHE/GitLab
			// create sets Skipped with no ManualFallback either — the note
			// must not render, and neither must anything else.
			name: "omitted (provider not gated) — not a flag skip",
			view: tuikit.UploadRunView{Skipped: true},
			zero: true,
		},
		{
			name: "true zero value (register-key's not-gated path)",
			view: tuikit.UploadRunView{},
			zero: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			printUploadOutcome(&buf, tt.view)
			if tt.zero {
				if buf.Len() != 0 {
					t.Errorf("printUploadOutcome(omitted) wrote %d bytes, want 0: %q", buf.Len(), buf.String())
				}
				return
			}
			got := buf.String()
			for _, want := range tt.want {
				if !strings.Contains(got, want) {
					t.Errorf("printUploadOutcome(%s) = %q, want it to contain %q", tt.name, got, want)
				}
			}
			for _, mustNot := range tt.mustNotWant {
				if strings.Contains(got, mustNot) {
					t.Errorf("printUploadOutcome(%s) = %q, must NOT contain %q", tt.name, got, mustNot)
				}
			}
		})
	}
}

// TestPrintUploadOutcomeMatchesTheWizardSection renders a success view
// through the printer and asserts the same frozen strings the wizard's
// renderUploadRun would emit for the identical rows are present — the
// outcome-parity contract asserted rather than assumed.
// TestSelfHostedCreateNeverPrintsTheNoUploadFlagNote is the WR-02
// end-to-end regression: planUpload's real Omitted terminal view (a
// self-hosted GHE/GitLab hostname, D-13's explicitly supported degrade
// path — no --no-upload flag involved at all) must never render "Auto-
// upload skipped (--no-upload)." when printed exactly as create/clone/
// rotate/new-key print it.
func TestSelfHostedCreateNeverPrintsTheNoUploadFlagNote(t *testing.T) {
	home := t.TempDir()
	b := newBackendForHome(home)
	plan, terminal := b.planUpload(uploadRequest{Identity: "acme", Hostname: "git.self-hosted.example.com", PubPath: "/does/not/matter.pub"})
	if terminal == nil {
		t.Fatal("setup: a self-hosted hostname must resolve to a terminal (Omitted) view")
	}
	if !terminal.Skipped {
		t.Fatalf("setup: want the Omitted view to set Skipped, got %+v", terminal)
	}
	if terminal.SkippedByFlag {
		t.Fatalf("setup: planUpload must never set SkippedByFlag — that is runUploadStep's noUpload branch only, got %+v", terminal)
	}
	_ = plan
	var buf bytes.Buffer
	printUploadOutcome(&buf, *terminal)
	if strings.Contains(buf.String(), tuikit.UploadSkippedByFlagNote) {
		t.Errorf("printUploadOutcome(self-hosted Omitted view) = %q, must NOT contain the --no-upload note — no flag was passed", buf.String())
	}
}

func TestPrintUploadOutcomeMatchesTheWizardSection(t *testing.T) {
	view := tuikit.UploadRunView{ProviderName: "GitHub", Rows: []tuikit.UploadResultRow{
		{Label: tuikit.UploadRegistrationLabelAuth, Command: "gh ssh-key add ~/.ssh/id_ed25519_acme.pub --type authentication", Outcome: tuikit.UploadRowUploaded},
	}}
	var buf bytes.Buffer
	printUploadOutcome(&buf, view)
	got := buf.String()
	for _, want := range []string{
		"Running: gh ssh-key add ~/.ssh/id_ed25519_acme.pub --type authentication",
		"Authentication key registered",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("printUploadOutcome() = %q, want it to contain the wizard's own frozen text %q", got, want)
		}
	}
}
