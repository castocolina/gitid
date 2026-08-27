// Package globalgit is the git-side twin of internal/globalssh: it reads the
// machine's global git configuration, classifies each policy option's state,
// and reports provenance — WITHOUT EVER WRITING ANYTHING. This is a hard
// invariant: the package contains no call to os.WriteFile, filewriter.Write,
// or os.Create, and that is asserted by a test in this package. All writes
// flow through internal/gitconfig.EnsureGlobalGit, which is the ONE owner of
// the global-git managed block (D-01, 07-CONTEXT.md).
//
// The two D-03 probes this package runs both use native git invocations:
//
//   - Effective probe: `git config --show-origin --show-scope --list -z` run
//     from a caller-supplied non-repository directory, so a repository-local
//     value is never mistaken for a global one (T-07-06).
//
//   - In-file probe: `git config --file <path> --list -z` WITHOUT --includes,
//     so only keys physically present in that file are returned — the physical
//     presence signal the classifier and its D-09 bundle aggregate consume
//     (the descendent of the retired conflict-scan contract in
//     internal/gitconfig, re-derived here from the probe results).
//
// classify.go turns the two probe results into one honest row per option,
// following globalssh/classify.go's branch order: decide by VALUE first, by
// source second.
package globalgit
