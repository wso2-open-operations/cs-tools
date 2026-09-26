import {
  Alert,
  Box,
  Button,
  Chip,
  Divider,
  Grid,
  LinearProgress,
  MenuItem,
  Stack,
  TextField,
  Tooltip,
  Typography,
  ToggleButton,
  ToggleButtonGroup,
} from "@wso2/oxygen-ui";
import { useState, type ReactNode } from "react";


import {
  useAcknowledge,
  useCreateNote,
  useLifecycle,
  usePatchProduct,
  useProductDetail,
  useUpdateNote,
} from "@features/plg/api/hooks";
import {
  SUBSCRIPTION_TIERS,
  tierDateField,
  type LifecycleStage,
  type Note,
  type ProductDetail,
  type SubscriptionTier,
  type HealthState,
  type UserRef,
  HEALTH_STATES,
  HEALTH_LABEL,
} from "@features/plg/api/types";
import {
  EmptyState,
  ErrorBlock,
  Field,
  LabelledChip,
  LoadingSpinner,
  SectionCard,
  StatusChip,
} from "@features/plg/components/common";
import { useMe } from "@features/plg/api/hooks";
import { formatDate, formatDateTime, humanizeEnum } from "@features/plg/utils/format";
import { PlaybookRuns } from "./PlaybookRuns";
import { StageFlow } from "./StageFlow";
import { selectLabelProps } from "@features/plg/components/selectLabelProps";
import { AXIS_BUTTON_SX, CONTROL_WIDTH } from "@features/plg/components/controls";

/**
 * The product tab — one organisation on one platform.
 *
 * This is the unit of work, so everything an engineer does lives here: the
 * stage, the playbooks running against it, the use case, the notes and the
 * history. The organisation above it holds only what the customer as a whole
 * owns.
 */
