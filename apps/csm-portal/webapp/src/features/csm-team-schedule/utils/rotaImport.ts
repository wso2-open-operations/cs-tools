/**
 * Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import type { RotaImportFile } from "../api/useImportRota";
import type { ScheduleCatalogue, ScheduleTier } from "../types";

/** The standard structure, offered as a template when the catalogue has not loaded. */
export const ROTA_IMPORT_TEMPLATE = {
  schemaVersion: 1,
  family: "SRE",
  rotaCode: "SRE_SAAS",
  period: { from: "2026-11-01", to: "2026-11-30" },
  // Merge, the mode that removes nothing: the template is what people edit and send back.
  mode: "merge",
  assignments: [
    { email: "user1@example.com", teamKey: "apollo", shiftCode: "SRE_TZ1_L1", from: "2026-11-02", to: "2026-11-06" },
    { email: "user2@example.com", teamKey: "apollo", shiftCode: "SRE_TZ1", tier: "L2", from: "2026-11-02", to: "2026-11-06" },
    { email: "user3@example.com", teamKey: "artemis", shiftCode: "SRE_TZ3", tier: "L1", from: "2026-11-02", to: "2026-11-08" },
    { email: "user4@example.com", teamKey: "apollo", shiftCode: "SRE_WE_TZ1", tier: "L1", from: "2026-11-07", to: "2026-11-08" },
  ],
  absences: [
    { email: "user2@example.com", teamKey: "apollo", kindCode: "ANNUAL_LEAVE", from: "2026-11-16", to: "2026-11-20" },
  ],
};

const TIERS: ScheduleTier[] = ["L1", "L2", "L3"];

function isoDay(d: Date): string {
  return d.toISOString().slice(0, 10);
}

/**
 * A template for one rota built from the catalogue: every window of the rota's zones (each tier where the window leaves it open)
 * and every kind of time away offered on its family, so it lists every code the roster can hold, in merge mode.
 * The emails are placeholders to replace with team members'. Assignments go on the rota's own teams, never a team a move sends people to,
 * and no two take one tier of a zone on the same day, so once the emails are real the entries do not refuse one another.
 */
export function buildRotaImportTemplate(rotaCode: string, catalogue: ScheduleCatalogue | undefined, today = new Date()): object {
  const rota = catalogue?.rotas?.find((r) => r.code === rotaCode);
  if (!catalogue || !rota) return ROTA_IMPORT_TEMPLATE;
  const rotaTeams = catalogue.teams.filter((t) => t.rotaCode === rotaCode).map((t) => t.key);
  const zones = new Set(catalogue.zones.filter((z) => z.rotaCode === rotaCode).map((z) => z.code));
  const kinds = catalogue.absenceKinds
    .filter((k) => !k.retired && (!k.family || k.family === rota.family))
    .sort((a, b) => a.sortOrder - b.sortOrder);
  const moveTargets = new Set(kinds.map((k) => k.movesToTeamKey).filter((k): k is string => Boolean(k)));
  const homeTeams = rotaTeams.filter((t) => !moveTargets.has(t));
  const teams = homeTeams.length > 0 ? homeTeams : rotaTeams;
  if (teams.length === 0) return ROTA_IMPORT_TEMPLATE;

  const first = new Date(Date.UTC(today.getUTCFullYear(), today.getUTCMonth() + 1, 1));
  const last = new Date(Date.UTC(first.getUTCFullYear(), first.getUTCMonth() + 1, 0));
  const days: Date[] = [];
  for (let d = new Date(first); d <= last; d = new Date(d.getTime() + 86_400_000)) days.push(d);
  const isWeekend = (d: Date): boolean => d.getUTCDay() === 0 || d.getUTCDay() === 6;
  const fits = (scope: "WEEKDAY" | "WEEKEND" | "ANY", d: Date): boolean =>
    scope === "ANY" || (scope === "WEEKEND") === isWeekend(d);

  // Days already taken per tier slot (zone and tier), so two entries never hold one slot on one day.
  const taken = new Map<string, Set<string>>();
  let cursor = 0;
  const dayFor = (scope: "WEEKDAY" | "WEEKEND" | "ANY", slot?: string): string => {
    const used = slot ? (taken.get(slot) ?? new Set<string>()) : new Set<string>();
    for (let i = 0; i < days.length; i++) {
      const d = days[(cursor + i) % days.length];
      const iso = isoDay(d);
      if (fits(scope, d) && !used.has(iso)) {
        cursor = (cursor + i + 1) % days.length;
        if (slot) taken.set(slot, used.add(iso));
        return iso;
      }
    }
    return isoDay(first);
  };

  const shifts = catalogue.shifts
    .filter((s) => s.family === rota.family && s.zoneCode !== undefined && zones.has(s.zoneCode))
    .sort((a, b) => a.sortOrder - b.sortOrder);
  const assignments: object[] = [];
  for (const s of shifts) {
    const tiers = s.isEscalation && !s.tier ? TIERS : [s.tier];
    for (const tier of tiers) {
      const slot = s.isEscalation && tier ? `${s.zoneCode}:${tier}` : undefined;
      const day = dayFor(s.dayScope, slot);
      const n = assignments.length + 1;
      assignments.push({
        email: `person${n}@example.com`,
        teamKey: teams[(n - 1) % teams.length],
        shiftCode: s.code,
        ...(tier && !s.tier ? { tier } : {}),
        from: day,
        to: day,
        ...(n === 1 ? { note: "optional" } : {}),
      });
    }
  }

  const absences = kinds.map((k, i) => {
    const day = dayFor("ANY");
    return {
      email: `away${i + 1}@example.com`,
      teamKey: teams[0],
      kindCode: k.code,
      ...(k.bucket === "ALLOCATION" ? { allocatedTo: "Example Customer" } : {}),
      from: day,
      to: day,
    };
  });

  return {
    schemaVersion: 1,
    family: rota.family,
    rotaCode,
    period: { from: isoDay(first), to: isoDay(last) },
    mode: "merge",
    assignments,
    absences,
  };
}

