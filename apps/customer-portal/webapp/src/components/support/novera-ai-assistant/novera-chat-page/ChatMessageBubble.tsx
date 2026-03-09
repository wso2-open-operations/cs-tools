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
  CircularProgress,
  Paper,
  Typography,
} from "@wso2/oxygen-ui";
import { Bot, FileText, User } from "@wso2/oxygen-ui-icons-react";
import ReactMarkdown from "react-markdown";
import { type JSX } from "react";
import type { Message } from "@pages/NoveraChatPage";
import RecommendationsCard from "@components/support/novera-ai-assistant/novera-chat-page/RecommendationsCard";
import { AVATAR_ICON_COLOR } from "./chatConstants";

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
  isCreateCaseLoading?: boolean;
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
  onCreateCase,
  isCreateCaseLoading = false,
}: ChatMessageBubbleProps): JSX.Element {
  const isUser = message.sender === "user";

  const displayText = message.isError ? "Something went wrong" : message.text;

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

  const avatarIcon = (
    <Paper
      sx={{
        width: (theme) => theme.spacing(4),
        height: (theme) => theme.spacing(4),
        borderRadius: "50%",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        flexShrink: 0,
      }}
    >
      {isUser ? (
        <User size={16} color={AVATAR_ICON_COLOR} />
      ) : (
        <Bot size={16} color={AVATAR_ICON_COLOR} />
      )}
    </Paper>
  );

  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: isUser ? "row-reverse" : "row",
        gap: 1.5,
        alignItems: isUser ? "flex-end" : "flex-start",
      }}
    >
      {avatarIcon}
      <Box sx={{ maxWidth: "80%" }}>
        <Paper
          sx={{
            p: 2,
            bgcolor: isUser ? "primary.main" : "background.paper",
            color: isUser ? "common.white" : "text.primary",
            borderRadius: (theme) =>
              isUser
                ? `${theme.spacing(2)} ${theme.spacing(2)} ${theme.spacing(0.5)} ${theme.spacing(2)}`
                : `${theme.spacing(0.5)} ${theme.spacing(2)} ${theme.spacing(2)} ${theme.spacing(2)}`,
            boxShadow: "none",
            border: "1px solid",
            borderColor: isUser ? "primary.main" : "divider",
          }}
        >
          {isUser ? (
            <Typography variant="body2" sx={{ whiteSpace: "pre-line" }}>
              {displayText}
            </Typography>
          ) : message.isError ? (
            <Typography variant="body2" color="error.main">
              {displayText}
            </Typography>
          ) : (
            <Box
              sx={{
                "& h1:first-of-type, & h2:first-of-type, & h3:first-of-type, & p:first-of-type":
                  { mt: 0 },
              }}
            >
              <ReactMarkdown components={markdownComponents}>
                {displayText}
              </ReactMarkdown>
            </Box>
          )}
          {!isUser &&
            (message.showCreateCaseAction || message.isError) &&
            onCreateCase &&
            (isCreateCaseLoading ? (
              <Box
                sx={{ mt: 2, display: "flex", alignItems: "center", gap: 1 }}
              >
                <Button
                  variant="outlined"
                  size="small"
                  color="warning"
                  disabled
                  startIcon={<CircularProgress color="inherit" size={14} />}
                >
                  Processing
                </Button>
                <Typography variant="caption" color="text.secondary">
                  Skip the chat and create a support case now
                </Typography>
              </Box>
            ) : (
              <Box
                sx={{
                  mt: 2,
                  display: "flex",
                  alignItems: "center",
                  gap: 1,
                  flexWrap: "wrap",
                }}
              >
                <Button
                  variant="outlined"
                  size="small"
                  color="warning"
                  onClick={onCreateCase}
                  startIcon={<FileText size={14} />}
                >
                  Create Case
                </Button>
                <Typography variant="caption" color="text.secondary">
                  Skip the chat and create a support case now
                </Typography>
              </Box>
            ))}
        </Paper>

        {!isUser &&
          message.recommendations &&
          message.recommendations.length > 0 && (
            <RecommendationsCard recommendations={message.recommendations} />
          )}

        <Box
          sx={{
            mt: 0.5,
            display: "flex",
            justifyContent: isUser ? "flex-end" : "flex-start",
          }}
        >
          <Typography variant="caption" color="text.secondary">
            {formattedTime}
          </Typography>
        </Box>
      </Box>
    </Box>
  );
}
