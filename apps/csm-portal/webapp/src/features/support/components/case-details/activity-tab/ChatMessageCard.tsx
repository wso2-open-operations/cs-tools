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
import { useCallback, useEffect, useMemo, useRef } from "react";
import type { JSX } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { linkifyBareUrls } from "@features/support/utils/support";

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

const markdownComponents: React.ComponentProps<
  typeof ReactMarkdown
>["components"] = {
  a: ({ href, children }) =>
    isSafeHref(href) ? (
      <Box
        component="a"
        href={href}
        target="_blank"
        rel="noopener noreferrer"
        sx={{ color: "primary.main", textDecoration: "underline" }}
      >
        {children}
      </Box>
    ) : (
      <Box component="span">{children}</Box>
    ),
  table: ({ children }) => (
    <Box sx={{ width: "100%", overflowX: "auto", mb: 1 }}>
      <Box
        component="table"
        sx={{
          width: "100%",
          borderCollapse: "collapse",
          minWidth: 420,
        }}
      >
        {children}
      </Box>
    </Box>
  ),
  thead: ({ children }) => <Box component="thead">{children}</Box>,
  tbody: ({ children }) => <Box component="tbody">{children}</Box>,
  tr: ({ children }) => (
    <Box component="tr" sx={{ borderBottom: "1px solid", borderColor: "divider" }}>
      {children}
    </Box>
  ),
  th: ({ children }) => (
    <Box
      component="th"
      sx={{
        textAlign: "left",
        p: 1,
        fontSize: "0.75rem",
        fontWeight: 600,
        color: "text.secondary",
      }}
    >
      {children}
    </Box>
  ),
  td: ({ children }) => (
    <Box
      component="td"
      sx={{
        p: 1,
        fontSize: "0.8125rem",
        verticalAlign: "top",
      }}
    >
      {children}
    </Box>
  ),
  img: ({ src = "", alt = "" }) => (
    <Box
      component="img"
      src={src}
      alt={alt}
      tabIndex={0}
      role="button"
      aria-label="Open image preview"
    />
  ),
};

/**
 * Card-style chat message using Paper without border or border radius.
 * Renders HTML message content with basic styling.
 *
 * @param {ChatMessageCardProps} props - Content and styling props.
 * @returns {JSX.Element} The chat message card.
 */
export default function ChatMessageCard({
  htmlContent,
  markdownContent,
  renderAsMarkdown = false,
  isCurrentUser,
  primaryBg,
  onImageClick,
}: ChatMessageCardProps): JSX.Element {
  const contentRef = useRef<HTMLDivElement>(null);
  const linkedHtml = useMemo(() => linkifyBareUrls(htmlContent), [htmlContent]);

  const setImageA11yAttributes = useCallback((root: HTMLDivElement) => {
    const images = root.querySelectorAll("img");
    images.forEach((image) => {
      image.setAttribute("tabindex", "0");
      image.setAttribute("role", "button");
      image.setAttribute("aria-label", "Open image preview");
    });
  }, []);

  const setAnchorAttributes = useCallback((root: HTMLDivElement) => {
    const anchors = root.querySelectorAll("a");
    anchors.forEach((anchor) => {
      const href = anchor.getAttribute("href") ?? "";
      if (!isSafeHref(href)) {
        return;
      }
      anchor.setAttribute("target", "_blank");
      anchor.setAttribute("rel", "noopener noreferrer");
    });
  }, []);

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

  const handleKeyDown = useCallback(
    (e: KeyboardEvent) => {
      const target = e.target as HTMLElement;
      if (
        target instanceof HTMLImageElement &&
        (e.key === "Enter" || e.key === " ")
      ) {
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
    setImageA11yAttributes(el);
    setAnchorAttributes(el);
    el.addEventListener("click", handleClick);
    el.addEventListener("keydown", handleKeyDown);
    return () => {
      el.removeEventListener("click", handleClick);
      el.removeEventListener("keydown", handleKeyDown);
    };
  }, [
    handleClick,
    handleKeyDown,
    setImageA11yAttributes,
    setAnchorAttributes,
    htmlContent,
    markdownContent,
    renderAsMarkdown,
  ]);

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
        overflow: "hidden",
      }}
    >
      <Box
        sx={{
          fontSize: "0.875rem",
          lineHeight: 1.6,
          overflowX: "auto",
          display: "grid",
          gridTemplateColumns: "minmax(0, 1fr)",
          width: "100%",
          minWidth: 0,
          "& h1": {
            fontSize: "1.125rem",
            fontWeight: 600,
            mt: 2,
            mb: 1,
          },
          "& h2": {
            fontSize: "1rem",
            fontWeight: 600,
            mt: 2,
            mb: 1,
          },
          "& h3": {
            fontSize: "0.875rem",
            fontWeight: 600,
            mt: 1.5,
            mb: 0.5,
          },
          "& ul, & ol": {
            mt: 0,
            mb: 1,
            pl: 2.5,
          },
          "& li": {
            mb: 0.5,
          },
          "& p": {
            margin: "0 0 0.25em 0",
            whiteSpace: "pre-wrap",
            overflowWrap: "anywhere",
          },
          "& p:last-child": { marginBottom: 0 },
          "& a": {
            color: "primary.main",
            textDecoration: "underline",
          },
          "& table": {
            width: "100%",
            minWidth: 420,
            borderCollapse: "collapse",
            mb: 1,
          },
          "& tr": {
            borderBottom: "1px solid",
            borderColor: "divider",
          },
          "& th": {
            textAlign: "left",
            p: 1,
            fontSize: "0.75rem",
            fontWeight: 600,
            color: "text.secondary",
          },
          "& td": {
            p: 1,
            fontSize: "0.8125rem",
            verticalAlign: "top",
          },
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
            px: 1,
            py: 0.25,
            whiteSpace: "pre",
          },
          "& pre": {
            overflowX: "auto",
            whiteSpace: "pre",
            backgroundColor: "action.disabledBackground",
            m: 0,
            p: 1,
            minWidth: 0,
            maxWidth: "100%",
          },
          "& pre code": {
            backgroundColor: "transparent",
          },
        }}
        ref={contentRef}
      >
        {renderAsMarkdown ? (
          <Box sx={{ minWidth: 0 }}>
            <ReactMarkdown components={markdownComponents} remarkPlugins={[remarkGfm]}>
              {markdownContent ?? ""}
            </ReactMarkdown>
          </Box>
        ) : (
          <Box
            dangerouslySetInnerHTML={{ __html: linkedHtml }}
            sx={{ minWidth: 0 }}
          />
        )}
      </Box>
    </Paper>
  );
}
