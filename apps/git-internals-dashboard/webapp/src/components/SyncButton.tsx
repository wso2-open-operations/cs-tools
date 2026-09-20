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

import { useEffect, useState } from "react";
import { RefreshCw } from "@wso2/oxygen-ui-icons-react";
import { Box, IconButton } from "@mui/material";
import { useManualSync, useSyncStatus } from "@api/hooks";
import { ApiError } from "@api/client";

/** Formats an ISO timestamp as "just now"/"Nm ago"/"Nh ago"/"Nd ago", or "—" for null. */
function fmtRelative(iso: string | null): string {
  if (!iso) return "—";
  const ms = Date.now() - new Date(iso).getTime();
  if (ms < 60_000) return "just now";
  const minutes = Math.floor(ms / 60_000);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

/** Earliest lastSyncedAt across repos, or null if none have synced yet. */
function oldestLastSynced(repos: Array<{ lastSyncedAt: string | null }>): string | null {
  const times = repos.map((r) => r.lastSyncedAt).filter((t): t is string => t != null);
  if (times.length === 0) return null;
  return times.reduce((oldest, t) => (t < oldest ? t : oldest));
}

/** Manual-sync trigger button: triggers a POST /sync/runs run and shows last-synced time and in-flight/error state. */
export function SyncButton() {
  const { data: status } = useSyncStatus();
  const [transientMessage, setTransientMessage] = useState<string | null>(null);
  const [hovered, setHovered] = useState(false);
  const mutation = useManualSync();

  // Fall back to the relative "Last synced" text a few seconds after a
  // successful sync, instead of pinning the "Synced — N issues..." message
  // (and the expanded box) forever.
  useEffect(() => {
    if (!transientMessage) return;
    const t = setTimeout(() => setTransientMessage(null), 5000);
    return () => clearTimeout(t);
  }, [transientMessage]);

  const lastSynced = status ? oldestLastSynced(status.repos) : null;
  const errorMessage = mutation.isError
    ? mutation.error instanceof ApiError
      ? mutation.error.message
      : "Sync failed"
    : null;

  let statusText = `Last synced ${fmtRelative(lastSynced)}`;
  if (mutation.isPending) statusText = "Syncing…";
  else if (errorMessage) statusText = errorMessage;
  else if (transientMessage) statusText = transientMessage;

  const expanded = hovered || mutation.isPending || !!errorMessage || !!transientMessage;

  return (
    <Box sx={{ display: "flex", alignItems: "center" }}>
      <Box
        sx={{
          display: "grid",
          gridTemplateColumns: expanded ? "1fr" : "0fr",
          opacity: expanded ? 1 : 0,
          marginRight: expanded ? "8px" : 0,
          transition: "grid-template-columns 0.25s ease, opacity 0.2s ease, margin-right 0.25s ease",
        }}
      >
        <Box
          component="span"
          sx={{
            display: "inline-block",
            minWidth: 0,
            overflow: "hidden",
            whiteSpace: "nowrap",
            fontSize: 12,
            color: errorMessage ? "var(--sla-violated)" : "var(--sla-fg3)",
          }}
        >
          {statusText}
        </Box>
      </Box>
      <IconButton
        onClick={() => {
          setTransientMessage(null);
          mutation.reset();
          mutation.mutate(undefined, {
            onSuccess: (summary) => {
              const issuesProcessed = summary.repos.reduce((s, r) => s + r.issuesProcessed, 0);
              const eventsInserted = summary.repos.reduce((s, r) => s + r.eventsInserted, 0);
              setTransientMessage(`Synced — ${issuesProcessed} issues, ${eventsInserted} events`);
            },
          });
        }}
        onMouseEnter={() => setHovered(true)}
        onMouseLeave={() => setHovered(false)}
        onFocus={() => setHovered(true)}
        onBlur={() => setHovered(false)}
        disabled={mutation.isPending}
        aria-label="Sync now"
        title="Sync now"
        sx={{
          height: 28,
          width: 28,
          borderRadius: "9px",
          border: "1px solid var(--sla-border)",
          bgcolor: "var(--sla-card)",
          color: "var(--sla-fg)",
          "&.Mui-disabled": { opacity: 0.6 },
        }}
      >
        <RefreshCw size={14} style={mutation.isPending ? { animation: "spin 1s linear infinite" } : undefined} />
      </IconButton>
      <style>{"@keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }"}</style>
    </Box>
  );
}
