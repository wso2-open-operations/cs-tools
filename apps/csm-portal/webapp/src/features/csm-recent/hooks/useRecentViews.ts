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
import { stripHtmlTags } from "@utils/sanitizeHtml";
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
 * Pointer (in `localStorage`, so every tab can read it synchronously at
 * startup) to the last `userid` this browser resolved. `localStorage.getItem`
 * is never async — the only reason resolving the active user's bucket takes
 * a moment is that the ID token decode does. Seeding `activeUserKey` from
 * this on load lets Recents render immediately from a same-user reload/new
 * tab, self-correcting once the real `userid` comes back if it's ever stale
 * (e.g. a different account signs in). Just an id already visible to the
 * user in their own token — not sensitive on its own.
 */
const LAST_USER_KEY_STORAGE_KEY = "csm.recentViews.v1.lastUserKey";

function readLastKnownUserKey(): string | null {
  try {
    return localStorage.getItem(LAST_USER_KEY_STORAGE_KEY);
  } catch {
    return null;
  }
}

/**
 * The signed-in user's stable id (ID token `userid` claim — NOT `sub`,
 * which this IdP issues per-session rather than per-account, so it changes
 * on every fresh sign-in and can never scope anything reliably) this tab is
 * currently using to scope reads/writes — initially an optimistic guess
 * from {@link readLastKnownUserKey}, corrected once the real `userid`
 * resolves (see {@link useSyncActiveUserKey}). Scoped to this tab's JS
 * runtime; each tab seeds and (if needed) corrects it independently.
 */
let activeUserKey: string | null = readLastKnownUserKey();

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
      // Don't leave an optimistic pointer at a user who just deliberately
      // signed out — the next tab (same or a different person) should start
      // from "pending" and resolve properly, not guess at the outgoing user.
      localStorage.removeItem(LAST_USER_KEY_STORAGE_KEY);
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

function readStorageKey(key: string): RecentView[] {
  try {
    const raw = localStorage.getItem(key);
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

function readStorage(): RecentView[] {
  return readStorageKey(currentStorageKey());
}

/**
 * Folds any views recorded before identity resolved (the shared "pending"
 * bucket — see {@link currentStorageKey}) into the now-resolved user's own
 * bucket, then clears "pending". Without this, that slice of history would
 * both vanish from the user's own Recents (reads move to the new bucket)
 * and stay behind in "pending" where the next pre-resolution window — even
 * a different user's — could read it, defeating the point of scoping.
 */
function migratePendingBucket(userKey: string): void {
  try {
    const pendingKey = `${STORAGE_KEY_BASE}.pending`;
    const pending = readStorageKey(pendingKey);
    if (pending.length === 0) {
      localStorage.removeItem(pendingKey);
      return;
    }
    const targetKey = `${STORAGE_KEY_BASE}.${userKey}`;
    const target = readStorageKey(targetKey);
    // Entries already scoped to this user win on conflict.
    const merged = capUnpinned([
      ...target,
      ...pending.filter(
        (p) => !target.some((t) => t.kind === p.kind && t.id === p.id),
      ),
    ]);
    localStorage.setItem(targetKey, JSON.stringify(merged));
    localStorage.removeItem(pendingKey);
  } catch {
    /* ignore */
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

/**
 * Resolves THIS component instance's view of the signed-in user's stable id
 * (ID token `userid` claim) and syncs it into the shared `activeUserKey`, so
 * `currentStorageKey()` resolves to that user's bucket. `activeUserKey` is a
 * plain module variable — scoped to this tab's JS runtime, not shared across
 * browser tabs — so every hook below calls this itself rather than trusting
 * that some *other* component elsewhere in the tree already resolved it
 * first. Without that, a component mounted before whichever one owned the
 * sync (or the very first component in a freshly opened tab) could read the
 * still-`null`/"pending" bucket indefinitely, with the real data sitting
 * untouched under the correct per-user key.
 */
function useSyncActiveUserKey(): void {
  const userid = useIdTokenClaims()?.userid;

  useEffect(() => {
    clearLegacyUnscopedKey();
  }, []);

  useEffect(() => {
    // Still resolving (or genuinely signed out) — keep whatever guess
    // `activeUserKey` already has (the cached last-known key, or "pending"
    // if this browser has never resolved one) rather than resetting to
    // `null`/"pending" on every render before `userid` is known.
    if (!userid) return;
    // The cached guess already matched — nothing to correct, no re-render.
    if (activeUserKey === userid) return;
    // Fold in anything recorded during this session's own pre-resolution
    // window before switching the active bucket over.
    if (activeUserKey === null) {
      migratePendingBucket(userid);
    }
    activeUserKey = userid;
    try {
      localStorage.setItem(LAST_USER_KEY_STORAGE_KEY, userid);
    } catch {
      /* ignore */
    }
    // Let every `useRecentViews()` instance in this tab — including ones
    // that resolved their own `userid` earlier and are just listening — pick
    // up the now-current bucket immediately, rather than waiting for the
    // next unrelated write to trigger a re-read.
    window.dispatchEvent(new CustomEvent(STORAGE_EVENT));
  }, [userid]);
}

/**
 * Kept as a separate export so a component can make the sync-on-mount
 * ordering explicit (e.g. near the app root) — but every hook below already
 * calls {@link useSyncActiveUserKey} itself, so mounting this isn't required
 * for correctness.
 */
export function useSyncRecentViewsIdentity(): void {
  useSyncActiveUserKey();
}

/** Read-only access to the recent-views list. */
export function useRecentViews(): RecentView[] {
  useSyncActiveUserKey();
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
 * Record a visit (call from each detail page's effect). De-dupes by `kind+id`,
 * bumps the existing entry to the top, caps the list at {@link MAX_ENTRIES}.
 */
export function useRecordRecentView(): (
  entry: Omit<RecentView, "visitedAt" | "pinned">,
) => void {
  useSyncActiveUserKey();
  return useCallback((entry: Omit<RecentView, "visitedAt" | "pinned">) => {
    const now = new Date().toISOString();
    // Title/subtitle/case-hit text ultimately come from backend/customer
    // free text (case subject, account/project name, assignee name) — not
    // sanitized before render (JSX text interpolation already escapes it
    // safely), but strip tag-like markup before it's persisted so it can't
    // do anything if a future change ever renders it somewhere less safe.
    const sanitized: Omit<RecentView, "visitedAt" | "pinned"> = {
      ...entry,
      title: stripHtmlTags(entry.title),
      subtitle: entry.subtitle ? stripHtmlTags(entry.subtitle) : entry.subtitle,
      caseHit: entry.caseHit
        ? {
            ...entry.caseHit,
            subject: stripHtmlTags(entry.caseHit.subject),
            assigneeName: entry.caseHit.assigneeName
              ? stripHtmlTags(entry.caseHit.assigneeName)
              : entry.caseHit.assigneeName,
          }
        : entry.caseHit,
    };
    const current = readStorage();
    const existing = current.find(
      (e) => e.kind === sanitized.kind && e.id === sanitized.id,
    );
    const filtered = current.filter(
      (e) => !(e.kind === sanitized.kind && e.id === sanitized.id),
    );
    // Re-visiting an entry must preserve its pinned state — a pin is not lost
    // just because the case was opened again.
    const next: RecentView[] = capUnpinned([
      { ...sanitized, visitedAt: now, pinned: existing?.pinned },
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
