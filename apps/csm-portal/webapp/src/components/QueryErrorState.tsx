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

import { Box, Button, Stack, Tooltip, Typography } from "@wso2/oxygen-ui";
import {
  AlertTriangle,
  Check,
  Copy,
  FileQuestion,
  Lock,
  ServerCrash,
  ShieldAlert,
  type LucideIcon,
} from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import { BackendApiError } from "@api/backend/client";
import { getErrorReferenceId } from "@utils/correlationId";

interface QueryErrorStateProps {
  message: string;
  /** Pass the raw error object to enable error-type-aware presentation and
   *  the tracking ID copy button when a correlation ID is available. */
  error?: unknown;
}

interface ErrorPresentation {
  Icon: LucideIcon;
  title: string;
}

function resolvePresentation(error: unknown): ErrorPresentation {
  if (error instanceof BackendApiError) {
    if (error.status === 401) return { Icon: Lock, title: "Session expired" };
    if (error.status === 403) return { Icon: ShieldAlert, title: "Access denied" };
    if (error.status === 404) return { Icon: FileQuestion, title: "Not found" };
    if (error.status >= 400 && error.status < 500) {
      return { Icon: AlertTriangle, title: "Something's not right" };
    }
  }
  return { Icon: ServerCrash, title: "Something went wrong" };
}

/**
 * Inline error state for data-loading failures. Shows an icon and title
 * derived from the HTTP status, the human-readable message, and — when a
 * correlation ID is present — a tracking ID copy button for support handoffs.
 */
export default function QueryErrorState({ message, error }: QueryErrorStateProps): JSX.Element {
  const { Icon, title } = resolvePresentation(error);
  const referenceId = getErrorReferenceId(error);
  const [copied, setCopied] = useState(false);

  const handleCopy = (): void => {
    if (!referenceId) return;
    void navigator.clipboard.writeText(referenceId).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    });
  };

  return (
    <Box sx={{ display: "flex", justifyContent: "center", px: 3, py: 4 }}>
      <Stack spacing={1.5} alignItems="center" sx={{ maxWidth: 420 }}>
        <Icon size={36} strokeWidth={1.5} style={{ opacity: 0.6 }} aria-hidden />
        <Typography variant="subtitle2" fontWeight={700}>
          {title}
        </Typography>
        <Typography variant="body2" color="text.secondary" textAlign="center">
          {message}
        </Typography>
        {referenceId && (
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              gap: 0.75,
              px: 1.5,
              py: 0.75,
              borderRadius: 1,
              bgcolor: "action.hover",
            }}
          >
            <Typography variant="caption" color="text.secondary" sx={{ fontFamily: "monospace" }}>
              Tracking ID: {referenceId}
            </Typography>
            <Tooltip title={copied ? "Copied!" : "Copy tracking ID"} placement="top">
              <Button
                size="small"
                variant="text"
                color="inherit"
                onClick={handleCopy}
                sx={{ minWidth: 0, p: 0.5, color: "text.disabled" }}
                aria-label="Copy reference ID"
              >
                {copied ? <Check size={13} /> : <Copy size={13} />}
              </Button>
            </Tooltip>
          </Box>
        )}
      </Stack>
    </Box>
  );
}
