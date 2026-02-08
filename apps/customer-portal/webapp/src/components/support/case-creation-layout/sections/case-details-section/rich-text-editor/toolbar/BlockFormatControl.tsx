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

import {
  IconButton,
  Menu,
  MenuItem,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import { ChevronDown } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import { getBlockDisplay } from "@utils/richTextEditor";

export interface BlockTag {
  value: string;
  label: string;
  variant:
    | "h1"
    | "h2"
    | "h3"
    | "h4"
    | "h5"
    | "h6"
    | "subtitle1"
    | "subtitle2"
    | "body1"
    | "body2"
    | "caption";
}

export interface BlockFormatControlProps {
  currentTag: string;
  tags: BlockTag[];
  onChange: (tag: string) => void;
  disabled?: boolean;
}

/**
 * Block format dropdown control (Paragraph, H1-H6, Code).
 *
 * @param {BlockFormatControlProps} props - Component props.
 * @returns {JSX.Element} The rendered block format control.
 */
export function BlockFormatControl({
  currentTag,
  tags,
  onChange,
  disabled = false,
}: BlockFormatControlProps): JSX.Element {
  const [anchorEl, setAnchorEl] = useState<HTMLElement | null>(null);

  const handleClick = (e: React.MouseEvent<HTMLButtonElement>) => {
    setAnchorEl(anchorEl ? null : e.currentTarget);
  };

  const handleClose = () => {
    setAnchorEl(null);
  };

  const handleSelect = (tag: string) => {
    onChange(tag);
    handleClose();
  };

  return (
    <>
      <Tooltip title="Block format">
        <span>
          <IconButton
            size="small"
            onClick={handleClick}
            disabled={disabled}
            aria-label="Block format"
            sx={{ display: "flex", alignItems: "center", gap: 0.25 }}
          >
            <Typography
              component="span"
              variant={getBlockDisplay(currentTag).variant}
              sx={{
                fontSize: "0.8125rem",
                lineHeight: 1.2,
                maxWidth: 100,
                overflow: "hidden",
                textOverflow: "ellipsis",
                whiteSpace: "nowrap",
                display: "block",
              }}
            >
              {getBlockDisplay(currentTag).label}
            </Typography>
            <ChevronDown size={16} />
          </IconButton>
        </span>
      </Tooltip>
      <Menu
        open={Boolean(anchorEl)}
        anchorEl={anchorEl}
        onClose={handleClose}
        anchorOrigin={{ vertical: "bottom", horizontal: "left" }}
      >
        {tags.map(({ value: v, label, variant }) => (
          <MenuItem key={v} onClick={() => handleSelect(v)}>
            <Typography variant={variant}>{label}</Typography>
          </MenuItem>
        ))}
      </Menu>
    </>
  );
}
