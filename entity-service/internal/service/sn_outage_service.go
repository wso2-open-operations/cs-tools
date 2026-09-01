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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// snOutageConfigurationItemRef mirrors a Choreo outage configuration-item reference.
type snOutageConfigurationItemRef struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ClassName string `json:"className"`
}

// snOutageIncidentRef mirrors a Choreo outage's linked-incident reference.
type snOutageIncidentRef struct {
	ID               string  `json:"id"`
	Number           string  `json:"number"`
	ShortDescription string  `json:"shortDescription"`
	State            *string `json:"state"`
}

// snOutage mirrors the outage shape shared by the Choreo create, search, and
// PATCH responses. PublishesToStatusPage/StatusPageCloud are computed by the
// backing data source and must be surfaced exactly as received -- see
// domain.Outage.
type snOutage struct {
	ID                         string                         `json:"id"`
	Number                     string                         `json:"number"`
	Type                       *string                        `json:"type"`
	Status                     *string                        `json:"status"`
	Begin                      string                         `json:"begin"`
	End                        *string                        `json:"end"`
	Duration                   *string                        `json:"duration"`
	ShortDescription           string                         `json:"shortDescription"`
	ConfigurationItem          *snOutageConfigurationItemRef  `json:"configurationItem"`
	Incident                   *snOutageIncidentRef           `json:"incident"`
	AffectedConfigurationItems []snOutageConfigurationItemRef `json:"affectedConfigurationItems"`
	PublishesToStatusPage      bool                           `json:"publishesToStatusPage"`
	StatusPageCloud            *string                        `json:"statusPageCloud"`
	CreatedOn                  string                         `json:"createdOn"`
	CreatedBy                  string                         `json:"createdBy"`
	UpdatedOn                  string                         `json:"updatedOn"`
	UpdatedBy                  string                         `json:"updatedBy"`
}

// mapSNOutageToView maps a Choreo outage payload to the domain representation,
// shared by CreateOutage, SearchOutages, GetOutageByID, and UpdateOutage.
func mapSNOutageToView(o snOutage) domain.Outage {
	view := domain.Outage{
		ID:                    sysidToUUID(o.ID),
		Number:                o.Number,
		Type:                  o.Type,
		Status:                o.Status,
		Begin:                 o.Begin,
		End:                   o.End,
		Duration:              o.Duration,
		ShortDescription:      o.ShortDescription,
		PublishesToStatusPage: o.PublishesToStatusPage,
		StatusPageCloud:       o.StatusPageCloud,
		CreatedOn:             o.CreatedOn,
		CreatedBy:             o.CreatedBy,
		UpdatedOn:             o.UpdatedOn,
		UpdatedBy:             o.UpdatedBy,
	}
	if o.ConfigurationItem != nil {
		view.ConfigurationItem = &domain.OutageConfigurationItemRef{
			ID: sysidToUUID(o.ConfigurationItem.ID), Name: o.ConfigurationItem.Name, ClassName: o.ConfigurationItem.ClassName,
		}
	}
	if o.Incident != nil {
		view.Incident = &domain.OutageIncidentRef{
			ID: sysidToUUID(o.Incident.ID), Number: o.Incident.Number,
			ShortDescription: o.Incident.ShortDescription, State: o.Incident.State,
		}
	}
	if len(o.AffectedConfigurationItems) > 0 {
		affected := make([]domain.OutageConfigurationItemRef, 0, len(o.AffectedConfigurationItems))
		for _, ci := range o.AffectedConfigurationItems {
			affected = append(affected, domain.OutageConfigurationItemRef{ID: sysidToUUID(ci.ID), Name: ci.Name, ClassName: ci.ClassName})
		}
		view.AffectedConfigurationItems = affected
	}
	return view
}

var validOutageType = map[domain.OutageType]bool{
	domain.OutageTypeOutage:      true,
	domain.OutageTypeDegradation: true,
	domain.OutageTypePlanned:     true,
}

