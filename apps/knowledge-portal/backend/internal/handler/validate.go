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
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Input bounds. maxSearchQueryRunes matches entity-service's own cap so a value
// accepted here is never rejected upstream.
const (
	maxBodyBytes        = 16 << 10
	maxSearchQueryRunes = 200
	defaultLimit        = 20
	maxLimit            = 50
	maxOffset           = 10000
	// sortRecent is the only ordering entity-service offers: newest first.
	sortRecent = "recent"
)

// uuidPattern accepts only the canonical 8-4-4-4-12 hex form of a UUID, in
// either case. Nothing else gets through: no "/", ".", "%", "?", "#", braces or
// whitespace, so an id can never alter an upstream URL.
var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// normalizeID returns id in lower case and true when it is a canonical UUID.
// Every id that reaches an upstream call, the active set or a response goes
// through it, so one article has exactly one spelling here.
func normalizeID(id string) (string, bool) {
	if !uuidPattern.MatchString(id) {
		return "", false
	}
	return strings.ToLower(id), true
}

// searchRequest is the public search body. There is deliberately no states
// field: unknown fields are rejected, so a caller cannot ask for drafts.
type searchRequest struct {
	KnowledgeBaseID string `json:"knowledgeBaseId"`
	SearchQuery     string `json:"searchQuery"`
	SortBy          string `json:"sortBy"`
	Limit           int    `json:"limit"`
	Offset          int    `json:"offset"`
}

// decodeSearchRequest reads and validates the search body. It returns the
// normalised request, or the status and message to answer with. An empty body is
// an empty search.
func decodeSearchRequest(w http.ResponseWriter, r *http.Request) (searchRequest, int, string) {
	var req searchRequest
	if r.Body == nil {
		return req, 0, ""
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return req, http.StatusRequestEntityTooLarge, ErrMsgTooLarge
		}
		return req, http.StatusBadRequest, ErrMsgBadRequest
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return req, http.StatusRequestEntityTooLarge, ErrMsgTooLarge
		}
		return req, http.StatusBadRequest, ErrMsgBadRequest
	}

	req.SearchQuery = strings.TrimSpace(req.SearchQuery)
	if req.KnowledgeBaseID != "" {
		kbID, ok := normalizeID(req.KnowledgeBaseID)
		if !ok {
			return req, http.StatusBadRequest, ErrMsgInvalidID
		}
		req.KnowledgeBaseID = kbID
	}
	switch {
	case !utf8.ValidString(req.SearchQuery), utf8.RuneCountInString(req.SearchQuery) > maxSearchQueryRunes, hasControlRune(req.SearchQuery):
		return req, http.StatusBadRequest, "searchQuery must be at most 200 printable characters."
	case req.Limit < 0 || req.Limit > maxLimit:
		return req, http.StatusBadRequest, "limit must be between 1 and 50."
	case req.Offset < 0 || req.Offset > maxOffset:
		return req, http.StatusBadRequest, "offset must be between 0 and 10000."
	case req.SortBy != "" && req.SortBy != sortRecent:
		return req, http.StatusBadRequest, errMsgUnsupportedBy
	}
	if req.Limit == 0 {
		req.Limit = defaultLimit
	}
	return req, 0, ""
}

func hasControlRune(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}
