import { createTheme } from '@mui/material/styles';

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

// 02-UX-DIRECTION.md §2 "Color semantics (restricted, ANSI-safe, adaptive)"
// — every colored state MUST also carry a glyph + a word; this palette only
// supplies the color half of that contract. Never used alone in the UI.
export const semanticColors = {
  healthy: '#4caf50', // green + ✓ + word
  warning: '#d4b106', // yellow + ! + word (ANSI-safe, not neon)
  error: '#e05252', // red + ✗ + word
  dim: '#8a8f98', // gray — helper text, disabled keys
  focus: '#e8e8ea', // reverse/bold surface, not a new hue
  accent: '#5aa9e6', // blue — the ONE accent color for focused-field + active-area
} as const;

/**
 * roles — the semantic style contract's WEB half, mirroring
 * internal/dummytui/theme.go's `Theme` struct 1:1 by role name (see
 * .planning/phases/02-design-all-mockups-checkpoint-1/02-STYLE-SPEC.md for
 * the full cross-media role table). Every consumer reaches for a NAMED role
 * here instead of reaching into semanticColors/sx ad-hoc, so the TUI and the
 * web demo stay provably in sync role-by-role. No new color values are
 * introduced — every role reuses semanticColors or MUI theme tokens.
 */
export const roles = {
  info: { color: '#3aa6a6' },
  label: { fontWeight: 700 },
  field: { border: '1px solid #2a2d33' },
  focusedField: { border: `1px solid ${semanticColors.accent}`, outline: `1px solid ${semanticColors.accent}` },
  blurredField: { border: '1px solid #2a2d33', opacity: 0.85 },
  hint: { color: semanticColors.dim },
  warning: { color: semanticColors.warning },
  error: { color: semanticColors.error },
  preview: { color: semanticColors.dim, opacity: 0.9 },
  disabledNav: { color: semanticColors.dim, opacity: 0.6 },
  activeArea: { border: `1px solid ${semanticColors.accent}` },
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
      default: '#0c0d10',
      paper: '#0c0d10',
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
  },
});

export default theme;
