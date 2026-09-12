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

import { useEffect, useState } from "react";
import type { BeDashboardWidget } from "@api/backend/types";

/**
 * A dashboard the builder is editing, persisted to `localStorage` only —
 * there is no dashboard CRUD API and none is added by this feature (the
 * deployed dashboard registry is a static config file, redeployed
 * out-of-band by a maintainer). See `DashboardBuilderEditorPage`'s own doc
 * comment for the deploy story.
 */
/**
 * One `includeSections` entry: which shared section to pull in, and the
 * per-dashboard adjustments that let one section definition serve dashboards
 * that are not identical.
 *
 * Mirrors the backend's own `SectionInclude`. `extraFilters` is deliberately
 * NOT exposed by the builder yet — it is ANDed into every included
 * case-family widget's filters, which is powerful and easy to get quietly
 * wrong, and no dashboard needs it from the UI today. A definition that
 * already carries one keeps it: it round-trips through `unknown[]` untouched
 * rather than being dropped on save.
 */
export interface SectionInclude {
  /** Name of the section in the shared section catalogue. */
  section: string;
  /** Prepended to each included widget's id, so a dashboard already shipping
   * prefixed widget ids can adopt a section without renaming its widgets. */
  idPrefix?: string;
  /** Overrides the section's own heading for this dashboard only. */
  displayName?: string;
  /** Whether the section leads or trails this dashboard's own widgets. */
  position?: "start" | "end";
  /** Opaque passthrough — see this type's own doc comment. */
  extraFilters?: unknown[];
}

export interface DashboardDraft {
  /**
   * Local draft id — the storage key. For a draft opened from a deployed
   * dashboard this equals that dashboard's own `id` (so returning to
   * "Edit" on the same dashboard always resumes the same in-progress
   * draft instead of starting over); for a brand-new dashboard this is a
   * generated id (see `newDraftId`) that never collides with a real
   * deployed dashboard id.
   */
  id: string;
  /**
   * The deployed dashboard this draft was opened from, if any — used only
   * to fetch its live `GET /dashboards/{id}` for the drift check (see
   * `useDashboardDrift`). Absent for a draft that was never opened from a
   * deployed dashboard (a brand-new, not-yet-deployed dashboard).
   */
  sourceDashboardId?: string;
  displayName: string;
  type?: "cre" | "sre" | "cs";
  isDefault: boolean;
  isTeamBased: boolean;
  targetTeam?: string;
  widgets: BeDashboardWidget[];
  /**
   * Section names with no widgets in them yet — UI-only scaffolding so
   * "Add section" can create an empty section shell before any widget is
   * placed in it. A name here that later gains a widget (some
   * `widget.section === name`) becomes redundant and is dropped on the
   * next save. Never appears in the deployed dashboard JSON shape, which
   * has no first-class section concept — sections there are purely an
   * emergent grouping of `widget.section` values.
   */
  emptySections: string[];
  /**
   * Shared sections this dashboard pulls in by name, written to the deployed
   * definition's own `includeSections` (see the backend's
   * `expandIncludedSections`).
   *
   * Not merged into `widgets`: the whole value of a reference is that the
   * section's content stays in one place and every dashboard using it changes
   * together. Expanding it here would produce a copy that silently stops
   * tracking the original.
   *
   * Absent on a draft written before this field existed, hence optional.
   */
  includeSections?: SectionInclude[];
  /** ISO timestamp of the last local save, shown in the editor and used to
   * order the builder's own "local drafts" list. */
  updatedAt: string;
}

const STORAGE_KEY = "csm.dashboardBuilder.drafts.v1";
const STORAGE_EVENT = "csm:dashboard-drafts-changed";

// Every REQUIRED `DashboardDraft` field is checked here — a record missing
// any of them (e.g. hand-edited in devtools, or written by an older/newer
// version of this feature with a different shape) is dropped by
// `readDraftsMap` rather than accepted and handed back to a caller that
// assumes the full shape. `updatedAt` in particular: `listDashboardDrafts`
// sorts on `a.updatedAt.localeCompare(b.updatedAt)`, which throws on
// `undefined` — a record missing it used to crash that sort outright rather
// than being silently ignored.
function isDraft(v: unknown): v is DashboardDraft {
  if (typeof v !== "object" || v === null) return false;
  const d = v as DashboardDraft;
  return (
    typeof d.id === "string" &&
    typeof d.displayName === "string" &&
    typeof d.isDefault === "boolean" &&
    typeof d.isTeamBased === "boolean" &&
    Array.isArray(d.widgets) &&
    Array.isArray(d.emptySections) &&
    typeof d.updatedAt === "string"
  );
}

