// src/server/db/normalize.ts
// Status-name normalization: formatting variants ("Re-Opened") map to their
// canonical name ("Reopened") before any taxonomy lookup. One implementation
// shared by the seed, the recompute job, and the incremental sync.
export function buildStatusNormalizer(aliases: ReadonlyArray<{ alias: string; canonical: string }>) {
  const map = new Map(aliases.map((a) => [a.alias, a.canonical]));
  return (status: string | null): string | null => {
    if (status == null) return null;
    const trimmed = status.trim(); // whitespace-only folding is safe;
    return map.get(trimmed) ?? trimmed; // anything more (case, hyphens) must
    // be an explicit alias row — automatic
    // folding could merge intentionally
    // distinct statuses.
  };
}
