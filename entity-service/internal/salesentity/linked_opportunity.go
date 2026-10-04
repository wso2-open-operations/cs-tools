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

const linkedOpportunitySearchPath = "/linked-opportunities/search"

// LinkedOpportunity is one Salesforce Linked_Opportunity__c as REST
// sales/sales-entity-service returns it (POST /linked-opportunities/search):
// the junction between a project (projectId, Project__c) and an opportunity
// (opportunityId, Opportunity__c). name is the "LO-..." record code.
type LinkedOpportunity struct {
	ID               string  `json:"id"`
	Name             *string `json:"name"`
	ProjectID        *string `json:"projectId"`
	OpportunityID    *string `json:"opportunityId"`
	LastModifiedDate *string `json:"lastModifiedDate"`
}

// GetLinkedOpportunity fetches one Linked_Opportunity__c via
// POST /linked-opportunities/search {id, limit: 1}. An empty result is a
// ServiceUnavailableError so the caller can retry, as for GetOpportunity.
func (c *Client) GetLinkedOpportunity(ctx context.Context, id string) (LinkedOpportunity, error) {
	var rows []LinkedOpportunity
	if err := c.searchWithRetry(ctx, linkedOpportunitySearchPath, idSearchRequest{ID: id, Limit: 1}, "linked opportunity", &rows); err != nil {
		return LinkedOpportunity{}, err
	}
	for _, l := range rows {
		if salesforceIDEqual(l.ID, id) {
			return l, nil
		}
	}
	if len(rows) == 0 {
		return LinkedOpportunity{}, NotFound("salesentity: linked opportunity not in search results")
	}
	return LinkedOpportunity{}, &apierror.ServiceUnavailableError{Msg: "salesentity: linked-opportunities/search returned an unexpected linked opportunity"}
}