var validOutageStatus = map[domain.OutageStatus]bool{
	domain.OutageStatusInProgress: true,
	domain.OutageStatusResolved:   true,
}

var validOutageCommunicationChannel = map[domain.OutageCommunicationChannel]bool{
	domain.OutageCommunicationChannelExternal:   true,
	domain.OutageCommunicationChannelInternal:   true,
	domain.OutageCommunicationChannelAdditional: true,
}

var validOutageSortField = map[domain.OutageSortField]bool{
	domain.OutageSortFieldBegin:     true,
	domain.OutageSortFieldEnd:       true,
	domain.OutageSortFieldNumber:    true,
	domain.OutageSortFieldCreatedOn: true,
	domain.OutageSortFieldUpdatedOn: true,
}

var validOutageSortOrder = map[domain.OutageSortOrder]bool{
	domain.OutageSortOrderAsc:  true,
	domain.OutageSortOrderDesc: true,
}

type snOutageService struct {
	client *integrationservice.Client
}

// NewServiceNowOutageService constructs an OutageService backed by the Choreo API.
func NewServiceNowOutageService(client *integrationservice.Client) OutageService {
	return &snOutageService{client: client}
}

// snCreateOutagePayload is the Choreo POST /outages request body.
type snCreateOutagePayload struct {
	Type                         string  `json:"type"`
	Begin                        string  `json:"begin"`
	End                          *string `json:"end,omitempty"`
	ShortDescription             string  `json:"shortDescription"`
	ConfigurationItemID          *string `json:"configurationItemId,omitempty"`
	IncidentID                   *string `json:"incidentId,omitempty"`
	ExternalCommunication        *string `json:"externalCommunication,omitempty"`
	InternalCommunication        *string `json:"internalCommunication,omitempty"`
	AcknowledgePublicPublication *bool   `json:"acknowledgePublicPublication,omitempty"`
}

// snCreateOutageResponse mirrors the Choreo POST /outages response.
type snCreateOutageResponse struct {
	Message string   `json:"message"`
	Outage  snOutage `json:"outage"`
}

// CreateOutage implements OutageService for the ServiceNow data source.
func (s *snOutageService) CreateOutage(ctx context.Context, req domain.CreateOutageRequest) (domain.CreateOutageResponse, error) {
	if req.Type == "" {
		return domain.CreateOutageResponse{}, &apierror.ValidationError{Msg: "type is required"}
	}
	if !validOutageType[req.Type] {
		return domain.CreateOutageResponse{}, &apierror.ValidationError{Msg: "invalid type: " + string(req.Type)}
	}
	if strings.TrimSpace(req.Begin) == "" {
		return domain.CreateOutageResponse{}, &apierror.ValidationError{Msg: "begin is required"}
	}
	if strings.TrimSpace(req.ShortDescription) == "" {
		return domain.CreateOutageResponse{}, &apierror.ValidationError{Msg: "shortDescription is required"}
	}
	if len(req.ShortDescription) > 160 {
		return domain.CreateOutageResponse{}, &apierror.ValidationError{Msg: "shortDescription must be 160 characters or fewer"}
	}
	if req.ConfigurationItemID != nil {
		if err := validateUUIDs("configurationItemId", []string{*req.ConfigurationItemID}); err != nil {
			return domain.CreateOutageResponse{}, err
		}
	}
	if req.IncidentID != nil {
		if err := validateUUIDs("incidentId", []string{*req.IncidentID}); err != nil {
			return domain.CreateOutageResponse{}, err
		}
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snCreateOutagePayload{
		Type:                         string(req.Type),
		Begin:                        req.Begin,
		End:                          req.End,
		ShortDescription:             req.ShortDescription,
		ExternalCommunication:        req.ExternalCommunication,
		InternalCommunication:        req.InternalCommunication,
		AcknowledgePublicPublication: req.AcknowledgePublicPublication,
	}
	if req.ConfigurationItemID != nil {
		payload.ConfigurationItemID = strPtr(uuidToSysid(*req.ConfigurationItemID))
	}
	if req.IncidentID != nil {
		payload.IncidentID = strPtr(uuidToSysid(*req.IncidentID))
	}

	raw, err := s.client.Post(ctx, "/outages", token, payload)
	if err != nil {
		return domain.CreateOutageResponse{}, err
	}

	var snResp snCreateOutageResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.CreateOutageResponse{}, fmt.Errorf("sn create outage: parse response: %w", err)
	}

	return domain.CreateOutageResponse{Message: snResp.Message, Outage: mapSNOutageToView(snResp.Outage)}, nil
}

