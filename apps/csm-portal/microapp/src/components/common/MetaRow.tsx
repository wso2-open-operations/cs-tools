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

import { Stack, Typography, type TypographyProps } from "@wso2/oxygen-ui";
import type { ReactNode } from "react";

/** One "label / value" row in an overview/detail card (Accounts, Projects, Deployments,
 * Vulnerabilities, ...) — the mobile equivalent of the webapp's grid-cell MetaCell (a single
 * narrow column reads better as stacked rows than as a multi-column grid). */
export function MetaRow({ label, children }: { label: string; children: ReactNode }) {
  return (
    <Stack direction="row" justifyContent="space-between" alignItems="flex-start" gap={2}>
      <Typography variant="body2" color="text.secondary" sx={{ flexShrink: 0 }}>
        {label}
      </Typography>
      <Stack alignItems="flex-end" sx={{ minWidth: 0 }}>
        {children}
      </Stack>
    </Stack>
  );
}

export function MetaValue({ mono, children, ...rest }: TypographyProps & { mono?: boolean }) {
  return (
    <Typography
      variant="body2"
      align="right"
      sx={{ wordBreak: "break-word", ...(mono && { fontFamily: "monospace" }) }}
      {...rest}
    >
      {children}
    </Typography>
  );
}
