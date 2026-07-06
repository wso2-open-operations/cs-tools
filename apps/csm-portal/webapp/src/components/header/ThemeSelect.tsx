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

import {
  Box,
  MenuItem,
  Select,
  type SelectChangeEvent,
} from "@wso2/oxygen-ui";
import { Palette } from "@wso2/oxygen-ui-icons-react";
import type { JSX } from "react";
import { useThemePreference } from "@context/theme/ThemePreferenceContext";
import { isThemeKey } from "@config/themeConfig";

/**
 * Header dropdown that switches the active Oxygen UI theme at runtime. The
 * choice is persisted per user (localStorage) by the ThemePreferenceProvider,
 * so it survives reloads. Orthogonal to the light/dark ColorSchemeToggle that
 * sits next to it — this picks the palette, that picks the colour scheme.
 */
export default function ThemeSelect(): JSX.Element {
  const { themeKey, setThemeKey, options } = useThemePreference();

  const handleChange = (e: SelectChangeEvent<string>): void => {
    const next = e.target.value;
    if (isThemeKey(next)) setThemeKey(next);
  };

  return (
    <Box sx={{ display: "flex", alignItems: "center" }}>
        <Select
          value={themeKey}
          onChange={handleChange}
          size="small"
          variant="standard"
          disableUnderline
          aria-label="Select theme"
          startAdornment={
            <Palette
              size={16}
              style={{ marginRight: 6, opacity: 0.7, flexShrink: 0 }}
            />
          }
          sx={{
            minWidth: 132,
            fontSize: "0.8125rem",
            color: "text.secondary",
            "& .MuiSelect-select": {
              display: "flex",
              alignItems: "center",
              py: 0.5,
            },
          }}
        >
          {options.map((o) => (
            <MenuItem key={o.key} value={o.key} sx={{ fontSize: "0.8125rem" }}>
              {o.label}
            </MenuItem>
          ))}
        </Select>
    </Box>
  );
}
