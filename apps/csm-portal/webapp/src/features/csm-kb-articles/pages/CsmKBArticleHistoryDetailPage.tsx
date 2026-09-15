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

import { Box, Button, Chip, ToggleButton, ToggleButtonGroup, Typography } from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX } from "react";
import { useParams } from "react-router";
import { useNavTransition } from "@hooks/useNavTransition";
import { useGetKBArticle } from "@features/csm-kb-articles/api/useGetKBArticle";
import { useKBArticleTimelineEvents } from "@features/csm-kb-articles/api/useKBArticleTimelineEvents";
import { stripHtml, wordDiff } from "@features/csm-kb-articles/utils/simpleWordDiff";

function formatDateTime(value: string): string {
  const d = new Date(value);
  return Number.isNaN(d.getTime())
    ? value
    : d.toLocaleString("en-US", { year: "numeric", month: "short", day: "numeric", hour: "numeric", minute: "2-digit" });
}

function DiffLegend(): JSX.Element {
  return (
    <Box sx={{ display: "flex", gap: 2, alignItems: "center", mb: 2 }}>
      <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
        <Box sx={{ width: 14, height: 14, borderRadius: "3px", bgcolor: "success.main" }} />
        <Typography variant="caption">Added</Typography>
      </Box>
      <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
        <Box sx={{ width: 14, height: 14, borderRadius: "3px", bgcolor: "error.main" }} />
        <Typography variant="caption">Removed</Typography>
      </Box>
    </Box>
  );
}

function InlineDiffView({ oldText, newText }: { oldText: string; newText: string }): JSX.Element {
  const tokens = useMemo(() => wordDiff(stripHtml(oldText), stripHtml(newText)), [oldText, newText]);
  const addedCount = tokens.filter((t) => t.type === "added").length;
  const removedCount = tokens.filter((t) => t.type === "removed").length;

  return (
    <Box>
      <DiffLegend />
      {addedCount === 0 && removedCount === 0 ? (
        <Typography variant="body2" color="text.secondary">
          No content changes between these two versions.
        </Typography>
      ) : (
        <>
          <Typography variant="caption" color="text.secondary" sx={{ display: "block", mb: 1 }}>
            +{addedCount} word{addedCount === 1 ? "" : "s"} added, -{removedCount} word{removedCount === 1 ? "" : "s"} removed
          </Typography>
          <Typography variant="body1" sx={{ lineHeight: 2.2 }}>
            {tokens.map((t, i) => (
              <Box
                key={i}
                component="span"
                sx={{
                  bgcolor: t.type === "added" ? "success.main" : t.type === "removed" ? "error.main" : "transparent",
                  color: t.type === "same" ? "text.primary" : "common.white",
                  fontWeight: t.type === "same" ? 400 : 700,
                  textDecoration: t.type === "removed" ? "line-through" : t.type === "added" ? "underline" : "none",
                  borderRadius: t.type === "same" ? 0 : "3px",
                  px: t.type === "same" ? 0 : "3px",
                }}
              >
                {t.text}
              </Box>
            ))}
          </Typography>
        </>
      )}
    </Box>
  );
}

function SideBySideView({ oldBody, newBody }: { oldBody: string; newBody: string }): JSX.Element {
  return (
    <Box sx={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 3 }}>
      <Box>
        <Typography variant="caption" color="text.secondary" sx={{ display: "block", mb: 1 }}>
          Previous version
        </Typography>
        <Box
          sx={{ border: 1, borderColor: "divider", borderRadius: 1, p: 2, lineHeight: 1.7 }}
          // eslint-disable-next-line react/no-danger -- author-authored rich
          // text HTML from our own Lexical editor.
          dangerouslySetInnerHTML={{ __html: oldBody }}
        />
      </Box>
      <Box>
        <Typography variant="caption" color="text.secondary" sx={{ display: "block", mb: 1 }}>
          This version
        </Typography>
        <Box
          sx={{ border: 1, borderColor: "divider", borderRadius: 1, p: 2, lineHeight: 1.7 }}
          // eslint-disable-next-line react/no-danger -- see above.
          dangerouslySetInnerHTML={{ __html: newBody }}
        />
      </Box>
    </Box>
  );
}

/**
 * Full-page history detail: compares each PUBLISHED version against the
 * one immediately before it (Version N-1 -> Version N), plus an
 * "Unpublished changes" view (current draft vs. last published) when
 * applicable. Shows who made each version -- per Sajith's Sep 11 feedback,
 * "if we can add who made that change into this panel... because I have
 * to remember and then go there" -- directly in the comparison header,
 * not just in a separate sidebar list.
 */
