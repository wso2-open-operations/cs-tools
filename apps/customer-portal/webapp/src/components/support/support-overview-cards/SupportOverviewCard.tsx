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
  Button,
  Paper,
  Typography,
  alpha,
  colors,
  useTheme,
} from "@wso2/oxygen-ui";
import { ArrowRight } from "@wso2/oxygen-ui-icons-react";
import type { ComponentType } from "react";
import type { JSX } from "react";

export interface SupportOverviewCardProps {
  title: string;
  subtitle: string;
  icon: ComponentType<{ size?: number; color?: string }>;
  iconVariant?: "orange" | "blue";
  children: React.ReactNode;
  footerButtonLabel: string;
  onFooterClick: () => void;
}

/**
 * Generic card for support overview sections (Outstanding Cases, Chat History).
 *
 * @param {SupportOverviewCardProps} props - Title, subtitle, icon, children, footer CTA.
 * @returns {JSX.Element} The rendered card.
 */
export default function SupportOverviewCard({
  title,
  subtitle,
  icon: Icon,
  iconVariant = "orange",
  children,
  footerButtonLabel,
  onFooterClick,
}: SupportOverviewCardProps): JSX.Element {
  const theme = useTheme();
  const paletteKey = iconVariant === "blue" ? "info" : "warning";
  const iconColor =
    iconVariant === "blue" ? colors.blue[600] : colors.orange[600];

  return (
    <Paper
      sx={{
        p: 2.5,
        display: "flex",
        flexDirection: "column",
        gap: 2,
      }}
    >
      <Box sx={{ display: "flex", alignItems: "center", gap: 2, mb: 1 }}>
        <Paper
          sx={{
            width: 40,
            height: 40,
            bgcolor: alpha(theme.palette[paletteKey].light, 0.1),
            color: theme.palette[paletteKey].light,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            flexShrink: 0,
          }}
        >
          <Icon size={20} color={iconColor} />
        </Paper>
        <Box>
          <Typography variant="h6" color="text.primary">
            {title}
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {subtitle}
          </Typography>
        </Box>
      </Box>

      <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }}>
        {children}
      </Box>

      <Box
        sx={{
          borderTop: 1,
          borderColor: "divider",
          pt: 2,
          mt: 1,
        }}
      />
      <Button
        fullWidth
        variant="text"
        color="warning"
        onClick={onFooterClick}
        endIcon={<ArrowRight size={16} />}
        sx={{
          justifyContent: "space-between",
          textTransform: "none",
          fontWeight: 500,
        }}
      >
        {footerButtonLabel}
      </Button>
    </Paper>
  );
}
