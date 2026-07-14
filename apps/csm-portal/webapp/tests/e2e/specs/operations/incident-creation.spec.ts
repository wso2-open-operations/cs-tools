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
// Incident creation (POST /incidents). Every real submit here creates a
// permanent ServiceNow incident with no delete endpoint, so — same rule as
// change requests — the happy-path test is deliberately the only one that
// actually submits, tagged via e2eIncidentSubject, and uses the lowest
// Impact/Urgency so it doesn't read as a real, urgent incident to anyone
// reviewing staging afterward.
//

import { test, expect, withRole } from "../../fixtures/test";
import { IncidentCreatePage } from "../../pages/IncidentCreatePage";
import { e2eIncidentSubject, INCIDENT_CREATE } from "../../utils/selectors";

withRole(test, "approver");

test.describe("incident creation — page structure", () => {
  test("requires every backend-mandated field before Create incident is enabled", async ({ page }) => {
    const incident = new IncidentCreatePage(page);
    await incident.goto();

    await expect(incident.createButton()).toBeDisabled();

    await incident.shortDescriptionField().fill(e2eIncidentSubject("validation check"));
    await expect(incident.createButton()).toBeDisabled();

    await incident.selectOption("Category", "Inquiry / Help");
    await incident.selectOption("Subcategory", "Information Request");
    await incident.selectOption("Contact type", "Email");
    await incident.selectOption("Impact", "Low");
    await incident.selectOption("Urgency", "Low");
    // Short description, Category, Subcategory, Contact type, Impact, and
    // Urgency are all filled now, but Service (backend-required, not part
    // of the original field spec) still isn't — button must stay disabled.
    await expect(incident.createButton()).toBeDisabled();

    await incident.pickService("e");
    await expect(incident.createButton()).toBeEnabled();
  });

  test("resets and re-filters subcategory when category changes", async ({ page }) => {
    const incident = new IncidentCreatePage(page);
    await incident.goto();

    await incident.selectOption("Category", "Security");
    await incident.selectOption("Subcategory", "Phishing");

    await incident.selectOption("Category", "Service Interruption");

    await incident.requiredSelectLocator("Subcategory").click();
    await expect(page.getByRole("option", { name: "Phishing" })).toHaveCount(0);
    await expect(page.getByRole("option", { name: "Full Outage" })).toBeVisible();
  });
});

test.describe("incident creation — happy path", () => {
  test("creates a real incident and lands on its detail page", async ({ page }) => {
    // Real network round trips (service search, then create), plus a
    // navigation and a second fetch to load the detail page — comfortably
    // exceeds the 30s default.
    test.setTimeout(60_000);

    const incident = new IncidentCreatePage(page);
    await incident.goto();

    const subject = e2eIncidentSubject("e2e incident creation");
    await incident.fillRequiredFieldsAndSubmit({
      shortDescription: subject,
      category: "Inquiry / Help",
      subcategory: "Information Request",
      contactType: "Email",
      impact: "Low",
      urgency: "Low",
      serviceQuery: "e",
    });

    // CsmIncidentDetailPage titles itself with the incident's own subject
    // once loaded, which is the strongest available confirmation that the
    // record we just created (not some other one) is what's showing.
    await expect(
      page.getByRole("heading", { level: 5, name: subject }),
    ).toBeVisible({ timeout: 15_000 });
  });
});

test.describe("incidents tab", () => {
  test("lists incidents and links to the create page", async ({ page }) => {
    await page.goto("/operations?tab=incidents");
    await expect(page.getByRole("tab", { name: "Incidents" })).toBeVisible();
    await expect(
      page.getByRole("button", { name: "Create incident" }),
    ).toBeVisible();

    await page.getByRole("button", { name: "Create incident" }).click();
    await expect(page).toHaveURL(new RegExp(INCIDENT_CREATE.path.replace("/", "\\/")));
  });
});
