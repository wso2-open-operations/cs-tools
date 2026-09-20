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

// Fallback UI for a TanStack Query request that rejected with no cached
// data — without this, isLoading simply goes false and data-dependent UI
// (a chart, a list, a detail panel) renders as if the request had
// succeeded with nothing in it, silently misreporting an error as an
// empty state.
import { Box } from "@mui/material";

interface ErrorStateProps {
  message: string;
  onRetry: () => void;
  /** Compact single-line layout for small/inline slots (a table body, a card). */
  compact?: boolean;
}

/** A message plus a Retry button, shown in place of the missing data (see the file-level comment above for why this exists). */
export function ErrorState({ message, onRetry, compact = false }: ErrorStateProps) {
  return (
    <Box
      sx={{
        display: "flex",
        flexDirection: compact ? "row" : "column",
        alignItems: "center",
        justifyContent: "center",
        gap: compact ? 1 : 1.25,
        px: 2.5,
        py: compact ? 2 : 6,
        textAlign: "center",
      }}
    >
      <Box component="span" sx={{ fontSize: compact ? 12.5 : 14, fontWeight: 600, color: "var(--sla-violated)" }}>
        {message}
      </Box>
      <Box
        component="button"
        type="button"
        onClick={onRetry}
        sx={{
          borderRadius: "7px",
          border: "1px solid var(--sla-border)",
          bgcolor: "var(--sla-card)",
          px: 1.25,
          py: "5px",
          fontSize: 12,
          fontWeight: 600,
          color: "var(--sla-fg2)",
          cursor: "pointer",
          "&:hover": { borderColor: "var(--sla-fg3)", color: "var(--sla-fg)" },
        }}
      >
        Retry
      </Box>
    </Box>
  );
}
