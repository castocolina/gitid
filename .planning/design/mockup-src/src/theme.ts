import { createElement } from 'react';
import { createTheme } from '@mui/material/styles';
import { healthInfoColor } from './data/recipeFixtures';

/**
 * gitid design mockup — terminal-skin MUI v7 theme.
 *
 * Built with the `/mui` skill (createTheme + ThemeProvider) under
 * agent-ui-ux-designer direction (02-UX-DIRECTION.md §0 Risk 1, §1, §2).
 *
 * This theme deliberately strips Material Design's visual idiom so the
 * mockup reads as *a screenshot of a terminal* — in the lineage of
 * lazygit/k9s/tig/htop — rather than a generic SaaS dashboard:
 *   - monospace font (self-hosted JetBrains Mono, no CDN — Pitfall 6)
 *   - shape.borderRadius: 0 (no rounded corners anywhere)
 *   - ALL shadows set to 'none' (no elevation, flat surfaces)
 *   - transitions.duration.* all 0ms — deterministic screenshot capture
 *     (02-RESEARCH.md Pitfall 5: MUI's default animated transitions make
 *     go-rod captures non-reproducible across runs)
 *   - ripple disabled by default (TouchRipple has no place in a keyboard-
 *     first terminal tool)
 *   - a dark, ANSI-like semantic palette matching 02-UX-DIRECTION.md §2's
 *     color-semantics table: healthy=green, warning=yellow,
 *     error/destructive=red, dim=gray, focus=reverse/bold (not a new hue)
 */

// terminalBg is the shared terminal background surface — the one layout
// color the palette's background.default/paper and the activeNav role's
// contrasting text both key from (kept as a single constant so the two can
// never drift).
const terminalBg = '#0c0d10';

// 02-UX-DIRECTION.md §2 "Color semantics (restricted, ANSI-safe, adaptive)"
// — every colored state MUST also carry a glyph + a word; this palette only
// supplies the color half of that contract. Never used alone in the UI.
//
// review-findings F7: `accent` below IS a genuinely new color value, added by
// 02-14 Task 1 for the focused-field contour and the active-area chrome (the
// TUI's ANSI-4 blue accent had no existing web equivalent). This is the ONE
// deliberate new color this plan introduces — documented as a deviation in
// 02-14-SUMMARY.md — every OTHER role below still reuses an existing
// semanticColors/MUI token.
export const semanticColors = {
  healthy: '#4caf50', // green + ✓ + word
  warning: '#d4b106', // yellow + ! + word (ANSI-safe, not neon)
  error: '#e05252', // red + ✗ + word
  dim: '#8a8f98', // gray — helper text, disabled keys
  focus: '#e8e8ea', // reverse/bold surface, not a new hue
  accent: '#5aa9e6', // blue — the ONE new accent color: focused-field, active-area, active-nav
} as const;

/**
 * roles — the semantic style contract's WEB half, mirroring
 * internal/dummytui/theme.go's `Theme` struct 1:1 BY ROLE NAME (see
 * .planning/phases/02-design-all-mockups-checkpoint-1/02-STYLE-SPEC.md for
 * the full cross-media role table). Color VALUES are shared with the TUI via
 * `semanticColors` above (and `healthInfoColor`, imported below rather than
 * re-hardcoded — review-findings F11).
 *
 * Checkpoint feedback U2 (upgrades review-finding F8): the live web demo is
 * now IN SYNC ROLE-BY-ROLE with the TUI — every semantic color the demo
 * renders flows through a named role here (or through the MUI palette
 * entries createTheme builds from `semanticColors` below, e.g. Alert
 * severities and TextField error states). The deliberate, documented
 * exceptions — mirroring the TUI's own role-less treatments — are:
 *   1. `semanticColors.focus` (the reverse/bold focus-and-selection
 *      surface: sub-tab strips, inline link text). The TUI's counterparts
 *      (styleReverse/styleSelected) are equally role-less by design —
 *      02-STYLE-SPEC.md §1 scope note.
 *   2. Pure LAYOUT grays with no semantic meaning: `#2a2d33` (borders/
 *      divider), `#5a5a5a` (the "no capability" pip), `#8a8a8a` (the S/G
 *      pip letter tint). These are chrome, not states.
 */
