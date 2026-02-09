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
  Code,
  Image as ImageIcon,
  Link,
  Paperclip,
} from "@wso2/oxygen-ui-icons-react";
import { useRef, type JSX } from "react";

export interface InsertControlsProps {
  onInsertLink: (anchorEl: HTMLElement) => void;
  onOpenCodeDialog: () => void;
  onImageSelect: (e: React.ChangeEvent<HTMLInputElement>) => void;
  onFileSelect: (e: React.ChangeEvent<HTMLInputElement>) => void;
  disabled?: boolean;
}

/**
 * Insert controls (Link, Code block, Image, File attachment).
 *
 * @param {InsertControlsProps} props - Component props.
 * @returns {JSX.Element} The rendered insert controls.
 */
export function InsertControls({
  onInsertLink,
  onOpenCodeDialog,
  onImageSelect,
  onFileSelect,
  disabled = false,
}: InsertControlsProps): JSX.Element {
  const imageInputRef = useRef<HTMLInputElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  return (
    <>
      <Tooltip title="Insert link">
        <span>
          <IconButton
            size="small"
            onClick={(e) => onInsertLink(e.currentTarget)}
            disabled={disabled}
            aria-label="Insert or edit link"
          >
            <Link size={18} />
          </IconButton>
        </span>
      </Tooltip>
      <Tooltip title="Insert code block">
        <span>
          <IconButton
            size="small"
            onClick={onOpenCodeDialog}
            disabled={disabled}
            aria-label="Insert code block"
          >
            <Code size={18} />
          </IconButton>
        </span>
      </Tooltip>
      <Tooltip title="Insert image">
        <span>
          <IconButton
            size="small"
            component="label"
            disabled={disabled}
            aria-label="Insert image"
          >
            <ImageIcon size={18} />
            <input
              ref={imageInputRef}
              type="file"
              accept="image/*"
              hidden
              onChange={(e) => {
                onImageSelect(e);
                e.currentTarget.value = "";
              }}
            />
          </IconButton>
        </span>
      </Tooltip>
      <Tooltip title="Attach file">
        <span>
          <IconButton
            size="small"
            component="label"
            disabled={disabled}
            aria-label="Attach file"
          >
            <Paperclip size={18} />
            <input
              ref={fileInputRef}
              type="file"
              hidden
              onChange={(e) => {
                onFileSelect(e);
                e.currentTarget.value = "";
              }}
            />
          </IconButton>
        </span>
      </Tooltip>
    </>
  );
}
