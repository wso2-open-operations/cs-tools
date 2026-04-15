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
import { type JSX } from "react";
import EmptyIcon from "@components/empty-state/EmptyIcon";

export interface CallsEmptyStateProps {
  action?: React.ReactNode;
}

/**
 * Renders the empty state for the calls panel.
 *
 * @param {CallsEmptyStateProps} props - Optional action to render below the message.
 * @returns {JSX.Element} The rendered empty state.
 */
export default function CallsEmptyState({
  action,
}: CallsEmptyStateProps): JSX.Element {
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
      <Typography variant="body2" color="text.secondary">
        No call requests found for this case.
      </Typography>
      {action && (
        <Box sx={{ display: "flex", justifyContent: "center", mt: 1 }}>
          {action}
        </Box>
      )}
    </Stack>
  );
}
