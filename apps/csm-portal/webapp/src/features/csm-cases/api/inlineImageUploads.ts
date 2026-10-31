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
 * `blob:` URL and the upload runs in the background. This module remembers, per
 * blob URL, how that upload went. When the comment is sent every blob URL in its
 * HTML is replaced by `/<attachment id>`, the reference entity-service and the
 * comment display already use, so a `blob:` URL never leaves the browser.
 *
 * It is kept at module level, not in the comment input, because the composer's
 * draft is held by its parent and outlives the input component.
 */

type UploadState =
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
 * Starts uploading `file` as an image of the case's comments and returns the
 * `blob:` URL to show in the editor meanwhile. The file must already have passed
 * the editor's type and size checks; entity-service checks them again.
 */
export function startInlineImageUpload(
  api: BackendApi,
  parentId: string,
  file: File,
): string {
  const previewUrl = URL.createObjectURL(file);
  uploads.set(previewUrl, { status: "pending" });
  publish();
  uploadEntityAttachment(api, {
    type: "comment",
    parentId,
    file,
    name: file.name || "image",
  }).then(
    (confirmed) => {
      // The draft was cleared while this ran: do not bring the entry back.
      if (!uploads.has(previewUrl)) return;
      uploads.set(previewUrl, { status: "done", id: confirmed.id.toLowerCase() });
      publish();
    },
    (err: unknown) => {
      if (!uploads.has(previewUrl)) return;
      uploads.set(previewUrl, {
        status: "failed",
        message: entityAttachmentErrorMessage(err, "An image could not be uploaded."),
      });
      publish();
    },
  );
  return previewUrl;
}

export interface InlineImageStatus {
  /** Images still uploading. */
  pending: number;
  /** The message of the first image whose upload failed. */
  failedMessage: string | null;
  /** `blob:` images this page did not upload (e.g. pasted in from another tab). */
  unknown: number;
}

/** How the uploads of the images in `html` stand. */
export function inspectInlineImages(html: string): InlineImageStatus {
  const status: InlineImageStatus = { pending: 0, failedMessage: null, unknown: 0 };
  for (const src of blobSources(html)) {
    const state = uploads.get(src);
    if (!state) status.unknown += 1;
    else if (state.status === "pending") status.pending += 1;
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
