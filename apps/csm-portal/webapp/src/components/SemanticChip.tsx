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

import { Chip, useTheme } from "@wso2/oxygen-ui";
import type { JSX } from "react";
import { pickAccessibleText } from "@utils/contrastText";

/** The semantic palette roles a chip may carry. */
export type SemanticRole = "error" | "warning" | "info" | "success" | "default";

interface SemanticChipProps {
  role: SemanticRole;
  label: string;
  size?: "small" | "medium";
  /** Bold label — use for priority signals (severity), not quiet metadata. */
  bold?: boolean;
  /** Pointer cursor when wrapped in a link / clickable row. */
  clickable?: boolean;
}

/**
 * A chip whose filled label always clears WCAG AA.
 *
 * A plain `<Chip color="error">` uses MUI's `contrastText`, computed against a
 * 3:1 floor, so white-on-saturated chip labels (~13px) sit around 3:1 and fail
 * AA (4.5:1) — in every theme, because it is the contrast-target that is wrong,
 * not the palette. This keeps the vivid `*.main` fill but picks the label colour
 * (near-black vs white) by the fill's measured luminance, which clears 4.5:1 for
 * the saturated semantic colours in all shipped themes and both colour modes.
 * The `default`/grey role has no accessible solid fill, so it renders outlined.
 */
export default function SemanticChip({
  role,
  label,
  size = "small",
  bold = false,
  clickable = false,
}: SemanticChipProps): JSX.Element {
  const theme = useTheme();

  // Resolve the palette fill defensively: an unexpected role (e.g. a free-form
  // value that slipped past the type) must degrade to the outlined default
  // chip, never read `.main` off an undefined palette entry and crash the page.
  const paletteEntry =
    role === "default"
      ? undefined
      : (theme.palette[role] as { main?: string } | undefined);

  if (!paletteEntry?.main) {
    return (
      <Chip
        size={size}
        variant="outlined"
        label={label}
        sx={clickable ? { cursor: "pointer" } : undefined}
      />
    );
  }

  const bg = paletteEntry.main;
  const fg = pickAccessibleText(bg);

  return (
    <Chip
      size={size}
      label={label}
      sx={{
        bgcolor: bg,
        color: fg,
        ...(bold ? { fontWeight: 600 } : {}),
        "& .MuiChip-label": { color: fg },
        ...(clickable ? { cursor: "pointer" } : {}),
      }}
    />
  );
}
