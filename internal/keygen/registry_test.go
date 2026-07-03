package keygen

import (
	"strings"
	"testing"
)

// TestRegistry_StubAlgorithmsNeverReturnMaterial asserts that every
// registered-but-not-yet-implemented algorithm (ecdsa-p256, ed25519-sk,
// ecdsa-sk) returns a named "not yet implemented" error AND a zero-value
// Material — the generation path must never leak partial key bytes for an
// algorithm the catalog cannot actually generate (T-01-21).
func TestRegistry_StubAlgorithmsNeverReturnMaterial(t *testing.T) {
	stubs := []string{"ecdsa-p256", "ed25519-sk", "ecdsa-sk"}
	for _, algo := range stubs {
		algo := algo
		t.Run(algo, func(t *testing.T) {
			mat, err := GenerateMaterial(Params{Algo: algo, Identity: "x", Comment: "x@gitid"})
			if err == nil {
				t.Fatalf("GenerateMaterial(%q) returned nil error, want not-yet-implemented error", algo)
			}
			if !strings.Contains(err.Error(), "not yet implemented") {
				t.Errorf("GenerateMaterial(%q) error = %q, want to contain \"not yet implemented\"", algo, err.Error())
			}
			if !strings.Contains(err.Error(), algo) {
				t.Errorf("GenerateMaterial(%q) error = %q, want to name the algorithm", algo, err.Error())
			}
			if len(mat.PrivPEM) != 0 || mat.PubLine != "" {
				t.Errorf("GenerateMaterial(%q) returned non-zero Material %+v, want zero value (no partial key bytes)", algo, mat)
			}
		})
	}
}

// TestRegistry_UnsupportedAlgorithmReturnsError asserts an unknown algorithm
// name returns a clear "unsupported algorithm" error, never a panic.
func TestRegistry_UnsupportedAlgorithmReturnsError(t *testing.T) {
	mat, err := GenerateMaterial(Params{Algo: "bogus", Identity: "x", Comment: "x@gitid"})
	if err == nil {
		t.Fatal("GenerateMaterial(bogus) returned nil error, want unsupported algorithm error")
	}
	if !strings.Contains(err.Error(), "unsupported algorithm") {
		t.Errorf("error = %q, want to contain \"unsupported algorithm\"", err.Error())
	}
	if len(mat.PrivPEM) != 0 || mat.PubLine != "" {
		t.Errorf("GenerateMaterial(bogus) returned non-zero Material %+v, want zero value", mat)
	}
}

// TestRegistry_RegisterAddsDispatchEntry asserts Register adds a name-keyed
// entry that GenerateMaterial subsequently dispatches through — proving the
// registry is a real extensibility seam, not a hard-coded switch.
func TestRegistry_RegisterAddsDispatchEntry(t *testing.T) {
	const name = "test-only-algo"
	called := false
	Register(name, func(_ Params) (Material, error) {
		called = true
		return Material{PubLine: "test-marker\n"}, nil
	})
	defer delete(registry, name)

	mat, err := GenerateMaterial(Params{Algo: name})
	if err != nil {
		t.Fatalf("GenerateMaterial(%q): %v", name, err)
	}
	if !called {
		t.Error("Register'd generator was not invoked by GenerateMaterial dispatch")
	}
	if mat.PubLine != "test-marker\n" {
		t.Errorf("PubLine = %q, want the registered generator's output", mat.PubLine)
	}
}

// TestRegistry_Ed25519StillDispatchesThroughRegistry asserts the default
// algorithm continues to work unchanged after the registry refactor.
func TestRegistry_Ed25519StillDispatchesThroughRegistry(t *testing.T) {
	mat, err := GenerateMaterial(Params{Algo: "ed25519", Identity: "x", Comment: "x@gitid"})
	if err != nil {
		t.Fatalf("GenerateMaterial(ed25519): %v", err)
	}
	if !strings.HasPrefix(mat.PubLine, "ssh-ed25519 ") {
		t.Errorf("PubLine = %q, want ssh-ed25519 prefix", mat.PubLine)
	}
}
