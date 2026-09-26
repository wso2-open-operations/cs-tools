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

package handler

import (
	"bytes"
	"encoding/json"
	"strings"
)

// sysidBodyKeys are the JSON keys whose values entity-service validates as
// dashed UUIDs (its validateUUIDs), matched case-insensitively the way
// encoding/json matches struct tags. A bare 32-hex sysid under one of these
// keys is rewritten to its dashed form; a 32-hex value under any other key is
// left alone, because not every id-shaped field is a ServiceNow sysid — an
// application id issued by the product-consumption service, for one, must go
// back exactly as it came.
var sysidBodyKeys = map[string]struct{}{
	"accountid": {}, "accountids": {}, "approvedbyid": {}, "approverid": {}, "approverids": {},
	"assignedengineerid": {}, "assignedteamid": {}, "assigneduserid": {}, "assignmentgroupid": {},
	"attachmentid": {}, "callerid": {}, "caseid": {}, "caseids": {}, "catalogid": {},
	"catalogitemid": {}, "causedbyid": {}, "changerequestid": {}, "chipids": {},
	"configurationitemid": {}, "configurationitemids": {}, "contactid": {}, "conversationid": {},
	"customergroupid": {}, "deployedproductid": {}, "deployedproductids": {}, "deploymentid": {},
	"deploymentids": {}, "deploymentproductids": {}, "emojiid": {}, "environmentids": {},
	"groupid": {}, "groupids": {}, "incidentid": {}, "incidentids": {}, "opportunityid": {},
	"parentid": {}, "parentids": {}, "parentincidentid": {}, "problemid": {}, "productid": {},
	"projectid": {}, "projectids": {}, "referenceid": {}, "relatedcaseid": {},
	"requestedbyid": {}, "resolvedbyid": {}, "serviceid": {}, "serviceids": {},
	"serviceofferingid": {}, "tagid": {}, "taskids": {}, "userid": {}, "userids": {},
	"versionid": {}, "watchlist": {},
}

// normalizeBodyIDs rewrites bare sysids under the keys in sysidBodyKeys to
// dashed UUIDs, at any depth of a JSON body. A body that contains none is
// returned byte-for-byte, so the common case is untouched; only a body that
// actually needed rewriting is re-encoded. On any decode problem the original
// body is returned — readJSONBody has already established that it is valid.
func normalizeBodyIDs(body []byte) []byte {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber() // keep large integers exact through the round trip
	var root any
	if err := dec.Decode(&root); err != nil {
		return body
	}
	if !normalizeValue(root, false) {
		return body
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(root); err != nil {
		return body
	}
	return bytes.TrimRight(buf.Bytes(), "\n")
}

// normalizeValue walks v in place, dashing bare sysids. underIDKey is true when
// v is the value (or an element of the array value) of a sysidBodyKeys key.
// Reports whether anything changed.
func normalizeValue(v any, underIDKey bool) bool {
	changed := false
	switch t := v.(type) {
	case map[string]any:
		for k, child := range t {
			_, isIDKey := sysidBodyKeys[strings.ToLower(k)]
			if s, ok := child.(string); ok {
				if isIDKey && sysidRe.MatchString(s) {
					t[k] = toDashedID(s)
					changed = true
				}
				continue
			}
			if normalizeValue(child, isIDKey) {
				changed = true
			}
		}
	case []any:
		for i, child := range t {
			if s, ok := child.(string); ok {
				if underIDKey && sysidRe.MatchString(s) {
					t[i] = toDashedID(s)
					changed = true
				}
				continue
			}
			if normalizeValue(child, underIDKey) {
				changed = true
			}
		}
	}
	return changed
}

// dashIfSysID dashes id when it is a bare 32-hex sysid and returns it unchanged
// otherwise. For ids read from a WebSocket frame, which never pass through
// readJSONBody.
func dashIfSysID(id string) string {
	if sysidRe.MatchString(id) {
		return toDashedID(id)
	}
	return id
}
