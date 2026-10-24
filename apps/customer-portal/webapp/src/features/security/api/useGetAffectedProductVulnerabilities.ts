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
import { useLogger } from "@hooks/useLogger";
import { ApiQueryKeys } from "@constants/apiConstants";
import { postProductVulnerabilitiesSearch } from "@features/security/api/usePostProductVulnerabilitiesSearch";
import {
  PRODUCT_VULNERABILITIES_AFFECTED_FRESH_MS,
  PRODUCT_VULNERABILITIES_AFFECTED_MAX_PAGES,
  PRODUCT_VULNERABILITIES_MAX_PAGE_SIZE,
} from "@features/security/constants/securityConstants";
import type { ProductVulnerability } from "@features/security/types/security";

/** Every loaded record of one CVE, plus how many the server reports in total. */
export type AffectedProductVulnerabilities = {
  productVulnerabilities: ProductVulnerability[];
  /** Total records of the CVE on the server; greater than the loaded count only when the page cap truncated. */
  totalRecords: number;
};

/**
 * Loads every record of one CVE (POST /products/vulnerabilities/search with the
 * exact `cveId` filter), 50 per page. The first page is fetched alone to learn
 * `totalRecords`; the remaining pages are then fetched in parallel, capped at
 * PRODUCT_VULNERABILITIES_AFFECTED_MAX_PAGES pages. Records are de-duplicated by id.
 *
 * Idle until a CVE is known and the user is signed in.
 *
 * @param {string | undefined} cveId - The CVE to load records for.
 * @returns {UseQueryResult<AffectedProductVulnerabilities, Error>} The records and the server total.
 */
export function useGetAffectedProductVulnerabilities(
  cveId: string | undefined,
): UseQueryResult<AffectedProductVulnerabilities, Error> {
  const logger = useLogger();
  const { isSignedIn, isLoading: isAuthLoading } = useAsgardeo();
  const authFetch = useAuthApiClient();
  const cve = cveId?.trim() ?? "";

  return useQuery<AffectedProductVulnerabilities, Error>({
    queryKey: [ApiQueryKeys.PRODUCT_VULNERABILITIES_AFFECTED, cve],
    queryFn: async (): Promise<AffectedProductVulnerabilities> => {
      const fetchPage = (pageIndex: number) =>
        postProductVulnerabilitiesSearch(authFetch, logger, {
          filters: { cveId: cve },
          pagination: {
            offset: pageIndex * PRODUCT_VULNERABILITIES_MAX_PAGE_SIZE,
            limit: PRODUCT_VULNERABILITIES_MAX_PAGE_SIZE,
          },
        });

      const first = await fetchPage(0);
      const totalRecords = first.totalRecords;
      const pageCount = Math.min(
        Math.ceil(totalRecords / PRODUCT_VULNERABILITIES_MAX_PAGE_SIZE),
        PRODUCT_VULNERABILITIES_AFFECTED_MAX_PAGES,
      );
      const rest = await Promise.all(
        Array.from({ length: Math.max(pageCount - 1, 0) }, (_, index) =>
          fetchPage(index + 1),
        ),
      );

      const byId = new Map<string, ProductVulnerability>();
      for (const page of [first, ...rest]) {
        for (const record of page.productVulnerabilities) {
          if (!byId.has(record.id)) byId.set(record.id, record);
        }
      }
      return { productVulnerabilities: [...byId.values()], totalRecords };
    },
    enabled: cve !== "" && isSignedIn && !isAuthLoading,
    staleTime: PRODUCT_VULNERABILITIES_AFFECTED_FRESH_MS,
  });
}
