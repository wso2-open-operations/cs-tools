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
	"log/slog"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

// Postgres enum type names backing ProjectMetadataResponse's choice lists,
// named exactly as their CREATE TYPE migration defines them.
const (
	caseStateEnumType             = "case_state_enum"              // migrations/000018_case_table.up.sql
	caseSeverityEnumType          = "case_severity_enum"           // migrations/000018_case_table.up.sql
	caseIssueTypeEnumType         = "case_issue_type_enum"         // migrations/000018_case_table.up.sql
	deploymentTypeEnumType        = "deployment_type_enum"         // migrations/000013_deployment_table.up.sql
	engagementTypeEnumType        = "engagement_type_enum"         // migrations/000019_work_item_extensions.up.sql
	engagementPaymentTypeEnumType = "engagement_payment_type_enum" // migrations/000019_work_item_extensions.up.sql
	changeRequestStateEnumType    = "change_request_state_enum"    // migrations/000047_change_request_table.up.sql
	changeRequestImpactEnumType   = "change_request_impact_enum"   // migrations/000047_change_request_table.up.sql
	timeCardStateEnumType         = "time_card_state_enum"         // migrations/000039_time_card_tables.up.sql
	conversationStateEnumType     = "conversation_state_enum"      // migrations/000057_conversation_table.up.sql
	callRequestStateEnumType      = "customer_call_state_enum"     // migrations/000072_customer_call_table.up.sql
)

// projectMetadataEnumTypes is every enum EnumLabels is asked for in one
// round trip by GetProjectMetadata.
var projectMetadataEnumTypes = []string{
	caseStateEnumType, caseSeverityEnumType, caseIssueTypeEnumType,
	deploymentTypeEnumType, engagementTypeEnumType, engagementPaymentTypeEnumType,
	changeRequestStateEnumType, changeRequestImpactEnumType,
	timeCardStateEnumType, conversationStateEnumType, callRequestStateEnumType,
}

// caseTypeRefItems is the fixed vocabulary case_service.go's own
// validCaseType map accepts for a case's "type" -- not a database table, so
// listed directly rather than queried. Shared with global search's case type,
// so both offer identical id/name pairs.
var caseTypeRefItems = repository.CaseTypeRefs

// choiceListFromLabels wraps raw Postgres enum labels (e.g. "S1", "OPEN") as
// ChoiceListItem, using the label itself as both id and label -- Postgres
// enums carry no separate numeric id/display-label pair the way ServiceNow's
// sys_choice records do.
func choiceListFromLabels(labels []string) []domain.ChoiceListItem {
	out := make([]domain.ChoiceListItem, 0, len(labels))
	for _, l := range labels {
		out = append(out, domain.ChoiceListItem{ID: l, Label: l})
	}
	return out
}

// callRequestStateChoices converts customer_call_state_enum labels to choice
// items in the vocabulary the call-request endpoints themselves accept (the
// lowercase domain id, e.g. "pending_on_wso2", with a display label) -- unlike
// choiceListFromLabels, whose raw UPPER_SNAKE labels those endpoints reject
// with "invalid state". A label with no domain state is skipped: a caller
// couldn't use it as a filter or update value anyway, and
// TestCallRequestStatesMatchMigration fails when the enum and the domain drift.
func callRequestStateChoices(labels []string) []domain.ChoiceListItem {
	out := make([]domain.ChoiceListItem, 0, len(labels))
	for _, l := range labels {
		st := repository.CallRequestStateFromEnum(l)
		if st.ID == "" {
			continue
		}
		out = append(out, domain.ChoiceListItem{ID: st.ID, Label: st.Label})
	}
	return out
}

type projectMetadataService struct {
	repo repository.ReferenceDataRepository
}

// NewProjectMetadataService constructs a Postgres-backed ProjectMetadataService.
func NewProjectMetadataService(repo repository.ReferenceDataRepository) ProjectMetadataService {
	return &projectMetadataService{repo: repo}
}

