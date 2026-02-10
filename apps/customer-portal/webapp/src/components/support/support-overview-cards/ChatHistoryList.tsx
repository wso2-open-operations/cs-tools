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
  CardActions,
  CardContent,
  Chip,
  Form,
  Stack,
  Typography,
  alpha,
  type Theme,
} from "@wso2/oxygen-ui";
import {
  Bot,
  CircleCheck,
  Clock,
  ExternalLink,
  Play,
} from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import type { ChatHistoryItem } from "@models/responses";
import { ChatAction } from "@constants/supportConstants";
import {
  getChatActionColor,
  getChatStatusAction,
  getChatStatusColor,
  resolveColorFromTheme,
} from "@utils/support";
import ChatHistorySkeleton from "@components/support/support-overview-cards/ChatHistorySkeleton";

export interface ChatHistoryListProps {
  items: ChatHistoryItem[];
  isLoading?: boolean;
  onItemAction?: (chatId: string, action: ChatAction) => void;
}

/**
 * Renders a list of chat history rows for the support overview card.
 *
 * @param {ChatHistoryListProps} props - Items and optional action handler.
 * @returns {JSX.Element} The list of chat rows.
 */
export default function ChatHistoryList({
  items,
  isLoading,
  onItemAction,
}: ChatHistoryListProps): JSX.Element {
  if (isLoading) {
    return <ChatHistorySkeleton />;
  }

  if (items.length === 0) {
    return (
      <Typography variant="body2" color="text.secondary">
        No chat history.
      </Typography>
    );
  }

  return (
    <Box
      sx={{ display: "flex", flexDirection: "column", gap: 1.5, width: "100%" }}
    >
      {items.map((item) => {
        const action = getChatStatusAction(item.status);
        const StatusIcon = action === ChatAction.VIEW ? CircleCheck : Clock;
        const chipColorPath = getChatStatusColor(item.status);

        return (
          <Form.CardButton
            key={item.chatId}
            onClick={() => onItemAction?.(item.chatId, action)}
            sx={{
              p: 2,
              display: "flex",
              flexDirection: "column",
              alignItems: "stretch",
              gap: 1,
              cursor: "pointer",
              "&:hover": {
                bgcolor: "action.hover",
              },
            }}
          >
            <CardContent
              sx={{
                p: 0,
                display: "flex",
                flexDirection: "column",
                gap: 0.5,
              }}
            >
              <Box sx={{ display: "flex", gap: 1.5, alignItems: "center" }}>
                <Box sx={{ display: "flex", flexShrink: 0 }}>
                  <Bot size={20} />
                </Box>
                <Box sx={{ minWidth: 0, flexGrow: 1 }}>
                  <Typography
                    variant="body2"
                    fontWeight={500}
                    color="text.primary"
                    sx={{
                      overflow: "hidden",
                      textOverflow: "ellipsis",
                      display: "-webkit-box",
                      WebkitLineClamp: 1,
                      WebkitBoxOrient: "vertical",
                    }}
                  >
                    {item.title}
                  </Typography>
                </Box>
              </Box>
              <Box sx={{ pl: 4 }}>
                <Stack direction="row" spacing={1} alignItems="center">
                  <Typography variant="caption" color="text.secondary">
                    {item.startedTime}
                  </Typography>
                  <Typography variant="caption" color="text.secondary">
                    •
                  </Typography>
                  <Typography variant="caption" color="text.secondary">
                    {item.messages}{" "}
                    {item.messages === 1 ? "message" : "messages"}
                  </Typography>
                  <Typography variant="caption" color="text.secondary">
                    •
                  </Typography>
                  <Typography variant="caption" color="text.secondary">
                    {item.kbArticles}{" "}
                    {item.kbArticles === 1 ? "KB article" : "KB articles"}
                  </Typography>
                </Stack>
              </Box>
            </CardContent>

            <CardActions sx={{ p: 0, justifyContent: "space-between" }}>
              <Chip
                size="small"
                variant="outlined"
                label={item.status}
                sx={{
                  px: 0.5,
                  bgcolor: (theme: Theme) =>
                    alpha(resolveColorFromTheme(chipColorPath, theme), 0.1),
                  color: chipColorPath,
                  "& .MuiChip-icon": {
                    color: "inherit",
                  },
                }}
                icon={<StatusIcon size={12} />}
              />
              <Button
                size="small"
                variant="text"
                color={getChatActionColor(action)}
                disableRipple
                onClick={(e) => {
                  e.stopPropagation();
                  onItemAction?.(item.chatId, action);
                }}
                startIcon={
                  action === ChatAction.VIEW ? (
                    <ExternalLink size={12} />
                  ) : (
                    <Play size={12} />
                  )
                }
                sx={{
                  textTransform: "none",
                  fontWeight: 500,
                  minWidth: 0,
                  p: 0,
                }}
              >
                {action === ChatAction.VIEW ? "View" : "Resume"}
              </Button>
            </CardActions>
          </Form.CardButton>
        );
      })}
    </Box>
  );
}
