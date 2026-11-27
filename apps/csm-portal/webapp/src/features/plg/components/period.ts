/**
 * The period vocabulary shared by every analytics surface.
 *
 * Separate from `PeriodSelect.tsx` because a `.tsx` module that exports both a
 * component and plain values breaks Vite's fast refresh — the component's file
 * has to export only components for a hot update to swap it in place. Same
 * reason `selectLabelProps.ts` is its own module.
 */

export const RANGE_OPTIONS = [
  { label: "Last 7 days", days: 7 },
  { label: "Last 14 days", days: 14 },
  { label: "Last 30 days", days: 30 },
];

export const DEFAULT_RANGE_DAYS = 30;

/** ISO (UTC) date that starts a window of `days` days ending today, for the `from` query parameter. */
export function isoDaysAgo(days: number): string {
  return new Date(Date.now() - (days - 1) * 86_400_000).toISOString().slice(0, 10);
}
