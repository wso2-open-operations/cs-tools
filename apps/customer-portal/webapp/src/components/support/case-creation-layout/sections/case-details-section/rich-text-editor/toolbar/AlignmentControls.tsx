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
  AlignCenter,
  AlignJustify,
  AlignLeft,
  AlignRight,
} from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";

export interface AlignmentControlsProps {
  onAlign: (alignment: string) => void;
  disabled?: boolean;
}

/**
 * Alignment controls (Left, Center, Right, Justify).
 *
 * @param {AlignmentControlsProps} props - Component props.
 * @returns {JSX.Element} The rendered alignment controls.
 */
export function AlignmentControls({
  onAlign,
  disabled = false,
}: AlignmentControlsProps): JSX.Element {
  return (
    <>
      <Tooltip title="Align left">
        <span>
          <IconButton
            size="small"
            onClick={() => onAlign("justifyLeft")}
            disabled={disabled}
            aria-label="Align left"
          >
            <AlignLeft size={18} />
          </IconButton>
        </span>
      </Tooltip>
      <Tooltip title="Align center">
        <span>
          <IconButton
            size="small"
            onClick={() => onAlign("justifyCenter")}
            disabled={disabled}
            aria-label="Align center"
          >
            <AlignCenter size={18} />
          </IconButton>
        </span>
      </Tooltip>
      <Tooltip title="Align right">
        <span>
          <IconButton
            size="small"
            onClick={() => onAlign("justifyRight")}
            disabled={disabled}
            aria-label="Align right"
          >
            <AlignRight size={18} />
          </IconButton>
        </span>
      </Tooltip>
      <Tooltip title="Justify">
        <span>
          <IconButton
            size="small"
            onClick={() => onAlign("justifyFull")}
            disabled={disabled}
            aria-label="Justify"
          >
            <AlignJustify size={18} />
          </IconButton>
        </span>
      </Tooltip>
    </>
  );
}
