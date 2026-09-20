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

import { useMemo } from "react";
import type { ParentRecordOption } from "@features/csm-operations/utils/changeRequests";
import { useSearchServiceRequestsForSelect } from "@features/csm-operations/api/useSearchServiceRequestsForSelect";
import { useSearchIncidentsForSelect } from "@features/csm-operations/api/useSearchIncidentsForSelect";

/**
 * Unified type-ahead search for the change-request create form's
 * "Originating service request" picker: runs the existing service-request
 * search (`POST /cases/search`, type-filtered to `service_request`, by CS
 * number/subject) and the existing incident search (`POST /incidents/search`,
 * by INC number/subject) in parallel and merges both into one option list —
 * matching the `(query, enabled, extra?) => {data, isFetching, isError}`
 * shape `AsyncEntitySelect` expects, same template as its two underlying
 * hooks. `extra` (a project id) is forwarded only to the service-request
 * search — incidents have no equivalent project-scoping search parameter
 * today (see `useSearchIncidentsForSelect`'s own doc comment).
 *
 * Service requests are listed before incidents in a merged results page —
 * an incident result can be found and selected here (so this form is ready
 * the moment the backend adds support for linking to one), but selecting one
 * must never reach the create-then-PATCH write: see
 * `CreateChangeRequestPage`'s `isIncidentParentSelected` gate, and the header
 * comment on `changeRequests.ts`'s "Originating service request picker"
 * section for exactly why.
 */
export function useSearchParentRecordsForSelect(
  query: string,
  enabled: boolean,
  projectId?: string,
): { data: ParentRecordOption[] | undefined; isFetching: boolean; isError: boolean } {
  const serviceRequests = useSearchServiceRequestsForSelect(query, enabled, projectId);
  const incidents = useSearchIncidentsForSelect(query, enabled);

  const data = useMemo<ParentRecordOption[] | undefined>(() => {
    if (serviceRequests.data === undefined && incidents.data === undefined) return undefined;
    const srOptions: ParentRecordOption[] = (serviceRequests.data ?? []).map((c) => ({
      kind: "service_request",
      id: c.id,
      number: c.number,
      subject: c.subject,
    }));
    // useSearchIncidentsForSelect already filters out any incident without an
    // id, so the cast below is safe.
    const incidentOptions: ParentRecordOption[] = (incidents.data ?? []).map((i) => ({
      kind: "incident",
      id: i.id as string,
      number: i.number,
      subject: i.subject,
    }));
    return [...srOptions, ...incidentOptions];
  }, [serviceRequests.data, incidents.data]);

  return {
    data,
    isFetching: serviceRequests.isFetching || incidents.isFetching,
    isError: serviceRequests.isError || incidents.isError,
  };
}
