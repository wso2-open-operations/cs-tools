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

import { Box, Button, Card, Chip, IconButton, Link, Skeleton, Tooltip, Typography } from "@wso2/oxygen-ui";
import { ArrowLeft, Check, Copy } from "@wso2/oxygen-ui-icons-react";
import { useEffect, useMemo, useState, type JSX, type ReactNode } from "react";
import { Link as RouterLink, useLocation, useParams } from "react-router";
import { useNavTransition } from "@hooks/useNavTransition";
import QueryErrorState from "@components/QueryErrorState";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import type { BeSreAlertDetail, BeSreAlertIncident } from "@api/backend/types";
import {
  incidentPriorityColor,
  incidentPriorityLabel,
  incidentStateColor,
  incidentStateLabel,
} from "@features/csm-operations/utils/incidents";
import { incidentPriorityBadge } from "@features/csm-operations/utils/incidentPriorityMatrix";
import { useGetSreAlert } from "@features/csm-operations/api/useSreAlerts";
import {
  SRE_ALERTS_LIST_PATH,
  sreAlertIncidentLevelLabel,
  sreAlertIncidentState,
  sreAlertSeverityColor,
  sreAlertSeverityLabel,
  sreAlertTierLabel,
} from "@features/csm-operations/utils/sreAlerts";

function formatDateTime(value: string | null | undefined): string {
  return formatBackendTimestampForDisplay(value, { dateStyle: "medium", timeStyle: "medium" }) ?? "—";
}

/** One labelled value in a card's grid, styled like the alert mute rule page's overview. */
function Field({ label, children, mono }: { label: string; children: ReactNode; mono?: boolean }): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25, minWidth: 0 }}>
      <Typography variant="caption" color="text.secondary" sx={{ textTransform: "uppercase", letterSpacing: 0.4 }}>
        {label}
      </Typography>
      <Typography
        component="div"
        variant="body2"
        sx={{ whiteSpace: "pre-wrap", wordBreak: "break-word", ...(mono && { fontFamily: "monospace" }) }}
      >
        {children === "" || children === null || children === undefined ? "—" : children}
      </Typography>
    </Box>
  );
}

/** A titled card; its body is a responsive grid of `Field`s unless `plain`. */
function Section({ title, children, plain }: { title: string; children: ReactNode; plain?: boolean }): JSX.Element {
  return (
    <Card variant="outlined" component="section" aria-label={title} sx={{ backgroundColor: "background.default" }}>
      <Box sx={{ px: 2, py: 1 }}>
        <Typography
          variant="caption"
          color="text.secondary"
          sx={{ textTransform: "uppercase", letterSpacing: 0.4, fontWeight: 600 }}
        >
          {title}
        </Typography>
      </Box>
      <Box
        sx={{
          px: 2,
          pb: 2,
          pt: 1.5,
          borderTop: 1,
          borderColor: "divider",
          ...(plain
            ? {}
            : {
                display: "grid",
                gridTemplateColumns: {
                  xs: "repeat(1, minmax(0, 1fr))",
                  sm: "repeat(2, minmax(0, 1fr))",
                  md: "repeat(4, minmax(0, 1fr))",
                },
                gap: 2,
              }),
        }}
      >
        {children}
      </Box>
    </Card>
  );
}

/** The alert's UUID with a copy button; the button confirms for a moment, or says the browser refused. */
function CopyableId({ id }: { id: string }): JSX.Element {
  const [state, setState] = useState<"idle" | "copied" | "failed">("idle");
  useEffect(() => {
    if (state === "idle") return;
    const t = setTimeout(() => setState("idle"), 2000);
    return () => clearTimeout(t);
  }, [state]);
  const copy = async (): Promise<void> => {
    try {
      await navigator.clipboard.writeText(id);
      setState("copied");
    } catch {
      setState("failed");
    }
  };
  const tooltip = state === "copied" ? "Copied" : state === "failed" ? "Couldn't copy. Select the ID instead." : "Copy ID";
  return (
    <Box sx={{ display: "flex", alignItems: "center", gap: 0.5, minWidth: 0 }}>
      <Typography
        variant="caption"
        color="text.secondary"
        sx={{ fontFamily: "monospace", wordBreak: "break-all" }}
        data-testid="sre-alert-uuid"
      >
        {id}
      </Typography>
      <Tooltip title={tooltip}>
        <IconButton size="small" aria-label="Copy alert ID" onClick={() => void copy()}>
          {state === "copied" ? <Check size={14} /> : <Copy size={14} />}
        </IconButton>
      </Tooltip>
    </Box>
  );
}

