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
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import { isMockMode, useBackendApi } from "@api/backend/client";
import { severityFromPriority, uiStateFromBe } from "@api/backend/mappers";
import type {
  BeAccount,
  BeCaseView,
  BeProject,
} from "@api/backend/types";
import { getMockCsmCaseDetailById } from "@features/csm-cases/api/mocks/casesMocks";
import type { CsmCaseDetail } from "@features/csm-cases/types/csmCases";

const MOCK_LATENCY_MS = 150;

/**
 * Build a CsmCaseDetail from the rich `BeCaseView`. The view embeds the
 * project / deployment / deployed-product / reporter as objects, so their
 * display names come straight off the response. Only the account (customer)
 * name is not embedded and is passed in after a best-effort lookup. Side
 * widgets the backend doesn't return yet (SLA clocks, watchers, tags, time
 * logs, attachments, linked items) default to empty / placeholder values.
 */
function detailFromBeCase(
  c: BeCaseView,
  account?: BeAccount,
): CsmCaseDetail {
  const customer = account?.name ?? "—";
  const reporter = c.createdBy?.displayName ?? c.createdBy?.email;
  const product = c.deployedProduct?.displayName ?? "—";
  return {
    id: c.id,
    caseNumber: c.number ?? c.id,
    wso2CaseId: c.wso2Id ?? c.id,
    subject: c.subject ?? "(no subject)",
    customer,
    accountId: account?.id ?? "",
    projectId: c.project?.id ?? "",
    projectName: c.project?.name ?? "—",
    product,
    severity: severityFromPriority(c.priority),
    state: uiStateFromBe(c.state),
    nextStates: (c.nextStates ?? []).map(uiStateFromBe),
    // The backend has no assignee field yet; `createdBy` is the reporter, not
    // the assigned engineer, so don't surface it here as the assignee.
    assignee: "Unassigned",
    assigneeIsMe: false,
    slaClockType: "ack",
    minutesToBreach: 0,
    createdAt: c.createdAt ?? "",
    updatedAt: c.updatedAt ?? c.createdAt ?? "",
    description: c.description ?? "",
    assignmentGroup: "grp.cre_team",
    createdBy: reporter,
    customerContext: {
      accountName: customer,
      tier: "subscription",
      region: account?.region ?? "—",
      // The reporter (createdBy) is the customer-side person who opened it.
      primaryContact: reporter ?? "—",
      primaryContactEmail: c.createdBy?.email ?? "—",
      accountManager: account?.ownerId ?? "—",
      openCases: 0,
    },
    productContext: {
      product,
      version: "—",
      deployment: c.deployment?.name ?? "—",
      environment: "prod",
    },
    slaClocks: [],
    watchers: [],
    linkedItems: [],
    tags: [],
    timeLogs: [],
    audit: [],
    attachments: [],
    isWatching: false,
  };
}

/**
 * Look up a single CSM case by id. Returns `null` (not an error) when the
 * id is unknown so the page can render a not-found state for that case.
 *
 * Calls `GET /cases/{id}` in LIVE mode and falls back to the seeded mock
 * detail when the mock toggle is on.
 */
export function useGetCsmCaseDetail(
  caseId: string | undefined,
): UseQueryResult<CsmCaseDetail | null, Error> {
  const logger = useLogger();
  const api = useBackendApi();

  return useQuery<CsmCaseDetail | null, Error>({
    queryKey: [ApiQueryKeys.CSM_CASE_DETAIL, caseId ?? ""],
    queryFn: async (): Promise<CsmCaseDetail | null> => {
      if (!caseId) return null;

      if (isMockMode()) {
        logger.debug(`[useGetCsmCaseDetail] Returning mock case ${caseId}`);
        await new Promise((r) => setTimeout(r, MOCK_LATENCY_MS));
        return getMockCsmCaseDetailById(caseId) ?? null;
      }

      const beCase = await api.get<BeCaseView>(
        `/cases/${encodeURIComponent(caseId)}`,
      );
      if (!beCase) return null;

      // The CaseView already embeds project / deployment / deployed-product /
      // reporter names, so no lookups are needed for those. Only the account
      // (customer) name is not embedded — resolve it best-effort via the
      // project's account (the embedded project ref carries id + name only, so
      // fetch the full project for its accountId, then the account). Failures
      // degrade gracefully: the case still renders with a "—" customer.
      let account: BeAccount | undefined;
      if (beCase.project?.id) {
        try {
          const fullProject = await api.get<BeProject>(
            `/projects/${encodeURIComponent(beCase.project.id)}`,
          );
          if (fullProject?.accountId) {
            account =
              (await api.get<BeAccount>(
                `/accounts/${encodeURIComponent(fullProject.accountId)}`,
              )) ?? undefined;
          }
        } catch (err) {
          logger.warn(
            `[useGetCsmCaseDetail] account hydrate failed: ${
              (err as Error).message
            }`,
          );
        }
      }
      return detailFromBeCase(beCase, account);
    },
    enabled: !!caseId,
    staleTime: 30_000,
  });
}
