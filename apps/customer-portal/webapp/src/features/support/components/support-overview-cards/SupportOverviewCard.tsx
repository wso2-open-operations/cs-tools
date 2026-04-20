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
import type { JSX } from "react";
import ErrorIndicator from "@components/error-indicator/ErrorIndicator";
import {
  SupportOverviewIconVariant,
  type SupportOverviewCardProps,
} from "@features/support/types/supportOverview";

function resolveSupportOverviewPaletteKey(
  iconVariant: SupportOverviewIconVariant | undefined,
): "info" | "warning" {
  switch (iconVariant ?? SupportOverviewIconVariant.Orange) {
    case SupportOverviewIconVariant.Blue:
      return "info";
    case SupportOverviewIconVariant.Orange:
    default:
      return "warning";
  }
}

function resolveSupportOverviewIconHex(
  iconVariant: SupportOverviewIconVariant | undefined,
): string {
  switch (iconVariant ?? SupportOverviewIconVariant.Orange) {
    case SupportOverviewIconVariant.Blue:
      return colors.blue[600];
    case SupportOverviewIconVariant.Orange:
    default:
      return colors.orange[600];
  }
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
  iconVariant = SupportOverviewIconVariant.Orange,
  children,
  footerButtonLabel,
  onFooterClick,
  footerButtons,
  sx,
  isError,
  headerAction,
}: SupportOverviewCardProps): JSX.Element {
  const theme = useTheme();
  const paletteKey = resolveSupportOverviewPaletteKey(iconVariant);
  const iconColor = resolveSupportOverviewIconHex(iconVariant);

  return (
    <Paper
      sx={[
        {
          p: 2.5,
          display: "flex",
          flexDirection: "column",
          gap: 1.5,
          width: "100%",
          minWidth: 0,
          overflow: "hidden",
        },
        ...(Array.isArray(sx) ? sx : [sx]),
      ]}
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
        <Box sx={{ minWidth: 0, flex: 1 }}>
          <Typography variant="h6" color="text.primary">
            {title}
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {subtitle}
          </Typography>
        </Box>
        {headerAction && (
          <Button
            variant="outlined"
            color="warning"
            onClick={headerAction.onClick}
            sx={{ textTransform: "none", flexShrink: 0 }}
          >
            {headerAction.label}
          </Button>
        )}
      </Box>

      <Box
        sx={{
          display: "flex",
          flexDirection: "column",
          gap: 1.5,
          width: "100%",
          justifyContent: isError ? "center" : "flex-start",
          alignItems: isError ? "center" : "stretch",
        }}
      >
        {isError ? (
          <ErrorIndicator entityName={title.toLowerCase()} size="medium" />
        ) : (
          children
        )}
      </Box>

      <Box
        sx={{
          borderTop: 1,
          borderColor: "divider",
        }}
      />
      {footerButtons && footerButtons.length > 0 ? (
        <Box
          sx={{
            display: "flex",
            border: 1,
            borderColor: "divider",
            borderRadius: 1,
            overflow: "hidden",
          }}
        >
          {footerButtons.map((btn, index) => (
            <Button
              key={index}
              fullWidth
              variant="text"
              color="warning"
              onClick={btn.onClick}
              endIcon={<ArrowRight size={16} />}
              sx={{
                flex: 1,
                justifyContent: "flex-start",
                textTransform: "none",
                fontWeight: 500,
                borderRadius: 0,
                borderRight: index < footerButtons.length - 1 ? 1 : 0,
                borderColor: "divider",
                "&:hover": {
                  bgcolor: alpha(colors.orange[50], 0.5),
                },
              }}
            >
              {btn.label}
            </Button>
          ))}
        </Box>
      ) : (
        <Button
          fullWidth
          variant="text"
          color="warning"
          onClick={onFooterClick}
          endIcon={<ArrowRight size={16} />}
          sx={{
            justifyContent: "flex-start",
            textTransform: "none",
            fontWeight: 500,
          }}
        >
          {footerButtonLabel}
        </Button>
      )}
    </Paper>
  );
}
