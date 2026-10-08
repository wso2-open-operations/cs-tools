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

//
// THE REFUSALS OF A CUSTOMER'S ANSWER CARRY A MACHINE-READABLE errorCode, on the real local stack.
//
// The customer webapp tells a hold, a moved schedule, an answer already given, a proposal where none
// can be made and a contact who was never asked apart by the `errorCode` of the error body, never by
// the wording of its message. This proves the code is really on the wire, whole, with the status and
// the message of every refusal unchanged:
//
//   refusal                                              status  errorCode
//   a proposed time while WSO2 has the change on hold     409     change_request_on_hold
//   an answer for a planned window that has moved         409     change_request_schedule_changed
//   an answer to an approval that is no longer pending    409     change_request_approval_not_pending
//   a proposed time where none can be proposed            409     change_request_not_proposable
//   a proposed time with no planned window to move         409     change_request_no_planned_window
//   (a proposed time while an approval that is not the customer's is still asked, 409
//   change_request_proposal_not_now, is pinned against the database in entity-service's
//   change_request_proposal_integration_test.go: it needs an internal approver row)
//   a contact the customer's request was never sent to    403     change_request_not_asked
//   a field a customer may not set                        403     change_request_forbidden
//
// and a refusal that has no name (a malformed date) carries no `errorCode` key at all. Each is read
// through the customer portal's backend (what the webapp calls) and straight at entity-service (the
// customer backend's machine token plus the customer's own ID token), so the code is shown to be the
// service's and to survive the backend's passthrough.
//
// ⚠️ STATE-CHANGING (re-seeds the stack's Postgres): needs E2E_POSTGRES_CONTAINER, and SKIPS without it.
// The direct entity-service calls need E2E_ENTITY_SERVICE_URL (the isolated stack's is
// http://localhost:18081) and are skipped without it.
//

import { test, expect } from "../../fixtures/test";
import { LOCAL_PERSONAS, withLocalSession } from "../../auth/localSessions";
import {
  FIXTURES,
  changeRequestRow,
  customerApi,
  entityAsCustomer,
  entityServiceUrl,
  futureWindow,
  psql,
  resetFixtures,
  withFixtureStack,
} from "../../utils/localStack";

withLocalSession(test, "dave");
withFixtureStack(test);

const { approval, review } = FIXTURES;

/** The error body, as the customer backend and entity-service both write it. */
type ErrorBody = { code?: number; message?: string; errorCode?: string };
const bodyOf = (result: { body: unknown }): ErrorBody => (result.body ?? {}) as ErrorBody;

/** A window nine days out, as RFC 3339 instants (what the API prints and accepts). */
const soon = futureWindow("UTC", { daysAhead: 9, startHour: 9, hours: 2 });

/**
 * One refusal, read both ways: through the customer backend as `via` and straight at entity-service.
 * Both must answer `status`, with a message, and `errorCode` (undefined: the key must be absent).
 */
async function expectRefusal(
  label: string,
  asPersona: "dave" | "erin",
  changeRequestId: string,
  body: unknown,
  expected: { status: number; errorCode: string | undefined; message?: RegExp },
): Promise<void> {
  const through = await customerApi(asPersona).patch(changeRequestId, body);
  const seen = `${label} through the customer backend: ${JSON.stringify(through.body)}`;
  expect(through.status, seen).toBe(expected.status);
  expect(bodyOf(through).errorCode, seen).toBe(expected.errorCode);
  if (expected.errorCode === undefined) {
    expect(Object.keys(bodyOf(through)), `${label}: a refusal that has no name carries no errorCode key`).not.toContain("errorCode");
  }
  // The code is never the whole story: the message stays, as human text.
  expect(typeof bodyOf(through).message, seen).toBe("string");
  expect((bodyOf(through).message ?? "").length, seen).toBeGreaterThan(0);

  if (!entityServiceUrl()) return;
  const direct = await entityAsCustomer(LOCAL_PERSONAS[asPersona].email, "PATCH", `/change-requests/${changeRequestId}`, body);
  const directSeen = `${label} straight at entity-service: ${JSON.stringify(direct.body)}`;
  expect(direct.status, directSeen).toBe(expected.status);
  expect(bodyOf(direct).errorCode, directSeen).toBe(expected.errorCode);
  expect(bodyOf(direct).code, directSeen).toBe(expected.status);
  if (expected.message) {
    expect(bodyOf(direct).message ?? "", directSeen).toMatch(expected.message);
  }
}

