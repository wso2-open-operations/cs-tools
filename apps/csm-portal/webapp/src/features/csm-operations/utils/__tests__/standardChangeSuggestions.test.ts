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
  suggestionProposalEvidence,
  suggestionSampleLabel,
  suggestionSampleText,
  suggestionSkippedText,
  SUGGESTION_SKIPPED_ERROR,
  SUGGESTION_SKIPPED_TOO_LARGE,
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

describe("suggestionActions", () => {
  it("every suggestion is proposed or dismissed: none is related to a template", () => {
    expect(suggestionActions(suggestion(), true)).toEqual({ propose: { blockedReason: null }, dismiss: true });
  });
  it("a reader gets none: they cannot propose or dismiss", () => {
    expect(suggestionActions(suggestion(), false)).toEqual({});
  });
  it("a dismissed one is only restored", () => {
    const d = suggestion({ dismissal: { id: "d1", reason: "one-off", dismissedOn: "2026-10-01T00:00:00Z" } });
    expect(suggestionActions(d, true)).toEqual({ restore: true });
  });
  it("says why Propose cannot be used: a near miss, an open proposal, nothing clean to start from", () => {
    expect(suggestionActions(suggestion({ eligible: false, unmet: ["closed 2 times in 12 months (needs 5)"] }), true).propose).toEqual({
      blockedReason: "Not yet a candidate: closed 2 times in 12 months (needs 5).",
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
});

describe("suggestionCriteriaText", () => {
  it("says how many, from what, by which thresholds", () => {
    expect(
      suggestionCriteriaText({
        suggestions: [suggestion()],
        criteria: { windowMonths: 12, minClosed: 5, minSuccessPercent: 90, planSimilarityPercent: 50 },
        scanned: { changeRequests: 41, clusters: 5, eligible: 1, dismissed: 0 },
      }),
    ).toBe(
      "1 suggestion from 41 change requests completed in the last 12 months that were not raised from a template. " +
        "A suggestion is the same change closed at least 5 times, at least 90% of them without a rollback, with no incident linked and no high risk or impact.",
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

describe("a suggestion built from part of its subject", () => {
  it("names how many of how many were compared, and nothing when all were", () => {
    expect(suggestionSampleLabel({ sample: { compared: 500, total: 3000 } })).toBe("Newest 500 of 3,000");
    expect(suggestionSampleText({ sample: { compared: 500, total: 3000 } })).toBe(
      "Based on the newest 500 of 3,000 change requests with this subject; the older ones were not compared.",
    );
    expect(suggestionSampleLabel({ sample: null })).toBeNull();
    expect(suggestionSampleText({})).toBeNull();
  });
});

describe("suggestionSkippedText", () => {
  const skipped = (n: number, reason = SUGGESTION_SKIPPED_TOO_LARGE, from = 0) =>
    Array.from({ length: n }, (_, i) => ({ customer: { id: `a${from + i}`, name: `Customer ${from + i + 1}` }, reason }));
  it("says the customer's suggestions were not computed, not that it has none", () => {
    expect(suggestionSkippedText(skipped(1))).toBe(
      "Customer 1's suggestions were not computed in the latest refresh: it has too many change requests to compute in time.",
    );
  });
  it("names up to five customers, then how many more", () => {
    expect(suggestionSkippedText(skipped(2))).toBe(
      "The suggestions of 2 customers were not computed in the latest refresh (Customer 1, Customer 2): each has too many change requests to compute in time.",
    );
    expect(suggestionSkippedText(skipped(7))).toContain("(Customer 1, Customer 2, Customer 3, Customer 4, Customer 5 and 2 more)");
  });
  it("says an error stopped a customer, not that it is too large", () => {
    expect(suggestionSkippedText(skipped(1, SUGGESTION_SKIPPED_ERROR))).toBe(
      "Customer 1's suggestions were not computed in the latest refresh: an error stopped it, and the next refresh tries again.",
    );
    expect(suggestionSkippedText(skipped(2, SUGGESTION_SKIPPED_ERROR))).toBe(
      "The suggestions of 2 customers were not computed in the latest refresh (Customer 1, Customer 2): an error stopped each, and the next refresh tries again.",
    );
  });
  it("gives each reason its own sentence, too large first", () => {
    expect(suggestionSkippedText([...skipped(1, SUGGESTION_SKIPPED_ERROR), ...skipped(2, SUGGESTION_SKIPPED_TOO_LARGE, 1)])).toBe(
      "The suggestions of 2 customers were not computed in the latest refresh (Customer 2, Customer 3): each has too many change requests to compute in time. " +
        "Customer 1's suggestions were not computed in the latest refresh: an error stopped it, and the next refresh tries again.",
    );
  });
  it("shows a reason it does not know as written", () => {
    expect(suggestionSkippedText(skipped(1, "paused by an operator"))).toBe(
      "Customer 1's suggestions were not computed in the latest refresh: paused by an operator.",
    );
  });
  it("is nothing when none was skipped", () => {
    expect(suggestionSkippedText([])).toBeNull();
    expect(suggestionSkippedText(undefined)).toBeNull();
  });
});
