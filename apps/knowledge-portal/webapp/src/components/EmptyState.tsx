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

import { Box, Stack, Typography } from "@wso2/oxygen-ui";
import { SearchX, type LucideIcon } from "@wso2/oxygen-ui-icons-react";
import type { JSX, ReactNode } from "react";

interface EmptyStateProps {
  title: string;
  message?: string;
  /** Short suggestions for what to try next. */
  tips?: readonly string[];
  /** A button or link rendered under the text. */
  action?: ReactNode;
  icon?: LucideIcon;
}

/** Nothing-to-show state for a list that loaded fine but is empty. */
export default function EmptyState({
  title,
  message,
  tips,
  action,
  icon: Icon = SearchX,
}: EmptyStateProps): JSX.Element {
  return (
    <Stack spacing={1} alignItems="center" sx={{ py: 6, px: 2, textAlign: "center" }}>
      <Box
        sx={{
          width: 56,
          height: 56,
          borderRadius: "50%",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          bgcolor: "action.hover",
          color: "text.secondary",
          mb: 0.5,
        }}
      >
        <Icon size={26} strokeWidth={1.75} aria-hidden />
      </Box>
      <Typography variant="subtitle1" component="p" fontWeight={600}>
        {title}
      </Typography>
      {message && (
        <Typography variant="body2" color="text.secondary" sx={{ maxWidth: 440 }}>
          {message}
        </Typography>
      )}
      {tips && tips.length > 0 && (
        <Box
          component="ul"
          sx={{
            m: 0,
            mt: 1,
            pl: 2.5,
            textAlign: "left",
            color: "text.secondary",
            typography: "body2",
            "& li + li": { mt: 0.5 },
          }}
        >
          {tips.map((tip) => (
            <li key={tip}>{tip}</li>
          ))}
        </Box>
      )}
      {action && <Box sx={{ pt: 1.5 }}>{action}</Box>}
    </Stack>
  );
}
