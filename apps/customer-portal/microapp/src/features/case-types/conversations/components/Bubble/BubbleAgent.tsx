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
import { Card, Stack, Typography } from "@wso2/oxygen-ui";

import type { ChatMessage } from "@features/case-types/conversations/types";

import { TypewriterText } from "@shared/components/common";

import type { MESSAGE_AUTHOR_TYPES } from "@shared/constants";
import { useDateTime } from "@shared/hooks";

export interface BubbleAgentProps extends ChatMessage {
  author: typeof MESSAGE_AUTHOR_TYPES.AGENT;
  animated?: boolean;
  /** Pass a string to show as the thinking label, or false to hide. */
  thinking?: string | boolean;
  onAnimationComplete?: () => void;
}

export function BubbleAgent({
  content,
  timestamp,
  animated = true,
  thinking = false,
  onAnimationComplete,
}: BubbleAgentProps) {
  const { fromNow } = useDateTime();

  return (
    <Stack direction="row" justifyContent="start" width="100%">
      <Card
        component={Stack}
        p={1.5}
        width="100%"
        sx={{
          bgcolor: "background.paper",
          border: "1px solid color-mix(in srgb, var(--oxygen-palette-primary-main) 25%, transparent)",
          boxShadow: "inset 0 0 16px color-mix(in srgb, var(--oxygen-palette-primary-main) 12%, transparent)",
          position: "relative",
          overflow: "hidden",
        }}
      >
        <Stack direction="row" justifyContent="space-between" gap={1} mb={0.5}>
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
              {thinking}
            </Typography>
          ) : timestamp ? (
            <Typography variant="subtitle2" color="text.disabled">
              {fromNow(timestamp)}
            </Typography>
          ) : null}
        </Stack>

        <Typography variant="body2" component="span" sx={{ "& > *": { margin: 0, lineHeight: 1.7 } }}>
          <TypewriterText
            tokens={content.split("")}
            animated={animated}
            pending={!!thinking}
            onAnimationComplete={onAnimationComplete}
          />
        </Typography>
      </Card>
    </Stack>
  );
}
