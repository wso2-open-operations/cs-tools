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

import { describe, expect, it } from "vitest";
import type { CaseAlertSettings } from "../types";
import {
  ADVANCED_GROUPS,
  LADDER_KEYS,
  creTimingRows,
  draftFor,
  teamEmailErrors,
  teamEmailRows,
  teamEmailRowsFor,
  fileOnlyRows,
  formatLadderValue,
  ladderKeysFor,
  ladderValueRequired,
  parseDraft,
  runningLine,
  valuesToCopy,
  type LadderKeySpec,
} from "./caseAlertLadderKeys";

function spec(key: string): LadderKeySpec {
  const s = LADDER_KEYS.find((k) => k.key === key);
  if (!s) throw new Error(`no key ${key}`);
  return s;
}

const draft = { text: "", checked: false, name: "", email: "", map: {} as Record<string, string> };

describe("caseAlertLadderKeys", () => {
  it("lists each key once, on the ladders the design gives it", () => {
    expect(new Set(LADDER_KEYS.map((k) => k.key)).size).toBe(LADDER_KEYS.length);
    expect(ladderKeysFor("CRE")).toHaveLength(31);
    expect(ladderKeysFor("SRE")).toHaveLength(19);
    expect(ladderKeysFor("SRE").map((k) => k.key)).not.toContain("heads.cre");
    expect(ladderKeysFor("CRE").map((k) => k.key)).not.toContain("timing.interval");
  });

  it("writes values for people", () => {
    expect(formatLadderValue(spec("teams.abts"), ["atlas", "castor"])).toBe("atlas, castor");
    expect(formatLadderValue(spec("trigger.priorities"), [])).toBe("any");
    expect(formatLadderValue(spec("trigger.excludeTeams"), [])).toBe("none");
    expect(formatLadderValue(spec("trigger.requireKnownTeam"), false)).toBe("No");
    expect(formatLadderValue(spec("alertDuty.perTeam"), 0)).toBe("0 (all of them)");
    expect(formatLadderValue(spec("teams.unassignedTeamLead"), "pool")).toBe("A lead from the pool");
    expect(formatLadderValue(spec("teams.rotaMembersToCall"), { LK_EVENING: 7 })).toBe("LK_EVENING=7");
    expect(formatLadderValue(spec("teams.rotaMembersToCall"), {})).toBe("no cap");
    expect(formatLadderValue(spec("heads.cre"), { name: "N", email: "n@x", phone: "+94770000000" })).toBe("N <n@x>");
    expect(formatLadderValue(spec("timing.interval"), undefined)).toBe("—");
  });

  it("never shows a link", () => {
    expect(formatLadderValue(spec("teams.leadership"), "https://chat.googleapis.com/v1/x?token=t")).toBe(
      "a link (not shown)",
    );
    expect(formatLadderValue(undefined, { url: "https://chat.googleapis.com/v1/x" })).not.toContain("googleapis");
  });

  it("reads each editor back into the value's own shape", () => {
    expect(parseDraft(spec("teams.abts"), { ...draft, text: " atlas,, castor " })).toEqual({
      ok: true,
      value: ["atlas", "castor"],
    });
    expect(parseDraft(spec("trigger.teams"), draft)).toEqual({ ok: true, value: [] });
    expect(parseDraft(spec("teams.rotaMembersToCall"), { ...draft, text: "lk_evening=7, USA = 2" })).toEqual({
      ok: true,
      value: { LK_EVENING: 7, USA: 2 },
    });
    expect(parseDraft(spec("teams.rotaMembersToCall"), { ...draft, text: "LK seven" }).ok).toBe(false);
    expect(parseDraft(spec("teams.teamLeadsToCall"), { ...draft, text: "3" })).toEqual({ ok: true, value: 3 });
    expect(parseDraft(spec("teams.teamLeadsToCall"), { ...draft, text: "three" }).ok).toBe(false);
    expect(parseDraft(spec("heads.cs"), { ...draft, name: "N" }).ok).toBe(false);
    expect(parseDraft(spec("timing.interval"), { ...draft, text: " 5m " })).toEqual({ ok: true, value: "5m" });
    // One field per team: blank means no Cc and is left out.
    expect(
      parseDraft(spec("email.teamLists"), {
        ...draft,
        map: { atlas: " atlas-team@example.com ", castor: "", americas: "americas@example.com" },
      }),
    ).toEqual({ ok: true, value: { atlas: "atlas-team@example.com", americas: "americas@example.com" } });
    expect(parseDraft(spec("email.teamLists"), { ...draft, map: { atlas: "atlas team" } }).ok).toBe(false);
    expect(parseDraft(spec("email.teamLists"), { ...draft, map: { atlas: "a@b@c" } }).ok).toBe(false);
    expect(teamEmailErrors({ ...draft, map: { atlas: "a@b@c", vega: "ok@example.com", castor: "" } })).toEqual({
      atlas: "Enter one email address, like team-list@example.com.",
    });
    expect(parseDraft(spec("email.teamLists"), draft)).toEqual({ ok: true, value: {} });
    expect(draftFor(spec("email.teamLists"), { Atlas: "atlas-team@example.com" }).map).toEqual({
      atlas: "atlas-team@example.com",
    });
    expect(parseDraft(spec("email.creTeamList"), { ...draft, text: " cre@example.com " })).toEqual({
      ok: true,
      value: "cre@example.com",
    });
    expect(parseDraft(spec("email.csTeamList"), { ...draft, text: "CS Team" }).ok).toBe(false);
    // Empty means no Cc, so a list can be cleared.
    expect(parseDraft(spec("email.creTeamList"), { ...draft, text: "  " })).toEqual({ ok: true, value: "" });
    expect(parseDraft(spec("email.csTeamList"), draft)).toEqual({ ok: true, value: "" });
    // Other text values still need one.
    expect(parseDraft(spec("teams.leadership"), draft).ok).toBe(false);
    expect(draftFor(spec("teams.rotaMembersToCall"), { LK: 2, USA: 1 }).text).toBe("LK=2, USA=1");
    expect(draftFor(spec("onLeave"), undefined).text).toBe("skip");
  });

  it("marks a value required exactly when an empty one is refused", () => {
    for (const s of LADDER_KEYS) {
      expect(ladderValueRequired(s), s.key).toBe(!parseDraft(s, draft).ok);
    }
    // A list, SHIFT=n pairs and a team map may be emptied; a number may not.
    expect(ladderValueRequired(spec("teams.abts"))).toBe(false);
    expect(ladderValueRequired(spec("teams.teamLeadsToCall"))).toBe(true);
    // The per-team mailing lists and an email box (empty means no Cc) may be emptied too.
    expect(ladderValueRequired(spec("email.teamLists"))).toBe(false);
    expect(ladderValueRequired(spec("email.creTeamList"))).toBe(false);
  });

  it("words the file-only settings, and leaves the heads row to CRE", () => {
    const f = { channel: "chat", "safety.maxCallsPerLadder": 4, "safety.allowedNumbers": 1, "phones.source": "x" };
    const rows = Object.fromEntries(fileOnlyRows("SRE", f).map((r) => [r.label, r.value]));
    expect(rows).toMatchObject({
      Channel: "Chat only",
      "Max calls per ladder": "4",
      "Highest step": "no cap",
      "Allowed numbers": "only 1 listed number",
      "Alerts on": "—",
      "Phone numbers from": "x",
    });
    expect(rows).not.toHaveProperty("Call the heads when nobody below can be called");
    expect(fileOnlyRows("CRE", f).map((r) => r.label)).toContain("Call the heads when nobody below can be called");
  });

  it("words the CRE-only file settings: the stopped reply, the rung emails and the rule table", () => {
    const f = {
      ackChat: false,
      "email.enabled": true,
      "email.teamLists": 2,
      "email.creTeamList": true,
      "email.csTeamList": false,
      "rules.custom": false,
    };
    const cre = Object.fromEntries(fileOnlyRows("CRE", f).map((r) => [r.label, r.value]));
    expect(cre).toMatchObject({
      "Reply in the thread when a chain stops": "No",
      "Email per step": "On (2 team lists, CRE list set, CS list not set)",
      "Who each step calls": "Built-in rules (Case Paging Rules sheet)",
    });
    expect(fileOnlyRows("CRE", { "email.enabled": false, "rules.custom": true }).map((r) => r.value)).toEqual(
      expect.arrayContaining(["Off", "The file's own rule table"]),
    );
    // SRE has no rung emails and no rule table.
    const sre = fileOnlyRows("SRE", f).map((r) => r.label);
    expect(sre).toContain("Reply in the thread when a chain stops");
    expect(sre).not.toContain("Email per step");
    expect(sre).not.toContain("Who each step calls");
  });

  it("words the CRE timing table from the alert service's minutes", () => {
    const rows = creTimingRows({
      S0: [[0], [1, 2, 3], [4, 5, 6], [8, 9, 10], [12, 13, 14]],
      S2: [[9, 12], [15, 19, 23], [30, 34, 38], [45, 49, 53], [60, 64, 68]],
    });
    expect(rows).toEqual([
      { severity: "S0", steps: ["+0", "+1, +2, +3", "+4, +5, +6", "+8, +9, +10", "+12, +13, +14"], ends: "+14 min" },
      { severity: "S2", steps: ["+9, +12", "+15, +19, +23", "+30, +34, +38", "+45, +49, +53", "+60, +64, +68"], ends: "+68 min" },
    ]);
    expect(creTimingRows(undefined)).toEqual([]);
    expect(creTimingRows({ S1: [[], [9]] })[0].steps).toEqual(["—", "+9", "—", "—", "—"]);
  });

  it("leaves out of the file-only block what the tab now edits", () => {
    const f = { enabled: true, channel: "both", "safety.maxCallsPerLadder": 4, "safety.allowedNumbers": 0 };
    const onTab = new Set(["enabled", "channel", "safety.maxCallsPerLadder", "safety.callWithoutVerifiedLeads"]);
    const labels = fileOnlyRows("SRE", f, onTab).map((r) => r.label);
    expect(labels).toEqual([
      "Highest step",
      "Allowed numbers",
      "Phone numbers from",
      "Reply in the thread when a chain stops",
    ]);
    // An alert service that does not report them on the tab yet: all still shown here.
    expect(fileOnlyRows("SRE", f).map((r) => r.label)).toContain("Alerts on");
  });

  it("puts only each family's switch and channel under On/off and channel, and the call limits on CRE and SRE under Advanced", () => {
    const keys = (l: "CRE" | "SRE" | "SME", group: string) =>
      ladderKeysFor(l)
        .filter((k) => k.group === group)
        .map((k) => k.key);
    for (const l of ["CRE", "SRE", "SME"] as const) {
      expect(keys(l, "On/off and channel")).toEqual(["enabled", "channel"]);
    }
    expect(keys("SME", "Call limits")).toEqual([]);
    expect(keys("CRE", "Call limits")).toEqual([
      "safety.maxCallsPerLadder",
      "safety.callWithoutVerifiedLeads",
      "safety.callHeadsWithoutLowerTiers",
    ]);
    expect(keys("SRE", "Call limits")).toEqual(keys("CRE", "Call limits"));
    expect(ADVANCED_GROUPS[0]).toBe("Call limits");
    expect(parseDraft(spec("channel"), { ...draft, text: "chat" })).toEqual({ ok: true, value: "chat" });
    expect(parseDraft(spec("safety.maxCallsPerLadder"), { ...draft, text: "4" })).toEqual({ ok: true, value: 4 });
    expect(formatLadderValue(spec("channel"), "both")).toBe("Calls and chat");
  });

  it("copies the file's named heads without a phone, and skips an unnamed one", () => {
    const s = {
      values: [],
      running: {
        reportedAt: "2026-10-13T10:00:00Z",
        mode: "shadow",
        ladders: {
          CRE: {
            "heads.cre": { value: { name: "N", email: "n@x", phone: "+94770000000" }, source: "file" },
            "heads.cs": { value: { name: "", email: "" }, source: "file" },
            "teams.americas": { value: null, source: "file" },
          },
        },
      },
    } as unknown as CaseAlertSettings;
    expect(valuesToCopy(s, "CRE")).toEqual({ "heads.cre": { name: "N", email: "n@x" } });
    expect(valuesToCopy(s, "SRE")).toEqual({});
  });
});

