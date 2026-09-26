/**
 * The backend contract, mirrored in TypeScript.
 *
 * Field names and optionality follow the Go domain package exactly: a Go
 * pointer is `T | null` here, never `T | undefined`, so "absent" means the same
 * thing on both sides of the wire.
 */

/**
 * The six stages, in movement order.
 *
 * A pairing moves FORWARD along the first five, or out to ABANDONED, which is
 * terminal. Three things that look like stages are deliberately not: being at
 * risk is a health state, what a customer pays is the subscription tier, and a
 * disqualified registration is a playbook at REGISTRATION.
 */
export const LIFECYCLE_STAGES = [
  "REGISTRATION",
  "PLG_CS_ELIGIBLE",
  "FIRST_VALUE_ACHIEVED",
  "ACTIVATED",
  "COMMERCIAL",
  "ABANDONED",
] as const;
export type LifecycleStage = (typeof LIFECYCLE_STAGES)[number];

/**
 * How a pairing is doing, wherever it has got to — the second axis.
 *
 * Set by an engineer, never derived. It decides which kind of playbook the
 * pairing is offered, so changing it changes the whole product tab.
 */
export const HEALTH_STATES = ["HEALTHY", "AT_RISK"] as const;
export type HealthState = (typeof HEALTH_STATES)[number];

export const HEALTH_LABEL: Record<HealthState, string> = {
  HEALTHY: "Healthy",
  AT_RISK: "At risk",
};

/** What a playbook is for. Three kinds, in the order the editor offers them. */
export const PLAYBOOK_TYPES = ["PROGRESSIVE", "RECOVERY", "SUSTAINING"] as const;
export type PlaybookType = (typeof PLAYBOOK_TYPES)[number];

export const PLAYBOOK_TYPE_LABEL: Record<PlaybookType, string> = {
  PROGRESSIVE: "Progressive Playbook",
  RECOVERY: "Recovery Playbook",
  SUSTAINING: "Sustaining Playbook",
};

export const PLAYBOOK_TYPE_HELP: Record<PlaybookType, string> = {
  PROGRESSIVE: "Carries a pairing forward, to the next stage.",
  RECOVERY: "Brings an at-risk pairing back to healthy, where it is.",
  SUSTAINING: "Keeps a healthy pairing steady. It is not a way out of the stage.",
};

/**
 * Which kinds a pairing is offered, which follows from its health and nothing
 * else. Mirrors ApplicablePlaybookTypes in the Go domain and the
 * plg_applicable_playbook_types SQL function.
 *
 * Used only to explain the menu to the reader — the menu itself is the
 * applicablePlaybookTypes the server sends, so a disagreement shows up as
 * wording that does not match the list rather than as a wrong list.
 */
export function applicablePlaybookTypes(h: HealthState): PlaybookType[] {
  return h === "AT_RISK" ? ["RECOVERY"] : ["PROGRESSIVE", "SUSTAINING"];
}

/**
 * The four tiers, in commercial order: arrive, try, try a bit longer, pay.
 *
 * ENTERPRISE is gone. It described how a contract was signed rather than what
 * the customer pays, and nothing branched on it. TRIAL_EXTENDED took its place
 * because that IS a state the portal acts on — it has its own end date, and a
 * customer sitting in it has had their trial rescued once already.
 */
export const SUBSCRIPTION_TIERS = ["FREE", "TRIAL", "TRIAL_EXTENDED", "PAYG"] as const;
export type SubscriptionTier = (typeof SUBSCRIPTION_TIERS)[number];

/** Tiers that run out, and therefore have a date to record. */
export const DATED_TIERS: readonly SubscriptionTier[] = ["TRIAL", "TRIAL_EXTENDED"];

/** The field holding a tier's end date. Undefined for the tiers that never end. */
export function tierDateField(
  tier: SubscriptionTier | null,
): "trialEndDate" | "trialExtendedDate" | undefined {
  if (tier === "TRIAL") return "trialEndDate";
  if (tier === "TRIAL_EXTENDED") return "trialExtendedDate";
  return undefined;
}

/**
 * What a task holds.
 *
 * CHECKLIST and SINGLE_SELECT are siblings: both offer a fixed list of authored
 * answers, and they differ only in how many may be chosen. Both require
 * `options`, and both report through the same view.
 */
export const TASK_VALUE_TYPES = [
  "BOOLEAN",
  "STRING",
  "NUMBER",
  "SINGLE_SELECT",
  "CHECKLIST",
] as const;
export type TaskValueType = (typeof TASK_VALUE_TYPES)[number];

export const TASK_VALUE_TYPE_LABEL: Record<TaskValueType, string> = {
  BOOLEAN: "Tick box",
  STRING: "Text",
  NUMBER: "Number",
  SINGLE_SELECT: "Choose one",
  CHECKLIST: "Choose any",
};

