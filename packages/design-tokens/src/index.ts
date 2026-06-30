/**
 * Gotra design tokens — the purple-first design system from the Frontend
 * Product & Design Bible. Brand purple is reserved for gradients, chart
 * accents, highlight states and CTAs; surfaces use neutral dark/light tokens.
 */

/** Primary purple scale (50–950). */
export const purple = {
  50: "#f5f3ff",
  100: "#ede9fe",
  200: "#ddd6fe",
  300: "#c4b5fd",
  400: "#a78bfa",
  500: "#8b5cf6",
  600: "#7c3aed",
  700: "#6d28d9",
  800: "#5b21b6",
  900: "#4c1d95",
  950: "#2e1065",
} as const;

/** Primary brand gradient: deep purple → electric purple. */
export const gradients = {
  primary: "linear-gradient(135deg, #4c1d95 0%, #7c3aed 50%, #a855f7 100%)",
  ai: "linear-gradient(135deg, #6d28d9 0%, #8b5cf6 50%, #c084fc 100%)",
} as const;

/** Radius scale (px). */
export const radius = {
  sm: 8,
  md: 12,
  lg: 16,
  xl: 20,
  "2xl": 24,
} as const;

/** Spacing scale (px) — 4px base. */
export const spacing = [4, 8, 12, 16, 20, 24, 32, 40, 48, 64, 80, 96, 128] as const;

/** Dashboard layout dimensions (px). */
export const layout = {
  sidebarWidth: 260,
  topBarHeight: 64,
  contentPadding: 24,
  gridGap: 24,
} as const;

/** Typography. */
export const typography = {
  fontPrimary: '"General Sans", "Inter", system-ui, sans-serif',
  fontFallback: '"Inter", system-ui, sans-serif',
  weights: [400, 500, 600, 700],
} as const;
