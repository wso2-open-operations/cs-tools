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

/** How a change request's rich-text plan fields read on its Plan tab. */
export const PLAN_RICH_TEXT_SX = {
  color: "text.secondary",
  fontSize: "0.875rem",
  lineHeight: 1.5,
  wordBreak: "break-word",
  // Newly generated comments no longer carry a per-run `white-space: pre-wrap`
  // inline style -- declared once here instead. Older comments carry their own
  // inline style and are unaffected either way.
  whiteSpace: "pre-wrap",
  "& p": { my: 0.5 },
  "& p:first-of-type": { mt: 0 },
  "& p:last-child": { mb: 0 },
  "& ul, & ol": { my: 0.5, pl: 3 },
  "& a": { color: "primary.main" },
  "& img": { maxWidth: "100%", height: "auto" },
  "& table": { borderCollapse: "collapse", width: "100%" },
  "& th, & td": { border: 1, borderColor: "divider", px: 1, py: 0.5, textAlign: "left" },
} as const;
