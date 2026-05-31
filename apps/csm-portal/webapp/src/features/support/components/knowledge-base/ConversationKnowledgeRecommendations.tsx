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
  Button,
  Chip,
  Paper,
  Skeleton,
  Stack,
  Typography,
} from "@wso2/oxygen-ui";
import { ExternalLink, FileText } from "@wso2/oxygen-ui-icons-react";
import { useMemo, type JSX } from "react";
import { KB_ARTICLE_VIEW_BASE_URL } from "@features/support/constants/supportConstants";
import { useConversationRecommendationsSearch } from "@features/support/api/useConversationRecommendationsSearch";
import type { ConversationMessage } from "@features/support/types/conversations";
import {
  buildRecommendationRequestFromConversationMessages,
  recommendationScoreToPercent,
} from "@features/support/utils/recommendations";
import ApiErrorState from "@components/error/ApiErrorState";

export type ConversationKnowledgeRecommendationsProps = {
  messages: ConversationMessage[];
};

/**
 * Lists KB article recommendations for a chat session using the shared recommendations API.
 *
 * @param {ConversationKnowledgeRecommendationsProps} props - Conversation messages.
 * @returns {JSX.Element} Recommendations section for the conversation details page.
 */
export default function ConversationKnowledgeRecommendations({
  messages,
}: ConversationKnowledgeRecommendationsProps): JSX.Element {
  const payload = useMemo(
    () => buildRecommendationRequestFromConversationMessages(messages),
    [messages],
  );

  const { data, isLoading, isError, error } = useConversationRecommendationsSearch(
    payload,
    true,
  );

  const openArticle = (articleId: string) => {
    window.open(
      `${KB_ARTICLE_VIEW_BASE_URL}${encodeURIComponent(articleId)}`,
      "_blank",
      "noopener,noreferrer",
    );
  };

  const items = data?.recommendations ?? [];

  return (
    <Paper
      variant="outlined"
      sx={{
        p: 3,
        display: "flex",
        flexDirection: "column",
        gap: 2,
      }}
    >
      <Stack direction="row" spacing={1.5} alignItems="center" sx={{ mb: 0 }}>
        <FileText size={18} />
        <Typography variant="h6" color="text.primary">
          Knowledge Base Articles Suggested
        </Typography>
      </Stack>

      {payload == null || isLoading ? (
        <Stack spacing={1.5}>
          {[1, 2].map((k) => (
            <Paper
              key={k}
              variant="outlined"
              sx={{ p: 2, display: "flex", flexDirection: "column", gap: 1 }}
            >
              <Box
                sx={{
                  display: "flex",
                  alignItems: "flex-start",
                  justifyContent: "space-between",
                  gap: 2,
                }}
              >
                <Box sx={{ flex: 1, minWidth: 0 }}>
                  <Stack
                    direction="row"
                    alignItems="center"
                    spacing={1}
                    sx={{ mb: 0.5 }}
                  >
                    <Skeleton variant="text" width="60%" height={20} />
                    <Skeleton variant="rounded" width={80} height={22} />
                  </Stack>
                </Box>
                <Skeleton variant="rounded" width={36} height={30} />
              </Box>
            </Paper>
          ))}
        </Stack>
      ) : isError ? (
        <ApiErrorState
          error={error}
          fallbackMessage="Could not load knowledge base recommendations. Please try again later."
        />
      ) : items.length === 0 ? (
        <Typography variant="body2" color="text.secondary">
          No matching knowledge base articles were found for this conversation.
        </Typography>
      ) : (
        <Stack spacing={1.5}>
          {data?.query ? (
            <Typography variant="body2" color="text.secondary">
              <strong>Topic:</strong> {data.query}
            </Typography>
          ) : null}
          {items.map((item, index) => (
            <Paper
              key={`${item.articleId}-${index}`}
              variant="outlined"
              role="button"
              tabIndex={0}
              onClick={() => openArticle(item.articleId)}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ")
                  openArticle(item.articleId);
              }}
              sx={{
                p: 2,
                display: "flex",
                flexDirection: "column",
                gap: 1,
                cursor: "pointer",
                "&:hover": {
                  borderColor: "primary.main",
                  boxShadow: 1,
                },
              }}
            >
              <Box
                sx={{
                  display: "flex",
                  alignItems: "flex-start",
                  justifyContent: "space-between",
                  gap: 2,
                }}
              >
                <Box sx={{ flex: 1, minWidth: 0 }}>
                  <Stack
                    direction="row"
                    alignItems="center"
                    spacing={1}
                    flexWrap="wrap"
                    sx={{ mb: 0.5 }}
                  >
                    <Typography
                      variant="body2"
                      color="text.primary"
                      fontWeight={500}
                    >
                      {item.title}
                    </Typography>
                    <Chip
                      size="small"
                      label={`${recommendationScoreToPercent(item.score)}% relevant`}
                      sx={{ height: 22, fontSize: "0.75rem" }}
                    />
                  </Stack>
                </Box>
                <Button
                  size="small"
                  variant="outlined"
                  aria-label={`Open article ${item.title}`}
                  onClick={(e) => {
                    e.stopPropagation();
                    openArticle(item.articleId);
                  }}
                  sx={{ flexShrink: 0 }}
                >
                  <ExternalLink size={16} />
                </Button>
              </Box>
            </Paper>
          ))}
        </Stack>
      )}
    </Paper>
  );
}
