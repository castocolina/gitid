// Package uploader inventory helpers answer what is already registered. They
// never mutate except through DeleteKey; inventory errors are non-gating D-15
// degradation signals for callers.
package uploader

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
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
		// --paginate: gh api's REST default page size is 30, and every
		// D-04/D-15/D-16/D-17 decision reads this list as if it were
		// complete. Without it, an account with more than 30 keys reports
		// registrations as missing that already exist, the D-17
		// confirmation never converges, and D-04's delete offer reports "no
		// matching old key found" for a key that is right there (WR-01).
		auth, err := inventoryFor(tool, toolPath, deps, []string{"api", "--paginate", "user/keys"}, RegistrationAuthentication)
		if err != nil {
			return nil, err
		}
		signing, err := inventoryFor(tool, toolPath, deps, []string{"api", "--paginate", "user/ssh_signing_keys"}, RegistrationSigning)
		if err != nil {
			return nil, err
		}
		return append(auth, signing...), nil
	case ToolGLab:
		// WR-10: `glab ssh-key list` has no gh-style `--paginate` all-pages
		// flag; verified against a real `glab ssh-key list --help` (v1.114.0):
		// it exposes `-p/--page` (default 1) and `-P/--per-page` (default
		// 30), the same REST-style page params gh's default (unpaginated)
		// call would use. Iterate --page until a page returns fewer than
		// glabPerPage records — the same "every D-04/D-15/D-16 decision
		// reads this list as exhaustive" reasoning WR-01 already established
		// for gh's --paginate applies here: a truncated glab inventory makes
		// MissingRegistrations re-upload a key the account already has, and
		// the resulting "already taken" response gets misclassified as
		// FailureCrossAccountConflict — a false claim the key belongs to
		// another account.
		return glabInventory(tool, toolPath, deps)
	default:
		return nil, fmt.Errorf("uploader: unknown tool %d", tool)
	}
}

// glabPerPage is the page size WR-10's fix requests from `glab ssh-key
// list`. It matches glab's own default (`-P/--per-page`, default 30) — an
// explicit value rather than relying on the default so a future glab
// release changing its default cannot silently change gitid's stop
// condition (a page shorter than glabPerPage means "no more records").
const glabPerPage = 30

// glabMaxPages (WR-07, review iteration 3) bounds glabInventory's loop —
// 1200 keys, far beyond any real account. The only termination condition
// otherwise is a short page; any endpoint that returns a full page
// regardless of --page (an older glab that silently ignores the unknown
// flag, a corporate proxy, a glab alias, a future flag rename) would spin
// forever inside a tea.Cmd goroutine with no cancellation, hanging the TUI
// with no visible cause. Hitting the cap degrades honestly (a non-gating
// D-15 inventory error, same as any other Inventory failure) instead of
// spinning.
const glabMaxPages = 40

// glabInventory reads glab's ssh-key list one page at a time, via -p/--page
// and -P/--per-page (verified against a real `glab ssh-key list --help`,
// v1.114.0 — glab has no `gh api --paginate`-style all-pages flag), and
// concatenates every page. It stops as soon as a page returns fewer than
// glabPerPage records — the REST-pagination convention every provider here
// follows — rather than looping until an empty page, so an account with
// exactly N*glabPerPage keys costs one extra, cheap, correctly-empty call
// instead of silently under-counting by relying on an off-by-one guess.
func glabInventory(tool Tool, toolPath string, deps Deps) ([]ExistingKey, error) {
	var all []ExistingKey
	for page := 1; page <= glabMaxPages; page++ {
		args := []string{"ssh-key", "list", "-F", "json", "--per-page", strconv.Itoa(glabPerPage), "--page", strconv.Itoa(page)}
		keys, err := inventoryFor(tool, toolPath, deps, args, RegistrationCombined)
		if err != nil {
			return nil, err
		}
		all = append(all, keys...)
		if len(keys) < glabPerPage {
			return all, nil
		}
	}
	return nil, fmt.Errorf("uploader: glab ssh-key list did not terminate after %d pages", glabMaxPages)
}

func inventoryFor(tool Tool, toolPath string, deps Deps, args []string, reg Registration) ([]ExistingKey, error) {
	out, code, err := deps.RunCmd(toolPath, args...)
	if err != nil || code != 0 {
		return nil, fmt.Errorf("uploader: %s %s failed: %w", toolName(tool), strings.Join(args, " "), wrapRunErr(err))
	}
	entries, perr := decodeProviderKeyPages(out)
	if perr != nil {
		return nil, fmt.Errorf("uploader: parsing %s: %w", strings.Join(args, " "), perr)
	}
	keys := make([]ExistingKey, 0, len(entries))
	for _, entry := range entries {
		keys = append(keys, ExistingKey{ID: entry.ID.String(), Title: entry.Title, Key: entry.Key, Registration: reg})
	}
	return keys, nil
}

