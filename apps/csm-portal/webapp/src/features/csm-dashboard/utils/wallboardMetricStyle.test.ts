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

import { describe, expect, it } from "vitest";
import {
  CRE_PRIMARY_ORDER,
  CRE_SECONDARY_ORDER,
  getStatTileColors,
  resolveDisplayNameAlias,
  sortByFixedOrder,
  SRE_SUBROW_ORDER,
  sreSubsectionFor,
} from "@features/csm-dashboard/utils/wallboardMetricStyle";

describe("getStatTileColors", () => {
  it("returns undefined for a metric with no emphasis in its section", () => {
    expect(getStatTileColors("cre", "In-Progress")).toBeUndefined();
    expect(getStatTileColors("fde", "Active Engagements")).toBeUndefined();
    expect(getStatTileColors("security", "Active")).toBeUndefined();
  });

  it("returns undefined for a metric name that only has emphasis in a different section", () => {
    // "New CR" is only emphasized under "sre", not "cre".
    expect(getStatTileColors("cre", "New CR")).toBeUndefined();
  });

  // The core regression this whole module exists to prevent: the same
  // displayName, "SLA Violations", must resolve to two different color
  // families depending purely on which section it's rendered under.
  it("resolves 'SLA Violations' to the rose family under cre", () => {
    const colors = getStatTileColors("cre", "SLA Violations");
    expect(colors?.label).toBe("#fb7185"); // rose-400
    expect(colors?.bg).toBe("rgba(136,19,55,0.3)"); // rose-900/30
    expect(colors?.border).toBe("#be123c"); // rose-700
  });

  it("resolves 'SLA Violations' to the red family under sre", () => {
    const colors = getStatTileColors("sre", "SLA Violations");
    expect(colors?.label).toBe("#f87171"); // red-400
    expect(colors?.bg).toBe("rgba(127,29,29,0.3)"); // red-900/30
    expect(colors?.border).toBe("#b91c1c"); // red-700
  });

  it("resolves Security's singular 'SLA Violation' to the same red family as sre", () => {
    const colors = getStatTileColors("security", "SLA Violation");
    expect(colors?.label).toBe("#f87171");
  });

  it("gives 'SLA Violations' the same value color and glow shadow in both cre and sre, despite the different label/bg/border family", () => {
    const cre = getStatTileColors("cre", "SLA Violations");
    const sre = getStatTileColors("sre", "SLA Violations");
    expect(cre?.value).toBe(sre?.value);
    expect(cre?.value).toBe("#fecdd3"); // rose-200, always
    expect(cre?.shadow).toBe(sre?.shadow);
    expect(cre?.shadow).toBe("rgba(251,113,133,0.8)");
  });

  it("resolves 'Open' to cyan uniformly across every section that has it", () => {
    for (const section of ["cre", "sre", "security"] as const) {
      const colors = getStatTileColors(section, "Open");
      expect(colors?.value).toBe("#a5f3fc"); // cyan-200
      expect(colors?.label).toBe("#22d3ee"); // cyan-400
    }
  });

  it("resolves CRE's 'Escalations' to amber, the only card that uses it", () => {
    const colors = getStatTileColors("cre", "Escalations");
    expect(colors?.value).toBe("#fde68a"); // amber-200
    expect(colors?.label).toBe("#fbbf24"); // amber-400
  });
});

describe("sreSubsectionFor", () => {
  it("maps incident/problem/change_request to their own named sub-row", () => {
    expect(sreSubsectionFor("incident")).toBe("Incidents");
    expect(sreSubsectionFor("problem")).toBe("Problems");
    expect(sreSubsectionFor("change_request")).toBe("Change Requests");
  });

  it("falls back to 'Service Requests' for anything else (case/service_request)", () => {
    expect(sreSubsectionFor("case")).toBe("Service Requests");
    expect(sreSubsectionFor("service_request")).toBe("Service Requests");
    expect(sreSubsectionFor("account")).toBe("Service Requests");
  });
});

