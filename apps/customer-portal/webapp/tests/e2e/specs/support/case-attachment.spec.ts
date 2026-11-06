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
// The attachment lifecycle on a case: upload, list, expand, collapse, download
// and delete — run against every project type.
//
// Two cases per project, and the split matters:
//
// - The **kept** case holds one upload permanently. The list, expand, collapse
//   and download tests all need a file to be there, and the expand test needs
//   specifically an image. Its upload is guarded on the file name, so a run only
//   uploads when the file is missing — otherwise every run would stack another
//   identical copy with nothing removing them.
// - The **delete** case is used for the upload-then-remove round trip, so it ends
//   exactly where it started. Kept separate so the delete never takes away the
//   fixture the other tests rely on.
//
// Attachments are the only records in this suite that can be deleted, which is
// what makes that round trip possible at all.
//

import fs from "node:fs";
import path from "node:path";
import { test, expect, withSession, type Page } from "../../fixtures/test";
import { CaseAttachmentsPage } from "../../pages/CaseAttachmentsPage";
import { CaseDetailPage } from "../../pages/CaseDetailPage";
import {
  ATTACHMENT_DATE_PATTERN,
  ATTACHMENT_FILES,
  ATTACHMENT_TARGETS,
  PROJECTS,
  FOREIGN_ATTACHMENT_INPUT,
} from "../../config/testData";
import { CASE_ATTACHMENTS } from "../../utils/selectors";
import { expectSuccess } from "../../utils/caseFlows";

withSession(test);

