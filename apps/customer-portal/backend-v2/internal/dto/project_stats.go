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
	"strings"

	"github.com/wso2-open-operations/cs-tools/apps/customer-portal/backend-v2/internal/entity"
)

// ReferenceItem is a flattened {id, label, count?} view of entity-service's
// ChoiceListItem/ReferenceTableItem — this API collapses both into one
// uniform shape for every project metadata/stats endpoint, dropping
// ReferenceTableItem's number/internalId fields (not useful for a filter
// dropdown or a stats breakdown).
type ReferenceItem struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Count *int   `json:"count,omitempty"`
	// Abbreviation is the short reference name (e.g. "APIM" for "API Manager").
	// The frontend declares it on its shared IdLabelRef and prefers it over the
	// label where a compact name is wanted — see recommendedLevelsProductKey in
	// ManageProductModal.tsx, which falls back to the label when it is absent.
	// Only entity-service's ReferenceTableItem carries it; ChoiceListItem has no
	// equivalent, so it stays nil on that path.
	Abbreviation *string `json:"abbreviation,omitempty"`
}

func mapChoiceListItem(i entity.ChoiceListItem) ReferenceItem {
	return ReferenceItem{ID: i.ID, Label: i.Label, Count: i.Count}
}

func mapChoiceListItems(items []entity.ChoiceListItem) []ReferenceItem {
	out := make([]ReferenceItem, 0, len(items))
	for _, i := range items {
		out = append(out, mapChoiceListItem(i))
	}
	return out
}

func mapReferenceTableItems(items []entity.ReferenceTableItem) []ReferenceItem {
	out := make([]ReferenceItem, 0, len(items))
	for _, i := range items {
		out = append(out, ReferenceItem{ID: i.ID, Label: i.Name, Count: i.Count, Abbreviation: i.Abbreviation})
	}
	return out
}

// restrictedChangeRequestStateIDs are excluded from ProjectFilterOptions'
// changeRequestStates and the change-request stats' state counts: ServiceNow's own
// numeric ids for the three pre-approval workflow states a customer is never shown
// on that data source (New, Assess, Authorize).
//
// A ServiceNow-sourced id is the one thing that tells the two data sources apart
// here, and it decides what Authorize ("-3") means:
//
//   - On the ServiceNow data source nothing is designated to a customer: every
//     change request is "legacy" and is visible in every state except these
//     three, and ServiceNow's own search does not hide them (entity-service
//     narrows a customer's search to the visible states and leaves these out of
//     the vocabulary it serves, and this list is the second line). So "-3" is
//     dropped, exactly like "-5" and "-4".
//   - On the Postgres data source a change request is visible once it was
//     designated to the customer, in whatever state it is in. A customer's
//     proposed time and a Re-schedule keep the change in Customer Approval, but
//     one that an older build sent back to Authorize (through the CAB again)
//     waits there and stays on their list, so the state filter has to offer it.
//     Its id there is the raw enum label ("AUTHORIZE", see
//     restrictedChangeRequestStateLabels), never "-3", so it passes this check
//     and is kept.
var restrictedChangeRequestStateIDs = map[string]bool{"-5": true, "-4": true, "-3": true}

// restrictedChangeRequestStateLabels is the Postgres-mode equivalent: on
// that data source ReferenceDataRepository.EnumLabels (entity-service)
// returns the raw enum label as id, e.g. {"id":"NEW"}, never a ServiceNow
// number, so the id check above never matches there and these two would
// leak into the response unfiltered without this. crStateIDs (see
// change_request_enum_mapping.go) also has no entries for them, by the same
// "no visible change request is ever in them" design (a designated change
// request left New when approval was requested and never returns to Assess), so
// they pass normalizeChoices unchanged and keep their raw label -- matched here
// before that happens.
//
// Authorize is not in this list: on the Postgres data source it is a state a
// designated customer's change request waits in. Its ServiceNow-sourced id is
// dropped by the id check above.
//
// Checked case-insensitively and kept alongside the id check above, not in
// place of it: a Postgres-mode label is reliably UPPER_SNAKE, but this
// endpoint also serves the ServiceNow data source, whose own raw label
// casing isn't guaranteed to match — dropping the id check here would trade
// one data source's gap for the other's.
var restrictedChangeRequestStateLabels = map[string]bool{"NEW": true, "ASSESS": true}