export function ProductTab({
  organizationId,
  productCode,
}: {
  organizationId: string;
  productCode: string;
}) {
  const { data: detail, isPending, error } = useProductDetail(organizationId, productCode);
  const { data: lifecycle } = useLifecycle();
  const acknowledge = useAcknowledge();

  if (error) return <ErrorBlock error={error} />;
  if (isPending || !detail) return <LoadingSpinner />;

  const progress = detail.progress;
  const percent = progress.taskTotal ? (progress.taskCompleted / progress.taskTotal) * 100 : 0;

  return (
    <Stack spacing={2}>
      {detail.isNew ? (
        <Alert
          severity="warning"
          action={
            <Button
              size="small"
              color="inherit"
              disabled={acknowledge.isPending}
              onClick={() => acknowledge.mutate({ orgPlatformId: detail.orgPlatformId })}
            >
              Acknowledge and take ownership
            </Button>
          }
        >
          Nobody has picked this registration up yet.
        </Alert>
      ) : null}

      <SectionCard
        title="Lifecycle"
        /* All three tracked axes, in one row above the diagram. The stage chip
           was here alone, which made the diagram look like the whole story of
           the pairing when two of the three things being tracked were further
           down the page. Reading them together is the point: COMMERCIAL and at
           risk on a free tier is a different situation from any one of those
           facts on its own.

           Each one says what it is. Unlabelled, "Commercial" and "Enterprise"
           sit side by side looking like the same kind of fact stated twice —
           one is a lifecycle stage and the other a subscription tier, and
           nothing on screen distinguished them.

           The words are STAGE, TIER and HEALTH rather than the longer
           "Lifecycle" and "Subscription": they have to fit three-across in a
           card header without wrapping, and each one matches the heading on the
           editor for that axis further down the page, so a reader can get from
           the chip to the control that changes it. "Lifecycle" would also
           collide with this card's own title, which covers two of the three. */
        action={
          <Stack direction="row" spacing={1.5} flexWrap="wrap" useFlexGap>
            <LabelledChip label="Stage" value={detail.lifecycleStage} kind="lifecycle" size="medium" />
            <LabelledChip label="Tier" value={detail.subscriptionTier} kind="tier" size="medium" />
            <LabelledChip label="Health" value={detail.healthState} kind="health" size="medium" />
          </Stack>
        }
      >
        <Typography variant="body2" color="text.secondary" mb={1}>
          At {detail.stageName} since {formatDate(detail.stageEnteredOn)}.{" "}
          {detail.carriesPlaybooks
            ? "Playbooks are available at this stage."
            : "No playbooks run at this stage — move the pairing on when the facts change."}
        </Typography>
        {lifecycle ? (
          <StageFlow
            catalogue={lifecycle}
            currentStage={detail.lifecycleStage}
            healthState={detail.healthState}
            height={300}
          />
        ) : null}

        {/* Each axis gets its own block, its own reason and its own Save.
            Health used to sit as a bare toggle immediately under the diagram,
            where it collided with the legend: two rows of small coloured things
            an inch apart, one a key to the picture and one a live control that
            wrote to the database the moment it was touched. The divider and the
            heading are what separate them now. */}
        <Divider sx={{ my: 2 }} />
        <StageEditor
          organizationId={organizationId}
          productCode={productCode}
          current={detail.lifecycleStage}
        />
        <Divider sx={{ my: 2 }} />
        <HealthEditor
          organizationId={organizationId}
          productCode={productCode}
          current={detail.healthState}
          enteredOn={detail.healthEnteredOn}
          updatedBy={detail.healthUpdatedBy}
        />
      </SectionCard>

      {/* The third tracked axis, and now the same shape and width as the other
          two. It spent a while in the narrow right-hand column beside
          Registration, which looked tidy and worked badly: AxisEditor reserves a
          fixed width for the picker, so in a five-of-twelve column the reason
          box got whatever was left — a field asking for a sentence, sized for a
          word. Editing a tier is the same act as moving a stage or marking a
          risk, so it gets the same room to do it in.

          It is a card of its own rather than a third block inside Lifecycle
          because the tier is not part of the lifecycle: it drives nothing, and
          folding it in would suggest it did. The header chips above already do
          the job of showing all three together. */}
      <SubscriptionCard
        organizationId={organizationId}
        productCode={productCode}
        tier={detail.subscriptionTier}
        trialEndDate={detail.trialEndDate}
        trialExtendedDate={detail.trialExtendedDate}
        enteredOn={detail.subscriptionEnteredOn}
        updatedBy={detail.subscriptionUpdatedBy}
      />

      {/* Full width, like the two cards above it. The stage and the playbooks
          running at that stage are one thought — the playbook is how a pairing
          leaves the stage above — so they are read close together, before the
          reference detail underneath. It also needs the width: a pairing can
          carry several runs, each with its own task list, and in half a row the
          task rows wrapped and the reason pickers became unreadable. */}
      <PlaybookRuns organizationId={organizationId} productCode={productCode} detail={detail} />

      <Grid container spacing={2}>
        <Grid size={{ xs: 12, md: 5 }}>
          <SectionCard fill title="Registration">
            <Field label="Registered email" value={detail.registeredEmail} />
            <Field label="Registered on" value={formatDate(detail.registeredOn)} />
            <Field
              label="Acknowledged"
              value={
                detail.acknowledgedOn
                  ? `${formatDate(detail.acknowledgedOn)} by ${detail.acknowledgedBy?.name ?? "—"}`
                  : "Not yet"
              }
            />
          </SectionCard>
        </Grid>

        <Grid size={{ xs: 12, md: 7 }}>
          <SectionCard fill title="Progress">
            <Stack direction="row" spacing={3} mb={1.5}>
              <Box>
                <Typography variant="h5">{progress.runActive}</Typography>
                <Typography variant="caption" color="text.secondary">
                  Playbooks open
                </Typography>
              </Box>
              <Box>
                <Typography variant="h5">{progress.runClosed}</Typography>
                <Typography variant="caption" color="text.secondary">
                  Closed
                </Typography>
              </Box>
              <Box>
                <Typography variant="h5">
                  {progress.taskCompleted}/{progress.taskTotal}
                </Typography>
                <Typography variant="caption" color="text.secondary">
                  Tasks done
                </Typography>
              </Box>
            </Stack>
            <LinearProgress variant="determinate" value={percent} sx={{ height: 8, borderRadius: 4 }} />
          </SectionCard>
        </Grid>
      </Grid>

      <Grid container spacing={2}>
        <Grid size={{ xs: 12, md: 7 }}>
          <NotesCard organizationId={organizationId} productCode={productCode} detail={detail} />
        </Grid>
        <Grid size={{ xs: 12, md: 5 }}>
          <SectionCard fill title="History">
            {detail.lifecycleHistory.length === 0 ? (
              <EmptyState message="Nothing recorded yet" />
            ) : (
              <Stack spacing={1.5}>
                {detail.lifecycleHistory.map((entry) => (
                  <Box key={entry.id}>
                    <Stack direction="row" spacing={1} alignItems="center" flexWrap="wrap" useFlexGap>
                      {/* One row per change, of whichever axis moved. A row may
                          carry more than one — a stage move and a tier change
                          made in the same action are one act of judgement, and
                          the API writes them together. */}
                      {/* Each row says which axis moved, for the same reason
                          the header chips do: "Commercial" and "Enterprise" are
                          a stage and a tier, and a timeline mixing all three
                          gave no way to tell which kind of change you were
                          reading. The label also survives the case where only
                          one axis moved and there is no other row to compare
                          against. */}
                      {entry.toStage ? (
                        <AxisChange label="Stage">
                          {entry.fromStage ? (
                            <>
                              <StatusChip value={entry.fromStage} kind="lifecycle" />
                              <Typography variant="caption">→</Typography>
                            </>
                          ) : null}
                          <StatusChip value={entry.toStage} kind="lifecycle" />
                        </AxisChange>
                      ) : null}
                      {entry.toHealth ? (
                        <AxisChange label="Health">
                          {entry.fromHealth ? (
                            <>
                              <StatusChip value={entry.fromHealth} kind="health" />
                              <Typography variant="caption">→</Typography>
                            </>
                          ) : null}
                          {/* kind="health" rather than a colour written out
                              here: this chip predated the health palette and
                              still said warning-amber for AT_RISK while the
                              diagram and the header chip had moved to red. */}
                          <StatusChip value={entry.toHealth} kind="health" />
                        </AxisChange>
                      ) : null}
                      {entry.toSubscription ? (
                        <AxisChange label="Tier">
                          {entry.fromSubscription ? (
                            <>
                              <StatusChip value={entry.fromSubscription} kind="tier" />
                              <Typography variant="caption">→</Typography>
                            </>
                          ) : null}
                          <StatusChip value={entry.toSubscription} kind="tier" />
                        </AxisChange>
                      ) : null}
                      <Typography variant="caption" color="text.secondary">
                        {formatDate(entry.changedOn)}
                        {entry.changedBy ? ` · ${entry.changedBy.name}` : ""}
                      </Typography>
                    </Stack>
                    {entry.reason ? (
                      <Typography variant="body2" color="text.secondary" mt={0.5}>
                        {entry.reason}
                      </Typography>
                    ) : null}
                  </Box>
                ))}
              </Stack>
            )}
          </SectionCard>
        </Grid>
      </Grid>
    </Stack>
  );
}