describe("resolveDisplayNameAlias", () => {
  it("relabels every known-mismatched live widget name to the original's exact label", () => {
    expect(resolveDisplayNameAlias("SRE - Open Incident")).toBe("Open");
    expect(resolveDisplayNameAlias("SRE - In-Progress Incidents")).toBe("In-Progress");
    expect(resolveDisplayNameAlias("Authorized CR")).toBe("Authorize CR");
    expect(resolveDisplayNameAlias("Customer Approval CR")).toBe("Cust. Approval CR");
    expect(resolveDisplayNameAlias("Open Service Request")).toBe("Open SR");
    expect(resolveDisplayNameAlias("Service Request - In-Progress")).toBe("In-Progress SR");
    expect(resolveDisplayNameAlias("Escalated")).toBe("Escalations");
    expect(resolveDisplayNameAlias("SRE - SLA Violations")).toBe("SLA Violations");
    expect(resolveDisplayNameAlias("30+ Days")).toBe("30+ Days Cases");
  });

  it("resolves SRE's aliased 'SLA Violations' to the correct red-family color afterward", () => {
    expect(getStatTileColors("sre", resolveDisplayNameAlias("SRE - SLA Violations"))?.label).toBe("#f87171");
  });

  it("resolves aliased '30+ Days Cases' into CRE_SECONDARY_ORDER's own 3rd slot", () => {
    const order: readonly string[] = CRE_SECONDARY_ORDER;
    expect(order.indexOf(resolveDisplayNameAlias("30+ Days"))).toBe(order.indexOf("30+ Days Cases"));
  });

  it("resolves aliased 'Escalated' -> 'Escalations' into CRE_PRIMARY_ORDER's own 4th slot (right of SLA Violations)", () => {
    const order: readonly string[] = CRE_PRIMARY_ORDER;
    expect(order.indexOf(resolveDisplayNameAlias("Escalated"))).toBe(order.indexOf("Escalations"));
    expect(order.indexOf("Escalations")).toBe(order.indexOf("SLA Violations") + 1);
  });

  it("strips a trailing '/Update' suffix off CRE's own 'Being Fixed' card, case- and spacing-insensitively", () => {
    expect(resolveDisplayNameAlias("Being Fixed/Update")).toBe("Being Fixed");
    expect(resolveDisplayNameAlias("Being Fixed / Update")).toBe("Being Fixed");
    expect(resolveDisplayNameAlias("Being Fixed/update.")).toBe("Being Fixed");
  });

  it("returns any other displayName unchanged", () => {
    expect(resolveDisplayNameAlias("Open")).toBe("Open");
    expect(resolveDisplayNameAlias("Escalations")).toBe("Escalations");
    expect(resolveDisplayNameAlias("Some Future Widget")).toBe("Some Future Widget");
  });

  it("inserts the missing hyphen in 'Inprogress' wherever it appears, case-insensitively — only the matched word is re-cased, the rest of the string is untouched", () => {
    expect(resolveDisplayNameAlias("Inprogress Problems")).toBe("In-Progress Problems");
    expect(resolveDisplayNameAlias("INPROGRESS PROBLEMS")).toBe("In-Progress PROBLEMS");
    expect(resolveDisplayNameAlias("inprogress problems")).toBe("In-Progress problems");
  });

  // The actual regression this was added for: once hyphenated correctly,
  // "Inprogress Problems" must resolve to the exact same string
  // SRE_SUBROW_ORDER's "Problems" entry expects, so the fixed-position
  // sort in WallboardSreSection actually recognizes it.
  it("resolves 'Inprogress Problems' to exactly what SRE_SUBROW_ORDER['Problems'] expects", () => {
    expect(resolveDisplayNameAlias("Inprogress Problems")).toBe(SRE_SUBROW_ORDER.Problems[0]);
  });

  // The whole reason alias resolution has to run BEFORE the emphasis
  // lookup, not just before rendering: an unaliased name would never
  // match `getStatTileColors`'s exact-string keys at all.
  it("produces a name that resolves correctly through getStatTileColors afterward", () => {
    const resolved = resolveDisplayNameAlias("SRE - Open Incident");
    expect(getStatTileColors("sre", resolved)?.value).toBe("#a5f3fc");
  });

  it("strips Security Report's 'Sec Report - ' prefix and relabels the remainder, case-insensitively", () => {
    expect(resolveDisplayNameAlias("Sec Report - Open")).toBe("Open");
    expect(resolveDisplayNameAlias("SEC REPORT - ACTIVE")).toBe("Active");
    expect(resolveDisplayNameAlias("sec report-being fixed")).toBe("In-Progress (Being Fixed)");
    expect(resolveDisplayNameAlias("Sec Report - SLA Violation")).toBe("SLA Violation");
  });

  it("leaves a Security Report suffix it doesn't recognize as just the de-prefixed text, rather than dropping it", () => {
    expect(resolveDisplayNameAlias("Sec Report - Some New Metric")).toBe("Some New Metric");
  });

  it("resolves Security Report's aliased 'Open'/'SLA Violation' to their correct security-section colors afterward", () => {
    expect(getStatTileColors("security", resolveDisplayNameAlias("Sec Report - Open"))?.value).toBe("#a5f3fc");
    expect(getStatTileColors("security", resolveDisplayNameAlias("Sec Report - SLA Violation"))?.label).toBe(
      "#f87171",
    );
  });

  it("strips FDE's own 'FDE - ' prefix and relabels every one of the six known suffixes, case-insensitively", () => {
    expect(resolveDisplayNameAlias("FDE - Open Onboarding")).toBe("Open Onboarding");
    expect(resolveDisplayNameAlias("FDE - In-Progress Onboarding")).toBe("In-Progress Onboarding");
    expect(resolveDisplayNameAlias("fde-active onboarding")).toBe("Active Onboarding");
    expect(resolveDisplayNameAlias("FDE - Open Engagements")).toBe("Open Engagements");
    expect(resolveDisplayNameAlias("FDE - In-Progress Engagements")).toBe("In-Progress Engagements");
    expect(resolveDisplayNameAlias("FDE - ACTIVE ENGAGEMENTS")).toBe("Active Engagements");
  });

  it("self-corrects an 'FDE - Inprogress ...' (missing-hyphen) suffix via the later hyphen-fix step, even with no matching FDE_SUFFIX_ALIASES entry", () => {
    expect(resolveDisplayNameAlias("FDE - Inprogress Onboarding")).toBe("In-Progress Onboarding");
  });

  it("resolves FDE's aliased 'Open Onboarding' to its correct emphasis color afterward", () => {
    expect(getStatTileColors("fde", resolveDisplayNameAlias("FDE - Open Onboarding"))?.value).toBe("#a5f3fc");
  });

  // The live config turned out inconsistent about the FDE prefix itself
  // (dash vs plain space vs none at all) and about a trailing "Cases"
  // word — three real variants found on the actual dashboard, not
  // hypothetical ones.
  it("handles a space-separated 'FDE ' prefix (no dash) plus a trailing 'Cases' suffix", () => {
    expect(resolveDisplayNameAlias("FDE Active Onboarding Cases")).toBe("Active Onboarding");
    expect(resolveDisplayNameAlias("FDE Open Engagements")).toBe("Open Engagements");
  });

  it("handles no FDE prefix at all, a trailing 'Cases' suffix, and a singular where the original is plural", () => {
    expect(resolveDisplayNameAlias("Active Engagement Cases")).toBe("Active Engagements");
  });

  // Regression test (CodeRabbit): the "Cases" strip above is only ever
  // meant for FDE's own known metric names. An unrelated, unrecognized
  // widget that merely happens to end in "Cases" — no FDE prefix, no
  // matching alias — must come back completely untouched, not have that
  // word silently dropped.
  it("preserves an unrecognized non-FDE metric name that happens to end in 'Cases', unchanged", () => {
    expect(resolveDisplayNameAlias("Some Future Cases")).toBe("Some Future Cases");
  });
});

