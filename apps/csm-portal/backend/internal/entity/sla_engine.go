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

// SearchActiveSLAStatusesManage calls GET /sla-status?admin=true on the
// entity service -- the CSM-admin-facing request of that endpoint (the
// plain, admin=false request is internal-callers-only and not exposed by
// this backend). limit/offset/source are forwarded as-is; a blank value is
// omitted so entity-service applies its own default.
func (c *CustomerEntityClient) SearchActiveSLAStatusesManage(ctx context.Context, limit, offset, source string) ([]byte, error) {
	q := url.Values{"admin": {"true"}}
	if limit != "" {
		q.Set("limit", limit)
	}
	if offset != "" {
		q.Set("offset", offset)
	}
	if source != "" {
		q.Set("source", source)
	}
	return c.do(ctx, http.MethodGet, "/sla-status?"+q.Encode(), nil)
}

// RequestSLARecalculation calls POST /sla-engine/recalculate on the entity
// service -- the CSM portal's "Recalculate SLA" admin action. No request
// body; entity-service responds 204 once the request has been published.
func (c *CustomerEntityClient) RequestSLARecalculation(ctx context.Context) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/sla-engine/recalculate", nil)
}
