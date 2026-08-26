package globalssh

import (
	"testing"
)

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
	deps := depsForOut(cannedResolved, cannedResolved, "~/.ssh/config", []byte(config), "/etc/ssh/ssh_config", nil, nil)
	conforming, total, offenders, err := PerAliasConformance(deps)
	if err != nil {
		t.Fatalf("PerAliasConformance() error = %v", err)
	}
	if conforming != 1 || total != 2 || len(offenders) != 1 || offenders[0] != "work" {
		t.Fatalf("PerAliasConformance() = (%d, %d, %q), want (1, 2, [work])", conforming, total, offenders)
	}
}

func TestPerAliasConformanceEmpty(t *testing.T) {
	deps := depsForOut(cannedResolved, cannedResolved, "~/.ssh/config", nil, "/etc/ssh/ssh_config", nil, nil)
	conforming, total, offenders, err := PerAliasConformance(deps)
	if err != nil {
		t.Fatalf("PerAliasConformance() error = %v", err)
	}
	if conforming != 0 || total != 0 || len(offenders) != 0 {
		t.Fatalf("PerAliasConformance() = (%d, %d, %q), want (0, 0, none)", conforming, total, offenders)
	}
}
