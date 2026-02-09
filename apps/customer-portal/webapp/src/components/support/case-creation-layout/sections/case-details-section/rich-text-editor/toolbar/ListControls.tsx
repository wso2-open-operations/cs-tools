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
import {
  IndentDecrease,
  IndentIncrease,
  List,
  ListOrdered,
} from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";

export interface ListControlsProps {
  onCommand: (command: string) => void;
  disabled?: boolean;
}

/**
 * List and indentation controls.
 *
 * @param {ListControlsProps} props - Component props.
 * @returns {JSX.Element} The rendered list controls.
 */
export function ListControls({
  onCommand,
  disabled = false,
}: ListControlsProps): JSX.Element {
  return (
    <>
      <Tooltip title="Bullet list">
        <span>
          <IconButton
            size="small"
            onClick={() => onCommand("insertUnorderedList")}
            disabled={disabled}
            aria-label="Bullet list"
          >
            <List size={18} />
          </IconButton>
        </span>
      </Tooltip>
      <Tooltip title="Numbered list">
        <span>
          <IconButton
            size="small"
            onClick={() => onCommand("insertOrderedList")}
            disabled={disabled}
            aria-label="Numbered list"
          >
            <ListOrdered size={18} />
          </IconButton>
        </span>
      </Tooltip>
      <Tooltip title="Decrease indent">
        <span>
          <IconButton
            size="small"
            onClick={() => onCommand("outdent")}
            disabled={disabled}
            aria-label="Decrease indent"
          >
            <IndentDecrease size={18} />
          </IconButton>
        </span>
      </Tooltip>
      <Tooltip title="Increase indent">
        <span>
          <IconButton
            size="small"
            onClick={() => onCommand("indent")}
            disabled={disabled}
            aria-label="Increase indent"
          >
            <IndentIncrease size={18} />
          </IconButton>
        </span>
      </Tooltip>
    </>
  );
}
