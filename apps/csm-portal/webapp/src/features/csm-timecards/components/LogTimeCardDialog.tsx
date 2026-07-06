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

import { useMemo, useState, type JSX, type KeyboardEvent } from "react";
import {
  Avatar,
  Box,
  Button,
  Chip,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  FormControlLabel,
  InputAdornment,
  LinearProgress,
  MenuItem,
  Switch,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";
import { Clock, Search, X } from "@wso2/oxygen-ui-icons-react";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useIdTokenClaims } from "@hooks/useIdTokenClaims";
import { initialsOf, resolveUserInfo } from "@utils/userClaims";
import { useSearchUsers } from "@features/csm-users/api/useSearchUsers";
import {
  INTERNAL_USER_ROLES,
  type NormalizedUser,
} from "@features/csm-users/types/csmUsers";
import TimeCardStatusChip from "@features/csm-timecards/components/TimeCardStatusChip";
import {
  ACTIVITY_BUCKETS,
  DEFAULT_BILLABLE,
  DEFAULT_ISSUE_COMPLEXITY,
  ISSUE_COMPLEXITY_OPTIONS,
  WORK_LOG_MAX,
} from "@features/csm-timecards/constants/timeCardConstants";
import type {
  ActivityBreakdown,
  ActivityKey,
  CreateTimeCardInput,
  IssueComplexity,
  TimeCardApprover,
} from "@features/csm-timecards/types/timeCards";
import {
  emptyBreakdown,
  timeCardDraftErrors,
  totalMinutes,
} from "@features/csm-timecards/utils/timeCardTotals";
import { localTodayIso } from "@features/csm-timecards/utils/timeSheetWeek";

interface LogTimeCardDialogProps {
  /** The case the time was spent on — always known, this dialog only opens
   * from a case's Time tracking tab (the backend requires a real case UUID,
   * which only a case context can provide). */
  caseId: string;
  caseNumber: string;
  projectId: string;
  projectName: string;
  /** True while the create mutation is in flight. */
  isSubmitting: boolean;
  onClose: () => void;
  onSubmit: (input: CreateTimeCardInput) => void;
}

interface ApproverOption {
  id: string;
  name: string;
  email?: string;
}

function fullName(u: NormalizedUser): string {
  return u.name.trim() || u.userName;
}

/** One activity row: a labelled whole-minutes input plus a proportion bar
 * (relative to the current logged total so each bar shows share-of-work, not
 * share-of-day). */
function ActivityRow({
  label,
  value,
  total,
  onChange,
  onBlur,
}: {
  label: string;
  value: number;
  /** Running total across all buckets — used to size the proportion bar. */
  total: number;
  onChange: (next: number) => void;
  onBlur?: () => void;
}): JSX.Element {
  const pct = total > 0 ? Math.min(100, (value / total) * 100) : 0;
  return (
    <Box
      sx={{
        display: "grid",
        gridTemplateColumns: "1fr 96px",
        alignItems: "center",
        columnGap: 1.5,
        rowGap: 0.5,
      }}
    >
      <Typography variant="body2">{label}</Typography>
      <TextField
        type="number"
        size="small"
        value={Number.isFinite(value) ? value : 0}
        onChange={(e) => onChange(Math.max(0, Math.round(Number(e.target.value) || 0)))}
        slotProps={{ htmlInput: { min: 0, step: 1, "aria-label": label } }}
        onBlur={onBlur}
        sx={{ width: 96 }}
      />
      <Box sx={{ gridColumn: "1 / -1", mt: -0.25 }}>
        <LinearProgress
          variant="determinate"
          value={pct}
          sx={{ height: 4, borderRadius: 2 }}
        />
      </Box>
    </Box>
  );
}

/**
 * "Log time" form. Mirrors the ServiceNow fields (date, the five activity
 * buckets, work-log comment, issue complexity, approver) with a live
 * running total, per-activity proportion bars, and inline validation. There
 * was a "Category" field here too, but it was never actually sent anywhere
 * (`usePostTimeCard`'s payload has no such field) — removed rather than kept
 * as a choice that silently did nothing.
 * Creating a card submits it immediately — the backend has no draft step, so
 * there is no "Pending" status and no edit-after-create (see the module-level
 * note in `types/timeCards.ts` on why edit isn't supported).
 */
