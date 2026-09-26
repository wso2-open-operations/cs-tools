// Package domain holds every request/response type and enum the PLG CS portal
// exposes.
//
// Conventions:
//   - JSON field names are camelCase
//   - timestamp fields use the "On" suffix (registeredOn, createdOn)
//   - business dates keep the "Date" suffix (trialEndDate)
//   - optional response fields are pointers, so absent means JSON null, never ""
//   - enum values are UPPER_SNAKE_CASE strings matching the PostgreSQL enums
package domain

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

// ---------------------------------------------------------------------------
// Enums
// ---------------------------------------------------------------------------

// LifecycleStage is how far one organisation+platform pairing has got.
//
// The stage belongs to the pairing, not the organisation: the same customer can
// be Commercial on API Platform and still at Registration on Agent Platform.
//
// A pairing moves FORWARD along the first five, or out to ABANDONED, which is
// terminal. Nothing moves backwards — enforced by plg_check_stage_move() in the
// schema, and pre-checked in the BFF so the message is readable.
type LifecycleStage string

// The six stages, in movement order.
//
// Three things that look like stages are deliberately not: being at risk is a
// health state (a pairing can be at risk anywhere, and being so should not cost
// it its place), what a customer pays is the subscription tier, and a
// disqualified registration is an outcome — a playbook at REGISTRATION that
// ends the pairing at ABANDONED, and unlike a stage it can
// record why.
const (
	StageRegistration       LifecycleStage = "REGISTRATION"
	StagePlgCsEligible      LifecycleStage = "PLG_CS_ELIGIBLE"
	StageFirstValueAchieved LifecycleStage = "FIRST_VALUE_ACHIEVED"
	StageActivated          LifecycleStage = "ACTIVATED"
	StageCommercial         LifecycleStage = "COMMERCIAL"
	StageAbandoned          LifecycleStage = "ABANDONED"
)

// LifecycleStageOrder is the progression, and the order the diagram draws.
//
// ABANDONED is last but is not the step after COMMERCIAL — it hangs off every
// stage. Anything comparing positions to decide whether a move is forward must
// treat it as the exception it is; StageProgression below excludes it for
// exactly that reason.
var LifecycleStageOrder = []LifecycleStage{
	StageRegistration, StagePlgCsEligible, StageFirstValueAchieved,
	StageActivated, StageCommercial, StageAbandoned,
}

// StageProgression is the five stages a pairing advances through, in order.
//
// Separate from LifecycleStageOrder because ABANDONED is reachable from all of
// them and follows none of them. A caller asking "what comes next" wants this.
var StageProgression = []LifecycleStage{
	StageRegistration, StagePlgCsEligible, StageFirstValueAchieved,
	StageActivated, StageCommercial,
}

// NextStage returns the stage a progressive playbook at s carries a pairing to,
// and false when there is none — at COMMERCIAL, and at ABANDONED.
//
// A progressive playbook does not declare a destination: the destination is a
// property of the stage it sits at, so it is derived rather than authored — and
// therefore cannot be authored wrongly.
func NextStage(s LifecycleStage) (LifecycleStage, bool) {
	for i, stage := range StageProgression {
		if stage == s && i+1 < len(StageProgression) {
			return StageProgression[i+1], true
		}
	}
	return "", false
}

// CanMoveStage reports whether a pairing may move from one stage to another.
//
// Mirrors plg_check_stage_move() exactly. Duplicated deliberately: the trigger
// is the invariant and this is the courtesy, so the caller gets a sentence
// rather than a constraint violation. If the two ever disagree the trigger wins,
// which is the right way round.
func CanMoveStage(from, to LifecycleStage) bool {
	if from == StageAbandoned {
		return false // terminal
	}
	if to == StageAbandoned {
		return true // reachable from anywhere
	}
	fromIdx, toIdx := -1, -1
	for i, s := range StageProgression {
		if s == from {
			fromIdx = i
		}
		if s == to {
			toIdx = i
		}
	}
	return fromIdx >= 0 && toIdx > fromIdx
}

// HealthState is how a pairing is doing, wherever it has got to.
//
// The second axis, and the reason the stage list could shrink from nine to six.
// Set by an engineer, never derived; defaults to HEALTHY. There is no third
// "unknown" value, because an engineer who has not looked is not the same as a
// customer in trouble and the work queue should not treat them alike.
type HealthState string

// The two health states.
const (
	HealthHealthy HealthState = "HEALTHY"
	HealthAtRisk  HealthState = "AT_RISK"
)

// HealthStateOrder is the display order for filters and tiles.
var HealthStateOrder = []HealthState{HealthHealthy, HealthAtRisk}

// ValidHealthState is the allow-list checked in the service layer.
var ValidHealthState = map[HealthState]bool{HealthHealthy: true, HealthAtRisk: true}

// PlaybookType is what a playbook is trying to do.
type PlaybookType string

// The three kinds of playbook.
const (
	// PlaybookProgressive carries a pairing forward, to the next stage.
	PlaybookProgressive PlaybookType = "PROGRESSIVE"
	// PlaybookRecovery carries a pairing from AT_RISK back to HEALTHY, without
	// moving it.
	PlaybookRecovery PlaybookType = "RECOVERY"
	// PlaybookSustaining keeps a healthy pairing healthy where it is. It is not
	// a route out of the stage, which is exactly what distinguishes it from
	// PlaybookProgressive — the quarterly review is real work that is not
	// trying to promote anyone.
	PlaybookSustaining PlaybookType = "SUSTAINING"
)

// PlaybookTypeOrder is the order the playbook editor offers the kinds in.
var PlaybookTypeOrder = []PlaybookType{
	PlaybookProgressive, PlaybookRecovery, PlaybookSustaining,
}

// PlaybookTypeLabel is the wording the UI shows. Held here rather than in the
// webapp so the API and the screen cannot drift apart.
var PlaybookTypeLabel = map[PlaybookType]string{
	PlaybookProgressive: "Progressive Playbook",
	PlaybookRecovery:    "Recovery Playbook",
	PlaybookSustaining:  "Sustaining Playbook",
}

// ValidPlaybookType is the allow-list checked in the service layer.
var ValidPlaybookType = map[PlaybookType]bool{
	PlaybookProgressive: true, PlaybookRecovery: true, PlaybookSustaining: true,
}

// ApplicablePlaybookTypes is the mapping that decides what a pairing is offered.
//
// The same rule the plg_applicable_playbook_types SQL function holds, written
// twice on purpose — the function so SQL callers need not know it, and here so
// Go callers need not query. Keep the two in step; they are checked against
// each other by the pairing view, which serves this list from SQL.
//
// It returns a set rather than a value because a healthy pairing has two kinds
// of work available at once: the play that moves it on and the play that keeps
// it steady. Only AT_RISK narrows to a single kind, because when an account is
// in trouble neither advancing it nor maintaining it is the job in front of the
// engineer.
func ApplicablePlaybookTypes(h HealthState) []PlaybookType {
	if h == HealthAtRisk {
		return []PlaybookType{PlaybookRecovery}
	}
	return []PlaybookType{PlaybookProgressive, PlaybookSustaining}
}

// ValidLifecycleStage is the allow-list checked in the service layer.
var ValidLifecycleStage = func() map[LifecycleStage]bool {
	m := make(map[LifecycleStage]bool, len(LifecycleStageOrder))
	for _, s := range LifecycleStageOrder {
		m[s] = true
	}
	return m
}()

