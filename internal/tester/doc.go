// Package tester performs SSH connectivity tests for gitid identities.
// It runs two phases: an explicit-key test using ssh -i <keypath> -T <host>,
// and a resolved-config test using ssh -T <alias> plus ssh -G <alias>.
// It returns a structured Result carrying the exact command run (input) and the
// raw combined output (TEST-03). Classification is strictly by output substring —
// PASS / ReachableNotUploaded / Failure — never by the (unreliable) exit code
// (D-01, Pitfall 2). ParseResolved reads `ssh -G` lowercase keys (Pitfall 3).
// All operations are read-only (no side effects).
package tester
