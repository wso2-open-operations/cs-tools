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
import { useMemo, type JSX } from "react";
import { sanitizeRichTextHtml } from "@utils/sanitizeHtml";
import { looksLikeHtml } from "../utils/allocations";

/**
 * An update's body. Plain text keeps its line breaks and is rendered as text.
 * Content that carries markup (updates written upstream can be HTML) goes
 * through the portal's HTML sanitizer first; raw content is never injected.
 */
export default function UpdateContent({ content }: { content: string }): JSX.Element {
  const html = useMemo(
    () => (looksLikeHtml(content) ? sanitizeRichTextHtml(content) : null),
    [content],
  );
  if (html !== null) {
    return (
      <Box
        data-testid="update-content-html"
        sx={{ typography: "body2", overflowWrap: "anywhere", "& img": { maxWidth: "100%" } }}
        // Sanitized above with the same policy as case comments.
        dangerouslySetInnerHTML={{ __html: html }}
      />
    );
  }
  return (
    <Box sx={{ typography: "body2", whiteSpace: "pre-wrap", overflowWrap: "anywhere" }}>
      {content}
    </Box>
  );
}
