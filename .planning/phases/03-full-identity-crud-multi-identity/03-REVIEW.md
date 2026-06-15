---
phase: 03-full-identity-crud-multi-identity
reviewed: 2026-06-10T00:00:00Z
depth: standard
files_reviewed: 14
files_reviewed_list:
  - cmd/gitid/add.go
  - cmd/gitid/delete.go
  - cmd/gitid/list.go
  - cmd/gitid/main.go
  - cmd/gitid/update.go
  - internal/filewriter/block.go
  - internal/filewriter/filewriter.go
  - internal/gitconfig/fragment.go
  - internal/gitconfig/reader.go
  - internal/identity/delete.go
  - internal/identity/identity.go
  - internal/identity/loader.go
  - internal/identity/update.go
  - internal/sshconfig/reader.go
findings:
  critical: 2
  warning: 7
  info: 5
  total: 14
status: issues_fixed
---

# Phase 3: Code Review Report

> **Fixes applied 2026-06-10** — the 2 Critical findings and 3 key Warnings were fixed
> (TDD: RED regression test → GREEN fix, one atomic commit each) and merged to `main`:
> - **CR-01** `4a92045` — exact first-field principal match in `RemoveAllowedSignersLine` (no more superstring-email collisions across identities)
> - **CR-02** `db79a13` — key deletion routed through `filewriter.BackupAndRemove` (recoverable `.bak.<ts>`, atomic per file)
> - **WR-02** `159dc5d` — expand `~` in reconstructed pub path before reading in `update`
> - **WR-04** `9c13048` — `RemoveBlock` preserves foreign trailing blank line (SC-3 byte-for-byte)
> - **WR-06** `7ef012f` — reconstruction reports port `0` (unset) instead of fabricating `22`
>
> Remaining WR-01/03/05/07 and IN-01..05 were reviewed and deferred (not in the approved fix scope). Full suite (`go test -race ./...`) + `golangci-lint` green after fixes.

> **Second review round (independent reviewer, post-completion) 2026-06-10** — a fresh
> independent reviewer (superpowers:requesting-code-review) caught **3 Critical bugs +
> 1 Important this first GSD pass and the verifier both missed** — same blind spot:
> they trusted that removal paths were symmetric with their write-side counterparts.
> All confirmed against code and fixed (TDD RED→GREEN, atomic commits on `main`):
> - **#1 Critical** `41b86a7` — `RemoveBlock`/`ReplaceBlock` matched markers with
>   `TrimRight(line,"\n")`, leaving `\r` on CRLF configs → `delete` silently no-op'd
>   (while `list` showed the identity, since `ListBlocks` normalised CRLF). Now all
>   three trim `"\n\r"` for comparison while preserving foreign line-endings byte-for-byte.
> - **#2 + #3 Critical** `2386b6e` — `WriteAllowedSigners` writes a *block* but removal
>   was line-keyed by `GitEmail` → orphan empty sentinels accreted, and deleting an
>   Incomplete identity (no fragment ⇒ `GitEmail==""`) left its signer line on disk.
>   New `RemoveAllowedSignersBlock(path, name)` removes the whole block by identity
>   name (symmetric with the writer; no `GitEmail` dependency).
> - **#4 Important** `7f1bf98` — `update`'s Provider prompt was previewed but never
>   applied; since `Provider` is alias-derived at reconstruction (not independently
>   persisted), the misleading standalone prompt was removed (alias is the real lever).
>
> Lesson recorded: the GSD reviewer + verifier shared a blind spot that an *independent*
> reviewer surfaced — write/remove symmetry and CRLF tolerance are worth an explicit check.
> Full `go test -race ./...` + `golangci-lint` green after fixes.

**Reviewed:** 2026-06-10
**Depth:** standard
**Files Reviewed:** 14
**Status:** issues_found

## Summary

Phase 3 implements list/update/delete and identity reconstruction from managed
blocks across `~/.ssh/config`, `~/.gitconfig`, fragment files, and
`allowed_signers`. Safe-write discipline is largely sound: mutations route
through `filewriter` (backup → atomic temp → rename → chmod), exec is arg-slice
(no shell), and identity names are gated by `identityNameRe` + `sanitizeName`.
Private-key deletion is correctly gated behind a separate keep-by-default
confirmation.

