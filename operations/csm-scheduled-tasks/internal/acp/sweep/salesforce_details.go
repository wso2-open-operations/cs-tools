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

package sweep

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/acp/recipients"
	"github.com/wso2-open-operations/cs-tools/operations/csm-scheduled-tasks/internal/acp/salesentity"
)

// DetailSource gives the sweep the Salesforce-owned details of one project:
// it returns the project with those fields filled in and the reader the
// project's decisions read from. Run calls it once per project, after the
// open-project list (with the closure states and the notices sent, which only
// CSM has) came from CSM.
type DetailSource interface {
	forProject(ctx context.Context, proj project) (project, entityReader, error)
}

// CSMDetails reads every detail from CSM, through reader. The tests use it to
// exercise the sweep against a single CSM reader.
func CSMDetails(reader entityReader) DetailSource { return csmDetails{reader: reader} }

type csmDetails struct{ reader entityReader }

func (d csmDetails) forProject(_ context.Context, proj project) (project, entityReader, error) {
	return proj, d.reader, nil
}

// SalesforceReader is the part of the sales-entity-service client the sweep
// uses. Satisfied by *salesentity.Client.
type SalesforceReader interface {
	GetProject(ctx context.Context, id string) (salesentity.Project, error)
	GetCustomer(ctx context.Context, id string) (salesentity.Customer, error)
	ProjectContacts(ctx context.Context, projectID string) ([]salesentity.ProjectContact, error)
	AccountContacts(ctx context.Context, accountID string) ([]salesentity.Contact, error)
	LinkedOpportunities(ctx context.Context, projectID string) ([]salesentity.LinkedOpportunity, error)
	Opportunities(ctx context.Context, ids []string) ([]salesentity.Opportunity, error)
	Invoices(ctx context.Context, opportunityID string) ([]salesentity.Invoice, error)
}

// NewSalesforceDetails reads the details live from Salesforce through sf.
//
// Per project it reads the Salesforce project (by the CSM project's sfId) for
// the start and end dates and its account, and the account for its people,
// classification and primary partner. The project keeps its CSM id, key,
// name and closure records, since every write goes to CSM by that id. The
// account becomes the Salesforce one (id = its Salesforce id, which the
// returned reader uses for contacts); its name stays CSM's, for the notices.
func NewSalesforceDetails(sf SalesforceReader) DetailSource { return salesforceDetails{sf: sf} }

type salesforceDetails struct{ sf SalesforceReader }

func (d salesforceDetails) forProject(ctx context.Context, proj project) (project, entityReader, error) {
	sfID := strings.TrimSpace(stringValue(proj.SfID))
	if sfID == "" {
		return proj, nil, errors.New("project has no Salesforce id")
	}
	sp, err := d.sf.GetProject(ctx, sfID)
	if err != nil {
		return proj, nil, fmt.Errorf("salesforce project: %w", err)
	}
	if proj.StartDate, err = salesforceDate(sp.StartDate); err != nil {
		return proj, nil, fmt.Errorf("salesforce project start date: %w", err)
	}
	if proj.EndDate, err = salesforceDate(sp.EndDate); err != nil {
		return proj, nil, fmt.Errorf("salesforce project end date: %w", err)
	}

	r := &salesforceReader{sf: d.sf, projectSfID: sfID}
	customerID := strings.TrimSpace(stringValue(sp.CustomerID))
	if customerID == "" {
		proj.Account = nil
		return proj, r, nil
	}
	cu, err := d.sf.GetCustomer(ctx, customerID)
	if err != nil {
		return proj, nil, fmt.Errorf("salesforce account: %w", err)
	}
	r.customer = &cu
	name := ""
	if proj.Account != nil {
		name = proj.Account.Name
	}
	isPartner := strings.EqualFold(strings.TrimSpace(stringValue(cu.AccountClassification)), "Partner")
	proj.Account = &projectAccountRef{ID: customerID, Name: name, IsPartner: &isPartner}
	return proj, r, nil
}

