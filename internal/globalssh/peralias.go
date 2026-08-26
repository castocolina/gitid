package globalssh

import (
	"sort"

	"github.com/castocolina/gitid/internal/sshconfig"
)

// PerAliasConformance verifies IdentitiesOnly where the recipe scopes it: each
// gitid-managed alias. A Host * write would affect unrelated hosts, so this row
// is verify-only and OptionPolicy.WritableToHostStar always rejects it.
func PerAliasConformance(deps Deps) (conforming int, total int, offenders []string, err error) {
	_, content, err := deps.ReadConfig()
	if err != nil {
		return 0, 0, nil, err
	}
	return perAliasFromContent(content)
}

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
