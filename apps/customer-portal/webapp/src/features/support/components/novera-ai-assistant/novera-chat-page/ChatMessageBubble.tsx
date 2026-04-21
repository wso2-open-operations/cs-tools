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

import type { ChatMessageBubbleProps } from "@features/support/types/supportComponents";
import { Avatar, Box, Paper, Stack, Typography, alpha, useTheme } from "@wso2/oxygen-ui";
import { Bot, User } from "@wso2/oxygen-ui-icons-react";
import MarkdownIt from "markdown-it";
import { type JSX, useEffect, useMemo, useRef } from "react";
import { ChatSender } from "@features/support/types/conversations";
import {
  NOVERA_ANALYZING_PLACEHOLDER_TEXT,
  NOVERA_DISPLAY_NAME,
} from "@features/support/constants/chatConstants";
import RecommendationsCard from "@features/support/components/novera-ai-assistant/novera-chat-page/RecommendationsCard";
import { resolveDisplayTimeZone } from "@utils/dateTime";

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

const md = new MarkdownIt({ linkify: true, breaks: false });

/** Block unsafe href attributes before the HTML reaches the DOM. */
const defaultLinkOpenRenderer =
  md.renderer.rules.link_open ??
  ((tokens, idx, options, _env, self) => self.renderToken(tokens, idx, options));

md.renderer.rules.link_open = (tokens, idx, options, env, self) => {
  const token = tokens[idx];
  const hrefIndex = token.attrIndex("href");
  if (hrefIndex >= 0) {
    const href = token.attrs?.[hrefIndex]?.[1];
    if (!isSafeHref(href)) {
      // Strip the href so the anchor renders as plain text wrapper
      token.attrs?.splice(hrefIndex, 1);
    } else {
      token.attrSet("target", "_blank");
      token.attrSet("rel", "noopener noreferrer");
    }
  }
  return defaultLinkOpenRenderer(tokens, idx, options, env, self);
};

function MarkdownContent({ text }: { text: string }) {
  const html = useMemo(() => md.render(text), [text]);
  return <Box dangerouslySetInnerHTML={{ __html: html }} />;
}

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
}: ChatMessageBubbleProps): JSX.Element {
  const isUser = message.sender === ChatSender.USER;

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
    streamContainerRef.current.scrollTop =
      streamContainerRef.current.scrollHeight;
  }, [message.isStreaming, streamBodyText]);

  // Safely format timestamp with date and time, fallback for invalid dates
  let formattedDateTime = "--";
  try {
    const dateObj =
      message.timestamp instanceof Date
        ? message.timestamp
        : new Date(message.timestamp);

    if (!Number.isNaN(dateObj.getTime())) {
      const tz = resolveDisplayTimeZone();
      const timeStr = dateObj.toLocaleTimeString("en-US", {
        hour: "2-digit",
        minute: "2-digit",
        hour12: true,
        timeZone: tz,
      });
      const dateStr = dateObj.toLocaleDateString("en-US", {
        month: "short",
        day: "numeric",
        year: "numeric",
        timeZone: tz,
      });
      formattedDateTime = `${dateStr} ${timeStr}`;
    }
  } catch {
    // formattedDateTime remains "--"
  }

  const theme = useTheme();
  const primaryLight = theme.palette.primary?.light ?? "#fa7b3f";
  const primaryBg = alpha(primaryLight, 0.1);

  if (isUser) {
    return (
      <Stack direction="row" alignItems="flex-start" sx={{ flexDirection: "row-reverse", gap: 2 }}>
        <Avatar
          sx={{
            width: 32,
            height: 32,
            fontSize: "0.75rem",
            flexShrink: 0,
            bgcolor: primaryBg,
            color: theme.palette.primary.main,
          }}
        >
          <User size={14} />
        </Avatar>
        <Stack spacing={0.75} sx={{ minWidth: 0, alignItems: "flex-end" }}>
          <Stack direction="row" alignItems="center" sx={{ flexDirection: "row-reverse", gap: 1 }}>
            <Typography variant="body2" color="text.primary" fontWeight={500}>
              {message.createdBy || "You"}
            </Typography>
            <Typography variant="caption" color="text.secondary">
              {formattedDateTime}
            </Typography>
          </Stack>
          <Paper
            elevation={0}
            sx={{
              p: 1.25,
              bgcolor: primaryBg,
              color: "text.primary",
              boxShadow: "none",
            }}
          >
            <Typography variant="body2" sx={{ whiteSpace: "pre-line" }}>
              {displayText}
            </Typography>
          </Paper>
        </Stack>
      </Stack>
    );
  }

  // Bot message
  return (
    <Box>
      <Stack direction="row" alignItems="flex-start" sx={{ gap: 2 }}>
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
        <Stack spacing={0.75} sx={{ minWidth: 0, flex: 1 }}>
          <Stack direction="row" alignItems="center" sx={{ gap: 1 }}>
            <Typography variant="body2" color="text.primary" fontWeight={500}>
              {NOVERA_DISPLAY_NAME}
            </Typography>
            {!hideFeedbackRow && (
              <Typography variant="caption" color="text.secondary">
                {formattedDateTime}
              </Typography>
            )}
          </Stack>

          {/* Message content */}
          <Box>
            {message.isError ? (
              <Typography variant="body2" color="error.main">
                {displayText}
              </Typography>
            ) : showThinkingStreamFrame ? (
              <Paper
                sx={(t) => ({
                  position: "relative",
                  bgcolor: alpha(t.palette.text.primary, 0.03),
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
                      sx={(t) => ({
                        color: "primary.main",
                        whiteSpace: "pre-wrap",
                        lineHeight: STREAM_LINE_HEIGHT,
                        mb: 0.5,
                        backgroundImage: `linear-gradient(90deg, ${t.palette.primary.dark} 0%, ${t.palette.primary.main} 30%, ${t.palette.primary.light} 55%, ${t.palette.primary.main} 80%, ${t.palette.primary.dark} 100%)`,
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
              >
                <MarkdownContent text={displayText} />
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
          </Box>
        </Stack>
      </Stack>

      {/* Recommendations - shown after message content */}
      {message.recommendations && message.recommendations.length > 0 && (
        <Box sx={{ ml: "48px", mt: 1 }}>
          <RecommendationsCard recommendations={message.recommendations} />
        </Box>
      )}
    </Box>
  );
}
