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

import { useParams, useLocation } from "react-router";
import { useModifierAwareNavigate } from "@hooks/useModifierAwareNavigate";
import { useState, useMemo, useEffect, type ChangeEvent } from "react";
import useGetProjectDetails from "@api/useGetProjectDetails";
import useGetProjectFeatures from "@api/useGetProjectFeatures";
import useGetProjectFilters from "@api/useGetProjectFilters";
import useGetProjectCases from "@api/useGetProjectCases";
import { useGetProjectCasesStats } from "@features/dashboard/api/useGetProjectCasesStats";
import { useLoader } from "@context/linear-loader/LoaderContext";
import { getProjectSeverityPolicy } from "@utils/permission";
import { hasListSearchOrFilters } from "@features/support/utils/support";
import { normalizeEngagementLabel } from "@features/dashboard/utils/dashboard";
import type { AllCasesFilterValues } from "@features/support/types/cases";
import {
  CaseStatus,
  CaseType,
} from "@features/support/constants/supportConstants";
import { SortOrder } from "@/types/common";
import { ENGAGEMENTS_PAGE_SIZE } from "@/features/engagements/constants/engagements";
import {
  EngagementsSortField,
  type EngagementsStatKey,
} from "@features/engagements/types/engagements";
import {
  buildEngagementSearchRequest,
  buildEngagementDetailPath,
  computeEngagementsCasesAreaLoading,
  computeEngagementsStatsLoading,
  computeEngagementsTotalItems,
  getEngagementsCurrentPageCases,
  parseEngagementsSortField,
} from "@features/engagements/utils/engagements";

/**
 * State, data fetching, and handlers for {@link EngagementsPage}.
 *
 * @returns Navigation, filters, pagination, loading flags, and case rows for the engagements list.
 */
