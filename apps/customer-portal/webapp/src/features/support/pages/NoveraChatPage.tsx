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
  NOVERA_WELCOME_MESSAGE_ID,
} from "@features/support/constants/chatConstants";
import {
  formatChatHistoryForClassification,
  buildEnvProducts,
} from "@features/support/utils/caseCreation";
import { filterDeploymentsForCaseCreation } from "@utils/permission";
import { htmlToPlainText } from "@features/support/utils/richTextEditor";
import { usePiiGuard } from "@features/support/hooks/usePiiGuard";
import PiiWarningDialog from "@features/support/components/dialogs/PiiWarningDialog";
import { ChatSender } from "@features/support/types/conversations";
import { MAX_FEEDBACK_TAGS } from "@features/support/constants/feedbackTags";
import type {
  ChatNavState,
  Message,
} from "@features/support/types/conversations";
import ChatHeader from "@features/support/components/novera-ai-assistant/novera-chat-page/ChatHeader";
import ChatInput from "@features/support/components/novera-ai-assistant/novera-chat-page/ChatInput";
import ChatMessageList from "@features/support/components/novera-ai-assistant/novera-chat-page/ChatMessageList";
import TokenRequestModal from "@features/support/components/novera-ai-assistant/novera-chat-page/TokenRequestModal";
import ChatSkeleton from "@features/support/components/novera-ai-assistant/novera-chat-page/ChatSkeleton";
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

