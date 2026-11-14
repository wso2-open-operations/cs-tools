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

/**
 * Module-level batching ("dataloader") for resolving a user's canonical id
 * from their email via `POST /users/search`. A case-detail page can carry
 * dozens of actors (comments, attachments, activity entries, watchers) with
 * `id: null` on their {@link UserReference} — one per hook consumer would be
 * an N+1 request storm. Instead, every email requested within a short window
 * is collected here and resolved with a single search call, then the
 * per-email result is handed back to each caller's own `useQuery` (see
 * `useResolvedUserId`), so react-query's own per-key cache still dedupes and
 * reuses the mapping across the rest of the session.
 *
 * `/users/search`'s `emails` filter caps at 50 entries, so a batch larger
 * than that is split into chunks — vanishingly unlikely on a single page, but
 * cheap to guard against.
 */

const MAX_EMAILS_PER_SEARCH = 50;
// Flushing on a macrotask (not a microtask) gives every widget mounted in the
// same commit — and any that mount a tick or two later, e.g. a lazily loaded
// attachments list — a chance to join the same batch, at the cost of a few
// milliseconds of latency before the first names become links.
const FLUSH_DELAY_MS = 10;

/** What one email resolved to: the user's id, and whether they are an
 * internal user (WSO2 staff). */
export interface ResolvedUser {
  id: string;
  isInternal: boolean;
}

/** Resolves a batch of emails to `email (lowercased) -> user` pairs. */
export type EmailBatchFetcher = (
  emails: string[],
) => Promise<Map<string, ResolvedUser>>;

interface PendingEntry {
  resolve: (user: ResolvedUser | null) => void;
  reject: (err: unknown) => void;
}

let pending = new Map<string, PendingEntry[]>();
let flushTimer: ReturnType<typeof setTimeout> | undefined;
let flushFetcher: EmailBatchFetcher | undefined;

function chunk<T>(items: T[], size: number): T[][] {
  const out: T[][] = [];
  for (let i = 0; i < items.length; i += size) {
    out.push(items.slice(i, i + size));
  }
  return out;
}

async function flush(): Promise<void> {
  const batch = pending;
  const fetcher = flushFetcher;
  pending = new Map();
  flushTimer = undefined;
  flushFetcher = undefined;
  if (batch.size === 0 || !fetcher) return;

  const emails = Array.from(batch.keys());
  try {
    const resolved = new Map<string, ResolvedUser>();
    for (const group of chunk(emails, MAX_EMAILS_PER_SEARCH)) {
      const found = await fetcher(group);
      for (const [email, user] of found) resolved.set(email, user);
    }
    for (const [email, entries] of batch) {
      const user = resolved.get(email) ?? null;
      entries.forEach((e) => e.resolve(user));
    }
  } catch (err) {
    // A failed lookup must never surface as an error to the caller — it
    // degrades to plain text (see UserRefLink) — but it also must not be
    // cached as a permanent "not found", so reject rather than resolve(null).
    // react-query's own retry/backoff governs whether this email is tried
    // again; the query is never left in a cached-success state.
    for (const entries of batch.values()) {
      entries.forEach((e) => e.reject(err));
    }
  }
}

/**
 * Queues `email` for resolution in the next batch and returns a promise for
 * its user id (`null` when the search comes back with no match — a
 * confirmed-empty result, safe to cache as a negative). `fetcher` performs
 * the actual network call; the same one should be passed by every caller in
 * a given session (they all resolve to an equivalent authenticated client).
 */
export function requestUserIdByEmail(
  email: string,
  fetcher: EmailBatchFetcher,
): Promise<ResolvedUser | null> {
  return new Promise((resolve, reject) => {
    const key = email.trim().toLowerCase();
    const entries = pending.get(key) ?? [];
    entries.push({ resolve, reject });
    pending.set(key, entries);
    flushFetcher = fetcher;
    if (!flushTimer) {
      flushTimer = setTimeout(() => {
        void flush();
      }, FLUSH_DELAY_MS);
    }
  });
}

/** Test-only: force any pending batch to resolve immediately. */
export function __flushPendingUserIdResolutionsForTest(): Promise<void> {
  if (flushTimer) clearTimeout(flushTimer);
  flushTimer = undefined;
  return flush();
}
