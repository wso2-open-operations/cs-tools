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

package salesentity

import (
	"context"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

const opportunityLineItemSearchPath = "/opportunity-line-items/search"

// GetOpportunityLineItem fetches one Salesforce OpportunityLineItem via
// POST /opportunity-line-items/search {id, limit: 1}. The standalone record
// has the same shape as the one embedded in an Opportunity, plus the parent
// opportunityId the line item ingest resolves. An empty result is a
// ServiceUnavailableError so the caller can retry, as for GetOpportunity.
func (c *Client) GetOpportunityLineItem(ctx context.Context, id string) (SubscriptionLineItem, error) {
	var rows []SubscriptionLineItem
	if err := c.searchWithRetry(ctx, opportunityLineItemSearchPath, idSearchRequest{ID: id, Limit: 1}, "opportunity line item", &rows); err != nil {
		return SubscriptionLineItem{}, err
	}
	for _, li := range rows {
		if li.ID != nil && salesforceIDEqual(*li.ID, id) {
			return li, nil
		}
	}
	if len(rows) == 0 {
		return SubscriptionLineItem{}, NotFound("salesentity: opportunity line item not found")
	}
	return SubscriptionLineItem{}, &apierror.ServiceUnavailableError{Msg: "salesentity: opportunity-line-items/search returned an unexpected line item"}
}
