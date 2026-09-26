import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Alert,
  alpha,
  Box,
  Button,
  Card,
  Chip,
  Grid,
  IconButton,
  MenuItem,
  Stack,
  Switch,
  TextField,
  Tooltip,
  Typography,
} from "@wso2/oxygen-ui";
import {
  ArrowDown as DownIcon,
  ArrowUp as UpIcon,
  ChevronDown as ExpandMoreIcon,
  Lock as LockIcon,
  Plus as AddIcon,
  Square as CheckBoxIcon,
  Trash2 as DeleteIcon,
} from "@wso2/oxygen-ui-icons-react";
import { useMemo, useState } from "react";



import {
  useCreatePlaybook,
  useDeletePlaybook,
  useLifecycle,
  usePatchPlaybook,
  usePlaybooks,
  useProducts,
  useReplacePlaybookTasks,
} from "@features/plg/api/hooks";
import {
  MIN_CHECKLIST_OPTIONS,
  TASK_VALUE_TYPES,
  type ChecklistOption,
  type LifecycleCatalogue,
  type LifecycleStage,
  type PlaybookType,
  PLAYBOOK_TYPES,
  PLAYBOOK_TYPE_LABEL,
  PLAYBOOK_TYPE_HELP,
  typeNeedsOptions,
  TASK_VALUE_TYPE_LABEL,
  type Playbook,
  type PlaybookTaskInput,
  type TaskValueType,
} from "@features/plg/api/types";
import { EmptyState, ErrorBlock, LoadingBlock, PageHeader, SectionCard, StatusChip } from "@features/plg/components/common";
import { selectLabelProps } from "@features/plg/components/selectLabelProps";

/**
 * The playbook manager.
 *
 * A playbook belongs to a platform and a source stage, and declares the stage it
 * aims at. Only the six stages that carry playbooks can be chosen, and only the
 * targets those stages allow — the pickers read the same catalogue the database
 * constrains against, so an editor cannot build something the backend will
 * refuse.
 *
 * Editing a template never touches runs already in flight. Task instances are
 * copies, so a change reaches new runs only.
 */
export default function PlaybooksPage() {
  const { data: products } = useProducts();
  const { data: lifecycle } = useLifecycle();
  const [productCode, setProductCode] = useState("");
  const [creating, setCreating] = useState(false);
  const { data: playbooks, isPending, error } = usePlaybooks(productCode || undefined);

  const grouped = useMemo(() => {
    const map = new Map<string, Playbook[]>();
    for (const pb of playbooks ?? []) {
      const list = map.get(pb.product.code) ?? [];
      list.push(pb);
      map.set(pb.product.code, list);
    }
    return map;
  }, [playbooks]);

  return (
    <Box sx={{ display: "flex", flexDirection: "column" }}>
      {/* One root element, because csm-portal's AppLayout renders <Outlet /> into a
          column flex box. A fragment made each child of this page its own flex item,
          and MUI Card is overflow:hidden — so flexbox shrank the filter card and it
          clipped instead of the page scrolling. */}
      <PageHeader
        title="Playbook manager"
        subtitle="Templates that move a pairing along one of the seven paths"
        actions={
          <Stack direction="row" spacing={1}>
            <TextField
              select
              {...selectLabelProps(productCode)}
              size="small"
              label="Platform"
              value={productCode}
              onChange={(e) => setProductCode(e.target.value)}
              sx={{ minWidth: 220 }}
            >
              <MenuItem value="">All platforms</MenuItem>
              {(products ?? []).map((p) => (
                <MenuItem key={p.code} value={p.code}>
                  {p.name}
                </MenuItem>
              ))}
            </TextField>
            <Button
              variant="contained"
              startIcon={<AddIcon />}
              disabled={!productCode}
              onClick={() => setCreating(true)}
            >
              New playbook
            </Button>
          </Stack>
        }
      />

      {!productCode ? (
        <Alert severity="info" sx={{ mb: 2 }}>
          Pick a platform to add a playbook to it.
        </Alert>
      ) : null}

      {creating && productCode && lifecycle ? (
        <Box mb={2}>
          <NewPlaybookForm
            productCode={productCode}
            catalogue={lifecycle}
            onDone={() => setCreating(false)}
          />
        </Box>
      ) : null}

      {error ? <ErrorBlock error={error} /> : null}
      {isPending && !playbooks ? <LoadingBlock height={400} /> : null}

      {playbooks && playbooks.length === 0 ? <EmptyState message="No playbooks yet" /> : null}

      <Stack spacing={2}>
        {[...grouped.entries()].map(([code, list]) => (
          <SectionCard key={code} title={list[0]?.product.name ?? code}>
            <Stack spacing={1}>
              {list.map((pb) => (
                <PlaybookCard key={pb.id} playbook={pb} />
              ))}
            </Stack>
          </SectionCard>
        ))}
      </Stack>
    </Box>
  );
}

