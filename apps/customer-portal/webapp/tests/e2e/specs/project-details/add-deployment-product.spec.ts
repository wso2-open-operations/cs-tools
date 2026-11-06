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
// Products on a deployment: adding one, then managing it through the Manage
// Product modal — a new description, and an update-history entry.
//
// ⚠️ Writes to a REAL backend. The test creates its own deployment first rather
// than using one that already exists, so it never edits a record another spec
// depends on; that deployment is left behind, and the product with it.
//
// The deployment lifecycle itself — add, edit, delete — is add-deployment.spec.ts.
//
// Two visits to the Manage Product modal, because "Save Changes" closes it and
// the update save sends only the update list: a description left unsaved would
// be discarded rather than carried along.
//

import { test, expect, withSession, type Page } from "../../fixtures/test";
import { ProjectDeploymentsPage } from "../../pages/ProjectDeploymentsPage";
import {
  DEPLOYMENT_PRODUCT_INPUT,
  PROJECTS,
  ProjectType,
} from "../../config/testData";
import { expectSuccess, skipWhenUnconfigured } from "../../utils/caseFlows";
import { createDeployment } from "../../utils/deploymentFlows";
import { MANAGE_PRODUCT } from "../../utils/selectors";

withSession(test);

/**
 * Today's date, formatted for a `date` input.
 *
 * Built from local date parts rather than an ISO string, because `toISOString`
 * is UTC and would land on the wrong day either side of midnight for anyone not
 * on UTC.
 *
 * @returns A "YYYY-MM-DD" value.
 */
