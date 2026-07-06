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
// software distributed under the License is distributed on
// an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

import { Box, Paper, Typography, alpha, colors } from "@wso2/oxygen-ui";
import { Type } from "@wso2/oxygen-ui-icons-react";
import { useRef, type JSX } from "react";
import {
  useFontSize,
  type FontSizeOption,
} from "@context/font-size/FontSizeContext";
import { useDarkMode } from "@utils/useDarkMode";
import {
  SETTINGS_DISPLAY_FONT_SIZE_DESCRIPTION,
  SETTINGS_DISPLAY_FONT_SIZE_OPTIONS,
  SETTINGS_DISPLAY_FONT_SIZE_TITLE,
  SETTINGS_DISPLAY_HEADER_BODY,
  SETTINGS_DISPLAY_HEADER_TITLE,
} from "@features/settings/constants/settingsConstants";

/**
 * Display settings tab: lets the user adjust the portal-wide font size.
 *
 * @returns {JSX.Element} The component.
 */
export default function SettingsDisplay(): JSX.Element {
  const { fontSize, setFontSizeOption } = useFontSize();
  const isDarkMode = useDarkMode();
  const groupRef = useRef<HTMLDivElement>(null);

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      <Paper
        sx={{
          p: 2.5,
          bgcolor: alpha(colors.orange[500], 0.06),
          border: "1px solid",
          borderColor: alpha(colors.orange[500], 0.2),
        }}
      >
        <Box sx={{ display: "flex", alignItems: "flex-start", gap: 2 }}>
          <Paper
            sx={{
              width: 40,
              height: 40,
              bgcolor: alpha(colors.orange[500], 0.12),
              color: colors.orange[700],
              display: "flex",
              alignItems: "center",
              justifyContent: "center",
              flexShrink: 0,
              border: "none",
            }}
          >
            <Type size={20} color={colors.orange[600]} />
          </Paper>
          <Box sx={{ flex: 1 }}>
            <Typography variant="h6" color="text.primary" sx={{ mb: 0.5 }}>
              {SETTINGS_DISPLAY_HEADER_TITLE}
            </Typography>
            <Typography variant="body2" color="text.secondary">
              {SETTINGS_DISPLAY_HEADER_BODY}
            </Typography>
          </Box>
        </Box>
      </Paper>

      <Box>
        <Box sx={{ mb: 2 }}>
          <Typography variant="h6" color="text.primary" sx={{ mb: 0.5 }}>
            {SETTINGS_DISPLAY_FONT_SIZE_TITLE}
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {SETTINGS_DISPLAY_FONT_SIZE_DESCRIPTION}
          </Typography>
        </Box>

        <Box
          ref={groupRef}
          role="radiogroup"
          aria-label={SETTINGS_DISPLAY_FONT_SIZE_TITLE}
          sx={{
            display: "grid",
            gridTemplateColumns: {
              xs: "1fr 1fr",
              sm: "repeat(4, 1fr)",
            },
            gap: 1.5,
          }}
        >
          {SETTINGS_DISPLAY_FONT_SIZE_OPTIONS.map((option, index) => {
            const isSelected = fontSize === option.id;
            return (
              <Paper
                key={option.id}
                role="radio"
                tabIndex={isSelected ? 0 : -1}
                aria-checked={isSelected}
                aria-label={`Font size: ${option.label}`}
                onClick={() => setFontSizeOption(option.id as FontSizeOption)}
                onKeyDown={(e) => {
                  const opts = SETTINGS_DISPLAY_FONT_SIZE_OPTIONS;
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    setFontSizeOption(option.id as FontSizeOption);
                  } else if (
                    e.key === "ArrowRight" ||
                    e.key === "ArrowDown"
                  ) {
                    e.preventDefault();
                    const next = (index + 1) % opts.length;
                    setFontSizeOption(opts[next].id as FontSizeOption);
                    const radios = groupRef.current?.querySelectorAll<HTMLElement>('[role="radio"]');
                    radios?.[next]?.focus();
                  } else if (
                    e.key === "ArrowLeft" ||
                    e.key === "ArrowUp"
                  ) {
                    e.preventDefault();
                    const prev = (index - 1 + opts.length) % opts.length;
                    setFontSizeOption(opts[prev].id as FontSizeOption);
                    const radios = groupRef.current?.querySelectorAll<HTMLElement>('[role="radio"]');
                    radios?.[prev]?.focus();
                  }
                }}
                sx={{
                  p: 2,
                  cursor: "pointer",
                  border: "2px solid",
                  borderColor: isSelected
                    ? colors.orange[500]
                    : isDarkMode
                      ? alpha(colors.grey[600], 0.3)
                      : alpha(colors.grey[400], 0.4),
                  bgcolor: isSelected
                    ? alpha(colors.orange[500], 0.06)
                    : "background.paper",
                  display: "flex",
                  flexDirection: "column",
                  alignItems: "center",
                  gap: 1,
                  transition: "border-color 0.15s ease, background-color 0.15s ease",
                  userSelect: "none",
                  "&:hover": {
                    borderColor: isSelected
                      ? colors.orange[600]
                      : colors.orange[300],
                    bgcolor: isSelected
                      ? alpha(colors.orange[500], 0.1)
                      : alpha(colors.orange[500], 0.03),
                  },
                  "&:focus-visible": {
                    outline: `2px solid ${colors.orange[500]}`,
                    outlineOffset: 2,
                  },
                }}
              >
                <Typography
                  sx={{
                    fontSize: option.size,
                    fontWeight: 600,
                    lineHeight: 1,
                    color: isSelected ? colors.orange[600] : "text.secondary",
                  }}
                >
                  Aa
                </Typography>
                <Typography
                  variant="caption"
                  sx={{
                    color: isSelected ? colors.orange[700] : "text.secondary",
                    fontWeight: isSelected ? 600 : 400,
                  }}
                >
                  {option.label}
                </Typography>
                <Typography
                  variant="caption"
                  sx={{
                    color: "text.disabled",
                    fontSize: "0.65rem",
                  }}
                >
                  {option.description}
                </Typography>
              </Paper>
            );
          })}
        </Box>
      </Box>
    </Box>
  );
}
