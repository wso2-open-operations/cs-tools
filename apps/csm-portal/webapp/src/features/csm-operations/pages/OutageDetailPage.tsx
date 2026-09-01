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

import {
  Alert,
  Box,
  Button,
  Card,
  Chip,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  Skeleton,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import {
  ArrowLeft,
  CheckCircle,
  Link as LinkIcon,
  Megaphone,
  Pencil,
  RotateCcw,
} from "@wso2/oxygen-ui-icons-react";
import { type JSX, type ReactNode, useEffect, useState } from "react";
import { useLocation } from "react-router";
import { formatBackendTimestampForDisplay } from "@utils/dateTime";
import { BackendApiError } from "@api/backend/client";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useGetOutage, usePatchOutage } from "@features/csm-operations/api/useOutages";
import {
  useAddOutageCommunication,
  useGetOutageCommunications,
} from "@features/csm-operations/api/useOutageCommunications";
import EditOutageDialog from "@features/csm-operations/components/EditOutageDialog";
import CloseOutageDialog from "@features/csm-operations/components/CloseOutageDialog";
import {
  outageStatusColor,
  outageStatusLabel,
  outageTypeColor,
  outageTypeLabel,
} from "@features/csm-operations/utils/outages";
import type { BeOutageCommunicationChannel, BeOutageConfigurationItemRef } from "@api/backend/types";
import { useNavTransition } from "@hooks/useNavTransition";
import { useNormalizedIdParam } from "@hooks/useNormalizedIdParam";
import { useRecordRecentView } from "@features/csm-recent/hooks/useRecentViews";

const OPERATIONS_OUTAGES_PATH = "/operations?tab=outages";

const CHANNEL_LABEL: Record<BeOutageCommunicationChannel, string> = {
  external: "External (public)",
  internal: "Internal",
  additional: "Additional notes",
};

function formatDateTime(value?: string | null): string {
  return (
    formatBackendTimestampForDisplay(value, {
      dateStyle: "medium",
      timeStyle: "short",
    }) ?? "—"
  );
}

function MetaCell({ label, children }: { label: string; children: ReactNode }): JSX.Element {
  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 0.25, minWidth: 0 }}>
      <Typography
        variant="caption"
        color="text.secondary"
        sx={{ textTransform: "uppercase", letterSpacing: 0.4 }}
      >
        {label}
      </Typography>
      <Box sx={{ minWidth: 0 }}>{children}</Box>
    </Box>
  );
}

function CiRef({ value }: { value?: BeOutageConfigurationItemRef | null }): JSX.Element {
  if (!value) return <Typography variant="body2">—</Typography>;
  return (
    <Typography variant="body2">
      {value.name} <Typography component="span" variant="caption" color="text.secondary">({value.className})</Typography>
    </Typography>
  );
}

/**
 * Detail for a single outage (`GET /outages/{id}`): overview, links, and its
 * communications journal. There is no lifecycle action bar the way
 * incidents/problems have one — an outage has exactly two write actions,
 * Edit (classification/links) and Close/Reopen (`end`), both dialogs here.
 * `publishesToStatusPage`/`statusPageCloud` are rendered prominently: they
 * are the only signal an engineer has that this record — and any `external`
 * communication added to it — is publicly visible.
 */
