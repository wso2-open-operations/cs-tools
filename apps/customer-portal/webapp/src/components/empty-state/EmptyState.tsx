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
import { type JSX, type ReactNode } from "react";
import EmptyIcon from "@components/empty-state/EmptyIcon";

export interface EmptyStateProps {
  description: string;
  /** Optional second line, rendered below the description in a more muted tone. */
  secondaryDescription?: string;
  /** Optional call-to-action (e.g. a Button) rendered below the description. */
  action?: ReactNode;
}

/**
 * Generic EmptyState component for displaying an icon and a message.
 *
 * @param {EmptyStateProps} props - Component props.
 * @returns {JSX.Element} The rendered empty state.
 */
export default function EmptyState({
  description,
  secondaryDescription,
  action,
}: EmptyStateProps): JSX.Element {
  return (
    <Stack
      spacing={2}
      alignItems="center"
      justifyContent="center"
      sx={{ py: 4 }}
    >
      <Box
        sx={{
          width: 160,
          maxWidth: "100%",
          "& svg": { width: "100%", height: "auto" },
        }}
        aria-hidden
      >
        <EmptyIcon />
      </Box>
      <Stack spacing={0.5} alignItems="center">
        <Typography variant="body2" color="text.secondary" textAlign="center">
          {description}
        </Typography>
        {secondaryDescription && (
          <Typography variant="caption" color="text.disabled" textAlign="center">
            {secondaryDescription}
          </Typography>
        )}
      </Stack>
      {action && (
        <Box sx={{ display: "flex", justifyContent: "center", mt: 1 }}>
          {action}
        </Box>
      )}
    </Stack>
  );
}
