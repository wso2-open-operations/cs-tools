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

import { Box, Paper, Divider } from "@wso2/oxygen-ui";
import {
  useState,
  useRef,
  useEffect,
  useCallback,
  useMemo,
  type JSX,
} from "react";
import { flushSync } from "react-dom";
import { useNavigate, useParams, useLocation } from "react-router";
import { usePostProjectDeploymentsSearchAll } from "@api/usePostProjectDeploymentsSearch";
import { useGetConversationMessages } from "@features/support/api/useGetConversationMessages";
import useGetUserDetails from "@features/settings/api/useGetUserDetails";
import { usePostCaseClassifications } from "@features/support/api/usePostCaseClassifications";
import { useChatWebSocket } from "@features/support/api/useChatWebSocket";
import useGetProjectDetails from "@api/useGetProjectDetails";
import { useQueryClient, type InfiniteData } from "@tanstack/react-query";
import type { SearchProjectsResponse } from "@features/project-hub/types/projects";
import { ApiQueryKeys } from "@constants/apiConstants";
import type {
  SlotState,
  NoveraAction,
} from "@features/support/types/conversations";
import { NoveraActionType } from "@features/support/types/conversations";
import { useAllDeploymentProducts } from "@features/support/hooks/useAllDeploymentProducts";
import {
  DEFAULT_CONVERSATION_REGION,
  DEFAULT_CONVERSATION_TIER,
} from "@features/support/constants/conversationConstants";
import {
  CHAT_TYPING_CHARS_PER_TICK,
  CHAT_TYPING_INTERVAL_MS,
  NOVERA_ANALYZING_PLACEHOLDER_TEXT,
  NOVERA_INITIAL_WELCOME_TEXT,
} from "@features/support/constants/chatConstants";
import {
  formatChatHistoryForClassification,
  buildEnvProducts,
} from "@features/support/utils/caseCreation";
import { filterDeploymentsForCaseCreation } from "@utils/permission";
import { htmlToPlainText } from "@features/support/utils/richTextEditor";
import { ChatSender } from "@features/support/types/conversations";
import type {
  ChatNavState,
  Message,
} from "@features/support/types/conversations";
import ChatHeader from "@features/support/components/novera-ai-assistant/novera-chat-page/ChatHeader";
import ChatInput from "@features/support/components/novera-ai-assistant/novera-chat-page/ChatInput";
import ChatMessageList from "@features/support/components/novera-ai-assistant/novera-chat-page/ChatMessageList";
import ChatSkeleton from "@features/support/components/novera-ai-assistant/novera-chat-page/ChatSkeleton";
import { ROUTE_PREVIOUS_PAGE } from "@features/project-hub/constants/navigationConstants";
import {
  displayTextFromConversationContent,
  getFinalMessageFromPayload,
  sanitizeStreamToken,
  splitTokenForTyping,
} from "@features/support/utils/chat";
import {
  compareByCreatedOnThenId,
  dateFromApiCreatedOn,
} from "@features/support/utils/support";

/**
 * NoveraChatPage component to provide AI-powered support assistance.
 *
 * @returns {JSX.Element} The rendered NoveraChatPage.
 */
