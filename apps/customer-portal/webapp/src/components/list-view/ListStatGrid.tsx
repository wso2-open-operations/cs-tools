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

import { Box, StatCard, Skeleton, Tooltip, useTheme } from "@wso2/oxygen-ui";
import { type JSX } from "react";
import { Info } from "@wso2/oxygen-ui-icons-react";
import ErrorIndicator from "@components/error-indicator/ErrorIndicator";
import { type SupportStatConfig } from "@features/support/constants/supportConstants";

export interface ListStatGridProps<T extends string> {
  isLoading: boolean;
  configs: SupportStatConfig<T>[];
  stats: Partial<Record<T, number>> | undefined | null;
  isError?: boolean;
  entityName?: string;
  spacing?: number;
  itemSize?: {
    xs?: number;
    sm?: number;
    md?: number;
    lg?: number;
    xl?: number;
  };
  valueFormatter?: (value: number) => string | number;
  onStatClick?: (key: T) => void;
  nonClickableKeys?: T[];
}

/**
 * Maps MUI-style 12-column `size` segments to `grid-template-columns` repeat count.
 *
 * @param {number} segment - Grid segment (12 = one full-width column).
 * @returns {string} A `repeat(..., minmax(0, 1fr))` track list.
 */
function columnsFromSegment(segment: number): string {
  const n = 12 / segment;
  return `repeat(${n}, minmax(0, 1fr))`;
}

/**
 * ListStatGrid component to display a grid of support statistics.
 *
 * @param {ListStatGridProps} props - The props for the component.
 * @returns {JSX.Element} The rendered ListStatGrid component.
 */
export default function ListStatGrid<T extends string>({
  isLoading,
  configs,
  stats,
  isError,
  entityName = "statistics",
  spacing = 2,
  itemSize = { xs: 12, sm: 6, md: 3 },
  valueFormatter,
  onStatClick,
  nonClickableKeys,
}: ListStatGridProps<T>): JSX.Element {
  const theme = useTheme();
  const xs = itemSize.xs ?? 12;
  const sm = itemSize.sm ?? 6;
  const md = itemSize.md ?? 3;
  const lg = itemSize.lg ?? md;
  const xl = itemSize.xl ?? lg;

  return (
    <Box
      sx={{
        display: "grid",
        width: "100%",
        gap: spacing,
        gridTemplateColumns: {
          xs: columnsFromSegment(xs),
          sm: columnsFromSegment(sm),
          md: columnsFromSegment(md),
          lg: columnsFromSegment(lg),
          xl: columnsFromSegment(xl),
        },
      }}
    >
      {configs.map((stat) => {
        const SecondaryIcon = stat.secondaryIcon;
        const Icon = stat.icon;
        const isClickable = !!onStatClick && !nonClickableKeys?.includes(stat.key);

        return (
          <Box
            key={stat.key}
            onClick={isClickable ? () => onStatClick(stat.key) : undefined}
            role={isClickable ? "button" : undefined}
            tabIndex={isClickable ? 0 : undefined}
            onKeyDown={
              isClickable
                ? (e) => {
                    if (e.key === "Enter" || e.key === " ") {
                      e.preventDefault();
                      onStatClick(stat.key);
                    }
                  }
                : undefined
            }
            sx={{
              position: "relative",
              minWidth: 0,
              cursor: isClickable ? "pointer" : undefined,
              borderRadius: 1,
              transition: isClickable ? "box-shadow 0.2s ease, transform 0.15s ease" : undefined,
              "&:hover": isClickable
                ? {
                    boxShadow: `0 0 0 1px ${theme.palette.primary.main}, 0 4px 16px rgba(0,0,0,0.12)`,
                    transform: "translateY(-2px)",
                  }
                : undefined,
              "&:focus-visible": isClickable
                ? { outline: "2px solid", outlineOffset: 2 }
                : undefined,
            }}
          >
            {SecondaryIcon && (
              <Box
                sx={{
                  opacity: 0.4,
                  position: "absolute",
                  right: 24,
                  top: 20,
                  zIndex: 1,
                }}
              >
                <SecondaryIcon />
              </Box>
            )}
            {stat.tooltipText && (
              <Box
                sx={{
                  position: "absolute",
                  top: 56,
                  right: 14,
                  zIndex: 2,
                  display: "inline-flex",
                  alignItems: "center",
                  color: "text.secondary",
                  backgroundColor: "background.paper",
                }}
              >
                <Tooltip title={stat.tooltipText}>
                  <Box component="span" sx={{ display: "inline-flex" }}>
                    <Info size={14} />
                  </Box>
                </Tooltip>
              </Box>
            )}
            <StatCard
              label={stat.label}
              value={(
                isLoading ? (
                  <Skeleton
                    data-testid="Skeleton"
                    variant="rounded"
                    width={60}
                    height={24}
                  />
                ) : isError ? (
                  <ErrorIndicator entityName={entityName} />
                ) : stats?.[stat.key] != null ? (
                  valueFormatter ? (
                    valueFormatter(stats[stat.key] as number)
                  ) : (
                    stats[stat.key]
                  )
                ) : (
                  "--"
                )
              ) as unknown as string | number}
              icon={<Icon />}
              iconColor={stat.iconColor}
            />
          </Box>
        );
      })}
    </Box>
  );
}
