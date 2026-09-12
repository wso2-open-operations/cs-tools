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

import type { JSX } from "react";
import { Box, CircularProgress, Typography } from "@wso2/oxygen-ui";

/**
 * Small "label above value" primitive shared by the alert/smart-alert detail
 * modals (and mirroring `CallRequestDetailModal`'s local `Field`) — kept here
 * rather than duplicated a third time since both new modals use it.
 */
export function DetailField({
  label,
  value,
}: {
  label: string;
  value: JSX.Element | string;
}): JSX.Element {
  return (
    <Box sx={{ minWidth: 0, display: "flex", flexDirection: "column", gap: 0.5 }}>
      <Typography variant="caption" color="text.secondary">
        {label}
      </Typography>
      {typeof value === "string" ? (
        <Typography variant="body2" sx={{ whiteSpace: "pre-line", wordBreak: "break-word" }}>
          {value}
        </Typography>
      ) : (
        value
      )}
    </Box>
  );
}

/**
 * Renders an opaque free-form field (an alert's `description` / a smart
 * alert's `details`) that may be JSON. Attempts a `JSON.parse` +
 * pretty-print for readability, falling back to the raw string when it
 * isn't valid JSON — no per-source-format parser, just this one heuristic.
 */
export function JsonOrTextBlock({ value }: { value: string }): JSX.Element {
  const pretty = tryPrettyPrintJson(value);
  return (
    <Typography
      variant="body2"
      component="pre"
      sx={{
        m: 0,
        p: 1,
        bgcolor: "background.default",
        borderRadius: 1,
        overflowX: "auto",
        whiteSpace: pretty ? "pre" : "pre-line",
        wordBreak: "break-word",
        fontFamily: pretty ? "monospace" : "inherit",
        fontSize: pretty ? "0.8em" : "inherit",
      }}
    >
      {pretty ?? value}
    </Typography>
  );
}

function tryPrettyPrintJson(value: string): string | null {
  const trimmed = value.trim();
  if (!trimmed || (trimmed[0] !== "{" && trimmed[0] !== "[")) return null;
  try {
    return JSON.stringify(JSON.parse(trimmed), null, 2);
  } catch {
    return null;
  }
}

/** Centered spinner shown while a link-resolved record is loading. */
export function SnLinkLoadingState(): JSX.Element {
  return (
    <Box sx={{ display: "flex", justifyContent: "center", py: 4 }}>
      <CircularProgress size={28} />
    </Box>
  );
}

/**
 * Shown when the resolved id 404s (stale/unknown reference) or the fetch
 * otherwise failed — never a thrown error or a blank modal.
 */
export function SnLinkNotFoundState({ kind }: { kind: string }): JSX.Element {
  return (
    <Typography variant="body2" color="text.secondary" sx={{ py: 2 }}>
      This {kind} could no longer be found. It may have been removed, or the
      reference may be out of date.
    </Typography>
  );
}
