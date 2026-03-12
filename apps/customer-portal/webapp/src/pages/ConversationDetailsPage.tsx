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

import { useNavigate, useParams, useLocation } from "react-router";
import {
  Box,
  Button,
  Paper,
  Skeleton,
  Stack,
  Typography,
  Chip,
  Divider,
} from "@wso2/oxygen-ui";
import {
  ArrowLeft,
  MessageSquare,
  FileText,
  CircleCheck,
  CircleAlert,
  Clock,
} from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { useGetConversationMessages } from "@api/useGetConversationMessages";
import type { ConversationMessage, ChatHistoryItem } from "@models/responses";
import ErrorStateIcon from "@components/common/error-state/ErrorStateIcon";
import type { Message } from "@pages/NoveraChatPage";
import ChatMessageBubble from "@components/support/novera-ai-assistant/novera-chat-page/ChatMessageBubble";
import { alpha, useTheme } from "@wso2/oxygen-ui";
import { formatDateOnly, normalizeUtcDateString } from "@utils/support";

/**
 * ConversationDetailsPage displays the message history for a single conversation.
 *
 * @returns {JSX.Element} The rendered Conversation Details page.
 */
export default function ConversationDetailsPage(): JSX.Element {
  const navigate = useNavigate();
  const location = useLocation();
  const { projectId, conversationId } = useParams<{
    projectId: string;
    conversationId: string;
  }>();

  const locationState = location.state as {
    conversationSummary?: ChatHistoryItem;
  } | null;
  const summary = locationState?.conversationSummary;

  const {
    data,
    isLoading,
    isError,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
  } = useGetConversationMessages(conversationId || "", { pageSize: 10 });

  const messages: ConversationMessage[] =
    data?.pages?.flatMap((p) => p.comments) ?? [];

  const theme = useTheme();

  const chatMessages: Message[] = messages.map((msg) => {
    const isBot =
      msg.type?.toLowerCase() === "bot" ||
      msg.createdBy?.toLowerCase() === "novera";
    const normalizedDate = normalizeUtcDateString(msg.createdOn);
    return {
      id: msg.id,
      text: msg.content,
      sender: isBot ? "bot" : "user",
      timestamp: new Date(normalizedDate),
    };
  });

  const conversationStatus = summary?.status ?? "Open";
  const initialMessage = summary?.title ?? "";
  const startedTime = summary?.startedTime ?? "";
  const messageCount = summary?.messages;
  const kbArticles = summary?.kbArticles;

  const statusLower = conversationStatus.toLowerCase();
  let StatusIcon = Clock;
  let statusColor: string = theme.palette.info.main;

  if (statusLower === "resolved") {
    StatusIcon = CircleCheck;
    statusColor = theme.palette.success.main;
  } else if (statusLower === "abandoned") {
    StatusIcon = CircleAlert;
    statusColor = theme.palette.warning.main;
  } else if (statusLower === "converted") {
    StatusIcon = CircleCheck;
    statusColor = theme.palette.secondary.main;
  } else if (statusLower === "open" || statusLower === "active") {
    StatusIcon = Clock;
    statusColor = theme.palette.info.main;
  }

  const handleBack = () => {
    if (window.history.length > 1) {
      navigate(-1);
    } else if (projectId) {
      navigate(`/projects/${projectId}/support/conversations`);
    } else {
      navigate("/");
    }
  };

  return (
    <Stack spacing={3}>
      <Box>
        <Button
          startIcon={<ArrowLeft size={16} />}
          onClick={handleBack}
          sx={{ mb: 2 }}
          variant="text"
        >
          Back
        </Button>
        <Box>
          <Stack
            direction="row"
            spacing={1.5}
            alignItems="center"
            sx={{ mb: 1 }}
          >
            <Typography variant="h4" color="text.primary">
              Chat Session
            </Typography>
            <Chip
              size="small"
              variant="outlined"
              label={conversationStatus}
              icon={<StatusIcon size={12} />}
              sx={{
                height: 22,
                fontSize: "0.75rem",
                bgcolor: alpha(statusColor, 0.1),
                color: statusColor,
                "& .MuiChip-icon": {
                  color: "inherit",
                  ml: "6px",
                  mr: "6px",
                },
                "& .MuiChip-label": {
                  pl: 0,
                  pr: "6px",
                },
              }}
            />
          </Stack>
          {initialMessage && (
            <Typography variant="body2" color="text.secondary">
              {initialMessage}
            </Typography>
          )}
        </Box>
      </Box>

      {summary && (
        <Paper variant="outlined" sx={{ p: 3 }}>
          <Box
            sx={{
              display: "grid",
              gridTemplateColumns: { xs: "1fr 1fr", md: "1fr 1fr 1fr 1fr" },
              gap: 2,
            }}
          >
            <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
              <MessageSquare size={16} color={theme.palette.text.secondary} />
              <Box>
                <Typography variant="caption" color="text.secondary">
                  Started
                </Typography>
                <Typography variant="body2" color="text.primary">
                  {formatDateOnly(startedTime)}
                </Typography>
              </Box>
            </Box>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
              <MessageSquare size={16} color={theme.palette.text.secondary} />
              <Box>
                <Typography variant="caption" color="text.secondary">
                  Messages
                </Typography>
                <Typography variant="body2" color="text.primary">
                  {messageCount ?? 0}
                </Typography>
              </Box>
            </Box>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
              <FileText size={16} color={theme.palette.text.secondary} />
              <Box>
                <Typography variant="caption" color="text.secondary">
                  KB Articles
                </Typography>
                <Typography variant="body2" color="text.primary">
                  {kbArticles ?? 0}
                </Typography>
              </Box>
            </Box>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
              <MessageSquare size={16} color={theme.palette.text.secondary} />
              <Box>
                <Typography variant="caption" color="text.secondary">
                  Chat ID
                </Typography>
                <Typography variant="body2" color="text.primary">
                  {summary.chatId}
                </Typography>
              </Box>
            </Box>
          </Box>
        </Paper>
      )}

      <Paper
        variant="outlined"
        sx={{ p: 3, display: "flex", flexDirection: "column", gap: 2 }}
      >
        <Stack direction="row" spacing={1.5} alignItems="center" sx={{ mb: 1 }}>
          <MessageSquare size={18} />
          <Typography variant="h6" color="text.primary">
            Conversation
          </Typography>
        </Stack>

        {isLoading ? (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
            {[1, 2, 3].map((i) => (
              <Box key={i} sx={{ display: "flex", gap: 2 }}>
                <Skeleton variant="circular" width={32} height={32} />
                <Box sx={{ flex: 1 }}>
                  <Skeleton width="25%" height={16} sx={{ mb: 0.5 }} />
                  <Skeleton width="80%" height={20} />
                </Box>
              </Box>
            ))}
          </Box>
        ) : isError ? (
          <Box sx={{ textAlign: "center", py: 6 }}>
            <ErrorStateIcon width={180} height={120} />
            <Typography variant="body2" color="text.secondary" sx={{ mt: 2 }}>
              Could not load conversation messages.
            </Typography>
          </Box>
        ) : messages.length === 0 ? (
          <Typography variant="body2" color="text.secondary">
            No messages found for this conversation.
          </Typography>
        ) : (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
            {chatMessages.map((m, index) => (
              <Box key={m.id}>
                <ChatMessageBubble message={m} />
                {index < chatMessages.length - 1 && <Divider sx={{ my: 3 }} />}
              </Box>
            ))}
            {hasNextPage && (
              <Box sx={{ display: "flex", justifyContent: "center", mt: 2 }}>
                <Button
                  variant="outlined"
                  onClick={() => fetchNextPage()}
                  disabled={isFetchingNextPage}
                >
                  {isFetchingNextPage ? "Loading..." : "Load More Messages"}
                </Button>
              </Box>
            )}
          </Box>
        )}
      </Paper>

      <Paper
        variant="outlined"
        sx={{ p: 3, display: "flex", flexDirection: "column", gap: 2 }}
      >
        <Stack direction="row" spacing={1.5} alignItems="center" sx={{ mb: 1 }}>
          <FileText size={18} />
          <Typography variant="h6" color="text.primary">
            Knowledge Base Articles Suggested
          </Typography>
        </Stack>
        <Box sx={{ textAlign: "center", py: 4 }}>
          <ErrorStateIcon width={160} height={110} />
          <Typography variant="body2" color="text.secondary" sx={{ mt: 2 }}>
            KB article suggestions are not available yet.
          </Typography>
        </Box>
      </Paper>
    </Stack>
  );
}
