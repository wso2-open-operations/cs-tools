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
// The deployment lifecycle on the Deployments tab of the project details page,
// reached through the side nav as a user would: add, edit the description,
// rename and retype, create a non-production one, page the list, expand and
// collapse a card, and delete. A nested `validation` describe covers the modals'
// field gating and dismissal paths, and an `access` describe checks the tab is
// reachable per project type. Adding a product to a deployment is its own suite —
// see add-deployment-product.spec.ts.
//
// ⚠️ Writes to a REAL backend via POST /projects/{id}/deployments. Every test
// that needs a deployment creates its own so it can run alone, which is eight of
// them: add, edit-description, rename-and-retype, non-production create,
// expand/collapse, delete, and the two dismissal cases in `validation`. Only the
// delete test disposes of what it made, so a full run leaves seven behind.
//
// The rest create nothing: the six Add Deployment gating cases never submit, the
// pagination test only reads, and both access tests are read-only.
//
// "Delete" is a deactivation rather than a removal: it PATCHes
// `{ active: false }` to the same endpoint the edit modal uses, and the record
// drops out of the list. There is no DELETE verb, so the deployments left by the
// other tests can be tidied from the UI but not truly removed.
//
// The name carries a timestamp because the backend rejects a duplicate with
// `409 {"message":"A deployment with the same name already exists for the
// project."}` (confirmed live). A fixed name therefore only ever works on the
// first run; every later one failed, and — because the response wait originally
// required a 2xx — failed as an unexplained 180s timeout rather than as a 409.
//
// Everything that writes is scoped to Managed Cloud Subscription: the Add
// Deployment button is withheld for a Restricted project, and deployment access
// is a per-project feature flag. The read-only `access` describe runs against
// every project in DEPLOYMENT_ACCESS_PROJECTS, since it creates nothing.
//

import { test, expect, withSession, type Page } from "../../fixtures/test";
import { ProjectDeploymentsPage } from "../../pages/ProjectDeploymentsPage";
import {
  DEPLOYMENT_ACCESS_PROJECTS,
  DEPLOYMENT_INPUT,
  DeploymentType,
  PROJECTS,
  ProjectType,
} from "../../config/testData";
import { expectSuccess, skipWhenUnconfigured } from "../../utils/caseFlows";
import { createDeployment } from "../../utils/deploymentFlows";
import { DEPLOYMENTS_LIST } from "../../utils/selectors";

withSession(test);


/**
 * Counts the deployment writes a test causes, so "nothing was saved" can be
 * asserted rather than assumed.
 *
 * Only the two write endpoints. Listing deployments is *also* a POST — to
 * `/deployments/search` — so matching "POST to anything under deployments"
 * would count every refetch as a write.
 *
 * @param page - Test page.
 * @param projectId - Project whose deployments to watch.
 * @returns A reader for the count so far.
 */
function countDeploymentWrites(page: Page, projectId: string): () => number {
  let writes = 0;
  page.on("request", (request) => {
    const path = new URL(request.url()).pathname;
    const method = request.method();
    const isCreate =
      method === "POST" && path.endsWith(`/projects/${projectId}/deployments`);
    const isUpdate =
      method === "PATCH" &&
      new RegExp(`/projects/${projectId}/deployments/[^/]+$`).test(path);
    if (isCreate || isUpdate) writes += 1;
  });
  return () => writes;
}

