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

import { Box, Typography } from "@wso2/oxygen-ui";
import type { JSX, ReactNode } from "react";
import type { LucideIcon } from "@wso2/oxygen-ui-icons-react";
import type { WallboardSection } from "@features/csm-dashboard/utils/wallboardMetricStyle";

interface PanelAccent {
  iconColor: string;
  chipBg: string;
  chipBorder: string;
  lineColor: string;
}

// Ported from `digiops-cs`'s `Wallboard.tsx`: each of the four panels has
// its own fixed accent color, independent of the emphasis colors used
// inside its own stat tiles (`wallboardMetricStyle.ts`) — e.g. the SRE
// panel's own icon/border is `emerald`, a color that never appears as a
// tile emphasis anywhere.
const PANEL_ACCENTS: Record<WallboardSection, PanelAccent> = {
  cre: { iconColor: "#22d3ee", chipBg: "rgba(22,78,99,0.5)", chipBorder: "#0e7490", lineColor: "rgba(34,211,238,0.4)" },
  sre: { iconColor: "#34d399", chipBg: "rgba(6,78,59,0.5)", chipBorder: "#047857", lineColor: "rgba(52,211,153,0.4)" },
  security: { iconColor: "#fb7185", chipBg: "rgba(136,19,55,0.5)", chipBorder: "#be123c", lineColor: "rgba(251,113,133,0.4)" },
  fde: { iconColor: "#c084fc", chipBg: "rgba(88,28,135,0.5)", chipBorder: "#7e22ce", lineColor: "rgba(192,132,252,0.4)" },
};

export interface WallboardPanelProps {
  section: WallboardSection;
  title: string;
  icon: LucideIcon;
  children: ReactNode;
}

/**
 * One of the four CS Overview panels (CRE / SRE / Security Report / FDE).
 * Matches the original's own panel chrome: `bg-gray-800` card,
 * `border-gray-700`, `rounded-2xl` (16px), a thin gradient accent line
 * along the top edge, and an icon chip + title header row.
 */
export default function WallboardPanel({ section, title, icon: Icon, children }: WallboardPanelProps): JSX.Element {
  const accent = PANEL_ACCENTS[section];

  return (
    <Box
      sx={{
        position: "relative",
        overflow: "hidden",
        bgcolor: "#1f2937",
        border: "1px solid #374151",
        borderRadius: "16px",
        p: "12px",
        display: "flex",
        flexDirection: "column",
        height: "100%",
        boxShadow: "0 10px 15px -3px rgba(0,0,0,0.3)",
      }}
    >
      <Box
        sx={{
          position: "absolute",
          top: 0,
          left: 0,
          right: 0,
          height: "4px",
          background: `linear-gradient(to right, transparent, ${accent.lineColor}, transparent)`,
        }}
      />
      <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, mb: 1.5, position: "relative", zIndex: 1 }}>
        <Box
          sx={{
            display: "flex",
            p: "6px",
            bgcolor: accent.chipBg,
            borderRadius: "8px",
            border: "1px solid",
            borderColor: accent.chipBorder,
          }}
        >
          <Icon color={accent.iconColor} size={16} />
        </Box>
        <Typography sx={{ fontSize: "0.95rem", fontWeight: 700, color: "#fff", letterSpacing: "0.01em" }}>
          {title}
        </Typography>
      </Box>
      <Box sx={{ position: "relative", zIndex: 1, flex: 1, minHeight: 0, display: "flex", flexDirection: "column" }}>
        {children}
      </Box>
    </Box>
  );
}
