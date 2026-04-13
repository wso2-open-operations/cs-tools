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
import { useNavigate } from "react-router";
import { useAsgardeo } from "@asgardeo/react";
import useGetProjectCases from "@api/useGetProjectCases";
import { useGetProjectCasesPage } from "@api/useGetProjectCasesPage";
import useGetProjectFilters from "@api/useGetProjectFilters";
import { usePostProjectDeploymentsSearchInfinite } from "@api/usePostProjectDeploymentsSearch";
import type { FilterField } from "@components/common/filter-panel/FilterPopover";
import CasesTableHeader from "@components/dashboard/cases-table/CasesTableHeader";
import CasesFilters from "@components/dashboard/cases-table/CasesFilters";
import CasesList from "@components/dashboard/cases-table/CasesList";
import { mapSeverityToDisplay, isS0Case, deriveFilterLabels } from "@utils/support";
import { isS0SeverityLabel } from "@constants/dashboardConstants";
import { CaseType, ALL_CASES_FILTER_DEFINITIONS } from "@constants/supportConstants";
import type { CaseListItem, CaseSearchResponse } from "@/types/cases";
import { SortOrder } from "@/types/common";

const OUTSTANDING_STATUS_IDS = [1, 10, 18, 1003, 1006] as const;

const isClosedStatus = (label?: string): boolean =>
  label?.trim().toLowerCase() === "closed";

interface CasesTableProps {
  projectId: string;
  excludeS0?: boolean;
  hasAgent?: boolean;
  includeDeploymentFilter?: boolean;
}

interface CasesTableFilterValues {
  [key: string]: string | number | undefined;
  statusId?: string | number;
  severityId?: string | number;
  issueTypes?: string | number;
  deploymentId?: string | number;
}

