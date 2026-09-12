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
import { ApiQueryKeys, BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  BeAttachment,
  BeAttachmentSearchPayload,
  BeAttachmentSearchResponse,
} from "@api/backend/types";
import type { DeploymentAttachment } from "@features/csm-projects/types/csmProjects";

const PAGE_LIMIT = BE_MAX_PAGE_LIMIT;

/** Maps a `BeAttachment` (reference-generic) to the deployment-scoped UI shape. */
export function deploymentAttachmentFromBe(att: BeAttachment): DeploymentAttachment {
  return {
    id: att.id,
    name: att.name,
    contentType: att.type,
    sizeBytes: att.sizeBytes,
    description: att.description ?? null,
    uploadedBy:
      att.createdBy?.name?.trim() || att.createdBy?.email?.trim() || "Unknown",
    uploadedOn: att.createdOn,
    downloadUrl: att.downloadUrl ?? null,
  };
}

/**
 * All attachments on a deployment, via `POST /attachments/search` scoped to
 * `referenceType: "deployment"`. The query is disabled until a deployment id
 * is provided.
 */
export function useSearchDeploymentAttachments(
  deploymentId: string | undefined,
): UseQueryResult<DeploymentAttachment[], Error> {
  const api = useBackendApi();

  return useQuery<DeploymentAttachment[], Error>({
    queryKey: [ApiQueryKeys.DEPLOYMENT_ATTACHMENTS, deploymentId ?? ""],
    queryFn: async (): Promise<DeploymentAttachment[]> => {
      const all: DeploymentAttachment[] = [];
      for (let offset = 0; ; offset += PAGE_LIMIT) {
        const payload: BeAttachmentSearchPayload = {
          referenceId: deploymentId as string,
          referenceType: "deployment",
          pagination: { offset, limit: PAGE_LIMIT },
        };
        const res = await api.post<
          BeAttachmentSearchPayload,
          BeAttachmentSearchResponse
        >("/attachments/search", payload);
        const page = res.attachments ?? [];
        all.push(...page.map(deploymentAttachmentFromBe));
        if (page.length < PAGE_LIMIT) break;
      }
      return all;
    },
    enabled: !!deploymentId,
    staleTime: 30_000,
  });
}
