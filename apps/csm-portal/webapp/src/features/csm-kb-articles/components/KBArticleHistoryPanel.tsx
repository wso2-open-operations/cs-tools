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
import { type JSX } from "react";
import FloatingSlidePanel from "@components/FloatingSlidePanel";
import { useNavTransition } from "@hooks/useNavTransition";
import { useKBArticleTimelineEvents, type KBArticleTimelineEvent } from "@features/csm-kb-articles/api/useKBArticleTimelineEvents";
import type { KBArticle } from "@features/csm-kb-articles/types/csmKbArticles";

function formatDateTime(value: string): string {
  const d = new Date(value);
  return Number.isNaN(d.getTime())
    ? value
    : d.toLocaleString("en-US", { year: "numeric", month: "short", day: "numeric", hour: "numeric", minute: "2-digit" });
}

function TimelineEventRow({
  event,
  actorName,
  articleId,
}: {
  event: KBArticleTimelineEvent;
  actorName: string;
  articleId: string;
}): JSX.Element {
  const navigate = useNavTransition();
  const hasPrevious = event.previousTitle !== null;

  return (
    <Box sx={{ borderLeft: 2, borderColor: "divider", pl: 2, pb: 3, position: "relative" }}>
      <Box sx={{ position: "absolute", left: -5, top: 2, width: 8, height: 8, borderRadius: "50%", bgcolor: "primary.main" }} />
      <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
        <Chip size="small" label={event.label} color={event.color} variant="outlined" />
        <Typography variant="body2">{actorName}</Typography>
        <Typography variant="caption" color="text.secondary">
          {formatDateTime(event.timestamp)}
        </Typography>
      </Box>

      {hasPrevious && (
        <Typography
          variant="caption"
          color="primary"
          sx={{ cursor: "pointer", display: "inline-block", mt: 0.5, textDecoration: "underline" }}
          onClick={() => navigate(`/knowledge/my-articles/${articleId}/history?entry=${event.id}`)}
        >
          See detailed changes
        </Typography>
      )}
    </Box>
  );
}

interface KBArticleHistoryPanelProps {
  open: boolean;
  article: KBArticle | null;
  onClose: () => void;
}

/**
 * Quick lifecycle overview for one article -- who did what, and when. For
 * the actual content diff, this panel deliberately stays thin and links out
 * to a dedicated full-page detail view (KBArticleHistoryDetailPage) instead
 * of showing the diff inline here: Sajith's Sep 10 feedback was that this
 * small panel is good for the event timeline, but a diff needs a larger
 * screen -- "we can allow to see the content at this stage... go to a new
 * page or something like that."
 */
export default function KBArticleHistoryPanel({ open, article, onClose }: KBArticleHistoryPanelProps): JSX.Element {
  const { events, nameById } = useKBArticleTimelineEvents(article);

  return (
    <FloatingSlidePanel open={open} ariaLabel="Article history">
      <Box sx={{ display: "flex", flexDirection: "column", height: "100%" }}>
        <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", p: 2, borderBottom: 1, borderColor: "divider" }}>
          <Typography variant="subtitle1">History</Typography>
          <IconButton size="small" onClick={onClose} aria-label="Close history">
            <X size={16} />
          </IconButton>
        </Box>
        <Box sx={{ p: 2, overflowY: "auto", flex: 1 }}>
          {events.map((event) => (
            <TimelineEventRow
              key={event.id}
              event={event}
              actorName={nameById.get(event.actorId) ?? "—"}
              articleId={article?.id ?? ""}
            />
          ))}
        </Box>
      </Box>
    </FloatingSlidePanel>
  );
}
