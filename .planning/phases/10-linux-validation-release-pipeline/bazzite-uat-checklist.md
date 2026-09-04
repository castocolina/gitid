# Bazzite Manual UAT Checklist

This is the runnable, per-release manual UAT checklist for the D-03
container-invisible risk items — the second half of D-01's two-part Linux
validation vehicle. The `fedora:latest` container CI job proves everything a
container CAN exercise; this checklist proves the five things it structurally
CANNOT: SELinux enforcement, a real Wayland/desktop session, and a real
`gcr-ssh-agent`/no-agent environment.

Run this checklist on the user's real Bazzite machine, once per release. Each
section gives the exact command, what a PASS looks like, and the ledger row
to update afterward.

## 1. SELinux spot-check on `~/.ssh` writes

**Command:** run `gitid` to write or update an identity (any create/edit
flow that touches `~/.ssh/config` or a key file), then immediately run:

```sh
ls -Z ~/.ssh
```

**PASS:** the SELinux context on the files gitid just wrote is unchanged, or
labeled `ssh_home_t` (or an equivalent user-context label consistent with the
rest of `~/.ssh`). The documented SELinux failure mode is server-side
`authorized_keys` handling, not user-context writes to `~/.ssh/config` or key
files — so a denial or a mislabeled context here would be a genuine,
previously unseen finding, not an expected outcome.

**FAIL looks like:** an `avc: denied` entry in `journalctl` / `ausearch`
around the write, or a context other than the rest of `~/.ssh`'s established
label.

## 2. `/home` to `/var/home` symlink includeIf resolution

**Command:** create an identity with `gitid` (so its `includeIf` fragment is
written to `~/.gitconfig`), then from inside a real clone under the
symlinked home run:

```sh
git config --show-origin --get-regexp .
```

**PASS:** the correct identity's `user.name`/`user.email` resolve for that
clone — i.e. the `includeIf` `gitdir:`/`hasconfig:` match correctly follows
the `/home -> /var/home` symlink Bazzite (and other Fedora Atomic/Silverblue
derivatives) use, rather than silently failing to match because the recorded
path and the resolved real path differ.

**FAIL looks like:** `--show-origin` shows the fragment file was never
included, or a DIFFERENT identity's `user.name`/`user.email` resolves than
the one that should apply for that repo.

## 3. Real-terminal TUI rendering (Ptyxis, Konsole, Wayland)

**Command:** launch `gitid` interactively, once in Ptyxis and once in
Konsole, both under a real Wayland session:

```sh
gitid
```

**PASS:** no glyph corruption (box-drawing characters, unicode symbols all
render as designed), no color-scheme breakage (the theme's colors render as
intended in both terminal emulators' default color schemes), and mouse
click-to-focus works exactly as it does on macOS (clicking a field or button
focuses/activates it, no dead zones or offset misclicks).

**FAIL looks like:** mangled box-drawing characters, wrong/washed-out colors,
or mouse clicks landing on the wrong element (an offset or scaling bug
specific to one terminal emulator's Wayland reporting).

## 4. `wl-clipboard` presence (KDE/GNOME Wayland)

**Command:** from the Identity Manager, use gitid's copy-public-key action,
then paste outside the terminal (e.g. into a text editor or browser address
bar).

**PASS:** the public key lands in the system clipboard via `wl-copy` (or the
GNOME/KDE Wayland clipboard equivalent gitid's `internal/deps`/clipboard
layer shells out to) with no error surfaced to the user.

**FAIL looks like:** a visible error/toast in gitid, or the copy action
completing with no error but nothing actually landing in the system
clipboard on paste.

## 5. `ssh-add` / no-agent graceful degradation

**Command:** on a machine (or a shell) with no running SSH agent — Fedora
KDE ships none by default — confirm gitid does not error or hang anywhere it
optionally shells to `ssh-add`:

```sh
unset SSH_AUTH_SOCK
gitid
```

**PASS:** a clean advisory/skip wherever gitid would normally offer to add a
key to a running agent — never a crash, never a hang waiting on a
nonexistent agent socket.

**FAIL looks like:** gitid hangs waiting on `$SSH_AUTH_SOCK`, or crashes with
an unhandled error instead of degrading gracefully.

## After running all five

Update the corresponding rows in
[`../../../../PLATFORM-NOTES.md`](../../../../PLATFORM-NOTES.md) (relative to
this file) from "Pending manual UAT" to either:

- **"Verified"** — with a one-line note describing what was confirmed, or
- **"Accepted limitation"** — with the workaround, if a real gap was found
  and is being consciously accepted rather than fixed this release.

**Before committing any finding:** redact any real hostname, username, or
local filesystem path that appeared in a command's output or an error
message. These files are public and repo-committed — only the PASS/FAIL
verdict and a generic description belong in the ledger, never
machine-identifying detail.
