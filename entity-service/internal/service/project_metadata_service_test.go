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
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
)

type fakeReferenceDataRepo struct {
	enums       map[string][]string
	projectType *repository.ProjectTypeRow
}

func (f *fakeReferenceDataRepo) ListProjectTypes(context.Context) ([]repository.ProjectTypeRow, error) {
	return nil, nil
}
func (f *fakeReferenceDataRepo) GetProjectByID(context.Context, string) (bool, *repository.ProjectTypeRow, error) {
	return true, f.projectType, nil
}
func (f *fakeReferenceDataRepo) EnumLabels(context.Context, []string) (map[string][]string, error) {
	return f.enums, nil
}

// The call-request endpoints reject upper-case state values ("invalid state"),
// so the ids the metadata offers must be the lowercase domain ids -- each one
// has to pass the same validation the endpoints apply.
func TestGetProjectMetadata_CallRequestStatesUseAPIVocabulary(t *testing.T) {
	repo := &fakeReferenceDataRepo{enums: map[string][]string{
		callRequestStateEnumType: {"PENDING_ON_WSO2", "SCHEDULED", "SOME_FUTURE_STATE", "WSO2_REJECTED"},
	}}
	resp, err := NewProjectMetadataService(repo).GetProjectMetadata(context.Background(), testUUID)
	if err != nil {
		t.Fatalf("GetProjectMetadata: %v", err)
	}

	got := resp.CallRequestStates
	if len(got) != 3 {
		t.Fatalf("want 3 states (the unknown enum label skipped), got %+v", got)
	}
	wantLabels := map[string]string{"pending_on_wso2": "Pending on WSO2", "scheduled": "Scheduled", "wso2_rejected": "WSO2 Rejected"}
	for _, c := range got {
		if wantLabels[c.ID] != c.Label {
			t.Errorf("state %+v: want id/label pair from %v", c, wantLabels)
		}
		if _, ok := validCallRequestStates[domainCallRequestState(c.ID)]; !ok {
			t.Errorf("metadata offers id %q that the call-request endpoints would reject", c.ID)
		}
	}
}

func domainCallRequestState(id string) domain.CallRequestStateType {
	return domain.CallRequestStateType(id)
}

// A project whose type has entitlement columns set on project_type gets
// those, not the zero-value defaults.
func TestGetProjectMetadata_FeaturesFromEntitlementRow(t *testing.T) {
	repo := &fakeReferenceDataRepo{
		projectType: &repository.ProjectTypeRow{
			ID:   testUUID,
			Name: "Managed Cloud Subscription",

			HasServiceRequestReadAccess: true,
			HasChangeRequestReadAccess:  true,
			HasUsageMetricsReadAccess:   true,
			AcceptedSeverityValues:      []string{"S1", "S0"},
			SrProductCategories:         []string{"MS", "PC"},
		},
	}
	resp, err := NewProjectMetadataService(repo).GetProjectMetadata(context.Background(), testUUID)
	if err != nil {
		t.Fatalf("GetProjectMetadata: %v", err)
	}

	f := resp.Features
	if !f.HasServiceRequestReadAccess || !f.HasChangeRequestReadAccess || !f.HasUsageMetricsReadAccess {
		t.Errorf("features = %+v, want the entitlement row's true flags", f)
	}
	if f.HasSraReadAccess {
		t.Errorf("HasSraReadAccess = true, want false (not set on the entitlement row)")
	}
	wantSeverities := map[string]string{"10": "Critical (P1)", "14": "Catastrophic (P0)"}
	if len(f.AcceptedSeverityValues) != len(wantSeverities) {
		t.Fatalf("AcceptedSeverityValues = %+v, want 2 entries", f.AcceptedSeverityValues)
	}
	for _, sv := range f.AcceptedSeverityValues {
		if wantSeverities[sv.ID] != sv.Label {
			t.Errorf("severity %+v: want label %q", sv, wantSeverities[sv.ID])
		}
	}
	if len(f.SrProductCategories) != 2 || f.SrProductCategories[0] != "ms" || f.SrProductCategories[1] != "pc" {
		t.Errorf("SrProductCategories = %v, want [ms pc]", f.SrProductCategories)
	}
}

// A project whose type has no entitlement columns set (the four types
// FEATURE_MATRIX itself has no entry for -- defaulted FALSE/nil by the
// migration) keeps the pre-existing all-false/empty behavior -- no
// regression for types not yet confirmed.
func TestGetProjectMetadata_UnmappedProjectTypeStaysAllFalse(t *testing.T) {
	repo := &fakeReferenceDataRepo{
		projectType: &repository.ProjectTypeRow{ID: testUUID, Name: "Regular"},
	}
	resp, err := NewProjectMetadataService(repo).GetProjectMetadata(context.Background(), testUUID)
	if err != nil {
		t.Fatalf("GetProjectMetadata: %v", err)
	}

	f := resp.Features
	if f.HasServiceRequestReadAccess || f.HasChangeRequestReadAccess || f.HasDeploymentReadAccess ||
		f.HasUsageMetricsReadAccess || f.HasComponentAnalysisReadAccess {
		t.Errorf("features = %+v, want all false for an unmapped project type", f)
	}
	if len(f.AcceptedSeverityValues) != 0 {
		t.Errorf("AcceptedSeverityValues = %+v, want empty", f.AcceptedSeverityValues)
	}
}

// A project with no project_type at all (project.project_type_id is NULL)
// must stay all-false -- there is no project_type row to read entitlements
// from.
func TestGetProjectMetadata_NoProjectTypeSkipsEntitlementLookup(t *testing.T) {
	repo := &fakeReferenceDataRepo{projectType: nil}
	resp, err := NewProjectMetadataService(repo).GetProjectMetadata(context.Background(), testUUID)
	if err != nil {
		t.Fatalf("GetProjectMetadata: %v", err)
	}
	if resp.Features.HasServiceRequestReadAccess {
		t.Error("HasServiceRequestReadAccess = true, want false with no project type")
	}
}

// An unrecognized case_severity_enum label (a future enum value this
// service doesn't know yet, or a bad stored value) is skipped, not turned
// into a blank entry -- the two recognized labels around it must still come
// through untouched.
func TestSeverityChoiceItems_UnrecognizedLabelSkipped(t *testing.T) {
	got := severityChoiceItems(context.Background(), []string{"S1", "UNKNOWN", "S0"})
	if len(got) != 2 {
		t.Fatalf("got %+v, want 2 entries (UNKNOWN skipped)", got)
	}
	if got[0].ID != "10" || got[0].Label != "Critical (P1)" {
		t.Errorf("got[0] = %+v, want {10, Critical (P1)}", got[0])
	}
	if got[1].ID != "14" || got[1].Label != "Catastrophic (P0)" {
		t.Errorf("got[1] = %+v, want {14, Catastrophic (P0)}", got[1])
	}
}
