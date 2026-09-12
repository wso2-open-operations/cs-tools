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

import { type JSX } from "react";
import { Box, Stack, Typography, alpha, useTheme } from "@wso2/oxygen-ui";
import { Ban } from "@wso2/oxygen-ui-icons-react";
import { useDarkMode } from "@utils/useDarkMode";

interface NoPortalAccessPageProps {
  message?: string;
}

/**
 * Shown in place of the whole app shell when `GET /users/me` comes back
 * 401/403 and stays that way even after `useAuthApiClient`'s own
 * recover-or-redirect-to-sign-in chain has had its chance — see
 * `AuthGuard.tsx`'s `AuthorizedAppShell`. Deliberately not framed as an
 * "error" (no illustration, no logout link): the copy is a plain,
 * non-technical status message.
 */
export default function NoPortalAccessPage({
  message,
}: NoPortalAccessPageProps): JSX.Element {
  const theme = useTheme();
  const isDarkMode = useDarkMode();

  return (
    <Box
      sx={{
        display: "flex",
        flex: 1,
        justifyContent: "center",
        alignItems: "center",
        px: 3,
      }}
    >
      <Stack
        spacing={2.5}
        alignItems="center"
        sx={{ maxWidth: 440, textAlign: "center" }}
      >
        <Box
          sx={{
            width: 105,
            height: 105,
            borderRadius: "50%",
            bgcolor: alpha(theme.palette.grey[500], isDarkMode ? 0.12 : 0.16),
            border: 1,
            borderColor: alpha(theme.palette.grey[500], isDarkMode ? 0.24 : 0.3),
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
          }}
        >
          <Ban
            size={44}
            color={isDarkMode ? theme.palette.grey[400] : theme.palette.grey[600]}
          />
        </Box>
        <Typography variant="h5" fontWeight={600}>
          You don&apos;t have access to this portal yet
        </Typography>
        {message && (
          <Typography variant="body1" color="text.secondary">
            {message}
          </Typography>
        )}
      </Stack>
    </Box>
  );
}
