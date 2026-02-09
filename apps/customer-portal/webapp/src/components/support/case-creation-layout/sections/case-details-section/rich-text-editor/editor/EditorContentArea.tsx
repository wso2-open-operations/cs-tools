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

import { Box } from "@wso2/oxygen-ui";
import type { JSX, KeyboardEvent, RefObject } from "react";

export interface EditorAreaProps {
  editorRef: RefObject<HTMLDivElement | null>;
  placeholder: string;
  disabled: boolean;
  minHeight: number;
  onInput: () => void;
  onKeyDown: (e: KeyboardEvent<HTMLDivElement>) => void;
  onPaste: (e: React.ClipboardEvent<HTMLDivElement>) => void;
  onClick: (e: React.MouseEvent<HTMLDivElement>) => void;
}

/**
 * ContentEditable editor area for the rich text editor.
 *
 * @param {EditorAreaProps} props - Component props.
 * @returns {JSX.Element} The rendered editor area.
 */
export function EditorContentArea({
  editorRef,
  placeholder,
  disabled,
  minHeight,
  onInput,
  onKeyDown,
  onPaste,
  onClick,
}: EditorAreaProps): JSX.Element {
  return (
    <Box
      ref={editorRef}
      contentEditable={!disabled}
      suppressContentEditableWarning
      role="textbox"
      aria-multiline
      aria-disabled={disabled}
      aria-label={placeholder}
      onInput={onInput}
      onKeyDown={onKeyDown}
      onPaste={onPaste}
      onClick={onClick}
      data-placeholder={placeholder}
      sx={{
        minHeight,
        p: 2,
        outline: "none",
        fontSize: "0.857rem",
        lineHeight: 1.5,
        color: disabled ? "text.disabled" : "text.primary",
        cursor: disabled ? "not-allowed" : "text",
        "&:empty::before": {
          content: `attr(data-placeholder)`,
          color: "text.disabled",
          fontSize: "0.857rem",
          lineHeight: 1.5,
          pointerEvents: "none",
        },
        "& pre": {
          overflow: "auto",
          p: 2,
          fontFamily: "monospace",
          fontSize: "0.875rem",
          margin: "8px 0",
          bgcolor: "action.hover",
          color: "text.primary",
        },
        "& .rich-text-image-wrap": {
          display: "inline-block",
          position: "relative",
          margin: "8px 0",
          maxWidth: "100%",
        },
        "& .rich-text-inline-img": {
          maxWidth: "100%",
          maxHeight: 480,
          width: "auto",
          height: "auto",
          display: "block",
          verticalAlign: "middle",
          objectFit: "contain",
        },
        "& .rich-text-image-delete": {
          position: "absolute",
          top: 4,
          right: 4,
          width: 24,
          height: 24,
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          fontSize: 18,
          lineHeight: 1,
          padding: 0,
        },
        "& ul, & ol": { pl: 3 },
        "& h1": { fontSize: "2.571rem", fontWeight: 400 },
        "& h2": { fontSize: "2.143rem", fontWeight: 400 },
        "& h3": { fontSize: "1.714rem", fontWeight: 400 },
        "& h4": { fontSize: "1.286rem", fontWeight: 400 },
        "& h5": { fontSize: "1.143rem", fontWeight: 400 },
        "& h6": { fontSize: "1rem", fontWeight: 500 },
        "& .subtitle1": { fontSize: "1.286rem" },
        "& .subtitle2": { fontSize: "1rem", fontWeight: 400 },
        "& .body1": { fontSize: "1rem" },
        "& .body2": { fontSize: "0.857rem", fontWeight: 400 },
        "& .caption": { fontSize: "0.786rem", fontWeight: 400 },
      }}
    />
  );
}
