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

import { Avatar, Box, Chip, Paper, Skeleton, Typography, useTheme } from "@wso2/oxygen-ui";
import { Bot } from "@wso2/oxygen-ui-icons-react";
import { useCallback, useEffect, useMemo, useRef, type JSX } from "react";
import RelativeTime from "@components/RelativeTime";
import SemanticChip from "@components/SemanticChip";
import { pickAccessibleText } from "@utils/contrastText";
import { sanitizeRichTextHtml, stripLightModeInlineStyles } from "@utils/sanitizeHtml";
import { useDarkMode } from "@utils/useDarkMode";
import { markdownToHtml } from "@utils/renderMarkdown";
import { initialsOf } from "@utils/userClaims";
import { useResolvedInlineImageHtml } from "@features/csm-cases/api/useResolvedInlineImageHtml";
import {
  convertCodeTagsToHtml,
  hasDisplayableContent,
  hasSingleCodeWrapper,
  linkifyBareUrls,
  stripAllCodeBlocks,
  stripCodeWrapper,
  stripCustomerCommentAddedLabel,
} from "@features/csm-cases/utils/commentContent";
import type {
  CsmCaseComment,
  CsmCommentAuthorRole,
} from "@features/csm-cases/types/csmCases";

interface CsmCaseCommentBubbleProps {
  comment: CsmCaseComment;
  /** Opens the fullscreen image preview for an inline `<img>` in the comment body. */
  onImageClick?: (src: string) => void;
}

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

const ROLE_LABEL: Record<CsmCommentAuthorRole, string> = {
  customer: "Customer",
  wso2_engineer: "WSO2",
  system: "System",
  chatbot: "Chatbot",
};

const ROLE_COLOR: Record<
  CsmCommentAuthorRole,
  "default" | "primary" | "warning"
> = {
  customer: "default",
  wso2_engineer: "primary",
  system: "warning",
  chatbot: "default",
};