// snSearchOutagesPayload is the Choreo POST /outages/search request body.
// This is a CLOSED record on the Choreo side (OutageSearchPayload) with every
// field flat at the top level -- no `filters`/`pagination`/`sortBy` wrapper
// objects, unlike this service's other search payloads. Sending a nested
// shape here 400s (confirmed live: the field simply doesn't bind).
type snSearchOutagesPayload struct {
	Types                []string `json:"types,omitempty"`
	Statuses             []string `json:"statuses,omitempty"`
	ConfigurationItemIDs []string `json:"configurationItemIds,omitempty"`
	IncidentIDs          []string `json:"incidentIds,omitempty"`
	BeginFrom            string   `json:"beginFrom,omitempty"`
	BeginTo              string   `json:"beginTo,omitempty"`
	PublishedOnly        *bool    `json:"publishedOnly,omitempty"`
	SearchTerm           string   `json:"searchTerm,omitempty"`
	Limit                int      `json:"limit,omitempty"`
	Offset               int      `json:"offset,omitempty"`
	SortBy               string   `json:"sortBy,omitempty"`
	SortOrder            string   `json:"sortOrder,omitempty"`
}

// snSearchOutagesResponse mirrors the Choreo POST /outages/search response.
type snSearchOutagesResponse struct {
	Outages            []snOutage `json:"outages"`
	Offset             int        `json:"offset"`
	Limit              int        `json:"limit"`
	TotalRecords       int        `json:"totalRecords"`
	AppliedBeginFrom   string     `json:"appliedBeginFrom"`
	BeginFromDefaulted bool       `json:"beginFromDefaulted"`
}

