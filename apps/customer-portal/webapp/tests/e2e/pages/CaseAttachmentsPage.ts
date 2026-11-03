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

import path from "node:path";
import {
  type Download,
  type Locator,
  type Page,
  type Response,
  expect,
} from "../fixtures/test";
import { CASE_ATTACHMENTS, CASE_DETAIL } from "../utils/selectors";

/** How long to allow for the tab and its uploads to resolve. */
const LOAD_TIMEOUT_MS = 60_000;

/**
 * Page object for a case's Attachments tab.
 *
 * Covers both directions: uploading through the shared UploadAttachmentModal, and
 * deleting through the per-row control and its confirmation dialog. The delete
 * path exists so the spec can clean up rather than leave records behind.
 */
export class CaseAttachmentsPage {
  constructor(private readonly page: Page) {}

  private main(): Locator {
    return this.page.getByTestId(CASE_DETAIL.mainTestId);
  }

  /** Switches to the Attachments tab. Its label carries a live count, so the
   * name is matched on the prefix. Safe to call again after an upload, which
   * navigates and resets the page to its default Activity tab. */
  async openTab(): Promise<void> {
    const tab = this.page.getByRole("tab", { name: CASE_ATTACHMENTS.tab });
    await expect(tab).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
    await tab.click();
    await expect(this.uploadButton()).toBeVisible({ timeout: LOAD_TIMEOUT_MS });

    // The upload button renders while the list is still loading, so waiting on it
    // alone is not enough: a caller counting rows straight afterwards reads zero
    // and concludes the file is absent. Wait for the list to actually resolve —
    // either the empty state or a first row.
    const settled = this.main()
      .getByText(CASE_ATTACHMENTS.emptyMessage)
      .or(this.main().getByRole("button", { name: /^Delete / }));
    await expect(settled.first()).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
  }

  uploadButton(): Locator {
    return this.main().getByRole("button", {
      name: CASE_ATTACHMENTS.uploadButton,
      exact: true,
    });
  }

  modal(): Locator {
    return this.page.getByRole("dialog");
  }

  /**
   * Attachment rows matching a file name.
   *
   * Returns every match rather than narrowing to one: the same file can be
   * attached repeatedly, so callers assert the count is non-zero instead of
   * asserting visibility on what may legitimately be several rows.
   */
  attachment(fileName: string): Locator {
    return this.main().getByText(fileName, { exact: true });
  }

  /**
   * One locator per attachment, using the per-row delete control.
   *
   * The delete button is the reliable per-row marker: the row's *text* is
   * rendered twice for responsive layout, so counting names or sizes would
   * double every attachment, whereas there is exactly one delete control each.
   */
  rows(): Locator {
    return this.main().getByRole("button", { name: /^Delete / });
  }

  /** File names of the listed attachments, read from the delete controls. */
  async listedFileNames(): Promise<string[]> {
    const labels = await this.rows().evaluateAll((els) =>
      els.map((el) => el.getAttribute("aria-label") ?? ""),
    );
    return labels.map((label) => label.replace(/^Delete /, ""));
  }

  /** The Attachments tab, whose label carries the live count. */
  tab(): Locator {
    return this.page.getByRole("tab", { name: CASE_ATTACHMENTS.tab });
  }

  /** The count shown in the tab label, or null when it is not rendered. */
  async tabCount(): Promise<number | null> {
    const label = await this.tab().innerText();
    const match = CASE_ATTACHMENTS.tabCountPattern.exec(label.trim());
    return match ? Number(match[1]) : null;
  }

  emptyMessage(): Locator {
    return this.main().getByText(CASE_ATTACHMENTS.emptyMessage);
  }

  /**
   * The list row for an attachment — the container holding its name, size,
   * uploader and upload date.
   *
   * Reached by stepping up two levels from the name element: the row has no id or
   * test id, and the name sits in its own node inside it. Verified live, where
   * the container reads
   * "<name> | <size> | • | Uploaded by <email> | • | <date>".
   *
   * Narrowed with `.first()` because the list renders each row twice — a
   * responsive layout keeps a desktop and a compact variant in the DOM — so the
   * locator would otherwise be ambiguous.
   *
   * @param fileName - Attachment file name.
   * @returns Locator for the row container.
   */
  attachmentRow(fileName: string): Locator {
    return this.attachment(fileName).first().locator("xpath=../..");
  }

  /** The per-row delete control, whose accessible name embeds the file name. */
  deleteButton(fileName: string): Locator {
    return this.main().getByRole("button", {
      name: `Delete ${fileName}`,
      exact: true,
    });
  }

  /**
   * The preview toggle on an image row.
   *
   * Its label is generic ("Expand image"), not per file, so it cannot be tied to
   * a name — callers assert there is exactly one before using it.
   */
  expandImageButton(): Locator {
    return this.main().getByRole("button", {
      name: CASE_ATTACHMENTS.expandImageButton,
      exact: true,
    });
  }

  collapseImageButton(): Locator {
    return this.main().getByRole("button", {
      name: CASE_ATTACHMENTS.collapseImageButton,
      exact: true,
    });
  }

  /** The expanded preview image, which takes its alt text from the file name. */
  previewImage(fileName: string): Locator {
    return this.main().getByRole("img", { name: fileName, exact: true });
  }

