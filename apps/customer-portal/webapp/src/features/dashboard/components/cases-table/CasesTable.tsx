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
import {
  type JSX,
  useState,
  useMemo,
  useEffect,
  type ChangeEvent,
} from "react";
import { useNavigate } from "react-router";
import { useAsgardeo } from "@asgardeo/react";
import useGetProjectCases from "@api/useGetProjectCases";
import { useGetProjectCasesPage } from "@api/useGetProjectCasesPage";
import useGetProjectFilters from "@api/useGetProjectFilters";
import { usePostProjectDeploymentsSearchInfinite } from "@api/usePostProjectDeploymentsSearch";
import CasesTableHeader from "@features/dashboard/components/cases-table/CasesTableHeader";
import CasesFilters from "@features/dashboard/components/cases-table/CasesFilters";
import CasesList from "@features/dashboard/components/cases-table/CasesList";
import {
  countCasesTableActiveFilters,
  filterCasesTableMetadataOptions,
  mapCasesTableFilterOptionLabel,
  navigateToProjectCaseDetail,
  resolveCasesTableDefaultStatusIds,
  resolveCasesTableSearchStatusIds,
} from "@features/dashboard/utils/casesTable";
import { isS0Case, deriveFilterLabels } from "@features/support/utils/support";
import {
  CaseType,
  ALL_CASES_FILTER_DEFINITIONS,
} from "@features/support/constants/supportConstants";
import type {
  CaseListItem,
  CaseSearchResponse,
} from "@features/support/types/cases";
import { SortOrder } from "@/types/common";
import type {
  CasesTableFilterValues,
  CasesTableProps,
  TableFilter,
} from "@/features/dashboard/types/casesTable";

/**
 * Dashboard outstanding cases listing with filters, pagination, and case navigation.
 *
 * @returns {JSX.Element} Cases table container
 */