export default function OutageDetailPage(): JSX.Element {
  const id = useNormalizedIdParam("id");
  const navigate = useNavTransition();
  const backState = useLocation().state as { from?: string } | undefined;
  const backTarget = backState?.from ?? OPERATIONS_OUTAGES_PATH;
  const { data, isLoading, isError } = useGetOutage(id);
  const { showError } = useErrorBanner();
  const patchOutage = usePatchOutage();
  const [editOpen, setEditOpen] = useState(false);
  const [closeOpen, setCloseOpen] = useState(false);

  const { data: communicationsData } = useGetOutageCommunications(id);
  const addCommunication = useAddOutageCommunication();
  const [channel, setChannel] = useState<BeOutageCommunicationChannel>("internal");
  const [body, setBody] = useState("");
  const [ackExternal, setAckExternal] = useState(false);

  const recordView = useRecordRecentView();
  useEffect(() => {
    if (!data?.id) return;
    recordView({
      kind: "outage",
      id: data.id,
      title: [data.number, data.shortDescription].filter((s): s is string => !!s?.trim()).join(" · ") || "(no description)",
      subtitle: data.status ? outageStatusLabel(data.status) : undefined,
      href: `/operations/outages/${data.id}`,
    });
  }, [data, recordView]);

  const back = (): void => navigate(backTarget);

  const BackButton = (
    <Button variant="text" size="small" startIcon={<ArrowLeft size={16} />} onClick={back} sx={{ alignSelf: "flex-start" }}>
      Back
    </Button>
  );

  if (isLoading) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
        <Skeleton variant="rounded" height={32} width={240} />
        <Skeleton variant="rounded" height={260} />
      </Box>
    );
  }

  if (isError) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {BackButton}
        <Typography variant="body1" color="error">
          Could not load outage {id}.
        </Typography>
      </Box>
    );
  }

  if (!data) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {BackButton}
        <Typography variant="h5">Outage not found</Typography>
        <Typography variant="body2" color="text.secondary">
          No outage with id <code>{id}</code>.
        </Typography>
      </Box>
    );
  }

  const outage = data;
  const isOpen = !outage.end;
  const requiresAckToPostExternal = outage.publishesToStatusPage && channel === "external";
  const canPostCommunication =
    body.trim().length > 0 && !addCommunication.isPending && (!requiresAckToPostExternal || ackExternal);

  const onPostCommunication = (): void => {
    if (!id || !canPostCommunication) return;
    addCommunication.mutate(
      { outageId: id, payload: { channel, body: body.trim() } },
      {
        onSuccess: () => {
          setBody("");
          setAckExternal(false);
        },
        onError: (err) => {
          const msg =
            err instanceof BackendApiError && err.status < 500 && err.message
              ? err.message
              : "Could not add the communication. Please try again.";
          showError(msg, err);
        },
      },
    );
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 2.5 }}>
      {BackButton}

      <Box sx={{ display: "flex", gap: 2, alignItems: "flex-start", flexWrap: { xs: "wrap", md: "nowrap" }, justifyContent: "space-between" }}>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 1, flex: 1, minWidth: 0 }}>
          <Typography variant="h6" sx={{ fontFamily: "monospace", fontWeight: 700, letterSpacing: 0.2, lineHeight: 1.2 }}>
            {outage.number || outage.id}
          </Typography>
          <Box sx={{ display: "flex", alignItems: "center", gap: 1, flexWrap: "wrap" }}>
            <Chip size="small" color={outageStatusColor(outage.status)} label={outageStatusLabel(outage.status)} />
            <Chip size="small" variant="outlined" color={outageTypeColor(outage.type)} label={outageTypeLabel(outage.type)} />
            {outage.publishesToStatusPage && (
              <Chip
                size="small"
                color="warning"
                icon={<Megaphone size={14} />}
                label={`Public${outage.statusPageCloud ? ` — ${outage.statusPageCloud}` : ""}`}
              />
            )}
          </Box>
          <Typography variant="h5">{outage.shortDescription || "Outage"}</Typography>
        </Box>
        <Box sx={{ flexShrink: 0, alignSelf: { xs: "stretch", md: "flex-start" } }}>
          <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
            {isOpen ? (
              <Button
                variant="contained"
                size="small"
                color="success"
                startIcon={<CheckCircle size={14} />}
                onClick={() => setCloseOpen(true)}
              >
                Close outage
              </Button>
            ) : (
              <Button
                variant="outlined"
                size="small"
                startIcon={<RotateCcw size={14} />}
                disabled={patchOutage.isPending}
                onClick={() => {
                  if (!id) return;
                  patchOutage.mutate(
                    { id, patch: { end: null } },
                    {
                      onError: (err) => {
                        const msg =
                          err instanceof BackendApiError && err.status < 500 && err.message
                            ? err.message
                            : "Could not reopen the outage. Please try again.";
                        showError(msg, err);
                      },
                    },
                  );
                }}
              >
                Reopen
              </Button>
            )}
            <Button variant="outlined" size="small" startIcon={<Pencil size={14} />} onClick={() => setEditOpen(true)}>
              Edit
            </Button>
          </Box>
        </Box>
      </Box>

      <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
        <Typography variant="subtitle2">Overview</Typography>
        <Box sx={{ display: "grid", gap: 2, gridTemplateColumns: { xs: "1fr", sm: "repeat(2, minmax(0, 1fr))", md: "repeat(3, minmax(0, 1fr))" } }}>
          <MetaCell label="Begin">
            <Typography variant="body2">{formatDateTime(outage.begin)}</Typography>
          </MetaCell>
          <MetaCell label="End">
            <Typography variant="body2">{outage.end ? formatDateTime(outage.end) : "Ongoing"}</Typography>
          </MetaCell>
          <MetaCell label="Duration">
            <Typography variant="body2">{outage.duration || "—"}</Typography>
          </MetaCell>
          <MetaCell label="Configuration item">
            <CiRef value={outage.configurationItem} />
          </MetaCell>
          <MetaCell label="Incident">
            {outage.incident ? (
              <Chip
                size="small"
                variant="outlined"
                clickable
                icon={<LinkIcon size={14} />}
                label={outage.incident.number}
                onClick={() => navigate(`/operations/incidents/${outage.incident?.id}`)}
              />
            ) : (
              <Typography variant="body2">—</Typography>
            )}
          </MetaCell>
          <MetaCell label="Created">
            <Typography variant="body2">{formatDateTime(outage.createdOn)} · {outage.createdBy || "—"}</Typography>
          </MetaCell>
          <MetaCell label="Last updated">
            <Typography variant="body2">{formatDateTime(outage.updatedOn)} · {outage.updatedBy || "—"}</Typography>
          </MetaCell>
        </Box>
        {!!outage.affectedConfigurationItems?.length && (
          <MetaCell label={`Affected configuration items (${outage.affectedConfigurationItems.length})`}>
            <Box sx={{ display: "flex", flexWrap: "wrap", gap: 0.75 }}>
              {outage.affectedConfigurationItems.map((ci) => (
                <Chip key={ci.id} size="small" variant="outlined" label={ci.name} />
              ))}
            </Box>
          </MetaCell>
        )}
      </Card>

      <Card sx={{ p: 2.5, display: "flex", flexDirection: "column", gap: 2 }}>
        <Typography variant="subtitle2">
          Communications{" "}
          {outage.communicationCounts && (
            <Typography component="span" variant="caption" color="text.secondary">
              ({outage.communicationCounts.external} external · {outage.communicationCounts.internal} internal ·{" "}
              {outage.communicationCounts.additional} additional)
            </Typography>
          )}
        </Typography>

        <Box sx={{ display: "flex", flexDirection: "column", gap: 1 }}>
          {(communicationsData?.communications ?? []).length === 0 ? (
            <Typography variant="body2" color="text.secondary">
              No communications yet.
            </Typography>
          ) : (
            communicationsData?.communications.map((c) => (
              <Box key={c.id} sx={{ p: 1.5, border: 1, borderColor: "divider", borderRadius: 1 }}>
                <Box sx={{ display: "flex", justifyContent: "space-between", alignItems: "center", mb: 0.5 }}>
                  <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
                    <Chip size="small" variant="outlined" label={CHANNEL_LABEL[c.channel]} />
                    {c.isPublic && <Chip size="small" color="warning" icon={<Megaphone size={12} />} label="Public" />}
                  </Box>
                  <Typography variant="caption" color="text.secondary">
                    {formatDateTime(c.createdOn)} · {c.createdBy}
                  </Typography>
                </Box>
                <Typography variant="body2" sx={{ whiteSpace: "pre-wrap" }}>
                  {c.body}
                </Typography>
              </Box>
            ))
          )}
        </Box>

        <Box sx={{ display: "flex", flexDirection: "column", gap: 1.5, pt: 1, borderTop: 1, borderColor: "divider" }}>
          <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }}>
            <FormControl size="small" sx={{ minWidth: 200 }} disabled={addCommunication.isPending}>
              <InputLabel id="outage-comm-channel-label">Channel</InputLabel>
              <Select
                labelId="outage-comm-channel-label"
                label="Channel"
                value={channel}
                onChange={(e) => {
                  setChannel(e.target.value as BeOutageCommunicationChannel);
                  setAckExternal(false);
                }}
              >
                <MenuItem value="internal">{CHANNEL_LABEL.internal}</MenuItem>
                <MenuItem value="external">{CHANNEL_LABEL.external}</MenuItem>
                <MenuItem value="additional">{CHANNEL_LABEL.additional}</MenuItem>
              </Select>
            </FormControl>
          </Box>
          {requiresAckToPostExternal && (
            <Alert severity="warning">
              This outage is public. An external communication is echoed verbatim on the
              status page{outage.statusPageCloud ? ` (${outage.statusPageCloud})` : ""}.
              <Button size="small" onClick={() => setAckExternal(true)} sx={{ ml: 1 }} disabled={ackExternal}>
                {ackExternal ? "Acknowledged" : "Acknowledge and continue"}
              </Button>
            </Alert>
          )}
          <TextField
            label="Communication"
            value={body}
            onChange={(e) => setBody(e.target.value)}
            fullWidth
            multiline
            minRows={2}
            disabled={addCommunication.isPending}
          />
          <Box sx={{ display: "flex", justifyContent: "flex-end" }}>
            <Button variant="contained" size="small" onClick={onPostCommunication} disabled={!canPostCommunication}>
              {addCommunication.isPending ? "Posting…" : "Post communication"}
            </Button>
          </Box>
        </Box>
      </Card>

      {editOpen && (
        <EditOutageDialog
          outage={outage}
          isSaving={patchOutage.isPending}
          saveError={
            patchOutage.isError
              ? patchOutage.error instanceof BackendApiError && patchOutage.error.message
                ? patchOutage.error.message
                : "Could not update the outage. Please try again."
              : null
          }
          onClose={() => {
            if (!patchOutage.isPending) setEditOpen(false);
          }}
          onSave={(patch) =>
            patchOutage.mutate(
              { id: outage.id, patch },
              {
                onSuccess: () => setEditOpen(false),
                onError: (err) => {
                  const msg =
                    err instanceof BackendApiError && err.status < 500 && err.message
                      ? err.message
                      : "Could not update the outage. Please try again.";
                  showError(msg, err);
                },
              },
            )
          }
        />
      )}

      {closeOpen && (
        <CloseOutageDialog
          begin={outage.begin}
          isSaving={patchOutage.isPending}
          onClose={() => {
            if (!patchOutage.isPending) setCloseOpen(false);
          }}
          onConfirm={(end) =>
            patchOutage.mutate(
              { id: outage.id, patch: { end } },
              {
                onSuccess: () => setCloseOpen(false),
                onError: (err) => {
                  const msg =
                    err instanceof BackendApiError && err.status < 500 && err.message
                      ? err.message
                      : "Could not close the outage. Please try again.";
                  showError(msg, err);
                },
              },
            )
          }
        />
      )}
    </Box>
  );
}
