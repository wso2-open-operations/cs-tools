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
  Tooltip,
  Typography,
  Divider,
  alpha,
} from "@wso2/oxygen-ui";
import {
  ArrowLeft,
  Bot,
  MessageSquare,
  FileText,
  Flag,
  Clock,
  Hash,
  User,
} from "@wso2/oxygen-ui-icons-react";
import {
  useState,
  useRef,
  useEffect,
  useCallback,
  useMemo,
  type JSX,
} from "react";
import { flushSync } from "react-dom";
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
import {
  NOVERA_DISPLAY_NAME,
  CHAT_TYPING_INTERVAL_MS,
  CHAT_TYPING_CHARS_PER_TICK,
  NOVERA_ANALYZING_PLACEHOLDER_TEXT,
} from "@features/support/constants/chatConstants";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import {
  buildBotMarkdownComponents,
  TextWithLinks,
} from "@features/support/utils/markdown";
import { useChatWebSocket } from "@features/support/api/useChatWebSocket";
import useGetProjectDetails from "@api/useGetProjectDetails";
import useGetUserDetails from "@features/settings/api/useGetUserDetails";
import { htmlToPlainText } from "@features/support/utils/richTextEditor";
import {
  getFinalMessageFromPayload,
  sanitizeStreamToken,
  splitTokenForTyping,
} from "@features/support/utils/chat";
import ChatInput from "@features/support/components/novera-ai-assistant/novera-chat-page/ChatInput";
import ChatMessageBubble from "@features/support/components/novera-ai-assistant/novera-chat-page/ChatMessageBubble";
import LoadingDotsBubble from "@features/support/components/novera-ai-assistant/novera-chat-page/LoadingDotsBubble";

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

  const markdownComponents: React.ComponentProps<
    typeof ReactMarkdown
  >["components"] = useMemo(() => buildBotMarkdownComponents(), []);

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
              px: 0.75,
              py: 0,
              whiteSpace: "normal",
              display: "inline",
            },
            "& pre": {
              overflowX: "auto",
              maxWidth: "100%",
              whiteSpace: "pre-wrap",
              wordBreak: "break-word",
              backgroundColor: "action.disabledBackground",
              p: 1,
              m: 0,
              boxSizing: "border-box",
            },
            "& pre code": {
              backgroundColor: "transparent",
              display: "block",
              boxSizing: "border-box",
              px: 0,
              py: 0.5,
              whiteSpace: "pre-wrap",
              wordBreak: "break-word",
            },
          }}
        >
          {message.isLoading ? (
            <Typography
              variant="body2"
              color="text.secondary"
              sx={{ fontStyle: "italic" }}
            >
              ...
            </Typography>
          ) : message.isError ? (
            <Typography
              variant="body2"
              color="error.main"
              sx={{ whiteSpace: "pre-wrap", wordBreak: "break-word" }}
            >
              {message.text}
            </Typography>
          ) : isBot ? (
            <ReactMarkdown
              components={markdownComponents}
              remarkPlugins={[remarkGfm]}
            >
              {message.text ?? ""}
            </ReactMarkdown>
          ) : (
            <Typography
              variant="body2"
              sx={{
                whiteSpace: "pre-wrap",
                wordBreak: "break-word",
                overflowWrap: "anywhere",
              }}
            >
              <TextWithLinks text={message.text ?? ""} />
            </Typography>
          )}
        </Paper>
      </Stack>
    </Stack>
  );
}

