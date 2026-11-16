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

import { IconButton, Tooltip } from "@wso2/oxygen-ui";
import { X } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";

/**
 * The button that stops a running download, shown beside its progress while the
 * file downloads. Stopping leaves nothing saved (a half-written file the person
 * picked is removed) and shows no error.
 */
export default function DownloadCancelButton({
  name,
  onCancel,
}: {
  /** File name, for the accessible label. */
  name: string;
  onCancel: () => void;
}): JSX.Element {
  return (
    <Tooltip title="Cancel download">
      <IconButton
        size="small"
        aria-label={`Cancel download of ${name}`}
        sx={{ color: "text.secondary" }}
        onClick={onCancel}
      >
        <X size={16} aria-hidden />
      </IconButton>
    </Tooltip>
  );
}
