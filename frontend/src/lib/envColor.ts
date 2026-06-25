// Per-environment dot colours for the env switcher pill. Stored as a hex string
// on the environment; unset falls back to a neutral grey.
export const ENV_COLORS = [
  "#22c55e", // green
  "#eab308", // amber
  "#ef4444", // red
  "#3b82f6", // blue
  "#a855f7", // purple
  "#06b6d4", // cyan
  "#ec4899", // pink
  "#94a3b8", // slate
] as const;

const DEFAULT = "#64748b"; // neutral grey when unset

export function envColor(color: string | undefined): string {
  return color && color.trim() ? color : DEFAULT;
}
