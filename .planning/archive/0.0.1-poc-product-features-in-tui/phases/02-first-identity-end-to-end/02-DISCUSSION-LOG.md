# Phase 2: First Identity End-to-End - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-06-09
**Phase:** 2-first-identity-end-to-end
**Areas discussed:** New-key test gate, Phase 2 user entry point, Key naming + passphrase + algorithm, Alias + match defaults / create modes

---

## New-key test gate

### Q1 — How to classify the pre-write `ssh -i ... -T git@host` result for a new key

| Option | Description | Selected |
|--------|-------------|----------|
| Parse output, not exit code | Match 'successfully authenticated'→PASS; 'Permission denied (publickey)'→reachable-but-unuploaded; connection/DNS/timeout→FAILURE | ✓ (primary) |
| Exit-code only | Treat exit 0 as pass — wrong, ssh -T exits 1 on success | |

**User's choice:** "both" — interpreted as: classify primarily by output string, with the exit code as a corroborating secondary signal (D-01).

### Q2 — On new-key 'reachable but not-yet-uploaded'

| Option | Description | Selected |
|--------|-------------|----------|
| Write, then guide upload + re-test | Treat as expected new-key state: write artifacts (backup+confirm) → clipboard → upload steps → resolved re-test after upload | ✓ |
| Block write until uploaded | Don't write until ssh -i shows 'successfully authenticated' | |
| Ask each run | Prompt proceed-vs-wait every time | |

**User's choice:** Write, then guide upload + re-test (D-02).

---

## Phase 2 user entry point

### Q1 — How the user drives 'create one identity' in Phase 2

| Option | Description | Selected |
|--------|-------------|----------|
| Real minimal Cobra command | `gitid identity add` — real but minimal, becomes Phase 5 foundation | ✓ |
| Temporary harness/main | Throwaway main/make harness; Cobra deferred to Phase 5 | |
| Library + tests only | Prove via integration tests, no user entry | |

**User's choice:** Real minimal Cobra command (D-04).

### Q2 — How create inputs are supplied

| Option | Description | Selected |
|--------|-------------|----------|
| Interactive prompts | Prompt each field with defaults shown | ✓ |
| Flags only | All inputs as Cobra flags | |
| Flags with prompt fallback | Flags when provided, prompt for missing | |

**User's choice:** Interactive prompts (D-05).

---

## Key naming + passphrase + algorithm

### Q1 — Generated key filename scheme

| Option | Description | Selected |
|--------|-------------|----------|
| id_ed25519_<identity> | OpenSSH-conventional id_<type>_<name> | ✓ (as id_<algo>_<identity>) |
| gitid_<identity> | Namespaced under binary name | |
| <identity> | Shortest, ambiguous | |

**User's choice (free text):** Filename should include the algorithm (D-06), AND introduced a new requirement: probe available tools / algorithms, warn when default (ed25519) unavailable, target best with fallbacks, let the user pick best.

### Q2 — Passphrase policy

| Option | Description | Selected |
|--------|-------------|----------|
| Optional — prompt, allow empty | Prompt but accept empty; Keychain stores it on macOS | ✓ |
| Always passphraseless | No passphrase | |
| Always require | Mandatory non-empty | |

**User's choice:** Optional — prompt, allow empty (D-07).

### Q3 — Load into agent/Keychain on generate

| Option | Description | Selected |
|--------|-------------|----------|
| Yes — ssh-add on generate | ssh-add (macOS --apple-use-keychain) immediately | ✓ |
| No — rely on AddKeysToAgent | Let Host * load on first use | |

**User's choice:** Yes — ssh-add on generate (D-08).

### Follow-up — Scope of algorithm choice / amendment to "ed25519-only"

| Option | Description | Selected |
|--------|-------------|----------|
| (a) Probe + warn + ed25519-with-fallback, single-algorithm offered | Detect support; ed25519 normal path; warn + best fallback (rsa-4096/ecdsa) if missing | ✓ |
| (b) Full algorithm picker | Always list every supported algorithm, user chooses each create | |

**User's choice:** (a) — narrow refinement of PROJECT.md "ed25519-only" (D-09).

### Follow-up — No acceptable algorithm available

**User's request (free text):** When none of the top fallback algorithms is available, point to the OpenSSH repo link with per-OS install steps (macOS / Linux). Captured as D-14 (Phase-2 mini-DOC-01).

---

## Alias + match defaults / create modes

### Q1 — First-identity host binding

| Option | Description | Selected |
|--------|-------------|----------|
| Always an alias | Every identity gets an alias; uniform | (default) |
| Real host, alias on collision | First identity claims real host | |

**User's choice (free text):** "Let the user choose" — surfaced the three create modes (new key / reuse key / alias an existing identity). Resolved: host binding user-chosen, **alias pre-selected as default**, real host overridable (D-12). The three create modes captured as D-10.

### Q2 — Alias naming pattern

| Option | Description | Selected |
|--------|-------------|----------|
| <identity>.<provider> | e.g. work.github.com | ✓ |
| <provider>-<identity> | e.g. github.com-work | |

**User's choice:** `<identity>.<provider>` (D-12).

### Q3 — Default match strategy + hasconfig in Phase 2

| Option | Description | Selected |
|--------|-------------|----------|
| gitdir default, hasconfig available | gitdir:~/git/<identity>/ default; hasconfig also selectable | ✓ |
| gitdir only in Phase 2 | Defer hasconfig renderer | |

**User's choice:** gitdir default, hasconfig available (D-13).

### Follow-up — (A) MVP sequencing / (B) host-bind default

**User's choice:** (A) — prove create-new (#1) end-to-end first; reuse-key (#2) and account-alias (#3) as fast-follow plans within Phase 2 (D-11). (B) not overridden → alias pre-selected default applied (D-12).

---

## Claude's Discretion

- Confirmation/preview UX (unified preview of all four artifacts + single confirm; dry-run).
- `allowed_signers` at `~/.ssh/allowed_signers`; `gpg.ssh.allowedSignersFile` wired globally.
- Backup format `<file>.bak.<timestamp>`.
- Managed-block sentinels `# BEGIN/END gitid managed: <name>`.
- gitconfig writes via `git config`-exec; raw text for `includeIf`.
- SSH parse/render via `kevinburke/ssh_config` with post-write decode-pass validation.
- Exact GitHub/GitLab upload-instruction wording (UP-01/UP-02).

## Deferred Ideas

- Full algorithm picker / per-identity key-type field (later).
- Full doctor health checks DOC-01..07 (Phase 4; D-14 ships only the key-gen install hint).
- List/update/delete/startup reconstruction IDENT-03/04/05/07 (Phase 3).
- `insteadOf` rewriting, `add repo`, global config toggles, adopt fragments (v2).
