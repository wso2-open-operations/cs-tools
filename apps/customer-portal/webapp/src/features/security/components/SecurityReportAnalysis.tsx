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
  type ChangeEvent,
} from "react";
import { useNavigate, useParams } from "react-router";
import { CaseType } from "@features/support/constants/supportConstants";
import useGetProjectCases from "@api/useGetProjectCases";
import useGetProjectFilters from "@api/useGetProjectFilters";
import { SortOrder } from "@features/dashboard/types/common";
import SecurityReportAnalysisSkeleton from "@features/security/components/SecurityReportAnalysisSkeleton";
import TabBar from "@components/tab-bar/TabBar";
import ListSearchBar from "@components/list-view/ListSearchBar";
import ListFiltersPanel from "@components/list-view/ListFiltersPanel";
import ListResultsBar from "@components/list-view/ListResultsBar";
import ListPagination from "@components/list-view/ListPagination";
import ListCard from "@components/list-view/ListCard";
import { countListSearchAndFilters } from "@features/support/utils/support";
import type { AllCasesFilterValues, CaseListItem } from "@features/support/types/cases";
import type { CaseMetadataResponse } from "@features/support/types/cases";

const SECURITY_FILTER_DEFINITIONS = [
  {
    id: "status",
    filterKey: "statusId",
    metadataKey: "caseStates",
  },
];

/**
 * SecurityReportAnalysis displays security vulnerability reports uploaded for analysis.
 * @returns {JSX.Element}
 */
const SecurityReportAnalysis = (): JSX.Element => {
  const navigate = useNavigate();
  const { projectId } = useParams<{ projectId: string }>();

  const [viewMode, setViewMode] = useState<"my" | "all">("all");
  const [searchTerm, setSearchTerm] = useState("");
  const [isFiltersOpen, setIsFiltersOpen] = useState(false);
  const [filters, setFilters] = useState<AllCasesFilterValues>({});
  const [sortField, setSortField] = useState<
    "createdOn" | "updatedOn" | "severity" | "state"
  >("createdOn");
  const [sortOrder, setSortOrder] = useState<SortOrder>(SortOrder.DESC);
  const [page, setPage] = useState(1);
  const pageSize = 10;

  const { data: filterMetadata } = useGetProjectFilters(projectId || "");

  const caseSearchRequest = useMemo(
    () => ({
      filters: {
        caseTypes: [CaseType.SECURITY_REPORT_ANALYSIS],
        createdByMe: viewMode === "my" ? true : undefined,
        statusIds: filters.statusId ? [Number(filters.statusId)] : undefined,
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
    [filters, searchTerm, sortField, sortOrder, viewMode],
  );

  const { data, isLoading, hasNextPage, fetchNextPage } = useGetProjectCases(
    projectId || "",
    caseSearchRequest,
    { enabled: !!projectId },
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
  }, [displayedCases, page]);

  const totalPages = Math.ceil(totalItems / pageSize);

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

  const handleSortFieldChange = (
    value: "createdOn" | "updatedOn" | "severity" | "state",
  ) => {
    setSortField(value);
    setPage(1);
  };

  const handlePageChange = (_event: ChangeEvent<unknown>, value: number) => {
    setPage(value);
  };

  const reportViewTabs = useMemo(
    () => [
      { id: "my", label: "My Reports" },
      { id: "all", label: "All Reports" },
    ],
    [],
  );

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
      {/* Header */}
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
            Security Report Analysis
          </Typography>
          <Typography variant="body2" color="text.secondary">
            Security vulnerability reports uploaded for analysis
          </Typography>
        </Box>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap" }}>
          <TabBar
            tabs={reportViewTabs}
            activeTab={viewMode}
            onTabChange={(tabId) => {
              setViewMode(tabId as "my" | "all");
              setPage(1);
            }}
            sx={{ mb: 0, height: 32 }}
          />
          <Button
            variant="contained"
            color="warning"
            startIcon={<Plus size={16} />}
            onClick={handleCreateReport}
            size="small"
          >
            Create
          </Button>
        </Box>
      </Box>

      {/* Search + Filters */}
      <ListSearchBar
        searchPlaceholder="Search reports by case number, title, or description..."
        searchTerm={searchTerm}
        onSearchChange={handleSearchChange}
        isFiltersOpen={isFiltersOpen}
        onFiltersToggle={() => setIsFiltersOpen(!isFiltersOpen)}
        activeFiltersCount={countListSearchAndFilters(searchTerm, filters)}
        onClearFilters={handleClearFilters}
        filtersContent={
          <ListFiltersPanel
            filterDefinitions={SECURITY_FILTER_DEFINITIONS}
            filters={filters}
            resolveOptions={(def) => {
              const raw = (filterMetadata as CaseMetadataResponse | undefined)?.[
                def.metadataKey as keyof CaseMetadataResponse
              ];
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

      {/* Results count + sort */}
      <ListResultsBar
        shownCount={paginatedCases.length}
        totalCount={totalItems}
        entityLabel="reports"
        sortFieldOptions={[
          { value: "createdOn", label: "Created date" },
          { value: "updatedOn", label: "Updated date" },
          { value: "severity", label: "Severity" },
          { value: "state", label: "State" },
        ]}
        sortField={sortField}
        onSortFieldChange={(v) =>
          handleSortFieldChange(
            v as "createdOn" | "updatedOn" | "severity" | "state",
          )
        }
        sortOrder={sortOrder}
        onSortOrderChange={handleSortChange}
      />

      {/* Reports list */}
      <Box>
        {isLoading ? (
          <SecurityReportAnalysisSkeleton />
        ) : paginatedCases.length === 0 ? (
          <Box sx={{ textAlign: "center", py: 6 }}>
            <Typography variant="body1" color="text.secondary">
              No reports found.
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
        totalPages={totalPages}
        page={page}
        onChange={handlePageChange}
      />
    </Paper>
  );
};

export default SecurityReportAnalysis;