export default function CsmCaseCommentBubble({
  comment,
  onImageClick,
}: CsmCaseCommentBubbleProps): JSX.Element | null {
  const theme = useTheme();
  const isDarkMode = useDarkMode();
  const contentRef = useRef<HTMLDivElement>(null);
  const isBot = comment.authorRole === "chatbot";
  // A chatbot (Novera) message body is Markdown; render it to HTML first. Every
  // other comment body is already rich-text HTML and goes through the same
  // code-wrapper/label-stripping pipeline the customer portal uses, since bot
  // replies never carry ServiceNow's [code] wrapper tags or the "Customer
  // comment added" label.
  const preprocessed = useMemo(() => {
    if (isBot) return markdownToHtml(comment.bodyHtml);
    const raw = comment.bodyHtml ?? "";
    const isFullCodeWrap = hasSingleCodeWrapper(raw);
    const codeBlockCount = raw.match(/\[code\]/gi)?.length ?? 0;
    const afterCode = isFullCodeWrap
      ? stripCodeWrapper(raw)
      : codeBlockCount > 1
        ? stripAllCodeBlocks(raw)
        : convertCodeTagsToHtml(raw);
    return stripCustomerCommentAddedLabel(afterCode);
  }, [comment.bodyHtml, isBot]);
  const darkModeHtml = isDarkMode
    ? stripLightModeInlineStyles(preprocessed)
    : preprocessed;
  const safeHtml = useMemo(
    () => sanitizeRichTextHtml(darkModeHtml),
    [darkModeHtml],
  );
  const { resolvedHtml, isLoading: isImagesLoading } =
    useResolvedInlineImageHtml(safeHtml);
  // Applied last, on the already-resolved/sanitized HTML — no re-sanitize.
  const renderHtml = useMemo(
    () => linkifyBareUrls(resolvedHtml),
    [resolvedHtml],
  );

  const setImageA11yAttributes = useCallback((root: HTMLDivElement) => {
    root.querySelectorAll("img").forEach((image) => {
      image.setAttribute("tabindex", "0");
      image.setAttribute("role", "button");
      image.setAttribute("aria-label", "Open image preview");
    });
  }, []);

  const setAnchorAttributes = useCallback((root: HTMLDivElement) => {
    root.querySelectorAll("a").forEach((anchor) => {
      const href = anchor.getAttribute("href") ?? "";
      if (!isSafeHref(href)) return;
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
    renderHtml,
  ]);

  const isSystem = comment.authorRole === "system";

  if (!hasDisplayableContent(comment)) {
    return null;
  }

  if (isSystem) {
    return (
      <Box
        id={comment.id}
        sx={{
          display: "flex",
          alignItems: "center",
          gap: 1,
          py: 0.5,
          px: 1,
          color: "text.secondary",
          scrollMarginTop: 96,
        }}
      >
        <SemanticChip role="warning" label="System" />
        {isImagesLoading ? (
          <Skeleton variant="text" width="40%" sx={{ flex: 1 }} />
        ) : (
          <Box
            ref={contentRef}
            sx={{
              flex: 1,
              minWidth: 0,
              overflowWrap: "anywhere",
              wordBreak: "break-word",
              "& p": { m: 0 },
              "& a": { color: "primary.main" },
              ...{ "& *": { fontSize: "0.875rem" } },
            }}
            dangerouslySetInnerHTML={{ __html: renderHtml }}
          />
        )}
        <Typography variant="caption" color="text.secondary">
          <RelativeTime iso={comment.createdAt} href={`#${comment.id}`} />
        </Typography>
      </Box>
    );
  }

  const isInternal = !!comment.internal;

  // The author avatar carries the brand orange for WSO2 engineers; its initials
  // must stay legible on that fill (white-on-orange ~2.4:1 fails AA). Pick the
  // text colour by the fill's luminance, and use a dark-enough grey for
  // customers (grey.500 also fails with either text colour).
  const avatarBg =
    comment.authorRole === "wso2_engineer"
      ? theme.palette.primary.main
      : theme.palette.grey[700];
  const avatarFg = pickAccessibleText(avatarBg);

  return (
    <Box
      id={comment.id}
      sx={{ display: "flex", gap: 1.5, alignItems: "flex-start", scrollMarginTop: 96 }}
    >
      <Avatar
        sx={{
          bgcolor: avatarBg,
          color: avatarFg,
          width: 32,
          height: 32,
          fontSize: "0.85rem",
        }}
      >
        {isBot ? <Bot size={16} /> : initialsOf(comment.authorName)}
      </Avatar>
      <Paper
        variant="outlined"
        sx={{
          p: 1.5,
          flex: 1,
          minWidth: 0,
          display: "flex",
          flexDirection: "column",
          gap: 0.75,
          ...(isInternal && {
            bgcolor: "action.hover",
            borderColor: "divider",
            borderLeftWidth: "3px",
            borderLeftColor: "primary.main",
          }),
        }}
      >
        <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
          <Typography variant="subtitle2">{comment.authorName}</Typography>
          {comment.authorRole !== "wso2_engineer" && (
            <Chip
              size="small"
              label={ROLE_LABEL[comment.authorRole]}
              color={ROLE_COLOR[comment.authorRole]}
              variant="outlined"
            />
          )}
          {/* A filled chip "bubble" marks the work note; paired with the tinted
              background it reads as internal without a heavy banner. */}
          {isInternal && <SemanticChip role="default" variant="outlined" label="Internal note" />}
          <Typography variant="caption" color="text.secondary">
            <RelativeTime iso={comment.createdAt} href={`#${comment.id}`} />
          </Typography>
        </Box>
        <Box
          sx={{
            minWidth: 0,
            overflowWrap: "anywhere",
            wordBreak: "break-word",
            "& p": { m: 0 },
            "& p + p": { mt: 0.75 },
            "& ul, & ol": { ml: 3, my: 0.5 },
            "& code": {
              bgcolor: "background.default",
              px: 0.5,
              borderRadius: 0.5,
              fontFamily: "monospace",
              fontSize: "0.85em",
              overflowWrap: "anywhere",
            },
            "& pre": {
              bgcolor: "background.default",
              p: 1,
              borderRadius: 1,
              overflowX: "auto",
              fontFamily: "monospace",
              fontSize: "0.85em",
            },
            "& a": { color: "primary.main" },
            "& img": { maxWidth: "100%", cursor: onImageClick ? "pointer" : "default" },
            "& br": { display: "block", content: '""', mt: 0.5 },
            "& blockquote": {
              borderLeft: 3,
              borderColor: "divider",
              pl: 1.5,
              ml: 0,
              my: 0.75,
              color: "text.secondary",
              fontStyle: "italic",
            },
            "& h1, & h2, & h3": { mt: 1, mb: 0.5 },
            // Novera answers arrive as Markdown tables; the markdown-it renderer
            // wraps each in `.md-table-wrap`, which scrolls horizontally while
            // the table keeps its native display (preserving a11y table roles).
            "& .md-table-wrap": { overflowX: "auto", maxWidth: "100%", my: 0.75 },
            // Raw (non-markdown) `<table>` elements from backend HTML aren't
            // wrapped in `.md-table-wrap`, so give the table itself the same
            // horizontal-scroll behavior directly.
            "& table": {
              display: "block",
              overflowX: "auto",
              maxWidth: "100%",
              width: "max-content",
              minWidth: "100%",
              borderCollapse: "collapse",
            },
            "& th, & td": {
              border: 1,
              borderColor: "divider",
              px: 1,
              py: 0.5,
              textAlign: "left",
            },
            "& th": { bgcolor: "action.hover", fontWeight: 600 },
          }}
        >
          {isImagesLoading ? (
            <Skeleton variant="rounded" width="100%" height={120} />
          ) : (
            <Box ref={contentRef} dangerouslySetInnerHTML={{ __html: renderHtml }} />
          )}
        </Box>
      </Paper>
    </Box>
  );
}
