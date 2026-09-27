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

// Ported from apps/support-portal-lite/webapp's own
// features/spl/cases/components/CaseStateCard.tsx. Error prop type swapped
// from useSplApi's GetApiResponseError to this app's own React-Query Error.
import { Card, CardActionArea, CardContent, CircularProgress, Typography } from "@wso2/oxygen-ui";
import { alpha, useTheme, useColorScheme } from "@mui/material/styles";
import { CircleAlertIcon } from "@wso2/oxygen-ui-icons-react";
import type { CaseDetailsWithCount } from "../api/splCaseTypes";

export default function CaseStateCard({
  state,
  color,
  data,
  loading,
  error,
  setCaseState,
}: {
  state: string;
  /** A {light, dark} pair, not one static color — see SplCasesPage's COLORS
   *  for why a single hex can't have good contrast against the card's own
   *  background in both modes. */
  color: { light: string; dark: string };
  data: CaseDetailsWithCount | undefined;
  loading: boolean;
  error: Error | null | undefined;
  setCaseState: (state: string) => void;
}) {
  const theme = useTheme();
  // theme.palette.mode is not live under oxygen-ui's CSS-variables theme
  // (extendTheme()) — confirmed empirically. useColorScheme() is the hook
  // that actually tracks the live scheme.
  const { mode: colorMode, systemMode } = useColorScheme();
  const isDark = (colorMode === "system" ? systemMode : colorMode) === "dark";
  const countColor = isDark ? color.dark : color.light;

  return (
    <Card
      sx={{
        width: 290,
        height: 175,
        cursor: "pointer",
        // One uniform, slightly darker neutral for all six cards (regardless
        // of state) instead of the default white/paper background — reads as
        // a deliberate set of tiles against the "Overall Case Summary" panel
        // rather than blending into it. Per-state identity stays in the
        // count's own text color (the `color` prop below), not the card.
        backgroundColor: isDark ? theme.palette.grey[800] : theme.palette.grey[200],
        // Same warm-orange hover identity in both modes — a solid light
        // peach reads fine on a light card but washes out a dark one, so an
        // alpha overlay (which composites against whatever's underneath)
        // stands in for the literal hex the source app used.
        "&:hover": { backgroundColor: alpha("#ff7300", isDark ? 0.24 : 0.35) },
        display: "flex",
        alignContent: "center",
      }}
    >
      <CardActionArea onClick={() => setCaseState(state)} disabled={loading || !!error}>
        <CardContent
          sx={{ display: "flex", justifyContent: "center", flexDirection: !error && !loading ? "column" : undefined }}
        >
          {loading ? (
            <CircularProgress color="primary" />
          ) : error ? (
            <CircleAlertIcon size={36} color={theme.palette.error.main} />
          ) : (
            data && (
              <>
                <Typography align="center" gutterBottom variant="h3" component="div" color={countColor}>
                  {data.count}
                </Typography>
                <Typography align="center" gutterBottom variant="h5" component="div">
                  {state}
                </Typography>
              </>
            )
          )}
        </CardContent>
      </CardActionArea>
    </Card>
  );
}
