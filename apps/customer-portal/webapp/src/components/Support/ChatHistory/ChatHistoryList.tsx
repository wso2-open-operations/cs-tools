// Copyright (c) 2025 WSO2 LLC. (https://www.wso2.com).
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

import { Box, Button, Card, Chip, Divider, Typography } from "@mui/material";
import React from "react";
import {
  ArrowRightIcon,
  BotIcon,
  AlertCircleIcon,
  CheckCircleIcon,
  ClockIcon,
  ExternalLinkIcon,
  ChatIcon,
  PlayIcon,
} from "../../../assets/icons/common-icons";

export interface ChatSession {
  id: string;
  title: string;
  status: string;
  time: string;
  messages: number;
  size: string;
  isResolved: boolean;
}

interface ChatHistoryListProps {
  chats: ChatSession[];
}

const getStatusConfig = (isResolved: boolean, status: string) => {
  if (isResolved) {
    return {
      bg: "#f0fdf4", // green-50
      text: "#16a34a", // green-600
      itemBg: "#f0fdf4",
      itemBorder: "#e5e7eb", // gray-200
      hoverBorder: "#d1d5db", // gray-300
      icon: <CheckCircleIcon width={12} height={12} />,
    };
  }
  if (status === "Abandoned") {
    return {
      bg: "#f3f4f6", // gray-50
      text: "#4b5563", // gray-600
      itemBg: "#f9fafb", // gray-50
      itemBorder: "#93c5fd", // blue-300 (per input styling, specifically for Abandoned item 5 in input)
      // wait, input example item 5 is Abandoned: bg-gray-50 border-blue-300 hover:border-blue-400.
      hoverBorder: "#60a5fa", // blue-400
      icon: <AlertCircleIcon width={12} height={12} />,
    };
  }
  // Still Open
  return {
    bg: "#eff6ff", // blue-50
    text: "#2563eb", // blue-600
    itemBg: "#eff6ff", // blue-50
    itemBorder: "#93c5fd", // blue-300
    hoverBorder: "#60a5fa", // blue-400
    icon: <ClockIcon width={12} height={12} />,
  };
};

export const ChatHistoryList: React.FC<ChatHistoryListProps> = ({ chats }) => {
  return (
    <Card
      sx={{
        p: 3,
        borderRadius: "12px",
        border: "1px solid",
        borderColor: "grey.200",
        height: "100%",
        boxShadow: "none",
      }}
    >
      <Box sx={{ display: "flex", alignItems: "center", gap: 2, mb: 3 }}>
        <Box
          sx={{
            width: 40,
            height: 40,
            backgroundColor: "#dbeafe", // blue-100
            borderRadius: "8px",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            color: "#2563eb", // blue-600
          }}
        >
          <ChatIcon width={20} height={20} />
        </Box>
        <Box>
          <Typography variant="h6" sx={{ fontWeight: 600 }}>
            TODO
          </Typography>
          {/* <Typography variant="caption" color="text.secondary">
            Recent Novera conversations
          </Typography> */}
        </Box>
      </Box>

      {/* <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {chats.map((chat) => {
          const config = getStatusConfig(chat.isResolved, chat.status);
          const isAbandoned = chat.status === "Abandoned";
          const isOpen = !chat.isResolved && !isAbandoned;

          return (
            <Box
              key={chat.id}
              sx={{
                p: 2,
                border: "1px solid",
                backgroundColor: config.itemBg,
                borderColor: "grey.200", // Default border grey.200
                borderRadius: "8px",
                cursor: "pointer",
                transition: "all 0.2s",
                "&:hover": {
                  borderColor: config.hoverBorder,
                  boxShadow: "0 1px 2px 0 rgba(0, 0, 0, 0.05)",
                },
              }}
            >
              <Box
                sx={{
                  display: "flex",
                  gap: 1.5,
                  mb: 1.5,
                  alignItems: "flex-start",
                }}
              >
                <Box sx={{ color: "grey.400", mt: 0.5 }}>
                  <BotIcon width={20} height={20} />
                </Box>
                <Box sx={{ minWidth: 0, flex: 1 }}>
                  <Typography
                    variant="body2"
                    sx={{
                      mb: 1,
                      fontWeight: 400,
                      display: "-webkit-box",
                      WebkitLineClamp: 2,
                      WebkitBoxOrient: "vertical",
                      overflow: "hidden",
                    }}
                  >
                    {chat.title}
                  </Typography>
                  <Box
                    sx={{
                      display: "flex",
                      alignItems: "center",
                      gap: 1,
                      flexWrap: "wrap",
                      color: "text.secondary",
                    }}
                  >
                    <Typography variant="caption">{chat.time}</Typography>
                    <Typography variant="caption">•</Typography>
                    <Typography variant="caption">
                      {chat.messages} messages
                    </Typography>
                    <Typography variant="caption">•</Typography>
                    <Typography variant="caption">{chat.size}</Typography>
                  </Box>
                </Box>
              </Box>

              <Box
                sx={{
                  display: "flex",
                  justifyContent: "space-between",
                  alignItems: "center",
                }}
              >
                <Chip
                  icon={config.icon}
                  label={chat.status}
                  size="small"
                  sx={{
                    backgroundColor: "transparent",
                    color: config.text,
                    height: 24,
                    fontSize: "0.75rem",
                    fontWeight: 500,
                    p: 0,
                    "& .MuiChip-icon": { color: "inherit", ml: 0 },
                    "& .MuiChip-label": { pl: 1 },
                  }}
                />
                <Button
                  size="small"
                  startIcon={
                    isOpen || isAbandoned ? (
                      <PlayIcon width={12} height={12} />
                    ) : (
                      <ExternalLinkIcon width={12} height={12} />
                    )
                  }
                  sx={{
                    minWidth: "auto",
                    height: 28,
                    fontSize: "0.75rem",
                    color: isOpen || isAbandoned ? "#2563eb" : "text.secondary",
                    "&:hover": {
                      backgroundColor:
                        isOpen || isAbandoned ? "#dbeafe" : "#f3f4f6",
                    },
                    textTransform: "none",
                    px: 1.5,
                  }}
                >
                  {isOpen || isAbandoned ? "Resume" : "View"}
                </Button>
              </Box>
            </Box>
          );
        })}
      </Box> */}

      {/* <Divider sx={{ my: 2 }} />

      <Button
        fullWidth
        endIcon={<ArrowRightIcon width={16} height={16} />}
        sx={{
          justifyContent: "space-between",
          color: "#2563eb",
          "&:hover": { backgroundColor: "#eff6ff", color: "#1d4ed8" },
          textTransform: "none",
        }}
      >
        View all chat history
      </Button> */}
    </Card>
  );
};
