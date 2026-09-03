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

/** How often CS Overview auto-refetches its dashboard metadata and every
 * tile's own count, in milliseconds — matches `digiops-cs`'s own
 * Wallboard.tsx (`REFRESH_INTERVAL = 60` seconds), per Chamara's ask
 * (confirmed 60s over his own initial 30s guess, once told what the
 * original reference actually uses). Passed explicitly by
 * `WallboardDashboard`/`WallboardStatTile`/`WallboardSecondaryStat` into
 * `useDashboard`/`useWidgetData`'s own optional `refetchIntervalMs`
 * parameter — every other dashboard's call site leaves that unset, so
 * this constant only ever affects CS Overview. */
export const CS_OVERVIEW_REFETCH_INTERVAL_MS = 60_000;

/**
 * Per-metric glow colors for the CS Overview dashboard, ported field-for-
 * field from `digiops-cs`'s `Wallboard.tsx` `StatCard` — read directly from
 * its source (not approximated) to get two easy-to-miss details right:
 *
 * 1. The VALUE color is driven purely by `alertType` ("cyan"/"red"/"amber")
 *    and is always exactly one of three fixed Tailwind `-200` shades,
 *    regardless of which section renders it — the original's own
 *    `alertColorClass` ignores the per-call `colorClass` prop entirely once
 *    an alert is active.
 * 2. "Red" is NOT one color — CRE's "SLA Violations" card uses Tailwind's
 *    `rose` family (label/bg/border), while SRE's "SLA Violations" and
 *    Security's "SLA Violation" use `red` (a different Tailwind palette).
 *    Both still show the same rose-200 value text and the same fixed
 *    rose-400-rgb glow shadow — only label/background/border differ. This
 *    is why the lookup below is keyed by (section, displayName), not by
 *    displayName alone: "SLA Violations" is literally the same string in
 *    both CRE and SRE but must resolve to different colors.
 *
 * A card whose `alertType` prop was never set in the original (e.g. FDE's
 * "In-Progress Onboarding", Security's "Active") LOOKS like it should be
 * colored (it's passed a non-white `colorClass`), but the original's own
 * `hasAlert = alertType && value > 0` gate means that prop is dead code —
 * those cards always render plain white. Confirmed against the real
 * `CS_Dashboard.png` screenshot, where they are in fact plain white, not
 * blue/purple/orange. Do not add entries for them here.
 */
export type WallboardSection = "cre" | "sre" | "security" | "fde";

export interface StatTileColors {
  /** The big number's own color — always a `-200` shade. */
  value: string;
  /** The label underneath — a `-400` shade, but from whichever color
   * family (`rose` vs `red`) this exact (section, metric) pair uses. */
  label: string;
  /** Tile background — the `-900` shade at 30% opacity. */
  bg: string;
  /** Tile border — the `-700` shade. */
  border: string;
  /** `drop-shadow`/`text-shadow` color for the glowing value — fixed per
   * `alertType`, independent of the label/bg/border color family. */
  shadow: string;
}

const CYAN: StatTileColors = {
  value: "#a5f3fc",
  label: "#22d3ee",
  bg: "rgba(22,78,99,0.3)",
  border: "#0e7490",
  shadow: "rgba(34,211,238,0.8)",
};

const AMBER: StatTileColors = {
  value: "#fde68a",
  label: "#fbbf24",
  bg: "rgba(120,53,15,0.3)",
  border: "#b45309",
  shadow: "rgba(251,191,36,0.8)",
};

// Tailwind `rose` family — CRE's own "SLA Violations" only.
const RED_ROSE: StatTileColors = {
  value: "#fecdd3",
  label: "#fb7185",
  bg: "rgba(136,19,55,0.3)",
  border: "#be123c",
  shadow: "rgba(251,113,133,0.8)",
};

// Tailwind `red` family — SRE's "SLA Violations" and Security's "SLA
// Violation". Value and shadow are identical to RED_ROSE (both driven by
// `alertType: "red"` alone in the original); only label/bg/border differ.
const RED_RED: StatTileColors = {
  value: "#fecdd3",
  label: "#f87171",
  bg: "rgba(127,29,29,0.3)",
  border: "#b91c1c",
  shadow: "rgba(251,113,133,0.8)",
};

