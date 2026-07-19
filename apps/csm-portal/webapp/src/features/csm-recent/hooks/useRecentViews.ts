// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import { useCallback, useEffect, useState } from "react";
import { useIdTokenClaims } from "@hooks/useIdTokenClaims";
import type { QuickCaseHit } from "@features/csm-cases/api/useQuickCaseSearch";

export type RecentViewKind =
  | "case"
  | "project"
  | "account"
  | "search"
  | "page";

const RECENT_VIEW_KINDS: readonly RecentViewKind[] = [
  "case",
  "project",
  "account",
  "search",
  "page",
];

function isRecentViewKind(v: unknown): v is RecentViewKind {
  return (
    typeof v === "string" &&
    (RECENT_VIEW_KINDS as readonly string[]).includes(v)
  );
}

export interface RecentView {
  kind: RecentViewKind;
  id: string;
  title: string;
  subtitle?: string;
  href: string;
  /** ISO timestamp of the last visit. */
  visitedAt: string;
  /**
   * Pinned entries form the engineer's working set: they are never evicted by
   * the recency cap and surface in a dedicated "Pinned" group. Absent/false for
   * ordinary history entries.
   */
  pinned?: boolean;
  /**
   * Severity/status/ownership snapshot for `kind: "case"` entries, captured
   * at record time — lets the quick-nav palette render Pinned/Recent cases
   * as the same rich card a live case search hit gets, without re-fetching.
   * Absent for non-case kinds and for entries recorded before this field
   * existed.
   */
  caseHit?: Omit<QuickCaseHit, "id">;
}

/**
 * Base for the per-user storage key (see {@link currentStorageKey}). Kept
 * distinct from the old flat key of the same literal value only for the
 * one-time {@link clearLegacyUnscopedKey} cleanup below — every read/write
 * always goes through the suffixed key.
 */
const STORAGE_KEY_BASE = "csm.recentViews.v1";
/** Recency cap for UNPINNED entries only — pinned entries are always kept. */
const MAX_ENTRIES = 12;
const STORAGE_EVENT = "csm:recent-views-changed";

/**
 * The signed-in user's stable id (ID token `sub` claim), or `null` before
 * it's resolved. Every read/write is scoped under this so a different
 * engineer signing in on the same browser never sees a previous user's
 * recent/pinned cases — see {@link useSyncRecentViewsIdentity}.
 */
let activeUserKey: string | null = null;

function currentStorageKey(): string {
  return `${STORAGE_KEY_BASE}.${activeUserKey ?? "pending"}`;
}

let legacyKeyCleared = false;
/** One-time removal of the old, unscoped key from before per-user scoping existed. */
function clearLegacyUnscopedKey(): void {
  if (legacyKeyCleared) return;
  legacyKeyCleared = true;
  try {
    localStorage.removeItem(STORAGE_KEY_BASE);
  } catch {
    /* ignore */
  }
}

// Wipe the active user's bucket (including pinned entries) on an explicit
// sign-out. Registered once at module load, not tied to any component, so
// it fires reliably regardless of where in the tree sign-out is triggered.
// "app:signing-out" is dispatched ONLY by the manual "Sign out" action
// (UserProfile.tsx) and the idle-timeout auto sign-out (IdleTimeoutProvider.tsx)
// — never by a silent re-auth/token-refresh — so this never clears data out
// from under a user who is still signed in.
if (typeof window !== "undefined") {
  window.addEventListener("app:signing-out", () => {
    try {
      localStorage.removeItem(currentStorageKey());
    } catch {
      /* ignore */
    }
    window.dispatchEvent(new CustomEvent(STORAGE_EVENT));
  });
}

/**
 * Enforce the recency cap on unpinned entries while keeping every pinned entry,
 * preserving overall (recency) order. Pinned entries are the working set the
 * engineer manages explicitly, so they must survive any number of new visits.
 */
function capUnpinned(entries: RecentView[]): RecentView[] {
  const kept: RecentView[] = [];
  let unpinned = 0;
  for (const e of entries) {
    if (e.pinned) {
      kept.push(e);
    } else if (unpinned < MAX_ENTRIES) {
      kept.push(e);
      unpinned += 1;
    }
  }
  return kept;
}

