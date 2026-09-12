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
  Alert,
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  IconButton,
  Skeleton,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TableRow,
  TextField,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { Check, Eye, X } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX } from "react";
import QueryErrorState from "@components/QueryErrorState";
import RefreshButton from "@components/RefreshButton";
import { useUsersByIds } from "@features/csm-kb-articles/api/useUsersByIds";
import { usePatchKBArticleState } from "@features/csm-kb-articles/api/usePatchKBArticleState";
import { useSearchKBArticles } from "@features/csm-kb-articles/api/useSearchKBArticles";
import { useListKnowledgeBases } from "@features/csm-kb-articles/api/useListKnowledgeBases";
import { useMyManagedKnowledgeBases } from "@features/csm-kb-articles/api/useMyManagedKnowledgeBases";
import KBArticlePreviewDrawer from "@features/csm-kb-articles/components/KBArticlePreviewDrawer";
import KBMultiSelectFilter from "@features/csm-kb-articles/components/KBMultiSelectFilter";
import { KB_QUICK_PREVIEW_EYE_ATTRIBUTE } from "@features/csm-kb-articles/utils/kbQuickPreviewEye";
import type { KBArticle } from "@features/csm-kb-articles/types/csmKbArticles";

function formatDate(value: string): string {
  const d = new Date(value);
  return Number.isNaN(d.getTime())
    ? value
    : d.toLocaleDateString("en-US", { year: "numeric", month: "short", day: "numeric" });
}

/** One row's approve/reject actions plus the quick-preview eye. Isolated so
 * each row owns its own mutation hook instance
 * (`usePatchKBArticleState(article.id)`) — approving one row never disables
 * the buttons on any other row. */
function ReviewQueueRow({
  article,
  knowledgeBaseName,
  authorName,
  isPreviewOpen,
  onPreviewClick,
  onRejectClick,
}: {
  article: KBArticle;
  knowledgeBaseName: string;
  authorName: string;
  isPreviewOpen: boolean;
  onPreviewClick: (article: KBArticle) => void;
  onRejectClick: (article: KBArticle) => void;
}): JSX.Element {
  const patchState = usePatchKBArticleState(article.id);

  const handleApprove = (): void => {
    patchState.mutate({ state: "published" });
  };

  return (
    <TableRow>
      <TableCell>
        <Tooltip title={`Quick preview ${article.title}`}>
          <IconButton
            size="small"
            aria-label={`Quick preview ${article.title}`}
            aria-pressed={isPreviewOpen}
            {...{ [KB_QUICK_PREVIEW_EYE_ATTRIBUTE]: "true" }}
            onClick={(e) => {
              e.stopPropagation();
              onPreviewClick(article);
            }}
          >
            <Eye size={16} />
          </IconButton>
        </Tooltip>
      </TableCell>
      <TableCell>
        <Typography variant="body2" noWrap>
          {article.title}
        </Typography>
      </TableCell>
      <TableCell>{knowledgeBaseName}</TableCell>
      <TableCell>{authorName}</TableCell>
      <TableCell>{formatDate(article.updatedOn)}</TableCell>
      <TableCell align="right">
        <Box sx={{ display: "flex", gap: 1, justifyContent: "flex-end" }}>
          <Button
            size="small"
            variant="outlined"
            color="error"
            startIcon={<X size={14} />}
            onClick={() => onRejectClick(article)}
            disabled={patchState.isPending}
          >
            Reject
          </Button>
          <Button
            size="small"
            variant="contained"
            color="success"
            startIcon={<Check size={14} />}
            onClick={handleApprove}
            disabled={patchState.isPending}
          >
            Approve
          </Button>
        </Box>
        {patchState.isError && (
          <Typography variant="caption" color="error" sx={{ display: "block", mt: 0.5 }}>
            {patchState.error instanceof Error
              ? patchState.error.message
              : "Action failed."}
          </Typography>
        )}
      </TableCell>
    </TableRow>
  );
}

/**
 * Reject-with-comment dialog. A separate component (not inline in the page)
 * so its own mutation hook is scoped to whichever article is currently
 * targeted, mirroring ReviewQueueRow's per-row hook isolation above.
 */
function RejectDialog({
  article,
  onClose,
}: {
  article: KBArticle | null;
  onClose: () => void;
}): JSX.Element {
  const [comment, setComment] = useState("");
  const patchState = usePatchKBArticleState(article?.id);

  const handleConfirm = (): void => {
    patchState.mutate(
      { state: "draft", rejectionComment: comment.trim() || undefined },
      { onSuccess: () => { setComment(""); onClose(); } },
    );
  };

  return (
    <Dialog open={Boolean(article)} onClose={onClose} fullWidth maxWidth="sm">
      <DialogTitle>Reject "{article?.title}"</DialogTitle>
      <DialogContent sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 1 }}>
        <Typography variant="body2" color="text.secondary">
          This sends the article back to the author as a draft, along with
          your comment below.
        </Typography>
        <TextField
          label="Reason for rejection"
          value={comment}
          onChange={(e) => setComment(e.target.value)}
          multiline
          minRows={3}
          fullWidth
        />
        {patchState.isError && (
          <Alert severity="error">
            {patchState.error instanceof Error
              ? patchState.error.message
              : "Failed to reject this article."}
          </Alert>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose} disabled={patchState.isPending}>
          Cancel
        </Button>
        <Button
          variant="contained"
          color="error"
          onClick={handleConfirm}
          disabled={patchState.isPending}
        >
          Reject article
        </Button>
      </DialogActions>
    </Dialog>
  );
}

