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

import { Box, Button, Stack, Typography } from "@wso2/oxygen-ui";
import type { JSX } from "react";

interface ProfileLoadErrorStateProps {
  /** Re-issues the failed profile request. */
  onRetry: () => void;
}

/**
 * Shown in place of the app content when the signed-in user's profile could
 * not be loaded for a reason other than "not entitled" (a server or network
 * failure). Deliberately dependency-light: it renders inside the auth shell,
 * before any backend client is usable.
 */
export default function ProfileLoadErrorState({
  onRetry,
}: ProfileLoadErrorStateProps): JSX.Element {
  return (
    <Box
      role="alert"
      sx={{ display: "flex", flex: 1, justifyContent: "center", alignItems: "center", px: 3, py: 6 }}
    >
      <Stack spacing={1.5} alignItems="center" sx={{ maxWidth: 420 }}>
        <Typography variant="subtitle1" fontWeight={700}>
          Something went wrong
        </Typography>
        <Typography variant="body2" color="text.secondary" textAlign="center">
          We couldn&apos;t load your profile. Check your connection and try again.
        </Typography>
        <Button size="small" variant="outlined" onClick={onRetry}>
          Try again
        </Button>
      </Stack>
    </Box>
  );
}
