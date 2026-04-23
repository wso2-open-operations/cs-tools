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
  alpha,
  useTheme,
} from "@wso2/oxygen-ui";
import { BookOpen, ExternalLink } from "@wso2/oxygen-ui-icons-react";
import EmptyIcon from "@components/empty-state/EmptyIcon";
import { useMemo, type JSX } from "react";
import useGetCaseComments from "@features/support/api/useGetCaseComments";
import { KB_ARTICLE_VIEW_BASE_URL } from "@features/support/constants/supportConstants";
import { useConversationRecommendationsSearch } from "@features/support/api/useConversationRecommendationsSearch";
import type { CaseDetails } from "@features/support/types/cases";
import {
  buildRecommendationRequestFromCase,
  recommendationScoreToPercent,
} from "@features/support/utils/recommendations";

export type CaseKnowledgeBaseRecommendationsProps = {
  caseId: string;
  projectId: string;
  data: CaseDetails | undefined;
};

/**
 * Knowledge Base tab: loads case-linked context and shows API-driven article recommendations.
 *
 * @param {CaseKnowledgeBaseRecommendationsProps} props - Case identifiers and details.
 * @returns {JSX.Element} Recommendations panel for the case details Knowledge Base tab.
 */
export default function CaseKnowledgeBaseRecommendations({
  caseId,
  projectId,
  data,
}: CaseKnowledgeBaseRecommendationsProps): JSX.Element {
  const theme = useTheme();
  const accent = theme.palette.primary.main;

  const { data: commentsData, isLoading: isCommentsLoading } =
    useGetCaseComments(projectId, caseId, {
      offset: 0,
    });

  const comments = useMemo(
    () => commentsData?.comments ?? [],
    [commentsData?.comments],
  );

  const payload = useMemo(
    () => buildRecommendationRequestFromCase(data, comments),
    [data, comments],
  );

  const {
    data: recData,
    isLoading: isRecLoading,
    isError,
  } = useConversationRecommendationsSearch(
    payload,
    !isCommentsLoading && !!payload,
  );

  const openArticle = (articleId: string) => {
    window.open(
      `${KB_ARTICLE_VIEW_BASE_URL}${encodeURIComponent(articleId)}`,
      "_blank",
      "noopener,noreferrer",
    );
  };

  const items = recData?.recommendations ?? [];
  const isLoading = isCommentsLoading || (payload != null && isRecLoading);

  const skeletonCard = (k: number) => (
    <Paper
      key={k}
      variant="outlined"
      sx={{ p: 2, display: "flex", alignItems: "center", gap: 2 }}
    >
      <Skeleton
        variant="rounded"
        width={36}
        height={36}
        sx={{ flexShrink: 0 }}
      />
      <Stack
        direction="row"
        alignItems="center"
        spacing={1}
        sx={{ flex: 1, minWidth: 0 }}
      >
        <Skeleton variant="text" width="55%" height={20} />
        <Skeleton
          variant="rounded"
          width={80}
          height={22}
          sx={{ flexShrink: 0 }}
        />
      </Stack>
      <Skeleton
        variant="rounded"
        width={100}
        height={32}
        sx={{ flexShrink: 0 }}
      />
    </Paper>
  );

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
      {payload == null && !isCommentsLoading ? (
        <Typography variant="body2" color="text.secondary">
          Add a title or description, or post activity on this case, to request
          article recommendations.
        </Typography>
      ) : isLoading ? (
        <Stack spacing={2}>{[1, 2, 3].map((k) => skeletonCard(k))}</Stack>
      ) : isError ? (
        <Typography variant="body2" color="text.secondary">
          Could not load knowledge base recommendations. Please try again later.
        </Typography>
      ) : items.length === 0 ? (
        <Stack
          spacing={2}
          alignItems="center"
          justifyContent="center"
          sx={{ py: 4 }}
        >
          <Box
            sx={{
              width: 160,
              maxWidth: "100%",
              "& svg": { width: "100%", height: "auto" },
            }}
            aria-hidden
          >
            <EmptyIcon />
          </Box>
          <Typography variant="body2" color="text.secondary">
            No matching knowledge base articles were found for this case.
          </Typography>
        </Stack>
      ) : (
        <Stack spacing={2}>
          {recData?.query ? (
            <Typography variant="body2" color="text.secondary">
              <strong>Topic:</strong> {recData.query}
            </Typography>
          ) : null}
          {items.map((item, index) => (
            <Paper
              key={`${item.articleId}-${index}`}
              variant="outlined"
              onClick={() => openArticle(item.articleId)}
              sx={{
                p: 2,
                display: "flex",
                alignItems: "center",
                gap: 2,
                cursor: "pointer",
                "&:hover": { boxShadow: 2, borderColor: "primary.main" },
              }}
            >
              <Box
                sx={{
                  width: 36,
                  height: 36,
                  flexShrink: 0,
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  bgcolor: alpha(accent, 0.08),
                  borderRadius: 1,
                }}
              >
                <BookOpen size={18} color={accent} />
              </Box>
              <Stack
                direction="row"
                alignItems="center"
                spacing={1}
                sx={{ flex: 1, minWidth: 0 }}
              >
                <Typography variant="subtitle2" color="text.primary" noWrap>
                  {item.title}
                </Typography>
                <Chip
                  size="small"
                  label={`${recommendationScoreToPercent(item.score)}% relevant`}
                  sx={{ height: 22, fontSize: "0.75rem", flexShrink: 0 }}
                />
              </Stack>
              <Button
                size="small"
                variant="outlined"
                endIcon={<ExternalLink size={14} />}
                onClick={(e) => {
                  e.stopPropagation();
                  openArticle(item.articleId);
                }}
                sx={{ flexShrink: 0 }}
              >
                View Article
              </Button>
            </Paper>
          ))}
        </Stack>
      )}
    </Box>
  );
}
