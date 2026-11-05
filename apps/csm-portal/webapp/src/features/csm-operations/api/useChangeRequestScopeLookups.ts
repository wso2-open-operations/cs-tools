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
import { useMemo } from "react";
import { ApiQueryKeys } from "@constants/apiConstants";
import { useBackendApi } from "@api/backend/client";
import type {
  BeChangeRequestLinkOptionsPayload,
  BeChangeRequestLinkOptionsResponse,
  BeCustomerContact,
} from "@api/backend/types";

export interface ScopeOption {
  id: string;
  label: string;
  /** A deployment's type (its environment role, lower case); absent on other options. */
  type?: string;
}

/** A selectable deployment with what choosing it contributes to the form. */
export interface ScopeDeployment extends ScopeOption {
  /** Deployment products this deployment carries; `undefined` until the
   * lookup for the current selection has settled. */
  products: ScopeOption[] | undefined;
}

export interface ChangeRequestScopeLookups {
  deployments: ScopeDeployment[];
  /**
   * The project's registered contacts: the change request's read-only
   * "Customer Group" (empty until the lookup settles or when the project has
   * none). Display names only — nothing here is ever sent back.
   */
  customerContacts: BeCustomerContact[];
  /** True once the lookup for the current project has settled (contacts are known). */
  contactsReady: boolean;
  isLoading: boolean;
  isError: boolean;
  refetch: () => void;
}

/**
 * Everything the change-request form's project -> deployments /
 * deployment products cascade needs, from one backend lookup
 * (`POST /change-requests/link-options`): the project's deployments, the
 * project's registered contacts (the read-only Customer Group) and, for the
 * deployments chosen so far, the deployment products that follow from them. Disabled (empty) until a
 * project is chosen. While the lookup for a changed selection is in flight the
 * previous deployment list stays on screen, but products are reported as not
 * yet known rather than stale.
 */
export function useChangeRequestScopeLookups(
  projectId: string | undefined,
  deploymentIds: string[],
): ChangeRequestScopeLookups {
  const api = useBackendApi();
  // Order-insensitive key so re-ordering a selection does not refetch.
  const chosenKey = [...deploymentIds].sort().join(",");

  const query = useQuery<BeChangeRequestLinkOptionsResponse, Error>({
    queryKey: [ApiQueryKeys.CHANGE_REQUEST_LINK_OPTIONS, projectId ?? "", chosenKey],
    queryFn: () =>
      api.post<BeChangeRequestLinkOptionsPayload, BeChangeRequestLinkOptionsResponse>(
        "/change-requests/link-options",
        {
          projectId: projectId as string,
          ...(deploymentIds.length > 0 ? { deploymentIds } : {}),
        },
      ),
    enabled: !!projectId,
    // Keep the previous result on screen while a changed *deployment selection*
    // loads, but never carry one project's deployments over to another project.
    placeholderData: (previous, previousQuery) =>
      previousQuery?.queryKey[1] === (projectId ?? "") ? previous : undefined,
    staleTime: 60_000,
  });

  const data = query.data;
  // `keepPreviousData` keeps showing the last result while a new one loads;
  // that result's products belong to the previous selection.
  const productsSettled = !!data && !query.isPlaceholderData;

  const deployments = useMemo<ScopeDeployment[]>(() => {
    if (!data) return [];
    const chosen = new Set(deploymentIds);
    return (data.deployments ?? []).map((d) => ({
      id: d.id,
      label: d.name || d.id,
      type: d.type || undefined,
      products:
        productsSettled && chosen.has(d.id)
          ? (data.deploymentProducts ?? [])
              .filter((p) => !p.deployment || p.deployment.id === d.id)
              .map((p) => ({ id: p.id, label: p.name || p.id }))
          : undefined,
    }));
  }, [data, deploymentIds, productsSettled]);

  return {
    deployments: projectId ? deployments : [],
    customerContacts: projectId && data ? (data.customerContacts ?? []) : [],
    contactsReady: !!projectId && !!data,
    isLoading: !!projectId && query.isLoading,
    isError: !!projectId && query.isError,
    refetch: () => void query.refetch(),
  };
}
