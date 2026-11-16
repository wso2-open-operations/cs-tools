package service

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// OrganizationService serves the organisation list and the overview tab.
type OrganizationService interface {
	Search(ctx context.Context, req domain.SearchOrganizationsRequest) (domain.SearchOrganizationsResponse, error)
	Get(ctx context.Context, id string) (*domain.OrganizationDetail, error)
	Patch(ctx context.Context, req domain.PatchOrganizationRequest, actorID string) (*domain.OrganizationDetail, error)
	Aggregate(ctx context.Context, req domain.AggregateOrganizationsRequest) (domain.AggregateResponse, error)
}

type organizationService struct {
	repo repository.OrganizationRepository
}

// NewOrganizationService wires an OrganizationService over its repository.
func NewOrganizationService(repo repository.OrganizationRepository) OrganizationService {
	return &organizationService{repo: repo}
}

// validateOrganizationFilters checks the enums and the date range.
//
// The owner filter carries ids, so it is checked as UUIDs rather than passed
// through as free text. Country names are free text, each one non-blank and
// at most maxPlgCountryNameLength characters, and never with countryUnknown
// (no organisation has both a country and none). Owner emails each have
// text on both sides of an @ and are at most maxPlgOwnerEmailLength
// characters, and never come with unowned. The playbook task and answer
// codes are 1 to maxPlgPlaybookCodeLength characters; playbookAnswers and
// playbookUnanswered each need playbookTask and never come together.
func validateOrganizationFilters(f *domain.OrganizationSearchFilters) error {
	if err := validateEnums("lifecycleStage", f.LifecycleStages, domain.ValidLifecycleStage); err != nil {
		return err
	}
	if err := validateEnums("subscriptionTier", f.SubscriptionTiers, domain.ValidSubscriptionTier); err != nil {
		return err
	}
	if err := plgValidateUUIDs("filters.ownerIds", f.OwnerIDs); err != nil {
		return err
	}
	if f.RegisteredFrom != nil && f.RegisteredTo != nil && f.RegisteredTo.Before(*f.RegisteredFrom) {
		return &apierror.ValidationError{Msg: "registeredTo must not be earlier than registeredFrom"}
	}
	for _, n := range f.CountryNames {
		if n = strings.TrimSpace(n); n == "" || utf8.RuneCountInString(n) > maxPlgCountryNameLength {
			return &apierror.ValidationError{Msg: fmt.Sprintf("filters.countryNames entries must be 1 to %d characters", maxPlgCountryNameLength)}
		}
	}
	if len(f.CountryNames) > 0 && f.CountryUnknown != nil && *f.CountryUnknown {
		return &apierror.ValidationError{Msg: "filters.countryNames and filters.countryUnknown cannot be combined"}
	}
	for _, e := range f.OwnerEmails {
		e = strings.TrimSpace(e)
		if local, host, ok := strings.Cut(e, "@"); !ok || local == "" || host == "" || utf8.RuneCountInString(e) > maxPlgOwnerEmailLength {
			return &apierror.ValidationError{Msg: fmt.Sprintf("filters.ownerEmails entries must be email addresses of at most %d characters", maxPlgOwnerEmailLength)}
		}
	}
	if len(f.OwnerEmails) > 0 && f.Unowned != nil && *f.Unowned {
		return &apierror.ValidationError{Msg: "filters.ownerEmails and filters.unowned cannot be combined"}
	}
	return validatePlaybookFilters(f)
}

// validatePlaybookFilters checks the playbook answer filters.
func validatePlaybookFilters(f *domain.OrganizationSearchFilters) error {
	unanswered := f.PlaybookUnanswered != nil && *f.PlaybookUnanswered
	task := strings.TrimSpace(f.PlaybookTask)
	if f.PlaybookTask != "" && (task == "" || utf8.RuneCountInString(task) > maxPlgPlaybookCodeLength) {
		return &apierror.ValidationError{Msg: fmt.Sprintf("filters.playbookTask must be 1 to %d characters", maxPlgPlaybookCodeLength)}
	}
	for _, c := range f.PlaybookAnswers {
		if c = strings.TrimSpace(c); c == "" || utf8.RuneCountInString(c) > maxPlgPlaybookCodeLength {
			return &apierror.ValidationError{Msg: fmt.Sprintf("filters.playbookAnswers entries must be 1 to %d characters", maxPlgPlaybookCodeLength)}
		}
	}
	if task == "" && (len(f.PlaybookAnswers) > 0 || unanswered) {
		return &apierror.ValidationError{Msg: "filters.playbookAnswers and filters.playbookUnanswered need filters.playbookTask"}
	}
	if len(f.PlaybookAnswers) > 0 && unanswered {
		return &apierror.ValidationError{Msg: "filters.playbookAnswers and filters.playbookUnanswered cannot be combined"}
	}
	return nil
}

