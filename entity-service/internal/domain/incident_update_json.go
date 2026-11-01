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

// incidentUpdateNullable lists the UpdateIncidentRequest fields whose explicit
// null clears them, and what setting each to "null" means on the struct: a
// non-nil outer pointer that holds nil. Every other pointer field keeps
// encoding/json's reading of null as "omitted".
var incidentUpdateNullable = []struct {
	name    string
	setNull func(*UpdateIncidentRequest)
}{
	{"parentIncidentId", func(r *UpdateIncidentRequest) { r.ParentIncidentID = new(*string) }},
	{"environment", func(r *UpdateIncidentRequest) { r.Environment = new(*string) }},
}

// UnmarshalJSON keeps an explicit null on a pointer-to-pointer field distinct
// from an omitted one, so `{"parentIncidentId": null}` (the edit dialog's
// cleared Parent incident picker) unlinks the incident from its parent.
//
// encoding/json decodes null into a pointer by setting the pointer itself to
// nil, which for a **string field is "not provided": the save succeeded and
// the link stayed. The service, the repository and the ServiceNow adapter
// treat a non-nil outer pointer holding nil as "clear"; this is what lets an
// HTTP request reach them in that shape. Same approach as
// PatchOutageRequest.
//
// Unknown fields are still rejected, as decodeRequest's DisallowUnknownFields
// would: a custom UnmarshalJSON is handed the raw object and the outer
// decoder's setting does not reach inside it.
func (r *UpdateIncidentRequest) UnmarshalJSON(data []byte) error {
	type plain UpdateIncidentRequest
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
	req := UpdateIncidentRequest(p)
	for _, f := range incidentUpdateNullable {
		// Match the key the way the struct decode just did: case-insensitively.
		// Two spellings in one body are rejected rather than resolved: the
		// struct decode keeps whichever came last, this map cannot see order,
		// and guessing wrong turns a set into a clear.
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
