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

import { ListingTable } from "@wso2/oxygen-ui";
import { type JSX, useState, useMemo, useEffect } from "react";
import { usePostProductVulnerabilitiesSearch } from "@api/usePostProductVulnerabilitiesSearch";
import { useGetVulnerabilitiesMetaData } from "@api/useGetVulnerabilitiesMetaData";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import FilterPopover, {
  type FilterField,
} from "@components/common/filter-panel/FilterPopover";
import ProductVulnerabilitiesTableHeader from "@components/security/ProductVulnerabilitiesTableHeader";
import ProductVulnerabilitiesList from "@components/security/ProductVulnerabilitiesList";

export interface ProductVulnerabilitiesTableProps {
  onTotalRecordsChange?: (total: number) => void;
  onError?: (isError: boolean) => void;
  onVulnerabilityClick?: (vulnerability: { id: string }) => void;
}

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
  const debouncedSearch = useDebouncedValue(searchInput, 350);
  const [filters, setFilters] = useState<Record<string, string>>({});
  const [isFilterOpen, setIsFilterOpen] = useState(false);
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(10);

  const { data: metaData, isError: isMetaDataError } =
    useGetVulnerabilitiesMetaData();
  const severityOptions = useMemo(
    () =>
      metaData?.severities?.map((s) => ({ value: s.id, label: s.label })) ?? [],
    [metaData?.severities],
  );

  const dynamicFilterFields: FilterField[] = useMemo(
    () => [
      {
        id: "severityId",
        label: "Severity",
        type: "select",
        options: severityOptions,
      },
    ],
    [severityOptions],
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

  const { data, isFetching, isError } =
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

  const handleFilterSearch = (newFilters: Record<string, string>) => {
    setFilters(newFilters);
    setPage(0);
  };

  const handleUpdateFilter = (field: string, value: string) => {
    setFilters((prev) => ({ ...prev, [field]: value }));
    setPage(0);
  };

  const handleRemoveFilter = (field: string) => {
    const newFilters = { ...filters };
    delete newFilters[field];
    setFilters(newFilters);
    setPage(0);
  };

  const handleClearFilters = () => {
    setFilters({});
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

  const activeFilterFields = useMemo(
    () =>
      dynamicFilterFields
        .filter((f) => f.type === "select")
        .map((field) => ({
          id: field.id,
          label: field.label,
          options: (field.options || []).map((opt) =>
            typeof opt === "string" ? { value: opt, label: opt } : opt,
          ),
        })),
    [dynamicFilterFields],
  );

  const appliedFilters = useMemo(() => {
    const result: Record<string, string> = {};
    for (const [key, value] of Object.entries(filters)) {
      if (!value) continue;
      if (key === "severityId") {
        const label = severityOptions.find((o) => o.value === value)?.label;
        result[key] = label ?? value;
      } else {
        result[key] = value;
      }
    }
    return result;
  }, [filters, severityOptions]);

  const displayFiltersForPopover = useMemo(() => ({ ...filters }), [filters]);

  return (
    <ListingTable.Container sx={{ width: "100%", mb: 4, p: 3 }}>
      <ProductVulnerabilitiesTableHeader
        searchValue={searchInput}
        onSearchChange={(v) => {
          setSearchInput(v);
          setPage(0);
        }}
        onFilterIconClick={() => setIsFilterOpen(true)}
        activeFiltersCount={
          activeFilterFields.filter((f) => appliedFilters[f.id]).length
        }
        appliedFilters={appliedFilters}
        filterFields={activeFilterFields}
        onRemoveFilter={handleRemoveFilter}
        onClearAll={handleClearFilters}
        onUpdateFilter={handleUpdateFilter}
      />

      <ProductVulnerabilitiesList
        isLoading={isFetching || (!data && !isError)}
        isError={isError}
        data={paginatedData}
        page={page}
        rowsPerPage={rowsPerPage}
        onPageChange={handleChangePage}
        onRowsPerPageChange={handleChangeRowsPerPage}
        onVulnerabilityClick={onVulnerabilityClick}
      />

      <FilterPopover
        open={isFilterOpen}
        onClose={() => setIsFilterOpen(false)}
        onSearch={handleFilterSearch}
        initialFilters={displayFiltersForPopover}
        fields={dynamicFilterFields}
        title="Filter Vulnerabilities"
        isLoading={!metaData}
        isError={isMetaDataError}
      />
    </ListingTable.Container>
  );
};

export default ProductVulnerabilitiesTable;
