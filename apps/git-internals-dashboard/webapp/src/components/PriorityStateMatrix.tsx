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
import type { Overview, MatrixRow } from "@api/types";
import { acrylicSurfaceSx } from "@lib/surfaces";

const MONO = "var(--font-mono)";

// CSS-var references (theme-following, like every other accent in the app) —
// safe here because these only ever feed a plain `bgcolor`, never rgba() math.
const P_ACCENT: Record<string, string> = {
  P1: "var(--sla-p1)",
  P2: "var(--sla-p2)",
  P3: "var(--sla-p3)",
  P4: "var(--sla-p4)",
};

// column key -> { label, drill bucket, hex color }. Literal hex (not a CSS
// var) is required here: `rgba()` below parses it for the heat-map alpha
// blend, which `var(--sla-*)` strings can't do. Kept in sync with the
// AcrylicPurpleTheme values in theme/global.css (error/warning/success/primary).
// The first PRODUCT_SIDE_COL_COUNT columns are the mutually exclusive
// "On product team side" group this widget borders and titles below; `cs`
// sits outside that group, separated by a gutter.
const COLS = [
  { key: "violated", label: "Violated", bucket: "violated", hex: "#d32f2f" },
  { key: "atRisk", label: "At risk", bucket: "at_risk", hex: "#ed6c02" },
  { key: "onTrack", label: "On track", bucket: "on_track", hex: "#2e7d32" },
  { key: "cs", label: "CS side", bucket: "cs", hex: "#646cff" },
] as const;

const PRODUCT_SIDE_COL_COUNT = 3;

type ColKey = (typeof COLS)[number]["key"];

/** Converts a "#rrggbb" hex color plus alpha into an `rgba(...)` string. */
function rgba(hex: string, a: number): string {
  const n = parseInt(hex.slice(1), 16);
  return `rgba(${(n >> 16) & 255},${(n >> 8) & 255},${n & 255},${a})`;
}

// ---------------------------------------------------------------------------
// Grid layout
//
// Column tracks (1-based grid lines):
//   1      priority badge (+ trailing space before the group)
//   2      product group inner pad (left)   ─┐
//   3–5    violated / at risk / on track     │ bordered group
//   6      product group inner pad (right)  ─┘
//   7      CS side
//   8      total
//
// The two pad tracks give the tiles breathing room inside the border (pad +
// column gap); the CS column sits right after, one ordinary column-gap clear
// of the border. Using real tracks instead of negative margins keeps every
// distance deterministic and lets clicks on the tiles work unaffected.
// ---------------------------------------------------------------------------
const GAP_PX = 6;
const GROUP_PAD_PX = 2; // + GAP_PX = 8px between border and tiles on every side
const BADGE_WIDTH_PX = 64;
const LABEL_TRACK_PX = 70; // badge + 6px extra space before the group

const GRID_COLUMNS = [
  `${LABEL_TRACK_PX}px`,
  `${GROUP_PAD_PX}px`,
  `repeat(${PRODUCT_SIDE_COL_COUNT}, minmax(0, 1fr))`,
  `${GROUP_PAD_PX}px`,
  "minmax(0, 1fr)",
  "58px",
].join(" ");

const LABEL_COL = 1;
const GROUP_COL_START = 2; // left pad track
const GROUP_COL_END = 7; // exclusive: line after the right pad track
const FIRST_PRODUCT_COL = 3;
const CS_COL = 7;
const TOTAL_COL = 8;

/** Grid column for the i-th entry of COLS. */
function colFor(i: number): number {
  return i < PRODUCT_SIDE_COL_COUNT ? FIRST_PRODUCT_COL + i : CS_COL + (i - PRODUCT_SIDE_COL_COUNT);
}

// Grid rows, referenced explicitly (rather than left to auto-flow) so the
// group border can span every row without the auto-placement algorithm
// routing other cells around it. A trailing pad row gives the border the same
// inner spacing at the bottom as it has on the sides.
const TITLE_ROW = 1;
const HEADER_ROW = 2;

