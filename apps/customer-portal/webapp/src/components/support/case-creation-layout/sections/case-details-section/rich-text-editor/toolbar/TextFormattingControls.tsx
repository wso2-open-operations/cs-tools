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
  Bold,
  Italic,
  Strikethrough,
  Underline,
} from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";

export interface ActiveFormats {
  bold: boolean;
  italic: boolean;
  underline: boolean;
}

export interface TextFormattingControlsProps {
  activeFormats: ActiveFormats;
  onFormat: (command: string) => void;
  disabled?: boolean;
}

/**
 * Text formatting controls (Bold, Italic, Underline, Strikethrough).
 *
 * @param {TextFormattingControlsProps} props - Component props.
 * @returns {JSX.Element} The rendered text formatting controls.
 */
export function TextFormattingControls({
  activeFormats,
  onFormat,
  disabled = false,
}: TextFormattingControlsProps): JSX.Element {
  return (
    <>
      <Tooltip title={activeFormats.bold ? "Bold (active)" : "Bold"}>
        <span>
          <IconButton
            size="small"
            onClick={() => onFormat("bold")}
            disabled={disabled}
            aria-label="Bold"
            sx={activeFormats.bold ? { bgcolor: "action.selected" } : undefined}
          >
            <Bold size={18} />
          </IconButton>
        </span>
      </Tooltip>
      <Tooltip title={activeFormats.italic ? "Italic (active)" : "Italic"}>
        <span>
          <IconButton
            size="small"
            onClick={() => onFormat("italic")}
            disabled={disabled}
            aria-label="Italic"
            sx={
              activeFormats.italic ? { bgcolor: "action.selected" } : undefined
            }
          >
            <Italic size={18} />
          </IconButton>
        </span>
      </Tooltip>
      <Tooltip
        title={activeFormats.underline ? "Underline (active)" : "Underline"}
      >
        <span>
          <IconButton
            size="small"
            onClick={() => onFormat("underline")}
            disabled={disabled}
            aria-label="Underline"
            sx={
              activeFormats.underline
                ? { bgcolor: "action.selected" }
                : undefined
            }
          >
            <Underline size={18} />
          </IconButton>
        </span>
      </Tooltip>
      <Tooltip title="Strikethrough">
        <span>
          <IconButton
            size="small"
            onClick={() => onFormat("strikeThrough")}
            disabled={disabled}
            aria-label="Strikethrough"
          >
            <Strikethrough size={18} />
          </IconButton>
        </span>
      </Tooltip>
    </>
  );
}
