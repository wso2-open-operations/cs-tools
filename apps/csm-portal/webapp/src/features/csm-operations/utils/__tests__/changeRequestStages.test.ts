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
import type { BeChangeRequestState } from "@api/backend/types";
import {
  buildChangeRequestLifecycle,
  CHANGE_REQUEST_LIFECYCLE_ORDER,
  CHANGE_REQUEST_STAGE_CAPTIONS,
  changeRequestLifecycleStatusText,
  isChangeRequestLifecycleState,
  type BuildChangeRequestLifecycleInput,
  type ChangeRequestLifecycleStatus,
} from "@features/csm-operations/utils/changeRequestStages";

/** The customer portal's order (`CHANGE_REQUEST_STATE_ORDER`), hard-coded on purpose. */
const CUSTOMER_PORTAL_ORDER: BeChangeRequestState[] = [
  "new",
  "assess",
  "authorize",
  "customer_approval",
  "scheduled",
  "implement",
  "review",
  "customer_review",
  "rollback",
  "closed",
  "canceled",
];

/** The customer portal's captions, hard-coded on purpose (`buildChangeRequestWorkflowStages`). */
const CUSTOMER_PORTAL_CAPTIONS: Record<BeChangeRequestState, string> = {
  new: "Change request created",
  assess: "Technical assessment completed",
  authorize: "Internal authorization obtained",
  customer_approval: "Customer approval received",
  scheduled: "Maintenance window scheduled",
  implement: "Change implementation",
  review: "Internal review",
  customer_review: "Customer validation",
  rollback: "Change rollback if needed",
  closed: "Change request completed",
  canceled: "Change request canceled",
};

const LABELS: Record<BeChangeRequestState, string> = {
  new: "New",
  assess: "Assess",
  authorize: "Authorize",
  customer_approval: "Customer Approval",
  scheduled: "Scheduled",
  implement: "Implement",
  review: "Review",
  customer_review: "Customer Review",
  rollback: "Rollback",
  closed: "Closed",
  canceled: "Canceled",
};

const SHORT: Record<string, ChangeRequestLifecycleStatus> = {
  d: "done",
  c: "current",
  p: "pending",
  n: "not-taken",
  u: "unrecorded",
  r: "rejected",
};

/** "d d c p ..." -> statuses in the customer portal's order, one per stage. */
function row(spec: string): ChangeRequestLifecycleStatus[] {
  return spec.split(" ").map((c) => SHORT[c]!);
}

function statuses(input: BuildChangeRequestLifecycleInput): ChangeRequestLifecycleStatus[] {
  return buildChangeRequestLifecycle(input).map((n) => n.status);
}

/** Statuses keyed by stage, so a flag combination can be compared against the full line. */
function byKey(input: BuildChangeRequestLifecycleInput): Partial<Record<BeChangeRequestState, ChangeRequestLifecycleStatus>> {
  return Object.fromEntries(buildChangeRequestLifecycle(input).map((n) => [n.key, n.status]));
}

// Columns: new assess authorize customer_approval scheduled implement review customer_review rollback closed canceled
const FULL_LINE: Record<string, string> = {
  new: "c p p p p p p p n p n",
  assess: "d c p p p p p p n p n",
  authorize: "d d c p p p p p n p n",
  customer_approval: "d d d c p p p p n p n",
  scheduled: "d d d d c p p p n p n",
  implement: "d d d d d c p p n p n",
  review: "d d d d d d c p n p n",
  customer_review: "d d d d d d d c n p n",
  closed: "d d d d d d d d n c n",
};

const FLAGS: Array<boolean | undefined> = [true, false, undefined];