const STAT_TILE_STYLES: Record<WallboardSection, Record<string, StatTileColors>> = {
  cre: {
    Open: CYAN,
    "SLA Violations": RED_ROSE,
    Escalations: AMBER,
  },
  sre: {
    Open: CYAN,
    "SLA Violations": RED_RED,
    "New CR": CYAN,
    "Open SR": CYAN,
  },
  security: {
    Open: CYAN,
    "SLA Violation": RED_RED,
  },
  fde: {
    "Open Onboarding": CYAN,
  },
};

/** Returns this (section, displayName) pair's glow colors, or `undefined`
 * for a plain tile (white value/label, neutral gray background/border —
 * the caller's own default, not represented here). */
export function getStatTileColors(
  section: WallboardSection,
  displayName: string,
): StatTileColors | undefined {
  return STAT_TILE_STYLES[section]?.[displayName];
}

/** CRE's four "primary" metrics, in the exact order the original renders
 * its 2x2 primary grid. Any CRE widget whose displayName isn't in this
 * list is a "secondary" metric — rendered as a plain (never glow-capable)
 * tile in the smaller 3-column grid below, ordered by `CRE_SECONDARY_ORDER`
 * — matching the original's `SecondaryStat`, which has no `alertType`
 * concept at all. */
export const CRE_PRIMARY_ORDER = ["Open", "In-Progress", "SLA Violations", "Escalations"] as const;

/** CRE's own secondary-tier metrics, in the original's exact fixed
 * position order — most notably "Waiting on WSO2" (2nd) well before
 * "Awaiting Info" (last), the reverse of what the live config's own
 * widget-array order otherwise produced. A widget whose displayName isn't
 * in this list keeps its original relative order, appended after every
 * named one — see `sortByFixedOrder`.
 *
 * Keyed on the ALREADY-ALIASED name, not the raw live-config one:
 * `resolveDisplayNameAlias` always runs (in `WallboardDashboard`) before
 * a widget ever reaches this ordering step, so "30+ Days" (the live
 * config's own name) has already become "30+ Days Cases" by the time
 * sorting happens — matching it here, not the pre-alias name, is what
 * this list needs. */
export const CRE_SECONDARY_ORDER = [
  "At WSO2 Incidents",
  "Waiting on WSO2",
  "30+ Days Cases",
  "Waiting On Product",
  "Being Fixed",
  "Migration",
  "Created Today",
  "Resolved Today",
  "Awaiting Info",
] as const;

/** Sorts `items` by their `displayName`'s position in `order` — e.g.
 * CRE's secondary tier, or one of SRE's four sub-rows. Matched
 * case-insensitively: the live config's casing has repeatedly turned out
 * to differ from the original Wallboard.tsx labels this module's other
 * tables are keyed on (see e.g. `resolveDisplayNameAlias`), and a case
 * mismatch here doesn't just mislabel a card the way it does elsewhere —
 * it silently drops the item to the very end of its row instead of
 * sorting it correctly. An item whose displayName (after that
 * case-insensitive match) isn't in `order` keeps its original relative
 * order, appended after every named one. `Array.prototype.sort` is a
 * stable sort (guaranteed since ES2019), so ties (including every "not
 * found" item sharing the same fallback rank) preserve their original
 * relative order rather than shuffling. */
export function sortByFixedOrder<T extends { displayName: string }>(items: T[], order: readonly string[]): T[] {
  const lowerOrder = order.map((name) => name.toLowerCase());
  return [...items].sort((a, b) => {
    const rankOf = (item: T): number => {
      const i = lowerOrder.indexOf(item.displayName.toLowerCase());
      return i === -1 ? order.length : i;
    };
    return rankOf(a) - rankOf(b);
  });
}

/** SRE widgets are grouped into four labeled sub-rows purely by their own
 * `resourceType` — a structural field every widget already carries, rather
 * than a second "subsection" string the backend's dashboard schema has no
 * field for. `incident`/`problem`/`change_request` map to their own row;
 * everything else (the Open SR / In-Progress SR pair) falls into "Service
 * Requests" by elimination — matching the original's own four `<div>`
 * blocks (Incidents/Problems/Change Requests/Service Requests). */
