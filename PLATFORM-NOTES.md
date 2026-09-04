# Platform Notes

This is the per-distro portability ledger for `gitid` (PLAT-03). It exists
because Linux validation for this project runs in two parts (D-01):

1. **A `fedora:latest` container job in CI** — runs the full automated suite
   (`make test` with `-race`, `make lint`, `make test-e2e`) on every
   push-to-main and release tag, after a `dnf install` step that adds the
   packages the bare Fedora image doesn't ship (`git`, `openssh-clients`,
   `make`, `nodejs`, `tar`, `gzip`).
2. **One documented manual UAT on the user's real Bazzite machine per
   release**, covering exactly the residue a container structurally cannot
   exercise: no SELinux enforcement, no real Wayland/desktop session, no
   `gcr-ssh-agent`. Bazzite's userland IS Fedora's (image delivery + gaming
   additions are the only delta), so this container-plus-real-machine pair
   gives full coverage without needing dedicated Bazzite CI hardware. The
   runbook for that manual pass lives at
   [`.planning/phases/10-linux-validation-release-pipeline/bazzite-uat-checklist.md`](.planning/phases/10-linux-validation-release-pipeline/bazzite-uat-checklist.md).

| Distro | Aspect | Status | Workaround | Issue |
|--------|--------|--------|------------|-------|
| Fedora | `ssh -V` version-string parsing | Verified: no distro suffix on `OpenSSH_10.2p1`, unlike Debian/Ubuntu's suffixed form; the existing parser already handles both | n/a | none |
| Fedora (container) | CI prerequisites | Verified: the base image ships none of git/openssh-clients/make/nodejs by default; installed via dnf in the CI job | `dnf install -y --setopt=install_weak_deps=False git openssh-clients make nodejs tar gzip` | none |
| Bazzite | `wl-clipboard` presence (KDE/GNOME Wayland) | Pending manual UAT | none yet | none |
| Bazzite | `ssh-add` / no-agent graceful degradation (Fedora KDE ships none by default; GNOME uses the `gcr-ssh-agent` systemd user socket) | Pending manual UAT | none yet | none |
| Bazzite | SELinux spot-check on `~/.ssh` writes | Pending manual UAT, verify-once-and-log | none yet | none |
| Bazzite | `/home` to `/var/home` symlink includeIf resolution | Pending manual UAT | none yet | none |
| Bazzite | Real-terminal TUI rendering (Ptyxis, Konsole, Wayland) | Pending manual UAT | none yet | none |

## How this ledger is updated

Every Bazzite UAT finding (see
[`bazzite-uat-checklist.md`](.planning/phases/10-linux-validation-release-pipeline/bazzite-uat-checklist.md))
updates a row here — a fixed gap gets an Issue link, an accepted limitation
gets a Workaround note. This happens once per release, alongside the manual
UAT pass.
