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
// Deterministic Change Request lifecycle coverage — the compulsory-
// assigned-team gate, Assess-entry approver auto-provisioning, the
// approve/cancel-sibling cascade from Assess to Authorize, and a terminal
// approval display — run against the four fixed-UUID fixtures in
// scripts/csm-compose/seed-entity-service.sql (CR-FIXED-001..004), not a
// freshly self-provisioned CR the way change-request-detail.spec.ts works.
//
// That's a deliberate, necessary difference, not a style choice:
// CreateChangeRequest always 503s against this stack's own Postgres data
// source (work_item.number has no DB sequence — see entity-service's own
// CLAUDE.md, "CreateCase and case numbers"/"Change requests"), so there is
// no "create one, then drive it" path available locally at all. The fixed
// fixtures exist specifically to give this spec something to navigate
// straight to by id.
//
// The flip side of a fixed fixture: CR-FIXED-002/003 each get moved forward
// by exactly the transition this spec exercises (New -> Assess, and an
// approval decision), and that move is NOT reset by re-running the seed
// file — `ON CONFLICT (id) DO NOTHING` only ever applies on first insert, so
// a mutated row stays mutated. Unlike staging (where change-request-detail's
// self-provisioned CRs are simply abandoned, never reused), this spec is
// meant to run before every local push, so its own fixtures have to come
// back to their starting state every time. resetFixtures() below does that
// with plain, idempotent UPDATEs against the already-running local
// docker-compose Postgres (`csm-platform-postgres-1`) before anything else
// runs — CR-FIXED-001/004 are read-only fixtures (nothing here ever mutates
// them) and need no reset.
//
// Runs only against the local stack (E2E_NO_WEBSERVER=1, see
// package.json's "test:e2e:cr-lifecycle") signed in as jane.doe@example.com
// — the approver CR-FIXED-003/004 are seeded with (see
// tests/e2e/auth/generate-session.spec.ts for how that session is minted
// with no human step, as the "crApprover" role).
//

import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { test, expect, withRole } from "../../fixtures/test";
import { ChangeRequestDetailPage } from "../../pages/ChangeRequestDetailPage";

const execFileAsync = promisify(execFile);

const CR_NO_TEAM = "00000000-0000-0000-0000-000000001001";
const CR_WITH_TEAM = "00000000-0000-0000-0000-000000001002";
const CR_PENDING_APPROVAL = "00000000-0000-0000-0000-000000001003";
const CR_RESOLVED = "00000000-0000-0000-0000-000000001004";

const APOLLO_GROUP_ID = "00000000-0000-0000-0000-000000000901"; // "Example Corp ABT" — see seed-entity-service.sql
const STAGE_ID = "00000000-0000-0000-0000-000000001005";
const JANE_APPROVER_ROW = "00000000-0000-0000-0000-000000001006";
const JOHN_APPROVER_ROW = "00000000-0000-0000-0000-000000001007";

/** Restores CR-FIXED-002/003 to the exact starting state documented in
 * seed-entity-service.sql, unconditionally, via the already-running local
 * docker-compose Postgres container — see this file's own top comment for
 * why a fixed fixture needs this instead of the idempotent seed file's own
 * `ON CONFLICT DO NOTHING` inserts. Plain UPDATEs, not re-running the seed
 * file, since the rows already exist after the first ever seed. */
async function resetFixtures(): Promise<void> {
  const sql = `
    UPDATE change_request SET state = 'NEW'::change_request_state_enum, requested_by_user_id = NULL WHERE id = '${CR_WITH_TEAM}';
    UPDATE work_item SET assignment_group_id = '${APOLLO_GROUP_ID}' WHERE id = '${CR_WITH_TEAM}';
    DELETE FROM approval_stage_approver WHERE work_item_id = '${CR_WITH_TEAM}';
    DELETE FROM approval_stage WHERE work_item_id = '${CR_WITH_TEAM}';

    -- requested_by_user_id stays NULL here (not jane.doe/john.smith, both
    -- team members): PatchChangeRequest now auto-cancels the change's own
    -- requester instead of leaving them Requested (mirrors real ServiceNow,
    -- confirmed live against CHG0039122 — see entity-service's own CLAUDE.md).
    -- Either seeded user as requester would turn this fixture's "both
    -- members become pending approvers" demonstration into a demonstration
    -- of that unrelated exclusion instead.
    UPDATE change_request SET state = 'ASSESS'::change_request_state_enum, requested_by_user_id = NULL WHERE id = '${CR_PENDING_APPROVAL}';
    -- The "approve cascades to Authorize" test below approves Jane's row,
    -- which (since entity-service also auto-provisions an Authorize-stage
    -- now, not just Assess) creates a SECOND approval_stage + a fresh pair
    -- of approver rows for this same work item, under new gen_random_uuid()
    -- ids neither upsert below ever matches. Left alone, those accumulate
    -- across runs -- the Approvals table ends up with two rows per approver,
    -- and Playwright's strict-mode approverRow("Jane Doe") then matches more
    -- than one and fails. Delete anything that isn't this fixture's own
    -- known seeded stage/approvers before re-seeding them.
    DELETE FROM approval_stage_approver WHERE work_item_id = '${CR_PENDING_APPROVAL}'
      AND id NOT IN ('${JANE_APPROVER_ROW}', '${JOHN_APPROVER_ROW}');
    DELETE FROM approval_stage WHERE work_item_id = '${CR_PENDING_APPROVAL}' AND id <> '${STAGE_ID}';
    INSERT INTO approval_stage (id, created_on, updated_on, created_by, updated_by, work_item_id, assignment_group_id, raw_status)
      VALUES ('${STAGE_ID}', now(), now(), 'seed', 'seed', '${CR_PENDING_APPROVAL}', '${APOLLO_GROUP_ID}', 'requested')
      ON CONFLICT (id) DO UPDATE SET raw_status = 'requested';
    INSERT INTO approval_stage_approver (id, created_on, updated_on, created_by, updated_by, stage_id, work_item_id, approver_user_id, status)
      VALUES
        ('${JANE_APPROVER_ROW}', now(), now(), 'seed', 'seed', '${STAGE_ID}', '${CR_PENDING_APPROVAL}', '00000000-0000-0000-0000-000000000001', 'requested'),
        ('${JOHN_APPROVER_ROW}', now(), now(), 'seed', 'seed', '${STAGE_ID}', '${CR_PENDING_APPROVAL}', '00000000-0000-0000-0000-000000000002', 'requested')
      ON CONFLICT (id) DO UPDATE SET status = 'requested';
  `;
  await execFileAsync("docker", [
    "exec",
    "-i",
    "csm-platform-postgres-1",
    "psql",
    "-U",
    "postgres",
    "-d",
    "csm_platform",
    "-v",
    "ON_ERROR_STOP=1",
    "-c",
    sql,
  ]);
}

