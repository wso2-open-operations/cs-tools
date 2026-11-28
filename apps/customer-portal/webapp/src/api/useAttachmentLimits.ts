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

import { useQuery } from "@tanstack/react-query";
import { useAsgardeo } from "@asgardeo/react";
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import { ApiQueryKeys } from "@constants/apiConstants";
import {
  DEFAULT_ENTITY_ATTACHMENT_MAX_MB,
  DEFAULT_INLINE_IMAGES_TOTAL_MAX_MB,
} from "@config/attachmentLimitsConfig";

const MIB = 1024 * 1024;

/** The attachment size limits, in bytes, as `GET /attachments/limits` returns them. */
export interface AttachmentLimits {
  /** Largest file of any attachment. */
  maxFileBytes: number;
  /** Most the inline images of one comment or description may add up to. */
  maxInlineImagesTotalBytes: number;
  /** Files above this size download straight from storage; 0 when direct transfers are off. */
  directTransferThresholdBytes: number;
}

/** Used until the limits arrive, and when the request fails. */
export const DEFAULT_ATTACHMENT_LIMITS: AttachmentLimits = {
  maxFileBytes: DEFAULT_ENTITY_ATTACHMENT_MAX_MB * MIB,
  maxInlineImagesTotalBytes: DEFAULT_INLINE_IMAGES_TOTAL_MAX_MB * MIB,
  directTransferThresholdBytes: 0,
};

/** The server's limits, kept for 5 minutes, or the defaults while loading (no direct download then) or on error. */
export default function useAttachmentLimits(): AttachmentLimits {
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();

  const { data } = useQuery<AttachmentLimits, Error>({
    queryKey: [ApiQueryKeys.ATTACHMENT_LIMITS],
    queryFn: async (): Promise<AttachmentLimits> => {
      const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
      if (!baseUrl) {
        throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
      }
      const response = await authFetch(`${baseUrl}/attachments/limits`, {
        method: "GET",
      });
      if (!response.ok) {
        throw new Error(`Error fetching attachment limits: ${response.statusText}`);
      }
      const body = (await response.json()) as Partial<AttachmentLimits>;
      return {
        maxFileBytes: positiveOr(body.maxFileBytes, DEFAULT_ATTACHMENT_LIMITS.maxFileBytes),
        maxInlineImagesTotalBytes: positiveOr(
          body.maxInlineImagesTotalBytes,
          DEFAULT_ATTACHMENT_LIMITS.maxInlineImagesTotalBytes,
        ),
        directTransferThresholdBytes: positiveOr(body.directTransferThresholdBytes, 0),
      };
    },
    enabled: isSignedIn && !isAuthLoading,
    staleTime: 5 * 60 * 1000,
  });

  return data ?? DEFAULT_ATTACHMENT_LIMITS;
}

function positiveOr(value: unknown, fallback: number): number {
  return typeof value === "number" && Number.isFinite(value) && value > 0 ? value : fallback;
}