describe("the workflow's stages", () => {
  it("are the customer portal's eleven, in its order", () => {
    expect([...CHANGE_REQUEST_LIFECYCLE_ORDER]).toEqual(CUSTOMER_PORTAL_ORDER);
    expect(buildChangeRequestLifecycle({ state: "new" }).map((n) => n.key)).toEqual(CUSTOMER_PORTAL_ORDER);
  });

  it("carry the customer portal's captions verbatim, and the CSM portal's state labels", () => {
    expect(CHANGE_REQUEST_STAGE_CAPTIONS).toEqual(CUSTOMER_PORTAL_CAPTIONS);
    for (const node of buildChangeRequestLifecycle({ state: "new" })) {
      expect(node.caption).toBe(CUSTOMER_PORTAL_CAPTIONS[node.key]);
      expect(node.label).toBe(LABELS[node.key]);
    }
  });

  it("recognises exactly the eleven states", () => {
    for (const s of CUSTOMER_PORTAL_ORDER) expect(isChangeRequestLifecycleState(s)).toBe(true);
    for (const s of ["", null, undefined, "on_hold", "NEW", "Closed"]) expect(isChangeRequestLifecycleState(s)).toBe(false);
  });
});

describe("a change on the path (new ... closed), checkboxes unknown", () => {
  for (const [state, spec] of Object.entries(FULL_LINE)) {
    it(`${state}: ${spec}`, () => {
      expect(statuses({ state })).toEqual(row(spec));
    });
  }

  it("never marks Rollback or Canceled done or current while the change is on the path", () => {
    for (const state of Object.keys(FULL_LINE)) {
      const s = byKey({ state });
      expect(s.rollback).toBe("not-taken");
      expect(s.canceled).toBe("not-taken");
    }
  });

  it("marks exactly one stage current", () => {
    for (const state of Object.keys(FULL_LINE)) {
      expect(statuses({ state }).filter((x) => x === "current")).toHaveLength(1);
    }
  });
});

describe("a change that was rolled back", () => {
  const proof = [{ stage: "Customer Review", status: "APPROVED" }];

  it("marks Rollback current and every stage through Review done", () => {
    const s = byKey({ state: "rollback", approvals: [], hasCustomerContacts: true });
    expect(s).toEqual({
      new: "done",
      assess: "done",
      authorize: "done",
      customer_approval: "done",
      scheduled: "done",
      implement: "done",
      review: "done",
      customer_review: "not-taken",
      rollback: "current",
      closed: "not-taken",
      canceled: "not-taken",
    });
  });

  it("marks Customer Review done only when a Customer Review stage proves it was entered", () => {
    expect(statuses({ state: "rollback", approvals: proof })).toEqual(row("d d d d d d d d c n n"));
    expect(statuses({ state: "rollback", approvals: [], hasCustomerContacts: true })).toEqual(
      row("d d d d d d d n c n n"),
    );
  });

  it("counts a Customer Review stage whatever its outcome (a rejection aside), and whatever the name spelling", () => {
    for (const status of ["APPROVED", "PENDING", "CANCELLED", "REQUESTED"]) {
      expect(byKey({ state: "rollback", approvals: [{ stage: "Customer Review", status }] }).customer_review).toBe("done");
    }
    expect(byKey({ state: "rollback", approvals: [{ stage: "customer_review", status: "PENDING" }] }).customer_review).toBe("done");
  });

  it("marks Customer Review rejected when the customer rejected it, which is what rolls the change back", () => {
    for (const status of ["REJECTED", " rejected "]) {
      expect(statuses({ state: "rollback", approvals: [{ stage: "Customer Review", status }] })).toEqual(
        row("d d d d d d d r c n n"),
      );
    }
    // Whichever stage row says it: one rejection among the rows is enough.
    const rows = [
      { stage: "Customer Review", status: "CANCELLED" },
      { stage: "Customer Review", status: "REJECTED" },
    ];
    expect(byKey({ state: "rollback", approvals: rows }).customer_review).toBe("rejected");
  });

  it("is not moved by other stages' rows, a Review row included", () => {
    const others = [
      { stage: "Peer Approval", status: "APPROVED" },
      { stage: "CAB Approval", status: "APPROVED" },
      { stage: "Customer Approval", status: "APPROVED" },
      { stage: "Review", status: "APPROVED" },
      { stage: "Customer Approval", status: "REJECTED" },
    ];
    expect(byKey({ state: "rollback", approvals: others, hasCustomerContacts: true }).customer_review).toBe("not-taken");
  });

  describe("Customer Review without a stage row", () => {
    it("is not taken only when the project has customer contacts, who would have been asked", () => {
      expect(byKey({ state: "rollback", approvals: [], hasCustomerContacts: true }).customer_review).toBe("not-taken");
    });

    it("cannot be told when the project has no contacts: a rollback from Customer Review leaves no row either", () => {
      expect(byKey({ state: "rollback", approvals: [], hasCustomerContacts: false }).customer_review).toBe("unrecorded");
    });

    it("cannot be told when whether it has contacts is not known", () => {
      expect(byKey({ state: "rollback", approvals: [] }).customer_review).toBe("unrecorded");
      expect(byKey({ state: "rollback", approvals: [], hasCustomerContacts: undefined }).customer_review).toBe("unrecorded");
    });

    it("keeps the other stages as they are whatever the contacts say", () => {
      const without = byKey({ state: "rollback", approvals: [], hasCustomerContacts: false });
      const withContacts = byKey({ state: "rollback", approvals: [], hasCustomerContacts: true });
      expect({ ...without, customer_review: "x" }).toEqual({ ...withContacts, customer_review: "x" });
    });
  });

  it("cannot say about Customer Review while the approvals are not loaded", () => {
    expect(byKey({ state: "rollback" }).customer_review).toBe("unrecorded");
    expect(byKey({ state: "rollback", approvals: undefined, hasCustomerContacts: true }).customer_review).toBe("unrecorded");
  });

  it("leaves Customer Review off the line when it is not required", () => {
    const nodes = buildChangeRequestLifecycle({ state: "rollback", customerReviewRequired: false, approvals: [] });
    expect(nodes.map((n) => n.key)).not.toContain("customer_review");
    expect(nodes.at(-4)!.key).toBe("review");
    expect(nodes.find((n) => n.key === "rollback")!.status).toBe("current");
  });
});

