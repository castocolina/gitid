package globalgit

import "sort"

// SetKey is one git config key actually SET somewhere on this machine (not a
// catalogue entry — git has no such catalogue, D-01/D-02): its lowercase
// key, its effective value, the scope word git printed (global/system/local/
// command), and its origin file (or non-file origin string, e.g. "command
// line"), with the "file:" prefix already stripped.
type SetKey struct {
	Key, Value, Scope, Origin string
}

// AllSetKeys returns every git config key actually set on the machine
// (PROP-02), delegating to the EXISTING unexported effectiveProbe(deps) —
// per D-E, this does NOT add a second `git config` invocation and does NOT
// narrow the scope with `--global`: the system scope is real and is exactly
// the "set at a scope gitid cannot change" provenance class this screen
// exists to expose.
//
// deps.NonRepoCwd MUST NOT be inside any git repository — inherited
// unchanged from effectiveProbe's own contract (RESEARCH Pitfall 1):
// running from inside a repository would fold that repo's local scope (and
// any includeIf-matched fragment) into a screen labelled Global, silently
// misrepresenting the machine's global/system state as if it were the
// repo's. The caller (cmd/gitid/wiring.go) supplies the SAME non-repository
// directory the existing Options probe already uses (b.fragmentDir).
//
// This list is honestly scoped to "what is SET" — never a fabricated
// catalogue of every possible git config key, because git's key space is
// open-ended per subsystem and no such catalogue exists (D-01/D-02).
//
// The result is sorted ascending by Key so two calls over the same payload
// produce byte-identical output — Go map iteration order is not stable, and
// an unsorted list would make the render (and every downstream golden) flap.
func AllSetKeys(deps Deps) ([]SetKey, error) {
	m, err := effectiveProbe(deps)
	if err != nil {
		return nil, err
	}
	out := make([]SetKey, 0, len(m))
	for k, e := range m {
		out = append(out, SetKey{Key: k, Value: e.Value, Scope: e.Scope, Origin: e.Origin})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}
