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

package middleware

import (
	"net/http"
	"net/url"
	"strings"
)

// NormalizeSysIDs rewrites every path segment and query-string value that is a
// bare ServiceNow sysid (32 hex characters, no hyphens) into the dashed
// 8-4-4-4-12 UUID form the handlers and entity-service expect.
//
// Ids used to reach this API without hyphens, so a client holding one from
// before the switch — a cached bundle, persisted query state, a bookmarked URL
// — would otherwise be rejected with a 400 by the strict UUID checks in the
// handlers. Rewriting the path before routing means the {id}-style path values
// every handler reads are already canonical, with no per-handler conversion.
//
// Ids in a JSON body are converted separately, by the handler package's
// readJSONBody, because that needs to know which keys hold ids.
func NormalizeSysIDs(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := normalizePathSysIDs(r.URL.Path)
		rawQuery, queryChanged := normalizeQuerySysIDs(r.URL.RawQuery)
		if path == r.URL.Path && !queryChanged {
			next.ServeHTTP(w, r)
			return
		}
		// Shallow-copy the request and its URL rather than mutating the
		// caller's — the outer middleware still holds the original.
		r2 := new(http.Request)
		*r2 = *r
		u := *r.URL
		if path != r.URL.Path {
			u.Path = path
			// RawPath is only set when the escaped form differs from the
			// default encoding of Path; a stale one would win over the
			// rewritten Path.
			u.RawPath = ""
		}
		if queryChanged {
			u.RawQuery = rawQuery
		}
		r2.URL = &u
		next.ServeHTTP(w, r2)
	})
}

// normalizeQuerySysIDs dashes every bare-sysid value in a raw query string,
// whatever its key — the API's query parameters are ids, enums, dates and
// paging numbers, none of which is a 32-hex string that must stay bare.
// Returns the original string and false when nothing changed, so an untouched
// query keeps its exact encoding and parameter order.
func normalizeQuerySysIDs(rawQuery string) (string, bool) {
	if rawQuery == "" {
		return rawQuery, false
	}
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return rawQuery, false
	}
	changed := false
	for _, vs := range values {
		for i, v := range vs {
			if dashed := dashSysID(v); dashed != v {
				vs[i] = dashed
				changed = true
			}
		}
	}
	if !changed {
		return rawQuery, false
	}
	return values.Encode(), true
}

// normalizePathSysIDs dashes each bare-sysid segment of path, leaving every
// other segment untouched. Returns path itself when nothing changed.
func normalizePathSysIDs(path string) string {
	segments := strings.Split(path, "/")
	changed := false
	for i, seg := range segments {
		if dashed := dashSysID(seg); dashed != seg {
			segments[i] = dashed
			changed = true
		}
	}
	if !changed {
		return path
	}
	return strings.Join(segments, "/")
}

// dashSysID converts a 32-hex string into a lowercase dashed UUID, returning
// anything else unchanged.
func dashSysID(s string) string {
	if len(s) != 32 {
		return s
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return s
		}
	}
	s = strings.ToLower(s)
	return s[0:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:32]
}