/**
 * ConversationDetailsPage displays the message history for a single conversation
 * and allows continuing the conversation inline via the AI chatbot.
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

  const { data: projectDetails } = useGetProjectDetails(projectId || "");
  const { data: userDetails } = useGetUserDetails();
  const accountId = projectDetails?.account?.id || projectId || "";
  const currentUserDisplayName = userDetails?.email ?? "You";

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

  // Chat continuation state
  const [inputValue, setInputValueState] = useState("");
  const [resetTrigger, setResetTrigger] = useState(0);
  const [isSending, setIsSending] = useState(false);
  const [isInputDisabled, setIsInputDisabled] = useState(false);
  const [newMessages, setNewMessages] = useState<Message[]>([]);
  const inputValueRef = useRef("");
  const activeBotMessageIdRef = useRef<string | null>(null);
  const tokenQueueRef = useRef<string[]>([]);
  const pendingFinalRef = useRef<{
    payload: Record<string, unknown>;
    finalMessage: string;
  } | null>(null);

  const setInputValueAndRef = useCallback((v: string) => {
    inputValueRef.current = v;
    setInputValueState(v);
  }, []);

  const upsertActiveBotMessage = useCallback(
    (updater: (msg: Message) => Message, fallback?: () => Message) => {
      setNewMessages((prev) => {
        const activeId = activeBotMessageIdRef.current;
        if (!activeId) return prev;
        let found = false;
        const next = prev.map((m) => {
          if (m.id !== activeId) return m;
          found = true;
          return updater(m);
        });
        if (!found && fallback) next.push(fallback());
        return next;
      });
    },
    [],
  );

  const flushPendingFinalIfReady = useCallback(() => {
    if (tokenQueueRef.current.length > 0) return;
    const pending = pendingFinalRef.current;
    if (!pending) return;
    const activeId = activeBotMessageIdRef.current;
    if (!activeId) return;

    let appliedFinal = false;
    flushSync(() => {
      setNewMessages((prev) => {
        const idx = prev.findIndex((m) => m.id === activeId);
        if (idx === -1) return prev;
        pendingFinalRef.current = null;
        const { finalMessage } = pending;
        const msg = prev[idx];
        const next = [...prev];
        next[idx] = {
          ...msg,
          isLoading: false,
          isError: false,
          text: finalMessage || msg.text,
          isStreaming: false,
          thinkingSteps: [],
          thinkingLabel: null,
        };
        appliedFinal = true;
        return next;
      });
    });
    if (appliedFinal) {
      setIsSending(false);
      if (pending?.payload?.token_warning === "session_limit_reached") {
        setIsInputDisabled(true);
      }
    }
  }, []);

  const dequeueOneTypedToken = useCallback(() => {
    const token = tokenQueueRef.current.shift();
    if (token === undefined) return;
    upsertActiveBotMessage((msg) => {
      const isPlaceholder =
        msg.text === NOVERA_ANALYZING_PLACEHOLDER_TEXT || msg.text === "";
      return {
        ...msg,
        isLoading: false,
        text: isPlaceholder ? token : `${msg.text}${token}`,
        isStreaming: true,
      };
    });
  }, [upsertActiveBotMessage]);

  useEffect(() => {
    const id = window.setInterval(() => {
      if (tokenQueueRef.current.length > 0) dequeueOneTypedToken();
      flushPendingFinalIfReady();
    }, CHAT_TYPING_INTERVAL_MS);
    return () => window.clearInterval(id);
  }, [dequeueOneTypedToken, flushPendingFinalIfReady]);

  const { connect, sendUserMessage } = useChatWebSocket({
    onEvent: (event) => {
      switch (event.type) {
        case "thinking_start":
          upsertActiveBotMessage(
            (msg) => ({
              ...msg,
              isLoading: false,
              text: NOVERA_ANALYZING_PLACEHOLDER_TEXT,
              thinkingSteps: [],
              thinkingLabel: null,
              isStreaming: false,
            }),
            () => ({
              id: activeBotMessageIdRef.current ?? `bot-${Date.now()}`,
              sender: ChatSender.BOT,
              timestamp: new Date(),
              text: NOVERA_ANALYZING_PLACEHOLDER_TEXT,
              thinkingSteps: [],
              thinkingLabel: null,
              isStreaming: false,
            }),
          );
          break;
        case "thinking_step": {
          const label = String(event.label ?? event.step ?? "Working...");
          upsertActiveBotMessage((msg) => ({
            ...msg,
            isLoading: false,
            thinkingSteps: [...(msg.thinkingSteps ?? []), label],
            thinkingLabel: label,
          }));
          break;
        }
        case "thinking_end":
          upsertActiveBotMessage((msg) => ({
            ...msg,
            isLoading: false,
            thinkingLabel: msg.thinkingLabel,
          }));
          break;
        case "token": {
          const token = String(event.content ?? "");
          const cleaned = sanitizeStreamToken(token);
          if (cleaned.length === 0) break;
          for (const part of splitTokenForTyping(
            cleaned,
            CHAT_TYPING_CHARS_PER_TICK,
          )) {
            tokenQueueRef.current.push(part);
          }
          break;
        }
        case "final": {
          const payload = (event.payload ?? {}) as Record<string, unknown>;
          const finalMessage = getFinalMessageFromPayload(payload);
          pendingFinalRef.current = { payload, finalMessage };
          flushPendingFinalIfReady();
          break;
        }
        case "error":
          pendingFinalRef.current = null;
          tokenQueueRef.current = [];
          upsertActiveBotMessage((msg) => ({
            ...msg,
            isLoading: false,
            isError: true,
            text: String(event.message ?? "Something went wrong"),
            thinkingSteps: [],
            isStreaming: false,
          }));
          setIsSending(false);
          break;
        default:
          break;
      }
    },
    onError: () => {
      pendingFinalRef.current = null;
      tokenQueueRef.current = [];
      upsertActiveBotMessage((msg) => ({
        ...msg,
        isLoading: false,
        isError: true,
        text: "WebSocket connection error.",
        thinkingSteps: [],
        isStreaming: false,
      }));
      setIsSending(false);
    },
  });

  const sendViaWebSocket = useCallback(
    async (text: string): Promise<void> => {
      if (!projectId || !accountId) return;
      const botMessageId = `bot-${Date.now()}`;
      activeBotMessageIdRef.current = botMessageId;
      pendingFinalRef.current = null;
      tokenQueueRef.current = [];

      setNewMessages((prev) => [
        ...prev,
        {
          id: `user-${Date.now()}`,
          text,
          sender: ChatSender.USER,
          isCurrentUser: true,
          timestamp: new Date(),
          createdBy: currentUserDisplayName,
        },
        {
          id: botMessageId,
          text: "",
          sender: ChatSender.BOT,
          timestamp: new Date(),
          isLoading: true,
        },
      ]);
      setIsSending(true);

      try {
        await connect(projectId);
        await sendUserMessage({
          type: "user_message",
          accountId,
          conversationId: conversationId ?? "",
          message: text,
          envProducts: {},
        });
      } catch {
        tokenQueueRef.current = [];
        setNewMessages((prev) =>
          prev.map((m) =>
            m.id === botMessageId
              ? {
                  ...m,
                  isLoading: false,
                  isError: true,
                  text: "Could not connect to chatbot stream.",
                }
              : m,
          ),
        );
        setIsSending(false);
      }
    },
    [
      accountId,
      connect,
      conversationId,
      currentUserDisplayName,
      projectId,
      sendUserMessage,
    ],
  );

  const handleSendMessage = useCallback(async (): Promise<boolean> => {
    const text = htmlToPlainText(inputValueRef.current).trim();
    if (!text || isSending || !projectId || !accountId) return false;
    setInputValueAndRef("");
    setResetTrigger((prev) => prev + 1);
    await sendViaWebSocket(text);
    return true;
  }, [accountId, isSending, projectId, sendViaWebSocket, setInputValueAndRef]);

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
      navigate(`/projects/${projectId}/support/conversations`, {
        state: { fromBack: true },
      });
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
            {summary.createdBy && (
              <>
                <Divider orientation="vertical" sx={{ height: 40 }} />
                <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, minWidth: 0 }}>
                  <User size={16} style={{ flexShrink: 0 }} />
                  <Box sx={{ minWidth: 0 }}>
                    <Typography variant="body2" color="text.secondary">
                      Created by
                    </Typography>
                    <Tooltip title={summary.createdBy}>
                      <Typography
                        variant="body2"
                        color="text.primary"
                        sx={{
                          overflow: "hidden",
                          textOverflow: "ellipsis",
                          whiteSpace: "nowrap",
                          maxWidth: 180,
                        }}
                      >
                        {summary.createdBy}
                      </Typography>
                    </Tooltip>
                  </Box>
                </Box>
              </>
            )}
          </Box>
        </Paper>
      )}

      <Paper
        variant="outlined"
        sx={{ display: "flex", flexDirection: "column", overflow: "hidden" }}
      >
        <Box sx={{ p: 3, display: "flex", flexDirection: "column", gap: 2 }}>
          <Stack
            direction="row"
            spacing={1.5}
            alignItems="center"
            sx={{ mb: 1 }}
          >
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
          ) : messages.length === 0 && newMessages.length === 0 ? (
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
                  {index < chatMessages.length - 1 && (
                    <Divider sx={{ my: 3 }} />
                  )}
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
              {newMessages.length > 0 && (
                <>
                  {chatMessages.length > 0 && <Divider sx={{ my: 1 }} />}
                  {newMessages.map((m) =>
                    m.isLoading ? (
                      <LoadingDotsBubble key={m.id} />
                    ) : (
                      <ChatMessageBubble key={m.id} message={m} />
                    ),
                  )}
                </>
              )}
            </Box>
          )}
        </Box>

        {conversationAction === ConversationListRowAction.Resume &&
          projectId &&
          conversationId && (
            <>
              <Divider />
              <ChatInput
                inputValue={inputValue}
                setInputValue={setInputValueAndRef}
                onSend={handleSendMessage}
                isSending={isSending}
                resetTrigger={resetTrigger}
                disabled={isInputDisabled}
              />
            </>
          )}
      </Paper>

      <ConversationKnowledgeRecommendations messages={messages} />
    </Stack>
  );
}
