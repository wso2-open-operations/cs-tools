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
	"reflect"
	"strings"
	"testing"
)

// decodePatchCRStrict mirrors handler.decodeRequest: DisallowUnknownFields on
// the outer decoder.
func decodePatchCRStrict(t *testing.T, body string) (PatchChangeRequestRequest, error) {
	t.Helper()
	var req PatchChangeRequestRequest
	dec := json.NewDecoder(bytes.NewReader([]byte(body)))
	dec.DisallowUnknownFields()
	err := dec.Decode(&req)
	return req, err
}

// heldNull reports whether a pointer-to-pointer (or pointer-to-slice) field
// holds the "explicit null" state: a non-nil outer pointer whose target is nil.
func heldNull(field any) bool {
	v := reflect.ValueOf(field)
	return v.Kind() == reflect.Pointer && !v.IsNil() && v.Elem().IsNil()
}

// Every pointer-to-pointer field (and environmentIds, which is refused for any
// value) keeps an explicit null apart from an omitted field.
func TestPatchChangeRequestRequest_ExplicitNull(t *testing.T) {
	cases := map[string]func(PatchChangeRequestRequest) any{
		"implementationPlan":     func(r PatchChangeRequestRequest) any { return r.ImplementationPlan },
		"priority":               func(r PatchChangeRequestRequest) any { return r.Priority },
		"category":               func(r PatchChangeRequestRequest) any { return r.Category },
		"requestedById":          func(r PatchChangeRequestRequest) any { return r.RequestedByID },
		"affectedServicesText":   func(r PatchChangeRequestRequest) any { return r.AffectedServicesText },
		"affectedComponentsText": func(r PatchChangeRequestRequest) any { return r.AffectedComponentsText },
		"rollbackDurationText":   func(r PatchChangeRequestRequest) any { return r.RollbackDurationText },
		"durationInput":          func(r PatchChangeRequestRequest) any { return r.DurationInput },
		"customerGroupId":        func(r PatchChangeRequestRequest) any { return r.CustomerGroupID },
		"environmentIds":         func(r PatchChangeRequestRequest) any { return r.EnvironmentIDs },
	}
	if len(cases) != len(changeRequestPatchNullable) {
		t.Fatalf("the test covers %d fields, the decoder lists %d", len(cases), len(changeRequestPatchNullable))
	}
	for name, get := range cases {
		t.Run(name, func(t *testing.T) {
			req, err := decodePatchCRStrict(t, `{"`+name+`": null}`)
			if err != nil {
				t.Fatal(err)
			}
			if !heldNull(get(req)) {
				t.Fatalf(`{"%s": null} must decode as an explicit null (non-nil, holding nil), got %#v`, name, get(req))
			}

			req, err = decodePatchCRStrict(t, `{"title": "x"}`)
			if err != nil {
				t.Fatal(err)
			}
			if v := reflect.ValueOf(get(req)); !v.IsNil() {
				t.Fatalf("an omitted %s must stay nil, got %#v", name, get(req))
			}
		})
	}
}

func TestPatchChangeRequestRequest_ValuesStillDecode(t *testing.T) {
	req, err := decodePatchCRStrict(t, `{
		"implementationPlan": "<p>plan</p>",
		"priority": "high",
		"durationInput": 3600,
		"environmentIds": [],
		"deploymentIds": ["11111111-1111-1111-1111-111111111111"],
		"title": "t"
	}`)
	if err != nil {
		t.Fatal(err)
	}
	if req.ImplementationPlan == nil || *req.ImplementationPlan == nil || **req.ImplementationPlan != "<p>plan</p>" {
		t.Errorf("implementationPlan = %#v", req.ImplementationPlan)
	}
	if req.Priority == nil || *req.Priority == nil || **req.Priority != ChangeRequestPriorityHigh {
		t.Errorf("priority = %#v", req.Priority)
	}
	if req.DurationInput == nil || *req.DurationInput == nil || **req.DurationInput != 3600 {
		t.Errorf("durationInput = %#v", req.DurationInput)
	}
	if req.EnvironmentIDs == nil || *req.EnvironmentIDs == nil || len(*req.EnvironmentIDs) != 0 {
		t.Errorf("environmentIds [] must decode as a non-nil empty list, got %#v", req.EnvironmentIDs)
	}
	if req.DeploymentIDs == nil || len(*req.DeploymentIDs) != 1 {
		t.Errorf("deploymentIds = %#v", req.DeploymentIDs)
	}
	if req.Title == nil || *req.Title != "t" {
		t.Errorf("title = %#v", req.Title)
	}
}

// A null on the list fields that are not listed stays "omitted": an empty array
// is how a list is cleared. A null on a plain *string field stays omitted too.
func TestPatchChangeRequestRequest_NullStaysOmittedElsewhere(t *testing.T) {
	req, err := decodePatchCRStrict(t, `{"deploymentIds": null, "deploymentProductIds": null, "title": null, "implementationDeviation": null, "onHoldReason": null}`)
	if err != nil {
		t.Fatal(err)
	}
	if req.DeploymentIDs != nil || req.DeploymentProductIDs != nil || req.Title != nil || req.ImplementationDeviation != nil || req.OnHoldReason != nil {
		t.Fatalf("these nulls must read as omitted, got %+v", req)
	}
}

func TestPatchChangeRequestRequest_CaseInsensitiveKey(t *testing.T) {
	// encoding/json binds "Priority" to the Priority field, so the null check must too.
	req, err := decodePatchCRStrict(t, `{"Priority": null}`)
	if err != nil {
		t.Fatal(err)
	}
	if !heldNull(req.Priority) {
		t.Fatalf(`{"Priority": null} must decode as an explicit null, got %#v`, req.Priority)
	}

	// Two spellings: the struct decode would keep the last (a set) while a key
	// lookup saw the null (a clear). Neither is safe to guess.
	for _, body := range []string{
		`{"priority": null, "Priority": "high"}`,
		`{"Priority": "high", "priority": null}`,
	} {
		if _, err := decodePatchCRStrict(t, body); err == nil {
			t.Errorf("%s: conflicting spellings of priority must be rejected", body)
		}
	}
}

func TestPatchChangeRequestRequest_RejectsUnknownFields(t *testing.T) {
	_, err := decodePatchCRStrict(t, `{"implementationPlan": null, "bogus": 1}`)
	if err == nil {
		t.Fatal("an unknown field must still be rejected")
	}
	// handler.decodeErrMsg turns this exact prefix into "unknown field ... in request body".
	if !strings.HasPrefix(err.Error(), "json: unknown field ") {
		t.Fatalf("error = %q, want the json package's unknown-field error", err)
	}
}

// A marshalled request (the write-back failure record) reads back the same:
// an omitted field stays omitted, an explicit null stays a null.
func TestPatchChangeRequestRequest_MarshalRoundTrip(t *testing.T) {
	in, err := decodePatchCRStrict(t, `{"implementationPlan": null, "title": "t"}`)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(b, &keys); err != nil {
		t.Fatal(err)
	}
	if raw, ok := keys["implementationPlan"]; !ok || string(raw) != "null" {
		t.Errorf("implementationPlan must marshal as null, got %s", b)
	}
	if _, ok := keys["priority"]; ok {
		t.Errorf("an omitted priority must not be marshalled, got %s", b)
	}
	out, err := decodePatchCRStrict(t, string(b))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round trip changed the request:\n in  %+v\n out %+v", in, out)
	}
}
