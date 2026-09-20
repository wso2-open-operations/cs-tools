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

package repository

import (
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

// TestChangeRequestChangeModelToType covers all eight real
// change_request_change_model_enum labels (migration 000055) -- a coverage
// gap here would let a mistyped or omitted label silently drop a real
// change request's type on read.
func TestChangeRequestChangeModelToType(t *testing.T) {
	want := map[string]domain.ChangeRequestType{
		"AZURE":                domain.ChangeRequestTypeAzure,
		"CHANGE_REGISTRATION":  domain.ChangeRequestTypeChangeRegistration,
		"CLOUD_INFRASTRUCTURE": domain.ChangeRequestTypeCloudInfrastructure,
		"EMERGENCY":            domain.ChangeRequestTypeEmergency,
		"INFRA":                domain.ChangeRequestTypeInfra,
		"NORMAL":               domain.ChangeRequestTypeNormal,
		"STANDARD":             domain.ChangeRequestTypeStandard,
		"UNAUTHORIZED_CHANGE":  domain.ChangeRequestTypeUnauthorizedChange,
	}
	if len(changeRequestChangeModelToType) != len(want) {
		t.Fatalf("changeRequestChangeModelToType has %d entries, want %d", len(changeRequestChangeModelToType), len(want))
	}
	for enumValue, wantType := range want {
		got, ok := changeRequestChangeModelToType[enumValue]
		if !ok {
			t.Errorf("changeRequestChangeModelToType[%q] missing", enumValue)
			continue
		}
		if got != wantType {
			t.Errorf("changeRequestChangeModelToType[%q] = %q, want %q", enumValue, got, wantType)
		}
	}
}

// TestChangeRequestTypeToChangeModel_SupportedTypesRoundTrip proves every
// change-model-backed ChangeRequestType can be written back to its
// original enum label -- this is the map PatchChangeRequest's Type
// handling consults before persisting.
func TestChangeRequestTypeToChangeModel_SupportedTypesRoundTrip(t *testing.T) {
	for enumValue, t1 := range changeRequestChangeModelToType {
		got, ok := changeRequestTypeToChangeModel[t1]
		if !ok {
			t.Errorf("changeRequestTypeToChangeModel[%q] missing (from enum %q)", t1, enumValue)
			continue
		}
		if got != enumValue {
			t.Errorf("changeRequestTypeToChangeModel[%q] = %q, want %q", t1, got, enumValue)
		}
	}
}

// TestChangeRequestTypeToChangeModel_UnsupportedTypesRejected proves the
// two pre-existing ChangeRequestType values that predate change_model
// (migration 000055) have no reverse mapping -- PatchChangeRequest relies
// on this absence to reject them with a ValidationError instead of
// silently writing a wrong or empty change_model value.
func TestChangeRequestTypeToChangeModel_UnsupportedTypesRejected(t *testing.T) {
	for _, unsupported := range []domain.ChangeRequestType{
		domain.ChangeRequestTypeModel,
		domain.ChangeRequestTypeSiteReliabilityOps,
	} {
		if _, ok := changeRequestTypeToChangeModel[unsupported]; ok {
			t.Errorf("changeRequestTypeToChangeModel[%q] unexpectedly present -- PatchChangeRequest would silently accept a type with no real change_model equivalent", unsupported)
		}
	}
}
