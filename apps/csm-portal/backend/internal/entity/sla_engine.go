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

package entity

import (
	"context"
	"net/http"
	"net/url"
)

// SLAStatusManageFilters are the CSM Portal's SLA Engine admin table's own
// optional filters, forwarded to entity-service's GET /sla-status one-to-one
// -- see that endpoint's own doc comment (service.ParseSLAStatusFilters)
// for the accepted values of each. Every field is optional; the zero value
// applies no filtering, matching this endpoint's original behavior.
type SLAStatusManageFilters struct {
	Severity      string
	CaseType      string
	Search        string
	ElapsedBucket string
	Status        string
}

// SearchActiveSLAStatusesManage calls GET /sla-status?admin=true on the
// entity service -- the CSM-admin-facing request of that endpoint (the
// plain, admin=false request is internal-callers-only and not exposed by
// this backend). limit/offset are forwarded as-is; a blank value is omitted
// so entity-service applies its own default. There used to be a source
// param forwarded here too -- dropped along with entity-service's own
// sla.source column (migration 0268 there); entity-service already ignores
// one if it's still sent, but there's no reason to keep sending it. filters
// are forwarded the same way -- a blank field is simply omitted, so a caller
// sending none of them sees entity-service's original, unfiltered behavior.
func (c *CustomerEntityClient) SearchActiveSLAStatusesManage(ctx context.Context, limit, offset string, filters SLAStatusManageFilters) ([]byte, error) {
	q := url.Values{"admin": {"true"}}
	if limit != "" {
		q.Set("limit", limit)
	}
	if offset != "" {
		q.Set("offset", offset)
	}
	if filters.Severity != "" {
		q.Set("severity", filters.Severity)
	}
	if filters.CaseType != "" {
		q.Set("caseType", filters.CaseType)
	}
	if filters.Search != "" {
		q.Set("search", filters.Search)
	}
	if filters.ElapsedBucket != "" {
		q.Set("elapsedBucket", filters.ElapsedBucket)
	}
	if filters.Status != "" {
		q.Set("status", filters.Status)
	}
	return c.do(ctx, http.MethodGet, "/sla-status?"+q.Encode(), nil)
}

// RequestSLARecalculation calls POST /sla-engine/recalculate on the entity
// service -- the CSM portal's "Recalculate SLA" admin action. No request
// body; entity-service responds 204 once the request has been published.
func (c *CustomerEntityClient) RequestSLARecalculation(ctx context.Context) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/sla-engine/recalculate", nil)
}