However, two correctness defects in the shared `allowed_signers` rewrite path
can delete or corrupt a *different* identity's signing trust, and the delete
flow does not back up the private key it irreversibly removes. Several warnings
concern partial-failure recovery, substring-based matching, and reconstruction
edge cases that produce silently wrong output.

## Critical Issues

### CR-01: `RemoveAllowedSignersLine` matches by substring — deletes the wrong identity's signing line

**File:** `internal/gitconfig/reader.go:117-125`
**Issue:** The allowed_signers rewrite keeps every line *except* those where
`strings.Contains(line, identityEmail) && strings.Contains(line, namespaces="git")`.
Substring matching on the email is unsafe across identities whose emails share a
common prefix. An `allowed_signers` file containing both:

```
alice@corp.com namespaces="git" ssh-ed25519 AAAA...
alice@corp.com.attacker.example namespaces="git" ssh-ed25519 BBBB...
```

When deleting/disabling signing for `alice@corp.com`, BOTH lines are removed
because `Contains("alice@corp.com.attacker.example", "alice@corp.com")` is true.
The inverse is also broken: deleting `alice@corp` (if some other identity used a
shorter email) would nuke `alice@corp.com`. This is invoked from both
`identity.Delete` (delete.go:82) and `identity.Update` signing-off
(update.go:85), so a delete/update of one identity can silently strip another
identity's commit-verification trust. allowed_signers is shared/global state and
must be matched on the exact principal field, not by substring.

**Fix:** Parse each line into fields and compare the first whitespace-delimited
principal token exactly, rather than substring-scanning the whole line:
```go
for _, line := range strings.Split(string(existing), "\n") {
    fields := strings.Fields(line)
    // allowed_signers format: PRINCIPAL [options] keytype keydata
    if len(fields) >= 1 && fields[0] == identityEmail &&
        strings.Contains(line, `namespaces="git"`) {
        continue // remove this exact-principal line
    }
    kept = append(kept, line)
}
```

### CR-02: Delete removes the private key with no backup — irreversible loss with no recovery path

**File:** `cmd/gitid/delete.go:196-204`, `internal/identity/delete.go:90-94`
**Issue:** Every other artifact in the delete flow is backed up before removal
(SSH config, gitconfig, fragment, allowed_signers all return a `.bak.<ts>`
path). The private key is the single most valuable, least-recreatable artifact,
yet `RemoveKeyFiles` calls `os.Remove(keyPath)` / `os.Remove(pubPath)` directly
with no backup. The project's CLAUDE.md mandate is "Never write to a user's
config without a timestamped backup"; the spirit (and the explicit
keep-by-default gating) implies the irreversible path should at minimum preserve
recoverability the way the rest of the code does. A user who confirms "yes" to
the second prompt expecting reversibility consistent with the rest of the
manifest loses the key permanently. Worse: the private key deletion is NOT
routed through `filewriter` at all, bypassing the safe-write chokepoint the
phase brief explicitly requires for user-file mutations.

Additionally, `RemoveKeyFiles` removes the private key first, then the public
key; if the private remove succeeds but the public remove fails, the operation
returns an error mid-way with the private key already gone and no backup.

