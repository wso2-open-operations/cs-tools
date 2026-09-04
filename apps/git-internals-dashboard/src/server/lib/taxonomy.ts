// src/server/lib/taxonomy.ts
// Config-driven status taxonomy for the API layer. Pure, synchronous reads
// over the in-process config — no DB round-trip, no TTL cache needed (the
// config object itself is already memoized by @/server/config).
import { loadConfig } from "@/server/config";

export interface StatusDefRow {
  name: string;
  category: string;
  accruesSla: boolean;
  isTerminal: boolean;
  sortOrder: number;
}

/** Status names categorized CS_SIDE (the statuses currently on the CS side of the board). */
export function getCsStatuses(): string[] {
  return loadConfig()
    .taxonomy.statuses.filter((s) => s.category === "CS_SIDE")
    .sort((a, b) => a.sortOrder - b.sortOrder)
    .map((s) => s.name);
}

/** Status names categorized PRODUCT_SIDE (the statuses currently owned by the product team). */
export function getProductSideStatuses(): string[] {
  return loadConfig()
    .taxonomy.statuses.filter((s) => s.category === "PRODUCT_SIDE")
    .sort((a, b) => a.sortOrder - b.sortOrder)
    .map((s) => s.name);
}

/** Full taxonomy rows, sortOrder-ascending. */
export function getStatusDefinitions(): StatusDefRow[] {
  return [...loadConfig().taxonomy.statuses]
    .sort((a, b) => a.sortOrder - b.sortOrder)
    .map((s) => ({
      name: s.name,
      category: s.category,
      accruesSla: s.accruesSla,
      isTerminal: s.isTerminal,
      sortOrder: s.sortOrder,
    }));
}