// TaskValueType is what one playbook task holds.
//
// Tasks are not checkboxes, because most of them never were: a research task
// wants a paragraph, a fit score wants a number, and only a bookend genuinely
// wants a tick.
type TaskValueType string

// Task value types.
const (
	// ValueBoolean is a tick. The only type the two bookends may use.
	ValueBoolean TaskValueType = "BOOLEAN"
	// ValueString is free text — for answers that are descriptions.
	ValueString TaskValueType = "STRING"
	ValueNumber TaskValueType = "NUMBER"
	// ValueChecklist is a set of predefined reasons, one or more of which is
	// ticked. It exists for the answers that are neither yes/no nor prose but a
	// choice from a known list — "why was this domain disqualified?".
	//
	// Completion is *any* reason ticked, not all of them: recording one reason is
	// recording why.
	ValueChecklist TaskValueType = "CHECKLIST"
	// ValueSingleSelect is a fixed list of answers of which exactly one is
	// chosen. CHECKLIST's sibling: same authored options, same reporting through
	// plg_run_task_reason_v, different cardinality.
	//
	// The chosen option's CODE is stored in value_text. It is a single scalar
	// answer, which is what value_text is; what distinguishes it from STRING is
	// not the storage but the options beside it, which constrain what may be
	// written there.
	ValueSingleSelect TaskValueType = "SINGLE_SELECT"
)

// TaskValueTypeOrder is the order the task editor offers the types in.
var TaskValueTypeOrder = []TaskValueType{
	ValueBoolean, ValueString, ValueNumber, ValueSingleSelect, ValueChecklist,
}

// TypeNeedsOptions reports whether a task type requires an authored answer list.
// The two list types do; nothing else may carry one.
func TypeNeedsOptions(t TaskValueType) bool {
	return t == ValueChecklist || t == ValueSingleSelect
}

// ValidTaskValueType is the allow-list checked in the service layer.
var ValidTaskValueType = map[TaskValueType]bool{
	ValueBoolean: true, ValueString: true, ValueNumber: true,
	ValueChecklist: true, ValueSingleSelect: true,
}

// SubscriptionTier is recorded by hand — there is no subscription feed.
//
// It overlaps deliberately with LifecycleStage: PAYG names the same commercial
// fact the COMMERCIAL stage does. What this adds beyond the stage is how the
// customer is paying today — FREE, mid-TRIAL, or on an extension — which the
// lifecycle cannot express.
type SubscriptionTier string

// Subscription tiers.
const (
	TierFree  SubscriptionTier = "FREE"
	TierTrial SubscriptionTier = "TRIAL"
	// TierTrialExtended is a trial that has been granted more time. It is its
	// own tier rather than a flag on TRIAL because it carries its own end date
	// and because "we already extended this one" is the single most useful
	// thing to know about a conversation that is running long.
	TierTrialExtended SubscriptionTier = "TRIAL_EXTENDED"
	TierPayg          SubscriptionTier = "PAYG"
)

// SubscriptionTierOrder is the display order for filters and the donut. It is
// the commercial progression: arrive, try, try a bit longer, pay.
var SubscriptionTierOrder = []SubscriptionTier{
	TierFree, TierTrial, TierTrialExtended, TierPayg,
}

// ValidSubscriptionTier is the allow-list checked in the service layer.
var ValidSubscriptionTier = map[SubscriptionTier]bool{
	TierFree: true, TierTrial: true, TierTrialExtended: true, TierPayg: true,
}

// DatedTier reports whether a tier has an end date to record. Only TRIAL and
// TRIAL_EXTENDED do; FREE and PAYG do not run out.
func DatedTier(t SubscriptionTier) bool {
	return t == TierTrial || t == TierTrialExtended
}

// RunStatus is the state of one playbook run, derived from its bookend tasks
// rather than stored.
type RunStatus string

// Run status values.
const (
	RunNotStarted RunStatus = "NOT_STARTED"
	RunActive     RunStatus = "ACTIVE"
	RunClosed     RunStatus = "CLOSED"
)

// Reserved bookend task codes. Every playbook opens with TaskCodeInitiate and
// ends with TaskCodeClose; the service layer injects them and refuses to let an
// editor remove, reorder or retype them. Completing the first makes a run
// active, completing the last closes it — which is what the work queue counts.
const (
	TaskCodeInitiate = "INITIATE_PLAYBOOK"
	TaskCodeClose    = "CLOSE_PLAYBOOK"
)

// IsBookendTask reports whether a task code is one of the two reserved bookends.
func IsBookendTask(code string) bool {
	return code == TaskCodeInitiate || code == TaskCodeClose
}

// ---------------------------------------------------------------------------
// Shared
// ---------------------------------------------------------------------------

// Pagination is entity-service's own — see entity.go. PLG declared an
// identical copy while it was a standalone service; the merge deletes it
// rather than renaming it, because there was never a second concept here.

