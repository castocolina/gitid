package sshconfig

import (
	"bytes"
	"fmt"

	"github.com/kevinburke/ssh_config"
)

// Parse decodes SSH config bytes into a *ssh_config.Config using the
// comment-preserving kevinburke/ssh_config parser. It is the round-trip seam:
// callers Parse, inspect or re-render via cfg.String(), and Parse again with a
// stable result (CONTEXT D-12/D-13).
//
// content is the full config file body; an empty slice parses to an empty
// (but valid) config so callers do not special-case a missing file.
func Parse(content []byte) (*ssh_config.Config, error) {
	cfg, err := ssh_config.Decode(bytes.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("parsing ssh config: %w", err)
	}
	return cfg, nil
}
