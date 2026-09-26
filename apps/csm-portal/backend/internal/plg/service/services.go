package service

import (
	"context"
	"strings"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/apierror"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/domain"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/plg/repository"
)

// ---------------------------------------------------------------------------
// Reference
// ---------------------------------------------------------------------------

// ReferenceService serves products, engineers and the lifecycle catalogue.
type ReferenceService interface {
	ListProducts(ctx context.Context) ([]domain.Product, error)
	ListCSUsers(ctx context.Context) ([]domain.UserRef, error)
	LifecycleCatalogue(ctx context.Context) (*domain.LifecycleCatalogue, error)
}

type referenceService struct {
	repo repository.ReferenceRepository
}

// NewReferenceService wires a ReferenceService over its repository.
func NewReferenceService(repo repository.ReferenceRepository) ReferenceService {
	return &referenceService{repo: repo}
}

func (s *referenceService) ListProducts(ctx context.Context) ([]domain.Product, error) {
	return s.repo.ListProducts(ctx)
}

func (s *referenceService) ListCSUsers(ctx context.Context) ([]domain.UserRef, error) {
	return s.repo.ListCSUsers(ctx)
}

// There is deliberately no Me method here. The identity middleware resolves the
// caller before any handler runs — it has to, in order to reject an unknown one
// — so the handler reads the UserRef from the context instead of asking for it
// again.
//
// The trap this avoids: falling back to a synthesised user for an address the
// directory has not seen. An invented user passes every check until the first
// attributed write, where it fails as a foreign-key violation.

func (s *referenceService) LifecycleCatalogue(ctx context.Context) (*domain.LifecycleCatalogue, error) {
	return s.repo.LifecycleCatalogue(ctx)
}

// ---------------------------------------------------------------------------
// Organisation
// ---------------------------------------------------------------------------

// OrganizationService owns the organisation list and the overview tab.
type OrganizationService interface {
	Search(ctx context.Context, req domain.SearchOrganizationsRequest) (domain.SearchOrganizationsResponse, error)
	Get(ctx context.Context, id string) (*domain.OrganizationDetail, error)
	Patch(ctx context.Context, req domain.PatchOrganizationRequest) (*domain.OrganizationDetail, error)
}

type organizationService struct {
	repo repository.OrganizationRepository
}

// NewOrganizationService wires an OrganizationService over its repository.
func NewOrganizationService(repo repository.OrganizationRepository) OrganizationService {
	return &organizationService{repo: repo}
}

func (s *organizationService) Search(ctx context.Context, req domain.SearchOrganizationsRequest) (domain.SearchOrganizationsResponse, error) {
	if err := validateSearchFilters(&req.Filters); err != nil {
		return domain.SearchOrganizationsResponse{}, err
	}
	normalizePagination(&req.Pagination)

	orgs, total, err := s.repo.Search(ctx, req)
	if err != nil {
		return domain.SearchOrganizationsResponse{}, err
	}
	return domain.SearchOrganizationsResponse{
		Organizations: orgs,
		Total:         total,
		Limit:         req.Pagination.Limit,
		Offset:        req.Pagination.Offset,
	}, nil
}

func (s *organizationService) Get(ctx context.Context, id string) (*domain.OrganizationDetail, error) {
	if err := validateUUID("organizationId", id); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, id)
}

func (s *organizationService) Patch(ctx context.Context, req domain.PatchOrganizationRequest) (*domain.OrganizationDetail, error) {
	if err := validateUUID("organizationId", req.ID); err != nil {
		return nil, err
	}
	// An organisation always has an owner once it has one: the work queue reads
	// ownership to decide whose queue a pairing sits in, and clearing it would
	// drop every one of that customer's pairings out of everybody's view while
	// leaving them in the queue. Reassign to a person instead.
	// THE RULE STAYS HERE. entity-service will happily write a NULL owner — it
	// stores what it is told — so this is the only thing standing between the
	// portal and an organisation nobody can see. That is the split working: the
	// data owner keeps referential integrity, PLG keeps the reason.
	if req.OwnerID == nil || strings.TrimSpace(*req.OwnerID) == "" {
		return nil, apierror.Validation(
			"ownerId must name a CS engineer — an organisation cannot be left unassigned once claimed")
	}
	if err := s.repo.Patch(ctx, req); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, req.ID)
}