describe("sortByFixedOrder", () => {
  function item(displayName: string): { displayName: string } {
    return { displayName };
  }

  it("sorts items by their position in the given order, regardless of input array order", () => {
    const input = [item("Awaiting Info"), item("Migration"), item("Waiting on WSO2"), item("At WSO2 Incidents")];
    const sorted = sortByFixedOrder(input, CRE_SECONDARY_ORDER);
    expect(sorted.map((i) => i.displayName)).toEqual([
      "At WSO2 Incidents",
      "Waiting on WSO2",
      "Migration",
      "Awaiting Info",
    ]);
  });

  it("keeps an unrecognized item in its original relative position, appended after every named one", () => {
    const input = [item("Some New Metric"), item("Migration"), item("At WSO2 Incidents")];
    const sorted = sortByFixedOrder(input, CRE_SECONDARY_ORDER);
    expect(sorted.map((i) => i.displayName)).toEqual(["At WSO2 Incidents", "Migration", "Some New Metric"]);
  });

  it("does not mutate the input array", () => {
    const input = [item("Migration"), item("At WSO2 Incidents")];
    const original = [...input];
    sortByFixedOrder(input, CRE_SECONDARY_ORDER);
    expect(input).toEqual(original);
  });

  // The actual regression this was added for: a live-config casing
  // mismatch ("WAITING ON WSO2" vs "Waiting on WSO2") must still sort
  // correctly, not silently fall to the end of the row.
  it("matches case-insensitively, so a live-config casing mismatch still sorts correctly instead of falling to the end", () => {
    const input = [item("AWAITING INFO"), item("waiting on wso2"), item("At Wso2 Incidents")];
    const sorted = sortByFixedOrder(input, CRE_SECONDARY_ORDER);
    expect(sorted.map((i) => i.displayName)).toEqual(["At Wso2 Incidents", "waiting on wso2", "AWAITING INFO"]);
  });

  // CRE_SECONDARY_ORDER is keyed on the ALREADY-ALIASED "30+ Days Cases"
  // (resolveDisplayNameAlias turns the live config's own "30+ Days" into
  // that before a widget ever reaches sorting) — this proves the
  // post-alias name slots into its correct 3rd position (between Waiting
  // on WSO2 and Waiting On Product) rather than falling to the end.
  it("places '30+ Days Cases' in its correct 3rd position, between Waiting on WSO2 and Waiting On Product", () => {
    const input = [
      item("Waiting On Product"),
      item("30+ Days Cases"),
      item("Waiting on WSO2"),
      item("At WSO2 Incidents"),
    ];
    const sorted = sortByFixedOrder(input, CRE_SECONDARY_ORDER);
    expect(sorted.map((i) => i.displayName)).toEqual([
      "At WSO2 Incidents",
      "Waiting on WSO2",
      "30+ Days Cases",
      "Waiting On Product",
    ]);
  });
});
