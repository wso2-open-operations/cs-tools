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

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

const (
	// snProjectResolvePageSize is the largest page the projects/search
	// endpoint serves (it caps limit at 50).
	snProjectResolvePageSize = 50
	// snProjectResolveMaxPages bounds the scan: the search is a substring
	// match on name and key, so a key that is a fragment of many others
	// could otherwise page for a long time. Exceeding it is an error, not a miss.
	snProjectResolveMaxPages = 10
)

// snProjectPoster is the one Client method the resolver uses.
type snProjectPoster interface {
	Post(ctx context.Context, path string, userIDToken string, payload any) (json.RawMessage, error)
}

// snProjectIDResolver implements ProjectIDResolver over the ServiceNow
// projects/search endpoint. It sends no end-user token: the Salesforce ingest
// runs from a webhook and a retry worker with no caller, so the integration
// service falls back to its own service identity. That identity must see every
// project (the endpoint filters to the caller's projects otherwise), or every
// lookup misses and the ingest keeps retrying instead of inserting.
type snProjectIDResolver struct {
	client snProjectPoster
}

// NewServiceNowProjectIDResolver constructs a ProjectIDResolver backed by the
// Choreo ServiceNow API. client must be non-nil.
func NewServiceNowProjectIDResolver(client *integrationservice.Client) ProjectIDResolver {
	return &snProjectIDResolver{client: client}
}

// ResolveProjectID implements ProjectIDResolver. The endpoint's searchQuery
// is a free-text substring match, so results are filtered to an exact,
// case-sensitive key match here, the way project.key is compared in Postgres.
// One match yields sysidToUUID(sys_id). No match is found=false. Several
// distinct matches are a ConflictError: the lookup does not guess.
func (r *snProjectIDResolver) ResolveProjectID(ctx context.Context, key string) (string, bool, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", false, &apierror.ValidationError{Msg: "project key is required"}
	}
	var ids []string
	nearMisses := 0
	for page := 0; page < snProjectResolveMaxPages; page++ {
		offset := page * snProjectResolvePageSize
		raw, err := r.client.Post(ctx, "/projects/search", "", snSearchProjectsPayload{
			Filters:    snProjectFilters{SearchQuery: key},
			Pagination: snProjectPagination{Limit: snProjectResolvePageSize, Offset: offset},
		})
		if err != nil {
			return "", false, err
		}
		var resp snProjectsResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			return "", false, fmt.Errorf("sn project id lookup: parse response: %w", err)
		}
		for _, p := range resp.Projects {
			pk := strings.TrimSpace(p.Key)
			switch {
			case pk == key:
				id := strings.ToLower(sysidToUUID(p.ID))
				if !isCanonicalUUID(id) {
					return "", false, fmt.Errorf("sn project id lookup: project with key %q has an id that is not a 32-character hex id", key)
				}
				if !containsString(ids, id) {
					ids = append(ids, id)
				}
			case strings.EqualFold(pk, key):
				nearMisses++
			}
		}
		if len(resp.Projects) < snProjectResolvePageSize || offset+len(resp.Projects) >= resp.TotalRecords {
			return r.pick(ctx, key, ids, nearMisses)
		}
	}
	return "", false, fmt.Errorf("sn project id lookup: key %q matched more than %d candidates, refusing to guess", key, snProjectResolveMaxPages*snProjectResolvePageSize)
}

func (r *snProjectIDResolver) pick(ctx context.Context, key string, ids []string, nearMisses int) (string, bool, error) {
	switch len(ids) {
	case 0:
		if nearMisses > 0 {
			// Keys are compared case-sensitively, as project.key is; a project
			// whose key differs only in case never matches and keeps retrying.
			slog.WarnContext(ctx, "sn project id lookup: only case-different keys match, treating as not found", "key", key, "candidates", nearMisses)
		}
		return "", false, nil
	case 1:
		return ids[0], true, nil
	default:
		return "", false, &apierror.ConflictError{Msg: fmt.Sprintf("the backing system has %d distinct projects with key %q; refusing to guess which id to use", len(ids), key)}
	}
}
