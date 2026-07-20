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

import { useMemo, useState } from "react";
import {
  AdapterDateFns,
  Autocomplete,
  Button,
  Chip,
  DatePickers,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  FormControlLabel,
  MenuItem,
  Stack,
  Switch,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { useQuery } from "@tanstack/react-query";
import { format, parse } from "date-fns";
import { adminUsers } from "@src/services/adminUsers";
import { useUserStore } from "@src/store/user";
import type {
  ActivityBreakdown,
  ActivityKey,
  CreateTimeCardInput,
  IssueComplexity,
  TimeCardApprover,
} from "@src/types";
import { useDebouncedValue } from "@utils/useDebouncedValue";
import {
  ACTIVITY_BUCKETS,
  DEFAULT_BILLABLE,
  DEFAULT_ISSUE_COMPLEXITY,
  ISSUE_COMPLEXITY_OPTIONS,
  WORK_LOG_MAX,
  emptyBreakdown,
  timeCardDraftErrors,
  totalMinutes,
} from "@utils/timecard";

const { LocalizationProvider, DatePicker } = DatePickers;

const ISO_DATE = "yyyy-MM-dd";

// The Acrylic theme renders popup papers translucent — force an opaque surface
// so a picker/dropdown opening over the dialog is actually readable. Mirrors
// TimeCardFiltersSheet's OPAQUE_POPUP.
const OPAQUE_POPUP = { sx: { backgroundColor: "background.default", backgroundImage: "none" } };

function todayIso(): string {
  return format(new Date(), ISO_DATE);
}

function fromIsoDate(iso: string): Date | null {
  if (!iso) return null;
  const d = parse(iso, ISO_DATE, new Date());
  return Number.isNaN(d.getTime()) ? null : d;
}

interface ApproverOption {
  id: string;
  name: string;
  email?: string;
}

interface LogTimeCardDialogProps {
  /** The case the time was spent on — always known, this dialog only opens
   * from a case's Time Tracking tab. */
  caseId: string;
  caseNumber: string;
  projectId: string;
  projectName: string;
  /** True while the create mutation is in flight. */
  isSubmitting: boolean;
  /** Set when the last submit attempt failed — cleared by the caller on retry. */
  error?: string | null;
  onClose: () => void;
  onSubmit: (input: CreateTimeCardInput) => void;
}

/**
 * "Log time" form. Mirrors the webapp's LogTimeCardDialog (same fields: date,
 * the five activity buckets, work-log comment, issue complexity, approver),
 * adapted to this microapp's single-column mobile layout and conventions.
 * Creating a card submits it immediately — the backend has no draft step.
 */
export function LogTimeCardDialog({
  caseId,
  caseNumber,
  projectId,
  projectName,
  isSubmitting,
  error,
  onClose,
  onSubmit,
}: LogTimeCardDialogProps) {
  const me = useUserStore((s) => s.user);

  const [date, setDate] = useState(todayIso());
  const [issueComplexity, setIssueComplexity] = useState<IssueComplexity>(DEFAULT_ISSUE_COMPLEXITY);
  const [billable, setBillable] = useState<boolean>(DEFAULT_BILLABLE);
  const [breakdown, setBreakdown] = useState<ActivityBreakdown>(emptyBreakdown());
  const [workLogComment, setWorkLogComment] = useState("");
  const [approver, setApprover] = useState<TimeCardApprover | null>(null);
  const [approverInput, setApproverInput] = useState("");
  const [touched, setTouched] = useState<Set<string>>(new Set());
  const touch = (field: string): void => setTouched((prev) => new Set(prev).add(field));
  const isTouched = (field: string): boolean => touched.has(field);

  const total = totalMinutes(breakdown);
  const errors = timeCardDraftErrors({
    date,
    breakdown,
    workLogComment,
    approverId: approver?.id,
  });
  const isValid = Object.keys(errors).length === 0;

  const debouncedApproverInput = useDebouncedValue(approverInput.trim(), 300);
  const { data, isFetching } = useQuery(adminUsers.search(debouncedApproverInput));
  const hasApproverInput = approverInput.trim().length > 0;
  // Approvers must be real internal accounts, and never the signed-in user —
  // nothing server-side stops picking yourself, which would let a submitter
  // approve their own time. Mirrors the webapp's LogTimeCardDialog.
  const candidates: ApproverOption[] = useMemo(() => {
    if (!hasApproverInput) return [];
    const myEmail = (me?.email ?? "").toLowerCase();
    return (data?.users ?? [])
      .filter((u) => !!u.email && u.email.toLowerCase() !== myEmail && u.active !== false)
      .map((u) => ({ id: u.id, name: u.name || u.userName, email: u.email }));
  }, [data, hasApproverInput, me?.email]);

  const setActivity = (key: ActivityKey, next: number): void => setBreakdown((prev) => ({ ...prev, [key]: next }));

  const ALL_FIELDS = ["date", "minutes", "workLogComment", "approver"];
  const handleSubmit = (): void => {
    if (!isValid || !approver) {
      setTouched(new Set(ALL_FIELDS));
      return;
    }
    onSubmit({
      caseId,
      caseNumber,
      projectId,
      projectName,
      date,
      breakdown,
      billable,
      workLogComment: workLogComment.trim(),
      issueComplexity,
      approver,
    });
  };

  return (
    <Dialog
      open
      onClose={isSubmitting ? undefined : onClose}
      fullWidth
      maxWidth="xs"
      slotProps={{ paper: { sx: { backgroundImage: "none", backgroundColor: "background.default" } } }}
    >
      <DialogTitle>Log time · {caseNumber}</DialogTitle>
      <DialogContent dividers>
        <Stack gap={2}>
          <Typography variant="body2" color="text.secondary">
            {projectName || "—"}
          </Typography>

          <LocalizationProvider dateAdapter={AdapterDateFns}>
            <DatePicker
              label="Date"
              value={fromIsoDate(date)}
              onChange={(next) => {
                setDate(next instanceof Date && !Number.isNaN(next.getTime()) ? format(next, ISO_DATE) : "");
                touch("date");
              }}
              slotProps={{
                textField: {
                  size: "small",
                  fullWidth: true,
                  required: true,
                  error: isTouched("date") && !!errors.date,
                  helperText: isTouched("date") ? errors.date : undefined,
                },
                field: { clearable: true },
                desktopPaper: OPAQUE_POPUP,
                mobilePaper: OPAQUE_POPUP,
              }}
            />
          </LocalizationProvider>

          <Divider />

          <Stack direction="row" alignItems="baseline" justifyContent="space-between">
            <Typography variant="subtitle2">Time breakdown (minutes)</Typography>
            <Typography variant="subtitle2" color="primary">
              {total} min total
            </Typography>
          </Stack>
          {isTouched("minutes") && errors.minutes && (
            <Typography variant="caption" color="error">
              {errors.minutes}
            </Typography>
          )}
          <Stack gap={1.25}>
            {ACTIVITY_BUCKETS.map((b) => (
              <Stack key={b.key} direction="row" alignItems="center" justifyContent="space-between" gap={1.5}>
                <Typography variant="body2">{b.label}</Typography>
                <TextField
                  type="number"
                  size="small"
                  value={Number.isFinite(breakdown[b.key]) ? breakdown[b.key] : 0}
                  onChange={(e) => setActivity(b.key, Math.max(0, Math.round(Number(e.target.value) || 0)))}
                  onBlur={() => touch("minutes")}
                  slotProps={{ htmlInput: { min: 0, step: 1, "aria-label": b.label } }}
                  sx={{ width: 96 }}
                />
              </Stack>
            ))}
          </Stack>

          <Stack direction="row" alignItems="center" justifyContent="space-between" flexWrap="wrap" gap={1}>
            <TextField
              select
              label="Issue complexity"
              size="small"
              value={issueComplexity}
              onChange={(e) => setIssueComplexity(e.target.value as IssueComplexity)}
              sx={{ minWidth: 140 }}
              slotProps={{ select: { MenuProps: { slotProps: { paper: OPAQUE_POPUP } } } }}
            >
              {ISSUE_COMPLEXITY_OPTIONS.map((o) => (
                <MenuItem key={o} value={o}>
                  {o}
                </MenuItem>
              ))}
            </TextField>
            <FormControlLabel
              control={<Switch checked={billable} onChange={(e) => setBillable(e.target.checked)} />}
              label={billable ? "Billable" : "Non-billable"}
              labelPlacement="start"
              sx={{ ml: 0 }}
            />
          </Stack>

          <TextField
            label="Work log comment"
            required
            multiline
            minRows={3}
            fullWidth
            value={workLogComment}
            onChange={(e) => setWorkLogComment(e.target.value.slice(0, WORK_LOG_MAX))}
            onBlur={() => touch("workLogComment")}
            error={isTouched("workLogComment") && !!errors.workLogComment}
            helperText={
              isTouched("workLogComment") && errors.workLogComment
                ? errors.workLogComment
                : `${WORK_LOG_MAX - workLogComment.length} characters left`
            }
          />

          <Stack gap={0.75}>
            <Typography variant="subtitle2">Approver (team lead)</Typography>
            {approver ? (
              <Chip label={approver.name} onDelete={() => setApprover(null)} sx={{ alignSelf: "flex-start" }} />
            ) : (
              <Autocomplete
                size="small"
                options={candidates}
                loading={isFetching}
                getOptionLabel={(o) => o.name}
                isOptionEqualToValue={(a, b) => a.id === b.id}
                filterOptions={(opts) => opts}
                noOptionsText={!hasApproverInput ? "Type to search for an approver…" : "No matching engineers."}
                onChange={(_, next) => {
                  if (next) {
                    setApprover({ id: next.id, name: next.name });
                    setApproverInput("");
                  }
                }}
                onInputChange={(_, next, reason) => {
                  if (reason === "input") setApproverInput(next);
                  else if (reason === "clear") setApproverInput("");
                }}
                onBlur={() => touch("approver")}
                slotProps={{ paper: OPAQUE_POPUP }}
                renderInput={(params) => (
                  <TextField
                    {...params}
                    placeholder="Search engineers by name or email…"
                    error={isTouched("approver") && !!errors.approver}
                    helperText={isTouched("approver") ? errors.approver : undefined}
                  />
                )}
              />
            )}
          </Stack>

          {error && (
            <Typography variant="body2" color="error">
              {error}
            </Typography>
          )}
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={onClose} disabled={isSubmitting}>
          Cancel
        </Button>
        <Button variant="contained" onClick={handleSubmit} disabled={isSubmitting || (touched.size > 0 && !isValid)}>
          {isSubmitting ? "Submitting…" : "Submit for review"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