// ---------------------------------------------------------------------------
// The organisation + platform pairing
// ---------------------------------------------------------------------------

// OrgPlatformService owns the product tab.
//
// Every mutation answers with the reloaded ProductDetail rather than the changed
// fragment: completing "Initiate playbook" moves the run's status, the pairing's
// progress counts and the work queue at once, so returning only the task would
// leave the caller to guess the rest.
type OrgPlatformService interface {
	Get(ctx context.Context, orgID, productCode string) (*domain.ProductDetail, error)
	Patch(ctx context.Context, req domain.PatchOrgPlatformRequest, actor string) (*domain.ProductDetail, error)
	AttachPlaybook(ctx context.Context, req domain.AttachPlaybookRequest, actor string) (*domain.ProductDetail, error)
	DetachRun(ctx context.Context, runID string) (*domain.ProductDetail, error)
	PatchRunTask(ctx context.Context, req domain.PatchRunTaskRequest, actor string) (*domain.ProductDetail, error)
	CreateNote(ctx context.Context, req domain.CreateNoteRequest, actor string) (*domain.ProductDetail, error)
	UpdateNote(ctx context.Context, req domain.UpdateNoteRequest, actor string) (*domain.ProductDetail, error)

	SearchRegistrations(ctx context.Context, req domain.SearchRegistrationsRequest) (domain.SearchRegistrationsResponse, error)
	Acknowledge(ctx context.Context, req domain.AcknowledgeRequest, actor string) (*domain.ProductDetail, error)
}

type orgPlatformService struct {
	repo repository.OrgPlatformRepository
}

// NewOrgPlatformService wires an OrgPlatformService over its repository.
func NewOrgPlatformService(repo repository.OrgPlatformRepository) OrgPlatformService {
	return &orgPlatformService{repo: repo}
}

func (s *orgPlatformService) requirePairing(orgID, productCode string) error {
	if err := validateUUID("organizationId", orgID); err != nil {
		return err
	}
	if strings.TrimSpace(productCode) == "" {
		return apierror.Validation("product is required")
	}
	return nil
}

func (s *orgPlatformService) Get(ctx context.Context, orgID, productCode string) (*domain.ProductDetail, error) {
	if err := s.requirePairing(orgID, productCode); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, orgID, productCode)
}

// reasonNeeded reports whether this request moves one of the three tracked
// axes. Trial end date is not one of them: it is a date copied off a contract,
// not a judgement anyone has to defend.
func reasonNeeded(req domain.PatchOrgPlatformRequest) bool {
	return req.LifecycleStage != nil || req.HealthState != nil || req.SubscriptionTier != nil
}

// blank reports a reason that is missing or only whitespace. A space would
// satisfy NOT NULL and satisfy nobody reading the timeline.
func blank(s *string) bool { return s == nil || strings.TrimSpace(*s) == "" }

