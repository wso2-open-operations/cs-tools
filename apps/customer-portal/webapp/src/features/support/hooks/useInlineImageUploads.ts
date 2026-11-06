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


import { useCallback, useEffect, useRef, useState } from "react";
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import type { ImageUploadHandler } from "@components/rich-text-editor/localImageUpload";
import {
  uploadInlineImagesBeforeCreate,
  type InlineImageDraft,
  type InlineImageUploadResult,
} from "@features/support/utils/inlineImageUploads";

export interface InlineImageUploads {
  /** Pass to the Editor's `onImageUpload`: keeps the image as a local preview. */
  onImageUpload: ImageUploadHandler;
  /**
   * Uploads the description's local images (in parallel) and returns the HTML
   * with `/<attachment id>` sources, or the failure. Call it first when the form
   * is submitted, and create the record only when it succeeds.
   */
  uploadImages: (html: string) => Promise<InlineImageUploadResult>;
  /** True while `uploadImages` is running. */
  isUploadingImages: boolean;
  /** How many images are still being uploaded. */
  pendingImages: number;
  /** Frees the local previews, e.g. after the record was created. */
  release: () => void;
}

/**
 * The shared state for a create form whose rich-text description takes inline
 * images: the images stay local previews while the user writes, and are
 * uploaded to the project when the form is submitted, before the record is
 * created. Images that uploaded stay uploaded across a failed attempt.
 *
 * @param projectId - The project the record is created in.
 * @returns {InlineImageUploads} The handler for the editor and the upload step.
 */
export function useInlineImageUploads(
  projectId: string | undefined,
): InlineImageUploads {
  const authFetch = useAuthApiClient();
  const draftRef = useRef<InlineImageDraft>({
    queued: new Map(),
    ids: new Map(),
  });
  const previewUrlsRef = useRef<Set<string>>(new Set());
  const [pendingImages, setPendingImages] = useState(0);
  const [isUploadingImages, setIsUploadingImages] = useState(false);

  const release = useCallback(() => {
    previewUrlsRef.current.forEach((url) => URL.revokeObjectURL(url));
    previewUrlsRef.current.clear();
    draftRef.current.queued.clear();
    draftRef.current.ids.clear();
  }, []);

  useEffect(() => release, [release]);

  const onImageUpload = useCallback<ImageUploadHandler>(
    async (file, previewUrl) => {
      previewUrlsRef.current.add(previewUrl);
      draftRef.current.queued.set(previewUrl, file);
    },
    [],
  );

  const uploadImages = useCallback(
    async (html: string): Promise<InlineImageUploadResult> => {
      if (!projectId) return { ok: true, html };
      setIsUploadingImages(true);
      try {
        return await uploadInlineImagesBeforeCreate(
          authFetch,
          projectId,
          html,
          draftRef.current,
          setPendingImages,
        );
      } finally {
        setIsUploadingImages(false);
        setPendingImages(0);
      }
    },
    [authFetch, projectId],
  );

  return { onImageUpload, uploadImages, isUploadingImages, pendingImages, release };
}
