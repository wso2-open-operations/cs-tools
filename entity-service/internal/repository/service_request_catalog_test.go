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
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
)

const (
	srCatalogID = "0c1c1c1e-0000-4000-8000-000000000007"
	srItemID    = "0c1c1c1e-0000-4000-8000-000000000008"
	srVarA      = "0c1c1c1e-0000-4000-8000-00000000000a"
	srVarB      = "0c1c1c1e-0000-4000-8000-00000000000b"
)

func TestServiceRequestCatalogData_Shape(t *testing.T) {
	raw, err := ServiceRequestCatalogData(strings.ToUpper(srCatalogID), srItemID, []domain.Variable{
		{ID: srVarA, Value: "first"},
		{ID: strings.ToUpper(srVarB), Value: "line 1\nline \"2\""},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("not a JSON object: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("want exactly catalogId, catalogItemId and variables, got %s", raw)
	}
	var catalogID string
	if err := json.Unmarshal(got["catalogId"], &catalogID); err != nil || catalogID != srCatalogID {
		t.Errorf("catalogId = %s, want the lower-cased id", got["catalogId"])
	}
	var vars map[string]string
	if err := json.Unmarshal(got["variables"], &vars); err != nil {
		t.Fatalf("variables is not an object of strings: %v", err)
	}
	if vars[srVarA] != "first" || vars[srVarB] != "line 1\nline \"2\"" || len(vars) != 2 {
		t.Errorf("variables = %v", vars)
	}
}

func TestServiceRequestCatalogData_Refusals(t *testing.T) {
	ok := []domain.Variable{{ID: srVarA, Value: "x"}}
	many := make([]domain.Variable, MaxServiceRequestVariables+1)
	for i := range many {
		many[i] = domain.Variable{ID: srVarA, Value: "x"}
	}
	// Each answer is within its own limit; together they are over the total.
	var big []domain.Variable
	for i := 0; i <= MaxServiceRequestVariablesBytes/MaxServiceRequestVariableValueBytes; i++ {
		id := []byte(srVarA)
		id[0] = "0123456789abcdef"[i%16]
		id[1] = "0123456789abcdef"[i/16]
		big = append(big, domain.Variable{ID: string(id), Value: strings.Repeat("x", MaxServiceRequestVariableValueBytes)})
	}
	tests := []struct {
		name, catalog, item string
		vars                []domain.Variable
		want                string
	}{
		{"catalog id", "nope", srItemID, ok, "catalogId must be a valid UUID"},
		{"item id", srCatalogID, "", ok, "catalogItemId must be a valid UUID"},
		{"variable id", srCatalogID, srItemID, []domain.Variable{{ID: "q", Value: "x"}}, "variables[0].id must be a valid UUID"},
		{"duplicate variable", srCatalogID, srItemID, append(append([]domain.Variable{}, ok...), ok...), "repeats a variable"},
		{"too many", srCatalogID, srItemID, many, "at most"},
		{"answer too long", srCatalogID, srItemID, []domain.Variable{{ID: srVarA, Value: strings.Repeat("x", MaxServiceRequestVariableValueBytes+1)}}, "longer than"},
		{"NUL in answer", srCatalogID, srItemID, []domain.Variable{{ID: srVarA, Value: "a\x00b"}}, "NUL"},
		{"total too large", srCatalogID, srItemID, big, "in all"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ServiceRequestCatalogData(tc.catalog, tc.item, tc.vars)
			var ve *apierror.ValidationError
			if !errors.As(err, &ve) || !strings.Contains(ve.Msg, tc.want) {
				t.Fatalf("want a ValidationError containing %q, got %v", tc.want, err)
			}
		})
	}
}

// The transfer writes json_data for a service request and for no other target.
func TestInsertTransferredCaseSQL_JSONDataOnlyForServiceRequest(t *testing.T) {
	for target, want := range map[string]bool{"case": false, "engagement": false, "service_request": true, "security_report_analysis": false} {
		if got := strings.Contains(insertTransferredCaseSQL(target), "json_data"); got != want {
			t.Errorf("%s insert mentions json_data = %v, want %v", target, got, want)
		}
	}
}
