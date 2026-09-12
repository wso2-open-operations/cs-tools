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
import UserRefLink from "@components/UserRefLink";
import { pickAccessibleText } from "@utils/contrastText";
import { sanitizeRichTextHtml, stripLightModeInlineStyles } from "@utils/sanitizeHtml";
import { useDarkMode } from "@utils/useDarkMode";
import { markdownToHtml } from "@utils/renderMarkdown";
import { initialsOf } from "@utils/userClaims";
import { useResolvedInlineImageHtml } from "@features/csm-cases/api/useResolvedInlineImageHtml";
import { replaceCallRequestLinks } from "@features/csm-cases/utils/callRequestLinks";
import { replaceSnLinks, type SnLinkType } from "@features/csm-cases/utils/snLinkRegistry";
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
  onImageClick?: (src: string, alt?: string) => void;
  /** Opens the call-request detail popup for a call-request link embedded in the comment body. */
  onCallRequestClick?: (sysId: string) => void;
  /** Opens the alert/smart-alert detail popup for an alert-reference marker embedded in the comment body. */
  onSnLinkClick?: (type: SnLinkType, id: string) => void;
  /** Drops the author avatar and prefixes the name with "Commented by "
   * instead — the avatar column eats a disproportionate share of a narrow
   * container's width (e.g. `CasePreviewContent`'s ~420px drawer); the full
   * Activities tab and the chat transcript dialog have room for it, so this
   * defaults to off rather than changing either of those. */
  compact?: boolean;
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
  chatbot: "AI Agent",
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
  onCallRequestClick,
  onSnLinkClick,
  compact = false,
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
  // All three run last, on the already-resolved/sanitized HTML — no
  // re-sanitize. `replaceCallRequestLinks`/`replaceSnLinks` must both run
  // before `linkifyBareUrls`: they swap their respective bare backing-store
  // URLs for our own `<span data-…>` markers, and `linkifyBareUrls` would
  // otherwise linkify those same bare URLs into plain external `<a>`s first.
  // The order between the two marker passes doesn't matter — they match
  // disjoint URL patterns. Their output is generated entirely by us from a
  // regex-validated hex sysid (never raw comment text passed through
  // unescaped), so running after sanitization is safe.
  const renderHtml = useMemo(
    () => linkifyBareUrls(replaceSnLinks(replaceCallRequestLinks(resolvedHtml))),
    [resolvedHtml],
  );

  const setImageA11yAttributes = useCallback(
    (root: HTMLDivElement) => {
      if (!onImageClick) return;
      root.querySelectorAll("img").forEach((image) => {
        image.setAttribute("tabindex", "0");
        image.setAttribute("role", "button");
        image.setAttribute("aria-label", "Open image preview");
      });
    },
    [onImageClick],
  );

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
          onImageClick(src, target.alt || undefined);
        }
        return;
      }
      const callRequestMarker = target.closest?.("[data-call-request-sysid]");
      if (callRequestMarker && onCallRequestClick) {
        const sysId = callRequestMarker.getAttribute("data-call-request-sysid");
        if (sysId) {
          e.preventDefault();
          onCallRequestClick(sysId);
        }
        return;
      }
      const snLinkMarker = target.closest?.("[data-sn-link-type]");
      if (snLinkMarker && onSnLinkClick) {
        const type = snLinkMarker.getAttribute("data-sn-link-type");
        const id = snLinkMarker.getAttribute("data-sn-link-id");
        if ((type === "alert" || type === "smartAlert") && id) {
          e.preventDefault();
          onSnLinkClick(type, id);
        }
      }
    },
    [onImageClick, onCallRequestClick, onSnLinkClick],
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
          onImageClick(src, target.alt || undefined);
        }
        return;
      }
      if (e.key === "Enter" || e.key === " ") {
        const callRequestMarker = target.closest?.("[data-call-request-sysid]");
        if (callRequestMarker && onCallRequestClick) {
          const sysId = callRequestMarker.getAttribute("data-call-request-sysid");
          if (sysId) {
            e.preventDefault();
            onCallRequestClick(sysId);
            return;
          }
        }
        const snLinkMarker = target.closest?.("[data-sn-link-type]");
        if (snLinkMarker && onSnLinkClick) {
          const type = snLinkMarker.getAttribute("data-sn-link-type");
          const id = snLinkMarker.getAttribute("data-sn-link-id");
          if ((type === "alert" || type === "smartAlert") && id) {
            e.preventDefault();
            onSnLinkClick(type, id);
          }
        }
      }
    },
    [onImageClick, onCallRequestClick, onSnLinkClick],
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
              // A system entry is backend HTML too, so it needs the same width
              // containment as a regular comment body — see the full note on the
              // rich-text host below for why `min-width: 0` alone doesn't hold.
              maxWidth: "100%",
              contain: "inline-size",
              overflowX: "auto",
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
      {!compact && (
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
      )}
      <Paper
        variant="outlined"
        sx={{
          p: 1.5,
          flex: 1,
          minWidth: 0,
          display: "flex",
          flexDirection: "column",
          gap: 0.75,
          // The default outlined-Paper divider is near-invisible against the
          // page background in some theme presets; an elevated surface +
          // stronger border keeps entries visually distinct in both themes.
          bgcolor: "background.paper",
          borderColor: "action.disabled",
          ...(isInternal && {
            bgcolor: "action.hover",
            borderColor: "divider",
            borderLeftWidth: "3px",
            borderLeftColor: "primary.main",
          }),
        }}
      >
        <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
          <Typography variant="subtitle2" sx={{ fontWeight: 600 }}>
            {compact && "Commented by "}
            <UserRefLink
              name={comment.authorName}
              email={comment.authorUser?.email || comment.authorEmail}
              userId={comment.authorUser?.id}
            />
          </Typography>
          {!comment.synthetic && comment.authorRole !== "wso2_engineer" && (
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
            maxWidth: "100%",
            // Backend HTML can put an explicit pixel width on *any* element — a
            // Word/Excel paste arrives as `<div style="width:2400px">`, and a
            // `<pre>`/`<p>` can carry one just as easily — so the per-tag rules
            // below can never cover every case.
            //
            // `contain: inline-size` is what actually stops it: it makes this
            // box's own width independent of its contents, so an over-wide child
            // can no longer inflate the *intrinsic* min-content width that
            // otherwise propagates up the whole chain (bubble → feed → tab →
            // page root → AppShell.Main) and drags the page off-screen, cutting
            // off the header actions, the Overview grid's last column, and the
            // timeline toolbar. `min-width: 0` / `overflow` alone do NOT do this:
            // they zero a *flex item's* automatic minimum size, not the
            // min-content contribution travelling up through block ancestors —
            // verified empirically against this exact layout chain, where the
            // page still blew out to 3080px with overflow set but no containment.
            //
            // `overflowX` then makes that over-wide content reachable by
            // scrolling inside the comment, rather than being clipped away.
            contain: "inline-size",
            overflowX: "auto",
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
            // `maxWidth` matters as much as `overflowX` here: a `<pre>` carrying
            // an explicit `width` would otherwise just *be* that wide, and
            // `overflow-x` would have nothing to scroll.
            "& pre": {
              bgcolor: "background.default",
              p: 1,
              borderRadius: 1,
              overflowX: "auto",
              maxWidth: "100%",
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
