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

import type {
  CaseAlertChannel,
  CaseAlertFileOnly,
  CaseAlertLadder,
  CaseAlertRoomLadder,
  CaseAlertSettings,
  PagingChainCode,
} from "../types";

/** The ladder whose settings each chain shows. SME has none here. */
export const CHAIN_LADDER: Partial<Record<PagingChainCode, CaseAlertLadder>> = {
  CRE: "CRE",
  SRE_SAAS: "SRE",
  SRE_IAAS: "SRE",
  SRE_U2: "SRE",
  SRE_B_CENTRAL: "SRE",
};

/**
 * The ladder values that can be saved on the Alert Settings tab: the rest of the
 * paging file's cre: and sre: keys, by the file's own name. entity-service
 * holds the same list and checks every value; the limits here are only hints
 * for the inputs.
 */

export type LadderKeyGroup =
  | "On/off and channel"
  | "Who gets alerts"
  | "Call limits"
  | "When a ladder starts"
  | "How it climbs"
  | "Heads"
  | "Case Alert email"
  | "Chat messages";

export const LADDER_KEY_GROUPS: LadderKeyGroup[] = [
  "On/off and channel",
  "Who gets alerts",
  "Call limits",
  "When a ladder starts",
  "How it climbs",
  "Heads",
  "Case Alert email",
  "Chat messages",
];

/** How a value is edited: a checkbox, a number, a pick from a few, a line of
 *  text, a comma-separated list, SHIFT=n pairs, key=team pairs, key=address
 *  pairs, a time like 5m, or a head's name and email. */
export type LadderKeyEditor =
  | "bool"
  | "int"
  | "enum"
  | "text"
  | "list"
  | "shiftCounts"
  | "teamMap"
  | "teamEmails"
  | "duration"
  | "head";

export interface LadderKeySpec {
  key: string;
  ladders: CaseAlertRoomLadder[];
  group: LadderKeyGroup;
  label: string;
  help: string;
  editor: LadderKeyEditor;
  /** For a number, its range; for a list, how many items; for SHIFT=n
   *  pairs, the most for one shift. */
  min?: number;
  max?: number;
  options?: { value: string; label: string }[];
  /** What an empty list, text or map means, for people. */
  empty?: string;
  /** What 0 means, for a number where it is not "nobody". */
  zero?: string;
  placeholder?: string;
  /** A text value's field: its label (else "New value") and input type. */
  inputLabel?: string;
  inputType?: "email";
}

const BOTH: CaseAlertRoomLadder[] = ["CRE", "SRE"];
const CRE: CaseAlertRoomLadder[] = ["CRE"];
const SRE: CaseAlertRoomLadder[] = ["SRE"];
const ALL: CaseAlertRoomLadder[] = ["CRE", "SRE", "SME"];
const SRE_SME: CaseAlertRoomLadder[] = ["SRE", "SME"];
const SME: CaseAlertRoomLadder[] = ["SME"];

/** The channels a ladder can use, most to least. */
const CHANNEL_OPTIONS = [
  { value: "both", label: "Calls and chat" },
  { value: "chat", label: "Chat only" },
  { value: "call", label: "Calls only" },
  { value: "log", label: "Log only (no calls or chat)" },
];