func isRestrictedChangeRequestState(s ReferenceItem) bool {
	return restrictedChangeRequestStateIDs[s.ID] || restrictedChangeRequestStateLabels[strings.ToUpper(s.Label)]
}

// ProjectFilterOptions is the portal's response for GET /projects/{id}/filters
// — a flattened, filter-dropdown-ready view of entity-service's project
// metadata. Deliberately excludes entity-service's ProjectFeatures (see
// ProjectFeatures below, its own endpoint) and per-severity allocation
// minutes' internal framing (kept as-is, it's just a lookup map the
// frontend needs).
type ProjectFilterOptions struct {
	CaseStates                  []ReferenceItem `json:"caseStates"`
	Severities                  []ReferenceItem `json:"severities"`
	IssueTypes                  []ReferenceItem `json:"issueTypes"`
	DeploymentTypes             []ReferenceItem `json:"deploymentTypes"`
	CallRequestStates           []ReferenceItem `json:"callRequestStates"`
	ChangeRequestStates         []ReferenceItem `json:"changeRequestStates"`
	ChangeRequestImpacts        []ReferenceItem `json:"changeRequestImpacts"`
	ConversationStates          []ReferenceItem `json:"conversationStates"`
	CaseTypes                   []ReferenceItem `json:"caseTypes"`
	TimeCardStates              []ReferenceItem `json:"timeCardStates"`
	EngagementTypes             []ReferenceItem `json:"engagementTypes"`
	EngagementPaymentTypes      []ReferenceItem `json:"engagementPaymentTypes"`
	// ResolutionCodes/Causes back the resolution fields the webapp must
	// collect before closing (or proposing a solution for) a case — see
	// PATCH /cases/{id}'s own dto.UpdateCaseRequest doc comment.
	ResolutionCodes             []ReferenceItem `json:"resolutionCodes"`
	Causes                      []ReferenceItem `json:"causes"`
	SeverityBasedAllocationTime map[string]int  `json:"severityBasedAllocationTime"`
}

// MapProjectFilterOptions builds the portal response from entity-service's
// ProjectMetadataResponse.
func MapProjectFilterOptions(m entity.ProjectMetadataResponse) ProjectFilterOptions {
	changeRequestStates := make([]ReferenceItem, 0, len(m.ChangeRequestStates))
	for _, s := range mapChoiceListItems(m.ChangeRequestStates) {
		if !isRestrictedChangeRequestState(s) {
			changeRequestStates = append(changeRequestStates, s)
		}
	}

	return ProjectFilterOptions{
		CaseStates:                  normalizeCaseStateChoices(mapChoiceListItems(m.CaseStates)),
		Severities:                  normalizeCaseSeverityChoices(mapChoiceListItems(m.Severities)),
		IssueTypes:                  normalizeCaseIssueTypeChoices(mapChoiceListItems(m.IssueTypes)),
		DeploymentTypes:             normalizeDeploymentTypeChoices(mapChoiceListItems(m.DeploymentTypes)),
		CallRequestStates:           mapChoiceListItems(m.CallRequestStates),
		ChangeRequestStates:         normalizeChangeRequestStateChoices(changeRequestStates),
		ChangeRequestImpacts:        normalizeChangeRequestImpactChoices(mapChoiceListItems(m.ChangeRequestImpacts)),
		ConversationStates:          normalizeConversationStateChoices(mapChoiceListItems(m.ConversationStates)),
		CaseTypes:                   mapReferenceTableItems(m.CaseTypes),
		TimeCardStates:              mapChoiceListItems(m.TimeCardStates),
		EngagementTypes:             normalizeCaseEngagementTypeChoices(mapChoiceListItems(m.EngagementTypes)),
		EngagementPaymentTypes:      mapChoiceListItems(m.EngagementPaymentTypes),
		ResolutionCodes:             mapChoiceListItems(m.ResolutionCodes),
		Causes:                      mapChoiceListItems(m.Causes),
		SeverityBasedAllocationTime: m.SeverityBasedAllocationTime,
	}
}

