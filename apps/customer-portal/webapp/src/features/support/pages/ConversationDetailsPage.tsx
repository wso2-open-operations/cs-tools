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
  Avatar,
  Box,
  Button,
  Paper,
  Skeleton,
  Stack,
  Typography,
  Divider,
  alpha,
} from "@wso2/oxygen-ui";
import {
  ArrowLeft,
  Bot,
  MessageSquare,
  FileText,
  Play,
  Flag,
  Clock,
  Hash,
} from "@wso2/oxygen-ui-icons-react";
import { useMemo, type JSX } from "react";
import { useGetConversationMessages } from "@features/support/api/useGetConversationMessages";
import type {
  ConversationMessage,
  ChatHistoryItem,
} from "@features/support/types/conversations";
import { ChatSender } from "@features/support/types/conversations";
import ApiErrorState from "@components/error/ApiErrorState";
import type { Message } from "@features/support/types/conversations";
import ConversationKnowledgeRecommendations from "@features/support/components/knowledge-base/ConversationKnowledgeRecommendations";
import { useTheme } from "@wso2/oxygen-ui";
import {
  compareByCreatedOnThenId,
  dateFromApiCreatedOn,
  formatDateOnly,
  formatCommentDate,
  getInitials,
} from "@features/support/utils/support";
import { ROUTE_PREVIOUS_PAGE } from "@features/project-hub/constants/navigationConstants";
import { ConversationListRowAction } from "@features/support/types/conversations";
import { resolveConversationListRowAction } from "@features/support/utils/conversationsList";
import { NOVERA_DISPLAY_NAME } from "@features/support/constants/chatConstants";
import MarkdownIt from "markdown-it";

const md = new MarkdownIt({ linkify: true, breaks: true });