export const LADDER_KEYS: LadderKeySpec[] = [
  // On/off and channel: a switch and a channel, nothing else. The alert
  // service's file keeps a master switch and an emergency off switch outside
  // this page. The call limits that sat here are under Advanced.
  {
    key: "enabled",
    ladders: ALL,
    group: "On/off and channel",
    label: "Alerts on",
    help: "Whether this ladder pages anyone at all. The alert service's own master switch must be on too.",
    editor: "bool",
  },
  {
    key: "channel",
    ladders: ALL,
    group: "On/off and channel",
    label: "Channel",
    help: "How people are reached. Calls cost money and wake people; chat only posts cards. A team's own channel can only narrow this.",
    editor: "enum",
    options: CHANNEL_OPTIONS,
  },
  {
    key: "safety.maxCallsPerLadder",
    ladders: BOTH,
    group: "Call limits",
    label: "Max calls per ladder",
    help: "The most calls one incident or case may place. Extra calls are dropped and noted.",
    editor: "int",
    min: 1,
    max: 50,
  },
  {
    key: "safety.callWithoutVerifiedLeads",
    ladders: BOTH,
    group: "Call limits",
    label: "Call before every lead has a number",
    help: "Off, no calls are placed while any ABT lead has no phone number. On, calls go out anyway.",
    editor: "bool",
  },
  {
    key: "safety.callHeadsWithoutLowerTiers",
    ladders: BOTH,
    group: "Call limits",
    label: "Call the heads when nobody below can be called",
    help: "Off, the top steps are held when nobody on the lower steps has a number. On, they are called anyway.",
    editor: "bool",
  },

  // Who is paged
  {
    key: "teams.abtType",
    ladders: BOTH,
    group: "Who gets alerts",
    label: "ABT team type",
    help: "The team type the ABT teams have, e.g. cre-abt or sre-abt.",
    editor: "text",
    placeholder: "e.g. cre-abt",
  },
  {
    key: "teams.abts",
    ladders: BOTH,
    group: "Who gets alerts",
    label: "ABT teams",
    help: "The teams this ladder pages, by Team Schedule key. Set, it wins over the team type.",
    editor: "list",
    max: 30,
    empty: "none (uses the team type)",
    placeholder: "e.g. atlas, castor",
  },
  {
    key: "teams.americas",
    ladders: CRE,
    group: "Who gets alerts",
    label: "Americas team",
    help: "The CRE team that covers the Americas night, by Team Schedule key.",
    editor: "text",
    placeholder: "e.g. americas",
  },
  {
    key: "teams.teamLeads",
    ladders: CRE,
    group: "Who gets alerts",
    label: "Team lead pool",
    help: "The teams whose leads can be called at the team lead step.",
    editor: "list",
    max: 30,
    empty: "every ABT team",
    placeholder: "e.g. atlas, castor",
  },
  {
    key: "teams.rotaTeams",
    ladders: CRE,
    group: "Who gets alerts",
    label: "Rota teams",
    help: "The teams whose people on the rota can be called.",
    editor: "list",
    max: 30,
    empty: "the ABT teams and Americas",
    placeholder: "e.g. atlas, americas",
  },
  {
    key: "teams.americasWeekendRotaShifts",
    ladders: CRE,
    group: "Who gets alerts",
    label: "Americas weekend shifts",
    help: "The Team Schedule shift codes the Americas weekend rota member comes from.",
    editor: "list",
    max: 10,
    empty: "CRE_WEEKEND_NIGHT",
    placeholder: "e.g. CRE_WEEKEND_NIGHT",
  },
  {
    key: "teams.leadership",
    ladders: CRE,
    group: "Who gets alerts",
    label: "Leadership team",
    help: "Where a head is looked up when the head is not named below.",
    editor: "text",
    placeholder: "e.g. cre-leadership",
  },
  {
    key: "teams",
    ladders: SME,
    group: "Who gets alerts",
    label: "Special Ops teams",
    help: "Which SME team a Special Ops alert pages, when the alert does not name the SME team itself.",
    editor: "teamMap",
    max: 50,
    empty: "none (the alert names the SME team)",
    placeholder: "e.g. asgardeo-ops=asgardeo",
  },
  {
    key: "teams.defaultRota",
    ladders: SRE,
    group: "Who gets alerts",
    label: "Default rota",
    help: "The SRE rota that pages an incident with no SRE team of its own.",
    editor: "text",
    placeholder: "e.g. SRE_SAAS",
  },

  // When a ladder starts
  {
    key: "trigger.priorities",
    ladders: BOTH,
    group: "When a ladder starts",
    label: "Priorities",
    help: "Only these priorities start the ladder, S0 to S4.",
    editor: "list",
    max: 5,
    empty: "any",
    placeholder: "e.g. S0, S1, S2",
  },
  {
    key: "trigger.teams",
    ladders: BOTH,
    group: "When a ladder starts",
    label: "Only these teams",
    help: "Only cases or incidents assigned to these CSM Portal teams start the ladder, by team name.",
    editor: "list",
    max: 100,
    empty: "any",
  },
  {
    key: "trigger.excludeTeams",
    ladders: BOTH,
    group: "When a ladder starts",
    label: "Never these teams",
    help: "Cases or incidents assigned to these CSM Portal teams never start the ladder, by team name. This wins over the list above.",
    editor: "list",
    max: 100,
    empty: "none",
  },
  {
    key: "trigger.shifts",
    ladders: BOTH,
    group: "When a ladder starts",
    label: "Shifts",
    help: "Only start in these shifts: LK, LK_MORNING, LK_EVENING, LK_WEEKEND, USA, USA_WEEKEND.",
    editor: "list",
    max: 6,
    empty: "any",
    placeholder: "e.g. LK, LK_MORNING",
  },
  {
    key: "trigger.requireKnownTeam",
    ladders: BOTH,
    group: "When a ladder starts",
    label: "Needs a known team",
    help: "Don't start when the team can't be worked out, rather than guess.",
    editor: "bool",
  },

  // How it climbs
  {
    key: "alertDuty.tiers",
    ladders: CRE,
    group: "How it climbs",
    label: "Alert duty tiers",
    help: "The alert duty nominations in use, T1 to T9.",
    editor: "list",
    max: 9,
    empty: "none",
    placeholder: "e.g. T1, T2, T3",
  },
  {
    key: "alertDuty.perTeam",
    ladders: CRE,
    group: "How it climbs",
    label: "Alert duty per team",
    help: "How many nominees a step takes from each team.",
    editor: "int",
    min: 0,
    max: 10,
    zero: "all of them",
  },
  {
    key: "teams.teamLeadsToCall",
    ladders: CRE,
    group: "How it climbs",
    label: "Team leads to call",
    help: "How many leads the team leads step calls from the pool.",
    editor: "int",
    min: 0,
    max: 20,
    zero: "all of them",
  },
  {
    key: "teams.rotaMembersToCall",
    ladders: CRE,
    group: "How it climbs",
    label: "Rota members to call",
    help: "A cap on how many people on the rota are called, per shift. Name only the shifts to cap.",
    editor: "shiftCounts",
    max: 50,
    empty: "no cap",
    placeholder: "e.g. LK_EVENING=7",
  },
  {
    key: "teams.unassignedTeamLead",
    ladders: CRE,
    group: "How it climbs",
    label: "Team lead for a case with no team",
    help: "Call a lead from the pool, or call nobody and climb.",
    editor: "enum",
    options: [
      { value: "pool", label: "A lead from the pool" },
      { value: "none", label: "Nobody" },
    ],
  },
  {
    key: "teams.unassignedTeamLeadCount",
    ladders: CRE,
    group: "How it climbs",
    label: "Leads for a case with no team",
    help: "How many leads from the pool that step calls.",
    editor: "int",
    min: 0,
    max: 10,
  },
  {
    key: "onLeave",
    ladders: ALL,
    group: "How it climbs",
    label: "People on leave",
    help: "Skip people on leave that day, or still call them after everyone else on the step.",
    editor: "enum",
    options: [
      { value: "skip", label: "Skip them" },
      { value: "call", label: "Call them" },
    ],
  },
  {
    key: "acknowledgement.requireBoth",
    ladders: CRE,
    group: "How it climbs",
    label: "Needs both to stop",
    help: "Stop only when the case has left New and has a public comment. Off, either one stops it.",
    editor: "bool",
  },
  {
    key: "timing.initialWait",
    ladders: SRE_SME,
    group: "How it climbs",
    label: "Wait before the first call",
    help: "From 0s to 30m, written like 0s or 2m.",
    editor: "duration",
    placeholder: "e.g. 0s",
  },
  {
    key: "timing.interval",
    ladders: SRE_SME,
    group: "How it climbs",
    label: "Time between steps",
    help: "From 1m to 30m, written like 5m.",
    editor: "duration",
    placeholder: "e.g. 5m",
  },
  {
    key: "timing.includeL4",
    ladders: SRE,
    group: "How it climbs",
    label: "Call L4",
    help: "After L3, also call the lead of the SRE team on the incident.",
    editor: "bool",
  },

  // Heads
  {
    key: "heads.cre",
    ladders: CRE,
    group: "Heads",
    label: "Head of CRE",
    help: "Called on the last steps. Their phone number comes from their profile.",
    editor: "head",
    empty: "not named (looked up in the leadership team)",
  },
  {
    key: "heads.cs",
    ladders: CRE,
    group: "Heads",
    label: "Head of CS",
    help: "Called on the last step. Their phone number comes from their profile.",
    editor: "head",
    empty: "not named (looked up in the leadership team)",
  },

  // Case Alert email: the Case Alert Notification sent at L1 to L4 of a
  // customer case's CRE chain. The lists are mailing addresses, Cc'd only.
  {
    key: "email.enabled",
    ladders: CRE,
    group: "Case Alert email",
    label: "Email per step",
    help: "Send the Case Alert Notification email at L1 to L4 of a case's chain (L0 gets none), to the step's team leads or head, Cc the team lists below.",
    editor: "bool",
  },
  {
    key: "email.teamLists",
    ladders: CRE,
    group: "Case Alert email",
    label: "ABT team lists (Cc on L1 and L2)",
    help: "Each team's mailing list is Cc'd on the email to its team lead (L1) and on the email to every ABT team lead (L2). The Americas list is Cc'd at night.",
    editor: "teamEmails",
    max: 30,
    empty: "none (no Cc on L1 and L2)",
    placeholder: "team-list@example.com",
  },
  {
    key: "email.creTeamList",
    ladders: CRE,
    group: "Case Alert email",
    label: "CRE team list (Cc on L3)",
    help: "Cc'd on the L3 email to the Head of CRE.",
    editor: "text",
    empty: "none (no Cc on L3)",
    placeholder: "cre-team@example.com",
    inputLabel: "Mailing list",
    inputType: "email",
  },
  {
    key: "email.csTeamList",
    ladders: CRE,
    group: "Case Alert email",
    label: "CS team list (Cc on L4)",
    help: "Cc'd on the L4 email to the Head of CS.",
    editor: "text",
    empty: "none (no Cc on L4)",
    placeholder: "cs-team@example.com",
    inputLabel: "Mailing list",
    inputType: "email",
  },

  // Chat messages
  {
    key: "unansweredChat",
    ladders: SRE_SME,
    group: "Chat messages",
    label: "Say when nobody answered",
    help: "After L1 to L3 were all called and nobody acknowledged, post that the calls have ended.",
    editor: "bool",
  },
  {
    key: "smeHandoffChat",
    ladders: SRE,
    group: "Chat messages",
    label: "Say when handed to SMEs",
    help: "When an incident goes to an SME team, post that SRE alerts stopped and the SMEs are being alerted.",
    editor: "bool",
  },
];

