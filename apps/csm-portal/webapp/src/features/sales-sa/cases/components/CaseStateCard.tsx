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
// features/spl/cases/components/CaseStateCard.tsx, then restyled to match
// CSM Portal's own dashboard widget tile (DashboardWidgetTile.tsx's
// shape==="count" body: an icon circle, a caption label and a large plain
// count, on an outlined card with a primary-tinted hover) instead of a
// solid-grey box with a per-state text color -- that's this card's own
// previous style here, replaced during the SPL port; this restores it.
// Deliberately NOT importing DashboardWidgetTile itself: that component
// fetches its own data from a backend-configurable widget registry
// (useWidgetData, resourceType/filters), which doesn't exist for SPL's six
// fixed case states -- only its visual shape is reused, not its data layer
// or its click-through-to-a-different-page behavior (this card still
// toggles CasesPage's own inline state view, same as before).
import { Box, Card, CardActionArea, CircularProgress, Typography } from "@wso2/oxygen-ui";
import { Briefcase, CircleAlertIcon } from "@wso2/oxygen-ui-icons-react";
import { alpha, useTheme } from "@mui/material/styles";
import type { CaseDetailsWithCount } from "../api/caseTypes";

export default function CaseStateCard({
  state,
  data,
  loading,
  error,
  setCaseState,
}: {
  state: string;
  data: CaseDetailsWithCount | undefined;
  loading: boolean;
  error: Error | null | undefined;
  setCaseState: (state: string) => void;
}) {
  const theme = useTheme();

  return (
    <Card
      variant="outlined"
      sx={{
        width: 290,
        // Same hover identity as DashboardWidgetTile's own count-shape tile
        // (widgetHoverSx there): a transparent border that tints on hover,
        // plus a small lift -- no boxShadow.
        border: "1px solid transparent",
        transition: "border-color 0.2s ease, background-color 0.2s ease, transform 0.15s ease",
        "&:hover": {
          borderColor: theme.palette.primary.main,
          bgcolor: alpha(theme.palette.primary.main, 0.06),
          transform: "translateY(-1px)",
        },
      }}
    >
      <CardActionArea
        onClick={() => setCaseState(state)}
        disabled={loading || !!error}
        sx={{ p: 1.75, height: "100%" }}
      >
        {loading ? (
          <Box sx={{ display: "flex", justifyContent: "center", py: 2 }}>
            <CircularProgress color="primary" />
          </Box>
        ) : error ? (
          <Box sx={{ display: "flex", justifyContent: "center", py: 2 }}>
            <CircleAlertIcon size={36} color={theme.palette.error.main} />
          </Box>
        ) : (
          data && (
            <Box sx={{ display: "flex", alignItems: "flex-start", gap: 1.25 }}>
              <Box
                sx={{
                  p: 0.75,
                  mt: 0.25,
                  borderRadius: "50%",
                  bgcolor: alpha(theme.palette.primary.light, 0.1),
                  color: theme.palette.primary.light,
                  display: "flex",
                  alignItems: "center",
                  justifyContent: "center",
                  flexShrink: 0,
                }}
              >
                <Briefcase size={16} />
              </Box>
              <Box sx={{ minWidth: 0, flex: 1 }}>
                <Typography variant="caption" color="text.secondary" noWrap>
                  {state}
                </Typography>
                <Typography
                  noWrap
                  sx={{ mt: 0.5, lineHeight: 1.1, fontWeight: 400, fontSize: "3.25rem" }}
                >
                  {data.count.toLocaleString()}
                </Typography>
              </Box>
            </Box>
          )
        )}
      </CardActionArea>
    </Card>
  );
}
