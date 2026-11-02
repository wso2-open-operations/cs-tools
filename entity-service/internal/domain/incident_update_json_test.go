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

package domain

import (
	"bytes"
	"encoding/json"
	"testing"
)

func decodeIncidentUpdate(t *testing.T, body string) (UpdateIncidentRequest, error) {
	t.Helper()
	var req UpdateIncidentRequest
	dec := json.NewDecoder(bytes.NewReader([]byte(body)))
	dec.DisallowUnknownFields()
	err := dec.Decode(&req)
	return req, err
}

func TestUpdateIncidentRequestParentIncidentID(t *testing.T) {
	// The edit dialog's cleared Parent incident picker, verbatim.
	req, err := decodeIncidentUpdate(t, `{"parentIncidentId": null}`)
	if err != nil {
		t.Fatal(err)
	}
	if req.ParentIncidentID == nil || *req.ParentIncidentID != nil {
		t.Fatalf(`{"parentIncidentId": null} must decode as clear (non-nil holding nil), got %v`, req.ParentIncidentID)
	}

	const id = "8a6f1c2e-3b4d-4e5f-8a9b-0c1d2e3f4a5b"
	req, err = decodeIncidentUpdate(t, `{"parentIncidentId": "`+id+`"}`)
	if err != nil {
		t.Fatal(err)
	}
	if req.ParentIncidentID == nil || *req.ParentIncidentID == nil || **req.ParentIncidentID != id {
		t.Fatalf("a value must decode as a set, got %v", req.ParentIncidentID)
	}

	req, err = decodeIncidentUpdate(t, `{"subject": "x"}`)
	if err != nil {
		t.Fatal(err)
	}
	if req.ParentIncidentID != nil || req.Environment != nil {
		t.Fatal("an omitted field must stay nil (leave it alone)")
	}
	if req.Subject == nil || *req.Subject != "x" {
		t.Fatal("other fields must still decode")
	}
}

func TestUpdateIncidentRequestEnvironmentNull(t *testing.T) {
	req, err := decodeIncidentUpdate(t, `{"environment": null}`)
	if err != nil {
		t.Fatal(err)
	}
	if req.Environment == nil || *req.Environment != nil {
		t.Fatalf(`{"environment": null} must decode as clear, got %v`, req.Environment)
	}
}

func TestUpdateIncidentRequestOtherNullsStayOmitted(t *testing.T) {
	// Only the listed fields clear on null; a null elsewhere is still "omitted".
	req, err := decodeIncidentUpdate(t, `{"parentId": null, "assignedEngineerId": null}`)
	if err != nil {
		t.Fatal(err)
	}
	if req.ParentID != nil || req.AssignedEngineerID != nil {
		t.Fatalf("a null on a single-pointer field must stay nil, got %v %v", req.ParentID, req.AssignedEngineerID)
	}
}

func TestUpdateIncidentRequestCaseAndDuplicates(t *testing.T) {
	req, err := decodeIncidentUpdate(t, `{"ParentIncidentId": null}`)
	if err != nil {
		t.Fatal(err)
	}
	if req.ParentIncidentID == nil || *req.ParentIncidentID != nil {
		t.Fatalf(`{"ParentIncidentId": null} must decode as clear, got %v`, req.ParentIncidentID)
	}
	if _, err := decodeIncidentUpdate(t, `{"parentIncidentId": null, "ParentIncidentId": "8a6f1c2e-3b4d-4e5f-8a9b-0c1d2e3f4a5b"}`); err == nil {
		t.Fatal("two spellings of one field must be refused")
	}
	if _, err := decodeIncidentUpdate(t, `{"parentIncidentId": null, "nope": 1}`); err == nil {
		t.Fatal("an unknown field must still be refused")
	}
}