export const roles = {
  info: { color: healthInfoColor },
  label: { fontWeight: 700 },
  field: { border: '1px solid #2a2d33' },
  focusedField: { border: `1px solid ${semanticColors.accent}`, outline: `1px solid ${semanticColors.accent}`, color: semanticColors.accent },
  blurredField: { border: '1px solid #2a2d33', opacity: 0.85 },
  hint: { color: semanticColors.dim },
  warning: { color: semanticColors.warning },
  error: { color: semanticColors.error },
  preview: { color: semanticColors.dim, opacity: 0.9 },
  disabledNav: { color: semanticColors.dim, opacity: 0.6 },
  activeArea: { border: `1px solid ${semanticColors.accent}`, color: semanticColors.accent },
  // Checkpoint feedback U1: the ACTIVE main-nav item carries the shared
  // accent as a BACKGROUND (mirrors the TUI's Theme.ActiveNav — bold +
  // bright-white on the ANSI-4 blue), clearly saying "I am at 1/2/3/4"
  // instead of a flat monochrome invert.
  activeNav: {
    background: semanticColors.accent,
    borderColor: semanticColors.accent,
    color: terminalBg,
    fontWeight: 700,
  },
  // review-findings F11: the Go Theme struct carries a Healthy role with no
  // web counterpart — added here for full 1:1 name parity (cheap, mechanical).
  healthy: { color: semanticColors.healthy },
  // activeNavDimmed (D4, checkpoint-2 contract): the ACTIVE main-nav item
  // while a modal/edit/ceremony pane captures keys — accent text/border,
  // TRANSPARENT background (fontWeight 700), mirroring the TUI's NEW
  // Theme.ActiveNavDimmed role. Distinct from BOTH the full activeNav
  // background treatment (no pane capturing keys) and disabledNav (an
  // INACTIVE tab while capturing).
  activeNavDimmed: {
    background: 'transparent',
    borderColor: semanticColors.accent,
    color: semanticColors.accent,
    fontWeight: 700,
  },
} as const;

// All 25 MUI shadow elevations flattened to 'none' — a terminal cell has no
// elevation. This is an explicit array (not a single override), matching
// MUI's Shadows tuple shape (25 entries, elevation 0-24).
const flatShadows = Array(25).fill('none') as unknown as ReturnType<
  typeof createTheme
>['shadows'];

const terminalFontFamily =
  '"JetBrains Mono", ui-monospace, SFMono-Regular, Menlo, Consolas, monospace';

export const theme = createTheme({
  palette: {
    mode: 'dark',
    background: {
      default: terminalBg,
      paper: terminalBg,
    },
    text: {
      primary: '#e8e8ea',
      secondary: semanticColors.dim,
    },
    primary: {
      main: semanticColors.focus,
    },
    success: {
      main: semanticColors.healthy,
    },
    warning: {
      main: semanticColors.warning,
    },
    error: {
      main: semanticColors.error,
    },
    divider: '#2a2d33',
  },
  shape: {
    borderRadius: 0,
  },
  shadows: flatShadows,
  typography: {
    fontFamily: terminalFontFamily,
    fontSize: 14,
    allVariants: {
      fontFamily: terminalFontFamily,
    },
  },
  transitions: {
    duration: {
      shortest: 0,
      shorter: 0,
      short: 0,
      standard: 0,
      complex: 0,
      enteringScreen: 0,
      leavingScreen: 0,
    },
  },
  components: {
    MuiButtonBase: {
      defaultProps: {
        disableRipple: true,
        disableTouchRipple: true,
      },
    },
    MuiButton: {
      styleOverrides: {
        // review-findings F6: MUI's default Button uppercases its label via
        // textTransform; the frozen button copy (`[ Skip Git ]`, `[ Continue
        // ]`, …) must render byte-identical to the TUI, which has no such
        // transform — disable it globally, terminal-faithful either way.
        root: {
          textTransform: 'none',
        },
      },
    },
    MuiPaper: {
      styleOverrides: {
        root: {
          backgroundImage: 'none',
          border: '1px solid #2a2d33',
        },
      },
    },
    MuiCssBaseline: {
      styleOverrides: {
        body: {
          fontFamily: terminalFontFamily,
        },
      },
    },
    // review-findings F8: the `label` role (fontWeight 700) is now routed
    // through every MUI field label via a theme-level override — the TUI
    // bolds field labels (styleBold); the web previously left MUI's default
    // (non-bold) label weight untouched (designer LOW-2).
    MuiInputLabel: {
      styleOverrides: {
        root: {
          fontWeight: roles.label.fontWeight,
        },
      },
    },
    MuiFormLabel: {
      styleOverrides: {
        root: {
          fontWeight: roles.label.fontWeight,
        },
      },
    },
    // D3 (checkpoint-2 contract): terminal-glyph checkbox/radio — the
    // FROZEN glyphs shared with the TUI (checkbox ☐/☑, radio ○/●), routed
    // through theme-level defaultProps so EVERY MuiCheckbox/MuiRadio in the
    // app renders them at once, removing the stock Material icons
    // everywhere in a single change.
    MuiCheckbox: {
      defaultProps: {
        icon: createElement('span', { 'aria-hidden': true }, '☐'),
        checkedIcon: createElement('span', { 'aria-hidden': true }, '☑'),
      },
      styleOverrides: {
        root: {
          '&.Mui-disabled': { color: roles.hint.color },
        },
      },
    },
    MuiRadio: {
      defaultProps: {
        icon: createElement('span', { 'aria-hidden': true }, '○'),
        checkedIcon: createElement('span', { 'aria-hidden': true }, '●'),
      },
      styleOverrides: {
        root: {
          '&.Mui-disabled': { color: roles.hint.color },
        },
      },
    },
  },
});

export default theme;
