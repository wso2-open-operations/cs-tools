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

import { ListingTable, Divider, Box } from "@wso2/oxygen-ui";
import { type JSX, useState, useMemo, useEffect } from "react";
import { usePostProductVulnerabilitiesSearch } from "@features/security/api/usePostProductVulnerabilitiesSearch";
import { useGetVulnerabilitiesMetaData } from "@features/security/api/useGetVulnerabilitiesMetaData";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import ProductVulnerabilitiesTableHeader from "@features/security/components/ProductVulnerabilitiesTableHeader";
import ProductVulnerabilitiesFilters from "@features/security/components/ProductVulnerabilitiesFilters";
import ProductVulnerabilitiesList from "@features/security/components/ProductVulnerabilitiesList";
import {
  PRODUCT_VULNERABILITIES_DEFAULT_ROWS_PER_PAGE,
  PRODUCT_VULNERABILITIES_OPTIONS_FETCH_LIMIT,
  PRODUCT_VULNERABILITIES_SEARCH_DEBOUNCE_MS,
} from "@features/security/constants/securityConstants";
import type { ProductVulnerabilitiesTableProps } from "@features/security/types/security";
import {
  countProductVulnerabilityTableActiveFilters,
} from "@features/security/utils/productVulnerabilitiesTable";

// Stable request object for fetching all vulnerabilities to populate dropdown options.
// Kept outside the component to avoid creating a new object reference on every render.
const OPTIONS_FETCH_REQUEST = {
  pagination: { offset: 0, limit: PRODUCT_VULNERABILITIES_OPTIONS_FETCH_LIMIT },
};

/**
 * Product Vulnerabilities table using the same structure as the dashboard outstanding cases table.
 * Fetches data via POST /products/vulnerabilities/search.
 * @returns {JSX.Element}
 */
