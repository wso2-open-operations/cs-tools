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
// Incident detail (edit + comment). Unlike incident-creation.spec.ts (which
// only ever submits once, tagged, and asserts nothing beyond "it landed on
// its own detail page"), these specs need a live incident to edit/comment
// on — but there's still no delete endpoint, so we never touch a
// pre-existing record. Each test self-provisions its own tagged incident via
// IncidentCreatePage first, reads its id back off the post-create URL, and
// only then drives IncidentDetailPage against that one record. If
// provisioning itself fails (e.g. no seeded Service to pick in this staging
// tenant), the test self-skips rather than failing — same rule as the
// approval self-skip below.
//

import { test, expect, withRole } from "../../fixtures/test";
import { IncidentCreatePage } from "../../pages/IncidentCreatePage";
import { IncidentDetailPage } from "../../pages/IncidentDetailPage";
import { e2eIncidentSubject } from "../../utils/selectors";

withRole(test, "approver");

/** Creates a tagged incident and returns its id, parsed off the URL the app
 * lands on after create. Returns `undefined` (instead of throwing) when
 * provisioning didn't make it to the detail page, so callers can self-skip. */
async function provisionIncident(
  page: import("@playwright/test").Page,
  label: string,
): Promise<{ id: string; subject: string } | undefined> {
  const incident = new IncidentCreatePage(page);
  await incident.goto();

  const subject = e2eIncidentSubject(label);
  await incident.fillRequiredFieldsAndSubmit({
    shortDescription: subject,
    category: "Inquiry / Help",
    subcategory: "Information Request",
    channel: "Email",
    impact: "Low",
    urgency: "Low",
    serviceQuery: "e",
  });

  const match = page.url().match(/\/operations\/incidents\/([^/?#]+)/);
  if (!match) return undefined;
  return { id: match[1], subject };
}

test.describe("incident detail — edit fields", () => {
  test("edits Subject and an internal work note, then reflects on the detail page", async ({ page }) => {
    test.setTimeout(60_000);

    const provisioned = await provisionIncident(page, "e2e incident detail edit");
    test.skip(!provisioned, "incident provisioning did not reach the detail page");
    const { id, subject } = provisioned!;

    const detail = new IncidentDetailPage(page);
    await detail.goto(id);

    await detail.openEditDialog();
    const editedSubject = `${subject} [edited]`;
    await detail.subjectField().fill(editedSubject);
    // Resolution code/notes only render once the incident's State is
    // "Resolved"/"Closed" — a New incident's State select only offers
    // "New" / "In Progress", so
    // those fields never appear here. "Internal work note" is always
    // present and optional, same as the resolution fields would have been.
    await detail.workNoteField().fill("[E2E] work note added by incident-detail.spec.ts");
    await detail.saveEdit();

    await expect(
      page.getByRole("heading", { level: 5, name: editedSubject }),
    ).toBeVisible({ timeout: 15_000 });
  });
});

test.describe("incident detail — add work note / comment", () => {
  test("adds an internal work note and it appears on the incident", async ({ page }) => {
    test.setTimeout(60_000);

    const provisioned = await provisionIncident(page, "e2e incident detail comment");
    test.skip(!provisioned, "incident provisioning did not reach the detail page");
    const { id } = provisioned!;

    const detail = new IncidentDetailPage(page);
    await detail.goto(id);

    const note = `[E2E] work note — ${new Date().toISOString()}`;
    await detail.addComment(note, { internal: true });

    await expect(page.getByText(note)).toBeVisible({ timeout: 15_000 });
  });
});