export function ladderKeysFor(ladder: CaseAlertRoomLadder): LadderKeySpec[] {
  return LADDER_KEYS.filter((k) => k.ladders.includes(ladder));
}

/** Something that reads like a link is never shown: no value is one, and a
 *  webhook URL must never reach the page. */
function safe(text: string): string {
  return /:\/\//.test(text) ? "a link (not shown)" : text;
}

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}

/** A head as the file and the saved value have it. A phone number in the
 *  file is not shown or copied: here it comes from the person's profile. */
function headOf(v: unknown): { name: string; email: string } | null {
  if (!isRecord(v)) return null;
  return {
    name: typeof v.name === "string" ? v.name.trim() : "",
    email: typeof v.email === "string" ? v.email.trim() : "",
  };
}

/** A value as people read it: lists comma-separated, Yes or No, a head as
 *  "Name <email>", and an empty one said in words. */
export function formatLadderValue(spec: LadderKeySpec | undefined, value: unknown): string {
  if (value === undefined || value === null) return "—";
  const empty = spec?.empty ?? "none";
  if (typeof value === "boolean") return value ? "Yes" : "No";
  if (typeof value === "number") return value === 0 && spec?.zero ? `0 (${spec.zero})` : String(value);
  if (typeof value === "string") {
    if (value.trim() === "") return empty;
    return safe(spec?.options?.find((o) => o.value === value)?.label ?? value);
  }
  if (Array.isArray(value)) return value.length === 0 ? empty : value.map((v) => safe(String(v))).join(", ");
  if (spec?.editor === "head") {
    const h = headOf(value);
    if (!h || (!h.name && !h.email)) return empty;
    return safe(h.name && h.email ? `${h.name} <${h.email}>` : h.email || h.name);
  }
  if (isRecord(value)) {
    const entries = Object.entries(value);
    return entries.length === 0 ? empty : entries.map(([k, v]) => safe(`${k}=${String(v)}`)).join(", ");
  }
  return safe(JSON.stringify(value));
}

