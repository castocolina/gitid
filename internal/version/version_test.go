package version

import (
	"runtime/debug"
	"testing"
)

// TestResolveFromBuildInfo_GoInstallStampedVersion covers the `go install
// github.com/.../gitid@v1.0.0` case: bi.Main.Version is populated and used
// verbatim.
func TestResolveFromBuildInfo_GoInstallStampedVersion(t *testing.T) {
	bi := &debug.BuildInfo{
		Main: debug.Module{Version: "v1.0.0"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "abc1234"},
			{Key: "vcs.time", Value: "2026-07-08T00:00:00Z"},
			{Key: "vcs.modified", Value: "false"},
		},
	}
	got := resolveFromBuildInfo(bi, true)
	want := Info{Version: "v1.0.0", Commit: "abc1234", BuildDate: "2026-07-08T00:00:00Z"}
	if got != want {
		t.Fatalf("resolveFromBuildInfo() = %+v, want %+v", got, want)
	}
}

// TestResolveFromBuildInfo_PlainGoBuildCleanDevVersion covers a plain `go
// build`, clean working tree: Main.Version is the unpopulated "(devel)"
// sentinel, so Info.Version stays "(devel)" and only vcs.revision/vcs.time
// are carried through.
func TestResolveFromBuildInfo_PlainGoBuildCleanDevVersion(t *testing.T) {
	bi := &debug.BuildInfo{
		Main: debug.Module{Version: "(devel)"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "def5678"},
			{Key: "vcs.time", Value: "2026-07-09T00:00:00Z"},
			{Key: "vcs.modified", Value: "false"},
		},
	}
	got := resolveFromBuildInfo(bi, true)
	want := Info{Version: "(devel)", Commit: "def5678", BuildDate: "2026-07-09T00:00:00Z"}
	if got != want {
		t.Fatalf("resolveFromBuildInfo() = %+v, want %+v", got, want)
	}
}

// TestResolveFromBuildInfo_PlainGoBuildDirtyAppendsSuffix covers the same
// plain-`go build` case with an uncommitted working tree: vcs.modified=true
// must append "+dirty" to Info.Version.
func TestResolveFromBuildInfo_PlainGoBuildDirtyAppendsSuffix(t *testing.T) {
	bi := &debug.BuildInfo{
		Main: debug.Module{Version: "(devel)"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "ghi9012"},
			{Key: "vcs.time", Value: "2026-07-10T00:00:00Z"},
			{Key: "vcs.modified", Value: "true"},
		},
	}
	got := resolveFromBuildInfo(bi, true)
	want := Info{Version: "(devel)+dirty", Commit: "ghi9012", BuildDate: "2026-07-10T00:00:00Z"}
	if got != want {
		t.Fatalf("resolveFromBuildInfo() = %+v, want %+v", got, want)
	}
}

// TestResolveFromBuildInfo_NotOKReturnsDevelZeroValue covers
// debug.ReadBuildInfo()'s own failure mode (ok == false, e.g. a binary built
// without module mode) — resolveFromBuildInfo must not panic on a nil
// *debug.BuildInfo and must return the "(devel)" sentinel with no commit/date.
func TestResolveFromBuildInfo_NotOKReturnsDevelZeroValue(t *testing.T) {
	got := resolveFromBuildInfo(nil, false)
	want := Info{Version: "(devel)"}
	if got != want {
		t.Fatalf("resolveFromBuildInfo(nil, false) = %+v, want %+v", got, want)
	}
}

// TestResolve_PrefersLdflagsStampWhenPresent asserts Resolve()'s first
// branch: when the package-level ldflags vars are non-empty, they are
// returned verbatim with no debug.ReadBuildInfo() involvement.
func TestResolve_PrefersLdflagsStampWhenPresent(t *testing.T) {
	origVersion, origCommit, origBuildDate := version, commit, buildDate
	t.Cleanup(func() { version, commit, buildDate = origVersion, origCommit, origBuildDate })
	version, commit, buildDate = "1.2.3", "abc1234", "2026-07-08"

	got := Resolve()
	want := Info{Version: "1.2.3", Commit: "abc1234", BuildDate: "2026-07-08"}
	if got != want {
		t.Fatalf("Resolve() = %+v, want %+v", got, want)
	}
}

// TestResolve_FallsBackToBuildInfoNeverPanicsAndNonEmptyVersion is the
// smoke-level test for Resolve()'s real debug.ReadBuildInfo() path: it
// exercises the REAL build info of the running go test binary. Per the
// task's <behavior>, it asserts only that Resolve() never panics and
// returns a non-empty Info.Version — never the exact literal, which is
// non-deterministic across machines/commits.
func TestResolve_FallsBackToBuildInfoNeverPanicsAndNonEmptyVersion(t *testing.T) {
	origVersion, origCommit, origBuildDate := version, commit, buildDate
	t.Cleanup(func() { version, commit, buildDate = origVersion, origCommit, origBuildDate })
	version, commit, buildDate = "", "", ""

	got := Resolve()
	if got.Version == "" {
		t.Fatal("Resolve() with empty ldflags vars must return a non-empty Version from debug.ReadBuildInfo()")
	}
}
