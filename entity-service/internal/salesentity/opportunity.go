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
	"bytes"
	"context"
	"encoding/json"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
)

const opportunitySearchPath = "/opportunities/search"

// Opportunity is the subset of a REST sales/sales-entity-service
// OpportunityData (POST /opportunities/search) the Opportunity ingest needs.
// Field names follow that service's contract (an account id is a
// "customerId", an OpportunityLineItem a "subscription line item").
//
// LastModifiedDate, EulaVersion, Type and EngagementCode are an additive
// Sales Entity extension for the Salesforce sync; a build without it simply
// omits the keys. EulaVersion is an OptionalString because the ingest must
// tell "Sales Entity sent eulaVersion: null" (the record has no EULA version,
// store the 3.4 default) from "Sales Entity does not send eulaVersion at all"
// (an older build, keep the stored value).
type Opportunity struct {
	ID                          string                 `json:"id"`
	Name                        *string                `json:"name"`
	CustomerID                  string                 `json:"customerId"`
	StageName                   *string                `json:"stageName"`
	IsWon                       *bool                  `json:"isWon"`
	SupportAccountEndDateRollUp *string                `json:"supportAccountEndDateRollUp"`
	SubscriptionLineItems       []SubscriptionLineItem `json:"subscriptionLineItems"`
	LastModifiedDate            *string                `json:"lastModifiedDate"`
	EulaVersion                 OptionalString         `json:"eulaVersion"`
	Type                        *string                `json:"type"`
	EngagementCode              *string                `json:"engagementCode"`
}

// SubscriptionLineItem is one Salesforce OpportunityLineItem embedded in an
// Opportunity.
type SubscriptionLineItem struct {
	ID               *string              `json:"id"`
	OpportunityID    *string              `json:"opportunityId"`
	Name             *string              `json:"name"`
	Quantity         *float64             `json:"quantity"`
	TotalPrice       *float64             `json:"totalPrice"`
	Environment      *string              `json:"environment"`
	Classification   *string              `json:"classification"`
	ServiceStartDate *string              `json:"serviceStartDate"`
	ServiceEndDate   *string              `json:"serviceEndDate"`
	Product          *SubscriptionProduct `json:"product"`
	LastModifiedDate *string              `json:"lastModifiedDate"`
}

// SubscriptionProduct is the Product2 embedded in a SubscriptionLineItem.
type SubscriptionProduct struct {
	ID             *string `json:"id"`
	Name           *string `json:"name"`
	Description    *string `json:"description"`
	ProductUnit    *string `json:"productUnit"`
	Family         *string `json:"family"`
	ProductCode    *string `json:"productCode"`
	EngProductCode *string `json:"engProductCode"`
}

// OptionalString is a nullable JSON string that also records whether its key
// was present at all. encoding/json calls UnmarshalJSON for a present key,
// null included, and never for an absent one, so Present is false exactly
// when the key is missing.
type OptionalString struct {
	Present bool
	Value   *string
}

// UnmarshalJSON implements json.Unmarshaler.
func (o *OptionalString) UnmarshalJSON(b []byte) error {
	o.Present = true
	if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		o.Value = nil
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	o.Value = &s
	return nil
}

// MarshalJSON implements json.Marshaler (null when absent or null).
func (o OptionalString) MarshalJSON() ([]byte, error) {
	if o.Value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(*o.Value)
}

// GetOpportunity fetches one Salesforce Opportunity, with its line items
// embedded, via POST /opportunities/search {id, limit: 1}. An empty result is
// a ServiceUnavailableError so the caller can retry — the Salesforce event
// can arrive before the record is visible to the query, same as
// GetProjectContact.
func (c *Client) GetOpportunity(ctx context.Context, id string) (Opportunity, error) {
	var rows []Opportunity
	if err := c.searchWithRetry(ctx, opportunitySearchPath, idSearchRequest{ID: id, Limit: 1}, "opportunity", &rows); err != nil {
		return Opportunity{}, err
	}
	for _, o := range rows {
		if salesforceIDEqual(o.ID, id) {
			return o, nil
		}
	}
	if len(rows) == 0 {
		return Opportunity{}, NotFound("salesentity: opportunity not found")
	}
	return Opportunity{}, &apierror.ServiceUnavailableError{Msg: "salesentity: opportunities/search returned an unexpected opportunity"}
}