/**
 * One axis's change within a history row: the word for the axis, then the
 * chips. The same STAGE / TIER / HEALTH vocabulary as the card header, so the
 * timeline and the current state name things the same way.
 */
function AxisChange({ label, children }: { label: string; children: ReactNode }) {
  return (
    <Stack direction="row" spacing={0.625} alignItems="center">
      <Typography
        variant="caption"
        color="text.secondary"
        sx={{ textTransform: "uppercase", letterSpacing: 0.4, whiteSpace: "nowrap" }}
      >
        {label}
      </Typography>
      {children}
    </Stack>
  );
}

/**
 * Moving the pairing to another stage.
 *
 * Every stage is offered, not just the seven playbook paths. Those paths say
 * where playbooks may exist; they are not a track the pairing has to run on.
 * Nothing transitions *into* Disqualified, At Risk or Abandoned, so restricting
 * this picker to them would make three stages unreachable.
 */
/**
 * One editor shape for every tracked axis.
 *
 * Stage, health and subscription are three different facts, but the act of
 * changing one is identical in all three cases: pick a value, say why, press
 * Save. They were three hand-written blocks that had drifted — the stage had a
 * reason box and a Save, the tier had a reason box and a Save that appeared
 * only after a change, and health had neither and wrote on the first click.
 * Sharing the component is what makes "nothing reaches the database by
 * accident" a property of the page rather than a habit of whoever edited it
 * last.
 *
 * The reason box is always rendered and disabled until something changes,
 * rather than appearing on change. A field that pops into existence shifts the
 * Save button out from under the cursor at the exact moment the cursor is
 * heading for it.
 */