/**
 * The stages a playbook may sit on: all of them except ABANDONED.
 *
 * Every stage carries playbooks, of every kind, so the only exclusion is the
 * terminal one — nothing progresses out of ABANDONED, and a pairing there is
 * already gone.
 */
function stageOptions(catalogue: LifecycleCatalogue) {
  return catalogue.stages.filter((s) => !s.terminal);
}

function NewPlaybookForm({
  productCode,
  catalogue,
  onDone,
}: {
  productCode: string;
  catalogue: LifecycleCatalogue;
  onDone: () => void;
}) {
  const create = useCreatePlaybook(productCode);
  const sources = stageOptions(catalogue);

  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  // The authoring sequence: the product is chosen above, then the kind, then
  // the stage. Kind before stage because it changes what the stage means — the
  // same stage holds a playbook that moves a pairing on, one that rescues it and
  // one that holds it steady, and those are not alternatives to each other.
  //
  // BOTH START EMPTY. They defaulted to "Progressive" and the first stage,
  // which meant an author who filled in a name and pressed Create published a
  // playbook they had made no decision about — and the two defaults happen to
  // be the most common real answer, so the mistake produced a plausible
  // playbook at the wrong stage rather than an obvious one. An empty box is
  // what turns the choice into a choice.
  const [playbookType, setPlaybookType] = useState<PlaybookType | "">("");
  const [stage, setStage] = useState<LifecycleStage | "">("");
  const [tasks, setTasks] = useState<PlaybookTaskInput[]>([{ name: "", valueType: "BOOLEAN" }]);

  const nameMissing = name.trim() === "";
  const kindMissing = playbookType === "";
  const stageMissing = stage === "";
  const incomplete = nameMissing || kindMissing || stageMissing;

  // What this playbook achieves, in the stage's own words. There is no target to
  // pick — the destination is a property of the stage — so it is shown, not asked.
  const stageInfo = catalogue.stages.find((s) => s.stage === stage);
  const outcome = !playbookType
    ? "Pick a kind first — it decides what the stage below means."
    : playbookType === "RECOVERY"
      ? "Brings an at-risk pairing back to healthy, without moving it."
      : playbookType === "SUSTAINING"
        ? "Keeps a healthy pairing steady. It is not a way out of the stage."
        : !stageInfo
          ? "Pick the stage this playbook runs at."
          : stageInfo.nextStage
            ? `Carries a pairing on to ${
                catalogue.stages.find((s) => s.stage === stageInfo.nextStage)?.name ??
                stageInfo.nextStage
              }.`
            : "The last stage in the progression — nothing follows it.";

  return (
    <Card sx={{ p: 2 }}>
      <Typography variant="h6" mb={1.5}>
        New playbook
      </Typography>

      <Grid container spacing={1.5}>
        <Grid size={{ xs: 12, md: 6 }}>
          <TextField
            fullWidth
            size="small"
            required
            label="Name"
            value={name}
            error={nameMissing}
            helperText={nameMissing ? "A playbook needs a name" : " "}
            onChange={(e) => setName(e.target.value)}
          />
        </Grid>
        <Grid size={{ xs: 12, sm: 6, md: 3 }}>
          <TextField
            select
            {...selectLabelProps(playbookType)}
            fullWidth
            size="small"
            required
            label="Kind"
            value={playbookType}
            error={kindMissing}
            helperText={
              playbookType ? PLAYBOOK_TYPE_HELP[playbookType] : "Required — what is this playbook for?"
            }
            onChange={(e) => setPlaybookType(e.target.value as PlaybookType)}
          >
            {PLAYBOOK_TYPES.map((t) => (
              <MenuItem key={t} value={t}>
                {PLAYBOOK_TYPE_LABEL[t]}
              </MenuItem>
            ))}
          </TextField>
        </Grid>
        <Grid size={{ xs: 12, sm: 6, md: 3 }}>
          <TextField
            select
            {...selectLabelProps(stage)}
            fullWidth
            size="small"
            required
            label="Runs at stage"
            value={stage}
            error={stageMissing}
            helperText={outcome}
            onChange={(e) => setStage(e.target.value as LifecycleStage)}
          >
            {sources.map((s) => (
              <MenuItem key={s.stage} value={s.stage}>
                {s.name}
              </MenuItem>
            ))}
          </TextField>
        </Grid>
        <Grid size={12}>
          <TextField
            fullWidth
            size="small"
            label="Description"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
          />
        </Grid>
      </Grid>

      <TaskEditor tasks={tasks} onChange={setTasks} />

      {create.error ? <ErrorBlock error={create.error} /> : null}

      <Stack direction="row" spacing={1} mt={2}>
        <Tooltip
          title={incomplete ? "Name, kind and stage are all required" : ""}
        >
          <span>
            <Button
              variant="contained"
              disabled={incomplete || create.isPending}
              onClick={() =>
                create.mutate(
                  {
                    name: name.trim(),
                    description: description.trim() || null,
                    lifecycleStage: stage as LifecycleStage,
                    playbookType: playbookType as PlaybookType,
                    tasks: tasks.filter((t) => t.name.trim()),
                  },
                  { onSuccess: onDone },
                )
              }
            >
              {create.isPending ? "Creating…" : "Create playbook"}
            </Button>
          </span>
        </Tooltip>
        <Button onClick={onDone}>Cancel</Button>
      </Stack>
    </Card>
  );
}

