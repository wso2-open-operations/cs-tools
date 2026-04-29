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

import { Alert, Fade, Snackbar as MuiSnackbar } from "@wso2/oxygen-ui";
import { BanIcon, CheckIcon, InfoIcon, TriangleAlertIcon } from "@wso2/oxygen-ui-icons-react";
import { type ReactNode } from "react";

const DEFAULT_DURATION = 4000;

const ICONS: Record<Severity, ReactNode> = {
  success: <CheckIcon fontSize="inherit" />,
  error: <BanIcon fontSize="inherit" />,
  info: <InfoIcon fontSize="inherit" />,
  warning: <TriangleAlertIcon fontSize="inherit" />,
};

export type Severity = "success" | "error" | "info" | "warning";

interface SnackbarProps {
  message: string;
  severity: Severity;
  open: boolean;
  onClose: () => void;
}

export function Snackbar({ message, severity, open, onClose }: SnackbarProps) {
  return (
    <MuiSnackbar
      open={open}
      onClose={(_, reason) => {
        if (reason === "clickaway" || reason == "escapeKeyDown") return;
        else onClose();
      }}
      slots={{ transition: Fade }}
      anchorOrigin={{ vertical: "top", horizontal: "center" }}
      autoHideDuration={DEFAULT_DURATION}
      sx={{ top: `calc(var(--app-bar-height) + 80px)`, zIndex: 1000 }}
    >
      <Alert icon={ICONS[severity]} severity={severity} onClose={onClose}>
        {message}
      </Alert>
    </MuiSnackbar>
  );
}
