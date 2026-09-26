import { PLAYBOOK_TYPE_LABEL } from "@features/plg/api/types";
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Alert,
  Box,
  Button,
  Checkbox,
  Chip,
  FormControlLabel,
  FormGroup,
  IconButton,
  LinearProgress,
  MenuItem,
  Stack,
  TextField,
  Tooltip,
  Typography,
  Radio,
  RadioGroup,
} from "@wso2/oxygen-ui";
import {
  ChevronDown as ExpandMoreIcon,
  Trash2 as DeleteIcon,
  Undo2 as UndoIcon,
} from "@wso2/oxygen-ui-icons-react";
import { useState } from "react";




import { useAttachPlaybook, useDetachRun, usePatchRunTask } from "@features/plg/api/hooks";
import { TASK_CODE_INITIATE } from "@features/plg/api/types";
import type { PlaybookRun, ProductDetail, RunTask } from "@features/plg/api/types";
import { EmptyState, ErrorBlock, SectionCard, StatusChip } from "@features/plg/components/common";
import { formatDate, formatDateTime } from "@features/plg/utils/format";
import { selectLabelProps } from "@features/plg/components/selectLabelProps";

/**
 * The playbooks running on one pairing.
 *
 * A run is a copy of its template, never a reference, so a template edited
 * tomorrow cannot rewrite work recorded today. That is why the tasks here have
 * their own names and types rather than reading through to the playbook.
 */
export function PlaybookRuns({
  organizationId,
  productCode,
  detail,
}: {
  organizationId: string;
  productCode: string;
  detail: ProductDetail;
}) {
  const attach = useAttachPlaybook(organizationId, productCode);
  const [selected, setSelected] = useState("");

  const alreadyRunning = new Set(detail.runs.map((r) => r.playbookId));
  const available = detail.availablePlaybooks.filter((p) => !alreadyRunning.has(p.id));

  return (
    <SectionCard title="Playbooks">
      {detail.carriesPlaybooks ? (
        <Stack direction="row" spacing={1} mb={2}>
          <TextField
            select
            {...selectLabelProps(selected)}
            fullWidth
            size="small"
            label={
              available.length
                ? detail.healthState === "AT_RISK"
                  ? "Add a recovery playbook"
                  : "Add a playbook for this stage"
                : "No playbooks left to add here"
            }
            value={selected}
            disabled={!available.length}
            onChange={(e) => setSelected(e.target.value)}
          >
            {/* The kind is shown on every row because the menu can now hold
                two of them at once: a healthy pairing is offered the plays that
                move it on AND the plays that keep it steady, and those are
                different decisions with the same-looking names. An at-risk
                pairing sees recovery plays only, so the chip is redundant
                there — and a chip that is only sometimes informative is still
                better than a name that is sometimes ambiguous. */}
            {available.map((p) => (
              <MenuItem key={p.id} value={p.id}>
                <Stack direction="row" spacing={1} alignItems="center">
                  <span>{p.name}</span>
                  <StatusChip value={p.playbookType} kind="playbookType" />
                </Stack>
              </MenuItem>
            ))}
          </TextField>
          <Button
            variant="contained"
            disabled={!selected || attach.isPending}
            onClick={() => attach.mutate(selected, { onSuccess: () => setSelected("") })}
          >
            Add
          </Button>
        </Stack>
      ) : (
        <Typography variant="body2" color="text.secondary" mb={2}>
          No {detail.healthState === "AT_RISK" ? "recovery" : "progressive or sustaining"} playbook
          has been authored for {detail.stageName}. Nothing here is broken — it means an owner records
          the change when the facts change, rather than working through a play.
        </Typography>
      )}
      {attach.error ? <ErrorBlock error={attach.error} /> : null}

      {detail.runs.length === 0 ? (
        <EmptyState message="No playbooks have been added to this pairing" />
      ) : (
        <Stack spacing={1}>
          {detail.runs.map((run) => (
            <RunCard
              key={run.id}
              run={run}
              organizationId={organizationId}
              productCode={productCode}
            />
          ))}
        </Stack>
      )}
    </SectionCard>
  );
}