/**
 * What the author has actually written, as a comparable string.
 *
 * Compares name, description, type and reasons in order — not `code`, which the
 * author never edits and the backend fills in. Anything else and a freshly
 * saved list would keep reading as unsaved forever.
 */
function taskListSignature(tasks: PlaybookTaskInput[]): string {
  return JSON.stringify(
    tasks.map((t) => [
      t.name.trim(),
      (t.description ?? "").trim(),
      t.valueType ?? "BOOLEAN",
      (t.options ?? []).map((o) => o.label.trim()),
    ]),
  );
}

function PlaybookCard({ playbook }: { playbook: Playbook }) {
  const patch = usePatchPlaybook(playbook.id);
  const replaceTasks = useReplacePlaybookTasks(playbook.id);
  const remove = useDeletePlaybook();

  // The bookends are structural, so the editor works on the middle only and the
  // backend puts them back where they belong.
  const saved: PlaybookTaskInput[] = playbook.tasks
    .filter((t) => !t.isBookend)
    .map((t) => ({
      code: t.code,
      name: t.name,
      description: t.description,
      valueType: t.valueType,
      options: t.options ?? undefined,
    }));

  const [tasks, setTasks] = useState<PlaybookTaskInput[]>(saved);

  // The four fields above the task list are staged behind the same Save as the
  // tasks. Writing them on blur or on click would make the bar at the bottom of
  // this card untrue — it would claim nothing above it had reached the server
  // while four things already had —
  // and it made the Active switch a one-click way to withdraw a playbook from
  // every pairing that might otherwise have been offered it.
  const [name, setName] = useState(playbook.name);
  const [description, setDescription] = useState(playbook.description ?? "");
  const [kind, setKind] = useState<PlaybookType>(playbook.playbookType);
  const [active, setActive] = useState(playbook.active);

  // Editing is local until it is saved. Adding a task shows it immediately —
  // which is what makes the editor usable — but it exists only in the browser
  // until "Save changes" sends it, so the author needs to be told the
  // difference. `dirty` is what tells them.
  const tasksDirty = taskListSignature(tasks) !== taskListSignature(saved);
  const detailsDirty =
    name.trim() !== playbook.name ||
    description.trim() !== (playbook.description ?? "") ||
    kind !== playbook.playbookType ||
    active !== playbook.active;
  const dirty = tasksDirty || detailsDirty;

  // A name is the one field with no sensible empty value — the card would lose
  // its heading — so an empty box blocks the save rather than sending a blank.
  const nameMissing = name.trim() === "";

  // A saved edit is the new baseline. Both mutations invalidate the playbook
  // query, so the props arrive updated on the next render and `dirty` returns
  // to false on its own — nothing here has to remember what was just sent.
  const discard = () => {
    setTasks(saved);
    setName(playbook.name);
    setDescription(playbook.description ?? "");
    setKind(playbook.playbookType);
    setActive(playbook.active);
  };

  // Only what actually changed is sent. Two requests when both halves moved,
  // because the details and the task list are separate endpoints — the task
  // replacement is the destructive one and is kept on its own route so it
  // cannot be triggered by a rename.
  const saveAll = () => {
    if (detailsDirty) {
      patch.mutate({
        name: name.trim(),
        description: description.trim() || null,
        playbookType: kind,
        active,
      });
    }
    if (tasksDirty) replaceTasks.mutate(tasks.filter((t) => t.name.trim()));
  };


  return (
    <Accordion disableGutters sx={{ border: "1px solid", borderColor: "divider", borderRadius: 2 }}>
      <AccordionSummary expandIcon={<ExpandMoreIcon />}>
        <Box flexGrow={1} mr={2}>
          <Stack direction="row" spacing={1} alignItems="center" flexWrap="wrap" useFlexGap>
            <Typography variant="body2" fontWeight={600}>
              {playbook.name}
            </Typography>
            <StatusChip value={playbook.lifecycleStage} kind="lifecycle" />
            <Chip size="small" label={PLAYBOOK_TYPE_LABEL[playbook.playbookType]} />
            {!playbook.active ? <Chip size="small" label="Inactive" /> : null}
          </Stack>
          <Typography variant="caption" color="text.secondary">
            {playbook.tasks.length} tasks · {playbook.activeRuns} open of {playbook.runCount} runs
          </Typography>
        </Box>
        <Tooltip
          title={
            playbook.runCount
              ? "In use by a pairing — it can be deactivated but not deleted"
              : "Delete this playbook"
          }
        >
          <span>
            <IconButton
              size="small"
              disabled={playbook.runCount > 0 || remove.isPending}
              onClick={(e) => {
                e.stopPropagation();
                remove.mutate(playbook.id);
              }}
            >
              <DeleteIcon size={20} />
            </IconButton>
          </span>
        </Tooltip>
      </AccordionSummary>

      <AccordionDetails>
        <Grid container spacing={1.5} mb={1}>
          <Grid size={{ xs: 12, md: 6 }}>
            <TextField
              fullWidth
              size="small"
              required
              label="Name"
              value={name}
              error={nameMissing}
              helperText={nameMissing ? "A playbook needs a name" : " "}
              onChange={(e) => setName(e.target.value)}
            />
          </Grid>
          <Grid size={{ xs: 12, sm: 6, md: 3 }}>
            <TextField
              select
              {...selectLabelProps(kind)}
              fullWidth
              size="small"
              label="Kind"
              value={kind}
              onChange={(e) => setKind(e.target.value as PlaybookType)}
              helperText={PLAYBOOK_TYPE_HELP[kind]}
            >
              {PLAYBOOK_TYPES.map((t) => (
                <MenuItem key={t} value={t}>
                  {PLAYBOOK_TYPE_LABEL[t]}
                </MenuItem>
              ))}
            </TextField>
          </Grid>
          <Grid size={{ xs: 12, sm: 6, md: 3 }}>
            <Stack direction="row" spacing={1} alignItems="center" height="100%">
              <Switch checked={active} onChange={(e) => setActive(e.target.checked)} />
              <Typography variant="body2">{active ? "Active" : "Inactive"}</Typography>
            </Stack>
          </Grid>
          <Grid size={12}>
            <TextField
              fullWidth
              size="small"
              label="Description"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </Grid>
        </Grid>

        {patch.error ? <ErrorBlock error={patch.error} /> : null}

        <TaskEditor tasks={tasks} onChange={setTasks} />

        {replaceTasks.error ? <ErrorBlock error={replaceTasks.error} /> : null}
        {remove.error ? <ErrorBlock error={remove.error} /> : null}

        {/* The confirm step. Nothing typed above has reached the server yet, so
            this bar is the only thing standing between an edit and losing it on
            navigation — it says so plainly, and offers the way back. */}
        <Stack
          direction="row"
          spacing={1}
          alignItems="center"
          flexWrap="wrap"
          useFlexGap
          mt={1.5}
          sx={{
            p: dirty ? 1.25 : 0,
            borderRadius: 2,
            border: dirty ? "1px solid" : "none",
            borderColor: "warning.light",
            // alpha() rather than a "warning.50" token: MUI's palette carries
            // main/light/dark, not numeric shades, so that key resolves to
            // nothing and the bar would lose its tint in silence.
            bgcolor: dirty ? (theme) => alpha(theme.palette.warning.main, 0.08) : "transparent",
          }}
        >
          <Button
            variant="contained"
            size="small"
            disabled={!dirty || nameMissing || replaceTasks.isPending || patch.isPending}
            onClick={saveAll}
          >
            {replaceTasks.isPending || patch.isPending ? "Saving…" : "Save changes"}
          </Button>
          {dirty ? (
            <>
              <Button
                size="small"
                variant="text"
                color="inherit"
                    disabled={replaceTasks.isPending || patch.isPending}
                onClick={discard}
              >
                Discard
              </Button>
              <Typography variant="caption" sx={{ fontWeight: 600 }}>
                {tasksDirty && detailsDirty
                  ? "Unsaved changes to the details and the task list"
                  : tasksDirty
                    ? "Unsaved changes to the task list"
                    : "Unsaved changes to the details"}
              </Typography>
            </>
          ) : (
            <Typography variant="caption" color="text.secondary">
              Saved. Runs already in flight keep the tasks they started with.
            </Typography>
          )}
        </Stack>
      </AccordionDetails>
    </Accordion>
  );
}