// Border matches the "On Product Team Side" title's own color (at reduced
// opacity, so the outline reads as subtle rather than a hard rule), so the
// label and the outline it captions read as one visual unit.
const GROUP_BORDER = "1px solid color-mix(in srgb, var(--sla-fg3) 25%, transparent)";
const GROUP_FILL = "color-mix(in srgb, var(--sla-fg3) 4%, transparent)";

interface MatrixProps {
  matrix: Overview["matrix"];
  onDrill: (bucket: string, priority?: string) => void;
}

/** Priority × outcome heat-map grid, each cell drillable into its filtered issue list. */
export function PriorityStateMatrix({ matrix, onDrill }: MatrixProps) {
  // Per-column max across the priority rows drives the heat-map intensity.
  const colMax: Record<ColKey, number> = { violated: 1, atRisk: 1, onTrack: 1, cs: 1 };
  for (const c of COLS) {
    colMax[c.key] = Math.max(1, ...matrix.rows.map((r) => r.cells[c.key]));
  }

  const allRow = HEADER_ROW + 1 + matrix.rows.length;
  const padRow = allRow + 1;

  return (
    <Box sx={{ ...acrylicSurfaceSx, borderRadius: "16px", border: "1px solid var(--sla-border)", px: "22px", py: 2.5, boxShadow: "0 1px 2px rgba(17,24,39,.04)" }}>
      <Box component="h2" sx={{ m: 0, fontSize: 15, fontWeight: 600, lineHeight: 1.2, letterSpacing: "-0.01em" }}>
        Priority × SLA state
      </Box>
      <Box sx={{ mb: 2, mt: 0.25, fontSize: 12.5, color: "var(--sla-fg3)" }}>
        Where the load sits. Darker cell = more issues.
      </Box>

      <Box
        sx={{
          display: "grid",
          alignItems: "center",
          gap: `${GAP_PX}px`,
          gridTemplateColumns: GRID_COLUMNS,
          gridTemplateRows: `repeat(${allRow}, auto) ${GROUP_PAD_PX}px`,
        }}
      >
        {/* Border + faint fill around the product-side trio (violated / at
            risk / on track). `alignSelf: stretch` is essential: the grid
            centers items vertically, and an empty box that isn't stretched
            collapses to a zero-height line through the middle row. Purely
            visual, so clicks fall through to the tiles it sits behind. */}
        <Box
          aria-hidden
          sx={{
            gridColumn: `${GROUP_COL_START} / ${GROUP_COL_END}`,
            gridRow: `${TITLE_ROW} / ${padRow + 1}`,
            alignSelf: "stretch",
            justifySelf: "stretch",
            pointerEvents: "none",
            border: GROUP_BORDER,
            backgroundColor: GROUP_FILL,
            borderRadius: "12px",
          }}
        />

        {/* product-side group title */}
        <Box
          sx={{
            gridColumn: `${FIRST_PRODUCT_COL} / ${FIRST_PRODUCT_COL + PRODUCT_SIDE_COL_COUNT}`,
            gridRow: TITLE_ROW,
            pt: "8px",
            textAlign: "center",
            fontSize: 10.5,
            fontWeight: 600,
            lineHeight: 1.2,
            color: "var(--sla-fg3)",
          }}
        >
          On Product Team Side
        </Box>

        {/* header row */}
        {COLS.map((c, i) => (
          <Box key={c.key} sx={{ gridColumn: colFor(i), gridRow: HEADER_ROW, textAlign: "center", fontSize: 10.5, fontWeight: 600, lineHeight: 1.2, color: "var(--sla-fg3)" }}>
            {c.label}
          </Box>
        ))}
        <Box sx={{ gridColumn: TOTAL_COL, gridRow: HEADER_ROW, textAlign: "center", fontSize: 10.5, fontWeight: 600, lineHeight: 1.2, color: "var(--sla-fg2)" }}>Total</Box>

        {/* priority rows */}
        {matrix.rows.map((row, i) => (
          <Row key={row.key} row={row} colMax={colMax} onDrill={onDrill} gridRow={HEADER_ROW + 1 + i} />
        ))}

        {/* All row */}
        <Box sx={{ gridColumn: LABEL_COL, gridRow: allRow, width: BADGE_WIDTH_PX, justifySelf: "start", pt: 0.5, textAlign: "center", fontSize: 10.5, fontWeight: 600, lineHeight: 1, color: "var(--sla-fg3)" }}>All</Box>
        {COLS.map((c, i) => {
          const n = matrix.totals[c.key];
          return (
            <Box
              key={c.key}
              component="button"
              type="button"
              onClick={() => onDrill(c.bucket)}
              sx={{
                gridColumn: colFor(i),
                gridRow: allRow,
                pt: 0.5,
                textAlign: "center",
                lineHeight: 1,
                transition: "opacity 0.15s",
                fontFamily: MONO,
                fontSize: 14,
                fontWeight: 600,
                color: n > 0 ? c.hex : "var(--sla-fg3)",
                background: "none",
                border: "none",
                cursor: "pointer",
                "&:hover": { opacity: 0.6 },
              }}
            >
              {n}
            </Box>
          );
        })}
        <Box
          component="button"
          type="button"
          onClick={() => onDrill("tracked")}
          sx={{ gridColumn: TOTAL_COL, gridRow: allRow, pt: 0.5, textAlign: "center", lineHeight: 1, color: "var(--sla-fg)", transition: "opacity 0.15s", fontFamily: MONO, fontSize: 14, fontWeight: 600, background: "none", border: "none", cursor: "pointer", "&:hover": { opacity: 0.6 } }}
        >
          {matrix.grandTotal}
        </Box>
      </Box>
    </Box>
  );
}