const ProductVulnerabilitiesTable = ({
  onTotalRecordsChange,
  onError,
  onVulnerabilityClick,
}: ProductVulnerabilitiesTableProps): JSX.Element => {
  const [searchInput, setSearchInput] = useState("");
  const debouncedSearch = useDebouncedValue(
    searchInput,
    PRODUCT_VULNERABILITIES_SEARCH_DEBOUNCE_MS,
  );
  const [filters, setFilters] = useState<Record<string, string | number>>({});
  const [isFilterOpen, setIsFilterOpen] = useState(false);
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(
    PRODUCT_VULNERABILITIES_DEFAULT_ROWS_PER_PAGE,
  );

  // --- Metadata for severity options ---
  const { data: metaData } = useGetVulnerabilitiesMetaData();
  const severityOptions = useMemo(
    () =>
      metaData?.severities?.map((s) => ({ value: s.id, label: s.label })) ?? [],
    [metaData?.severities],
  );

  // --- All-records fetch for product name / version dropdown options ---
  // Uses the stable OPTIONS_FETCH_REQUEST constant to avoid re-fetching on every render.
  const { data: allVulnerabilitiesData } = usePostProductVulnerabilitiesSearch(
    OPTIONS_FETCH_REQUEST,
  );

  // Combine the bulk-fetch records with the current page so that options are
  // immediately available from the first paginated response, even while the
  // larger OPTIONS query is still in-flight.
  const allKnownVulnerabilities = useMemo(() => {
    const seen = new Map<string, { productName?: string | null; productVersion?: string | null }>();
    allVulnerabilitiesData?.productVulnerabilities.forEach((v) => seen.set(v.id, v));
    data?.productVulnerabilities.forEach((v) => seen.set(v.id, v));
    return Array.from(seen.values());
  }, [allVulnerabilitiesData, data]);

  const productOptions = useMemo(() => {
    const names = new Set<string>();
    allKnownVulnerabilities.forEach((v) => {
      if (v.productName) names.add(v.productName);
    });
    return Array.from(names)
      .sort()
      .map((name) => ({ value: name, label: name }));
  }, [allKnownVulnerabilities]);

  const productVersionOptions = useMemo(() => {
    if (!filters.productName) return [];
    const versions = new Set<string>();
    allKnownVulnerabilities
      .filter((v) => v.productName === filters.productName)
      .forEach((v) => {
        if (v.productVersion) versions.add(v.productVersion);
      });
    return Array.from(versions)
      .sort()
      .map((v) => ({ value: v, label: v }));
  }, [allKnownVulnerabilities, filters.productName]);

  // --- Main paginated search request ---
  const searchRequest = useMemo(
    () => ({
      filters: {
        searchQuery: (debouncedSearch?.trim() || undefined) as
          | string
          | undefined,
        severityId: filters.severityId ? Number(filters.severityId) : undefined,
        productName: (filters.productName as string) || undefined,
        productVersion: (filters.productVersion as string) || undefined,
      },
      pagination: {
        offset: page * rowsPerPage,
        limit: rowsPerPage,
      },
    }),
    [debouncedSearch, filters, page, rowsPerPage],
  );

  const { data, isLoading, isError } =
    usePostProductVulnerabilitiesSearch(searchRequest);

  useEffect(() => {
    if (data?.totalRecords !== undefined) {
      onTotalRecordsChange?.(data.totalRecords);
    }
  }, [data?.totalRecords, onTotalRecordsChange]);

  useEffect(() => {
    if (isError) onError?.(true);
  }, [isError, onError]);

  const paginatedData = useMemo(() => {
    if (!data) return undefined;
    return {
      vulnerabilities: data.productVulnerabilities,
      totalRecords: data.totalRecords,
    };
  }, [data]);

  const handleUpdateFilter = (field: string, value: string | number) => {
    setFilters((prev) => ({ ...prev, [field]: value }));
    setPage(0);
  };

  const handleClearFilters = () => {
    setFilters({});
    setSearchInput("");
    setPage(0);
  };

  const handleChangePage = (_event: unknown, newPage: number) => {
    setPage(newPage);
  };

  const handleChangeRowsPerPage = (
    event: React.ChangeEvent<HTMLInputElement>,
  ) => {
    setRowsPerPage(parseInt(event.target.value, 10));
    setPage(0);
  };

  const activeFilterCount = useMemo(
    () =>
      countProductVulnerabilityTableActiveFilters(searchInput, filters),
    [filters, searchInput],
  );

  return (
    <ListingTable.Container sx={{ width: "100%", mb: 4, p: 3 }}>
      <ProductVulnerabilitiesTableHeader
        searchValue={searchInput}
        onSearchChange={(v) => {
          setSearchInput(v);
          setPage(0);
        }}
        onFilterToggle={() => {
          if (activeFilterCount > 0) {
            handleClearFilters();
          } else {
            setIsFilterOpen(!isFilterOpen);
          }
        }}
        isFiltersOpen={isFilterOpen}
        activeFiltersCount={activeFilterCount}
      />

      {/* Filter dropdowns section */}
      {isFilterOpen && (
        <>
          <Divider sx={{ my: 2 }} />
          <Box sx={{ mb: 3 }}>
            <ProductVulnerabilitiesFilters
              filters={filters}
              severityOptions={severityOptions}
              productOptions={productOptions}
              productVersionOptions={productVersionOptions}
              onFilterChange={handleUpdateFilter}
              onClearFilters={handleClearFilters}
            />
          </Box>
        </>
      )}

      <ProductVulnerabilitiesList
        isLoading={isLoading || (!data && !isError)}
        isError={isError}
        data={paginatedData}
        page={page}
        rowsPerPage={rowsPerPage}
        onPageChange={handleChangePage}
        onRowsPerPageChange={handleChangeRowsPerPage}
        onVulnerabilityClick={onVulnerabilityClick}
      />
    </ListingTable.Container>
  );
};

export default ProductVulnerabilitiesTable;