// ProjectFeatures is the portal's response for GET /projects/{id}/features —
// entity-service's ProjectMetadataResponse.Features, minus its ProjectType
// reference (an internal ServiceNow project-type lookup, not something the
// frontend's feature-flag consumers need).
type ProjectFeatures struct {
	AcceptedSeverityValues         []ReferenceItem `json:"acceptedSeverityValues"`
	HasServiceRequestWriteAccess   bool            `json:"hasServiceRequestWriteAccess"`
	HasServiceRequestReadAccess    bool            `json:"hasServiceRequestReadAccess"`
	HasSraWriteAccess              bool            `json:"hasSraWriteAccess"`
	HasSraReadAccess               bool            `json:"hasSraReadAccess"`
	HasChangeRequestReadAccess     bool            `json:"hasChangeRequestReadAccess"`
	HasEngagementsReadAccess       bool            `json:"hasEngagementsReadAccess"`
	HasUpdatesReadAccess           bool            `json:"hasUpdatesReadAccess"`
	HasTimeLogsReadAccess          bool            `json:"hasTimeLogsReadAccess"`
	HasDeploymentWriteAccess       bool            `json:"hasDeploymentWriteAccess"`
	HasDeploymentReadAccess        bool            `json:"hasDeploymentReadAccess"`
	HasComponentAnalysisReadAccess bool            `json:"hasComponentAnalysisReadAccess"`
	HasUsageMetricsReadAccess      bool            `json:"hasUsageMetricsReadAccess"`
	DefaultCaseProductCategories   []string        `json:"defaultCaseProductCategories,omitempty"`
	SrProductCategories            []string        `json:"srProductCategories,omitempty"`
}

// MapProjectFeatures builds the portal response from entity-service's
// ProjectMetadataResponse.
func MapProjectFeatures(m entity.ProjectMetadataResponse) ProjectFeatures {
	return ProjectFeatures{
		AcceptedSeverityValues:         normalizeCaseSeverityChoices(mapChoiceListItems(m.Features.AcceptedSeverityValues)),
		HasServiceRequestWriteAccess:   m.Features.HasServiceRequestWriteAccess,
		HasServiceRequestReadAccess:    m.Features.HasServiceRequestReadAccess,
		HasSraWriteAccess:              m.Features.HasSraWriteAccess,
		HasSraReadAccess:               m.Features.HasSraReadAccess,
		HasChangeRequestReadAccess:     m.Features.HasChangeRequestReadAccess,
		HasEngagementsReadAccess:       m.Features.HasEngagementsReadAccess,
		HasUpdatesReadAccess:           m.Features.HasUpdatesReadAccess,
		HasTimeLogsReadAccess:          m.Features.HasTimeLogsReadAccess,
		HasDeploymentWriteAccess:       m.Features.HasDeploymentWriteAccess,
		HasDeploymentReadAccess:        m.Features.HasDeploymentReadAccess,
		HasComponentAnalysisReadAccess: m.Features.HasComponentAnalysisReadAccess,
		HasUsageMetricsReadAccess:      m.Features.HasUsageMetricsReadAccess,
		DefaultCaseProductCategories:   m.Features.DefaultCaseProductCategories,
		SrProductCategories:            m.Features.SrProductCategories,
	}
}

// caseStateIDOpen is the ServiceNow case state ID meaning "open", used to
// pick the open-case count out of a case-stats state breakdown. This is
// this API's own default; if cs-tools' ServiceNow instance uses different
// case state IDs, this needs to become configurable here too.
const caseStateIDOpen = "1"

// conversationStateID{Open,Active,Resolved,Abandoned} are the ServiceNow
// conversation state IDs used to pick specific counts out of a
// conversation-stats state breakdown. These are this API's own defaults —
// see caseStateIDOpen's doc comment on the same caveat.
const (
	conversationStateIDOpen      = "1"
	conversationStateIDActive    = "2"
	conversationStateIDResolved  = "3"
	conversationStateIDAbandoned = "5"
)

