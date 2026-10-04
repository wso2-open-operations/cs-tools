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
	"io"
	"net/http"
	"strings"
)

// Bounds ClampSearchPagination applies to a search body's pagination object.
// maxSearchPageLimit is the largest page the portal itself asks for;
// maxSearchPageOffset is well beyond the deepest page any list walks to.
const (
	maxSearchPageLimit  = 100
	maxSearchPageOffset = 100000
)

// IsSearchRoute reports whether a route pattern is a POST .../search route,
// the routes ClampSearchPagination applies to.
func IsSearchRoute(pattern string) bool {
	return strings.HasPrefix(pattern, "POST ") && strings.HasSuffix(pattern, "/search")
}

// ClampSearchPagination bounds `pagination.limit` to [1, maxSearchPageLimit]
// and `pagination.offset` to [0, maxSearchPageOffset] in a search request
// body before the handler sees it. Only numeric values present in the body
// are touched; every other byte of a body that needs no change is passed
// through as sent, and any other field of one that does is carried through
// as json.RawMessage. A body that is too large, not a JSON object, or has no
// pagination object is passed through untouched for the handler's own checks
// to answer.
func ClampSearchPagination(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Body == nil || r.Body == http.NoBody {
			next(w, r)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBodyBytes+1))
		if err != nil {
			// Let the handler report the read failure on what remains.
			r.Body = io.NopCloser(io.MultiReader(bytes.NewReader(body), r.Body))
			next(w, r)
			return
		}
		if len(body) <= maxRequestBodyBytes {
			if clamped, ok := clampPagination(body); ok {
				body = clamped
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
		} else {
			r.Body = io.NopCloser(io.MultiReader(bytes.NewReader(body), r.Body))
		}
		r.ContentLength = -1
		next(w, r)
	}
}

// clampPagination returns the body with pagination bounds applied, and
// whether anything changed.
func clampPagination(body []byte) ([]byte, bool) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil || envelope == nil {
		return nil, false
	}
	rawPagination, ok := envelope["pagination"]
	if !ok {
		return nil, false
	}
	var pagination map[string]json.RawMessage
	if err := json.Unmarshal(rawPagination, &pagination); err != nil || pagination == nil {
		return nil, false
	}
	changed := false
	clamp := func(key string, lo, hi int64) {
		raw, ok := pagination[key]
		if !ok {
			return
		}
		var n json.Number
		if err := json.Unmarshal(raw, &n); err != nil {
			return
		}
		f, err := n.Float64()
		if err != nil {
			return
		}
		var v int64
		switch {
		case f < float64(lo):
			v = lo
		case f > float64(hi):
			v = hi
		default:
			return
		}
		b, _ := json.Marshal(v)
		pagination[key] = b
		changed = true
	}
	clamp("limit", 1, maxSearchPageLimit)
	clamp("offset", 0, maxSearchPageOffset)
	if !changed {
		return nil, false
	}
	pb, err := json.Marshal(pagination)
	if err != nil {
		return nil, false
	}
	envelope["pagination"] = pb
	out, err := json.Marshal(envelope)
	if err != nil {
		return nil, false
	}
	return out, true
}
