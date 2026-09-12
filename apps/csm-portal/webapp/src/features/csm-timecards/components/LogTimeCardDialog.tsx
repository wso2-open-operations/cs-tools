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
  AdapterDateFns,
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
  InputAdornment,
  LinearProgress,
  MenuItem,
  Switch,
  TextField,
  Typography,
} from "@wso2/oxygen-ui";

// See CsmTimeCardsPage's identical comment: DesktopDatePicker avoids the
// mobile dialog (title bar + Cancel/OK) the plain DatePicker falls back to
// below the sm breakpoint, on this desktop-only portal page.
const { DesktopDatePicker: DatePicker, LocalizationProvider } = DatePickers;
import { Clock, Search, X } from "@wso2/oxygen-ui-icons-react";
import Editor from "@components/rich-text-editor/Editor";
import { useDebouncedValue } from "@hooks/useDebouncedValue";
import { useIdTokenClaims } from "@hooks/useIdTokenClaims";
import { initialsOf, resolveUserInfo } from "@utils/userClaims";
import { useSearchUsers } from "@features/csm-users/api/useSearchUsers";
import type { NormalizedUser } from "@features/csm-users/types/csmUsers";
import { useRecentApprovers } from "@features/csm-timecards/api/useTimeSheets";
import TimeCardStatusChip from "@features/csm-timecards/components/TimeCardStatusChip";
import type { SeverityOrUnset } from "@features/csm-dashboard/types/abtDashboard";
import {
  ACTIVITY_BUCKETS,
  DEFAULT_BILLABLE,
  DEFAULT_ISSUE_COMPLEXITY,
  ISSUE_COMPLEXITY_OPTIONS,
  NON_BILLABLE_SEVERITIES,
  TIMECARD_APPROVER_GROUP,
  WORK_LOG_MAX,
} from "@features/csm-timecards/constants/timeCardConstants";
import type {
  ActivityBreakdown,
  ActivityKey,
  CreateTimeCardInput,
  CsmTimeCard,
  IssueComplexity,
  TimeCardApprover,
  UpdateTimeCardInput,
} from "@features/csm-timecards/types/timeCards";
import {
  emptyBreakdown,
  timeCardDraftErrors,
  totalMinutes,
} from "@features/csm-timecards/utils/timeCardTotals";
import { localTodayIso } from "@features/csm-timecards/utils/timeSheetWeek";
import { formatDateOnly, parseDateOnly } from "@utils/dateTime";

/** Either a create ({@link CreateTimeCardInput}, POST) or an edit
 * ({@link UpdateTimeCardInput}, PATCH) submission — the caller distinguishes
 * by checking for `cardId`, present only on the edit shape. */
export type LogTimeCardSubmit = CreateTimeCardInput | UpdateTimeCardInput;

interface LogTimeCardDialogProps {
  /** The case the time was spent on — always known, this dialog only opens
   * from a case's Time tracking tab (the backend requires a real case UUID,
   * which only a case context can provide). */
  caseId: string;
  caseNumber: string;
  /** Determines whether the billable switch is editable — see
   * NON_BILLABLE_SEVERITIES. Optional because a caller editing a card from a
   * cross-case context (the Time cards page's "My time sheets" tab) has no
   * severity to hand in; the switch stays enabled there rather than being
   * force-disabled on a guess, and the backend's own business rule still
   * enforces the real non-billable-severities constraint server-side either
   * way (see NON_BILLABLE_SEVERITIES's doc comment). Also `"unset"` when the
   * case has no severity value at all — treated the same as "no severity to
   * hand in" below (switch stays enabled, backend still enforces server-side). */
  caseSeverity?: SeverityOrUnset;
  projectId: string;
  projectName: string;
  /** True while the create/edit mutation is in flight. */
  isSubmitting: boolean;
  /**
   * When set, the dialog opens in **edit mode** for this already-submitted
   * card instead of logging a new one: every field prefills from the card's
   * current values (see `CsmTimeCard.breakdown`/`issueComplexity`, confirmed
   * live to round-trip), the approver becomes read-only (matching
   * ServiceNow's own UX once a card is submitted — see `UpdateTimeCardInput`),
   * the title/submit label change, and `onSubmit` is called with an
   * {@link UpdateTimeCardInput} (`cardId` set) instead of a
   * {@link CreateTimeCardInput}.
   */
  editingCard?: CsmTimeCard;
  onClose: () => void;
  onSubmit: (input: LogTimeCardSubmit) => void;
}

