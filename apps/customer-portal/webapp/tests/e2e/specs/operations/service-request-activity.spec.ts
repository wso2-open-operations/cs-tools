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
// Comments and attachments on a service request.
//
// A service request detail page is the case detail page: ServiceRequestDetails
// renders CaseDetailsContent with `isServiceRequest`, which hides Knowledge Base
// and Escalation, shows Related Change Requests only when the request has one,
// and leaves Activity and Attachments exactly as a case has them. The comment
// and attachment endpoints are the case ones too — `/cases/{id}/comments` and
// `/cases/{id}/attachments` — so the same page objects serve here.
//
// ⚠️ Both tests WRITE, and neither record can be removed:
//
// - a comment cannot be deleted, so the spec posts one only when its own earlier
//   comment is not already on the request, keyed on the fixed text;
// - the attachment could be deleted, but is deliberately kept — the listing
//   assertions need a file to be there, so it is uploaded once and left.
//
// Scoped to Managed Cloud Subscription, the only project with service requests.
//

import { test, expect, withSession } from "../../fixtures/test";
import { CaseAttachmentsPage } from "../../pages/CaseAttachmentsPage";
import { CaseDetailPage } from "../../pages/CaseDetailPage";
import {
  ATTACHMENT_FILES,
  PROJECTS,
  SERVICE_REQUEST_ACTIVITY_INPUT,
  SERVICE_REQUEST_VIEWS,
} from "../../config/testData";
import { CASE_ATTACHMENTS } from "../../utils/selectors";
import { expectSuccess } from "../../utils/caseFlows";

withSession(test);

test.describe("Service Request Activity", () => {
  // A cold service request load plus a tab switch and an upload; the 30s default
  // is not enough.
  test.describe.configure({ timeout: 180_000 });

  const view = SERVICE_REQUEST_VIEWS[0];
  const project = view ? PROJECTS[view.projectType] : undefined;

  test("comments on a service request", async ({ page }) => {
    test.skip(
      !view || !project?.id || !view.serviceRequestId,
      `A service request fixture is needed. Fill SERVICE_REQUEST_VIEWS in ` +
        `tests/e2e/config/testData.ts.`,
    );

    const serviceRequest = new CaseDetailPage(page);
    await serviceRequest.openServiceRequest(
      (project as { id: string }).id,
      view.serviceRequestId,
    );

    const text = SERVICE_REQUEST_ACTIVITY_INPUT.comment;

    // Post only when absent: comments cannot be deleted, so an unguarded post
    // would add one on every run.
    const alreadyPosted = (await serviceRequest.comment(text).count()) > 0;

    if (alreadyPosted) {
      console.log(`Service request: comment already present`);
    } else {
      const response = await serviceRequest.addComment(text);
      await expectSuccess(response, "add comment to service request");
    }

    // Listed on the request's activity, whichever run posted it.
    await expect(serviceRequest.comment(text)).toBeVisible({
      timeout: 30_000,
    });

    console.log(
      `Service request ${view.serviceRequestId}: comment ` +
        `${alreadyPosted ? "present" : "added"}`,
    );
  });

  test("attaches a file to a service request", async ({ page }) => {
    test.skip(
      !view || !project?.id || !view.serviceRequestId,
      `A service request fixture is needed. Fill SERVICE_REQUEST_VIEWS in ` +
        `tests/e2e/config/testData.ts.`,
    );

    const serviceRequest = new CaseDetailPage(page);
    await serviceRequest.openServiceRequest(
      (project as { id: string }).id,
      view.serviceRequestId,
    );

    const attachments = new CaseAttachmentsPage(page);
    await attachments.openTab();

    const kept = ATTACHMENT_FILES.kept;

    // Upload only when absent, and leave it in place: the listing assertions
    // below need a file to be there, and nothing removes it between runs.
    const alreadyAttached = (await attachments.attachment(kept.name).count()) > 0;

    if (alreadyAttached) {
      console.log(`Service request: ${kept.name} already attached`);
    } else {
      const response = await attachments.upload(kept.path);
      await expectSuccess(response, "upload attachment to service request");
    }

    // Listed under its own name. Asserted as "at least one row" rather than by
    // visibility: the list renders each row twice for responsive layout.
    await expect(attachments.attachment(kept.name)).not.toHaveCount(0);

    const row = attachments.attachmentRow(kept.name);
    await expect(row).toContainText(kept.name);
    await expect(row).toContainText(kept.size);
    await expect
      .soft(row, "uploader")
      .toContainText(CASE_ATTACHMENTS.row.uploadedByPrefix);

    // The tab's own count agrees with the number of rows — a mismatch means one
    // of the two is stale.
    const names = await attachments.listedFileNames();
    expect(await attachments.tabCount()).toBe(names.length);

    console.log(
      `Service request ${view.serviceRequestId}: ${names.length} attachment(s), ` +
        `${kept.name} ${alreadyAttached ? "present" : "uploaded"}`,
    );
  });
});
