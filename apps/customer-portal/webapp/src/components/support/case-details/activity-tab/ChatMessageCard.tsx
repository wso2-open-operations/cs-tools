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

import { Box, Button, Divider, Paper } from "@wso2/oxygen-ui";
import { ChevronDown } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";

// Line count threshold for showing expand button (approximately 4 lines).
const COLLAPSE_CHAR_THRESHOLD = 200;

export interface ChatMessageCardProps {
  htmlContent: string;
  isExpanded: boolean;
  onToggleExpand: () => void;
  isCurrentUser: boolean;
  primaryBg: string;
}

/**
 * Card-style chat message with collapsible long content and "Show more" button.
 * Uses Paper without border or border radius.
 *
 * @param {ChatMessageCardProps} props - Content, expand state, and styling.
 * @returns {JSX.Element} The chat message card.
 */
export default function ChatMessageCard({
  htmlContent,
  isExpanded,
  onToggleExpand,
  isCurrentUser,
  primaryBg,
}: ChatMessageCardProps): JSX.Element {
  const plainLength = htmlContent.replace(/<[^>]+>/g, "").length;
  const showExpandButton = plainLength > COLLAPSE_CHAR_THRESHOLD;

  return (
    <Paper
      elevation={0}
      sx={{
        display: "flex",
        flexDirection: "column",
        gap: 1.5,
        p: 1.5,
        maxWidth: "100%",
        bgcolor: isCurrentUser ? primaryBg : "background.paper",
        border: "none",
        borderRadius: 0,
        boxShadow: "none",
      }}
    >
      <Box
        sx={{
          fontSize: "0.75rem",
          fontFamily: "monospace",
          "& p": {
            margin: "0 0 0.5em 0",
            whiteSpace: "pre-wrap",
            wordBreak: "break-word",
          },
          "& p:last-child": { marginBottom: 0 },
          "& img": {
            display: "block",
            maxWidth: "100%",
            maxHeight: 320,
            height: "auto",
            objectFit: "contain",
            mt: 0.5,
            mb: 0.5,
          },
          "& br": { display: "block", content: '""', marginTop: "0.25em" },
          ...(!isExpanded &&
            showExpandButton && {
              display: "-webkit-box",
              WebkitLineClamp: 4,
              WebkitBoxOrient: "vertical" as const,
              overflow: "hidden",
            }),
        }}
        dangerouslySetInnerHTML={{ __html: htmlContent }}
      />
      {showExpandButton && (
        <>
          <Divider sx={{ my: 0.25 }} />
          <Button
            size="small"
            variant="text"
            onClick={onToggleExpand}
            endIcon={
              <ChevronDown
                size={14}
                style={{
                  transform: isExpanded ? "rotate(180deg)" : "none",
                  transition: "transform 0.2s",
                }}
              />
            }
            sx={{
              alignSelf: "stretch",
              justifyContent: "center",
              fontSize: "0.75rem",
              color: "text.secondary",
              "&:hover": {
                color: "text.primary",
                bgcolor: "action.hover",
              },
            }}
          >
            {isExpanded ? "Show less" : "Show more"}
          </Button>
        </>
      )}
    </Paper>
  );
}