/**
 * The task list as the author sees it.
 *
 * The two bookends are rendered in place — first and last, dimmed and locked —
 * rather than left out and explained in a note. They are real tasks that will
 * appear on every run, so showing them is simply more honest than describing
 * them: the author sees the shape of what they are building, and why their own
 * tasks are numbered from two.
 *
 * They are display-only. `tasks` and `onChange` still carry the middle of the
 * list alone, which is exactly what the API expects, so nothing about the
 * contract changes.
 */
function TaskEditor({
  tasks,
  onChange,
}: {
  tasks: PlaybookTaskInput[];
  onChange: (tasks: PlaybookTaskInput[]) => void;
}) {
  const update = (index: number, patch: Partial<PlaybookTaskInput>) =>
    onChange(tasks.map((t, i) => (i === index ? { ...t, ...patch } : t)));

  /**
   * Switching the type manages the options for the author.
   *
   * Becoming a checklist seeds two blank reasons, because two is the minimum and
   * an empty editor gives no clue that reasons are required. Ceasing to be one
   * drops them, because the API refuses options on any other type.
   */
  const changeType = (index: number, valueType: TaskValueType) => {
    // Both list types behave identically here — "choose one" and "choose any"
    // differ in how the answer is recorded, not in how the answers are authored.
    if (typeNeedsOptions(valueType)) {
      const existing = tasks[index].options ?? [];
      const seeded =
        existing.length >= MIN_CHECKLIST_OPTIONS
          ? existing
          : [...existing, ...Array(MIN_CHECKLIST_OPTIONS - existing.length).fill({ code: "", label: "" })];
      update(index, { valueType, options: seeded });
      return;
    }
    update(index, { valueType, options: undefined });
  };

  const move = (index: number, delta: number) => {
    const next = [...tasks];
    const target = index + delta;
    if (target < 0 || target >= next.length) return;
    [next[index], next[target]] = [next[target], next[index]];
    onChange(next);
  };

  return (
    <Box mt={1.5}>
      <Typography variant="subtitle2" mb={1}>
        Tasks
      </Typography>

      <Stack spacing={1}>
        <BookendTask
          position={1}
          name="Initiate playbook"
          hint="Completing this starts the run"
        />

        {tasks.map((task, index) => (
          <Box key={index}>
            <Stack direction="row" spacing={1} alignItems="flex-start">
              <TextField
                size="small"
                label={`Task ${index + 2}`}
                fullWidth
                value={task.name}
                onChange={(e) => update(index, { name: e.target.value })}
              />
              <TextField
                select
                {...selectLabelProps(task.valueType ?? "BOOLEAN")}
                size="small"
                label="Holds"
                value={task.valueType ?? "BOOLEAN"}
                onChange={(e) => changeType(index, e.target.value as TaskValueType)}
                sx={{ minWidth: 150 }}
              >
                {TASK_VALUE_TYPES.map((t) => (
                  <MenuItem key={t} value={t}>
                    {TASK_VALUE_TYPE_LABEL[t]}
                  </MenuItem>
                ))}
              </TextField>
              <IconButton size="small" onClick={() => move(index, -1)} disabled={index === 0}>
                <UpIcon size={20} />
              </IconButton>
              <IconButton size="small" onClick={() => move(index, 1)} disabled={index === tasks.length - 1}>
                <DownIcon size={20} />
              </IconButton>
              <IconButton size="small" onClick={() => onChange(tasks.filter((_, i) => i !== index))}>
                <DeleteIcon size={20} />
              </IconButton>
            </Stack>

            {typeNeedsOptions(task.valueType ?? "BOOLEAN") ? (
              <ReasonEditor
                options={task.options ?? []}
                onChange={(options) => update(index, { options })}
              />
            ) : null}
          </Box>
        ))}

        <BookendTask
          position={tasks.length + 2}
          name="Close playbook"
          hint="Completing this closes the run"
        />
      </Stack>

      <Button
        size="small"
        startIcon={<AddIcon />}
        sx={{ mt: 1 }}
        onClick={() => onChange([...tasks, { name: "", valueType: "BOOLEAN" }])}
      >
        Add task
      </Button>
    </Box>
  );
}

