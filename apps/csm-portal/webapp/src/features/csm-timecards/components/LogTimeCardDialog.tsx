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
  Alert,
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
import type { NormalizedUser } from "@features/csm-users/types/csmUsers";
import TimeCardStatusChip from "@features/csm-timecards/components/TimeCardStatusChip";
import {
  ACTIVITY_BUCKETS,
  DEFAULT_BILLABLE,
  DEFAULT_ISSUE_COMPLEXITY,
  DEFAULT_TIME_CARD_CATEGORY,
  ISSUE_COMPLEXITY_OPTIONS,
  MOCK_APPROVERS,
  TASK_TYPE_LABEL,
  TASK_TYPES,
  TIME_CARD_CATEGORIES,
  WORK_LOG_MAX,
} from "@features/csm-timecards/constants/timeCardConstants";
import type {
  ActivityBreakdown,
  ActivityKey,
  CreateTimeCardInput,
  CsmTimeCard,
  IssueComplexity,
  TaskType,
  TimeCardApprover,
  TimeCardCategory,
} from "@features/csm-timecards/types/timeCards";
import {
  emptyBreakdown,
  timeCardDraftErrors,
  totalHours,
} from "@features/csm-timecards/utils/timeCardTotals";
import { localTodayIso } from "@features/csm-timecards/utils/timeSheetWeek";

