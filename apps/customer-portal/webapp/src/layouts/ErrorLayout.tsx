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

import { type JSX, type ReactNode } from "react";
import { Box } from "@wso2/oxygen-ui";
import { notificationBannerConfig } from "@config/notificationBannerConfig";
import GlobalNotificationBanner from "@components/notification-banner/GlobalNotificationBanner";
import TopBanner from "@components/top-banner/TopBanner";

interface ErrorLayoutProps {
  children: ReactNode;
}

/**
 * ErrorLayout component for displaying error pages without the sidebar navigation.
 *
 * @returns {JSX.Element} The ErrorLayout component.
 */
export default function ErrorLayout({ children }: ErrorLayoutProps): JSX.Element {
  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: "column",
        height: "100dvh",
        overflow: "hidden",
      }}
    >
      <TopBanner />
      <GlobalNotificationBanner visible={notificationBannerConfig.visible} />
      <Box
        sx={{
          flex: 1,
          minHeight: 0,
          width: "100%",
          display: "flex",
          flexDirection: "column",
          overflow: "auto",
          p: 3,
        }}
      >
        {children}
      </Box>
    </Box>
  );
}
