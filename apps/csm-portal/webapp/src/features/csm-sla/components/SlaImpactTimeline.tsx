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

import { Box, Button, Card, Chip, Skeleton, Tooltip, Typography } from "@wso2/oxygen-ui";
import {
  ArrowRight,
  CalendarClock,
  CircleCheck,
  Headset,
  History,
  Lightbulb,
  PackageCheck,
  RefreshCw,
  RotateCcw,
  TriangleAlert,
  User,
} from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX, type ReactNode } from "react";
import {
  isFeatureEnabled,
  isPreviewedOnly,
  SLA_TIMELINE_FEATURE_ID,
} from "@config/featureFlags";
import { useFeaturePreviewOverrides } from "@config/featurePreview";
import { usePortalAccess, useSafeCurrentUserId } from "@context/current-user/usePortalAccess";
import {
  formatAbsoluteForUser,
  formatBackendTimestampForDisplay,
} from "@utils/dateTime";
import {
  formatCalendarDuration,
  layoutSlaTrack,
  summarizeSlaSides,
  type SlaTrackLayout,
  type SlaTrackMarker,
  type SlaTrackSegment,
  type SlaTrackSide,
} from "@features/csm-sla/utils/slaTrackLayout";
import {
  buildSlaImpactTimeline,
  type SlaTimelineEvent,
  type SlaTimelineSide,
} from "@features/csm-sla/utils/slaImpactTimeline";
import type { CaseAuditEntry } from "@features/csm-cases/types/csmCases";

type Tone = "info" | "warning" | "success" | "error" | "neutral";

const SIDE_LABEL: Record<SlaTimelineSide, string> = {
  customer: "Customer side",
  wso2: "WSO2 side",
};

// Palette roles only, so light and dark both follow the theme. The filled
// badge pairs `<role>.main` with `<role>.contrastText` (the same pairing the
// filled status chips use); the side is also spelled out as text, so colour is
// never the only cue.
function eventTone(event: SlaTimelineEvent): Tone {
  switch (event.kind) {
    case "state":
      return event.side === "customer" ? "warning" : "info";
    case "closed":
    case "workaround":
    case "update_released":
      return "success";
    case "severity":
      return "error";
    default:
      return "neutral";
  }
}

function eventIcon(event: SlaTimelineEvent): ReactNode {
  switch (event.kind) {
    case "state":
      return event.side === "customer" ? <User size={16} /> : <Headset size={16} />;
    case "closed":
      return <CircleCheck size={16} />;
    case "reopened":
      return <RotateCcw size={16} />;
    case "eta":
      return <CalendarClock size={16} />;
    case "workaround":
    case "workaround_withdrawn":
      return <Lightbulb size={16} />;
    case "severity":
      return <TriangleAlert size={16} />;
    default:
      return <PackageCheck size={16} />;
  }
}

const MARKER_PX = 32;
const LANE_STEP_PX = 44;
const TRACK_PX = 10;
const PLOT_INSET_PX = 24;

const TRACK_COLOR: Record<SlaTrackSide, string> = {
  wso2: "info.main",
  customer: "warning.main",
  closed: "text.secondary",
};

const LEGEND: { side: SlaTrackSide; label: string }[] = [
  { side: "wso2", label: "WSO2 side" },
  { side: "customer", label: "Customer side" },
  { side: "closed", label: "Closed" },
];

function tickLabel(ms: number, spanMs: number): string {
  const iso = new Date(ms).toISOString();
  const options: Intl.DateTimeFormatOptions =
    spanMs < 2 * 24 * 60 * 60 * 1000
      ? { month: "short", day: "numeric", hour: "numeric", minute: "2-digit" }
      : spanMs > 300 * 24 * 60 * 60 * 1000
        ? { month: "short", day: "numeric", year: "numeric" }
        : { month: "short", day: "numeric" };
  return formatBackendTimestampForDisplay(iso, options) ?? "";
}

function describeChanges(event: SlaTimelineEvent): string[] {
  return event.changes.map((c) => `${c.label}: ${c.from ?? "None"} to ${c.to ?? "Cleared"}`);
}