export type ParsedRotaImport = { file: RotaImportFile; error?: undefined } | { file?: undefined; error: string };

/** Reads an import file's text and checks the shape the server needs before anything is sent, and that it is the rota on screen. */
export function parseRotaImportFile(text: string, rotaCode: string): ParsedRotaImport {
  let data: unknown;
  try {
    data = JSON.parse(text);
  } catch {
    return { error: "The file is not valid JSON." };
  }
  if (typeof data !== "object" || data === null || Array.isArray(data)) {
    return { error: "The file must hold one JSON object." };
  }
  const f = data as Record<string, unknown>;
  const period = f.period as Record<string, unknown> | undefined;
  if (f.schemaVersion !== 1) return { error: "schemaVersion must be 1." };
  if (typeof f.rotaCode !== "string" || f.rotaCode.trim() === "") return { error: "rotaCode is missing." };
  if (f.rotaCode.trim().toUpperCase() !== rotaCode.toUpperCase()) {
    return { error: `This file is for ${f.rotaCode}, but the roster shows ${rotaCode}. Switch the rota first.` };
  }
  if (typeof period !== "object" || period === null || typeof period.from !== "string" || typeof period.to !== "string") {
    return { error: "period must have from and to dates (YYYY-MM-DD)." };
  }
  if (f.mode !== "replace" && f.mode !== "merge") return { error: 'mode must be "replace" or "merge".' };
  for (const key of ["assignments", "absences"] as const) {
    if (f[key] !== undefined && !Array.isArray(f[key])) return { error: `${key} must be a list.` };
  }
  const entries = ((f.assignments as unknown[]) ?? []).length + ((f.absences as unknown[]) ?? []).length;
  if (entries === 0) return { error: "The file has no assignments and no absences." };
  return { file: data as RotaImportFile };
}

/** Whether a reader may be offered "Import rota" for a rota: an SRE rota with one of its teams in the teams they may edit; the server checks every entry again. */
export function rotaImportOffered(
  rotaCode: string | undefined,
  rotas: readonly { code: string; family: string }[],
  teams: readonly { key: string; rotaCode?: string | null }[],
  editableTeams: readonly string[],
): boolean {
  if (!rotaCode || !rotas.some((r) => r.code === rotaCode && r.family === "SRE")) return false;
  const editable = new Set(editableTeams.map((k) => k.toLowerCase()));
  return teams.some((t) => t.rotaCode === rotaCode && editable.has(t.key.toLowerCase()));
}