function today(): string {
  const date = new Date();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${date.getFullYear()}-${month}-${day}`;
}


/**
 * Counts the product writes a test causes, so "nothing was saved" can be
 * asserted rather than assumed.
 *
 * Only the two write endpoints. Reading a deployment's products is *also* a
 * POST — to `/products/search` — so matching "POST to anything under products"
 * would count every refetch as a write.
 *
 * @param page - Test page.
 * @returns A reader for the count so far.
 */
function countProductWrites(page: Page): () => number {
  let writes = 0;
  page.on("request", (request) => {
    const path = new URL(request.url()).pathname;
    const method = request.method();
    const isCreate =
      method === "POST" && /\/deployments\/[^/]+\/products$/.test(path);
    const isUpdate =
      method === "PATCH" &&
      /\/deployments\/[^/]+\/products\/[^/]+$/.test(path);
    if (isCreate || isUpdate) writes += 1;
  });
  return () => writes;
}

test.describe("Deployment Product", () => {
  // Creates a deployment, adds a product and reopens the manage modal twice, so
  // this needs the create flow's budget on top of its own.
  test.describe.configure({ timeout: 180_000 });

  const project = PROJECTS[ProjectType.MANAGED_CLOUD_SUBSCRIPTION];

  /**
   * Creates a deployment and adds the configured product to it.
   *
   * Both tests need a product to act on, and each makes its own rather than
   * leaning on the other having run — which holds in a whole-file run but not
   * when one test is run alone.
   *
   * @param page - Test page.
   * @returns The page object and the deployment's name.
   */
  async function addProductToNewDeployment(page: Page): Promise<{
    deployments: ProjectDeploymentsPage;
    deploymentName: string;
  }> {
    const { deployments, deploymentName } = await createDeployment(
      page,
      project,
    );

    //
    // The product goes on the deployment this test just created, so it adds
    // nothing to any pre-existing record — the deployment is the permanent
    // leftover, and the product rides along with it.
    //
    await deployments.expandDeployment(deploymentName);
    await deployments.openAddProductModal();

    await deployments.fillProduct(
      DEPLOYMENT_PRODUCT_INPUT.productName,
      DEPLOYMENT_PRODUCT_INPUT.version,
      DEPLOYMENT_PRODUCT_INPUT.cores,
      DEPLOYMENT_PRODUCT_INPUT.tps,
      DEPLOYMENT_PRODUCT_INPUT.description,
    );

    // Only product and version are required, so this also confirms the optional
    // fields did not leave the form invalid.
    await expect(deployments.productSubmitButton()).toBeEnabled();

    const productResponse = await deployments.submitProduct();
    await expectSuccess(productResponse, "add product to deployment");

    // The modal closes on success, so it staying open would mean the product was
    // accepted but the UI never moved on.
    await expect(deployments.modal()).toBeHidden();

    // And the product is listed under the deployment it was added to. The list
    // refetches after the create, so this needs more than the default 5s.
    //
    // Asserted as "at least one" rather than by visibility: the same product
    // could be listed more than once if a run ever added it twice, and that
    // should not turn into a strict-mode error here.
    await expect(
      deployments.deploymentProduct(
        DEPLOYMENT_PRODUCT_INPUT.listedProductName,
      ),
    ).not.toHaveCount(0, { timeout: 60_000 });

    console.log(
      `Added product to ${deploymentName}: ` +
        `${DEPLOYMENT_PRODUCT_INPUT.productName} ` +
        `${DEPLOYMENT_PRODUCT_INPUT.version}`,
    );

    return { deployments, deploymentName };
  }

  test(`${ProjectType.MANAGED_CLOUD_SUBSCRIPTION} — adds a product to a deployment`, async ({
    page,
  }) => {
    skipWhenUnconfigured(project);
    await addProductToNewDeployment(page);
  });

  test(`${ProjectType.MANAGED_CLOUD_SUBSCRIPTION} — updates a product's details and update history`, async ({
    page,
  }) => {
    skipWhenUnconfigured(project);

    const { deployments } = await addProductToNewDeployment(page);

    //
    // And manage it: a new description, then an update-history entry.
    //
    // Two visits to the modal, because "Save Changes" closes it (handleSave
    // calls onClose). The description also has to be saved on its own — the
    // update save sends only the update list, so an unsaved description would be
    // discarded rather than carried along with the update.
    //
    const listedProduct = DEPLOYMENT_PRODUCT_INPUT.listedProductName;

    await deployments.openManageProductModal(listedProduct);
    await deployments.openManageProductTab(MANAGE_PRODUCT.tabs.details);
    await deployments
      .manageProductDescriptionInput()
      .fill(DEPLOYMENT_PRODUCT_INPUT.updatedDescription);

    const descriptionResponse = await deployments.saveManageProduct(
      deployments.manageProductSaveButton(),
    );
    await expectSuccess(descriptionResponse, "save product description");
    await expect(deployments.modal()).toBeHidden();

    // Reopened, so the description is read back from the server rather than from
    // the form that was just typed into.
    //
    // Polled by reopening rather than waiting inside one open modal: the modal
    // seeds its fields from the product record once, when it opens, and never
    // re-reads them. Opening it before the list behind it has reloaded therefore
    // shows the old description for as long as it stays open — a longer timeout
    // on the field does not help, which a 30s one proved.
    await expect
      .poll(
        async () => {
          await deployments.openManageProductModal(listedProduct);
          const value = await deployments
            .manageProductDescriptionInput()
            .inputValue();
          if (value !== DEPLOYMENT_PRODUCT_INPUT.updatedDescription) {
            await deployments.closeManageProductModal();
          }
          return value;
        },
        {
          message: "the saved description should be read back from the record",
          timeout: 60_000,
          intervals: [1000, 2000, 3000],
        },
      )
      .toBe(DEPLOYMENT_PRODUCT_INPUT.updatedDescription);

    await deployments.openManageProductTab(MANAGE_PRODUCT.tabs.history);
    await deployments.fillNewUpdate(
      DEPLOYMENT_PRODUCT_INPUT.update.level,
      today(),
      DEPLOYMENT_PRODUCT_INPUT.update.description,
    );

    await expect(deployments.addUpdateButton()).toBeEnabled();
    const updateResponse = await deployments.saveManageProduct(
      deployments.addUpdateButton(),
    );
    await expectSuccess(updateResponse, "add update history entry");

    // Both footer buttons save through the same endpoint, so the body is what
    // says this was the update rather than another description save.
    const updatePayload = JSON.parse(
      updateResponse.request().postData() ?? "{}",
    ) as {
      updates?: { updateLevel?: number; date?: string; details?: string }[];
    };
    const added = updatePayload.updates?.find(
      (entry) =>
        entry.updateLevel === Number(DEPLOYMENT_PRODUCT_INPUT.update.level),
    );
    expect(added, "the update should carry the chosen level").toBeDefined();
    expect(added?.date).toBe(today());
    expect(added?.details).toBe(DEPLOYMENT_PRODUCT_INPUT.update.description);

    await expect(
      page.getByText(MANAGE_PRODUCT.updateAddedMessage),
    ).toBeVisible();

    // The product now reports that level as its current one — the visible
    // consequence of the entry, rendered with a "U" prefix.
    await expect(deployments.currentUpdateLevelLabel()).toBeVisible();
    await expect(
      deployments.currentUpdateLevel(DEPLOYMENT_PRODUCT_INPUT.update.level),
    ).toBeVisible({ timeout: 30_000 });

    console.log(
      `Updated ${listedProduct}: description saved, update level ` +
        `${DEPLOYMENT_PRODUCT_INPUT.update.level} applied ${today()}`,
    );
  });


  test(`${ProjectType.MANAGED_CLOUD_SUBSCRIPTION} — edits a product's core count and TPS`, async ({
    page,
  }) => {
    skipWhenUnconfigured(project);

    // The description path is covered above; cores and TPS are separate fields in
    // handleSave's body, built independently of each other and of the
    // description.
    const { deployments } = await addProductToNewDeployment(page);
    const listedProduct = DEPLOYMENT_PRODUCT_INPUT.listedProductName;

    await deployments.openManageProductModal(listedProduct);
    await expect(deployments.manageProductCoresInput()).toHaveValue(
      DEPLOYMENT_PRODUCT_INPUT.cores,
    );
    await expect(deployments.manageProductTpsInput()).toHaveValue(
      DEPLOYMENT_PRODUCT_INPUT.tps,
    );

    await deployments
      .manageProductCoresInput()
      .fill(DEPLOYMENT_PRODUCT_INPUT.editedCores);
    await deployments
      .manageProductTpsInput()
      .fill(DEPLOYMENT_PRODUCT_INPUT.editedTps);

    const response = await deployments.saveManageProduct(
      deployments.manageProductSaveButton(),
    );
    await expectSuccess(response, "save product cores and TPS");

    const payload = JSON.parse(response.request().postData() ?? "{}") as {
      cores?: number;
      tps?: number;
    };
    expect(payload.cores).toBe(Number(DEPLOYMENT_PRODUCT_INPUT.editedCores));
    expect(payload.tps).toBe(Number(DEPLOYMENT_PRODUCT_INPUT.editedTps));

    await expect(deployments.modal()).toBeHidden();

    // Read back from the record. Polled by reopening because the modal seeds its
    // fields once, on open, and never re-reads them.
    await expect
      .poll(
        async () => {
          await deployments.openManageProductModal(listedProduct);
          const value = await deployments
            .manageProductCoresInput()
            .inputValue();
          if (value !== DEPLOYMENT_PRODUCT_INPUT.editedCores) {
            await deployments.closeManageProductModal();
          }
          return value;
        },
        { timeout: 60_000, intervals: [1000, 2000, 3000] },
      )
      .toBe(DEPLOYMENT_PRODUCT_INPUT.editedCores);
    await expect(deployments.manageProductTpsInput()).toHaveValue(
      DEPLOYMENT_PRODUCT_INPUT.editedTps,
    );

    console.log(
      `Edited ${listedProduct}: ${DEPLOYMENT_PRODUCT_INPUT.editedCores} cores, ` +
        `${DEPLOYMENT_PRODUCT_INPUT.editedTps} TPS`,
    );
  });

  test(`${ProjectType.MANAGED_CLOUD_SUBSCRIPTION} — edits and deletes an update-history entry`, async ({
    page,
  }) => {
    skipWhenUnconfigured(project);

    const { deployments } = await addProductToNewDeployment(page);
    const listedProduct = DEPLOYMENT_PRODUCT_INPUT.listedProductName;
    const level = DEPLOYMENT_PRODUCT_INPUT.update.level;

    // An entry to act on. The add path itself is covered above.
    await deployments.openManageProductModal(listedProduct);
    await deployments.openManageProductTab(MANAGE_PRODUCT.tabs.history);
    await deployments.fillNewUpdate(
      level,
      today(),
      DEPLOYMENT_PRODUCT_INPUT.update.description,
    );
    await expectSuccess(
      await deployments.saveManageProduct(deployments.addUpdateButton()),
      "add update history entry",
    );

    // Edit it. The whole list is sent on every save, so the assertion is that
    // the entry at this level comes back carrying the new description.
    await deployments.updateRowEditButton(level).click();
    await deployments
      .editUpdateDescriptionInput()
      .fill(DEPLOYMENT_PRODUCT_INPUT.update.editedDescription);

    const editResponse = await deployments.saveManageProduct(
      deployments.editUpdateSaveButton(),
    );
    await expectSuccess(editResponse, "edit update history entry");

    const edited = (
      JSON.parse(editResponse.request().postData() ?? "{}") as {
        updates?: { updateLevel?: number; details?: string }[];
      }
    ).updates?.find((entry) => entry.updateLevel === Number(level));
    expect(edited?.details).toBe(
      DEPLOYMENT_PRODUCT_INPUT.update.editedDescription,
    );

    // Delete it. Immediate — no confirmation step, unlike deleting a product.
    await expect(deployments.updateRowDeleteButton(level)).toBeVisible();
    const deleteResponse = await deployments.saveManageProduct(
      deployments.updateRowDeleteButton(level),
    );
    await expectSuccess(deleteResponse, "delete update history entry");

    // Gone from the list that was sent — the save carries what remains, so its
    // absence is the deletion.
    const remaining = (
      JSON.parse(deleteResponse.request().postData() ?? "{}") as {
        updates?: { updateLevel?: number }[];
      }
    ).updates;
    expect(
      remaining?.some((entry) => entry.updateLevel === Number(level)),
      "the deleted level should not be in the saved list",
    ).toBe(false);

    await expect(deployments.updateRowDeleteButton(level)).toHaveCount(0);

    console.log(`Edited and deleted update U${level} on ${listedProduct}`);
  });

  test(`${ProjectType.MANAGED_CLOUD_SUBSCRIPTION} — offers only higher update levels and adds a second entry`, async ({
    page,
  }) => {
    skipWhenUnconfigured(project);

    const { deployments } = await addProductToNewDeployment(page);
    const listedProduct = DEPLOYMENT_PRODUCT_INPUT.listedProductName;
    const level = DEPLOYMENT_PRODUCT_INPUT.update.level;
    const nextLevel = DEPLOYMENT_PRODUCT_INPUT.update.nextLevel;

    await deployments.openManageProductModal(listedProduct);
    await deployments.openManageProductTab(MANAGE_PRODUCT.tabs.history);

    // A product with no history offers this level.
    expect(await deployments.newUpdateLevelOptions()).toContain(level);

    await deployments.fillNewUpdate(
      level,
      today(),
      DEPLOYMENT_PRODUCT_INPUT.update.description,
    );
    await expectSuccess(
      await deployments.saveManageProduct(deployments.addUpdateButton()),
      "add first update history entry",
    );

    // Wait for the readout to report the new level before reading the offer. The
    // save resolving is not the same as the tab having re-rendered from it —
    // reading the options straight after the response returned the pre-save list
    // (every level from 1 to 13), which looked like the filter not working at
    // all.
    await expect(deployments.currentUpdateLevel(level)).toBeVisible({
      timeout: 30_000,
    });

    // Now that the product is at that level, it is no longer on offer — the
    // select lists only levels above the current one, which is what stops the
    // same level being added twice.
    const offered = await deployments.newUpdateLevelOptions();
    expect(
      offered,
      `${level} should no longer be offered once the product is at it`,
    ).not.toContain(level);
    expect(offered).toContain(nextLevel);

    await deployments.fillNewUpdate(
      nextLevel,
      today(),
      DEPLOYMENT_PRODUCT_INPUT.update.description,
    );
    const secondResponse = await deployments.saveManageProduct(
      deployments.addUpdateButton(),
    );
    await expectSuccess(secondResponse, "add second update history entry");

    // Both entries are in the saved list — the second does not replace the first.
    const levels = (
      JSON.parse(secondResponse.request().postData() ?? "{}") as {
        updates?: { updateLevel?: number }[];
      }
    ).updates?.map((entry) => entry.updateLevel);
    expect(levels).toContain(Number(level));
    expect(levels).toContain(Number(nextLevel));

    console.log(
      `Added updates U${level} and U${nextLevel} to ${listedProduct}`,
    );
  });

  test(`${ProjectType.MANAGED_CLOUD_SUBSCRIPTION} — deletes a product from a deployment`, async ({
    page,
  }) => {
    skipWhenUnconfigured(project);

    const { deployments } = await addProductToNewDeployment(page);
    const listedProduct = DEPLOYMENT_PRODUCT_INPUT.listedProductName;

    await deployments.openDeleteProductModal(
      listedProduct,
      DEPLOYMENT_PRODUCT_INPUT.version,
    );

    const deleteResponse = await deployments.confirmDeleteProduct();
    await expectSuccess(deleteResponse, "delete deployment product");

    // Deleting reuses the product update endpoint, so the body is the only thing
    // that distinguishes it from an edit: the record is deactivated, not removed.
    const payload = JSON.parse(deleteResponse.request().postData() ?? "{}") as {
      active?: boolean;
    };
    expect(payload.active, "a delete should deactivate the product").toBe(false);

    await expect(deployments.modal()).toBeHidden();

    // And it leaves the deployment's product list. Scoped by the row control,
    // which carries the product label — the deployment card is this run's own,
    // so no other product could satisfy it.
    await expect(deployments.deploymentProduct(listedProduct)).toHaveCount(0, {
      timeout: 30_000,
    });

    console.log(`Deleted product from the deployment: ${listedProduct}`);
  });

  //
  // Validation. Nothing below saves anything, and each test counts the writes
  // that left the browser to prove it — the counter is armed *after* the setup,
  // which legitimately creates a deployment and a product to act on.
  //
  test.describe("validation", () => {
    test(`${ProjectType.MANAGED_CLOUD_SUBSCRIPTION} — gates the Add Product form and cancels without creating`, async ({
      page,
    }) => {
      skipWhenUnconfigured(project);

      const { deployments, deploymentName } = await createDeployment(
        page,
        project,
      );
      await deployments.expandDeployment(deploymentName);

      const writes = countProductWrites(page);
      await deployments.openAddProductModal();

      // Nothing chosen yet: product and version are the only required fields.
      await expect(deployments.productSubmitButton()).toBeDisabled();

      // Version is disabled until a product is chosen — the two are ordered, and
      // the version list is fetched per product.
      await expect(deployments.productVersionSelect()).toBeDisabled();

      await deployments.selectProduct(DEPLOYMENT_PRODUCT_INPUT.productName);
      await expect(deployments.productVersionSelect()).toBeEnabled();
      await expect(deployments.productSubmitButton()).toBeDisabled();

      await deployments.selectProductVersion(DEPLOYMENT_PRODUCT_INPUT.version);
      await expect(deployments.productSubmitButton()).toBeEnabled();

      // Core Count, TPS and Description are optional — left empty, the form is
      // still submittable.
      await expect(deployments.productCoresInput()).toHaveValue("");
      await expect(deployments.productTpsInput()).toHaveValue("");
      await expect(deployments.productDescriptionInput()).toHaveValue("");
      await expect(deployments.productSubmitButton()).toBeEnabled();

      await deployments.cancelProductModal();
      await expect(deployments.modal()).toBeHidden();

      // And nothing was added to the deployment.
      await expect(
        deployments.deploymentProduct(
          DEPLOYMENT_PRODUCT_INPUT.listedProductName,
        ),
      ).toHaveCount(0);
      expect(writes(), "cancelling must not create a product").toBe(0);
    });

    test(`${ProjectType.MANAGED_CLOUD_SUBSCRIPTION} — gates the Manage Product form and closes without saving`, async ({
      page,
    }) => {
      skipWhenUnconfigured(project);

      const { deployments } = await addProductToNewDeployment(page);
      const listedProduct = DEPLOYMENT_PRODUCT_INPUT.listedProductName;

      const writes = countProductWrites(page);
      await deployments.openManageProductModal(listedProduct);

      // Saving with nothing changed sends nothing: the modal builds its body
      // from the changed fields only, and returns early when that is empty. It
      // also stays open, since only a real save closes it.
      await deployments.manageProductSaveButton().click();
      await expect(deployments.modal()).toBeVisible();

      await deployments.openManageProductTab(MANAGE_PRODUCT.tabs.history);

      // The Add New Update form starts empty, and level and date are both
      // required. The section mounts after the tab does — its level list is
      // fetched — so this waits for the control rather than assuming it is
      // there.
      await expect(deployments.newUpdateLevelSelect()).toBeEnabled({
        timeout: 60_000,
      });
      await expect(deployments.newUpdateAppliedOnInput()).toHaveValue("");
      await expect(deployments.addUpdateButton()).toBeDisabled();

      await deployments.selectNewUpdateLevel(
        DEPLOYMENT_PRODUCT_INPUT.update.level,
      );
      await expect(
        deployments.addUpdateButton(),
        "a level without a date should not be addable",
      ).toBeDisabled();

      await deployments.newUpdateAppliedOnInput().fill(today());
      await expect(deployments.addUpdateButton()).toBeEnabled();

      // The update's own description is optional — the form is complete without
      // it.
      await expect(deployments.newUpdateDescriptionInput()).toHaveValue("");
      await expect(deployments.addUpdateButton()).toBeEnabled();

      await deployments.closeManageProductModal();
      expect(writes(), "closing must not save anything").toBe(0);
    });


    test(`${ProjectType.MANAGED_CLOUD_SUBSCRIPTION} — drops an invalid core count instead of saving it`, async ({
      page,
    }) => {
      skipWhenUnconfigured(project);

      const { deployments } = await addProductToNewDeployment(page);
      const listedProduct = DEPLOYMENT_PRODUCT_INPUT.listedProductName;

      await deployments.openManageProductModal(listedProduct);
      await deployments
        .manageProductCoresInput()
        .fill(DEPLOYMENT_PRODUCT_INPUT.invalidCores);

      const response = await deployments.saveManageProduct(
        deployments.manageProductSaveButton(),
      );

      // The value never reaches the backend as a number: the modal's validator
      // turns a negative into `undefined`, which JSON.stringify drops — so the
      // request goes out with an empty body and the backend rejects it.
      const payload = JSON.parse(response.request().postData() ?? "{}") as {
        cores?: number;
      };
      expect(
        payload.cores,
        "an invalid core count should not be sent",
      ).toBeUndefined();

      // ⚠️ Asserting a 400 records what happens today rather than endorsing it.
      // A negative core count is not caught in the form at all: it is turned
      // into nothing, sent as an empty update, and refused by the API. The user
      // sees whatever that error surfaces as, not "core count must be positive".
      // Worth treating as a product defect.
      // The wording changed on staging (live 2026-10-22); either is the API
      // refusing an update that carries nothing.
      expect(response.status(), "the empty update is rejected").toBe(400);
      expect(await response.text()).toMatch(
        /At least one of cores or tps or description or updates should be provided|Provide either cores\/tps\/description\/updates or active/,
      );

      // The record keeps the value it had — nothing was applied.
      await deployments.closeManageProductModal();
      await deployments.openManageProductModal(listedProduct);
      await expect(deployments.manageProductCoresInput()).toHaveValue(
        DEPLOYMENT_PRODUCT_INPUT.cores,
      );

      console.log(
        `Invalid core count "${DEPLOYMENT_PRODUCT_INPUT.invalidCores}" was ` +
          `dropped and the empty update rejected with 400`,
      );
    });

    test(`${ProjectType.MANAGED_CLOUD_SUBSCRIPTION} — keeps the product when the delete dialog is dismissed`, async ({
      page,
    }) => {
      skipWhenUnconfigured(project);

      const { deployments } = await addProductToNewDeployment(page);
      const listedProduct = DEPLOYMENT_PRODUCT_INPUT.listedProductName;

      const writes = countProductWrites(page);
      await deployments.openDeleteProductModal(
        listedProduct,
        DEPLOYMENT_PRODUCT_INPUT.version,
      );

      await deployments.dismissDeleteProductModal();
      await expect(deployments.modal()).toBeHidden();

      // Still listed — Go Back is not a slow delete.
      await expect(
        deployments.deploymentProduct(listedProduct),
      ).not.toHaveCount(0);
      expect(writes(), "going back must not delete the product").toBe(0);
    });
  });
});
