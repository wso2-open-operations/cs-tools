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

// UnmarshalJSON keeps an explicit `"end": null` distinct from an omitted end.
//
// *** THE PORTAL'S REOPEN BUTTON SENDS EXACTLY {"end": null}. *** encoding/json
// decodes null into a pointer by setting the pointer itself to nil, so the
// **string End came out nil -- "not provided" -- and the update was rejected
// with "at least one field must be provided". Every reopen failed. The service
// and repository already treat a non-nil End holding nil as "reopen"; this is
// what lets the request reach them in that shape.
//
// Unknown fields are still rejected, as decodeRequest's DisallowUnknownFields
// would: a custom UnmarshalJSON is handed the raw object and the outer
// decoder's setting does not reach inside it.
func (r *PatchOutageRequest) UnmarshalJSON(data []byte) error {
	type plain PatchOutageRequest
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
	// Match "end" the way the struct decode just did -- case-insensitively,
	// so {"End": null} reopens too. Two spellings in one body ({"end": null,
	// "End": "..."}) are rejected rather than resolved: the struct decode keeps
	// whichever came last, this map cannot see order, and guessing wrong turns
	// a close into a reopen.
	var endRaw json.RawMessage
	matches := 0
	for k, v := range keys {
		if strings.EqualFold(k, "end") {
			endRaw = v
			matches++
		}
	}
	if matches > 1 {
		return fmt.Errorf(`"end" is given %d times with different capitalisation`, matches)
	}
	if matches == 1 && bytes.Equal(bytes.TrimSpace(endRaw), []byte("null")) {
		var reopen *string
		p.End = &reopen
	}
	*r = PatchOutageRequest(p)
	return nil
}
