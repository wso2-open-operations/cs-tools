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
// Change request creation (POST /change-requests). Unlike case creation,
// there's no safe low-severity option to file under — every real submit
// here creates a permanent ServiceNow change request with no delete
// endpoint, so the happy-path test is deliberately the only one that
// actually submits, and its subject is always E2E-tagged (see
// e2eChangeRequestSubject) so it's identifiable in staging afterward.
//

import { test, expect, withRole } from "../../fixtures/test";
import { ChangeRequestCreatePage } from "../../pages/ChangeRequestCreatePage";
import { e2eChangeRequestSubject } from "../../utils/selectors";

withRole(test, "approver");

test.describe("change request creation — page structure", () => {
  test("requires a subject before Create change request is enabled", async ({ page }) => {
    const cr = new ChangeRequestCreatePage(page);
    await cr.goto();

    await expect(cr.createButton()).toBeDisabled();
    await cr.subjectField().fill(e2eChangeRequestSubject("validation check"));
    await expect(cr.createButton()).toBeEnabled();
  });
});

test.describe("change request creation — happy path", () => {
  test("creates a real change request and lands on its detail page", async ({ page }) => {
    // Real network round trip to create, then a navigation and a second
    // fetch to load the detail page — comfortably exceeds the 30s default.
    test.setTimeout(60_000);

    const cr = new ChangeRequestCreatePage(page);
    await cr.goto();

    const subject = e2eChangeRequestSubject("e2e change request creation");
    await cr.fillSubjectAndSubmit(subject);

    // CsmChangeRequestDetailPage titles itself with the CR's own subject
    // once loaded, which is the strongest available confirmation that the
    // record we just created (not some other one) is what's showing.
    await expect(
      page.getByRole("heading", { level: 5, name: subject }),
    ).toBeVisible({ timeout: 15_000 });
  });
});