/**
 * The reasons a checklist task offers.
 *
 * Indented under its task and rail-marked, so it reads as belonging to the row
 * above rather than as another task. Codes are not shown: the backend derives
 * them from the labels, and an author writing "Unreachable email" should not
 * have to think about UNREACHABLE_EMAIL.
 */
function ReasonEditor({
  options,
  onChange,
}: {
  options: ChecklistOption[];
  onChange: (options: ChecklistOption[]) => void;
}) {
  const setLabel = (index: number, label: string) =>
    onChange(options.map((o, i) => (i === index ? { ...o, label } : o)));

  const belowMinimum = options.length <= MIN_CHECKLIST_OPTIONS;

  return (
    <Box sx={{ ml: 2, mt: 1, pl: 2, borderLeft: "2px solid", borderColor: "divider" }}>
      <Typography variant="caption" color="text.secondary" display="block" mb={0.75}>
        Reasons to choose from — the engineer ticks one or more
      </Typography>

      <Stack spacing={0.75}>
        {options.map((option, index) => (
          <Stack key={index} direction="row" spacing={1} alignItems="center">
            {/* Lucide icons take no sx, and draw in currentColor — so the
                colour comes from a wrapper rather than from the icon. */}
            <Box sx={{ display: "flex", color: "text.disabled" }}>
              <CheckBoxIcon size={20} />
            </Box>
            <TextField
              size="small"
              fullWidth
              label={`Reason ${index + 1}`}
              value={option.label}
              onChange={(e) => setLabel(index, e.target.value)}
            />
            <Tooltip title={belowMinimum ? `A checklist needs at least ${MIN_CHECKLIST_OPTIONS} reasons` : "Remove"}>
              <span>
                <IconButton
                  size="small"
                  disabled={belowMinimum}
                  onClick={() => onChange(options.filter((_, i) => i !== index))}
                >
                  <DeleteIcon size={20} />
                </IconButton>
              </span>
            </Tooltip>
          </Stack>
        ))}
      </Stack>

      <Button size="small" startIcon={<AddIcon />} sx={{ mt: 0.5 }}
        onClick={() => onChange([...options, { code: "", label: "" }])}>
        Add reason
      </Button>
    </Box>
  );
}

/**
 * One of the two automatic tasks, shown where it will actually sit.
 *
 * Deliberately not a disabled TextField: a greyed-out input invites clicking,
 * and an author would reasonably wonder why it will not take a value. A flat
 * row with a lock reads as "this is part of the structure", which is what it is.
 */
function BookendTask({
  position,
  name,
  hint,
}: {
  position: number;
  name: string;
  hint: string;
}) {
  return (
    <Stack
      direction="row"
      spacing={1}
      alignItems="center"
      sx={{
        px: 1.5,
        py: 1,
        borderRadius: 1,
        border: "1px dashed",
        borderColor: "divider",
        bgcolor: "action.hover",
        color: "text.disabled",
        // The editable rows sit above a 40px input; matching the height keeps
        // the column of tasks reading as one list rather than two.
        minHeight: 40,
      }}
    >
      <LockIcon size={20} />
      <Typography variant="body2" fontWeight={600} sx={{ color: "text.secondary" }}>
        {position}. {name}
      </Typography>
      <Typography variant="caption" sx={{ flexGrow: 1 }}>
        {hint}
      </Typography>
      <Chip size="small" variant="outlined" label="Added automatically" />
    </Stack>
  );
}