// salesforceDate reads a Salesforce Date ("2026-10-13", or a datetime whose
// first ten characters are the date) as UTC midnight, the value CSM's copy
// carries. Empty is nil.
func salesforceDate(v *string) (*time.Time, error) {
	s := strings.TrimSpace(stringValue(v))
	if s == "" {
		return nil, nil
	}
	if len(s) > 10 {
		s = s[:10]
	}
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// salesforceReader answers the sweep's per-project reads from Salesforce in
// the shapes the CSM reads return, so the decision code is unchanged. It is
// bound to one project: the ids the sweep passes for the project are its CSM
// ids and are not used; the account id it passes is the Salesforce one
// forProject put on the project. Each search returns every row on its first
// page with total set, which ends the sweep's paging loops.
type salesforceReader struct {
	sf          SalesforceReader
	projectSfID string
	customer    *salesentity.Customer
}

func (r *salesforceReader) GetAccount(ctx context.Context, accountID string) ([]byte, error) {
	cu := r.customer
	if cu == nil || cu.ID != accountID {
		got, err := r.sf.GetCustomer(ctx, accountID)
		if err != nil {
			return nil, err
		}
		cu = &got
	}
	hasPrimaryPartner := strings.TrimSpace(stringValue(cu.PrimaryPartnerAccountID)) != ""
	return json.Marshal(accountDTO{
		AccountManager:        personFromUser(cu.Owner),
		TechnicalOwner:        personFromEmail(cu.TechnicalOwner),
		RenewalAccountManager: personFromUser(cu.RenewalManager),
		HasPrimaryPartner:     &hasPrimaryPartner,
	})
}

func personFromUser(u *salesentity.User) *personRefDTO {
	if u == nil || (u.Name == nil && u.Email == nil) {
		return nil
	}
	return &personRefDTO{Name: stringValue(u.Name), Email: u.Email}
}

// personFromEmail: Salesforce gives the technical owner as an email only, so
// it is also the name.
func personFromEmail(email *string) *personRefDTO {
	if strings.TrimSpace(stringValue(email)) == "" {
		return nil
	}
	return &personRefDTO{Name: *email, Email: email}
}

// salesforceBusinessContactRole is the Salesforce role label of a business
// contact; CSM stores it as the project role BUSINESS_CONTACT, which the
// recipients rule reads.
const salesforceBusinessContactRole = "Business Contact"

func (r *salesforceReader) SearchProjectContacts(ctx context.Context, _ string, body []byte) ([]byte, error) {
	if !firstPage(body) {
		return json.Marshal(projectContactSearchResponse{})
	}
	members, err := r.sf.ProjectContacts(ctx, r.projectSfID)
	if err != nil {
		return nil, err
	}
	var out []projectContactDTO
	for _, m := range members {
		// A deactivated membership is not a contact of the project; CSM's
		// contact list leaves it out too.
		if strings.EqualFold(strings.TrimSpace(stringValue(m.State)), "DEACTIVATED") {
			continue
		}
		email := strings.TrimSpace(m.Email)
		name := ""
		if m.Contact != nil {
			if e := strings.TrimSpace(stringValue(m.Contact.Email)); e != "" {
				email = e
			}
			name = strings.TrimSpace(stringValue(m.Contact.Name))
		}
		if name == "" {
			name = email
		}
		roles := make([]string, 0, len(m.Roles))
		for _, role := range m.Roles {
			if strings.EqualFold(strings.TrimSpace(role), salesforceBusinessContactRole) {
				role = recipients.BusinessContactRole
			}
			roles = append(roles, role)
		}
		out = append(out, projectContactDTO{Name: name, Email: email, Roles: roles})
	}
	return json.Marshal(projectContactSearchResponse{Contacts: out, Total: len(out)})
}

func (r *salesforceReader) SearchAccountContacts(ctx context.Context, accountID string, body []byte) ([]byte, error) {
	if !firstPage(body) {
		return json.Marshal(accountContactSearchResponse{})
	}
	contacts, err := r.sf.AccountContacts(ctx, accountID)
	if err != nil {
		return nil, err
	}
	out := make([]accountContactDTO, 0, len(contacts))
	for _, c := range contacts {
		out = append(out, accountContactDTO{Name: stringValue(c.Name), Email: strings.TrimSpace(stringValue(c.Email)), IsPrimary: c.IsPrimaryContact})
	}
	return json.Marshal(accountContactSearchResponse{Contacts: out, Total: len(out)})
}

func (r *salesforceReader) SearchProjectOpportunityLinks(ctx context.Context, body []byte) ([]byte, error) {
	if !firstPage(body) {
		return json.Marshal(searchProjectOpportunityLinksResponse{})
	}
	links, err := r.sf.LinkedOpportunities(ctx, r.projectSfID)
	if err != nil {
		return nil, err
	}
	out := make([]projectOpportunityLinkDTO, 0, len(links))
	for _, l := range links {
		oppID := strings.TrimSpace(stringValue(l.OpportunityID))
		if oppID == "" {
			continue
		}
		out = append(out, projectOpportunityLinkDTO{ID: l.ID, Project: &entityRefDTO{ID: r.projectSfID}, Opportunity: &entityRefDTO{ID: oppID}})
	}
	return json.Marshal(searchProjectOpportunityLinksResponse{Links: out, Total: len(out)})
}

func (r *salesforceReader) GetOpportunity(ctx context.Context, id string) ([]byte, error) {
	opps, err := r.sf.Opportunities(ctx, []string{id})
	if err != nil {
		return nil, err
	}
	for _, o := range opps {
		if o.ID == id {
			return json.Marshal(opportunityDTO{
				ID:                 o.ID,
				Name:               o.Name,
				Stage:              o.StageName,
				EulaVersion:        nonBlank(o.EulaVersion),
				EulaVersionDecimal: eulaVersionDecimal(o.EulaVersion),
			})
		}
	}
	return nil, fmt.Errorf("opportunity %s: %w", id, salesentity.ErrNotFound)
}

func (r *salesforceReader) SearchInvoices(ctx context.Context, body []byte) ([]byte, error) {
	var req searchInvoicesRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, fmt.Errorf("parse invoices search request: %w", err)
	}
	if req.Pagination.Offset > 0 {
		return json.Marshal(searchInvoicesResponse{})
	}
	invoices, err := r.sf.Invoices(ctx, req.OpportunityID)
	if err != nil {
		return nil, err
	}
	out := make([]invoiceDTO, 0, len(invoices))
	for _, inv := range invoices {
		out = append(out, invoiceDTO{
			ID:               stringValue(inv.ID),
			Name:             inv.Name,
			InvoiceDate:      dateOnly(inv.InvoiceDate),
			InvoicedPaidDate: dateOnly(inv.PaidDate),
			InvoicedDueDate:  dateOnly(inv.DueDate),
			Opportunity:      &entityRefDTO{ID: req.OpportunityID},
			Classification:   inv.Classification,
			SfID:             inv.ID,
		})
	}
	return json.Marshal(searchInvoicesResponse{Invoices: out, Total: len(out)})
}