describe("a change that was canceled", () => {
  it("marks Canceled current and, with no stage rows, nothing done", () => {
    for (const approvals of [undefined, []]) {
      expect(statuses({ state: "canceled", approvals })).toEqual(row("u u u u u u u u n n c"));
    }
  });

  it("never marks Rollback or Closed (not taken) as anything but not-taken", () => {
    const s = byKey({ state: "canceled", approvals: [{ stage: "Review", status: "APPROVED" }] });
    expect(s.rollback).toBe("not-taken");
    expect(s.closed).toBe("not-taken");
    expect(s.canceled).toBe("current");
  });

  describe("approvals that prove a stage was passed", () => {
    it("a stage row proves every stage BEFORE its state was passed, not its own", () => {
      // CAB Approval still pending: the change was in Authorize, so New and Assess are behind it.
      expect(
        statuses({
          state: "canceled",
          approvals: [
            { stage: "Peer Approval", status: "APPROVED" },
            { stage: "CAB Approval", status: "PENDING" },
          ],
        }),
      ).toEqual(row("d d u u u u u u n n c"));
    });

    it("an APPROVED stage proves its own state was passed too", () => {
      expect(
        statuses({
          state: "canceled",
          approvals: [
            { stage: "Peer Approval", status: "APPROVED" },
            { stage: "CAB Approval", status: "APPROVED" },
          ],
        }),
      ).toEqual(row("d d d u u u u u n n c"));
    });

    it("a Customer Approval row proves Authorize was passed; approved, it proves Customer Approval too", () => {
      const waiting = [{ stage: "Customer Approval", status: "PENDING" }];
      expect(statuses({ state: "canceled", approvals: waiting })).toEqual(row("d d d u u u u u n n c"));
      const approved = [{ stage: "Customer Approval", status: "APPROVED" }];
      expect(statuses({ state: "canceled", approvals: approved })).toEqual(row("d d d d u u u u n n c"));
    });

    it("an internal stage's rejection proves only that the change reached it (it leaves the change's state alone)", () => {
      for (const stage of ["Peer Approval", "CAB Approval", "Review", "Customer Review"]) {
        const reached = statuses({ state: "canceled", approvals: [{ stage, status: "REJECTED" }] });
        expect(reached, stage).not.toContain("rejected");
        expect(reached.at(-1), stage).toBe("current");
      }
      expect(statuses({ state: "canceled", approvals: [{ stage: "CAB Approval", status: "REJECTED" }] })).toEqual(
        row("d d u u u u u u n n c"),
      );
    });

    it("a Review stage's approval proves the change reached Review, not that it left it", () => {
      // Approving the Review stage only records the decision: the change stays in Review
      // until an engineer moves it on, and it may be canceled right there.
      expect(statuses({ state: "canceled", approvals: [{ stage: "Review", status: "APPROVED" }] })).toEqual(
        row("d d d d d d u u n n c"),
      );
      expect(
        statuses({
          state: "canceled",
          approvals: [
            { stage: "Peer Approval", status: "APPROVED" },
            { stage: "CAB Approval", status: "APPROVED" },
            { stage: "Review", status: "APPROVED" },
          ],
        }),
      ).toEqual(row("d d d d d d u u n n c"));
    });

    it("a Review row proves Scheduled and Implement were passed", () => {
      expect(statuses({ state: "canceled", approvals: [{ stage: "Review", status: "PENDING" }] })).toEqual(
        row("d d d d d d u u n n c"),
      );
    });

    it("a Customer Review row proves Review was passed", () => {
      expect(statuses({ state: "canceled", approvals: [{ stage: "Customer Review", status: "CANCELLED" }] })).toEqual(
        row("d d d d d d d u n n c"),
      );
    });

    it("reads the strongest proof across all rows, in any order", () => {
      const rows = [
        { stage: "Customer Review", status: "REQUESTED" },
        { stage: "Peer Approval", status: "APPROVED" },
        { stage: "Review", status: "APPROVED" },
      ];
      expect(statuses({ state: "canceled", approvals: rows })).toEqual(row("d d d d d d d u n n c"));
      expect(statuses({ state: "canceled", approvals: [...rows].reverse() })).toEqual(row("d d d d d d d u n n c"));
    });

    it("knows every spelling of a stage the backend may send", () => {
      const done = (stage: string, status = "APPROVED"): number =>
        statuses({ state: "canceled", approvals: [{ stage, status }] }).filter((x) => x === "done").length;
      expect(done("Assess")).toBe(2); // Peer Approval: New, Assess
      expect(done("Authorize")).toBe(3);
      expect(done("CAB Approval")).toBe(3);
      // An Emergency change raised before ECAB was retired still carries such a stage: read as CAB's.
      expect(done("ECAB Approval")).toBe(3);
      expect(done("Emergency CAB")).toBe(3);
      expect(done("peer approval")).toBe(2);
      expect(done("Customer Approval", " approved ")).toBe(4);
    });

    it("ignores a stage it does not know, and a status in any case", () => {
      expect(statuses({ state: "canceled", approvals: [{ stage: "Legal sign-off", status: "APPROVED" }] })).toEqual(
        row("u u u u u u u u n n c"),
      );
    });

    it("never marks a stage done that is off the line, and skips it when counting", () => {
      const nodes = buildChangeRequestLifecycle({
        state: "canceled",
        customerApprovalRequired: false,
        customerReviewRequired: false,
        // Proves Review was passed, though neither customer stage is on the line.
        approvals: [{ stage: "Customer Review", status: "CANCELLED" }],
      });
      expect(nodes.map((n) => [n.key, n.status])).toEqual([
        ["new", "done"],
        ["assess", "done"],
        ["authorize", "done"],
        ["scheduled", "done"],
        ["implement", "done"],
        ["review", "done"],
        ["rollback", "not-taken"],
        ["closed", "not-taken"],
        ["canceled", "current"],
      ]);
    });
  });

  describe("a customer's rejection at Customer Approval", () => {
    const rejection = [
      { stage: "Peer Approval", status: "APPROVED" },
      { stage: "CAB Approval", status: "APPROVED" },
      { stage: "Customer Approval", status: "REJECTED" },
    ];

    it("proves where the change ended: the stages before are done, Customer Approval is rejected, every stage after is not taken", () => {
      expect(statuses({ state: "canceled", approvals: rejection })).toEqual(row("d d d r n n n n n n c"));
    });

    it("needs nothing but the rejected Customer Approval stage to prove it", () => {
      expect(
        statuses({ state: "canceled", approvals: [{ stage: "Customer Approval", status: " rejected " }] }),
      ).toEqual(row("d d d r n n n n n n c"));
    });

    it("leaves the optional Customer Review off the line when it is not required, the later stages still not taken", () => {
      const nodes = buildChangeRequestLifecycle({
        state: "canceled",
        customerReviewRequired: false,
        approvals: rejection,
      });
      expect(nodes.map((n) => [n.key, n.status])).toEqual([
        ["new", "done"],
        ["assess", "done"],
        ["authorize", "done"],
        ["customer_approval", "rejected"],
        ["scheduled", "not-taken"],
        ["implement", "not-taken"],
        ["review", "not-taken"],
        ["rollback", "not-taken"],
        ["closed", "not-taken"],
        ["canceled", "current"],
      ]);
    });

    it("is never read as history not recorded", () => {
      expect(statuses({ state: "canceled", approvals: rejection })).not.toContain("unrecorded");
    });

    it("does not apply to a change that is not canceled", () => {
      for (const state of Object.keys(FULL_LINE)) {
        expect(statuses({ state, approvals: rejection })).toEqual(statuses({ state }));
      }
    });

    it("yields to a recorded customer approval (the change cannot have both)", () => {
      expect(statuses({ state: "canceled", approvals: rejection, customerApproved: true })).toEqual(
        row("d d d d u u u u n n c"),
      );
    });
  });

  describe("a recorded customer approval (hasCustomerApproved)", () => {
    it("proves Customer Approval was passed with no stage row at all, the bypass of a project without contacts", () => {
      const internal = [
        { stage: "Peer Approval", status: "APPROVED" },
        { stage: "CAB Approval", status: "APPROVED" },
      ];
      expect(statuses({ state: "canceled", approvals: internal, customerApproved: true })).toEqual(
        row("d d d d u u u u n n c"),
      );
      expect(statuses({ state: "canceled", approvals: internal, customerApproved: false })).toEqual(
        row("d d d u u u u u n n c"),
      );
      expect(statuses({ state: "canceled", approvals: internal })).toEqual(row("d d d u u u u u n n c"));
    });

    it("proves nothing more than Customer Approval: the change may have been canceled in Scheduled", () => {
      expect(statuses({ state: "canceled", approvals: [], customerApproved: true })).toEqual(row("d d d d u u u u n n c"));
    });

    it("counts alongside the stronger proof of a later stage", () => {
      expect(
        statuses({ state: "canceled", approvals: [{ stage: "Review", status: "PENDING" }], customerApproved: true }),
      ).toEqual(row("d d d d d d u u n n c"));
    });

    it("is read only for a canceled change", () => {
      for (const state of Object.keys(FULL_LINE)) {
        expect(statuses({ state, customerApproved: true })).toEqual(statuses({ state }));
      }
    });
  });

  it("uses the approvals only for a canceled or rolled-back change", () => {
    const rows = [{ stage: "Review", status: "APPROVED" }];
    for (const state of Object.keys(FULL_LINE)) {
      expect(statuses({ state, approvals: rows })).toEqual(statuses({ state }));
    }
  });
});

