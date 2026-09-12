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
  Avatar,
  Box,
  Button,
  Chip,
  DatePickers,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  FormControlLabel,
  LinearProgress,
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
  CaseSeverity,
  CreateTimeCardInput,
  IssueComplexity,
  TimeCardApprover,
} from "@src/types";
import { SEVERITY_LABELS } from "@components/support/config";
import { TimeCardStateChip } from "@components/timecards/TimeCardStateChip";
import { initialsOf } from "@utils/initials";
import { useDebouncedValue } from "@utils/useDebouncedValue";
import {
  ACTIVITY_BUCKETS,
  DEFAULT_BILLABLE,
  DEFAULT_ISSUE_COMPLEXITY,
  ISSUE_COMPLEXITY_OPTIONS,
  MAX_MINUTES_PER_DAY,
  NON_BILLABLE_SEVERITIES,
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

/** One activity row: a labelled whole-minutes input plus a proportion bar (relative to the
 * current logged total, so each bar shows share-of-work, not share-of-day). Mirrors the webapp's
 * ActivityRow. */
function ActivityRow({
  label,
  value,
  total,
  onChange,
  onBlur,
}: {
  label: string;
  value: number;
  total: number;
  onChange: (next: number) => void;
  onBlur: () => void;
}) {
  const pct = total > 0 ? Math.min(100, (value / total) * 100) : 0;
  return (
    <Box sx={{ display: "grid", gridTemplateColumns: "1fr 96px", alignItems: "center", columnGap: 1.5, rowGap: 0.5 }}>
      <Typography variant="body2">{label}</Typography>
      <TextField
        // Stays type="number" — a manually-filtered type="text" turned out unreliable to type
        // into on the microapp's mobile WebView (some builds never fired onChange for digit
        // keys). The native number input is what's actually well-supported for typing everywhere,
        // and already keeps the value numeric; only the increment/decrement spinner is unwanted
        // here, so that's hidden via CSS below instead of swapping the input type.
        type="number"
        size="small"
        value={Number.isFinite(value) ? value : 0}
        onChange={(e) => onChange(Math.max(0, Math.round(Number(e.target.value) || 0)))}
        onBlur={onBlur}
        slotProps={{
          htmlInput: { min: 0, max: MAX_MINUTES_PER_DAY, step: 1, inputMode: "numeric", "aria-label": label },
        }}
        sx={{
          width: 96,
          "& input[type=number]": { MozAppearance: "textfield" },
          "& input[type=number]::-webkit-outer-spin-button, & input[type=number]::-webkit-inner-spin-button": {
            WebkitAppearance: "none",
            margin: 0,
          },
        }}
      />
      <Box sx={{ gridColumn: "1 / -1", mt: -0.25 }}>
        <LinearProgress variant="determinate" value={pct} sx={{ height: 4, borderRadius: 2 }} />
      </Box>
    </Box>
  );
}

interface LogTimeCardDialogProps {
  /** The case the time was spent on — always known, this dialog only opens
   * from a case's Time Tracking tab or its "More" menu. */
  caseId: string;
  caseNumber: string;
  /** Determines whether the billable switch is editable — see NON_BILLABLE_SEVERITIES. A null/
   * missing severity (e.g. a case type that doesn't carry one) is treated the same as a
   * non-billable severity, since only "low" (S4) is ever left editable. */
  caseSeverity: CaseSeverity | null;
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
  caseSeverity,
  projectId,
  projectName,
  isSubmitting,
  error,
  onClose,
  onSubmit,
}: LogTimeCardDialogProps) {
  const me = useUserStore((s) => s.user);

  const isAlwaysNonBillable = !caseSeverity || NON_BILLABLE_SEVERITIES.includes(caseSeverity);

  const [date, setDate] = useState(todayIso());
  const [issueComplexity, setIssueComplexity] = useState<IssueComplexity>(DEFAULT_ISSUE_COMPLEXITY);
  const [billable, setBillable] = useState<boolean>(isAlwaysNonBillable ? false : DEFAULT_BILLABLE);
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
      slotProps={{
        paper: {
          sx: {
            backgroundImage: "none",
            backgroundColor: "background.default",
            // Explicit cap (rather than relying on the default `calc(100% - 64px)`) so the dialog
            // never grows taller than the visible viewport in the microapp's WebView — without it,
            // the last field(s) and the action buttons can end up clipped below the fold with no
            // way to scroll to them. `dvh`, not `vh` — `vh` resolves against the layout viewport,
            // which in a mobile WebView (or a desktop browser with devtools docked, shrinking the
            // available frame) can be taller than what's actually visible, letting the cap sit
            // below the real fold anyway. Matches AttachmentPreviewDialog's use of `100dvh`.
            display: "flex",
            flexDirection: "column",
            maxHeight: "85dvh",
          },
        },
      }}
    >
      <DialogTitle>Log time · {caseNumber}</DialogTitle>
      <DialogContent dividers>
        <Stack gap={2}>
          <Stack direction="row" alignItems="center" justifyContent="space-between" gap={1}>
            <Stack direction="row" alignItems="center" gap={1}>
              <Avatar src={me?.avatarUrl} sx={{ width: 28, height: 28, fontSize: "0.75rem" }}>
                {initialsOf(me?.name ?? "")}
              </Avatar>
              <Typography variant="body2">{me?.name}</Typography>
            </Stack>
            <TimeCardStateChip state="submitted" />
          </Stack>

          <Typography variant="body2" color="text.secondary">
            {projectName || "—"}
          </Typography>

          <LocalizationProvider dateAdapter={AdapterDateFns}>
            <DatePicker
              label="Date"
              value={fromIsoDate(date)}
              // Time can only be logged for today or earlier, never in advance. Both props are
              // set: `disableFuture` is MUI's dedicated flag for this exact rule (robust to any
              // `maxDate`/"now" edge case), `maxDate` pins the same boundary explicitly.
              disableFuture
              maxDate={new Date()}
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
              <ActivityRow
                key={b.key}
                label={b.label}
                value={breakdown[b.key]}
                total={total}
                onChange={(next) => setActivity(b.key, Math.min(MAX_MINUTES_PER_DAY, next))}
                onBlur={() => touch("minutes")}
              />
            ))}
          </Stack>

          <Stack direction="row" alignItems="center" justifyContent="space-between" flexWrap="wrap" gap={1}>
            <TextField
              select
              label="Issue complexity"
              size="small"
              value={issueComplexity}
              onChange={(e) => setIssueComplexity(e.target.value as IssueComplexity)}
              sx={{ minWidth: 120, flexShrink: 0 }}
              slotProps={{ select: { MenuProps: { slotProps: { paper: OPAQUE_POPUP } } } }}
            >
              {ISSUE_COMPLEXITY_OPTIONS.map((o) => (
                <MenuItem key={o} value={o}>
                  {o}
                </MenuItem>
              ))}
            </TextField>
            {/* Bounded so the caption below wraps onto its own line(s) instead of growing this
                column wide enough to push the row into flex-wrap at normal dialog widths, which
                is what put the toggle on its own line under Issue complexity. flexWrap stays on
                the row itself (rather than switching to nowrap) as a fallback for genuinely
                narrow viewports where even 120px (Issue complexity) + 150px (this column) can't
                both fit — there it still wraps instead of clipping. */}
            <Stack alignItems="flex-end" gap={0.25} sx={{ maxWidth: 150 }}>
              <FormControlLabel
                control={
                  <Switch
                    checked={billable}
                    disabled={isAlwaysNonBillable}
                    onChange={(e) => setBillable(e.target.checked)}
                  />
                }
                label={billable ? "Billable" : "Non-billable"}
                labelPlacement="start"
                sx={{ ml: 0 }}
              />
              {isAlwaysNonBillable && (
                <Typography variant="caption" color="text.secondary" sx={{ textAlign: "right" }}>
                  Always non-billable{caseSeverity ? ` for ${SEVERITY_LABELS[caseSeverity]} cases` : ""}.
                </Typography>
              )}
            </Stack>
          </Stack>

          <TextField
            // No `placeholder` here: MUI forces an outlined field's label into a permanently
            // shrunk state whenever a placeholder is also set (so the two never overlap), which
            // reads as the label crowding the top border on this field's theme — every other
            // multiline field in the app (Close notes, lead comment) sticks to label-only for the
            // same reason, so this matches that established, correctly-spaced look.
            label="Work log comment"
            required
            multiline
            minRows={3}
            // Caps how tall the field can grow before it scrolls internally instead — without
            // this a long comment keeps pushing the rest of the form (and the Submit button)
            // further down, which is what made the field clip off-screen in the first place.
            maxRows={6}
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
        <Button variant="contained" onClick={handleSubmit} disabled={isSubmitting}>
          {isSubmitting ? "Submitting…" : "Submit for review"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