interface LogTimeCardDialogProps {
  /** Preset task id; omitted when logging from the standalone page. */
  caseId?: string;
  /** Preset task reference; when omitted the user types one (free text). */
  caseNumber?: string;
  /** Project of the parent case (passed from case detail; carried onto the card). */
  projectId?: string;
  projectName?: string;
  /** Preset task type (case tab → "case", grid → the row's type). */
  taskType?: TaskType;
  /** Lock the date to a specific day (grid cell create). */
  presetDate?: string;
  /** When set, the dialog edits this card (prefilled) instead of creating one. */
  editing?: CsmTimeCard;
  /** Existing cards used to warn about a duplicate entry (same case + day). */
  existingCards?: CsmTimeCard[];
  /** True while the create/update mutation is in flight. */
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

/** One activity row: a labelled hours input plus a proportion bar (relative to
 * the current logged total so each bar shows share-of-work, not share-of-day). */
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
        onChange={(e) => onChange(Math.max(0, Number(e.target.value) || 0))}
        slotProps={{ htmlInput: { min: 0, step: 0.25, "aria-label": label } }}
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
 * Improved "Create Time Card" form. Mirrors the ServiceNow fields (date,
 * category, the five activity buckets, work-log comment, issue complexity,
 * approver) but adds a live running total, per-activity proportion bars, an
 * auto-filled engineer + Pending status, and inline validation. The task is
 * preset when opened from a case; on the standalone page the user types it.
 */
export default function LogTimeCardDialog({
  caseId,
  caseNumber,
  projectId,
  projectName,
  taskType: taskTypeProp,
  presetDate,
  editing,
  existingCards,
  isSubmitting,
  onClose,
  onSubmit,
}: LogTimeCardDialogProps): JSX.Element {
  const me = resolveUserInfo(useIdTokenClaims());
  const effectiveCaseNumber = caseNumber ?? editing?.caseNumber;
  const effectiveCaseId = caseId ?? editing?.caseId;
  const effectiveProjectId = projectId ?? editing?.projectId ?? "";
  const effectiveProjectName = projectName ?? editing?.projectName ?? "";
  const presetCase = !!effectiveCaseNumber;

  const [taskType, setTaskType] = useState<TaskType>(
    taskTypeProp ?? editing?.taskType ?? "case",
  );
  const [caseInput, setCaseInput] = useState(effectiveCaseNumber ?? "");
  const [date, setDate] = useState(presetDate ?? editing?.date ?? localTodayIso());
  const [category, setCategory] = useState<TimeCardCategory>(
    editing?.category ?? DEFAULT_TIME_CARD_CATEGORY,
  );
  const [issueComplexity, setIssueComplexity] = useState<IssueComplexity>(
    editing?.issueComplexity ?? DEFAULT_ISSUE_COMPLEXITY,
  );
  const [billable, setBillable] = useState<boolean>(
    editing?.billable ?? DEFAULT_BILLABLE,
  );
  const [breakdown, setBreakdown] = useState<ActivityBreakdown>(
    editing ? { ...editing.breakdown } : emptyBreakdown(),
  );
  const [workLogComment, setWorkLogComment] = useState(
    editing?.workLogComment ?? "",
  );
  const [approver, setApprover] = useState<TimeCardApprover | null>(
    editing?.approvers[0] ?? null,
  );
  const [approverInput, setApproverInput] = useState("");
  const [touched, setTouched] = useState<Set<string>>(new Set());
  const touch = (field: string): void =>
    setTouched((prev) => new Set(prev).add(field));
  const isTouched = (field: string): boolean => touched.has(field);

  const total = totalHours(breakdown);
  const errors = timeCardDraftErrors({
    date,
    breakdown,
    workLogComment,
    approverId: approver?.id,
  });
  const caseError =
    !presetCase && !caseInput.trim() ? "Enter a case number." : undefined;
  const isValid = Object.keys(errors).length === 0 && !caseError;

  // Soft warning (not a blocker): an entry already exists for this case on this
  // day — the engineer probably means to edit it rather than create a second one.
  const resolvedCaseNumber = (effectiveCaseNumber ?? caseInput).trim();
  const duplicateCard =
    resolvedCaseNumber && date
      ? (existingCards ?? []).find(
          (c) =>
            c.id !== editing?.id &&
            c.caseNumber === resolvedCaseNumber &&
            c.date === date,
        )
      : undefined;

  const search = useDebouncedValue(approverInput.trim(), 300);
  const { data } = useSearchUsers({
    ...(search.length > 0 && { searchQuery: search }),
    pagination: { limit: 6, offset: 0 },
  });
  // Live results when the backend answers; otherwise a static mock list so the
  // approver can still be chosen offline (FE-first).
  const hasApproverInput = approverInput.trim().length > 0;
  const candidates: ApproverOption[] = useMemo(() => {
    if (!hasApproverInput) return [];
    const live = (data?.users ?? [])
      .filter((u) => u.userType === "internal" && !!u.email)
      .map((u) => ({ id: u.id, name: fullName(u), email: u.email }));
    if (live.length > 0) return live;
    const q = approverInput.trim().toLowerCase();
    return MOCK_APPROVERS.filter((a) => a.name.toLowerCase().includes(q));
  }, [data, approverInput, hasApproverInput]);

  const setActivity = (key: ActivityKey, next: number): void =>
    setBreakdown((prev) => ({ ...prev, [key]: next }));

  const ALL_FIELDS = ["case", "date", "hours", "workLogComment", "approver"];
  const handleSubmit = (): void => {
    if (!isValid || !approver) {
      setTouched(new Set(ALL_FIELDS));
      return;
    }
    const resolvedNumber = (effectiveCaseNumber ?? caseInput).trim();
    onSubmit({
      taskType,
      caseId: effectiveCaseId ?? resolvedNumber,
      caseNumber: resolvedNumber,
      projectId: effectiveProjectId,
      projectName: effectiveProjectName,
      date,
      category,
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
      <DialogTitle>
        {editing ? "Edit time card" : "Log time"}
        {effectiveCaseNumber ? ` · ${effectiveCaseNumber}` : ""}
      </DialogTitle>
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
            <TimeCardStatusChip state="pending" />
          </Box>

          {/* Task (preset → read-only; standalone → type + reference) */}
          {presetCase ? (
            <Typography variant="body2" color="text.secondary">
              Task: {TASK_TYPE_LABEL[taskType]} · {effectiveCaseNumber}
              {effectiveProjectName ? ` · ${effectiveProjectName}` : ""}
            </Typography>
          ) : (
            <Box
              sx={{
                display: "grid",
                gap: 1.5,
                gridTemplateColumns: { xs: "1fr", sm: "180px 1fr" },
              }}
            >
              <TextField
                select
                label="Task type"
                size="small"
                value={taskType}
                onChange={(e) => setTaskType(e.target.value as TaskType)}
              >
                {TASK_TYPES.map((t) => (
                  <MenuItem key={t.key} value={t.key}>
                    {t.label}
                  </MenuItem>
                ))}
              </TextField>
              <TextField
                label="Task reference"
                size="small"
                required
                placeholder="e.g. CS0352584"
                value={caseInput}
                onChange={(e) => setCaseInput(e.target.value)}
                onBlur={() => touch("case")}
                error={isTouched("case") && !!caseError}
                helperText={isTouched("case") ? caseError : undefined}
              />
            </Box>
          )}

          {/* Date + category */}
          <Box
            sx={{
              display: "grid",
              gap: 1.5,
              gridTemplateColumns: { xs: "1fr", sm: "1fr 1fr" },
            }}
          >
            <TextField
              type="date"
              label="Date"
              size="small"
              required
              disabled={!!presetDate}
              value={date}
              onChange={(e) => setDate(e.target.value)}
              onBlur={() => touch("date")}
              error={isTouched("date") && !!errors.date}
              helperText={isTouched("date") ? errors.date : undefined}
              slotProps={{ inputLabel: { shrink: true } }}
            />
            <TextField
              select
              label="Category"
              size="small"
              value={category}
              onChange={(e) => setCategory(e.target.value as TimeCardCategory)}
            >
              {TIME_CARD_CATEGORIES.map((c) => (
                <MenuItem key={c} value={c}>
                  {c}
                </MenuItem>
              ))}
            </TextField>
          </Box>

          {duplicateCard && (
            <Alert severity="warning" sx={{ py: 0 }}>
              You already logged {duplicateCard.totalHours.toFixed(2)}h on{" "}
              {resolvedCaseNumber} for this day. Consider editing that entry instead of
              creating a duplicate.
            </Alert>
          )}

          <Divider />

          {/* Time breakdown */}
          <Box
            sx={{
              display: "flex",
              alignItems: "baseline",
              justifyContent: "space-between",
            }}
          >
            <Typography variant="subtitle2">Time breakdown (hours)</Typography>
            <Box sx={{ display: "flex", alignItems: "center", gap: 0.5 }}>
              <Clock size={14} />
              <Typography variant="subtitle2" color="primary">
                {total.toFixed(2)}h total
              </Typography>
            </Box>
          </Box>
          {isTouched("hours") && errors.hours && (
            <Typography variant="caption" color="error">
              {errors.hours}
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
                onBlur={() => touch("hours")}
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
          {editing ? "Save changes" : "Submit for review"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
