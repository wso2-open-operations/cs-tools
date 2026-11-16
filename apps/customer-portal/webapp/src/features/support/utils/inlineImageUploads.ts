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


import {
  confirmStoredUpload,
  uploadStoredAttachment,
  UploadConfirmPendingError,
  type AuthFetch,
} from "@api/storedAttachments";
import {
  blobImageSources,
  hasBlobImageSource,
  rewriteBlobImageSources,
} from "@features/support/utils/inlineCommentImages";

/** The error shown when a blob: image is left in the HTML that has no file to upload. */
export const INLINE_IMAGE_MISSING_MESSAGE =
  "An image in the description could not be uploaded. Remove it and try again.";

/** What the images belong to: the project of a record about to be created, or an existing record. */
export type InlineImageScope = { projectId: string } | { parentId: string };

/** The images of a draft that are waiting to be uploaded, and the ones that already were. */
export interface InlineImageDraft {
  /** The file behind each local preview URL that has not been uploaded yet. */
  queued: Map<string, File>;
  /** Attachment id of each preview URL that finished uploading (kept across retries). */
  ids: Map<string, string>;
}

export type InlineImageUploadResult =
  | { ok: true; html: string }
  | { ok: false; message: string };

/**
 * Uploads every local (blob:) image of a rich-text description before the record
 * it belongs to exists, then points each `<img>` at its attachment:
 * `src="/<attachment id>"`, which entity-service links when the record is created.
 *
 * The uploads run in parallel and are scoped to the project (`projectId`). An
 * image that already uploaded (its id is in `draft.ids`) is not uploaded again, so
 * a retry after a failure only sends the ones that did not finish. When any
 * upload fails the others still complete and are kept, and the result is a
 * failure with the first error's message: the caller must not create the record.
 * HTML with no local image is returned as it is, with no request.
 *
 * @param authFetch - The authenticated fetch of the portal.
 * @param scope - The project the record is being created in, or (for a comment on an existing record) that record's id.
 * @param html - The description HTML from the editor.
 * @param draft - The queued files and the ids already obtained.
 * @param onImageUploaded - Called with how many images are left: first all of them, then after each one finishes.
 * @returns {Promise<InlineImageUploadResult>} The HTML with `/<id>` sources, or the failure.
 */
export async function uploadInlineImagesBeforeCreate(
  authFetch: AuthFetch,
  scope: InlineImageScope,
  html: string,
  draft: InlineImageDraft,
  onImageUploaded?: (remaining: number) => void,
): Promise<InlineImageUploadResult> {
  // The same preview can appear more than once (a copied image): upload it once.
  const toUpload = Array.from(new Set(blobImageSources(html))).filter(
    (url) => !draft.ids.has(url) && draft.queued.has(url),
  );
  let remaining = toUpload.length;
  if (remaining > 0) onImageUploaded?.(remaining);

  const uploadOne = async (previewUrl: string): Promise<void> => {
    const file = draft.queued.get(previewUrl) as File;
    const input = {
      type: "comment" as const,
      ...scope,
      file,
      name: file.name || "image",
    };
    let id: string;
    try {
      id = (await uploadStoredAttachment(authFetch, input)).id;
    } catch (error) {
      // The file is already in place: finish the upload instead of repeating it.
      if (!(error instanceof UploadConfirmPendingError)) throw error;
      id = (
        await confirmStoredUpload(authFetch, {
          type: "comment",
          attachmentId: error.attachmentId,
          chunks: error.chunks,
        })
      ).id;
    }
    draft.ids.set(previewUrl, id);
    draft.queued.delete(previewUrl);
    remaining -= 1;
    onImageUploaded?.(remaining);
  };

  const outcomes = await Promise.allSettled(toUpload.map(uploadOne));
  const failure = outcomes.find(
    (o): o is PromiseRejectedResult => o.status === "rejected",
  );
  if (failure) {
    const reason: unknown = failure.reason;
    return {
      ok: false,
      message:
        reason instanceof Error && reason.message
          ? reason.message
          : "Failed to upload an image.",
    };
  }

  const rewritten = rewriteBlobImageSources(html, draft.ids);
  if (hasBlobImageSource(rewritten)) {
    return { ok: false, message: INLINE_IMAGE_MISSING_MESSAGE };
  }
  return { ok: true, html: rewritten };
}
