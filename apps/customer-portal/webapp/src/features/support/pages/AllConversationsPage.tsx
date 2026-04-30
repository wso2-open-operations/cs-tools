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

import {
  useParams,
  useSearchParams,
  useLocation,
} from "react-router";
import { useModifierAwareNavigate } from "@hooks/useModifierAwareNavigate";
import {
  useState,
  useMemo,
  useEffect,
  type JSX,
  type ChangeEvent,
} from "react";
import { useSessionState } from "@hooks/useSessionState";
import { Divider, Stack } from "@wso2/oxygen-ui";
import { useLoader } from "@context/linear-loader/LoaderContext";
import useGetProjectFilters from "@api/useGetProjectFilters";
import { useSearchConversations } from "@features/support/api/useSearchConversations";
import type {
  AllConversationsFilterValues,
  Conversation,
} from "@features/support/types/conversations";
import { ALL_CONVERSATIONS_FILTER_DEFINITIONS } from "@features/support/constants/supportConstants";
import type { CaseMetadataResponse } from "@features/support/types/cases";
import ListPageHeader from "@components/list-view/ListPageHeader";
import ListSearchBar from "@components/list-view/ListSearchBar";
import ListFiltersPanel from "@components/list-view/ListFiltersPanel";
import ListResultsBar from "@components/list-view/ListResultsBar";
import ListPagination from "@components/list-view/ListPagination";
import AllConversationsList from "@features/support/components/all-conversations/AllConversationsList";
import {
  hasListSearchOrFilters,
  countListSearchAndFilters,
} from "@features/support/utils/support";
import { ConversationStatus } from "@features/support/constants/supportConstants";
import { SortOrder } from "@/types/common";

/**
 * AllConversationsPage component to display all conversations with filters, search, and pagination.
 *
 * @returns {JSX.Element} The rendered All Conversations page.
 */
