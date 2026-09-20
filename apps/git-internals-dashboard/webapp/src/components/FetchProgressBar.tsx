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

import { useState, type ReactNode } from "react";
import { LinearProgress } from "@mui/material";
import { ActiveContext, SetActiveContext } from "@lib/fetchProgress";

/**
 * Holds whether any routed page is currently showing placeholder data while a
 * background refetch is in flight. Wraps AppShell so the progress bar it
 * renders (pinned above the sticky header) can reflect the state of whatever
 * page is mounted below it via `useReportFetchProgress`.
 */
export function FetchProgressProvider({ children }: { children: ReactNode }) {
  const [active, setActive] = useState(false);
  return (
    <ActiveContext.Provider value={active}>
      <SetActiveContext.Provider value={setActive}>{children}</SetActiveContext.Provider>
    </ActiveContext.Provider>
  );
}

/**
 * Thin, non-intrusive indicator that a background refetch is in flight while
 * `keepPreviousData` keeps the prior filter's data on screen. Fixed to the
 * top edge of the viewport, above AppShell's sticky header, so it never
 * reserves layout space and reads as "the page is busy" rather than "this
 * one row is busy".
 */
export function FetchProgressBar({ active }: { active: boolean }) {
  if (!active) return null;
  return <LinearProgress sx={{ position: "fixed", top: 0, left: 0, right: 0, height: 3, zIndex: 30 }} />;
}
