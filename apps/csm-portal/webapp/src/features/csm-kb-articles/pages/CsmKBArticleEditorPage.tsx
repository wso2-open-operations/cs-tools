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
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  Skeleton,
  TextField,
  Typography,
  type SelectChangeEvent,
} from "@wso2/oxygen-ui";
import { ArrowLeft, Pencil, Trash2 } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import { useParams } from "react-router";
import Editor from "@components/rich-text-editor/Editor";
import QueryErrorState from "@components/QueryErrorState";
import { useNavTransition } from "@hooks/useNavTransition";
import { useCurrentUser } from "@context/current-user/CurrentUserContext";
import { useGetKBArticle } from "@features/csm-kb-articles/api/useGetKBArticle";
import { useCreateKBArticle } from "@features/csm-kb-articles/api/useCreateKBArticle";
import { usePatchKBArticleContent } from "@features/csm-kb-articles/api/usePatchKBArticleContent";
import { usePatchKBArticleState } from "@features/csm-kb-articles/api/usePatchKBArticleState";
import { useListKnowledgeBases } from "@features/csm-kb-articles/api/useListKnowledgeBases";
import { useSearchTeams } from "@features/csm-kb-articles/api/useSearchTeams";
import { useGetKBArticleHistory } from "@features/csm-kb-articles/api/useGetKBArticleHistory";
import { useDeleteKBArticle } from "@features/csm-kb-articles/api/useDeleteKBArticle";
import KBArticleHistoryPanel from "@features/csm-kb-articles/components/KBArticleHistoryPanel";

// Hardcoded until the KB picker is built (needs a GET /knowledge-bases
// list endpoint, not yet implemented). Matches the API Manager KB seeded
// locally for testing.
const DEFAULT_KNOWLEDGE_BASE_ID = "752eaea7-79e3-4899-9d42-ba37d342779b";

// Read-only status messaging for the author's own view -- never a button,
// just a plain sentence describing where the article currently stands.
const STATUS_LABELS: Record<string, string> = {
  draft: "Draft",
  pending_review: "Pending review",
  published: "Published",
  retired: "Retired",
};

function formatDateTime(value: string): string {
  const d = new Date(value);
  return Number.isNaN(d.getTime())
    ? value
    : d.toLocaleString("en-US", { year: "numeric", month: "short", day: "numeric", hour: "numeric", minute: "2-digit" });
}

/**
 * Create-or-edit form for a single KB article. `/knowledge/my-articles/new`
 * renders this with no `id` param (create mode); `/knowledge/my-articles/{id}`
 * renders it against an existing article.
 *
 * Published articles render as a clean read-only view. Clicking "Edit this
 * article" only flips a *local* `isEditingPublished` flag -- it makes no
 * API call and does not touch the article's real state. The article stays
 * genuinely published until the user actually does something: clicking
 * "Cancel" discards the in-progress edit with no side effects at all;
 * clicking "Save changes" or "Submit for review" is what actually performs
 * the published -> draft transition (and, for Submit, the follow-on
 * draft -> pending_review transition too), immediately followed by saving
 * whatever content was typed. Merely opening and closing the edit view
 * must never change anything server-side.
 *
 * "Version N" reflects how many times this article has actually been
 * published (see versionCount below), not every lifecycle transition.
 */
