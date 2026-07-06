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

import { Box, Card, Skeleton, Typography } from "@wso2/oxygen-ui";
import { PieChart } from "@wso2/oxygen-ui-charts-react";
import { type JSX } from "react";

// Donut sits in a fixed square; the legend fills the rest of the card width so
// a wide card doesn't leave a big empty band around a small pie.
const DONUT_SIZE = 200;

export interface CompositionSlice {
  id: string;
  name: string;
  value: number;
  color: string;
}

interface CompositionDonutProps {
  title: string;
  description: string;
  slices: CompositionSlice[];
  total: number;
  isLoading: boolean;
  isError: boolean;
  /** Noun for the empty state, e.g. "cases". */
  emptyNoun?: string;
  /**
   * When provided, slices and legend rows become clickable and call this with
   * the slice `id` (e.g. to navigate to a filtered list). Omit for a static
   * chart.
   */
  onSliceClick?: (id: string) => void;
}

/**
 * A donut chart (left) with a value/percentage legend (right) and a centred
 * total, matching the customer-portal dashboard charts. Zero-value slices drop
 * from the ring but stay in the legend so every category remains visible.
 */
export default function CompositionDonut({
  title,
  description,
  slices,
  total,
  isLoading,
  isError,
  emptyNoun = "cases",
  onSliceClick,
}: CompositionDonutProps): JSX.Element {
  const pieData = slices.filter((s) => s.value > 0);
  const isEmpty = !isLoading && !isError && total === 0;

  return (
    <Card variant="outlined" sx={{ height: "100%", p: 2.5 }}>
      <Typography variant="subtitle1" fontWeight={600}>
        {title}
      </Typography>
      <Typography variant="caption" color="text.secondary">
        {description}
      </Typography>

      {isLoading ? (
        <Box sx={{ display: "flex", alignItems: "center", gap: 3, mt: 2 }}>
          <Skeleton variant="circular" width={DONUT_SIZE} height={DONUT_SIZE} sx={{ flexShrink: 0 }} />
          <Box sx={{ flex: 1, display: "flex", flexDirection: "column", gap: 1 }}>
            {slices.map((s) => (
              <Skeleton key={s.id} variant="rounded" height={18} />
            ))}
          </Box>
        </Box>
      ) : isError ? (
        <Box sx={{ height: DONUT_SIZE, display: "flex", alignItems: "center", justifyContent: "center" }}>
          <Typography variant="body2" color="text.secondary">
            Could not load {title.toLowerCase()}.
          </Typography>
        </Box>
      ) : isEmpty ? (
        <Box sx={{ height: DONUT_SIZE, display: "flex", alignItems: "center", justifyContent: "center" }}>
          <Typography variant="body2" color="text.disabled">
            No {emptyNoun} to show.
          </Typography>
        </Box>
      ) : (
        <Box
          sx={{
            mt: 2,
            display: "flex",
            flexDirection: { xs: "column", sm: "row" },
            alignItems: "center",
            gap: 3,
          }}
        >
          <Box
            sx={{
              position: "relative",
              width: DONUT_SIZE,
              height: DONUT_SIZE,
              flexShrink: 0,
              ...(onSliceClick && {
                "& .recharts-pie-sector": { cursor: "pointer" },
              }),
            }}
          >
            <PieChart
              data={pieData}
              colors={pieData.map((s) => s.color)}
              legend={{ show: false }}
              tooltip={{ show: true }}
              width="100%"
              height={DONUT_SIZE}
              pies={[
                {
                  dataKey: "value",
                  nameKey: "name",
                  innerRadius: "62%",
                  outerRadius: "100%",
                  paddingAngle: 1,
                  startAngle: 90,
                  endAngle: -270,
                  label: false,
                  labelLine: false,
                  ...(onSliceClick && {
                    onClick: (_data: unknown, index: number) => {
                      const slice = pieData[index];
                      if (slice) onSliceClick(slice.id);
                    },
                  }),
                },
              ]}
            />
            <Box
              sx={{
                position: "absolute",
                top: "50%",
                left: "50%",
                transform: "translate(-50%, -50%)",
                textAlign: "center",
                pointerEvents: "none",
              }}
            >
              <Typography variant="h4">{total}</Typography>
              <Typography variant="caption" color="text.secondary">
                Total
              </Typography>
            </Box>
          </Box>

          {/* Legend: every category, with its count and share. */}
          <Box sx={{ flex: 1, minWidth: 0, width: "100%", display: "flex", flexDirection: "column", gap: 1 }}>
            {slices.map((s) => {
              const pct = total > 0 ? Math.round((s.value / total) * 100) : 0;
              return (
                <Box
                  key={s.id}
                  onClick={onSliceClick ? () => onSliceClick(s.id) : undefined}
                  sx={{
                    display: "flex",
                    alignItems: "center",
                    justifyContent: "space-between",
                    gap: 1,
                    cursor: onSliceClick ? "pointer" : "default",
                  }}
                >
                  <Box sx={{ display: "flex", alignItems: "center", gap: 1, minWidth: 0 }}>
                    <Box
                      sx={{
                        width: 12,
                        height: 12,
                        borderRadius: "50%",
                        bgcolor: s.color,
                        flexShrink: 0,
                      }}
                    />
                    <Typography variant="body2" color="text.secondary" noWrap>
                      {s.name}
                    </Typography>
                  </Box>
                  <Typography variant="body2" fontWeight={600} sx={{ whiteSpace: "nowrap" }}>
                    {s.value} ({pct}%)
                  </Typography>
                </Box>
              );
            })}
          </Box>
        </Box>
      )}
    </Card>
  );
}
