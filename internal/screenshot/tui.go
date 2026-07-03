//go:build screenshot

package screenshot

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Result is the outcome of a screenshot render: the PNG's path on disk and
// its deterministic SHA-256 hash, computed after StripPNGMetadata so the
// hash reflects only the visual pixel content (D-01/D-02/D-04).
type Result struct {
	PNGPath string
	SHA256  string
}

// TUIOptions configures a single CaptureTUI render. Every field that affects
// the rendered pixels (FontFile, Theme) must be held fixed across runs — see
// Pitfall 6 in 01-RESEARCH.md: freeze's default font discovery is NOT
// CI-deterministic (it falls back to whatever monospace font the OS happens
// to have installed), so FontFile is always required, on every OS.
type TUIOptions struct {
	// FreezeBin is the path to the freeze binary. If empty, "freeze" is
	// resolved via exec.LookPath (installed by `make setup-env` at the
	// pinned github.com/charmbracelet/freeze@v0.2.2).
	FreezeBin string

	// FontFile is the vendored monospace TTF passed to freeze's
	// --font.file flag, so rendering never depends on system-font
	// discovery (Pitfall 6). Required.
	FontFile string

	// Theme is the fixed freeze --theme name. Required.
	Theme string

	// Width and Height record the terminal geometry (columns x rows) the
	// golden View() dump was captured at (D-04: fixed capture geometry,
	// e.g. 100x30 per the CONTEXT.md example). They are metadata about how
	// the golden string was produced; freeze itself sizes the output image
	// to fit the rendered content, so they are not passed to freeze.
	Width, Height int

	// OutDir is the directory the PNG is written under, e.g.
	// .planning/design/_spike/tui. Created if missing.
	OutDir string

	// Name is the output file's base name (without extension).
	Name string
}

// CaptureTUI renders a captured Bubble Tea View() dump (golden — a plain
// string that may contain ANSI escape codes) to a deterministic PNG via
// freeze: vendored font, fixed theme, stripped timestamp metadata, and a
// recorded SHA-256 golden hash (D-01/D-02/D-04).
//
// golden is written to a private temp file first, then passed to freeze as
// a bare positional file argument — confirmed via `freeze --help` this
// session (`freeze main.go [-o code.svg] [--flags]`) and empirically
// verified to render raw ANSI escape codes with correct color, not just
// syntax-highlighted plain text. This resolves RESEARCH.md's Open Question
// 1: no `--execute "cat golden"` subprocess re-invocation is needed.
func CaptureTUI(golden string, opts TUIOptions) (Result, error) {
	if opts.FontFile == "" {
		return Result{}, fmt.Errorf("screenshot: CaptureTUI: FontFile is required for deterministic rendering (Pitfall 6)")
	}
	if opts.Theme == "" {
		return Result{}, fmt.Errorf("screenshot: CaptureTUI: Theme is required for deterministic rendering")
	}
	if opts.OutDir == "" || opts.Name == "" {
		return Result{}, fmt.Errorf("screenshot: CaptureTUI: OutDir and Name are required")
	}

	freezeBin := opts.FreezeBin
	if freezeBin == "" {
		var err error
		freezeBin, err = exec.LookPath("freeze")
		if err != nil {
			return Result{}, fmt.Errorf("screenshot: CaptureTUI: freeze binary not found on PATH (run `make setup-env`): %w", err)
		}
	}

	if err := os.MkdirAll(opts.OutDir, 0o750); err != nil {
		return Result{}, fmt.Errorf("screenshot: CaptureTUI: creating output dir %q: %w", opts.OutDir, err)
	}

	goldenPath, cleanup, err := writeGoldenTempFile(golden)
	if err != nil {
		return Result{}, err
	}
	defer cleanup()

	pngPath := filepath.Join(opts.OutDir, opts.Name+".png")
	args := []string{goldenPath, "-o", pngPath, "--font.file", opts.FontFile, "--theme", opts.Theme}
	cmd := exec.Command(freezeBin, args...) //nolint:gosec // arg-slice form, no shell; freezeBin resolved via exec.LookPath/explicit config, all other args are fixed gitid-owned paths/flags (G204)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return Result{}, fmt.Errorf("screenshot: CaptureTUI: running freeze: %w\n%s", err, out)
	}

	return finalizePNG(pngPath)
}

// writeGoldenTempFile writes golden to a private temp .txt file and returns
// its path plus a cleanup func that removes it.
func writeGoldenTempFile(golden string) (path string, cleanup func(), err error) {
	f, err := os.CreateTemp("", "gitid-screenshot-tui-*.txt")
	if err != nil {
		return "", nil, fmt.Errorf("screenshot: creating golden temp file: %w", err)
	}
	path = f.Name()
	cleanup = func() { _ = os.Remove(path) }

	if _, err := f.WriteString(golden); err != nil {
		_ = f.Close()
		cleanup()
		return "", nil, fmt.Errorf("screenshot: writing golden temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("screenshot: closing golden temp file: %w", err)
	}
	return path, cleanup, nil
}

// finalizePNG reads a freshly-rendered PNG, strips non-deterministic
// metadata, rewrites the file with the stripped bytes, and returns its
// Result (path + the stripped file's SHA-256 golden hash).
func finalizePNG(pngPath string) (Result, error) {
	raw, err := os.ReadFile(pngPath) //nolint:gosec // pngPath is gitid-controlled (OutDir/Name from validated caller config), never external input (G304)
	if err != nil {
		return Result{}, fmt.Errorf("screenshot: reading rendered PNG %q: %w", pngPath, err)
	}
	stripped, err := StripPNGMetadata(raw)
	if err != nil {
		return Result{}, fmt.Errorf("screenshot: stripping metadata from %q: %w", pngPath, err)
	}
	if err := os.WriteFile(pngPath, stripped, 0o600); err != nil { //nolint:gosec // pngPath is gitid-controlled (OutDir/Name from validated caller config), never external input (G703)
		return Result{}, fmt.Errorf("screenshot: rewriting stripped PNG %q: %w", pngPath, err)
	}
	hash, err := HashPNG(stripped)
	if err != nil {
		return Result{}, fmt.Errorf("screenshot: hashing %q: %w", pngPath, err)
	}
	return Result{PNGPath: pngPath, SHA256: hash}, nil
}
