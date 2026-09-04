// Package version resolves gitid's build-time version metadata using a
// hybrid strategy (D-09): it prefers linker-injected values (goreleaser,
// `make build`/`make build-cross` with -X) and falls back to Go's own
// module/VCS build info (`go install @version`, plain `go build`) when
// those are empty, so `gitid --version` and `gitid version [--json]` are
// truthful on every build path.
package version

import "runtime/debug"

// These three identifiers are vars rather than consts because the Go
// linker's -X flag can only overwrite a variable's initial value; a const
// is folded at compile time. The linker reports NO error when -X names a
// symbol that does not exist, so a mismatch between these names and the
// Makefile's -X github.com/castocolina/gitid/internal/version.<name> paths
// silently produces an unstamped binary. e2e/release_e2e_test.go is the
// guard that catches that mismatch.
//
// No literal default: a non-empty compiled-in default would mask the
// debug.ReadBuildInfo() fallback path D-09 needs for the go-install/plain-
// go-build cases (a non-empty zero-value would make Resolve() always take
// the ldflags branch, even when the linker never touched these vars).
var (
	version   string
	commit    string
	buildDate string
)

// Info is the resolved version metadata gitid's --version output and the
// `gitid version [--json]` subcommand both consume.
type Info struct {
	Version   string
	Commit    string
	BuildDate string
}

// Resolve returns the build's version metadata: the ldflags-stamped values
// verbatim when present, or a debug.ReadBuildInfo()-derived fallback
// otherwise.
func Resolve() Info {
	if version != "" {
		return Info{Version: version, Commit: commit, BuildDate: buildDate}
	}
	bi, ok := debug.ReadBuildInfo()
	return resolveFromBuildInfo(bi, ok)
}

// resolveFromBuildInfo is Resolve()'s pure fallback path: a table-testable
// seam that never calls debug.ReadBuildInfo() itself, so every build-path
// case (a go-install-stamped Main.Version; a plain-go-build "(devel)" clean
// tree with only vcs.revision/vcs.time; the same dirty-working-tree
// variant) is exercised against a hand-constructed fixture rather than a
// real subprocess build.
func resolveFromBuildInfo(bi *debug.BuildInfo, ok bool) Info {
	info := Info{Version: "(devel)"}
	if !ok || bi == nil {
		return info
	}
	if bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		info.Version = bi.Main.Version
	}
	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			info.Commit = s.Value
		case "vcs.time":
			info.BuildDate = s.Value
		case "vcs.modified":
			if s.Value == "true" {
				info.Version += "+dirty"
			}
		}
	}
	return info
}