describe("a state the workflow does not know", () => {
  for (const state of [undefined, null, "", "on_hold", "NEW"]) {
    it(`${JSON.stringify(state)}: nothing is current or done`, () => {
      expect(statuses({ state })).toEqual(row("p p p p p p p p n p n"));
    });
  }
});

describe("the optional customer stages", () => {
  const allStates = [...Object.keys(FULL_LINE), "rollback", "canceled", null, "on_hold"];

  for (const state of allStates) {
    for (const approval of FLAGS) {
      for (const review of FLAGS) {
        it(`${String(state)}, approval ${String(approval)}, review ${String(review)}`, () => {
          const input: BuildChangeRequestLifecycleInput = {
            state,
            customerApprovalRequired: approval,
            customerReviewRequired: review,
            approvals: [],
          };
          const keys = buildChangeRequestLifecycle(input).map((n) => n.key);
          const expected = CUSTOMER_PORTAL_ORDER.filter((k) => {
            if (k === "customer_approval") return approval !== false || state === k;
            if (k === "customer_review") return review !== false || state === k;
            return true;
          });
          expect(keys).toEqual(expected);

          // Leaving a stage off never changes how the others read, except that
          // a canceled change's proof is counted on the path that is shown.
          if (state !== "canceled") {
            const full = byKey({ state, approvals: [] });
            const shown = byKey(input);
            for (const k of keys) expect(shown[k]).toBe(full[k]);
          }
        });
      }
    }
  }

  it("keeps a customer stage the change is in, whatever its flag says", () => {
    expect(byKey({ state: "customer_approval", customerApprovalRequired: false }).customer_approval).toBe("current");
    expect(byKey({ state: "customer_review", customerReviewRequired: false }).customer_review).toBe("current");
  });

  it("an unknown flag keeps the stage; only an explicit false removes it", () => {
    const keys = (a?: boolean, r?: boolean): BeChangeRequestState[] =>
      buildChangeRequestLifecycle({ state: "implement", customerApprovalRequired: a, customerReviewRequired: r }).map((n) => n.key);
    expect(keys(undefined, undefined)).toHaveLength(11);
    expect(keys(false, undefined)).not.toContain("customer_approval");
    expect(keys(false, undefined)).toContain("customer_review");
    expect(keys(undefined, false)).toContain("customer_approval");
    expect(keys(undefined, false)).not.toContain("customer_review");
    expect(keys(false, false)).toHaveLength(9);
  });
});