function MarkerTooltip({ event }: { event: SlaTimelineEvent }): JSX.Element {
  const when = formatAbsoluteForUser(event.at);
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 0.5, py: 0.25 }}>
      <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
        <Typography variant="body2" sx={{ fontWeight: 600 }} color="inherit">
          {event.title}
        </Typography>
        {event.side ? (
          <Chip
            size="small"
            color={event.side === "customer" ? "warning" : "info"}
            label={SIDE_LABEL[event.side]}
          />
        ) : null}
      </Box>
      {event.changes.map((c) => (
        <Box key={c.label} sx={{ display: "flex", alignItems: "center", gap: 0.75, flexWrap: "wrap" }}>
          <Typography variant="caption" color="inherit">{`${c.label}:`}</Typography>
          <Typography variant="caption" color="inherit">{c.from ?? "None"}</Typography>
          <ArrowRight size={12} aria-label="to" />
          <Typography variant="caption" sx={{ fontWeight: 600 }} color="inherit">
            {c.to ?? "Cleared"}
          </Typography>
        </Box>
      ))}
      <Typography variant="caption" color="inherit">
        {[when ?? "Unknown time", event.actor].filter(Boolean).join(" · ")}
      </Typography>
    </Box>
  );
}

function Marker({ marker, trackY }: { marker: SlaTrackMarker; trackY: number }): JSX.Element {
  const { event, lane, leftPct } = marker;
  const tone = eventTone(event);
  const when = formatAbsoluteForUser(event.at);
  const above = lane % 2 === 1;
  const offset = lane === 0 ? 0 : (above ? -1 : 1) * Math.ceil(lane / 2) * LANE_STEP_PX;
  const centerY = trackY + offset;
  const label = [event.title, ...describeChanges(event), event.actor, when]
    .filter(Boolean)
    .join(", ");
  return (
    <>
      {lane > 0 ? (
        <Box
          aria-hidden
          sx={{
            position: "absolute",
            left: `${leftPct}%`,
            width: 2,
            ml: "-1px",
            top: Math.min(centerY, trackY),
            height: Math.abs(offset),
            bgcolor: "divider",
          }}
        />
      ) : null}
      <Tooltip title={<MarkerTooltip event={event} />} arrow enterTouchDelay={0} leaveTouchDelay={5000}>
        <Box
          component="button"
          type="button"
          aria-label={label}
          sx={{
            position: "absolute",
            left: `${leftPct}%`,
            top: centerY - MARKER_PX / 2,
            ml: `${-MARKER_PX / 2}px`,
            width: MARKER_PX,
            height: MARKER_PX,
            p: 0,
            borderRadius: "50%",
            cursor: "pointer",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            border: 2,
            borderColor: tone === "neutral" ? "text.secondary" : "background.paper",
            bgcolor: tone === "neutral" ? "background.paper" : `${tone}.main`,
            color: tone === "neutral" ? "text.primary" : `${tone}.contrastText`,
            "&:focus-visible": { outline: "2px solid", outlineColor: "text.primary", outlineOffset: 2 },
          }}
        >
          {eventIcon(event)}
        </Box>
      </Tooltip>
    </>
  );
}

function segmentTitle(seg: SlaTrackSegment): string {
  const side = seg.side === "closed" ? "Closed" : SIDE_LABEL[seg.side];
  const duration = seg.side === "closed" && seg.durationMs === 0 ? "" : `: ${formatCalendarDuration(seg.durationMs)}`;
  return `${side}${duration}${seg.ongoing ? " (ongoing)" : ""}`;
}

const CALENDAR_TIME_NOTE = "Calendar time; the SLA table above uses business time.";

