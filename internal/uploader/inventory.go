// Package uploader inventory helpers answer what is already registered. They
// never mutate except through DeleteKey; inventory errors are non-gating D-15
// degradation signals for callers.
package uploader

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ExistingKey is one provider inventory record.
type ExistingKey struct {
	ID           string
	Title        string
	Key          string
	Registration Registration
}

type providerKey struct {
	ID    json.Number `json:"id"`
	Title string      `json:"title"`
	Key   string      `json:"key"`
}

// Inventory reads all key registrations for tool or returns a non-gating error.
func Inventory(tool Tool, toolPath string, deps Deps) ([]ExistingKey, error) {
	switch tool {
	case ToolGH:
		auth, err := inventoryFor(tool, toolPath, deps, []string{"api", "user/keys"}, RegistrationAuthentication)
		if err != nil {
			return nil, err
		}
		signing, err := inventoryFor(tool, toolPath, deps, []string{"api", "user/ssh_signing_keys"}, RegistrationSigning)
		if err != nil {
			return nil, err
		}
		return append(auth, signing...), nil
	case ToolGLab:
		return inventoryFor(tool, toolPath, deps, []string{"ssh-key", "list", "-F", "json"}, RegistrationCombined)
	default:
		return nil, fmt.Errorf("uploader: unknown tool %d", tool)
	}
}

func inventoryFor(tool Tool, toolPath string, deps Deps, args []string, reg Registration) ([]ExistingKey, error) {
	out, code, err := deps.RunCmd(toolPath, args...)
	if err != nil || code != 0 {
		return nil, fmt.Errorf("uploader: %s %s failed: %w", toolName(tool), strings.Join(args, " "), wrapRunErr(err))
	}
	var entries []providerKey
	if err := json.Unmarshal([]byte(out), &entries); err != nil {
		return nil, fmt.Errorf("uploader: parsing %s: %w", strings.Join(args, " "), err)
	}
	keys := make([]ExistingKey, 0, len(entries))
	for _, entry := range entries {
		keys = append(keys, ExistingKey{ID: entry.ID.String(), Title: entry.Title, Key: entry.Key, Registration: reg})
	}
	return keys, nil
}

// NormalizeKeyBlob drops the comment because the key, not its machine-scoped
// title, is D-15's dedupe truth.
func NormalizeKeyBlob(line string) string {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return ""
	}
	return fields[0] + " " + fields[1]
}

// HasRegistration reports whether pubLine is registered for reg.
func HasRegistration(existing []ExistingKey, pubLine string, reg Registration) bool {
	blob := NormalizeKeyBlob(pubLine)
	if blob == "" {
		return false
	}
	for _, key := range existing {
		if key.Registration == reg && NormalizeKeyBlob(key.Key) == blob {
			return true
		}
	}
	return false
}

// MissingRegistrations returns the D-16 missing-type diff for pubLine.
func MissingRegistrations(existing []ExistingKey, pubLine string, want []Registration) []Registration {
	var missing []Registration
	for _, reg := range want {
		if !HasRegistration(existing, pubLine, reg) {
			missing = append(missing, reg)
		}
	}
	return missing
}

// DeleteKey removes one numeric provider resource ID. A title or path must never
// be accepted where an ID is expected.
func DeleteKey(tool Tool, toolPath, id string, deps Deps) (string, error) {
	if !isAllDigits(id) {
		return "", fmt.Errorf("uploader: refusing invalid provider key ID %q", id)
	}
	args, err := deleteArgs(tool, id)
	if err != nil {
		return "", err
	}
	out, code, runErr := deps.RunCmd(toolPath, args...)
	if runErr != nil || code != 0 {
		return trimOutput(out), fmt.Errorf("uploader: %s delete failed (exit %d): %w", toolName(tool), code, wrapRunErr(runErr))
	}
	return trimOutput(out), nil
}

// DeleteRecordedKey is the preferred delete entry point because it carries the
// inventory record's ID and title together; DeleteKey remains the low-level primitive.
func DeleteRecordedKey(tool Tool, toolPath string, rec ExistingKey, deps Deps) (string, error) {
	return DeleteKey(tool, toolPath, rec.ID, deps)
}

// DeleteCommandPreview renders the exact argv DeleteKey runs.
func DeleteCommandPreview(tool Tool, toolPath, id string) string {
	args, err := deleteArgs(tool, id)
	if err != nil {
		return fmt.Sprintf("(preview unavailable: %s)", err)
	}
	return strings.Join(append([]string{toolPath}, args...), " ")
}

func deleteArgs(tool Tool, id string) ([]string, error) {
	switch tool {
	case ToolGH:
		return []string{"ssh-key", "delete", id, "--yes"}, nil
	case ToolGLab:
		return []string{"ssh-key", "delete", id}, nil
	default:
		return nil, fmt.Errorf("uploader: unknown tool %d", tool)
	}
}

// FindByTitle finds an exact, machine-scoped title match.
func FindByTitle(existing []ExistingKey, title string) (ExistingKey, bool) {
	for _, key := range existing {
		if key.Title == title {
			return key, true
		}
	}
	return ExistingKey{}, false
}
