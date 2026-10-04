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

const invoiceSearchPath = "/invoices/search"

// Invoice is the subset of a REST sales/sales-entity-service Invoice
// (POST /invoices/search, one Salesforce Invoice__c) the invoice ingest
// needs. The standalone search populates every field, unlike the invoices
// embedded in an Opportunity, and it does not filter out the auto-created
// invoices. OpportunityID is the parent lookup (Opportunity_Name__c, a lookup
// despite its name); Amount is Invoiced_Amount__c; CurrencyCode is
// CurrencyIsoCode and is not stored.
type Invoice struct {
	ID                     *string  `json:"id"`
	Name                   *string  `json:"name"`
	OpportunityID          *string  `json:"opportunityId"`
	Amount                 *float64 `json:"amount"`
	DueDate                *string  `json:"dueDate"`
	PaidDate               *string  `json:"paidDate"`
	InvoiceDate            *string  `json:"invoiceDate"`
	CurrencyCode           *string  `json:"currencyCode"`
	Classification         *string  `json:"classification"`
	Description            *string  `json:"description"`
	ServiceStartDate       *string  `json:"serviceStartDate"`
	ServiceEndDate         *string  `json:"serviceEndDate"`
	OriginalInvoiceDueDate *string  `json:"originalInvoiceDueDate"`
	LastModifiedDate       *string  `json:"lastModifiedDate"`
}

// GetInvoice fetches one Salesforce Invoice__c via POST /invoices/search
// {id, limit: 1}. An empty result is a ServiceUnavailableError so the caller
// can retry, as for GetOpportunity.
func (c *Client) GetInvoice(ctx context.Context, id string) (Invoice, error) {
	var rows []Invoice
	if err := c.searchWithRetry(ctx, invoiceSearchPath, idSearchRequest{ID: id, Limit: 1}, "invoice", &rows); err != nil {
		return Invoice{}, err
	}
	for _, inv := range rows {
		if inv.ID != nil && salesforceIDEqual(*inv.ID, id) {
			return inv, nil
		}
	}
	if len(rows) == 0 {
		return Invoice{}, NotFound("salesentity: invoice not found")
	}
	return Invoice{}, &apierror.ServiceUnavailableError{Msg: "salesentity: invoices/search returned an unexpected invoice"}
}
