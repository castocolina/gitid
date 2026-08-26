package sshconfig

import "strings"

// DirectiveHit is one occurrence of a requested directive found by a
// ScanDirectives pass: the canonical key spelling, its value, the enclosing
// Host pattern (empty when the directive is not nested inside a Host stanza),
// the source path the caller told the scanner about, and the 1-based line
// number the directive actually appeared on.
type DirectiveHit struct {
	Key         string
	Value       string
	HostPattern string
	SourcePath  string
	Line        int
}

// ScanDirectives walks content line by line, tracking the enclosing Host
// pattern, and returns one hit per occurrence of a requested key with its
// 1-based line number. It is the directive-aware, line-aware, best-effort
// parser API the global-ssh provenance label and the shadow-naming pass both
// need: AllHostStanzas cannot serve that purpose, because it exposes only an
// alias and a hostname, with no directive value and no line number — which is
// exactly why the review's HIGH provenance finding forbids building a label
// from it.
//
// Scope honesty: the scanner is single-file and Include-unaware, exactly like
// its neighbours in reader.go (AllHostStanzas, MatchingHostStanzas), so it is
// BEST-EFFORT naming only and callers MUST treat an empty result as "cannot
// name" rather than "does not exist" — a directive that reaches the machine
// through a user Include, a Match block or an environment option is invisible
// here. A caller must never promise a line the scanner did not actually
// report.
func ScanDirectives(content []byte, sourcePath string, keys []string) []DirectiveHit {
	want := make(map[string]string, len(keys)) // lowercase -> canonical spelling
	for _, k := range keys {
		want[strings.ToLower(k)] = k
	}
	var hits []DirectiveHit
	pattern := ""
	for i, line := range strings.Split(string(content), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 2 {
			continue
		}
		if strings.EqualFold(fields[0], "Host") {
			// Track the enclosing Host pattern (first alias token); an empty
			// pattern means "not nested inside a Host stanza". Multiple Host
			// lines simply move the current stanza.
			pattern = fields[1]
			continue
		}
		key, ok := want[strings.ToLower(fields[0])]
		if !ok {
			continue
		}
		hits = append(hits, DirectiveHit{
			Key:         key,
			Value:       fields[1],
			HostPattern: pattern,
			SourcePath:  sourcePath,
			Line:        i + 1, // 1-based, as the provenance label promises
		})
	}
	return hits
}
