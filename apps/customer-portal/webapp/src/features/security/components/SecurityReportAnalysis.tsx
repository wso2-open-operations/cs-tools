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

import { Box, Button, Paper, Typography } from "@wso2/oxygen-ui";
import { Plus } from "@wso2/oxygen-ui-icons-react";
import {
  useState,
  useMemo,
  useEffect,
  type JSX,
} from "react";
import { useNavigate, useParams } from "react-router";
import { useSessionState } from "@hooks/useSessionState";
import { CaseType } from "@features/support/constants/supportConstants";
import useGetProjectCases from "@api/useGetProjectCases";
import useGetProjectFilters from "@api/useGetProjectFilters";
import useGetProjectDetails from "@api/useGetProjectDetails";
import useGetProjectFeatures from "@api/useGetProjectFeatures";
import { SortOrder } from "@/types/common";
import SecurityReportAnalysisSkeleton from "@features/security/components/SecurityReportAnalysisSkeleton";
import TabBar from "@components/tab-bar/TabBar";
import ListSearchBar from "@components/list-view/ListSearchBar";
import ListFiltersPanel from "@components/list-view/ListFiltersPanel";
import ListResultsBar from "@components/list-view/ListResultsBar";
import ListPagination from "@components/list-view/ListPagination";
import ListCard from "@components/list-view/ListCard";
import { countListSearchAndFilters } from "@features/support/utils/support";
import type {
  AllCasesFilterValues,
  CaseListItem,
  CaseMetadataResponse,
} from "@features/support/types/cases";
import {
  SECURITY_REPORT_ANALYSIS_CREATE_BUTTON_LABEL,
  SECURITY_REPORT_ANALYSIS_EMPTY_MESSAGE,
  SECURITY_REPORT_ANALYSIS_PAGE_SIZE,
  SECURITY_REPORT_ANALYSIS_SUBTITLE,
  SECURITY_REPORT_ANALYSIS_TITLE,
  SECURITY_REPORT_ENTITY_LABEL,
  SECURITY_REPORT_FILTER_DEFINITIONS,
  SECURITY_REPORT_SEARCH_PLACEHOLDER,
  SECURITY_REPORT_SORT_OPTIONS,
  SECURITY_REPORT_VIEW_TABS,
} from "@features/security/constants/securityConstants";
import {
  SecurityReportCaseSortField,
  SecurityReportViewMode,
} from "@features/security/types/security";
import {
  parseSecurityReportCaseSortField,
  parseSecurityReportViewMode,
} from "@features/security/utils/securityPage";
import { getProjectPermissions, isProjectRestricted } from "@utils/permission";

type SecurityReportAnalysisProps = {
  fixedStatusIds?: number[];
};

/**
 * SecurityReportAnalysis displays security vulnerability reports uploaded for analysis.
 *
 * @returns {JSX.Element}
 */