function RunCard({
  run,
  organizationId,
  productCode,
}: {
  run: PlaybookRun;
  organizationId: string;
  productCode: string;
}) {
  const detach = useDetachRun(organizationId, productCode);
  const percent = run.taskTotal ? (run.taskCompleted / run.taskTotal) * 100 : 0;

  // Until the opening bookend is ticked, nothing else in the run may be
  // recorded. "Initiate playbook" is the moment the work starts, so a task
  // answered before it would be an answer to a question nobody had asked yet —
  // and it would leave the run counted as NOT_STARTED while carrying values.
  //
  // Read from the task rather than from run.runStatus: a CLOSED run is also
  // initiated, and testing the task directly says so without a second rule.
  const initiated = run.tasks.some((t) => t.code === TASK_CODE_INITIATE && t.isCompleted);

  return (
    <Accordion disableGutters sx={{ border: "1px solid", borderColor: "divider", borderRadius: 2 }}>
      <AccordionSummary expandIcon={<ExpandMoreIcon />}>
        <Box flexGrow={1} mr={2}>
          <Stack direction="row" spacing={1} alignItems="center" flexWrap="wrap" useFlexGap>
            <Typography variant="body2" fontWeight={600}>
              {run.playbookName}
            </Typography>
            <StatusChip value={run.runStatus} kind="runStatus" />
            <Chip
              size="small"
              variant="outlined"
              label={PLAYBOOK_TYPE_LABEL[run.playbookType]}
            />
          </Stack>
          <Typography variant="caption" color="text.secondary">
            {run.taskCompleted}/{run.taskTotal} tasks
            {run.nextTaskName ? ` · next: ${run.nextTaskName}` : ""}
          </Typography>
          <LinearProgress variant="determinate" value={percent} sx={{ mt: 0.75, height: 5, borderRadius: 3 }} />
        </Box>
        {run.runStatus !== "CLOSED" ? (
          <Tooltip title="Remove this playbook from the pairing">
            <IconButton
              size="small"
              onClick={(e) => {
                e.stopPropagation();
                detach.mutate(run.id);
              }}
            >
              <DeleteIcon size={20} />
            </IconButton>
          </Tooltip>
        ) : null}
      </AccordionSummary>

      <AccordionDetails>
        {run.description ? (
          <Typography variant="body2" color="text.secondary" mb={1.5}>
            {run.description}
          </Typography>
        ) : null}
        {detach.error ? <ErrorBlock error={detach.error} /> : null}
        {!initiated ? (
          <Alert severity="info" sx={{ mb: 1.5 }}>
            Tick <strong>Initiate playbook</strong> to start this run. The remaining tasks stay read-only
            until you do.
          </Alert>
        ) : null}
        <Stack spacing={1}>
          {run.tasks.map((task) => (
            <TaskRow
              key={task.id}
              task={task}
              organizationId={organizationId}
              productCode={productCode}
              locked={!initiated && task.code !== TASK_CODE_INITIATE}
            />
          ))}
        </Stack>
        <Typography variant="caption" color="text.secondary" display="block" mt={1.5}>
          Added {formatDate(run.createdOn)}
          {run.addedBy ? ` by ${run.addedBy.name}` : ""}
        </Typography>
      </AccordionDetails>
    </Accordion>
  );
}

/**
 * The reasons on a checklist task.
 *
 * Every tick sends the *whole* set rather than a delta, which is what the API
 * expects: it makes the call idempotent, and two engineers ticking at once
 * cannot interleave into a state neither of them chose.
 *
 * Unticking the last reason clears the task, because a reason picker with no
 * reason ticked is an unanswered question — the same rule the backend applies.
 */