/** One priority tier's row of heat-mapped, drillable outcome cells. */
function Row({
  row,
  colMax,
  onDrill,
  gridRow,
}: {
  row: MatrixRow;
  colMax: Record<ColKey, number>;
  onDrill: (bucket: string, priority?: string) => void;
  gridRow: number;
}) {
  const accent = P_ACCENT[row.code] ?? "var(--sla-no-sla)";
  return (
    <>
      {/* White text is a deliberate exception: the badge's background
          rotates across 4 accent hues (one per priority), and no single
          Oxygen UI contrastText token covers all 4 — contrast is weakest on
          the P4/grey tile. See --sla-contrast-text for the single-hue case
          (used elsewhere) where a real token does apply. */}
      <Box component="span" sx={{ gridColumn: LABEL_COL, gridRow, width: BADGE_WIDTH_PX, justifySelf: "start", borderRadius: "6px", py: 0.75, textAlign: "center", fontSize: 12, fontWeight: 700, lineHeight: 1, color: "#fff", fontFamily: MONO, bgcolor: accent }}>
        {row.code}
      </Box>
      {COLS.map((c, i) => {
        const n = row.cells[c.key];
        const a = n > 0 ? 0.1 + 0.55 * (n / colMax[c.key]) : 0;
        return (
          <Box
            key={c.key}
            component="button"
            type="button"
            onClick={() => onDrill(c.bucket, row.key)}
            sx={{
              gridColumn: colFor(i),
              gridRow,
              borderRadius: "8px",
              py: "11px",
              textAlign: "center",
              lineHeight: 1,
              transition: "opacity 0.15s",
              fontFamily: MONO,
              fontSize: 15,
              fontWeight: 600,
              background: n > 0 ? rgba(c.hex, a) : "var(--sla-surface-muted)",
              color: n > 0 ? c.hex : "var(--sla-fg3)",
              border: "none",
              cursor: "pointer",
              "&:hover": { opacity: 0.7 },
            }}
          >
            {n}
          </Box>
        );
      })}
      <Box
        component="button"
        type="button"
        onClick={() => onDrill("tracked", row.key)}
        sx={{ gridColumn: TOTAL_COL, gridRow, textAlign: "center", lineHeight: 1, color: "var(--sla-fg)", transition: "opacity 0.15s", fontFamily: MONO, fontSize: 15, fontWeight: 600, background: "none", border: "none", cursor: "pointer", "&:hover": { opacity: 0.6 } }}
      >
        {row.total}
      </Box>
    </>
  );
}
