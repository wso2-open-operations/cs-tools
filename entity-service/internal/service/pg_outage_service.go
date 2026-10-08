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

package service

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// pgOutageService is the Postgres implementation of OutageService.
//
// *** IT MIMICS THE SERVICENOW SERVICE, DELIBERATELY AND LITERALLY. ***
// Same validation, same error messages, same response shapes -- the portal is
// already built against them, and the whole point is that the Outages page
// keeps working unchanged once DATA_SOURCE leaves "servicenow". Where this
// file differs from sn_outage_service.go, that is a bug unless the difference
// is commented.
//
// *** AND IT EXISTS BECAUSE THE PAGE WAS SIMPLY OFF WITHOUT IT. *** routes.go
// registered the outage endpoints only under DataSourceServiceNow, with no
// else branch, so a deployment running postgres-servicenow-dual-write served
// 404 on every outage URL -- which is what staging was doing.
//
// It is also what makes the cloud status port testable end to end: a portal
// create has to land in the Postgres outage table for the sweep to see it.
// Creating it in ServiceNow instead would leave the sweep reading an empty
// table and look like a defect in the webhook port.
type pgOutageService struct {
	repo repository.OutageRepository
}

// NewOutageService constructs the Postgres-backed OutageService.
func NewOutageService(repo repository.OutageRepository) OutageService {
	return &pgOutageService{repo: repo}
}

// outageSearchDefaultWindow is how far back a search reaches when the caller
// gives no beginFrom. The table is dominated by a historical bulk load, so an
// unbounded default would page through years of it; the applied bound is
// reported back rather than left implicit.
const outageSearchDefaultWindow = -6 * 30 * 24 * time.Hour

// actorOf names whoever is writing, for created_by/updated_by.
func actorOf(ctx context.Context) string {
	id := auth.IdentityFromContext(ctx)
	if id.UserEmail != "" {
		return id.UserEmail
	}
	if id.ClientID != "" {
		return id.ClientID
	}
	return "entity-service"
}

// requirePublicationAck enforces the gate CreateOutage and UpdateOutage share:
// if the chosen configuration item resolves to a monitored cloud, the caller
// must say so explicitly.
//
// *** THE GATE IS A 409, NOT A 400. *** The request is well-formed; what is
// missing is consent to publish. The portal relies on that distinction to
// show its publication warning rather than a validation error.
func (s *pgOutageService) requirePublicationAck(ctx context.Context, offeringID *string, ack *bool) error {
	publishes, _, err := s.repo.PublicationFor(ctx, offeringID)
	if err != nil {
		return err
	}
	if publishes && (ack == nil || !*ack) {
		return &apierror.ConflictError{Msg: "acknowledgePublicPublication is required: this outage is publicly visible"}
	}
	return nil
}

// CreateOutage implements OutageService for the Postgres data source.
func (s *pgOutageService) CreateOutage(ctx context.Context, req domain.CreateOutageRequest) (domain.CreateOutageResponse, error) {
	in, err := s.prepareCreate(ctx, req)
	if err != nil {
		return domain.CreateOutageResponse{}, err
	}
	out, err := s.repo.Create(ctx, in)
	if err != nil {
		return domain.CreateOutageResponse{}, err
	}
	return domain.CreateOutageResponse{Message: "Outage created successfully.", Outage: out}, nil
}

