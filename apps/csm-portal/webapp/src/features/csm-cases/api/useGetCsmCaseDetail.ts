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
import {
  severityFromPriority,
  uiStateFromBe,
} from "@api/backend/mappers";
import type {
  BeAccount,
  BeCase,
  BeProject,
} from "@api/backend/types";
import { getMockCsmCaseDetailById } from "@features/csm-cases/api/mocks/casesMocks";
import type { CsmCaseDetail } from "@features/csm-cases/types/csmCases";

const MOCK_LATENCY_MS = 150;

/**
 * Build a thin CsmCaseDetail from the lean `BeCase` payload plus optional
 * hydrated project/account context. Side widgets (SLA clocks, watchers, tags,
 * time logs, attachments, linked items, customer + product context) are not
 * yet returned by the backend, so they default to empty / placeholder values.
 * The UI degrades gracefully — sections render empty states rather than
 * crashing.
 */
/**
 * `createdBy` is typed `string` but the live API returns a `{id, email}`
 * UserRef. Coerce either shape to a display string so rendering it never
 * throws "Objects are not valid as a React child".
 */
function createdByName(v: unknown): string | undefined {
  if (typeof v === "string") return v || undefined;
  if (v && typeof v === "object") {
    const ref = v as { email?: string; id?: string };
    return ref.email ?? ref.id ?? undefined;
  }
  return undefined;
}

function detailFromBeCase(
  c: BeCase,
  project?: BeProject,
  account?: BeAccount,
): CsmCaseDetail {
  const customer = account?.name ?? "—";
  return {
    id: c.id,
    caseNumber: c.number ?? c.id,
    wso2CaseId: c.wso2Id ?? c.id,
    subject: c.subject ?? "(no subject)",
    customer,
    accountId: project?.accountId ?? account?.id ?? "",
    projectId: c.projectId ?? "",
    projectName: project?.name ?? project?.projectKey ?? "—",
    product: "—",
    severity: severityFromPriority(c.priority),
    state: uiStateFromBe(c.state),
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
    createdBy: createdByName(c.createdBy),
    customerContext: {
      accountName: customer,
      tier: "subscription",
      region: account?.region ?? "—",
      primaryContact: "—",
      primaryContactEmail: "—",
      accountManager: account?.ownerId ?? "—",
      openCases: 0,
    },
    productContext: {
      product: "—",
      version: "—",
      deployment: "—",
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

      const beCase = await api.get<BeCase>(
        `/cases/${encodeURIComponent(caseId)}`,
      );
      if (!beCase) return null;

      // Hydrate project + account so the customer / project links in the
      // page header are not empty strings (which would no-op the click). Use
      // the by-id endpoints (GET /projects/{id}, GET /accounts/{id}) so this is
      // exact rather than search-and-filter, which silently missed records
      // outside the first page. `get` resolves 404/204 to null; failures
      // degrade gracefully — the case still renders with "—" placeholders.
      let project: BeProject | undefined;
      let account: BeAccount | undefined;
      if (beCase.projectId) {
        try {
          project =
            (await api.get<BeProject>(
              `/projects/${encodeURIComponent(beCase.projectId)}`,
            )) ?? undefined;
        } catch (err) {
          logger.warn(
            `[useGetCsmCaseDetail] project hydrate failed: ${
              (err as Error).message
            }`,
          );
        }
      }
      if (project?.accountId) {
        try {
          account =
            (await api.get<BeAccount>(
              `/accounts/${encodeURIComponent(project.accountId)}`,
            )) ?? undefined;
        } catch (err) {
          logger.warn(
            `[useGetCsmCaseDetail] account hydrate failed: ${
              (err as Error).message
            }`,
          );
        }
      }
      return detailFromBeCase(beCase, project, account);
    },
    enabled: !!caseId,
    staleTime: 30_000,
  });
}
