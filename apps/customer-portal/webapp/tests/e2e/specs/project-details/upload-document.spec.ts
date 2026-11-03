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
// Uploading a document to a deployment.
//
// Project Details → Deployments → expand a deployment → Documents → Upload.
//
// The section reuses the case-attachment dialog but not its wording: in
// deployment mode UploadAttachmentModal titles itself "Add Document" and posts
// to `/deployments/{id}/attachments`, so the CASE_ATTACHMENTS constants do not
// apply.
//
// A successful run leaves a REAL document on the deployment — the portal
// exposes no delete for these, so there is no cleanup to do. The name is
// therefore stamped per run, so repeat runs and overlapping runs add
// distinguishable rows instead of contending for one.
//
// The file is set directly on the dropzone's hidden input. Clicking "Choose
// file" opens the OS picker, which Playwright cannot drive.
//
// ⚠️ KNOWN RED on staging as of 2026-10-06: the POST comes back 500
// `{"message":"Failed to create deployment attachment."}` for a well-formed
// payload (`name`, `type: "image/png"`, base64 `content`), on every file size
// tried. The failure is downstream of the BFF — note that message does not even
// match this repo's `service.bal`, which says "Failed to create a new
// attachment", so staging runs a different build. The test is deliberately left
// asserting success rather than tolerating the 500: it should go green the
// moment the service is fixed, and stay red until then.
//

import { test, expect, withSession } from "../../fixtures/test";
import { ProjectDeploymentsPage } from "../../pages/ProjectDeploymentsPage";
import { DEPLOYMENT_DOCUMENT_INPUT, PROJECTS } from "../../config/testData";
import { DEPLOYMENT_DOCUMENTS } from "../../utils/selectors";
import { isSuccess, skipWhenUnconfigured } from "../../utils/caseFlows";

withSession(test);

const project = PROJECTS[DEPLOYMENT_DOCUMENT_INPUT.projectType];

test.describe("Deployment Documents", () => {
  // A project-details load, a deployment expand, an upload round trip and the
  // list refetch.
  test.describe.configure({ timeout: 180_000 });

  test("uploads a document to a deployment", async ({ page }) => {
    skipWhenUnconfigured(project);

    const deployments = new ProjectDeploymentsPage(page);
    await deployments.openDeploymentsTab(project.id);

    // Act on whichever deployment the project actually has, rather than a name
    // from config: deployment names are project data and have changed under
    // this suite before, and any deployment will do for a document.
    const firstCard = deployments.deploymentCards().first();
    await expect(
      firstCard,
      "the project should have at least one deployment to attach a document to",
    ).toBeVisible({ timeout: 60_000 });

    const deploymentName = (await deployments.deploymentNames())[0];
    expect(deploymentName, "could not read a deployment name").toBeTruthy();
    await deployments.expandDeployment(deploymentName);

    // The Documents section belongs to the expanded card.
    await expect(deployments.documentsSection()).toBeVisible({
      timeout: 60_000,
    });

    // Stamped per run: the document is real while it exists, and two runs
    // overlapping must not contend for the same name.
    const documentName = `${DEPLOYMENT_DOCUMENT_INPUT.namePrefix} ${Date.now()}`;

    const response = await deployments.uploadDocument(
      DEPLOYMENT_DOCUMENT_INPUT.file,
      documentName,
    );

    // Asserted on the wire: the dialog closes either way, so only the response
    // shows the document was actually stored.
    expect(
      isSuccess(response.status()),
      `the upload failed (${response.status()})`,
    ).toBe(true);
    expect(new URL(response.url()).pathname).toMatch(
      /\/deployments\/[^/]+\/attachments$/,
    );

    // And it is listed. The dialog closing is not evidence of that — the list
    // refetches separately, which is the part a regression would break.
    await expect(deployments.uploadDocumentDialog()).toBeHidden({
      timeout: 30_000,
    });
    await expect(
      deployments.documentRow(documentName).first(),
      "the uploaded document should appear in the Documents list",
    ).toBeVisible({ timeout: 60_000 });

    // The empty state must be gone, which is what distinguishes "listed" from
    // "the section simply rendered".
    await expect(
      page.getByText(DEPLOYMENT_DOCUMENTS.emptyMessage),
    ).toHaveCount(0);

    console.log(
      `Deployment Documents: uploaded "${documentName}" to "${deploymentName}"`,
    );
  });
});