// prepareCreate validates req and resolves it into the repository's write
// shape, touching nothing. It is CreateOutage's whole validation half, split
// out so outageSNFirstService can run EXACTLY the same checks (same order,
// same messages) before it calls the external system: a request Postgres
// would reject must be rejected before anything is created upstream, or the
// external record would be orphaned by a failure that was knowable up front.
func (s *pgOutageService) prepareCreate(ctx context.Context, req domain.CreateOutageRequest) (repository.OutageWrite, error) {
	// Validation is copied from sn_outage_service.go verbatim, messages
	// included: a caller must not be able to tell the data sources apart.
	if req.Type == "" {
		return repository.OutageWrite{}, &apierror.ValidationError{Msg: "type is required"}
	}
	if !validOutageType[req.Type] {
		return repository.OutageWrite{}, &apierror.ValidationError{Msg: "invalid type: " + string(req.Type)}
	}
	if strings.TrimSpace(req.Begin) == "" {
		return repository.OutageWrite{}, &apierror.ValidationError{Msg: "begin is required"}
	}
	if strings.TrimSpace(req.ShortDescription) == "" {
		return repository.OutageWrite{}, &apierror.ValidationError{Msg: "shortDescription is required"}
	}
	if utf8.RuneCountInString(req.ShortDescription) > 160 {
		return repository.OutageWrite{}, &apierror.ValidationError{Msg: "shortDescription must be 160 characters or fewer"}
	}
	if req.ConfigurationItemID != nil {
		if err := validateUUIDs("configurationItemId", []string{*req.ConfigurationItemID}); err != nil {
			return repository.OutageWrite{}, err
		}
	}
	if req.IncidentID != nil {
		if err := validateUUIDs("incidentId", []string{*req.IncidentID}); err != nil {
			return repository.OutageWrite{}, err
		}
	}

	begin, err := parseOutageTime(req.Begin, "begin")
	if err != nil {
		return repository.OutageWrite{}, err
	}
	var end *time.Time
	if req.End != nil && strings.TrimSpace(*req.End) != "" {
		t, err := parseOutageTime(*req.End, "end")
		if err != nil {
			return repository.OutageWrite{}, err
		}
		if t.Before(begin) {
			return repository.OutageWrite{}, &apierror.ValidationError{Msg: "end must not be before begin"}
		}
		end = &t
	}

	if err := s.requirePublicationAck(ctx, req.ConfigurationItemID, req.AcknowledgePublicPublication); err != nil {
		return repository.OutageWrite{}, err
	}
	affected, err := normaliseAffectedCIIDs(req.AffectedConfigurationItemIDs)
	if err != nil {
		return repository.OutageWrite{}, err
	}
	if err := s.requireAffectedPublicationAck(ctx, affected, req.AcknowledgePublicPublication); err != nil {
		return repository.OutageWrite{}, err
	}
	impact, err := normaliseOutageLabel(req.Impact, "impact")
	if err != nil {
		return repository.OutageWrite{}, err
	}
	state, err := normaliseOutageLabel(req.State, "state")
	if err != nil {
		return repository.OutageWrite{}, err
	}

	return repository.OutageWrite{
		Type:                  string(req.Type),
		Begin:                 begin,
		End:                   end,
		ShortDescription:      req.ShortDescription,
		ServiceOfferingID:     req.ConfigurationItemID,
		IncidentID:            req.IncidentID,
		ExternalCommunication: req.ExternalCommunication,
		InternalCommunication: req.InternalCommunication,
		// Unticked unless the caller ticks it, as on ServiceNow's form.
		NotifyInternalStakeholders: req.NotifyInternalStakeholders != nil && *req.NotifyInternalStakeholders,
		OutageCommunication:        req.OutageCommunication != nil && *req.OutageCommunication,
		Impact:                     nonEmpty(impact),
		State:                      nonEmpty(state),
		AffectedCIIDs:              affected,
		Actor:                      actorOf(ctx),
	}, nil
}

// parseOutageTime accepts the two shapes the API documents: RFC3339 and the
// space-separated "YYYY-MM-DD HH:mm:ss" the portal sends.
func parseOutageTime(raw, field string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.UTC(), nil
	}
	if t, err := time.Parse("2006-01-02 15:04:05", raw); err == nil {
		return t.UTC(), nil
	}
	return time.Time{}, &apierror.ValidationError{Msg: "invalid " + field + ": expected YYYY-MM-DD HH:mm:ss or ISO-8601"}
}

// SearchOutages implements OutageService for the Postgres data source.
func (s *pgOutageService) SearchOutages(ctx context.Context, req domain.SearchOutagesRequest) (domain.SearchOutagesResponse, error) {
	for _, t := range req.Filters.Types {
		if !validOutageType[t] {
			return domain.SearchOutagesResponse{}, &apierror.ValidationError{Msg: "invalid type: " + string(t)}
		}
	}
	for _, st := range req.Filters.Statuses {
		if !validOutageStatus[st] {
			return domain.SearchOutagesResponse{}, &apierror.ValidationError{Msg: "invalid status: " + string(st)}
		}
	}
	if req.SortBy.Field != "" && !validOutageSortField[req.SortBy.Field] {
		return domain.SearchOutagesResponse{}, &apierror.ValidationError{Msg: "invalid sort field: " + string(req.SortBy.Field)}
	}
	if req.SortBy.Order != "" && !validOutageSortOrder[req.SortBy.Order] {
		return domain.SearchOutagesResponse{}, &apierror.ValidationError{Msg: "invalid sort order: " + string(req.SortBy.Order)}
	}
	if len(req.Filters.ConfigurationItemIDs) > 0 {
		if err := validateUUIDs("configurationItemIds", req.Filters.ConfigurationItemIDs); err != nil {
			return domain.SearchOutagesResponse{}, err
		}
	}
	if len(req.Filters.IncidentIDs) > 0 {
		if err := validateUUIDs("incidentIds", req.Filters.IncidentIDs); err != nil {
			return domain.SearchOutagesResponse{}, err
		}
	}

	beginFrom := time.Now().UTC().Add(outageSearchDefaultWindow)
	defaulted := true
	if req.Filters.BeginFrom != nil && strings.TrimSpace(*req.Filters.BeginFrom) != "" {
		t, err := parseOutageTime(*req.Filters.BeginFrom, "beginFrom")
		if err != nil {
			return domain.SearchOutagesResponse{}, err
		}
		beginFrom, defaulted = t, false
	}

	outages, total, err := s.repo.Search(ctx, req, beginFrom)
	if err != nil {
		return domain.SearchOutagesResponse{}, err
	}

	limit := req.Pagination.Limit
	if limit <= 0 {
		limit = 20
	}
	return domain.SearchOutagesResponse{
		Outages:            outages,
		Total:              total,
		Limit:              limit,
		Offset:             req.Pagination.Offset,
		AppliedBeginFrom:   beginFrom.Format(time.RFC3339),
		BeginFromDefaulted: defaulted,
	}, nil
}

