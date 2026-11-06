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
import type { JSX, ReactNode } from "react";

interface ArticleListProps {
  label: string;
  /** Marks the list busy while its contents are being replaced. */
  busy?: boolean;
  children: ReactNode;
}

/** Semantic list wrapper: a `<ul>` of `<li>` rows with the bullets and padding reset. */
export default function ArticleList({ label, busy, children }: ArticleListProps): JSX.Element {
  return (
    <Box
      component="ul"
      aria-label={label}
      aria-busy={busy || undefined}
      sx={{
        listStyle: "none",
        m: 0,
        p: 0,
        display: "flex",
        flexDirection: "column",
        gap: 1.5,
        opacity: busy ? 0.6 : 1,
        transition: "opacity 120ms",
      }}
    >
      {children}
    </Box>
  );
}

export function ArticleListItem({ children }: { children: ReactNode }): JSX.Element {
  return <Box component="li">{children}</Box>;
}