/** What an inline editor holds while it is open. */
export interface LadderDraft {
  text: string;
  checked: boolean;
  name: string;
  email: string;
  /** Team key to address, for the team email lists. */
  map: Record<string, string>;
}

/** One team in the team email lists' editor. */
export interface TeamEmailRow {
  key: string;
  name: string;
  /** Said under the name: when the list is used, or that it is not. */
  note?: string;
  /** A team the value names that is no longer an ABT team. */
  stale?: boolean;
}

const AMERICAS_NOTE = "Cc'd at night (21:00-06:00 IST)";
const STALE_NOTE = "No longer an ABT team: this list is not used";

function titleCase(key: string): string {
  return key.replace(/(^|[-_.\s])(\w)/g, (_, sep: string, c: string) => `${sep === "" ? "" : " "}${c.toUpperCase()}`);
}

/** The teams whose mailing list the Case Alert email can Cc: the ladder's
 *  ABT teams and its Americas team, as saved here or in use, named as the
 *  Team Schedule names them. With no ABT list known, every team of the
 *  ladder's family is offered. The alert service drops a list for any other
 *  team, so a value naming one is shown marked (teamEmailRowsFor). */
export function teamEmailRows(s: CaseAlertSettings, ladder: CaseAlertRoomLadder): TeamEmailRow[] {
  const inUse = (key: string): unknown =>
    s.values.find((v) => v.ladder === ladder && v.key === key)?.value ?? s.running?.ladders[ladder]?.[key]?.value;
  const names = new Map(
    (s.routeOptions?.teams ?? []).filter((t) => t.family === ladder).map((t) => [t.key.toLowerCase(), t.name] as const),
  );
  const abtValue = inUse("teams.abts");
  const abts = (Array.isArray(abtValue) ? abtValue : [])
    .filter((k): k is string => typeof k === "string" && k.trim() !== "")
    .map((k) => k.trim().toLowerCase());
  const americasValue = inUse("teams.americas");
  const americas = typeof americasValue === "string" ? americasValue.trim().toLowerCase() : "";
  const keys = abts.length > 0 ? abts : [...names.keys()].filter((k) => k !== americas);
  const rows: TeamEmailRow[] = [];
  const seen = new Set<string>();
  for (const key of keys) {
    if (seen.has(key)) continue;
    seen.add(key);
    rows.push({ key, name: names.get(key) ?? titleCase(key) });
  }
  if (americas && !seen.has(americas)) {
    rows.push({ key: americas, name: names.get(americas) ?? titleCase(americas), note: AMERICAS_NOTE });
  }
  return rows;
}

