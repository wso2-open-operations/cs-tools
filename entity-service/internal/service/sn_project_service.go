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

// Package service is declared in interfaces.go.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// snProjectsResponse mirrors the Choreo POST /projects/search response.
type snProjectsResponse struct {
	Projects     []snProject `json:"projects"`
	TotalRecords int         `json:"totalRecords"`
	Offset       int         `json:"offset"`
	Limit        int         `json:"limit"`
}

// snProjectClosureFields groups the closure-related fields shared by
// snProject and snProjectDetailsResponse; ServiceNow specific, no Postgres
// equivalent. Embedded anonymously; JSON field names are unaffected.
type snProjectClosureFields struct {
	ClosureState                    *string         `json:"closureState"`
	EndDateClosureState             *string         `json:"endDateClosureState"`
	InvoiceDueDateClosureState      *string         `json:"invoiceDueDateClosureState"`
	ComplianceViolationClosureState *string         `json:"complianceViolationClosureState"`
	ComplianceViolationDate         *string         `json:"complianceViolationDate"`
	SuspensionProcessState          json.RawMessage `json:"suspensionProcessState"`
}

type snProject struct {
	ID        string                  `json:"id"`
	Name      string                  `json:"name"`
	Key       string                  `json:"key"`
	Type      snProjectType           `json:"type"`
	StartDate *string                 `json:"startDate"`
	EndDate   string                  `json:"endDate"`
	CreatedOn string                  `json:"createdOn"`
	Account   snProjectSummaryAccount `json:"account"`
	// ActiveCasesCount is required (not a pointer) because the portal's
	// ProjectListItem types it as a non-optional number.
	ActiveCasesCount int `json:"activeCasesCount"`
	snProjectClosureFields
	// OnboardingStatus/OnboardingOwner support onboarding-scoped dashboard
	// queries. Nil when the project has no onboarding engagement tracked.
	OnboardingStatus *string      `json:"onboardingStatus"`
	OnboardingOwner  *snPersonRef `json:"onboardingOwner"`
}

type snProjectType struct {
	Name string `json:"name"`
}

// snProjectSummaryAccount is the compact account reference embedded in each
// search result. ID/Name are empty when the project has no linked account.
// Region/SubRegion/ArrToday support onboarding-scoped dashboard queries and
// are nil when not tracked upstream.
type snProjectSummaryAccount struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Region    *string `json:"region"`
	SubRegion *string `json:"subRegion"`
	ArrToday  *string `json:"arrToday"`
	// Partner is the linked account's raw ServiceNow customer_account.partner passthrough,
	// merged in by the Ballerina entity-service. Named into domain.ProjectSearchAccountRef.IsPartner.
	Partner *bool `json:"partner"`
}

// snSearchProjectsPayload is the Choreo POST /projects/search request body.
type snSearchProjectsPayload struct {
	Filters    snProjectFilters    `json:"filters"`
	Pagination snProjectPagination `json:"pagination"`
}

type snProjectFilters struct {
	SearchQuery   string `json:"searchQuery,omitempty"`
	ClosureStatus string `json:"closureStatus,omitempty"`
	EndDateFrom   string `json:"endDateFrom,omitempty"`
	EndDateTo     string `json:"endDateTo,omitempty"`
	SortBy        string `json:"sortBy,omitempty"`
	SortOrder     string `json:"sortOrder,omitempty"`
	AccountID     string `json:"accountId,omitempty"`
	// OnboardingStatus/ArrTodayGte/SubRegion support onboarding-scoped
	// dashboard queries.
	OnboardingStatus []string `json:"onboardingStatus,omitempty"`
	ArrTodayGte      string   `json:"arrTodayGte,omitempty"`
	SubRegion        string   `json:"subRegion,omitempty"`
}

type snProjectPagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// snCreatedOnLayout is the datetime format used by the Choreo API.
const snCreatedOnLayout = "2006-01-02 15:04:05"

// snAltCreatedOnLayout is an alternate datetime format ServiceNow occasionally
// returns instead of snCreatedOnLayout for the same fields (createdOn, updatedOn,
// resolvedOn, autoclosureStateTime, ...). Root cause (confirmed via Choreo prod
// logs): an SN-side script include reads these fields with
// GlideRecord.getDisplayValue(), which renders per session locale, instead of
// getValue(), which is always canonical ISO. Fixing that is out of scope here
// (the SN scoped app is shared with the live customer portal) — see
// parseSNDateTime, the Go-side tolerant parser that works around it.
const snAltCreatedOnLayout = "01-02-2006 15:04:05"

// snDateLayout is the date-only format used by the Choreo API for start/end dates.
const snDateLayout = "2006-01-02"