export default function LogTimeCardDialog({
  caseId,
  caseNumber,
  projectId,
  projectName,
  isSubmitting,
  onClose,
  onSubmit,
}: LogTimeCardDialogProps): JSX.Element {
  const me = resolveUserInfo(useIdTokenClaims());

  const [date, setDate] = useState(localTodayIso());
  const [issueComplexity, setIssueComplexity] = useState<IssueComplexity>(
    DEFAULT_ISSUE_COMPLEXITY,
  );
  const [billable, setBillable] = useState<boolean>(DEFAULT_BILLABLE);
  const [breakdown, setBreakdown] = useState<ActivityBreakdown>(emptyBreakdown());
  const [workLogComment, setWorkLogComment] = useState("");
  const [approver, setApprover] = useState<TimeCardApprover | null>(null);
  const [approverInput, setApproverInput] = useState("");
  const [touched, setTouched] = useState<Set<string>>(new Set());
  const touch = (field: string): void =>
    setTouched((prev) => new Set(prev).add(field));
  const isTouched = (field: string): boolean => touched.has(field);

  const total = totalMinutes(breakdown);
  const errors = timeCardDraftErrors({
    date,
    breakdown,
    workLogComment,
    approverId: approver?.id,
  });
  const isValid = Object.keys(errors).length === 0;

  const search = useDebouncedValue(approverInput.trim(), 300);
  const { data } = useSearchUsers({
    filters: {
      ...(search.length > 0 && { searchQuery: search }),
      // Approvers must be real internal accounts — the backend requires a
      // real UUID in `approverIds`, so there is no offline/mock fallback
      // (a fabricated id would always be rejected on submit).
      roles: INTERNAL_USER_ROLES,
      active: true,
    },
    pagination: { limit: 6, offset: 0 },
  });
  const hasApproverInput = approverInput.trim().length > 0;
  // An approver needs an id (always present) and is expected to carry an
  // email; `userType` is postgres-only (absent on the ServiceNow source, the
  // live data here), so only gate on it when present — the `roles`/`active`
  // filters above already restrict server-side. Mirrors AssignEngineerDialog.
  // Excludes the signed-in user: nothing server-side stops picking yourself
  // as approver, which would let a submitter approve their own time.
  const candidates: ApproverOption[] = useMemo(() => {
    if (!hasApproverInput) return [];
    const myEmail = me.email.toLowerCase();
    return (data?.users ?? [])
      .filter(
        (u) =>
          !!u.email &&
          u.email.toLowerCase() !== myEmail &&
          u.active !== false &&
          (u.userType ? u.userType === "internal" : true),
      )
      .map((u) => ({ id: u.id, name: fullName(u), email: u.email }));
  }, [data, hasApproverInput, me.email]);

  const setActivity = (key: ActivityKey, next: number): void =>
    setBreakdown((prev) => ({ ...prev, [key]: next }));

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

  /** Submit on Enter, except inside the multiline work-log (where Enter = newline). */
  const handleKeyDown = (e: KeyboardEvent): void => {
    if (
      e.key === "Enter" &&
      !e.shiftKey &&
      (e.target as HTMLElement).tagName !== "TEXTAREA"
    ) {
      e.preventDefault();
      handleSubmit();
    }
  };

  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>Log time · {caseNumber}</DialogTitle>
      <DialogContent dividers>
        <Box
          onKeyDown={handleKeyDown}
          sx={{ display: "flex", flexDirection: "column", gap: 2 }}
        >
          {/* Engineer + status (auto, read-only) */}
          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              gap: 1,
            }}
          >
            <Box sx={{ display: "flex", alignItems: "center", gap: 1 }}>
              <Avatar
                src={me.avatarUrl}
                sx={{ width: 28, height: 28, fontSize: "0.75rem" }}
              >
                {initialsOf(me.fullName)}
              </Avatar>
              <Typography variant="body2">{me.fullName}</Typography>
            </Box>
            <TimeCardStatusChip state="submitted" />
          </Box>

          <Typography variant="body2" color="text.secondary">
            Task: {caseNumber}
            {projectName ? ` · ${projectName}` : ""}
          </Typography>

          <TextField
            type="date"
            label="Date"
            size="small"
            required
            value={date}
            onChange={(e) => setDate(e.target.value)}
            onBlur={() => touch("date")}
            error={isTouched("date") && !!errors.date}
            helperText={isTouched("date") ? errors.date : undefined}
            slotProps={{ inputLabel: { shrink: true } }}
            sx={{ maxWidth: { sm: 220 }, minWidth: 160 }}
          />

          <Divider />

          {/* Time breakdown */}
          <Box
            sx={{
              display: "flex",
              alignItems: "baseline",
              justifyContent: "space-between",
            }}
          >
            <Typography variant="subtitle2">Time breakdown (minutes)</Typography>
            <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
              <Clock size={14} />
              <Typography variant="subtitle2" color="primary">
                {total} min total
              </Typography>
            </Box>
          </Box>
          {isTouched("minutes") && errors.minutes && (
            <Typography variant="caption" color="error">
              {errors.minutes}
            </Typography>
          )}
          <Box sx={{ display: "flex", flexDirection: "column", gap: 1.25 }}>
            {ACTIVITY_BUCKETS.map((b) => (
              <ActivityRow
                key={b.key}
                label={b.label}
                value={breakdown[b.key]}
                total={total}
                onChange={(next) => setActivity(b.key, next)}
                onBlur={() => touch("minutes")}
              />
            ))}
          </Box>

          <Box
            sx={{
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              flexWrap: "wrap",
              gap: 1,
            }}
          >
            <TextField
              select
              label="Issue complexity"
              size="small"
              value={issueComplexity}
              onChange={(e) => setIssueComplexity(e.target.value as IssueComplexity)}
              sx={{ maxWidth: { sm: 220 }, minWidth: 160 }}
            >
              {ISSUE_COMPLEXITY_OPTIONS.map((o) => (
                <MenuItem key={o} value={o}>
                  {o}
                </MenuItem>
              ))}
            </TextField>
            <FormControlLabel
              control={
                <Switch
                  checked={billable}
                  onChange={(e) => setBillable(e.target.checked)}
                />
              }
              label={billable ? "Billable" : "Non-billable"}
              labelPlacement="start"
              sx={{ ml: 0 }}
            />
          </Box>

          {/* Work log */}
          <TextField
            label="Work log comment"
            required
            multiline
            minRows={3}
            value={workLogComment}
            onChange={(e) =>
              setWorkLogComment(e.target.value.slice(0, WORK_LOG_MAX))
            }
            onBlur={() => touch("workLogComment")}
            error={isTouched("workLogComment") && !!errors.workLogComment}
            helperText={
              isTouched("workLogComment") && errors.workLogComment
                ? errors.workLogComment
                : `${WORK_LOG_MAX - workLogComment.length} characters left`
            }
          />

          {/* Approver */}
          <Box sx={{ display: "flex", flexDirection: "column", gap: 0.75 }}>
            <Typography variant="subtitle2">Approver (team lead)</Typography>
            {approver ? (
              <Chip
                label={approver.name}
                onDelete={() => setApprover(null)}
                deleteIcon={<X size={14} />}
                sx={{ alignSelf: "flex-start" }}
              />
            ) : (
              <>
                <TextField
                  size="small"
                  placeholder="Search engineers by name or email…"
                  value={approverInput}
                  onChange={(e) => setApproverInput(e.target.value)}
                  onBlur={() => touch("approver")}
                  error={isTouched("approver") && !!errors.approver}
                  helperText={isTouched("approver") ? errors.approver : undefined}
                  slotProps={{
                    input: {
                      startAdornment: (
                        <InputAdornment position="start">
                          <Search size={16} />
                        </InputAdornment>
                      ),
                    },
                  }}
                />
                <Box
                  sx={{
                    display: "flex",
                    flexDirection: "column",
                    border: 1,
                    borderColor: "divider",
                    borderRadius: 1,
                    maxHeight: 180,
                    overflowY: "auto",
                  }}
                >
                  {!hasApproverInput ? (
                    <Typography
                      variant="caption"
                      color="text.secondary"
                      sx={{ p: 1 }}
                    >
                      Start typing to search for an approver.
                    </Typography>
                  ) : candidates.length === 0 ? (
                    <Typography
                      variant="caption"
                      color="text.secondary"
                      sx={{ p: 1 }}
                    >
                      No matching engineers.
                    </Typography>
                  ) : (
                    candidates.map((u) => (
                      <Button
                        key={u.id}
                        data-testid="approver-candidate"
                        variant="text"
                        color="inherit"
                        onClick={() => {
                          setApprover({ id: u.id, name: u.name });
                          setApproverInput("");
                        }}
                        sx={{
                          justifyContent: "flex-start",
                          textTransform: "none",
                          px: 1,
                          py: 0.5,
                          gap: 1,
                        }}
                      >
                        <Avatar sx={{ width: 24, height: 24, fontSize: "0.7rem" }}>
                          {initialsOf(u.name)}
                        </Avatar>
                        <Box sx={{ minWidth: 0, textAlign: "left" }}>
                          <Typography variant="body2" noWrap>
                            {u.name}
                          </Typography>
                          {u.email && (
                            <Typography
                              variant="caption"
                              color="text.secondary"
                              noWrap
                              sx={{ display: "block" }}
                            >
                              {u.email}
                            </Typography>
                          )}
                        </Box>
                      </Button>
                    ))
                  )}
                </Box>
              </>
            )}
          </Box>
        </Box>
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={onClose} disabled={isSubmitting}>
          Cancel
        </Button>
        <Button
          variant="contained"
          onClick={handleSubmit}
          disabled={isSubmitting || (touched.size > 0 && !isValid)}
        >
          Submit for review
        </Button>
      </DialogActions>
    </Dialog>
  );
}
