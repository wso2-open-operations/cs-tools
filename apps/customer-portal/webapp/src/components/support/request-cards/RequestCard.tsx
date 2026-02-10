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

import type { JSX } from "react";
import {
  Box,
  Button,
  Paper,
  Typography,
  alpha,
  useTheme,
} from "@wso2/oxygen-ui";
import { ArrowRight } from "@wso2/oxygen-ui-icons-react";
import type { ComponentType } from "react";

type PaletteKey = "info" | "secondary" | "warning" | "primary";

export interface RequestCardProps {
  title: string;
  subtitle: string;
  icon: ComponentType<{ size?: number; color?: string }>;
  paletteKey: PaletteKey;
  accentColor: string;
  infoBoxTitle: string;
  infoBoxDescription: string;
  bulletItems: readonly string[];
  secondaryButtonLabel: string;
  onSecondaryClick: () => void;
  primaryButton?: {
    label: string;
    onClick: () => void;
    icon: ComponentType<{ size?: number }>;
  };
}

/**
 * Generic card for Service Requests and Change Requests on the Support page.
 * Uses theme palette for backgrounds via alpha(theme.palette[paletteKey].light, 0.1).
 *
 * @param {RequestCardProps} props - Title, icon, info box content, buttons.
 * @returns {JSX.Element} The rendered card.
 */
export default function RequestCard({
  title,
  subtitle,
  icon: Icon,
  paletteKey,
  accentColor,
  infoBoxTitle,
  infoBoxDescription,
  bulletItems,
  secondaryButtonLabel,
  onSecondaryClick,
  primaryButton,
}: RequestCardProps): JSX.Element {
  const theme = useTheme();
  const palette = theme.palette[paletteKey] as {
    light?: string;
    main?: string;
  };

  const iconBgColor = alpha(
    palette.light ?? palette.main ?? theme.palette.grey[300],
    0.1,
  );
  const infoBoxBgColor = alpha(
    palette.light ?? palette.main ?? theme.palette.grey[300],
    0.1,
  );

  return (
    <Paper
      sx={{
        p: 2.5,
        display: "flex",
        flexDirection: "column",
        gap: 2,
        height: "100%",
        justifyContent: "space-between",
      }}
    >
      <Box sx={{ display: "flex", alignItems: "center", gap: 2, mb: 1 }}>
        <Paper
          sx={{
            width: 40,
            height: 40,
            bgcolor: iconBgColor,
            color: accentColor,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            flexShrink: 0,
          }}
        >
          <Icon size={20} color={accentColor} />
        </Paper>
        <Box>
          <Typography variant="h6">{title}</Typography>
          <Typography variant="body2">{subtitle}</Typography>
        </Box>
      </Box>

      <Paper
        sx={{
          bgcolor: infoBoxBgColor,
          p: 2.5,
          flexGrow: 1,
          display: "flex",
          flexDirection: "column",
        }}
      >
        <Typography variant="body1" sx={{ mb: 1, fontWeight: 400 }}>
          {infoBoxTitle}
        </Typography>
        <Typography variant="body2" sx={{ mb: 2 }}>
          {infoBoxDescription}
        </Typography>
        <Box
          component="ul"
          sx={{
            m: 0,
            pl: 2.5,
            display: "flex",
            flexDirection: "column",
            gap: 1,
            listStyle: "none",
          }}
        >
          {bulletItems.map((item) => (
            <Box
              component="li"
              key={item}
              sx={{
                display: "flex",
                alignItems: "center",
                gap: 1,
                typography: "body2",
                color: "text.secondary",
              }}
            >
              <Box
                sx={{
                  width: 6,
                  height: 6,
                  bgcolor: accentColor,
                  borderRadius: "50%",
                  flexShrink: 0,
                }}
              />
              {item}
            </Box>
          ))}
        </Box>
      </Paper>

      <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5, mt: 1 }}>
        {primaryButton && (
          <Button
            fullWidth
            variant="contained"
            color="warning"
            startIcon={<primaryButton.icon size={16} />}
            onClick={primaryButton.onClick}
            sx={{ textTransform: "none" }}
          >
            {primaryButton.label}
          </Button>
        )}
        <Button
          fullWidth
          variant="outlined"
          color="inherit"
          endIcon={<ArrowRight size={16} />}
          onClick={onSecondaryClick}
          sx={{
            textTransform: "none",
            borderColor: "divider",
            color: "text.primary",
          }}
        >
          {secondaryButtonLabel}
        </Button>
      </Box>
    </Paper>
  );
}
