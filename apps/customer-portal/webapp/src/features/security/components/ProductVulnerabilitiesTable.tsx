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
  PRODUCT_VULNERABILITIES_SEARCH_DEBOUNCE_MS,
} from "@features/security/constants/securityConstants";
import type { ProductVulnerabilitiesTableProps } from "@features/security/types/security";
import {
  countProductVulnerabilityTableActiveFilters,
} from "@features/security/utils/productVulnerabilitiesTable";

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

  const { data: metaData } = useGetVulnerabilitiesMetaData();
  const severityOptions = useMemo(
    () =>
      metaData?.severities?.map((s) => ({ value: s.id, label: s.label })) ?? [],
    [metaData?.severities],
  );

  const searchRequest = useMemo(
    () => ({
      filters: {
        searchQuery: (debouncedSearch?.trim() || undefined) as
          | string
          | undefined,
        severityId: filters.severityId ? Number(filters.severityId) : undefined,
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
              onFilterChange={handleUpdateFilter}
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
