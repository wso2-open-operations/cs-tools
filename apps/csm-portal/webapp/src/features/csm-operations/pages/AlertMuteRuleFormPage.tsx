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
  Alert,
  Autocomplete,
  Box,
  Button,
  DatePickers,
  FormControl,
  FormControlLabel,
  FormLabel,
  Paper,
  Radio,
  RadioGroup,
  Skeleton,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { ArrowLeft } from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState, type JSX, type ReactNode } from "react";
import { useLocation, useParams } from "react-router";
import { useNavTransition } from "@hooks/useNavTransition";
import QueryErrorState from "@components/QueryErrorState";
import {
  formatBackendTimestampForDisplay,
  listSupportedTimeZones,
  resolveDisplayTimeZone,
  utcMsToZonedInputValue,
} from "@utils/dateTime";
import { useGetAlertMuteRule, usePatchAlertMuteRule, usePostAlertMuteRule } from "@features/csm-operations/api/useAlertMuteRules";
import type { AlertMuteRuleCondition, AlertMuteRuleInput, AlertMuteRule } from "@features/csm-operations/types/alertMuteRules";
import {
  MAX_ALERT_MUTE_RULE_DESCRIPTION,
  MAX_ALERT_MUTE_RULE_NAME,
  REQUIRED_ALERT_MUTE_RULE_FIELDS,
  alertMuteRuleState,
  formatDuration,
  pickerInstant,
  pickerValuesInZone,
  timeZoneLabel,
  useNow,
  validateAlertMuteRule,
  type AlertMuteRuleErrors,
} from "@features/csm-operations/utils/alertMuteRules";
import AlertMuteRuleConditionsEditor from "@features/csm-operations/components/AlertMuteRuleConditionsEditor";

const { DatePicker, TimePicker, LocalizationProvider } = DatePickers;

const LIST_PATH = "/operations/alert-mute-rules";

/** Quick lengths, in minutes, that start when the rule is created. */
const QUICK_DURATIONS_MIN = [30, 60, 120] as const;
const DEFAULT_DURATION_MIN = 60;

interface FormState {
  name: string;
  description: string;
  /** A quick length starting at Create, or "custom" for picked start and end times. */
  duration: (typeof QUICK_DURATIONS_MIN)[number] | "custom";
  timeZone: string;
  /** Picker values: only their wall-clock parts are read, in `timeZone`. */
  startDate: Date | null;
  startTime: Date | null;
  endDate: Date | null;
  endTime: Date | null;
  conditions: AlertMuteRuleCondition[];
}

function emptyForm(): FormState {
  return {
    name: "",
    description: "",
    duration: DEFAULT_DURATION_MIN,
    timeZone: resolveDisplayTimeZone(),
    startDate: null,
    startTime: null,
    endDate: null,
    endTime: null,
    conditions: REQUIRED_ALERT_MUTE_RULE_FIELDS.map((field) => ({ field, value: "" })),
  };
}

function formFromRule(rule: AlertMuteRule): FormState {
  const start = pickerValuesInZone(rule.startOn, rule.timeZone);
  const end = pickerValuesInZone(rule.endOn, rule.timeZone);
  return {
    name: rule.name,
    description: rule.description,
    duration: "custom",
    timeZone: rule.timeZone,
    startDate: start.date,
    startTime: start.time,
    endDate: end.date,
    endTime: end.time,
    conditions: rule.conditions,
  };
}

/** The form as the API takes it; a quick length starts at `now`. */
function toInput(form: FormState, now: number): AlertMuteRuleInput {
  const instant = (date: Date | null, time: Date | null): string => pickerInstant(date, time, form.timeZone);
  const quick = form.duration !== "custom";
  return {
    name: form.name.trim(),
    description: form.description.trim(),
    timeZone: form.timeZone,
    startOn: quick ? new Date(now).toISOString() : instant(form.startDate, form.startTime),
    endOn: quick ? new Date(now + (form.duration as number) * 60_000).toISOString() : instant(form.endDate, form.endTime),
    conditions: form.conditions.map((c) => ({ field: c.field, value: c.value.trim() })),
  };
}

function Section({ title, subtitle, children }: { title: string; subtitle?: string; children: ReactNode }): JSX.Element {
  return (
    <Paper variant="outlined" sx={{ p: 3, display: "flex", flexDirection: "column", gap: 2.5 }}>
      <Box>
        <Typography variant="h6">{title}</Typography>
        {subtitle && (
          <Typography variant="body2" color="text.secondary">
            {subtitle}
          </Typography>
        )}
      </Box>
      {children}
    </Paper>
  );
}

/**
 * Create (`/operations/alert-mute-rules/new`) or edit (`…/:id/edit`) an alert
 * rule, laid out like CloudWatch's "Create alarm mute rule": the rule's
 * details, a one-time schedule entered in a chosen time zone, and what to mute
 * (key/value conditions on the alert's fields, always including service and
 * environment). Only a rule that hasn't started can be edited.
 */
