package globalssh

import (
	"sort"

	"github.com/castocolina/gitid/internal/sshconfig"
)

// perAliasFromContent verifies IdentitiesOnly where the recipe scopes it:
// each gitid-managed alias. A Host * write would affect unrelated hosts, so
// this row is verify-only and OptionPolicy.WritableToHostStar always rejects
// it. The ONE caller is Statuses, which already holds the config content
// from its own concurrent read — there is deliberately no exported
// deps.ReadConfig()-wrapping variant (WR-10: an earlier PerAliasConformance
// wrapper was production-unused, referenced only from its own tests).
func perAliasFromContent(content []byte) (conforming int, total int, offenders []string, err error) {
	hosts, err := sshconfig.ParseManagedHosts(content)
	if err != nil {
		return 0, 0, nil, err
	}
	for alias, host := range hosts {
		total++
		if host.IdentitiesOnly {
			conforming++
			continue
		}
		offenders = append(offenders, alias)
	}
	sort.Strings(offenders)
	return conforming, total, offenders, nil
}