/** The rows, plus any team the value names that they don't, marked: the
 *  alert service drops its list, and leaving it hidden would keep it saved
 *  with nobody able to see or clear it. */
export function teamEmailRowsFor(rows: TeamEmailRow[], value: unknown): TeamEmailRow[] {
  if (!isRecord(value)) return rows;
  const known = new Set(rows.map((r) => r.key));
  const extra = Object.keys(value)
    .map((k) => k.trim().toLowerCase())
    .filter((k) => k !== "" && !known.has(k))
    .map((key) => ({ key, name: titleCase(key), note: STALE_NOTE, stale: true }));
  return [...rows, ...extra];
}

/** One bare address, as the alert service and the entity service take it. */
export function isMailingAddress(v: string): boolean {
  return /^[^\s@,;<>"]+@[^\s@,;<>"]+$/.test(v);
}

/** The team lists' fields that do not hold one address, by team. */
export function teamEmailErrors(draft: LadderDraft): Record<string, string> {
  const out: Record<string, string> = {};
  for (const [key, raw] of Object.entries(draft.map)) {
    const v = raw.trim();
    if (v !== "" && !isMailingAddress(v)) out[key] = "Enter one email address, like team-list@example.com.";
  }
  return out;
}

/** The editor's starting point: the saved value, else the one in use. */
export function draftFor(spec: LadderKeySpec, value: unknown): LadderDraft {
  const draft: LadderDraft = { text: "", checked: value === true, name: "", email: "", map: {} };
  if (spec.editor === "head") {
    const h = headOf(value);
    return { ...draft, name: h?.name ?? "", email: h?.email ?? "" };
  }
  if (Array.isArray(value)) return { ...draft, text: value.map(String).join(", ") };
  if (spec.editor === "teamEmails") {
    const map: Record<string, string> = {};
    if (isRecord(value)) {
      for (const [k, v] of Object.entries(value)) if (typeof v === "string") map[k.trim().toLowerCase()] = v;
    }
    return { ...draft, map };
  }
  if ((spec.editor === "shiftCounts" || spec.editor === "teamMap") && isRecord(value)) {
    return { ...draft, text: Object.entries(value).map(([k, v]) => `${k}=${String(v)}`).join(", ") };
  }
  if (typeof value === "string" || typeof value === "number") return { ...draft, text: String(value) };
  if (spec.editor === "enum") return { ...draft, text: spec.options?.[0]?.value ?? "" };
  return draft;
}

export type ParsedDraft = { ok: true; value: unknown } | { ok: false; error: string };

/** The draft as the value to save, or why it can't be. Only the shape is
 *  checked here; the server checks the limits and says which key it refused. */