func (s *orgPlatformService) Patch(ctx context.Context, req domain.PatchOrgPlatformRequest, actor string) (*domain.ProductDetail, error) {
	if err := s.requirePairing(req.OrganizationID, req.ProductCode); err != nil {
		return nil, err
	}
	if err := validateEnum("lifecycleStage", req.LifecycleStage, domain.ValidLifecycleStage); err != nil {
		return nil, err
	}
	if err := validateEnum("subscriptionTier", req.SubscriptionTier, domain.ValidSubscriptionTier); err != nil {
		return nil, err
	}
	if err := validateEnum("healthState", req.HealthState, domain.ValidHealthState); err != nil {
		return nil, err
	}
	// Every tracked axis needs a reason, health included. It is tempting to
	// exempt health, on the theory that demanding a sentence means engineers
	// leave a customer unmarked rather than write one — but the result is a
	// timeline of entries reading "someone marked this at risk" and
	// nothing else — the one kind of history nobody can act on, and the one an
	// engineer picking the account up on Monday most needs to read. The
	// sentence is cheaper than its absence, so it is now required here, in the
	// slice, and by a NOT NULL on the history column underneath both.
	if reasonNeeded(req) && blank(req.Reason) {
		return nil, apierror.Validation(
			"a lifecycle, health or subscription change needs a reason — say why it moved")
	}
	if err := parseDate("trialEndDate", req.TrialEndDate); err != nil {
		return nil, err
	}
	if err := parseDate("trialExtendedDate", req.TrialExtendedDate); err != nil {
		return nil, err
	}
	// Health counts as a field. It is the second axis, and a request that moves
	// only it is a complete request — an engineer marking a customer at risk has
	// said something, even though nothing about the stage changed.
	if req.LifecycleStage == nil && req.HealthState == nil && req.SubscriptionTier == nil &&
		req.TrialEndDate == nil && !req.ClearTrialEndDate &&
		req.TrialExtendedDate == nil && !req.ClearTrialExtendedDate {
		return nil, apierror.Validation("at least one field must be supplied")
	}
	req.Reason = trimmedOrNil(req.Reason)

	if err := s.repo.Patch(ctx, req, actor); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, req.OrganizationID, req.ProductCode)
}

func (s *orgPlatformService) AttachPlaybook(ctx context.Context, req domain.AttachPlaybookRequest, actor string) (*domain.ProductDetail, error) {
	if err := s.requirePairing(req.OrganizationID, req.ProductCode); err != nil {
		return nil, err
	}
	if err := validateUUID("playbookId", req.PlaybookID); err != nil {
		return nil, err
	}
	if err := s.repo.AttachPlaybook(ctx, req, actor); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, req.OrganizationID, req.ProductCode)
}

func (s *orgPlatformService) DetachRun(ctx context.Context, runID string) (*domain.ProductDetail, error) {
	if err := validateUUID("playbookRunId", runID); err != nil {
		return nil, err
	}
	orgID, code, err := s.repo.DetachRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, orgID, code)
}

// PatchRunTask records a task's value.
//
// The type rules live here because they are about intent rather than storage:
//
//   - Sending a value is what completes the task. That is what "execute the
//     task" means from the UI — type the paragraph, press save, it is done.
//   - A value the task's type cannot hold is rejected here rather than written
//     into a column nothing reads. The database CHECK would refuse it anyway;
//     this turns that into an explanation.
//   - clearValue reopens the task. is_completed follows automatically, because
//     it is generated from the value.
func (s *orgPlatformService) PatchRunTask(ctx context.Context, req domain.PatchRunTaskRequest, actor string) (*domain.ProductDetail, error) {
	if err := validateUUID("taskId", req.ID); err != nil {
		return nil, err
	}
	req.TextValue = trimmedOrNil(req.TextValue)

	supplied := req.BoolValue != nil || req.NumberValue != nil ||
		req.TextValue != nil || req.CheckedCodes != nil
	if !supplied && !req.ClearValue {
		return nil, apierror.Validation("supply a value, or clearValue to reopen the task")
	}

	shape, err := s.repo.RunTaskShape(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	switch shape.ValueType {
	case domain.ValueBoolean:
		if req.NumberValue != nil || req.TextValue != nil || req.CheckedCodes != nil {
			return nil, apierror.Validation("this task is a tick box; send boolValue")
		}
	case domain.ValueNumber:
		if req.BoolValue != nil || req.TextValue != nil || req.CheckedCodes != nil {
			return nil, apierror.Validation("this task holds a number; send numberValue")
		}
	case domain.ValueString:
		if req.BoolValue != nil || req.NumberValue != nil || req.CheckedCodes != nil {
			return nil, apierror.Validation("this task holds text; send textValue")
		}
	case domain.ValueSingleSelect:
		// SINGLE_SELECT stores the chosen option's CODE in the text column, so
		// textValue is the field to send — the same storage as STRING, with the
		// offered options constraining what may go in it. Without this case a
		// SINGLE_SELECT task validated as nothing at all: any string was stored
		// as the answer, including one the task never offered.
		if req.BoolValue != nil || req.NumberValue != nil || req.CheckedCodes != nil {
			return nil, apierror.Validation("this task takes one of its offered answers; send textValue")
		}
		if err := normalizeSelectedCode(req.TextValue, shape.OptionCodes); err != nil {
			return nil, err
		}
	case domain.ValueChecklist:
		if req.BoolValue != nil || req.NumberValue != nil || req.TextValue != nil {
			return nil, apierror.Validation("this task is a checklist; send checkedCodes")
		}
		if err := normalizeCheckedCodes(req.CheckedCodes, shape.OptionCodes); err != nil {
			return nil, err
		}
	}

	// An empty set is the same intent as clearing: no reason ticked is no answer.
	if req.CheckedCodes != nil && len(*req.CheckedCodes) == 0 {
		req.CheckedCodes = nil
		req.ClearValue = true
	}

	// A tick box set to false is indistinguishable from unanswered — there is no
	// stored "no" in this model — so treat it as reopening rather than writing a
	// FALSE nobody can tell apart from a blank.
	if shape.ValueType == domain.ValueBoolean && req.BoolValue != nil && !*req.BoolValue {
		req.BoolValue = nil
		req.ClearValue = true
	}

	orgID, code, err := s.repo.PatchRunTask(ctx, req, actor)
	if err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, orgID, code)
}