func countForState(stateCount []ReferenceItem, stateID string) *int {
	for _, s := range stateCount {
		if s.ID == stateID {
			return s.Count
		}
	}
	return nil
}

// ProjectStats is the headline counters on the project dashboard.
type ProjectStats struct {
	OpenCases                      *int    `json:"openCases,omitempty"`
	ActiveChats                    *int    `json:"activeChats,omitempty"`
	Deployments                    *int    `json:"deployments,omitempty"`
	SLAStatus                      *string `json:"slaStatus,omitempty"`
	OutstandingCaseCount           *int    `json:"outstandingCaseCount,omitempty"`
	OutstandingServiceRequestCount *int    `json:"outstandingServiceRequestCount,omitempty"`
	OutstandingEngagementCount     *int    `json:"outstandingEngagementCount,omitempty"`
	OutstandingSraCount            *int    `json:"outstandingSraCount,omitempty"`
	OutstandingChangeRequestCount  *int    `json:"outstandingChangeRequestCount,omitempty"`
	OutstandingAnnouncementCount   *int    `json:"outstandingAnnouncementCount,omitempty"`
}

// RecentActivity is the recent-activity panel on the project dashboard.
type RecentActivity struct {
	TotalHours       *float64 `json:"totalHours,omitempty"`
	BillableHours    *float64 `json:"billableHours,omitempty"`
	LastDeploymentOn *string  `json:"lastDeploymentOn,omitempty"`
}

// ProjectDashboardStats is the portal's response for GET /projects/{id}/stats
// — combines entity-service's case/conversation/deployment/activity stats
// into one dashboard view.
type ProjectDashboardStats struct {
	ProjectStats   ProjectStats   `json:"projectStats"`
	RecentActivity RecentActivity `json:"recentActivity"`
}

// BuildProjectDashboardStats combines up to four independently-fetched stats
// responses into the dashboard view, using graceful-degradation behavior:
// any source that failed to load is passed as nil and its fields are simply
// omitted from the response, rather than failing the whole request.
func BuildProjectDashboardStats(
	caseStats *entity.ProjectCaseStatsResponse,
	conversationStats *entity.ProjectConversationStatsResponse,
	deploymentStats *entity.ProjectDeploymentStatsResponse,
	activityStats *entity.ProjectStatsResponse,
) ProjectDashboardStats {
	var out ProjectDashboardStats

	if caseStats != nil {
		mapped := MapProjectCaseStats(*caseStats)
		out.ProjectStats.OpenCases = countForState(mapped.StateCount, caseStateIDOpen)
	}
	if conversationStats != nil {
		activeCount := conversationStats.ActiveCount
		out.ProjectStats.ActiveChats = &activeCount
	}
	if deploymentStats != nil {
		totalCount := deploymentStats.TotalCount
		out.ProjectStats.Deployments = &totalCount
		out.RecentActivity.LastDeploymentOn = deploymentStats.LastDeploymentOn
	}
	if activityStats != nil {
		slaStatus := activityStats.SLAStatus
		out.ProjectStats.SLAStatus = &slaStatus
		out.ProjectStats.OutstandingCaseCount = &activityStats.OutstandingCount.CaseCount
		out.ProjectStats.OutstandingServiceRequestCount = &activityStats.OutstandingCount.ServiceRequestCount
		out.ProjectStats.OutstandingEngagementCount = &activityStats.OutstandingCount.EngagementCount
		out.ProjectStats.OutstandingSraCount = &activityStats.OutstandingCount.SraCount
		out.ProjectStats.OutstandingChangeRequestCount = &activityStats.OutstandingCount.ChangeRequestCount
		out.ProjectStats.OutstandingAnnouncementCount = &activityStats.OutstandingCount.AnnouncementCount
		out.RecentActivity.TotalHours = &activityStats.TotalHours
		out.RecentActivity.BillableHours = &activityStats.BillableHours
	}
	return out
}