/** The incident's priority as the incident page shows it: the P1–P5 badge when known, else a plain chip. */
function IncidentPriority({ priority }: { priority: string | null }): JSX.Element {
  if (!priority) return <>—</>;
  const badge = incidentPriorityBadge(priority);
  return badge ? (
    <Chip
      size="small"
      label={`${badge.label} (${badge.code})`}
      sx={{ bgcolor: badge.bg, color: badge.fg, fontWeight: 600 }}
    />
  ) : (
    <Chip size="small" variant="outlined" color={incidentPriorityColor(priority)} label={incidentPriorityLabel(priority)} />
  );
}

/**
 * The incident this alert is linked to, as a related-record card like the
 * incident page's own Linked records: number (opening the incident, whose
 * Back returns here), subject, state, priority, impact, urgency and opened date.
 */
function LinkedIncidentCard({
  incident,
  navState,
}: {
  incident: BeSreAlertIncident | null;
  navState: { from: string; parentState: unknown };
}): JSX.Element {
  if (!incident) {
    return (
      <Card component="section" aria-label="Incident" sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
        <Typography variant="subtitle2">Incident</Typography>
        <Typography variant="body2" color="text.secondary">
          Not linked to an incident
        </Typography>
      </Card>
    );
  }
  const state = sreAlertIncidentState(incident.state);
  return (
    <Card component="section" aria-label="Incident" sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
      <Typography variant="subtitle2">Incident</Typography>
      <Box sx={{ display: "flex", alignItems: "center", gap: 1.5, flexWrap: "wrap", minWidth: 0 }}>
        <Link
          component={RouterLink}
          to={`/operations/incidents/${encodeURIComponent(incident.id)}`}
          state={navState}
          sx={{ fontFamily: "monospace", fontWeight: 700 }}
        >
          {incident.number || incident.id}
        </Link>
        {state && <Chip size="small" color={incidentStateColor(state)} label={incidentStateLabel(state)} />}
        <Typography variant="body2" sx={{ minWidth: 0, wordBreak: "break-word" }}>
          {incident.subject || "—"}
        </Typography>
      </Box>
      <Box
        sx={{
          display: "grid",
          gridTemplateColumns: {
            xs: "repeat(2, minmax(0, 1fr))",
            md: "repeat(4, minmax(0, 1fr))",
          },
          gap: 2,
        }}
      >
        <Field label="Priority">
          <IncidentPriority priority={incident.priority} />
        </Field>
        <Field label="Impact">{sreAlertIncidentLevelLabel(incident.impact)}</Field>
        <Field label="Urgency">{sreAlertIncidentLevelLabel(incident.urgency)}</Field>
        <Field label="Opened">{incident.openedOn ? formatDateTime(incident.openedOn) : ""}</Field>
      </Box>
    </Card>
  );
}

/**
 * The raw body indented when it is JSON, else as it came. The text is
 * re-indented rather than parsed and serialised again, so every value stays
 * exactly as the source sent it (JSON.parse would round an integer above
 * 2^53).
 */
function formatRawBody(raw: string): string {
  try {
    JSON.parse(raw);
  } catch {
    return raw;
  }
  let out = "";
  let depth = 0;
  let inString = false;
  let escaped = false;
  const newline = (): string => `\n${"  ".repeat(depth)}`;
  for (let i = 0; i < raw.length; i++) {
    const c = raw[i];
    if (inString) {
      out += c;
      if (escaped) escaped = false;
      else if (c === "\\") escaped = true;
      else if (c === '"') inString = false;
      continue;
    }
    switch (c) {
      case '"':
        inString = true;
        out += c;
        break;
      case " ":
      case "\t":
      case "\n":
      case "\r":
        break;
      case "{":
      case "[": {
        const close = c === "{" ? "}" : "]";
        let next = i + 1;
        while (next < raw.length && " \t\n\r".includes(raw[next])) next++;
        if (raw[next] === close) {
          out += c + close;
          i = next;
        } else {
          depth++;
          out += c + newline();
        }
        break;
      }
      case "}":
      case "]":
        depth--;
        out += newline() + c;
        break;
      case ",":
        out += "," + newline();
        break;
      case ":":
        out += ": ";
        break;
      default:
        out += c;
    }
  }
  return out;
}

