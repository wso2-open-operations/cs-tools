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

import { Alert, Box } from "@wso2/oxygen-ui";
import {
  BANNER_HEADER_GAP_PX,
  BANNER_RIGHT_GAP_PX,
  HEADER_HEIGHT_PX,
} from "@constants/errorBannerConstants";
import type { JSX } from "react";

interface ErrorBannerProps {
  /** User-facing error message; supplied by the caller. */
  message: string;
  onClose: () => void;
}

/**
 * ErrorBanner component displayed above the footer at the right corner.
 * Uses Oxygen UI Alert component. Displays the message passed from the caller.
 *
 * @param {ErrorBannerProps} props - Component props.
 * @returns {JSX.Element} The ErrorBanner JSX.
 */
export default function ErrorBanner({
  message,
  onClose,
}: ErrorBannerProps): JSX.Element {
  return (
    <Box
      sx={{
        position: "fixed",
        top: HEADER_HEIGHT_PX + BANNER_HEADER_GAP_PX,
        right: BANNER_RIGHT_GAP_PX,
        width: 400,
        zIndex: 10,
      }}
    >
      <Alert
        severity="error"
        onClose={onClose}
        elevation={6}
        sx={{ width: "100%" }}
      >
        {message}
      </Alert>
    </Box>
  );
}
