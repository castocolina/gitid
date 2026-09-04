package main

// version_cmd.go is the `gitid version [--json]` subcommand (Phase 10,
// D-11): a diagnostic surface, exactly like `gitid debug`, that shares
// internal/version.Resolve() with the `--version` flag (composeVersion /
// versionString in main.go) so all three surfaces agree byte-for-byte.

import (
	"runtime"

	"github.com/spf13/cobra"

	"github.com/castocolina/gitid/internal/version"
)

// versionSchema is the stable schema tag for `gitid version --json`'s
// output document, mirroring healthDocument's Schema-field pattern
// (cmd/gitid/health.go).
const versionSchema = "gitid.version/v1"

// versionDocument is the `gitid version --json` output shape. Field names
// are Claude's Discretion per 10-CONTEXT.md ("gitid version --json field
// names (align with internal/version struct)").
type versionDocument struct {
	Schema    string `json:"schema"`
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
	Platform  string `json:"platform"`
}

// newVersionCmd builds `gitid version [--json]`. With no flags it prints
// the identical line `gitid --version` prints (versionString()); with
// --json it prints one line of JSON built from the SAME version.Resolve()
// call, so the two surfaces can never disagree.
func newVersionCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print gitid's build-stamped version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if jsonOut {
				info := version.Resolve()
				return writeJSON(cmd.OutOrStdout(), versionDocument{
					Schema:    versionSchema,
					Version:   info.Version,
					Commit:    info.Commit,
					BuildDate: info.BuildDate,
					Platform:  runtime.GOOS + "/" + runtime.GOARCH,
				})
			}
			_, err := cmd.OutOrStdout().Write([]byte(versionString() + "\n"))
			return err
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "print version metadata as a single-line JSON document")
	return cmd
}
