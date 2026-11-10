package service

import (
	"context"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// PairingService serves the product tab and the registrations panel.
//
// Reads here, writes in plg_pairing_writes.go. The split is not cosmetic: every
// write carries a precondition and answers with a row count, never with a
// meaning, because what a failed precondition means is a PLG rule and PLG's
// rules are the BFF's. See plg-docs/ENTITY-SERVICE-CONTRACT.md.
type PairingService interface {
	Get(ctx context.Context, orgID, productCode string) (*domain.ProductDetail, error)
	SearchRegistrations(ctx context.Context, req domain.SearchRegistrationsRequest) (domain.SearchRegistrationsResponse, error)
	RegistrationVolumeTrend(ctx context.Context, req domain.RegistrationVolumeTrendRequest) (domain.CaseVolumeTrendResponse, error)

	// The writes live in plg_pairing_writes.go. Embedded rather than listed
	// again so the two files cannot drift apart.
	PairingWriter
}

type pairingService struct {
	repo repository.OrgPlatformRepository
}

// NewPairingService wires a PairingService over its repository.
func NewPairingService(repo repository.OrgPlatformRepository) PairingService {
	return &pairingService{repo: repo}
}

func (s *pairingService) Get(ctx context.Context, orgID, productCode string) (*domain.ProductDetail, error) {
	if err := validateUUID("organizationId", orgID); err != nil {
		return nil, err
	}
	// productCode is matched against either the code or the id by the query, so
	// it is not validated as a UUID here — "API_PLATFORM" is the common case.
	return s.repo.Get(ctx, orgID, productCode)
}

func (s *pairingService) SearchRegistrations(ctx context.Context, req domain.SearchRegistrationsRequest) (domain.SearchRegistrationsResponse, error) {
	if err := validateOrganizationFilters(&req.Filters); err != nil {
		return domain.SearchRegistrationsResponse{}, err
	}
	plgNormalizePagination(&req.Pagination)

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

// RegistrationVolumeTrend counts the registrations req's filters match per
// day, week or month of req's window (POST /plg/registrations/volume-trend).
func (s *pairingService) RegistrationVolumeTrend(ctx context.Context, req domain.RegistrationVolumeTrendRequest) (domain.CaseVolumeTrendResponse, error) {
	return s.registrationVolumeTrend(ctx, req, time.Now().UTC())
}

// registrationVolumeTrend is RegistrationVolumeTrend on a given today, for
// tests. The window is checked and resolved exactly as a case trend's
// (resolveCaseVolumeTrendWindow), and the days are grouped the same way
// (volumeTrendPoints), so the two trends draw alike.
func (s *pairingService) registrationVolumeTrend(ctx context.Context, req domain.RegistrationVolumeTrendRequest, today time.Time) (domain.CaseVolumeTrendResponse, error) {
	if err := validateOrganizationFilters(&req.Filters); err != nil {
		return domain.CaseVolumeTrendResponse{}, err
	}
	window, err := resolveCaseVolumeTrendWindow(domain.CaseVolumeTrendRequest{
		Days: req.Days, StartDate: req.StartDate, EndDate: req.EndDate, Bucket: req.Bucket,
	}, today)
	if err != nil {
		return domain.CaseVolumeTrendResponse{}, err
	}
	counts, err := s.repo.RegistrationVolumeTrend(ctx,
		domain.SearchRegistrationsRequest{UnacknowledgedOnly: req.UnacknowledgedOnly, Filters: req.Filters},
		window.start, window.end)
	if err != nil {
		return domain.CaseVolumeTrendResponse{}, err
	}
	days := int(window.end.Sub(window.start).Hours()/24) + 1
	allDays := make([]string, days)
	for i := range allDays {
		allDays[i] = window.start.AddDate(0, 0, i).Format("2006-01-02")
	}
	resp := domain.CaseVolumeTrendResponse{
		Days: days,
		Series: []domain.CaseVolumeTrendSeries{
			{Label: "Registered", Points: volumeTrendPoints(allDays, window.bucket, func(d string) int { return counts[d] })},
		},
	}
	if window.fixed {
		resp.StartDate = window.start.Format("2006-01-02")
		resp.EndDate = window.end.Format("2006-01-02")
		resp.Bucket = window.bucket
	}
	return resp, nil
}