// maxPlgCountryNameLength is the longest country name a filter takes.
const maxPlgCountryNameLength = 100

// maxPlgOwnerEmailLength is the longest owner email address a filter takes,
// the longest an email address can be.
const maxPlgOwnerEmailLength = 254

// maxPlgPlaybookCodeLength is the longest playbook task or answer code a
// filter takes.
const maxPlgPlaybookCodeLength = 100

// maxPlgAggregateGroups is the most groups an organisation aggregate returns
// on its own; a larger maxGroups is refused.
const maxPlgAggregateGroups = 50

// Aggregate counts the organisations the search's filters match per group
// (POST /plg/organizations/aggregate): groupBy "owner" or "country".
func (s *organizationService) Aggregate(ctx context.Context, req domain.AggregateOrganizationsRequest) (domain.AggregateResponse, error) {
	if req.GroupBy != "owner" && req.GroupBy != "country" {
		return domain.AggregateResponse{}, &apierror.ValidationError{Msg: fmt.Sprintf("groupBy must be %q or %q, got %q", "owner", "country", req.GroupBy)}
	}
	if err := validatePlgMaxGroups(req.MaxGroups); err != nil {
		return domain.AggregateResponse{}, err
	}
	if err := validateOrganizationFilters(&req.Filters); err != nil {
		return domain.AggregateResponse{}, err
	}
	if req.GroupBy == "country" {
		return s.repo.AggregateByCountry(ctx, req.Filters, req.MaxGroups)
	}
	return s.repo.AggregateByOwner(ctx, req.Filters, req.MaxGroups)
}

// validatePlgMaxGroups refuses a maxGroups outside 0 (every group) to
// maxPlgAggregateGroups.
func validatePlgMaxGroups(maxGroups int) error {
	if maxGroups < 0 || maxGroups > maxPlgAggregateGroups {
		return &apierror.ValidationError{Msg: fmt.Sprintf("maxGroups must be between 0 and %d", maxPlgAggregateGroups)}
	}
	return nil
}

func (s *organizationService) Search(ctx context.Context, req domain.SearchOrganizationsRequest) (domain.SearchOrganizationsResponse, error) {
	if err := validateOrganizationFilters(&req.Filters); err != nil {
		return domain.SearchOrganizationsResponse{}, err
	}
	plgNormalizePagination(&req.Pagination)

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

// Patch sets the organisation's CS owner, and does not judge the value.
//
// WHAT IS DELIBERATELY ABSENT. Nothing here refuses to clear an owner, even
// though an ownerless organisation is a hole — the work queue reads ownership
// to decide whose queue a pairing sits in. That is a PLG lifecycle rule, and
// lifecycle rules live in the BFF (see plg-docs/ENTITY-SERVICE-CONTRACT.md,
// S1), which rejects an empty ownerId with a 400 before this is ever called.
//
// So this will happily write a NULL owner if asked. That is not an oversight:
// entity-service's job is to store what it is told and keep referential
// integrity, and a later caller with a legitimate reason to unassign should not
// have to fight a rule that belongs to someone else's product.
func (s *organizationService) Patch(ctx context.Context, req domain.PatchOrganizationRequest, actorID string) (*domain.OrganizationDetail, error) {
	// Same rule as every other attributed write: an owner change that records
	// nobody is worse than one that is refused.
	if err := validateActor(actorID); err != nil {
		return nil, err
	}
	if err := validateUUID("organizationId", req.ID); err != nil {
		return nil, err
	}
	if req.OwnerID != nil && *req.OwnerID != "" {
		if err := validateUUID("ownerId", *req.OwnerID); err != nil {
			return nil, err
		}
	}
	err := s.repo.Patch(ctx, req, actorID)
	plgAudit(ctx, "set owner", actorID, err, "organizationId", req.ID)
	if err != nil {
		return nil, err
	}
	// Returns the reloaded detail: the caller's response is the whole
	// organisation, and reading it back is what makes the owner's name and email
	// correct without the caller resolving them.
	return s.repo.Get(ctx, req.ID)
}