/** The two types that carry an authored answer list; nothing else may. */
export function typeNeedsOptions(t: TaskValueType): boolean {
  return t === "CHECKLIST" || t === "SINGLE_SELECT";
}

export type RunStatus = "NOT_STARTED" | "ACTIVE" | "CLOSED";

/** The two reserved tasks every playbook opens and closes with. */
export const TASK_CODE_INITIATE = "INITIATE_PLAYBOOK";
export const TASK_CODE_CLOSE = "CLOSE_PLAYBOOK";

/** The fewest reasons a checklist may offer — mirrors domain.MinChecklistOptions. */
export const MIN_CHECKLIST_OPTIONS = 2;

/**
 * One reason a CHECKLIST task offers.
 *
 * Code and label are separate because labels get reworded, and analysis groups
 * on the code. The editor may leave `code` out — the backend derives it from
 * the label.
 */
export interface ChecklistOption {
  code: string;
  label: string;
}

export interface UserRef {
  /** `"user".id` — the identifier. */
  id: string;
  /**
   * Carried for display only. Nothing in the UI should compare or send it as
   * an identity — that is what `id` is for.
   */
  email: string;
  name: string;
}

export interface ProductRef {
  id: string;
  code: string;
  name: string;
}

export interface Product extends ProductRef {
  displayOrder: number;
  active: boolean;
}

// ---------------------------------------------------------------------------
// Lifecycle catalogue
// ---------------------------------------------------------------------------

export interface LifecycleStageInfo {
  stage: LifecycleStage;
  name: string;
  displayOrder: number;
  description: string | null;
  /**
   * Where a progressive playbook here carries a pairing. Null at COMMERCIAL,
   * which has nowhere further to go, and at ABANDONED. A playbook names no
   * destination of its own: there is no choice of one to offer.
   */
  nextStage: LifecycleStage | null;
  /** ABANDONED: reachable from every stage, with no way out. */
  terminal: boolean;
  /** How many playbooks exist here, by kind, across all products. */
  progressivePlaybooks: number;
  recoveryPlaybooks: number;
  sustainingPlaybooks: number;
}

/** The stage list. There are no paths any more — the order IS the progression. */
export interface LifecycleCatalogue {
  stages: LifecycleStageInfo[];
}

// ---------------------------------------------------------------------------
// Organisations
// ---------------------------------------------------------------------------

export interface PlatformStage {
  orgPlatformId: string;
  product: ProductRef;
  lifecycleStage: LifecycleStage;
  stageName: string;
  registeredOn: string;
  isNew: boolean;
}

export interface OrganizationSummary {
  id: string;
  organizationName: string;
  registeredEmail: string;
  registeredName: string | null;
  registeredOn: string;
  platformStages: PlatformStage[];
  owner: UserRef | null;
}

export interface OrganizationSearchFilters {
  query?: string;
  organizationName?: string;
  registeredEmail?: string;
  productCodes?: string[];
  lifecycleStages?: LifecycleStage[];
  subscriptionTiers?: SubscriptionTier[];
  ownerIds?: string[];
  registeredFrom?: string | null;
  registeredTo?: string | null;
  unowned?: boolean;
}

export interface Pagination {
  limit: number;
  offset: number;
}

export interface SearchOrganizationsRequest {
  filters?: OrganizationSearchFilters;
  pagination?: Pagination;
  sortBy?: "registered" | "name";
  sortOrder?: "asc" | "desc";
}

export interface SearchOrganizationsResponse {
  organizations: OrganizationSummary[];
  total: number;
  limit: number;
  offset: number;
}

export interface PlatformCard {
  orgPlatformId: string;
  product: ProductRef;
  lifecycleStage: LifecycleStage;
  stageName: string;
  stageEnteredOn: string;
  registeredOn: string;
  subscriptionTier: SubscriptionTier | null;
  runActive: number;
  runTotal: number;
  taskCompleted: number;
  taskTotal: number;
  isNew: boolean;
}

export interface OrganizationDetail {
  id: string;
  organizationName: string;
  createdOn: string;
  registeredEmail: string;
  registeredName: string | null;
  owner: UserRef | null;
  countryName: string | null;
  companyNameFromDomain: string | null;
  /** The source's stable id for the company. */
  moesifCompanyId: string | null;
  /** True when the Lead fields were resolved from the registrant's other organisations. */
  fieldsInherited: boolean;
  platformCards: PlatformCard[];
}

// ---------------------------------------------------------------------------
// Playbook templates
// ---------------------------------------------------------------------------

export interface PlaybookTask {
  id: string;
  code: string;
  name: string;
  description: string | null;
  sequenceNo: number;
  valueType: TaskValueType;
  /** The reasons offered. Empty for every type but CHECKLIST. */
  options: ChecklistOption[] | null;
  active: boolean;
  isBookend: boolean;
}

