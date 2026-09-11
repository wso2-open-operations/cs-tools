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
  Box,
  Chip,
  IconButton,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TablePagination,
  TableRow,
  TextField,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { Eye } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type ChangeEvent, type JSX } from "react";
import QueryErrorState from "@components/QueryErrorState";
import RefreshButton from "@components/RefreshButton";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useNavTransition } from "@hooks/useNavTransition";
import { useUsersByIds } from "@features/csm-kb-articles/api/useUsersByIds";
import { useSearchKBArticles } from "@features/csm-kb-articles/api/useSearchKBArticles";
import { useListKnowledgeBases } from "@features/csm-kb-articles/api/useListKnowledgeBases";
import { useSearchTeams } from "@features/csm-kb-articles/api/useSearchTeams";
import KBArticlePreviewDrawer from "@features/csm-kb-articles/components/KBArticlePreviewDrawer";
import KBMultiSelectFilter from "@features/csm-kb-articles/components/KBMultiSelectFilter";
import { KB_QUICK_PREVIEW_EYE_ATTRIBUTE } from "@features/csm-kb-articles/utils/kbQuickPreviewEye";
import type {
  KBArticle,
  KBArticleState,
  SearchKBArticlesRequest,
} from "@features/csm-kb-articles/types/csmKbArticles";
import { BE_MAX_PAGE_LIMIT } from "@constants/apiConstants";

const DEFAULT_ROWS_PER_PAGE = 20;
const ROWS_PER_PAGE_OPTIONS = [10, 20, BE_MAX_PAGE_LIMIT];

const STATE_LABELS: Record<KBArticleState, string> = {
  draft: "Draft",
  pending_review: "Pending review",
  published: "Published",
  retired: "Retired",
};

const STATE_COLORS: Record<KBArticleState, "default" | "warning" | "success"> = {
  draft: "default",
  pending_review: "warning",
  published: "success",
  retired: "default",
};

const STATE_OPTIONS = Object.entries(STATE_LABELS).map(([value, label]) => ({ value, label }));

function formatDate(value: string): string {
  const d = new Date(value);
  return Number.isNaN(d.getTime())
    ? value
    : d.toLocaleDateString("en-US", { year: "numeric", month: "short", day: "numeric" });
}

/**
 * "All" — every KB article platform-wide, not scoped to author or approver
 * (unlike the "My" and "To Review" tabs). An oversight/browsing view for
 * seeing overall activity.
 *
 * Filters: Status is sent to the backend (SearchKBArticlesRequest.states is
 * already array-capable). Knowledge base and Author are filtered
 * client-side against the current page's results -- the backend only
 * accepts a single knowledgeBaseId/authorId today, and this dataset is
 * bounded/small, matching the same client-side-filtering approach already
 * used elsewhere in this codebase (see useSearchProducts). The Author
 * filter's option list is therefore limited to authors present on already-
 * loaded pages, not the full platform-wide author list -- a real
 * limitation, not a UI bug, worth revisiting if this needs to scale further.
 *
 * Team filter (requested on the Sep 10 call, for browsing/visibility only,
 * not for restricting approval) is not yet implemented -- it needs new
 * backend plumbing to resolve which team an article's author belongs to,
 * which nothing in this feature currently does.
 */
