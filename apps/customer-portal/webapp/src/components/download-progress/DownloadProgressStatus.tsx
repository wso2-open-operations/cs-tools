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


import { Box } from "@wso2/oxygen-ui";
import type { JSX } from "react";

/** A screen reader is told of a change every this many percent, not on every one. */
const ANNOUNCE_STEP = 10;

/**
 * Tells a screen reader how a download is going. It lives beside the download
 * button, not inside it: a disabled button is skipped by assistive technology,
 * so a status inside it would never be read. The region stays mounted while
 * nothing downloads, so the first message is announced.
 */
export default function DownloadProgressStatus({
  name,
  downloading,
  progress,
}: {
  /** File name, for the message. */
  name: string;
  downloading: boolean;
  /** Percentage (0-100), or null while it is not known. */
  progress: number | null;
}): JSX.Element {
  let message = "";
  if (downloading) {
    message =
      progress === null
        ? `Downloading ${name}`
        : `Downloading ${name}: ${Math.floor(progress / ANNOUNCE_STEP) * ANNOUNCE_STEP}%`;
  }
  return (
    <Box
      component="span"
      role="status"
      aria-live="polite"
      aria-atomic="true"
      sx={{
        position: "absolute",
        width: 1,
        height: 1,
        overflow: "hidden",
        clip: "rect(0 0 0 0)",
        whiteSpace: "nowrap",
      }}
    >
      {message}
    </Box>
  );
}
