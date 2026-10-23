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
  Chip,
  MenuItem,
  Select,
  Skeleton,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { useState, type JSX } from "react";
import { useNavigate, useParams } from "react-router";
import { useSlaCalendars } from "../api/useSlaCalendars";
import {
  useCreateSlaDefinition,
  useSetSlaDefinitionStatus,
  useSlaDefinition,
  useSlaDefinitions,
  useUpdateSlaDefinition,
} from "../api/useSlaDefinitions";
import { useSlaFieldCatalog, useSlaWorkItemTypes } from "../api/useSlaFieldRegistry";
import PredicateConditionEditor from "../components/PredicateConditionEditor";
import ThresholdListEditor from "../components/ThresholdListEditor";
import type {
  SlaDefinitionStatus,
  SlaStartMode,
  SlaTimezoneSource,
  UpsertSlaDefinitionRequest,
} from "../types";
import type { PredicateNode } from "../utils/condition";

const EMPTY: UpsertSlaDefinitionRequest = {
  name: "",
  workItemType: "",
  calendarId: "",
  timezoneSource: "definition",
  definitionTimezone: "UTC",
  startMode: "now",
  targetLabel: "",
  durationSeconds: 3600,
  startCondition: null,
  thresholdPercentages: [100],
};

const STATUS_NEXT: Partial<Record<SlaDefinitionStatus, { to: SlaDefinitionStatus; label: string }[]>> = {
  draft: [
    { to: "published", label: "Publish" },
    { to: "retired", label: "Retire" },
  ],
  published: [{ to: "retired", label: "Retire" }],
};

/**
 * Create/edit an SLA definition (section 6.1/6.1.1/6.1.2). The four
 * condition editors are recursive predicate-tree builders (R4); only the
 * start condition is required (R18). Publishing is its own, separate act
 * (R14) — this page never changes status as a side effect of saving a field
 * edit.
 */