function AxisEditor({
  heading,
  control,
  changed,
  reasonLabel,
  reasonHelp,
  saveLabel,
  pending,
  error,
  footer,
  onSave,
  onReset,
}: {
  heading: string;
  control: ReactNode;
  changed: boolean;
  reasonLabel: string;
  reasonHelp: string;
  saveLabel: string;
  pending: boolean;
  error: unknown;
  footer?: ReactNode;
  onSave: (reason: string, clear: () => void) => void;
  onReset: () => void;
}) {
  const [reason, setReason] = useState("");
  const trimmed = reason.trim();
  const missingReason = changed && trimmed === "";

  const reset = () => {
    setReason("");
    onReset();
  };

  return (
    <Box>
      <Typography
        variant="caption"
        color="text.secondary"
        sx={{ textTransform: "uppercase", letterSpacing: 0.4 }}
      >
        {heading}
      </Typography>
      <Stack
        direction={{ xs: "column", sm: "row" }}
        spacing={1.5}
        alignItems="flex-start"
        mt={0.75}
      >
        <Box sx={{ width: { xs: "100%", sm: CONTROL_WIDTH }, flexShrink: 0 }}>{control}</Box>
        <TextField
          size="small"
          fullWidth
          required
          label={reasonLabel}
          value={reason}
          disabled={!changed}
          error={missingReason}
          /* A space keeps the row height constant whether or not the message is
             showing, so nothing below it moves as the reader types. */
          helperText={missingReason ? reasonHelp : " "}
          onChange={(e) => setReason(e.target.value)}
        />
        <Stack direction="row" spacing={1} sx={{ flexShrink: 0 }}>
          <Tooltip title={missingReason ? "A reason is required" : ""}>
            <span>
              <Button
                variant="contained"
                size="small"
                sx={AXIS_BUTTON_SX}
                disabled={!changed || missingReason || pending}
                onClick={() => onSave(trimmed, () => setReason(""))}
              >
                {pending ? "Saving…" : saveLabel}
              </Button>
            </span>
          </Tooltip>
          {changed ? (
            <Button
              variant="text"
              color="inherit"
              size="small"
              sx={AXIS_BUTTON_SX}
              disabled={pending}
              onClick={reset}
            >
              Cancel
            </Button>
          ) : null}
        </Stack>
      </Stack>
      {footer}
      {error ? <ErrorBlock error={error} /> : null}
    </Box>
  );
}

function StageEditor({
  organizationId,
  productCode,
  current,
}: {
  organizationId: string;
  productCode: string;
  current: LifecycleStage;
}) {
  const { data: lifecycle } = useLifecycle();
  const patch = usePatchProduct(organizationId, productCode);
  const [stage, setStage] = useState<LifecycleStage>(current);

  return (
    <AxisEditor
      heading="Stage"
      changed={stage !== current}
      reasonLabel="Why is it moving?"
      reasonHelp="Say why it moved — this becomes the history entry"
      saveLabel="Save stage"
      pending={patch.isPending}
      error={patch.error}
      onReset={() => setStage(current)}
      onSave={(reason, clear) =>
        patch.mutate({ lifecycleStage: stage, reason }, { onSuccess: clear })
      }
      control={
        <TextField
          select
          {...selectLabelProps(stage)}
          fullWidth
          size="small"
          label="Move to stage"
          value={stage}
          onChange={(e) => setStage(e.target.value as LifecycleStage)}
        >
          {/* Only the moves the API will accept: forward along the progression,
              or out to ABANDONED. Offering the rest and letting the request fail
              would be the same information delivered later and less kindly. */}
          {(lifecycle?.stages ?? [])
            .filter((st) => {
              if (st.stage === current) return true;
              if (st.terminal) return true;
              const order = lifecycle?.stages.find((x) => x.stage === current)?.displayOrder ?? 0;
              return st.displayOrder > order;
            })
            .map((st) => (
              <MenuItem key={st.stage} value={st.stage}>
                {st.name}
              </MenuItem>
            ))}
        </TextField>
      }
    />
  );
}

