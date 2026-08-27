package main

// ssh_schema_test.go carries the frozen JSON schema contract fixtures
// (WR-06 review finding): the enum vocabularies, the exact document key
// sets, and the helper that extracts an object's top-level keys for
// exact-key-set assertions. All thirteen declarations here were previously
// declared in the production ssh.go, referenced ONLY from ssh_test.go — they
// shipped in the release binary even though the `unused` linter could never
// flag them (test references count as uses). The schema contract belongs
// with the test that enforces it, not with the production command wiring.

import "encoding/json"

var (
	sshOptionRecordKeys = []string{
		"key", "current_value", "recommended_value", "risk", "scope", "state",
		"source", "source_file", "source_line", "not_applicable_reason",
		"version_note", "probe_error",
	}
	sshOptionsDocKeys = []string{"schema", "options"}
	sshStorageDocKeys = []string{"schema", "layout", "target_path", "main_config_path", "include_line_present"}
	sshApplyDocKeys   = []string{
		"schema", "dry_run", "applied", "declined", "target_path", "backups",
		"restored", "advisories", "simulation_inconclusive", "simulation_note",
		"error", "exit_code",
	}
	sshMigrateDocKeys = []string{
		"schema", "dry_run", "from_layout", "to_layout", "moved_identities",
		"moved_globals", "backups", "restored", "error", "exit_code",
	}
	sshStateEnum    = []string{"needs-action", "already-set", "differs", "not-applicable"}
	sshSourceEnum   = []string{"gitid-parsed", "outside-gitid", "system-file", "baseline", "inconclusive"}
	sshRiskEnum     = []string{"low", "medium", "high"}
	sshScopeEnum    = []string{"global", "per-alias"}
	sshNAReasonEnum = []string{"none", "platform", "version-too-old", "version-unverified", "nothing-to-verify", "probe-failed"}
	sshLayoutEnum   = []string{"include", "in-file"}
)

// jsonObjectKeys returns raw's top-level JSON object keys, for exact-key-set
// schema assertions (assertExactKeys / assertExactKeysFrom in ssh_test.go).
func jsonObjectKeys(raw []byte) ([]string, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	return keys, nil
}