export interface Playbook {
  id: string;
  product: ProductRef;
  name: string;
  description: string | null;
  lifecycleStage: LifecycleStage;
  stageName: string;
  playbookType: PlaybookType;
  displayOrder: number;
  active: boolean;
  tasks: PlaybookTask[];
  runCount: number;
  activeRuns: number;
  createdOn: string;
  updatedOn: string;
}

export interface PlaybookTaskInput {
  code?: string;
  name: string;
  description?: string | null;
  valueType?: TaskValueType;
  /** Required for CHECKLIST, and must be absent otherwise. */
  options?: ChecklistOption[];
}

export interface CreatePlaybookRequest {
  name: string;
  description?: string | null;
  lifecycleStage: LifecycleStage;
  playbookType: PlaybookType;
  tasks: PlaybookTaskInput[];
}

export interface PatchPlaybookRequest {
  name?: string;
  description?: string | null;
  lifecycleStage?: LifecycleStage;
  playbookType?: PlaybookType;
  active?: boolean;
}

// ---------------------------------------------------------------------------
// Playbook execution
// ---------------------------------------------------------------------------

export interface RunTask {
  id: string;
  playbookRunId: string;
  playbookTaskId: string;
  code: string;
  name: string;
  description: string | null;
  sequenceNo: number;
  valueType: TaskValueType;
  boolValue: boolean | null;
  numberValue: number | null;
  textValue: string | null;
  /** Copied from the template with the rest of the task. */
  options: ChecklistOption[] | null;
  /** The reasons ticked so far. */
  checkedCodes: string[] | null;
  isCompleted: boolean;
  completedOn: string | null;
  completedBy: UserRef | null;
  isBookend: boolean;
}

export interface PlaybookRun {
  id: string;
  orgPlatformId: string;
  playbookId: string;
  playbookName: string;
  description: string | null;
  playbookStage: LifecycleStage;
  playbookType: PlaybookType;
  runStatus: RunStatus;
  taskTotal: number;
  taskCompleted: number;
  nextTaskName: string | null;
  addedBy: UserRef | null;
  createdOn: string;
  tasks: RunTask[];
}

export interface RunProgress {
  runTotal: number;
  runActive: number;
  runClosed: number;
  taskTotal: number;
  taskCompleted: number;
}

export interface Note {
  id: string;
  body: string;
  author: UserRef | null;
  createdOn: string;
  /** Null until the note is first edited — nothing stores an "edited" flag. */
  updatedOn: string | null;
  updatedBy: UserRef | null;
}

/**
 * One recorded change, of whichever axis moved.
 *
 * Every "to" half is nullable: a row carries a stage move, a health move, a
 * subscription change, or several at once. Which halves are populated is how an
 * entry says what kind of event it was.
 */
export interface LifecycleEntry {
  id: string;
  fromStage: LifecycleStage | null;
  toStage: LifecycleStage | null;
  toName: string | null;
  fromHealth: HealthState | null;
  toHealth: HealthState | null;
  fromSubscription: SubscriptionTier | null;
  toSubscription: SubscriptionTier | null;
  reason: string | null;
  changedBy: UserRef | null;
  changedOn: string;
}

export interface ProductDetail {
  orgPlatformId: string;
  organizationId: string;
  organizationName: string;
  product: ProductRef;

  lifecycleStage: LifecycleStage;
  stageName: string;
  stageEnteredOn: string;

  /** The second axis, with who last set it and when. */
  healthState: HealthState;
  healthEnteredOn: string;
  healthUpdatedBy: UserRef | null;
  /** Which kinds of playbook this pairing is offered — follows from health. */
  applicablePlaybookTypes: PlaybookType[];
  /** Whether a playbook of that kind actually exists here to attach. */
  carriesPlaybooks: boolean;

  registeredOn: string;
  registeredEmail: string;

  /** The third axis, with the provenance the other two carry. */
  subscriptionTier: SubscriptionTier | null;
  subscriptionEnteredOn: string | null;
  subscriptionUpdatedBy: UserRef | null;
  trialEndDate: string | null;
  trialExtendedDate: string | null;
  /**
   * Whichever of the two dates the CURRENT tier makes live — resolved by the
   * database so the rule has one home. Null on FREE and PAYG, which do not end.
   */
  currentPeriodEndDate: string | null;

  acknowledgedOn: string | null;
  acknowledgedBy: UserRef | null;
  isNew: boolean;

  runs: PlaybookRun[];
  progress: RunProgress;
  availablePlaybooks: Playbook[];

  notes: Note[];
  lifecycleHistory: LifecycleEntry[];
}