/**
 * One tier's end date, staged behind a Save.
 *
 * The date takes no reason — it is copied off a contract rather than a
 * judgement anyone has to defend — but it is still staged, because a date
 * picker is three clicks of which any one can land on the wrong day, and
 * without a Save each of those clicks was a write. The cost of a button here is
 * one press; the cost of no button is a trial that silently ends on the wrong
 * date.
 *
 * Rendered only for a tier that HAS an end date, so it carries no "which tier
 * is this?" logic of its own — the caller decides that, and remounts this on a
 * key when the answer changes.
 */
function PeriodDateEditor({
  label,
  value,
  pending,
  onSave,
}: {
  label: string;
  value: string | null;
  pending: boolean;
  onSave: (day: string) => void;
}) {
  const asDay = (v: string | null) => (v ? v.slice(0, 10) : "");
  const [draft, setDraft] = useState(asDay(value));
  const changed = draft !== asDay(value);

  return (
    <Box>
      <Typography
        variant="caption"
        color="text.secondary"
        sx={{ textTransform: "uppercase", letterSpacing: 0.4 }}
      >
        {label}
      </Typography>
      <Stack
        direction={{ xs: "column", sm: "row" }}
        spacing={1}
        alignItems={{ sm: "flex-start" }}
        mt={0.75}
      >
        {/* The same width as the tier picker above, so the two controls line up
            down the card. Left to stretch it would run the whole width of a
            full-width card — an enormous box for eight characters of date. */}
        <TextField
          size="small"
          type="date"
          label={label}
          sx={{ width: { xs: "100%", sm: CONTROL_WIDTH } }}
          slotProps={{ inputLabel: { shrink: true } }}
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
        />
        <Button
          variant="contained"
          size="small"
          sx={AXIS_BUTTON_SX}
          disabled={!changed || pending}
          onClick={() => onSave(draft)}
        >
          {pending ? "Saving…" : "Save date"}
        </Button>
        {changed ? (
          <Button
            variant="text"
            color="inherit"
            size="small"
            sx={AXIS_BUTTON_SX}
            onClick={() => setDraft(asDay(value))}
          >
            Cancel
          </Button>
        ) : null}
      </Stack>
    </Box>
  );
}

/** Tier and its end date, both recorded by hand — there is no subscription feed. */
function SubscriptionCard({
  organizationId,
  productCode,
  tier,
  trialEndDate,
  trialExtendedDate,
  enteredOn,
  updatedBy,
}: {
  organizationId: string;
  productCode: string;
  tier: SubscriptionTier | null;
  trialEndDate: string | null;
  trialExtendedDate: string | null;
  enteredOn: string | null;
  updatedBy: UserRef | null;
}) {
  const patch = usePatchProduct(organizationId, productCode);
  const [draftTier, setDraftTier] = useState<SubscriptionTier | "">(tier ?? "");

  // Which date this pairing has, if any, follows the tier that is SAVED — not
  // the one staged in the picker above. A date belongs to the tier actually on
  // the record; offering an extension date beside an unsaved "Trial extended"
  // selection would invite someone to set a date for a tier they then abandon.
  const dateField = tierDateField(tier);

  return (
    <SectionCard title="Subscription">
      <Stack spacing={2}>
        <AxisEditor
          heading="Tier"
          changed={draftTier !== "" && draftTier !== tier}
          reasonLabel="Why did the tier change?"
          reasonHelp="Money moving is the kind of fact a colleague will want explained"
          saveLabel="Save tier"
          pending={patch.isPending}
          error={patch.error}
          onReset={() => setDraftTier(tier ?? "")}
          onSave={(reason, clear) =>
            patch.mutate(
              { subscriptionTier: draftTier as SubscriptionTier, reason },
              { onSuccess: clear },
            )
          }
          control={
            <TextField
              select
              {...selectLabelProps(draftTier)}
              fullWidth
              size="small"
              label="Tier"
              value={draftTier}
              onChange={(e) => setDraftTier(e.target.value as SubscriptionTier)}
            >
              {SUBSCRIPTION_TIERS.map((t) => (
                <MenuItem key={t} value={t}>
                  {humanizeEnum(t)}
                </MenuItem>
              ))}
            </TextField>
          }
          footer={
            tier && updatedBy ? (
              <Typography variant="caption" color="text.secondary">
                Set by {updatedBy.name}
                {enteredOn ? ` · ${formatDate(enteredOn)}` : ""}
              </Typography>
            ) : (
              <Typography variant="caption" color="text.secondary">
                No tier recorded yet.
              </Typography>
            )
          }
        />

        {dateField ? (
          <>
            <Divider />
            {/* Keyed on the field so switching tier remounts it: a draft belongs
                to one date, and carrying a half-typed trial date across to the
                extension would offer to save it against the wrong one. */}
            <PeriodDateEditor
              key={dateField}
              label={dateField === "trialEndDate" ? "Trial ends" : "Extended trial ends"}
              value={dateField === "trialEndDate" ? trialEndDate : trialExtendedDate}
              pending={patch.isPending}
              onSave={(day) =>
                patch.mutate(
                  day
                    ? { [dateField]: day }
                    : {
                        [dateField === "trialEndDate"
                          ? "clearTrialEndDate"
                          : "clearTrialExtendedDate"]: true,
                      },
                )
              }
            />
          </>
        ) : null}

        {patch.error ? <ErrorBlock error={patch.error} /> : null}
      </Stack>
    </SectionCard>
  );
}

