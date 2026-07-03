package keygen

import "fmt"

// generatorFunc generates in-memory key Material for a Params request. Every
// entry in registry is one of two shapes: a real generator (e.g.
// generateEd25519, generateRSA4096) or a notYetImplemented stub. Registry
// PRESENCE never implies generation support — only entries backed by a real
// generator can return key material; stubs always error (T-01-21).
type generatorFunc func(p Params) (Material, error)

// registry is the name-keyed algorithm dispatch table GenerateMaterial looks
// up. It is populated by init() below via Register, so adding a new
// algorithm (real or stub) never requires touching GenerateMaterial itself.
var registry = map[string]generatorFunc{}

// Register adds or overrides the generator for the named algorithm in the
// package-level dispatch table. Algorithm names are the same strings the
// catalog (catalog.go) uses as AlgoInfo.Name and callers pass as Params.Algo.
func Register(name string, gen generatorFunc) {
	registry[name] = gen
}

// notYetImplemented returns a generatorFunc that always returns a zero-value
// Material and a named "not yet implemented" error, regardless of the Params
// passed in. It is used to register algorithms the catalog (KEY-01) lists as
// one of the top 5 but that gitid cannot yet generate (ecdsa-p256,
// ed25519-sk, ecdsa-sk, per D-05/D-06) — registering the name lets the
// catalog surface the algorithm without pretending it is generatable. It
// NEVER returns partial key bytes.
func notYetImplemented(name string) generatorFunc {
	return func(_ Params) (Material, error) {
		return Material{}, fmt.Errorf("keygen: algorithm %q is not yet implemented", name)
	}
}

// init registers every algorithm the registry knows about: the two real
// generators (ed25519, rsa-4096) and the three not-yet-implemented stubs that
// round out the top-5 catalog (KEY-01/KEY-02).
func init() {
	Register("ed25519", generateEd25519)
	Register("rsa-4096", generateRSA4096)
	Register("ecdsa-p256", notYetImplemented("ecdsa-p256"))
	Register("ed25519-sk", notYetImplemented("ed25519-sk"))
	Register("ecdsa-sk", notYetImplemented("ecdsa-sk"))
}
