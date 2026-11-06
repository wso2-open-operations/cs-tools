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
import type { BeStandardChangeSuggestion } from "@api/backend/types";
import {
  suggestionActions,
  suggestionCriteriaText,
  suggestionMatchLabel,
  suggestionProposalEvidence,
  suggestionRevisionEvidence,
  suggestionUpdatedText,
} from "@features/csm-operations/utils/standardChangeSuggestions";
import {
  standardChangesPath,
  standardChangesSearch,
  standardChangesShow,
} from "@features/csm-operations/utils/standardChanges";

function suggestion(over: Partial<BeStandardChangeSuggestion> = {}): BeStandardChangeSuggestion {
  return {
    key: "deploy capps pekin prod",
    customer: { id: "acct-1", name: "Acme" },
    subject: "Deploy CApps to Pekin prod",
    memberCount: 4,
    closedCount: 4,
    rollbackCount: 0,
    successRate: 1,
    incidents: [],
    risk: { low: 4, moderate: 0, high: 0, unknown: 0 },
    impact: { low: 4, moderate: 0, high: 0, unknown: 0 },
    firstCompletedOn: "2026-09-01T00:00:00Z",
    lastCompletedOn: "2026-10-30T00:00:00Z",
    eligible: true,
    unmet: [],
    match: "new",
    template: null,
    proposeFrom: { id: "cr-1", name: "CHG0000001" },
    evidence: [
      { id: "cr-2", name: "CHG0000002" },
      { id: "cr-3", name: "CHG0000003" },
    ],
    openProposals: [],
    members: [],
    dismissal: null,
    ...over,
  };
}

const TEMPLATE = { id: "tpl-1", number: "STD0000001", name: "Deploy CApps", state: "active", score: 0.9 };

describe("suggestionMatchLabel", () => {
  it("names the template it relates to", () => {
    expect(suggestionMatchLabel({ match: "covered", template: TEMPLATE })).toBe("Covered by STD0000001");
    expect(suggestionMatchLabel({ match: "close", template: TEMPLATE })).toBe("Close to STD0000001: revise it");
    expect(suggestionMatchLabel({ match: "new", template: null })).toBe("New");
  });
});

describe("suggestionActions", () => {
  it("a new one is proposed and dismissed", () => {
    expect(suggestionActions(suggestion(), true)).toEqual({ propose: { blockedReason: null }, dismiss: true });
  });
  it("a close one is a revision of its template", () => {
    expect(suggestionActions(suggestion({ match: "close", template: TEMPLATE }), true)).toEqual({
      revise: { blockedReason: null },
      dismiss: true,
    });
  });
  it("a covered one opens its template", () => {
    expect(suggestionActions(suggestion({ match: "covered", template: TEMPLATE }), true)).toEqual({ openTemplate: true, dismiss: true });
  });
  it("a reader only opens templates", () => {
    expect(suggestionActions(suggestion(), false)).toEqual({});
    expect(suggestionActions(suggestion({ match: "covered", template: TEMPLATE }), false)).toEqual({ openTemplate: true });
  });
  it("a dismissed one is only restored", () => {
    const d = suggestion({ dismissal: { id: "d1", reason: "one-off", dismissedOn: "2026-10-01T00:00:00Z" } });
    expect(suggestionActions(d, true)).toEqual({ restore: true });
  });
  it("says why Propose cannot be used: a near miss, an open proposal, nothing clean to start from", () => {
    expect(suggestionActions(suggestion({ eligible: false, unmet: ["closed 2 times in 12 months (needs 3)"] }), true).propose).toEqual({
      blockedReason: "Not yet a candidate: closed 2 times in 12 months (needs 3).",
    });
    expect(suggestionActions(suggestion({ openProposals: [{ id: "p1", name: "SCP0000004" }] }), true).propose).toEqual({
      blockedReason: "Already being proposed in SCP0000004.",
    });
    expect(suggestionActions(suggestion({ proposeFrom: null }), true).propose?.blockedReason).toMatch(/without a deviation/);
  });
});

describe("the evidence a proposal cites", () => {
  it("starts with the source, then the others, within 100", () => {
    expect(suggestionProposalEvidence(suggestion())).toEqual(["cr-1", "cr-2", "cr-3"]);
    const many = Array.from({ length: 120 }, (_, i) => ({ id: `cr-${i + 10}`, name: `CHG${i}` }));
    expect(suggestionProposalEvidence(suggestion({ evidence: many }))).toHaveLength(100);
    expect(suggestionProposalEvidence(suggestion({ proposeFrom: null }))).toEqual([]);
  });
  it("a revision cites every closed member, a clean one or not", () => {
    expect(suggestionRevisionEvidence(suggestion({ proposeFrom: null }))).toEqual(["cr-2", "cr-3"]);
    expect(suggestionRevisionEvidence(suggestion())).toEqual(["cr-1", "cr-2", "cr-3"]);
  });
});

describe("suggestionCriteriaText", () => {
  it("says how many, from what, by which thresholds", () => {
    expect(
      suggestionCriteriaText({
        suggestions: [suggestion()],
        criteria: { windowMonths: 12, minClosed: 3, minSuccessPercent: 90, planSimilarityPercent: 50, coveredPercent: 80, closePercent: 50 },
        scanned: { changeRequests: 41, clusters: 5, eligible: 1, dismissed: 0 },
      }),
    ).toBe(
      "1 suggestion from 41 change requests completed in the last 12 months that were not raised from a template. " +
        "A suggestion is the same change closed at least 3 times, at least 90% of them without a rollback, with no incident linked and no high risk or impact.",
    );
  });
});

describe("suggestionUpdatedText", () => {
  it("says when the snapshot was published, in the time zone given", () => {
    expect(suggestionUpdatedText("2026-11-06T08:30:00Z", "Asia/Colombo")).toBe("Updated Nov 6, 02:00 PM");
    expect(suggestionUpdatedText(null)).toBeNull();
    expect(suggestionUpdatedText(undefined)).toBeNull();
  });
});

describe("the Suggested place in the URL", () => {
  it("is show=suggested, read back and kept by the search helper", () => {
    expect(standardChangesPath("suggested")).toBe("/operations/change-requests?view=standard&show=suggested");
    expect(standardChangesShow(new URLSearchParams("view=standard&show=suggested"))).toBe("suggested");
    expect(standardChangesSearch(new URLSearchParams("view=standard&show=suggested&crState=new"), "suggested").toString()).toBe(
      "view=standard&show=suggested&crState=new",
    );
  });
});