/** The comment trail. Append-only: a note is a record of what was true then. */
function NotesCard({
  organizationId,
  productCode,
  detail,
}: {
  organizationId: string;
  productCode: string;
  detail: ProductDetail;
}) {
  const [body, setBody] = useState("");
  const create = useCreateNote(organizationId, productCode);

  return (
    <SectionCard title="Notes">
      <Stack direction="row" spacing={1} mb={2}>
        <TextField
          fullWidth
          size="small"
          multiline
          placeholder="Add a note"
          value={body}
          onChange={(e) => setBody(e.target.value)}
        />
        <Button
          variant="contained"
          disabled={!body.trim() || create.isPending}
          onClick={() => create.mutate(body.trim(), { onSuccess: () => setBody("") })}
        >
          Add
        </Button>
      </Stack>
      {create.error ? <ErrorBlock error={create.error} /> : null}

      {detail.notes.length === 0 ? (
        <EmptyState message="No notes yet" />
      ) : (
        <Stack spacing={1.5}>
          {detail.notes.map((note) => (
            <NoteRow
              key={note.id}
              note={note}
              organizationId={organizationId}
              productCode={productCode}
            />
          ))}
        </Stack>
      )}
    </SectionCard>
  );
}

/**
 * One note in the trail, editable by whoever wrote it.
 *
 * The id is on show, in monospace and select-all-on-click, because that is what
 * makes the edit history reachable: there is deliberately no UI for reading
 * superseded wordings, but the id never changes, so
 *
 *   SELECT body, edited_on, edited_by FROM plg_note_revision
 *    WHERE note_id = '<the id>' ORDER BY edited_on;
 *
 * answers "what did this say before" without a screen having to exist for it.
 */
function NoteRow({
  note,
  organizationId,
  productCode,
}: {
  note: Note;
  organizationId: string;
  productCode: string;
}) {
  // The resolved caller. The comparison below is an identity check, so it is
  // made on the id, never on the address.
  const { data: me } = useMe();
  const update = useUpdateNote(organizationId, productCode);
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(note.body);

  // Only the author, matching what the API enforces — offering an edit control
  // that answers 403 would be worse than not offering one. Compared on id: two
  // engineers cannot share one, and an address is no longer the identity.
  const mine = !!me?.id && note.author?.id === me.id;
  const changed = draft.trim() !== note.body && draft.trim() !== "";

  const start = () => {
    setDraft(note.body);
    setEditing(true);
  };

  return (
    <Box>
      <Stack direction="row" spacing={1} alignItems="center" flexWrap="wrap" useFlexGap>
        <Chip size="small" variant="outlined" label={note.author?.name ?? "Unknown"} />
        <Typography variant="caption" color="text.secondary">
          {formatDateTime(note.createdOn)}
        </Typography>
        {note.updatedOn ? (
          <Tooltip
            title={`Edited ${formatDateTime(note.updatedOn)}${
              note.updatedBy ? ` by ${note.updatedBy.name}` : ""
            } — earlier wordings are kept against this note's id`}
          >
            <Typography variant="caption" color="text.secondary" sx={{ fontStyle: "italic" }}>
              · edited {formatDateTime(note.updatedOn)}
            </Typography>
          </Tooltip>
        ) : null}
        {mine && !editing ? (
          <Button size="small" variant="text" onClick={start} sx={{ minWidth: 0, py: 0 }}>
            Edit
          </Button>
        ) : null}
      </Stack>

      {editing ? (
        <Stack spacing={1} mt={0.75}>
          <TextField
            fullWidth
            size="small"
            multiline
            autoFocus
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
          />
          <Stack direction="row" spacing={1} alignItems="center">
            <Button
              size="small"
              variant="contained"
              disabled={!changed || update.isPending}
              onClick={() =>
                update.mutate(
                  { noteId: note.id, body: draft.trim() },
                  { onSuccess: () => setEditing(false) },
                )
              }
            >
              {update.isPending ? "Saving…" : "Save"}
            </Button>
            <Button
              size="small"
              variant="text"
              color="inherit"
              disabled={update.isPending}
              onClick={() => setEditing(false)}
            >
              Cancel
            </Button>
            <Typography variant="caption" color="text.secondary">
              The wording it replaces is kept.
            </Typography>
          </Stack>
        </Stack>
      ) : (
        <Typography variant="body2" mt={0.5}>
          {note.body}
        </Typography>
      )}

      <Typography
        variant="caption"
        color="text.disabled"
        display="block"
        mt={0.5}
        sx={{ fontFamily: "monospace", fontSize: 11, userSelect: "all" }}
      >
        {note.id}
      </Typography>

      {update.error ? <ErrorBlock error={update.error} /> : null}
    </Box>
  );
}