// parseSNDateTime parses a ServiceNow datetime string, tolerating both formats
// observed in production. It tries snCreatedOnLayout first — the overwhelmingly
// common case, and the fast path — and only on failure falls back to
// snAltCreatedOnLayout.
//
// Before this helper existed, a bare time.Parse(snCreatedOnLayout, ...) on one
// of these fields would return an error, and callers turned that into a 500 —
// even though the underlying SN write (case update, new comment) had already
// succeeded. See the case/comment PATCH and POST paths in sn_case_service.go
// and sn_comment_service.go.
//
// callSite and field identify where the value came from, for the WARN logged
// when the fallback format is the one that actually matched: this keeps the
// upstream format drift visible/monitorable instead of silently masked. If
// neither format parses, the original snCreatedOnLayout error is returned
// unchanged — a genuinely malformed value still fails the request.
func parseSNDateTime(ctx context.Context, callSite, field, value string) (time.Time, error) {
	t, err := time.Parse(snCreatedOnLayout, value)
	if err == nil {
		return t, nil
	}
	if alt, altErr := time.Parse(snAltCreatedOnLayout, value); altErr == nil {
		slog.WarnContext(ctx, "sn date field in alternate MM-DD-YYYY format, applied fallback parse",
			"callSite", callSite, "field", field)
		return alt, nil
	}
	return time.Time{}, err
}

// isProjectContractEnded reports whether endDate's day has fully elapsed as
// of now — mirroring apps/customer-portal/webapp/src/utils/permission.ts's
// isProjectContractEnded exactly (end-of-day UTC comparison, strictly after),
// so a project the customer portal itself treats as contract-ended is
// treated the same way here. endDate is nil when the backing data source has
// no end date recorded, in which case the contract is never considered
// ended.
func isProjectContractEnded(endDate *time.Time, now time.Time) bool {
	if endDate == nil {
		return false
	}
	endOfDay := time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 23, 59, 59, 999000000, time.UTC)
	return now.After(endOfDay)
}

type snProjectService struct {
	client     *integrationservice.Client
	pgFallback ProjectService
}

// NewSNProjectService constructs a ProjectService backed by the Choreo API.
// pgFallback is used for GetProjectByID (no SN single-project endpoint).
func NewServiceNowProjectService(client *integrationservice.Client, pgFallback ProjectService) ProjectService {
	return &snProjectService{client: client, pgFallback: pgFallback}
}

// SearchProjects implements ProjectService. It calls the Choreo API, parses the
// response, and maps each item to domain.ProjectView so the shape is identical
// to the Postgres path.
func (s *snProjectService) SearchProjects(ctx context.Context, req domain.SearchProjectsRequest) (domain.SearchProjectsResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchProjectsResponse{}, err
	}
	if err := validateSearchQuery(req.SearchQuery); err != nil {
		return domain.SearchProjectsResponse{}, err
	}
	if err := validateProjectSearchFilters(req); err != nil {
		return domain.SearchProjectsResponse{}, err
	}
	var accountSysid string
	if req.AccountID != "" {
		if err := validateUUIDs("accountId", []string{req.AccountID}); err != nil {
			return domain.SearchProjectsResponse{}, err
		}
		accountSysid = uuidToSysid(req.AccountID)
	}

	token := middleware.UserIDTokenFromContext(ctx)

	if len(req.ExcludeClosureStates) == 0 && len(req.ExcludeSubscriptionTypes) == 0 && len(req.ExcludeProjectKeys) == 0 {
		views, total, err := s.fetchProjectsPage(ctx, req, accountSysid, token, req.Pagination.Limit, req.Pagination.Offset)
		if err != nil {
			return domain.SearchProjectsResponse{}, err
		}
		return domain.SearchProjectsResponse{
			Projects: views,
			Total:    total,
			Limit:    req.Pagination.Limit,
			Offset:   req.Pagination.Offset,
			HasMore:  req.Pagination.Offset+len(views) < total,
		}, nil
	}

	// ServiceNow's projects/search endpoint has no filter parameter for
	// excluding a set of closure states, subscription types, or project keys
	// (unlike ClosureStatus's own single-value include filter above), so this
	// pages through every match with a bounded loop, filters in Go, then
	// applies the caller's requested offset/limit over the filtered result.
	filtered, err := s.fetchAllProjectsFiltered(ctx, req, accountSysid, token)
	if err != nil {
		return domain.SearchProjectsResponse{}, err
	}

	total := len(filtered)
	start := req.Pagination.Offset
	if start > total {
		start = total
	}
	end := start + req.Pagination.Limit
	if end > total {
		end = total
	}

	return domain.SearchProjectsResponse{
		Projects: filtered[start:end],
		Total:    total,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
		HasMore:  end < total,
	}, nil
}

// maxExcludeFilterPages bounds fetchAllProjectsFiltered's paging loop against
// a wrong/always-true upstream hasMore signal, mirroring the same safety-bound
// convention the webapp's own paged-query hooks use.
const maxExcludeFilterPages = 200

// snExcludeFilterPageSize is the page size fetchAllProjectsFiltered uses
// internally, independent of the caller's own requested limit — that limit
// applies to the filtered result, not to how many rows are fetched from
// ServiceNow per round trip. Capped at maxLimit (50): that's the backing data
// source's own hard ceiling per page (see maxLimit's doc comment in
// user_service.go), not just this service's own default.
const snExcludeFilterPageSize = maxLimit

