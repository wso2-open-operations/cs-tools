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

import { useParams, useNavigate } from "react-router";
import { useState, useMemo, type JSX, type ChangeEvent } from "react";
import {
  Box,
  Button,
  Stack,
  Select,
  MenuItem,
  Typography,
  FormControl,
  InputLabel,
  Pagination,
} from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import { useGetProjectCasesStats } from "@api/useGetProjectCasesStats";
import useGetCasesFilters from "@api/useGetCasesFilters";
import useGetProjectCases from "@api/useGetProjectCases";
import type { AllCasesFilterValues } from "@models/responses";
import AllCasesStatCards from "@components/support/all-cases/AllCasesStatCards";
import AllCasesSearchBar from "@components/support/all-cases/AllCasesSearchBar";
import AllCasesList from "@components/support/all-cases/AllCasesList";

/**
 * AllCasesPage component to display all cases with stats, filters, and search.
 *
 * @returns {JSX.Element} The rendered All Cases page.
 */
export default function AllCasesPage(): JSX.Element {
  const navigate = useNavigate();
  const { projectId } = useParams<{ projectId: string }>();

  const [searchTerm, setSearchTerm] = useState("");
  const [isFiltersOpen, setIsFiltersOpen] = useState(false);
  const [filters, setFilters] = useState<AllCasesFilterValues>({});
  const [sortOrder, setSortOrder] = useState<"desc" | "asc">("desc");
  const [page, setPage] = useState(1);
  const pageSize = 10;

  // Fetch stats
  const {
    data: stats,
    isFetching: isStatsLoading,
    isError: isStatsError,
  } = useGetProjectCasesStats(projectId || "");

  // Fetch filter metadata
  const { data: filterMetadata } = useGetCasesFilters(projectId || "");

  // Fetch all cases
  const { data: casesData, isFetching: isCasesLoading } = useGetProjectCases(
    projectId || "",
    {
      pagination: { offset: 0, limit: 1000 },
      sortBy: { field: "createdOn", order: "desc" },
    },
  );

  const allCases = casesData?.cases ?? [];

  // Frontend filtering and search
  const filteredAndSearchedCases = useMemo(() => {
    let filtered = [...allCases];

    // Apply search filter
    if (searchTerm.trim()) {
      const searchLower = searchTerm.toLowerCase();
      filtered = filtered.filter(
        (caseItem) =>
          caseItem.number?.toLowerCase().includes(searchLower) ||
          caseItem.title?.toLowerCase().includes(searchLower) ||
          caseItem.description?.toLowerCase().includes(searchLower),
      );
    }

    // Apply status filter
    if (filters.statusId) {
      filtered = filtered.filter(
        (caseItem) => caseItem.status?.id === filters.statusId,
      );
    }

    // Apply severity filter
    if (filters.severityId) {
      filtered = filtered.filter(
        (caseItem) => caseItem.severity?.id === filters.severityId,
      );
    }

    // Apply category/case type filter
    if (filters.caseTypes) {
      filtered = filtered.filter(
        (caseItem) => caseItem.type?.id === filters.caseTypes,
      );
    }

    // Apply deployment filter
    if (filters.deploymentId) {
      filtered = filtered.filter(
        (caseItem) => caseItem.deployment?.id === filters.deploymentId,
      );
    }

    // Apply sorting
    filtered.sort((a, b) => {
      const dateA = new Date(a.createdOn).getTime() || 0;
      const dateB = new Date(b.createdOn).getTime() || 0;
      return sortOrder === "desc" ? dateB - dateA : dateA - dateB;
    });

    return filtered;
  }, [allCases, searchTerm, filters, sortOrder]);

  // Pagination logic
  const paginatedCases = useMemo(() => {
    const startIndex = (page - 1) * pageSize;
    return filteredAndSearchedCases.slice(startIndex, startIndex + pageSize);
  }, [filteredAndSearchedCases, page]);

  const totalPages = Math.ceil(filteredAndSearchedCases.length / pageSize);

  const handlePageChange = (_event: ChangeEvent<unknown>, value: number) => {
    setPage(value);
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
    setPage(1);
  };

  const handleSortChange = (value: "desc" | "asc") => {
    setSortOrder(value);
    setPage(1);
  };

  const handleSearchChange = (value: string) => {
    setSearchTerm(value);
    setPage(1);
  };

  return (
    <Stack spacing={3}>
      {/* Back button and header */}
      <Box>
        <Button
          startIcon={<ArrowLeft size={16} />}
          onClick={() => navigate("..")}
          sx={{ mb: 2 }}
          variant="text"
        >
          Back to Support Center
        </Button>
        <Box>
          <Typography variant="h4" color="text.primary" sx={{ mb: 1 }}>
            All Cases
          </Typography>
          <Typography variant="body2" color="text.secondary">
            Manage and track all your support cases
          </Typography>
        </Box>
      </Box>

      {/* Stat cards */}
      <AllCasesStatCards
        isLoading={isStatsLoading}
        isError={isStatsError}
        stats={stats}
      />

      <AllCasesSearchBar
        searchTerm={searchTerm}
        onSearchChange={handleSearchChange}
        isFiltersOpen={isFiltersOpen}
        onFiltersToggle={() => setIsFiltersOpen(!isFiltersOpen)}
        filters={filters}
        filterMetadata={filterMetadata}
        onFilterChange={handleFilterChange}
        onClearFilters={handleClearFilters}
      />

      {/* Sort and results count */}
      <Box
        sx={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
        }}
      >
        <Typography variant="body2" color="text.secondary">
          Showing {paginatedCases.length} of {filteredAndSearchedCases.length}{" "}
          cases
        </Typography>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <FormControl size="small" sx={{ minWidth: 180 }}>
            <InputLabel id="sort-label">Sort</InputLabel>
            <Select<"desc" | "asc">
              labelId="sort-label"
              id="sort"
              value={sortOrder}
              label="Sort"
              onChange={(e) =>
                handleSortChange(e.target.value as "desc" | "asc")
              }
            >
              <MenuItem value="desc">
                <Typography variant="body2">Newest First</Typography>
              </MenuItem>
              <MenuItem value="asc">
                <Typography variant="body2">Oldest First</Typography>
              </MenuItem>
            </Select>
          </FormControl>
        </Box>
      </Box>

      {/* Cases list */}
      <AllCasesList cases={paginatedCases} isLoading={isCasesLoading} />

      {/* Pagination */}
      {totalPages > 1 && (
        <Box sx={{ display: "flex", justifyContent: "flex-end", mt: 4 }}>
          <Pagination
            count={totalPages}
            page={page}
            onChange={handlePageChange}
            color="primary"
            variant="outlined"
            shape="rounded"
          />
        </Box>
      )}
    </Stack>
  );
}
