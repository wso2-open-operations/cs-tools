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

import { Box, Paper, Typography, IconButton, alpha } from "@wso2/oxygen-ui";
import { Bot, User, ThumbsUp, ThumbsDown } from "@wso2/oxygen-ui-icons-react";
import ReactMarkdown from "react-markdown";
import { type JSX, useEffect, useRef, useState } from "react";
import { ChatSender } from "@/types/conversations";
import type { Message } from "@/types/conversations";
import {
  NOVERA_ANALYZING_PLACEHOLDER_TEXT,
  NOVERA_DISPLAY_NAME,
} from "@constants/chatConstants";
import RecommendationsCard from "@components/support/novera-ai-assistant/novera-chat-page/RecommendationsCard";

/** Tighter line breaks while tokens stream (model often sends blank lines). */
function collapseStreamLineBreaks(s: string): string {
  return s.replace(/\n{2,}/g, "\n");
}

const thinkingLegendDotsSx = {
  display: "inline-block",
  ml: 0.25,
  animation: "noveraThinkingDots 1s ease-in-out infinite",
  "@keyframes noveraThinkingDots": {
    "0%, 100%": { opacity: 0.2 },
    "50%": { opacity: 1 },
  },
} as const;

const STREAM_LINE_HEIGHT = 1.35;
const STREAM_VISIBLE_LINE_COUNT = 3;
const STREAM_MAX_HEIGHT_EM = STREAM_LINE_HEIGHT * STREAM_VISIBLE_LINE_COUNT;

const primaryWaveTextSx = {
  display: "inline-block",
  backgroundSize: "220% 100%",
  WebkitBackgroundClip: "text",
  backgroundClip: "text",
  WebkitTextFillColor: "transparent",
  animation: "noveraWaveText 1.8s linear infinite",
  "@keyframes noveraWaveText": {
    "0%": { backgroundPosition: "220% 0" },
    "100%": { backgroundPosition: "-220% 0" },
  },
} as const;

/** Safe URL protocols for markdown links. Blocks javascript:, data:, etc. */
const SAFE_PROTOCOLS = ["http:", "https:"];

function isSafeHref(href: string | undefined): href is string {
  if (!href || typeof href !== "string") return false;
  try {
    const parsed = new URL(href, "https://invalid.invalid");
    return SAFE_PROTOCOLS.includes(parsed.protocol);
  } catch {
    return false;
  }
}

interface ChatMessageBubbleProps {
  message: Message;
  onCreateCase?: () => void;
  onThumbsUp?: (messageId: string) => void;
  onThumbsDown?: (messageId: string) => void;
}

/** Typography mapping for markdown elements (bot messages). */
const markdownComponents: React.ComponentProps<
  typeof ReactMarkdown
>["components"] = {
  h1: ({ children }) => (
    <Typography
      variant="h6"
      component="h1"
      sx={{ mt: 2, mb: 1, fontWeight: 600 }}
    >
      {children}
    </Typography>
  ),
  h2: ({ children }) => (
    <Typography
      variant="subtitle1"
      component="h2"
      sx={{ mt: 2, mb: 1, fontWeight: 600 }}
    >
      {children}
    </Typography>
  ),
  h3: ({ children }) => (
    <Typography
      variant="subtitle2"
      component="h3"
      sx={{ mt: 1.5, mb: 0.5, fontWeight: 600 }}
    >
      {children}
    </Typography>
  ),
  p: ({ children }) => (
    <Typography variant="body2" component="p" sx={{ mb: 1, lineHeight: 1.6 }}>
      {children}
    </Typography>
  ),
  ul: ({ children }) => (
    <Box component="ul" sx={{ m: 0, pl: 2.5, mb: 1 }}>
      {children}
    </Box>
  ),
  ol: ({ children }) => (
    <Box component="ol" sx={{ m: 0, pl: 2.5, mb: 1 }}>
      {children}
    </Box>
  ),
  li: ({ children }) => (
    <Typography
      variant="body2"
      component="li"
      sx={{ mb: 0.5, lineHeight: 1.6 }}
    >
      {children}
    </Typography>
  ),
  strong: ({ children }) => (
    <Box component="strong" sx={{ fontWeight: 600 }}>
      {children}
    </Box>
  ),
  a: ({ href, children }) =>
    isSafeHref(href) ? (
      <Typography
        component="a"
        href={href}
        target="_blank"
        rel="noopener noreferrer"
        variant="body2"
        sx={{ color: "primary.main", textDecoration: "underline" }}
      >
        {children}
      </Typography>
    ) : (
      <Typography variant="body2" component="span">
        {children}
      </Typography>
    ),
};