/** Today's date as the attachment list renders it, e.g. "Aug 13, 2026". */
function todayLabel(): string {
  return new Date().toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

test.describe("Case Attachment", () => {
  // Each test is a cold case load plus a tab switch, and the image preview holds
  // a 4s skeleton before rendering; the 30s default is not enough.
  test.describe.configure({ timeout: 180_000 });

  const kept = ATTACHMENT_FILES.kept;
  const transient = ATTACHMENT_FILES.transient;

  for (const target of ATTACHMENT_TARGETS) {
    const project = PROJECTS[target.projectType];

    test.describe(target.projectType, () => {
      /** Opens the kept case's Attachments tab. */
      async function openKeptCase(page: Page): Promise<CaseAttachmentsPage> {
        const caseDetail = new CaseDetailPage(page);
        await caseDetail.open(project.id, target.caseId);
        const attachments = new CaseAttachmentsPage(page);
        await attachments.openTab();
        return attachments;
      }

      /**
       * Makes sure the kept case has its attachment, uploading only when it is
       * missing.
       *
       * Idempotent, and called by every test that needs the file rather than
       * only by the upload test: the list, expand, collapse and download tests
       * all depend on it being there, and leaning on the upload test to have run
       * first makes them silently order-dependent. That holds in a whole-file
       * run (`workers: 1`, `fullyParallel: false`) but not when a single test is
       * run on its own, which is exactly when the file might not yet exist.
       *
       * The guard is what keeps this safe to call repeatedly — an unguarded
       * upload would stack another identical copy every time, with nothing
       * removing them.
       *
       * @param attachments - The kept case's open Attachments tab.
       * @returns Whether the file was already attached.
       */
      async function ensureKeptAttachment(
        attachments: CaseAttachmentsPage,
      ): Promise<boolean> {
        const alreadyAttached =
          (await attachments.attachment(kept.name).count()) > 0;

        if (alreadyAttached) {
          console.log(`${target.projectType}: ${kept.name} already attached`);
        } else {
          console.log(`${target.projectType}: ${kept.name} missing, uploading`);
          const response = await attachments.upload(kept.path);
          await expectSuccess(response, "upload");
        }
        return alreadyAttached;
      }

      test("uploads an attachment to the case", async ({ page }) => {
        test.skip(
          !project.id || !target.caseId,
          `${target.projectType} needs a project id and a case id. ` +
            `Fill them in tests/e2e/config/testData.ts.`,
        );

        const attachments = await openKeptCase(page);
        const alreadyAttached = await ensureKeptAttachment(attachments);

        // Listed under its own name. Asserted as "at least one row" rather than by
        // visibility: the list renders each row twice for responsive layout, and a
        // multi-match locator would be a strict-mode violation.
        await expect(attachments.attachment(kept.name)).not.toHaveCount(0);

        const row = attachments.attachmentRow(kept.name);
        await expect(row).toContainText(kept.name);
        await expect(row).toContainText(kept.size);

        // Today's date only holds on the run that actually uploaded; later runs
        // keep the original date, so those assert a date is rendered at all.
        await expect(row).toContainText(
          alreadyAttached ? ATTACHMENT_DATE_PATTERN : todayLabel(),
        );
      });

      test("lists the case attachments", async ({ page }) => {
        test.skip(
          !project.id || !target.caseId,
          `${target.projectType} needs a project id and a case id.`,
        );

        const attachments = await openKeptCase(page);
        await ensureKeptAttachment(attachments);

        await expect(attachments.emptyMessage()).toHaveCount(0);
        await expect(attachments.rows()).not.toHaveCount(0);

        // The tab's own count must agree with the number of rows — a mismatch
        // means one of the two is stale.
        const names = await attachments.listedFileNames();
        expect(await attachments.tabCount()).toBe(names.length);

        for (const name of names) {
          const row = attachments.attachmentRow(name);
          await expect
            .soft(row, `${name}: size`)
            .toContainText(CASE_ATTACHMENTS.row.sizePattern);
          await expect
            .soft(row, `${name}: uploader`)
            .toContainText(CASE_ATTACHMENTS.row.uploadedByPrefix);
          await expect
            .soft(row, `${name}: upload date`)
            .toContainText(CASE_ATTACHMENTS.row.datePattern);
        }
      });

      test("expands an image attachment and loads the preview", async ({
        page,
      }) => {
        test.skip(
          !project.id || !target.caseId,
          `${target.projectType} needs a project id and a case id.`,
        );

        const attachments = await openKeptCase(page);
        await ensureKeptAttachment(attachments);

        // The preview toggle only renders for image attachments, so this depends
        // on the kept upload being the PNG.
        await expect(attachments.attachment(kept.name)).not.toHaveCount(0);
        await attachments.expandImage(kept.name);

        // The toggle flips once expanded — the row's own confirmation that the
        // preview is open rather than merely present.
        await expect(attachments.collapseImageButton()).toBeVisible();
      });

      test("collapses an expanded image attachment", async ({ page }) => {
        test.skip(
          !project.id || !target.caseId,
          `${target.projectType} needs a project id and a case id.`,
        );

        const attachments = await openKeptCase(page);
        await ensureKeptAttachment(attachments);

        // Expand first: collapsing is only meaningful from an open preview, and
        // starting from a known state keeps this independent of the sibling test.
        await attachments.expandImage(kept.name);
        await attachments.collapseImage(kept.name);
      });

      test("downloads an attachment", async ({ page }) => {
        test.skip(
          !project.id || !target.caseId,
          `${target.projectType} needs a project id and a case id.`,
        );

        const attachments = await openKeptCase(page);
        await ensureKeptAttachment(attachments);

        const download = await attachments.download(kept.name);

        // What a user would end up with on disk, from the anchor's `download`
        // attribute.
        expect(download.suggestedFilename()).toBe(kept.name);

        // Comparing bytes against the fixture proves the content actually came
        // back, not merely that a download was triggered. Read from the fixture
        // rather than a hardcoded number, so replacing the file keeps both sides
        // in step.
        const downloadedPath = await download.path();
        expect(downloadedPath, "download produced no file").toBeTruthy();

        const fixturePath = path.join(process.cwd(), "tests", "e2e", kept.path);
        expect(fs.statSync(downloadedPath as string).size).toBe(
          fs.statSync(fixturePath).size,
        );
      });

      test("deletes an attachment", async ({ page }) => {
        test.skip(
          !project.id || !target.deleteCaseId,
          `${target.projectType} needs a project id and a delete case id.`,
        );

        const caseDetail = new CaseDetailPage(page);
        await caseDetail.open(project.id, target.deleteCaseId);

        const attachments = new CaseAttachmentsPage(page);
        await attachments.openTab();

        // Clear anything a previously failed run left behind, so the upload below
        // starts from a known-empty state rather than adding a second copy.
        await attachments.deleteAll(transient.name);

        // The removal runs from `finally` so a failed assertion above still
        // leaves the case as it was found. The pre-clean above would sweep it up
        // on the next run either way, but attachments are the only record in
        // this suite that can be deleted, so there is no reason to leave one
        // sitting on the tenant in the meantime.
        try {
          const response = await attachments.upload(transient.path);
          await expectSuccess(response, "upload");

          const row = attachments.attachmentRow(transient.name);
          await expect(row).toContainText(transient.name);
          await expect(row).toContainText(transient.size);
          await expect(row).toContainText(todayLabel());
        } finally {
          // Errors are swallowed so cleanup cannot replace the failure that got
          // us here — a cleanup error would otherwise become the reported one
          // and hide the real cause. On the happy path a delete that silently
          // failed is still caught, by the absence assertions below.
          //
          // The row's control opens a "Confirm Action" dialog, which deleteAll
          // asserts on before confirming.
          await attachments.deleteAll(transient.name).catch(() => undefined);
        }

        // Gone from the list, so the case ends as it started.
        await expect(attachments.attachment(transient.name)).toHaveCount(0);
        await expect(attachments.deleteButton(transient.name)).toHaveCount(0);
      });
    });
  }

  // ✅ READ-ONLY. Only the uploader may delete an attachment; this one was
  // uploaded by someone else.
  test("does not offer delete on another user's attachment", async ({ page }) => {
    const { projectType, caseId, fileName } = FOREIGN_ATTACHMENT_INPUT;
    const project = PROJECTS[projectType];
    test.skip(!project.id, `${projectType} needs a project id.`);

    await new CaseDetailPage(page).open(project.id, caseId);
    const attachments = new CaseAttachmentsPage(page);
    await attachments.openTab();
    await expect(
      attachments.attachment(fileName).first(),
      `${fileName} should be listed on the case`,
    ).toBeVisible({ timeout: 60_000 });

    // Not usable: either not rendered at all, or rendered disabled (the repo
    // disables it with "Only the uploader can delete this attachment").
    const remove = attachments.deleteButton(fileName);
    const present = (await remove.count()) > 0;
    if (present) {
      await expect(remove.first(), `deleting ${fileName} should not be allowed`).toBeDisabled();
    } else {
      await expect(remove).toHaveCount(0);
    }
    console.log(
      `Attachments: delete on ${fileName} is ${present ? "shown but disabled" : "hidden"}`,
    );
  });
});
