"use client";

import { useState } from "react";
import { RefreshCw } from "@wso2/oxygen-ui-icons-react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { Box, IconButton } from "@mui/material";
import { api, useSyncStatus } from "@/lib/api";

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
  const queryClient = useQueryClient();
  const { data: status } = useSyncStatus();
  const [transientMessage, setTransientMessage] = useState<string | null>(null);

  const mutation = useMutation({
    mutationFn: api.postSyncManual,
    onSuccess: (summary) => {
      const issuesProcessed = summary.repos.reduce((s, r) => s + r.issuesProcessed, 0);
      const eventsInserted = summary.repos.reduce((s, r) => s + r.eventsInserted, 0);
      setTransientMessage(`Synced — ${issuesProcessed} issues, ${eventsInserted} events`);
      void queryClient.invalidateQueries();
    },
  });

  const lastSynced = status ? oldestLastSynced(status.repos) : null;
  const errorMessage = mutation.isError ? (mutation.error as Error).message : null;

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
          mutation.mutate();
        }}
        disabled={mutation.isPending}
        aria-label="Sync now"
        title="Sync now"
        sx={{
          height: 28, width: 28, borderRadius: "9px", border: "1px solid var(--sla-border)",
          bgcolor: "var(--sla-card)", color: "var(--sla-fg)", "&.Mui-disabled": { opacity: 0.6 },
        }}
      >
        <RefreshCw
          size={14}
          style={mutation.isPending ? { animation: "spin 1s linear infinite" } : undefined}
        />
      </IconButton>
      <style>{"@keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }"}</style>
    </Box>
  );
}