// GetOutageByID implements OutageService for the Postgres data source.
func (s *pgOutageService) GetOutageByID(ctx context.Context, id string) (domain.OutageDetail, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.OutageDetail{}, err
	}
	return s.repo.GetByID(ctx, id)
}

// UpdateOutage implements OutageService for the Postgres data source.
func (s *pgOutageService) UpdateOutage(ctx context.Context, req domain.PatchOutageRequest) (domain.PatchOutageResponse, error) {
	if err := validateUUIDs("id", []string{req.ID}); err != nil {
		return domain.PatchOutageResponse{}, err
	}
	if req.Type != nil && !validOutageType[*req.Type] {
		return domain.PatchOutageResponse{}, &apierror.ValidationError{Msg: "invalid type: " + string(*req.Type)}
	}
	if req.ShortDescription != nil {
		if strings.TrimSpace(*req.ShortDescription) == "" {
			return domain.PatchOutageResponse{}, &apierror.ValidationError{Msg: "shortDescription is required"}
		}
		if utf8.RuneCountInString(*req.ShortDescription) > 160 {
			return domain.PatchOutageResponse{}, &apierror.ValidationError{Msg: "shortDescription must be 160 characters or fewer"}
		}
	}
	if req.ConfigurationItemID != nil {
		if err := validateUUIDs("configurationItemId", []string{*req.ConfigurationItemID}); err != nil {
			return domain.PatchOutageResponse{}, err
		}
		// Only re-gate when the configuration item itself is changing: a
		// patch that merely closes an outage must not demand consent again.
		if err := s.requirePublicationAck(ctx, req.ConfigurationItemID, req.AcknowledgePublicPublication); err != nil {
			return domain.PatchOutageResponse{}, err
		}
	}
	if req.IncidentID != nil {
		if err := validateUUIDs("incidentId", []string{*req.IncidentID}); err != nil {
			return domain.PatchOutageResponse{}, err
		}
	}

	impact, err := normaliseOutageLabel(req.Impact, "impact")
	if err != nil {
		return domain.PatchOutageResponse{}, err
	}
	state, err := normaliseOutageLabel(req.State, "state")
	if err != nil {
		return domain.PatchOutageResponse{}, err
	}

	var affected *[]string
	if req.AffectedConfigurationItemIDs != nil {
		ids, err := normaliseAffectedCIIDs(*req.AffectedConfigurationItemIDs)
		if err != nil {
			return domain.PatchOutageResponse{}, err
		}
		// Gate only what this edit ADDS: re-saving an outage whose affected
		// CIs already publish must not demand consent again.
		current, err := s.repo.GetByID(ctx, req.ID)
		if err != nil {
			return domain.PatchOutageResponse{}, err
		}
		have := map[string]bool{}
		for _, ci := range current.Outage.AffectedConfigurationItems {
			have[strings.ToLower(ci.ID)] = true
		}
		var added []string
		for _, id := range ids {
			if !have[strings.ToLower(id)] {
				added = append(added, id)
			}
		}
		if err := s.requireAffectedPublicationAck(ctx, added, req.AcknowledgePublicPublication); err != nil {
			return domain.PatchOutageResponse{}, err
		}
		affected = &ids
	}

	patch := repository.OutagePatch{
		AffectedCIIDs:              affected,
		ID:                         req.ID,
		ShortDescription:           req.ShortDescription,
		ServiceOfferingID:          req.ConfigurationItemID,
		IncidentID:                 req.IncidentID,
		NotifyInternalStakeholders: req.NotifyInternalStakeholders,
		OutageCommunication:        req.OutageCommunication,
		Impact:                     impact,
		State:                      state,
		Actor:                      actorOf(ctx),
	}
	if req.Type != nil {
		t := string(*req.Type)
		patch.Type = &t
	}

	// Parsing happens HERE and only here, with the same formats CreateOutage
	// accepts. The repository used to parse RFC3339 only, so the portal --
	// which sends "YYYY-MM-DD HH:mm:ss" -- could create an outage but not
	// close one.
	if req.Begin != nil {
		t, err := parseOutageTime(*req.Begin, "begin")
		if err != nil {
			return domain.PatchOutageResponse{}, err
		}
		patch.Begin = &t
	}
	if req.End != nil {
		if *req.End == nil {
			// Explicit null reopens; the inner nil has to survive to the
			// repository or a reopen silently becomes a no-op.
			var reopen *time.Time
			patch.End = &reopen
		} else {
			t, err := parseOutageTime(**req.End, "end")
			if err != nil {
				return domain.PatchOutageResponse{}, err
			}
			tp := &t
			patch.End = &tp
		}
	}

	// *** VALIDATE THE EFFECTIVE INTERVAL, NOT THE SUBMITTED FIELDS. *** A
	// patch that sets only end has to be checked against the STORED begin,
	// or an end before the outage started is accepted -- and a negative
	// interval renders as a nonsense duration on the public status page.
	// CreateOutage already refuses this; without the stored-value lookup the
	// same request would sail through as an update.
	if patch.Begin != nil || (patch.End != nil && *patch.End != nil) {
		current, err := s.repo.GetByID(ctx, req.ID)
		if err != nil {
			return domain.PatchOutageResponse{}, err
		}
		effBegin, err := effectiveInstant(patch.Begin, current.Begin)
		if err != nil {
			return domain.PatchOutageResponse{}, err
		}
		var effEnd *time.Time
		if patch.End != nil {
			effEnd = *patch.End
		} else if current.End != nil {
			effEnd, err = effectiveInstant(nil, *current.End)
			if err != nil {
				return domain.PatchOutageResponse{}, err
			}
		}
		if effBegin != nil && effEnd != nil && effEnd.Before(*effBegin) {
			return domain.PatchOutageResponse{}, &apierror.ValidationError{Msg: "end must not be before begin"}
		}
	}

	out, err := s.repo.Update(ctx, patch)
	if err != nil {
		return domain.PatchOutageResponse{}, err
	}
	return domain.PatchOutageResponse{Message: "Outage updated successfully.", Outage: out}, nil
}