/**
 * Health: the second axis, and the only control on this page that changes what
 * the page offers.
 *
 * Marking a pairing at risk swaps the playbook menu from progressive and
 * sustaining plays to recovery ones — see ProductDetail.applicablePlaybookTypes
 * — so it sits in the Lifecycle card next to the stage rather than in a
 * different corner of the tab.
 *
 * It takes a reason, like the other two axes. It is tempting to exempt it, on
 * the argument that going at risk is self-evident from the account and that
 * demanding a sentence means engineers leave it unmarked — but what the history
 * fills up with then is rows saying that somebody changed something on a
 * Tuesday — and "why is this account at risk" is the first question the next
 * engineer to open it has. The sentence is cheaper than its absence.
 *
 * It also no longer writes on the first click. A toggle group beside a diagram
 * legend is an easy thing to hit by accident, and every hit was a database
 * write and a history row.
 */
function HealthEditor({
  organizationId,
  productCode,
  current,
  enteredOn,
  updatedBy,
}: {
  organizationId: string;
  productCode: string;
  current: HealthState;
  enteredOn: string;
  updatedBy: UserRef | null;
}) {
  const patch = usePatchProduct(organizationId, productCode);
  const [draft, setDraft] = useState<HealthState>(current);

  return (
    <AxisEditor
      heading="Health"
      changed={draft !== current}
      reasonLabel="What changed?"
      reasonHelp="Say what happened — this is what the next engineer reads"
      saveLabel="Save health"
      pending={patch.isPending}
      error={patch.error}
      onReset={() => setDraft(current)}
      onSave={(reason, clear) =>
        patch.mutate({ healthState: draft, reason }, { onSuccess: clear })
      }
      control={
        <ToggleButtonGroup
          exclusive
          fullWidth
          size="small"
          value={draft}
          sx={{ height: 40 }}
          /* next is null when the pressed button was already selected. Ignoring
             that keeps a value always chosen: health has no "unset". */
          onChange={(_, next: HealthState | null) => {
            if (next) setDraft(next);
          }}
        >
          {HEALTH_STATES.map((h) => (
            <ToggleButton
              key={h}
              value={h}
              disabled={patch.isPending}
              color={h === "AT_RISK" ? "error" : "success"}
            >
              {HEALTH_LABEL[h]}
            </ToggleButton>
          ))}
        </ToggleButtonGroup>
      }
      footer={
        updatedBy ? (
          <Typography variant="caption" color="text.secondary">
            {HEALTH_LABEL[current]} since {formatDate(enteredOn)}, set by {updatedBy.name}
          </Typography>
        ) : (
          <Typography variant="caption" color="text.secondary">
            Nobody has changed this — it has been {HEALTH_LABEL[current].toLowerCase()} since
            registration.
          </Typography>
        )
      }
    />
  );
}
