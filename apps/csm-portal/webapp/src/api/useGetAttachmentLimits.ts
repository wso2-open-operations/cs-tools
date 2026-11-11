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

import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type { BeAttachmentLimits } from "@api/backend/types";
import {
  BYTES_PER_MB,
  DEFAULT_ENTITY_ATTACHMENT_MAX_MB,
  DEFAULT_INLINE_IMAGE_MAX_MB,
} from "@config/attachmentLimitsConfig";

/**
 * The attachment sizes entity-service enforces (`GET /attachments/limits`).
 * The server sends them cacheable for five minutes, so they are kept as long.
 * `enabled` lets the app shell start the request once the caller is allowed in,
 * so the upload dialogs usually find the answer already cached.
 */
export function useGetAttachmentLimits(enabled = true): UseQueryResult<BeAttachmentLimits, Error> {
  const api = useBackendApi();
  return useQuery<BeAttachmentLimits, Error>({
    queryKey: [ApiQueryKeys.ATTACHMENT_LIMITS],
    queryFn: async (): Promise<BeAttachmentLimits> => {
      const limits = await api.get<BeAttachmentLimits>("/attachments/limits");
      if (!limits) throw new Error("No attachment limits returned.");
      return limits;
    },
    staleTime: 5 * 60_000,
    enabled,
  });
}

export interface AttachmentLimits {
  /** Largest single attachment, in bytes. */
  maxFileBytes: number;
  /** Most inline-image bytes in one comment or description (and the largest single image). */
  maxInlineImagesBytes: number;
}

/**
 * The limits to check against: the server's once loaded, the built-in defaults
 * while loading or if the request failed. The server enforces the real limit
 * either way; this only decides the early check and the text shown.
 */
export function useAttachmentLimits(): AttachmentLimits {
  const { data } = useGetAttachmentLimits();
  return {
    maxFileBytes: positive(data?.maxFileBytes) ?? DEFAULT_ENTITY_ATTACHMENT_MAX_MB * BYTES_PER_MB,
    maxInlineImagesBytes:
      positive(data?.maxInlineImagesTotalBytes) ?? DEFAULT_INLINE_IMAGE_MAX_MB * BYTES_PER_MB,
  };
}

function positive(value: number | undefined): number | undefined {
  return typeof value === "number" && Number.isFinite(value) && value > 0 ? value : undefined;
}