// decodeProviderKeyPages parses out as one or more concatenated top-level
// JSON array documents — the shape `gh api --paginate` produces for a
// paginated REST list: each page's JSON array is written back-to-back with
// no separator, which json.Unmarshal cannot parse in a single call.
// json.Decoder decodes sequential top-level JSON values from a stream, so
// looping Decode until io.EOF reassembles every page into one flat list. A
// single-page response (glab's "-F json", or a gh --paginate response that
// happened to fit in one page) decodes in exactly one iteration.
func decodeProviderKeyPages(out string) ([]providerKey, error) {
	dec := json.NewDecoder(strings.NewReader(out))
	var all []providerKey
	for {
		var page []providerKey
		if err := dec.Decode(&page); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		all = append(all, page...)
	}
	return all, nil
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

// DeleteKey removes one numeric provider resource ID from reg's namespace. A
// title or path must never be accepted where an ID is expected. reg selects
// the delete endpoint: GitHub's authentication and signing key registrations
// are separate REST resources (/user/keys/{id} vs /user/ssh_signing_keys/{id})
// with independent, freely-colliding ID spaces, so the caller MUST know which
// namespace id belongs to — an id that is valid for one namespace can name a
// completely unrelated resource in the other.
func DeleteKey(tool Tool, toolPath string, reg Registration, id string, deps Deps) (string, error) {
	if !isAllDigits(id) {
		return "", fmt.Errorf("uploader: refusing invalid provider key ID %q", id)
	}
	args, err := deleteArgs(tool, reg, id)
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
// inventory record's ID, title, AND registration namespace together;
// DeleteKey remains the low-level primitive. Threading rec.Registration
// through is load-bearing (see DeleteKey's doc comment) — deleting by ID
// alone risks addressing the wrong provider resource.
func DeleteRecordedKey(tool Tool, toolPath string, rec ExistingKey, deps Deps) (string, error) {
	return DeleteKey(tool, toolPath, rec.Registration, rec.ID, deps)
}

// DeleteCommandPreview renders the exact argv DeleteKey runs for reg.
func DeleteCommandPreview(tool Tool, reg Registration, toolPath, id string) string {
	args, err := deleteArgs(tool, reg, id)
	if err != nil {
		return fmt.Sprintf("(preview unavailable: %s)", err)
	}
	return previewLine(toolPath, args)
}

// deleteArgs renders the delete argv scoped to reg's namespace. See DeleteKey's
// doc comment for why a single fixed `gh ssh-key delete <id>` call is wrong.
func deleteArgs(tool Tool, reg Registration, id string) ([]string, error) {
	switch tool {
	case ToolGH:
		switch reg {
		case RegistrationAuthentication:
			return []string{"api", "-X", "DELETE", "user/keys/" + id}, nil
		case RegistrationSigning:
			return []string{"api", "-X", "DELETE", "user/ssh_signing_keys/" + id}, nil
		default:
			return nil, fmt.Errorf("uploader: unsupported delete registration %d for %s", reg, toolName(tool))
		}
	case ToolGLab:
		return []string{"ssh-key", "delete", id}, nil
	default:
		return nil, fmt.Errorf("uploader: unknown tool %d", tool)
	}
}

// FindByTitle finds an exact, machine-scoped title match. It is a general
// lookup helper — for a destructive delete decision, prefer OldKeyCandidates,
// which additionally excludes the key that was just (re-)registered and
// refuses to guess when a title is shared by more than one distinct old key.
func FindByTitle(existing []ExistingKey, title string) (ExistingKey, bool) {
	for _, key := range existing {
		if key.Title == title {
			return key, true
		}
	}
	return ExistingKey{}, false
}

// OldKeyCandidates returns every ExistingKey record that unambiguously
// represents the OLD key on this machine for a D-04 rotate delete offer:
// title-matching, EXCLUDING any record whose key blob equals currentBlob —
// the key that was just (re-)registered under the identical title moments
// earlier by the SAME rotate ceremony (planUpload runs before this offer
// resolves, so the new key is already inventoried under the same title).
// GitHub records one entry PER registration type (authentication + signing)
// for the same physical key, so more than one candidate is expected and
// correct — but only when every surviving candidate shares exactly one
// distinct key blob. If candidates span more than one distinct blob (a prior
// rotation left its own title collision, or the inventory otherwise cannot
// disambiguate), the old key cannot be identified safely and
// OldKeyCandidates returns nil: the caller must refuse rather than guess
// which record is safe to delete.
//
// WR-01: the exclusion above only protects the destructive decision when
// currentBlob and every candidate's blob can actually be COMPARED. A blank
// currentBlob (the account's .pub read succeeded but was empty/truncated)
// or a blank candidate blob (a provider/CLI field rename that omits "key")
// makes NormalizeKeyBlob return "", which used to either skip the exclusion
// entirely or let a single all-blank record pass the "exactly one distinct
// blob" check — both silently re-open the exact CR-01 defect this function
// exists to close. "I could not read the key material" must never be
// treated as "they do not match": both cases now refuse (return nil).
func OldKeyCandidates(existing []ExistingKey, title, currentBlob string) []ExistingKey {
	if currentBlob == "" {
		return nil // cannot prove which record is the NEW key — refuse
	}
	var candidates []ExistingKey
	blobs := make(map[string]bool)
	for _, rec := range existing {
		if rec.Title != title {
			continue
		}
		blob := NormalizeKeyBlob(rec.Key)
		if blob == "" {
			return nil // a record we cannot compare makes the whole set unsafe
		}
		if blob == currentBlob {
			continue // this is the key we just registered — never offer it
		}
		candidates = append(candidates, rec)
		blobs[blob] = true
	}
	if len(candidates) == 0 || len(blobs) != 1 {
		return nil
	}
	return candidates
}
