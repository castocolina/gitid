package globalssh

import "sort"

// Directive is one directive `ssh -G` resolved: the lowercase key OpenSSH
// emits and its resolved value, trimmed.
type Directive struct {
	Key   string
	Value string
}

// AllDirectives returns EVERY directive `ssh -G` resolves for the Host *
// wildcard context — the full, bounded ~90-directive set, not gitid's
// curated six-row Policy subset (PROP-01). It is an EXPORTED sibling of the
// existing unexported effective(deps): the returned set is GLOBAL precisely
// because effective(deps) probes ProbeHost, the wildcard-only `.invalid`
// sentinel that matches only a `Host *` stanza and never a per-alias block
// (D-A, RESEARCH Pitfall 5). Per D-A this function does not write a second
// `ssh -G` parser and does not touch parseResolvedOptions, runProbe, or
// ProbeHost — they already do exactly the right thing.
//
// The result is sorted ascending by Key so two calls over the same payload
// produce byte-identical output — Go map iteration order is not stable, and
// an unsorted list would make the render (and every downstream golden) flap.
func AllDirectives(deps Deps) ([]Directive, error) {
	m, err := effective(deps)
	if err != nil {
		return nil, err
	}
	out := make([]Directive, 0, len(m))
	for k, v := range m {
		out = append(out, Directive{Key: k, Value: v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}
