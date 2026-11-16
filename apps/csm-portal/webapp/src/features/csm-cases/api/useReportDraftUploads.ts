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
import { useBackendApi } from "@api/backend/client";
import type { PendingAttachment } from "@components/attachments/pendingAttachment";
import {
  uploadReportDrafts,
  type ReportDraftProgress,
  type ReportDraftUploadResult,
  type StoredReportDraft,
} from "@features/csm-cases/api/reportDraftUploads";

/**
 * Uploads a security report's files before its case is created (see
 * {@link uploadReportDrafts}) and exposes the file being sent, so the form can show
 * its progress with a Cancel. Files that finished are remembered for the life of the
 * form, so pressing Submit again after a failed create sends none of them twice.
 *
 * @param projectId - The project the case is being created in.
 * @returns The `upload` call, `cancel`, the `progress` of the file in flight, and `isUploading`.
 */
export function useReportDraftUploads(projectId: string | undefined): {
  upload: (files: readonly PendingAttachment[]) => Promise<ReportDraftUploadResult>;
  cancel: () => void;
  progress: ReportDraftProgress | null;
  isUploading: boolean;
} {
  const api = useBackendApi();
  const stored = useRef(new Map<File, StoredReportDraft>());
  const controller = useRef<AbortController | null>(null);
  const [progress, setProgress] = useState<ReportDraftProgress | null>(null);
  const [isUploading, setIsUploading] = useState(false);
  const mounted = useRef(true);

  // Leaving the form aborts the file in flight and stops a later upload from starting,
  // so the caller never goes on to create the case from a form that is gone.
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
      controller.current?.abort();
    };
  }, []);

  const upload = useCallback(
    async (files: readonly PendingAttachment[]): Promise<ReportDraftUploadResult> => {
      if (!projectId) {
        return { ok: false, cancelled: false, message: "Select a project first." };
      }
      if (!mounted.current) return { ok: false, cancelled: true, message: "The upload was cancelled." };
      const own = new AbortController();
      controller.current = own;
      setIsUploading(true);
      try {
        return await uploadReportDrafts(api, projectId, files, stored.current, {
          signal: own.signal,
          onProgress: setProgress,
        });
      } finally {
        if (controller.current === own) controller.current = null;
        if (mounted.current) setIsUploading(false);
      }
    },
    [api, projectId],
  );

  const cancel = useCallback(() => controller.current?.abort(), []);

  return { upload, cancel, progress, isUploading };
}