export const SRE_SUBSECTION_ORDER = ["Incidents", "Problems", "Change Requests", "Service Requests"] as const;
export type SreSubsection = (typeof SRE_SUBSECTION_ORDER)[number];

export function sreSubsectionFor(resourceType: string): SreSubsection {
  switch (resourceType) {
    case "incident":
      return "Incidents";
    case "problem":
      return "Problems";
    case "change_request":
      return "Change Requests";
    default:
      return "Service Requests";
  }
}

/** Each SRE sub-row's own fixed card order, matching the original's exact
 * position of each `StatCard` within its `<div>` block — most notably
 * Problems, where "In-Progress Problems" renders BEFORE "Open Problems"
 * (the reverse of what its own resourceType-grouped array order would
 * otherwise produce). A widget whose (already-aliased) displayName isn't
 * in its row's list here keeps its original relative order, appended
 * after every named one — same tolerant-fallback behavior as
 * `CRE_PRIMARY_ORDER`. */
export const SRE_SUBROW_ORDER: Record<SreSubsection, readonly string[]> = {
  Incidents: ["Open", "SLA Violations", "In-Progress"],
  Problems: ["In-Progress Problems", "Open Problems"],
  "Change Requests": ["New CR", "Authorize CR", "Approved CR", "Cust. Approval CR", "Scheduled CR"],
  "Service Requests": ["Open SR", "In-Progress SR"],
};

/**
 * The CS Overview dashboard's live widget config carries a handful of
 * `displayName` values that don't exactly match the original
 * `Wallboard.tsx` labels — configured before this styling work started,
 * not something this frontend can rewrite at the source. This table
 * relabels them for display purposes only (and, critically, BEFORE the
 * emphasis lookup above runs — `getStatTileColors` matches exact strings
 * like "Open", so an unaliased "SRE - Open Incident" would silently never
 * get its cyan glow at all).
 *
 * Extend this table, the same way `METRIC_EMPHASIS` above is extended, if
 * another mismatched label turns up — don't add a fuzzy/heuristic
 * fallback here.
 */
const DISPLAY_NAME_ALIASES: Record<string, string> = {
  "SRE - Open Incident": "Open",
  "SRE - In-Progress Incidents": "In-Progress",
  "SRE - SLA Violations": "SLA Violations",
  "Authorized CR": "Authorize CR",
  "Customer Approval CR": "Cust. Approval CR",
  "Open Service Request": "Open SR",
  "Service Request - In-Progress": "In-Progress SR",
  // CRE's own — "Escalated" here instead of "Escalations" meant it never
  // matched CRE_PRIMARY_ORDER's exact "Escalations" entry, so it fell
  // into the secondary tier instead of its correct 4th primary-grid
  // position (right of SLA Violations).
  Escalated: "Escalations",
  // CRE's own — the live widget is named "30+ Days" (see
  // CRE_SECONDARY_ORDER's own note on this), but should DISPLAY as the
  // original's full "30+ Days Cases" text.
  "30+ Days": "30+ Days Cases",
};

/** A stray "/Update" (or "/ Update") suffix seen tacked onto CRE's own
 * "Being Fixed" card in the live config — handled as a pattern rather
 * than an exact-string alias above, since its precise raw text (spacing,
 * capitalization, trailing punctuation) wasn't confirmed exactly. */
const TRAILING_UPDATE_SUFFIX = /\s*\/\s*update\.?\s*$/i;

/** The live config spells "In-Progress" without its hyphen on at least
 * SRE's own Problems row ("Inprogress Problems"). Handled as a
 * case-insensitive whole-word pattern, not another exact-string alias
 * table entry, since the same missing hyphen could plausibly turn up on
 * a widget this table doesn't already know about — an exact entry would
 * only catch the one instance already found, not the underlying typo. */
const MISSING_IN_PROGRESS_HYPHEN = /\binprogress\b/i;

/** Every Security Report widget in the live config carries a "Sec Report
 * - " prefix the original Wallboard.tsx labels never had (e.g. "Sec
 * Report - Open"). A case-insensitive prefix pattern, not four more exact
 * -string `DISPLAY_NAME_ALIASES` entries, since the prefix's own precise
 * casing/spacing wasn't confirmed exactly — only the four suffixes after
 * it are known values. */
