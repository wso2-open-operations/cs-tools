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

// Consolidates the source app's ErrorPage/ItemNotFound/NoResults/NoDataAvailable
// inline-state components into one parameterized component, same as
// one-wso2's own plain-MUI port of this file — rebuilt on @wso2/oxygen-ui
// here to match csm-portal's design system.
import { Box, LinearProgress, Paper, Stack, Typography } from "@wso2/oxygen-ui";
import { AlertCircleIcon, SearchXIcon, type LucideIcon } from "@wso2/oxygen-ui-icons-react";
import { alpha, useColorScheme } from "@mui/material/styles";

export function InlineStatePanel({
  icon: Icon,
  title,
  subtitle,
  fullHeight = true,
}: {
  icon: LucideIcon;
  title: string;
  subtitle: string;
  fullHeight?: boolean;
}) {
  return (
    <Paper
      elevation={1}
      sx={{ height: fullHeight ? "100%" : undefined, py: 6, display: "flex", justifyContent: "center" }}
    >
      <Stack direction="column" spacing={2} justifyContent="center" alignItems="center">
        <Box sx={{ color: "text.disabled" }}>
          <Icon size={80} />
        </Box>
        <Typography align="center" variant="h5">
          {title}
        </Typography>
        <Typography align="center" variant="body1" color="text.secondary">
          {subtitle}
        </Typography>
      </Stack>
    </Paper>
  );
}

export function ErrorPanel() {
  return <InlineStatePanel icon={AlertCircleIcon} title="Error!" subtitle="Something went wrong." />;
}

export function NotFoundPanel() {
  return (
    <InlineStatePanel icon={SearchXIcon} title="Item Not Found!" subtitle="Couldn't find the requested item." />
  );
}

export function NoResultsPanel() {
  return (
    <Box sx={{ p: 3, textAlign: "center" }}>
      <Typography variant="body2" color="text.secondary">
        No results found.
      </Typography>
    </Box>
  );
}

export function NoDataPanel({
  message = "No data available",
  description = "There are no items to display. Try adjusting your filters or check back later.",
}: {
  message?: string;
  description?: string;
}) {
  return (
    <Box sx={{ p: 4, textAlign: "center" }}>
      <Typography variant="subtitle1">{message}</Typography>
      <Typography variant="body2" color="text.secondary">
        {description}
      </Typography>
    </Box>
  );
}

export function LinearLoadingPanel() {
  // theme.palette.mode is not live under oxygen-ui's CSS-variables theme
  // (extendTheme()) — confirmed empirically. useColorScheme() is the hook
  // that actually tracks the live scheme.
  const { mode: colorMode, systemMode } = useColorScheme();
  const isDark = (colorMode === "system" ? systemMode : colorMode) === "dark";
  // "#ffd1bf" (a light peach) was a fixed light-canvas track color — an
  // alpha-blended version of the same orange brand accent reads correctly
  // against a dark track background too, same pattern as the hover tints
  // elsewhere in this domain (CaseStateCard, DefaultTable, SearchResultBox).
  const trackBg = alpha("#ff7300", isDark ? 0.24 : 0.2);
  return (
    <Box sx={{ display: "flex", flexDirection: "column", alignItems: "center" }}>
      <Box sx={{ mt: 2.5, width: "25%" }}>
        <LinearProgress
          sx={{
            width: "100%",
            backgroundColor: trackBg,
            "& .MuiLinearProgress-barColorPrimary": { backgroundColor: "#ff7300" },
          }}
        />
      </Box>
      <Typography variant="body1" color="text.secondary" sx={{ mt: -1 }}>
        Loading...
      </Typography>
    </Box>
  );
}