function ReasonPicker({
  task,
  onChange,
  busy,
  locked,
}: {
  task: RunTask;
  onChange: (checkedCodes: string[]) => void;
  busy: boolean;
  locked: boolean;
}) {
  const checked = new Set(task.checkedCodes ?? []);

  const toggle = (code: string) => {
    const next = new Set(checked);
    if (next.has(code)) {
      next.delete(code);
    } else {
      next.add(code);
    }
    // Sent in the task's own option order, so the stored set reads the way the
    // playbook author wrote it rather than the order someone happened to click.
    onChange((task.options ?? []).map((o) => o.code).filter((c) => next.has(c)));
  };

  return (
    <Box mt={0.5}>
      <FormGroup>
        {(task.options ?? []).map((option) => (
          <FormControlLabel
            key={option.code}
            control={
              <Checkbox
                size="small"
                sx={{ py: 0.25 }}
                checked={checked.has(option.code)}
                disabled={busy || locked}
                onChange={() => toggle(option.code)}
              />
            }
            label={<Typography variant="body2">{option.label}</Typography>}
          />
        ))}
      </FormGroup>
      {checked.size > 0 ? (
        <Typography variant="caption" color="text.secondary">
          {checked.size} of {(task.options ?? []).length} reasons recorded
        </Typography>
      ) : locked ? null : (
        <Typography variant="caption" color="text.secondary">
          Tick at least one reason to complete this task
        </Typography>
      )}
    </Box>
  );
}

/**
 * One task.
 *
 * Recording a value is what completes a task — there is no separate tick to
 * forget, and no way for a status and a value to disagree, because completion is
 * computed from the value in the database.
 *
 * `locked` means the run has not been initiated yet. Every control is disabled
 * and dimmed rather than hidden: the engineer should be able to read the whole
 * playbook before starting it, and see what starting it will ask of them. The
 * lock is advice, not enforcement — the API accepts these values in any order,
 * so a script or a replay is unaffected.
 */
