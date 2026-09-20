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

type fakeReferenceDataRepo struct{ enums map[string][]string }

func (f *fakeReferenceDataRepo) ListProjectTypes(context.Context) ([]repository.ProjectTypeRow, error) {
	return nil, nil
}
func (f *fakeReferenceDataRepo) GetProjectByID(context.Context, string) (bool, *repository.ProjectTypeRow, error) {
	return true, nil, nil
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
