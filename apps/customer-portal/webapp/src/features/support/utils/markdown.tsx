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
import type ReactMarkdown from "react-markdown";

/** Safe URL protocols for markdown links. */
export const SAFE_PROTOCOLS = ["http:", "https:"];

/**
 * Validates markdown links against the allowed protocols.
 *
 * @param href - Link value from markdown anchor.
 * @returns True when URL is safe to render as an anchor.
 */
export function isSafeHref(href: string | undefined): href is string {
  if (!href || typeof href !== "string") return false;
  try {
    const parsed = new URL(href);
    return SAFE_PROTOCOLS.includes(parsed.protocol);
  } catch {
    return false;
  }
}

/**
 * Builds shared markdown component renderers for bot messages.
 *
 * @returns ReactMarkdown components map.
 */
export function buildBotMarkdownComponents(): React.ComponentProps<
  typeof ReactMarkdown
>["components"] {
  return {
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
  };
}