const SecurityReportAnalysis = ({ fixedStatusIds }: SecurityReportAnalysisProps): JSX.Element => {
  const navigate = useNavigate();
  const { projectId } = useParams<{ projectId: string }>();
  const { data: projectDetails, isLoading: isProjectLoading } =
    useGetProjectDetails(projectId || "");
  const { data: projectFeatures, isLoading: isProjectFeaturesLoading } =
    useGetProjectFeatures(projectId || "");
  const areFeaturePermissionsReady =
    !isProjectLoading && !isProjectFeaturesLoading && !!projectFeatures;
  const isSecurityReportAvailable =
    areFeaturePermissionsReady &&
    getProjectPermissions(projectDetails?.type?.label, { projectFeatures })
      .hasSecurityReportAnalysis;
  const canCreateSecurityReport =
    isSecurityReportAvailable &&
    !isProjectRestricted(projectDetails?.closureState);

  const [viewMode, setViewMode] = useState<SecurityReportViewMode>(
    SecurityReportViewMode.ALL,
  );
  const sessionPrefix = `${projectId ?? "unknown"}-security-reports`;
  const validSecuritySortFields = SECURITY_REPORT_SORT_OPTIONS.map((o) => o.value as string);
  const isValidSecuritySortField = (v: unknown): v is SecurityReportCaseSortField =>
    typeof v === "string" && validSecuritySortFields.includes(v);
  const [searchTerm, setSearchTerm] = useSessionState(`${sessionPrefix}-search`, "", undefined, { popOnly: true });
  const [isFiltersOpen, setIsFiltersOpen] = useState(false);
  const [filters, setFilters] = useSessionState<AllCasesFilterValues>(`${sessionPrefix}-filters`, {}, undefined, { popOnly: true });
  const [sortField, setSortField] = useSessionState<SecurityReportCaseSortField>(
    `${sessionPrefix}-sortField`,
    SecurityReportCaseSortField.createdOn,
    isValidSecuritySortField,
    { popOnly: true },
  );
  const [sortOrder, setSortOrder] = useSessionState<SortOrder>(`${sessionPrefix}-sortOrder`, SortOrder.DESC, undefined, { popOnly: true });
  const [page, setPage] = useSessionState<number>(`${sessionPrefix}-page`, 1, undefined, { popOnly: true });
  const pageSize = SECURITY_REPORT_ANALYSIS_PAGE_SIZE;

  const { data: filterMetadata } = useGetProjectFilters(projectId || "");

  const caseSearchRequest = useMemo(
    () => ({
      filters: {
        caseTypes: [CaseType.SECURITY_REPORT_ANALYSIS],
        createdByMe:
          viewMode === SecurityReportViewMode.MY ? true : undefined,
        statusIds: fixedStatusIds !== undefined
          ? (fixedStatusIds.length > 0 ? fixedStatusIds : undefined)
          : (filters.statusId ? [Number(filters.statusId)] : undefined),
        severityId: filters.severityId ? Number(filters.severityId) : undefined,
        issueId: filters.issueTypes ? Number(filters.issueTypes) : undefined,
        deploymentId: filters.deploymentId || undefined,
        searchQuery: searchTerm.trim() || undefined,
      },
      sortBy: {
        field: sortField,
        order: sortOrder,
      },
    }),
    [filters, searchTerm, sortField, sortOrder, viewMode, fixedStatusIds],
  );

  const { data, isLoading, hasNextPage, fetchNextPage } = useGetProjectCases(
    projectId || "",
    caseSearchRequest,
    {
      enabled:
        !!projectId && areFeaturePermissionsReady && isSecurityReportAvailable,
    },
  );

  useEffect(() => {
    if (!data || !hasNextPage) return;
    void fetchNextPage();
  }, [data, hasNextPage, fetchNextPage]);

  const displayedCases = useMemo(
    () => data?.pages.flatMap((p) => p.cases) ?? [],
    [data],
  );

  const totalItems = displayedCases.length;

  const paginatedCases = useMemo(() => {
    const startIndex = (page - 1) * pageSize;
    return displayedCases.slice(startIndex, startIndex + pageSize);
  }, [displayedCases, page, pageSize]);

  const handleCreateReport = () => {
    navigate(`/projects/${projectId}/support/security-report/create`);
  };

  const handleCaseClick = (caseItem: CaseListItem) => {
    navigate(
      `/projects/${projectId}/security-center/security-report-analysis/${caseItem.id}`,
    );
  };

  const handleSearchChange = (value: string) => {
    setSearchTerm(value);
    setPage(1);
  };

  const handleFilterChange = (field: string, value: string) => {
    setFilters((prev) => ({ ...prev, [field]: value || undefined }));
    setPage(1);
  };

  const handleClearFilters = () => {
    setFilters({});
    setSearchTerm("");
    setPage(1);
  };

  const handleSortChange = (value: SortOrder) => {
    setSortOrder(value);
    setPage(1);
  };

  const handleSortFieldChange = (value: string) => {
    setSortField(parseSecurityReportCaseSortField(value));
    setPage(1);
  };

  const handlePageChange = (_event: unknown, value: number) => {
    setPage(value);
  };

  const reportViewTabs = useMemo(() => [...SECURITY_REPORT_VIEW_TABS], []);

  const sortFieldOptions = useMemo(
    () => [...SECURITY_REPORT_SORT_OPTIONS],
    [],
  );

  if (areFeaturePermissionsReady && projectDetails && !isSecurityReportAvailable) {
    return (
      <Paper
        sx={{
          width: "100%",
          mb: 4,
          p: 3,
          display: "flex",
          flexDirection: "column",
          gap: 1,
        }}
      >
        <Typography variant="h5" color="text.primary">
          {SECURITY_REPORT_ANALYSIS_TITLE}
        </Typography>
        <Typography variant="body2" color="text.secondary">
          Security report analysis is not available for this project type.
        </Typography>
      </Paper>
    );
  }

  return (
    <Paper
      sx={{
        width: "100%",
        mb: 4,
        p: 3,
        display: "flex",
        flexDirection: "column",
        gap: 3,
      }}
    >
      <Box
        sx={{
          display: "flex",
          flexDirection: { xs: "column", sm: "row" },
          justifyContent: "space-between",
          alignItems: { xs: "flex-start", sm: "center" },
          gap: 2,
        }}
      >
        <Box>
          <Typography variant="h5" color="text.primary" sx={{ mb: 0.5 }}>
            {SECURITY_REPORT_ANALYSIS_TITLE}
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {SECURITY_REPORT_ANALYSIS_SUBTITLE}
          </Typography>
        </Box>
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            gap: 1.5,
            flexWrap: "wrap",
          }}
        >
          <TabBar
            tabs={reportViewTabs}
            activeTab={viewMode}
            onTabChange={(tabId) => {
              setViewMode(parseSecurityReportViewMode(tabId));
              setPage(1);
            }}
            sx={{ mb: 0, height: 32 }}
          />
          {canCreateSecurityReport && (
            <Button
              variant="contained"
              color="warning"
              startIcon={<Plus size={16} />}
              onClick={handleCreateReport}
              size="small"
            >
              {SECURITY_REPORT_ANALYSIS_CREATE_BUTTON_LABEL}
            </Button>
          )}
        </Box>
      </Box>

      <ListSearchBar
        searchPlaceholder={SECURITY_REPORT_SEARCH_PLACEHOLDER}
        searchTerm={searchTerm}
        onSearchChange={handleSearchChange}
        isFiltersOpen={isFiltersOpen}
        onFiltersToggle={() => setIsFiltersOpen(!isFiltersOpen)}
        activeFiltersCount={countListSearchAndFilters(searchTerm, filters)}
        onClearFilters={handleClearFilters}
        hideFiltersButton={fixedStatusIds !== undefined}
        filtersContent={
          <ListFiltersPanel
            filterDefinitions={SECURITY_REPORT_FILTER_DEFINITIONS}
            filters={filters}
            resolveOptions={(def) => {
              const raw = (
                filterMetadata as CaseMetadataResponse | undefined
              )?.[def.metadataKey as keyof CaseMetadataResponse];
              if (!Array.isArray(raw)) return [];
              return raw.map((item: { label: string; id: string }) => ({
                label: item.label,
                value: item.id,
              }));
            }}
            onFilterChange={handleFilterChange}
          />
        }
      />

      <ListResultsBar
        shownCount={paginatedCases.length}
        totalCount={totalItems}
        entityLabel={SECURITY_REPORT_ENTITY_LABEL}
        sortFieldOptions={sortFieldOptions}
        sortField={sortField}
        onSortFieldChange={handleSortFieldChange}
        sortOrder={sortOrder}
        onSortOrderChange={handleSortChange}
      />

      <Box>
        {isLoading ? (
          <SecurityReportAnalysisSkeleton />
        ) : paginatedCases.length === 0 ? (
          <Box sx={{ textAlign: "center", py: 6 }}>
            <Typography variant="body1" color="text.secondary">
              {SECURITY_REPORT_ANALYSIS_EMPTY_MESSAGE}
            </Typography>
          </Box>
        ) : (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
            {paginatedCases.map((caseItem) => (
              <ListCard
                key={caseItem.id}
                caseItem={caseItem}
                onClick={handleCaseClick}
              />
            ))}
          </Box>
        )}
      </Box>

      <ListPagination
        totalRecords={totalItems}
        page={page}
        rowsPerPage={pageSize}
        onPageChange={handlePageChange}
        onRowsPerPageChange={() => undefined}
        rowsPerPageOptions={[pageSize]}
      />
    </Paper>
  );
};

export default SecurityReportAnalysis;
