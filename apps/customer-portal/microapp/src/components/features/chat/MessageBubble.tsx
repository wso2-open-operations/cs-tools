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
  alpha,
  Avatar,
  Box,
  Divider,
  Paper,
  Skeleton,
  Stack,
  Typography,
  type SxProps,
  type Theme,
} from "@wso2/oxygen-ui";
import { Bot, User } from "@wso2/oxygen-ui-icons-react";
import { KBCard } from "./KBCard";
import { ChecklistItem } from "./ChecklistItem";
import { TypewriterText } from "../../shared";

export type MessageBlock =
  | { type: "text"; value: string }
  | { type: "checklist"; items: string[] }
  | { type: "kb"; items: { id: string; title: string }[] };

export interface ChatMessage {
  author: "you" | "assistant";
  blocks: MessageBlock[];
  timestamp?: string;
  animated?: boolean;
  thinking?: boolean | string;
  onAnimationComplete?: () => void;
}

const AVATAR_SIZE = 32;

function NoveraAvatar() {
  return (
    <Box
      sx={{
        width: AVATAR_SIZE,
        height: AVATAR_SIZE,
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
  );
}

function UserAvatar() {
  return (
    <Avatar
      sx={{
        width: AVATAR_SIZE,
        height: AVATAR_SIZE,
        bgcolor: (theme) => alpha(theme.palette.primary.light, 0.1),
        color: (theme) => theme.palette.primary.main,
      }}
    >
      <User size={16} />
    </Avatar>
  );
}

export function MessageBubble({
  author,
  blocks,
  timestamp = "Just Now",
  sx,
  animated = true,
  thinking = false,
  onAnimationComplete,
}: ChatMessage & { sx?: SxProps<Theme> }) {
  const you = author === "you";

  return (
    <Stack direction="row" gap={1.5} sx={{ flexDirection: you ? "row-reverse" : "row", ...sx }}>
      {you ? <UserAvatar /> : <NoveraAvatar />}

      <Stack gap={0.5} sx={{ flex: you ? "0 1 auto" : 1, maxWidth: you ? "80%" : undefined, minWidth: 0 }}>
        <Stack direction="row" gap={1} alignItems="center" justifyContent={you ? "flex-end" : "flex-start"}>
          <Typography variant="body2" fontWeight="medium">
            {you ? "You" : "Novera"}
          </Typography>
          {!you && thinking ? (
            <Typography
              noWrap
              variant="caption"
              sx={{
                fontWeight: "medium",
                background: "linear-gradient(90deg, #aaa 25%, #fff 50%, #aaa 75%)",
                backgroundSize: "200% 100%",
                WebkitBackgroundClip: "text",
                WebkitTextFillColor: "transparent",
                backgroundClip: "text",
                animation: "shimmer 1.5s infinite linear",
                "@keyframes shimmer": {
                  from: { backgroundPosition: "200% center" },
                  to: { backgroundPosition: "-200% center" },
                },
                opacity: "80%",
              }}
            >
              {typeof thinking === "string" ? thinking : "Thinking"}
            </Typography>
          ) : (
            <Typography variant="caption" color="text.secondary">
              {timestamp}
            </Typography>
          )}
        </Stack>

        <Box
          component={you ? Paper : "div"}
          elevation={0}
          sx={
            you ? { bgcolor: (theme) => alpha(theme.palette.primary.main, 0.1), boxShadow: "none", p: 1.25 } : undefined
          }
        >
          <Stack gap={2}>
            {blocks.map((block, index) => {
              switch (block.type) {
                case "text":
                  return (
                    <Typography
                      key={index}
                      variant="body2"
                      component="span"
                      sx={{ "& > *": { margin: 0, lineHeight: 1.7 } }}
                    >
                      <TypewriterText
                        tokens={block.value.split("")}
                        animated={animated}
                        pending={!!thinking}
                        onAnimationComplete={onAnimationComplete}
                      />
                    </Typography>
                  );

                case "checklist":
                  return (
                    <Stack key={index} gap={1}>
                      {block.items.map((item, i) => (
                        <ChecklistItem key={i}>{item}</ChecklistItem>
                      ))}
                    </Stack>
                  );

                case "kb":
                  return (
                    <Stack key={index} gap={1.5}>
                      <Divider sx={{ my: 1 }} />
                      {block.items.map((item, i) => (
                        <KBCard key={i} id={item.id} title={item.title} />
                      ))}
                    </Stack>
                  );

                default:
                  return null;
              }
            })}
          </Stack>
        </Box>
      </Stack>
    </Stack>
  );
}

interface MessageBubbleSkeletonProps {
  author?: "you" | "assistant";
  sx?: SxProps<Theme>;
}

export function MessageBubbleSkeleton({ author = "assistant", sx }: MessageBubbleSkeletonProps) {
  const isYou = author === "you";

  return (
    <Stack direction="row" gap={1.5} width="100%" sx={{ flexDirection: isYou ? "row-reverse" : "row", ...sx }}>
      <Skeleton variant="circular" width={AVATAR_SIZE} height={AVATAR_SIZE} />

      <Stack gap={0.5} sx={{ flex: isYou ? "0 1 auto" : 1, minWidth: 0 }}>
        <Stack direction="row" gap={1} justifyContent={isYou ? "flex-end" : "flex-start"}>
          <Skeleton variant="text" width={50} height={20} />
          <Skeleton variant="text" width={60} height={20} />
        </Stack>

        <Stack gap={1}>
          <Skeleton variant="text" width={isYou ? 150 : "90%"} height={20} />
          <Skeleton variant="text" width={isYou ? 100 : "75%"} height={20} />
        </Stack>
      </Stack>
    </Stack>
  );
}