withRole(test, "crApprover");

test.beforeAll(async () => {
  await resetFixtures();
});

test.describe("change request lifecycle — compulsory team gate", () => {
  test("Move to Assess is disabled with no assigned team, and states why", async ({ page }) => {
    const detail = new ChangeRequestDetailPage(page);
    await detail.goto(CR_NO_TEAM);

    const blocked = page.getByLabel(/Move to Assess: .*assigned team/i);
    await expect(blocked).toBeVisible();
    await expect(blocked.getByRole("button", { name: "Move to Assess" })).toBeDisabled();
  });
});

test.describe("change request lifecycle — Assess-entry auto-provisioning", () => {
  test("Move to Assess succeeds once a team is assigned, and provisions that team's members as approvers", async ({
    page,
  }) => {
    test.setTimeout(60_000);

    const detail = new ChangeRequestDetailPage(page);
    await detail.goto(CR_WITH_TEAM);

    const moveToAssessButton = detail.moveToAssessButton();
    await expect(moveToAssessButton).toBeEnabled();

    const [response] = await Promise.all([
      page.waitForResponse(
        (r) => new RegExp(`/change-requests/${CR_WITH_TEAM}$`).test(r.url()) && r.request().method() === "PATCH",
        { timeout: 15_000 },
      ),
      detail.moveToAssess(),
    ]);
    expect(response.ok(), `Move to Assess PATCH failed (${response.status()})`).toBeTruthy();

    await expect(moveToAssessButton).toBeHidden({ timeout: 15_000 });

    // Both of the assigned team's seeded members (Jane Doe, John Smith —
    // see scripts/csm-compose/seed-entity-service.sql) should now appear as
    // Requested approvers, with no manual provisioning step.
    await expect(detail.approverStatus("Jane Doe")).toHaveText("Requested");
    await expect(detail.approverStatus("John Smith")).toHaveText("Requested");
  });
});

test.describe("change request lifecycle — approve cascades to Authorize", () => {
  test("approving one of two pending approvers cascades the state to Authorize and cancels the other", async ({
    page,
  }) => {
    test.setTimeout(60_000);

    const detail = new ChangeRequestDetailPage(page);
    await detail.goto(CR_PENDING_APPROVAL);

    // Only the signed-in user's (Jane Doe's) own pending row renders an
    // Approve button — John Smith's sibling row has none.
    await expect(detail.approverStatus("Jane Doe")).toHaveText("Requested");
    await expect(detail.approverStatus("John Smith")).toHaveText("Requested");
    await expect(detail.approveButton("John Smith")).toHaveCount(0);

    const approveButton = detail.approveButton("Jane Doe");
    await expect(approveButton).toBeVisible();

    const [response] = await Promise.all([
      page.waitForResponse((r) => /\/change-requests\/[^/]+\/approvals?/.test(r.url()), { timeout: 15_000 }),
      approveButton.click(),
    ]);
    expect(response.ok(), `Approve decision failed (${response.status()})`).toBeTruthy();

    await expect(detail.approverStatus("Jane Doe")).toHaveText("Approved");
    await expect(detail.approverStatus("John Smith")).toHaveText("Cancelled");
  });
});

test.describe("change request lifecycle — terminal approval display", () => {
  test("an already-decided change request shows its resolved approval state with no pending actions", async ({
    page,
  }) => {
    const detail = new ChangeRequestDetailPage(page);
    await detail.goto(CR_RESOLVED);

    await expect(detail.approverStatus("Jane Doe")).toHaveText("Approved");
    await expect(detail.approverStatus("John Smith")).toHaveText("Cancelled");
    await expect(detail.approveButton()).toHaveCount(0);
    await expect(detail.rejectButton()).toHaveCount(0);
  });
});