export default function NoveraChatPage(): JSX.Element {
  const navigate = useNavigate();
  const { projectId, conversationId: urlConversationId } = useParams<{
    projectId: string;
    conversationId?: string;
  }>();
  const location = useLocation();
  const navState = location.state as ChatNavState | null;
  const initialUserMessage = navState?.initialUserMessage;
  const conversationResponse = navState?.conversationResponse;
  const preloadedMessages = navState?.messages;
  const navAccountId = navState?.accountId;
  const chatNumber = navState?.chatNumber;
  const { data: userDetails } = useGetUserDetails();
  const currentUserEmail = userDetails?.email?.toLowerCase() ?? "";

  const handleBack = () => {
    if (projectId) {
      navigate(`/projects/${projectId}/support`, { state: { fromBack: true } });
    } else {
      navigate(ROUTE_PREVIOUS_PAGE);
    }
  };

  const {
    data: allProjectDeployments,
    isLoading: isDeploymentsLoading,
  } = usePostProjectDeploymentsSearchAll(projectId || "");
  const { data: projectDetails } = useGetProjectDetails(projectId || "");
  const queryClient = useQueryClient();
  const projectTypeId = useMemo(() => {
    const cached = queryClient.getQueriesData<InfiniteData<SearchProjectsResponse>>({
      queryKey: [ApiQueryKeys.PROJECTS, "infinite"],
    });
    for (const [, data] of cached) {
      if (!data) continue;
      for (const page of data.pages) {
        const match = page.projects.find((p) => p.id === projectId);
        if (match?.type?.id) return match.type.id;
      }
    }
    return projectDetails?.type?.id ?? "";
  }, [queryClient, projectId, projectDetails?.type?.id]);
  const projectDeployments = useMemo(
    () =>
      filterDeploymentsForCaseCreation(
        allProjectDeployments,
        projectDetails?.type?.label,
      ),
    [allProjectDeployments, projectDetails?.type?.label],
  );
  const { productsByDeploymentId, isLoading: isProductsLoading } =
    useAllDeploymentProducts(projectDeployments);
  const isAllProductsLoading = isDeploymentsLoading || isProductsLoading;
  const envProducts = useMemo(
    () => buildEnvProducts(productsByDeploymentId, projectDeployments),
    [productsByDeploymentId, projectDeployments],
  );
  const { mutateAsync: classifyCase } = usePostCaseClassifications();
  const accountId =
    navAccountId || projectDetails?.account?.id || projectId || "";
  const [conversationId, setConversationId] = useState<string | null>(
    () => urlConversationId ?? conversationResponse?.conversationId ?? null,
  );

  const {
    data: conversationHistory,
    isLoading: isLoadingHistory,
    fetchNextPage,
    hasNextPage,
    isFetchingNextPage,
  } = useGetConversationMessages(urlConversationId || "", { pageSize: 10 });
  const [isCreateCaseLoading, setIsCreateCaseLoading] = useState(false);
  const [isWaitingForClassification, setIsWaitingForClassification] =
    useState(false);
  const [messages, setMessages] = useState<Message[]>(() => {
    if (preloadedMessages && preloadedMessages.length > 0) {
      return preloadedMessages.map((message, index) => ({
        ...message,
        id: message.id || `restored-${index}`,
        sender:
          message.sender === ChatSender.BOT ? ChatSender.BOT : ChatSender.USER,
          isCurrentUser:
            message.sender === ChatSender.USER
              ? (message.isCurrentUser ?? true)
              : false,
        timestamp:
          message.timestamp instanceof Date
            ? message.timestamp
            : new Date(message.timestamp ?? Date.now()),
      }));
    }
    if (conversationResponse?.message) {
      const userMsg = initialUserMessage?.trim();
      const msgs: Message[] = [
        {
          id: "3",
          text: conversationResponse.message,
          sender: ChatSender.BOT,
          timestamp: new Date(),
          showCreateCaseAction: conversationResponse.actions != null,
          slotState: conversationResponse.slotState,
          recommendations:
            conversationResponse.recommendations?.recommendations,
        },
      ];
      if (userMsg) {
        msgs.unshift({
          id: "2",
          text: userMsg,
          sender: ChatSender.USER,
          isCurrentUser: true,
          timestamp: new Date(),
        });
      }
      return msgs;
    }
    if (urlConversationId) {
      return [];
    }

    const botWelcome: Message = {
      id: "1",
      text: NOVERA_INITIAL_WELCOME_TEXT,
      sender: ChatSender.BOT,
      timestamp: new Date(),
    };
    return [botWelcome];
  });

  // Load and convert conversation history when resuming.
  useEffect(() => {
    if (!urlConversationId || !conversationHistory?.pages) return;
    if (lastProcessedConversationIdRef.current !== urlConversationId) {
      lastProcessedConversationIdRef.current = urlConversationId;
      processedHistoryPageCountRef.current = 0;
    }
    const pageCount = conversationHistory.pages.length;
    if (pageCount <= processedHistoryPageCountRef.current) return;
    processedHistoryPageCountRef.current = pageCount;

    const allMessages = conversationHistory.pages.flatMap(
      (page) => page.comments,
    );

    const convertedMessages: Message[] = allMessages
      .slice()
      .sort(compareByCreatedOnThenId)
      .map((msg, index) => {
        const isBot =
          msg.type?.toLowerCase() === "bot" ||
          msg.createdBy?.toLowerCase() === "novera";
        const messageCreatorEmail = msg.createdBy?.toLowerCase() ?? "";
        const isCurrentUserMessage =
          !isBot &&
          (currentUserEmail.length > 0
            ? messageCreatorEmail.length > 0 &&
              messageCreatorEmail === currentUserEmail
            : true);
        const createdByDisplayName = [
          msg.createdByFirstName,
          msg.createdByLastName,
        ]
          .filter((name) => Boolean(name && name.trim()))
          .join(" ")
          .trim();

        return {
          id: msg.id || `msg-${index}`,
          text: displayTextFromConversationContent(msg.content || "", isBot),
          sender: isBot ? ChatSender.BOT : ChatSender.USER,
          isCurrentUser: isBot ? false : isCurrentUserMessage,
          timestamp: dateFromApiCreatedOn(msg.createdOn),
          createdBy: createdByDisplayName || msg.createdBy || undefined,
          showCreateCaseAction: false,
        };
      });

    setMessages((prev) => {
      // Keep optimistic/nav-state messages when history API returns empty.
      if (convertedMessages.length === 0 && prev.length > 0) {
        return prev;
      }
      return convertedMessages;
    });
    queryClient.invalidateQueries({
      queryKey: [ApiQueryKeys.CONVERSATION_MESSAGES, urlConversationId, 10],
    });
  }, [urlConversationId, conversationHistory, queryClient, currentUserEmail]);


  // Update URL with conversationId from describe-issue flow
  useEffect(() => {
    if (
      !urlConversationId &&
      conversationResponse?.conversationId &&
      projectId
    ) {
      navigate(
        `/projects/${projectId}/support/chat/${conversationResponse.conversationId}`,
        { replace: true },
      );
    }
  }, [urlConversationId, conversationResponse, projectId, navigate]);

  const performClassification = useCallback(async () => {
    if (!projectId) {
      navigate("/");
      setIsCreateCaseLoading(false);
      setIsWaitingForClassification(false);
      return;
    }

    try {
      const chatHistory = formatChatHistoryForClassification(messages);
      if (chatHistory) {
        try {
          const classificationResponse = await classifyCase({
            chatHistory,
            envProducts,
            region: DEFAULT_CONVERSATION_REGION,
            tier: DEFAULT_CONVERSATION_TIER,
            projectTypeId,
          });
          navigate(`/projects/${projectId}/support/chat/create-case`, {
            state: { messages, classificationResponse, conversationId },
          });
        } catch {
          navigate(`/projects/${projectId}/support/chat/create-case`, {
            state: { messages, conversationId },
          });
        }
      } else {
        navigate(`/projects/${projectId}/support/chat/create-case`, {
          state: { messages, conversationId },
        });
      }
    } finally {
      setIsCreateCaseLoading(false);
      setIsWaitingForClassification(false);
    }
  }, [
    projectId,
    navigate,
    messages,
    envProducts,
    classifyCase,
    conversationId,
    projectTypeId,
  ]);

  const handleCreateCase = useCallback(() => {
    setIsCreateCaseLoading(true);

    if (isAllProductsLoading) {
      setIsWaitingForClassification(true);
    } else {
      performClassification();
    }
  }, [isAllProductsLoading, performClassification]);

  useEffect(() => {
    if (isWaitingForClassification && !isAllProductsLoading) {
      setIsWaitingForClassification(false);
      performClassification();
    }
  }, [isWaitingForClassification, isAllProductsLoading, performClassification]);
  const [showRichText, setShowRichText] = useState(false);
  const [isInputDisabled, setIsInputDisabled] = useState(false);
  const [inputValue, setInputValue] = useState("");
  const inputValueRef = useRef("");
  const [resetTrigger, setResetTrigger] = useState(0);
  const [isSending, setIsSending] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const activeBotMessageIdRef = useRef<string | null>(null);
  const initialMessageSentRef = useRef(false);
  const processedHistoryPageCountRef = useRef(0);
  const lastProcessedConversationIdRef = useRef<string | null>(null);
  const tokenQueueRef = useRef<string[]>([]);
  const pendingFinalRef = useRef<{
    payload: Record<string, unknown>;
    finalMessage: string;
  } | null>(null);
  const TYPING_INTERVAL_MS = CHAT_TYPING_INTERVAL_MS;
  const TYPING_CHARS_PER_TICK = CHAT_TYPING_CHARS_PER_TICK;

  const upsertActiveBotMessage = useCallback(
    (updater: (message: Message) => Message, fallback?: () => Message) => {
      setMessages((prev) => {
        const activeId = activeBotMessageIdRef.current;
        if (!activeId) return prev;
        let found = false;
        const next = prev.map((msg) => {
          if (msg.id !== activeId) return msg;
          found = true;
          return updater(msg);
        });
        if (!found && fallback) {
          next.push(fallback());
        }
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
      setMessages((prev) => {
        const idx = prev.findIndex((m) => m.id === activeId);
        if (idx === -1) {
          return prev;
        }
        pendingFinalRef.current = null;
        const { finalMessage, payload } = pending;
        const msg = prev[idx];
        const parsedActions = Array.isArray(payload.actions)
          ? (payload.actions as NoveraAction[])
          : undefined;
        const next = [...prev];
        next[idx] = {
          ...msg,
          isLoading: false,
          isError: false,
          text: finalMessage || msg.text,
          showCreateCaseAction: payload.actions != null,
          slotState: payload.slotState as SlotState | undefined,
          thinkingSteps: [],
          thinkingLabel: null,
          isStreaming: false,
          actions: parsedActions,
        };
        appliedFinal = true;
        return next;
      });
    });
    if (appliedFinal) {
      setIsSending(false);
      const actions = Array.isArray(pending?.payload?.actions)
        ? (pending.payload.actions as NoveraAction[])
        : [];
      if (actions.some((a) => a.type === NoveraActionType.SolutionProposed)) {
        setShowRichText(true);
      }
      if (actions.some((a) => a.type === NoveraActionType.SolutionWorked)) {
        setIsInputDisabled(true);
      }
    }
  }, [setShowRichText]);

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
      if (tokenQueueRef.current.length > 0) {
        dequeueOneTypedToken();
      }
      flushPendingFinalIfReady();
    }, TYPING_INTERVAL_MS);
    return () => window.clearInterval(id);
  }, [dequeueOneTypedToken, flushPendingFinalIfReady, TYPING_INTERVAL_MS]);

  const { connect, sendUserMessage } = useChatWebSocket({
    onEvent: (event) => {
      switch (event.type) {
        case "conversation_created": {
          const nextConversationId = String(event.conversationId ?? "");
          if (nextConversationId) {
            setConversationId(nextConversationId);
            if (!urlConversationId && projectId) {
              navigate(
                `/projects/${projectId}/support/chat/${nextConversationId}`,
                {
                  replace: true,
                },
              );
            }
          }
          break;
        }
        case "thinking_start": {
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
              id: `bot-${Date.now()}`,
              sender: ChatSender.BOT,
              timestamp: new Date(),
              text: NOVERA_ANALYZING_PLACEHOLDER_TEXT,
              thinkingSteps: [],
              thinkingLabel: null,
              isStreaming: false,
            }),
          );
          break;
        }
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
          if (cleaned.length === 0) {
            break;
          }
          for (const part of splitTokenForTyping(
            cleaned,
            TYPING_CHARS_PER_TICK,
          )) {
            tokenQueueRef.current.push(part);
          }
          break;
        }
        case "final": {
          const payload = (event.payload ?? {}) as Record<string, unknown>;
          const finalMessage = getFinalMessageFromPayload(payload);
          const nextConversationId = String(payload.conversationId ?? "");
          if (nextConversationId) {
            setConversationId(nextConversationId);
            if (!urlConversationId && projectId) {
              navigate(
                `/projects/${projectId}/support/chat/${nextConversationId}`,
                {
                  replace: true,
                },
              );
            }
          }
          pendingFinalRef.current = { payload, finalMessage };
          flushPendingFinalIfReady();
          const activeConversationId = nextConversationId || urlConversationId;
          if (activeConversationId) {
            queryClient.invalidateQueries({
              queryKey: [ApiQueryKeys.CONVERSATION_MESSAGES, activeConversationId, 10],
            });
          }
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

  const setInputValueAndRef = useCallback((v: string) => {
    inputValueRef.current = v;
    setInputValue(v);
  }, []);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  const sendViaWebSocket = useCallback(
    async (text: string): Promise<boolean> => {
      if (!projectId || !accountId) return false;
      const hasEnvProducts = Object.keys(envProducts).length > 0;
      const botMessageId = `bot-${Date.now()}`;
      activeBotMessageIdRef.current = botMessageId;
      pendingFinalRef.current = null;
      tokenQueueRef.current = [];

      setMessages((prev) => [
        ...prev,
        {
          id: `user-${Date.now()}`,
          text,
          sender: ChatSender.USER,
          isCurrentUser: true,
          timestamp: new Date(),
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
          envProducts: hasEnvProducts ? envProducts : {},
        });
        return true;
      } catch {
        tokenQueueRef.current = [];
        setMessages((prev) =>
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
        return false;
      }
    },
    [
      accountId,
      connect,
      conversationId,
      envProducts,
      projectId,
      sendUserMessage,
    ],
  );

  const handleSolutionWorked = useCallback(() => {
    if (isSending) return;
    void sendViaWebSocket("This Resolved My Issue");
  }, [isSending, sendViaWebSocket]);

  const handleSendMessage = useCallback(async (): Promise<boolean> => {
    const text = htmlToPlainText(inputValueRef.current).trim();
    if (!text || isSending || !projectId) return false;
    setInputValueAndRef("");
    setResetTrigger((prev) => prev + 1);
    return sendViaWebSocket(text);
  }, [isSending, projectId, sendViaWebSocket, setInputValueAndRef]);

  useEffect(() => {
    if (!initialUserMessage?.trim()) return;
    if (urlConversationId) return;
    if (initialMessageSentRef.current) return;
    initialMessageSentRef.current = true;
    void sendViaWebSocket(initialUserMessage.trim());
  }, [initialUserMessage, sendViaWebSocket, urlConversationId]);

  return (
    <Box
      sx={{
        height: (theme) => `calc(100vh - ${theme.spacing(21)})`,
        display: "flex",
      }}
    >
      <Box
        sx={{
          flex: 1,
          display: "flex",
          flexDirection: "column",
          width: "100%",
          overflow: "visible",
        }}
      >
        <Box sx={{ mt: -1.5, mx: -3 }}>
          <ChatHeader
            onBack={handleBack}
            onCreateCase={handleCreateCase}
            isCreateCaseLoading={isCreateCaseLoading || isAllProductsLoading}
            chatNumber={chatNumber}
          />
        </Box>

        {/* Chat window */}
        <Paper
          sx={{
            flex: 1,
            display: "flex",
            flexDirection: "column",
            overflow: "hidden",
          }}
        >
          {isLoadingHistory && urlConversationId && messages.length === 0 ? (
            <ChatSkeleton />
          ) : (
            <ChatMessageList
              messages={messages}
              messagesEndRef={messagesEndRef}
              onCreateCase={handleCreateCase}
              onSolutionWorked={handleSolutionWorked}
              onFetchOlder={
                urlConversationId && hasNextPage && !isFetchingNextPage
                  ? () => fetchNextPage()
                  : undefined
              }
              isFetchingOlder={isFetchingNextPage}
            />
          )}

          <Divider />

          <ChatInput
            onSend={handleSendMessage}
            inputValue={inputValue}
            setInputValue={setInputValueAndRef}
            onCreateCase={handleCreateCase}
            isCreateCaseLoading={isCreateCaseLoading || isAllProductsLoading}
            isSending={isSending}
            resetTrigger={resetTrigger}
            forceRichText={showRichText}
            disabled={isInputDisabled}
          />
        </Paper>
      </Box>
    </Box>
  );
}