function readStorage(): RecentView[] {
  try {
    const raw = localStorage.getItem(currentStorageKey());
    if (!raw) return [];
    const parsed: unknown = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    // Light shape guard — drop anything that doesn't look like a RecentView.
    return parsed.filter(
      (v): v is RecentView =>
        typeof v === "object" &&
        v !== null &&
        typeof (v as RecentView).id === "string" &&
        // Validate kind against the known set so a malformed entry can't later
        // crash consumers that index by kind.
        isRecentViewKind((v as RecentView).kind) &&
        typeof (v as RecentView).title === "string" &&
        typeof (v as RecentView).href === "string" &&
        typeof (v as RecentView).visitedAt === "string",
    );
  } catch {
    return [];
  }
}

function writeStorage(entries: RecentView[]): void {
  try {
    localStorage.setItem(currentStorageKey(), JSON.stringify(entries));
    // Notify other listeners in the same tab — storage events only fire
    // across tabs, so we dispatch a CustomEvent for in-tab subscribers.
    window.dispatchEvent(new CustomEvent(STORAGE_EVENT));
  } catch {
    // ignore quota errors etc.
  }
}

/** Read-only access to the recent-views list. */
export function useRecentViews(): RecentView[] {
  const [entries, setEntries] = useState<RecentView[]>(() => readStorage());

  useEffect(() => {
    const sync = () => setEntries(readStorage());
    window.addEventListener(STORAGE_EVENT, sync);
    window.addEventListener("storage", sync);
    return () => {
      window.removeEventListener(STORAGE_EVENT, sync);
      window.removeEventListener("storage", sync);
    };
  }, []);

  return entries;
}

/**
 * Resolves the signed-in user's stable id (ID token `sub` claim) and scopes
 * all recent-views reads/writes to it, so a different engineer signing in on
 * the same browser profile never sees a previous user's recent/pinned cases.
 * Mount once near the app root (e.g. `AppLayout`) — every `useRecentViews`/
 * `useRecordRecentView`/`toggleRecentViewPin`/`clearRecentViews` call site
 * reads whichever bucket this last resolved to, with no changes needed at
 * those call sites.
 */
export function useSyncRecentViewsIdentity(): void {
  const sub = useIdTokenClaims()?.sub;

  useEffect(() => {
    clearLegacyUnscopedKey();
  }, []);

  useEffect(() => {
    activeUserKey = sub ?? null;
    // Let already-mounted `useRecentViews()` instances pick up the
    // now-current bucket immediately, rather than waiting for the next
    // unrelated write to trigger a re-read.
    window.dispatchEvent(new CustomEvent(STORAGE_EVENT));
  }, [sub]);
}

/**
 * Record a visit (call from each detail page's effect). De-dupes by `kind+id`,
 * bumps the existing entry to the top, caps the list at {@link MAX_ENTRIES}.
 */
export function useRecordRecentView(): (
  entry: Omit<RecentView, "visitedAt" | "pinned">,
) => void {
  return useCallback((entry: Omit<RecentView, "visitedAt" | "pinned">) => {
    const now = new Date().toISOString();
    const current = readStorage();
    const existing = current.find(
      (e) => e.kind === entry.kind && e.id === entry.id,
    );
    const filtered = current.filter(
      (e) => !(e.kind === entry.kind && e.id === entry.id),
    );
    // Re-visiting an entry must preserve its pinned state — a pin is not lost
    // just because the case was opened again.
    const next: RecentView[] = capUnpinned([
      { ...entry, visitedAt: now, pinned: existing?.pinned },
      ...filtered,
    ]);
    writeStorage(next);
  }, []);
}

/**
 * Pin or unpin an entry by `kind`+`id`. Pinning keeps it in the working set
 * indefinitely; unpinning returns it to ordinary history (subject to the cap).
 * No-op if the entry is not currently tracked.
 */
export function toggleRecentViewPin(kind: RecentViewKind, id: string): void {
  const current = readStorage();
  const next = current.map((e) =>
    e.kind === kind && e.id === id ? { ...e, pinned: !e.pinned } : e,
  );
  writeStorage(capUnpinned(next));
}

export function clearRecentViews(): void {
  // Keep pinned entries — "Clear" wipes browsing history, not the working set
  // the engineer deliberately assembled.
  writeStorage(readStorage().filter((e) => e.pinned));
}