// firstPage reports whether a sweep search body asks for the first page.
func firstPage(body []byte) bool {
	var req struct {
		Pagination pagination `json:"pagination"`
	}
	_ = json.Unmarshal(body, &req)
	return req.Pagination.Offset == 0
}

// dateOnly keeps the date part of a Salesforce date or datetime; blank is nil.
func dateOnly(v *string) *string {
	s := strings.TrimSpace(stringValue(v))
	if s == "" {
		return nil
	}
	if len(s) > 10 {
		s = s[:10]
	}
	return &s
}

// nonBlank returns nil for a nil or blank value, as CSM stores it.
func nonBlank(v *string) *string {
	if strings.TrimSpace(stringValue(v)) == "" {
		return nil
	}
	return v
}

// eulaVersionPattern and eulaVersionDecimal reproduce entity-service's
// deriveEulaVersion (the value CSM stored as eula_version_decimal): the first
// x.y in the EULA text, 3.4 when the version is empty, none otherwise.
var eulaVersionPattern = regexp.MustCompile(`\d+\.\d+`)

const defaultEulaVersionDecimal = "3.4"

func eulaVersionDecimal(v *string) *string {
	s := strings.TrimSpace(stringValue(v))
	if s == "" {
		d := defaultEulaVersionDecimal
		return &d
	}
	if m := eulaVersionPattern.FindString(s); m != "" {
		return &m
	}
	return nil
}
