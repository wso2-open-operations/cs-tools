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
import { type JSX, useState, useMemo, useCallback, useEffect } from "react";
import { useNavigate } from "react-router";
import { useAsgardeo } from "@asgardeo/react";
import { getNoveraChatEnabled } from "@utils/settingsStorage";
import useGetProjectCases from "@api/useGetProjectCases";
import { useGetProjectCasesPage } from "@api/useGetProjectCasesPage";
import useGetProjectFilters from "@api/useGetProjectFilters";
import { useGetDeployments } from "@api/useGetDeployments";
import FilterPopover, {
  type FilterField,
} from "@components/common/filter-panel/FilterPopover";
import CasesTableHeader from "@components/dashboard/cases-table/CasesTableHeader";
import CasesList from "@components/dashboard/cases-table/CasesList";
import {
  normalizeCaseTypeOptions,
  mapSeverityToDisplay,
  isS0Case,
} from "@utils/support";
import { isS0SeverityLabel } from "@constants/dashboardConstants";
import type { CaseListItem, CaseSearchResponse } from "@models/responses";

const OUTSTANDING_STATUS_IDS = [1, 10, 18, 1003, 1006] as const;

interface CasesTableProps {
  projectId: string;
  excludeS0?: boolean;
}

const CasesTable = ({
  projectId,
  excludeS0 = false,
}: CasesTableProps): JSX.Element => {
  const navigate = useNavigate();
  const { isLoading: isAuthLoading } = useAsgardeo();
  const [filters, setFilters] = useState<Record<string, any>>({});
  const [isFilterOpen, setIsFilterOpen] = useState(false);
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(5);
  const [showAll, setShowAll] = useState(false);
  const [isLoadingAll, setIsLoadingAll] = useState(false);

  const {
    data: filtersMetadata,
    isFetching: isFetchingFilters,
    isError: isErrorFilters,
  } = useGetProjectFilters(projectId);

  // Fetch deployments for the deployment filter
  const { data: deploymentsData } = useGetDeployments(projectId);

  const severityOptions = (filtersMetadata?.severities ?? [])
    .filter((s) => !excludeS0 || !isS0SeverityLabel(s.label))
    .map((s) => ({
      label: mapSeverityToDisplay(s.label),
      value: s.id,
    }));

  const dynamicFilterFields: FilterField[] = [
    {
      id: "statusId",
      label: "Status",
      type: "select",
      options: (
        filtersMetadata?.caseStates ??
        filtersMetadata?.statuses ??
        []
      ).map((s) => ({ label: s.label, value: s.id })),
    },
    {
      id: "severityId",
      label: "Severity",
      type: "select",
      options: severityOptions,
    },
    {
      id: "issueTypes",
      label: "Category",
      type: "select",
      options:
        filtersMetadata?.issueTypes?.map((t) => ({
          label: t.label,
          value: t.id,
        })) || [],
    },
    {
      id: "deploymentId",
      label: "Deployment",
      type: "select",
      options:
        deploymentsData?.deployments?.map((d) => ({
          label: d.type?.label || d.name,
          value: d.id,
        })) || [],
    },
    {
      id: "caseTypes",
      label: "Case Type",
      type: "select",
      options: normalizeCaseTypeOptions(filtersMetadata?.caseTypes || []),
    },
  ];

  const caseSearchRequest = useMemo(
    () => ({
      filters: {
        statusId: filters.statusId ? Number(filters.statusId) : undefined,
        statusIds: filters.statusId ? undefined : [...OUTSTANDING_STATUS_IDS],
        severityId: filters.severityId ? Number(filters.severityId) : undefined,
        issueId: filters.issueTypes ? Number(filters.issueTypes) : undefined,
        deploymentId: filters.deploymentId || undefined,
        caseTypes: filters.caseTypes?.length ? [filters.caseTypes] : undefined,
      },
      sortBy: {
        field: "createdOn",
        order: "desc" as const,
      },
    }),
    [filters],
  );

  const offset = page * rowsPerPage;
  const limit = rowsPerPage;

  const pageQuery = useGetProjectCasesPage(
    projectId,
    caseSearchRequest,
    offset,
    limit,
    { enabled: !showAll },
  );

  const infiniteQuery = useGetProjectCases(projectId, caseSearchRequest, {
    enabled: showAll,
  });

  useEffect(() => {
    if (showAll && infiniteQuery.hasNextPage && !infiniteQuery.isFetchingNextPage && isLoadingAll) {
      void infiniteQuery.fetchNextPage();
    } else if (showAll && !infiniteQuery.hasNextPage && isLoadingAll) {
      setIsLoadingAll(false);
    }
  }, [
    showAll,
    infiniteQuery.hasNextPage,
    infiniteQuery.isFetchingNextPage,
    infiniteQuery.fetchNextPage,
    isLoadingAll,
    infiniteQuery.data,
  ]);

  const paginatedData = useMemo((): CaseSearchResponse => {
    const filterS0 = (items: CaseListItem[]): CaseListItem[] =>
      excludeS0 ? items.filter((c) => !isS0Case(c)) : items;

    if (showAll) {
      if (!infiniteQuery.data) {
        return { cases: [], totalRecords: 0, offset: 0, limit: 0 };
      }
      const rawCases = infiniteQuery.data.pages.flatMap((p) => p.cases ?? []);
      const cases = filterS0(rawCases);
      const totalRecords = excludeS0
        ? cases.length
        : (infiniteQuery.data.pages[0]?.totalRecords ?? cases.length);
      return {
        cases,
        totalRecords,
        offset: 0,
        limit: cases.length,
      };
    }
    const pageData = pageQuery.data;
    const rawCases = (pageData?.cases ?? []) as CaseListItem[];
    const cases = filterS0(rawCases);
    const totalRecords = excludeS0
      ? cases.length
      : (pageData?.totalRecords ?? 0);
    return { cases, totalRecords, offset, limit };
  }, [showAll, infiniteQuery.data, pageQuery.data, offset, limit, excludeS0]);

  const isFetchingCases = showAll
    ? isLoadingAll || infiniteQuery.isFetching || infiniteQuery.isFetchingNextPage
    : pageQuery.isFetching;
  const isError = showAll ? infiniteQuery.isError : pageQuery.isError;

  const handleFilterSearch = (newFilters: Record<string, any>) => {
    setFilters(newFilters);
    setPage(0);
    setShowAll(false);
    setIsLoadingAll(false);
  };

  const handleUpdateFilter = (field: string, value: any) => {
    setFilters((prev) => ({
      ...prev,
      [field]: value,
    }));
    setPage(0);
    setShowAll(false);
    setIsLoadingAll(false);
  };

  const handleRemoveFilter = (field: string) => {
    const newFilters = { ...filters };
    delete newFilters[field];
    setFilters(newFilters);
    setPage(0);
    setShowAll(false);
    setIsLoadingAll(false);
  };

  const handleClearFilters = () => {
    setFilters({});
    setPage(0);
    setIsLoadingAll(false);
  };

  const handleAllCases = () => {
    setShowAll(true);
    setIsLoadingAll(true);
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

  const handleCreateCase = useCallback(() => {
    if (getNoveraChatEnabled()) {
      navigate(`/${projectId}/support/chat/describe-issue`);
    } else {
      navigate(`/${projectId}/support/chat/create-case`, {
        state: { skipChat: true },
      });
    }
  }, [navigate, projectId]);

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
        onAllCases={handleAllCases}
        onCreateCase={handleCreateCase}
      />

      {/* Cases list */}
      <CasesList
        isLoading={isFetchingCases || isAuthLoading}
        isError={isError}
        data={paginatedData}
        page={page}
        rowsPerPage={rowsPerPage}
        onPageChange={handleChangePage}
        onRowsPerPageChange={handleChangeRowsPerPage}
        onCaseClick={(c) => navigate(`/${projectId}/support/cases/${c.id}`)}
        showPagination={!showAll}
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
