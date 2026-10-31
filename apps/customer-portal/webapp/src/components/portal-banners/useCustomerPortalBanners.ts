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
import { useAsgardeo } from "@asgardeo/react";
import { useAuthApiClient } from "@/hooks/useAuthApiClient";
import { CORRELATION_ID_HEADER, newCorrelationId } from "@utils/correlationId";

/** A published banner the customer shell may render. */
export interface CustomerPortalBanner {
  id: string;
  sortOrder: number;
  html: string;
  startsAt: string | null;
  expiresAt: string | null;
}

/** Published banners for the customer portal. Hidden until the person is signed in. */
export function useCustomerPortalBanners(): UseQueryResult<CustomerPortalBanner[], Error> {
  const { isSignedIn, isLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();
  return useQuery<CustomerPortalBanner[], Error>({
    queryKey: ["portal-banners"],
    enabled: isSignedIn && !isLoading,
    queryFn: async () => {
      const baseUrl = window.config?.CUSTOMER_PORTAL_BACKEND_BASE_URL;
      if (!baseUrl) {
        throw new Error("CUSTOMER_PORTAL_BACKEND_BASE_URL is not configured");
      }
      const response = await authFetch(`${baseUrl}/banners`, {
        method: "GET",
        cache: "no-store",
        headers: { [CORRELATION_ID_HEADER]: newCorrelationId() },
      });
      if (!response.ok) {
        throw new Error("Could not load banners.");
      }
      const data = (await response.json()) as { banners?: CustomerPortalBanner[] };
      return data.banners ?? [];
    },
  });
}
