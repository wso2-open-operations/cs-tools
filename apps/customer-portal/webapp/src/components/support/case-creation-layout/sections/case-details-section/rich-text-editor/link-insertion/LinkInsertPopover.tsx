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

import { Box, Button, Popover, TextField, Typography } from "@wso2/oxygen-ui";
import { useEffect, useRef, useState, type JSX } from "react";

export interface LinkPopoverProps {
  open: boolean;
  anchor: HTMLElement | null;
  defaultUrl?: string;
  defaultText?: string;
  onInsert: (url: string, text: string) => void;
  onClose: () => void;
}

/**
 * Link insertion popover for the rich text editor.
 *
 * @param {LinkPopoverProps} props - Component props.
 * @returns {JSX.Element} The rendered link popover.
 */
export function LinkInsertPopover({
  open,
  anchor,
  defaultUrl = "",
  defaultText = "",
  onInsert,
  onClose,
}: LinkPopoverProps): JSX.Element {
  const [url, setUrl] = useState(defaultUrl);
  const [text, setText] = useState(defaultText);
  const urlInputRef = useRef<HTMLInputElement>(null);
  const textInputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (open) {
      setUrl(defaultUrl);
      setText(defaultText);
      const timer = setTimeout(() => {
        if (defaultText) {
          urlInputRef.current?.focus();
        } else {
          textInputRef.current?.focus();
        }
      }, 0);
      return () => clearTimeout(timer);
    }
  }, [open, defaultUrl, defaultText]);

  const handleInsert = () => {
    onInsert(url.trim(), text.trim());
  };

  const handleKeyDown = (
    e: React.KeyboardEvent,
    nextRef?: React.RefObject<HTMLInputElement | null>,
  ) => {
    if (e.key === "Enter") {
      e.preventDefault();
      if (nextRef) {
        nextRef.current?.focus();
      } else {
        handleInsert();
      }
    }
  };

  return (
    <Popover
      open={open}
      anchorEl={anchor}
      onClose={onClose}
      anchorOrigin={{ vertical: "bottom", horizontal: "left" }}
    >
      <Box sx={{ p: 2, minWidth: 280 }}>
        <Typography variant="caption" sx={{ display: "block", mb: 1 }}>
          Text
        </Typography>
        <TextField
          inputRef={textInputRef}
          value={text}
          onChange={(e) => setText(e.target.value)}
          placeholder="Link text"
          fullWidth
          size="small"
          sx={{ mb: 2 }}
          onKeyDown={(e) => handleKeyDown(e, urlInputRef)}
        />
        <Typography variant="caption" sx={{ display: "block", mb: 1 }}>
          Link URL
        </Typography>
        <TextField
          inputRef={urlInputRef}
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          placeholder="https://..."
          fullWidth
          size="small"
          sx={{ mb: 2 }}
          onKeyDown={(e) => handleKeyDown(e)}
        />
        <Button
          size="small"
          variant="contained"
          onClick={handleInsert}
          fullWidth
          disabled={!url.trim()}
        >
          Apply
        </Button>
      </Box>
    </Popover>
  );
}