// fetchAllProjectsFiltered pages through every ServiceNow match for req
// (ignoring req.Pagination — the caller applies that to the returned slice)
// and returns the subset that matches none of ExcludeClosureStates,
// ExcludeSubscriptionTypes, or ExcludeProjectKeys.
func (s *snProjectService) fetchAllProjectsFiltered(ctx context.Context, req domain.SearchProjectsRequest, accountSysid, token string) ([]domain.ProjectView, error) {
	excludeClosure := make(map[string]struct{}, len(req.ExcludeClosureStates))
	for _, v := range req.ExcludeClosureStates {
		excludeClosure[v] = struct{}{}
	}
	excludeType := make(map[domain.SubscriptionType]struct{}, len(req.ExcludeSubscriptionTypes))
	for _, v := range req.ExcludeSubscriptionTypes {
		excludeType[v] = struct{}{}
	}
	excludeKey := make(map[string]struct{}, len(req.ExcludeProjectKeys))
	for _, v := range req.ExcludeProjectKeys {
		excludeKey[v] = struct{}{}
	}

	var filtered []domain.ProjectView
	offset := 0
	for page := 0; page < maxExcludeFilterPages; page++ {
		views, total, err := s.fetchProjectsPage(ctx, req, accountSysid, token, snExcludeFilterPageSize, offset)
		if err != nil {
			return nil, err
		}
		for _, v := range views {
			if v.ClosureState != nil {
				if _, excluded := excludeClosure[*v.ClosureState]; excluded {
					continue
				}
			}
			if _, excluded := excludeType[v.SubscriptionType]; excluded {
				continue
			}
			if _, excluded := excludeKey[v.Key]; excluded {
				continue
			}
			filtered = append(filtered, v)
		}
		offset += len(views)
		if offset >= total || len(views) == 0 {
			return filtered, nil
		}
	}
	// The loop above ran out of pages before exhausting every upstream match
	// (offset never reached total). Returning `filtered` here would silently
	// report a truncated slice as the complete, authoritative result — the
	// caller derives Total and HasMore directly from its length, so a caller
	// resolving an announcement audience could under-count real recipients
	// and never know. Fail loudly instead of guessing.
	return nil, &apierror.ServiceUnavailableError{Msg: fmt.Sprintf(
		"too many matching projects to apply excludeClosureStates/excludeSubscriptionTypes/excludeProjectKeys safely (exceeded %d upstream pages of %d) — narrow the search with an additional filter",
		maxExcludeFilterPages, snExcludeFilterPageSize,
	)}
}

// fetchProjectsPage calls ServiceNow's projects/search endpoint for one page
// and maps the response to domain.ProjectView, returning the page alongside
// ServiceNow's own reported total match count. limit/offset are passed
// separately from req.Pagination so fetchAllProjectsFiltered can page with its
// own internal page size independent of the caller's requested window.
func (s *snProjectService) fetchProjectsPage(ctx context.Context, req domain.SearchProjectsRequest, accountSysid, token string, limit, offset int) ([]domain.ProjectView, int, error) {
	payload := snSearchProjectsPayload{
		Filters: snProjectFilters{
			SearchQuery:      req.SearchQuery,
			ClosureStatus:    req.ClosureStatus,
			EndDateFrom:      req.EndDateFrom,
			EndDateTo:        req.EndDateTo,
			SortBy:           req.SortBy,
			SortOrder:        req.SortOrder,
			AccountID:        accountSysid,
			OnboardingStatus: req.OnboardingStatus,
			ArrTodayGte:      req.ArrTodayGte,
			SubRegion:        req.SubRegion,
		},
		Pagination: snProjectPagination{Limit: limit, Offset: offset},
	}
	raw, err := s.client.Post(ctx, "/projects/search", token, payload)
	if err != nil {
		return nil, 0, err
	}

	var snResp snProjectsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return nil, 0, fmt.Errorf("sn projects: parse response: %w", err)
	}

	views := make([]domain.ProjectView, 0, len(snResp.Projects))
	for _, p := range snResp.Projects {
		createdOn, err := time.Parse(snCreatedOnLayout, p.CreatedOn)
		if err != nil {
			return nil, 0, fmt.Errorf("sn projects: parse createdOn %q: %w", p.CreatedOn, err)
		}
		subType, knownType := snTypeNameToSubscriptionType(p.Type.Name)
		if !knownType {
			slog.WarnContext(ctx, "sn projects: unrecognized subscription type from ServiceNow",
				"projectID", p.ID, "typeName", p.Type.Name)
		}
		var startDate *time.Time
		if p.StartDate != nil && *p.StartDate != "" {
			parsed, err := time.Parse(snDateLayout, *p.StartDate)
			if err != nil {
				return nil, 0, fmt.Errorf("sn projects: parse startDate %q: %w", *p.StartDate, err)
			}
			startDate = &parsed
		}
		var endDate *time.Time
		if p.EndDate != "" {
			parsed, err := time.Parse(snDateLayout, p.EndDate)
			if err != nil {
				return nil, 0, fmt.Errorf("sn projects: parse endDate %q: %w", p.EndDate, err)
			}
			endDate = &parsed
		}
		var account *domain.ProjectSearchAccountRef
		if p.Account.ID != "" {
			account = &domain.ProjectSearchAccountRef{
				ID:        sysidToUUID(p.Account.ID),
				Name:      p.Account.Name,
				Region:    p.Account.Region,
				SubRegion: p.Account.SubRegion,
				ArrToday:  p.Account.ArrToday,
				IsPartner: p.Account.Partner,
			}
		}
		var onboardingOwner *domain.PersonRef
		if p.OnboardingOwner != nil && p.OnboardingOwner.ID != "" {
			onboardingOwner = &domain.PersonRef{
				ID:    sysidToUUID(p.OnboardingOwner.ID),
				Name:  p.OnboardingOwner.Name,
				Email: nilIfEmpty(p.OnboardingOwner.Email),
			}
		}
		views = append(views, domain.ProjectView{
			ID:               sysidToUUID(p.ID),
			Name:             p.Name,
			Key:              p.Key,
			SubscriptionType: subType,
			StartDate:        startDate,
			EndDate:          endDate,
			CreatedOn:        createdOn,
			Account:          account,
			ActiveCasesCount: p.ActiveCasesCount,
			ProjectClosureFields: domain.ProjectClosureFields{
				ClosureState:                    p.ClosureState,
				EndDateClosureState:             p.EndDateClosureState,
				InvoiceDueDateClosureState:      p.InvoiceDueDateClosureState,
				ComplianceViolationClosureState: p.ComplianceViolationClosureState,
				ComplianceViolationDate:         p.ComplianceViolationDate,
				SuspensionProcessState:          p.SuspensionProcessState,
			},
			OnboardingStatus: p.OnboardingStatus,
			OnboardingOwner:  onboardingOwner,
		})
	}

	return views, snResp.TotalRecords, nil
}