const SEC_REPORT_PREFIX = /^sec report\s*-\s*/i;

const SEC_REPORT_SUFFIX_ALIASES: Record<string, string> = {
  open: "Open",
  active: "Active",
  "being fixed": "In-Progress (Being Fixed)",
  "sla violation": "SLA Violation",
};

function resolveSecReportAlias(name: string): string {
  if (!SEC_REPORT_PREFIX.test(name)) return name;
  const suffix = name.replace(SEC_REPORT_PREFIX, "").trim();
  return SEC_REPORT_SUFFIX_ALIASES[suffix.toLowerCase()] ?? suffix;
}

/**
 * FDE's own widgets in the live config have turned out far less
 * consistent than Security Report's — a leading "FDE" prefix that's
 * sometimes dash-separated ("FDE - Open Onboarding"), sometimes just
 * space-separated ("FDE Active Onboarding"), and sometimes absent
 * entirely; a trailing "Cases"/"Case" word tacked on some of them ("FDE
 * Active Onboarding Cases", "Active Engagement Cases"); and at least one
 * singular where the original is plural ("Active Engagement" vs "Active
 * Engagements"). Rather than one more exact-string entry per variant
 * (which the last two rounds of fixes already showed doesn't keep up),
 * this strips BOTH optional wrappers first and looks the bare remainder
 * up case- and plural-insensitively — the lookup table itself is still a
 * fixed, exact list of the six known FDE metrics, not a fuzzy/regex
 * fallback; only the wrapping around them is tolerant.
 */
const FDE_PREFIX = /^fde[\s-]+/i;
const TRAILING_CASES_SUFFIX = /\s+cases?\.?\s*$/i;

const FDE_SUFFIX_ALIASES: Record<string, string> = {
  "open onboarding": "Open Onboarding",
  "in-progress onboarding": "In-Progress Onboarding",
  "active onboarding": "Active Onboarding",
  "open engagement": "Open Engagements",
  "open engagements": "Open Engagements",
  "in-progress engagement": "In-Progress Engagements",
  "in-progress engagements": "In-Progress Engagements",
  "active engagement": "Active Engagements",
  "active engagements": "Active Engagements",
};

function resolveFdeAlias(name: string): string {
  const withoutPrefix = name.replace(FDE_PREFIX, "").trim();
  const bareName = withoutPrefix.replace(TRAILING_CASES_SUFFIX, "").trim();
  // On a lookup miss, the fallback depends on whether an FDE prefix was
  // actually present:
  // - Prefix WAS present (`withoutPrefix !== name`, e.g. "FDE -
  //   Inprogress Onboarding"): still fall back to the STRIPPED name, not
  //   the original — the later MISSING_IN_PROGRESS_HYPHEN pipeline step
  //   needs the FDE prefix gone to finish correcting a missing-hyphen
  //   variant; falling back to the untouched original would leave that
  //   prefix stuck on permanently instead.
  // - No FDE prefix at all (`withoutPrefix === name`): the "Cases" strip
  //   above was only ever meant for FDE's own known metric names (e.g.
  //   "Active Engagement Cases", which DOES hit the lookup and never
  //   reaches this fallback) — an unrelated, unrecognized widget that
  //   merely happens to end in "Cases" must come back untouched, not
  //   silently lose that word.
  return (
    FDE_SUFFIX_ALIASES[bareName.toLowerCase()] ?? (withoutPrefix === name ? name : bareName)
  );
}

/** Resolves a widget's raw `displayName` to the exact label the original
 * Wallboard.tsx uses. Apply this BEFORE both rendering the label text and
 * looking up its emphasis colors — every other function in this module
 * (`getStatTileColors`, `getMetricEmphasis`) expects the already-resolved
 * name, not the raw backend one. */
export function resolveDisplayNameAlias(rawDisplayName: string): string {
  const sectionPrefixResolved = resolveFdeAlias(resolveSecReportAlias(rawDisplayName));
  const aliased = DISPLAY_NAME_ALIASES[sectionPrefixResolved] ?? sectionPrefixResolved;
  return aliased
    .replace(TRAILING_UPDATE_SUFFIX, "")
    .replace(MISSING_IN_PROGRESS_HYPHEN, "In-Progress")
    .trim();
}
