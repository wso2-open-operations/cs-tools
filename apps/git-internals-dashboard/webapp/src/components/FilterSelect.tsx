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

import { Children, isValidElement, type ReactElement, type ReactNode } from "react";
import { MenuItem, Select, type SelectChangeEvent } from "@mui/material";

interface FilterSelectProps {
  value: string;
  onChange: (v: string) => void;
  children: ReactNode;
}

// A native <select>'s options popup is rendered by the OS/browser, not the
// page — no CSS/theme can reach it. MUI's Select renders its popup via
// Popover/MenuItem, which Oxygen UI's AcrylicBaseTheme already themes
// (background.paper + blur.medium, see MuiPopover/MuiMenuItem overrides in
// node_modules/@wso2/oxygen-ui) — no custom sx needed for the popup itself.
export function FilterSelect({ value, onChange, children }: FilterSelectProps) {
  // MUI warns (and renders a blank selection) when `value` doesn't match any
  // child MenuItem's value — e.g. a URL carrying an ABT team that currently
  // has no open issues, so it isn't in the option list below. Rendering an
  // extra MenuItem for that exact value keeps the select showing it instead
  // of silently falling back to blank. Children.toArray flattens the mix of
  // static <MenuItem>s and mapped arrays every call site passes as children.
  const knownValues = new Set(
    Children.toArray(children)
      .filter(
        (c): c is ReactElement<{ value: string }> =>
          isValidElement<{ value: string }>(c) && typeof c.props.value === "string",
      )
      .map((c) => c.props.value),
  );
  const missing = value !== "all" && !knownValues.has(value);

  return (
    <Select
      value={value}
      onChange={(e: SelectChangeEvent) => onChange(e.target.value)}
      size="small"
      sx={{ minWidth: 150 }}
    >
      {children}
      {missing && <MenuItem value={value}>{value}</MenuItem>}
    </Select>
  );
}
