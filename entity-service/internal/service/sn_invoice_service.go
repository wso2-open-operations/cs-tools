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

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/middleware"
	integrationservice "github.com/wso2-open-operations/cs-tools/entity-service/internal/servicenow-integration-service"
)

// snInvoicesResponse mirrors the Choreo POST /invoices/search response.
type snInvoicesResponse struct {
	Invoices     []snInvoice `json:"invoices"`
	TotalRecords int         `json:"totalRecords"`
	Offset       int         `json:"offset"`
	Limit        int         `json:"limit"`
}

// snInvoice mirrors the Choreo Invoice shape. Every field but ID is nilable: ServiceNow can
// omit any of them entirely for a sparsely-populated row. Date fields are date-only strings
// (yyyy-MM-dd, snDateLayout), same as project start/end dates.
type snInvoice struct {
	ID                     string           `json:"id"`
	Name                   *string          `json:"name"`
	InvoicedAmount         *string          `json:"invoicedAmount"`
	InvoiceDate            *string          `json:"invoiceDate"`
	InvoicedPaidDate       *string          `json:"invoicedPaidDate"`
	InvoicedDueDate        *string          `json:"invoicedDueDate"`
	InvoiceOriginalDueDate *string          `json:"invoiceOriginalDueDate"`
	Opportunity            *snCaseEntityRef `json:"opportunity"`
	Classification         *string          `json:"classification"`
	SfID                   *string          `json:"sfId"`
}

// snInvoiceSearchPayload is the Choreo POST /invoices/search request body.
type snInvoiceSearchPayload struct {
	Filters    snInvoiceFilters    `json:"filters,omitempty"`
	Pagination snProjectPagination `json:"pagination"`
}

type snInvoiceFilters struct {
	OpportunityID string `json:"opportunityId,omitempty"`
}

type snInvoiceService struct {
	client *integrationservice.Client
}

// NewServiceNowInvoiceService constructs an InvoiceService backed by the Choreo API.
func NewServiceNowInvoiceService(client *integrationservice.Client) InvoiceService {
	return &snInvoiceService{client: client}
}

// snOptionalDate validates an optional date-only ServiceNow field (snDateLayout), returning nil
// for an absent or empty value and the original date-only string when valid. Mirrors
// optionalSNProjectDate's semantics for project dates: a present-but-malformed value is an
// error rather than a silent nil. The value is kept as a string (not time.Time) so it marshals
// as a bare YYYY-MM-DD, matching openapi.yaml's `format: date` for these fields.
func snOptionalDate(label string, v *string) (*string, error) {
	if v == nil || *v == "" {
		return nil, nil
	}
	if _, err := time.Parse(snDateLayout, *v); err != nil {
		return nil, fmt.Errorf("sn invoices: parse %s %q: %w", label, *v, err)
	}
	return v, nil
}

func snInvoiceToDomain(i snInvoice) (domain.Invoice, error) {
	invoiceDate, err := snOptionalDate("invoiceDate", i.InvoiceDate)
	if err != nil {
		return domain.Invoice{}, err
	}
	invoicedPaidDate, err := snOptionalDate("invoicedPaidDate", i.InvoicedPaidDate)
	if err != nil {
		return domain.Invoice{}, err
	}
	invoicedDueDate, err := snOptionalDate("invoicedDueDate", i.InvoicedDueDate)
	if err != nil {
		return domain.Invoice{}, err
	}
	invoiceOriginalDueDate, err := snOptionalDate("invoiceOriginalDueDate", i.InvoiceOriginalDueDate)
	if err != nil {
		return domain.Invoice{}, err
	}

	return domain.Invoice{
		ID:                     sysidToUUID(i.ID),
		Name:                   i.Name,
		InvoicedAmount:         i.InvoicedAmount,
		InvoiceDate:            invoiceDate,
		InvoicedPaidDate:       invoicedPaidDate,
		InvoicedDueDate:        invoicedDueDate,
		InvoiceOriginalDueDate: invoiceOriginalDueDate,
		Opportunity:            snEntityRefFromCaseRef(i.Opportunity),
		Classification:         i.Classification,
		SfID:                   i.SfID,
	}, nil
}

// SearchInvoices implements InvoiceService.
func (s *snInvoiceService) SearchInvoices(ctx context.Context, req domain.SearchInvoicesRequest) (domain.SearchInvoicesResponse, error) {
	if err := normalizePagination(&req.Pagination); err != nil {
		return domain.SearchInvoicesResponse{}, err
	}
	var opportunitySysid string
	if req.OpportunityID != "" {
		if err := validateUUIDs("opportunityId", []string{req.OpportunityID}); err != nil {
			return domain.SearchInvoicesResponse{}, err
		}
		opportunitySysid = uuidToSysid(req.OpportunityID)
	}

	token := middleware.UserIDTokenFromContext(ctx)

	payload := snInvoiceSearchPayload{
		Filters:    snInvoiceFilters{OpportunityID: opportunitySysid},
		Pagination: snProjectPagination{Limit: req.Pagination.Limit, Offset: req.Pagination.Offset},
	}

	raw, err := s.client.Post(ctx, "/invoices/search", token, payload)
	if err != nil {
		return domain.SearchInvoicesResponse{}, err
	}

	var snResp snInvoicesResponse
	if err := json.Unmarshal(raw, &snResp); err != nil {
		return domain.SearchInvoicesResponse{}, fmt.Errorf("sn invoices: parse response: %w", err)
	}

	invoices := make([]domain.Invoice, 0, len(snResp.Invoices))
	for _, i := range snResp.Invoices {
		inv, err := snInvoiceToDomain(i)
		if err != nil {
			return domain.SearchInvoicesResponse{}, err
		}
		invoices = append(invoices, inv)
	}

	return domain.SearchInvoicesResponse{
		Invoices: invoices,
		Total:    snResp.TotalRecords,
		Limit:    req.Pagination.Limit,
		Offset:   req.Pagination.Offset,
		HasMore:  req.Pagination.Offset+len(invoices) < snResp.TotalRecords,
	}, nil
}

// GetInvoiceByID implements InvoiceService.
func (s *snInvoiceService) GetInvoiceByID(ctx context.Context, id string) (domain.Invoice, error) {
	if err := validateUUIDs("id", []string{id}); err != nil {
		return domain.Invoice{}, err
	}

	token := middleware.UserIDTokenFromContext(ctx)

	raw, err := s.client.Get(ctx, "/invoices/"+uuidToSysid(id), token)
	if err != nil {
		return domain.Invoice{}, err
	}

	var i snInvoice
	if err := json.Unmarshal(raw, &i); err != nil {
		return domain.Invoice{}, fmt.Errorf("sn invoices: parse invoice response: %w", err)
	}

	return snInvoiceToDomain(i)
}