// GetProjectMetadata implements ProjectMetadataService. Every choice list
// backed by a Postgres enum type is populated for real; fields with no
// Postgres source today are left at their zero value with a TODO comment
// below, rather than fabricated -- see this codebase's convention of
// flagging genuine data-source gaps instead of inventing data (comment_repo.go,
// case_repo.go).
func (s *projectMetadataService) GetProjectMetadata(ctx context.Context, projectID string) (domain.ProjectMetadataResponse, error) {
	if err := validateUUIDs("id", []string{projectID}); err != nil {
		return domain.ProjectMetadataResponse{}, err
	}

	found, projectType, err := s.repo.GetProjectByID(ctx, projectID)
	if err != nil {
		return domain.ProjectMetadataResponse{}, err
	}
	if !found {
		return domain.ProjectMetadataResponse{}, &apierror.NotFoundError{Msg: "project not found"}
	}

	labels, err := s.repo.EnumLabels(ctx, projectMetadataEnumTypes)
	if err != nil {
		return domain.ProjectMetadataResponse{}, fmt.Errorf("project metadata: %w", err)
	}

	var projectTypeRef domain.ReferenceTableItem
	if projectType != nil {
		projectTypeRef = domain.ReferenceTableItem{ID: projectType.ID, Name: projectType.Name}
	}

	features := domain.ProjectFeatures{
		ProjectType:            projectTypeRef,
		AcceptedSeverityValues: make([]domain.ChoiceListItem, 0),
	}
	// projectType's Has*Access/severity/category fields are already resolved
	// by GetProjectByID's join against project_type (migration 000085) --
	// all false/empty for a project with no type, or a type FEATURE_MATRIX
	// itself has no entry for (Cloud Support - Platformer, Internal,
	// Platformer Subscription, Regular), same as before this migration
	// existed.
	if projectType != nil {
		features.HasServiceRequestWriteAccess = projectType.HasServiceRequestWriteAccess
		features.HasServiceRequestReadAccess = projectType.HasServiceRequestReadAccess
		features.HasChangeRequestReadAccess = projectType.HasChangeRequestReadAccess
		features.HasSraWriteAccess = projectType.HasSraWriteAccess
		features.HasSraReadAccess = projectType.HasSraReadAccess
		features.HasEngagementsReadAccess = projectType.HasEngagementsReadAccess
		features.HasUpdatesReadAccess = projectType.HasUpdatesReadAccess
		features.HasDeploymentWriteAccess = projectType.HasDeploymentWriteAccess
		features.HasDeploymentReadAccess = projectType.HasDeploymentReadAccess
		features.HasTimeLogsReadAccess = projectType.HasTimeLogsReadAccess
		features.HasComponentAnalysisReadAccess = projectType.HasComponentAnalysisReadAccess
		features.HasUsageMetricsReadAccess = projectType.HasUsageMetricsReadAccess
		features.AcceptedSeverityValues = severityChoiceItems(ctx, projectType.AcceptedSeverityValues)
		features.DefaultCaseProductCategories = lowercaseAll(projectType.DefaultCaseProductCategories)
		features.SrProductCategories = lowercaseAll(projectType.SrProductCategories)
	}

	return domain.ProjectMetadataResponse{
		CaseStates:           choiceListFromLabels(labels[caseStateEnumType]),
		CallRequestStates:    callRequestStateChoices(labels[callRequestStateEnumType]),
		ChangeRequestStates:  choiceListFromLabels(labels[changeRequestStateEnumType]),
		ConversationStates:   choiceListFromLabels(labels[conversationStateEnumType]),
		TimeCardStates:       choiceListFromLabels(labels[timeCardStateEnumType]),
		ChangeRequestImpacts: choiceListFromLabels(labels[changeRequestImpactEnumType]),
		Severities:           choiceListFromLabels(labels[caseSeverityEnumType]),
		// SeverityBasedAllocationTime: no per-severity SLA-allocation-time
		// table exists in Postgres yet. Empty (not nil) so it serializes as
		// {} rather than null -- portal callers treat it as a non-optional
		// object. TODO: populate once one does.
		SeverityBasedAllocationTime: make(map[string]int),
		IssueTypes:                  choiceListFromLabels(labels[caseIssueTypeEnumType]),
		DeploymentTypes:             choiceListFromLabels(labels[deploymentTypeEnumType]),
		CaseTypes:                   caseTypeRefItems,
		EngagementTypes:             choiceListFromLabels(labels[engagementTypeEnumType]),
		EngagementPaymentTypes:      choiceListFromLabels(labels[engagementPaymentTypeEnumType]),
		Features:                    features,
	}, nil
}

// severityEnumToChoice maps case_severity_enum's own labels (migration
// 000018) to the {ServiceNow numeric id, display label} pair
// ProjectFeatures.AcceptedSeverityValues has always carried on the
// ServiceNow data source (snChoiceOption.id/label) -- the same ids
// case_repo.go's severityToSNLabel resolves S0..S4 from/to, and the same
// display labels backend-v2's own caseSeverityDisplayLabels uses.
// AcceptedSeverityValues carries ServiceNow's own numeric ids, which
// backend-v2's normalizeCaseSeverityChoices passes through unchanged rather
// than relabeling, so the label has to be correct here already.
var severityEnumToChoice = map[string]domain.ChoiceListItem{
	"S1": {ID: "10", Label: "Critical (P1)"},
	"S2": {ID: "11", Label: "High (P2)"},
	"S3": {ID: "12", Label: "Medium (P3)"},
	"S4": {ID: "13", Label: "Low (P4)"},
	"S0": {ID: "14", Label: "Catastrophic (P0)"},
}

// severityChoiceItems converts stored case_severity_enum labels to the
// {id, label} pairs ProjectFeatures.AcceptedSeverityValues expects, skipping
// any label this service doesn't recognize rather than emitting a blank one.
// A skip is logged rather than silent: case_severity_enum gaining a new
// label (or a bad value somehow stored) would otherwise quietly truncate the
// slice with nothing to point at when investigating.
func severityChoiceItems(ctx context.Context, enumLabels []string) []domain.ChoiceListItem {
	out := make([]domain.ChoiceListItem, 0, len(enumLabels))
	for _, l := range enumLabels {
		choice, ok := severityEnumToChoice[l]
		if !ok {
			slog.WarnContext(ctx, "project metadata: unrecognized case_severity_enum label in accepted_severity_values", "label", l)
			continue
		}
		out = append(out, choice)
	}
	return out
}

// lowercaseAll converts deployed_product_category_enum's UPPER_SNAKE labels
// (migration 000014) to the lowercase form ProjectFeatures.
// DefaultCaseProductCategories/SrProductCategories have always carried --
// matches the webapp's own ProductCategory enum (features/project-details/
// types/deployments.ts: CLOUD = "cl", PDP = "pdp"). A nil slice stays nil,
// not an empty one, matching *ProductCategories' omitempty json tag.
func lowercaseAll(labels []string) []string {
	if labels == nil {
		return nil
	}
	out := make([]string, len(labels))
	for i, l := range labels {
		out[i] = strings.ToLower(l)
	}
	return out
}