// ResolvedCountBreakdown is the portal's own copy of entity-service's
// resolved-count breakdown, kept separate per this package's "always map
// through dto" convention.
type ResolvedCountBreakdown struct {
	Total          int `json:"total"`
	CurrentMonth   int `json:"currentMonth"`
	PastThirtyDays int `json:"pastThirtyDays"`
}

func mapResolvedCountBreakdown(r entity.ResolvedCountBreakdown) ResolvedCountBreakdown {
	return ResolvedCountBreakdown{Total: r.Total, CurrentMonth: r.CurrentMonth, PastThirtyDays: r.PastThirtyDays}
}

// CaseStatsChangeRate is the portal's own copy of the case-stats change-rate figures.
type CaseStatsChangeRate struct {
	ResolvedEngagements float64 `json:"resolvedEngagements"`
	AverageResponseTime float64 `json:"averageResponseTime"`
}

// CasesTrend is the portal's own copy of a case-count trend bucket, with
// severities mapped through ReferenceItem instead of entity.ChoiceListItem.
type CasesTrend struct {
	Period     string          `json:"period"`
	Severities []ReferenceItem `json:"severities"`
}

func mapCasesTrend(trends []entity.CasesTrend) []CasesTrend {
	out := make([]CasesTrend, 0, len(trends))
	for _, t := range trends {
		out = append(out, CasesTrend{Period: t.Period, Severities: normalizeCaseSeverityChoices(mapChoiceListItems(t.Severities))})
	}
	return out
}

// ProjectCaseStats is the portal's response for GET /projects/{id}/cases/stats.
type ProjectCaseStats struct {
	TotalCount                     int                    `json:"totalCount"`
	ActiveCount                    int                    `json:"activeCount"`
	OutstandingCount               int                    `json:"outstandingCount"`
	ActionRequiredCount            int                    `json:"actionRequiredCount"`
	AverageResponseTime            float64                `json:"averageResponseTime"`
	ResolvedCases                  ResolvedCountBreakdown `json:"resolvedCases"`
	ChangeRate                     CaseStatsChangeRate    `json:"changeRate"`
	StateCount                     []ReferenceItem        `json:"stateCount"`
	SeverityCount                  []ReferenceItem        `json:"severityCount"`
	OutstandingSeverityCount       []ReferenceItem        `json:"outstandingSeverityCount"`
	CaseTypeCount                  []ReferenceItem        `json:"caseTypeCount"`
	CasesTrend                     []CasesTrend           `json:"casesTrend"`
	EngagementTypeCount            []ReferenceItem        `json:"engagementTypeCount"`
	OutstandingEngagementTypeCount []ReferenceItem        `json:"outstandingEngagementTypeCount"`
}

// MapProjectCaseStats builds the portal response from entity-service's
// ProjectCaseStatsResponse.
func MapProjectCaseStats(r entity.ProjectCaseStatsResponse) ProjectCaseStats {
	return ProjectCaseStats{
		TotalCount:                     r.TotalCount,
		ActiveCount:                    r.ActiveCount,
		OutstandingCount:               r.OutstandingCount,
		ActionRequiredCount:            r.ActionRequiredCount,
		AverageResponseTime:            r.AverageResponseTime,
		ResolvedCases:                  mapResolvedCountBreakdown(r.ResolvedCount),
		ChangeRate:                     CaseStatsChangeRate(r.ChangeRate),
		StateCount:                     normalizeCaseStateChoices(mapChoiceListItems(r.StateCount)),
		SeverityCount:                  normalizeCaseSeverityChoices(mapChoiceListItems(r.SeverityCount)),
		OutstandingSeverityCount:       normalizeCaseSeverityChoices(mapChoiceListItems(r.OutstandingSeverityCount)),
		CaseTypeCount:                  mapReferenceTableItems(r.CaseTypeCount),
		CasesTrend:                     mapCasesTrend(r.CasesTrend),
		EngagementTypeCount:            normalizeCaseEngagementTypeChoices(mapChoiceListItems(r.EngagementTypeCount)),
		OutstandingEngagementTypeCount: normalizeCaseEngagementTypeChoices(mapChoiceListItems(r.OutstandingEngagementTypeCount)),
	}
}

