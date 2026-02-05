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
import { type JSX, useState, useEffect } from "react";
import { useNavigate } from "react-router";
import { useAsgardeo } from "@asgardeo/react";
import useGetProjectCases from "@/api/useGetProjectCases";
import useGetCasesFilters from "@/api/useGetCasesFilters";
import { useLoader } from "@/context/linear-loader/LoaderContext";
import FilterPopover, {
  type FilterField,
} from "@/components/common/filter-panel/FilterPopover";
import type { CaseSearchRequest } from "@/models/requests";
import CasesTableHeader from "./CasesTableHeader";
import CasesList from "./CasesList";

interface CasesTableProps {
  projectId: string;
}

const CasesTable = ({ projectId }: CasesTableProps): JSX.Element => {
  const navigate = useNavigate();
  const { isLoading: isAuthLoading } = useAsgardeo();
  const { showLoader, hideLoader } = useLoader();
  const [filters, setFilters] = useState<Record<string, any>>({});
  const [isFilterOpen, setIsFilterOpen] = useState(false);
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(5);

  const {
    data: filtersMetadata,
    isFetching: isFetchingFilters,
    isError: isErrorFilters,
  } = useGetCasesFilters(projectId);

  const dynamicFilterFields: FilterField[] = [
    {
      id: "statusId",
      label: "Status",
      type: "select",
      options:
        filtersMetadata?.statuses.map((s) => ({
          label: s.label,
          value: s.id,
        })) || [],
    },
    {
      id: "severityId",
      label: "Severity",
      type: "select",
      options:
        filtersMetadata?.severities.map((s) => ({
          label: s.label,
          value: s.id,
        })) || [],
    },
    {
      id: "caseTypes",
      label: "Case Type",
      type: "select",
      options: filtersMetadata?.caseTypes.map((t) => t.label) || [],
    },
    {
      id: "deploymentId",
      label: "Deployment",
      type: "select",
      options: [
        { label: "Development", value: "Development" },
        { label: "Production", value: "Production" },
        { label: "QA", value: "QA" },
        { label: "Staging", value: "Staging" },
      ],
    },
  ];

  const hasFilters = Object.values(filters).some(
    (val) => val !== undefined && val !== "" && val !== null,
  );

  const requestBody: CaseSearchRequest = {
    ...(hasFilters && {
      filters: {
        deploymentId: filters.deploymentId || undefined,
        severityId: filters.severityId
          ? parseInt(filters.severityId, 10)
          : undefined,
        statusId: filters.statusId ? parseInt(filters.statusId, 10) : undefined,
        caseTypes: filters.caseTypes ? [filters.caseTypes] : undefined,
      },
    }),
    pagination: {
      offset: page * rowsPerPage,
      limit: rowsPerPage,
    },
    sortBy: {
      field: "createdOn",
      order: "desc",
    },
  };

  const {
    data,
    isFetching: isFetchingCases,
    isError,
  } = useGetProjectCases(projectId, requestBody);

  const tableLoading = isAuthLoading || isFetchingCases || isFetchingFilters;

  useEffect(() => {
    if (tableLoading) {
      showLoader();
    } else {
      hideLoader();
    }
    return () => hideLoader();
  }, [tableLoading, showLoader, hideLoader]);

  const handleFilterSearch = (newFilters: Record<string, any>) => {
    setFilters(newFilters);
    setPage(0);
  };

  const handleUpdateFilter = (field: string, value: any) => {
    setFilters((prev) => ({
      ...prev,
      [field]: value,
    }));
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

  const activeFilterFields = dynamicFilterFields.map((field) => ({
    id: field.id,
    label: field.label,
    options: field.options,
  }));

  const getDisplayValue = (fieldId: string, value: any): string => {
    if (!value) return "";
    const field = dynamicFilterFields.find((f) => f.id === fieldId);
    if (field?.options) {
      const option = field.options.find((opt) =>
        typeof opt === "string" ? opt === value : opt.value === value,
      );
      if (option) {
        return typeof option === "string" ? option : option.label;
      }
    }
    return String(value);
  };

  const mappedAppliedFilters = Object.entries(filters).reduce(
    (acc, [key, value]) => {
      acc[key] = getDisplayValue(key, value);
      return acc;
    },
    {} as Record<string, string>,
  );

  return (
    <ListingTable.Container sx={{ width: "100%", mb: 4, p: 3 }}>
      {/* Header */}
      <CasesTableHeader
        activeFiltersCount={Object.keys(filters).length}
        appliedFilters={mappedAppliedFilters}
        filterFields={activeFilterFields}
        onRemoveFilter={handleRemoveFilter}
        onUpdateFilter={handleUpdateFilter}
        onClearAll={handleClearFilters}
        onFilterClick={() => setIsFilterOpen(true)}
        onCreateCase={() => navigate(`/${projectId}/support/chat`)}
      />

      {/* Cases list */}
      <CasesList
        isLoading={isFetchingCases || isAuthLoading}
        isError={isError}
        data={data}
        page={page}
        rowsPerPage={rowsPerPage}
        onPageChange={handleChangePage}
        onRowsPerPageChange={handleChangeRowsPerPage}
      />

      {/* Filter popover */}
      <FilterPopover
        open={isFilterOpen}
        onClose={() => setIsFilterOpen(false)}
        onSearch={handleFilterSearch}
        initialFilters={filters}
        fields={dynamicFilterFields}
        title="Filter Cases"
        isLoading={isFetchingFilters || isAuthLoading}
        isError={isErrorFilters}
      />
    </ListingTable.Container>
  );
};

export default CasesTable;