// normalizeCheckedCodes tidies the submitted reasons and refuses any the task
// does not offer.
//
// The database cannot check this — an array element has no foreign key to the
// options beside it — so an unrecognised code would otherwise be stored happily
// and then never match anything in an analysis. Better to refuse it by name.
// normalizeSelectedCode upper-cases and trims a SINGLE_SELECT answer in place,
// and refuses one the task does not offer.
//
// The database cannot do this check: its constraint only requires the text be
// non-blank, because the offered options live on the task row as JSONB rather
// than in a foreign key. So this is the only thing standing between the API and
// an answer nobody can interpret.
func normalizeSelectedCode(value *string, offered []string) error {
	if value == nil {
		return nil
	}
	code := strings.ToUpper(strings.TrimSpace(*value))
	if code == "" {
		// An empty answer is the same intent as clearing, handled by the caller.
		*value = ""
		return nil
	}
	for _, c := range offered {
		if code == c {
			*value = code
			return nil
		}
	}
	return apierror.Validation("this task does not offer the answer " + code +
		" (it offers: " + strings.Join(offered, ", ") + ")")
}

func normalizeCheckedCodes(codes *[]string, offered []string) error {
	if codes == nil {
		return nil
	}
	valid := make(map[string]bool, len(offered))
	for _, c := range offered {
		valid[c] = true
	}

	seen := make(map[string]bool, len(*codes))
	out := make([]string, 0, len(*codes))
	for _, raw := range *codes {
		code := strings.ToUpper(strings.TrimSpace(raw))
		if code == "" {
			continue
		}
		if !valid[code] {
			return apierror.Validation("this task does not offer the reason " + code +
				" (it offers: " + strings.Join(offered, ", ") + ")")
		}
		if seen[code] {
			continue
		}
		seen[code] = true
		out = append(out, code)
	}
	*codes = out
	return nil
}

func (s *orgPlatformService) CreateNote(ctx context.Context, req domain.CreateNoteRequest, actor string) (*domain.ProductDetail, error) {
	if err := s.requirePairing(req.OrganizationID, req.ProductCode); err != nil {
		return nil, err
	}
	req.Body = strings.TrimSpace(req.Body)
	if req.Body == "" {
		return nil, apierror.Validation("body is required")
	}
	if err := s.repo.CreateNote(ctx, req, actor); err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, req.OrganizationID, req.ProductCode)
}

