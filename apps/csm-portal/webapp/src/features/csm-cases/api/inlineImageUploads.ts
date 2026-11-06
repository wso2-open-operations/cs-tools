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

import { useMemo, useSyncExternalStore } from "react";
import type { BackendApi } from "@api/backend/client";
import {
  entityAttachmentErrorMessage,
  uploadEntityAttachment,
} from "@features/csm-cases/api/entityAttachments";

/**
 * Images pasted or picked into a case comment are stored as entity-service
 * `comment` attachments instead of being embedded in the comment as base64.
 *
 * While the comment is being written the editor shows the file through a local
 * `blob:` URL and nothing is sent. The upload starts when the comment is sent
 * ({@link uploadQueuedImages}), so an image that is removed or never posted
 * leaves nothing behind on the server. This module remembers, per blob URL, the
 * file and how its upload went. Once the comment's images are stored every blob
 * URL in its HTML is replaced by `/<attachment id>`, the reference entity-service
 * and the comment display already use, so a `blob:` URL never leaves the browser.
 *
 * It is kept at module level, not in the comment input, because the composer's
 * draft is held by its parent and outlives the input component.
 */

type UploadState =
  | { status: "queued"; file: File }
  | { status: "pending" }
  | { status: "done"; id: string }
  | { status: "failed"; message: string };

const uploads = new Map<string, UploadState>();
const listeners = new Set<() => void>();
let version = 0;

function publish(): void {
  version += 1;
  listeners.forEach((listener) => listener());
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

/** The `src` of every `blob:` image or element in `html`, in order. */
const BLOB_SRC = /(\bsrc\s*=\s*)(["'])(blob:[^"']*)\2/gi;

function blobSources(html: string): string[] {
  return Array.from(html.matchAll(BLOB_SRC), (m) => m[3]);
}

/**
 * Keeps `file` for the comment's images and returns the `blob:` URL to show in
 * the editor. Nothing is uploaded yet: {@link uploadQueuedImages} does that when
 * the comment is sent. The file must already have passed the editor's type and
 * size checks; entity-service checks them again.
 */
export function queueInlineImage(file: File): string {
  const previewUrl = URL.createObjectURL(file);
  uploads.set(previewUrl, { status: "queued", file });
  publish();
  return previewUrl;
}

/**
 * Uploads, one after another, the images in `html` that have not been uploaded
 * yet, as images of the case's comments. It does not throw: each image ends up
 * `done` or `failed`, which {@link inspectInlineImages} reports. An image that
 * already failed is not tried again (the person removes it and adds it again).
 */
export async function uploadQueuedImages(
  api: BackendApi,
  parentId: string,
  html: string,
): Promise<void> {
  for (const previewUrl of blobSources(html)) {
    const state = uploads.get(previewUrl);
    if (state?.status !== "queued") continue;
    uploads.set(previewUrl, { status: "pending" });
    publish();
    try {
      const confirmed = await uploadEntityAttachment(api, {
        type: "comment",
        parentId,
        file: state.file,
        name: state.file.name || "image",
      });
      // The draft was cleared while this ran: do not bring the entry back.
      if (!uploads.has(previewUrl)) continue;
      uploads.set(previewUrl, { status: "done", id: confirmed.id.toLowerCase() });
    } catch (err: unknown) {
      if (!uploads.has(previewUrl)) continue;
      uploads.set(previewUrl, {
        status: "failed",
        message: entityAttachmentErrorMessage(err, "An image could not be uploaded."),
      });
    }
    publish();
  }
}

export interface InlineImageStatus {
  /** Images being uploaded right now. */
  pending: number;
  /** Images added to the comment that have not been uploaded yet. */
  queued: number;
  /** The message of the first image whose upload failed. */
  failedMessage: string | null;
  /** `blob:` images this page did not upload (e.g. pasted in from another tab). */
  unknown: number;
}

/** How the uploads of the images in `html` stand. */
export function inspectInlineImages(html: string): InlineImageStatus {
  const status: InlineImageStatus = { pending: 0, queued: 0, failedMessage: null, unknown: 0 };
  for (const src of blobSources(html)) {
    const state = uploads.get(src);
    if (!state) status.unknown += 1;
    else if (state.status === "pending") status.pending += 1;
    else if (state.status === "queued") status.queued += 1;
    else if (state.status === "failed") status.failedMessage ??= state.message;
  }
  return status;
}

/**
 * Replaces each uploaded image's `blob:` URL by `/<attachment id>`. A `blob:`
 * URL whose upload is not done is left alone, so call this only once
 * {@link inspectInlineImages} reports nothing pending, failed or unknown.
 */
export function resolveInlineImageSources(html: string): string {
  return html.replace(BLOB_SRC, (whole, attr: string, quote: string, src: string) => {
    const state = uploads.get(src);
    return state?.status === "done" ? `${attr}${quote}/${state.id}${quote}` : whole;
  });
}

/**
 * Removes the images whose upload failed, so the comment can be sent without
 * them. Images still uploading, uploaded or unknown are left as they are. The
 * HTML is parsed, so a whole `<img>` element goes whatever its attributes hold
 * (an `alt` with a `>` in it, say); a comment with no failed image is returned
 * untouched, not re-serialized.
 */
export function dropFailedImages(html: string): string {
  const isFailed = (src: string | null) => !!src && uploads.get(src)?.status === "failed";
  if (!blobSources(html).some(isFailed)) return html;
  const body = new DOMParser().parseFromString(html, "text/html").body;
  body.querySelectorAll("img").forEach((image) => {
    if (isFailed(image.getAttribute("src"))) image.remove();
  });
  return body.innerHTML;
}

/** Whether `html` still has a `blob:` source in it. */
export function hasBlobSource(html: string): boolean {
  return blobSources(html).length > 0;
}

/**
 * Frees the preview URLs of the images in `html` once the comment is sent or its
 * draft is discarded. An upload still running for one of them is forgotten: its
 * result is ignored when it arrives.
 */
export function releaseInlineImages(html: string): void {
  for (const src of blobSources(html)) {
    URL.revokeObjectURL(src);
    uploads.delete(src);
  }
}

/** Re-renders with the state of the uploads of the images in `html`. */
export function useInlineImageStatus(html: string): InlineImageStatus {
  const current = useSyncExternalStore(subscribe, () => version);
  // eslint-disable-next-line react-hooks/exhaustive-deps -- `current` is the trigger
  return useMemo(() => inspectInlineImages(html), [html, current]);
}