export default function CsmKBArticlesAllPage(): JSX.Element {
  const navigate = useNavTransition();
  const [searchInput, setSearchInput] = useState("");
  const [kbFilter, setKbFilter] = useState<string[]>([]);
  const [stateFilter, setStateFilter] = useState<string[]>([]);
  const [authorFilter, setAuthorFilter] = useState<string[]>([]);
  const [teamFilter, setTeamFilter] = useState<string[]>([]);
  const [page, setPage] = useState(0);
  const [rowsPerPage, setRowsPerPage] = useState(DEFAULT_ROWS_PER_PAGE);
  const [previewArticle, setPreviewArticle] = useState<KBArticle | null>(null);

  const debouncedSearch = useDebouncedValue(searchInput, 300);

  const { data: kbList } = useListKnowledgeBases();
  const { data: teams } = useSearchTeams();
  const teamOptions = (teams ?? []).map((t) => ({ value: t.id, label: t.name }));
  const kbNameById = useMemo(() => {
    const map = new Map<string, string>();
    for (const kb of kbList?.knowledgeBases ?? []) map.set(kb.id, kb.name);
    return map;
  }, [kbList]);
  const kbOptions = useMemo(
    () => (kbList?.knowledgeBases ?? []).map((kb) => ({ value: kb.id, label: kb.name })),
    [kbList],
  );

  const request = useMemo<SearchKBArticlesRequest>(() => {
    const searchQuery = debouncedSearch.trim() || undefined;
    return {
      pagination: { limit: rowsPerPage, offset: page * rowsPerPage },
      ...(searchQuery && { searchQuery }),
      ...(stateFilter.length > 0 && { states: stateFilter as KBArticleState[] }),
      ...(teamFilter.length > 0 && { teamKeys: teamFilter }),
    };
  }, [debouncedSearch, page, rowsPerPage, stateFilter, teamFilter]);

  const { data, isLoading, isFetching, isError, error, refetch, dataUpdatedAt } =
    useSearchKBArticles(request);

  const allArticles = data?.articles ?? [];

  // Resolve every author and updatedBy id present on this page in one
  // batched call, rather than two separate lookups.
  const userIds = useMemo(() => {
    const ids = new Set<string>();
    for (const a of allArticles) {
      ids.add(a.authorId);
      if (a.updatedBy) ids.add(a.updatedBy);
    }
    return Array.from(ids);
  }, [allArticles]);
  const { data: nameById = new Map<string, string>() } = useUsersByIds(userIds);

  const authorOptions = useMemo(() => {
    const ids = new Set(allArticles.map((a) => a.authorId));
    return Array.from(ids).map((id) => ({ value: id, label: nameById.get(id) ?? id }));
  }, [allArticles, nameById]);

  const articles = useMemo(
    () =>
      allArticles.filter(
        (a) =>
          (kbFilter.length === 0 || kbFilter.includes(a.knowledgeBaseId)) &&
          (authorFilter.length === 0 || authorFilter.includes(a.authorId)),
      ),
    [allArticles, kbFilter, authorFilter],
  );
  const total = data?.total ?? 0;

  const handleSearchChange = (e: ChangeEvent<HTMLInputElement>): void => {
    setSearchInput(e.target.value);
    setPage(0);
  };

  const handleChangeRowsPerPage = (e: ChangeEvent<HTMLInputElement>): void => {
    setRowsPerPage(parseInt(e.target.value, 10));
    setPage(0);
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
        <Typography variant="body2" color="text.secondary">
          Every article across every knowledge base.
        </Typography>
        <RefreshButton
          onRefresh={() => void refetch()}
          isFetching={isFetching}
          updatedAt={dataUpdatedAt}
          label="Refresh articles"
        />
      </Box>

      <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }}>
        <TextField
          size="small"
          label="Search articles"
          placeholder="Search by title"
          value={searchInput}
          onChange={handleSearchChange}
          slotProps={{ htmlInput: { "aria-label": "Search articles by title" } }}
          sx={{ minWidth: 240 }}
        />
        <KBMultiSelectFilter label="Knowledge base" options={kbOptions} values={kbFilter} onChange={(v) => { setKbFilter(v); setPage(0); }} minWidth={220} />
        <KBMultiSelectFilter label="Status" options={STATE_OPTIONS} values={stateFilter} onChange={(v) => { setStateFilter(v); setPage(0); }} minWidth={180} />
        <KBMultiSelectFilter label="Author" options={authorOptions} values={authorFilter} onChange={(v) => { setAuthorFilter(v); setPage(0); }} minWidth={200} />
        <KBMultiSelectFilter label="Team" options={teamOptions} values={teamFilter} onChange={(v) => { setTeamFilter(v); setPage(0); }} minWidth={200} />
      </Box>

      <Box sx={{ border: 1, borderColor: "divider", borderRadius: 1, overflow: "hidden" }}>
        <TableContainer>
          <Table size="small" sx={{ "& .MuiTableCell-root": { borderColor: "divider" } }}>
            <TableHead>
              <TableRow sx={{ bgcolor: "action.hover" }}>
                <TableCell width={48} />
                <TableCell>Title</TableCell>
                <TableCell>Knowledge base</TableCell>
                <TableCell>Author</TableCell>
                <TableCell>Status</TableCell>
                <TableCell>Last updated</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {isLoading || isFetching ? (
                Array.from({ length: rowsPerPage }).map((_, i) => (
                  <TableRow key={i}>
                    <TableCell />
                    <TableCell><Skeleton variant="rounded" width="80%" height={18} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width="60%" height={18} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width="60%" height={18} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width={90} height={22} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width="60%" height={18} /></TableCell>
                  </TableRow>
                ))
              ) : isError ? (
                <TableRow>
                  <TableCell colSpan={6} align="center">
                    <QueryErrorState
                      message={
                        error instanceof Error && error.message.trim()
                          ? error.message
                          : "Failed to load KB articles."
                      }
                      error={error}
                    />
                  </TableCell>
                </TableRow>
              ) : articles.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={6} align="center" sx={{ py: 4 }}>
                    <Typography variant="body2" color="text.secondary">
                      No articles match these filters.
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                articles.map((a) => (
                  <TableRow
                    key={a.id}
                    hover
                    onClick={() => navigate(`/knowledge/my-articles/${a.id}`)}
                    sx={{ cursor: "pointer" }}
                  >
                    <TableCell>
                      <Tooltip title={`Quick preview ${a.title}`}>
                        <IconButton
                          size="small"
                          aria-label={`Quick preview ${a.title}`}
                          aria-pressed={previewArticle?.id === a.id}
                          {...{ [KB_QUICK_PREVIEW_EYE_ATTRIBUTE]: "true" }}
                          onClick={(e) => {
                            e.stopPropagation();
                            setPreviewArticle((prev) => (prev?.id === a.id ? null : a));
                          }}
                        >
                          <Eye size={16} />
                        </IconButton>
                      </Tooltip>
                    </TableCell>
                    <TableCell>
                      <Typography variant="body2" noWrap>
                        {a.title}
                      </Typography>
                    </TableCell>
                    <TableCell>{kbNameById.get(a.knowledgeBaseId) ?? "—"}</TableCell>
                    <TableCell>{nameById.get(a.authorId) ?? "—"}</TableCell>
                    <TableCell>
                      {a.state === "draft" && a.rejectionComment ? (
                        <Chip size="small" label="Rejected" color="error" variant="outlined" />
                      ) : (
                        <Chip
                          size="small"
                          label={STATE_LABELS[a.state]}
                          color={STATE_COLORS[a.state]}
                          variant="outlined"
                        />
                      )}
                    </TableCell>
                    <TableCell>{formatDate(a.updatedOn)}</TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </TableContainer>
        <TablePagination
          component="div"
          count={total}
          page={page}
          onPageChange={(_, newPage) => setPage(newPage)}
          rowsPerPage={rowsPerPage}
          onRowsPerPageChange={handleChangeRowsPerPage}
          rowsPerPageOptions={ROWS_PER_PAGE_OPTIONS}
          showFirstButton
          showLastButton
        />
      </Box>

      <KBArticlePreviewDrawer
        article={previewArticle}
        knowledgeBaseName={previewArticle ? kbNameById.get(previewArticle.knowledgeBaseId) ?? "—" : ""}
        authorName={previewArticle ? nameById.get(previewArticle.authorId) ?? "—" : ""}
        updatedByName={previewArticle?.updatedBy ? nameById.get(previewArticle.updatedBy) ?? "—" : "—"}
        onClose={() => setPreviewArticle(null)}
      />
    </Box>
  );
}
