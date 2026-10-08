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
// THE PLANNED TIME IS VALIDATED BY THE SERVER, on the real local stack.
//
// A customer's proposed implementation time used to be taken as typed: a start in the past
// answered 200, and odd Postgres date words ("tomorrow", "now", "infinity") were cast by the
// database and could re-schedule the change request. Only the webapp blocked a past start.
// Now the planned dates are parsed strictly (RFC 3339, or "YYYY-MM-DD HH:MM:SS" in UTC, years
// 2000 to 2100), a proposal is a START (the planned length stays, so an end, when sent, must be
// the end that start implies) and must be in the future from an asked contact; and the refusal
// leaves everything as it was. The fixture has a planned window (a proposal is refused on a
// change with none: see the propose spec), so a refusal here is the date's, not the missing window's.
//
// What is proved, with a customer's own token:
//   1. Through the customer portal's backend AND straight at entity-service (the customer
//      backend's machine token plus the customer's ID token, which is what the backend forwards),
//      every one of: "tomorrow", "now", "infinity", "-infinity", "today", "epoch", a date with no
//      time, a start in the past (both date styles), an end that is not the planned length after
//      the start, a year outside 2000-2100, garbage and an empty string, in either field, is a
//      400 in the service's own words, and NOTHING changes: the state, the window, the proposed
//      time, the approver rows (no stage, no cancelled request, no CAB stage) and the row's own
//      updated_on.
//   2. A good start right after all those refusals still works, in the whole loop: dave proposes
//      (RFC 3339 with an offset, stored as the UTC instant; the state and the window do not move),
//      WSO2 accepts it through the CSM portal's backend, and the change is Scheduled at that
//      start with the planned length.
//   3. An end-only proposal is refused (a proposal needs a new start), whatever the planned
//      start's age; the start alone, or with the end it implies, is the way in.
//   4. The same strings are refused on WSO2 staff's PATCH and on create (the format; a staff
//      member may plan in the past), and a refused create leaves no change request behind.
//
// ⚠️ STATE-CHANGING (re-seeds, raises change requests): needs E2E_POSTGRES_CONTAINER and
// E2E_CSM_BFF_URL, and SKIPS without them. The direct entity-service calls need E2E_ENTITY_SERVICE_URL
// (the isolated stack's is http://localhost:18081) and are skipped without it.
//

import { test, expect } from "../../fixtures/test";
import { LOCAL_PERSONAS, withLocalSession } from "../../auth/localSessions";
import {
  FIXTURES,
  addHours,
  approverRows,
  changeRequestRow,
  customerApi,
  deleteRaisedChanges,
  entityAsCustomer,
  entityServiceUrl,
  futureWindow,
  proposalRow,
  psql,
  raiseChange,
  requestApproval,
  resetFixtures,
  staffAcceptsProposal,
  staffApi,
  staffDecides,
  storedState,
  withFixtureStack,
  RAISED_PREFIX,
} from "../../utils/localStack";

withLocalSession(test, "dave");
withFixtureStack(test, true);

const { approval, projectId } = FIXTURES;

const FORMAT = /must be a valid date-time, either RFC 3339 .* or YYYY-MM-DD HH:MM:SS in UTC, in the years 2000 to 2100/;
const PAST = /in the past: a proposed implementation time must be one still to come/;
// A proposal keeps the planned length, so an end that is not the end the start implies is refused, and the service says so
// with the end it wants. The customer backend checks the ORDER of the two dates before the request leaves it (an end
// before, or equal to, the start), in entity-service's own words for that, so either layer's refusal is the right one.
const KEEPS_LENGTH =
  /moves the start and keeps the planned length of .*: plannedEndOn must be |the planned start must not be (after|the same as) the planned end/;

/** Everything a refused proposal must leave alone. */
async function snapshot(id: string) {
  return {
    row: await changeRequestRow(id),
    proposal: await proposalRow(id),
    approvers: await approverRows(id),
    updatedOn: (await psql(`select updated_on from work_item where id = '${id}'`)).trim(),
  };
}

