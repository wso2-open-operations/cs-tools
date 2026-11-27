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

import { Alert, AlertTitle, Box, Button, Card, Chip, Skeleton, Typography } from "@wso2/oxygen-ui";
import { ArrowLeft, ArrowUpRight, TriangleAlert } from "@wso2/oxygen-ui-icons-react";
import { useEffect, useMemo, type JSX, type ReactNode } from "react";
import { Link as RouterLink, useLocation, useNavigate, useParams } from "react-router";
import { useNavTransition } from "@hooks/useNavTransition";
import QueryErrorState from "@components/QueryErrorState";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import type { BeSreAlertDefaultedField, BeSreAlertDetail, BeSreAlertIncident } from "@api/backend/types";
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
  sreAlertDefaultedMessage,
  sreAlertIncidentState,
  sreAlertSeverityColor,
  sreAlertSeverityLabel,
  sreAlertTierLabel,
} from "@features/csm-operations/utils/sreAlerts";

function formatDateTime(value: string | null | undefined): string {
  return formatBackendTimestampForDisplay(value, { dateStyle: "medium", timeStyle: "medium" }) ?? "—";
}

/** One labelled value in the Overview grid, styled like the incident page's; `defaulted` marks a value the payload did not carry. */
function Field({
  label,
  children,
  mono,
  defaulted,
}: {
  label: string;
  children: ReactNode;
  mono?: boolean;
  defaulted?: boolean;
}): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25, minWidth: 0 }}>
      <Typography variant="caption" color="text.secondary" sx={{ textTransform: "uppercase", letterSpacing: 0.4 }}>
        {label}
      </Typography>
      <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap", minWidth: 0 }}>
        <Typography
          component="div"
          variant="body2"
          sx={{ whiteSpace: "pre-wrap", wordBreak: "break-word", ...(mono && { fontFamily: "monospace" }) }}
        >
          {children === "" || children === null || children === undefined ? "—" : children}
        </Typography>
        {defaulted && (
          <Chip size="small" variant="outlined" label="Default" title="Not in the payload: the source's default" sx={{ height: 20, fontSize: 11 }} />
        )}
      </Box>
    </Box>
  );
}

/**
 * The fields the alert's payload did not carry, each with the default its
 * source's transform used, one line per field in the same words.
 */
