package globalssh

import (
	"testing"
)

// WR-10: the exported PerAliasConformance wrapper was production-unused (a
// thin, redundant deps.ReadConfig() indirection over perAliasFromContent —
// the function Statuses actually calls) and has been removed. These tests
// now exercise perAliasFromContent directly — the SAME function under test,
// with the dead indirection gone.

func TestPerAliasConformance(t *testing.T) {
	const config = `# BEGIN gitid managed: personal
Host personal.github.com
  IdentitiesOnly yes
# END gitid managed: personal
# BEGIN gitid managed: work
Host work.github.com
  IdentitiesOnly no
# END gitid managed: work
`
	conforming, total, offenders, err := perAliasFromContent([]byte(config))
	if err != nil {
		t.Fatalf("perAliasFromContent() error = %v", err)
	}
	if conforming != 1 || total != 2 || len(offenders) != 1 || offenders[0] != "work" {
		t.Fatalf("perAliasFromContent() = (%d, %d, %q), want (1, 2, [work])", conforming, total, offenders)
	}
}

func TestPerAliasConformanceEmpty(t *testing.T) {
	conforming, total, offenders, err := perAliasFromContent(nil)
	if err != nil {
		t.Fatalf("perAliasFromContent() error = %v", err)
	}
	if conforming != 0 || total != 0 || len(offenders) != 0 {
		t.Fatalf("perAliasFromContent() = (%d, %d, %q), want (0, 0, none)", conforming, total, offenders)
	}
}