interface ApproverOption {
  id: string;
  name: string;
  email?: string;
}

function fullName(u: NormalizedUser): string {
  return u.name.trim() || u.userName;
}

/** One clickable approver candidate — shared between the "recently selected"
 * list (shown before typing) and the live search results (shown once typed),
 * so the two never drift into rendering the row differently. */
function ApproverCandidateButton({
  option,
  onSelect,
}: {
  option: ApproverOption;
  onSelect: (option: ApproverOption) => void;
}): JSX.Element {
  return (
    <Button
      data-testid="approver-candidate"
      variant="text"
      color="inherit"
      onClick={() => onSelect(option)}
      sx={{
        justifyContent: "flex-start",
        textTransform: "none",
        px: 1,
        py: 0.5,
        gap: 1,
      }}
    >
      <Avatar sx={{ width: 24, height: 24, fontSize: "0.7rem" }}>
        {initialsOf(option.name)}
      </Avatar>
      <Box sx={{ minWidth: 0, textAlign: "left" }}>
        <Typography variant="body2" noWrap>
          {option.name}
        </Typography>
        {option.email && (
          <Typography
            variant="caption"
            color="text.secondary"
            noWrap
            sx={{ display: "block" }}
          >
            {option.email}
          </Typography>
        )}
      </Box>
    </Button>
  );
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
 * "Log time" form, doubling as the **edit** form for an already-submitted
 * card (see `editingCard`). Mirrors the ServiceNow fields (date, the five
 * activity buckets, work-log comment, issue complexity, approver) with a live
 * running total, per-activity proportion bars, and inline validation. There
 * was a "Category" field here too, but it was never actually sent anywhere
 * (`usePostTimeCard`'s payload has no such field) — removed rather than kept
 * as a choice that silently did nothing.
 * Creating a card submits it immediately — the backend has no draft step, so
 * there is no "Pending" status. Editing is only offered (see `cardActions`)
 * to the card's own submitter while it's still `submitted`, matching what
 * the backend itself enforces server-side.
 */
export default function LogTimeCardDialog({
  caseId,
  caseNumber,
  caseSeverity,
  projectId,
  projectName,
  isSubmitting,
  editingCard,
  onClose,
  onSubmit,
}: LogTimeCardDialogProps): JSX.Element {
  const me = resolveUserInfo(useIdTokenClaims());
  const isEditMode = !!editingCard;

  const isAlwaysNonBillable =
    !!caseSeverity &&
    caseSeverity !== "unset" &&
    NON_BILLABLE_SEVERITIES.includes(caseSeverity);

  const [date, setDate] = useState(editingCard?.workDate ?? localTodayIso());
  const [issueComplexity, setIssueComplexity] = useState<IssueComplexity>(
    editingCard?.issueComplexity ?? DEFAULT_ISSUE_COMPLEXITY,
  );
  const [billable, setBillable] = useState<boolean>(
    editingCard ? editingCard.billable : isAlwaysNonBillable ? false : DEFAULT_BILLABLE,
  );
  const [breakdown, setBreakdown] = useState<ActivityBreakdown>(
    editingCard?.breakdown ?? emptyBreakdown(),
  );
  // workLogComment is rich-text HTML (see Editor below); an edited card's
  // existing comment is already HTML on the wire, so it loads straight in.
  const [workLogComment, setWorkLogComment] = useState(editingCard?.workLogComment ?? "");
  // The approver is read-only once editing (see UpdateTimeCardInput's doc
  // comment) — this state only exists to satisfy timeCardDraftErrors'
  // approver-required check and is never sent on an edit submit.
  const [approver, setApprover] = useState<TimeCardApprover | null>(
    editingCard?.approvers?.[0] ?? null,
  );
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
    // approvers is optional on a card, so an edited card may legitimately have
    // none. The field is read-only in edit mode and never sent, so requiring it
    // here would only make Save invalid and then disabled, blocking the edit
    // outright for exactly those cards.
    requireApprover: !isEditMode,
  });
  const isValid = Object.keys(errors).length === 0;

  const search = useDebouncedValue(approverInput.trim(), 300);
  const { data } = useSearchUsers({
    filters: {
      ...(search.length > 0 && { searchQuery: search }),
      // Approvers must hold the timecard_approver role — listing every
      // internal WSO2 account here (the old INTERNAL_USER_ROLES filter)
      // let a submitter pick literally anyone at the company, including
      // people with no CS/team-lead involvement at all.
      roleIds: [TIMECARD_APPROVER_GROUP],
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

  // Previously-selected approvers (digiops-cs#2839) — surfaced before the
  // engineer types anything, and prioritized within the typed results, so
  // picking the same team lead again doesn't require retyping the same
  // search every time. Only fetched in create mode: the approver field is
  // read-only once editing, so there's nothing for this list to feed there.
  const { data: rawRecentApprovers = [] } = useRecentApprovers(!isEditMode);
  const recentApproverIds = useMemo(
    () => rawRecentApprovers.map((a) => a.id),
    [rawRecentApprovers],
  );
  // Re-checked against the same active/TIMECARD_APPROVER_GROUP eligibility as
  // live search candidates: `rawRecentApprovers` is derived purely from past
  // card submissions (see useRecentApprovers), so an approver deactivated or
  // removed from the role since then would otherwise still show up here.
  // Only queried once there's something to check (empty `userIds` would
  // otherwise be an unscoped users search).
  const { data: recentEligibility } = useSearchUsers(
    {
      filters: { userIds: recentApproverIds, roleIds: [TIMECARD_APPROVER_GROUP], active: true },
      pagination: { limit: recentApproverIds.length || 1, offset: 0 },
    },
    recentApproverIds.length > 0,
  );
  const recentApprovers: ApproverOption[] = useMemo(() => {
    if (recentApproverIds.length === 0) return [];
    const eligibleIds = new Set((recentEligibility?.users ?? []).map((u) => u.id));
    return rawRecentApprovers.filter((a) => eligibleIds.has(a.id));
  }, [rawRecentApprovers, recentApproverIds, recentEligibility]);
  // Merges recents matching the current query ahead of the live search's own
  // candidates, deduped by id — a recent approver who still matches what was
  // typed should stay at the top rather than getting buried in whatever order
  // useSearchUsers's page happens to return.
  const approverCandidates: ApproverOption[] = useMemo(() => {
    if (!hasApproverInput) return recentApprovers;
    const typed = approverInput.trim().toLowerCase();
    const recentMatches = recentApprovers.filter((a) =>
      a.name.toLowerCase().includes(typed),
    );
    const recentIds = new Set(recentMatches.map((a) => a.id));
    return [...recentMatches, ...candidates.filter((c) => !recentIds.has(c.id))];
  }, [approverInput, candidates, hasApproverInput, recentApprovers]);

  const setActivity = (key: ActivityKey, next: number): void =>
    setBreakdown((prev) => ({ ...prev, [key]: next }));

  const ALL_FIELDS = ["date", "minutes", "workLogComment", "approver"];
  const handleSubmit = (): void => {
    if (!isValid) {
      setTouched(new Set(ALL_FIELDS));
      return;
    }
    if (isEditMode && editingCard) {
      onSubmit({
        cardId: editingCard.id,
        date,
        breakdown,
        billable,
        workLogComment: workLogComment.trim(),
        issueComplexity,
      });
      return;
    }
    // Create only: an approver is mandatory here, and isValid already covers
    // it (requireApprover is true outside edit mode). This narrows the type.
    if (!approver) {
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

  /** Submit on Enter, except inside a multiline field (where Enter = newline)
   * — the plain-textarea case (TEXTAREA) and the rich-text work log comment
   * (a contenteditable div, not a TEXTAREA, so it needs its own check). */
  const handleKeyDown = (e: KeyboardEvent): void => {
    const target = e.target as HTMLElement;
    if (
      e.key === "Enter" &&
      !e.shiftKey &&
      target.tagName !== "TEXTAREA" &&
      !target.isContentEditable
    ) {
      e.preventDefault();
      handleSubmit();
    }
  };

  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>
        {isEditMode ? "Edit time card" : "Log time"} · {caseNumber}
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
              <Typography variant="body2">
                {isEditMode ? editingCard.userName : me.fullName}
              </Typography>
            </Box>
            <TimeCardStatusChip state="submitted" />
          </Box>

          <Typography variant="body2" color="text.secondary">
            Task: {caseNumber}
            {projectName ? ` · ${projectName}` : ""}
          </Typography>

          <LocalizationProvider dateAdapter={AdapterDateFns}>
            <DatePicker
              label="Date"
              value={parseDateOnly(date)}
              onChange={(next) => {
                setDate(
                  next instanceof Date && !Number.isNaN(next.getTime())
                    ? formatDateOnly(next)
                    : "",
                );
                touch("date");
              }}
              slotProps={{
                textField: {
                  size: "small",
                  required: true,
                  error: isTouched("date") && !!errors.date,
                  helperText: isTouched("date") ? errors.date : undefined,
                  sx: { maxWidth: { sm: 220 }, minWidth: 160 },
                },
                field: { clearable: true },
              }}
            />
          </LocalizationProvider>

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
              slotProps={{
                // `issueComplexity` always holds a real value (no empty
                // option), so the label is always shrunk -- see
                // MultiSelectField.tsx's doc comment for why this override
                // is needed at all against oxygen-ui's own theme.
                inputLabel: { shrink: true, sx: { top: "0px !important" } },
                select: { notched: true },
              }}
            >
              {ISSUE_COMPLEXITY_OPTIONS.map((o) => (
                <MenuItem key={o} value={o}>
                  {o}
                </MenuItem>
              ))}
            </TextField>
            <Box sx={{ display: "flex", flexDirection: "column", alignItems: "flex-end" }}>
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
                <Typography variant="caption" color="text.secondary">
                  Always non-billable for {caseSeverity} cases.
                </Typography>
              )}
            </Box>
          </Box>

          {/* Work log — rich text, matching ServiceNow's own work-notes
              convention. Never truncated on input (unlike the old plain
              TextField's live .slice()): naively cutting HTML at a fixed
              offset risks slicing a tag in half and corrupting the markup.
              The WORK_LOG_MAX cap is still enforced (see
              timeCardDraftErrors), just not by mid-typing truncation. */}
          <Box>
            <Typography
              id="work-log-comment-label"
              variant="caption"
              color={
                isTouched("workLogComment") && errors.workLogComment
                  ? "error"
                  : "text.secondary"
              }
              sx={{ display: "block", mb: 0.5 }}
            >
              Work log comment *
            </Typography>
            <Box role="group" aria-labelledby="work-log-comment-label">
              <Editor
                value={workLogComment}
                onChange={setWorkLogComment}
                onBlur={() => touch("workLogComment")}
                placeholder="What did you work on?"
                minHeight={80}
                maxHeight={240}
                toolbarVariant="full"
              />
            </Box>
            <Typography
              variant="caption"
              color={
                isTouched("workLogComment") && errors.workLogComment
                  ? "error"
                  : "text.secondary"
              }
              sx={{ display: "block", mt: 0.5 }}
            >
              {isTouched("workLogComment") && errors.workLogComment
                ? errors.workLogComment
                : `${WORK_LOG_MAX - workLogComment.length} characters left`}
            </Typography>
          </Box>

          {/* Approver — read-only once editing: ServiceNow's own UX locks
              this field after submit, and the portal follows that rather
              than letting an edit silently reassign the approver even
              though the backend would technically accept the change. */}
          <Box sx={{ display: "flex", flexDirection: "column", gap: 0.75 }}>
            <Typography variant="subtitle2">Approver (team lead)</Typography>
            {isEditMode ? (
              <Chip label={approver?.name ?? "—"} sx={{ alignSelf: "flex-start" }} />
            ) : approver ? (
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
                    approverCandidates.length === 0 ? (
                      <Typography
                        variant="caption"
                        color="text.secondary"
                        sx={{ p: 1 }}
                      >
                        Start typing to search for an approver.
                      </Typography>
                    ) : (
                      <>
                        <Typography
                          variant="caption"
                          color="text.secondary"
                          sx={{ px: 1, pt: 1 }}
                        >
                          Recently selected
                        </Typography>
                        {approverCandidates.map((u) => (
                          <ApproverCandidateButton
                            key={u.id}
                            option={u}
                            onSelect={(picked) => {
                              setApprover({ id: picked.id, name: picked.name });
                              setApproverInput("");
                            }}
                          />
                        ))}
                      </>
                    )
                  ) : approverCandidates.length === 0 ? (
                    <Typography
                      variant="caption"
                      color="text.secondary"
                      sx={{ p: 1 }}
                    >
                      No matching engineers.
                    </Typography>
                  ) : (
                    approverCandidates.map((u) => (
                      <ApproverCandidateButton
                        key={u.id}
                        option={u}
                        onSelect={(picked) => {
                          setApprover({ id: picked.id, name: picked.name });
                          setApproverInput("");
                        }}
                      />
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
          {isEditMode ? "Save changes" : "Submit for review"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
