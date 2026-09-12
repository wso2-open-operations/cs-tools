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

import { Box, Typography } from "@wso2/oxygen-ui";
import { type JSX, useMemo } from "react";
import { markdownToHtmlProse } from "@utils/renderMarkdown";
import { sanitizeRichTextHtml } from "@utils/sanitizeHtml";
import { HELP_TOPIC_CONTENT } from "@features/help/utils/helpContent";
import { resolveHelpImages } from "@features/help/utils/helpImages";

export interface HelpTopicSectionProps {
  /** The topic's bare id (e.g. `"operations"`) — the key `HELP_TOPIC_CONTENT`
   * and the anchor target on `HelpPage` are both keyed by. */
  topicId: string;
}

/**
 * Renders one Help topic's static Markdown as sanitized HTML. `HelpPage`
 * renders one of these per topic declared in `csmNavItems.ts`'s `help` node,
 * each wrapped in its own `<section>` that the page's table-of-contents
 * anchors link to — this component only resolves `topicId` to its Markdown
 * source and renders it; it has no topic list of its own to keep in sync,
 * and (unlike its `HelpTopicPage` route-era predecessor) no route param to
 * read: the id is a plain prop supplied by the caller.
 *
 * Pipeline mirrors `CsmCaseCommentBubble.tsx`'s Markdown rendering, but uses
 * `markdownToHtmlProse` rather than `markdownToHtml`: Help content is
 * long-form prose hand-wrapped at a fixed column width in its Markdown
 * source, and `markdownToHtml`'s `breaks: true` (needed for chat messages)
 * would render every one of those source line-wraps as a forced `<br>`.
 * `markdownToHtmlProse` (escapes any raw HTML in the source) -> `resolveHelpImages`
 * (rewrites `images/<name>` refs to their real bundled asset URL) ->
 * `sanitizeRichTextHtml` (defence in depth before `dangerouslySetInnerHTML`,
 * required for every such call in this app regardless of the content's
 * origin) -> render.
 */
export default function HelpTopicSection({ topicId }: HelpTopicSectionProps): JSX.Element {
  const source = HELP_TOPIC_CONTENT[topicId];

  const html = useMemo(() => {
    if (source === undefined) return undefined;
    return sanitizeRichTextHtml(resolveHelpImages(markdownToHtmlProse(source)));
  }, [source]);

  if (html === undefined) {
    return (
      <Typography color="text.secondary">
        This help topic couldn&apos;t be found.
      </Typography>
    );
  }

  return (
    <Box
      sx={{
        minWidth: 0,
        maxWidth: "100%",
        overflowWrap: "break-word",
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
          maxWidth: "100%",
          fontFamily: "monospace",
          fontSize: "0.85em",
        },
        "& a": { color: "primary.main" },
        "& img": { maxWidth: "100%" },
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
        "& .md-table-wrap": { overflowX: "auto", maxWidth: "100%", my: 0.75 },
        "& table": {
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
      dangerouslySetInnerHTML={{ __html: html }}
    />
  );
}