function SideSummary({ layout }: { layout: SlaTrackLayout }): JSX.Element {
  if (!layout.sidesKnown) {
    return (
      <Typography variant="body2" color="text.secondary">
        Not enough history to split this case's time by side.
      </Typography>
    );
  }
  const sum = summarizeSlaSides(layout.segments);
  const rows: { side: SlaTimelineSide; ms: number; pct: number }[] = [
    { side: "wso2", ms: sum.wso2Ms, pct: sum.wso2Pct },
    { side: "customer", ms: sum.customerMs, pct: sum.customerPct },
  ];
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
      <Box sx={{ display: "flex", gap: 3, flexWrap: "wrap", alignItems: "baseline" }}>
        {rows.map((r) => (
          <Box key={r.side} sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
            <Box
              aria-hidden
              sx={{ width: 10, height: 10, borderRadius: "50%", bgcolor: TRACK_COLOR[r.side] }}
            />
            <Typography variant="body2">
              {`${SIDE_LABEL[r.side]}: `}
              <strong>{formatCalendarDuration(r.ms)}</strong>
              {` (${Math.round(r.pct)}%)`}
            </Typography>
          </Box>
        ))}
        <Typography variant="body2" color="text.secondary">
          {`Open period: ${formatCalendarDuration(sum.openMs)}`}
        </Typography>
      </Box>
      <Box
        role="img"
        aria-label={`Share of the open period: WSO2 side ${Math.round(sum.wso2Pct)}%, customer side ${Math.round(sum.customerPct)}%`}
        sx={{ display: "flex", height: 8, borderRadius: 8, overflow: "hidden", bgcolor: "divider" }}
      >
        <Box sx={{ width: `${sum.wso2Pct}%`, bgcolor: TRACK_COLOR.wso2 }} />
        <Box sx={{ width: `${sum.customerPct}%`, bgcolor: TRACK_COLOR.customer }} />
      </Box>
      <Typography variant="caption" color="text.secondary">
        {sum.closedMs > 0
          ? `${CALENDAR_TIME_NOTE} Time spent closed (${formatCalendarDuration(sum.closedMs)}) is not counted.`
          : CALENDAR_TIME_NOTE}
      </Typography>
    </Box>
  );
}

function TimelineTrack({ layout }: { layout: SlaTrackLayout }): JSX.Element {
  const aboveLevels = Math.ceil(layout.maxLane / 2);
  const belowLevels = Math.floor(layout.maxLane / 2);
  const trackY = aboveLevels * LANE_STEP_PX + MARKER_PX / 2 + 4;
  const tickY = trackY + belowLevels * LANE_STEP_PX + MARKER_PX / 2 + 8;

  return (
    <Box sx={{ overflowX: "auto", maxWidth: "100%", pb: 0.5 }}>
      <Box sx={{ minWidth: layout.baseWidthPx + 2 * PLOT_INSET_PX, px: `${PLOT_INSET_PX}px` }}>
        <Box
          role="group"
          aria-label="SLA events, oldest to newest"
          sx={{ position: "relative", height: tickY + 28 }}
        >
          <Box
            aria-hidden
            sx={{
              position: "absolute",
              left: 0,
              right: 0,
              top: trackY - TRACK_PX / 2,
              height: TRACK_PX,
              borderRadius: TRACK_PX,
              overflow: "hidden",
              bgcolor: "divider",
            }}
          >
            {layout.segments.map((seg, i) => (
              <Tooltip key={i} title={segmentTitle(seg)} arrow followCursor>
                <Box
                  sx={{
                    position: "absolute",
                    top: 0,
                    bottom: 0,
                    left: `${seg.leftPct}%`,
                    width: `${seg.widthPct}%`,
                    bgcolor: TRACK_COLOR[seg.side],
                  }}
                />
              </Tooltip>
            ))}
          </Box>
          {layout.ticks.map((t) => (
            <Box
              key={t.leftPct}
              aria-hidden
              sx={{ position: "absolute", left: `${t.leftPct}%`, top: tickY, width: 0 }}
            >
              <Box sx={{ width: 1, height: 6, bgcolor: "text.secondary", mx: "auto" }} />
              <Typography
                variant="caption"
                color="text.secondary"
                sx={{
                  display: "block",
                  whiteSpace: "nowrap",
                  transform: "translateX(-50%)",
                  // Keep the first and last label inside the plot.
                  ...(t.leftPct === 0 && { transform: "translateX(-8px)" }),
                  ...(t.leftPct === 100 && { transform: "translateX(calc(-100% + 8px))" }),
                }}
              >
                {t.label}
              </Typography>
            </Box>
          ))}
          {layout.markers.map((m) => (
            <Marker key={m.event.id} marker={m} trackY={trackY} />
          ))}
        </Box>
      </Box>
    </Box>
  );
}