function readDraftsMap(): Record<string, DashboardDraft> {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return {};
    const parsed: unknown = JSON.parse(raw);
    if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) return {};
    const out: Record<string, DashboardDraft> = {};
    for (const [id, value] of Object.entries(parsed as Record<string, unknown>)) {
      if (isDraft(value)) out[id] = value;
    }
    return out;
  } catch {
    return {};
  }
}

function writeDraftsMap(map: Record<string, DashboardDraft>): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(map));
    // In-tab listeners (the native `storage` event only fires cross-tab).
    window.dispatchEvent(new CustomEvent(STORAGE_EVENT));
  } catch {
    // ignore quota / serialization errors — the in-memory draft the editor
    // is holding is unaffected, only persistence silently fails.
  }
}

/** A new, never-colliding draft id for a brand-new (not-yet-deployed)
 * dashboard. */
export function newDraftId(): string {
  return `draft-${crypto.randomUUID()}`;
}

/** A new, never-colliding widget id, for a widget created in the builder. */
export function newWidgetId(): string {
  return `widget-${crypto.randomUUID()}`;
}

/** Reads a single draft by id, or `undefined` if none is saved. */
export function getDashboardDraft(id: string): DashboardDraft | undefined {
  return readDraftsMap()[id];
}

/** Every locally saved draft, most recently updated first. */
export function listDashboardDrafts(): DashboardDraft[] {
  return Object.values(readDraftsMap()).sort((a, b) => b.updatedAt.localeCompare(a.updatedAt));
}

/** Saves (creates or overwrites) a draft, stamping `updatedAt` to now. */
export function saveDashboardDraft(draft: Omit<DashboardDraft, "updatedAt">): DashboardDraft {
  const stamped: DashboardDraft = { ...draft, updatedAt: new Date().toISOString() };
  const map = readDraftsMap();
  map[draft.id] = stamped;
  writeDraftsMap(map);
  return stamped;
}

/** Deletes a draft by id. No-op if it doesn't exist. */
export function deleteDashboardDraft(id: string): void {
  const map = readDraftsMap();
  if (!(id in map)) return;
  delete map[id];
  writeDraftsMap(map);
}

/** Reactive list of every locally saved draft — updates across components
 * and browser tabs. Mirrors `useSavedFilterViews`'s own storage-event
 * idiom (see `savedFilterViews.ts`). */
export function useDashboardDrafts(): DashboardDraft[] {
  const [drafts, setDrafts] = useState<DashboardDraft[]>(() => listDashboardDrafts());
  useEffect(() => {
    const sync = () => setDrafts(listDashboardDrafts());
    window.addEventListener(STORAGE_EVENT, sync);
    window.addEventListener("storage", sync);
    return () => {
      window.removeEventListener(STORAGE_EVENT, sync);
      window.removeEventListener("storage", sync);
    };
  }, []);
  return drafts;
}

/** Reactive single draft, kept in sync across components/tabs the same way
 * `useDashboardDrafts` is — used by the editor page so its own `saveDraft`
 * elsewhere (or another tab's edit of the same draft id) is reflected
 * without a manual re-read. */
export function useDashboardDraft(id: string | undefined): DashboardDraft | undefined {
  const [draft, setDraft] = useState<DashboardDraft | undefined>(() =>
    id ? getDashboardDraft(id) : undefined,
  );
  useEffect(() => {
    const sync = () => setDraft(id ? getDashboardDraft(id) : undefined);
    sync();
    window.addEventListener(STORAGE_EVENT, sync);
    window.addEventListener("storage", sync);
    return () => {
      window.removeEventListener(STORAGE_EVENT, sync);
      window.removeEventListener("storage", sync);
    };
  }, [id]);
  return draft;
}