export function parseDraft(spec: LadderKeySpec, draft: LadderDraft): ParsedDraft {
  const text = draft.text.trim();
  switch (spec.editor) {
    case "bool":
      return { ok: true, value: draft.checked };
    case "int":
      return /^-?\d+$/.test(text)
        ? { ok: true, value: Number(text) }
        : { ok: false, error: `${spec.label}: enter a whole number.` };
    case "list":
      // An empty list is a value too: it replaces the file's list with none.
      return {
        ok: true,
        value: text
          .split(",")
          .map((v) => v.trim())
          .filter(Boolean),
      };
    case "shiftCounts": {
      const out: Record<string, number> = {};
      for (const part of text.split(",").map((v) => v.trim()).filter(Boolean)) {
        const m = /^([A-Za-z_]+)\s*=\s*(\d+)$/.exec(part);
        if (!m) return { ok: false, error: `${spec.label}: write each as SHIFT=number, e.g. LK_EVENING=7.` };
        out[m[1].toUpperCase()] = Number(m[2]);
      }
      return { ok: true, value: out };
    }
    case "teamMap": {
      const out: Record<string, string> = {};
      for (const part of text.split(",").map((v) => v.trim()).filter(Boolean)) {
        const m = /^([A-Za-z0-9_.-]+)\s*=\s*([A-Za-z0-9_.-]+)$/.exec(part);
        if (!m) return { ok: false, error: `${spec.label}: write each as key=team, e.g. asgardeo-ops=asgardeo.` };
        out[m[1].toLowerCase()] = m[2].toLowerCase();
      }
      return { ok: true, value: out };
    }
    case "teamEmails": {
      if (Object.keys(teamEmailErrors(draft)).length > 0) {
        return { ok: false, error: `${spec.label}: fix the addresses marked below.` };
      }
      const out: Record<string, string> = {};
      for (const [key, raw] of Object.entries(draft.map)) {
        const v = raw.trim();
        if (v !== "") out[key.trim().toLowerCase()] = v;
      }
      return { ok: true, value: out };
    }
    case "head": {
      const name = draft.name.trim();
      const email = draft.email.trim();
      return name && email
        ? { ok: true, value: { name, email } }
        : { ok: false, error: `${spec.label}: enter a name and an email.` };
    }
    default:
      // An email list may be emptied: empty means no Cc (spec.empty).
      if (!text && spec.inputType === "email") return { ok: true, value: "" };
      if (!text) return { ok: false, error: `${spec.label}: enter a value.` };
      if (spec.inputType === "email" && !isMailingAddress(text)) {
        return { ok: false, error: `${spec.label}: enter one email address, like ${spec.placeholder ?? "team-list@example.com"}.` };
      }
      return { ok: true, value: text };
  }
}

/** Whether parseDraft refuses an empty value for this key, so its editor marks
 *  the box required: a number, a word, a duration or a choice. A list, SHIFT=n
 *  pairs, a team map, the per-team mailing lists and an email box (empty means
 *  no Cc) may be left empty; a switch is never empty; a head's name and email
 *  are both needed, and its editor says so. */
export function ladderValueRequired(spec: LadderKeySpec): boolean {
  switch (spec.editor) {
    case "bool":
    case "list":
    case "shiftCounts":
    case "teamMap":
    case "teamEmails":
      return false;
    default:
      return spec.inputType !== "email";
  }
}

/** The values the alert service runs with from its file and that are not
 *  saved here yet, ready to save in one go. A head the file leaves unnamed is
 *  left out: there is nothing to save for it. */
export function valuesToCopy(s: CaseAlertSettings, ladder: CaseAlertRoomLadder): Record<string, unknown> {
  const running = s.running?.ladders[ladder];
  if (!running) return {};
  const saved = new Set(s.values.filter((v) => v.ladder === ladder).map((v) => v.key));
  const out: Record<string, unknown> = {};
  for (const spec of ladderKeysFor(ladder)) {
    const r = running[spec.key];
    if (!r || r.source !== "file" || saved.has(spec.key) || r.value === undefined || r.value === null) continue;
    if (spec.editor === "head") {
      const h = headOf(r.value);
      if (h?.name && h.email) out[spec.key] = h;
      continue;
    }
    out[spec.key] = r.value;
  }
  return out;
}

/** A channel in words. */
export const CHANNEL_WORDS: Record<CaseAlertChannel, string> = {
  both: "Calls and chat",
  chat: "Chat only",
  call: "Calls only",
  log: "Log only (no calls or chat)",
};

export function channelWords(channel: string | undefined): string | undefined {
  if (!channel) return undefined;
  return CHANNEL_WORDS[channel as CaseAlertChannel] ?? channel;
}

const ALL_CHANNELS: CaseAlertChannel[] = ["both", "chat", "call", "log"];

/** The channels a team can have under its ladder's: the same or less, never
 *  more. Every one when the ladder's isn't known. */
export function teamChannelsUnder(ladderChannel: string | undefined): CaseAlertChannel[] {
  switch (ladderChannel) {
    case "chat":
      return ["chat", "log"];
    case "call":
      return ["call", "log"];
    case "log":
      return ["log"];
    default:
      return ALL_CHANNELS;
  }
}

const PHONE_WORDS: Record<string, string> = {
  profile: "their profile",
  none: "only numbers in the file",
};

function yesNo(v: boolean | undefined): string {
  return v === undefined ? "—" : v ? "Yes" : "No";
}