// snProjectDetailsResponse mirrors the Choreo GET /projects/{id} response.
type snProjectDetailsResponse struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	Key       string           `json:"key"`
	SfID      string           `json:"sfId"`
	CreatedOn string           `json:"createdOn"`
	StartDate string           `json:"startDate"`
	EndDate   string           `json:"endDate"`
	Type      snProjectType    `json:"type"`
	Account   snProjectAccount `json:"account"`

	// Query/onboarding entitlement balances and onboarding milestones.
	// ServiceNow types the hour fields as decimals and may omit any of them, so
	// each is a pointer — a nil balance ("not tracked for this project") is a
	// different fact from a zero one ("tracked, none left").
	TotalQueryHours          *float64 `json:"totalQueryHours"`
	ConsumedQueryHours       *float64 `json:"consumedQueryHours"`
	RemainingQueryHours      *float64 `json:"remainingQueryHours"`
	TotalOnboardingHours     *float64 `json:"totalOnboardingHours"`
	ConsumedOnboardingHours  *float64 `json:"consumedOnboardingHours"`
	RemainingOnboardingHours *float64 `json:"remainingOnboardingHours"`
	GoLiveDate               *string  `json:"goLiveDate"`
	GoLivePlanDate           *string  `json:"goLivePlanDate"`
	OnboardingExpiryDate     *string  `json:"onboardingExpiryDate"`
	OnboardingStatus         *string  `json:"onboardingStatus"`
	HasSr                    bool     `json:"hasSr"`
	snProjectClosureFields
	// OnboardingOwner is absent/empty for projects with no onboarding
	// engagement at all, and detail-endpoint-only — never present on the
	// search/list response.
	OnboardingOwner *snPersonRef `json:"onboardingOwner"`
}

type snProjectAccount struct {
	ID              string  `json:"id"`
	Name            string  `json:"name"`
	ActivationDate  *string `json:"activationDate"`
	SupportTier     string  `json:"supportTier"`
	Region          *string `json:"region"`
	HasAgent        bool    `json:"hasAgent"`
	HasKbReferences bool    `json:"hasKbReferences"`
	// The owning and technical contacts live on the account, not the project —
	// matching Ballerina's ProjectResponse.account and the portal's
	// ProjectDetailsAccount type.
	OwnerEmail          *string `json:"ownerEmail"`
	TechnicalOwnerEmail *string `json:"technicalOwnerEmail"`
	DeactivationDate    *string `json:"deactivationDate"`
	// Partner is the linked account's raw ServiceNow customer_account.partner passthrough,
	// merged in by the Ballerina entity-service. Named into domain.ProjectAccountRef.IsPartner.
	Partner *bool `json:"partner"`
}

// GetProjectByID implements ProjectService by calling the Choreo GET /projects/{id} endpoint.
// optionalSNProjectDate parses an optional ServiceNow date, returning nil when
// the field is absent or empty.
//
// A present-but-malformed value is an error rather than a silent nil, matching
// how this file already treated account activationDate before these fields were
// added. Note this means one bad date fails the whole project read; that
// strictness is inherited deliberately rather than newly introduced, so all
// optional project dates behave the same way.
func optionalSNProjectDate(label string, v *string) (*time.Time, error) {
	if v == nil || *v == "" {
		return nil, nil
	}
	t, err := time.Parse(snDateLayout, *v)
	if err != nil {
		return nil, fmt.Errorf("sn projects: parse %s %q: %w", label, *v, err)
	}
	return &t, nil
}

