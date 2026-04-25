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

import type { ChatHistoryItem } from "@features/support/types/conversations";
import {
  Box,
  Button,
  CardActions,
  CardContent,
  Form,
  Stack,
  Typography,
} from "@wso2/oxygen-ui";
import { Bot, ExternalLink, Play } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { ChatAction } from "@features/support/constants/supportConstants";
import {
  getChatActionColor,
  getChatStatusAction,
  getConversationStatusColor,
  formatDateTime,
} from "@features/support/utils/support";

export interface ChatHistoryCardProps {
  item: ChatHistoryItem;
  onItemAction?: (chatId: string, action: ChatAction) => void;
}

/**
 * Renders a single chat history card item for the support overview.
 *
 * @param {ChatHistoryCardProps} props - Chat item and action handler.
 * @returns {JSX.Element} The chat history card.
 */
export default function ChatHistoryCard({
  item,
  onItemAction,
}: ChatHistoryCardProps): JSX.Element {
  const action = getChatStatusAction(item.status);
  const statusColorPath = getConversationStatusColor(item.status);

  return (
    <Form.CardButton
      onClick={() => onItemAction?.(item.chatId, ChatAction.VIEW)}
      sx={{
        p: 2,
        width: "100%",
        minWidth: 0,
        overflow: "hidden",
        display: "flex",
        flexDirection: "column",
        alignItems: "stretch",
        gap: 1,
      }}
    >
      <CardContent
        sx={{
          p: 0,
          display: "flex",
          flexDirection: "column",
          gap: 1,
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
        <Box sx={{ pl: 4, minWidth: 0 }}>
          <Stack
            direction="row"
            spacing={1}
            alignItems="center"
            sx={{ minWidth: 0, flexWrap: "wrap", rowGap: 0.5 }}
          >
            <Typography variant="caption" color="text.secondary">
              {formatDateTime(item.startedTime, "short") ?? "--"}
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

      <CardActions sx={{ p: 0, justifyContent: "space-between", minWidth: 0 }}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 0.75, minWidth: 0 }}>
          <Box
            sx={{
              width: 8,
              height: 8,
              bgcolor: statusColorPath,
              borderRadius: "50%",
              flexShrink: 0,
            }}
          />
          <Typography
            variant="caption"
            sx={{
              color: statusColorPath,
              minWidth: 0,
              overflow: "hidden",
              textOverflow: "ellipsis",
              whiteSpace: "nowrap",
            }}
          >
            {item.status}
          </Typography>
        </Box>
        <Button
          component="span"
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
}
