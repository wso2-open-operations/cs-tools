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

const projectSearchPath = "/projects/search"

// Project is the subset of a REST sales/sales-entity-service Project
// (POST /projects/search) the Project ingest reads: one Salesforce
// Project__c. The field names follow that service's contract, where the
// project is a "subscription" of an account: key is Project_Key__c, type is
// Project_Type__c and customerId is Account__c. Dates are Salesforce Date
// values ("2026-09-29"); lastModifiedDate is a DateTime.
//
// Sales Entity also returns status, closureStates, infraCostMode,
// useCaseCategory and muteQuietReportUntil. They are left out on purpose:
// the closure states flow from CSM to Salesforce, not back
// (SALESFORCE_SYNC_PLAN.md §6), and the rest have no CSM column.
type Project struct {
	ID                      string  `json:"id"`
	Name                    *string `json:"name"`
	Key                     *string `json:"key"`
	Description             *string `json:"description"`
	Type                    *string `json:"type"`
	StartDate               *string `json:"startDate"`
	EndDate                 *string `json:"endDate"`
	ComplianceViolationDate *string `json:"complianceViolationDate"`
	GoLiveDate              *string `json:"goLiveDate"`
	CustomerID              *string `json:"customerId"`
	LastModifiedDate        *string `json:"lastModifiedDate"`
}

// GetProject fetches one Salesforce Project__c via POST /projects/search
// {id, limit: 1}. An empty result is a ServiceUnavailableError so the caller
// can retry: the Salesforce event can arrive before the record is visible to
// the query, same as GetProjectContact.
func (c *Client) GetProject(ctx context.Context, id string) (Project, error) {
	var rows []Project
	if err := c.searchWithRetry(ctx, projectSearchPath, idSearchRequest{ID: id, Limit: 1}, "project", &rows); err != nil {
		return Project{}, err
	}
	for _, p := range rows {
		if salesforceIDEqual(p.ID, id) {
			return p, nil
		}
	}
	if len(rows) == 0 {
		return Project{}, NotFound("salesentity: project not in search results")
	}
	return Project{}, &apierror.ServiceUnavailableError{Msg: "salesentity: projects/search returned an unexpected project"}
}