export interface PatchOrgPlatformRequest {
  lifecycleStage?: LifecycleStage;
  /** The second axis. Independent of the stage — send either, both or neither. */
  healthState?: HealthState;
  /**
   * Why the axis moved. Required whenever lifecycleStage, healthState or
   * subscriptionTier is sent — the server rejects the request without it, and
   * the history column underneath is NOT NULL.
   */
  reason?: string | null;
  subscriptionTier?: SubscriptionTier;
  trialEndDate?: string | null;
  clearTrialEndDate?: boolean;
  trialExtendedDate?: string | null;
  clearTrialExtendedDate?: boolean;
}

export interface PatchRunTaskRequest {
  boolValue?: boolean | null;
  numberValue?: number | null;
  textValue?: string | null;
  /**
   * The complete set of ticked reasons, not a delta — so the call is idempotent
   * and two engineers ticking at once cannot interleave. An empty array clears
   * the task.
   */
  checkedCodes?: string[];
  clearValue?: boolean;
}

// ---------------------------------------------------------------------------
// Registrations
// ---------------------------------------------------------------------------

export interface RegistrationItem {
  orgPlatformId: string;
  organizationId: string;
  organizationName: string;
  product: ProductRef;
  registeredEmail: string;
  registeredName: string | null;
  registeredOn: string;
  lifecycleStage: LifecycleStage;
  stageName: string;
  owner: UserRef | null;
}

export interface SearchRegistrationsRequest {
  unacknowledgedOnly?: boolean;
  filters?: OrganizationSearchFilters;
  pagination?: Pagination;
}

export interface SearchRegistrationsResponse {
  registrations: RegistrationItem[];
  total: number;
  limit: number;
  offset: number;
}

// ---------------------------------------------------------------------------
// Work queue
// ---------------------------------------------------------------------------

/** Why a pairing is in the queue. Derived, never stored. */
export const QUEUE_REASONS = ["NO_PLAYBOOK", "NOT_STARTED", "IN_PROGRESS"] as const;
export type QueueReason = (typeof QUEUE_REASONS)[number];

export const QUEUE_REASON_LABEL: Record<QueueReason, string> = {
  NO_PLAYBOOK: "No playbook added",
  NOT_STARTED: "Not started",
  IN_PROGRESS: "In progress",
};

/**
 * One pairing needing attention.
 *
 * The unit is the pairing, not the playbook run: it enters the queue when
 * acknowledged and leaves when it reaches a stage carrying no playbooks.
 */
export interface WorkQueueItem {
  orgPlatformId: string;
  organizationId: string;
  organizationName: string;
  product: ProductRef;
  currentStage: LifecycleStage;
  stageName: string;
  reason: QueueReason;

  runActive: number;
  runTotal: number;
  taskCompleted: number;
  taskTotal: number;
  /** How many playbooks could still be added at this stage. */
  availablePlaybooks: number;

  playbookId: string | null;
  playbookName: string | null;
  nextTaskCode: string | null;
  nextTaskName: string | null;

  owner: UserRef | null;
  /** Who triaged it — not always the owner. */
  acknowledgedBy: UserRef | null;
  acknowledgedOn: string | null;
  registeredOn: string;
}

/**
 * One tile: pairings at one stage for one product.
 *
 * Product and stage partition the queue, so the tiles sum to the list total.
 */
export interface StageTile {
  lifecycleStage: LifecycleStage;
  stageName: string;
  pairings: number;
  /** How many of those are at risk. A tile says what kind of work it holds. */
  atRisk: number;
  noPlaybook: number;
  notStarted: number;
  inProgress: number;
}

export interface ProductQueueGroup {
  product: ProductRef;
  pairings: number;
  tiles: StageTile[];
}

export interface WorkQueueResponse {
  productGroups: ProductQueueGroup[];
  items: WorkQueueItem[];
  total: number;
  byReason: Record<QueueReason, number>;
}

export interface WorkQueueQuery {
  mine?: boolean;
  owner?: string;
  organizationId?: string[];
  product?: string[];
  stage?: string[];
  reason?: string[];
  playbookId?: string[];
  taskCode?: string[];
}

// ---------------------------------------------------------------------------
// Analytics
// ---------------------------------------------------------------------------

export interface CountByLabel {
  label: string;
  count: number;
}

export interface TimeBucketSeries {
  label: string;
  counts: number[];
}

export interface TimeSeries {
  buckets: string[];
  series: TimeBucketSeries[];
}

export interface DashboardSummary {
  totalOrganizations: number;
  totalRegistrations: number;
  newRegistrations: number;
  pairingsNeedingAttention: number;
  trialsEndingSoon: number;
}

export interface DashboardAnalytics {
  summary: DashboardSummary;
  registrationsByProduct: CountByLabel[];
  registrationsByLifecycleStage: CountByLabel[];
  subscriptionMix: CountByLabel[];
  registrationsOverTime: TimeSeries;
}
