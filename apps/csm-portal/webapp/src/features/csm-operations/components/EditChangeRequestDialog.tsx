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
  Box,
  Button,
  DatePickers,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormHelperText,
  Typography,
} from "@wso2/oxygen-ui";
import { useCallback, useMemo, useState, type JSX } from "react";
import { useSearchGroups } from "@api/useSearchGroups";
import type {
  BeChangeRequestDetail,
  BeGroup,
  BePatchChangeRequestPayload,
} from "@api/backend/types";
import AsyncEntitySelect from "@components/AsyncEntitySelect";
import Editor from "@components/rich-text-editor/Editor";
import {
  formatDateTimeLocal,
  isPastDateTime,
  parseDateTimeLocal,
} from "@utils/dateTime";
import { isBlankHtml, sanitizeRichTextHtml } from "@utils/sanitizeHtml";

const { DateTimePicker, LocalizationProvider } = DatePickers;

interface EditChangeRequestDialogProps {
  cr: BeChangeRequestDetail;
  /** True while the PATCH is in flight; disables the actions. */
  isSaving: boolean;
  /**
   * User-facing message for the most recent failed save, if any. Rendered
   * inline in the dialog so the rejection is visible even if a page-level
   * error banner is occluded or the dialog is otherwise the only thing the
   * user is looking at.
   */
  saveError?: string | null;
  onClose: () => void;
  /** Submit only the changed fields (`PATCH /change-requests/{id}`). */
  onSave: (patch: BePatchChangeRequestPayload) => void;
}

/**
 * Convert a backend timestamp (`YYYY-MM-DD HH:MM:SS`, or ISO `T`-separated) to
 * the `YYYY-MM-DDTHH:MM` shape this form's state (and the DateTimePicker via
 * {@link parseDateTimeLocal}) uses. The value is treated as plain wall-clock
 * text so no timezone shift is applied.
 */
function toDateTimeLocal(raw?: string | null): string {
  const m = /^(\d{4}-\d{2}-\d{2})[ T](\d{2}:\d{2})/.exec(raw?.trim() ?? "");
  return m ? `${m[1]}T${m[2]}` : "";
}

/** Convert a `datetime-local` value back to the BE's `YYYY-MM-DD HH:MM:SS`. */
function toBackendDateTime(local: string): string {
  return `${local.replace("T", " ")}:00`;
}

/** One long-form plan field, edited as rich text. */
interface RichTextPlanField {
  /** Frozen seed handed to the editor; never re-sent as the editor changes. */
  initialHtml: string;
  /** Current editor HTML. */
  html: string;
  onChange: (next: string) => void;
  /** True once the user has actually changed the content. */
  isDirty: boolean;
  /** What to put in the patch: `""` for a cleared field, else the HTML. */
  outgoing: string;
}

/**
 * State for a plan field that is edited as rich text rather than plain text.
 *
 * Dirty-tracking is the whole difficulty here. The editor normalizes markup
 * when it loads a stored value — `<p>A</p>` comes back out as
 * `<p><span style="white-space: pre-wrap;">A</span></p>` — so comparing the
 * editor's HTML against the stored HTML as strings marks every seeded field
 * dirty before the user has touched anything, and the dialog would then patch
 * fields nobody edited.
 *
 * So the baseline is the editor's *own* first emission rather than the stored
 * string: whatever it produces from the seed is, by definition, the unedited
 * state. Two cases, and they differ because the editor only emits on load when
 * there is something to load:
 *
 * - Stored content is non-blank: the editor injects it and emits once before
 *   any user input, so the first emission is the baseline.
 * - Stored content is blank: nothing is injected and nothing is emitted, so
 *   the first emission would be the user's own typing. Baseline is fixed to
 *   "blank" up front instead, and dirtiness is `!isBlankHtml`.
 *
 * A cleared field goes out as `""`, not the editor's `<p><br></p>` — an empty
 * paragraph reads as "the plan says nothing" rather than "there is no plan".
 */
