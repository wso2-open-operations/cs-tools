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

import { useSuspenseQuery } from "@tanstack/react-query";
import { cases } from "@features/cases/api/cases.queries";
import { chats } from "@features/chats/api/chats.queries";
import { changeRequests } from "@features/changes/api/changes.queries";
import { serviceRequests } from "@features/service-requests/api/service-requests.queries";
import { securityReportAnalysis } from "@features/sra/api/sra.queries";
import { engagements } from "@features/engagements/api/engagements.queries";
import { announcements } from "@features/announcements/api/announcements.queries";
import {
  OUTSTANDING_CASE_STATUS_IDS,
  OUTSTANDING_CHANGE_REQUESTS_STATUS_IDS,
  OUTSTANDING_CONVERSATIONS_STATUS_IDS,
} from "@shared/constants/status.constants";

const LIMIT = 5;

export function useOutstandingCases(projectId: string) {
  const { data } = useSuspenseQuery(
    cases.all(projectId, {
      filters: { statusIds: OUTSTANDING_CASE_STATUS_IDS },
      pagination: { limit: LIMIT },
      sortBy: { field: "createdOn", order: "desc" },
    }),
  );
  return { data };
}

export function useOutstandingChats(projectId: string) {
  const { data } = useSuspenseQuery(
    chats.all(projectId, {
      filters: { stateKeys: OUTSTANDING_CONVERSATIONS_STATUS_IDS },
      pagination: { limit: LIMIT },
      sortBy: { field: "createdOn", order: "desc" },
    }),
  );
  return { data };
}

export function useOutstandingChangeRequests(projectId: string) {
  const { data } = useSuspenseQuery(
    changeRequests.all(projectId, {
      filters: { stateKeys: OUTSTANDING_CHANGE_REQUESTS_STATUS_IDS },
      pagination: { offset: 0, limit: LIMIT },
    }),
  );
  return { data };
}

export function useOutstandingServiceRequests(projectId: string) {
  const { data } = useSuspenseQuery(
    serviceRequests.all(projectId, {
      filters: { statusIds: OUTSTANDING_CASE_STATUS_IDS },
      pagination: { limit: LIMIT },
      sortBy: { field: "createdOn", order: "desc" },
    }),
  );
  return { data };
}

export function useOutstandingSecurityReports(projectId: string) {
  const { data } = useSuspenseQuery(
    securityReportAnalysis.all(projectId, {
      filters: { statusIds: OUTSTANDING_CASE_STATUS_IDS },
      pagination: { limit: LIMIT },
      sortBy: { field: "createdOn", order: "desc" },
    }),
  );
  return { data };
}

export function useOutstandingEngagements(projectId: string) {
  const { data } = useSuspenseQuery(
    engagements.all(projectId, {
      filters: { statusIds: OUTSTANDING_CASE_STATUS_IDS },
      pagination: { limit: LIMIT },
      sortBy: { field: "createdOn", order: "desc" },
    }),
  );
  return { data };
}

export function useOutstandingAnnouncements(projectId: string) {
  const { data } = useSuspenseQuery(
    announcements.all(projectId, {
      filters: { statusIds: OUTSTANDING_CASE_STATUS_IDS },
      pagination: { limit: LIMIT },
      sortBy: { field: "createdOn", order: "desc" },
    }),
  );
  return { data };
}