/** The CRE rung emails in words: off, or on with how many team lists and
 *  whether the CRE and CS lists are set. The addresses never reach the page. */
function rungEmailWords(f: CaseAlertFileOnly): string {
  const on = f["email.enabled"];
  if (on === undefined) return "—";
  if (!on) return "Off";
  const lists = f["email.teamLists"] ?? 0;
  const parts = [
    `${lists} team ${lists === 1 ? "list" : "lists"}`,
    `CRE list ${f["email.creTeamList"] ? "set" : "not set"}`,
    `CS list ${f["email.csTeamList"] ? "set" : "not set"}`,
  ];
  return `On (${parts.join(", ")})`;
}

/** The CRE chain's steps, in the order the timing table lists them. */
export const CRE_STEP_LABELS = ["L0", "L1", "L2", "L3", "L4"] as const;

/** The severities the timing table lists, most urgent first. */
export const CRE_TIMING_SEVERITIES = ["S0", "S1", "S2", "S3", "S4"] as const;

/** One row of the CRE timing table: the severity, each step's call minutes
 *  in words ("+1, +2, +3"; "—" for a step with no calls), and when the chain
 *  ends. Severities the alert service did not report are left out. */
export function creTimingRows(
  timing: CaseAlertFileOnly["timing"],
): { severity: string; steps: string[]; ends: string }[] {
  if (!timing) return [];
  return CRE_TIMING_SEVERITIES.filter((s) => Array.isArray(timing[s])).map((severity) => {
    const steps = timing[severity] ?? [];
    const words = CRE_STEP_LABELS.map((_, i) => {
      const calls = steps[i] ?? [];
      return calls.length === 0 ? "—" : calls.map((m) => `+${m}`).join(", ");
    });
    const all = steps.flat();
    return { severity, steps: words, ends: all.length === 0 ? "—" : `+${Math.max(...all)} min` };
  });
}

/** The settings only the file can set, as rows for people. The heads row is
 *  CRE's only: SRE has no heads. Allowed numbers are a count; the numbers
 *  never reach the page. */
export function fileOnlyRows(
  ladder: CaseAlertRoomLadder,
  f: CaseAlertFileOnly,
  /** Keys reported among the ladder's tab values: shown there, not here. */
  onTab: ReadonlySet<string> = new Set(),
): { label: string; value: string }[] {
  const maxCalls = f["safety.maxCallsPerLadder"];
  const maxLevel = f["safety.maxLevel"];
  const allowed = f["safety.allowedNumbers"];
  const rows: { key: string; label: string; value: string }[] = [
    { key: "enabled", label: "Alerts on", value: yesNo(f.enabled) },
    { key: "channel", label: "Channel", value: channelWords(f.channel) ?? "—" },
    {
      key: "safety.maxCallsPerLadder",
      label: "Max calls per ladder",
      value: maxCalls === undefined ? "—" : maxCalls === 0 ? "no cap" : String(maxCalls),
    },
    { key: "safety.maxLevel", label: "Highest step", value: maxLevel === undefined ? "no cap" : `L${maxLevel}` },
    {
      key: "safety.allowedNumbers",
      label: "Allowed numbers",
      value:
        allowed === undefined
          ? "—"
          : allowed === 0
            ? "any number"
            : `only ${allowed} listed ${allowed === 1 ? "number" : "numbers"}`,
    },
    {
      key: "safety.callWithoutVerifiedLeads",
      label: "Call before every lead has a number",
      value: yesNo(f["safety.callWithoutVerifiedLeads"]),
    },
  ];
  if (ladder === "CRE") {
    rows.push({
      key: "safety.callHeadsWithoutLowerTiers",
      label: "Call the heads when nobody below can be called",
      value: yesNo(f["safety.callHeadsWithoutLowerTiers"]),
    });
  }
  const phones = f["phones.source"];
  rows.push({ key: "phones.source", label: "Phone numbers from", value: phones ? (PHONE_WORDS[phones] ?? phones) : "—" });
  rows.push({ key: "ackChat", label: "Reply in the thread when a chain stops", value: yesNo(f.ackChat) });
  if (ladder === "CRE") {
    rows.push({ key: "email.enabled", label: "Email per step", value: rungEmailWords(f) });
    const custom = f["rules.custom"];
    rows.push({
      key: "rules.custom",
      label: "Who each step calls",
      value: custom === undefined ? "—" : custom ? "The file's own rule table" : "Built-in rules (Case Paging Rules sheet)",
    });
  }
  return rows
    .filter((r) => !onTab.has(r.key))
    .map(({ label, value }) => ({ label, value: safe(value) }));
}

/** The limits of a key, in one line, for the editor. Hints only: the server
 *  checks them. */