function ConversationMsgBubble({
  message,
  isCurrentUser,
}: {
  message: Message;
  isCurrentUser: boolean;
}): JSX.Element {
  const theme = useTheme();
  const primaryLight = theme.palette.primary?.light ?? "#fa7b3f";
  const primaryBg = alpha(primaryLight, 0.1);
  const isBot = message.sender === ChatSender.BOT;
  const isRight = isCurrentUser && !isBot;

  const initials = isBot ? null : getInitials(message.createdBy || "?");
  const displayName = isBot
    ? NOVERA_DISPLAY_NAME
    : message.createdBy || "Unknown";

  const timestamp =
    message.timestamp instanceof Date
      ? message.timestamp.toISOString()
      : String(message.timestamp ?? "");

  const html = useMemo(() => md.render(message.text ?? ""), [message.text]);

  return (
    <Stack
      direction="row"
      alignItems="flex-start"
      sx={{ flexDirection: isRight ? "row-reverse" : "row", gap: 2 }}
    >
      {isBot ? (
        <Box
          sx={{
            width: 32,
            height: 32,
            borderRadius: "50%",
            background: "linear-gradient(135deg, #EA580C 0%, #F97316 100%)",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            flexShrink: 0,
          }}
        >
          <Bot size={16} color="white" />
        </Box>
      ) : (
        <Avatar
          sx={{
            width: 32,
            height: 32,
            fontSize: "0.75rem",
            flexShrink: 0,
            bgcolor: isRight
              ? primaryBg
              : alpha(theme.palette.info?.light ?? "#0288d1", 0.2),
            color: isRight
              ? theme.palette.primary.main
              : (theme.palette.info?.main ?? "#0288d1"),
          }}
        >
          {initials}
        </Avatar>
      )}
      <Stack
        spacing={0.75}
        sx={{
          width: "100%",
          maxWidth: isBot ? "100%" : 800,
          minWidth: 0,
          alignItems: isRight ? "flex-end" : "flex-start",
        }}
      >
        <Stack
          direction="row"
          alignItems="center"
          flexWrap="wrap"
          sx={{ flexDirection: isRight ? "row-reverse" : "row", gap: 1 }}
        >
          <Typography variant="body2" color="text.primary" fontWeight={500}>
            {displayName}
          </Typography>
          <Typography variant="caption" color="text.secondary">
            {formatCommentDate(timestamp)}
          </Typography>
        </Stack>
        <Paper
          elevation={0}
          sx={{
            p: 1.25,
            width: "100%",
            bgcolor: isRight ? primaryBg : "background.paper",
            fontSize: "0.875rem",
            lineHeight: 1.6,
            "& p": {
              margin: "0 0 0.25em 0",
              whiteSpace: "pre-wrap",
              wordBreak: "break-word",
            },
            "& p:last-child": { marginBottom: 0 },
            "& ul, & ol": { mt: 0, mb: 1, pl: 2.5 },
            "& li": { mb: 0.5 },
            "& a": { color: "primary.main", textDecoration: "underline" },
            "& code": {
              fontFamily: "monospace",
              backgroundColor: "action.hover",
              px: 0.5,
            },
            "& pre": {
              overflowX: "auto",
              backgroundColor: "action.disabledBackground",
              p: 1,
              m: 0,
            },
          }}
        >
          {isBot ? (
            <Box dangerouslySetInnerHTML={{ __html: html }} />
          ) : (
            <Typography
              variant="body2"
              sx={{ whiteSpace: "pre-wrap", wordBreak: "break-word" }}
            >
              {message.text}
            </Typography>
          )}
        </Paper>
      </Stack>
    </Stack>
  );
}

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
    error,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
  } = useGetConversationMessages(conversationId || "", { pageSize: 10 });

  const messages: ConversationMessage[] = useMemo(() => {
    const raw = data?.pages?.flatMap((p) => p.comments) ?? [];
    return [...raw].sort(compareByCreatedOnThenId);
  }, [data]);

  const chatMessages: Message[] = useMemo(
    () =>
      messages.map((msg) => {
        const isBot =
          msg.type?.toLowerCase() === "bot" ||
          msg.createdBy?.toLowerCase() === "novera";
        const createdByDisplayName = [
          msg.createdByFirstName,
          msg.createdByLastName,
        ]
          .filter((name) => Boolean(name && name.trim()))
          .join(" ")
          .trim();
        return {
          id: msg.id,
          text: msg.content,
          sender: isBot ? ChatSender.BOT : ChatSender.USER,
          timestamp: dateFromApiCreatedOn(msg.createdOn),
          createdBy: createdByDisplayName || msg.createdBy || "Unknown",
          createdOnRaw: msg.createdOn ?? "--",
          showFeedbackActions: false,
        };
      }),
    [messages],
  );

  const conversationStatus = summary?.status;
  const conversationStatusLabel = conversationStatus ?? "--";
  const startedTime = summary?.startedTime ?? "";
  const messageCount = summary?.messages;
  const kbArticles = summary?.kbArticles;

  const conversationAction =
    resolveConversationListRowAction(conversationStatus);

  const handleBack = () => {
    const returnTo = (location.state as { returnTo?: string } | null)?.returnTo;
    if (returnTo) {
      navigate(returnTo, { state: { fromBack: true } });
      return;
    }
    if (window.history.length > 1) {
      navigate(ROUTE_PREVIOUS_PAGE);
    } else if (projectId) {
      navigate(`/projects/${projectId}/support/conversations`, { state: { fromBack: true } });
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
        <Box
          sx={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            gap: 2,
          }}
        >
          <Typography variant="h4" color="text.primary">
            Chat Session
          </Typography>
          {conversationAction === ConversationListRowAction.Resume &&
            projectId &&
            conversationId && (
              <Button
                size="small"
                variant="outlined"
                color="warning"
                startIcon={<Play size={14} />}
                onClick={() =>
                  navigate(
                    `/projects/${projectId}/support/chat/${conversationId}`,
                    {
                      state: { chatNumber: summary?.chatNumber },
                    },
                  )
                }
                sx={{ textTransform: "none", fontWeight: 500 }}
              >
                Resume
              </Button>
            )}
        </Box>
      </Box>

      {summary && (
        <Paper variant="outlined" sx={{ p: 3 }}>
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              gap: 1.5,
              justifyContent: "space-between",
              width: "100%",
            }}
          >
            <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
              <Flag size={16} />
              <Box>
                <Typography variant="body2" color="text.secondary">
                  Status
                </Typography>
                <Typography variant="body2" color="text.primary">
                  {conversationStatusLabel}
                </Typography>
              </Box>
            </Box>
            <Divider orientation="vertical" sx={{ height: 40 }} />
            <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
              <Clock size={16} />
              <Box>
                <Typography variant="body2" color="text.secondary">
                  Started
                </Typography>
                <Typography variant="body2" color="text.primary">
                  {formatDateOnly(startedTime)}
                </Typography>
              </Box>
            </Box>
            <Divider orientation="vertical" sx={{ height: 40 }} />
            <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
              <MessageSquare size={16} />
              <Box>
                <Typography variant="body2" color="text.secondary">
                  Messages
                </Typography>
                <Typography variant="body2" color="text.primary">
                  {messageCount ?? 0}
                </Typography>
              </Box>
            </Box>
            <Divider orientation="vertical" sx={{ height: 40 }} />
            <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
              <FileText size={16} />
              <Box>
                <Typography variant="body2" color="text.secondary">
                  KB Articles
                </Typography>
                <Typography variant="body2" color="text.primary">
                  {kbArticles ?? 0}
                </Typography>
              </Box>
            </Box>
            <Divider orientation="vertical" sx={{ height: 40 }} />
            <Box sx={{ display: "flex", alignItems: "center", gap: 1.5 }}>
              <Hash size={16} />
              <Box>
                <Typography variant="body2" color="text.secondary">
                  Chat Number
                </Typography>
                <Typography variant="body2" color="text.primary">
                  {summary.chatNumber ?? "--"}
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
          <ApiErrorState
            error={error}
            fallbackMessage="Could not load conversation messages."
          />
        ) : messages.length === 0 ? (
          <Typography variant="body2" color="text.secondary">
            No messages found for this conversation.
          </Typography>
        ) : (
          <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
            {chatMessages.map((m, index) => (
              <Box key={m.id}>
                <ConversationMsgBubble
                  message={m}
                  isCurrentUser={m.sender === ChatSender.USER}
                />
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

      <ConversationKnowledgeRecommendations messages={messages} />
    </Stack>
  );
}