// SearchOutages implements OutageService for the ServiceNow data source.
func (s *snOutageService) SearchOutages(ctx context.Context, req domain.SearchOutagesRequest) (domain.SearchOutagesResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchOutagesResponse{}, err
	}
	for _, t := range req.Filters.Types {
		if !validOutageType[t] {
			return domain.SearchOutagesResponse{}, &apierror.ValidationError{Msg: "types contains invalid value: " + string(t)}
		}
	}
	for _, st := range req.Filters.Statuses {
		if !validOutageStatus[st] {
			return domain.SearchOutagesResponse{}, &apierror.ValidationError{Msg: "statuses contains invalid value: " + string(st)}
		}
	}
	if err := validateUUIDs("configurationItemIds", req.Filters.ConfigurationItemIDs); err != nil {
		return domain.SearchOutagesResponse{}, err
	}
	if err := validateUUIDs("incidentIds", req.Filters.IncidentIDs); err != nil {
		return domain.SearchOutagesResponse{}, err
	}
	if req.SortBy.Field != "" && !validOutageSortField[req.SortBy.Field] {
		return domain.SearchOutagesResponse{}, &apierror.ValidationError{Msg: "sortBy.field contains invalid value: " + string(req.SortBy.Field)}
	}
	if req.SortBy.Order != "" && !validOutageSortOrder[req.SortBy.Order] {
		return domain.SearchOutagesResponse{}, &apierror.ValidationError{Msg: "sortBy.order contains invalid value: " + string(req.SortBy.Order)}
	}

	token := middleware.UserIDTokenFromContext(ctx)

	types := make([]string, 0, len(req.Filters.Types))
	for _, t := range req.Filters.Types {
		types = append(types, string(t))
	}
	statuses := make([]string, 0, len(req.Filters.Statuses))
	for _, st := range req.Filters.Statuses {
		statuses = append(statuses, string(st))
	}

	var sortOrder string
	if req.SortBy.Field != "" {
		sortOrder = string(req.SortBy.Order)
		if sortOrder == "" {
			sortOrder = "desc"
		}
	}

	payload := snSearchOutagesPayload{
		Types:                types,
		Statuses:             statuses,
		ConfigurationItemIDs: uuidsToSysids(req.Filters.ConfigurationItemIDs),
		IncidentIDs:          uuidsToSysids(req.Filters.IncidentIDs),
		BeginFrom:            stringPtrValue(req.Filters.BeginFrom),
		BeginTo:              stringPtrValue(req.Filters.BeginTo),
		PublishedOnly:        req.Filters.PublishedOnly,
		SearchTerm:           req.Filters.SearchTerm,
		Limit:                req.Pagination.Limit,
		Offset:               req.Pagination.Offset,
		SortBy:               string(req.SortBy.Field),
		SortOrder:            sortOrder,
	}

	raw, err := s.client.Post(ctx, "/outages/search", token, payload)
	if err != nil {
		return domain.SearchOutagesResponse{}, err
	}

	var snResp snSearchOutagesResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchOutagesResponse{}, fmt.Errorf("sn outages: parse search response: %w", err)
	}

	outages := make([]domain.Outage, 0, len(snResp.Outages))
	for _, o := range snResp.Outages {
		outages = append(outages, mapSNOutageToView(o))
	}

	return domain.SearchOutagesResponse{
		Outages:            outages,
		Total:              snResp.TotalRecords,
		Limit:              req.Pagination.Limit,
		Offset:             req.Pagination.Offset,
		AppliedBeginFrom:   snResp.AppliedBeginFrom,
		BeginFromDefaulted: snResp.BeginFromDefaulted,
	}, nil
}

// snOutageDetailInner mirrors the Choreo OutageDetails schema: the outage
// shape plus per-channel communication counts, both nested inside the
// response's "outage" key.
type snOutageDetailInner struct {
	snOutage
	CommunicationCounts struct {
		External   int `json:"external"`
		Internal   int `json:"internal"`
		Additional int `json:"additional"`
	} `json:"communicationCounts"`
}

// snOutageDetail mirrors the Choreo GET /outages/{id} response (OutageResponse):
// the detail object is wrapped under "outage", not flat at the top level --
// unmarshaling into a flat struct silently produced an all-zero-value result
// with no error, since json.Unmarshal ignores unmatched top-level keys.
type snOutageDetail struct {
	Outage snOutageDetailInner `json:"outage"`
}

