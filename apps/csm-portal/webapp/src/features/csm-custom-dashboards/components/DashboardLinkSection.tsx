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

import {
  Alert,
  Box,
  FormControlLabel,
  IconButton,
  Skeleton,
  Switch,
  TextField,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { Check, Copy } from "@wso2/oxygen-ui-icons-react";
import { type FocusEvent, type JSX, useState } from "react";
import {
  useDeleteCustomDashboardLink,
  useGetCustomDashboardLink,
  usePutCustomDashboardLink,
} from "@features/csm-custom-dashboards/api/useDashboardLink";
import { dashboardLinkUrl } from "@features/csm-custom-dashboards/utils/dashboardLinkUrl";

/** "Opened by 1 person", "Opened by 3 people", "Not opened yet". */
function openedByText(count: number): string {
  if (count === 0) return "Not opened yet.";
  return `Opened by ${count} ${count === 1 ? "person" : "people"}.`;
}

interface DashboardLinkSectionProps {
  /** The caller's dashboard; `undefined` while the Share dialog is closed (nothing is read). */
  dashboardId: string | undefined;
}

/**
 * The Share dialog's "Anyone with the link" part: a switch that turns the
 * dashboard's link on or off, and while it is on, the link with a Copy button
 * and how many people opened it (never who). Anyone with CSM Portal Viewer
 * access who opens the link can view the dashboard under Shared Dashboards
 * while the link stays on; turning it off takes it away from them, and
 * turning it on again makes a new link.
 */
export default function DashboardLinkSection({ dashboardId }: DashboardLinkSectionProps): JSX.Element {
  const link = useGetCustomDashboardLink(dashboardId);
  const enable = usePutCustomDashboardLink();
  const disable = useDeleteCustomDashboardLink();
  const [error, setError] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);

  const busy = enable.isPending || disable.isPending;
  const on = link.data?.enabled === true;
  const url = on && link.data?.linkId ? dashboardLinkUrl(link.data.linkId) : null;

  /** Turns the link on or off; a failure keeps the switch as it was and says which way it failed. */
  const handleToggle = async (next: boolean): Promise<void> => {
    if (!dashboardId) return;
    setError(null);
    setCopied(false);
    try {
      if (next) {
        await enable.mutateAsync(dashboardId);
      } else {
        await disable.mutateAsync(dashboardId);
      }
    } catch {
      setError(next ? "The link couldn't be turned on. Try again in a moment." : "The link couldn't be turned off. Try again in a moment.");
    }
  };

  /** Copies the link to the clipboard; if the browser refuses, asks the user to copy it by hand. */
  const handleCopy = async (): Promise<void> => {
    if (!url) return;
    try {
      await navigator.clipboard.writeText(url);
      setCopied(true);
    } catch {
      setError("The link couldn't be copied. Select it and copy it instead.");
    }
  };

  return (
    <Box aria-label="Share by link" sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
      <Typography variant="subtitle2">Anyone with the link</Typography>
      {error && (
        <Alert severity="error" onClose={() => setError(null)}>
          {error}
        </Alert>
      )}
      {link.isLoading ? (
        <Skeleton variant="rounded" height={36} />
      ) : link.isError || link.data === null ? (
        <Typography variant="body2" color="error">
          Could not load this dashboard&apos;s link.
        </Typography>
      ) : (
        <>
          <FormControlLabel
            control={
              <Switch
                checked={on}
                disabled={busy}
                onChange={(_, checked) => void handleToggle(checked)}
                slotProps={{ input: { "aria-label": "Anyone with CSM Portal Viewer access and the link can view" } }}
              />
            }
            label={
              <Typography variant="body2">
                {on ? "Anyone with CSM Portal Viewer access and the link can view" : "Off: only the people and teams you share with can view"}
              </Typography>
            }
          />
          {url && (
            <>
              <TextField
                value={url}
                size="small"
                fullWidth
                slotProps={{
                  htmlInput: { readOnly: true, "aria-label": "Dashboard link", onFocus: (e: FocusEvent<HTMLInputElement>) => e.target.select() },
                  input: {
                    endAdornment: (
                      <Tooltip title={copied ? "Copied" : "Copy link"}>
                        <IconButton size="small" aria-label="Copy link" onClick={() => void handleCopy()}>
                          {copied ? <Check size={16} /> : <Copy size={16} />}
                        </IconButton>
                      </Tooltip>
                    ),
                  },
                }}
              />
              <Typography variant="caption" color="text.secondary">
                {openedByText(link.data?.openedBy ?? 0)} Turning the link off removes the dashboard from everyone who
                has it only through the link.
              </Typography>
            </>
          )}
        </>
      )}
    </Box>
  );
}
