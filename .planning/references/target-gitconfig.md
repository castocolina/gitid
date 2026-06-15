# Reference: target `~/.gitconfig` structure

Captured from a user-provided gist during initialization. Shows the **structural**
end-state: `insteadOf` HTTPS→SSH rewrites, an unconditional `include`, and
`includeIf` blocks using **both** `hasconfig:remote.*.url:` and `gitdir:` match
strategies, each pointing at a per-identity fragment.

```gitconfig
[core]
excludesfile = ~/.gitignore_global
ignorecase = false

[push]
autoSetupRemote = true

[url "git@github.com:"]
insteadOf = https://github.com/
[url "git@github.companyname.com:"]
insteadOf = https://github.companyname.com/
[url "git@gitlab.com:"]
insteadOf = https://gitlab.com/
[url "git@gitlab.companyname.com:"]
insteadOf = https://gitlab.companyname.com/
[url "git@bitbucket.org:"]
insteadOf = https://bitbucket.org/

[include]
path = ~/.gitconfig_default

[includeIf "hasconfig:remote.*.url:git@github.companyname.com:*/**"]
path = ~/.gitconfig_companyname
[includeIf "hasconfig:remote.*.url:https://github.companyname.com/*/*"]
path = ~/.gitconfig_companyname
[includeIf "hasconfig:remote.*.url:git@personal.github.com:*/**"]
path = ~/.gitconfig_personal
[includeIf "hasconfig:remote.*.url:https://github.com/yourusername:*/**"]
path = ~/.gitconfig_personal
[includeIf "hasconfig:remote.*.url:git@gitlab.companyname.com:*/**"]
path = ~/.gitconfig_gitlab_work
[includeIf "hasconfig:remote.*.url:https://gitlab.companyname.com:*/**"]
path = ~/.gitconfig_gitlab_work
[includeIf "gitdir:~/work/"]
path = ~/.gitconfig_work
[includeIf "gitdir:~/personal/"]
path = ~/.gitconfig_personal_alt
```

## Notable elements
- **Dual match strategy:** the same identity can be selected by remote URL
  (`hasconfig:`) *and* by directory (`gitdir:`). The PRD makes `gitdir:` the
  default suggestion (matching the `~/git/<client>/` layout) with `hasconfig:` as
  an option — confirm both are renderable.
- **insteadOf rewrites** are listed per provider — this is the Phase 2 "URL
  rewriting with HTTPS suggestion" feature in its end state.
- **Fragment referencing:** the plain-style fragment names here (`~/.gitconfig_personal`)
  are exactly the kind of pre-existing files the PRD's "adopt existing fragments"
  feature should detect and offer to migrate into `~/.gitconfig.d/`.
- The PRD's per-identity fragment adds `gpg.format=ssh`, `user.signingkey`,
  `commit.gpgsign true` (signing wiring not present in this older reference).