func (s *snProjectService) GetProjectByID(ctx context.Context, id string) (domain.ProjectDetailsView, error) {
	token := middleware.UserIDTokenFromContext(ctx)

	raw, err := s.client.Get(ctx, "/projects/"+uuidToSysid(id), token)
	if err != nil {
		return domain.ProjectDetailsView{}, err
	}

	var sn snProjectDetailsResponse
	if err := json.Unmarshal(raw, &sn); err != nil {
		return domain.ProjectDetailsView{}, fmt.Errorf("sn projects: parse detail response: %w", err)
	}

	createdOn, err := time.Parse(snCreatedOnLayout, sn.CreatedOn)
	if err != nil {
		return domain.ProjectDetailsView{}, fmt.Errorf("sn projects: parse createdOn %q: %w", sn.CreatedOn, err)
	}

	startDate, err := optionalSNProjectDate("startDate", &sn.StartDate)
	if err != nil {
		return domain.ProjectDetailsView{}, err
	}

	endDate, err := optionalSNProjectDate("endDate", &sn.EndDate)
	if err != nil {
		return domain.ProjectDetailsView{}, err
	}

	subType, knownType := snTypeNameToSubscriptionType(sn.Type.Name)
	if !knownType {
		slog.WarnContext(ctx, "sn projects: unrecognized subscription type from ServiceNow",
			"projectID", sn.ID, "typeName", sn.Type.Name)
	}

	activationDate, err := optionalSNProjectDate("account activationDate", sn.Account.ActivationDate)
	if err != nil {
		return domain.ProjectDetailsView{}, err
	}
	deactivationDate, err := optionalSNProjectDate("account deactivationDate", sn.Account.DeactivationDate)
	if err != nil {
		return domain.ProjectDetailsView{}, err
	}
	goLiveDate, err := optionalSNProjectDate("goLiveDate", sn.GoLiveDate)
	if err != nil {
		return domain.ProjectDetailsView{}, err
	}
	goLivePlanDate, err := optionalSNProjectDate("goLivePlanDate", sn.GoLivePlanDate)
	if err != nil {
		return domain.ProjectDetailsView{}, err
	}
	onboardingExpiryDate, err := optionalSNProjectDate("onboardingExpiryDate", sn.OnboardingExpiryDate)
	if err != nil {
		return domain.ProjectDetailsView{}, err
	}

	var onboardingOwner *domain.PersonRef
	if sn.OnboardingOwner != nil && sn.OnboardingOwner.ID != "" {
		onboardingOwner = &domain.PersonRef{
			ID:    sysidToUUID(sn.OnboardingOwner.ID),
			Name:  sn.OnboardingOwner.Name,
			Email: nilIfEmpty(sn.OnboardingOwner.Email),
		}
	}

	return domain.ProjectDetailsView{
		ID:               sysidToUUID(sn.ID),
		SfID:             sn.SfID,
		Name:             sn.Name,
		Key:              sn.Key,
		SubscriptionType: subType,
		StartDate:        startDate,
		EndDate:          endDate,
		CreatedOn:        createdOn,
		UpdatedOn:        createdOn,
		HasSr:            sn.HasSr,
		ProjectClosureFields: domain.ProjectClosureFields{
			ClosureState:                    sn.ClosureState,
			EndDateClosureState:             sn.EndDateClosureState,
			InvoiceDueDateClosureState:      sn.InvoiceDueDateClosureState,
			ComplianceViolationClosureState: sn.ComplianceViolationClosureState,
			ComplianceViolationDate:         sn.ComplianceViolationDate,
			SuspensionProcessState:          sn.SuspensionProcessState,
		},
		ProjectEngagementFields: domain.ProjectEngagementFields{
			TotalQueryHours:          sn.TotalQueryHours,
			ConsumedQueryHours:       sn.ConsumedQueryHours,
			RemainingQueryHours:      sn.RemainingQueryHours,
			TotalOnboardingHours:     sn.TotalOnboardingHours,
			ConsumedOnboardingHours:  sn.ConsumedOnboardingHours,
			RemainingOnboardingHours: sn.RemainingOnboardingHours,
			GoLiveDate:               goLiveDate,
			GoLivePlanDate:           goLivePlanDate,
			OnboardingExpiryDate:     onboardingExpiryDate,
			OnboardingStatus:         sn.OnboardingStatus,
		},
		Account: domain.ProjectAccountRef{
			ID:                  sysidToUUID(sn.Account.ID),
			Name:                sn.Account.Name,
			ActivationDate:      activationDate,
			DeactivationDate:    deactivationDate,
			Tier:                sn.Account.SupportTier,
			Region:              sn.Account.Region,
			AgentEnabled:        sn.Account.HasAgent,
			KbReferencesEnabled: sn.Account.HasKbReferences,
			OwnerEmail:          sn.Account.OwnerEmail,
			TechnicalOwnerEmail: sn.Account.TechnicalOwnerEmail,
			IsPartner:           sn.Account.Partner,
		},
		OnboardingOwner: onboardingOwner,
	}, nil
}

// snProjectUpdatePayload is the Choreo PATCH /projects/{id} request body.
type snProjectUpdatePayload struct {
	HasAgent                        *bool           `json:"hasAgent,omitempty"`
	HasKbReferences                 *bool           `json:"hasKbReferences,omitempty"`
	EndDateClosureState             *string         `json:"endDateClosureState,omitempty"`
	InvoiceDueDateClosureState      *string         `json:"invoiceDueDateClosureState,omitempty"`
	ComplianceViolationClosureState *string         `json:"complianceViolationClosureState,omitempty"`
	SuspensionProcessState          json.RawMessage `json:"suspensionProcessState,omitempty"`
}

// snProjectUpdateResponse mirrors the Choreo PATCH /projects/{id} response.
type snProjectUpdateResponse struct {
	Message string                `json:"message"`
	Project snProjectUpdateResult `json:"project"`
}