export default function CsmKBArticleHistoryDetailPage(): JSX.Element {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavTransition();
  const [mode, setMode] = useState<"inline" | "side-by-side">("inline");

  const { data: article } = useGetKBArticle(id);
  const { events, nameById } = useKBArticleTimelineEvents(article ?? null);

  const publishedVersions = useMemo(
    () => [...events].filter((e) => e.label === "Approved & published").reverse(),
    [events],
  );

  const hasUnpublishedChanges = useMemo(() => {
    if (!article || publishedVersions.length === 0) return false;
    const latest = publishedVersions[publishedVersions.length - 1];
    return stripHtml(latest.body) !== stripHtml(article.body);
  }, [article, publishedVersions]);

  const [selection, setSelection] = useState<number | "unpublished" | null>(null);
  const defaultSelection: number | "unpublished" | null = hasUnpublishedChanges
    ? "unpublished"
    : publishedVersions.length >= 2
      ? publishedVersions.length - 1
      : null;
  const effectiveSelection = selection ?? defaultSelection;

  let oldTitle = "";
  let oldBody = "";
  let newTitle = "";
  let newBody = "";
  let heading = "";
  let actorName = "";
  let actorLabel = "";
  let changeTimestamp = "";
  let hasComparison = false;

  if (effectiveSelection === "unpublished" && article) {
    const latest = publishedVersions[publishedVersions.length - 1];
    oldTitle = latest?.title ?? "";
    oldBody = latest?.body ?? "";
    newTitle = article.title;
    newBody = article.body;
    heading = "Unpublished changes";
    actorLabel = "Last edited by";
    actorName = article.updatedOn ? nameById.get(events[0]?.actorId ?? "") ?? "—" : "—";
    changeTimestamp = article.updatedOn;
    hasComparison = true;
  } else if (typeof effectiveSelection === "number" && effectiveSelection > 0) {
    const prev = publishedVersions[effectiveSelection - 1];
    const curr = publishedVersions[effectiveSelection];
    oldTitle = prev.title;
    oldBody = prev.body;
    newTitle = curr.title;
    newBody = curr.body;
    heading = `Version ${effectiveSelection} → Version ${effectiveSelection + 1}`;
    actorLabel = "Published by";
    actorName = nameById.get(curr.actorId) ?? "—";
    changeTimestamp = curr.timestamp;
    hasComparison = true;
  }

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Button
        variant="text"
        size="small"
        startIcon={<ArrowLeft size={16} />}
        onClick={() => navigate(`/knowledge/my-articles/${id}`)}
        sx={{ alignSelf: "flex-start" }}
      >
        Back to article
      </Button>

      <Typography variant="h6">History detail: {article?.title}</Typography>

      <Box sx={{ display: "flex", gap: 3, alignItems: "flex-start" }}>
        <Box sx={{ width: 260, flexShrink: 0, display: "flex", flexDirection: "column", gap: 1 }}>
          {hasUnpublishedChanges && (
            <Box
              onClick={() => setSelection("unpublished")}
              sx={{
                cursor: "pointer",
                p: 1.5,
                borderRadius: 1,
                border: 2,
                borderColor: effectiveSelection === "unpublished" ? "primary.main" : "divider",
                bgcolor: effectiveSelection === "unpublished" ? "action.selected" : "transparent",
              }}
            >
              <Typography variant="body2" fontWeight={600}>
                Unpublished changes
              </Typography>
              <Typography variant="caption" color="text.secondary">
                Current draft vs. last published
              </Typography>
            </Box>
          )}

          <Typography variant="caption" color="text.secondary" sx={{ px: 0.5, mt: hasUnpublishedChanges ? 1 : 0 }}>
            Published versions
          </Typography>

          {publishedVersions.length === 0 ? (
            <Typography variant="body2" color="text.secondary" sx={{ px: 0.5 }}>
              This article has never been published.
            </Typography>
          ) : (
            publishedVersions
              .map((v, i) => ({ v, i }))
              .reverse()
              .map(({ v, i }) => (
                <Box
                  key={v.id}
                  onClick={() => setSelection(i)}
                  sx={{
                    cursor: i === 0 ? "default" : "pointer",
                    p: 1.5,
                    borderRadius: 1,
                    border: 1,
                    borderColor: effectiveSelection === i ? "primary.main" : "divider",
                    bgcolor: effectiveSelection === i ? "action.selected" : "transparent",
                    opacity: i === 0 ? 0.6 : 1,
                  }}
                >
                  <Chip size="small" label={`Version ${i + 1}`} color={i === 0 ? "default" : "success"} variant="outlined" sx={{ mb: 0.5 }} />
                  <Typography variant="caption" display="block">
                    {nameById.get(v.actorId) ?? "—"}
                  </Typography>
                  <Typography variant="caption" color="text.secondary" display="block">
                    {formatDateTime(v.timestamp)}
                  </Typography>
                  {i === 0 && (
                    <Typography variant="caption" color="text.secondary" sx={{ fontStyle: "italic" }}>
                      First published version
                    </Typography>
                  )}
                </Box>
              ))
          )}
        </Box>

        <Box sx={{ flex: 1, minWidth: 0 }}>
          {hasComparison ? (
            <>
              <Box sx={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", mb: 2 }}>
                <Box>
                  <Typography variant="subtitle1">{heading}</Typography>
                  <Typography variant="body2" color="text.secondary">
                    {actorLabel} <strong>{actorName}</strong> · {formatDateTime(changeTimestamp)}
                  </Typography>
                </Box>
                <ToggleButtonGroup size="small" value={mode} exclusive onChange={(_e, next) => next && setMode(next)}>
                  <ToggleButton value="inline">Diff</ToggleButton>
                  <ToggleButton value="side-by-side">Side by side</ToggleButton>
                </ToggleButtonGroup>
              </Box>

              {oldTitle !== newTitle && (
                <Typography variant="body2" sx={{ mb: 2 }}>
                  Title: <s>{oldTitle}</s> → {newTitle}
                </Typography>
              )}

              {mode === "inline" ? (
                <InlineDiffView oldText={oldBody} newText={newBody} />
              ) : (
                <SideBySideView oldBody={oldBody} newBody={newBody} />
              )}
            </>
          ) : (
            <Typography variant="body2" color="text.secondary">
              {publishedVersions.length === 0
                ? "Publish this article at least once to start tracking versions."
                : "Select a version on the left to compare it with the one before it."}
            </Typography>
          )}
        </Box>
      </Box>
    </Box>
  );
}