// ConversationStats is the portal's response for GET /projects/{id}/conversations/stats
// — a handful of specific counts picked out of entity-service's state
// breakdown; this is deliberately a thinner response, which also drops the
// converted/session/total counts an internal OverallConversationStats-equivalent
// would carry.
type ConversationStats struct {
	OpenCount      *int `json:"openCount,omitempty"`
	ActiveCount    *int `json:"activeCount,omitempty"`
	ResolvedCount  *int `json:"resolvedCount,omitempty"`
	AbandonedCount *int `json:"abandonedCount,omitempty"`
}

// MapConversationStats builds the portal response from entity-service's
// ProjectConversationStatsResponse.
//
// StateCount is normalised first (normalizeConversationStateChoices): on the
// Postgres data source entity-service returns the raw enum label as the id
// ({"id":"ACTIVE"}), so a lookup by ServiceNow's numeric id ("2") found nothing
// and every count came back absent. Support's Active Chats card then showed 0
// while the Active Chats list behind it (which filters on that same numeric
// state key) held hundreds of conversations. Ids that are already numeric
// (the ServiceNow data source) pass through untouched.
func MapConversationStats(r entity.ProjectConversationStatsResponse) ConversationStats {
	stateCount := normalizeConversationStateChoices(mapChoiceListItems(r.StateCount))
	return ConversationStats{
		OpenCount:      countForState(stateCount, conversationStateIDOpen),
		ActiveCount:    countForState(stateCount, conversationStateIDActive),
		ResolvedCount:  countForState(stateCount, conversationStateIDResolved),
		AbandonedCount: countForState(stateCount, conversationStateIDAbandoned),
	}
}

// ProjectSupportStats is the portal's response for GET /projects/{id}/stats/support.
type ProjectSupportStats struct {
	OngoingCases                 *int `json:"ongoingCases,omitempty"`
	ActiveChats                  *int `json:"activeChats,omitempty"`
	ResolvedPast30DaysCasesCount *int `json:"resolvedPast30DaysCasesCount,omitempty"`
	ResolvedChats                *int `json:"resolvedChats,omitempty"`
}

// BuildProjectSupportStats combines independently-fetched case and
// conversation stats into the support-stats view, using graceful-degradation
// behavior for this endpoint: either source may be nil (failed to load)
// without failing the whole request.
func BuildProjectSupportStats(caseStats *entity.ProjectCaseStatsResponse, conversationStats *entity.ProjectConversationStatsResponse) ProjectSupportStats {
	var out ProjectSupportStats
	if caseStats != nil {
		activeCount := caseStats.ActiveCount
		out.OngoingCases = &activeCount
		pastThirtyDays := caseStats.ResolvedCount.PastThirtyDays
		out.ResolvedPast30DaysCasesCount = &pastThirtyDays
	}
	if conversationStats != nil {
		mapped := MapConversationStats(*conversationStats)
		out.ActiveChats = mapped.ActiveCount
		// Resolved via Chat (Last 30d): the 30-day figure when entity-service sends one,
		// else the Resolved count of the state breakdown (not limited to any period),
		// which is what a build that predates the 30-day figure, and the ServiceNow data
		// source, still send.
		out.ResolvedChats = mapped.ResolvedCount
		if conversationStats.ResolvedPastThirtyDays != nil {
			resolved := *conversationStats.ResolvedPastThirtyDays
			out.ResolvedChats = &resolved
		}
	}
	return out
}

// ProjectTimeCardStats is the portal's response for GET /projects/{id}/time-cards/stats.
// entity-service's response has no fields worth restricting, so this is a
// direct passthrough shape — kept as its own portal type (rather than reusing
// entity.ProjectTimeCardStatsResponse directly) purely for this package's
// "always map through dto" convention.
type ProjectTimeCardStats struct {
	TotalHours       float64 `json:"totalHours"`
	BillableHours    float64 `json:"billableHours"`
	NonBillableHours float64 `json:"nonBillableHours"`
}