// UpdateNote corrects the wording of a note.
//
// The rule that matters is in the repository, where the current author is
// already in hand: only its author may edit a note. What is here is the same
// shape as CreateNote, because an edit has to satisfy everything a new note
// does — an edit that blanks the body is not an edit, it is a delete, and
// deleting is not offered.
func (s *orgPlatformService) UpdateNote(ctx context.Context, req domain.UpdateNoteRequest, actor string) (*domain.ProductDetail, error) {
	if err := validateUUID("noteId", req.ID); err != nil {
		return nil, err
	}
	req.Body = strings.TrimSpace(req.Body)
	if req.Body == "" {
		return nil, apierror.Validation("body is required")
	}
	orgID, code, err := s.repo.UpdateNote(ctx, req, actor)
	if err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, orgID, code)
}

func (s *orgPlatformService) SearchRegistrations(ctx context.Context, req domain.SearchRegistrationsRequest) (domain.SearchRegistrationsResponse, error) {
	if err := validateSearchFilters(&req.Filters); err != nil {
		return domain.SearchRegistrationsResponse{}, err
	}
	normalizePagination(&req.Pagination)

	items, total, err := s.repo.SearchRegistrations(ctx, req)
	if err != nil {
		return domain.SearchRegistrationsResponse{}, err
	}
	return domain.SearchRegistrationsResponse{
		Registrations: items,
		Total:         total,
		Limit:         req.Pagination.Limit,
		Offset:        req.Pagination.Offset,
	}, nil
}

func (s *orgPlatformService) Acknowledge(ctx context.Context, req domain.AcknowledgeRequest, actor string) (*domain.ProductDetail, error) {
	if err := validateUUID("orgPlatformId", req.OrgPlatformID); err != nil {
		return nil, err
	}
	if err := s.repo.Acknowledge(ctx, req, actor); err != nil {
		return nil, err
	}
	orgID, code, err := s.repo.LocatePairing(ctx, req.OrgPlatformID)
	if err != nil {
		return nil, err
	}
	return s.repo.Get(ctx, orgID, code)
}

// ---------------------------------------------------------------------------
// Analytics
// ---------------------------------------------------------------------------

// AnalyticsService serves the dashboard and the work queue.
type AnalyticsService interface {
	Dashboard(ctx context.Context, rng domain.AnalyticsRange) (*domain.DashboardAnalytics, error)
	WorkQueue(ctx context.Context, f domain.WorkQueueFilters) (*domain.WorkQueueResponse, error)
}

type analyticsService struct {
	repo repository.AnalyticsRepository
}

// NewAnalyticsService wires an AnalyticsService over its repository.
func NewAnalyticsService(repo repository.AnalyticsRepository) AnalyticsService {
	return &analyticsService{repo: repo}
}

func (s *analyticsService) Dashboard(ctx context.Context, rng domain.AnalyticsRange) (*domain.DashboardAnalytics, error) {
	if rng.From != nil && rng.To != nil && rng.To.Before(*rng.From) {
		return nil, apierror.Validation("to must not be earlier than from")
	}
	return s.repo.Dashboard(ctx, rng)
}

func (s *analyticsService) WorkQueue(ctx context.Context, f domain.WorkQueueFilters) (*domain.WorkQueueResponse, error) {
	// Owners are carried as ids; the "mine" shortcut has already been resolved
	// to one by the handler.
	for i := range f.OwnerIDs {
		f.OwnerIDs[i] = strings.TrimSpace(f.OwnerIDs[i])
	}
	for _, id := range f.OrganizationIDs {
		if err := validateUUID("organizationId", id); err != nil {
			return nil, err
		}
	}
	for _, id := range f.PlaybookIDs {
		if err := validateUUID("playbookId", id); err != nil {
			return nil, err
		}
	}
	if err := validateEnums("stage", f.LifecycleStages, domain.ValidLifecycleStage); err != nil {
		return nil, err
	}
	if err := validateEnums("reason", f.Reasons, domain.ValidQueueReason); err != nil {
		return nil, err
	}
	return s.repo.WorkQueue(ctx, f)
}
