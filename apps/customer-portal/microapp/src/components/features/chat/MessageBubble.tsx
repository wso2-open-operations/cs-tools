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

import { Box, Card, Divider, pxToRem, Skeleton, Stack, Typography, type SxProps, type Theme } from "@wso2/oxygen-ui";
import { Sparkle } from "@wso2/oxygen-ui-icons-react";
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
    <Stack direction="row" justifyContent={you ? "end" : "start"}>
      <Card
        component={Stack}
        p={1.5}
        ml={you ? 10 : undefined}
        width={you ? "fit-content" : "100%"}
        sx={{ ...sx, bgcolor: "background.paper" }}
      >
        {!you && (
          <Stack direction="row" justifyContent="space-between" gap={1} mb={0.5}>
            <Box color="primary.main">
              <Sparkle size={pxToRem(18)} />
            </Box>
            {thinking ? (
              <Typography
                noWrap
                variant="subtitle2"
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
              <Typography variant="subtitle2" color="text.disabled">
                {timestamp}
              </Typography>
            )}
          </Stack>
        )}

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

        {you && (
          <Stack direction="row" justifyContent="end">
            <Typography variant="subtitle2" color="text.disabled" mt={1}>
              {timestamp}
            </Typography>
          </Stack>
        )}
      </Card>
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
    <Stack direction="row" justifyContent={isYou ? "end" : "start"} width="100%">
      <Card
        component={Stack}
        p={1.5}
        ml={isYou ? 10 : undefined}
        width={isYou ? "fit-content" : "100%"}
        sx={{
          ...sx,
          bgcolor: "background.paper",
          borderStyle: "dashed",
          borderWidth: 1,
          borderColor: "divider",
        }}
      >
        {!isYou && (
          <Stack direction="row" justifyContent="start" gap={1} mb={1.5}>
            <Skeleton variant="circular" width={pxToRem(18)} height={pxToRem(18)} />
            <Skeleton variant="text" width={60} height={20} />
          </Stack>
        )}

        <Stack gap={1}>
          <Skeleton variant="text" width={isYou ? 150 : "90%"} height={20} />
          <Skeleton variant="text" width={isYou ? 100 : "75%"} height={20} />
        </Stack>

        {isYou && (
          <Stack direction="row" justifyContent="end" mt={1}>
            <Skeleton variant="text" width={50} height={20} />
          </Stack>
        )}
      </Card>
    </Stack>
  );
}