describe("an Emergency change's line: Assess is never taken (New -> Authorize, one CAB approval)", () => {
  // Columns (both customer boxes off, as an Emergency change has them): new assess authorize scheduled implement review rollback closed canceled
  const emergency = (input: BuildChangeRequestLifecycleInput): BuildChangeRequestLifecycleInput => ({
    type: "emergency",
    customerApprovalRequired: false,
    customerReviewRequired: false,
    ...input,
  });

  it.each([
    ["new", "c n p p p p n p n"],
    ["authorize", "d n c p p p n p n"],
    ["scheduled", "d n d c p p n p n"],
    ["implement", "d n d d c p n p n"],
    ["review", "d n d d d c n p n"],
    ["closed", "d n d d d d n c n"],
    ["rollback", "d n d d d d c n n"],
  ])("in %s it reads %s", (state, expected) => {
    expect(statuses(emergency({ state }))).toEqual(row(expected));
  });

  it("labels the line New, Assess, Authorize, Scheduled, Implement, Review, Rollback, Closed, Canceled, with Assess not taken", () => {
    const nodes = buildChangeRequestLifecycle(emergency({ state: "authorize" }));
    expect(nodes.map((n) => n.label)).toEqual([
      "New",
      "Assess",
      "Authorize",
      "Scheduled",
      "Implement",
      "Review",
      "Rollback",
      "Closed",
      "Canceled",
    ]);
    expect(nodes.find((n) => n.key === "assess")?.status).toBe("not-taken");
  });

  it("is the same muted status the line already uses for the stages a change does not take", () => {
    expect(changeRequestLifecycleStatusText("not-taken")).toBe("not taken");
  });

  it("a canceled Emergency change: Assess is not taken even though a CAB stage proves Authorize was reached", () => {
    expect(statuses(emergency({ state: "canceled" }))).toEqual(row("u n u u u u n n c"));
    expect(statuses(emergency({ state: "canceled", approvals: [{ stage: "CAB Approval", status: "REQUESTED" }] }))).toEqual(
      row("d n u u u u n n c"),
    );
    expect(statuses(emergency({ state: "canceled", approvals: [{ stage: "CAB Approval", status: "APPROVED" }] }))).toEqual(
      row("d n d u u u n n c"),
    );
    // An older Emergency change's ECAB stage proves the same thing.
    expect(statuses(emergency({ state: "canceled", approvals: [{ stage: "ECAB Approval", status: "APPROVED" }] }))).toEqual(
      row("d n d u u u n n c"),
    );
  });

  it("an Emergency change that is somehow in Assess is shown where it is", () => {
    expect(statuses(emergency({ state: "assess" }))).toEqual(row("d c p p p p n p n"));
  });

  it("an Emergency change in a customer state (raised before the rule) keeps that stage on its line, and only that one", () => {
    const nodes = buildChangeRequestLifecycle({ state: "customer_approval", type: "emergency", customerApprovalRequired: true });
    expect(nodes.map((n) => [n.key, n.status])).toEqual([
      ["new", "done"],
      ["assess", "not-taken"],
      ["authorize", "done"],
      ["customer_approval", "current"],
      ["scheduled", "pending"],
      ["implement", "pending"],
      ["review", "pending"],
      ["rollback", "not-taken"],
      ["closed", "pending"],
      ["canceled", "not-taken"],
    ]);
  });

  it("ignores the stored boxes of an Emergency change: the flow never asks the customer, so neither stage is on the line", () => {
    for (const flags of [{ customerApprovalRequired: true, customerReviewRequired: true }, {}, { customerApprovalRequired: undefined }]) {
      const keys = buildChangeRequestLifecycle({ state: "authorize", type: "emergency", ...flags }).map((n) => n.key);
      expect(keys).not.toContain("customer_approval");
      expect(keys).not.toContain("customer_review");
      expect(keys).toHaveLength(9);
    }
  });

  it("keeps a customer gate the record shows an Emergency change went through: the customer's approval, or a stage row", () => {
    expect(buildChangeRequestLifecycle({ state: "scheduled", type: "emergency", customerApproved: true }).map((n) => n.key)).toContain(
      "customer_approval",
    );
    const keys = buildChangeRequestLifecycle({
      state: "closed",
      type: "emergency",
      approvals: [
        { stage: "ECAB Approval", status: "APPROVED" },
        { stage: "Customer Approval", status: "APPROVED" },
        { stage: "Customer Review", status: "APPROVED" },
      ],
    }).map((n) => n.key);
    expect(keys).toContain("customer_approval");
    expect(keys).toContain("customer_review");
  });

  it("a Normal change is untouched by all of this: its boxes alone decide", () => {
    expect(buildChangeRequestLifecycle({ state: "authorize", type: "normal", customerApprovalRequired: true }).map((n) => n.key)).toContain(
      "customer_approval",
    );
    expect(buildChangeRequestLifecycle({ state: "authorize", type: "normal" })).toHaveLength(11);
  });

  it("only Emergency skips Assess: Normal, Standard and an unknown type still pass through it", () => {
    for (const type of ["normal", "standard", undefined, null, "model"]) {
      expect(byKey({ state: "authorize", type, customerApprovalRequired: false, customerReviewRequired: false }).assess).toBe("done");
    }
    expect(byKey({ state: "new", type: "normal" }).assess).toBe("pending");
  });

  it("reads the type however the backend cases it", () => {
    expect(byKey({ state: "authorize", type: " Emergency " }).assess).toBe("not-taken");
  });
});

describe("changeRequestLifecycleStatusText", () => {
  it("has words for every status", () => {
    expect(changeRequestLifecycleStatusText("done")).toBe("done");
    expect(changeRequestLifecycleStatusText("current")).toBe("current");
    expect(changeRequestLifecycleStatusText("pending")).toBe("upcoming");
    expect(changeRequestLifecycleStatusText("not-taken")).toBe("not taken");
    expect(changeRequestLifecycleStatusText("unrecorded")).toBe("history not recorded");
    expect(changeRequestLifecycleStatusText("rejected")).toBe("rejected by the customer");
  });
});
