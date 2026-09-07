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

// Port of v3's src/components/SyncButton.tsx (POST /api/sync/manual ->
// POST /sync/runs, D3).
import { useState } from "react";
import { RefreshCw } from "@wso2/oxygen-ui-icons-react";
import { Box, IconButton } from "@mui/material";
import { useManualSync, useSyncStatus } from "@api/hooks";
import { ApiError } from "@api/client";

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

function oldestLastSynced(repos: Array<{ lastSyncedAt: string | null }>): string | null {
  const times = repos.map((r) => r.lastSyncedAt).filter((t): t is string => t != null);
  if (times.length === 0) return null;
  return times.reduce((oldest, t) => (t < oldest ? t : oldest));
}

export function SyncButton() {
  const { data: status } = useSyncStatus();
  const [transientMessage, setTransientMessage] = useState<string | null>(null);
  const mutation = useManualSync();

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

  return (
    <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
      <Box component="span" sx={{ fontSize: 12, color: errorMessage ? "var(--sla-violated)" : "var(--sla-fg3)" }}>
        {statusText}
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
