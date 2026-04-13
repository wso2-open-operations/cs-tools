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
  Typography,
  FormControl,
  InputLabel,
  Select,
  MenuItem,
  Pagination,
} from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import useGetProjectFilters from "@api/useGetProjectFilters";
import { useGetProjectCasesPage } from "@api/useGetProjectCasesPage";
import { CaseType } from "@constants/supportConstants";
import type { AnnouncementFilterValues } from "@constants/supportConstants";
import AnnouncementsSearchBar from "@components/support/announcements/AnnouncementsSearchBar";
import AnnouncementList from "@components/support/announcements/AnnouncementList";
import { hasListSearchOrFilters } from "@utils/support";
import { SortOrder } from "@/types/common";
import AllCasesListSkeleton from "@components/support/all-cases/AllCasesListSkeleton";
import DOMPurify from "dompurify";

/**
 * AnnouncementsPage component to display announcements with stats, search, and filters (filter dropdowns disabled).
 *
 * @returns {JSX.Element} The rendered Announcements page.
 */
export default function AnnouncementsPage(): JSX.Element {
  const navigate = useNavigate();
  const { projectId } = useParams<{ projectId: string }>();

  const [searchTerm, setSearchTerm] = useState("");
  const [isFiltersOpen, setIsFiltersOpen] = useState(false);
  const [filters, setFilters] = useState<AnnouncementFilterValues>({});
  const [sortOrder, setSortOrder] = useState<SortOrder>(SortOrder.DESC);
  const [page, setPage] = useState(1);
  const pageSize = 10;

  const { data: filterMetadata } = useGetProjectFilters(projectId || "");

  const caseSearchRequest = useMemo(
    () => ({
      filters: {
        caseTypes: [CaseType.ANNOUNCEMENT],
        statusIds: filters.statusId ? [Number(filters.statusId)] : undefined,
        searchQuery: searchTerm.trim() || undefined,
      },
      sortBy: {
        field: "createdOn",
        order: sortOrder,
      },
    }),
    [filters, searchTerm, sortOrder],
  );

  const offset = (page - 1) * pageSize;

  const { data, isLoading: isCasesQueryLoading } = useGetProjectCasesPage(
    projectId || "",
    caseSearchRequest,
    offset,
    pageSize,
  );

  const cases = data?.cases ?? [];
  const totalRecords = data?.totalRecords ?? 0;
  const totalPages = Math.max(1, Math.ceil(totalRecords / pageSize));
  const isCasesAreaLoading = isCasesQueryLoading;

  const handlePageChange = (_event: ChangeEvent<unknown>, value: number) => {
    setPage(value);
  };

  const handleSortChange = (value: SortOrder) => {
    setSortOrder(value);
    setPage(1);
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

  const listHasRefinement = hasListSearchOrFilters(searchTerm, filters);

  return (
    <Stack spacing={3}>
      <Box>
        <Button
          startIcon={<ArrowLeft size={16} />}
          onClick={() => navigate("..")}
          sx={{ mb: 2 }}
          variant="text"
        >
          Back
        </Button>
        <Box>
          <Typography variant="h4" color="text.primary" sx={{ mb: 1 }}>
            Announcements
          </Typography>
          <Typography
            variant="body2"
            color="text.secondary"
            component="div"
            dangerouslySetInnerHTML={{
              __html: DOMPurify.sanitize(
                "View and manage announcements for your project",
              ),
            }}
          />
        </Box>
      </Box>

      <AnnouncementsSearchBar
        searchTerm={searchTerm}
        onSearchChange={handleSearchChange}
        isFiltersOpen={isFiltersOpen}
        onFiltersToggle={() => setIsFiltersOpen(!isFiltersOpen)}
        filters={filters}
        filterMetadata={filterMetadata}
        onFilterChange={handleFilterChange}
        onClearFilters={handleClearFilters}
      />

      <Box
        sx={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
        }}
      >
        <Typography variant="body2" color="text.secondary">
          Showing {cases.length} of {totalRecords} announcements
        </Typography>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <FormControl size="small" sx={{ minWidth: 180 }}>
            <InputLabel id="announcements-order-by-label">Order by</InputLabel>
            <Select<SortOrder>
              labelId="announcements-order-by-label"
              id="announcements-order-by"
              value={sortOrder}
              label="Order by"
              onChange={(e) =>
                handleSortChange(e.target.value as SortOrder)
              }
            >
              <MenuItem value={SortOrder.DESC}>
                <Typography variant="body2">Newest first</Typography>
              </MenuItem>
              <MenuItem value={SortOrder.ASC}>
                <Typography variant="body2">Oldest first</Typography>
              </MenuItem>
            </Select>
          </FormControl>
        </Box>
      </Box>

      {isCasesAreaLoading ? (
        <AllCasesListSkeleton />
      ) : (
        <AnnouncementList
          cases={cases}
          isLoading={false}
          hasListRefinement={listHasRefinement}
          onCaseClick={(c) => navigate(`/projects/${projectId}/announcements/${c.id}`)}
        />
      )}

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