function RawPayload({ rawBody }: { rawBody: string | null }): JSX.Element {
  const text = useMemo(() => (rawBody === null ? null : formatRawBody(rawBody)), [rawBody]);
  if (text === null) {
    return (
      <Typography variant="body2" color="text.secondary">
        Only the first alert of a message keeps its payload.
      </Typography>
    );
  }
  return (
    <Box
      component="pre"
      data-testid="sre-alert-raw-payload"
      sx={{
        m: 0,
        p: 1.5,
        maxHeight: 480,
        overflow: "auto",
        fontFamily: "monospace",
        fontSize: 12,
        lineHeight: 1.5,
        whiteSpace: "pre",
        bgcolor: "action.hover",
        borderRadius: 1,
      }}
    >
      {text}
    </Box>
  );
}

/**
 * One SRE alert (`/operations/alerts/:id`, by UUID or ALT number): its
 * number and UUID, what raised it, how it was delivered, the incident it is
 * linked to, and the source's raw payload. Back returns to `state.from` (the
 * filtered list it was opened from) with that page's own state.
 */
export default function SreAlertDetailPage(): JSX.Element {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavTransition();
  const location = useLocation();
  const nav = location.state as { from?: string; parentState?: unknown } | undefined;
  const backTarget = nav?.from ?? SRE_ALERTS_LIST_PATH;
  const backState = nav?.parentState ?? undefined;
  const { data: alert, isLoading, isError, error } = useGetSreAlert(id);

  const back = (
    <Button
      variant="text"
      size="small"
      startIcon={<ArrowLeft size={16} />}
      onClick={() => navigate(backTarget, { state: backState })}
      sx={{ alignSelf: "flex-start" }}
    >
      Back
    </Button>
  );

  if (isLoading) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {back}
        <Skeleton variant="rounded" height={48} width={280} />
        <Skeleton variant="rounded" height={200} />
        <Skeleton variant="rounded" height={120} />
      </Box>
    );
  }
  if (isError || !alert) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {back}
        <QueryErrorState message={isError ? error?.message || "Failed to load the alert." : "Alert not found."} error={error} />
      </Box>
    );
  }

  return <SreAlertDetail alert={alert} back={back} />;
}

function SreAlertDetail({ alert, back }: { alert: BeSreAlertDetail; back: ReactNode }): JSX.Element {
  const location = useLocation();
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      {back}

      <Box sx={{ display: "flex", flexDirection: "column", gap: 1, minWidth: 0 }}>
        <Typography variant="h6" sx={{ fontFamily: "monospace", fontWeight: 700, letterSpacing: 0.2, lineHeight: 1.2 }}>
          {alert.number || alert.id}
        </Typography>
        <CopyableId id={alert.id} />
        <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
          {alert.severity && (
            <Chip size="small" color={sreAlertSeverityColor(alert.severity)} label={sreAlertSeverityLabel(alert.severity)} />
          )}
          {alert.tier && (
            <Chip
              size="small"
              variant="outlined"
              color={alert.tier === "priority" ? "primary" : "default"}
              label={`${sreAlertTierLabel(alert.tier)} tier`}
            />
          )}
        </Box>
      </Box>

      <Section title="Alert">
        <Field label="Source">{alert.source}</Field>
        <Field label="Service">{alert.service}</Field>
        <Field label="Environment">{alert.environment}</Field>
        <Field label="Metric">{alert.metricName}</Field>
        <Field label="Category">{alert.category}</Field>
        <Field label="Unique identifier" mono>
          {alert.uniqueIdentifier}
        </Field>
        <Field label="Assignment group">{alert.assignmentGroup}</Field>
        <Box sx={{ gridColumn: "1 / -1" }}>
          <Field label="Description">{alert.description}</Field>
        </Box>
      </Section>

      <Section title="Delivery">
        <Field label="Received on">{formatDateTime(alert.receivedOn)}</Field>
        <Field label="Created on">{formatDateTime(alert.createdOn)}</Field>
      </Section>

      <LinkedIncidentCard
        incident={alert.incident}
        navState={{ from: `${location.pathname}${location.search}`, parentState: location.state ?? null }}
      />

      <Section title="Raw payload" plain>
        <RawPayload rawBody={alert.rawBody} />
      </Section>
    </Box>
  );
}
