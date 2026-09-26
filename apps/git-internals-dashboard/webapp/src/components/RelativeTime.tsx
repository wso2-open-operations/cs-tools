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

import { useSyncExternalStore } from "react";
import { Box, Tooltip } from "@mui/material";
import { formatAbsoluteTimestamp, formatRelativeTime } from "@lib/time";

const TICK_MS = 60_000;

// A single shared interval, ticking every subscribed RelativeTime's "now"
// together, so a table of many timestamps re-renders off one timer instead
// of one per row. Starts with the first subscriber and stops with the last —
// idle tables cost nothing.
const listeners = new Set<() => void>();
let now = Date.now();
let intervalId: ReturnType<typeof setInterval> | null = null;

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  if (intervalId === null) {
    const fresh = Date.now();
    if (fresh - now >= TICK_MS) {
      now = fresh;
      listener();
    }
    intervalId = setInterval(() => {
      now = Date.now();
      listeners.forEach((l) => l());
    }, TICK_MS);
  }
  return () => {
    listeners.delete(listener);
    if (listeners.size === 0 && intervalId !== null) {
      clearInterval(intervalId);
      intervalId = null;
    }
  };
}

// A snapshot getter returning a variable set only from the interval's own
// callback (never Date.now() itself) keeps this pure to call during render.
function getSnapshot(): number {
  return now;
}

function useTickerNow(): number {
  return useSyncExternalStore(subscribe, getSnapshot);
}

interface RelativeTimeProps {
  iso: string | null | undefined;
}

/** Relative timestamp ("7h ago"), with the exact timestamp shown on hover. No tooltip when iso itself is absent. */
export function RelativeTime({ iso }: RelativeTimeProps) {
  const tickerNow = useTickerNow();
  const text = formatRelativeTime(iso, tickerNow);
  const span = (
    <Box component="span" sx={{ whiteSpace: "nowrap" }}>
      {text}
    </Box>
  );

  if (iso == null) return span;

  return (
    <Tooltip title={formatAbsoluteTimestamp(iso) ?? "Unknown time"} placement="top" arrow>
      {span}
    </Tooltip>
  );
}