**Fix:** Route key removal through `filewriter.BackupAndRemove` (same atomic
backup-then-rename used for the fragment), preserving a timestamped backup, and
report the key backup paths in the delete summary:
```go
RemoveKeyFiles: func(keyPath, pubPath string) (string, string, error) {
    keyBak, kerr := filewriter.BackupAndRemove(keyPath)
    if kerr != nil { return "", "", fmt.Errorf("removing private key %s: %w", keyPath, kerr) }
    pubBak, perr := filewriter.BackupAndRemove(pubPath)
    if perr != nil { return keyBak, "", fmt.Errorf("removing public key %s: %w", pubPath, perr) }
    return keyBak, pubBak, nil
},
```
(Backups inherit `filewriter`'s 0600 mode, keeping private material non-world-readable.)

## Warnings

### WR-01: Delete is not transactional — a mid-sequence failure leaves the identity half-deleted

**File:** `internal/identity/delete.go:47-96`
**Issue:** Delete performs five sequential effects (SSH write, gitconfig write,
fragment remove, allowed_signers remove, key remove). If
`RemoveAllowedSigners` (line 82) or `RemoveKeyFiles` (line 91) fails, the SSH
block and includeIf block are already gone but the fragment/allowed_signers/key
remain. The function returns an error, but the user is left with a partially
deleted identity and no rollback. The first two backups are returned in `res`,
but `runIdentityDelete` discards `res` on error (`delete.go:136-139` returns
before printing backup paths), so the user is not even told where the backups
are. At minimum, surface the partial-progress backups on error.

**Fix:** On error, still print the backup paths already collected in `res`, or
collect backups and only print failures, so the user can recover the partially
modified files.

### WR-02: Update silently writes the WRONG signing key path after a structural change

**File:** `internal/identity/update.go:57,75,91`
**Issue:** Update re-renders the SSH host block and fragment using
`edited.KeyPath` / `edited.PubPath`, but the update prompts (update.go:120-152)
never let the user edit the key path — `edited` is a copy of `existing`, so
`KeyPath`/`PubPath` carry whatever reconstruction produced. If reconstruction
left `KeyPath` empty, `runIdentityUpdate` backfills it (update.go:100-102) to
`id_ed25519_<name>`, but reconstruction in `loader.go:44` sets
`acct.KeyPath = ssh.IdentityFile` *verbatim from the SSH config*, which may be a
tilde path (`~/.ssh/...`) that `os.ReadFile` cannot expand. `readPubLine`
(update.go:110) then fails on a `~`-prefixed `PubPath`, aborting the entire
update with "reading public key for signing" — after the SSH and gitconfig
blocks were already rewritten (lines 58, 63). Net effect: signing-on updates can
half-apply then hard-fail on any identity whose IdentityFile uses `~`.

**Fix:** Expand `~` in `KeyPath`/`PubPath` after reconstruction (resolve against
`os.UserHomeDir()`), or read the pub via a path that tolerates tilde, before any
write occurs — and validate the pub is readable up front so the write sequence
does not begin if it will fail.

### WR-03: `RemoveAllowedSignersLine` writes a backup on every call even when nothing matched

**File:** `internal/gitconfig/reader.go:116-125`
**Issue:** The doc comment claims "Idempotent when no matching line exists," but
the function unconditionally calls `filewriter.Write` (line 125) regardless of
whether any line was removed. Every signing-off update or delete therefore
creates a new `allowed_signers.bak.<ts>` even when the file is unchanged,
accumulating backup clutter and rewriting the file (which can normalize trailing
newlines — `strings.Join(kept, "\n")` drops a trailing blank the original may
have had). True idempotency requires detecting "no change" and skipping the
write.

**Fix:**
```go
result := strings.Join(kept, "\n")
if result == string(existing) {
    return "", nil // nothing removed — no rewrite, no backup
}
return filewriter.Write(path, []byte(result), 0o600)
```

### WR-04: `RemoveBlock` blank-line consumption can delete a foreign blank-separated line's separator

**File:** `internal/filewriter/block.go:84-89`
**Issue:** After removing a managed block, the code consumes "one trailing blank
line." But the blank line following a managed `END` may be a deliberate
separator the user placed between the managed block and their own following
content. Removing it silently mutates foreign formatting (the phase brief
requires foreign content preserved "byte-for-byte"). Worse, when the managed
block is immediately followed by a foreign `Host` stanza separated by exactly
one blank line, that separator is consumed and the two stanzas become visually
fused. This is a round-trip fidelity regression rather than data loss, but it
violates the stated byte-for-byte guarantee.

**Fix:** Only consume the trailing blank line when it was demonstrably
introduced by gitid (e.g., when the line *before* BEGIN was also blank, i.e. the
block is blank-padded on both sides), or drop the consumption entirely and let
the writer normalize on the next `ReplaceBlock`.

### WR-05: Reconstruction marks fragment-only or includeIf-only identities incompletely / inconsistently

**File:** `internal/identity/loader.go:64-73`
**Issue:** The "fragment side" check only runs `if acct.FragmentPath != ""`.
When the includeIf block is missing (`gc.FragmentPath == ""`), `FragmentPath`
stays empty, so the fragment is never read and "fragment-file" is never added to
`missing` — even if a fragment file physically exists on disk. The
`Incomplete` string then reports `gitconfig-includeif-block` but omits the
fragment status entirely, giving the user an inconsistent picture. An identity
present only as a fragment file (no SSH, no includeIf) is invisible to
reconstruction because `nameUnion` (loader.go:83) only unions SSH + gitconfig
block names, never fragment filenames.

**Fix:** Document explicitly that fragment-only identities are out of scope, or
include fragment-directory scanning in `nameUnion`. At minimum, when
`FragmentPath` is empty, derive the default fragment path from the name so the
existence check still runs and `missing` is reported consistently.

### WR-06: `parseHostBlockBody` defaults port to 22 but reconstruction/list treats 0 as "absent"

**File:** `internal/sshconfig/reader.go:67`, `cmd/gitid/list.go:98`
**Issue:** `parseHostBlockBody` defaults `port` to 22 when no `Port` directive is
present. But the create flow defaults port to 443 (add.go:281), and the alt-ssh
endpoints (`ssh.github.com`, `altssh.gitlab.com`) require 443. A gitid-managed
block that genuinely omits `Port` will reconstruct as port 22 — wrong for these
hosts — and `list.go:98` prints `port: 22`, misleading the user about the actual
resolved port. Meanwhile `list.go:98` only prints the port when `!= 0`, but
reconstruction never yields 0 (it forces 22), so the "absent" branch is dead.
The default-22 vs default-443 mismatch is a latent correctness/UX bug.

**Fix:** Have `parseHostBlockBody` return port 0 (or a sentinel) when no `Port`
directive is present, and let the display/use layer apply the correct
provider-aware default, rather than silently asserting 22.

### WR-07: `promptPort` silently swallows invalid input and falls back to default

**File:** `cmd/gitid/add.go:504-510`
**Issue:** `promptPort` returns the default on any non-numeric or non-positive
input with no warning. A user who fat-fingers `44e` intending `443` silently
gets the default (443 for create, or `existing.Port` for update) with no
indication their input was ignored. For a tool that writes connectivity-critical
config, silently discarding input is a footgun. It also accepts ports > 65535
(no upper bound), which `strconv.Atoi` happily parses.

**Fix:** On parse failure, re-prompt or at least emit a warning; bound the value
to 1..65535.

## Info

### IN-01: Inconsistent error-context prefixes between add/update/delete and the identity package

**File:** `internal/identity/delete.go:138` vs `cmd/gitid/delete.go:138`
**Issue:** `identity.Delete` errors are prefixed `identity:` and then re-wrapped
by `runIdentityDelete` with `identity delete:`, producing doubled prefixes like
`identity delete: identity: removing ssh block: ...`. Cosmetic but noisy.
**Fix:** Drop the inner `identity:` prefix in the identity package or the outer
one in the command layer.

### IN-02: `list.go` provider fallback to raw Hostname is misleading

**File:** `cmd/gitid/list.go:90-97`
**Issue:** When provider can't be derived from the alias, the code prints the
full `Hostname` (e.g. `ssh.github.com`) under the `provider:` label. Labeling a
hostname as a provider is confusing.
**Fix:** Either label it `host:` in the fallback, or extract the second-level
domain like `update.go:128-131` already does.

### IN-03: Duplicated path-backfill block across update.go and delete.go

**File:** `cmd/gitid/update.go:88-105`, `cmd/gitid/delete.go:86-103`
**Issue:** The "fill gitid-managed paths from HOME" block is copy-pasted
verbatim (and references a no-longer-shown `rotate.go gatherRotateAccount`). Drift
risk if defaults change in one place.
**Fix:** Extract a shared `fillManagedPaths(acct *identity.Account, home, name)` helper.

### IN-04: `ReadFragment` swallows all git errors as "Missing"

**File:** `internal/gitconfig/reader.go:77-81`
**Issue:** Any error from `git config --list` (corrupt fragment, permission
denied, git not installed) is collapsed to `FragmentInfo{Missing: true}` with a
nil error. A genuinely broken fragment is then indistinguishable from an absent
one, and list/update will show the identity as missing its fragment rather than
surfacing the real problem.
**Fix:** Distinguish "file absent" (stat IsNotExist) from "git failed" and at
least propagate or log the latter.

### IN-05: `defaultHostname` fallback `provider + ".com"` produces nonsense for arbitrary providers

**File:** `cmd/gitid/add.go:429-438`
**Issue:** For any provider other than github/gitlab, the default hostname
becomes `<provider>.com`, which is almost always wrong (e.g.
`bitbucket` → `bitbucket.com`, not `bitbucket.org`). Low impact since it's an
editable default, but it can mislead.
**Fix:** Leave the hostname blank (force explicit entry) for unknown providers.

---

_Reviewed: 2026-06-10_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