// GetOutageByID implements OutageService for the ServiceNow data source.
func (s *snOutageService) GetOutageByID(ctx context.Context, id string) (domain.OutageDetail, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.OutageDetail{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	raw, err := s.client.Get(ctx, "/outages/"+uuidToSysid(id), token)
	if err != nil {
		return domain.OutageDetail{}, err
	}

	var sn snOutageDetail
	if err := json.Unmarshal(raw, &sn); err != nil {
		return domain.OutageDetail{}, fmt.Errorf("sn get outage: parse response: %w", err)
	}

	return domain.OutageDetail{
		Outage: mapSNOutageToView(sn.Outage.snOutage),
		CommunicationCounts: domain.OutageCommunicationCounts{
			External: sn.Outage.CommunicationCounts.External, Internal: sn.Outage.CommunicationCounts.Internal, Additional: sn.Outage.CommunicationCounts.Additional,
		},
	}, nil
}

// snPatchOutagePayload is the Choreo PATCH /outages/{id} request body. End is
// json.RawMessage so an explicit null ("end": null, which reopens the outage)
// can be distinguished from an omitted field -- omitempty drops a nil
// RawMessage entirely. See domain.PatchOutageRequest.
type snPatchOutagePayload struct {
	Type                         *string         `json:"type,omitempty"`
	Begin                        *string         `json:"begin,omitempty"`
	End                          json.RawMessage `json:"end,omitempty"`
	ShortDescription             *string         `json:"shortDescription,omitempty"`
	ConfigurationItemID          *string         `json:"configurationItemId,omitempty"`
	IncidentID                   *string         `json:"incidentId,omitempty"`
	AcknowledgePublicPublication *bool           `json:"acknowledgePublicPublication,omitempty"`
}

// snPatchOutageResponse mirrors the Choreo PATCH /outages/{id} response.
type snPatchOutageResponse struct {
	Message string   `json:"message"`
	Outage  snOutage `json:"outage"`
}

// UpdateOutage implements OutageService for the ServiceNow data source.
func (s *snOutageService) UpdateOutage(ctx context.Context, req domain.PatchOutageRequest) (domain.PatchOutageResponse, error) {
	if err := validateUUIDs("id", []string{req.ID}); err != nil {
		return domain.PatchOutageResponse{}, err
	}

	hasUpdate := req.Type != nil || req.Begin != nil || req.End != nil || req.ShortDescription != nil ||
		req.ConfigurationItemID != nil || req.IncidentID != nil || req.AcknowledgePublicPublication != nil
	if !hasUpdate {
		return domain.PatchOutageResponse{}, &apierror.ValidationError{Msg: "at least one field must be provided"}
	}

	if req.Type != nil && !validOutageType[*req.Type] {
		return domain.PatchOutageResponse{}, &apierror.ValidationError{Msg: "invalid type: " + string(*req.Type)}
	}
	if req.ConfigurationItemID != nil {
		if err := validateUUIDs("configurationItemId", []string{*req.ConfigurationItemID}); err != nil {
			return domain.PatchOutageResponse{}, err
		}
	}
	if req.IncidentID != nil {
		if err := validateUUIDs("incidentId", []string{*req.IncidentID}); err != nil {
			return domain.PatchOutageResponse{}, err
		}
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snPatchOutagePayload{
		Begin:                        req.Begin,
		ShortDescription:             req.ShortDescription,
		AcknowledgePublicPublication: req.AcknowledgePublicPublication,
	}
	if req.Type != nil {
		v := string(*req.Type)
		payload.Type = &v
	}
	if req.ConfigurationItemID != nil {
		payload.ConfigurationItemID = strPtr(uuidToSysid(*req.ConfigurationItemID))
	}
	if req.IncidentID != nil {
		payload.IncidentID = strPtr(uuidToSysid(*req.IncidentID))
	}
	if req.End != nil {
		var v any
		if *req.End != nil {
			v = **req.End
		}
		raw, err := rawJSONOrNull(v)
		if err != nil {
			return domain.PatchOutageResponse{}, fmt.Errorf("sn patch outage: marshal end: %w", err)
		}
		payload.End = raw
	}

	raw, err := s.client.Patch(ctx, "/outages/"+uuidToSysid(req.ID), token, payload)
	if err != nil {
		return domain.PatchOutageResponse{}, err
	}

	var snResp snPatchOutageResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.PatchOutageResponse{}, fmt.Errorf("sn patch outage: parse response: %w", err)
	}

	return domain.PatchOutageResponse{Message: snResp.Message, Outage: mapSNOutageToView(snResp.Outage)}, nil
}

// snAddOutageCommunicationPayload is the Choreo POST /outages/{id}/communications request body.
type snAddOutageCommunicationPayload struct {
	Channel string `json:"channel"`
	Body    string `json:"body"`
}

// snOutageCommunication mirrors a single Choreo outage communication entry.
type snOutageCommunication struct {
	ID        string `json:"id"`
	Channel   string `json:"channel"`
	Body      string `json:"body"`
	IsPublic  bool   `json:"isPublic"`
	CreatedOn string `json:"createdOn"`
	CreatedBy string `json:"createdBy"`
}

func mapSNOutageCommunicationToView(c snOutageCommunication) domain.OutageCommunication {
	return domain.OutageCommunication{
		ID:        sysidToUUID(c.ID),
		Channel:   domain.OutageCommunicationChannel(c.Channel),
		Body:      c.Body,
		IsPublic:  c.IsPublic,
		CreatedOn: c.CreatedOn,
		CreatedBy: c.CreatedBy,
	}
}

// snAddOutageCommunicationResponse mirrors the Choreo POST /outages/{id}/communications response.
type snAddOutageCommunicationResponse struct {
	Message       string                `json:"message"`
	Communication snOutageCommunication `json:"communication"`
}

// AddOutageCommunication implements OutageService for the ServiceNow data source.
func (s *snOutageService) AddOutageCommunication(ctx context.Context, req domain.AddOutageCommunicationRequest) (domain.AddOutageCommunicationResponse, error) {
	if err := validateUUIDs("id", []string{req.OutageID}); err != nil {
		return domain.AddOutageCommunicationResponse{}, err
	}
	if req.Channel == "" {
		return domain.AddOutageCommunicationResponse{}, &apierror.ValidationError{Msg: "channel is required"}
	}
	if !validOutageCommunicationChannel[req.Channel] {
		return domain.AddOutageCommunicationResponse{}, &apierror.ValidationError{Msg: "invalid channel: " + string(req.Channel)}
	}
	if strings.TrimSpace(req.Body) == "" {
		return domain.AddOutageCommunicationResponse{}, &apierror.ValidationError{Msg: "body is required"}
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snAddOutageCommunicationPayload{Channel: string(req.Channel), Body: req.Body}

	raw, err := s.client.Post(ctx, "/outages/"+uuidToSysid(req.OutageID)+"/communications", token, payload)
	if err != nil {
		return domain.AddOutageCommunicationResponse{}, err
	}

	var snResp snAddOutageCommunicationResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.AddOutageCommunicationResponse{}, fmt.Errorf("sn add outage communication: parse response: %w", err)
	}

	return domain.AddOutageCommunicationResponse{Message: snResp.Message, Communication: mapSNOutageCommunicationToView(snResp.Communication)}, nil
}

// snSearchOutageCommunicationsPayload is the Choreo POST /outages/{id}/communications/search
// request body. Closed record on the Choreo side (OutageCommunicationSearchPayload) -- flat,
// no pagination wrapper, same class of mismatch as snSearchOutagesPayload.
type snSearchOutageCommunicationsPayload struct {
	Channels []string `json:"channels,omitempty"`
	Limit    int      `json:"limit,omitempty"`
	Offset   int      `json:"offset,omitempty"`
}

// snSearchOutageCommunicationsResponse mirrors the Choreo POST /outages/{id}/communications/search response.
type snSearchOutageCommunicationsResponse struct {
	Communications []snOutageCommunication `json:"communications"`
	Offset         int                     `json:"offset"`
	Limit          int                     `json:"limit"`
	TotalRecords   int                     `json:"totalRecords"`
}

// SearchOutageCommunications implements OutageService for the ServiceNow data source.
func (s *snOutageService) SearchOutageCommunications(ctx context.Context, req domain.SearchOutageCommunicationsRequest) (domain.SearchOutageCommunicationsResponse, error) {
	if err := validateUUIDs("id", []string{req.OutageID}); err != nil {
		return domain.SearchOutageCommunicationsResponse{}, err
	}
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchOutageCommunicationsResponse{}, err
	}
	for _, c := range req.Channels {
		if !validOutageCommunicationChannel[c] {
			return domain.SearchOutageCommunicationsResponse{}, &apierror.ValidationError{Msg: "channels contains invalid value: " + string(c)}
		}
	}

	token := middleware.UserIDTokenFromContext(ctx)

	channels := make([]string, 0, len(req.Channels))
	for _, c := range req.Channels {
		channels = append(channels, string(c))
	}

	payload := snSearchOutageCommunicationsPayload{
		Channels: channels,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
	}

	raw, err := s.client.Post(ctx, "/outages/"+uuidToSysid(req.OutageID)+"/communications/search", token, payload)
	if err != nil {
		return domain.SearchOutageCommunicationsResponse{}, err
	}

	var snResp snSearchOutageCommunicationsResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchOutageCommunicationsResponse{}, fmt.Errorf("sn search outage communications: parse response: %w", err)
	}

	comms := make([]domain.OutageCommunication, 0, len(snResp.Communications))
	for _, c := range snResp.Communications {
		comms = append(comms, mapSNOutageCommunicationToView(c))
	}

	return domain.SearchOutageCommunicationsResponse{
		Communications: comms,
		Total:          snResp.TotalRecords,
		Limit:          req.Pagination.Limit,
		Offset:         req.Pagination.Offset,
	}, nil
}

