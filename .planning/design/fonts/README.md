# Vendored fonts — deterministic screenshot-tui rendering

## Why a vendored font

`freeze` (the ANSI terminal-output -> PNG renderer used by `make screenshot-tui`)
falls back to whatever monospace font the host OS happens to have installed when
`--font.file` is not given. A CI runner's font set rarely matches a developer's
local machine, so two content-identical renders can differ purely from
font-rendering noise (kerning, glyph coverage, anti-aliasing) — not a real visual
regression. See RESEARCH.md "Pitfall 6".

`internal/screenshot/tui_capture_test.go` always passes this exact vendored file
to freeze's `--font.file` flag, on every OS, so rendering never depends on
system-font discovery.

## Provenance

| Field | Value |
|-------|-------|
| Font | JetBrains Mono |
| File | `JetBrainsMono-Regular.ttf` |
| Source | https://github.com/JetBrains/JetBrainsMono |
| Pinned tag | `v2.304` |
| Fetched from | `https://raw.githubusercontent.com/JetBrains/JetBrainsMono/v2.304/fonts/ttf/JetBrainsMono-Regular.ttf` |
| SHA-256 | `a0bf60ef0f83c5ed4d7a75d45838548b1f6873372dfac88f71804491898d138f` |
| License | SIL Open Font License 1.1 (`OFL.txt`, vendored alongside the TTF, fetched from the same pinned tag) |
| License source | https://github.com/JetBrains/JetBrainsMono/blob/v2.304/OFL.txt |

The OFL explicitly permits redistribution (including bundling inside another
software distribution) as long as the font is not sold by itself and the license
text travels with it — both satisfied here (`OFL.txt` is vendored alongside the
TTF, and the font is used only as a rendering input for gitid's own dev tooling,
never sold on its own).

## Regenerating / verifying

```bash
curl -sSfL -o /tmp/verify.ttf \
  "https://raw.githubusercontent.com/JetBrains/JetBrainsMono/v2.304/fonts/ttf/JetBrainsMono-Regular.ttf"
shasum -a 256 /tmp/verify.ttf .planning/design/fonts/JetBrainsMono-Regular.ttf
# both hashes must match a0bf60ef0f83c5ed4d7a75d45838548b1f6873372dfac88f71804491898d138f
```
