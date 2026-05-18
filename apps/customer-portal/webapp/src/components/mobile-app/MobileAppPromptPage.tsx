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
import { type JSX } from "react";
import {
  DeviceType,
  MobileOs,
  type MobileDeviceInfo,
} from "@/types/mobileDevice";
import { getMobileAppStoreUrl } from "@config/mobileAppConfig";

export interface MobileAppPromptPageProps {
  device: MobileDeviceInfo;
}

const STORE_LABELS: Record<MobileOs, string> = {
  [MobileOs.Ios]: "Download on the App Store",
  [MobileOs.Android]: "Get it on Google Play",
};

const DEVICE_COPY: Record<DeviceType, string> = {
  [DeviceType.Phone]: "mobile phone",
  [DeviceType.Tablet]: "tablet",
  [DeviceType.Desktop]: "device",
};

/**
 * Full-screen prompt directing mobile users to install the WSO2 super app
 * and access Customer Portal as an in-app micro app.
 *
 * @param {MobileAppPromptPageProps} props - Detected mobile device info.
 * @returns {JSX.Element} The mobile app download prompt page.
 */
export default function MobileAppPromptPage({
  device,
}: MobileAppPromptPageProps): JSX.Element {
  const storeUrl = getMobileAppStoreUrl(device.os);
  const deviceLabel = DEVICE_COPY[device.deviceType];
  const osLabel = device.os === MobileOs.Ios ? "iOS" : "Android";

  return (
    <Box
      sx={{
        minHeight: "100dvh",
        width: "100%",
        maxWidth: "100vw",
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "center",
        px: 3,
        py: 4,
        boxSizing: "border-box",
        bgcolor: "background.default",
      }}
    >
      <Stack
        spacing={2}
        alignItems="center"
        sx={{ width: "100%", maxWidth: 400, textAlign: "center" }}
      >
        <Typography variant="h5" fontWeight={700}>
          Get the WSO2 Super App
        </Typography>

        <Typography
          variant="body1"
          color="text.secondary"
          sx={{ lineHeight: 1.75 }}
        >
          Customer Portal is available as a default micro app inside the WSO2
          Super App. Install the WSO2 Super App on your {osLabel} {deviceLabel},
          then log in to the WSO2 Super App.
        </Typography>

        {storeUrl ? (
          <Button
            variant="contained"
            color="primary"
            size="large"
            fullWidth
            href={storeUrl}
            target="_blank"
            rel="noopener noreferrer"
            data-testid="mobile-app-download-button"
          >
            {STORE_LABELS[device.os]}
          </Button>
        ) : (
          <Typography variant="body2" color="text.secondary">
            The WSO2 app download link is not configured. Please contact your
            administrator.
          </Typography>
        )}
      </Stack>
    </Box>
  );
}
