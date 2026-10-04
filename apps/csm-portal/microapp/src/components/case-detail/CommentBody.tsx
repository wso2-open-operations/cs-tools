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

import { useState } from "react";
import { Box, Button, Skeleton, Typography } from "@wso2/oxygen-ui";
import { useResolvedInlineImageHtml } from "@utils/useResolvedInlineImageHtml";

// Some comments (state-change audit entries, etc.) carry real HTML (`<br><p>...</p>`) rather than
// plain text — rendering those as plain text shows the literal tags. Same sniff-then-sanitize
// pattern as the webapp's UpdatesPage (apps/csm-portal/webapp), which hits the same ambiguity.
const HTML_FORMAT_RE =
  /<\/?(p|span|div|ul|ol|li|strong|em|b|i|br|h[1-6]|a[\s>]|table|tr|td|th|code|pre|blockquote|img)\b/i;

const HTML_CONTENT_SX = {
  fontSize: "0.875rem",
  "& p": { margin: "0 0 0.4em 0" },
  "& p:last-child": { marginBottom: 0 },
  "& a": { color: "primary.main", textDecoration: "underline" },
  "& ul, & ol": { mt: 0, mb: 0.5, pl: 2.5 },
  // Comment HTML can embed images at their original (often desktop-sized)
  // dimensions — without a width cap they overflow the microapp's narrow
  // viewport instead of scaling down to fit it.
  "& img": { maxWidth: "100%", height: "auto", display: "block" },
} as const;

// System-generated entries (escalation-flow summaries, etc.) can carry a raw JSON dump thousands
// of characters long, which blows the comment card out to several screens' height. Collapse
// anything past this length behind a toggle instead.
const TRUNCATE_AT = 500;

export function CommentBody({ content }: { content: string }) {
  const [expanded, setExpanded] = useState(false);
  const isLong = content.length > TRUNCATE_AT;
  // Detected against the full content, not the truncated slice — otherwise HTML that only
  // appears past the truncation point would render as plain text while collapsed and switch to
  // HTML once expanded.
  const isHtml = HTML_FORMAT_RE.test(content);
  const truncated = isLong && !expanded;
  const shown = truncated ? content.slice(0, TRUNCATE_AT) : content;

  // Resolve `.iix` inline-image references (comment/description HTML embeds images as auth-gated
  // refs the WebView can't fetch directly). The hook sanitises its output as the final step, so
  // the sliced content is passed in raw and the ellipsis is appended outside the HTML.
  const { resolvedHtml, isLoading: imagesLoading } = useResolvedInlineImageHtml(isHtml ? shown : "");

  return (
    <Box sx={{ overflowWrap: "anywhere" }}>
      {isHtml ? (
        <>
          {imagesLoading ? (
            <Skeleton variant="rounded" height={80} />
          ) : (
            <Box
              dangerouslySetInnerHTML={{ __html: resolvedHtml }}
              sx={{ ...HTML_CONTENT_SX, color: "text.primary" }}
            />
          )}
          {truncated && (
            <Typography variant="body2" color="text.primary">
              …
            </Typography>
          )}
        </>
      ) : (
        <Typography variant="body2" color="text.primary" sx={{ whiteSpace: "pre-wrap" }}>
          {shown}
          {truncated && "…"}
        </Typography>
      )}

      {isLong && (
        <Button size="small" onClick={() => setExpanded((v) => !v)} sx={{ minWidth: 0, px: 0.5 }}>
          {expanded ? "Show less" : "Show more"}
        </Button>
      )}
    </Box>
  );
}
