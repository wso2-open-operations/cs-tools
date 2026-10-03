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

// POST /customer-health/summary is a search endpoint (POST verb, but a
// read, triggered by filter changes) — modeled as a useQuery keyed on the
// payload, same as this app's other POST-as-search endpoints, rather than a
// useMutation fired from an effect (the source app's own useSplApi-based
// pattern, which this rewrite intentionally does not carry over).
import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { useBackendApi } from "@api/backend/client";
import type { AccountSummary } from "./customerHealthTypes";

export interface CustomerHealthSummaryPayload {
  offset: number;
  limit: number;
  email: string;
  phrase: string;
  risks: string;
  region: string[];
  product: string;
  abtTeam: string;
  healthStatus: string;
}

export interface CustomerHealthSummaryResponse {
  data: AccountSummary[];
  totalCount: number;
}

export function useCustomerHealthSummary(
  payload: CustomerHealthSummaryPayload,
): UseQueryResult<CustomerHealthSummaryResponse, Error> {
  const backendApi = useBackendApi();
  return useQuery<CustomerHealthSummaryResponse, Error>({
    queryKey: ["spl-customer-health-summary", payload],
    queryFn: () =>
      backendApi.post<CustomerHealthSummaryPayload, CustomerHealthSummaryResponse>(
        "/customer-health/summary",
        payload,
      ),
  });
}

/** One-shot (non-hook) call for the CSV export's manual pagination loop. */
export function fetchCustomerHealthSummaryPage(
  backendApi: ReturnType<typeof useBackendApi>,
  payload: CustomerHealthSummaryPayload,
): Promise<CustomerHealthSummaryResponse> {
  return backendApi.post<CustomerHealthSummaryPayload, CustomerHealthSummaryResponse>(
    "/customer-health/summary",
    payload,
  );
}
