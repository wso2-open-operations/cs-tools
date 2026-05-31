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
  PRODUCT_VULNERABILITIES_ALL_FETCH_LIMIT,
  PRODUCT_VULNERABILITIES_DEFAULT_ROWS_PER_PAGE,
  PRODUCT_VULNERABILITIES_SEARCH_DEBOUNCE_MS,
} from "@features/security/constants/securityConstants";
import type {
  ProductVulnerabilitiesTableProps,
  ProductVulnerability,
} from "@features/security/types/security";
import { countProductVulnerabilityTableActiveFilters } from "@features/security/utils/productVulnerabilitiesTable";

// Stable request object — fetches the full dataset once.
// The BFF transparently paginates through the entity service in batches of 50.
// Kept at module level so the React Query cache key is stable across re-renders.
const FETCH_ALL_REQUEST = {
  filters: {},
  pagination: { offset: 0, limit: PRODUCT_VULNERABILITIES_ALL_FETCH_LIMIT },
};

/**
 * Product Vulnerabilities table.
 *
 * Architecture note:
 * - All ~1500 records are fetched once via a single "fetch all" request.
 * - The BFF handles batching to the entity service (which is capped at 50/request).
 * - All filtering, sorting, and pagination are performed client-side for instant response.
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

  // ── Severity dropdown metadata ──────────────────────────────────────────────
  const { data: metaData } = useGetVulnerabilitiesMetaData();
  const severityOptions = useMemo(
    () =>
      metaData?.severities?.map((s) => ({ value: s.id, label: s.label })) ?? [],
    [metaData?.severities],
  );

  // ── Full dataset (fetched once, all filtering/pagination done client-side) ───
  const {
    data: allData,
    isLoading,
    isError,
  } = usePostProductVulnerabilitiesSearch(FETCH_ALL_REQUEST);

  const allVulnerabilities = useMemo(
    (): ProductVulnerability[] => allData?.productVulnerabilities ?? [],
    [allData],
  );

  // ── Dropdown options ─────────────────────────────────────────────────────────
  const productOptions = useMemo(() => {
    const names = new Set<string>();
    allVulnerabilities.forEach((v) => {
      if (v.productName) names.add(v.productName);
    });
    return Array.from(names)
      .sort()
      .map((name) => ({ value: name, label: name }));
  }, [allVulnerabilities]);

  const productVersionOptions = useMemo(() => {
    if (!filters.productName) return [];
    const versions = new Set<string>();
    allVulnerabilities
      .filter((v) => v.productName === filters.productName)
      .forEach((v) => {
        if (v.productVersion) versions.add(v.productVersion);
      });
    return Array.from(versions)
      .sort()
      .map((v) => ({ value: v, label: v }));
  }, [allVulnerabilities, filters.productName]);

  // ── Client-side filtering ────────────────────────────────────────────────────
  const filteredVulnerabilities = useMemo((): ProductVulnerability[] => {
    let items = allVulnerabilities;

    if (filters.severityId) {
      items = items.filter(
        (v) => String(v.severity?.id) === String(filters.severityId),
      );
    }
    if (filters.productName) {
      items = items.filter(
        (v) => v.productName === (filters.productName as string),
      );
    }
    if (filters.productVersion) {
      items = items.filter(
        (v) => v.productVersion === (filters.productVersion as string),
      );
    }
    if (debouncedSearch?.trim()) {
      const q = debouncedSearch.trim().toLowerCase();
      items = items.filter(
        (v) =>
          v.cveId?.toLowerCase().includes(q) ||
          v.componentName?.toLowerCase().includes(q) ||
          v.vulnerabilityId?.toLowerCase().includes(q),
      );
    }

    return items;
  }, [allVulnerabilities, filters, debouncedSearch]);

  // ── Client-side pagination ───────────────────────────────────────────────────
  const paginatedData = useMemo(() => {
    const total = filteredVulnerabilities.length;
    const offset = page * rowsPerPage;
    return {
      vulnerabilities: filteredVulnerabilities.slice(offset, offset + rowsPerPage),
      totalRecords: total,
    };
  }, [filteredVulnerabilities, page, rowsPerPage]);

  // Clamp the current page when the filtered dataset shrinks (e.g. after a
  // filter change) so the table never shows an empty page while valid rows exist.
  useEffect(() => {
    const maxPage = Math.max(
      Math.ceil(filteredVulnerabilities.length / rowsPerPage) - 1,
      0,
    );
    if (page > maxPage) {
      setPage(maxPage);
    }
  }, [filteredVulnerabilities.length, rowsPerPage, page]);

  useEffect(() => {
    onTotalRecordsChange?.(paginatedData.totalRecords);
  }, [paginatedData.totalRecords, onTotalRecordsChange]);

  useEffect(() => {
    onError?.(isError);
  }, [isError, onError]);

  // ── Handlers ────────────────────────────────────────────────────────────────
  const handleUpdateFilter = (field: string, value: string | number) => {
    setFilters((prev) => ({
      ...prev,
      [field]: value,
      ...(field === "productName" ? { productVersion: "" } : {}),
    }));
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
    () => countProductVulnerabilityTableActiveFilters("", filters),
    [filters],
  );

  // ── Render ───────────────────────────────────────────────────────────────────
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

      {!isLoading && isFilterOpen && (
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
        isLoading={isLoading}
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