// AddOutageCommunication implements OutageService for the Postgres data source.
func (s *pgOutageService) AddOutageCommunication(ctx context.Context, req domain.AddOutageCommunicationRequest) (domain.AddOutageCommunicationResponse, error) {
	if err := validateUUIDs("id", []string{req.OutageID}); err != nil {
		return domain.AddOutageCommunicationResponse{}, err
	}
	if !validOutageCommunicationChannel[req.Channel] {
		return domain.AddOutageCommunicationResponse{}, &apierror.ValidationError{Msg: "invalid channel: " + string(req.Channel)}
	}
	if strings.TrimSpace(req.Body) == "" {
		return domain.AddOutageCommunicationResponse{}, &apierror.ValidationError{Msg: "body is required"}
	}

	detail, err := s.repo.GetByID(ctx, req.OutageID)
	if err != nil {
		return domain.AddOutageCommunicationResponse{}, err
	}

	// Only the external channel publishes, so only it is gated -- an internal
	// note on a publicly visible outage is still internal.
	if req.Channel == domain.OutageCommunicationChannelExternal && detail.PublishesToStatusPage &&
		(req.AcknowledgePublicPublication == nil || !*req.AcknowledgePublicPublication) {
		return domain.AddOutageCommunicationResponse{}, &apierror.ConflictError{
			Msg: "acknowledgePublicPublication is required: this entry is publicly visible"}
	}

	comm, err := s.repo.AddCommunication(ctx, req.OutageID, req.Channel, req.Body, actorOf(ctx))
	if err != nil {
		return domain.AddOutageCommunicationResponse{}, err
	}
	return domain.AddOutageCommunicationResponse{Message: "Communication added successfully.", Communication: comm}, nil
}