function TaskRow({
  task,
  organizationId,
  productCode,
  locked = false,
}: {
  task: RunTask;
  organizationId: string;
  productCode: string;
  locked?: boolean;
}) {
  const patch = usePatchRunTask(organizationId, productCode);
  const [text, setText] = useState(task.textValue ?? "");
  const [num, setNum] = useState(task.numberValue === null ? "" : String(task.numberValue));

  const save = (body: Parameters<typeof patch.mutate>[0]["body"]) =>
    patch.mutate({ taskId: task.id, body });

  const busy = patch.isPending || locked;

  const row = (
    <Box
      sx={{
        p: 1.25,
        borderRadius: 2,
        border: "1px solid",
        borderColor: task.isCompleted ? "success.light" : "divider",
        bgcolor: task.isBookend ? "action.hover" : "transparent",
        opacity: locked ? 0.55 : 1,
      }}
    >
      <Stack direction="row" spacing={1} alignItems="flex-start">
        {task.valueType === "BOOLEAN" ? (
          <Checkbox
            size="small"
            sx={{ p: 0.5 }}
            checked={task.isCompleted}
            disabled={busy}
            onChange={(e) => save(e.target.checked ? { boolValue: true } : { clearValue: true })}
          />
        ) : null}

        <Box flexGrow={1}>
          <Stack direction="row" spacing={1} alignItems="center" flexWrap="wrap" useFlexGap>
            <Typography variant="body2" fontWeight={task.isBookend ? 700 : 500}>
              {task.name}
            </Typography>
            {task.valueType !== "BOOLEAN" ? <StatusChip value={task.valueType} kind="valueType" /> : null}
            {task.isBookend ? <Chip size="small" variant="outlined" label="Structural" /> : null}
          </Stack>

          {task.description ? (
            <Typography variant="caption" color="text.secondary" display="block">
              {task.description}
            </Typography>
          ) : null}

          {task.valueType === "STRING" ? (
            <Stack direction="row" spacing={1} mt={1}>
              <TextField
                fullWidth
                size="small"
                multiline
                placeholder="Record the answer"
                value={text}
                disabled={locked}
                onChange={(e) => setText(e.target.value)}
              />
              <Button
                size="small"
                variant="outlined"
                disabled={busy || text.trim() === (task.textValue ?? "").trim()}
                onClick={() => save(text.trim() ? { textValue: text.trim() } : { clearValue: true })}
              >
                Save
              </Button>
            </Stack>
          ) : null}

          {task.valueType === "CHECKLIST" ? (
            <ReasonPicker
              task={task}
              onChange={(checkedCodes) => save({ checkedCodes })}
              busy={patch.isPending}
              locked={locked}
            />
          ) : null}

          {task.valueType === "SINGLE_SELECT" ? (
            <SingleChoicePicker
              task={task}
              onChange={(code) => save({ textValue: code })}
              busy={patch.isPending}
              locked={locked}
            />
          ) : null}

          {task.valueType === "NUMBER" ? (
            <Stack direction="row" spacing={1} mt={1} alignItems="center">
              <TextField
                size="small"
                type="number"
                placeholder="Record the number"
                value={num}
                disabled={locked}
                onChange={(e) => setNum(e.target.value)}
                sx={{ maxWidth: 200 }}
              />
              <Button
                size="small"
                variant="outlined"
                disabled={busy || num === (task.numberValue === null ? "" : String(task.numberValue))}
                onClick={() => save(num === "" ? { clearValue: true } : { numberValue: Number(num) })}
              >
                Save
              </Button>
            </Stack>
          ) : null}

          {task.isCompleted && task.completedOn ? (
            <Typography variant="caption" color="text.secondary" display="block" mt={0.5}>
              Completed {formatDateTime(task.completedOn)}
              {task.completedBy ? ` by ${task.completedBy.name}` : ""}
            </Typography>
          ) : null}
        </Box>

        {task.isCompleted &&
        task.valueType !== "BOOLEAN" &&
        task.valueType !== "CHECKLIST" &&
        task.valueType !== "SINGLE_SELECT" ? (
          <Tooltip title="Clear the value and reopen this task">
            <IconButton size="small" disabled={busy} onClick={() => save({ clearValue: true })}>
              <UndoIcon size={20} />
            </IconButton>
          </Tooltip>
        ) : null}
      </Stack>

      {patch.error ? <ErrorBlock error={patch.error} /> : null}
    </Box>
  );

  // A disabled control swallows pointer events, so the tooltip goes on the row
  // rather than on the input the engineer is actually trying to click.
  return locked ? (
    <Tooltip title="Tick “Initiate playbook” first — this run has not been started" followCursor>
      <Box>{row}</Box>
    </Tooltip>
  ) : (
    row
  );
}


/**
 * A SINGLE_SELECT task: the same authored options as a checklist, but exactly
 * one answer.
 *
 * Radios rather than a dropdown. The option list is short by construction — two
 * is the minimum and playbook authors write handfuls, not hundreds — and radios
 * show every choice at once, which is what an engineer reading a playbook wants.
 * A dropdown would hide the alternatives behind a click.
 *
 * The chosen option's CODE travels in `textValue`: a single-select's answer is a
 * scalar, and that is the field the API has for one. What makes it a choice
 * rather than free text is the options beside it, which the server validates
 * against.
 */
function SingleChoicePicker({
  task,
  onChange,
  busy,
  locked,
}: {
  task: RunTask;
  onChange: (code: string) => void;
  busy: boolean;
  locked: boolean;
}) {
  const options = task.options ?? [];
  const chosen = task.textValue ?? "";

  return (
    <Box mt={0.5}>
      <RadioGroup value={chosen} onChange={(_, code) => onChange(code)}>
        {options.map((option) => (
          <FormControlLabel
            key={option.code}
            value={option.code}
            control={<Radio size="small" sx={{ py: 0.25 }} disabled={busy || locked} />}
            label={<Typography variant="body2">{option.label}</Typography>}
          />
        ))}
      </RadioGroup>
      {chosen === "" && !locked ? (
        <Typography variant="caption" color="text.secondary">
          Choose one to complete this task
        </Typography>
      ) : null}
    </Box>
  );
}
