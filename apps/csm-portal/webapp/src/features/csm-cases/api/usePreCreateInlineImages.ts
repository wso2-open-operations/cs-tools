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

import { useCallback, useMemo } from "react";
import { useBackendApi } from "@api/backend/client";
import type { ImageUploadHandler } from "@components/rich-text-editor/richTextEditor";
import {
  queueInlineImage,
  releaseInlineImages,
  uploadImagesBeforeCreate,
} from "@features/csm-cases/api/inlineImageUploads";

/** The message to show when {@link PreCreateInlineImages.uploadImages} rejected. */
export function preCreateImagesErrorMessage(err: unknown): string {
  return err instanceof Error && err.message ? err.message : "An image could not be uploaded.";
}

export interface PreCreateInlineImages {
  /**
   * Give this to the description's editor as `onImageUpload`. Undefined while
   * there is no project to store images under, which turns image insertion off
   * in that editor (it never embeds base64).
   */
  onImageUpload: ImageUploadHandler | undefined;
  /**
   * Call on submit, before the create request: uploads the description's local
   * images as draft images of the project and resolves to the HTML to send
   * (each `blob:` replaced by `/<attachment id>`). Rejects with a message fit to
   * show when an image cannot be stored; create nothing then and keep the form.
   * Calling it again after a failure only uploads the images still missing.
   */
  uploadImages: (html: string) => Promise<string>;
  /** Call once the record is created: frees the local previews. */
  release: (html: string) => void;
}

/**
 * The pre-create image flow of a form with a rich-text description, scoped to
 * the form's project: images stay local `blob:` previews while the person writes
 * and are uploaded only on submit, ahead of the single create request (see
 * {@link uploadImagesBeforeCreate}).
 */
export function usePreCreateInlineImages(projectId: string | undefined): PreCreateInlineImages {
  const api = useBackendApi();
  const onImageUpload = useMemo<ImageUploadHandler | undefined>(
    () => (projectId ? (file: File) => queueInlineImage(file) : undefined),
    [projectId],
  );
  const uploadImages = useCallback(
    (html: string) => uploadImagesBeforeCreate(api, projectId, html),
    [api, projectId],
  );
  return { onImageUpload, uploadImages, release: releaseInlineImages };
}