// SearchOutageCommunications implements OutageService for the Postgres data source.
func (s *pgOutageService) SearchOutageCommunications(ctx context.Context, req domain.SearchOutageCommunicationsRequest) (domain.SearchOutageCommunicationsResponse, error) {
	if err := validateUUIDs("id", []string{req.OutageID}); err != nil {
		return domain.SearchOutageCommunicationsResponse{}, err
	}
	for _, c := range req.Channels {
		if !validOutageCommunicationChannel[c] {
			return domain.SearchOutageCommunicationsResponse{}, &apierror.ValidationError{Msg: "invalid channel: " + string(c)}
		}
	}

	items, total, err := s.repo.SearchCommunications(ctx, req)
	if err != nil {
		return domain.SearchOutageCommunicationsResponse{}, err
	}
	limit := req.Pagination.Limit
	if limit <= 0 {
		limit = 50
	}
	return domain.SearchOutageCommunicationsResponse{
		Communications: items, Total: total, Limit: limit, Offset: req.Pagination.Offset,
	}, nil
}

// GetOutageMetadata implements OutageService for the Postgres data source.
//
// The three choice lists are this service's own enums rather than a query:
// they are compile-time constants in domain, and reading them from the
// database would let a stray row offer a type the API would then reject.
// Only the cloud list is live, because only it can change without a deploy.
func (s *pgOutageService) GetOutageMetadata(ctx context.Context) (domain.OutageMetadataResponse, error) {
	clouds, err := s.repo.MonitoredClouds(ctx)
	if err != nil {
		return domain.OutageMetadataResponse{}, err
	}
	return domain.OutageMetadataResponse{
		Types: []domain.OutageChoice{
			{Value: string(domain.OutageTypeOutage), Label: "Outage"},
			{Value: string(domain.OutageTypeDegradation), Label: "Degradation"},
			{Value: string(domain.OutageTypePlanned), Label: "Planned"},
		},
		Statuses: []domain.OutageChoice{
			{Value: string(domain.OutageStatusInProgress), Label: "In Progress"},
			{Value: string(domain.OutageStatusResolved), Label: "Resolved"},
		},
		CommunicationChannels: []domain.OutageCommunicationChannelMeta{
			{Value: string(domain.OutageCommunicationChannelExternal), Label: "External", IsPublic: true},
			{Value: string(domain.OutageCommunicationChannelInternal), Label: "Internal", IsPublic: false},
			{Value: string(domain.OutageCommunicationChannelAdditional), Label: "Additional", IsPublic: false},
		},
		StatusPageClouds: clouds,
	}, nil
}

// effectiveInstant returns the patched value when one is supplied, otherwise
// the stored one parsed back from its wire form. Stored values are written by
// this service in RFC3339, so a parse failure here is a bug rather than bad
// input -- it is reported rather than silently ignored.
func effectiveInstant(patched *time.Time, stored string) (*time.Time, error) {
	if patched != nil {
		return patched, nil
	}
	if strings.TrimSpace(stored) == "" {
		return nil, nil
	}
	t, err := parseOutageTime(stored, "stored timestamp")
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// outageLabelMaxRunes is the width of outage.impact and outage.state
// (VARCHAR(40), migration 0116).
const outageLabelMaxRunes = 40

// normaliseOutageLabel trims Impact or State and enforces the column width,
// so an over-long value is a 400 naming the field rather than a 500 from the
// database. nil stays nil (not provided); "" stays "" (cleared on update,
// unset on create).
func normaliseOutageLabel(v *string, field string) (*string, error) {
	if v == nil {
		return nil, nil
	}
	t := strings.TrimSpace(*v)
	if utf8.RuneCountInString(t) > outageLabelMaxRunes {
		return nil, &apierror.ValidationError{Msg: fmt.Sprintf("%s must be %d characters or fewer", field, outageLabelMaxRunes)}
	}
	return &t, nil
}

// nonEmpty drops a cleared value on create, where "" and absent both mean
// "not set" and the column should stay NULL.
func nonEmpty(v *string) *string {
	if v == nil || *v == "" {
		return nil
	}
	return v
}

// normaliseAffectedCIIDs validates and de-duplicates affected configuration
// item ids, keeping first-seen order.
func normaliseAffectedCIIDs(ids []string) ([]string, error) {
	if err := validateUUIDs("affectedConfigurationItemIds", ids); err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		key := strings.ToLower(id)
		if !seen[key] {
			seen[key] = true
			out = append(out, id)
		}
	}
	return out, nil
}

// requireAffectedPublicationAck applies the publication gate to affected CIs:
// an affected offering with a status-page monitor makes the outage public on
// that cloud exactly as the main CI does, so adding one needs the same consent.
func (s *pgOutageService) requireAffectedPublicationAck(ctx context.Context, added []string, ack *bool) error {
	for i := range added {
		if err := s.requirePublicationAck(ctx, &added[i], ack); err != nil {
			return err
		}
	}
	return nil
}
