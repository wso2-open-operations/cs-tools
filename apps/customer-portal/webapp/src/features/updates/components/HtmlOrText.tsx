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

import { Box } from "@wso2/oxygen-ui";
import DOMPurify from "dompurify";
import type { JSX } from "react";
import { DESCRIPTION_PURIFY_CONFIG, stripLightModeInlineStyles } from "@utils/common";

// Matches actual HTML formatting tags — not XML/config tag-like strings e.g. <Product_Home>.
export const HTML_FORMAT_RE =
  /<\/?(p|span|div|ul|ol|li|strong|em|b|i|br|h[1-6]|a[\s>]|table|tr|td|th|code|pre|blockquote)\b/i;

export function decodeEntities(s: string): string {
  return s
    .replace(/&amp;/g, "&")
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .replace(/&quot;/g, '"')
    .replace(/&#39;/g, "'")
    .replace(/&nbsp;/g, " ");
}

export const HTML_CONTENT_SX = {
  fontSize: "0.875rem",
  lineHeight: 1.7,
  color: "text.secondary",
  "& p": { margin: "0 0 0.4em 0" },
  "& p:last-child": { marginBottom: 0 },
  "& a": { color: "primary.main", textDecoration: "underline" },
  "& ul, & ol": { mt: 0, mb: 0.5, pl: 2.5 },
  "& li": { mb: 0.25 },
  "& strong, & b": { fontWeight: 600, color: "text.primary" },
};

/**
 * Renders a string that may contain real HTML markup or plain text with HTML
 * entities (e.g. &lt;Product_Home&gt;). Real HTML formatting tags are sanitized
 * via DOMPurify with optional dark-mode inline-style stripping. Plain text has
 * entities decoded and newlines converted to <br> so angle-bracket content
 * renders correctly.
 */
export function HtmlOrText({
  content,
  isDark,
}: {
  content: string;
  isDark: boolean;
}): JSX.Element {
  if (HTML_FORMAT_RE.test(content)) {
    const stripped = isDark ? stripLightModeInlineStyles(content) : content;
    return (
      <Box
        dangerouslySetInnerHTML={{ __html: DOMPurify.sanitize(stripped, DESCRIPTION_PURIFY_CONFIG) }}
        sx={HTML_CONTENT_SX}
      />
    );
  }

  const decoded = decodeEntities(content);
  const safeHtml = decoded
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/\n/g, "<br>");
  return <Box dangerouslySetInnerHTML={{ __html: safeHtml }} sx={HTML_CONTENT_SX} />;
}