// MapProjectTimeCardStats builds the portal response from entity-service's
// ProjectTimeCardStatsResponse.
func MapProjectTimeCardStats(r entity.ProjectTimeCardStatsResponse) ProjectTimeCardStats {
	return ProjectTimeCardStats{
		TotalHours:       r.TotalHours,
		BillableHours:    r.BillableHours,
		NonBillableHours: r.NonBillableHours,
	}
}

// ProjectChangeRequestStats is the portal's response for
// GET /projects/{id}/stats/change-requests.
type ProjectChangeRequestStats struct {
	TotalCount          int                    `json:"totalCount"`
	ActiveCount         int                    `json:"activeCount"`
	OutstandingCount    int                    `json:"outstandingCount"`
	ActionRequiredCount int                    `json:"actionRequiredCount"`
	StateCount          []ReferenceItem        `json:"stateCount"`
	ResolvedCount       ResolvedCountBreakdown `json:"resolvedCount"`
}

// MapProjectChangeRequestStats builds the portal response from
// entity-service's ProjectChangeRequestStatsResponse.
//
// StateCount is normalized the same way GET /projects/{id}/filters'
// changeRequestStates is (see normalizeChangeRequestStateChoices): on the
// Postgres data source entity-service returns the raw enum as both id and
// label (e.g. {"id":"SCHEDULED","label":"SCHEDULED"}), but the Operations page
// finds its Scheduled / Customer Approval / Customer Review counts by the
// display label, so an un-normalized "SCHEDULED" never matched and the
// Upcoming Changes card fell back to "--" while the list beside it showed
// Scheduled changes.
//
// New and Assess are left out of StateCount, as they are out of the filter
// options: entity-service lists every state with a count, but a customer is only
// counted the change requests designated to them and none of those is ever in
// either, so they would be two rows of 0 under raw ids ("NEW", "ASSESS") no
// screen has a name for. Authorize is kept on the Postgres data source, as
// {id: "-3", label: "Authorize"}: a change request the customer proposed a new
// time for waits there. On the ServiceNow data source (ids "-5", "-4", "-3") all
// three are left out, as they are of the filter options: see
// restrictedChangeRequestStateIDs.
func MapProjectChangeRequestStats(r entity.ProjectChangeRequestStatsResponse) ProjectChangeRequestStats {
	stateCount := make([]ReferenceItem, 0, len(r.StateCount))
	for _, s := range mapChoiceListItems(r.StateCount) {
		if !isRestrictedChangeRequestState(s) {
			stateCount = append(stateCount, s)
		}
	}
	return ProjectChangeRequestStats{
		TotalCount:          r.TotalCount,
		ActiveCount:         r.ActiveCount,
		OutstandingCount:    r.OutstandingCount,
		ActionRequiredCount: r.ActionRequiredCount,
		StateCount:          normalizeChangeRequestStateChoices(stateCount),
		ResolvedCount:       mapResolvedCountBreakdown(r.ResolvedCount),
	}
}

// UsageStats is the counter trio the Usage Metrics page reads from
// GET /projects/{id}/stats/usage.
//
// entity-service's ProjectStatsResponse carries more than this (hours, SLA
// status, outstanding counts), but the frontend's own contract for this
// endpoint is exactly these three fields — see UsageStatsResponse in
// webapp/src/features/project-details/types/usage.ts — and the Ballerina
// backend's mapUsageStats returns the same three. Match the frontend's
// contract, not entity-service's superset; the richer numbers are already
// exposed through GET /projects/{id}/stats.
type UsageStats struct {
	DeploymentCount      int `json:"deploymentCount"`
	DeployedProductCount int `json:"deployedProductCount"`
	InstanceCount        int `json:"instanceCount"`
}

// MapUsageStats trims entity-service's project stats to the Usage Metrics view.
func MapUsageStats(r entity.ProjectStatsResponse) UsageStats {
	return UsageStats{
		DeploymentCount:      r.DeploymentCount,
		DeployedProductCount: r.DeployedProductCount,
		InstanceCount:        r.InstanceCount,
	}
}