export default function AllConversationsPage(): JSX.Element {
  const navigate = useModifierAwareNavigate();
  const location = useLocation();
  const returnTo = (location.state as { returnTo?: string } | null)?.returnTo;
  const { projectId } = useParams<{ projectId: string }>();
  const [searchParams] = useSearchParams();
  const createdByMe = searchParams.get("createdByMe") === "true";
  const rawStatusFilter = searchParams.get("statusFilter");
  const statusFilter: "active" | "resolvedViaChat" | null =
    rawStatusFilter === "active" || rawStatusFilter === "resolvedViaChat"
      ? rawStatusFilter
      : null;

  const sessionPrefix = `${projectId ?? "unknown"}-conversations`;
  const [searchTerm, setSearchTerm] = useSessionState(
    `${sessionPrefix}-search`,
    "",
    undefined,
    { popOnly: true },
  );
  const [filters, setFilters] = useSessionState<AllConversationsFilterValues>(
    `${sessionPrefix}-filters`,
    {},
    undefined,
    { popOnly: true },
  );
  const [isFiltersOpen, setIsFiltersOpen] = useState(
    () => hasListSearchOrFilters(searchTerm, filters),
  );
  const [sortField, setSortField] = useSessionState<"createdOn" | "updatedOn">(
    `${sessionPrefix}-sortField`,
    "updatedOn",
    undefined,
    { popOnly: true },
  );
  const [sortOrder, setSortOrder] = useSessionState<SortOrder>(
    `${sessionPrefix}-sortOrder`,
    SortOrder.DESC,
    undefined,
    { popOnly: true },
  );
  const [page, setPage] = useSessionState<number>(
    `${sessionPrefix}-page`,
    1,
    undefined,
    { popOnly: true },
  );
  const [rowsPerPage, setRowsPerPage] = useSessionState<number>(
    `${sessionPrefix}-rowsPerPage`,
    10,
    undefined,
    { popOnly: true },
  );

  const { data: filterMetadata } = useGetProjectFilters(projectId || "");

  const fixedStateKeys = useMemo<number[] | undefined>(() => {
    if (!statusFilter || !filterMetadata?.conversationStates) return undefined;
    if (statusFilter === "active") {
      return filterMetadata.conversationStates
        .filter((s) => s.label === ConversationStatus.ACTIVE)
        .map((s) => Number(s.id));
    }
    if (statusFilter === "resolvedViaChat") {
      return filterMetadata.conversationStates
        .filter((s) => s.label === ConversationStatus.RESOLVED)
        .map((s) => Number(s.id));
    }
    return undefined;
  }, [statusFilter, filterMetadata?.conversationStates]);

  const searchRequest = useMemo(
    () => ({
      filters: {
        searchQuery: searchTerm.trim() || undefined,
        stateKeys:
          fixedStateKeys ??
          (filters.stateId ? [Number(filters.stateId)] : undefined),
        createdByMe: createdByMe || undefined,
      },
      pagination: {
        offset: (page - 1) * rowsPerPage,
        limit: rowsPerPage,
      },
      sortBy: {
        field: sortField,
        order: sortOrder,
      },
    }),
    [
      fixedStateKeys,
      searchTerm,
      filters.stateId,
      page,
      rowsPerPage,
      sortField,
      sortOrder,
      createdByMe,
    ],
  );

  const {
    data,
    isLoading: isConversationsLoading,
    isError: isConversationsError,
  } = useSearchConversations(projectId || "", searchRequest);

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

  const handleRowsPerPageChange = (newSize: number) => {
    setRowsPerPage(newSize);
    setPage(1);
  };

  const handleFilterChange = (field: string, value: string) => {
    const key = field as keyof AllConversationsFilterValues;
    setFilters((prev) => ({
      ...prev,
      [key]: value || undefined,
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
      <ListPageHeader
        title={
          statusFilter === "active"
            ? "Active Chats"
            : statusFilter === "resolvedViaChat"
              ? "Resolved via Chat Last 30d"
              : createdByMe
                ? "My Chat History"
                : "All Chat History"
        }
        description={
          statusFilter === "active"
            ? "Conversations currently in progress"
            : statusFilter === "resolvedViaChat"
              ? "Conversations that were resolved via chat during the last 30 days"
              : createdByMe
                ? "Browse and search your conversation history with Novera"
                : "Browse and search your complete conversation history with Novera"
        }
        backLabel="Back to Support Center"
        onBack={() => (returnTo ? navigate(returnTo) : navigate(".."))}
      />

      {statusFilter ? (
        <Divider />
      ) : (
        <ListSearchBar
          searchPlaceholder="Search chats by message, ID, or category..."
          searchTerm={searchTerm}
          onSearchChange={handleSearchChange}
          isFiltersOpen={isFiltersOpen}
          onFiltersToggle={() => setIsFiltersOpen(!isFiltersOpen)}
          activeFiltersCount={countListSearchAndFilters("", filters)}
          onClearFilters={handleClearFilters}
          filtersContent={
            <ListFiltersPanel
              filterDefinitions={ALL_CONVERSATIONS_FILTER_DEFINITIONS}
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
      )}

      <ListResultsBar
        shownCount={conversations.length}
        totalCount={totalRecords}
        entityLabel="chat sessions"
        sortFieldOptions={[
          { value: "updatedOn", label: "Updated on" },
          { value: "createdOn", label: "Created on" },
        ]}
        sortField={sortField}
        onSortFieldChange={(v) =>
          handleSortFieldChange(v as "createdOn" | "updatedOn")
        }
        sortOrder={sortOrder}
        onSortOrderChange={handleSortChange}
      />

      <AllConversationsList
        conversations={conversations}
        isLoading={isConversationsAreaLoading}
        isError={isConversationsError}
        hasListRefinement={listHasRefinement}
        onConversationClick={handleConversationClick}
      />

      <ListPagination
        totalRecords={totalRecords}
        page={page}
        rowsPerPage={rowsPerPage}
        onPageChange={handlePageChange}
        onRowsPerPageChange={handleRowsPerPageChange}
      />
    </Stack>
  );
}