const CasesTable = ({
  projectId,
  excludeS0 = false,
  hasAgent = false,
  includeDeploymentFilter = true,
}: CasesTableProps): JSX.Element => {
  const navigate = useNavigate();
  const { isLoading: isAuthLoading } = useAsgardeo();
  const [filters, setFilters] = useState<CasesTableFilterValues>({});
  const [isFilterOpen, setIsFilterOpen] = useState(false);
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(5);
  const [showAll, setShowAll] = useState(false);
  const [isLoadingAll, setIsLoadingAll] = useState(false);

  const {
    data: filtersMetadata,
  } = useGetProjectFilters(projectId);

  // Fetch deployments for the deployment filter
  const deploymentsQuery = usePostProjectDeploymentsSearchInfinite(projectId, {
    pageSize: 10,
    enabled: !!projectId,
  });
  const deploymentsList = useMemo(
    () => deploymentsQuery.data?.pages.flatMap((p) => p.deployments ?? []) ?? [],
    [deploymentsQuery.data],
  );

  const effectiveFilters: CasesTableFilterValues = useMemo(
    () =>
      includeDeploymentFilter ? filters : { ...filters, deploymentId: undefined },
    [filters, includeDeploymentFilter],
  );

  const dynamicFilterFields: Array<FilterField & { type: "select" }> = useMemo(() => {
    return ALL_CASES_FILTER_DEFINITIONS
      .filter(
        (def) =>
          def.id !== "caseType" &&
          (includeDeploymentFilter || def.id !== "deployment"),
      )
      .map((def) => {
      const { label } = deriveFilterLabels(def.id);

      const isDeploymentFilter = def.id === "deployment";
      let options: { label: string; value: string }[];
      if (isDeploymentFilter) {
        options =
          deploymentsList.map((deployment) => ({
            label: deployment.type?.label || deployment.name,
            value: deployment.id,
          })) ?? [];
      } else {
        const metadataOptions = filtersMetadata?.[def.metadataKey as keyof typeof filtersMetadata];
        if (!Array.isArray(metadataOptions)) {
          options = [];
        } else {
          const filtered: Array<{ label: string; id: string }> =
            def.metadataKey === "severities" && excludeS0
              ? metadataOptions.filter(
                  (item: { label: string }) =>
                    !isS0SeverityLabel(item.label),
                )
              : def.metadataKey === "caseStates"
                ? (metadataOptions as Array<{ label: string; id: string }>).filter(
                    (s) => !isClosedStatus(s.label),
                  )
                : metadataOptions;
          options = filtered.map((item) => ({
            label:
              def.metadataKey === "severities"
                ? mapSeverityToDisplay(item.label)
                : item.label,
            value: item.id,
          }));
        }
      }

        return {
          id: def.filterKey,
          label,
          type: "select" as const,
          options,
          ...(isDeploymentFilter
            ? {
                onLoadMore: () => {
                  if (
                    deploymentsQuery.hasNextPage &&
                    !deploymentsQuery.isFetchingNextPage
                  ) {
                    void deploymentsQuery.fetchNextPage();
                  }
                },
                hasMore: !!deploymentsQuery.hasNextPage,
                isFetchingMore: deploymentsQuery.isFetchingNextPage,
              }
            : null),
        };
      });
  }, [filtersMetadata, deploymentsList, excludeS0, includeDeploymentFilter, deploymentsQuery]);

  const caseSearchRequest = useMemo(
    () => {
      // If no status filter is applied, use all statuses except Closed (id: 3)
      const defaultStatusIds = filtersMetadata?.caseStates
        ?.filter((status) => !isClosedStatus(status.label))
        .map((status) => Number(status.id)) || [];
      
      return {
        filters: {
          statusIds: effectiveFilters.statusId
            ? [Number(effectiveFilters.statusId)]
            : defaultStatusIds.length > 0 
              ? defaultStatusIds 
              : [...OUTSTANDING_STATUS_IDS],
          caseTypes: [CaseType.DEFAULT_CASE],
          severityId: effectiveFilters.severityId
            ? Number(effectiveFilters.severityId)
            : undefined,
          issueId: effectiveFilters.issueTypes
            ? Number(effectiveFilters.issueTypes)
            : undefined,
          deploymentId: effectiveFilters.deploymentId
            ? String(effectiveFilters.deploymentId)
            : undefined,
        },
        sortBy: {
          field: "createdOn",
          order: SortOrder.DESC,
        },
      };
    },
    [effectiveFilters, filtersMetadata],
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
    if (!showAll) return;
    if (!infiniteQuery.hasNextPage) return;
    if (infiniteQuery.isFetchingNextPage) return;
    void infiniteQuery.fetchNextPage();
  }, [showAll, infiniteQuery]);

  const paginatedData = useMemo((): CaseSearchResponse => {
    const filterS0 = (items: CaseListItem[]): CaseListItem[] =>
      excludeS0 ? items.filter((c) => !isS0Case(c)) : items;

    if (showAll) {
      if (!infiniteQuery.data) {
        return { cases: [], totalRecords: 0, offset: 0, limit: 0 };
      }
      const rawCases = infiniteQuery.data.pages.flatMap((p) => p.cases ?? []);
      const cases = filterS0(rawCases);
      // For showAll mode, use the original total records from API
      const totalRecords = infiniteQuery.data.pages[0]?.totalRecords ?? 0;
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
    // For paginated mode, use the original total records from API
    const totalRecords = pageData?.totalRecords ?? 0;
    return { cases, totalRecords, offset, limit };
  }, [showAll, infiniteQuery.data, pageQuery.data, offset, limit, excludeS0]);

  const isFetchingCases = showAll
    ? isLoadingAll || infiniteQuery.isFetching || infiniteQuery.isFetchingNextPage
    : pageQuery.isFetching;
  const isError = showAll ? infiniteQuery.isError : pageQuery.isError;

  const handleClearFilters = () => {
    setFilters({});
    setPage(0);
    setIsLoadingAll(false);
  };

  const handleUpdateFilter = (field: string, value: string | number) => {
    setFilters((prev) => ({
      ...prev,
      [field]: value,
    }));
    setPage(0);
    setShowAll(false);
    setIsLoadingAll(false);
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

  const getActiveFiltersCount = () => {
    return Object.values(filters).filter((v) => v !== "" && v != null).length;
  };

  return (
    <ListingTable.Container sx={{ width: "100%", mb: 4, p: 3 }}>
      {/* Header */}
      <CasesTableHeader
        activeFiltersCount={getActiveFiltersCount()}
        isFiltersOpen={isFilterOpen}
        onFilterToggle={() => {
          if (getActiveFiltersCount() > 0) {
            handleClearFilters();
          } else {
            setIsFilterOpen(!isFilterOpen);
          }
        }}
        hasAgent={hasAgent}
      />

      {/* Filter dropdowns section */}
      {isFilterOpen && (
        <>
          <Divider sx={{ my: 2 }} />
          <Box sx={{ mb: 3 }}>
            <CasesFilters
              filters={filters}
              filterFields={dynamicFilterFields}
              onFilterChange={handleUpdateFilter}
            />
          </Box>
        </>
      )}

      {/* Cases list */}
      <CasesList
        isLoading={isFetchingCases || isAuthLoading}
        isError={isError}
        data={paginatedData}
        page={page}
        rowsPerPage={rowsPerPage}
        onPageChange={handleChangePage}
        onRowsPerPageChange={handleChangeRowsPerPage}
        onCaseClick={(c) => navigate(`/projects/${projectId}/support/cases/${c.id}`)}
        showPagination={!showAll}
        hasListRefinement={getActiveFiltersCount() > 0}
      />
    </ListingTable.Container>
  );
};

export default CasesTable;