export function useEngagementsPageState() {
  const navigate = useModifierAwareNavigate();
  const { projectId } = useParams<{ projectId: string }>();
  const location = useLocation();

  const initialEngagementTypeId = (
    location.state as { engagementTypeId?: string } | null
  )?.engagementTypeId;

  const initialEngagementTypeLabel = (
    location.state as { engagementTypeLabel?: string } | null
  )?.engagementTypeLabel;

  const initialEngagementTypeKeys = initialEngagementTypeId
    ? initialEngagementTypeId.split(",").map(Number).filter(Boolean)
    : undefined;

  const [searchTerm, setSearchTerm] = useState("");
  const [filters, setFilters] = useState<AllCasesFilterValues>(() => ({}));
  const [isFiltersOpen, setIsFiltersOpen] = useState(() =>
    hasListSearchOrFilters(searchTerm, filters),
  );
  const [sortField, setSortField] = useState<EngagementsSortField>(
    EngagementsSortField.CreatedOn,
  );
  const [sortOrder, setSortOrder] = useState<SortOrder>(SortOrder.DESC);
  const [page, setPage] = useState(1);
  const [rowsPerPage, setRowsPerPage] = useState(ENGAGEMENTS_PAGE_SIZE);
  const [fixedStatusIds, setFixedStatusIds] = useState<number[] | undefined>(
    undefined,
  );
  const [activeStatKey, setActiveStatKey] = useState<
    EngagementsStatKey | undefined
  >(undefined);

  const { data: project, isLoading: isProjectLoading } = useGetProjectDetails(
    projectId || "",
  );
  const { data: projectFeatures, isLoading: isProjectFeaturesLoading } =
    useGetProjectFeatures(projectId || "");
  const projectReady = !isProjectLoading && project !== undefined;
  const areFeaturePermissionsReady =
    projectReady && !isProjectFeaturesLoading && projectFeatures !== undefined;
  const severityPolicy = areFeaturePermissionsReady
    ? getProjectSeverityPolicy(project?.type?.label, { projectFeatures })
    : { excludeS0: false, restrictSeverityToLow: false };
  const { restrictSeverityToLow } = severityPolicy;

  const { data: filterMetadata } = useGetProjectFilters(projectId || "");

  const {
    data: stats,
    isLoading: isStatsQueryLoading,
    isError: isStatsError,
  } = useGetProjectCasesStats(projectId || "", {
    caseTypes: [CaseType.ENGAGEMENT],
    enabled: !!projectId,
  });

  const hasStatsResponse = stats !== undefined;
  const isStatsLoading = computeEngagementsStatsLoading(
    isStatsQueryLoading,
    hasStatsResponse,
    projectId,
  );

  const isChartNavigation = !!initialEngagementTypeId;

  // Outstanding (non-closed) status IDs — applied when navigating from the engagements chart.
  const chartNavStatusIds = useMemo(() => {
    if (!isChartNavigation || !filterMetadata?.caseStates) return undefined;
    const closedState = filterMetadata.caseStates.find(
      (s) => s.label === CaseStatus.CLOSED,
    );
    const closedId = closedState != null ? Number(closedState.id) : null;
    return filterMetadata.caseStates
      .map((s) => Number(s.id))
      .filter((id) => closedId === null || id !== closedId);
  }, [isChartNavigation, filterMetadata]);

  const chartNavEngagementLabel = useMemo(() => {
    if (!initialEngagementTypeId) return undefined;
    // Use label passed directly in navigation state (no loading wait).
    if (initialEngagementTypeLabel) return initialEngagementTypeLabel;
    // Fallback: derive from stats once loaded.
    if (!stats?.engagementTypeCount) return undefined;
    const ids = initialEngagementTypeId.split(",");
    const found = stats.engagementTypeCount.find((t) => ids.includes(t.id));
    return found ? normalizeEngagementLabel(found.label) : undefined;
  }, [initialEngagementTypeId, initialEngagementTypeLabel, stats]);

  const engagementSearchRequest = useMemo(() => {
    const base = buildEngagementSearchRequest(
      filters,
      searchTerm,
      sortField,
      sortOrder,
    );
    const withEngagementType = initialEngagementTypeKeys
      ? {
          ...base,
          filters: {
            ...base.filters,
            engagementTypeKeys: initialEngagementTypeKeys,
          },
        }
      : base;
    // Apply outstanding filter for chart navigation (non-closed states).
    const withChartStatus =
      isChartNavigation && chartNavStatusIds
        ? {
            ...withEngagementType,
            filters: {
              ...withEngagementType.filters,
              statusIds: chartNavStatusIds,
            },
          }
        : withEngagementType;
    if (fixedStatusIds !== undefined) {
      return {
        ...withChartStatus,
        filters: {
          ...withChartStatus.filters,
          statusIds: fixedStatusIds.length > 0 ? fixedStatusIds : undefined,
        },
      };
    }
    return withChartStatus;
  }, [
    filters,
    searchTerm,
    sortField,
    sortOrder,
    fixedStatusIds,
    initialEngagementTypeKeys,
    isChartNavigation,
    chartNavStatusIds,
  ]);

  const {
    data,
    isLoading: isCasesQueryLoading,
    isError: isCasesError,
    hasNextPage,
    fetchNextPage,
    isFetchingNextPage,
  } = useGetProjectCases(projectId || "", engagementSearchRequest, {
    enabled: !!projectId,
    pageSize: rowsPerPage,
  });

  const { showLoader, hideLoader } = useLoader();

  const hasCasesResponse = data !== undefined;
  const isCasesAreaLoading = computeEngagementsCasesAreaLoading(
    isCasesQueryLoading,
    hasCasesResponse,
    projectId,
  );

  const isInitialPageLoading = isCasesAreaLoading;

  useEffect(() => {
    if (isInitialPageLoading) {
      showLoader();
      return () => hideLoader();
    }
    hideLoader();
  }, [isInitialPageLoading, showLoader, hideLoader]);

  useEffect(() => {
    if (!data) return;
    const loadedPages = data.pages.length;
    if (page > loadedPages && hasNextPage && !isFetchingNextPage) {
      void fetchNextPage();
    }
  }, [page, data, hasNextPage, isFetchingNextPage, fetchNextPage]);

  const currentPageCases = useMemo(
    () => getEngagementsCurrentPageCases(data, page),
    [data, page],
  );

  const apiTotalRecords = data?.pages?.[0]?.totalRecords ?? 0;

  const totalItems = computeEngagementsTotalItems(
    apiTotalRecords,
    currentPageCases.length,
  );
  const paginatedCases = currentPageCases;

  const handlePageChange = (_e: ChangeEvent<unknown>, value: number) => {
    setPage(value);
  };

  const handleRowsPerPageChange = (newSize: number) => {
    setRowsPerPage(newSize);
    setPage(1);
  };

  const handleFilterChange = (field: string, value: string) => {
    setFilters((prev) => ({
      ...prev,
      [field]: value || undefined,
    }));
    setPage(1);
  };

  const handleClearFilters = () => {
    setFilters({});
    setSearchTerm("");
    setFixedStatusIds(undefined);
    setActiveStatKey(undefined);
    setPage(1);
  };

  const handleStatCardClick = (key: EngagementsStatKey) => {
    if (!filterMetadata?.caseStates) return;
    const getStateId = (label: string): number | null => {
      const s = filterMetadata.caseStates!.find((st) => st.label === label);
      return s != null ? Number(s.id) : null;
    };
    let ids: number[] | undefined;
    switch (key) {
      case "active": {
        const closedId = getStateId(CaseStatus.CLOSED);
        ids = filterMetadata.caseStates
          .map((s) => Number(s.id))
          .filter((id) => id !== closedId);
        break;
      }
      case "completed": {
        const id = getStateId(CaseStatus.CLOSED);
        ids = id != null ? [id] : undefined;
        break;
      }
      case "onHold": {
        const awaitingId = getStateId(CaseStatus.AWAITING_INFO);
        const waitingId = getStateId(CaseStatus.WAITING_ON_WSO2);
        ids = [awaitingId, waitingId].filter((id): id is number => id != null);
        break;
      }
      default:
        ids = undefined;
    }
    setFixedStatusIds(ids);
    setActiveStatKey(key);
    setFilters((prev) => ({ ...prev, statusId: undefined }));
    setPage(1);
  };

  const handleSortChange = (value: SortOrder) => {
    setSortOrder(value);
    setPage(1);
  };

  const handleSortFieldUiChange = (value: string) => {
    setSortField(parseEngagementsSortField(value));
    setPage(1);
  };

  const handleSearchChange = (value: string) => {
    setSearchTerm(value);
    setPage(1);
  };

  const listHasRefinement = hasListSearchOrFilters(searchTerm, {
    ...filters,
    severityId: undefined,
  });

  const onCaseClick =
    projectId !== undefined
      ? (caseItem: { id: string }) =>
          navigate(buildEngagementDetailPath(projectId, caseItem.id))
      : undefined;

  const engagementTypeOptions = useMemo(() => {
    if (!stats?.engagementTypeCount) return [];
    const DISPLAY_NAMES = [
      "Consultancy",
      "Onboarding",
      "Migration",
      "Follow Up",
    ];
    const DISPLAY_BY_LOWER = new Map(
      DISPLAY_NAMES.map((n) => [n.toLowerCase(), n]),
    );
    const grouped = new Map<string, string[]>();
    for (const t of stats.engagementTypeCount) {
      const displayName = DISPLAY_BY_LOWER.get(t.label.toLowerCase());
      if (!displayName) continue;
      if (!grouped.has(displayName)) grouped.set(displayName, []);
      grouped.get(displayName)!.push(t.id);
    }
    return DISPLAY_NAMES.filter((name) => grouped.has(name)).map((name) => ({
      value: grouped.get(name)!.join(","),
      label: name,
    }));
  }, [stats]);

  return {
    projectId,
    projectReady,
    restrictSeverityToLow,
    filterMetadata,
    stats,
    isStatsLoading,
    isStatsError,
    searchTerm,
    isFiltersOpen,
    setIsFiltersOpen,
    filters,
    sortField,
    sortOrder,
    page,
    rowsPerPage,
    paginatedCases,
    isCasesAreaLoading,
    isCasesError,
    listHasRefinement,
    totalItems,
    handlePageChange,
    handleRowsPerPageChange,
    handleFilterChange,
    handleClearFilters,
    handleSortChange,
    handleSortFieldUiChange,
    handleSearchChange,
    handleStatCardClick,
    isStatFiltered: fixedStatusIds !== undefined,
    activeStatKey,
    clearStatFilter: () => {
      setFixedStatusIds(undefined);
      setActiveStatKey(undefined);
      setPage(1);
    },
    onCaseClick,
    isChartNavigation,
    chartNavEngagementLabel,
    engagementTypeOptions,
  };
}
