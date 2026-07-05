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

import { Chip } from "@wso2/oxygen-ui";
import type { JSX } from "react";

/** The semantic palette roles a chip may carry. */
export type SemanticRole = "error" | "warning" | "info" | "success" | "default";

interface SemanticChipProps {
  role: SemanticRole;
  label: string;
  size?: "small" | "medium";
  variant?: "filled" | "outlined";
  /** Bold label — use for priority signals (severity), not quiet metadata. */
  bold?: boolean;
  /** Pointer cursor when wrapped in a link / clickable row. */
  clickable?: boolean;
}

/**
 * A semantic status chip that delegates colouring to MUI's built-in `color`
 * prop so it works correctly in CSS-variables themes (where
 * `theme.palette.info.main` returns a CSS variable reference, not a hex colour,
 * making a manual `bgcolor` sx override unreliable). The `default` role uses
 * the filled variant (subtle grey) rather than `outlined`, so all states render
 * consistently as status badges rather than button-like bordered outlines.
 */
export default function SemanticChip({
  role,
  label,
  size = "small",
  variant,
  bold = false,
  clickable = false,
}: SemanticChipProps): JSX.Element {
  return (
    <Chip
      size={size}
      // "default" is a valid MUI Chip color — filled grey, no border.
      color={role}
      variant={variant}
      label={label}
      sx={{
        ...(bold ? { "& .MuiChip-label": { fontWeight: 600 } } : {}),
        ...(clickable ? { cursor: "pointer" } : {}),
      }}
    />
  );
}
