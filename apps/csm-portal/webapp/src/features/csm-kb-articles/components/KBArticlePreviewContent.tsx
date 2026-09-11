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

import { Box, Chip, IconButton, Typography } from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { useNavTransition } from "@hooks/useNavTransition";
import type { KBArticle, KBArticleState } from "@features/csm-kb-articles/types/csmKbArticles";

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

function formatDateTime(value: string): string {
  const d = new Date(value);
  return Number.isNaN(d.getTime())
    ? value
    : d.toLocaleString("en-US", { year: "numeric", month: "short", day: "numeric", hour: "numeric", minute: "2-digit" });
}

interface KBArticlePreviewContentProps {
  article: KBArticle;
  knowledgeBaseName: string;
  authorName: string;
  updatedByName: string;
  onClose: () => void;
}

/**
 * Read-only "quick look" for a KB article — status, knowledge base, author,
 * last update, and a truncated body snippet — without leaving the list.
 * Deliberately thin, matching csm-cases' CasePreviewContent: a preview, not
 * a second full article page. Opening the real article is always one click
 * away via "View full article".
 */
export default function KBArticlePreviewContent({
  article,
  knowledgeBaseName,
  authorName,
  updatedByName,
  onClose,
}: KBArticlePreviewContentProps): JSX.Element {
  const navigate = useNavTransition();

  return (
    <Box sx={{ display: "flex", flexDirection: "column", height: "100%" }}>
      <Box sx={{ display: "flex", alignItems: "flex-start", justifyContent: "space-between", p: 2, borderBottom: 1, borderColor: "divider" }}>
        <Typography variant="subtitle1" sx={{ pr: 2 }}>
          {article.title}
        </Typography>
        <IconButton size="small" onClick={onClose} aria-label="Close preview">
          <X size={16} />
        </IconButton>
      </Box>

      <Box sx={{ p: 2, display: "flex", flexDirection: "column", gap: 2, overflowY: "auto", flex: 1 }}>
        <Box sx={{ display: "flex", gap: 1, flexWrap: "wrap" }}>
          {article.state === "draft" && article.rejectionComment ? (
            <Chip size="small" label="Rejected" color="error" variant="outlined" />
          ) : (
            <Chip size="small" label={STATE_LABELS[article.state]} color={STATE_COLORS[article.state]} variant="outlined" />
          )}
          <Chip size="small" label={knowledgeBaseName} variant="outlined" />
        </Box>

        <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5 }}>
          <Typography variant="caption" color="text.secondary">Author: {authorName}</Typography>
          <Typography variant="caption" color="text.secondary">Last updated {formatDateTime(article.updatedOn)} by {updatedByName}</Typography>
          <Typography variant="caption" color="text.secondary">Created {formatDateTime(article.createdOn)}</Typography>
        </Box>

        {article.rejectionComment && (
          <Typography variant="body2" color="warning.main">
            Rejected: "{article.rejectionComment}"
          </Typography>
        )}

        <Box
          sx={{
            fontSize: "0.9rem",
            lineHeight: 1.6,
            maxHeight: 480,
            overflowY: "auto",
            pr: 1,
          }}
          // eslint-disable-next-line react/no-danger -- article.body is
          // author-authored rich text HTML from our own Lexical editor.
          dangerouslySetInnerHTML={{ __html: article.body }}
        />
      </Box>

      <Box sx={{ p: 2, borderTop: 1, borderColor: "divider" }}>
        <Typography
          variant="body2"
          color="primary"
          sx={{ cursor: "pointer" }}
          onClick={() => navigate(`/knowledge/my-articles/${article.id}`)}
        >
          View full article →
        </Typography>
      </Box>
    </Box>
  );
}
