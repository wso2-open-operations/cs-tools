/**
 * The period vocabulary shared by every analytics surface.
 *
 * Separate from `PeriodSelect.tsx` because a `.tsx` module that exports both a
 * component and plain values breaks Vite's fast refresh — the component's file
 * has to export only components for a hot update to swap it in place. Same
 * reason `selectLabelProps.ts` is its own module.
 */

export const RANGE_OPTIONS = [
  { label: "Last 30 days", days: 30 },
  { label: "Last 90 days", days: 90 },
  { label: "Last 180 days", days: 180 },
  { label: "Last 12 months", days: 365 },
];

export const DEFAULT_RANGE_DAYS = 180;

/** ISO date `days` ago, for the `from` query parameter. */
export function isoDaysAgo(days: number): string {
  const d = new Date();
  d.setDate(d.getDate() - days);
  return d.toISOString().slice(0, 10);
}
