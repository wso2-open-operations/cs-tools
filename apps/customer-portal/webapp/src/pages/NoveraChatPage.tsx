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
import { useGetConversationMessages } from "@api/useGetConversationMessages";
import { usePostCaseClassifications } from "@api/usePostCaseClassifications";
import { useChatWebSocket } from "@api/useChatWebSocket";
import useGetProjectDetails from "@api/useGetProjectDetails";
import type { SlotState } from "@/types/conversations";
import { useAllDeploymentProducts } from "@hooks/useAllDeploymentProducts";
import {
  DEFAULT_CONVERSATION_REGION,
  DEFAULT_CONVERSATION_TIER,
} from "@constants/conversationConstants";
import {
  CHAT_TYPING_CHARS_PER_TICK,
  CHAT_TYPING_INTERVAL_MS,
  NOVERA_ANALYZING_PLACEHOLDER_TEXT,
  NOVERA_INITIAL_WELCOME_TEXT,
  VISIBILITY_EVENT_LISTENER,
} from "@constants/chatConstants";
import {
  formatChatHistoryForClassification,
  buildEnvProducts,
} from "@utils/caseCreation";
import { filterDeploymentsForCaseCreation } from "@utils/subscriptionUtils";
import { htmlToPlainText } from "@utils/richTextEditor";
import { ChatSender } from "@/types/conversations";
import type { ChatNavState, Message } from "@/types/conversations";
import ChatHeader from "@components/support/novera-ai-assistant/novera-chat-page/ChatHeader";
import ChatInput from "@components/support/novera-ai-assistant/novera-chat-page/ChatInput";
import ChatMessageList from "@components/support/novera-ai-assistant/novera-chat-page/ChatMessageList";
import ChatSkeleton from "@components/support/novera-ai-assistant/novera-chat-page/ChatSkeleton";
import { ROUTE_PREVIOUS_PAGE } from "@/constants/commonConstants";
import { displayTextFromConversationContent, getFinalMessageFromPayload, sanitizeStreamToken, splitTokenForTyping } from "@/utils/chat";
import { compareByCreatedOnThenId, dateFromApiCreatedOn } from "@utils/support";

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
  const navAccountId = navState?.accountId;
  const chatNumber = navState?.chatNumber;

  const handleBack = () => {
    if (projectId) {
      navigate(`/projects/${projectId}/support`);
    } else {
      navigate(ROUTE_PREVIOUS_PAGE);
    }
  };

  const { data: allProjectDeployments } = usePostProjectDeploymentsSearchAll(
    projectId || "",
  );
  const { data: projectDetails } = useGetProjectDetails(projectId || "");
  const projectDeployments = useMemo(
    () =>
      filterDeploymentsForCaseCreation(
        allProjectDeployments,
        projectDetails?.type?.label,
      ),
    [allProjectDeployments, projectDetails?.type?.label],
  );
  const { productsByDeploymentId, isLoading: isAllProductsLoading } =
    useAllDeploymentProducts(projectDeployments);
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
    refetch: refetchMessages,
  } = useGetConversationMessages(urlConversationId || "", { pageSize: 10 });
  const [isCreateCaseLoading, setIsCreateCaseLoading] = useState(false);
  const [isWaitingForClassification, setIsWaitingForClassification] =
    useState(false);
  const [messages, setMessages] = useState<Message[]>(() => {
    if (urlConversationId) {
      return [];
    }

    const botWelcome: Message = {
      id: "1",
      text: NOVERA_INITIAL_WELCOME_TEXT,
      sender: ChatSender.BOT,
      timestamp: new Date(),
    };

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
          timestamp: new Date(),
        });
      }
      return msgs;
    }
    return [botWelcome];
  });

  // Load and convert conversation history when resuming
  useEffect(() => {
    if (!urlConversationId || !conversationHistory?.pages) return;
    if (messages.length > 0) return;

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

        return {
          id: msg.id || `msg-${index}`,
          text: displayTextFromConversationContent(msg.content || "", isBot),
          sender: isBot ? ChatSender.BOT : ChatSender.USER,
          timestamp: dateFromApiCreatedOn(msg.createdOn),
          showCreateCaseAction: false,
        };
      });

    setMessages(convertedMessages);
  }, [urlConversationId, conversationHistory, messages.length]);

  // Refetch messages when user returns from create-case page (detects navigate back)
  useEffect(() => {
    if (!urlConversationId) return;

    const handleRefetch = () => {
      refetchMessages();
    };

    // Listen for visibility changes (when user returns to tab)
    document.addEventListener(VISIBILITY_EVENT_LISTENER, handleRefetch);

    return () => {
      document.removeEventListener(VISIBILITY_EVENT_LISTENER, handleRefetch);
    };
  }, [urlConversationId, refetchMessages]);

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
      const hasEnvProducts = Object.keys(envProducts).length > 0;

      if (chatHistory && hasEnvProducts) {
        try {
          const classificationResponse = await classifyCase({
            chatHistory,
            envProducts,
            region: DEFAULT_CONVERSATION_REGION,
            tier: DEFAULT_CONVERSATION_TIER,
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
  const [inputValue, setInputValue] = useState("");
  const inputValueRef = useRef("");
  const [resetTrigger, setResetTrigger] = useState(0);
  const [isSending, setIsSending] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const activeBotMessageIdRef = useRef<string | null>(null);
  const initialMessageSentRef = useRef(false);
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
        };
        appliedFinal = true;
        return next;
      });
    });
    if (appliedFinal) {
      setIsSending(false);
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
      if (tokenQueueRef.current.length > 0) {
        dequeueOneTypedToken();
      }
      flushPendingFinalIfReady();
    }, TYPING_INTERVAL_MS);
    return () => window.clearInterval(id);
  }, [dequeueOneTypedToken, flushPendingFinalIfReady, TYPING_INTERVAL_MS]);

  const { connect, sendUserMessage } = useChatWebSocket({
    onEvent: (event) => {
      if (event.type === "conversation_created") {
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
        return;
      }

      if (event.type === "thinking_start") {
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
        return;
      }

      if (event.type === "thinking_step") {
        const label = String(event.label ?? event.step ?? "Working...");
        upsertActiveBotMessage((msg) => ({
          ...msg,
          isLoading: false,
          thinkingSteps: [...(msg.thinkingSteps ?? []), label],
          thinkingLabel: label,
        }));
        return;
      }

      if (event.type === "thinking_end") {
        upsertActiveBotMessage((msg) => ({
          ...msg,
          isLoading: false,
          thinkingLabel: msg.thinkingLabel,
        }));
        return;
      }

      if (event.type === "token") {
        const token = String(event.content ?? "");
        const cleaned = sanitizeStreamToken(token);
        if (cleaned.length > 0) {
          for (const part of splitTokenForTyping(
            cleaned,
            TYPING_CHARS_PER_TICK,
          )) {
            tokenQueueRef.current.push(part);
          }
        }
        return;
      }

      if (event.type === "final") {
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
        return;
      }

      if (event.type === "error") {
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
            isCreateCaseLoading={isCreateCaseLoading}
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
            isCreateCaseLoading={isCreateCaseLoading}
            isSending={isSending}
            resetTrigger={resetTrigger}
          />
        </Paper>
      </Box>
    </Box>
  );
}
