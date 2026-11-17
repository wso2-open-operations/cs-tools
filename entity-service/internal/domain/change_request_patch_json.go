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
	"fmt"
	"strings"
)

// changeRequestPatchNullable lists the PatchChangeRequestRequest fields whose
// explicit null is a value of its own, and what setting each to "null" means
// on the struct: a non-nil outer pointer that holds nil.
//
// On the pointer-to-pointer fields that is "clear" (for durationInput only on
// the data source that talks to the previous system directly; the PostgreSQL
// data sources refuse any durationInput), except customerGroupId, which is
// listed (with environmentIds) so that its null is refused like any other
// value of a field that is no longer accepted. deploymentIds and
// deploymentProductIds are NOT listed: their null stays "omitted" (an empty
// array is how a list is cleared), as on every other plain pointer field.
var changeRequestPatchNullable = []struct {
	name    string
	setNull func(*PatchChangeRequestRequest)
}{
	{"implementationPlan", func(r *PatchChangeRequestRequest) { r.ImplementationPlan = new(*string) }},
	{"priority", func(r *PatchChangeRequestRequest) { r.Priority = new(*ChangeRequestPriority) }},
	{"category", func(r *PatchChangeRequestRequest) { r.Category = new(*ChangeRequestCategory) }},
	{"requestedById", func(r *PatchChangeRequestRequest) { r.RequestedByID = new(*string) }},
	{"affectedServicesText", func(r *PatchChangeRequestRequest) { r.AffectedServicesText = new(*string) }},
	{"affectedComponentsText", func(r *PatchChangeRequestRequest) { r.AffectedComponentsText = new(*string) }},
	{"rollbackDurationText", func(r *PatchChangeRequestRequest) { r.RollbackDurationText = new(*string) }},
	{"durationInput", func(r *PatchChangeRequestRequest) { r.DurationInput = new(*int) }},
	{"customerGroupId", func(r *PatchChangeRequestRequest) { r.CustomerGroupID = new(*string) }},
	{"environmentIds", func(r *PatchChangeRequestRequest) { r.EnvironmentIDs = new([]string) }},
}

// UnmarshalJSON keeps an explicit null on a pointer-to-pointer field distinct
// from an omitted one, so `{"implementationPlan": null}` clears the plan.
//
// encoding/json decodes null into a pointer by setting the pointer itself to
// nil, which for a **string field is "not provided". The service, the
// repository and the previous system's adapter treat a non-nil outer pointer
// holding nil as "clear"; this is what lets an HTTP request reach them in that
// shape. Same approach as PatchOutageRequest.
//
// Unknown fields are still rejected, as decodeRequest's DisallowUnknownFields
// would: a custom UnmarshalJSON is handed the raw object and the outer
// decoder's setting does not reach inside it.
func (r *PatchChangeRequestRequest) UnmarshalJSON(data []byte) error {
	type plain PatchChangeRequestRequest
	var p plain
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return err
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(data, &keys); err != nil {
		return err
	}
	req := PatchChangeRequestRequest(p)
	for _, f := range changeRequestPatchNullable {
		// Match the key the way the struct decode just did: case-insensitively.
		// Two spellings in one body ({"priority": null, "Priority": "high"}) are
		// rejected rather than resolved: the struct decode keeps whichever came
		// last, this map cannot see order, and guessing wrong turns a set into a
		// clear.
		var raw json.RawMessage
		matches := 0
		for k, v := range keys {
			if strings.EqualFold(k, f.name) {
				raw = v
				matches++
			}
		}
		if matches > 1 {
			return fmt.Errorf("%q is given %d times with different capitalisation", f.name, matches)
		}
		if matches == 1 && bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			f.setNull(&req)
		}
	}
	*r = req
	return nil
}