/** A pair of planned dates and what the refusal must say. */
const REFUSED: Array<{ name: string; start: unknown; end: unknown; says: RegExp }> = [
  { name: "tomorrow", start: "tomorrow", end: "tomorrow", says: FORMAT },
  { name: "now", start: "now", end: "now", says: FORMAT },
  { name: "infinity", start: "infinity", end: "infinity", says: FORMAT },
  { name: "-infinity", start: "-infinity", end: "infinity", says: FORMAT },
  { name: "today", start: "today", end: "today", says: FORMAT },
  { name: "epoch", start: "epoch", end: "epoch", says: FORMAT },
  { name: "a date with no time", start: "2031-03-01", end: "2031-03-02", says: FORMAT },
  { name: "a good start and 'tomorrow' for the end", start: "2031-03-01 10:00:00", end: "tomorrow", says: FORMAT },
  { name: "'tomorrow' for the start and a good end", start: "tomorrow", end: "2031-03-01 12:00:00", says: FORMAT },
  { name: "a start in the past, YYYY-MM-DD HH:MM:SS", start: "2020-01-01 10:00:00", end: "2020-01-01 12:00:00", says: PAST },
  { name: "a start in the past, RFC 3339", start: "2020-01-01T10:00:00Z", end: "2031-03-01T12:00:00Z", says: PAST },
  { name: "an end equal to the start (no length)", start: "2031-03-01 12:00:00", end: "2031-03-01 12:00:00", says: KEEPS_LENGTH },
  { name: "an end before the start", start: "2031-03-01 12:00:00", end: "2031-03-01 10:00:00", says: KEEPS_LENGTH },
  { name: "an end that is not the planned length after the start", start: "2031-03-01 12:00:00", end: "2031-03-01 18:00:00", says: KEEPS_LENGTH },
  { name: "the year 1999", start: "1999-03-01 12:00:00", end: "1999-03-01 14:00:00", says: FORMAT },
  { name: "the year 2101", start: "2101-03-01 12:00:00", end: "2101-03-01 14:00:00", says: FORMAT },
  { name: "garbage", start: "abc", end: "def", says: FORMAT },
  { name: "empty strings", start: "", end: "", says: FORMAT },
  { name: "a number", start: 20310301, end: 20310302, says: /./ },
];

