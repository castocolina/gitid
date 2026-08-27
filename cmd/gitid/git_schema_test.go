package main

// git_schema_test.go carries the frozen JSON schema contract fixtures for the
// git noun (plan 07-05 Task 2), mirroring ssh_schema_test.go: the exact
// document key sets and the state enum vocabulary. All declarations here were
// referenced ONLY from tests and must never ship in the release binary — the
// schema contract belongs with the test that enforces it.

var (
	gitOptionRecordKeys = []string{
		"key", "token", "current_value", "provenance", "recommended_value",
		"state", "probe_error",
	}
	gitOptionsDocKeys = []string{"schema", "options"}
	gitApplyDocKeys   = []string{
		"schema", "dry_run", "applied", "declined", "target_path", "backups",
		"restored", "advisories", "error", "exit_code",
	}
	gitFallbackDocKeys    = []string{"schema", "name", "email", "name_status", "email_status"}
	gitFallbackSetDocKeys = []string{
		"schema", "dry_run", "set_name", "set_email", "cleared_name",
		"cleared_email", "target_path", "backups", "restored", "advisories",
		"error", "exit_code",
	}

	// gitStateEnum is the exact JSON state taxonomy the render layer uses:
	// gitRowStateName's five outputs — the four GlobalGitOptionState values
	// plus the probe-error word. Pinned against the render layer by
	// TestGitJSONStateEnumsAreRenderLayerTaxonomy.
	gitStateEnum = []string{"needs-action", "already-set", "differs", "not-applicable", "probe-error"}
)