test.describe("Deployment", () => {
  // Spans a dashboard load, a side-nav navigation, a tab switch and a
  // filters-backed dropdown; the 30s default is not enough.
  test.describe.configure({ timeout: 180_000 });

  const project = PROJECTS[ProjectType.MANAGED_CLOUD_SUBSCRIPTION];

  test(`${ProjectType.MANAGED_CLOUD_SUBSCRIPTION} — adds a deployment`, async ({
    page,
  }) => {
    skipWhenUnconfigured(project);
    await createDeployment(page, project);
  });

  test(`${ProjectType.MANAGED_CLOUD_SUBSCRIPTION} — edits a deployment's description`, async ({
    page,
  }) => {
    skipWhenUnconfigured(project);

    // Edits a deployment this test just created rather than a pre-existing one,
    // so a run can never rewrite a fixture another spec depends on.
    const { deployments, deploymentName } = await createDeployment(page, project);

    await deployments.openEditDeploymentModal(deploymentName);

    // The form opens populated from the record, so this also confirms the modal
    // is editing the deployment that was clicked.
    await expect(deployments.editDeploymentNameInput()).toHaveValue(
      deploymentName,
    );
    await expect(deployments.editDeploymentDescriptionInput()).toHaveValue(
      DEPLOYMENT_INPUT.description,
    );

    await deployments
      .editDeploymentDescriptionInput()
      .fill(DEPLOYMENT_INPUT.updatedDescription);
    await expect(deployments.editDeploymentSubmitButton()).toBeEnabled();

    const updateResponse = await deployments.submitDeploymentEdit(project.id);
    await expectSuccess(updateResponse, "update deployment");

    const payload = JSON.parse(updateResponse.request().postData() ?? "{}") as {
      description?: string;
    };
    expect(payload.description).toBe(DEPLOYMENT_INPUT.updatedDescription);

    await expect(deployments.modal()).toBeHidden();

    // The card shows the new description. Scoped to this run's card: every
    // deployment on the project carries a description, and the base text is
    // shared, so a page-wide match would prove nothing.
    await expect(deployments.deploymentCard(deploymentName)).toContainText(
      DEPLOYMENT_INPUT.updatedDescription,
      { timeout: 30_000 },
    );

    console.log(
      `Edited deployment (${ProjectType.MANAGED_CLOUD_SUBSCRIPTION}): ` +
        `${deploymentName} description updated`,
    );
  });

  test(`${ProjectType.MANAGED_CLOUD_SUBSCRIPTION} — edits a deployment's name and type`, async ({
    page,
  }) => {
    skipWhenUnconfigured(project);

    // The description path is covered above; this exercises the other two fields
    // handleSubmit can send, which it builds independently of each other.
    const { deployments, deploymentName } = await createDeployment(
      page,
      project,
    );
    const renamed = `${deploymentName} ${DEPLOYMENT_INPUT.renameSuffix}`;

    // The rename first, on its own, because the card can be checked afterwards:
    // a renamed production deployment stays where it was in the list.
    await deployments.openEditDeploymentModal(deploymentName);
    await deployments.editDeploymentNameInput().fill(renamed);

    const renameResponse = await deployments.submitDeploymentEdit(project.id);
    await expectSuccess(renameResponse, "rename deployment");
    expect(
      (JSON.parse(renameResponse.request().postData() ?? "{}") as {
        name?: string;
      }).name,
    ).toBe(renamed);

    await expect(deployments.modal()).toBeHidden();
    await expect(deployments.deploymentEntry(renamed)).toHaveCount(1, {
      timeout: 30_000,
    });
    await expect(deployments.deploymentEntry(deploymentName)).toHaveCount(0);

    // Then the type, which is a separate field in the body — note the edit modal
    // calls it `typeKey` where the create form sends `deploymentTypeKey`.
    await deployments.openEditDeploymentModal(renamed);
    await deployments.selectEditDeploymentType(DEPLOYMENT_INPUT.alternateType);

    const retypeResponse = await deployments.submitDeploymentEdit(project.id);
    await expectSuccess(retypeResponse, "change deployment type");

    // Asserted as "a numeric key" rather than against a value: the keys are
    // tenant data, and neither the request nor the response carries the label.
    const payload = JSON.parse(retypeResponse.request().postData() ?? "{}") as {
      typeKey?: number;
    };
    expect(
      typeof payload.typeKey,
      "the type change should be sent as a numeric key",
    ).toBe("number");

    await expect(deployments.modal()).toBeHidden();

    // No card assertion after the type change: the list orders production
    // deployments first, so one retyped to Development drops off the first page
    // altogether (verified live). Paging to find it would assert the sort rather
    // than the edit.

    console.log(
      `Renamed deployment (${ProjectType.MANAGED_CLOUD_SUBSCRIPTION}): ` +
        `${renamed}, then retyped to ${DEPLOYMENT_INPUT.alternateType}`,
    );
  });

  test(`${ProjectType.MANAGED_CLOUD_SUBSCRIPTION} — offers every deployment type and creates a non-production one`, async ({
    page,
  }) => {
    skipWhenUnconfigured(project);

    const deployments = new ProjectDeploymentsPage(page);
    await deployments.openDeploymentsTab(project.id);
    await deployments.openAddDeploymentModal();

    // Every type the suite knows about is on offer, in the order the config
    // records. The options come from filter metadata, so this asserts the
    // tenant's vocabulary matches the fixture as much as it asserts the control.
    expect(await deployments.typeOptionLabels()).toEqual(
      Object.values(DeploymentType),
    );

    // And one can actually be created as something other than production — every
    // other test here uses Primary Production.
    const deploymentName = `${DEPLOYMENT_INPUT.namePrefix} ${new Date()
      .toISOString()
      .slice(0, 23)
      .replace(/[:T.]/g, "-")}`;

    await deployments.fillDeployment(
      deploymentName,
      DEPLOYMENT_INPUT.alternateType,
      DEPLOYMENT_INPUT.description,
    );
    await expect(deployments.modalSubmitButton()).toBeEnabled();

    const [createResponse] = await Promise.all([
      page.waitForResponse(
        (r) =>
          new URL(r.url()).pathname.endsWith(
            `/projects/${project.id}/deployments`,
          ) && r.request().method() === "POST",
      ),
      deployments.submit(),
    ]);
    await expectSuccess(createResponse, "create non-production deployment");

    // The type on the wire. The create form sends a numeric key rather than the
    // label, and the response carries only id, createdOn and createdBy — so this
    // is the only place the chosen type is observable.
    const createPayload = JSON.parse(
      createResponse.request().postData() ?? "{}",
    ) as { deploymentTypeKey?: number };
    expect(
      typeof createPayload.deploymentTypeKey,
      "the chosen type should be sent as a numeric key",
    ).toBe("number");

    const created = (await createResponse.json()) as { id?: string };
    expect(created.id, "backend returned no deployment id").toBeTruthy();

    await expect(deployments.modal()).toBeHidden();

    // No card assertion: the list orders production deployments first, so a
    // Development one is not among the first ten and paging to it would assert
    // the sort rather than the creation.

    console.log(
      `Created ${DEPLOYMENT_INPUT.alternateType} deployment: ${deploymentName}`,
    );
  });

  test(`${ProjectType.MANAGED_CLOUD_SUBSCRIPTION} — pages the deployments list and changes its page size`, async ({
    page,
  }) => {
    skipWhenUnconfigured(project);

    const deployments = new ProjectDeploymentsPage(page);
    await deployments.openDeploymentsTab(project.id);

    // Wait for the first page before reading the controls: while the search is
    // in flight the count is unknown and next is disabled for that reason alone.
    await expect(deployments.deploymentCards().first()).toBeVisible();
    await expect(deployments.displayedRows()).toBeVisible();

    const pageSize = DEPLOYMENTS_LIST.defaultRowsPerPage;
    expect(await deployments.displayedFromRow()).toBe(1);
    await expect(deployments.previousPageButton()).toBeDisabled();

    const next = deployments.nextPageButton();
    await expect(next).toBeVisible();

    // A project with a single page of deployments has nothing to page through,
    // and a disabled control is the right behaviour there.
    if (await next.isEnabled()) {
      await next.click();
      await expect
        .poll(() => deployments.displayedFromRow(), { timeout: 30_000 })
        .toBe(pageSize + 1);
      await expect(deployments.previousPageButton()).toBeEnabled();

      await deployments.previousPageButton().click();
      await expect
        .poll(() => deployments.displayedFromRow(), { timeout: 30_000 })
        .toBe(1);
      await expect(deployments.previousPageButton()).toBeDisabled();
    } else {
      console.log(
        `Deployments fit on one page (${ProjectType.MANAGED_CLOUD_SUBSCRIPTION})`,
      );
    }

    // A larger page fits more rows — but only up to however many exist, so the
    // expectation comes from the list's own total rather than assuming there are
    // more than one page's worth. On this project there are far more; on one with
    // a handful, `min` is what keeps the assertion true instead of demanding rows
    // that cannot be shown.
    const total = await deployments.displayedTotal();
    expect(total, "the range text should report a total").not.toBeNull();

    const larger = DEPLOYMENTS_LIST.rowsPerPageOptions.at(-1) as number;
    await deployments.selectRowsPerPage(larger);
    await expect
      .poll(() => deployments.displayedRowCount(), { timeout: 30_000 })
      .toBe(Math.min(larger, total as number));

    console.log(
      `Paged the deployments list and resized it to ${larger} per page`,
    );
  });

  test(`${ProjectType.MANAGED_CLOUD_SUBSCRIPTION} — expands and collapses a deployment card`, async ({
    page,
  }) => {
    skipWhenUnconfigured(project);

    const { deployments, deploymentName } = await createDeployment(
      page,
      project,
    );

    // Collapsed to begin with, and its contents are not merely hidden — the
    // accordion unmounts them, which is why nothing inside can be reached until
    // it is opened.
    await expect(
      deployments.deploymentSummary(deploymentName),
    ).toHaveAttribute("aria-expanded", "false");
    await expect(deployments.addProductButton()).toHaveCount(0);

    await deployments.expandDeployment(deploymentName);
    await expect(deployments.addProductButton()).toBeVisible();

    await deployments.collapseDeployment(deploymentName);
    await expect(deployments.addProductButton()).toHaveCount(0);

    console.log(`Expanded and collapsed ${deploymentName}`);
  });

  test(`${ProjectType.MANAGED_CLOUD_SUBSCRIPTION} — deletes a deployment`, async ({
    page,
  }) => {
    skipWhenUnconfigured(project);

    // Deletes a deployment this test just created — the only test here that
    // leaves the project as it found it.
    const { deployments, deploymentName } = await createDeployment(page, project);

    await deployments.openDeleteDeploymentModal(deploymentName);

    const deleteResponse = await deployments.confirmDeleteDeployment(
      project.id,
    );
    await expectSuccess(deleteResponse, "delete deployment");

    // Deleting reuses the update endpoint, so the body is the only thing that
    // distinguishes it from an edit: the record is deactivated, not removed.
    const payload = JSON.parse(deleteResponse.request().postData() ?? "{}") as {
      active?: boolean;
    };
    expect(payload.active, "a delete should deactivate the deployment").toBe(
      false,
    );

    await expect(deployments.modal()).toBeHidden();

    // And it leaves the list — the deactivated record is filtered out.
    await expect(deployments.deploymentEntry(deploymentName)).toHaveCount(0, {
      timeout: 30_000,
    });

    console.log(
      `Deleted deployment (${ProjectType.MANAGED_CLOUD_SUBSCRIPTION}): ` +
        `${deploymentName}`,
    );
  });

  //
  // Validation. The Add Deployment cases create nothing at all — the modal gates
  // its confirm button, the duplicate is rejected by the backend, and cancelling
  // never submits. The edit and delete cases need a deployment to act on, so
  // they make their own and leave it behind.
  //
  test.describe("validation", () => {
    /** Path of the create endpoint, distinct from the /deployments/search POST
     * that also fires on this page. */
    const createPath = `/projects/${project.id}/deployments`;

    test("keeps submit disabled until every required field is filled", async ({
      page,
    }) => {
      skipWhenUnconfigured(project);
      const deployments = new ProjectDeploymentsPage(page);
      await deployments.openDeploymentsTab(project.id);
      await deployments.openAddDeploymentModal();

      // isValid requires all three; check each step rather than only the endpoint,
      // so a rule that stopped covering one field would still fail here.
      await expect(deployments.modalSubmitButton()).toBeDisabled();

      await deployments.nameInput().fill(DEPLOYMENT_INPUT.namePrefix);
      await expect(deployments.modalSubmitButton()).toBeDisabled();

      await deployments.selectType(DEPLOYMENT_INPUT.type);
      await expect(deployments.modalSubmitButton()).toBeDisabled();

      await deployments.descriptionInput().fill(DEPLOYMENT_INPUT.description);
      await expect(deployments.modalSubmitButton()).toBeEnabled();
    });

    test("keeps submit disabled when only the description is missing", async ({
      page,
    }) => {
      skipWhenUnconfigured(project);
      const deployments = new ProjectDeploymentsPage(page);
      await deployments.openDeploymentsTab(project.id);
      await deployments.openAddDeploymentModal();

      await deployments.nameInput().fill(DEPLOYMENT_INPUT.namePrefix);
      await deployments.selectType(DEPLOYMENT_INPUT.type);

      await expect(deployments.modalSubmitButton()).toBeDisabled();
    });

    test("keeps submit disabled for a whitespace-only name", async ({ page }) => {
      skipWhenUnconfigured(project);
      const deployments = new ProjectDeploymentsPage(page);
      await deployments.openDeploymentsTab(project.id);
      await deployments.openAddDeploymentModal();

      // isValid trims before comparing, so spaces must not count as a name.
      await deployments.nameInput().fill("   ");
      await deployments.selectType(DEPLOYMENT_INPUT.type);
      await deployments.descriptionInput().fill(DEPLOYMENT_INPUT.description);

      await expect(deployments.modalSubmitButton()).toBeDisabled();
    });

    test("rejects a duplicate deployment name", async ({ page }) => {
      skipWhenUnconfigured(project);
      const deployments = new ProjectDeploymentsPage(page);
      await deployments.openDeploymentsTab(project.id);

      // The colliding name is READ OFF THE PAGE rather than taken from config.
      //
      // It used to come from `DEPLOYMENT_INPUT.existingName`, and that silently
      // stopped working: every deployment this suite creates carries a
      // timestamp suffix, so nothing is ever named exactly "Automation Test
      // Deployment" and the guard below skipped the test on every run — the
      // duplicate rule went unexercised while the suite reported green.
      //
      // Taking the name from the list cannot go stale, and it also removes the
      // hazard the old guard existed to prevent: a name that is listed is by
      // definition already taken, so the create cannot succeed and leave a
      // permanent deployment behind.
      //
      // Being listed also means being active; a deactivated one is filtered out.
      const existingNames = await deployments.deploymentNames();
      test.skip(
        existingNames.length === 0,
        `This project lists no deployments, so there is no name to collide ` +
          `with. Create one, or run the "adds a deployment" case first.`,
      );
      const duplicateName = existingNames[0];

      await deployments.openAddDeploymentModal();

      await deployments.fillDeployment(
        duplicateName,
        DEPLOYMENT_INPUT.type,
        DEPLOYMENT_INPUT.description,
      );

      const [response] = await Promise.all([
        page.waitForResponse(
          (r) =>
            new URL(r.url()).pathname.endsWith(createPath) &&
            r.request().method() === "POST",
        ),
        deployments.submit(),
      ]);

      // The backend enforces name uniqueness per project. Nothing is created, so
      // this case is safe to re-run.
      expect(
        response.status(),
        `creating a second deployment named "${duplicateName}" should be rejected`,
      ).toBe(409);
      expect(await response.text()).toContain("already exists");

      // The modal stays open on failure. Note the UI surfaces no visible error
      // here — from the user's side the button simply does nothing — which is
      // worth treating as a product defect rather than expected behaviour.
      await expect(deployments.modal()).toBeVisible();

      console.log(
        `Deployment validation: duplicate name "${duplicateName}" rejected with 409`,
      );
    });

    test("cancelling closes the modal without creating a deployment", async ({
      page,
    }) => {
      skipWhenUnconfigured(project);
      const deployments = new ProjectDeploymentsPage(page);
      await deployments.openDeploymentsTab(project.id);
      await deployments.openAddDeploymentModal();

      let createAttempts = 0;
      page.on("request", (r) => {
        if (
          new URL(r.url()).pathname.endsWith(createPath) &&
          r.method() === "POST"
        ) {
          createAttempts += 1;
        }
      });

      await deployments.fillDeployment(
        `${DEPLOYMENT_INPUT.namePrefix} cancelled`,
        DEPLOYMENT_INPUT.type,
        DEPLOYMENT_INPUT.description,
      );
      await deployments.cancel();

      expect(createAttempts, "cancel must not send a create request").toBe(0);
    });

    test("keeps submit disabled for a whitespace-only description", async ({
      page,
    }) => {
      skipWhenUnconfigured(project);
      const deployments = new ProjectDeploymentsPage(page);
      await deployments.openDeploymentsTab(project.id);
      await deployments.openAddDeploymentModal();

      // Description is trimmed before the check too, so spaces must not satisfy
      // it any more than they do for the name.
      await deployments.nameInput().fill(DEPLOYMENT_INPUT.namePrefix);
      await deployments.selectType(DEPLOYMENT_INPUT.type);
      await deployments.descriptionInput().fill("   ");

      await expect(deployments.modalSubmitButton()).toBeDisabled();
    });

    test(`${ProjectType.MANAGED_CLOUD_SUBSCRIPTION} — leaves a deployment unchanged when the edit modal is dismissed`, async ({
      page,
    }) => {
      skipWhenUnconfigured(project);

      const { deployments, deploymentName } = await createDeployment(
        page,
        project,
      );
      const writes = countDeploymentWrites(page, project.id);

      // Submitting with nothing changed sends nothing: the modal builds its body
      // from the changed fields only and just closes when that is empty. Unlike
      // the Add modal, Update is always enabled — its `isValid` checks only that
      // a project and deployment are in hand — so this is the rule that stands in
      // for required-field gating here.
      await deployments.openEditDeploymentModal(deploymentName);
      await deployments.editDeploymentSubmitButton().click();
      await expect(deployments.modal()).toBeHidden();
      expect(writes(), "an unchanged edit must not be sent").toBe(0);

      // And a change that is typed but cancelled is discarded.
      await deployments.openEditDeploymentModal(deploymentName);
      await deployments
        .editDeploymentDescriptionInput()
        .fill(DEPLOYMENT_INPUT.updatedDescription);
      await deployments.cancel();
      await expect(deployments.modal()).toBeHidden();

      expect(writes(), "cancelling must not send an update").toBe(0);
      await expect(deployments.deploymentCard(deploymentName)).toContainText(
        DEPLOYMENT_INPUT.description,
      );
    });

    test(`${ProjectType.MANAGED_CLOUD_SUBSCRIPTION} — keeps the deployment when the delete dialog is dismissed`, async ({
      page,
    }) => {
      skipWhenUnconfigured(project);

      const { deployments, deploymentName } = await createDeployment(
        page,
        project,
      );
      const writes = countDeploymentWrites(page, project.id);

      await deployments.openDeleteDeploymentModal(deploymentName);
      await deployments.dismissDeleteDeploymentModal();
      await expect(deployments.modal()).toBeHidden();

      // Still listed — Go Back is not a slow delete.
      await expect(deployments.deploymentEntry(deploymentName)).toHaveCount(1);
      expect(writes(), "going back must not delete the deployment").toBe(0);
    });
  });

  //
  // Deployment access. Read-only — nothing here creates a deployment, so it is
  // safe to run against every project that has the tab.
  //
  test.describe("access", () => {
    for (const projectType of DEPLOYMENT_ACCESS_PROJECTS) {
      const accessProject = PROJECTS[projectType];

      test(`${projectType} — reaches the Deployments tab and its list`, async ({
        page,
      }) => {
        test.skip(
          !accessProject.id,
          `${projectType} needs a project id. ` +
            `Fill it in tests/e2e/config/testData.ts.`,
        );

        const deployments = new ProjectDeploymentsPage(page);
        await deployments.openProjectDetails(accessProject.id);

        // The tab is gated on the project's `hasDeployments` flag, so its
        // presence is the access assertion — not decoration.
        await expect(deployments.deploymentsTab()).toBeVisible();
        await deployments.deploymentsTab().click();

        // The list resolves into one of its two real states: cards, or the empty
        // copy. Without this the test would pass against a still-loading tab.
        await expect(
          deployments
            .deploymentCards()
            .first()
            .or(deployments.emptyDeploymentsMessage()),
        ).toBeVisible();

        // Adding is offered, which is withheld for a Restricted project — so
        // this covers the closure-state rule for these fixtures at the same time.
        await expect(deployments.addDeploymentButton()).toBeVisible();

        const cards = await deployments.deploymentCards().count();
        console.log(`Deployments tab (${projectType}): ${cards} cards listed`);
      });
    }
  });
});
