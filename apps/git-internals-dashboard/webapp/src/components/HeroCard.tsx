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

import { Box } from "@mui/material";
import { SparkLine } from "@components/SparkLine";
import { acrylicSurfaceSx } from "@lib/surfaces";

const MONO = "var(--font-mono)";

interface HeroCardProps {
  label: string; // "Violated" | "At risk"
  n: number;
  delta: number; // today - yesterday
  spark: number[];
  accent: string; // CSS color when non-zero
  onClick?: () => void;
}

// Delta semantics: more violations/at-risk than yesterday is bad (red up); fewer is good (green down).
function Delta({ delta }: { delta: number }) {
  if (delta === 0) {
    return (
      <Box component="span" sx={{ fontWeight: 600, color: "var(--sla-fg3)" }}>
        • 0
      </Box>
    );
  }
  const up = delta > 0;
  return (
    <Box component="span" sx={{ fontWeight: 600, color: up ? "var(--sla-violated)" : "var(--sla-ok)" }}>
      {up ? "▲" : "▼"} {up ? "+" : ""}
      {delta}
    </Box>
  );
}

/** One hero-bar stat tile: a big number, its spark line, and yesterday's delta. */
export function HeroCard({ label, n, delta, spark, accent, onClick }: HeroCardProps) {
  const isZero = n === 0;
  const numColor = isZero ? "var(--sla-ok)" : accent;
  const topBorder = isZero ? "var(--sla-ok)" : accent;

  return (
    <Box
      sx={{
        ...acrylicSurfaceSx,
        borderRadius: "16px",
        border: "1px solid var(--sla-border)",
        p: "18px",
        boxShadow: "0 1px 2px rgba(17,24,39,.04)",
        borderTopWidth: 3,
        borderTopColor: topBorder,
        borderTopStyle: "solid",
      }}
    >
      <Box component="span" sx={{ whiteSpace: "nowrap", fontSize: 13, fontWeight: 600, color: "var(--sla-fg2)" }}>
        {label}
      </Box>

      <Box sx={{ mt: 1, display: "flex", alignItems: "flex-end", justifyContent: "space-between" }}>
        <Box sx={{ display: "flex", alignItems: "baseline", gap: "10px" }}>
          <Box
            component={onClick ? "button" : "span"}
            type={onClick ? "button" : undefined}
            onClick={onClick}
            title={onClick ? `View ${label.toLowerCase()} issues` : undefined}
            sx={{
              lineHeight: 0.9,
              letterSpacing: "-0.02em",
              transition: "opacity 0.15s",
              fontFamily: MONO,
              fontSize: 52,
              fontWeight: 600,
              color: numColor,
              background: "none",
              border: "none",
              p: 0,
              ...(onClick && { cursor: "pointer", "&:hover": { opacity: 0.6 } }),
            }}
          >
            {n}
          </Box>
          {isZero && (
            <Box
              component="span"
              sx={{ display: "inline-flex", alignItems: "center", gap: 0.5, fontSize: 12, fontWeight: 600, color: "var(--sla-ok)" }}
            >
              ✓ at target
            </Box>
          )}
        </Box>
        <SparkLine points={spark} color={numColor} />
      </Box>

      <Box sx={{ mt: 1, display: "flex", alignItems: "center", gap: 0.75, fontSize: 12, color: "var(--sla-fg3)" }}>
        <Delta delta={delta} />
        <span>vs. yesterday</span>
      </Box>
    </Box>
  );
}

interface CsHeroCardProps {
  n: number;
  /** Display name of the CS-side status the count is for (e.g. "Waiting on CS Team"). */
  label: string;
  onDrill?: () => void;
}

/** The hero bar's CS-team-side tile: the count of issues waiting on the CS team (WOC). */
export function CsHeroCard({ n, label, onDrill }: CsHeroCardProps) {
  const isZero = n === 0;
  const accent = isZero ? "var(--sla-ok)" : "var(--sla-cs)";
  const chipTint = isZero ? "var(--sla-ok-tint)" : "var(--sla-cs-tint)";

  return (
    <Box
      sx={{
        ...acrylicSurfaceSx,
        height: "100%",
        borderRadius: "16px",
        border: "1px solid var(--sla-border)",
        p: "18px",
        boxShadow: "0 1px 2px rgba(17,24,39,.04)",
        borderTopWidth: 3,
        borderTopColor: accent,
        borderTopStyle: "solid",
      }}
    >
      <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
        <Box component="span" sx={{ whiteSpace: "nowrap", fontSize: 13, fontWeight: 600, color: "var(--sla-fg2)" }}>
          On CS Team Side
        </Box>
        <Box
          component="span"
          sx={{
            borderRadius: "6px",
            px: "7px",
            py: "3px",
            fontSize: 10,
            fontWeight: 600,
            textTransform: "uppercase",
            letterSpacing: "0.05em",
            fontFamily: MONO,
            bgcolor: chipTint,
            color: accent,
          }}
        >
          POINT-IN-TIME
        </Box>
      </Box>

      <Box
        component="button"
        type="button"
        onClick={() => onDrill?.()}
        title={`View ${label} issues`}
        sx={{
          mt: 2,
          display: "block",
          textAlign: "left",
          transition: "opacity 0.15s",
          background: "none",
          border: "none",
          cursor: "pointer",
          p: 0,
          "&:hover": { opacity: 0.6 },
        }}
      >
        <Box sx={{ display: "flex", alignItems: "center", gap: "10px" }}>
          <Box sx={{ lineHeight: 0.85, letterSpacing: "-0.02em", fontFamily: MONO, fontSize: 44, fontWeight: 600, color: "var(--sla-cs)" }}>
            {n}
          </Box>
          {isZero && (
            <Box component="span" sx={{ fontSize: 12, fontWeight: 600, color: "var(--sla-ok)" }}>
              ✓
            </Box>
          )}
        </Box>
        <Box sx={{ mt: 1, display: "flex", alignItems: "center", gap: 0.75, fontSize: 12, color: "var(--sla-fg2)" }}>
          <Box component="span" sx={{ height: 10, width: 10, borderRadius: "2px", bgcolor: "var(--sla-cs)" }} />
          {label}
        </Box>
      </Box>
    </Box>
  );
}