  /**
   * Expands the image preview and waits for the picture to actually decode.
   *
   * The wait is generous because the row shows a skeleton for a guaranteed
   * minimum of 4s (PREVIEW_SKELETON_MIN_DISPLAY_MS) before the image appears at
   * all, on top of fetching it.
   *
   * @param fileName - Name of the image attachment.
   */
  async expandImage(fileName: string): Promise<void> {
    // The fixture case accumulates copies of the image from repeated upload
    // runs (6 on 2026-10-27), so act on the first rather than requiring one.
    await expect(
      this.expandImageButton().first(),
      "expected an image attachment to expand",
    ).toBeVisible({ timeout: LOAD_TIMEOUT_MS });
    await this.expandImageButton().first().click();

    const image = this.previewImage(fileName);
    await expect(image).toBeVisible({ timeout: LOAD_TIMEOUT_MS });

    // Visibility only proves the <img> is in the layout. `naturalWidth` is what
    // shows the browser actually decoded the data it was given — a broken or
    // empty source renders an element but leaves this at 0.
    await expect
      .poll(
        () => image.evaluate((el) => (el as HTMLImageElement).naturalWidth),
        {
          message: `${fileName} preview should decode to a real image`,
          timeout: LOAD_TIMEOUT_MS,
        },
      )
      .toBeGreaterThan(0);
  }

  /**
   * Collapses an open image preview and waits for it to go away.
   *
   * @param fileName - Name of the image attachment.
   */
  async collapseImage(fileName: string): Promise<void> {
    await expect(
      this.collapseImageButton(),
      "expected an expanded image preview to collapse",
    ).toHaveCount(1);
    await this.collapseImageButton().click();

    // The preview is unmounted rather than hidden, so the image should leave the
    // DOM entirely and the toggle should offer to expand again.
    await expect(this.previewImage(fileName)).toHaveCount(0);
    await expect(this.expandImageButton().first()).toBeVisible();
  }

  /** The per-row download control, whose accessible name embeds the file name. */
  downloadButton(fileName: string): Locator {
    return this.main().getByRole("button", {
      name: `Download ${fileName}`,
      exact: true,
    });
  }

  /**
   * Downloads an attachment and returns the captured download.
   *
   * The app fetches the content, wraps it in an object URL and clicks a
   * generated anchor carrying a `download` attribute (see
   * useGetAttachmentContent), which the browser surfaces as a download event —
   * so the listener has to be armed before the click.
   *
   * @param fileName - Name of the attachment to download.
   * @returns The captured download, for the caller to inspect.
   */
  async download(fileName: string): Promise<Download> {
    const [download] = await Promise.all([
      this.page.waitForEvent("download", { timeout: LOAD_TIMEOUT_MS }),
      // First of possibly several same-named copies — see expandImage.
      this.downloadButton(fileName).first().click(),
    ]);
    return download;
  }

  /**
   * Uploads a file through the modal and waits for the POST to land.
   *
   * Goes through the dropzone's "Choose file" button rather than driving the
   * hidden input directly, so the control a user actually clicks is exercised.
   * That button opens a real file picker, which Playwright intercepts as a
   * `filechooser` event — the listener therefore has to be armed before the
   * click, or the dialog is missed.
   *
   * The optional "Attachment name" field is left empty, which makes the list show
   * the file's own name.
   *
   * @param relativePath - File path, relative to the tests/e2e directory.
   * @returns The upload response, for the caller to assert on.
   */
  async upload(relativePath: string): Promise<Response> {
    await this.uploadButton().click();

    const modal = this.modal();
    await expect(modal).toBeVisible();
    // Scoped to the dialog: the "Upload Attachment" button that opened it is
    // still in the DOM behind it, so a page-wide match is ambiguous.
    await expect(
      modal.getByText(CASE_ATTACHMENTS.uploadModal.title),
    ).toBeVisible();

    const absolutePath = path.join(process.cwd(), "tests", "e2e", relativePath);
    const [chooser] = await Promise.all([
      this.page.waitForEvent("filechooser", { timeout: LOAD_TIMEOUT_MS }),
      modal
        .getByRole("button", {
          name: CASE_ATTACHMENTS.uploadModal.chooseFileButton,
          exact: true,
        })
        .click(),
    ]);
    await chooser.setFiles(absolutePath);

    const confirm = modal.getByRole("button", {
      name: CASE_ATTACHMENTS.uploadModal.confirmButton,
      exact: true,
    });
    await expect(confirm).toBeEnabled();

    const [response] = await Promise.all([
      this.page.waitForResponse(
        (r) =>
          /\/cases\/[^/]+\/attachments$/.test(new URL(r.url()).pathname) &&
          r.request().method() === "POST",
        { timeout: LOAD_TIMEOUT_MS },
      ),
      confirm.click(),
    ]);

    // A successful upload navigates, which drops the page back to its default
    // Activity tab — so the list is not on screen unless the tab is reopened.
    await this.openTab();
    return response;
  }

  /**
   * Removes every attachment with this file name.
   *
   * Deletes in a loop rather than once: the same file can be attached several
   * times — earlier failed runs left three copies behind — and a single delete
   * would leave the rest, while clicking a locator that matches several rows is
   * a strict-mode violation.
   *
   * Used for cleanup, so it tolerates there being nothing to remove and caps the
   * loop: cleanup must not mask the original failure with one of its own, nor
   * spin if a delete silently fails.
   *
   * @param fileName - Name of the attachments to remove.
   */
  async deleteAll(fileName: string): Promise<void> {
    const rows = this.deleteButton(fileName);
    for (let remaining = await rows.count(); remaining > 0; remaining -= 1) {
      await rows.first().click();

      const modal = this.modal();
      await expect(modal).toBeVisible();
      await expect(modal).toContainText(CASE_ATTACHMENTS.deleteModal.title);
      await modal
        .getByRole("button", {
          name: CASE_ATTACHMENTS.deleteModal.confirmButton,
          exact: true,
        })
        .click();
      await expect(modal).toBeHidden();

      // The list refetches, so wait for the row count to actually drop before
      // going round again.
      await expect(rows).toHaveCount(remaining - 1, {
        timeout: LOAD_TIMEOUT_MS,
      });
    }
  }
}
