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

import { LinearProgress } from "@wso2/oxygen-ui";
import type { JSX } from "react";

/**
 * The bar shown along the bottom edge of a file row while it downloads, the
 * same bar the upload dialog uses. Fills with the percentage once the size is
 * known and runs as an indeterminate bar before. The row must be
 * `position: relative`. It is decoration: the percentage is announced by
 * `DownloadProgressStatus`, so the bar is hidden from assistive technology.
 */
export default function DownloadProgressBar({
  progress,
}: {
  /** Percentage (0-100), or null while it is not known. */
  progress: number | null;
}): JSX.Element {
  const known = progress !== null;
  return (
    <LinearProgress
      aria-hidden
      variant={known ? "determinate" : "indeterminate"}
      value={known ? progress : undefined}
      sx={{ position: "absolute", left: 0, right: 0, bottom: 0, height: 4 }}
    />
  );
}
