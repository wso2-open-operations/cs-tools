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

import { Box, Button, Card, Chip, Skeleton, Typography } from "@wso2/oxygen-ui";
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
import type { JSX, ReactNode } from "react";
import { isSlaTimelineEnabled } from "@config/slaTimelineConfig";
import { formatAbsoluteForUser } from "@utils/dateTime";
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

function TimelineItem({ event, last }: { event: SlaTimelineEvent; last: boolean }): JSX.Element {
  const tone = eventTone(event);
  const when = formatAbsoluteForUser(event.at);
  return (
    <Box
      component="li"
      sx={{
        display: "flex",
        gap: 1.5,
        position: "relative",
        pb: last ? 0 : 2.5,
        // Connector down to the next event's badge.
        "&::before": last
          ? undefined
          : {
              content: '""',
              position: "absolute",
              left: 15,
              top: 32,
              bottom: 0,
              width: 2,
              bgcolor: "divider",
            },
      }}
    >
      <Box
        aria-hidden
        sx={{
          width: 32,
          height: 32,
          flexShrink: 0,
          borderRadius: "50%",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          ...(tone === "neutral"
            ? { bgcolor: "action.selected", color: "text.primary", border: 1, borderColor: "divider" }
            : { bgcolor: `${tone}.main`, color: `${tone}.contrastText` }),
        }}
      >
        {eventIcon(event)}
      </Box>
      <Box sx={{ minWidth: 0, flex: 1 }}>
        <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
          <Typography variant="body2" sx={{ fontWeight: 600 }}>
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
        <Typography variant="caption" color="text.secondary" component="div">
          {when ? <time dateTime={event.at}>{when}</time> : "Unknown time"}
          {event.actor ? ` · ${event.actor}` : ""}
        </Typography>
        {event.changes.map((c) => (
          <Box
            key={c.label}
            sx={{ display: "flex", alignItems: "center", gap: 0.75, flexWrap: "wrap", mt: 0.5 }}
          >
            <Typography variant="body2" color="text.secondary">
              {`${c.label}:`}
            </Typography>
            <Typography variant="body2">{c.from ?? "None"}</Typography>
            <ArrowRight size={14} aria-label="to" />
            <Typography variant="body2" sx={{ fontWeight: 600 }}>
              {c.to ?? "Cleared"}
            </Typography>
          </Box>
        ))}
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
}

/**
 * Chronological timeline of what moved a case's SLA: switches between the
 * customer and WSO2 side, fix ETA changes, workaround, severity, closure and
 * reopening. Rendered after the SLA table on the case SLA tab. Hidden when
 * the `CSM_PORTAL_SLA_TIMELINE_ENABLED` runtime flag is explicitly false.
 */
export function SlaImpactTimeline({
  entries,
  isLoading,
  isError,
  onRetry,
}: SlaImpactTimelineProps): JSX.Element | null {
  if (!isSlaTimelineEnabled()) return null;

  const events = buildSlaImpactTimeline(entries ?? []);

  return (
    <Card
      component="section"
      aria-label="SLA impact timeline"
      sx={{ p: 2.5, mt: 2, display: "flex", flexDirection: "column", gap: 2 }}
    >
      <Box sx={{ display: "flex", alignItems: "center", gap: 0.75, flexWrap: "wrap" }}>
        <History size={16} />
        <Typography variant="subtitle2">SLA impact timeline</Typography>
        {!isLoading && !isError ? (
          <Chip size="small" variant="outlined" label={`${events.length} events`} />
        ) : null}
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
        <Box component="ol" sx={{ listStyle: "none", m: 0, p: 0 }}>
          {events.map((e, i) => (
            <TimelineItem key={e.id} event={e} last={i === events.length - 1} />
          ))}
        </Box>
      )}
    </Card>
  );
}