test.describe("Local stack — the planned time is validated by the server", () => {
  test.describe.configure({ timeout: 300_000 });

  test.beforeEach(async () => {
    await deleteRaisedChanges();
    await resetFixtures();
  });
  test.afterAll(async () => {
    await deleteRaisedChanges();
  });

  test(`a customer's proposal with a date word, a date with no time, a past start, an empty or inverted window or a year out of range is a 400 in the service's words, and nothing changes: through the customer backend and straight at entity-service`, async () => {
    const dave = customerApi("dave");
    const before = await snapshot(approval.id);
    expect(before.row.state).toBe("CUSTOMER_APPROVAL");
    expect(before.approvers.map((r) => `${r.stage}|${r.email}|${r.state}`)).toEqual([
      "Customer Approval|dave.mendis@example.com|REQUESTED",
      "Customer Approval|erin.jayawardena@example.com|REQUESTED",
    ]);

    for (const bad of REFUSED) {
      const body = { plannedStartOn: bad.start, plannedEndOn: bad.end };
      const through = await dave.patch(approval.id, body);
      expect(through.status, `${bad.name} through the customer backend: ${JSON.stringify(through.body)}`).toBe(400);
      expect(JSON.stringify(through.body), bad.name).toMatch(bad.says);
      expect(await snapshot(approval.id), `${bad.name}: the customer backend's refusal changed something`).toEqual(before);

      if (entityServiceUrl()) {
        const direct = await entityAsCustomer(LOCAL_PERSONAS.dave.email, "PATCH", `/change-requests/${approval.id}`, body);
        expect(direct.status, `${bad.name} straight at entity-service: ${JSON.stringify(direct.body)}`).toBe(400);
        expect(JSON.stringify(direct.body), `${bad.name} (entity-service)`).toMatch(bad.says);
        expect(await snapshot(approval.id), `${bad.name}: entity-service's refusal changed something`).toEqual(before);
      }
    }

    // Still waiting, still askable, still planned as it was.
    const after = await dave.get(approval.id);
    expect(after.body.state?.label).toBe("Customer Approval");
    expect(after.body.customerCanAnswer).toBe(true);
  });

  test(`a good start right after the refusals still works, the whole loop: dave proposes it (RFC 3339 with an offset, stored as the UTC instant, nothing else moves), WSO2 accepts it, and the change is Scheduled at that start`, async () => {
    const dave = customerApi("dave");
    const planned = await changeRequestRow(approval.id);
    // Refused first (the state of the world a user is in after a typo)...
    expect((await dave.patch(approval.id, { plannedStartOn: "tomorrow", plannedEndOn: "tomorrow" })).status).toBe(400);
    expect((await dave.patch(approval.id, { plannedStartOn: "2020-01-01 10:00:00", plannedEndOn: "2020-01-01 12:00:00" })).status).toBe(400);
    expect(await storedState(approval.id)).toBe("CUSTOMER_APPROVAL");

    // ...then a real one, spelled in a zone that is not UTC: 5.5 hours ahead. The planned window is two hours.
    const window = futureWindow("UTC", { daysAhead: 10, startHour: 10, hours: 2 });
    const plus530 = (utc: string) => {
      const shifted = new Date(new Date(utc).getTime() + 330 * 60_000).toISOString();
      return `${shifted.slice(0, 19)}+05:30`;
    };
    const proposed = await dave.patch(approval.id, { plannedStartOn: plus530(window.startUtc), plannedEndOn: plus530(window.endUtc) });
    expect(proposed.status, JSON.stringify(proposed.body)).toBe(200);
    const row = await changeRequestRow(approval.id);
    expect([row.state, row.startUtc, row.endUtc], "the proposal moves nothing: the planned window is WSO2's until it answers").toEqual([
      "CUSTOMER_APPROVAL",
      planned.startUtc,
      planned.endUtc,
    ]);
    expect(await proposalRow(approval.id), "the proposed start, as the UTC instant").toEqual({ proposedUtc: window.startUtc, answer: "" });
    expect((await approverRows(approval.id)).filter((r) => r.stage === "Customer Approval").map((r) => r.state), "the requests stand").toEqual(["REQUESTED", "REQUESTED"]);

    // WSO2 accepts it: Scheduled at the proposed start, with the planned length.
    const accepted = await staffAcceptsProposal("alice", approval.id);
    expect(accepted.status, JSON.stringify(accepted.body)).toBe(200);
    const done = await changeRequestRow(approval.id);
    expect([done.state, done.startUtc, done.endUtc]).toEqual(["SCHEDULED", window.startUtc, addHours(window.startUtc, 2)]);
    expect(await proposalRow(approval.id)).toEqual({ proposedUtc: window.startUtc, answer: "AGREE" });
  });

  test("an end-only proposal is refused, whatever the age of the planned start: a proposal needs a new start", async () => {
    const dave = customerApi("dave");
    // The change was planned a day ago and ends in two days (a stored window that has begun).
    await psql(`update change_request set start_on = now() - interval '1 day', end_on = now() + interval '2 days' where id = '${approval.id}'`);
    const before = await snapshot(approval.id);
    const end = futureWindow("UTC", { daysAhead: 5, startHour: 18, hours: 1 }).startUtc;
    const refused = await dave.patch(approval.id, { plannedEndOn: end });
    expect(refused.status, JSON.stringify(refused.body)).toBe(400);
    expect(JSON.stringify(refused.body)).toMatch(/a proposed implementation time needs a new start: send plannedStartOn/);
    expect(await snapshot(approval.id), "the refusal changed something").toEqual(before);
    if (entityServiceUrl()) {
      const direct = await entityAsCustomer(LOCAL_PERSONAS.dave.email, "PATCH", `/change-requests/${approval.id}`, { plannedEndOn: end });
      expect(direct.status, JSON.stringify(direct.body)).toBe(400);
      expect(await snapshot(approval.id)).toEqual(before);
    }
    // The start alone is the way in (the end is derived), and so is the start with the end it implies.
    const window = futureWindow("UTC", { daysAhead: 5, startHour: 14, hours: 2 });
    const ok = await dave.patch(approval.id, { plannedStartOn: window.startUtc });
    expect(ok.status, JSON.stringify(ok.body)).toBe(200);
    expect(await proposalRow(approval.id)).toEqual({ proposedUtc: window.startUtc, answer: "" });
    expect((await changeRequestRow(approval.id)).state, "a proposal never moves the state").toBe("CUSTOMER_APPROVAL");
    const later = futureWindow("UTC", { daysAhead: 6, startHour: 14, hours: 2 });
    const withEnd = await dave.patch(approval.id, { plannedStartOn: later.startUtc, plannedEndOn: addHours(later.startUtc, 72) });
    // (the planned length is 72 hours here: that is the end the start implies)
    expect(withEnd.status, JSON.stringify(withEnd.body)).toBe(200);
    expect(await proposalRow(approval.id)).toEqual({ proposedUtc: later.startUtc, answer: "" });
  });

  test("the same strings are refused on a staff member's PATCH and on create (a past date is the staff's own business), and a refused create leaves no change request behind", async () => {
    const change = await raiseChange({ title: "dates, staff PATCH", projectId, approval: false, review: false });
    const jane = staffApi("jane");
    const unplanned = await changeRequestRow(change.id);
    expect([unplanned.startUtc, unplanned.endUtc]).toEqual(["", ""]);

    for (const bad of REFUSED.filter((r) => r.says === FORMAT)) {
      const patched = await jane.patch(change.id, { plannedStartOn: bad.start, plannedEndOn: bad.end });
      expect(patched.status, `${bad.name}: staff PATCH ${JSON.stringify(patched.body)}`).toBe(400);
      expect(JSON.stringify(patched.body)).toMatch(FORMAT);
      const row = await changeRequestRow(change.id);
      expect([row.startUtc, row.endUtc], `${bad.name}: the staff PATCH changed the window`).toEqual(["", ""]);

      const subject = `${RAISED_PREFIX}dates, create ${bad.name}`;
      const created = await jane.create({ subject, type: "normal", plannedStartDate: bad.start, plannedEndDate: bad.end });
      expect(created.status, `${bad.name}: create ${JSON.stringify(created.body)}`).toBe(400);
      expect(await psql(`select count(*) from work_item where subject = '${subject.replace(/'/g, "''")}'`), `${bad.name}: a refused create left a row`).toBe("0");
    }

    // Re-schedule (the staff's {state: "authorize"} out of Customer Approval) takes the same dates through the same parser: a
    // date word is a 400 and the change stays in Customer Approval, its customer request standing. (A Re-schedule never moves
    // the state: "authorize" is the wire name of the Time Change loop.)
    const rescheduled = await raiseChange({ title: "dates, re-schedule", projectId, approval: true, review: false });
    expect((await requestApproval(rescheduled.id)).status).toBe(200);
    await staffDecides("alice", rescheduled.id);
    await staffDecides("alice", rescheduled.id);
    expect(await storedState(rescheduled.id)).toBe("CUSTOMER_APPROVAL");
    const asked = await approverRows(rescheduled.id);
    for (const [start, end] of [["tomorrow", "2031-03-02 10:00:00"], ["now", "infinity"], ["2031-03-01", "2031-03-02"]] as const) {
      const refusedReschedule = await staffApi("alice").patch(rescheduled.id, { state: "authorize", plannedStartOn: start, plannedEndOn: end });
      expect(refusedReschedule.status, `re-schedule with ${start} / ${end}: ${JSON.stringify(refusedReschedule.body)}`).toBe(400);
      expect(JSON.stringify(refusedReschedule.body)).toMatch(FORMAT);
      expect(await storedState(rescheduled.id)).toBe("CUSTOMER_APPROVAL");
      expect(await approverRows(rescheduled.id), "a refused re-schedule left the requests as they were").toEqual(asked);
    }

    // A past window is staff's to plan (a retrospective record), in either good format.
    const past = await jane.patch(change.id, { plannedStartOn: "2020-01-01 10:00:00", plannedEndOn: "2020-01-01T12:00:00Z" });
    expect(past.status, JSON.stringify(past.body)).toBe(200);
    expect((await changeRequestRow(change.id)).startUtc).toBe("2020-01-01T10:00:00Z");
  });
});