export default function CsmKBArticleEditorPage(): JSX.Element {
  const { id } = useParams<{ id?: string }>();
  const navigate = useNavTransition();
  const { user } = useCurrentUser();

  const isCreateMode = !id;

  const { data: article, isLoading, isError, error } = useGetKBArticle(id);
  const { data: history } = useGetKBArticleHistory(id);
  const { data: kbList, isLoading: kbListLoading } = useListKnowledgeBases();
  const { data: teams } = useSearchTeams();
  const createArticle = useCreateKBArticle();
  const patchContent = usePatchKBArticleContent(id);
  const patchState = usePatchKBArticleState(id);
  const deleteArticle = useDeleteKBArticle();

  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [knowledgeBaseId, setKnowledgeBaseId] = useState("");
  const [teamKey, setTeamKey] = useState("");
  const [titleError, setTitleError] = useState<string | null>(null);
  const [deleteConfirmOpen, setDeleteConfirmOpen] = useState(false);
  const [isEditingPublished, setIsEditingPublished] = useState(false);
  const [historyOpen, setHistoryOpen] = useState(false);
  const [sequenceError, setSequenceError] = useState<string | null>(null);

  const [seededForId, setSeededForId] = useState<string | undefined>(undefined);
  if (article && seededForId !== id) {
    setTitle(article.title);
    setBody(article.body);
    setKnowledgeBaseId(article.knowledgeBaseId);
    setSeededForId(id);
  }
  // Only knowledge bases currently accepting new articles -- the backend
  // enforces this too (a deactivated KB rejects article creation outright),
  // this just keeps a deactivated one from ever being offered as a choice.
  const activeKnowledgeBases = (kbList?.knowledgeBases ?? []).filter((kb) => kb.isActive);

  const [kbDefaulted, setKbDefaulted] = useState(false);
  if (isCreateMode && !kbDefaulted && activeKnowledgeBases.length) {
    setKnowledgeBaseId(activeKnowledgeBases[0].id);
    setKbDefaulted(true);
  }

  const isPublished = article?.state === "published";
  const showReadOnlyPublishedView = isPublished && !isEditingPublished;
  const isReadOnly = !isCreateMode && article?.state !== "draft" && !isEditingPublished;
  const savePending =
    createArticle.isPending || patchContent.isPending || patchState.isPending || deleteArticle.isPending;
  const saveError = createArticle.error ?? patchContent.error ?? patchState.error;

  const versionCount = history?.history.filter((h) => h.state === "published").length ?? 0;
  const hasAnyHistory = (history?.history.length ?? 0) > 0;
  const isDeletable = article?.state === "draft" || article?.state === "pending_review";

  const handleKbChange = (e: SelectChangeEvent): void => {
    setKnowledgeBaseId(e.target.value);
  };

  const handleCancelEditPublished = (): void => {
    if (article) {
      setTitle(article.title);
      setBody(article.body);
    }
    setIsEditingPublished(false);
    setTitleError(null);
    setSequenceError(null);
  };

  /**
   * "Save changes" while editing a published article: this is the moment
   * the article actually leaves the published state -- transition to
   * draft first, then persist the edited content once that succeeds. If
   * either step fails, the article's real state is whatever the last
   * successful step left it at (surfaced via sequenceError); nothing here
   * silently loses that.
   */
  const saveEditedPublished = (trimmedTitle: string): void => {
    patchState.mutate(
      { state: "draft" },
      {
        onSuccess: () => {
          patchContent.mutate(
            { title: trimmedTitle, body },
            {
              onSuccess: () => setIsEditingPublished(false),
              onError: () =>
                setSequenceError(
                  "The article was moved to draft, but saving your changes failed. Your edits are still in this form -- try Save changes again.",
                ),
            },
          );
        },
      },
    );
  };

  const handleSaveDraft = (): void => {
    const trimmedTitle = title.trim();
    if (!trimmedTitle) {
      setTitleError("Title is required.");
      return;
    }
    if (!knowledgeBaseId) return;
    setTitleError(null);
    setSequenceError(null);

    if (isCreateMode) {
      if (!user?.id) return;
      createArticle.mutate(
        { knowledgeBaseId, title: trimmedTitle, body, authorId: user.id, ...(teamKey && { teamKey }) },
        { onSuccess: (res) => navigate(`/knowledge/my-articles/${res.article.id}`) },
      );
    } else if (isEditingPublished) {
      saveEditedPublished(trimmedTitle);
    } else {
      patchContent.mutate({ title: trimmedTitle, body });
    }
  };

  /**
   * "Submit for review" clicked directly on a published article, without
   * pressing "Save changes" first: still expected to work end to end --
   * draft transition, then save the typed content, then submit to
   * pending_review -- exactly as if the two actions had been done in
   * sequence.
   */
  const handleSubmitForReview = (): void => {
    setSequenceError(null);

    if (isEditingPublished) {
      const trimmedTitle = title.trim();
      if (!trimmedTitle) {
        setTitleError("Title is required.");
        return;
      }
      setTitleError(null);

      patchState.mutate(
        { state: "draft" },
        {
          onSuccess: () => {
            patchContent.mutate(
              { title: trimmedTitle, body },
              {
                onSuccess: () => {
                  patchState.mutate(
                    { state: "pending_review" },
                    {
                      onSuccess: () => navigate("/knowledge/my-articles"),
                      onError: () =>
                        setSequenceError(
                          "Your changes were saved, but submitting for review failed -- try Submit for review again.",
                        ),
                    },
                  );
                },
                onError: () =>
                  setSequenceError(
                    "The article was moved to draft, but saving your changes failed. Try Submit for review again.",
                  ),
              },
            );
          },
        },
      );
      return;
    }

    patchState.mutate({ state: "pending_review" }, { onSuccess: () => navigate("/knowledge/my-articles") });
  };

  const handleEditPublished = (): void => {
    // View-only: no API call. The article stays genuinely published until
    // Save changes or Submit for review is actually clicked.
    setIsEditingPublished(true);
  };

  const handleDeleteConfirmed = (): void => {
    if (!id) return;
    deleteArticle.mutate(id, { onSuccess: () => navigate("/knowledge/my-articles") });
  };

  if (!isCreateMode && isLoading) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <Skeleton variant="rounded" width="60%" height={32} />
        <Skeleton variant="rounded" width="100%" height={220} />
      </Box>
    );
  }

  if (!isCreateMode && isError) {
    return (
      <QueryErrorState
        message={error instanceof Error && error.message.trim() ? error.message : "Failed to load this KB article."}
        error={error}
      />
    );
  }

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Button
        variant="text"
        size="small"
        startIcon={<ArrowLeft size={16} />}
        onClick={() => navigate("/knowledge/my-articles")}
        sx={{ alignSelf: "flex-start" }}
      >
        Back to my articles
      </Button>

      <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
        <Typography variant="h6">
          {isCreateMode ? "New KB article" : showReadOnlyPublishedView ? article?.title : "Edit KB article"}
        </Typography>
        {article && (
          <Chip
            size="small"
            label={STATUS_LABELS[article.state]}
            variant="outlined"
            color={
              article.state === "published"
                ? "success"
                : article.state === "pending_review"
                  ? "warning"
                  : "default"
            }
          />
        )}
        {!isCreateMode && versionCount > 0 && <Chip size="small" label={`Version ${versionCount}`} variant="outlined" color="default" />}
        {!isCreateMode && hasAnyHistory && (
          <Typography
            variant="body2"
            color="primary"
            sx={{ cursor: "pointer", fontWeight: 500, textDecoration: "underline" }}
            onClick={() => setHistoryOpen(true)}
          >
            View history
          </Typography>
        )}
      </Box>

      {article && (
        <Typography variant="caption" color="text.secondary">
          Created {formatDateTime(article.createdOn)}
          {" · "}Last updated {formatDateTime(article.updatedOn)}
          {isPublished && article.publishedOn && <>{" · "}Published {formatDateTime(article.publishedOn)}</>}
        </Typography>
      )}

      {article?.rejectionComment && (
        <Alert severity="warning">
          This article was rejected by the approver: "{article.rejectionComment}"
        </Alert>
      )}
      {sequenceError && <Alert severity="error">{sequenceError}</Alert>}
      {saveError && (
        <Alert severity="error">{saveError instanceof Error ? saveError.message : "Failed to save this article."}</Alert>
      )}

      {showReadOnlyPublishedView ? (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 3, maxWidth: 1400, mx: "auto", width: "95%" }}>
          <Box
            sx={{
              border: 1,
              borderColor: "divider",
              borderRadius: 2,
              p: { xs: 3, md: 5 },
              lineHeight: 1.7,
              fontSize: "1.05rem",
              "& h1, & h2, & h3": { mt: 3, mb: 1.5, lineHeight: 1.3 },
              "& p": { mb: 2 },
              "& ul, & ol": { mb: 2, pl: 3 },
              "& pre, & code": {
                bgcolor: "action.hover",
                borderRadius: 1,
                px: "0.4em",
                py: "0.15em",
                fontSize: "0.9em",
              },
              "& pre": { p: 2, overflowX: "auto" },
            }}
            // eslint-disable-next-line react/no-danger -- article.body is
            // author-authored rich text HTML from our own Lexical editor,
            // not third-party/user-submitted-elsewhere content.
            dangerouslySetInnerHTML={{ __html: article?.body ?? "" }}
          />
          <Button
            variant="outlined"
            startIcon={<Pencil size={16} />}
            onClick={handleEditPublished}
            sx={{ alignSelf: "flex-start" }}
          >
            Edit this article
          </Button>
        </Box>
      ) : (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 3, maxWidth: 720 }}>
          {isCreateMode && (
            <FormControl size="small" fullWidth disabled={kbListLoading || savePending}>
              <InputLabel id="kb-select-label">Knowledge base</InputLabel>
              <Select labelId="kb-select-label" label="Knowledge base" value={knowledgeBaseId} onChange={handleKbChange}>
                {activeKnowledgeBases.map((kb) => (
                  <MenuItem key={kb.id} value={kb.id}>
                    {kb.name}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
          )}

          {isCreateMode && (
            <FormControl size="small" fullWidth disabled={savePending}>
              <InputLabel id="team-select-label">Team (optional)</InputLabel>
              <Select
                labelId="team-select-label"
                label="Team (optional)"
                value={teamKey}
                onChange={(e) => setTeamKey(e.target.value)}
              >
                <MenuItem value="">
                  <em>None</em>
                </MenuItem>
                {(teams ?? []).map((t) => (
                  <MenuItem key={t.id} value={t.id}>
                    {t.name}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
          )}

          <TextField
            label="Title"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            error={Boolean(titleError)}
            helperText={titleError}
            disabled={isReadOnly || savePending}
            fullWidth
          />

          <Box>
            <Typography
              id="kb-article-body-label"
              component="label"
              variant="caption"
              color="text.secondary"
              sx={{ display: "block", mb: 0.5 }}
            >
              Body
            </Typography>
            <Box role="group" aria-labelledby="kb-article-body-label">
              <Editor
                value={body}
                onChange={setBody}
                placeholder="Write the article…"
                minHeight={240}
                maxHeight={560}
                toolbarVariant="full"
                disabled={isReadOnly || savePending}
              />
            </Box>
          </Box>

          {!isReadOnly && (
            <Box sx={{ display: "flex", gap: 1, justifyContent: "space-between" }}>
              <Box sx={{ display: "flex", gap: 1 }}>
                {isEditingPublished && (
                  <Button variant="text" onClick={handleCancelEditPublished} disabled={savePending}>
                    Cancel
                  </Button>
                )}
                <Button variant="outlined" onClick={handleSaveDraft} disabled={savePending}>
                  {isCreateMode ? "Save draft" : "Save changes"}
                </Button>
                {!isCreateMode && (
                  <Button variant="contained" onClick={handleSubmitForReview} disabled={savePending}>
                    Submit for review
                  </Button>
                )}
              </Box>
              {!isCreateMode && isDeletable && (
                <Button
                  variant="text"
                  color="error"
                  startIcon={<Trash2 size={16} />}
                  onClick={() => setDeleteConfirmOpen(true)}
                  disabled={savePending}
                >
                  Delete
                </Button>
              )}
            </Box>
          )}
        </Box>
      )}

      <KBArticleHistoryPanel open={historyOpen} article={article ?? null} onClose={() => setHistoryOpen(false)} />

      <Dialog open={deleteConfirmOpen} onClose={() => setDeleteConfirmOpen(false)}>
        <DialogTitle>Delete this article?</DialogTitle>
        <DialogContent>
          <Typography variant="body2">
            This can't be undone. The article and its review history will be permanently removed.
          </Typography>
          {deleteArticle.isError && (
            <Alert severity="error" sx={{ mt: 2 }}>
              {deleteArticle.error instanceof Error ? deleteArticle.error.message : "Failed to delete this article."}
            </Alert>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDeleteConfirmOpen(false)} disabled={deleteArticle.isPending}>
            Cancel
          </Button>
          <Button variant="contained" color="error" onClick={handleDeleteConfirmed} disabled={deleteArticle.isPending}>
            Delete
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