function useRichTextPlanField(storedHtml?: string | null): RichTextPlanField {
  const stored = storedHtml ?? "";
  // Frozen at mount: the editor treats its `value` as an initial value, and
  // feeding the live HTML back in makes it re-seed itself mid-edit.
  const [initialHtml] = useState(stored);
  const [html, setHtml] = useState(stored);
  // `null` means "waiting for the editor's first emission"; `""` means the
  // baseline is known to be blank.
  const [baseline, setBaseline] = useState<string | null>(
    isBlankHtml(stored) ? "" : null,
  );

  const onChange = useCallback((next: string) => {
    setBaseline((current) => (current === null ? next : current));
    setHtml(next);
  }, []);

  const isDirty =
    baseline === null
      ? false
      : baseline === ""
        ? !isBlankHtml(html)
        : html !== baseline;

  // Sanitized on the way out, the same policy the detail page renders it back
  // through, so nothing the editor emits can widen what ends up stored.
  const outgoing = isBlankHtml(html) ? "" : sanitizeRichTextHtml(html);

  return { initialHtml, html, onChange, isDirty, outgoing };
}

/**
 * Edit the change-request fields the BE allows updating: the planned window,
 * the assignment group, and the rollback and test plans. Only changed fields
 * are sent, and the BE requires at least one, so Save is disabled until
 * something differs.
 *
 * `isCustomerApproved`/`isCustomerReviewed` are deliberately NOT exposed here
 * even though the BE patch contract still accepts them (see
 * `BePatchChangeRequestPayload`). Traced end to end (webapp -> BFF -> Go
 * entity-service -> Ballerina -> the SN scripted API's dedicated
 * `patchCustomerApproved`/`patchCustomerReviewed` handlers): both are gated
 * (only accepted while the CR is already in the "Customer Approval"/
 * "Customer Review" state) but flipping either is not a boolean-field edit —
 * it drives a real state transition, and the "off" direction is destructive
 * (`isCustomerApproved: false` moves the CR to Cancelled; `isCustomerReviewed:
 * false` moves it to Rollback, a terminal dead end with no reason capture).
 * A switch labelled "Customer approved"/"Customer reviewed" strongly implies
 * a record-keeping boolean, not a one-way cancel/rollback action, so this is
 * exactly the "inventing a capability the source system doesn't expose"
 * pattern the CR approval-mechanics review warned about (no SN UI action
 * exists for either state either). Removed as a UI affordance; the BE/Go
 * plumbing is left in place since nothing else depends on removing it.
 */
