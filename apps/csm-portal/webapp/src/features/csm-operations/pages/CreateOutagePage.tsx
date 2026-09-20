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
  AdapterDateFns,
  Box,
  Button,
  Card,
  DatePickers,
  FormControl,
  InputLabel,
  MenuItem,
  Select,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import { useState, type JSX } from "react";
import { useLocation, useNavigate } from "react-router";
import { BackendApiError } from "@api/backend/client";
import { useErrorBanner } from "@context/error-banner/ErrorBannerContext";
import { useGetOutageMetadata, usePostOutage } from "@features/csm-operations/api/useOutages";
import { useSearchConfigurationItems } from "@api/useSearchConfigurationItems";
import { useSearchIncidentsForSelect } from "@features/csm-operations/api/useSearchIncidentsForSelect";
import AsyncEntitySelect from "@components/AsyncEntitySelect";
import OutagePublicationNotice from "@features/csm-operations/components/OutagePublicationNotice";
import { outageTypeLabel } from "@features/csm-operations/utils/outages";
import { formatDateTimeLocal, parseDateTimeLocal } from "@utils/dateTime";
import type { BeConfigurationItem, BeCreateOutagePayload, BeIncident, BeOutageType } from "@api/backend/types";

const UNSET = "";
const OUTAGE_TYPES: BeOutageType[] = ["outage", "degradation", "planned"];

function configurationItemLabel(c: BeConfigurationItem): string {
  return c.name || c.id;
}

function incidentSearchLabel(i: BeIncident): string {
  return [i.number, i.subject].filter(Boolean).join(" — ") || i.id || "";
}

/** `YYYY-MM-DD HH:mm:ss` UTC, the shape `CHANGES-outage-api.md` documents
 * (ISO-8601 is also accepted, but the plain form avoids any ambiguity about
 * which timezone the wall-clock value the picker shows represents). */
function toBackendDateTime(local: string): string {
  return `${local.replace("T", " ")}:00`;
}

const OPERATIONS_OUTAGES_PATH = "/operations?tab=outages";

/**
 * Create-outage form against `POST /outages` (ServiceNow data source only).
 * A range-entry form, not the ServiceNow Start/Stop button pair the SN form
 * itself uses: `end` is optional here (omit for an ongoing outage; close it
 * later with a PATCH from the detail page). Supports being opened bare
 * (`/operations/outages/new`) or anchored from an incident's own "Create
 * outage" action (`state: { from, incidentId, configurationItemId }`) —
 * both pre-fills are additive best-effort, not required to be perfect: the
 * engineer can still change either before submitting.
 */