/**
 * "To Review" — articles pending review, filtered to only the knowledge
 * bases the current user manages. Rather than showing every pending
 * article platform-wide and hiding Approve/Reject per row for KBs the
 * caller doesn't manage, the list itself is scoped up front: every row
 * shown here is always one the caller can act on. A user who manages no
 * knowledge base simply sees an empty state.
 */
export default function CsmKBReviewQueuePage(): JSX.Element {
  const [rejectTarget, setRejectTarget] = useState<KBArticle | null>(null);
  const [previewArticle, setPreviewArticle] = useState<KBArticle | null>(null);
  const [kbFilter, setKbFilter] = useState<string[]>([]);
  const [authorFilter, setAuthorFilter] = useState<string[]>([]);

  const { data: myKbs, isLoading: myKbsLoading } = useMyManagedKnowledgeBases();
  const managedKbIds = useMemo(() => new Set(myKbs?.knowledgeBaseIds ?? []), [myKbs]);

  const { data, isLoading, isFetching, isError, error, refetch, dataUpdatedAt } =
    useSearchKBArticles({
      states: ["pending_review"],
      pagination: { limit: 50, offset: 0 },
    });

  const allArticles = data?.articles ?? [];
  const managedArticles = useMemo(
    () => allArticles.filter((a) => managedKbIds.has(a.knowledgeBaseId)),
    [allArticles, managedKbIds],
  );

  const { data: kbList } = useListKnowledgeBases();
  const kbNameById = useMemo(() => {
    const map = new Map<string, string>();
    for (const kb of kbList?.knowledgeBases ?? []) map.set(kb.id, kb.name);
    return map;
  }, [kbList]);
  const kbOptions = useMemo(
    () =>
      Array.from(new Set(managedArticles.map((a) => a.knowledgeBaseId))).map((id) => ({
        value: id,
        label: kbNameById.get(id) ?? id,
      })),
    [managedArticles, kbNameById],
  );

  const authorIds = useMemo(
    () => Array.from(new Set(managedArticles.map((a) => a.authorId))),
    [managedArticles],
  );
  const { data: authorNameById = new Map<string, string>() } = useUsersByIds(authorIds);
  const authorOptions = useMemo(
    () => authorIds.map((id) => ({ value: id, label: authorNameById.get(id) ?? id })),
    [authorIds, authorNameById],
  );

  const articles = useMemo(
    () =>
      managedArticles.filter(
        (a) =>
          (kbFilter.length === 0 || kbFilter.includes(a.knowledgeBaseId)) &&
          (authorFilter.length === 0 || authorFilter.includes(a.authorId)),
      ),
    [managedArticles, kbFilter, authorFilter],
  );

  const isPageLoading = isLoading || isFetching || myKbsLoading;

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 1 }}>
        <Typography variant="body2" color="text.secondary">
          Articles awaiting review in knowledge bases you manage.
        </Typography>
        <RefreshButton
          onRefresh={() => void refetch()}
          isFetching={isFetching}
          updatedAt={dataUpdatedAt}
          label="Refresh review queue"
        />
      </Box>

      <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }}>
        <KBMultiSelectFilter label="Knowledge base" options={kbOptions} values={kbFilter} onChange={setKbFilter} minWidth={220} />
        <KBMultiSelectFilter label="Author" options={authorOptions} values={authorFilter} onChange={setAuthorFilter} minWidth={200} />
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
                <TableCell>Submitted</TableCell>
                <TableCell align="right">Actions</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {isPageLoading ? (
                Array.from({ length: 5 }).map((_, i) => (
                  <TableRow key={i}>
                    <TableCell />
                    <TableCell><Skeleton variant="rounded" width="70%" height={18} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width="60%" height={18} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width="60%" height={18} /></TableCell>
                    <TableCell><Skeleton variant="rounded" width="50%" height={18} /></TableCell>
                    <TableCell align="right"><Skeleton variant="rounded" width={160} height={32} sx={{ ml: "auto" }} /></TableCell>
                  </TableRow>
                ))
              ) : isError ? (
                <TableRow>
                  <TableCell colSpan={6} align="center">
                    <QueryErrorState
                      message={
                        error instanceof Error && error.message.trim()
                          ? error.message
                          : "Failed to load the review queue."
                      }
                      error={error}
                    />
                  </TableCell>
                </TableRow>
              ) : articles.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={6} align="center" sx={{ py: 4 }}>
                    <Typography variant="body2" color="text.secondary">
                      {managedKbIds.size === 0
                        ? "You're not an approver for any knowledge base yet."
                        : "Nothing pending review right now."}
                    </Typography>
                  </TableCell>
                </TableRow>
              ) : (
                articles.map((a) => (
                  <ReviewQueueRow
                    key={a.id}
                    article={a}
                    knowledgeBaseName={kbNameById.get(a.knowledgeBaseId) ?? "—"}
                    authorName={authorNameById.get(a.authorId) ?? "—"}
                    isPreviewOpen={previewArticle?.id === a.id}
                    onPreviewClick={(article) =>
                      setPreviewArticle((prev) => (prev?.id === article.id ? null : article))
                    }
                    onRejectClick={setRejectTarget}
                  />
                ))
              )}
            </TableBody>
          </Table>
        </TableContainer>
      </Box>

      <RejectDialog article={rejectTarget} onClose={() => setRejectTarget(null)} />

      <KBArticlePreviewDrawer
        article={previewArticle}
        knowledgeBaseName={previewArticle ? kbNameById.get(previewArticle.knowledgeBaseId) ?? "—" : ""}
        authorName={previewArticle ? authorNameById.get(previewArticle.authorId) ?? "—" : ""}
        updatedByName="—"
        onClose={() => setPreviewArticle(null)}
      />
    </Box>
  );
}
