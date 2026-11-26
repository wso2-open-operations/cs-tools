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

import { Stack, Typography } from "@wso2/oxygen-ui";
import { Ban } from "@wso2/oxygen-ui-icons-react";

/**
 * Shown in place of the app's pages when the signed-in user holds no portal role, or when
 * `GET /users/me` answers 401/403. Mirrors the webapp's NoPortalAccessPage: a plain status
 * message, not an error.
 */
export function NoPortalAccess({ status }: { status?: number }) {
  return (
    <Stack alignItems="center" justifyContent="center" gap={2} py={8} px={2} textAlign="center">
      <Ban size={44} color="currentColor" />
      <Typography variant="h6" fontWeight={600}>
        You don&apos;t have access to this portal yet
      </Typography>
      <Typography variant="body2" color="text.secondary">
        {status === undefined
          ? "Ask an administrator to give you a portal role, then reopen the app."
          : `The portal did not accept your sign-in (it answered ${status}). If you already have a portal role, this app may not be allowed to reach the portal yet.`}
      </Typography>
    </Stack>
  );
}
