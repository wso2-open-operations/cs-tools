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

import { Box, Paper, Typography } from "@wso2/oxygen-ui";
import { Bot } from "@wso2/oxygen-ui-icons-react";
import { type JSX } from "react";

interface Message {
  id: string;
  text: string;
  sender: "user" | "bot";
  timestamp: Date;
}

interface ChatMessageBubbleProps {
  message: Message;
}

/**
 * Renders a single chat message bubble.
 *
 * Supports both user and bot messages with appropriate styling
 * and avatar display.
 *
 * @returns The ChatMessageBubble JSX element.
 */
export default function ChatMessageBubble({
  message,
}: ChatMessageBubbleProps): JSX.Element {
  const isUser = message.sender === "user";

  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: isUser ? "row-reverse" : "row",
        gap: 1.5,
      }}
    >
      {!isUser && (
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
          <Bot size={16} color="#ea580c" />
        </Paper>
      )}
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
          <Typography variant="body2" sx={{ whiteSpace: "pre-line" }}>
            {message.text}
          </Typography>
        </Paper>
      </Box>
    </Box>
  );
}