describe("teamEmailRows", () => {
  const base = (over: Partial<CaseAlertSettings>): CaseAlertSettings =>
    ({
      canManage: { view: true, roomLadders: ["CRE"], routeLadders: [] },
      rooms: [],
      routes: [],
      values: [],
      routeOptions: {
        groups: [],
        rotas: [],
        teams: [
          { key: "atlas", name: "Atlas", family: "CRE" },
          { key: "vega", name: "Vega", family: "CRE" },
          { key: "americas", name: "Americas", family: "CRE" },
          { key: "apollo", name: "Apollo", family: "SRE" },
        ],
      },
      running: {
        mode: "db",
        ladders: {
          CRE: {
            "teams.abts": { value: ["atlas", "vega"], source: "file" },
            "teams.americas": { value: "americas", source: "file" },
          },
        },
      },
      ...over,
    }) as CaseAlertSettings;

  it("offers each ABT team, then the Americas team at night, by the Team Schedule's names", () => {
    expect(teamEmailRows(base({}), "CRE")).toEqual([
      { key: "atlas", name: "Atlas" },
      { key: "vega", name: "Vega" },
      { key: "americas", name: "Americas", note: "Cc'd at night (21:00-06:00 IST)" },
    ]);
  });

  it("follows a saved ABT list over the one in use", () => {
    const s = base({
      values: [{ id: "x", ladder: "CRE", key: "teams.abts", value: ["vega"], updatedBy: "a@b", updatedOn: "2026-10-23T00:00:00Z" }],
    });
    expect(teamEmailRows(s, "CRE").map((r) => r.key)).toEqual(["vega", "americas"]);
  });

  it("offers the family's teams when no ABT list is known", () => {
    const s = base({ running: { mode: "db", ladders: { CRE: {} } } });
    expect(teamEmailRows(s, "CRE").map((r) => r.key)).toEqual(["atlas", "vega", "americas"]);
  });

  it("marks a team the value names that is not offered, so its list can be seen and cleared", () => {
    const rows = teamEmailRowsFor(teamEmailRows(base({}), "CRE"), { atlas: "a@example.com", castor: "c@example.com" });
    expect(rows.at(-1)).toEqual({ key: "castor", name: "Castor", note: "No longer an ABT team: this list is not used", stale: true });
  });
});

describe("runningLine", () => {
  const settings = (running: CaseAlertSettings["running"]) => ({ running }) as CaseAlertSettings;

  it("says when the alert service reported, and in which mode", () => {
    const line = runningLine(settings({ reportedAt: "2026-10-13T10:00:00Z", mode: "db", ladders: {} }));
    expect(line).toMatch(/^"In use" is what the alert service reported at .+, in db mode\.$/);
    expect(line).not.toContain("Invalid Date");
  });

  it("leaves the time out when the report has none", () => {
    expect(runningLine(settings({ mode: "shadow", ladders: {} }))).toBe(
      '"In use" is what the alert service reported, in shadow mode. In shadow mode it still uses its file; saved values take over in db mode.',
    );
  });

  it("says it hasn't reported yet when there is no report", () => {
    expect(runningLine(settings(undefined))).toMatch(/hasn't reported what it uses yet/);
  });
});
