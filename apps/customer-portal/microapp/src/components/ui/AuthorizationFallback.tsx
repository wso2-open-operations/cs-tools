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

import { Backdrop, Box, Button, pxToRem, Stack, Typography } from "@wso2/oxygen-ui";
import { LockIcon, LogOutIcon } from "@wso2/oxygen-ui-icons-react";
import { goToMyAppsScreen } from "../microapp-bridge";

export function AuthorizationFallback() {
  return (
    <Backdrop
      open={true}
      role="alertdialog"
      aria-modal="true"
      aria-labelledby="permission-title"
      component={Stack}
      sx={{
        bgcolor: "background.default",
        zIndex: 9999,
        textAlign: "center",
        alignItems: "center",
        gap: 2,
      }}
    >
      <Box color="primary.main">
        <LockIcon style={{ fontSize: pxToRem(30) }} />
      </Box>

      <Stack>
        <Typography variant="h6" fontWeight={600} id="permission-title">
          Permission Required
        </Typography>

        <Typography variant="body2" color="text.secondary" mt={0.3} px={1}>
          To use the Customer Portal, your organization needs an active Customer Support subscription.
        </Typography>
        <Typography variant="body2" color="text.secondary" mt={0.3}></Typography>
      </Stack>

      <Button variant="contained" color="secondary" sx={{ mt: 2 }} onClick={goToMyAppsScreen}>
        <LogOutIcon size={pxToRem(20)} style={{ transform: "scaleX(-1)", marginRight: 10 }} />
        Exit
      </Button>
      <Typography
        variant="subtitle2"
        color="text.secondary"
        mt={0.3}
        px={1}
        sx={{ position: "absolute", bottom: "calc(var(--safe-bottom))", opacity: 0.6 }}
      >
        If you're unsure, please check with your admin or contact support.
      </Typography>
    </Backdrop>
  );
}