// Max time (ms) to wait for the conversation id (delivered asynchronously over
// the chat WebSocket) before creating a case, so the created case links to the
// chat. Bounded so a missing id (e.g. socket down) can never block case creation.
const CONVERSATION_ID_WAIT_MS = 3000;

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
    navigate(-1);
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
  // Resolver for a pending "wait for the conversation id" promise (see
  // waitForConversationId). Resolved when conversation_created arrives, so a
  // case created moments after the first message still carries the id.
  const pendingIdResolveRef = useRef<((id: string | null) => void) | null>(null);

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
      id: NOVERA_WELCOME_MESSAGE_ID,
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

  // Wait (bounded) for the conversation id, which arrives asynchronously over
  // the chat WebSocket. Resolves immediately if it's already known; otherwise
  // when conversation_created fires, or with null after the timeout so case
  // creation is never blocked.
  const waitForConversationId = useCallback((): Promise<string | null> => {
    if (conversationId) {
      return Promise.resolve(conversationId);
    }
    return new Promise<string | null>((resolve) => {
      pendingIdResolveRef.current = resolve;
      setTimeout(() => {
        if (pendingIdResolveRef.current === resolve) {
          pendingIdResolveRef.current = null;
          resolve(null);
        }
      }, CONVERSATION_ID_WAIT_MS);
    });
  }, [conversationId]);

  const performClassification = useCallback(async () => {
    if (!projectId) {
      navigate("/");
      setIsCreateCaseLoading(false);
      setIsWaitingForClassification(false);
      return;
    }

    // Capture the conversation id (bounded wait) so the created case links back
    // to this chat; the backend then converts the chat on case creation. If the
    // id never arrives, we still proceed — case creation must never block.
    const chatConversationId = await waitForConversationId();

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
            state: {
              messages,
              classificationResponse,
              conversationId: chatConversationId,
            },
          });
        } catch {
          navigate(`/projects/${projectId}/support/chat/create-case`, {
            state: { messages, conversationId: chatConversationId },
          });
        }
      } else {
        navigate(`/projects/${projectId}/support/chat/create-case`, {
          state: { messages, conversationId: chatConversationId },
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
    projectTypeId,
    waitForConversationId,
  ]);

  const handleCreateCase = useCallback(() => {
    setIsCreateCaseLoading(true);

    // Always proceed — case creation must never block on the conversation id or
    // the WebSocket. performClassification waits (bounded) for the id so the
    // case links to the chat, then navigates.
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
  const piiGuard = usePiiGuard();
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
        const answerId =
          typeof payload.messageId === "string" ? payload.messageId : undefined;
        next[idx] = {
          ...msg,
          isLoading: false,
          isError: false,
          text: finalMessage || msg.text,
          showCreateCaseAction: payload.actions != null,
          showFeedbackActions: !!answerId,
          feedbackMessageId: answerId,
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

  const { connect, sendUserMessage, isConnected } = useChatWebSocket({
    onEvent: (event) => {
      switch (event.type) {
        case "conversation_created": {
          const nextConversationId = String(event.conversationId ?? "");
          if (nextConversationId) {
            setConversationId(nextConversationId);
            // Unblock a pending "wait for id" (e.g. a fast Create Case click)
            // so the case links to this chat.
            if (pendingIdResolveRef.current) {
              pendingIdResolveRef.current(nextConversationId);
              pendingIdResolveRef.current = null;
            }
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
        case "feedback_ack": {
          const ackId = String(event.messageId ?? "");
          const ackRating =
            event.rating === 1 ? 1 : event.rating === -1 ? -1 : null;
          // The server echoes what it actually stored, after dropping any tag it
          // does not recognise — so adopt its list unconditionally. An absent
          // `tags` field means it stored none (an older backend ignores the
          // field entirely), so fall back to [] rather than keeping our
          // optimistic value: showing chips as selected when nothing was saved
          // tells the user something untrue.
          const ackTags = Array.isArray(event.tags)
            ? (event.tags as unknown[]).filter(
                (t): t is string => typeof t === "string",
              )
            : undefined;
          if (ackId) {
            setMessages((prev) =>
              prev.map((m) =>
                m.feedbackMessageId === ackId
                  ? {
                      ...m,
                      feedbackRating: ackRating,
                      feedbackTags: ackTags ?? [],
                    }
                  : m,
              ),
            );
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

  // Answer feedback (👍/👎) over the existing chat socket (#2534). Optimistic:
  // reflect the vote immediately, revert if the send fails. feedback_ack later
  // confirms the persisted value.
  // Latest feedback submission per messageId, so a stale rejection cannot
  // roll back a newer vote. A ref, not state: it must not trigger a render.
  const feedbackSeqRef = useRef<Map<string, number>>(new Map());

  const submitFeedback = useCallback(
    (messageId: string, rating: 1 | -1, tags?: string[]) => {
      if (!projectId) return;

      // Mark this as the latest submission for the message. Clicking 👍 then
      // 👎 leaves two sends in flight; if the first rejects after the second
      // resolved, its rollback must not undo the newer vote.
      const seq = (feedbackSeqRef.current.get(messageId) ?? 0) + 1;
      feedbackSeqRef.current.set(messageId, seq);

      // Remember what was showing so a failure restores it, rather than
      // clearing a rating the user had already given (and we had persisted).
      let previousRating: 1 | -1 | null = null;
      setMessages((prev) =>
        prev.map((m) => {
          if (m.feedbackMessageId !== messageId) return m;
          previousRating = m.feedbackRating ?? null;
          return {
            ...m,
            feedbackRating: rating,
            ...(tags ? { feedbackTags: tags } : {}),
          };
        }),
      );

      void connect(projectId)
        .then(() =>
          sendUserMessage({
            type: "feedback",
            messageId,
            rating,
            conversationId: conversationId ?? undefined,
            accountId: accountId || undefined,
            projectId: projectId || undefined,
            // Names, not just ids: a reviewer reading the dashboard should see
            // who this came from without resolving sys_ids by hand. Already
            // loaded for this page, so this costs no extra request.
            accountName: projectDetails?.account?.name || undefined,
            projectName: projectDetails?.name || undefined,
            ...(tags ? { tags } : {}),
          }),
        )
        .catch(() => {
          if (feedbackSeqRef.current.get(messageId) !== seq) return;
          setMessages((prev) =>
            prev.map((m) =>
              m.feedbackMessageId === messageId
                ? { ...m, feedbackRating: previousRating }
                : m,
            ),
          );
        });
    },
    [
      accountId,
      connect,
      conversationId,
      projectId,
      projectDetails?.account?.name,
      projectDetails?.name,
      sendUserMessage,
    ],
  );

  /**
   * Toggle a reason tag on an answer that has already been rated. Re-sends the
   * same feedback event with the new list — the backend upserts on
   * (conversation, message), so this replaces rather than accumulating rows.
   * Capped client-side at MAX_FEEDBACK_TAGS; the server enforces the same limit.
   */
  const handleFeedbackTag = useCallback(
    (messageId: string, tag: string) => {
      const msg = messages.find((m) => m.feedbackMessageId === messageId);
      if (!msg?.feedbackRating) return;
      const current = msg.feedbackTags ?? [];
      const next = current.includes(tag)
        ? current.filter((t) => t !== tag)
        : current.length >= MAX_FEEDBACK_TAGS
          ? current
          : [...current, tag];
      if (next === current) return;
      submitFeedback(messageId, msg.feedbackRating, next);
    },
    [messages, submitFeedback],
  );

  const handleThumbsUp = useCallback(
    (messageId: string) => submitFeedback(messageId, 1),
    [submitFeedback],
  );
  const handleThumbsDown = useCallback(
    (messageId: string) => submitFeedback(messageId, -1),
    [submitFeedback],
  );

  // Feature-flagged (config.js): request a token limit increase over the chat
  // WebSocket. Kept off until the backend handler is ready.
  const tokenRequestEnabled =
    window.config?.CUSTOMER_PORTAL_NOVERA_TOKEN_REQUEST_ENABLED ?? false;
  const [isTokenModalOpen, setIsTokenModalOpen] = useState(false);

  const handleTokenIncreaseSubmit = useCallback(
    async (reason: string): Promise<void> => {
      if (!projectId || !accountId) {
        throw new Error("Unable to submit the request right now.");
      }
      await connect(projectId);
      await sendUserMessage({
        type: "token_increase_request",
        accountId,
        reason,
        limitType: "session",
      });
    },
    [projectId, accountId, connect, sendUserMessage],
  );

  const handleSendMessage = useCallback(async (): Promise<boolean> => {
    const text = htmlToPlainText(inputValueRef.current).trim();
    if (!text || isSending || !projectId) return false;

    // Warn about PII before sending. The input is only cleared once the send
    // actually proceeds, so choosing "Edit" preserves the user's text.
    piiGuard.checkBeforeSubmit(text, () => {
      setInputValueAndRef("");
      setResetTrigger((prev) => prev + 1);
      void sendViaWebSocket(text);
    });
    return true;
  }, [isSending, projectId, sendViaWebSocket, setInputValueAndRef, piiGuard]);

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
              onThumbsUp={isConnected ? handleThumbsUp : undefined}
              onThumbsDown={isConnected ? handleThumbsDown : undefined}
            onFeedbackTag={isConnected ? handleFeedbackTag : undefined}
              onSolutionWorked={handleSolutionWorked}
              onRequestTokenIncrease={
                tokenRequestEnabled
                  ? () => setIsTokenModalOpen(true)
                  : undefined
              }
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

      <PiiWarningDialog {...piiGuard.dialogProps} />

      {tokenRequestEnabled && (
        <TokenRequestModal
          open={isTokenModalOpen}
          onClose={() => setIsTokenModalOpen(false)}
          onSubmit={handleTokenIncreaseSubmit}
        />
      )}
    </Box>
  );
}
