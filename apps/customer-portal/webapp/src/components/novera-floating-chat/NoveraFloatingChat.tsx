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
  Card,
  Divider,
  IconButton,
  TextField,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import {
  Bot,
  ChevronUp,
  Minus,
  Send,
  Sparkles,
  X,
} from "@wso2/oxygen-ui-icons-react";
import {
  type JSX,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import { useLocation, useParams } from "react-router";
import { useFloatingNoveraVisibility } from "@context/floating-novera-visibility/FloatingNoveraVisibilityContext";
import useGetProjectDetails from "@api/useGetProjectDetails";
import { usePostProjectDeploymentsSearchAll } from "@api/usePostProjectDeploymentsSearch";
import { useAllDeploymentProducts } from "@features/support/hooks/useAllDeploymentProducts";
import { useChatWebSocket } from "@features/support/api/useChatWebSocket";
import type { Message } from "@features/support/types/conversations";
import { buildEnvProducts } from "@features/support/utils/caseCreation";
import { filterDeploymentsForCaseCreation } from "@features/project-details/utils/permissions";
import { getFinalMessageFromPayload } from "@features/support/utils/chat";
import { NOVERA_ANALYZING_PLACEHOLDER_TEXT } from "@features/support/constants/chatConstants";
import { ChatSender } from "@features/support/types/conversations";
import ChatMessageBubble from "@features/support/components/novera-ai-assistant/novera-chat-page/ChatMessageBubble";
import LoadingDotsBubble from "@features/support/components/novera-ai-assistant/novera-chat-page/LoadingDotsBubble";

const WELCOME_MESSAGE =
  "Hello! I'm Novera, your AI support assistant. How can I help you today?";

/**
 * Floating Novera chat widget rendered across project pages.
 *
 * @returns {JSX.Element | null} Floating widget, or null when not applicable.
 */
export default function NoveraFloatingChat(): JSX.Element | null {
  const location = useLocation();
  const { hideForDetailsActivityTab } = useFloatingNoveraVisibility();
  const { projectId } = useParams<{ projectId: string }>();
  const [isOpen, setIsOpen] = useState(false);
  const [isMinimized, setIsMinimized] = useState(false);
  const [input, setInput] = useState("");
  const [isSending, setIsSending] = useState(false);
  const [conversationId, setConversationId] = useState<string>("");
  const [messages, setMessages] = useState<Message[]>([
    {
      id: "welcome",
      text: WELCOME_MESSAGE,
      sender: ChatSender.BOT,
      timestamp: new Date(),
    },
  ]);
  const botMessageIdRef = useRef<string | null>(null);
  const messagesEndRef = useRef<HTMLDivElement | null>(null);

  const { data: projectDetails } = useGetProjectDetails(projectId || "");
  const hasAgent =
    projectDetails?.hasAgent ?? projectDetails?.account?.hasAgent ?? false;
  const accountId = projectDetails?.account?.id ?? projectId ?? "";

  const { data: allProjectDeployments } = usePostProjectDeploymentsSearchAll(
    projectId || "",
  );
  const projectDeployments = useMemo(
    () =>
      filterDeploymentsForCaseCreation(
        allProjectDeployments,
        projectDetails?.type?.label,
      ),
    [allProjectDeployments, projectDetails?.type?.label],
  );
  const { productsByDeploymentId } =
    useAllDeploymentProducts(projectDeployments);
  const envProducts = useMemo(
    () => buildEnvProducts(productsByDeploymentId, projectDeployments),
    [productsByDeploymentId, projectDeployments],
  );

  const updateActiveBotMessage = useCallback((nextText: string) => {
    const activeId = botMessageIdRef.current;
    if (!activeId) return;
    setMessages((prev) =>
      prev.map((msg) =>
        msg.id === activeId
          ? { ...msg, text: nextText, isLoading: false }
          : msg,
      ),
    );
  }, []);

  const upsertActiveBotMessage = useCallback(
    (updater: (message: Message) => Message, fallback?: () => Message) => {
      setMessages((prev) => {
        const activeId = botMessageIdRef.current;
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

  const { connect, sendUserMessage } = useChatWebSocket({
    onEvent: (event) => {
      if (event.type === "conversation_created") {
        const nextConversationId = String(event.conversationId ?? "");
        if (nextConversationId) {
          setConversationId(nextConversationId);
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
        if (!token) return;
        const activeId = botMessageIdRef.current;
        if (!activeId) return;
        setMessages((prev) =>
          prev.map((msg) =>
            msg.id === activeId
              ? {
                  ...msg,
                  text: `${msg.text ?? ""}${token}`,
                  isLoading: false,
                  isStreaming: true,
                }
              : msg,
          ),
        );
        return;
      }

      if (event.type === "final") {
        const payload = (event.payload ?? {}) as Record<string, unknown>;
        const finalMessage = getFinalMessageFromPayload(payload);
        const nextConversationId = String(payload.conversationId ?? "");
        if (nextConversationId) {
          setConversationId(nextConversationId);
        }
        upsertActiveBotMessage((msg) => ({
          ...msg,
          isLoading: false,
          isError: false,
          text: finalMessage || msg.text,
          thinkingSteps: [],
          thinkingLabel: null,
          isStreaming: false,
        }));
        setIsSending(false);
        return;
      }

      if (event.type === "error") {
        upsertActiveBotMessage((msg) => ({
          ...msg,
          isLoading: false,
          isError: true,
          text: String(
            event.message ?? "Something went wrong. Please try again.",
          ),
          thinkingSteps: [],
          thinkingLabel: null,
          isStreaming: false,
        }));
        setIsSending(false);
      }
    },
    onError: () => {
      upsertActiveBotMessage((msg) => ({
        ...msg,
        isLoading: false,
        isError: true,
        text: "WebSocket connection error. Please try again.",
        thinkingSteps: [],
        thinkingLabel: null,
        isStreaming: false,
      }));
      setIsSending(false);
    },
    onClose: () => {
      upsertActiveBotMessage((msg) => ({
        ...msg,
        isLoading: false,
        isError: true,
        text: "Something went wrong. Please try again.",
        thinkingSteps: [],
        thinkingLabel: null,
        isStreaming: false,
      }));
      setIsSending(false);
    },
  });

  const handleSend = useCallback(async () => {
    const text = input.trim();
    if (!text || !projectId || !accountId || isSending) return;

    const userMessage: Message = {
      id: `user-${Date.now()}`,
      text,
      sender: ChatSender.USER,
      timestamp: new Date(),
    };
    const botMessageId = `bot-${Date.now()}`;
    botMessageIdRef.current = botMessageId;
    const botLoadingMessage: Message = {
      id: botMessageId,
      text: "",
      sender: ChatSender.BOT,
      timestamp: new Date(),
      isLoading: true,
    };

    setMessages((prev) => [...prev, userMessage, botLoadingMessage]);
    setInput("");
    setIsSending(true);

    try {
      await connect(projectId);
      await sendUserMessage({
        type: "user_message",
        accountId,
        conversationId,
        message: text,
        envProducts,
      });
    } catch {
      updateActiveBotMessage("Unable to send message. Please try again.");
      setIsSending(false);
    }
  }, [
    accountId,
    connect,
    conversationId,
    envProducts,
    input,
    isSending,
    projectId,
    sendUserMessage,
    updateActiveBotMessage,
  ]);

  useEffect(() => {
    const lastMessage = messages[messages.length - 1];
    const isStreaming = lastMessage?.isStreaming === true;
    messagesEndRef.current?.scrollIntoView({
      behavior: isStreaming ? "auto" : "smooth",
    });
  }, [messages]);

  const isFloatingNoveraEnabled =
    window.config?.CUSTOMER_PORTAL_FLOATING_NOVERA_ENABLED !== false;

  if (!isFloatingNoveraEnabled || !projectId || !hasAgent) {
    return null;
  }

  const isSupportChatRoute =
    /\/projects\/[^/]+\/support\/chat(\/|$)/.test(location.pathname) ||
    /\/[^/]+\/support\/chat(\/|$)/.test(location.pathname);

  if (isSupportChatRoute || hideForDetailsActivityTab) {
    return null;
  }

  if (!isOpen) {
    return (
      <Box sx={{ position: "fixed", right: 24, bottom: 60, zIndex: 1300 }}>
        <Tooltip title="Chat with Novera AI" placement="left">
          <IconButton
            aria-label="Open Novera AI chat"
            onClick={() => {
              setIsOpen(true);
              setIsMinimized(false);
            }}
            sx={{
              width: 56,
              height: 56,
              boxShadow: 3,
              bgcolor: "primary.main",
              color: "common.white",
              "&:hover": {
                // Keep a yellow hover instead of transparent fade.
                bgcolor: "warning.main",
              },
            }}
          >
            <Bot size={24} color="white" />
          </IconButton>
        </Tooltip>
      </Box>
    );
  }

  return (
    <Box sx={{ position: "fixed", right: 24, bottom: 60, zIndex: 1300 }}>
      <Card sx={{ width: isMinimized ? 320 : 420 }}>
        <Box
          sx={{
            px: 2,
            py: 1.5,
            color: "common.white",
            background: "linear-gradient(90deg, #ff8f00 0%, #f57c00 100%)",
            borderRadius: isMinimized ? "inherit" : "unset",
          }}
        >
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
            }}
          >
            <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
              <Box
                sx={{
                  width: 32,
                  height: 32,
                  borderRadius: "50%",
                  bgcolor: "common.white",
                  color: "warning.main",
                  display: "inline-flex",
                  alignItems: "center",
                  justifyContent: "center",
                }}
              >
                <Bot size={18} />
              </Box>
              <Box>
                <Typography
                  variant="body2"
                  sx={{ fontWeight: 600, color: "inherit" }}
                >
                  Novera AI <Sparkles size={14} />
                </Typography>
                <Typography variant="caption" sx={{ color: "inherit" }}>
                  Always here to help
                </Typography>
              </Box>
            </Box>
            <Box sx={{ display: "flex", alignItems: "center" }}>
              <IconButton
                size="small"
                aria-label={isMinimized ? "Expand chat" : "Minimize chat"}
                sx={{ color: "common.white" }}
                onClick={() => setIsMinimized((v) => !v)}
              >
                {isMinimized ? <ChevronUp size={16} /> : <Minus size={16} />}
              </IconButton>
              <IconButton
                size="small"
                aria-label="Close chat"
                sx={{ color: "common.white" }}
                onClick={() => setIsOpen(false)}
              >
                <X size={16} />
              </IconButton>
            </Box>
          </Box>
        </Box>

        {!isMinimized && (
          <>
            <Box
              sx={{
                height: 420,
                overflowY: "auto",
                p: 2,
                bgcolor: "background.default",
              }}
            >
              {messages.map((msg) =>
                msg.isLoading ? (
                  <LoadingDotsBubble key={msg.id} />
                ) : (
                  <ChatMessageBubble key={msg.id} message={msg} />
                ),
              )}
              <div ref={messagesEndRef} />
            </Box>

            <Divider />
            <Box sx={{ p: 1.5, bgcolor: "background.paper" }}>
              <Box sx={{ display: "flex", gap: 1, alignItems: "center" }}>
                <TextField
                  fullWidth
                  placeholder="Ask Novera anything..."
                  multiline
                  minRows={2}
                  maxRows={4}
                  size="small"
                  value={input}
                  onChange={(e) => setInput(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter" && !e.shiftKey) {
                      e.preventDefault();
                      void handleSend();
                    }
                  }}
                />
                <IconButton
                  color="primary"
                  onClick={() => void handleSend()}
                  disabled={!input.trim() || isSending}
                >
                  <Send size={16} />
                </IconButton>
              </Box>
              <Typography variant="caption" color="text.secondary">
                Press Enter to send, Shift+Enter for new line
              </Typography>
            </Box>
          </>
        )}
      </Card>
    </Box>
  );
}