/**
 * Renders a single chat message bubble.
 *
 * Supports both user and bot messages with appropriate styling,
 * avatar display, markdown formatting for bot messages,
 * optional Create Case action, and displays error text when present.
 *
 * @returns The ChatMessageBubble JSX element.
 */
export default function ChatMessageBubble({
  message,
  onThumbsUp,
  onThumbsDown,
}: ChatMessageBubbleProps): JSX.Element {
  const isUser = message.sender === ChatSender.USER;
  const [thumbsState, setThumbsState] = useState<"up" | "down" | null>(null);
  const hasFeedbackSelection = thumbsState !== null;

  const displayText = message.isError ? "Something went wrong" : message.text;

  /** Until the final assistant message, hide thumbs and timestamp only (header stays). */
  const hideFeedbackRow =
    !message.isError &&
    (message.isStreaming ||
      !!message.thinkingLabel ||
      message.text === NOVERA_ANALYZING_PLACEHOLDER_TEXT);

  /** Faded frame wraps analyzing, live thinking steps, and streamed tokens. */
  const showThinkingStreamFrame = hideFeedbackRow;

  const streamBodyText = collapseStreamLineBreaks(displayText);
  const analyzingLeadText = "Novera is analyzing your request...";
  const showLiveThinkingStatus =
    !!message.thinkingLabel ||
    (!message.isStreaming &&
      message.text === NOVERA_ANALYZING_PLACEHOLDER_TEXT);
  const liveThinkingStatus = message.thinkingLabel ?? analyzingLeadText;
  const streamContainerRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!message.isStreaming) return;
    if (!streamContainerRef.current) return;
    streamContainerRef.current.scrollTop = streamContainerRef.current.scrollHeight;
  }, [message.isStreaming, streamBodyText]);

  // Safely format timestamp with fallback for invalid dates
  let formattedTime = "--";
  try {
    const dateObj =
      message.timestamp instanceof Date
        ? message.timestamp
        : new Date(message.timestamp);

    if (!Number.isNaN(dateObj.getTime())) {
      formattedTime = dateObj.toLocaleTimeString("en-US", {
        hour: "2-digit",
        minute: "2-digit",
        hour12: true,
      });
    }
  } catch {
    // formattedTime remains "--"
  }

  if (isUser) {
    // User message with avatar
    return (
      <Box
        sx={{
          display: "flex",
          justifyContent: "flex-end",
          gap: 1.5,
          alignItems: "flex-end",
        }}
      >
        <Box sx={{ maxWidth: "80%" }}>
          <Paper
            sx={{
              p: 2,
              bgcolor: "action.hover",
              color: "text.primary",
              borderRadius: (theme) =>
                `${theme.spacing(2)} ${theme.spacing(2)} ${theme.spacing(0.5)} ${theme.spacing(2)}`,
              boxShadow: "none",
              border: "1px solid",
              borderColor: "divider",
            }}
          >
            <Typography variant="body2" sx={{ whiteSpace: "pre-line" }}>
              {displayText}
            </Typography>
          </Paper>
        </Box>
        <Paper
          sx={{
            width: (theme) => theme.spacing(4),
            height: (theme) => theme.spacing(4),
            borderRadius: "50%",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            flexShrink: 0,
            bgcolor: "primary.lighter",
            color: "primary.main",
          }}
        >
          <User size={16} />
        </Paper>
      </Box>
    );
  }

  // Bot message - new custom layout
  return (
    <Box sx={{ maxWidth: "80%" }}>
      <Box sx={{ mb: 3 }}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 2, mb: 2 }}>
          <Box
            sx={{
              width: 24,
              height: 24,
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
          <Typography
            variant="body2"
            sx={{ fontWeight: 600, color: "text.primary" }}
          >
            {NOVERA_DISPLAY_NAME}
          </Typography>
        </Box>

        {/* Message content */}
        {message.isError ? (
          <Typography variant="body2" color="error.main">
            {displayText}
          </Typography>
        ) : showThinkingStreamFrame ? (
          <Paper
            sx={(theme) => ({
              position: "relative",
              mt: 2.5,
              bgcolor: alpha(theme.palette.text.primary, 0.03),
              px: 2,
              py: 2,
              pt: 2.25,
            })}
            aria-busy={message.isStreaming ? true : undefined}
          >
            <Typography
              component="span"
              variant="caption"
              sx={{
                position: "absolute",
                top: -10,
                px: 1,
                lineHeight: 1.2,
                fontWeight: 600,
                color: "text.primary",
                bgcolor: "background.paper",
                borderRadius: 1,
              }}
            >
              Thinking
              <Box component="span" aria-hidden sx={thinkingLegendDotsSx}>
                ...
              </Box>
            </Typography>
            <Box aria-live="polite">
              {showLiveThinkingStatus && (
                <Typography
                  variant="body2"
                  sx={(theme) => ({
                    color: "primary.main",
                    whiteSpace: "pre-wrap",
                    lineHeight: STREAM_LINE_HEIGHT,
                    mb: 0.5,
                    backgroundImage: `linear-gradient(90deg, ${theme.palette.primary.dark} 0%, ${theme.palette.primary.main} 30%, ${theme.palette.primary.light} 55%, ${theme.palette.primary.main} 80%, ${theme.palette.primary.dark} 100%)`,
                    ...primaryWaveTextSx,
                  })}
                >
                  {liveThinkingStatus}
                </Typography>
              )}
              {message.isStreaming && (
                <Typography
                  component="div"
                  variant="body2"
                  ref={streamContainerRef}
                  sx={{
                    whiteSpace: "pre-wrap",
                    lineHeight: STREAM_LINE_HEIGHT,
                    m: 0,
                    mt: 0.5,
                    color: "text.primary",
                    maxHeight: `${STREAM_MAX_HEIGHT_EM}em`,
                    overflowY: "auto",
                  }}
                >
                  {streamBodyText}
                </Typography>
              )}
            </Box>
          </Paper>
        ) : (
          <Box
            sx={{
              "& h1:first-of-type, & h2:first-of-type, & h3:first-of-type, & p:first-of-type":
                { mt: 0 },
            }}
            className="prose prose-sm max-w-none text-gray-800"
          >
            <ReactMarkdown components={markdownComponents}>
              {displayText}
            </ReactMarkdown>
            {message.thinkingSteps && message.thinkingSteps.length > 0 && (
              <Box sx={{ mt: 1 }}>
                {message.thinkingSteps.map((step, idx) => (
                  <Typography
                    key={`${message.id}-thinking-${idx}`}
                    variant="caption"
                    color="text.secondary"
                    sx={{ display: "block" }}
                  >
                    {`• ${step}`}
                  </Typography>
                ))}
              </Box>
            )}
          </Box>
        )}

        {/* Thumbs up/down and time — hide until final response */}
        {!hideFeedbackRow &&
          (!message.recommendations ||
            message.recommendations.length === 0) && (
            <Box sx={{ display: "flex", alignItems: "center", gap: 3, mt: 3 }}>
              <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                <IconButton
                  size="small"
                  onClick={() => {
                    if (hasFeedbackSelection) return;
                    setThumbsState("up");
                    onThumbsUp?.(message.id);
                  }}
                  aria-pressed={thumbsState === "up"}
                  sx={{
                    p: 0.5,
                    color:
                      thumbsState === "up" ? "success.main" : "text.secondary",
                    "&:hover": {
                      color: "success.main",
                      bgcolor: "success.lighter",
                    },
                  }}
                  aria-label="Like this response"
                >
                  <ThumbsUp size={14} />
                </IconButton>
                <IconButton
                  size="small"
                  onClick={() => {
                    if (hasFeedbackSelection) return;
                    setThumbsState("down");
                    onThumbsDown?.(message.id);
                  }}
                  aria-pressed={thumbsState === "down"}
                  sx={{
                    p: 0.5,
                    color:
                      thumbsState === "down" ? "error.main" : "text.secondary",
                    "&:hover": {
                      color: "error.main",
                      bgcolor: "error.lighter",
                    },
                  }}
                  aria-label="Dislike this response"
                >
                  <ThumbsDown size={14} />
                </IconButton>
              </Box>
              <Typography variant="caption" color="text.secondary">
                {formattedTime}
              </Typography>
            </Box>
          )}
      </Box>

      {/* Recommendations - shown after message content */}
      {!isUser &&
        message.recommendations &&
        message.recommendations.length > 0 && (
          <RecommendationsCard recommendations={message.recommendations} />
        )}
    </Box>
  );
}
