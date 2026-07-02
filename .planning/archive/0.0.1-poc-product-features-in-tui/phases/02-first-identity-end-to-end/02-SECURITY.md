---
phase: 2
slug: first-identity-end-to-end
status: secured
threats_open: 0
asvs_level: 1
created: 2026-06-09
---

# SECURITY.md — Phase 2: First Identity End-to-End

**Audit date:** 2026-06-09
**Phase:** 02-first-identity-end-to-end
**ASVS Level:** 1
**Auditor:** gsd-security-auditor (automated verification) + orchestrator follow-up
**Result:** SECURED — threats_open: 0 (34/34 dispositions resolved)

> Register origin: `register_authored_at_plan_time: true` — every 02-0N-PLAN.md
> carried a `<threat_model>` STRIDE block, so this audit VERIFIES the planned
> mitigations exist in the shipped code (it does not scan for new threats).

---

## Threat Verification Table

| Threat ID | Category | Disposition | Status | Evidence |
|-----------|----------|-------------|--------|----------|
| T-02-01 | InfoDisclosure | mitigate | CLOSED | `filewriter.go:14` backupMode=0600; `filewriter.go:37` copyFile called with backupMode; `filewriter.go:132` explicit os.Chmod(dst, mode) in copyFile |
| T-02-02 | DoS | mitigate | CLOSED | `filewriter.go:44-84` atomic temp→Sync→Rename; cleanup func removes temp on any error after creation; target left untouched |
| T-02-03 | InfoDisclosure | mitigate | CLOSED | `filewriter.go:73` os.Chmod(tmpName, mode) after Sync/Close, before Rename; never relies on umask; explicit mode contract in callers |
| T-02-04 | Tampering | mitigate | CLOSED | `filewriter.go:47` os.CreateTemp(dir, "gitid-*.tmp") — unique name, never a fixed suffix |
| T-02-05 | Tampering | mitigate | CLOSED | `filewriter.go:47,105,111` //nolint:gosec with trust rationale; modes all explicit; make lint clean per 02-01-SUMMARY.md |
| T-02-06 | Tampering/Elev | mitigate | CLOSED | `platform.go:56` exec.Command("ssh", "-Q", "key") — arg-slice, 3 fixed args, zero user input |
| T-02-07 | DoS | mitigate | CLOSED | `platform.go:54-60` uses `ssh -Q key` (not `ssh-keygen`); InstallHint at `platform.go:109-123` provides per-OS actionable D-14 guidance |
| T-02-08 | Spoofing | ACCEPT | CLOSED | `platform.go:87-98` SelectAlgorithm returns `warned = i > 0`; `add.go:55-63` caller checks `if warned { fp(out, "Note: ed25519 unavailable...") }` — downgrade surfaced to user |
| T-02-09 | InfoDisclosure | mitigate | CLOSED | `keygen.go:86` filewriter.Write(privPath, privPEM, 0o600); `keygen.go:89` filewriter.Write(pubPath, pubLine, 0o644); no os.WriteFile in production path |
| T-02-10 | CryptoWeak | mitigate | CLOSED | `keygen.go:67,69,80` ssh.MarshalPrivateKeyWithPassphrase / ssh.MarshalPrivateKey / ssh.MarshalAuthorizedKey from golang.org/x/crypto/ssh v0.53.0 |
| T-02-11 | Spoofing | mitigate | CLOSED | `signers.go:23` fmt.Sprintf("%s namespaces=\"git\" %s\n", email, keyText) — namespaces="git" hardcoded, email byte-identical to supplied value |
| T-02-33 | Tampering | mitigate | CLOSED | `signers.go:41-43` filewriter.ReplaceBlock(existing, identity, ...) then filewriter.Write(path, composed, 0644) — idempotent per-identity block; foreign lines preserved |
| T-02-12 | Tampering | mitigate | CLOSED | go.sum present with 20 entries; all four deps pinned (atotto/clipboard v0.1.4, kevinburke/ssh_config v1.6.0, spf13/cobra v1.10.2, golang.org/x/crypto v0.53.0); no install scripts in Go modules |
| T-02-SC | Tampering | mitigate | CLOSED | Same evidence as T-02-12; go.sum checksums verified at build time |
| T-02-13 | Spoofing | mitigate | CLOSED | `renderer.go:33` fmt.Fprintf(&b, "%sIdentitiesOnly yes\n", ...) — every RenderHostBlock call emits IdentitiesOnly yes + explicit IdentityFile |
| T-02-14 | DoS | mitigate | CLOSED | `renderer.go:58` IgnoreUnknown UseKeychain emitted first; `renderer.go:59` UseKeychain yes after; ordering enforced by sequential Fprintf calls |
| T-02-15 | Tampering | mitigate | CLOSED | `writer.go:47-49` hostBlock ReplaceBlock first, then globalBlock ReplaceBlock second — _global block always written last |
| T-02-16 | DoS | mitigate | CLOSED | `writer.go:58` filewriter.Write(configPath, composed, configMode) — no os.WriteFile in sshconfig/writer.go |
| T-02-17 | Tampering | mitigate | CLOSED | `writer.go:47` filewriter.ReplaceBlock(existing, accountName, hostBlock) — idempotent splice; foreign content untouched |
| T-02-18 | Tampering/Elev | mitigate | CLOSED | `fragment.go:25,28,31,55` validateValue/validateEmail reject newlines and [remote]; `fragment.go:65` exec.Command("git","config","--file",path,key,value) arg-slice; `tester.go:102` exec.Command("ssh", args...) arg-slice; `add.go:402` exec.Command("ssh-add", args...) arg-slice |
| T-02-19 | Spoofing | mitigate | CLOSED | `fragment.go:16` comment: "(a .pub PATH, never an inline key — SIGN-02)"; `fragment.go:39` {"user.signingkey", signingKeyPath} — signingKeyPath is a filesystem path |
| T-02-20 | DoS | mitigate | CLOSED | `fragment.go:79-81` validateValue checks strings.Contains(strings.ToLower(value), "[remote") — rejects at render time |
| T-02-21 | Spoofing | mitigate | CLOSED | `tester.go:49-58` ClassifyPreWrite uses strings.Contains substring match only; exit code discarded (line 78 `out, _ = run(args)`) |
| T-02-22 | InfoDisclosure | mitigate | CLOSED | `tester.go:25-29` Result carries Command (input) + Output (raw); error messages in identity.go:182-184 emit alias + command + output only, not private key body |
| T-02-23 | Tampering/Elev | mitigate | CLOSED | **Fixed in commit 6711bb1 (quick task 260609-qd6).** `add.go:217` `sanitizeName(prompt(...))` + `add.go:221` `if !identityNameRe.MatchString(name)` in gatherAddAccount; `add.go:269,273` same guard in gatherCreateInput. Reuses `identityNameRe = ^[A-Za-z0-9._-]+$` and `sanitizeName` from rotate.go (same package main). Tests `TestGatherCreateInputRejectsUnsafeName` / `TestGatherAddAccountRejectsUnsafeName` reject `../evil`, `a/b`, `name;rm`; accept `work`, `personal.gh`, `my-id_2`. Path-traversal into `~/.gitconfig.d/<name>` now blocked on all three entry points (add, add-account, rotate). |
| T-02-24 | DoS | mitigate | CLOSED | `identity.go:180-184` pre.Outcome == tester.Failure returns error before any write call; writes at lines 204-215 only reached after gate passes and in.Confirmed |
| T-02-25 | Repud/Spoof | mitigate | CLOSED | `identity.go:199-201` if !in.Confirmed returns PreWriteOnly=true with no writes; `add.go:306` in.Confirmed = confirm(r, out, "Write all four artifacts now?") — single y/N gate |
| T-02-26 | Spoofing | mitigate | CLOSED | `tester.go:66` "-o", "IdentitiesOnly=yes" in preWriteArgs; `renderer.go:33` IdentitiesOnly yes in every rendered host block |
| T-02-34 | Spoofing | mitigate | CLOSED | `identity.go:213` deps.WriteAllowedSigners(in.AllowedSignersPath, in.Name, signersLine) called as 4th write after WriteSSH/WriteGitconfig/WriteFragment |
| T-02-27 | InfoDisclosure | mitigate | CLOSED | `identity.go:182-184` error message emits alias + pre.Command + pre.Output only; `add.go:408-411` printPreWrite emits command+output; no private key path in user-facing log strings |
| T-02-28 | InfoDisclosure | mitigate | CLOSED | `derive.go:23-37` signer.PublicKey() only, private bytes never returned or logged; `modes.go:36` comment confirms; `add.go:367-371` WritePub dep writes pubLine at 0644 via filewriter |
| T-02-29 | Tampering | mitigate | CLOSED | `modes.go:127` Rotate calls runPipeline with same identity name; ReplaceBlock keyed by name replaces the old block in each artifact; `writer.go:47`, `signers.go:41` use ReplaceBlock |
| T-02-30 | Spoofing | mitigate | CLOSED | `modes.go:31` Reuse → runPipeline → `identity.go:186` RenderHostBlock emits IdentitiesOnly yes; resolved ssh -G assertion present in resolved test |
| T-02-31 | Repudiation | mitigate | CLOSED | `modes.go:120-128` Rotate: `rotateInput` sets Confirmed:true only after cmd-layer confirm gate in `rotate.go:68-71`; all modes share runPipeline confirmation check |
| T-02-32 | Tampering/Elev | mitigate | CLOSED | `rotate.go:28` identityNameRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`); `rotate.go:57` validated before use; arg-slice exec only throughout |

---

## Open Threats

None. All 34 threats CLOSED (33 mitigate verified + 1 accept documented).

The single blocker from the initial audit run (T-02-23 — missing identity-name
charset guard on the `add` / `add-account` entry points) was remediated in quick
task **260609-qd6** before this phase advanced: RED `6953a54` → GREEN `6711bb1`.

---

## Unregistered Flags

None detected. All SUMMARY.md threat flags map to registered threat IDs.

---

## Accepted Risks Log

| Risk ID | Threat Ref | Rationale | Accepted By | Date |
|---------|------------|-----------|-------------|------|
| AR-02-01 | T-02-08 | SelectAlgorithm returns `warned=true` for any non-ed25519 selection; cmd layer prints a "Note: ed25519 unavailable" message to the user (add.go:55-63). Ed25519 remains the default. The downgrade is surfaced, not silenced. | user (Phase 2 plan) | 2026-06-09 |

---

## Deferred Manual Verification

The following success criteria require real SSH provider access and are deferred per `02-VALIDATION.md`:
- IDENT-02: reuse flow derive+write `.pub` + clipboard copy with real key
- IDENT-06: add-account alias coexistence: `ssh -G` resolves same key
- KEY-01: rotate flow: backups present, four artifacts re-point to new key, `ssh -T` authenticates after upload
- CLIP-01: clipboard copy on macOS/Linux
- Auth + signing end-to-end: `git log --show-signature` shows "Good signature"

---

## Security Audit 2026-06-09

| Metric | Count |
|--------|-------|
| Threats in register | 34 |
| Closed | 34 |
| Open | 0 |
| Accepted risks | 1 (T-02-08) |

Initial run: 33 closed / 1 open (T-02-23). After remediation (quick 260609-qd6): 34 closed / 0 open → **SECURED**.