export default function EditChangeRequestDialog({
  cr,
  isSaving,
  saveError,
  onClose,
  onSave,
}: EditChangeRequestDialogProps): JSX.Element {
  const initialPlannedStart = useMemo(
    () => toDateTimeLocal(cr.plannedStartOn),
    [cr.plannedStartOn],
  );
  const initialPlannedEnd = useMemo(
    () => toDateTimeLocal(cr.plannedEndOn),
    [cr.plannedEndOn],
  );
  const initialAssignedTeamId = cr.assignedTeam?.id ?? "";
  const [plannedStart, setPlannedStart] = useState(initialPlannedStart);
  const [plannedEnd, setPlannedEnd] = useState(initialPlannedEnd);
  const [assignedTeamId, setAssignedTeamId] = useState(initialAssignedTeamId);
  const rollbackPlan = useRichTextPlanField(cr.rollbackPlan);
  const testPlan = useRichTextPlanField(cr.testPlan);

  // Client-side only, and only when both ends are set: the backing system
  // does its own validation and this must not become the thing that blocks a
  // legitimate save, so it surfaces inline rather than being enforced
  // server-side.
  const startDate = parseDateTimeLocal(plannedStart);
  const endDate = parseDateTimeLocal(plannedEnd);
  const plannedEndBeforeStart =
    !!startDate && !!endDate && endDate.getTime() <= startDate.getTime();

  const patch = useMemo<BePatchChangeRequestPayload>(() => {
    const next: BePatchChangeRequestPayload = {};
    if (plannedStart !== initialPlannedStart && plannedStart) {
      next.plannedStartOn = toBackendDateTime(plannedStart);
    }
    if (plannedEnd !== initialPlannedEnd && plannedEnd) {
      next.plannedEndOn = toBackendDateTime(plannedEnd);
    }
    if (assignedTeamId !== initialAssignedTeamId && assignedTeamId) {
      next.assignedTeamId = assignedTeamId;
    }
    // Unlike the pickers above, an emptied plan field is a real edit the BE
    // can accept, so "" is sent rather than skipped. Both plans are rich text
    // on both sides now — see `useRichTextPlanField` for why "changed" is not
    // a comparison against the stored string.
    if (rollbackPlan.isDirty) next.rollbackPlan = rollbackPlan.outgoing;
    if (testPlan.isDirty) next.testPlan = testPlan.outgoing;
    return next;
  }, [
    plannedStart,
    initialPlannedStart,
    plannedEnd,
    initialPlannedEnd,
    assignedTeamId,
    initialAssignedTeamId,
    rollbackPlan.isDirty,
    rollbackPlan.outgoing,
    testPlan.isDirty,
    testPlan.outgoing,
  ]);

  const hasChanges = Object.keys(patch).length > 0;
  // Non-blocking: editing a CR's planned start to a past instant is unusual
  // but not forbidden (e.g. recording when it actually started), so this
  // only warns.
  const plannedStartIsPast = isPastDateTime(startDate);

  // Rich-text plan field. The editor takes no `id`/native label, so the
  // visible label is a separate Typography tied to the control via
  // role="group" + aria-labelledby, and the helper text is referenced by
  // aria-describedby — same convention as the create page's Planning fields.
  const renderPlanField = (
    id: string,
    label: string,
    field: RichTextPlanField,
    helperText: string,
  ): JSX.Element => (
    <Box>
      <Typography
        id={`${id}-label`}
        variant="caption"
        color="text.secondary"
        sx={{ display: "block", mb: 0.5 }}
      >
        {label}
      </Typography>
      <Box role="group" aria-labelledby={`${id}-label`} aria-describedby={`${id}-help`}>
        <Editor
          value={field.initialHtml}
          onChange={field.onChange}
          minHeight={100}
          maxHeight={300}
          toolbarVariant="full"
          disabled={isSaving}
        />
      </Box>
      <FormHelperText id={`${id}-help`}>{helperText}</FormHelperText>
    </Box>
  );

  return (
    <Dialog open onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>Edit change request</DialogTitle>
      <DialogContent dividers>
        <Box sx={{ display: "flex", flexDirection: "column", gap: 2, pt: 0.5 }}>
          {saveError && (
            <Alert severity="error" sx={{ width: "100%" }}>
              {saveError}
            </Alert>
          )}
          {/*
            No `clearable` on either picker. The patch payload has no way to
            express "remove the planned date" — `plannedStartOn`/`plannedEndOn`
            are `string | undefined`, and an omitted key means "leave it
            alone" — so a clear affordance would appear to work and then
            silently save nothing. Widening the payload to express a null
            clear is the fix if this is ever actually needed.
          */}
          <LocalizationProvider dateAdapter={AdapterDateFns}>
            <DateTimePicker
              label="Planned start"
              value={startDate}
              onChange={(next) =>
                setPlannedStart(
                  next instanceof Date && !Number.isNaN(next.getTime())
                    ? formatDateTimeLocal(next)
                    : "",
                )
              }
              slotProps={{
                textField: {
                  size: "small",
                  fullWidth: true,
                  helperText: plannedStartIsPast
                    ? "This date is in the past."
                    : undefined,
                },
              }}
            />
            <DateTimePicker
              label="Planned end"
              value={endDate}
              onChange={(next) =>
                setPlannedEnd(
                  next instanceof Date && !Number.isNaN(next.getTime())
                    ? formatDateTimeLocal(next)
                    : "",
                )
              }
              slotProps={{
                textField: {
                  size: "small",
                  fullWidth: true,
                  error: plannedEndBeforeStart,
                  helperText: plannedEndBeforeStart
                    ? "Planned end must be after planned start."
                    : undefined,
                },
              }}
            />
          </LocalizationProvider>
          <AsyncEntitySelect<BeGroup>
            id="cr-edit-assigned-team"
            label="Assignment group"
            placeholder="Search groups…"
            value={assignedTeamId}
            onChange={setAssignedTeamId}
            disabled={isSaving}
            useSearch={useSearchGroups}
            getId={(g) => g.id}
            getLabel={(g) => g.name}
            knownLabel={cr.assignedTeam?.name}
            helperText="Required before approval can be requested."
          />
          {renderPlanField(
            "cr-edit-rollback-plan",
            "Rollback plan",
            rollbackPlan,
            "How this change is backed out if it goes wrong.",
          )}
          {renderPlanField(
            "cr-edit-test-plan",
            "Test plan",
            testPlan,
            "How the change is verified once implemented.",
          )}
        </Box>
      </DialogContent>
      <DialogActions>
        <Button color="inherit" onClick={onClose} disabled={isSaving}>
          Cancel
        </Button>
        <Button
          variant="contained"
          onClick={() => onSave(patch)}
          disabled={isSaving || !hasChanges || plannedEndBeforeStart}
        >
          {isSaving ? "Saving…" : "Save"}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
