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
import { Redo2, Undo2 } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";

export interface HistoryControlsProps {
  canUndo: boolean;
  canRedo: boolean;
  onUndo: () => void;
  onRedo: () => void;
  disabled?: boolean;
}

/**
 * History controls for the rich text editor.
 *
 * @param {HistoryControlsProps} props - Component props.
 * @returns {JSX.Element} The rendered history controls.
 */
export function HistoryControls({
  canUndo,
  canRedo,
  onUndo,
  onRedo,
  disabled = false,
}: HistoryControlsProps): JSX.Element {
  return (
    <>
      <Tooltip title="Undo">
        <span>
          <IconButton
            size="small"
            onClick={onUndo}
            disabled={disabled || !canUndo}
            aria-label="Undo"
          >
            <Undo2 size={18} />
          </IconButton>
        </span>
      </Tooltip>
      <Tooltip title="Redo">
        <span>
          <IconButton
            size="small"
            onClick={onRedo}
            disabled={disabled || !canRedo}
            aria-label="Redo"
          >
            <Redo2 size={18} />
          </IconButton>
        </span>
      </Tooltip>
    </>
  );
}
