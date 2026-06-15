// Package tester performs SSH connectivity tests for gitid identities.
// It runs two phases: an explicit-key test using ssh -i <keypath> -T <host>,
// and a resolved-config test using ssh -T <alias> plus ssh -G <alias>.
// It returns a structured result with a pass/fail indicator and raw output.
// All operations are read-only (no side effects).
//
// Implementation lands in a later phase (Phase 2+).
package tester
