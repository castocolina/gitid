package sshconfig

import (
	"fmt"
	"strconv"
	"strings"

	ssh_config "github.com/kevinburke/ssh_config"

	"github.com/castocolina/gitid/internal/filewriter"
)

// SSHHostInfo holds the fields extracted from a gitid-managed SSH Host block.
type SSHHostInfo struct {
	Alias          string
	Hostname       string
	Port           int // 0 ("unset") when the block has no explicit Port directive (WR-06)
	IdentityFile   string
	IdentitiesOnly bool
}

// ParseManagedHosts parses content (bytes of ~/.ssh/config), extracts all
// gitid-managed blocks via filewriter.ListBlocks, and for each block parses the
// SSH directives into SSHHostInfo. Keyed by identity name (D-01). Blocks that
// fail to parse return a zero-value SSHHostInfo (reconstruction incomplete
// marker, D-02). The _global block is skipped.
func ParseManagedHosts(content []byte) (map[string]SSHHostInfo, error) {
	blocks := filewriter.ListBlocks(content)
	result := make(map[string]SSHHostInfo, len(blocks))
	for _, b := range blocks {
		if b.Name == "_global" {
			continue // skip the macOS Host * global block
		}
		info, err := parseHostBlockBody(b.Body)
		if err != nil {
			// Best-effort (D-02): return zero-value so caller marks as incomplete.
			result[b.Name] = SSHHostInfo{}
			continue
		}
		result[b.Name] = info
	}
	return result, nil
}

// ParseAllHostIdentityFiles parses the full ~/.ssh/config content (all Host
// blocks, gitid-managed AND hand-written) and returns every unique IdentityFile
// path found. This is the D-12 data source for the unused-key cross-reference
// in CheckOrphans — a key referenced by any Host block (not just managed ones)
// must not be flagged as unused.
func ParseAllHostIdentityFiles(content []byte) []string {
	cfg, err := ssh_config.Decode(strings.NewReader(string(content)))
	if err != nil {
		return nil
	}
	seen := make(map[string]bool)
	var result []string
	for _, host := range cfg.Hosts {
		// Skip the implicit Host * (Pitfall A guard).
		if len(host.Patterns) == 1 && host.Patterns[0].String() == "*" {
			continue
		}
		for _, p := range host.Patterns {
			alias := p.String()
			idFile, getErr := cfg.Get(alias, "IdentityFile")
			if getErr != nil || idFile == "" {
				continue
			}
			if !seen[idFile] {
				seen[idFile] = true
				result = append(result, idFile)
			}
		}
	}
	return result
}

// parseHostBlockBody parses a single SSH host block body string into an
// SSHHostInfo. It skips the implicit Host * inserted by the kevinburke parser
// (Pitfall A guard: len(host.Patterns)==1 && host.Patterns[0].String()=="*").
func parseHostBlockBody(body string) (SSHHostInfo, error) {
	cfg, err := ssh_config.Decode(strings.NewReader(body))
	if err != nil {
		return SSHHostInfo{}, err
	}
	if len(cfg.Hosts) == 0 {
		return SSHHostInfo{}, fmt.Errorf("no Host block found")
	}
	for _, host := range cfg.Hosts {
		// Skip the implicit Host * inserted by newConfig() (Pitfall A).
		if len(host.Patterns) == 1 && host.Patterns[0].String() == "*" {
			continue
		}
		if len(host.Patterns) == 0 {
			continue
		}
		alias := host.Patterns[0].String()
		hostname, _ := cfg.Get(alias, "Hostname")
		portStr, _ := cfg.Get(alias, "Port")
		// Port 0 means "unset": when the block has no explicit Port directive we
		// must NOT fabricate 22, because gitid alt-ssh endpoints use 443 (WR-06).
		// The display/use layer applies the real provider-aware default and treats
		// 0 as absent (list.go only prints port when != 0).
		port := 0
		if n, atoiErr := strconv.Atoi(portStr); atoiErr == nil {
			port = n
		}
		identityFile, _ := cfg.Get(alias, "IdentityFile")
		identitiesOnly, _ := cfg.Get(alias, "IdentitiesOnly")
		return SSHHostInfo{
			Alias:          alias,
			Hostname:       hostname,
			Port:           port,
			IdentityFile:   identityFile,
			IdentitiesOnly: strings.EqualFold(identitiesOnly, "yes"),
		}, nil
	}
	return SSHHostInfo{}, fmt.Errorf("no non-wildcard Host block found")
}