// snOutageChoice mirrors a single {value, label} choice returned by outage
// metadata -- not a ChoiceListItem; the outage metadata endpoint emits
// `value`, not `id`, per the upstream entity-service's own contract.
type snOutageChoice struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// snOutageChannelMeta mirrors one communication-channel entry in outage
// metadata (OutageChannelChoice: value/label/isPublic).
type snOutageChannelMeta struct {
	Value    string `json:"value"`
	Label    string `json:"label"`
	IsPublic bool   `json:"isPublic"`
}

// snOutageMetadataResponse mirrors the upstream entity-service's
// GET /outages/metadata response, confirmed against its own contract -- the
// earlier {id,label}/`channels` guess was wrong on both counts.
type snOutageMetadataResponse struct {
	Types                 []snOutageChoice      `json:"types"`
	Statuses              []snOutageChoice      `json:"statuses"`
	CommunicationChannels []snOutageChannelMeta `json:"communicationChannels"`
	StatusPageClouds      []string              `json:"statusPageClouds"`
}

// GetOutageMetadata implements OutageService for the ServiceNow data source.
func (s *snOutageService) GetOutageMetadata(ctx context.Context) (domain.OutageMetadataResponse, error) {
	token := middleware.UserIDTokenFromContext(ctx)

	raw, err := s.client.Get(ctx, "/outages/metadata", token)
	if err != nil {
		return domain.OutageMetadataResponse{}, err
	}

	var sn snOutageMetadataResponse
	if err := json.Unmarshal(raw, &sn); err != nil {
		return domain.OutageMetadataResponse{}, fmt.Errorf("sn outage metadata: parse response: %w", err)
	}

	types := make([]domain.OutageChoice, 0, len(sn.Types))
	for _, t := range sn.Types {
		types = append(types, domain.OutageChoice{Value: t.Value, Label: t.Label})
	}
	statuses := make([]domain.OutageChoice, 0, len(sn.Statuses))
	for _, st := range sn.Statuses {
		statuses = append(statuses, domain.OutageChoice{Value: st.Value, Label: st.Label})
	}
	channels := make([]domain.OutageCommunicationChannelMeta, 0, len(sn.CommunicationChannels))
	for _, c := range sn.CommunicationChannels {
		channels = append(channels, domain.OutageCommunicationChannelMeta{Value: c.Value, Label: c.Label, IsPublic: c.IsPublic})
	}

	return domain.OutageMetadataResponse{
		Types:                 types,
		Statuses:              statuses,
		CommunicationChannels: channels,
		StatusPageClouds:      sn.StatusPageClouds,
	}, nil
}