type snProjectUpdateResult struct {
	ID                              string          `json:"id"`
	UpdatedBy                       string          `json:"updatedBy"`
	UpdatedOn                       string          `json:"updatedOn"`
	ClosureState                    *string         `json:"closureState"`
	EndDateClosureState             *string         `json:"endDateClosureState"`
	InvoiceDueDateClosureState      *string         `json:"invoiceDueDateClosureState"`
	ComplianceViolationClosureState *string         `json:"complianceViolationClosureState"`
	SuspensionProcessState          json.RawMessage `json:"suspensionProcessState"`
}

type snProjectUpdateService struct {
	client *integrationservice.Client
}

// NewServiceNowProjectUpdateService constructs a ProjectUpdateService backed by the Choreo API.
func NewServiceNowProjectUpdateService(client *integrationservice.Client) ProjectUpdateService {
	return &snProjectUpdateService{client: client}
}

// UpdateProject implements ProjectUpdateService by calling the Choreo PATCH
// /projects/{id} endpoint. Note: the overall closure state is not directly
// settable — SN derives it from the three closure sub-state fields via a
// business rule, so the response reflects whatever it recomputed to.
//
// Deliberately no enum validation on EndDateClosureState/InvoiceDueDateClosureState/
// ComplianceViolationClosureState (unlike ClosureStatus in SearchProjects):
// unlike the overall status, which this service's own business rule computes
// from a fixed 3-value set, these sub-states are set by an evolving SN Flow
// Designer process whose full value set isn't known here — live data has
// already shown values ("Pending Notified") beyond the small set observed
// during development. A validXxx map here would risk rejecting legitimate
// values the ACP automation needs to write.
//
// SuspensionProcessState is likewise not validated here: it is a free-form JSON
// object written by an existing, actively-used SN suspension flow, and this
// service passes it through opaquely without imposing any schema on its
// contents.
func (s *snProjectUpdateService) UpdateProject(ctx context.Context, id string, req domain.ProjectUpdateRequest) (domain.ProjectUpdateResponse, error) {
	if req.HasAgent == nil && req.HasKbReferences == nil && req.EndDateClosureState == nil &&
		req.InvoiceDueDateClosureState == nil && req.ComplianceViolationClosureState == nil &&
		req.SuspensionProcessState == nil {
		return domain.ProjectUpdateResponse{}, &apierror.ValidationError{Msg: "at least one field must be provided"}
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snProjectUpdatePayload{
		HasAgent:                        req.HasAgent,
		HasKbReferences:                 req.HasKbReferences,
		EndDateClosureState:             req.EndDateClosureState,
		InvoiceDueDateClosureState:      req.InvoiceDueDateClosureState,
		ComplianceViolationClosureState: req.ComplianceViolationClosureState,
		SuspensionProcessState:          req.SuspensionProcessState,
	}
	raw, err := s.client.Patch(ctx, "/projects/"+uuidToSysid(id), token, payload)
	if err != nil {
		return domain.ProjectUpdateResponse{}, err
	}

	var sn snProjectUpdateResponse
	if err := json.Unmarshal(raw, &sn); err != nil {
		return domain.ProjectUpdateResponse{}, fmt.Errorf("sn projects: parse update response: %w", err)
	}

	updatedOn, err := time.Parse(snCreatedOnLayout, sn.Project.UpdatedOn)
	if err != nil {
		return domain.ProjectUpdateResponse{}, fmt.Errorf("sn projects: parse updatedOn %q: %w", sn.Project.UpdatedOn, err)
	}

	return domain.ProjectUpdateResponse{
		Message: sn.Message,
		Project: domain.ProjectUpdateResult{
			ID:                              sysidToUUID(sn.Project.ID),
			UpdatedBy:                       sn.Project.UpdatedBy,
			UpdatedOn:                       updatedOn,
			ClosureState:                    sn.Project.ClosureState,
			EndDateClosureState:             sn.Project.EndDateClosureState,
			InvoiceDueDateClosureState:      sn.Project.InvoiceDueDateClosureState,
			ComplianceViolationClosureState: sn.Project.ComplianceViolationClosureState,
			SuspensionProcessState:          sn.Project.SuspensionProcessState,
		},
	}, nil
}

// validClosureStatuses is the set of values the "Update WSO2 Closure State"
// SN business rule ever computes for the overall closure state — the only
// three literal strings that rule's branches assign. Safe to enforce here
// because we own that rule's logic, unlike the closure *sub-state* fields
// (see updateProject), whose value set is set by an evolving SN Flow
// Designer process and isn't fully known.
var validClosureStatuses = map[string]struct{}{
	"Open":       {},
	"Suspended":  {},
	"Restricted": {},
}

// validSortOrders is the set of accepted SortOrder values.
var validSortOrders = map[string]struct{}{
	"":     {}, // unset defaults to the service's own default ordering
	"asc":  {},
	"desc": {},
}

// validateProjectSearchFilters rejects unknown closureStatus/sortBy/sortOrder
// values and malformed end-date filters before they reach ServiceNow.
func validateProjectSearchFilters(req domain.SearchProjectsRequest) error {
	if req.ClosureStatus != "" {
		if _, ok := validClosureStatuses[req.ClosureStatus]; !ok {
			return &apierror.ValidationError{Msg: "closureStatus must be one of: Open, Suspended, Restricted"}
		}
	}
	if req.SortBy != "" && req.SortBy != "endDate" {
		return &apierror.ValidationError{Msg: `sortBy must be "endDate" if provided`}
	}
	if _, ok := validSortOrders[req.SortOrder]; !ok {
		return &apierror.ValidationError{Msg: `sortOrder must be "asc" or "desc" if provided`}
	}
	if req.EndDateFrom != "" {
		if _, err := time.Parse(snDateLayout, req.EndDateFrom); err != nil {
			return &apierror.ValidationError{Msg: "endDateFrom must be a valid date (yyyy-MM-dd)"}
		}
	}
	if req.EndDateTo != "" {
		if _, err := time.Parse(snDateLayout, req.EndDateTo); err != nil {
			return &apierror.ValidationError{Msg: "endDateTo must be a valid date (yyyy-MM-dd)"}
		}
	}
	for _, s := range req.ExcludeClosureStates {
		if _, ok := validClosureStatuses[s]; !ok {
			return &apierror.ValidationError{Msg: "excludeClosureStates must each be one of: Open, Suspended, Restricted"}
		}
	}
	for _, t := range req.ExcludeSubscriptionTypes {
		if _, ok := validSubscriptionTypes[t]; !ok {
			return &apierror.ValidationError{Msg: "excludeSubscriptionTypes contains invalid value: " + string(t)}
		}
	}
	for _, k := range req.ExcludeProjectKeys {
		// Unlike ExcludeClosureStates/ExcludeSubscriptionTypes, project keys
		// have no fixed enum to validate against — only reject the input
		// shapes that could never be a real key and would otherwise silently
		// match nothing: empty (or, if a project's own Key were ever empty,
		// match every such project), and leading/trailing whitespace (the
		// exclusion check below matches the raw value exactly against a
		// project's own Key, so " APEXIA " would never match the real
		// "APEXIA" -- silently leaving that project eligible instead of
		// excluded, exactly the misconfiguration this endpoint exists to
		// prevent). Rejecting loudly here, rather than trimming and
		// accepting it, surfaces a bad CSM_ANNOUNCEMENT_EXCLUDED_PROJECT_KEYS
		// entry at request time instead of as a silent no-op exclusion.
		if strings.TrimSpace(k) == "" {
			return &apierror.ValidationError{Msg: "excludeProjectKeys must not contain empty values"}
		}
		if k != strings.TrimSpace(k) {
			return &apierror.ValidationError{Msg: "excludeProjectKeys must not contain leading or trailing whitespace: " + strconv.Quote(k)}
		}
	}
	return nil
}

// validSubscriptionTypes is the set of known SubscriptionType enum values.
var validSubscriptionTypes = map[domain.SubscriptionType]struct{}{
	domain.SubscriptionTypeDevelopmentSupport:       {},
	domain.SubscriptionTypeManagedCloudSubscription: {},
	domain.SubscriptionTypeEvaluationSubscription:   {},
	domain.SubscriptionTypeSubscription:             {},
	domain.SubscriptionTypeCloudEvaluationSupport:   {},
	domain.SubscriptionTypeInternal:                 {},
	domain.SubscriptionTypePlatformerSubscription:   {},
	domain.SubscriptionTypeCloudSupport:             {},
	domain.SubscriptionTypeProfessionalServices:     {},
}

// snTypeNameToSubscriptionType converts a SN project type name (e.g. "Cloud
// Support") to the domain SubscriptionType enum (e.g. "cloud_support").
//
// Never fails: an unrecognized name (blank, a legacy/typo'd label, or a type
// ServiceNow has added since validSubscriptionTypes was last updated) still
// returns a best-effort derived value instead of erroring. This used to
// return an error, which every caller propagated straight up as an opaque
// 500 -- harmless while the only callers were single-project lookups, but
// fetchEligibleProjectIDs (SearchProjectsByProductVersion's mandatory
// audience-exclusion check) now calls this once per project across the
// *entire* platform with no scoping filter at all, so a single project
// anywhere with an unrecognized type took down every caller's product-
// version audience resolution -- reported live as "Couldn't resolve the
// audience. Try again." The second return value reports whether the name
// matched a known enum member, so a caller that wants to know can log the
// mismatch without failing the request; every current caller here does.
func snTypeNameToSubscriptionType(name string) (domain.SubscriptionType, bool) {
	st := domain.SubscriptionType(strings.ToLower(strings.ReplaceAll(name, " ", "_")))
	_, known := validSubscriptionTypes[st]
	return st, known
}

// snContactSearchPayload is the Choreo POST /{resource}/{id}/contacts/search request
// body. It is shared by both the project-contacts and account-contacts endpoints.
type snContactSearchPayload struct {
	Filters    snContactFilters    `json:"filters,omitempty"`
	Pagination snProjectPagination `json:"pagination"`
}

type snContactFilters struct {
	SearchQuery string `json:"searchQuery,omitempty"`
}

// snProjectContactsResponse mirrors the Choreo POST /projects/{id}/contacts/search response.
type snProjectContactsResponse struct {
	Contacts     []snProjectContact `json:"contacts"`
	TotalRecords int                `json:"totalRecords"`
	Offset       int                `json:"offset"`
	Limit        int                `json:"limit"`
}

type snProjectContact struct {
	// ID is absent on instances that predate the upstream change, and null for a row
	// with no linked contact record.
	ID                     *string  `json:"id"`
	Name                   string   `json:"name"`
	Email                  string   `json:"email"`
	RegistrationState      string   `json:"registrationState"`
	NotificationsEnabled   bool     `json:"notificationsEnabled"`
	Roles                  []string `json:"roles"`
	CustomerContactPresent bool     `json:"customerContactPresent"`
	GrantsCaseAccess       bool     `json:"grantsCaseAccess"`
}

type snProjectContactService struct {
	client *integrationservice.Client
}

// NewServiceNowProjectContactService constructs a ProjectContactService backed by the Choreo API.
func NewServiceNowProjectContactService(client *integrationservice.Client) ProjectContactService {
	return &snProjectContactService{client: client}
}

// SearchProjectContacts implements ProjectContactService. Supported by the ServiceNow
// data source only; there is no Postgres fallback.
func (s *snProjectContactService) SearchProjectContacts(ctx context.Context, projectID string, req domain.SearchProjectContactsRequest) (domain.SearchProjectContactsResponse, error) {
	if err := validateUUIDs("id", []string{projectID}); err != nil {
		return domain.SearchProjectContactsResponse{}, err
	}
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchProjectContactsResponse{}, err
	}
	if err := validateSearchQuery(req.Filters.SearchQuery); err != nil {
		return domain.SearchProjectContactsResponse{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snContactSearchPayload{
		Filters:    snContactFilters{SearchQuery: req.Filters.SearchQuery},
		Pagination: snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}

	raw, err := s.client.Post(ctx, "/projects/"+uuidToSysid(projectID)+"/contacts/search", token, payload)
	if err != nil {
		return domain.SearchProjectContactsResponse{}, err
	}

	var snResp snProjectContactsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchProjectContactsResponse{}, fmt.Errorf("sn project contacts: parse response: %w", err)
	}

	contacts := make([]domain.ProjectContact, 0, len(snResp.Contacts))
	for _, c := range snResp.Contacts {
		// Convert only a real id; a nil or blank upstream id stays empty so the caller
		// renders the row unlinked instead of pointing at a bogus profile.
		var contactID *string
		if c.ID != nil && *c.ID != "" {
			id := sysidToUUID(*c.ID)
			contactID = &id
		}
		// Name is only known when a contact record is linked; a blank upstream name
		// stays nil rather than an empty string, matching the response-null contract.
		var name *string
		if c.Name != "" {
			name = strPtr(c.Name)
		}
		contacts = append(contacts, domain.ProjectContact{
			ID:                   contactID,
			Name:                 name,
			Email:                c.Email,
			RegistrationState:    c.RegistrationState,
			NotificationsEnabled: c.NotificationsEnabled,
			Roles:                c.Roles,
			// ServiceNow has no notion of the account-level role set at all
			// (it is derived from the Postgres membership tables), so this
			// data source answers with an empty list rather than a null --
			// absent, not unknown.
			AccountRoles:           []string{},
			CustomerContactPresent: c.CustomerContactPresent,
			GrantsCaseAccess:       c.GrantsCaseAccess,
		})
	}

	return domain.SearchProjectContactsResponse{
		Contacts: contacts,
		Total:    snResp.TotalRecords,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
	}, nil
}

// projectContactScanLimit bounds how many of a project's contacts GetProjectContact will
// scan. A project's contact list is a handful of people in practice, so paging to find one
// is cheaper than adding a by-id filter to the upstream resource. If a project ever exceeds
// this, the lookup reports not-found rather than silently scanning a partial list.
//
// It is pinned to maxLimit because the scan goes through SearchProjectContacts, which
// normalizes pagination and rejects anything larger outright: a bigger value here would
// fail every lookup with a validation error rather than widening the window.
const projectContactScanLimit = maxLimit

// GetProjectContact implements ProjectContactService.
//
// There is no by-id contact resource upstream, so this reads the project's contacts and
// picks the matching row. That keeps the whole feature inside this layer: no new upstream
// surface, and the row shape stays identical to the one the list returns, so a caller
// showing contact detail renders the same fields it already knows.
func (s *snProjectContactService) GetProjectContact(
	ctx context.Context, projectID, contactID string,
) (domain.ProjectContact, error) {
	if err := validateUUIDs("id", []string{projectID}); err != nil {
		return domain.ProjectContact{}, err
	}
	if err := validateUUIDs("contactId", []string{contactID}); err != nil {
		return domain.ProjectContact{}, err
	}

	page, err := s.SearchProjectContacts(ctx, projectID, domain.SearchProjectContactsRequest{
		Pagination: domain.Pagination{Limit: projectContactScanLimit, Offset: 0},
	})
	if err != nil {
		return domain.ProjectContact{}, err
	}

	for _, c := range page.Contacts {
		if c.ID != nil && *c.ID == contactID {
			return c, nil
		}
	}

	// Either the contact is not on this project, or their row has no linked contact
	// record and therefore no id to match. Both are "not a contact on this project" as
	// far as a caller is concerned.
	if page.Total > len(page.Contacts) {
		log.Printf("sn project contacts: project %s has %d contacts, only %d scanned; contact %s not found in that window",
			projectID, page.Total, len(page.Contacts), contactID)
	}
	return domain.ProjectContact{}, &apierror.NotFoundError{Msg: "contact not found on this project"}
}
