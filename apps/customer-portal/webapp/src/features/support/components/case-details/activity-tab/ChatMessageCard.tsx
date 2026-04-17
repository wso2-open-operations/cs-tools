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

import type { ChatMessageCardProps } from "@features/support/types/supportComponents";
import { Box, Paper } from "@wso2/oxygen-ui";
import { useCallback, useEffect, useRef } from "react";
import type { JSX } from "react";

/**
 * Card-style chat message using Paper without border or border radius.
 * Renders HTML message content with basic styling.
 *
 * @param {ChatMessageCardProps} props - Content and styling props.
 * @returns {JSX.Element} The chat message card.
 */
export default function ChatMessageCard({
  htmlContent,
  isCurrentUser,
  primaryBg,
  onImageClick,
}: ChatMessageCardProps): JSX.Element {
  const contentRef = useRef<HTMLDivElement>(null);

  const handleClick = useCallback(
    (e: MouseEvent) => {
      const target = e.target as HTMLElement;
      if (target.tagName === "IMG" && target instanceof HTMLImageElement) {
        const src = target.src || target.getAttribute("src");
        if (src && onImageClick) {
          e.preventDefault();
          onImageClick(src);
        }
      }
    },
    [onImageClick],
  );

  useEffect(() => {
    const el = contentRef.current;
    if (!el) return;
    el.addEventListener("click", handleClick);
    return () => {
      el.removeEventListener("click", handleClick);
    };
  }, [handleClick]);

  return (
    <Paper
      elevation={0}
      sx={{
        display: "flex",
        flexDirection: "column",
        gap: 1,
        p: 1.25,
        width: "100%",
        maxWidth: "100%",
        minHeight: "auto",
        bgcolor: isCurrentUser ? primaryBg : "background.paper",
        minWidth: 0,
      }}
    >
      <Box
        sx={{
          fontSize: "0.875rem",
          lineHeight: 1.5,
          overflowX: "auto",
          maxWidth: "100%",
          "& p": {
            margin: "0 0 0.25em 0",
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
          "& code": {
            fontFamily: "monospace",
            fontSize: "inherit",
            backgroundColor: "action.hover",
            px: 0.5,
            py: 0.25,
            whiteSpace: "pre",
            wordBreak: "normal",
          },
          "& pre": {
            overflowX: "auto",
            maxWidth: "100%",
            whiteSpace: "pre",
          },
        }}
        ref={contentRef}
        dangerouslySetInnerHTML={{ __html: htmlContent }}
      />
    </Paper>
  );
}