function DefaultedFieldsNotice({ fields }: { fields: BeSreAlertDefaultedField[] }): JSX.Element | null {
  if (fields.length === 0) return null;
  return (
    <Box component="section" aria-label="Defaults used">
      <Alert severity="info" role="note">
        <AlertTitle>Some fields were filled with defaults</AlertTitle>
        <Box component="ul" sx={{ m: 0, pl: 2.5 }}>
          {fields.map((d) => (
            <li key={d.field}>{sreAlertDefaultedMessage(d)}</li>
          ))}
        </Box>
      </Alert>
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
 * The Overview's Incident section, laid out like the incident page's Runbook
 * section: the incident this alert is linked to (alert.incident_id), with its
 * state, priority and opened date, and a button that opens
 * it (its Back returns to this alert).
 */
function AlertIncidentSection({
  incident,
  navState,
}: {
  incident: BeSreAlertIncident | null;
  navState: { from: string; parentState: unknown };
}): JSX.Element {
  let body: JSX.Element;
  if (!incident) {
    body = (
      <Typography variant="body2" color="text.secondary">
        This alert is not linked to an incident.
      </Typography>
    );
  } else {
    const state = sreAlertIncidentState(incident.state);
    body = (
      <Box
        sx={{
          display: "flex",
          flexWrap: "wrap",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 1.5,
          p: 1.5,
          borderRadius: 1,
          border: 1,
          borderColor: "divider",
          bgcolor: "action.hover",
        }}
      >
        <Box sx={{ display: "flex", alignItems: "flex-start", gap: 1.25, minWidth: 0 }}>
          <Box sx={{ color: "primary.main", display: "flex", pt: 0.25 }}>
            <TriangleAlert size={20} />
          </Box>
          <Box sx={{ display: "flex", flexDirection: "column", gap: 0.75, minWidth: 0 }}>
            <Typography variant="body1" sx={{ fontWeight: 600, wordBreak: "break-word" }}>
              {incident.subject || "Incident"} ({incident.number || incident.id})
            </Typography>
            <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
              {state && <Chip size="small" color={incidentStateColor(state)} label={incidentStateLabel(state)} />}
              <IncidentPriority priority={incident.priority} />
              <Typography variant="body2" color="text.secondary">
                Opened {formatDateTime(incident.openedOn)}
              </Typography>
            </Box>
          </Box>
        </Box>
        <Button
          variant="contained"
          size="small"
          component={RouterLink}
          to={`/operations/incidents/${encodeURIComponent(incident.id)}`}
          state={navState}
          endIcon={<ArrowUpRight size={14} />}
        >
          Open incident
        </Button>
      </Box>
    );
  }
  return (
    <Box
      component="section"
      aria-label="Incident"
      sx={{ display: "flex", flexDirection: "column", gap: 1, pt: 2, borderTop: 1, borderColor: "divider" }}
    >
      <Typography variant="caption" color="text.secondary" sx={{ textTransform: "uppercase", letterSpacing: 0.4 }}>
        Incident
      </Typography>
      {body}
    </Box>
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
 * One SRE alert (`/operations/alerts/:id`, by UUID or ALT number; an ALT
 * number in the address is replaced by the UUID), laid out
 * like an incident's page: its number, what fired as the heading, which
 * fields were filled with defaults, an Overview of its fields with the
 * incident it is linked to, and the source's raw payload. Back returns to `state.from` (the
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

  // The address always shows the alert's UUID: one opened by its ALT number is swapped in place, keeping its Back state.
  const replaceRoute = useNavigate();
  useEffect(() => {
    if (alert && id && id !== alert.id) {
      replaceRoute(
        { pathname: `${SRE_ALERTS_LIST_PATH}/${encodeURIComponent(alert.id)}`, search: location.search, hash: location.hash },
        { replace: true, state: location.state },
      );
    }
  }, [alert, id, location.search, location.hash, location.state, replaceRoute]);

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
  // The heading names what fired, as an incident's subject does.
  const title = [alert.metricName, alert.service].filter(Boolean).join(" on ") || "Alert";
  const defaulted = new Set((alert.defaultedFields ?? []).map((d) => d.field));
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      {back}

      <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5, minWidth: 0 }}>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1, minWidth: 0 }}>
          <Typography variant="h6" sx={{ fontFamily: "monospace", fontWeight: 700, letterSpacing: 0.2, lineHeight: 1.2 }}>
            {alert.number || alert.id}
          </Typography>
          <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
            {alert.severity && (
              <Chip
                size="small"
                color={sreAlertSeverityColor(alert.severity)}
                label={`${sreAlertSeverityLabel(alert.severity)}${defaulted.has("severity") ? " (default)" : ""}`}
              />
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
        <Typography variant="h5">{title}</Typography>
      </Box>

      <DefaultedFieldsNotice fields={alert.defaultedFields ?? []} />

      <Card component="section" aria-label="Overview" sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
        <Typography variant="subtitle2">Overview</Typography>
        <Box
          sx={{
            display: "grid",
            gap: 2,
            gridTemplateColumns: {
              xs: "1fr",
              sm: "repeat(2, minmax(0, 1fr))",
              md: "repeat(3, minmax(0, 1fr))",
            },
          }}
        >
          <Field label="Source">{alert.source}</Field>
          <Field label="Service" defaulted={defaulted.has("service")}>
            {alert.service}
          </Field>
          <Field label="Environment" defaulted={defaulted.has("environment")}>
            {alert.environment}
          </Field>
          <Field label="Metric" defaulted={defaulted.has("metric_name")}>
            {alert.metricName}
          </Field>
          <Field label="Category" defaulted={defaulted.has("category")}>
            {alert.category}
          </Field>
          <Field label="Assignment group">{alert.assignmentGroup}</Field>
          <Field label="Unique identifier" mono>
            {alert.uniqueIdentifier}
          </Field>
          <Field label="Received on">{formatDateTime(alert.receivedOn)}</Field>
          <Field label="Created on">{formatDateTime(alert.createdOn)}</Field>
        </Box>
        <AlertIncidentSection
          incident={alert.incident}
          navState={{ from: `${location.pathname}${location.search}`, parentState: location.state ?? null }}
        />
      </Card>

      <Card component="section" aria-label="Raw payload" sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 1.5 }}>
        <Typography variant="subtitle2">Raw payload</Typography>
        <RawPayload rawBody={alert.rawBody} />
      </Card>
    </Box>
  );
}
