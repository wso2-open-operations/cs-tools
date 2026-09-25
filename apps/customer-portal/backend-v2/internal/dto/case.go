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

package dto

import (
	"strconv"
	"time"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// Ref is a compact {id, name} reference to another entity (project,
// deployment, product, assigned team, etc.).
type Ref struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Tag is a free-text label attached to a case (e.g. "Security Announcement",
// attached to every case a security announcement creates) — customer-
// appropriate as-is, unlike most of this file's trimming: a tag is written
// specifically to be visible, not an internal CSM annotation.
type Tag struct {
	ID    string  `json:"id"`
	Label string  `json:"label"`
	Color *string `json:"color,omitempty"`
}

func mapTags(tags []entity.Tag) []Tag {
	if len(tags) == 0 {
		return nil
	}
	out := make([]Tag, 0, len(tags))
	for _, t := range tags {
		out = append(out, Tag{ID: t.ID, Label: t.Label, Color: t.Color})
	}
	return out
}

func mapRef(r *entity.EntityRef) *Ref {
	if r == nil {
		return nil
	}
	return &Ref{ID: r.ID, Name: r.Name}
}

// NumberRef is a compact reference to a case carrying its human-readable number.
type NumberRef struct {
	ID     string `json:"id"`
	Number string `json:"number"`
}

func mapNumberRef(r *entity.CaseNumberRef) *NumberRef {
	if r == nil {
		return nil
	}
	return &NumberRef{ID: r.ID, Number: r.Number}
}

// CaseSummary is one item of the portal's response for
// POST /projects/{id}/cases/search — shaped to match the frontend's
// CaseListItem type (apps/customer-portal/webapp/src/features/support/types/
// cases.ts) field-for-field, not entity-service's own SearchCaseView. Every
// enum-valued field (status, severity, issueType, engagementType, type) is
// an IDLabelRef, not a plain string, and status/severity/issueType/
// engagementType's .id is a ServiceNow numeric choice-list key the frontend
// still expects — see case_enum_mapping.go for the translation. Deliberately
// excludes entity-service's Catalog/CatalogItem (CMDB implementation
// detail), ParentCase/RelatedCase, Conversation — CaseListItem has no
// equivalent fields; the full picture is available via GET /cases/{id}.
type CaseSummary struct {
	ID               string      `json:"id"`
	InternalID       string      `json:"internalId,omitempty"`
	Number           string      `json:"number"`
	Title            *string     `json:"title,omitempty"`
	Description      *string     `json:"description,omitempty"`
	AssignedEngineer *IDLabelRef `json:"assignedEngineer,omitempty"`
	Project          IDLabelRef  `json:"project"`
	IssueType        *IDLabelRef `json:"issueType,omitempty"`
	EngagementType   *IDLabelRef `json:"engagementType,omitempty"`
	DeployedProduct  *IDLabelRef `json:"deployedProduct,omitempty"`
	Deployment       *IDLabelRef `json:"deployment,omitempty"`
	Severity         *IDLabelRef `json:"severity,omitempty"`
	Status           *IDLabelRef `json:"status,omitempty"`
	Type             *IDLabelRef `json:"type,omitempty"`
	CreatedOn        string      `json:"createdOn,omitempty"`
	UpdatedOn        string      `json:"updatedOn,omitempty"`
	CreatedBy        string      `json:"createdBy,omitempty"`
}

// SearchCasesResponse is the portal's response for
// POST /projects/{id}/cases/search. TotalRecords (not Total) to match the
// frontend's shared pagination envelope — see PaginationResponse in
// apps/customer-portal/webapp/src/types/common.ts.
type SearchCasesResponse struct {
	Cases        []CaseSummary `json:"cases"`
	TotalRecords int           `json:"totalRecords"`
	Limit        int           `json:"limit"`
	Offset       int           `json:"offset"`
}

func entityRefToIDLabel(r *entity.EntityRef) *IDLabelRef {
	if r == nil {
		return nil
	}
	return &IDLabelRef{ID: r.ID, Label: r.Name}
}

func assignedEngineerToIDLabel(r *entity.AssignedEngineerRef) *IDLabelRef {
	if r == nil {
		return nil
	}
	return &IDLabelRef{ID: r.ID, Label: r.Name}
}

// MapSearchCases builds the portal response from entity-service's SearchCasesResponse.
func MapSearchCases(r entity.SearchCasesResponse) SearchCasesResponse {
	cases := make([]CaseSummary, 0, len(r.Cases))
	for _, c := range r.Cases {
		cases = append(cases, CaseSummary{
			ID:               c.ID,
			InternalID:       c.InternalID,
			Number:           c.Number,
			Title:            c.Subject,
			Description:      c.Description,
			AssignedEngineer: assignedEngineerToIDLabel(c.AssignedEngineer),
			Project:          IDLabelRef{ID: c.Project.ID, Label: c.Project.Name},
			IssueType:        caseIssueTypeRef(c.IssueType),
			EngagementType:   caseEngagementTypeRef(c.EngagementType),
			DeployedProduct:  entityRefToIDLabel(c.DeployedProduct),
			Deployment:       entityRefToIDLabel(c.Deployment),
			Severity:         caseSeverityRef(c.Severity),
			Status:           caseStatusRef(c.State),
			Type:             caseTypeRef(c.Type),
			CreatedOn:        c.CreatedOn,
			UpdatedOn:        c.UpdatedOn,
			CreatedBy:        userRefIdentity(c.CreatedBy),
		})
	}
	return SearchCasesResponse{
		Cases:        cases,
		TotalRecords: r.Total,
		Limit:        r.Limit,
		Offset:       r.Offset,
	}
}

// currentUserFilterPlaceholder must match entity-service's own
// currentUserFilterPlaceholder (case_filters.go) exactly — it's the literal
// values entry a createdBy+eq filter must carry to mean "the authenticated
// caller".
const currentUserFilterPlaceholder = "__current_user_email__"

// CaseSearchFilters holds the optional filter criteria for
// POST /projects/{id}/cases/search — shaped to match the frontend's own
// CaseSearchFilters type (apps/customer-portal/webapp/src/features/support/
// types/cases.ts) field-for-field, the same type every case-search call site
// in the frontend shares. StatusIDs/SeverityIDs/IssueIDs/EngagementTypeKeys
// carry ServiceNow's numeric choice-list ids (the frontend was built against
// the old Ballerina backend, which forwarded these ids as-is) — see
// case_enum_mapping.go for how they're translated into entity-service's own
// enum vocabulary. CaseTypes needs no translation: entity-service's own
// "type"+"in" filter already accepts "default_case" as an alias for "case"
// (case_service.go's caseTypeAliases), and every other case type value is
// identical between the two.
//
// No ProjectIDs field: project scoping comes exclusively from the {id} path
// parameter (see BuildEntitySearchCasesRequest's projectID parameter), never
// from the request body — the frontend has never sent one, and letting the
// body set it would invite exactly the kind of unsatisfiable-AND-filter bug
// CreatedBy/CreatedByMe had (a body projectIds disagreeing with the path
// silently returning an empty result instead of erroring or being ignored).
type CaseSearchFilters struct {
	SearchQuery        string   `json:"searchQuery,omitempty"`
	StatusIDs          []int    `json:"statusIds,omitempty"`
	CaseTypes          []string `json:"caseTypes,omitempty"`
	SeverityIDs        []int    `json:"severityIds,omitempty"`
	IssueIDs           []int    `json:"issueIds,omitempty"`
	DeploymentIDs      []string `json:"deploymentIds,omitempty"`
	CreatedByMe        bool     `json:"createdByMe,omitempty"`
	CreatedBy          []string `json:"createdBy,omitempty"`
	EngagementTypeKeys []int    `json:"engagementTypeKeys,omitempty"`
	ClosedStartDate    string   `json:"closedStartDate,omitempty"`
	ClosedEndDate      string   `json:"closedEndDate,omitempty"`
	StartCreatedDate   string   `json:"startCreatedDate,omitempty"`
	EndCreatedDate     string   `json:"endCreatedDate,omitempty"`
	StartUpdatedDate   string   `json:"startUpdatedDate,omitempty"`
	EndUpdatedDate     string   `json:"endUpdatedDate,omitempty"`
}

// CaseSearchRequest is the portal's request body for
// POST /projects/{id}/cases/search.
type CaseSearchRequest struct {
	Filters    CaseSearchFilters `json:"filters"`
	SortBy     entity.CaseSort   `json:"sortBy"`
	Pagination entity.Pagination `json:"pagination"`
}

// BuildEntitySearchCasesRequest translates the portal's named-filter-field
// request into entity-service's current generic filter-predicate array
// contract (see entity.CaseFieldFilter) — every filter the portal exposes
// becomes one "field in/eq/gte/lte values" entry; SearchQuery, SortBy, and
// Pagination pass straight through unchanged. projectID (the {id} path
// parameter from POST /projects/{id}/cases/search — never the request body)
// always becomes a projectId+in filter with that single value, scoping every
// search to the one project in the URL. CreatedByMe becomes a createdBy+eq
// filter carrying currentUserFilterPlaceholder, exactly as entity-service's
// own case_filters.go expects, resolving the caller's identity server-side
// from the forwarded x-user-id-token. CreatedByMe takes precedence over
// CreatedBy when both are set: entity-service's filters array is AND-only,
// so a createdBy+in filter and a createdBy+eq filter together could never
// both match (barring CreatedBy coincidentally containing the caller's own
// email), silently returning an empty result set — CreatedBy is dropped
// entirely rather than combined.
func BuildEntitySearchCasesRequest(projectID string, req CaseSearchRequest) entity.SearchCasesRequest {
	var filters []entity.CaseFieldFilter

	addIn := func(field string, values []string) {
		if len(values) > 0 {
			filters = append(filters, entity.CaseFieldFilter{Field: field, Op: "in", Values: values})
		}
	}
	addDateRange := func(field, gte, lte string) {
		if gte != "" {
			filters = append(filters, entity.CaseFieldFilter{Field: field, Op: "gte", Values: []string{gte}})
		}
		if lte != "" {
			filters = append(filters, entity.CaseFieldFilter{Field: field, Op: "lte", Values: []string{lte}})
		}
	}

	filters = append(filters, entity.CaseFieldFilter{Field: "projectId", Op: "in", Values: []string{projectID}})
	addIn("type", req.Filters.CaseTypes)
	addIn("state", caseIDsToEnums(req.Filters.StatusIDs, caseStateIDToEnum))
	addIn("severity", caseIDsToEnums(req.Filters.SeverityIDs, caseSeverityIDToEnum))
	addIn("issueType", caseIDsToEnums(req.Filters.IssueIDs, caseIssueTypeIDToEnum))
	addIn("engagementType", caseIDsToEnums(req.Filters.EngagementTypeKeys, caseEngagementTypeIDToEnum))
	addIn("deploymentId", req.Filters.DeploymentIDs)
	addDateRange("createdOn", req.Filters.StartCreatedDate, req.Filters.EndCreatedDate)
	addDateRange("updatedOn", req.Filters.StartUpdatedDate, req.Filters.EndUpdatedDate)
	addDateRange("closedOn", req.Filters.ClosedStartDate, req.Filters.ClosedEndDate)

	// CreatedByMe takes precedence over CreatedBy: entity-service's filters
	// array is AND-only (case_filters.go), so a createdBy+in filter and a
	// createdBy+eq filter together could never both match (barring CreatedBy
	// coincidentally containing the caller's own email) — silently returning
	// an empty result set instead of the caller's own cases. A client
	// shouldn't send both, but if it does, honor the explicit "my cases"
	// intent rather than the list.
	if req.Filters.CreatedByMe {
		filters = append(filters, entity.CaseFieldFilter{Field: "createdBy", Op: "eq", Values: []string{currentUserFilterPlaceholder}})
	} else {
		addIn("createdBy", req.Filters.CreatedBy)
	}

	return entity.SearchCasesRequest{
		Filters: entity.SearchCasesFilters{
			SearchQuery: req.Filters.SearchQuery,
			Filters:     filters,
		},
		SortBy:     req.SortBy,
		Pagination: req.Pagination,
	}
}

// CaseDetailsAccount is the account reference embedded in CaseDetails,
// matching the frontend's own CaseDetailsAccount type ({type, id, label}).
type CaseDetailsAccount struct {
	Type  string `json:"type"`
	ID    string `json:"id"`
	Label string `json:"label"`
}

// CaseDetailsAssignedEngineer matches the frontend's own
// CaseDetailsAssignedEngineer type.
type CaseDetailsAssignedEngineer struct {
	ID    string `json:"id"`
	Label string `json:"label,omitempty"`
	Name  string `json:"name,omitempty"`
}

// CaseWatchListUser is one entry of CaseDetails.WatchList, matching the
// frontend's inline watchList item shape.
type CaseWatchListUser struct {
	ID       string `json:"id,omitempty"`
	UserName string `json:"userName,omitempty"`
	Name     string `json:"name,omitempty"`
	Email    string `json:"email,omitempty"`
}

// CaseDetails is the portal's response for GET /cases/{id} — shaped to
// match the frontend's own CaseDetails type
// (apps/customer-portal/webapp/src/features/support/types/cases.ts)
// field-for-field: Title (not Subject), Status (not State), every
// enum-valued field as IDLabelRef, every reference field as IDLabelRef (not
// Ref), plus InternalID/Account/ChangeRequests/WatchList, which
// entity-service's CaseView already carries but this dto previously left
// unmapped (some under an inaccurate "CSM-engineer-only" doc comment — see
// entity.CaseView's own doc comment on WatchList, which says otherwise).
//
// Deliberately excludes entity-service's AutoclosureStep/AutoclosureStateTime
// and BestCaseFixEta/MostLikelyFixEta/WorstCaseFixEta — genuinely
// CSM-engineer-facing only. CsManager and FindingsResolved/FindingsTotal
// have no entity-service equivalent on CaseView. CloseNotes and ResolutionNotes
// are deliberately NOT exposed here even though entity-service's CaseView carries them:
// they are internal CS-agent close/resolution notes, never meant for
// the customer-facing view.
type CaseDetails struct {
	ID               string                       `json:"id"`
	InternalID       string                       `json:"internalId"`
	Number           string                       `json:"number"`
	Title            string                       `json:"title"`
	Description      string                       `json:"description"`
	Product          *IDLabelRef                  `json:"product,omitempty"`
	Account          *CaseDetailsAccount          `json:"account,omitempty"`
	AssignedEngineer *CaseDetailsAssignedEngineer `json:"assignedEngineer,omitempty"`
	Project          *IDLabelRef                  `json:"project,omitempty"`
	Type             *IDLabelRef                  `json:"type,omitempty"`
	DeployedProduct  *IDLabelRef                  `json:"deployedProduct,omitempty"`
	RelatedCase      *IDLabelRef                  `json:"relatedCase,omitempty"`
	Conversation     *IDLabelRef                  `json:"conversation,omitempty"`
	IssueType        *IDLabelRef                  `json:"issueType,omitempty"`
	EngagementType   *IDLabelRef                  `json:"engagementType,omitempty"`
	Catalog          *IDLabelRef                  `json:"catalog,omitempty"`
	CatalogItem      *IDLabelRef                  `json:"catalogItem,omitempty"`
	ChangeRequests   []IDLabelRef                 `json:"changeRequests,omitempty"`
	AssignedTeam     *IDLabelRef                  `json:"assignedTeam,omitempty"`
	Deployment       *IDLabelRef                  `json:"deployment,omitempty"`
	Severity         *IDLabelRef                  `json:"severity,omitempty"`
	Status           *IDLabelRef                  `json:"status,omitempty"`
	CreatedOn        time.Time                    `json:"createdOn"`
	UpdatedOn        time.Time                    `json:"updatedOn"`
	ClosedOn         *time.Time                   `json:"closedOn,omitempty"`
	CreatedBy        string                       `json:"createdBy"`
	ParentCase       *NumberRef                   `json:"parentCase,omitempty"`
	ResolvedOn       *time.Time                   `json:"resolvedOn,omitempty"`
	WatchList        []CaseWatchListUser          `json:"watchList,omitempty"`
	// Exposed because the frontend's CaseDetails type declares them
	// (features/support/types/cases.ts). AcknowledgedBy and
	// EngagementPaymentType are decoded upstream but deliberately NOT exposed:
	// no frontend consumer, so per CLAUDE.md they stay trimmed until one exists.
	SLAResponseTime     *string `json:"slaResponseTime,omitempty"`
	ClosedBy            *Ref    `json:"closedBy,omitempty"`
	HasAutoClosed       *bool   `json:"hasAutoClosed,omitempty"`
	EngagementStartDate *string `json:"engagementStartDate,omitempty"`
	EngagementEndDate   *string `json:"engagementEndDate,omitempty"`
	// Duration is the upstream's humanised elapsed time, rendered as-is.
	Duration *string `json:"duration,omitempty"`
	// EscalationLevel is an {id, label} ref because that is what the frontend
	// reads (CaseDetailsContent: data?.escalationLevel?.id). entity-service
	// exposes only the level id, per its no-{id,label} response convention, so
	// the label is built here — the same split as case status and severity.
	EscalationLevel *IDLabelRef `json:"escalationLevel,omitempty"`
	IsEscalated     *bool       `json:"isEscalated,omitempty"`
	Tags            []Tag       `json:"tags,omitempty"`
	// AnnouncementType is only meaningful when Type.ID is "announcement" --
	// "GENERAL" or "SECURITY". Nil for every other case-like type.
	AnnouncementType *string `json:"announcementType,omitempty"`
}

// MapCaseDetails builds the portal response from entity-service's CaseView.
func MapCaseDetails(c entity.CaseView) CaseDetails {
	var account *CaseDetailsAccount
	if c.AccountDetails != nil {
		account = &CaseDetailsAccount{Type: c.AccountDetails.Type, ID: c.AccountDetails.ID, Label: c.AccountDetails.Name}
	}

	var assignedEngineer *CaseDetailsAssignedEngineer
	if c.AssignedEngineer != nil {
		assignedEngineer = &CaseDetailsAssignedEngineer{ID: c.AssignedEngineer.ID, Label: c.AssignedEngineer.Name, Name: c.AssignedEngineer.Name}
	}

	var relatedCase *IDLabelRef
	if c.RelatedCase != nil {
		relatedCase = &IDLabelRef{ID: c.RelatedCase.ID, Label: c.RelatedCase.Number}
	}

	changeRequests := make([]IDLabelRef, 0, len(c.LinkedChangeRequests))
	for _, cr := range c.LinkedChangeRequests {
		label := cr.Number
		if cr.Name != nil {
			label = *cr.Name
		}
		changeRequests = append(changeRequests, IDLabelRef{ID: cr.ID, Label: label})
	}

	var watchList []CaseWatchListUser
	if len(c.WatchList) > 0 {
		watchList = make([]CaseWatchListUser, 0, len(c.WatchList))
		for _, w := range c.WatchList {
			watchList = append(watchList, CaseWatchListUser{ID: w.ID, UserName: w.UserName, Name: w.Name, Email: w.Email})
		}
	}

	return CaseDetails{
		ID:                  c.ID,
		InternalID:          c.InternalID,
		Number:              c.Number,
		Title:               c.Subject,
		Description:         c.Description,
		Product:             entityRefToIDLabel(c.ProductDetails),
		Account:             account,
		AssignedEngineer:    assignedEngineer,
		Project:             &IDLabelRef{ID: c.ProjectDetails.ID, Label: c.ProjectDetails.Name},
		Type:                caseTypeRefFromPointer(c.Type),
		DeployedProduct:     deployedProductRefToIDLabel(c.DeployedProductDetails),
		RelatedCase:         relatedCase,
		Conversation:        entityRefToIDLabel(c.Conversation),
		IssueType:           caseIssueTypeRef(&c.IssueType),
		EngagementType:      caseEngagementTypeRef(c.EngagementType),
		Catalog:             entityRefToIDLabel(c.Catalog),
		CatalogItem:         entityRefToIDLabel(c.CatalogItem),
		ChangeRequests:      changeRequests,
		AssignedTeam:        entityRefToIDLabel(c.AssignedTeam),
		Deployment:          entityRefToIDLabel(c.DeploymentDetails),
		Severity:            caseSeverityRef(&c.Severity),
		Status:              caseStatusRef(c.State),
		CreatedOn:           c.CreatedOn,
		UpdatedOn:           c.UpdatedOn,
		ClosedOn:            c.ClosedOn,
		CreatedBy:           c.CreatedByDetails.Name,
		ParentCase:          mapNumberRef(c.ParentCase),
		ResolvedOn:          c.ResolvedOn,
		WatchList:           watchList,
		SLAResponseTime:     c.SLAResponseTime,
		ClosedBy:            mapRef(c.ClosedBy),
		HasAutoClosed:       c.HasAutoClosed,
		EngagementStartDate: c.EngagementStartDate,
		EngagementEndDate:   c.EngagementEndDate,
		Duration:            c.Duration,
		EscalationLevel:     caseEscalationLevelRef(c.EscalationLevel),
		IsEscalated:         c.IsEscalated,
		Tags:                mapTags(c.Tags),
		AnnouncementType:    c.AnnouncementType,
	}
}

// caseTypeRefFromPointer mirrors caseTypeRef for CaseView.Type, which is a
// *string (nullable) unlike SearchCaseView.Type (always present).
func caseTypeRefFromPointer(t *string) *IDLabelRef {
	if t == nil {
		return nil
	}
	return caseTypeRef(*t)
}

// deployedProductRefToIDLabel maps entity-service's DeployedProductRef
// (id + displayName, where displayName is a "product name + version"
// concatenation with no separately-addressable version field) to an
// IDLabelRef. The frontend's CaseDetailsDeployedProduct type additionally
// declares an optional version field, but there's no clean, lossless way to
// split it back out of DisplayName, so it's left unset rather than guessed.
func deployedProductRefToIDLabel(r *entity.DeployedProductRef) *IDLabelRef {
	if r == nil {
		return nil
	}
	return &IDLabelRef{ID: r.ID, Label: r.DisplayName}
}

// CaseCreateAttachment is one file attached at case-creation time, matching
// the frontend's CreateCaseRequest.attachments entry ({file, name}) — the
// same shape as entity.CaseAttachment, kept as its own type only for this
// package's "always mirror the frontend's own type name" convention.
type CaseCreateAttachment struct {
	Name string `json:"name"`
	File string `json:"file"`
}

// CreateCaseRequest is the portal's request body for POST /cases — shaped to
// match the frontend's own CreateCaseRequest type
// (apps/customer-portal/webapp/src/features/support/types/cases.ts)
// field-for-field. SeverityKey/IssueTypeKey carry ServiceNow's numeric
// choice-list ids (the frontend was built against the old Ballerina backend
// and still sends these, not entity-service's own string enum) — see
// case_enum_mapping.go for the translation. CatalogID/CatalogItemID/
// Variables aren't in the frontend's current CreateCaseRequest TS type (no
// caller creates a service_request case today), but are kept here as
// optional fields matching entity-service's/the old Ballerina backend's
// CaseCreatePayload contract (modules/entity/types.bal), so that flow works
// unmodified if the frontend adds it later.
type CreateCaseRequest struct {
	Title             string                 `json:"title"`
	Type              string                 `json:"type,omitempty"`
	ProjectID         string                 `json:"projectId"`
	DeploymentID      string                 `json:"deploymentId"`
	DeployedProductID string                 `json:"deployedProductId,omitempty"`
	Description       string                 `json:"description"`
	SeverityKey       *int                   `json:"severityKey,omitempty"`
	IssueTypeKey      *int                   `json:"issueTypeKey,omitempty"`
	CatalogID         string                 `json:"catalogId,omitempty"`
	CatalogItemID     string                 `json:"catalogItemId,omitempty"`
	Variables         []entity.Variable      `json:"variables,omitempty"`
	RelatedCaseID     string                 `json:"relatedCaseId,omitempty"`
	ConversationID    string                 `json:"conversationId,omitempty"`
	WatchList         []string               `json:"watchList,omitempty"`
	Attachments       []CaseCreateAttachment `json:"attachments,omitempty"`
}

// BuildEntityCreateCaseRequest translates the portal's request into
// entity-service's CreateCaseRequest. An unrecognized (or absent)
// SeverityKey/IssueTypeKey translates to an empty Severity/IssueType, which
// entity-service's own validation then rejects with 400 for case types that
// require them — this backend doesn't duplicate that validation itself.
func BuildEntityCreateCaseRequest(req CreateCaseRequest) entity.CreateCaseRequest {
	var severity, issueType string
	if req.SeverityKey != nil {
		severity = caseSeverityIDToEnum[strconv.Itoa(*req.SeverityKey)]
	}
	if req.IssueTypeKey != nil {
		issueType = caseIssueTypeIDToEnum[strconv.Itoa(*req.IssueTypeKey)]
	}
	attachments := make([]entity.CaseAttachment, 0, len(req.Attachments))
	for _, a := range req.Attachments {
		attachments = append(attachments, entity.CaseAttachment{Name: a.Name, File: a.File})
	}
	return entity.CreateCaseRequest{
		Type:              req.Type,
		ProjectID:         req.ProjectID,
		DeploymentID:      req.DeploymentID,
		DeployedProductID: req.DeployedProductID,
		Subject:           req.Title,
		Description:       req.Description,
		Severity:          severity,
		IssueType:         issueType,
		CatalogID:         req.CatalogID,
		CatalogItemID:     req.CatalogItemID,
		Variables:         req.Variables,
		RelatedCaseID:     req.RelatedCaseID,
		ConversationID:    req.ConversationID,
		WatchList:         req.WatchList,
		Attachments:       attachments,
	}
}

// CaseCreateResponse is the portal's response for POST /cases, matching the
// frontend's CreateCaseResponse type field-for-field.
type CaseCreateResponse struct {
	ID         string      `json:"id"`
	InternalID string      `json:"internalId,omitempty"`
	Number     string      `json:"number"`
	CreatedOn  time.Time   `json:"createdOn"`
	State      *IDLabelRef `json:"state,omitempty"`
	Type       *IDLabelRef `json:"type,omitempty"`
}

// MapCaseCreate builds the portal response from entity-service's
// CreateCaseResponse. Type is left nil: entity-service's CreateCaseDetails
// doesn't return the case type at all, so there's nothing to translate.
func MapCaseCreate(r entity.CreateCaseResponse) CaseCreateResponse {
	return CaseCreateResponse{
		ID:         r.Case.ID,
		InternalID: r.Case.InternalID,
		Number:     r.Case.Number,
		CreatedOn:  r.Case.CreatedOn,
		State:      caseStatusRef(r.Case.State),
	}
}

// UpdateCaseRequest is the portal's request shape for PATCH /cases/{id} —
// matches the frontend's own PatchCaseRequest type
// (apps/customer-portal/webapp/src/features/support/types/cases.ts) and the
// old Ballerina backend's CaseUpdatePayload (modules/entity/types.bal)
// exactly: only stateKey and watchList. Every other field
// entity.UpdateCaseRequest supports (severity, subject, description,
// resolutionCode, cause, closeNotes, and every internal WSO2 support
// operation — workState, assigneeEmail, case relinking, autocloseHoldUntil,
// fix-commitment dates) is neither sent by the frontend today nor part of
// this endpoint's real contract; don't reintroduce them speculatively.
// StateKey carries ServiceNow's numeric choice-list id (the frontend was
// built against the old Ballerina backend and still sends this, not
// entity-service's own string enum) — see case_enum_mapping.go for the
// translation. entity-service requires exactly one of State/WatchList (of
// the fields this portal DTO exposes) to be set — StateKey counts as State
// for that check.
//
// WatchList cannot be used to clear every watcher via an explicit empty
// array — this is a genuine end-to-end platform limitation, not something
// fixable in this dto layer: entity-service's own ServiceNow implementation
// checks `len(req.WatchList) > 0` before forwarding it upstream at all (see
// sn_case_service.go), so even an explicit `{"watchList": []}` would be
// silently dropped several layers up, regardless of whether this struct
// distinguished absent from empty. Validating watchList as "must be
// non-empty to count as provided" (see the handler) is therefore accurate,
// not a bug — don't change this to a `*[]string` to "fix" an empty-array
// case that entity-service can't honor anyway.
type UpdateCaseRequest struct {
	StateKey  *int     `json:"stateKey,omitempty"`
	WatchList []string `json:"watchList,omitempty"`
}

// BuildEntityUpdateCaseRequest converts the portal's restricted update
// request into entity-service's full request shape, leaving every excluded
// field zero/nil. An unrecognized StateKey translates to an empty State,
// which entity-service's own validation then rejects with 400.
func BuildEntityUpdateCaseRequest(id string, req UpdateCaseRequest) entity.UpdateCaseRequest {
	var state *string
	if req.StateKey != nil {
		s := caseStateIDToEnum[strconv.Itoa(*req.StateKey)]
		state = &s
	}
	return entity.UpdateCaseRequest{
		ID:        id,
		State:     state,
		WatchList: req.WatchList,
	}
}

// CaseUpdateResponse is the portal's response for PATCH /cases/{id}.
// Matches Ballerina v1 and the webapp's PatchCaseResponse contract.
type CaseUpdateResponse struct {
	ID        string    `json:"id"`
	UpdatedOn time.Time `json:"updatedOn"`
	State     string    `json:"state,omitempty"`
	UpdatedBy string    `json:"updatedBy,omitempty"`
	WatchList []string  `json:"watchList,omitempty"`
}

// MapCaseUpdate builds the portal response from entity-service's UpdateCaseResponse.
func MapCaseUpdate(r entity.UpdateCaseResponse) CaseUpdateResponse {
	c := r.Case

	var watchList []string
	if len(c.WatchList) > 0 {
		watchList = make([]string, 0, len(c.WatchList))
		for _, w := range c.WatchList {
			if w.Email != "" {
				watchList = append(watchList, w.Email)
			} else {
				watchList = append(watchList, w.UserName)
			}
		}
	}

	return CaseUpdateResponse{
		ID:        c.ID,
		UpdatedOn: c.UpdatedOn,
		State:     c.State,
		UpdatedBy: c.UpdatedBy,
		WatchList: watchList,
	}
}

// CaseCommentRequest is the portal's request shape for POST /cases/{id}/comments.
// entity-service also supports "work_note" and "activity" comment types, but
// those are internal WSO2 support annotations — the customer portal only
// ever lets a customer post a plain "comment"; see
// BuildEntityCreateCaseCommentRequest.
type CaseCommentRequest struct {
	Content string `json:"content"`
}

// BuildEntityCreateCaseCommentRequest converts the portal's comment request
// into entity-service's request shape, forcing Type to "comment" regardless
// of anything the caller might otherwise try to set.
func BuildEntityCreateCaseCommentRequest(caseID string, req CaseCommentRequest) entity.CreateCaseCommentRequest {
	return entity.CreateCaseCommentRequest{
		CaseID:  caseID,
		Type:    entity.CommentTypeComment,
		Content: req.Content,
	}
}

// CaseCommentResponse is the portal's response for POST /cases/{id}/comments.
type CaseCommentResponse struct {
	ID        string    `json:"id"`
	CreatedOn time.Time `json:"createdOn"`
	CreatedBy string    `json:"createdBy"`
}

// MapCaseComment builds the portal response from entity-service's CreateCaseCommentResponse.
func MapCaseComment(r entity.CreateCaseCommentResponse) CaseCommentResponse {
	return CaseCommentResponse{
		ID:        r.Comment.ID,
		CreatedOn: r.Comment.CreatedOn,
		CreatedBy: r.Comment.CreatedBy,
	}
}

// CaseFieldChange describes a single field's value change in a case's activity feed.
type CaseFieldChange struct {
	Field         string `json:"field"`
	FieldLabel    string `json:"fieldLabel"`
	PreviousValue string `json:"previousValue"`
	NewValue      string `json:"newValue"`
}

// CaseActivity is one entry in the portal's response for
// POST /cases/{id}/activities/search — a discriminated union on Type, like
// entity-service's CaseActivity. CreatedByFirstName/CreatedByLastName/
// CreatedByFullName are all read as distinct fields by the frontend
// (useGetCaseCommentsInfinite.ts), so all three are carried through — not
// redundant despite an earlier assumption here. Deliberately excludes the
// raw internal actor ID.
type CaseActivity struct {
	ID                 string            `json:"id"`
	Type               string            `json:"type"`
	Content            string            `json:"content,omitempty"`
	CreatedOn          time.Time         `json:"createdOn"`
	CreatedBy          string            `json:"createdBy"`
	CreatedByFirstName string            `json:"createdByFirstName,omitempty"`
	CreatedByLastName  string            `json:"createdByLastName,omitempty"`
	CreatedByFullName  string            `json:"createdByFullName,omitempty"`
	CommentType        *string           `json:"commentType,omitempty"`
	FileName           string            `json:"fileName,omitempty"`
	ContentType        string            `json:"contentType,omitempty"`
	SizeBytes          int               `json:"sizeBytes,omitempty"`
	DownloadURL        string            `json:"downloadUrl,omitempty"`
	Changes            []CaseFieldChange `json:"changes,omitempty"`
}

// SearchCaseActivitiesResponse is the portal's response for
// POST /cases/{id}/activities/search — Activities (not Activity) and
// TotalRecords (not Total) to match the frontend's actual read sites
// (useGetCaseCommentsInfinite.ts reads data.activities/data.totalRecords).
type SearchCaseActivitiesResponse struct {
	Activities   []CaseActivity `json:"activities"`
	TotalRecords int            `json:"totalRecords"`
	Limit        int            `json:"limit"`
	Offset       int            `json:"offset"`
	HasMore      bool           `json:"hasMore"`
}

// MapSearchCaseActivities builds the portal response from entity-service's SearchCaseActivitiesResponse.
func MapSearchCaseActivities(r entity.SearchCaseActivitiesResponse) SearchCaseActivitiesResponse {
	items := make([]CaseActivity, 0, len(r.Activity))
	for _, a := range r.Activity {
		// Work notes (internal WSO2 support annotations) must never reach the customer portal.
		if a.Type == "work_note" || a.Type == "work_notes" {
			continue
		}
		if a.CommentType != nil {
			ct := string(*a.CommentType)
			if ct == string(entity.CommentTypeWorkNote) || ct == "work_note" || ct == "work_notes" {
				continue
			}
		}

		var commentType *string
		if a.CommentType != nil {
			s := string(*a.CommentType)
			commentType = &s
		}

		var changes []CaseFieldChange
		if len(a.Changes) > 0 {
			changes = make([]CaseFieldChange, 0, len(a.Changes))
			for _, c := range a.Changes {
				changes = append(changes, CaseFieldChange{
					Field:         c.Field,
					FieldLabel:    c.FieldLabel,
					PreviousValue: c.PreviousValue,
					NewValue:      c.NewValue,
				})
			}
		}

		items = append(items, CaseActivity{
			ID:        a.ID,
			Type:      string(a.Type),
			Content:   a.Content,
			CreatedOn: a.CreatedOn,
			// Both carry the display name, which is what this mapper did before
			// entity-service replaced its string createdBy plus separate
			// createdByFullName with one UserReference. Keeping createdBy as the
			// name preserves the portal's existing output rather than quietly
			// switching it to an email; CommentBubble falls back to createdBy
			// when createdByFullName is empty, so both must resolve the same way.
			//
			// userRefDisplayNameOrSystem (not the plain userRefDisplayName used
			// elsewhere) so an automation/integration-authored activity — no
			// resolvable name or email — renders as "system" instead of an empty
			// string indistinguishable from a genuinely unknown author.
			CreatedBy:          userRefDisplayNameOrSystem(a.CreatedBy),
			CreatedByFirstName: a.CreatedByFirstName,
			CreatedByLastName:  a.CreatedByLastName,
			CreatedByFullName:  userRefDisplayNameOrSystem(a.CreatedBy),
			CommentType:        commentType,
			FileName:           a.FileName,
			ContentType:        a.ContentType,
			SizeBytes:          a.SizeBytes,
			DownloadURL:        a.DownloadURL,
			Changes:            changes,
		})
	}
	return SearchCaseActivitiesResponse{
		Activities:   items,
		TotalRecords: r.Total,
		Limit:        r.Limit,
		Offset:       r.Offset,
		HasMore:      r.HasMore,
	}
}