export default function CsmSlaDefinitionEditorPage(): JSX.Element {
  const { id } = useParams<{ id: string }>();
  const isNew = !id || id === "new";
  const navigate = useNavigate();

  const existing = useSlaDefinition(isNew ? undefined : id);
  const calendars = useSlaCalendars();
  const workItemTypes = useSlaWorkItemTypes();
  const allDefinitions = useSlaDefinitions();
  const create = useCreateSlaDefinition();
  const update = useUpdateSlaDefinition();
  const setStatus = useSetSlaDefinitionStatus();

  const [form, setForm] = useState<UpsertSlaDefinitionRequest>(EMPTY);
  const [loadedFromId, setLoadedFromId] = useState<string | undefined>(undefined);

  // Seeds the form from the loaded definition once per id — adjusting state
  // during render (not in an effect), per React's own guidance for
  // "reset state when a prop changes" (react-hooks/set-state-in-effect).
  if (!isNew && existing.data && loadedFromId !== existing.data.id) {
    setForm({
      name: existing.data.name,
      description: existing.data.description,
      workItemType: existing.data.workItemType,
      calendarId: existing.data.calendarId,
      timezoneSource: existing.data.timezoneSource,
      definitionTimezone: existing.data.definitionTimezone,
      accountTimezoneFallback: existing.data.accountTimezoneFallback,
      startMode: existing.data.startMode,
      targetLabel: existing.data.targetLabel,
      durationSeconds: existing.data.durationSeconds,
      pauseReason: existing.data.pauseReason,
      startCondition: existing.data.startCondition as PredicateNode | null,
      pauseCondition: existing.data.pauseCondition as PredicateNode | null | undefined,
      stopCondition: existing.data.stopCondition as PredicateNode | null | undefined,
      cancelCondition: existing.data.cancelCondition as PredicateNode | null | undefined,
      thresholdPercentages: existing.data.thresholdPercentages,
      alsoCompletesDefinitionIds: existing.data.alsoCompletesDefinitionIds,
    });
    setLoadedFromId(existing.data.id);
  }

  const fieldCatalog = useSlaFieldCatalog(form.workItemType);
  const enabledFields = (fieldCatalog.data ?? []).filter((f) => f.enabled);

  if (!isNew && existing.isLoading) return <Skeleton variant="rectangular" height={500} />;
  if (!isNew && existing.isError) return <Alert severity="error">Failed to load the SLA definition.</Alert>;

  const saveError = create.error ?? update.error ?? setStatus.error;
  const saving = create.isPending || update.isPending;

  const canSave = Boolean(
    form.name &&
      form.workItemType &&
      form.calendarId &&
      form.targetLabel &&
      form.durationSeconds > 0 &&
      form.startCondition,
  );

  const save = (): void => {
    if (isNew) {
      create.mutate(form, { onSuccess: (d) => navigate(`/admin/sla/definitions/${d.id}`, { replace: true }) });
    } else if (id) {
      update.mutate({ id, body: form });
    }
  };

  const otherDefinitions = (allDefinitions.data?.definitions ?? []).filter((d) => d.id !== id);

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3, maxWidth: 900 }}>
      {existing.data ? (
        <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
          <Chip label={existing.data.status} />
          {(STATUS_NEXT[existing.data.status] ?? []).map((t) => (
            <Button
              key={t.to}
              size="small"
              variant="outlined"
              disabled={setStatus.isPending}
              onClick={() => id && setStatus.mutate({ id, status: t.to })}
            >
              {t.label}
            </Button>
          ))}
        </Box>
      ) : null}

      <TextField
        label="Name"
        value={form.name}
        onChange={(e) => setForm({ ...form, name: e.target.value })}
        required
      />
      <TextField
        label="Description"
        value={form.description ?? ""}
        onChange={(e) => setForm({ ...form, description: e.target.value })}
        multiline
        minRows={2}
      />

      <Box sx={{ display: "flex", gap: 2 }}>
        <Select
          value={form.workItemType}
          displayEmpty
          onChange={(e) => setForm({ ...form, workItemType: String(e.target.value) })}
          sx={{ minWidth: 200 }}
        >
          <MenuItem value="" disabled>
            Work item type
          </MenuItem>
          {(workItemTypes.data ?? [])
            .filter((t) => t.enabled)
            .map((t) => (
              <MenuItem key={t.workItemType} value={t.workItemType}>
                {t.workItemType}
              </MenuItem>
            ))}
        </Select>
        <Select
          value={form.calendarId}
          displayEmpty
          onChange={(e) => setForm({ ...form, calendarId: String(e.target.value) })}
          sx={{ minWidth: 220 }}
        >
          <MenuItem value="" disabled>
            Calendar
          </MenuItem>
          {(calendars.data ?? []).map((c) => (
            <MenuItem key={c.id} value={c.id}>
              {c.name}
            </MenuItem>
          ))}
        </Select>
      </Box>

      <Box sx={{ display: "flex", gap: 2 }}>
        <Select
          value={form.timezoneSource}
          onChange={(e) =>
            setForm({ ...form, timezoneSource: e.target.value as SlaTimezoneSource })
          }
          sx={{ minWidth: 180 }}
        >
          <MenuItem value="definition">Fixed time zone</MenuItem>
          <MenuItem value="account">Account's time zone</MenuItem>
        </Select>
        {form.timezoneSource === "definition" ? (
          <TextField
            label="Time zone (IANA)"
            value={form.definitionTimezone ?? ""}
            onChange={(e) => setForm({ ...form, definitionTimezone: e.target.value })}
            placeholder="Asia/Colombo"
          />
        ) : (
          <TextField
            label="Fallback time zone (IANA)"
            value={form.accountTimezoneFallback ?? ""}
            onChange={(e) => setForm({ ...form, accountTimezoneFallback: e.target.value })}
            placeholder="UTC"
          />
        )}
      </Box>

      <Box sx={{ display: "flex", gap: 2 }}>
        <TextField
          label="Target label"
          value={form.targetLabel}
          onChange={(e) => setForm({ ...form, targetLabel: e.target.value })}
          placeholder="response / workaround / resolution"
        />
        <TextField
          label="Target duration (minutes)"
          type="number"
          value={Math.round(form.durationSeconds / 60)}
          onChange={(e) => setForm({ ...form, durationSeconds: Number(e.target.value) * 60 })}
        />
        <Select
          value={form.startMode}
          onChange={(e) => setForm({ ...form, startMode: e.target.value as SlaStartMode })}
          sx={{ minWidth: 180 }}
        >
          <MenuItem value="now">Start: now</MenuItem>
          <MenuItem value="retroactive">Start: retroactive</MenuItem>
          <MenuItem value="carry_over">Start: carry over</MenuItem>
        </Select>
      </Box>

      <Box>
        <Typography variant="subtitle1">Thresholds</Typography>
        <ThresholdListEditor
          value={form.thresholdPercentages}
          onChange={(thresholdPercentages) => setForm({ ...form, thresholdPercentages })}
        />
      </Box>

      <Box>
        <Typography variant="subtitle1">Start condition (required)</Typography>
        <PredicateConditionEditor
          value={form.startCondition as PredicateNode | null}
          onChange={(v) => setForm({ ...form, startCondition: v })}
          fields={enabledFields}
          required
        />
      </Box>

      <Box>
        <Typography variant="subtitle1">Pause condition</Typography>
        <TextField
          label="Pause reason"
          size="small"
          sx={{ mb: 1, maxWidth: 300 }}
          value={form.pauseReason ?? ""}
          onChange={(e) => setForm({ ...form, pauseReason: e.target.value })}
        />
        <PredicateConditionEditor
          value={form.pauseCondition as PredicateNode | null | undefined}
          onChange={(v) => setForm({ ...form, pauseCondition: v ?? undefined })}
          fields={enabledFields}
        />
      </Box>

      <Box>
        <Typography variant="subtitle1">Stop condition</Typography>
        <PredicateConditionEditor
          value={form.stopCondition as PredicateNode | null | undefined}
          onChange={(v) => setForm({ ...form, stopCondition: v ?? undefined })}
          fields={enabledFields}
        />
      </Box>

      <Box>
        <Typography variant="subtitle1">Cancel condition</Typography>
        <PredicateConditionEditor
          value={form.cancelCondition as PredicateNode | null | undefined}
          onChange={(v) => setForm({ ...form, cancelCondition: v ?? undefined })}
          fields={enabledFields}
        />
      </Box>

      <Box>
        <Typography variant="subtitle1">Also completes (R20)</Typography>
        <Typography variant="body2" color="text.secondary" sx={{ mb: 1 }}>
          When this definition's instance stops, also complete any running/paused instance of the definitions
          chosen below on the same work item — e.g. a "resolution" definition completing a "workaround" one.
        </Typography>
        <Select
          multiple
          value={form.alsoCompletesDefinitionIds ?? []}
          onChange={(e) =>
            setForm({
              ...form,
              alsoCompletesDefinitionIds:
                typeof e.target.value === "string" ? e.target.value.split(",") : e.target.value,
            })
          }
          renderValue={(selected) =>
            (selected as string[])
              .map((sid) => otherDefinitions.find((d) => d.id === sid)?.name ?? sid)
              .join(", ")
          }
          sx={{ minWidth: 300 }}
        >
          {otherDefinitions.map((d) => (
            <MenuItem key={d.id} value={d.id}>
              {d.name}
            </MenuItem>
          ))}
        </Select>
      </Box>

      {saveError ? <Alert severity="error">{saveError.message}</Alert> : null}

      <Box>
        <Button variant="contained" disabled={!canSave || saving} onClick={save}>
          {isNew ? "Create draft" : "Save"}
        </Button>
      </Box>
    </Box>
  );
}