const CasesTable = ({
  projectId,
  excludeS0 = false,
  restrictSeverityToLow = false,
  hasAgent = false,
  includeDeploymentFilter = true,
}: CasesTableProps): JSX.Element => {
  // navigate function
  const navigate = useNavigate();
  // asgardeo loading
  const { isLoading: isAuthLoading } = useAsgardeo();
  // filters
  const [filters, setFilters] = useState<CasesTableFilterValues>({});
  // filter open
  const [isFilterOpen, setIsFilterOpen] = useState(false);
  // page
  const [page, setPage] = useState(0);
  // rows per page
  const [rowsPerPage, setRowsPerPage] = useState(5);
  // show all
  const [showAll, setShowAll] = useState(false);
  // loading all
  const [isLoadingAll, setIsLoadingAll] = useState(false);

  const { data: filtersMetadata } = useGetProjectFilters(projectId);
  // deployments query
  const deploymentsQuery = usePostProjectDeploymentsSearchInfinite(projectId, {
    pageSize: 10,
    enabled: !!projectId,
  });

  // deployments list
  const deploymentsList = useMemo(
    () =>
      deploymentsQuery.data?.pages.flatMap((p) => p.deployments ?? []) ?? [],
    [deploymentsQuery.data],
  );

  // effective filters
  const effectiveFilters: CasesTableFilterValues = useMemo(
    () =>
      includeDeploymentFilter
        ? filters
        : { ...filters, deploymentId: undefined },
    [filters, includeDeploymentFilter],
  );

  // dynamic filter fields
  const dynamicFilterFields: Array<TableFilter> = useMemo(() => {
    return ALL_CASES_FILTER_DEFINITIONS.filter(
      (def) =>
        def.id !== "caseType" &&
        !(restrictSeverityToLow && def.metadataKey === "severities") &&
        (includeDeploymentFilter || def.id !== "deployment"),
    ).map((def) => {
      const { label } = deriveFilterLabels(def.id);

      const isDeploymentFilter = def.id === "deployment";
      let options: { label: string; value: string }[];
      switch (isDeploymentFilter) {
        case true:
          options =
            deploymentsList.map((deployment) => ({
              label: deployment.type?.label || deployment.name,
              value: deployment.id,
            })) ?? [];
          break;
        default: {
          const metadataOptions =
            filtersMetadata?.[def.metadataKey as keyof typeof filtersMetadata];
          const filtered = filterCasesTableMetadataOptions(
            def.metadataKey,
            metadataOptions,
            excludeS0,
            restrictSeverityToLow,
          );
          options = filtered.map((item) => ({
            label: mapCasesTableFilterOptionLabel(
              def.metadataKey,
              item.label,
            ),
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
  }, [
    filtersMetadata,
    deploymentsList,
    excludeS0,
    restrictSeverityToLow,
    includeDeploymentFilter,
    deploymentsQuery,
  ]);

  // case search request
  const caseSearchRequest = useMemo(() => {
    const defaultStatusIds = resolveCasesTableDefaultStatusIds(
      filtersMetadata?.caseStates,
    );

    return {
      filters: {
        statusIds: resolveCasesTableSearchStatusIds(
          effectiveFilters.statusId,
          defaultStatusIds,
        ),
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
  }, [effectiveFilters, filtersMetadata]);

  // offset
  const offset = page * rowsPerPage;
  // limit
  const limit = rowsPerPage;

  // page query
  const pageQuery = useGetProjectCasesPage(
    projectId,
    caseSearchRequest,
    offset,
    limit,
    { enabled: !showAll },
  );

  // infinite query
  const infiniteQuery = useGetProjectCases(projectId, caseSearchRequest, {
    enabled: showAll,
  });

  // fetch next page
  useEffect(() => {
    if (!showAll) return;
    if (!infiniteQuery.hasNextPage) return;
    if (infiniteQuery.isFetchingNextPage) return;
    void infiniteQuery.fetchNextPage();
  }, [showAll, infiniteQuery]);

  // paginated data
  const paginatedData = useMemo((): CaseSearchResponse => {
    const filterS0 = (items: CaseListItem[]): CaseListItem[] =>
      excludeS0 ? items.filter((c) => !isS0Case(c)) : items;

    if (showAll) {
      if (!infiniteQuery.data) {
        return { cases: [], totalRecords: 0, offset: 0, limit: 0 };
      }
      const rawCases = infiniteQuery.data.pages.flatMap((p) => p.cases ?? []);
      const cases = filterS0(rawCases);
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
    const totalRecords = pageData?.totalRecords ?? 0;
    return { cases, totalRecords, offset, limit };
  }, [showAll, infiniteQuery.data, pageQuery.data, offset, limit, excludeS0]);

  // fetching cases
  const isFetchingCases = showAll
    ? isLoadingAll ||
      infiniteQuery.isLoading ||
      infiniteQuery.isFetchingNextPage
    : pageQuery.isLoading;

  // error
  const isError = showAll ? infiniteQuery.isError : pageQuery.isError;

  // handle clear filters
  const handleClearFilters = () => {
    setFilters({});
    setPage(0);
    setIsLoadingAll(false);
  };

  // handle update filter
  const handleUpdateFilter = (field: string, value: string | number) => {
    setFilters((prev) => ({
      ...prev,
      [field]: value,
    }));
    setPage(0);
    setShowAll(false);
    setIsLoadingAll(false);
  };

  // handle change page
  const handleChangePage = (_event: unknown, newPage: number) => {
    setPage(newPage);
  };

  // handle change rows per page
  const handleChangeRowsPerPage = (event: ChangeEvent<HTMLInputElement>) => {
    setRowsPerPage(parseInt(event.target.value, 10));
    setPage(0);
  };

  // active filters count
  const activeFiltersCount = countCasesTableActiveFilters(filters);

  return (
    <ListingTable.Container sx={{ width: "100%", mb: 4, p: 3 }}>
      <CasesTableHeader
        activeFiltersCount={activeFiltersCount}
        isFiltersOpen={isFilterOpen}
        onFilterToggle={() => {
          if (activeFiltersCount > 0) {
            handleClearFilters();
          } else {
            setIsFilterOpen(!isFilterOpen);
          }
        }}
        hasAgent={hasAgent}
      />

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

      <CasesList
        isLoading={isFetchingCases || isAuthLoading}
        isError={isError}
        data={paginatedData}
        page={page}
        rowsPerPage={rowsPerPage}
        onPageChange={handleChangePage}
        onRowsPerPageChange={handleChangeRowsPerPage}
        onCaseClick={(c) =>
          navigateToProjectCaseDetail(navigate, projectId, c.id)
        }
        showPagination={!showAll}
        hasListRefinement={activeFiltersCount > 0}
      />
    </ListingTable.Container>
  );
};

export default CasesTable;