export function limitHint(spec: LadderKeySpec): string | null {
  const empty = `Empty means ${spec.empty ?? "none"}.`;
  switch (spec.editor) {
    case "int":
      return spec.min !== undefined && spec.max !== undefined ? `${spec.min} to ${spec.max}.` : null;
    case "list":
      return `Separate with commas${spec.max ? `, up to ${spec.max}` : ""}. ${empty}`;
    case "shiftCounts":
      return `SHIFT=number, separated with commas${spec.max ? `, each 0 to ${spec.max}` : ""}. ${empty}`;
    case "teamMap":
      return `key=team, separated with commas. ${empty}`;
    case "teamEmails":
      return "Leave a team empty for no Cc.";
    default:
      return null;
  }
}

/** A value as JSON with its keys in order, so two values that hold the same
 *  thing compare equal. */
function canonical(v: unknown): string {
  if (Array.isArray(v)) return `[${v.map(canonical).join(",")}]`;
  if (isRecord(v)) {
    return `{${Object.keys(v)
      .sort()
      .map((k) => `${JSON.stringify(k)}:${canonical(v[k])}`)
      .join(",")}}`;
  }
  return JSON.stringify(v) ?? "null";
}

/** Whether a saved value is the one in use. A head is compared by its name
 *  and email only: the file's may also carry a phone number. */
export function sameLadderValue(spec: LadderKeySpec | undefined, a: unknown, b: unknown): boolean {
  if (spec?.editor === "head") return canonical(headOf(a)) === canonical(headOf(b));
  return canonical(a) === canonical(b);
}

/** Where "In use" comes from, in one line. */
export function runningLine(s: CaseAlertSettings): string {
  const r = s.running;
  if (!r) {
    return "The alert service hasn't reported what it uses yet. It does when it starts in shadow or db mode.";
  }
  // The server leaves reportedAt out when it has no time for the report.
  const at = r.reportedAt ? ` at ${new Date(r.reportedAt).toLocaleString()}` : "";
  const base = `"In use" is what the alert service reported${at}, in ${r.mode} mode.`;
  return r.mode === "shadow"
    ? `${base} In shadow mode it still uses its file; saved values take over in db mode.`
    : base;
}

/** The ladder groups under Advanced: the rest of the paging file, after the
 *  switches and who gets alerts. */
export const ADVANCED_GROUPS: LadderKeyGroup[] = LADDER_KEY_GROUPS.filter(
  (g) => g !== "On/off and channel" && g !== "Who gets alerts",
);

/** How many of a ladder's values in these groups are saved here. */
export function savedInGroups(
  s: CaseAlertSettings,
  ladder: CaseAlertRoomLadder,
  groups: readonly LadderKeyGroup[],
): number {
  const keys = new Set(ladderKeysFor(ladder).filter((k) => groups.includes(k.group)).map((k) => k.key));
  return s.values.filter((v) => v.ladder === ladder && keys.has(v.key)).length;
}

/** The settings the Alert Settings tab shows for each ladder, in this order:
 *  the switch, the channel, which priorities start a chain, the call limits,
 *  then CRE's Case Alert email and SRE's and SME's time between steps. CRE's
 *  heads are not shown: left
 *  empty, the alert service finds them in the leadership team. Every other key
 *  stays in LADDER_KEYS and keeps its value (the file's, or one saved
 *  earlier); the tab just doesn't offer it. */
export const SHOWN_KEYS: Record<CaseAlertRoomLadder, readonly string[]> = {
  CRE: [
    "enabled",
    "channel",
    "trigger.priorities",
    "safety.maxCallsPerLadder",
    "safety.callWithoutVerifiedLeads",
    "email.enabled",
    "email.teamLists",
    "email.creTeamList",
    "email.csTeamList",
  ],
  SRE: [
    "enabled",
    "channel",
    "trigger.priorities",
    "safety.maxCallsPerLadder",
    "safety.callWithoutVerifiedLeads",
    "timing.interval",
  ],
  SME: ["enabled", "channel", "timing.interval"],
};

/** The specs of these keys on a ladder, in the order given. */
export function ladderKeysByName(ladder: CaseAlertRoomLadder, keys: readonly string[]): LadderKeySpec[] {
  const specs = ladderKeysFor(ladder);
  return keys.flatMap((k) => specs.filter((s) => s.key === k));
}

/** How many of a ladder's saved values are for keys the page doesn't show. */
export function savedOutside(s: CaseAlertSettings, ladder: CaseAlertRoomLadder, shown: readonly string[]): number {
  const keys = new Set(shown);
  return s.values.filter((v) => v.ladder === ladder && !keys.has(v.key)).length;
}