export default function AlertMuteRuleFormPage(): JSX.Element {
  const { id } = useParams<{ id: string }>();
  const isEdit = !!id;
  const navigate = useNavTransition();
  const nav = useLocation().state as { from?: string; parentState?: unknown } | undefined;
  const backTarget = nav?.from ?? LIST_PATH;
  const backState = nav?.parentState ?? undefined;
  const existing = useGetAlertMuteRule(id);
  const create = usePostAlertMuteRule();
  const update = usePatchAlertMuteRule();
  const saving = create.isPending || update.isPending;
  const saveError = create.error?.message ?? update.error?.message ?? null;
  const timeZones = useMemo(() => listSupportedTimeZones(), []);
  const now = useNow();

  const [form, setForm] = useState<FormState>(emptyForm);
  const [loadedId, setLoadedId] = useState<string | null>(null);
  const [errors, setErrors] = useState<AlertMuteRuleErrors>({});

  // Seed the form once the rule loads (state adjusted during render, not in an effect).
  if (existing.data && existing.data.id !== loadedId) {
    setForm(formFromRule(existing.data));
    setLoadedId(existing.data.id);
  }

  const set = <K extends keyof FormState>(key: K, value: FormState[K], clears?: keyof AlertMuteRuleErrors): void => {
    setForm((f) => ({ ...f, [key]: value }));
    if (clears) setErrors((e) => ({ ...e, [clears]: undefined }));
  };

  const save = (): void => {
    const at = Date.now();
    const input = toInput(form, at);
    const found = validateAlertMuteRule(input, at, !isEdit);
    setErrors(found);
    if (Object.keys(found).length > 0) return;
    // Opened from the rule's own page (Edit), the saved rule's page keeps that page's state; otherwise its Back
    // returns to wherever this form was opened from.
    const onSuccess = (rule: AlertMuteRule): void => {
      const detail = `${LIST_PATH}/${rule.id}`;
      navigate(detail, {
        replace: true,
        state: backTarget === detail ? backState : { from: backTarget, parentState: backState ?? null },
      });
    };
    if (isEdit && id) update.mutate({ id, input }, { onSuccess });
    else create.mutate(input, { onSuccess });
  };

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

  if (isEdit && (existing.isLoading || (existing.data && loadedId !== existing.data.id))) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {back}
        <Skeleton variant="rounded" height={320} />
      </Box>
    );
  }
  if (isEdit && existing.isError) {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {back}
        <QueryErrorState message={existing.error.message} error={existing.error} />
      </Box>
    );
  }
  if (isEdit && existing.data && alertMuteRuleState(existing.data, now) !== "scheduled") {
    return (
      <Box sx={{ display: "flex", flexDirection: "column", gap: 2 }}>
        {back}
        <Alert severity="warning">This alert mute rule has already started, so it can only be extended or ended.</Alert>
      </Box>
    );
  }

  const preview = toInput(form, now);
  const start = Date.parse(preview.startOn);
  const end = Date.parse(preview.endOn);
  const today = utcMsToZonedInputValue(now, form.timeZone).slice(0, 10).replaceAll("-", "/");

  const dateTimeRow = (
    which: "start" | "end",
    label: string,
    hint: string,
  ): JSX.Element => {
    const dateKey = which === "start" ? "startDate" : "endDate";
    const timeKey = which === "start" ? "startTime" : "endTime";
    const errorKey = which === "start" ? "startOn" : "endOn";
    const error = errors[errorKey];
    return (
      <Box sx={{ flex: "1 1 340px", display: "flex", flexDirection: "column", gap: 1 }}>
        <Box>
          <Typography variant="subtitle2">{label}</Typography>
          <Typography variant="caption" color="text.secondary">
            {hint}
          </Typography>
        </Box>
        <Box sx={{ display: "flex", gap: 1.5, alignItems: "flex-start" }}>
          <DatePicker
            label="Date"
            format="yyyy/MM/dd"
            value={form[dateKey]}
            onChange={(v: unknown) => set(dateKey, v instanceof Date ? v : null, errorKey)}
            slotProps={{
              textField: {
                size: "small",
                required: true,
                error: !!error,
                helperText: `For example: ${today}`,
                sx: { flex: 1 },
              },
            }}
          />
          <TimePicker
            label="Time"
            ampm={false}
            format="HH:mm"
            value={form[timeKey]}
            onChange={(v: unknown) => set(timeKey, v instanceof Date ? v : null, errorKey)}
            slotProps={{
              textField: {
                size: "small",
                required: true,
                error: !!error,
                helperText: "24-hour, for example 09:19",
                sx: { flex: 1 },
              },
            }}
          />
        </Box>
        {error && (
          <Typography variant="caption" color="error" role="alert">
            {error}
          </Typography>
        )}
      </Box>
    );
  };

  return (
    <Box sx={{ display: "flex", flexDirection: "column", gap: 3 }}>
      {back}
      <Typography variant="h5">{isEdit ? "Edit alert mute rule" : "Create alert mute rule"}</Typography>

      <Section title="Alert mute rule details">
        <TextField
          label="Alert mute rule name"
          value={form.name}
          onChange={(e) => set("name", e.target.value, "name")}
          required
          size="small"
          fullWidth
          error={!!errors.name}
          helperText={errors.name}
          slotProps={{ htmlInput: { maxLength: MAX_ALERT_MUTE_RULE_NAME } }}
        />
        <TextField
          label="Description (optional)"
          value={form.description}
          onChange={(e) => set("description", e.target.value, "description")}
          multiline
          minRows={3}
          fullWidth
          placeholder="Add context to help you remember what this alert mute rule is for."
          error={!!errors.description}
          helperText={errors.description}
          slotProps={{ htmlInput: { maxLength: MAX_ALERT_MUTE_RULE_DESCRIPTION } }}
        />
      </Section>

      <Section title="Schedule">
        <FormControl component="fieldset" fullWidth>
          <FormLabel component="legend" id="alert-mute-rule-duration-label" sx={{ mb: 1 }}>
            Duration
          </FormLabel>
          <RadioGroup
            aria-labelledby="alert-mute-rule-duration-label"
            value={String(form.duration)}
            onChange={(e) => {
              const v = e.target.value;
              set("duration", v === "custom" ? "custom" : (Number(v) as (typeof QUICK_DURATIONS_MIN)[number]));
              setErrors((er) => ({ ...er, startOn: undefined, endOn: undefined }));
            }}
            sx={{ gap: 1, maxWidth: 420 }}
          >
            {[...QUICK_DURATIONS_MIN, "custom" as const].map((d) => {
              const selected = String(form.duration) === String(d);
              return (
                <FormControlLabel
                  key={d}
                  value={String(d)}
                  control={<Radio size="small" />}
                  label={d === "custom" ? "Custom" : formatDuration(d * 60_000)}
                  sx={{
                    m: 0,
                    width: "100%",
                    pr: 2,
                    pl: 1,
                    py: 0.75,
                    border: 1,
                    borderRadius: 1,
                    borderColor: selected ? "primary.main" : "divider",
                    bgcolor: selected ? "action.selected" : "transparent",
                    "&:hover": { borderColor: selected ? "primary.main" : "text.secondary" },
                  }}
                />
              );
            })}
          </RadioGroup>
        </FormControl>
        {form.duration === "custom" ? (
          <>
            <Autocomplete
              size="small"
              options={timeZones}
              value={form.timeZone || null}
              getOptionLabel={(tz) => timeZoneLabel(tz)}
              onChange={(_, tz) => set("timeZone", tz ?? "", "timeZone")}
              renderInput={(params) => (
                <TextField
                  {...params}
                  label="Time zone"
                  required
                  error={!!errors.timeZone}
                  helperText={errors.timeZone ?? "Time zone for the schedule."}
                />
              )}
            />
            <LocalizationProvider dateAdapter={AdapterDateFns}>
              <Box sx={{ display: "flex", gap: 3, flexWrap: "wrap" }}>
                {dateTimeRow("start", "Start date and time", "When you want the alerts to be muted from.")}
                {dateTimeRow("end", "End date and time", "When you want the alerts to page again.")}
              </Box>
            </LocalizationProvider>
            {!Number.isNaN(start) && !Number.isNaN(end) && end > start && (
              <Typography variant="body2" color="text.secondary" role="status">
                {`Mutes alerts for ${formatDuration(end - start)}, until ${
                  formatBackendTimestampForDisplay(preview.endOn, { dateStyle: "medium", timeStyle: "short" }) ?? "—"
                } in your time zone.`}
              </Typography>
            )}
          </>
        ) : (
          <Typography variant="body2" color="text.secondary" role="status">
            {`Mutes alerts from when you create the rule until about ${
              formatBackendTimestampForDisplay(preview.endOn, { timeStyle: "short" }) ?? "—"
            }, ${timeZoneLabel(resolveDisplayTimeZone(), Date.parse(preview.endOn))}.`}
          </Typography>
        )}
      </Section>

      <Section
        title="What to mute"
        subtitle="Alerts are muted only when they match every condition. Values ignore case, and environment names match their usual spellings (prod, PROD and production are the same)."
      >
        <AlertMuteRuleConditionsEditor
          conditions={form.conditions}
          onChange={(next) => {
            set("conditions", next);
            setErrors((e) => ({ ...e, conditions: undefined, conditionRows: undefined }));
          }}
          error={errors.conditions}
          rowErrors={errors.conditionRows}
        />
      </Section>

      {saveError && (
        <Alert severity="error" role="alert">
          {saveError}
        </Alert>
      )}

      <Box sx={{ display: "flex", justifyContent: "flex-end", gap: 1 }}>
        <Button color="inherit" onClick={() => navigate(backTarget, { state: backState })} disabled={saving}>
          Cancel
        </Button>
        <Button variant="contained" onClick={save} loading={saving}>
          {isEdit ? "Save changes" : "Create alert mute rule"}
        </Button>
      </Box>
    </Box>
  );
}