// UserRef is a slim reference to a PLG CS engineer.
//
// ID is the identifier — `"user".id`. Email and Name are carried for display
// only and are never matched on. Keying on an email reads well right up until
// it is copied into every table that references a person.
//
// Email is still populated because a name alone is ambiguous on screen and the
// owner pickers show both. The distinction that matters is that nothing keys,
// joins or filters on it.
type UserRef struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// PlgProductRef is a slim reference to a supported WSO2 Cloud platform.
type PlgProductRef struct {
	ID   string `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

// PlgProduct is a supported platform.
// PlgProduct is the PLATFORM catalogue — the five things a pairing can be on:
// IAM, API Platform, Integration Platform, Agent Platform, Engineering
// Platform. It carries the plg prefix because entity-service's own `Product` is
// a different thing entirely (a ServiceNow product entity with a class), and
// two unrelated concepts sharing one word in one package is how a reader ends
// up holding the wrong one.
type PlgProduct struct {
	ID           string `json:"id"`
	Code         string `json:"code"`
	Name         string `json:"name"`
	DisplayOrder int    `json:"displayOrder"`
	Active       bool   `json:"active"`
}

// ---------------------------------------------------------------------------
// Lifecycle catalogue
// ---------------------------------------------------------------------------

// LifecycleStageInfo is one stage of the catalogue.
//
// StagePath is gone with the table it described. A stage no longer has a set of
// destinations to choose between: a progressive playbook goes to the next stage
// and a risk-intervention one goes nowhere, so NextStage below is a fact about
// the progression rather than a choice an author makes.
type LifecycleStageInfo struct {
	Stage        LifecycleStage `json:"stage"`
	Name         string         `json:"name"`
	DisplayOrder int            `json:"displayOrder"`
	Description  *string        `json:"description"`

	// NextStage is where a progressive playbook here carries a pairing. Nil at
	// COMMERCIAL, which has nowhere further to go, and at ABANDONED, which is
	// terminal.
	NextStage *LifecycleStage `json:"nextStage"`

	// Terminal marks ABANDONED: reachable from every stage, and with no way out.
	Terminal bool `json:"terminal"`

	// PlaybookCounts is how many playbooks exist here, by kind, across all
	// products. A stage with none authored is an author's problem, and this is
	// what lets the editor say so.
	ProgressivePlaybooks int `json:"progressivePlaybooks"`
	RecoveryPlaybooks    int `json:"recoveryPlaybooks"`
	SustainingPlaybooks  int `json:"sustainingPlaybooks"`
}

// LifecycleCatalogue is the stage list, served by GET /lifecycle.
//
// Just the stages. There is no separate path table to serve alongside them: the
// progression IS the stage order, with ABANDONED as the one exception, so each
// stage carries everything a caller would otherwise have joined for.
type LifecycleCatalogue struct {
	Stages []LifecycleStageInfo `json:"stages"`
}

// ---------------------------------------------------------------------------
// Organisation
// ---------------------------------------------------------------------------

// OrganizationSearchFilters is every filter the organisation list supports.
//
// PlgProduct, stage and tier match an organisation when *any* of its pairings
// match — the only coherent reading once the stage belongs to the pairing.
type OrganizationSearchFilters struct {
	Query             string             `json:"query"`
	OrganizationName  string             `json:"organizationName"`
	RegisteredEmail   string             `json:"registeredEmail"`
	ProductCodes      []string           `json:"productCodes"`
	LifecycleStages   []LifecycleStage   `json:"lifecycleStages"`
	SubscriptionTiers []SubscriptionTier `json:"subscriptionTiers"`
	OwnerIDs          []string           `json:"ownerIds"`
	RegisteredFrom    *time.Time         `json:"registeredFrom"`
	RegisteredTo      *time.Time         `json:"registeredTo"`
	Unowned           *bool              `json:"unowned"`
}

// SearchOrganizationsRequest is the body of POST /organizations/search.
type SearchOrganizationsRequest struct {
	Filters    OrganizationSearchFilters `json:"filters"`
	Pagination Pagination                `json:"pagination"`
	SortBy     string                    `json:"sortBy"`
	SortOrder  string                    `json:"sortOrder"`
}

// PlatformStage is one registered platform with its own lifecycle stage — the
// column that could not exist while the stage lived on the organisation.
type PlatformStage struct {
	OrgPlatformID  string         `json:"orgPlatformId"`
	Product        PlgProductRef  `json:"product"`
	LifecycleStage LifecycleStage `json:"lifecycleStage"`
	StageName      string         `json:"stageName"`
	RegisteredOn   time.Time      `json:"registeredOn"`
	IsNew          bool           `json:"isNew"`
}

// OrganizationSummary is one row of the organisation list — the five columns the
// requirement names, plus what the row needs to be clickable.
type OrganizationSummary struct {
	ID               string          `json:"id"`
	OrganizationName string          `json:"organizationName"`
	RegisteredEmail  string          `json:"registeredEmail"`
	RegisteredName   *string         `json:"registeredName"`
	RegisteredOn     time.Time       `json:"registeredOn"`
	PlatformStages   []PlatformStage `json:"platformStages"`
	Owner            *UserRef        `json:"owner"`
}

// SearchOrganizationsResponse is the paged organisation list.
type SearchOrganizationsResponse struct {
	Organizations []OrganizationSummary `json:"organizations"`
	Total         int                   `json:"total"`
	Limit         int                   `json:"limit"`
	Offset        int                   `json:"offset"`
}

// PlatformCard is one registered platform on the overview's "Registered
// products" card, and one tab in the strip above it.
type PlatformCard struct {
	OrgPlatformID    string            `json:"orgPlatformId"`
	Product          PlgProductRef     `json:"product"`
	LifecycleStage   LifecycleStage    `json:"lifecycleStage"`
	StageName        string            `json:"stageName"`
	StageEnteredOn   time.Time         `json:"stageEnteredOn"`
	RegisteredOn     time.Time         `json:"registeredOn"`
	SubscriptionTier *SubscriptionTier `json:"subscriptionTier"`
	RunActive        int               `json:"runActive"`
	RunTotal         int               `json:"runTotal"`
	TaskCompleted    int               `json:"taskCompleted"`
	TaskTotal        int               `json:"taskTotal"`
	IsNew            bool              `json:"isNew"`
}

// OrganizationDetail is the overview tab: two cards, which is all it keeps.
//
// The Lead fields here are read through plg_organization_v, so a sparse second
// registration shows its registrant's values rather than blanks. FieldsInherited
// says when that happened, so the UI can mark them.
type OrganizationDetail struct {
	ID                    string    `json:"id"`
	OrganizationName      string    `json:"organizationName"`
	CreatedOn             time.Time `json:"createdOn"`
	RegisteredEmail       string    `json:"registeredEmail"`
	RegisteredName        *string   `json:"registeredName"`
	Owner                 *UserRef  `json:"owner"`
	CountryName           *string   `json:"countryName"`
	CompanyNameFromDomain *string   `json:"companyNameFromDomain"`
	// MoesifCompanyID is the source's stable id for the company. Kept for
	// troubleshooting and for whatever later wants to join on it.
	MoesifCompanyID *string        `json:"moesifCompanyId"`
	FieldsInherited bool           `json:"fieldsInherited"`
	PlatformCards   []PlatformCard `json:"platformCards"`
}

// PatchOrganizationRequest is the body of PATCH /organizations/{id}.
//
// Lifecycle stage is deliberately absent: it belongs to the pairing. Ownership
// is organisation-level — one CS owner handles every pairing for a customer.
type PatchOrganizationRequest struct {
	ID      string  `json:"-"`
	OwnerID *string `json:"ownerId"`
}

// ---------------------------------------------------------------------------
// Playbook templates
// ---------------------------------------------------------------------------

// MinChecklistOptions is the fewest reasons a checklist may offer.
//
// One reason is a tick box wearing a costume — there would be nothing to choose
// between — and none could never be completed at all.
const MinChecklistOptions = 2

// ChecklistOption is one reason a CHECKLIST task offers.
//
// Code and Label are separate because labels get reworded: "not a business
// domain" becoming "personal email domain" must not orphan the history that
// counted it. Analysis groups on Code; the UI shows Label.
type ChecklistOption struct {
	Code  string `json:"code"`
	Label string `json:"label"`
}

// PlaybookTask is one task in a playbook template.
type PlaybookTask struct {
	ID          string        `json:"id"`
	Code        string        `json:"code"`
	Name        string        `json:"name"`
	Description *string       `json:"description"`
	SequenceNo  int           `json:"sequenceNo"`
	ValueType   TaskValueType `json:"valueType"`
	// Options are the reasons a CHECKLIST task offers, and are empty for every
	// other type.
	Options []ChecklistOption `json:"options"`
	Active  bool              `json:"active"`
	// IsBookend marks the reserved Initiate/Close tasks: not removable, not
	// reorderable, always BOOLEAN.
	IsBookend bool `json:"isBookend"`
}

// Playbook is a template scoped to a product, a lifecycle stage AND a kind.
//
// A PROGRESSIVE playbook
// carries a pairing to the next stage; a RECOVERY one restores its health
// without moving it; a SUSTAINING one keeps a healthy pairing steady where it
// is. Which kinds a pairing is offered depends on its health, so the three
// together — product, stage, type — decide exactly which pairings ever see
// this playbook.
type Playbook struct {
	ID             string         `json:"id"`
	Product        PlgProductRef  `json:"product"`
	Name           string         `json:"name"`
	Description    *string        `json:"description"`
	LifecycleStage LifecycleStage `json:"lifecycleStage"`
	StageName      string         `json:"stageName"`
	PlaybookType   PlaybookType   `json:"playbookType"`
	DisplayOrder   int            `json:"displayOrder"`
	Active         bool           `json:"active"`
	Tasks          []PlaybookTask `json:"tasks"`
	RunCount       int            `json:"runCount"`
	ActiveRuns     int            `json:"activeRuns"`
	CreatedOn      time.Time      `json:"createdOn"`
	UpdatedOn      time.Time      `json:"updatedOn"`
}

// PlaybookTaskInput is one desired task in a submitted task list.
type PlaybookTaskInput struct {
	Code        string        `json:"code"`
	Name        string        `json:"name"`
	Description *string       `json:"description"`
	ValueType   TaskValueType `json:"valueType"`
	// Options are required for CHECKLIST and must be absent otherwise. A code
	// left blank is derived from the label, the same way a task's is.
	Options []ChecklistOption `json:"options"`
}

// CreatePlaybookRequest is the body of POST /products/{product}/playbooks.
type CreatePlaybookRequest struct {
	ProductCode    string              `json:"-"`
	Name           string              `json:"name"`
	Description    *string             `json:"description"`
	LifecycleStage LifecycleStage      `json:"lifecycleStage"`
	PlaybookType   PlaybookType        `json:"playbookType"`
	Tasks          []PlaybookTaskInput `json:"tasks"`
}

// PatchPlaybookRequest updates playbook metadata. The task list is replaced
// through PUT /playbooks/{id}/tasks instead.
type PatchPlaybookRequest struct {
	ID             string          `json:"-"`
	Name           *string         `json:"name"`
	Description    *string         `json:"description"`
	LifecycleStage *LifecycleStage `json:"lifecycleStage"`
	PlaybookType   *PlaybookType   `json:"playbookType"`
	Active         *bool           `json:"active"`
}

// ReplacePlaybookTasksRequest replaces a playbook's task list.
//
// Runs already in flight are never touched: instances are copies, so a template
// edit reaches new runs only. That is the whole reconciliation policy — there is
// no plan/apply prompt, and no versioning to say which revision a run started
// from.
type ReplacePlaybookTasksRequest struct {
	PlaybookID string              `json:"-"`
	Tasks      []PlaybookTaskInput `json:"tasks"`
}

// ---------------------------------------------------------------------------
// Playbook execution
// ---------------------------------------------------------------------------

// RunTask is one task instance on one playbook run.
//
// It is a copy of the template task, not a reference, so editing a template can
// never silently rewrite work already recorded. Exactly one of BoolValue /
// NumberValue / TextValue / CheckedCodes is ever set, matching ValueType, and
// IsCompleted is generated from it by the database — a status and a value cannot
// disagree.
type RunTask struct {
	ID             string        `json:"id"`
	PlaybookRunID  string        `json:"playbookRunId"`
	PlaybookTaskID string        `json:"playbookTaskId"`
	Code           string        `json:"code"`
	Name           string        `json:"name"`
	Description    *string       `json:"description"`
	SequenceNo     int           `json:"sequenceNo"`
	ValueType      TaskValueType `json:"valueType"`
	BoolValue      *bool         `json:"boolValue"`
	NumberValue    *float64      `json:"numberValue"`
	TextValue      *string       `json:"textValue"`
	// Options are the reasons this task offers, copied from the template with
	// the rest of the task. CheckedCodes are the ones ticked. Both are empty
	// unless ValueType is CHECKLIST.
	Options      []ChecklistOption `json:"options"`
	CheckedCodes []string          `json:"checkedCodes"`
	IsCompleted  bool              `json:"isCompleted"`
	CompletedOn  *time.Time        `json:"completedOn"`
	CompletedBy  *UserRef          `json:"completedBy"`
	IsBookend    bool              `json:"isBookend"`
}

// PlaybookRun is one playbook running on one pairing.
type PlaybookRun struct {
	ID            string         `json:"id"`
	OrgPlatformID string         `json:"orgPlatformId"`
	PlaybookID    string         `json:"playbookId"`
	PlaybookName  string         `json:"playbookName"`
	Description   *string        `json:"description"`
	PlaybookStage LifecycleStage `json:"playbookStage"`
	PlaybookType  PlaybookType   `json:"playbookType"`
	RunStatus     RunStatus      `json:"runStatus"`
	TaskTotal     int            `json:"taskTotal"`
	TaskCompleted int            `json:"taskCompleted"`
	NextTaskName  *string        `json:"nextTaskName"`
	AddedBy       *UserRef       `json:"addedBy"`
	CreatedOn     time.Time      `json:"createdOn"`
	Tasks         []RunTask      `json:"tasks"`
}

// RunProgress rolls run and task counts up across one pairing.
type RunProgress struct {
	RunTotal      int `json:"runTotal"`
	RunActive     int `json:"runActive"`
	RunClosed     int `json:"runClosed"`
	TaskTotal     int `json:"taskTotal"`
	TaskCompleted int `json:"taskCompleted"`
}

// Note is one entry in a pairing's comment trail. Append-only.
type Note struct {
	ID        string    `json:"id"`
	Body      string    `json:"body"`
	Author    *UserRef  `json:"author"`
	CreatedOn time.Time `json:"createdOn"`

	// UpdatedOn is nil until the note is first edited, so "edited" is derived
	// rather than stored. The body it replaced is in plg_note_revision, keyed on
	// this note's ID — which is why the portal shows that ID beside the note.
	UpdatedOn *time.Time `json:"updatedOn"`
	UpdatedBy *UserRef   `json:"updatedBy"`
}

// UpdateNoteRequest is the body of PATCH /notes/{noteId}.
//
// Only the body may change. The author, the pairing and the created date are
// not the editor's to rewrite — an edit corrects what was said, not who said it
// or when.
type UpdateNoteRequest struct {
	ID   string `json:"-"`
	Body string `json:"body"`
}

// LifecycleEntry is one stage change on one pairing.
type LifecycleEntry struct {
	ID string `json:"id"`

	// The stage half. Nil on a row that recorded only a health change — the
	// timeline shows both kinds of event, and an entry says which one it was by
	// which half is populated.
	FromStage *LifecycleStage `json:"fromStage"`
	ToStage   *LifecycleStage `json:"toStage"`
	ToName    *string         `json:"toName"`

	// The health half. Nil on a row that recorded only a stage change.
	FromHealth *HealthState `json:"fromHealth"`
	ToHealth   *HealthState `json:"toHealth"`

	// The subscription half. Nil unless this row recorded a tier change.
	FromSubscription *SubscriptionTier `json:"fromSubscription"`
	ToSubscription   *SubscriptionTier `json:"toSubscription"`

	Reason    *string   `json:"reason"`
	ChangedBy *UserRef  `json:"changedBy"`
	ChangedOn time.Time `json:"changedOn"`
}

// ProductDetail is the product tab, in the order it renders.
type ProductDetail struct {
	OrgPlatformID    string        `json:"orgPlatformId"`
	OrganizationID   string        `json:"organizationId"`
	OrganizationName string        `json:"organizationName"`
	Product          PlgProductRef `json:"product"`

	// The lifecycle diagram — both axes.
	LifecycleStage LifecycleStage `json:"lifecycleStage"`
	StageName      string         `json:"stageName"`
	StageEnteredOn time.Time      `json:"stageEnteredOn"`

	// Health, with the same treatment the stage gets: the value, when it was
	// set, and by whom. An engineer sets it; nothing derives it.
	HealthState     HealthState `json:"healthState"`
	HealthEnteredOn time.Time   `json:"healthEnteredOn"`
	HealthUpdatedBy *UserRef    `json:"healthUpdatedBy"`

	// ApplicablePlaybookTypes are the kinds this pairing is currently offered,
	// which follows from its health. Sent rather than left to the caller so the
	// mapping lives in one place — see ApplicablePlaybookTypes.
	//
	// Plural since the SUSTAINING kind arrived: a healthy pairing is offered
	// both progressive and sustaining work.
	ApplicablePlaybookTypes []PlaybookType `json:"applicablePlaybookTypes"`

	// CarriesPlaybooks is now "does a playbook of the applicable kind actually
	// exist here", not "is this stage on a path". Every stage carries playbooks
	// in principle; the useful question is whether anyone has authored one.
	CarriesPlaybooks bool `json:"carriesPlaybooks"`

	// The registration metadata tile.
	RegisteredOn    time.Time `json:"registeredOn"`
	RegisteredEmail string    `json:"registeredEmail"`

	// The third axis, with the provenance the other two carry. Nil until
	// somebody records a tier — there is no subscription feed, so absent means
	// "nobody has said", not "free".
	SubscriptionTier      *SubscriptionTier `json:"subscriptionTier"`
	SubscriptionEnteredOn *time.Time        `json:"subscriptionEnteredOn"`
	SubscriptionUpdatedBy *UserRef          `json:"subscriptionUpdatedBy"`
	TrialEndDate          *time.Time        `json:"trialEndDate"`
	TrialExtendedDate     *time.Time        `json:"trialExtendedDate"`
	// CurrentPeriodEndDate is whichever of the two dates the CURRENT tier makes
	// live, resolved in SQL by plg_current_period_end_date so the rule has one
	// home. NULL on FREE and PAYG, which have no period that ends.
	CurrentPeriodEndDate *time.Time `json:"currentPeriodEndDate"`

	// Acknowledgement.
	AcknowledgedOn *time.Time `json:"acknowledgedOn"`
	AcknowledgedBy *UserRef   `json:"acknowledgedBy"`
	IsNew          bool       `json:"isNew"`

	// Playbook cards, plus what is available to add at this stage.
	Runs               []PlaybookRun `json:"runs"`
	Progress           RunProgress   `json:"progress"`
	AvailablePlaybooks []Playbook    `json:"availablePlaybooks"`

	// The comment trail, and this pairing's stage history. What the customer is
	// trying to build is recorded in the trail: dated, append-only, and able to
	// change without erasing what was understood before.
	Notes            []Note           `json:"notes"`
	LifecycleHistory []LifecycleEntry `json:"lifecycleHistory"`
}

// PatchOrgPlatformRequest updates one pairing. Every field is optional; at least
// one is required.
type PatchOrgPlatformRequest struct {
	OrganizationID string `json:"-"`
	ProductCode    string `json:"-"`

	// ExpectedStage is the stage the caller believed was current when it
	// decided to make this change. Sent as a precondition: the update applies
	// only while it still holds. Nil means "set it regardless" — a blind write.
	ExpectedStage *LifecycleStage `json:"expectedStage"`

	LifecycleStage *LifecycleStage `json:"lifecycleStage"`

	// Reason explains whichever axis moved, and is required whenever one does.
	//
	// One field rather than one per axis, because the history table has one
	// reason column: a request that moved two
	// axes at once would write the same sentence on both rows, which is the
	// honest outcome and not one the UI can produce — each axis has its own
	// Save button and sends its own request.
	Reason *string `json:"reason"`

	// HealthState moves the second axis. Independent of the stage: a request may
	// change either, both or neither, and a health change carries the same
	// reason field because "why" matters as much for going at-risk as for
	// moving stage.
	HealthState            *HealthState      `json:"healthState"`
	SubscriptionTier       *SubscriptionTier `json:"subscriptionTier"`
	TrialEndDate           *string           `json:"trialEndDate"`
	ClearTrialEndDate      bool              `json:"clearTrialEndDate"`
	TrialExtendedDate      *string           `json:"trialExtendedDate"`
	ClearTrialExtendedDate bool              `json:"clearTrialExtendedDate"`
}

// AttachPlaybookRequest adds a playbook to a pairing and copies its tasks.
type AttachPlaybookRequest struct {
	OrganizationID string `json:"-"`
	ProductCode    string `json:"-"`
	PlaybookID     string `json:"playbookId"`
}

// PatchRunTaskRequest records a task's value.
//
// Sending a value is what completes the task — there is no separate tick to
// forget. ClearValue exists because a JSON null cannot distinguish "not
// supplied" from "cleared".
type PatchRunTaskRequest struct {
	ID          string   `json:"-"`
	BoolValue   *bool    `json:"boolValue"`
	NumberValue *float64 `json:"numberValue"`
	TextValue   *string  `json:"textValue"`
	// CheckedCodes is the complete set of ticked reasons, not a delta. Sending
	// the whole set makes the call idempotent and means two engineers ticking at
	// once cannot interleave into a state neither chose.
	//
	// An empty array is a cleared task, which is why it is a pointer: a nil
	// slice means "not supplied" and an empty one means "none of them".
	CheckedCodes *[]string `json:"checkedCodes"`
	ClearValue   bool      `json:"clearValue"`
}

// CreateNoteRequest appends to a pairing's comment trail.
type CreateNoteRequest struct {
	OrganizationID string `json:"-"`
	ProductCode    string `json:"-"`
	Body           string `json:"body"`
}

// ---------------------------------------------------------------------------
// Registrations — the New Registrations panel
// ---------------------------------------------------------------------------

// RegistrationItem is one pairing awaiting acknowledgement.
type RegistrationItem struct {
	OrgPlatformID    string         `json:"orgPlatformId"`
	OrganizationID   string         `json:"organizationId"`
	OrganizationName string         `json:"organizationName"`
	Product          PlgProductRef  `json:"product"`
	RegisteredEmail  string         `json:"registeredEmail"`
	RegisteredName   *string        `json:"registeredName"`
	RegisteredOn     time.Time      `json:"registeredOn"`
	LifecycleStage   LifecycleStage `json:"lifecycleStage"`
	StageName        string         `json:"stageName"`
	Owner            *UserRef       `json:"owner"`
}

// SearchRegistrationsRequest is the body of POST /registrations/search.
type SearchRegistrationsRequest struct {
	UnacknowledgedOnly bool                      `json:"unacknowledgedOnly"`
	Filters            OrganizationSearchFilters `json:"filters"`
	Pagination         Pagination                `json:"pagination"`
}

// SearchRegistrationsResponse is the paged registration list.
type SearchRegistrationsResponse struct {
	Registrations []RegistrationItem `json:"registrations"`
	Total         int                `json:"total"`
	Limit         int                `json:"limit"`
	Offset        int                `json:"offset"`
}

// AcknowledgeRequest claims a pairing. Acknowledging also assigns the
// organisation's CS owner, since ownership is organisation-level.
type AcknowledgeRequest struct {
	OrgPlatformID string  `json:"-"`
	OwnerID       *string `json:"ownerId"`
}

// ---------------------------------------------------------------------------
// Work queue
// ---------------------------------------------------------------------------

// QueueReason says why a pairing is in the work queue.
//
// Derived, never stored, so it cannot disagree with the runs it describes.
type QueueReason string

// Queue reasons.
const (
	// There is deliberately no "finished" reason. A pairing with everything
	// closed and nothing left to attach LEAVES the queue rather than sitting on
	// it — see plg_work_queue_v's WHERE clause.
	//
	// Old doc, kept for one release so a reader meeting the name knows where it
	// went:
	ReasonNoPlaybook QueueReason = "NO_PLAYBOOK"
	// ReasonNotStarted — a playbook is attached but nobody has ticked Initiate.
	ReasonNotStarted QueueReason = "NOT_STARTED"
	// ReasonInProgress — at least one playbook is running.
	ReasonInProgress QueueReason = "IN_PROGRESS"
)

// QueueReasonOrder is the order the queue surfaces them in: what is waiting on a
// decision before what is already moving.
var QueueReasonOrder = []QueueReason{
	ReasonNoPlaybook, ReasonNotStarted, ReasonInProgress,
}

// ValidQueueReason is the allow-list checked in the service layer.
var ValidQueueReason = map[QueueReason]bool{
	ReasonNoPlaybook: true,
	ReasonNotStarted: true, ReasonInProgress: true,
}

// WorkQueueItem is one organisation+platform pairing needing attention.
//
// The unit is the pairing, not the playbook run: a pairing enters the queue when
// it is acknowledged and leaves when it reaches a stage carrying no playbooks.
// Whose queue it is comes from the organisation's owner, read at query time —
// so reassigning a customer moves all of its pairings at once.
type WorkQueueItem struct {
	// Health, on every row. An at-risk pairing is the most valuable thing in the
	// queue and the list has to be able to say so without a second query.
	HealthState     HealthState `json:"healthState"`
	HealthEnteredOn time.Time   `json:"healthEnteredOn"`

	OrgPlatformID    string         `json:"orgPlatformId"`
	OrganizationID   string         `json:"organizationId"`
	OrganizationName string         `json:"organizationName"`
	Product          PlgProductRef  `json:"product"`
	CurrentStage     LifecycleStage `json:"currentStage"`
	StageName        string         `json:"stageName"`
	Reason           QueueReason    `json:"reason"`

	// RunActive, RunTotal and the task counts describe the whole pairing, not
	// one run.
	RunActive     int `json:"runActive"`
	RunTotal      int `json:"runTotal"`
	TaskCompleted int `json:"taskCompleted"`
	TaskTotal     int `json:"taskTotal"`
	// AvailablePlaybooks is how many could still be added at this stage.
	AvailablePlaybooks int `json:"availablePlaybooks"`

	// The running playbook and its next open task, when something is running.
	PlaybookID   *string `json:"playbookId"`
	PlaybookName *string `json:"playbookName"`
	NextTaskCode *string `json:"nextTaskCode"`
	NextTaskName *string `json:"nextTaskName"`

	Owner *UserRef `json:"owner"`
	// AcknowledgedBy is who triaged this pairing. Often but not always the
	// owner: acknowledging an already-owned customer records you without
	// reassigning them, so this answers "who picked this up" where Owner
	// answers "whose work is it now".
	AcknowledgedBy *UserRef   `json:"acknowledgedBy"`
	AcknowledgedOn *time.Time `json:"acknowledgedOn"`
	RegisteredOn   time.Time  `json:"registeredOn"`
}

// StageTile is one tile: how many pairings sit at one stage for one product.
//
// PlgProduct and stage together partition the queue — every pairing has exactly one
// of each — so the tiles sum to the list total. Grouping by playbook did not:
// a pairing with two playbooks running counted twice, and one with none counted
// nowhere.
type StageTile struct {
	LifecycleStage LifecycleStage `json:"lifecycleStage"`
	StageName      string         `json:"stageName"`
	Pairings       int            `json:"pairings"`
	// AtRisk is how many of those pairings are in trouble. A tile saying
	// "12 pairings, 4 at risk" tells an engineer where to start; one saying
	// only "12" does not.
	AtRisk int `json:"atRisk"`
	// The reason breakdown within the tile, so a tile says what kind of work it
	// holds and not merely how much.
	NoPlaybook int `json:"noPlaybook"`
	NotStarted int `json:"notStarted"`
	InProgress int `json:"inProgress"`
}

// ProductQueueGroup is the tiles for one product.
type ProductQueueGroup struct {
	Product  PlgProductRef `json:"product"`
	Pairings int           `json:"pairings"`
	Tiles    []StageTile   `json:"tiles"`
}

// WorkQueueFilters narrows the queue. These are exactly the four the requirement
// names, plus the owner filter.
//
// OwnerIDs is a slice like every other filter here, and there is no "mine"
// shortcut: the BFF resolves the caller to an id and sends a one-element
// OwnerIDs. entity-service has no notion of "me", which is the point of leaving
// identity in the BFF.
type WorkQueueFilters struct {
	// HealthStates narrows to healthy or at-risk pairings. Like product and
	// stage — and unlike reason — it narrows the TILES as well as the list,
	// because "show me what is at risk" is a different queue, not a subset of
	// the one on screen.
	HealthStates    []HealthState
	OwnerIDs        []string
	OrganizationIDs []string
	ProductCodes    []string
	// LifecycleStages is what a tile click narrows by, alongside the product.
	LifecycleStages []LifecycleStage
	Reasons         []QueueReason
	PlaybookIDs     []string
	TaskCodes       []string
}

// WorkQueueResponse groups the per-product tiles and the pairing list.
type WorkQueueResponse struct {
	ProductGroups []ProductQueueGroup `json:"productGroups"`
	Items         []WorkQueueItem     `json:"items"`
	Total         int                 `json:"total"`
	// ByReason counts the whole queue by reason, so the header can say what kind
	// of work is waiting without the client summing the tiles.
	ByReason map[QueueReason]int `json:"byReason"`
}

// ---------------------------------------------------------------------------
// Analytics
// ---------------------------------------------------------------------------

// AnalyticsRange bounds every analytics query.
type AnalyticsRange struct {
	From *time.Time
	To   *time.Time
}

// CountByLabel is a generic labelled count used by every breakdown chart.
type CountByLabel struct {
	Label string `json:"label"`
	Count int    `json:"count"`
}

// TimeBucketSeries is one named series across shared time buckets.
type TimeBucketSeries struct {
	Label  string `json:"label"`
	Counts []int  `json:"counts"`
}

// TimeSeries is a set of series sharing one bucket axis.
type TimeSeries struct {
	Buckets []string           `json:"buckets"`
	Series  []TimeBucketSeries `json:"series"`
}

// DashboardSummary is the three headline tiles, plus counts other pages read
// from the same payload.
type DashboardSummary struct {
	TotalOrganizations int `json:"totalOrganizations"`
	// TotalRegistrations counts pairings, not registration rows — a registration
	// is unique to the organisation+product combination.
	TotalRegistrations int `json:"totalRegistrations"`
	// NewRegistrations is never period-scoped: a backlog that disappears because
	// someone narrowed the date range is a backlog nobody clears.
	NewRegistrations int `json:"newRegistrations"`
	// PairingsNeedingAttention is the work queue's own count, read from the same
	// view — so the dashboard tile and the page it links to always agree. Not
	// period-scoped, for the same reason NewRegistrations is not.
	PairingsNeedingAttention int `json:"pairingsNeedingAttention"`
	TrialsEndingSoon         int `json:"trialsEndingSoon"`
}

// DashboardAnalytics is the single payload the dashboard consumes.
type DashboardAnalytics struct {
	Summary                       DashboardSummary `json:"summary"`
	RegistrationsByProduct        []CountByLabel   `json:"registrationsByProduct"`
	RegistrationsByLifecycleStage []CountByLabel   `json:"registrationsByLifecycleStage"`
	SubscriptionMix               []CountByLabel   `json:"subscriptionMix"`
	RegistrationsOverTime         TimeSeries       `json:"registrationsOverTime"`
}

// ---------------------------------------------------------------------------
// Ingestion
// ---------------------------------------------------------------------------

// Registration is one incoming record, resolved into the portal's own terms.
//
// Deliberately not named after a source. The keys an event arrives with are
// configuration — see the source map — so this type describes what the *portal*
// needs, and a change of source moves a line of JSON rather than this struct.
//
// Every field but the first two is optional, because a source may simply not
// send it. Absent stays absent: the sparse-upsert rule keeps NULL meaning
// "never received".
type Registration struct {
	OrganizationName      string      `json:"organizationName"`
	RegisteredEmail       string      `json:"registeredEmail"`
	CreatedOn             *SourceTime `json:"createdOn"`
	InitiatedPlatform     string      `json:"initiatedPlatform"`
	CountryName           string      `json:"countryName"`
	CompanyNameFromDomain string      `json:"companyNameFromDomain"`
	CompanyID             string      `json:"companyId"`
	FirstName             string      `json:"firstName"`
	LastName              string      `json:"lastName"`

	// Extra holds the mapped extra fields, keyed by their portal-side name.
	// Not part of the wire contract: the caller sends Attributes instead, which
	// is Extra already resolved through the source map.
	Extra map[string]string `json:"-"`
}

// SourceTime is a timestamp as a source actually sends it.
//
// A plain time.Time would not do: Go's JSON decoder accepts RFC 3339 only, and
// real payloads have carried both "2026-09-03T10:59:41.472Z" and the US display
// form "10/23/2019 10:31 PM". Refusing either loses a whole registration over a
// date format.
type SourceTime struct {
	time.Time
}

// sourceTimeLayouts are tried in order, most specific first.
//
// Deliberately explicit rather than a clever parser: every entry is a format
// something has actually sent, so adding one is a one-line change with an
// obvious meaning.
var sourceTimeLayouts = []string{
	time.RFC3339,          // 2026-09-03T10:59:41.472Z  — what Moesif sends
	"2006-01-02T15:04:05", // 2026-09-03T11:11:48.132   — no zone
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
	"1/2/2006 3:04:05 PM",
	"1/2/2006 3:04 PM", // 10/23/2019 10:31 PM       — a US display format
	"1/2/2006 15:04:05",
	"1/2/2006 15:04",
	"2006-01-02",
	"1/2/2006",
}

// ParseSourceTime parses whichever of the known formats a value is in.
//
// A missing zone is read as UTC. That is a real assumption, not a neutral one: a
// late-evening local time read as UTC can land on the previous day, so a
// registration date can be out by one. It is the only defensible default without
// knowing the sending system's timezone, and better than refusing the record.
func ParseSourceTime(raw string) (*SourceTime, error) {
	if raw = strings.TrimSpace(raw); raw == "" {
		return nil, nil
	}
	for _, layout := range sourceTimeLayouts {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return &SourceTime{Time: parsed.UTC()}, nil
		}
	}
	// Present but unintelligible is reported rather than quietly replaced with
	// "now": an absent date means the source did not send one, and inventing a
	// registration date is worse than saying the value was not understood.
	// Constructed as a struct literal rather than through a helper: this package
	// uses entity-service's apierror verbatim, which exposes the error types and
	// no constructors.
	return nil, &apierror.ValidationError{Msg: fmt.Sprintf(
		"%q is not a date this portal recognises (accepted: RFC 3339 such as "+
			"2026-09-03T10:59:41Z, \"2026-09-03 10:59:41\", \"9/3/2026 10:59 AM\", \"2026-09-03\")", raw)}
}

// IngestResult reports what the portal did with one inbound payload.
type IngestResult struct {
	Status           string  `json:"status"`
	OrganizationID   string  `json:"organizationId"`
	OrganizationName string  `json:"organizationName"`
	OrgPlatformID    *string `json:"orgPlatformId"`
	PersonEmail      string  `json:"personEmail"`
	Message          string  `json:"message"`
}

// IngestBatchResult reports what happened to each payload in a batch delivery.
type IngestBatchResult struct {
	Accepted int            `json:"accepted"`
	Failed   int            `json:"failed"`
	Results  []IngestResult `json:"results"`
}

// AttributeScope says whether an extra source field describes the customer
// or one platform under it. The ingest cannot infer this, so the attribute map
// declares it.
type AttributeScope string

// Attribute scopes.
const (
	// ScopeOrganization attaches the value to the customer.
	ScopeOrganization AttributeScope = "organization"
	// ScopePlatform attaches it to the organisation+platform pairing.
	ScopePlatform AttributeScope = "platform"
)

// OrganizationAttribute is one extra source field, resolved through the
// attribute map and ready to store.
type OrganizationAttribute struct {
	Name        string         `json:"name"`
	Value       string         `json:"value"`
	SourceField string         `json:"sourceField"`
	Scope       AttributeScope `json:"scope"`
}

// ---------------------------------------------------------------------------
// The write contract
//
// The portal spans two services, so a write cannot hold a `SELECT … FOR UPDATE`
// across the read, the decision and the write: the BFF reads, decides, and by
// the time it writes another engineer may have moved the row.
//
// So every guarded write carries the precondition the BFF decided against, and
// entity-service applies it as a WHERE clause inside one transaction. The
// answer is a row count, not a success flag — `RowsAffected: 0` means the
// precondition no longer held, and it is the BFF's job to decide whether that
// is a 409, a 403 or a no-op. This layer does not know, which is deliberate:
// the rule is the BFF's, only the atomicity is ours.
//
// See plg-docs/ENTITY-SERVICE-CONTRACT.md for the endpoint each of these serves.
// ---------------------------------------------------------------------------

// WriteResult is the answer to a guarded write that has nothing to report but
// whether it landed.
//
// RowsAffected is deliberately not a bool. A caller that wants "did it work"
// can test `> 0`, but a caller replacing a task set wants the count, and one
// shape for both means no endpoint has to invent its own.
type WriteResult struct {
	RowsAffected int `json:"rowsAffected"`
}

// Applied reports whether a guarded write's precondition still held.
func (r WriteResult) Applied() bool { return r.RowsAffected > 0 }

// AcknowledgeResult answers POST /plg/registrations/{orgPlatformId}/acknowledge.
//
// The identifiers come back because the BFF's own response is a whole
// ProductDetail, and it needs to know which pairing to reload — it cannot read
// them off a row it never locked.
type AcknowledgeResult struct {
	RowsAffected   int    `json:"rowsAffected"`
	OrganizationID string `json:"organizationId"`
	ProductCode    string `json:"productCode"`
}

// Applied reports whether the registration was still unacknowledged.
func (r AcknowledgeResult) Applied() bool { return r.RowsAffected > 0 }

// PatchPairingResult answers PATCH /plg/pairings/{orgPlatformId}.
//
// CurrentStage is the stage after the write, so a BFF that lost the race can
// tell the caller what the stage actually is rather than only that it moved.
type PatchPairingResult struct {
	RowsAffected int            `json:"rowsAffected"`
	CurrentStage LifecycleStage `json:"currentStage"`
}

// Applied reports whether the expected stage still held.
func (r PatchPairingResult) Applied() bool { return r.RowsAffected > 0 }

// AttachPlaybookResult answers POST /plg/pairings/{orgPlatformId}/playbook-runs.
//
// AlreadyAttached and a zero RowsAffected are different outcomes and must not
// be conflated: the first is success (attaching is idempotent), the second
// means the playbook was not active and the BFF should answer 400.
type AttachPlaybookResult struct {
	RowsAffected    int    `json:"rowsAffected"`
	PlaybookRunID   string `json:"playbookRunId"`
	AlreadyAttached bool   `json:"alreadyAttached"`
}

// CreatePlaybookResult answers POST /plg/playbooks.
type CreatePlaybookResult struct {
	PlaybookID string `json:"playbookId"`
}

// ---------------------------------------------------------------------------
// Users
//
// PLG does not own the users table — entity-service does, and after the merge
// it is the same table csm-portal reads. So the slice exposes a search over it
// rather than any way to write one: the CS team is administered elsewhere.
// ---------------------------------------------------------------------------

// UserSearchFilters narrows POST /plg/users/search.
//
// Emails is how the BFF resolves the caller. It is a filter rather than a
// dedicated by-email endpoint because it is also how the owner pickers work,
// and one query serving both means one place where "which engineers exist" is
// answered.
type UserSearchFilters struct {
	IDs       []string `json:"ids"`
	Emails    []string `json:"emails"`
	UserTypes []string `json:"userTypes"`
	// Active filters on users.active. Nil means "either", which the owner
	// pickers never want and an attribution lookup always does: a note written
	// by someone who has since left must still render their name.
	Active *bool `json:"active"`
}

// PlgSearchUsersRequest is the body of POST /plg/users/search.
// PlgSearchUsersRequest is PLG's own, and stays its own after the merge —
// which is worth explaining, because entity-service has a SearchUsersRequest
// that looks like it would do.
//
// It would not. PLG restricts every user query to active INTERNAL staff: the
// table is shared with csm-portal, which keeps customers, partners and system
// actors in it, and an owner picker offering a customer is a data-integrity
// problem that surfaces as a confusing name on a screen. Upstream's
// SearchUsersFilters has no user_type field at all, so that restriction cannot
// be expressed through it.
//
// Reusing upstream's type would therefore mean applying the filter at each call
// site instead of structurally — turning a property the schema tests pin into a
// convention someone can forget. The duplication is the cheaper of the two.
type PlgSearchUsersRequest struct {
	Filters    UserSearchFilters `json:"filters"`
	Pagination Pagination        `json:"pagination"`
}

// PlgSearchUsersResponse is the reply to POST /plg/users/search.
type PlgSearchUsersResponse struct {
	Users  []UserRef `json:"users"`
	Total  int       `json:"total"`
	Limit  int       `json:"limit"`
	Offset int       `json:"offset"`
}

// ---------------------------------------------------------------------------
// Ingest
// ---------------------------------------------------------------------------

// IngestRegistrationsRequest is the body of POST /plg/registrations/ingest.
//
// A batch, because the queue can hand over several registrations in one
// delivery and each must land independently — one bad record does not reject
// the rest. Keeping the batch on this side of the wire is what preserves that:
// the BFF's poller forwards what it received rather than issuing one call per
// record and losing the per-record transaction.
type IngestRegistrationsRequest struct {
	Registrations []IngestRegistration `json:"registrations"`
}

// IngestRegistration is one registration on the wire, with its overflow
// attributes already mapped.
//
// The mapping is NOT done here. Which incoming key becomes which portal field
// is configuration — backend/source-map.json — and that file belongs to the
// BFF, which is what reads the queue. By the time a registration reaches
// entity-service the vocabulary question is settled and only the writing is
// left, which is the division this whole contract is built on.
type IngestRegistration struct {
	Registration
	Attributes []OrganizationAttribute `json:"attributes"`
}

// RecordIngestFailureRequest is the body of POST /plg/ingest-failures.
//
// An event consumed from the queue is deleted from it — there is no ack and no
// redelivery — so an event that cannot be turned into a registration has
// nowhere left to exist. This is where it goes.
type RecordIngestFailureRequest struct {
	EventID    *string         `json:"eventId"`
	EventType  *string         `json:"eventType"`
	ReceivedAt *time.Time      `json:"receivedAt"`
	Payload    json.RawMessage `json:"payload"`
	Failure    string          `json:"failure"`
}

// RecordIngestFailureResult answers POST /plg/ingest-failures.
//
// Deliberately empty. The row's generated id is NOT returned: parking a failure
// is fire-and-forget for the poller, and failures are worked through by querying
// plg_ingest_failure directly — `WHERE resolved_on IS NULL`, which is the
// partial index the table carries for exactly that sweep.
//
// It previously declared an `id` that nothing ever populated, so every success
// answered `{"id":""}`. An always-blank field is worse than no field: a caller
// can reasonably read it as the id, or read blank as failure when the write in
// fact succeeded. Returning `{}` says what is true — it worked, and there is
// nothing further to tell you.
type RecordIngestFailureResult struct{}

// ---------------------------------------------------------------------------
// The work-queue search body
// ---------------------------------------------------------------------------

// WorkQueueSearchFilters is WorkQueueFilters with JSON tags.
//
// A separate type rather than tags on WorkQueueFilters, because the two are
// genuinely different things: WorkQueueFilters is the internal shape the
// repository builds SQL from, and this is the wire shape.
//
// It is a POST body for the same reason every other multi-filter search in
// entity-service is: eight repeatable dimensions do not belong in a query
// string, and `POST …/search` is the convention this slice is merging into.
type WorkQueueSearchFilters struct {
	HealthStates    []HealthState    `json:"healthStates"`
	OwnerIDs        []string         `json:"ownerIds"`
	OrganizationIDs []string         `json:"organizationIds"`
	ProductCodes    []string         `json:"productCodes"`
	LifecycleStages []LifecycleStage `json:"lifecycleStages"`
	Reasons         []QueueReason    `json:"reasons"`
	PlaybookIDs     []string         `json:"playbookIds"`
	TaskCodes       []string         `json:"taskCodes"`
}

// ToFilters converts the wire shape to the repository's.
func (f WorkQueueSearchFilters) ToFilters() WorkQueueFilters {
	return WorkQueueFilters{
		HealthStates:    f.HealthStates,
		OwnerIDs:        f.OwnerIDs,
		OrganizationIDs: f.OrganizationIDs,
		ProductCodes:    f.ProductCodes,
		LifecycleStages: f.LifecycleStages,
		Reasons:         f.Reasons,
		PlaybookIDs:     f.PlaybookIDs,
		TaskCodes:       f.TaskCodes,
	}
}

// SearchWorkQueueRequest is the body of POST /plg/work-queue/search.
//
// No Pagination field, deliberately. The whole queue comes back capped at 500
// rows, with the tiles computed over that same set and the frontend filtering
// and grouping from there. Adding pagination would change what the tiles mean —
// they count the queue, not the page — so it is left out rather than
// half-added.
type SearchWorkQueueRequest struct {
	Filters WorkQueueSearchFilters `json:"filters"`
}

// RunTaskShape is a task instance's declared type and the option codes it
// offers, with no value attached.
//
// Exists so the BFF can reject a value the task cannot hold — "this task is a
// tick box; send boolValue" — before attempting a write. That message is a PLG
// rule and stayed in the BFF, but the shape it needs is data, and data lives
// here. Returning just the shape rather than the whole task keeps the check one
// small read instead of loading a pairing.
type RunTaskShape struct {
	ValueType   TaskValueType `json:"valueType"`
	OptionCodes []string      `json:"optionCodes"`
}

// PairingLocation is the organisation and product a pairing id belongs to.
//
// The portal addresses a pairing as org+product everywhere, but a few routes
// carry only the pairing's own id — an acknowledgement, a task, a note. This
// turns one into the other so the caller can reload the tab it just changed.
type PairingLocation struct {
	OrganizationID string `json:"organizationId"`
	ProductCode    string `json:"productCode"`
}
