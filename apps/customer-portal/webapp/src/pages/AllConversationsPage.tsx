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

import { useParams, useNavigate, useSearchParams, useLocation } from "react-router";
import {
  useState,
  useMemo,
  useEffect,
  type JSX,
  type ChangeEvent,
} from "react";
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
import { useLoader } from "@context/linear-loader/LoaderContext";
import useGetProjectFilters from "@api/useGetProjectFilters";
import { useSearchConversations } from "@api/useSearchConversations";
import { useGetConversationStats } from "@api/useGetConversationStats";
import type { AllConversationsFilterValues, Conversation } from "@/types/conversations";
import type { AllConversationsStatKey } from "@constants/supportConstants";
import AllConversationsStatCards from "@components/support/all-conversations/AllConversationsStatCards";
import AllConversationsSearchBar from "@components/support/all-conversations/AllConversationsSearchBar";
import AllConversationsList from "@components/support/all-conversations/AllConversationsList";
import { hasListSearchOrFilters } from "@utils/support";
import { SortOrder } from "@/types/common";

/**
 * AllConversationsPage component to display all conversations with filters, search, and pagination.
 *
 * @returns {JSX.Element} The rendered All Conversations page.
 */
export default function AllConversationsPage(): JSX.Element {
  const navigate = useNavigate();
  const location = useLocation();
  const returnTo = (location.state as { returnTo?: string } | null)?.returnTo;
  const { projectId } = useParams<{ projectId: string }>();
  const [searchParams] = useSearchParams();
  const createdByMe = searchParams.get("createdByMe") === "true";

  const [searchTerm, setSearchTerm] = useState("");
  const [isFiltersOpen, setIsFiltersOpen] = useState(false);
  const [filters, setFilters] = useState<AllConversationsFilterValues>({});
  const [sortField, setSortField] = useState<"createdOn" | "updatedOn">(
    "updatedOn",
  );
  const [sortOrder, setSortOrder] = useState<SortOrder>(SortOrder.DESC);
  const [page, setPage] = useState(1);
  const pageSize = 10;

  const { data: filterMetadata } = useGetProjectFilters(projectId || "");

  const searchRequest = useMemo(
    () => ({
      filters: {
        searchQuery: searchTerm.trim() || undefined,
        stateKeys: filters.stateId ? [Number(filters.stateId)] : undefined,
        createdByMe: createdByMe || undefined,
      },
      pagination: {
        offset: (page - 1) * pageSize,
        limit: pageSize,
      },
      sortBy: {
        field: sortField,
        order: sortOrder,
      },
    }),
    [searchTerm, filters.stateId, page, pageSize, sortField, sortOrder, createdByMe],
  );

  const {
    data,
    isLoading: isConversationsLoading,
    isError: isConversationsError,
  } = useSearchConversations(projectId || "", searchRequest);

  const {
    data: statsData,
    isLoading: isStatsLoading,
    isError: isStatsError,
  } = useGetConversationStats(projectId || "", {
    createdByMe: createdByMe || undefined,
  });

  const stats: Partial<Record<AllConversationsStatKey, number>> | undefined =
    statsData
      ? {
          resolved: statsData.resolvedCount,
          open: statsData.openCount,
          abandoned: statsData.abandonedCount,
          totalChats:
            statsData.resolvedCount +
            statsData.openCount +
            statsData.abandonedCount,
        }
      : undefined;

  const { showLoader, hideLoader } = useLoader();

  const hasDataResponse = data !== undefined;
  const isConversationsAreaLoading =
    isConversationsLoading || (!!projectId && !hasDataResponse);

  useEffect(() => {
    if (isConversationsAreaLoading) {
      showLoader();
      return () => hideLoader();
    }
    hideLoader();
  }, [isConversationsAreaLoading, showLoader, hideLoader]);

  const conversations = data?.conversations ?? [];
  const totalRecords = data?.totalRecords ?? 0;
  const totalPages = Math.max(1, Math.ceil(totalRecords / pageSize));

  const handleConversationClick = (conv: Conversation) => {
    if (!projectId) return;

    navigate(`/projects/${projectId}/support/conversations/${conv.id}`, {
      state: {
        conversationSummary: {
          chatId: conv.id,
          chatNumber: conv.number,
          title: conv.initialMessage || conv.number,
          startedTime: conv.createdOn,
          messages: conv.messageCount,
          kbArticles: 0,
          status: conv.state?.label ?? "Open",
        },
      },
    });
  };

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
    setSearchTerm("");
    setPage(1);
  };

  const handleSortChange = (value: SortOrder) => {
    setSortOrder(value);
    setPage(1);
  };

  const handleSortFieldChange = (value: "createdOn" | "updatedOn") => {
    setSortField(value);
    setPage(1);
  };

  const handleSearchChange = (value: string) => {
    setSearchTerm(value);
    setPage(1);
  };

  const listHasRefinement = hasListSearchOrFilters(searchTerm, filters);

  return (
    <Stack spacing={3}>
      <Box>
        <Button
          startIcon={<ArrowLeft size={16} />}
          onClick={() => (returnTo ? navigate(returnTo) : navigate(".."))}
          sx={{ mb: 2 }}
          variant="text"
        >
          Back to Support Center
        </Button>
        <Box>
          <Typography variant="h4" color="text.primary" sx={{ mb: 1 }}>
            {createdByMe ? "My Chat History" : "All Chat History"}
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {createdByMe
              ? "Browse and search your conversation history with Novera"
              : "Browse and search your complete conversation history with Novera"}
          </Typography>
        </Box>
      </Box>

      <AllConversationsStatCards
        isLoading={isStatsLoading}
        isError={isStatsError}
        stats={stats}
      />

      <AllConversationsSearchBar
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
          Showing {conversations.length} of {totalRecords} chat sessions
        </Typography>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <FormControl size="small" sx={{ minWidth: 180 }}>
            <InputLabel id="conversation-sort-by-label">Sort by</InputLabel>
            <Select<"createdOn" | "updatedOn">
              labelId="conversation-sort-by-label"
              id="conversation-sort-by"
              value={sortField}
              label="Sort by"
              onChange={(e) =>
                handleSortFieldChange(
                  e.target.value as "createdOn" | "updatedOn",
                )
              }
            >
              <MenuItem value="updatedOn">
                <Typography variant="body2">Updated on</Typography>
              </MenuItem>
              <MenuItem value="createdOn">
                <Typography variant="body2">Created on</Typography>
              </MenuItem>
            </Select>
          </FormControl>
          <FormControl size="small" sx={{ minWidth: 180 }}>
            <InputLabel id="conversation-order-by-label">Order by</InputLabel>
            <Select<SortOrder>
              labelId="conversation-order-by-label"
              id="conversation-order-by"
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

      <AllConversationsList
        conversations={conversations}
        isLoading={isConversationsAreaLoading}
        isError={isConversationsError}
        hasListRefinement={listHasRefinement}
        onConversationClick={handleConversationClick}
      />

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