test.describe("Local stack — a refused answer carries its machine-readable errorCode", () => {
  test.describe.configure({ timeout: 120_000 });

  test.beforeEach(async () => {
    await resetFixtures();
  });

  test(`a proposed time while ${approval.number} is on hold is a 409 change_request_on_hold; the answer itself is still taken afterwards`, async () => {
    await psql(`update change_request set is_on_hold = true, on_hold_reason = 'E2E hold' where id = '${approval.id}'`);

    await expectRefusal("a proposal on a held change", "dave", approval.id, { plannedStartOn: soon.startUtc, plannedEndOn: soon.endUtc }, {
      status: 409,
      errorCode: "change_request_on_hold",
      message: /on hold/,
    });
    expect((await changeRequestRow(approval.id)).state, "the refusal changed nothing").toBe("CUSTOMER_APPROVAL");

    // The hold refuses the proposal only: the customer is still being asked.
    const answered = await customerApi("dave").patch(approval.id, { isCustomerApproved: true });
    expect(answered.status, JSON.stringify(answered.body)).toBe(200);
    expect((await changeRequestRow(approval.id)).state).toBe("SCHEDULED");
  });

  test(`a proposed time on ${approval.number} with no planned window to move is a 409 change_request_no_planned_window; the answer itself is still taken afterwards`, async () => {
    await psql(`update change_request set start_on = NULL, end_on = NULL where id = '${approval.id}'`);

    await expectRefusal("a proposal over a change with no planned window", "dave", approval.id, { plannedStartOn: soon.startUtc, plannedEndOn: soon.endUtc }, {
      status: 409,
      errorCode: "change_request_no_planned_window",
      message: /no planned window to move/,
    });
    expect((await changeRequestRow(approval.id)).state, "the refusal changed nothing").toBe("CUSTOMER_APPROVAL");

    // Only the proposal is refused: the customer is still being asked.
    const answered = await customerApi("dave").patch(approval.id, { isCustomerApproved: true });
    expect(answered.status, JSON.stringify(answered.body)).toBe(200);
    expect((await changeRequestRow(approval.id)).state).toBe("SCHEDULED");
  });

  test(`an answer for a planned window that has moved on ${approval.number} is a 409 change_request_schedule_changed and records nothing`, async () => {
    await psql(`update change_request set start_on = '${soon.startUtc}', end_on = '${soon.endUtc}' where id = '${approval.id}'`);
    const other = futureWindow("UTC", { daysAhead: 15, startHour: 14, hours: 2 });

    await expectRefusal(
      "an answer for the window the page was shown",
      "dave",
      approval.id,
      { isCustomerApproved: true, expectedPlannedStartOn: other.startUtc, expectedPlannedEndOn: other.endUtc },
      { status: 409, errorCode: "change_request_schedule_changed", message: /changed after you opened/ },
    );
    expect((await changeRequestRow(approval.id)).state, "nothing was approved").toBe("CUSTOMER_APPROVAL");

    // The window the page really showed is the way through.
    const shown = await customerApi("dave").patch(approval.id, {
      isCustomerApproved: true,
      expectedPlannedStartOn: soon.startUtc,
      expectedPlannedEndOn: soon.endUtc,
    });
    expect(shown.status, JSON.stringify(shown.body)).toBe(200);
  });

  test(`an answer after a colleague answered is a 409 change_request_approval_not_pending, and a proposed time then is a 409 change_request_not_proposable`, async () => {
    const dave = await customerApi("dave").patch(approval.id, { isCustomerApproved: true });
    expect(dave.status, JSON.stringify(dave.body)).toBe(200);
    expect((await changeRequestRow(approval.id)).state).toBe("SCHEDULED");

    await expectRefusal("erin's answer after dave's", "erin", approval.id, { isCustomerApproved: true }, {
      status: 409,
      errorCode: "change_request_approval_not_pending",
      message: /no longer pending|nothing to answer/,
    });
    await expectRefusal("erin's proposal after dave's answer", "erin", approval.id, { plannedStartOn: soon.startUtc, plannedEndOn: soon.endUtc }, {
      status: 409,
      errorCode: "change_request_not_proposable",
      message: /Customer Approval/,
    });
    expect((await changeRequestRow(approval.id)).state, "the refusals changed nothing").toBe("SCHEDULED");
  });

  test(`a proposed time at Customer Review (${review.number}) is a 409 change_request_not_proposable`, async () => {
    await expectRefusal("a proposal at Customer Review", "dave", review.id, { plannedStartOn: soon.startUtc, plannedEndOn: soon.endUtc }, {
      status: 409,
      errorCode: "change_request_not_proposable",
      message: /Customer Approval/,
    });
    expect((await changeRequestRow(review.id)).state).toBe("CUSTOMER_REVIEW");
  });

  test(`a contact whose request on ${approval.number} was withdrawn is refused with a 403 change_request_not_asked, for an answer and for a proposed time`, async () => {
    await psql(
      `update approval_stage_approver a set state = 'CANCELLED' from "user" u ` +
        `where a.approver_user_id = u.id and a.work_item_id = '${approval.id}' ` +
        `and u.email = '${LOCAL_PERSONAS.erin.email}'`,
    );

    await expectRefusal("an answer by a contact who was not asked", "erin", approval.id, { isCustomerApproved: true }, {
      status: 403,
      errorCode: "change_request_not_asked",
    });
    await expectRefusal("a proposal by a contact who was not asked", "erin", approval.id, { plannedStartOn: soon.startUtc, plannedEndOn: soon.endUtc }, {
      status: 403,
      errorCode: "change_request_not_asked",
    });
    expect((await changeRequestRow(approval.id)).state, "the refusals changed nothing").toBe("CUSTOMER_APPROVAL");
  });

  test(`a field a customer may not set is a 403 change_request_forbidden, alone or beside an answer`, async () => {
    await expectRefusal("an answer with a title change", "dave", approval.id, { isCustomerApproved: true, title: "hijacked" }, {
      status: 403,
      errorCode: "change_request_forbidden",
    });
    await expectRefusal("a state change alone", "dave", approval.id, { state: "scheduled" }, {
      status: 403,
      errorCode: "change_request_forbidden",
    });
    const row = await changeRequestRow(approval.id);
    expect([row.state, row.title === "hijacked"], "the refusals changed nothing").toEqual(["CUSTOMER_APPROVAL", false]);
  });

  test(`a refusal that has no name carries no errorCode key: a malformed date is a plain 400`, async () => {
    await expectRefusal("a malformed window", "dave", approval.id, { plannedStartOn: "tomorrow", plannedEndOn: "tomorrow" }, {
      status: 400,
      errorCode: undefined,
    });
  });
});