interface SlaImpactTimelineProps {
  /** The case's audited field changes; the caller owns the fetch. */
  entries: CaseAuditEntry[] | undefined;
  isLoading: boolean;
  isError: boolean;
  onRetry: () => void;
  /** When the case was opened; the track starts here (else at its first event). */
  openedAt?: string;
  /** The case's current state; colours the track when no state change was audited. */
  currentState?: string;
}

/**
 * Horizontal track, open to now (or closure), of what moved a case's SLA, coloured by
 * whose side holds the case with a marker per event: switches between the
 * customer and WSO2 side, fix ETA changes, workaround, severity, closure and
 * reopening. Rendered after the SLA table on the case SLA tab. Hidden when
 * the `support.case-sla-timeline` feature is overridden to `hidden` (or `wip`)
 * in `CSM_PORTAL_FEATURE_OVERRIDES`.
 */
export function SlaImpactTimeline({
  entries,
  isLoading,
  isError,
  onRetry,
  openedAt,
  currentState,
}: SlaImpactTimelineProps): JSX.Element | null {
  const access = usePortalAccess();
  const { overrides: previewOverrides } = useFeaturePreviewOverrides(useSafeCurrentUserId());
  // Fixed per mount: the track's right edge is "now" as of when the tab opened.
  const [nowMs] = useState(() => Date.now());
  if (!isFeatureEnabled(SLA_TIMELINE_FEATURE_ID, access, previewOverrides)) return null;
  const previewed = isPreviewedOnly(SLA_TIMELINE_FEATURE_ID, access, previewOverrides);

  const events = buildSlaImpactTimeline(entries ?? []);
  const layout = layoutSlaTrack({ events, openedAt, currentState, nowMs, formatTick: tickLabel });

  return (
    <Card
      component="section"
      aria-label="SLA impact timeline"
      sx={{ p: 2.5, mt: 2, display: "flex", flexDirection: "column", gap: 2, minWidth: 0, maxWidth: "100%" }}
    >
      <Box sx={{ display: "flex", alignItems: "center", gap: 0.75, flexWrap: "wrap" }}>
        <History size={16} />
        <Typography variant="subtitle2">SLA impact timeline</Typography>
        {!isLoading && !isError ? (
          <Chip size="small" variant="outlined" label={`${events.length} events`} />
        ) : null}
        {previewed ? <Chip size="small" color="info" label="Preview" /> : null}
      </Box>

      {isLoading ? (
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5 }} aria-busy="true">
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} variant="rounded" height={44} />
          ))}
        </Box>
      ) : isError ? (
        <Box sx={{ display: "flex", flexDirection: "column", alignItems: "center", gap: 1, py: 3 }}>
          <Typography variant="body2" color="error">
            Could not load the SLA impact timeline for this case.
          </Typography>
          <Button
            size="small"
            variant="outlined"
            startIcon={<RefreshCw size={14} />}
            onClick={onRetry}
            sx={{ textTransform: "none" }}
          >
            Retry
          </Button>
        </Box>
      ) : events.length === 0 ? (
        <Typography variant="body2" color="text.secondary" sx={{ py: 1 }}>
          No SLA-affecting changes recorded for this case yet.
        </Typography>
      ) : (
        <>
          <SideSummary layout={layout} />
          <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }}>
            {LEGEND.map((l) => (
              <Box key={l.side} sx={{ display: "flex", alignItems: "center", gap: 0.75 }}>
                <Box
                  aria-hidden
                  sx={{ width: 20, height: TRACK_PX, borderRadius: TRACK_PX, bgcolor: TRACK_COLOR[l.side] }}
                />
                <Typography variant="caption">{l.label}</Typography>
              </Box>
            ))}
          </Box>
          <TimelineTrack layout={layout} />
        </>
      )}
    </Card>
  );
}
