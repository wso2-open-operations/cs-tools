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

export type RecentViewKind = "case" | "project" | "account";

export interface RecentView {
  kind: RecentViewKind;
  id: string;
  title: string;
  subtitle?: string;
  href: string;
  /** ISO timestamp of the last visit. */
  visitedAt: string;
}

const STORAGE_KEY = "csm.recentViews.v1";
const MAX_ENTRIES = 12;
const STORAGE_EVENT = "csm:recent-views-changed";

function readStorage(): RecentView[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    const parsed: unknown = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    // Light shape guard — drop anything that doesn't look like a RecentView.
    return parsed.filter(
      (v): v is RecentView =>
        typeof v === "object" &&
        v !== null &&
        typeof (v as RecentView).id === "string" &&
        typeof (v as RecentView).kind === "string" &&
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
    localStorage.setItem(STORAGE_KEY, JSON.stringify(entries));
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
 * Record a visit (call from each detail page's effect). De-dupes by `kind+id`,
 * bumps the existing entry to the top, caps the list at {@link MAX_ENTRIES}.
 */
export function useRecordRecentView(): (
  entry: Omit<RecentView, "visitedAt">,
) => void {
  return useCallback((entry: Omit<RecentView, "visitedAt">) => {
    const now = new Date().toISOString();
    const current = readStorage();
    const filtered = current.filter(
      (e) => !(e.kind === entry.kind && e.id === entry.id),
    );
    const next: RecentView[] = [
      { ...entry, visitedAt: now },
      ...filtered,
    ].slice(0, MAX_ENTRIES);
    writeStorage(next);
  }, []);
}

export function clearRecentViews(): void {
  writeStorage([]);
}