export default function CreateOutagePage(): JSX.Element {
  const navigate = useNavigate();
  const { showError } = useErrorBanner();
  const postOutage = usePostOutage();
  const { data: metadata } = useGetOutageMetadata();

  const backState = useLocation().state as
    | { from?: string; incidentId?: string; configurationItemId?: string }
    | undefined;
  const backTarget = backState?.from ?? OPERATIONS_OUTAGES_PATH;

  const [type, setType] = useState<BeOutageType | typeof UNSET>(UNSET);
  const [begin, setBegin] = useState("");
  const [end, setEnd] = useState("");
  const [shortDescription, setShortDescription] = useState("");
  const [configurationItemId, setConfigurationItemId] = useState(
    backState?.configurationItemId ?? "",
  );
  const [incidentId, setIncidentId] = useState(backState?.incidentId ?? "");
  const [externalCommunication, setExternalCommunication] = useState("");
  const [internalCommunication, setInternalCommunication] = useState("");
  const [acknowledged, setAcknowledged] = useState(false);
  const [touched, setTouched] = useState(false);

  const beginDate = parseDateTimeLocal(begin);
  const endDate = parseDateTimeLocal(end);
  const endBeforeBegin = !!beginDate && !!endDate && endDate.getTime() < beginDate.getTime();

  const isTypeValid = type !== UNSET;
  const isBeginValid = !!beginDate;
  const isShortDescriptionValid = shortDescription.trim().length > 0;
  const needsAcknowledgement = !!configurationItemId && !acknowledged;
  const canSubmit =
    isTypeValid &&
    isBeginValid &&
    isShortDescriptionValid &&
    !endBeforeBegin &&
    !needsAcknowledgement &&
    !postOutage.isPending;

  const handleSubmit = (): void => {
    if (!canSubmit) {
      setTouched(true);
      return;
    }

    const payload: BeCreateOutagePayload = {
      type: type as BeOutageType,
      begin: toBackendDateTime(begin),
      shortDescription: shortDescription.trim(),
    };
    if (end) payload.end = toBackendDateTime(end);
    if (configurationItemId) payload.configurationItemId = configurationItemId;
    if (incidentId) payload.incidentId = incidentId;
    if (externalCommunication.trim()) payload.externalCommunication = externalCommunication.trim();
    if (internalCommunication.trim()) payload.internalCommunication = internalCommunication.trim();
    if (configurationItemId) payload.acknowledgePublicPublication = acknowledged;

    postOutage.mutate(payload, {
      onSuccess: (created) =>
        navigate(`/operations/outages/${created.outage.id}`, {
          state: { from: backTarget },
        }),
      onError: (err) => {
        // 409 here is the publication-acknowledgement gate (or, less often,
        // a write-permission refusal) — the real backend message names the
        // cloud, worth surfacing verbatim rather than a generic fallback.
        const msg =
          err instanceof BackendApiError && err.status < 500 && err.message
            ? err.message
            : "Could not create the outage. Please try again.";
        showError(msg, err);
      },
    });
  };

  return (
    <Box sx={{ width: "100%", px: 3, py: 3 }}>
      <Button
        variant="text"
        startIcon={<ArrowLeft size={16} />}
        onClick={() => navigate(backTarget)}
        sx={{ mb: 1 }}
      >
        Back
      </Button>
      <Typography variant="h5" sx={{ mb: 2 }}>
        New outage
      </Typography>

      <Card variant="outlined" sx={{ p: 3 }}>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
          <Typography variant="subtitle2">Outage details</Typography>

          <TextField
            label="Short description"
            value={shortDescription}
            onChange={(e) => setShortDescription(e.target.value)}
            onBlur={() => setTouched(true)}
            fullWidth
            required
            error={touched && !isShortDescriptionValid}
            helperText={
              touched && !isShortDescriptionValid
                ? "Required — this is what appears on the public status page if this outage publishes."
                : undefined
            }
            disabled={postOutage.isPending}
            placeholder="Short summary of the outage"
          />

          <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }}>
            <FormControl
              fullWidth
              size="small"
              required
              disabled={postOutage.isPending}
              sx={{ flex: "1 1 220px" }}
              error={touched && !isTypeValid}
            >
              <InputLabel id="outage-type-label" shrink>
                Type
              </InputLabel>
              <Select
                labelId="outage-type-label"
                label="Type"
                value={type}
                displayEmpty
                onChange={(e) => setType(e.target.value as BeOutageType)}
              >
                <MenuItem value={UNSET}>
                  <Typography component="span" color="text.secondary">
                    -- Select --
                  </Typography>
                </MenuItem>
                {(metadata?.types.map((t) => t.value as BeOutageType) ?? OUTAGE_TYPES).map((t) => (
                  <MenuItem key={t} value={t}>
                    {outageTypeLabel(t)}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
          </Box>

          <DatePickers.LocalizationProvider dateAdapter={AdapterDateFns}>
            <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }}>
              <Box sx={{ flex: "1 1 220px" }}>
                <DatePickers.DateTimePicker
                  label="Begin"
                  value={beginDate}
                  onChange={(next) =>
                    setBegin(
                      next instanceof Date && !Number.isNaN(next.getTime())
                        ? formatDateTimeLocal(next)
                        : "",
                    )
                  }
                  slotProps={{
                    textField: {
                      size: "small",
                      fullWidth: true,
                      required: true,
                      error: touched && !isBeginValid,
                      helperText: touched && !isBeginValid ? "Required" : undefined,
                    },
                  }}
                />
              </Box>
              <Box sx={{ flex: "1 1 220px" }}>
                <DatePickers.DateTimePicker
                  label="End (leave blank if ongoing)"
                  value={endDate}
                  onChange={(next) =>
                    setEnd(
                      next instanceof Date && !Number.isNaN(next.getTime())
                        ? formatDateTimeLocal(next)
                        : "",
                    )
                  }
                  slotProps={{
                    textField: {
                      size: "small",
                      fullWidth: true,
                      error: endBeforeBegin,
                      helperText: endBeforeBegin ? "End must be after begin." : undefined,
                    },
                  }}
                />
              </Box>
            </Box>
          </DatePickers.LocalizationProvider>

          <Typography variant="caption" color="text.secondary">
            Linking
          </Typography>
          <Box sx={{ display: "flex", gap: 2, flexWrap: "wrap" }}>
            <Box sx={{ flex: "1 1 260px" }}>
              <AsyncEntitySelect<BeConfigurationItem>
                id="outage-configuration-item"
                label="Configuration item"
                placeholder="Search configuration items…"
                value={configurationItemId}
                onChange={(next) => {
                  setConfigurationItemId(next);
                  if (!next) setAcknowledged(false);
                }}
                disabled={postOutage.isPending}
                useSearch={useSearchConfigurationItems}
                getId={(c) => c.id}
                getLabel={configurationItemLabel}
                helperText="Must be a Service offering — this is the field that decides whether the outage is publicly visible."
              />
            </Box>
            <Box sx={{ flex: "1 1 260px" }}>
              <AsyncEntitySelect<BeIncident>
                id="outage-incident"
                label="Related incident"
                placeholder="Search incidents…"
                value={incidentId}
                onChange={setIncidentId}
                disabled={postOutage.isPending}
                useSearch={useSearchIncidentsForSelect}
                getId={(i) => i.id!}
                getLabel={incidentSearchLabel}
              />
            </Box>
          </Box>

          <OutagePublicationNotice
            hasConfigurationItem={!!configurationItemId}
            monitoredClouds={metadata?.statusPageClouds}
            acknowledged={acknowledged}
            onAcknowledgedChange={setAcknowledged}
            disabled={postOutage.isPending}
          />

          <Typography variant="caption" color="text.secondary">
            Communications (optional — seeds the journal; more can be added after creation)
          </Typography>
          <TextField
            label="External communication"
            value={externalCommunication}
            onChange={(e) => setExternalCommunication(e.target.value)}
            fullWidth
            multiline
            minRows={2}
            disabled={postOutage.isPending}
            helperText="Visible on the public status page if this outage publishes."
          />
          <TextField
            label="Internal communication"
            value={internalCommunication}
            onChange={(e) => setInternalCommunication(e.target.value)}
            fullWidth
            multiline
            minRows={2}
            disabled={postOutage.isPending}
          />
        </Box>

        <Box sx={{ display: "flex", justifyContent: "flex-end", gap: 1.5, mt: 2.5 }}>
          <Button variant="outlined" onClick={() => navigate(backTarget)}>
            Cancel
          </Button>
          <Button
            variant="contained"
            onClick={handleSubmit}
            disabled={!canSubmit}
            loading={postOutage.isPending}
          >
            Create outage
          </Button>
        </Box>
      </Card>
    </Box>
  );
}
