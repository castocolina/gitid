package sshconfig

import "strings"

// DirectiveSource names one slice of a real config file to be scanned by
// ScanDirectivesMulti. Path is the absolute path the content came from (used
// in every DirectiveHit.SourcePath the scan produces); Content is the slice of
// that file; LineOffset is added to each in-slice 1-based line to recover the
// true 1-based line in the file.
//
// LineOffset exists because a single file participates in the resolved order in
// TWO pieces when it contains an Include directive: the bytes above the
// directive are obtained BEFORE the included files, and the bytes below are
// obtained AFTER. A caller linearises that by emitting the entry point twice,
// around the included files, and the second slice needs a non-zero offset so
// that every hit reports the correct line in the file. Without it, a directive
// below an Include line would be misattributed by exactly the number of lines
// above it — a wrong file:line in a security warning is worse than none.
type DirectiveSource struct {
	Path       string
	Content    []byte
	LineOffset int
}

// ScanDirectivesMulti applies ScanDirectives to each source in the order
// given, adds each source's LineOffset to every hit's Line, and concatenates
// the results. The returned slice is therefore in RESOLUTION ORDER — the order
// the caller supplies — and every hit still carries the real file path and the
// true 1-based line in that file.
//
// The caller is responsible for supplying the sources in the correct resolution
// order; this function does NOT discover Include targets and does NOT model
// Match blocks. The result is BEST-EFFORT NAMING ONLY: a caller must never
// promise a line this function did not actually report, and must never use a
// scan result to DECIDE whether shadowing occurred (the probe decides that;
// this only labels). A directive that reaches the machine through a user
// Include the caller did not supply, a Match block, or an environment option
// is invisible here — the caller's honest bound, recorded in doc-comment form
// so nobody routes the decision through this API again (06-REVIEWS.md HIGH:
// AllHostStanzas cannot carry directive values or line numbers, and this
// function is the replacement that can — but it is still naming-only).
func ScanDirectivesMulti(sources []DirectiveSource, keys []string) []DirectiveHit {
	var all []DirectiveHit
	for _, src := range sources {
		hits := ScanDirectives(src.Content, src.Path, keys)
		if src.LineOffset != 0 {
			for i := range hits {
				hits[i].Line += src.LineOffset
			}
		}
		all = append(all, hits...)
	}
	return all
}

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
			// Track the enclosing Host pattern — ALL tokens after the Host
			// keyword, whitespace-joined, so that HostLineMatches can apply
			// the full negation-aware stanza rule (a single first-token miss
			// would make "Host * !gitid-probe.invalid" look like a plain "*"
			// stanza and match the probe host when it should not). An empty
			// pattern means "not nested inside a Host stanza". Multiple Host
			// lines simply move the current stanza.
			pattern = strings.Join(fields[1:], " ")
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
