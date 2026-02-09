// Copyright (c) 2026 WSO2 LLC. (https://www.wso2.com).
//
// WSO2 LLC. licenses this file to you under the Apache License,
// Version 2.0 (the "License"); you may not use this file except
// in compliance with the License. You may obtain a copy of the License at
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
import { FileCode } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";

export interface MarkdownControlProps {
  onClick: () => void;
  disabled?: boolean;
}

/**
 * Markdown editor control button.
 *
 * @param {MarkdownControlProps} props - Component props.
 * @returns {JSX.Element} The rendered markdown control.
 */
export function MarkdownControl({
  onClick,
  disabled = false,
}: MarkdownControlProps): JSX.Element {
  return (
    <Tooltip title="Edit as Markdown">
      <span>
        <IconButton
          size="small"
          onClick={onClick}
          disabled={disabled}
          aria-label="Edit as Markdown"
        >
          <FileCode size={18} />
        </IconButton>
      </span>
    </Tooltip>
  );
}
