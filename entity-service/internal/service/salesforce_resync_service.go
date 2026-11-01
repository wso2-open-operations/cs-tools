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
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/wso2-open-operations/cs-tools/entity-service/internal/apierror"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/auth"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/domain"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/repository"
	"github.com/wso2-open-operations/cs-tools/entity-service/internal/salesentity"
)

const (
	// resyncMaxSalesforceCalls is the number of Salesforce (sales-entity)
	// calls one preview or apply request may make. A component that would
	// need more is skipped, not partially compared.
	resyncMaxSalesforceCalls = 200
	// resyncOpportunityPageSize is the opportunities asked for per call: each
	// one carries its line items, so the page stays small.
	resyncOpportunityPageSize = 50
	// resyncListPageSize is the page size of every other list search (Sales
	// Entity caps a page at 1000).
	resyncListPageSize = 1000
	// resyncIDBatchSize is the record ids sent in one batch lookup of contacts.
	resyncIDBatchSize = 200
	// resyncTimeMargin is how much of the request's own deadline is kept free
	// for answering: with less left, no further Salesforce call is started
	// and the component is skipped as capped.
	resyncTimeMargin = 8 * time.Second
	// resyncMaxApplyErrors bounds the errors listed per component.
	resyncMaxApplyErrors = 50
	// resyncKeysetEpoch is the first keyset page's (inclusive) lower bound.
	resyncKeysetEpoch = "1970-01-01T00:00:00Z"
)

// errResyncCapped is returned when a request has used its Salesforce call (or
// time) budget; the component that hit it is skipped.
var errResyncCapped = errors.New("salesforce resync call budget exhausted")

// errResyncNoCursor is returned when a full page came back without the
// lastModifiedDate and id the next page is keyed on.
var errResyncNoCursor = errors.New("salesforce did not return lastModifiedDate for a record, so this list cannot be read completely")

// SalesEntityResyncClient is the list searches the resync makes against REST
// sales/sales-entity-service.
type SalesEntityResyncClient interface {
	SearchProjects(ctx context.Context, f salesentity.SearchFilter) ([]salesentity.Project, error)
	SearchOpportunities(ctx context.Context, f salesentity.SearchFilter) ([]salesentity.Opportunity, error)
	SearchLinkedOpportunities(ctx context.Context, f salesentity.SearchFilter) ([]salesentity.LinkedOpportunity, error)
	SearchInvoices(ctx context.Context, f salesentity.SearchFilter) ([]salesentity.Invoice, error)
	SearchContacts(ctx context.Context, f salesentity.SearchFilter) ([]salesentity.Contact, error)
	SearchProjectContacts(ctx context.Context, f salesentity.SearchFilter) ([]salesentity.ProjectContact, error)
}

type salesforceResyncService struct {
	ev         *salesforceEventService
	search     SalesEntityResyncClient
	reads      repository.SalesforceResyncReadRepository
	access     AccessService
	adminRoles []string

	mu      sync.Mutex
	running map[string]struct{}
}

// NewSalesforceResyncService builds the resync over an ingest service built by
// one of the NewSalesforceEventService constructors: each component is on
// only when the matching ingest is (its feature flag and dependencies), and
// the apply writes through that service's own ingest functions. adminRoles is
// AUTH_ADMIN_ROLES, as for the skill catalogue: only a caller whose token
// holds one of them may preview or apply.
func NewSalesforceResyncService(ingest SalesforceEventService, search SalesEntityResyncClient, reads repository.SalesforceResyncReadRepository, access AccessService, adminRoles []string) (SalesforceResyncService, error) {
	ev, ok := ingest.(*salesforceEventService)
	if !ok {
		return nil, errors.New("salesforce resync needs the Salesforce ingest service")
	}
	return &salesforceResyncService{ev: ev, search: search, reads: reads, access: access, adminRoles: adminRoles, running: map[string]struct{}{}}, nil
}

// requireAdmin is the admin gate, the same as the skill catalogue's edits: an
// internal caller with a user token whose roles hold one of AUTH_ADMIN_ROLES.
func (s *salesforceResyncService) requireAdmin(ctx context.Context) error {
	if err := RequireInternalCaller(ctx, s.access, "the Salesforce resync is only available to internal staff"); err != nil {
		return err
	}
	id := auth.IdentityFromContext(ctx)
	if id.UserEmail == "" {
		return &apierror.ForbiddenError{Msg: "the Salesforce resync needs a user token, not a service credential"}
	}
	if !holdsAnyRole(id.Roles, s.adminRoles) {
		return &apierror.ForbiddenError{Msg: "the Salesforce resync is for a CSM admin only"}
	}
	return nil
}

// lock admits one resync per target at a time (in this process); the returned
// func releases it.
func (s *salesforceResyncService) lock(scope, id string) (func(), error) {
	key := scope + ":" + strings.ToLower(strings.TrimSpace(id))
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, busy := s.running[key]; busy {
		return nil, &apierror.ConflictError{Msg: "a Salesforce resync is already running for this " + scope + "; wait for it to finish"}
	}
	s.running[key] = struct{}{}
	return func() {
		s.mu.Lock()
		delete(s.running, key)
		s.mu.Unlock()
	}, nil
}

// resyncComponentDef is one component of a scope.
type resyncComponentDef struct {
	key, label string
}

var (
	resyncAccountComponents = []resyncComponentDef{
		{domain.SalesforceResyncComponentAccount, "Account"},
		{domain.SalesforceResyncComponentPartners, "Partners"},
		{domain.SalesforceResyncComponentProjects, "Projects"},
		{domain.SalesforceResyncComponentOpportunities, "Opportunities and line items"},
		{domain.SalesforceResyncComponentLinkedOpportunities, "Linked opportunities"},
		{domain.SalesforceResyncComponentInvoices, "Invoices"},
		{domain.SalesforceResyncComponentContacts, "Contacts"},
		{domain.SalesforceResyncComponentMemberships, "Memberships"},
	}
	resyncProjectComponents = []resyncComponentDef{
		{domain.SalesforceResyncComponentProject, "Project"},
		{domain.SalesforceResyncComponentOpportunities, "Opportunities and line items"},
		{domain.SalesforceResyncComponentLinkedOpportunities, "Linked opportunities"},
		{domain.SalesforceResyncComponentInvoices, "Invoices"},
		{domain.SalesforceResyncComponentMemberships, "Memberships"},
	}
)

func resyncComponentsOf(scope string) ([]resyncComponentDef, error) {
	switch scope {
	case domain.SalesforceResyncScopeAccount:
		return resyncAccountComponents, nil
	case domain.SalesforceResyncScopeProject:
		return resyncProjectComponents, nil
	default:
		return nil, &apierror.ValidationError{Msg: `scope must be "account" or "project"`}
	}
}

// disabledReason is why a component cannot run, or "" when it can: its ingest
// feature flag is off or its ingest struct was never built, the ledger every
// ingest records into is missing, or (for the list components) there is no
// search client.
func (s *salesforceResyncService) disabledReason(key string) string {
	ev := s.ev
	if ev.support.States == nil {
		return "the salesforce_ingest_state ledger is not configured"
	}
	needsSearch := key != domain.SalesforceResyncComponentAccount && key != domain.SalesforceResyncComponentPartners && key != domain.SalesforceResyncComponentProject
	if needsSearch && s.search == nil {
		return "the Salesforce list search is not configured"
	}
	switch key {
	case domain.SalesforceResyncComponentAccount:
		if ev.repo == nil || ev.se == nil {
			return "account ingest is disabled (CSM_MIGRATION_SALESFORCE_ACCOUNT_INGEST_ENABLED is off)"
		}
	case domain.SalesforceResyncComponentPartners:
		if !ev.partners.enabled() {
			return "partner ingest is disabled (CSM_MIGRATION_SALESFORCE_PARTNER_INGEST_ENABLED is off)"
		}
		if ev.repo == nil {
			return "partner refresh needs the account ingest, which is disabled"
		}
	case domain.SalesforceResyncComponentProjects, domain.SalesforceResyncComponentProject:
		if !ev.project.enabled() {
			return "project ingest is disabled (CSM_MIGRATION_SALESFORCE_PROJECT_INGEST_ENABLED is off)"
		}
	case domain.SalesforceResyncComponentOpportunities:
		if !ev.opportunity.enabled() {
			return "opportunity ingest is disabled (CSM_MIGRATION_SALESFORCE_OPPORTUNITY_INGEST_ENABLED is off)"
		}
	case domain.SalesforceResyncComponentLinkedOpportunities:
		if !ev.linkedOpportunityEnabled() {
			return "linked opportunity ingest is disabled (CSM_MIGRATION_SALESFORCE_OPPORTUNITY_INGEST_ENABLED is off)"
		}
	case domain.SalesforceResyncComponentInvoices:
		if !ev.invoices.enabled() || !ev.opportunity.enabled() {
			return "invoice ingest is disabled (CSM_MIGRATION_SALESFORCE_OPPORTUNITY_INGEST_ENABLED is off)"
		}
	case domain.SalesforceResyncComponentContacts:
		if !ev.membership.enabled() || ev.membership.Contacts == nil {
			return "contact ingest is disabled (CSM_MIGRATION_SALESFORCE_MEMBERSHIP_INGEST_ENABLED is off)"
		}
	case domain.SalesforceResyncComponentMemberships:
		if !ev.membership.enabled() {
			return "membership ingest is disabled (CSM_MIGRATION_SALESFORCE_MEMBERSHIP_INGEST_ENABLED is off)"
		}
	}
	return ""
}

// callBudget counts the Salesforce calls of one request.
type callBudget struct {
	max  int
	used int
	// reason is set once the budget is exhausted: the cappedReason.
	reason string
}

// spend takes one call from the budget, or reports errResyncCapped when none
// is left or the request's deadline is too close to start another.
func (b *callBudget) spend(ctx context.Context) error {
	if b.reason != "" {
		return errResyncCapped
	}
	if dl, ok := ctx.Deadline(); ok && time.Until(dl) < resyncTimeMargin {
		b.reason = fmt.Sprintf("the request time limit is close after %d Salesforce calls; compare a smaller scope, or the components that were skipped one at a time", b.used)
		return errResyncCapped
	}
	if b.used >= b.max {
		b.reason = fmt.Sprintf("the limit of %d Salesforce calls per request was reached; the components that were skipped need more calls than that", b.max)
		return errResyncCapped
	}
	b.used++
	return nil
}

// resyncMemo caches one fetch for the life of a request, so components that
// share a list (projects, opportunities, links) read it once.
type resyncMemo[T any] struct {
	done bool
	val  T
	err  error
}

func (m *resyncMemo[T]) get(fn func() (T, error)) (T, error) {
	if !m.done {
		m.val, m.err = fn()
		m.done = true
	}
	return m.val, m.err
}

// membershipData is a project's memberships with the contacts they point at.
type membershipData struct {
	items    []salesentity.ProjectContact
	contacts map[string]salesentity.Contact // by 18-character contact sf id
}

// resyncRun is the state of one preview or apply request.
type resyncRun struct {
	s      *salesforceResyncService
	scope  string
	target domain.SalesforceResyncTarget
	budget *callBudget

	account     resyncMemo[salesentity.Customer]
	projects    resyncMemo[[]salesentity.Project]
	links       resyncMemo[[]salesentity.LinkedOpportunity]
	opps        resyncMemo[[]salesentity.Opportunity]
	invoices    resyncMemo[[]salesentity.Invoice]
	contacts    resyncMemo[[]salesentity.Contact]
	memberships resyncMemo[membershipData]
}

func (s *salesforceResyncService) newRun(ctx context.Context, scope, id string) (*resyncRun, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, &apierror.ValidationError{Msg: "id is required"}
	}
	var (
		t   repository.SalesforceResyncTarget
		err error
	)
	switch scope {
	case domain.SalesforceResyncScopeAccount:
		t, err = s.reads.TargetAccount(ctx, id)
	case domain.SalesforceResyncScopeProject:
		t, err = s.reads.TargetProject(ctx, id)
	default:
		return nil, &apierror.ValidationError{Msg: `scope must be "account" or "project"`}
	}
	if err != nil {
		return nil, err
	}
	return &resyncRun{
		s: s, scope: scope, budget: &callBudget{max: resyncMaxSalesforceCalls},
		target: domain.SalesforceResyncTarget{Scope: scope, ID: id, SfID: t.SfID, Name: t.Name},
	}, nil
}

// resyncPaged reads every page of a keyset-paginated list search, spending one
// call per page. A page shorter than pageSize is the last.
func resyncPaged[T any](ctx context.Context, b *callBudget, pageSize int, base salesentity.SearchFilter,
	fetch func(context.Context, salesentity.SearchFilter) ([]T, error), cursor func(T) (lastModified *string, id string)) ([]T, error) {
	f := base
	f.Limit = pageSize
	f.MinLastModifiedDate = resyncKeysetEpoch
	var all []T
	for {
		if err := b.spend(ctx); err != nil {
			return nil, err
		}
		rows, err := fetch(ctx, f)
		if err != nil {
			return nil, err
		}
		all = append(all, rows...)
		if len(rows) < pageSize {
			return all, nil
		}
		lm, id := cursor(rows[len(rows)-1])
		if strings.TrimSpace(derefString(lm)) == "" || strings.TrimSpace(id) == "" {
			return nil, errResyncNoCursor
		}
		if f.MinLastModifiedDate == *lm && f.MinID == id {
			return nil, errResyncNoCursor
		}
		f.MinLastModifiedDate, f.MinID = *lm, id
	}
}

// ---- Salesforce reads (memoised per request) ----

// projectSfIDs are the Salesforce ids of the projects in scope: the account's
// projects, or the one project.
func (r *resyncRun) projectSfIDs(ctx context.Context) ([]string, error) {
	if r.scope == domain.SalesforceResyncScopeProject {
		return []string{r.target.SfID}, nil
	}
	ps, err := r.sfProjects(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(ps))
	for _, p := range ps {
		if id := strings.TrimSpace(p.ID); id != "" {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

func (r *resyncRun) sfAccount(ctx context.Context) (salesentity.Customer, error) {
	return r.account.get(func() (salesentity.Customer, error) {
		if err := r.budget.spend(ctx); err != nil {
			return salesentity.Customer{}, err
		}
		return r.s.ev.se.GetCustomer(ctx, r.target.SfID)
	})
}

// sfProjects is the account's projects (account scope) or the one project
// (project scope, a single GetProject call).
func (r *resyncRun) sfProjects(ctx context.Context) ([]salesentity.Project, error) {
	return r.projects.get(func() ([]salesentity.Project, error) {
		if r.scope == domain.SalesforceResyncScopeProject {
			if err := r.budget.spend(ctx); err != nil {
				return nil, err
			}
			p, err := r.s.ev.project.SalesEntity.GetProject(ctx, r.target.SfID)
			if err != nil {
				return nil, err
			}
			return []salesentity.Project{p}, nil
		}
		return resyncPaged(ctx, r.budget, resyncListPageSize, salesentity.SearchFilter{CustomerID: r.target.SfID},
			r.s.search.SearchProjects, func(p salesentity.Project) (*string, string) { return p.LastModifiedDate, p.ID })
	})
}

// sfLinks is the links of every project in scope: one search per project.
func (r *resyncRun) sfLinks(ctx context.Context) ([]salesentity.LinkedOpportunity, error) {
	return r.links.get(func() ([]salesentity.LinkedOpportunity, error) {
		ids, err := r.projectSfIDs(ctx)
		if err != nil {
			return nil, err
		}
		var all []salesentity.LinkedOpportunity
		for _, id := range ids {
			ls, err := resyncPaged(ctx, r.budget, resyncListPageSize, salesentity.SearchFilter{ProjectID: id},
				r.s.search.SearchLinkedOpportunities, func(l salesentity.LinkedOpportunity) (*string, string) { return l.LastModifiedDate, l.ID })
			if err != nil {
				return nil, err
			}
			all = append(all, ls...)
		}
		return all, nil
	})
}

// sfOpportunities are the account's opportunities (account scope), or the
// ones the project's links name (project scope, in batches of ids).
func (r *resyncRun) sfOpportunities(ctx context.Context) ([]salesentity.Opportunity, error) {
	return r.opps.get(func() ([]salesentity.Opportunity, error) {
		if r.scope == domain.SalesforceResyncScopeAccount {
			return resyncPaged(ctx, r.budget, resyncOpportunityPageSize, salesentity.SearchFilter{CustomerID: r.target.SfID},
				r.s.search.SearchOpportunities, func(o salesentity.Opportunity) (*string, string) { return o.LastModifiedDate, o.ID })
		}
		links, err := r.sfLinks(ctx)
		if err != nil {
			return nil, err
		}
		var ids []string
		seen := map[string]bool{}
		for _, l := range links {
			id := strings.TrimSpace(derefString(l.OpportunityID))
			if id != "" && !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
		var all []salesentity.Opportunity
		for len(ids) > 0 {
			n := min(len(ids), resyncOpportunityPageSize)
			if err := r.budget.spend(ctx); err != nil {
				return nil, err
			}
			page, err := r.s.search.SearchOpportunities(ctx, salesentity.SearchFilter{IDs: ids[:n], Limit: resyncOpportunityPageSize})
			if err != nil {
				return nil, err
			}
			all = append(all, page...)
			ids = ids[n:]
		}
		return all, nil
	})
}

// sfInvoices are the invoices of every opportunity in scope: one search per
// opportunity (the search takes one opportunity id).
func (r *resyncRun) sfInvoices(ctx context.Context) ([]salesentity.Invoice, error) {
	return r.invoices.get(func() ([]salesentity.Invoice, error) {
		opps, err := r.sfOpportunities(ctx)
		if err != nil {
			return nil, err
		}
		var all []salesentity.Invoice
		for _, o := range opps {
			id := strings.TrimSpace(o.ID)
			if id == "" {
				continue
			}
			invs, err := resyncPaged(ctx, r.budget, resyncListPageSize, salesentity.SearchFilter{OpportunityID: id},
				r.s.search.SearchInvoices, func(i salesentity.Invoice) (*string, string) { return i.LastModifiedDate, derefString(i.ID) })
			if err != nil {
				return nil, err
			}
			all = append(all, invs...)
		}
		return all, nil
	})
}

// sfContacts are the account's contacts (account scope only).
func (r *resyncRun) sfContacts(ctx context.Context) ([]salesentity.Contact, error) {
	return r.contacts.get(func() ([]salesentity.Contact, error) {
		return resyncPaged(ctx, r.budget, resyncListPageSize, salesentity.SearchFilter{AccountID: r.target.SfID},
			r.s.search.SearchContacts, func(c salesentity.Contact) (*string, string) { return c.LastModifiedDate, derefString(c.ID) })
	})
}

// sfMemberships are the memberships of every project in scope (one search per
// project) and the contacts they point at (batch lookups by id).
func (r *resyncRun) sfMemberships(ctx context.Context) (membershipData, error) {
	return r.memberships.get(func() (membershipData, error) {
		ids, err := r.projectSfIDs(ctx)
		if err != nil {
			return membershipData{}, err
		}
		data := membershipData{contacts: map[string]salesentity.Contact{}}
		for _, id := range ids {
			pcs, err := resyncPaged(ctx, r.budget, resyncListPageSize, salesentity.SearchFilter{ProjectID: id},
				r.s.search.SearchProjectContacts, func(p salesentity.ProjectContact) (*string, string) { return p.LastModifiedDate, p.ID })
			if err != nil {
				return membershipData{}, err
			}
			data.items = append(data.items, pcs...)
		}
		var contactIDs []string
		seen := map[string]bool{}
		for _, pc := range data.items {
			if pc.Contact == nil {
				continue
			}
			if id := strings.TrimSpace(derefString(pc.Contact.ID)); id != "" && !seen[id] {
				seen[id] = true
				contactIDs = append(contactIDs, id)
			}
		}
		for len(contactIDs) > 0 {
			n := min(len(contactIDs), resyncIDBatchSize)
			if err := r.budget.spend(ctx); err != nil {
				return membershipData{}, err
			}
			page, err := r.s.search.SearchContacts(ctx, salesentity.SearchFilter{IDs: contactIDs[:n], Limit: resyncListPageSize})
			if err != nil {
				return membershipData{}, err
			}
			for _, c := range page {
				if id := strings.TrimSpace(derefString(c.ID)); id != "" {
					data.contacts[salesforceID18(id)] = c
				}
			}
			contactIDs = contactIDs[n:]
		}
		return data, nil
	})
}

// ---- Preview ----

// Preview implements SalesforceResyncService.
func (s *salesforceResyncService) Preview(ctx context.Context, req domain.SalesforceResyncRequest) (domain.SalesforceResyncPreview, error) {
	if err := s.requireAdmin(ctx); err != nil {
		return domain.SalesforceResyncPreview{}, err
	}
	defs, err := resyncComponentsOf(req.Scope)
	if err != nil {
		return domain.SalesforceResyncPreview{}, err
	}
	run, err := s.newRun(ctx, req.Scope, req.ID)
	if err != nil {
		return domain.SalesforceResyncPreview{}, err
	}
	release, err := s.lock(req.Scope, req.ID)
	if err != nil {
		return domain.SalesforceResyncPreview{}, err
	}
	defer release()

	out := domain.SalesforceResyncPreview{Target: run.target, Components: make([]domain.SalesforceResyncComponent, 0, len(defs))}
	for _, def := range defs {
		if reason := s.disabledReason(def.key); reason != "" {
			out.Components = append(out.Components, skippedResyncComponent(def.key, def.label, reason))
			continue
		}
		comp, err := run.preview(ctx, def)
		if err != nil {
			out.Components = append(out.Components, skippedResyncComponent(def.key, def.label, run.skipReason(ctx, err)))
			continue
		}
		out.Components = append(out.Components, comp)
	}
	if run.budget.reason != "" {
		reason := run.budget.reason
		out.Capped, out.CappedReason = true, &reason
	}
	return out, nil
}

// skipReason is the text a component's skippedReason carries for err; the
// detail of an unexpected error stays in the log.
func (r *resyncRun) skipReason(ctx context.Context, err error) string {
	switch {
	case errors.Is(err, errResyncCapped):
		return "not compared: " + r.budget.reason
	case errors.Is(err, errResyncNoCursor):
		return err.Error()
	}
	slog.ErrorContext(ctx, "salesforce resync: component could not be read", "scope", r.scope, "targetSfId", r.target.SfID, "err", err)
	return "could not be read: " + resyncSafeMessage(err)
}

// resyncSafeMessage is err's text when it is one of the service's own error
// types (whose messages are written for callers), else a fixed text.
func resyncSafeMessage(err error) string {
	var (
		unavailable *apierror.ServiceUnavailableError
		downstream  *apierror.DownstreamError
		unauth      *apierror.UnauthorizedError
		notFound    *apierror.NotFoundError
		invalid     *apierror.ValidationError
		conflict    *apierror.ConflictError
	)
	switch {
	case errors.As(err, &unavailable):
		return unavailable.Msg
	case errors.As(err, &downstream):
		return downstream.Msg
	case errors.As(err, &unauth):
		return unauth.Msg
	case errors.As(err, &notFound):
		return notFound.Msg
	case errors.As(err, &invalid):
		return invalid.Msg
	case errors.As(err, &conflict):
		return conflict.Msg
	case errors.Is(err, context.DeadlineExceeded):
		return "the request timed out"
	}
	return "see the entity-service log"
}

func (r *resyncRun) preview(ctx context.Context, def resyncComponentDef) (domain.SalesforceResyncComponent, error) {
	switch def.key {
	case domain.SalesforceResyncComponentAccount:
		return r.previewAccount(ctx, def)
	case domain.SalesforceResyncComponentPartners:
		return r.previewPartners(ctx, def)
	case domain.SalesforceResyncComponentProjects, domain.SalesforceResyncComponentProject:
		return r.previewProjects(ctx, def)
	case domain.SalesforceResyncComponentOpportunities:
		return r.previewOpportunities(ctx, def)
	case domain.SalesforceResyncComponentLinkedOpportunities:
		return r.previewLinks(ctx, def)
	case domain.SalesforceResyncComponentInvoices:
		return r.previewInvoices(ctx, def)
	case domain.SalesforceResyncComponentContacts:
		return r.previewContacts(ctx, def)
	case domain.SalesforceResyncComponentMemberships:
		return r.previewMemberships(ctx, def)
	}
	return domain.SalesforceResyncComponent{}, fmt.Errorf("unknown component %q", def.key)
}

// label is a record's display label: its name, else its Salesforce id.
func resyncLabel(name *string, sfID string) string {
	if n := strings.TrimSpace(derefString(name)); n != "" {
		return n
	}
	return sfID
}

// firstByKey keeps the first stored row per key (the rows are ranked, best
// first, as the ingest ranks duplicate sf_id copies); keys are 18-character
// Salesforce ids.
func firstByKey(rows []repository.SalesforceStoredRow) map[string]repository.SalesforceStoredRow {
	out := make(map[string]repository.SalesforceStoredRow, len(rows))
	for _, row := range rows {
		k := salesforceID18(row.Key)
		if k == "" {
			continue
		}
		if _, dup := out[k]; !dup {
			out[k] = row
		}
	}
	return out
}

func (r *resyncRun) previewAccount(ctx context.Context, def resyncComponentDef) (domain.SalesforceResyncComponent, error) {
	cust, err := r.sfAccount(ctx)
	if err != nil {
		return domain.SalesforceResyncComponent{}, err
	}
	sfID := strings.TrimSpace(cust.ID)
	if sfID == "" {
		sfID = r.target.SfID
	}
	rows, err := r.s.reads.Accounts(ctx, []string{sfID, r.target.SfID})
	if err != nil {
		return domain.SalesforceResyncComponent{}, err
	}
	b := newResyncComponent(def.key, def.label)
	label := resyncLabel(cust.Name, sfID)
	if st, ok := firstByKey(rows)[salesforceID18(sfID)]; ok {
		b.compare(sfID, label, accountDiffFields(ctx, cust, st.Cols))
	} else {
		b.missingInCSM(sfID, label)
	}
	return b.finish(), nil
}

func (r *resyncRun) previewPartners(ctx context.Context, def resyncComponentDef) (domain.SalesforceResyncComponent, error) {
	if err := r.budget.spend(ctx); err != nil {
		return domain.SalesforceResyncComponent{}, err
	}
	_, partners, err := r.s.ev.partners.SalesEntity.GetCustomerPartners(ctx, r.target.SfID)
	if err != nil {
		return domain.SalesforceResyncComponent{}, err
	}
	rows, err := r.s.reads.Partners(ctx, r.target.SfID)
	if err != nil {
		return domain.SalesforceResyncComponent{}, err
	}
	stored := firstByKey(rows)
	b := newResyncComponent(def.key, def.label)
	self := salesforceID18(r.target.SfID)
	seen := map[string]bool{}
	for _, p := range partners {
		id := salesforceID18(p.ID)
		if id == "" || id == self || seen[id] {
			continue
		}
		seen[id] = true
		if _, ok := stored[id]; ok {
			b.inSync()
		} else {
			b.missingInCSM(id, resyncLabel(p.Name, id))
		}
	}
	for id, row := range stored {
		if !seen[id] {
			b.onlyInCSM(id, resyncLabel(row.Cols["name"], id))
		}
	}
	return b.finish(), nil
}

// projectTypeName is the project type name the ingest would store for p: the
// Salesforce type when a project_type row has that name, else nil.
func (r *resyncRun) projectTypeName(ctx context.Context, p salesentity.Project, cache map[string]*string) (*string, error) {
	name := strings.TrimSpace(derefString(p.Type))
	if name == "" {
		return nil, nil
	}
	if v, ok := cache[name]; ok {
		return v, nil
	}
	id, err := r.s.ev.project.Projects.LookupProjectTypeIDByName(ctx, name)
	if err != nil {
		return nil, err
	}
	var v *string
	if id != nil {
		v = &name
	}
	cache[name] = v
	return v, nil
}

func (r *resyncRun) previewProjects(ctx context.Context, def resyncComponentDef) (domain.SalesforceResyncComponent, error) {
	sf, err := r.sfProjects(ctx)
	if err != nil {
		return domain.SalesforceResyncComponent{}, err
	}
	var sfIDs, keys []string
	for _, p := range sf {
		sfIDs = append(sfIDs, strings.TrimSpace(p.ID))
		if k := strings.TrimSpace(derefString(p.Key)); k != "" {
			keys = append(keys, k)
		}
	}
	accountScope := ""
	if r.scope == domain.SalesforceResyncScopeAccount {
		accountScope = r.target.SfID
	}
	rows, err := r.s.reads.Projects(ctx, sfIDs, keys, accountScope)
	if err != nil {
		return domain.SalesforceResyncComponent{}, err
	}
	bySf := firstByKey(rows)
	byKey := map[string]repository.SalesforceStoredRow{}
	for _, row := range rows {
		if k := derefString(row.Cols["key"]); k != "" {
			if _, dup := byKey[k]; !dup {
				byKey[k] = row
			}
		}
	}

	b := newResyncComponent(def.key, def.label)
	typeCache := map[string]*string{}
	matched := map[string]bool{}
	for _, p := range sf {
		sfID := strings.TrimSpace(p.ID)
		label := resyncLabel(p.Name, sfID)
		key := strings.TrimSpace(derefString(p.Key))
		st, ok := bySf[salesforceID18(sfID)]
		if !ok && key != "" {
			// The ingest also adopts the row with this key (a row whose
			// sf_id is empty or different) and stamps its sf_id.
			st, ok = byKey[key]
		}
		if !ok {
			b.missingInCSM(sfID, label)
			continue
		}
		matched[st.Key+"|"+derefString(st.Cols["key"])] = true
		if key == "" {
			// The ingest refuses a project with no Project_Key__c.
			b.differs(sfID, label, []domain.SalesforceResyncFieldDiff{{Field: "key", CsmValue: st.Cols["key"], SfValue: nil}})
			continue
		}
		written, err := r.projectTypeName(ctx, p, typeCache)
		if err != nil {
			return domain.SalesforceResyncComponent{}, err
		}
		b.compare(sfID, label, projectDiffFields(ctx, p, written, st.Cols))
	}
	if accountScope != "" {
		for _, row := range rows {
			if !matched[row.Key+"|"+derefString(row.Cols["key"])] && sameSfIDPtr(row.Cols["account_sf_id"], accountScope) {
				b.onlyInCSM(row.Key, resyncLabel(row.Cols["name"], row.Key))
			}
		}
	}
	return b.finish(), nil
}

func sameSfIDPtr(stored *string, sfID string) bool {
	return stored != nil && salesforceID18(strings.TrimSpace(*stored)) == salesforceID18(strings.TrimSpace(sfID))
}

func (r *resyncRun) previewOpportunities(ctx context.Context, def resyncComponentDef) (domain.SalesforceResyncComponent, error) {
	sf, err := r.sfOpportunities(ctx)
	if err != nil {
		return domain.SalesforceResyncComponent{}, err
	}
	ids := make([]string, 0, len(sf))
	for _, o := range sf {
		ids = append(ids, strings.TrimSpace(o.ID))
	}
	accountScope, projectScope := "", ""
	if r.scope == domain.SalesforceResyncScopeAccount {
		accountScope = r.target.SfID
	} else {
		projectScope = r.target.SfID
	}
	rows, err := r.s.reads.Opportunities(ctx, ids, accountScope, projectScope)
	if err != nil {
		return domain.SalesforceResyncComponent{}, err
	}
	stored := firstByKey(rows)
	storedIDs := make([]string, 0, len(stored))
	for _, row := range stored {
		storedIDs = append(storedIDs, row.Key)
	}
	itemRows, err := r.s.reads.LineItems(ctx, storedIDs)
	if err != nil {
		return domain.SalesforceResyncComponent{}, err
	}
	items := map[string]map[string]map[string]*string{} // opportunity sf id -> line item sf id -> columns
	for _, row := range itemRows {
		opp := salesforceID18(derefString(row.Cols["opportunity_sf_id"]))
		if items[opp] == nil {
			items[opp] = map[string]map[string]*string{}
		}
		if _, dup := items[opp][row.Key]; !dup {
			items[opp][row.Key] = row.Cols
		}
	}

	b := newResyncComponent(def.key, def.label)
	inSF := map[string]bool{}
	for _, o := range sf {
		sfID := salesforceID18(strings.TrimSpace(o.ID))
		inSF[sfID] = true
		label := resyncLabel(o.Name, sfID)
		st, ok := stored[sfID]
		if !ok {
			b.missingInCSM(sfID, label)
			continue
		}
		b.compare(sfID, label, opportunityDiffFields(ctx, o, st.Cols, items[sfID]))
	}
	for id, row := range stored {
		if inSF[id] {
			continue
		}
		// An account-scope row counts only when it is the account's own.
		if accountScope != "" && !sameSfIDPtr(row.Cols["account_sf_id"], accountScope) {
			continue
		}
		b.onlyInCSM(id, resyncLabel(row.Cols["name"], id))
	}
	return b.finish(), nil
}

func (r *resyncRun) previewLinks(ctx context.Context, def resyncComponentDef) (domain.SalesforceResyncComponent, error) {
	sf, err := r.sfLinks(ctx)
	if err != nil {
		return domain.SalesforceResyncComponent{}, err
	}
	projects, err := r.projectSfIDs(ctx)
	if err != nil {
		return domain.SalesforceResyncComponent{}, err
	}
	ids := make([]string, 0, len(sf))
	for _, l := range sf {
		ids = append(ids, strings.TrimSpace(l.ID))
	}
	rows, err := r.s.reads.Links(ctx, ids, projects)
	if err != nil {
		return domain.SalesforceResyncComponent{}, err
	}
	stored := firstByKey(rows)
	b := newResyncComponent(def.key, def.label)
	inSF := map[string]bool{}
	for _, l := range sf {
		sfID := salesforceID18(strings.TrimSpace(l.ID))
		inSF[sfID] = true
		label := resyncLabel(l.Name, sfID)
		st, ok := stored[sfID]
		if !ok {
			b.missingInCSM(sfID, label)
			continue
		}
		b.compare(sfID, label, linkDiffFields(ctx, l, st.Cols))
	}
	inScope := map[string]bool{}
	for _, p := range projects {
		inScope[salesforceID18(p)] = true
	}
	for id, row := range stored {
		if !inSF[id] && row.Cols["project_sf_id"] != nil && inScope[salesforceID18(*row.Cols["project_sf_id"])] {
			b.onlyInCSM(id, resyncLabel(row.Cols["number"], id))
		}
	}
	return b.finish(), nil
}

func (r *resyncRun) previewInvoices(ctx context.Context, def resyncComponentDef) (domain.SalesforceResyncComponent, error) {
	sf, err := r.sfInvoices(ctx)
	if err != nil {
		return domain.SalesforceResyncComponent{}, err
	}
	opps, err := r.sfOpportunities(ctx)
	if err != nil {
		return domain.SalesforceResyncComponent{}, err
	}
	oppIDs := make([]string, 0, len(opps))
	inScope := map[string]bool{}
	for _, o := range opps {
		oppIDs = append(oppIDs, strings.TrimSpace(o.ID))
		inScope[salesforceID18(strings.TrimSpace(o.ID))] = true
	}
	ids := make([]string, 0, len(sf))
	for _, i := range sf {
		ids = append(ids, strings.TrimSpace(derefString(i.ID)))
	}
	rows, err := r.s.reads.Invoices(ctx, ids, oppIDs)
	if err != nil {
		return domain.SalesforceResyncComponent{}, err
	}
	stored := firstByKey(rows)
	b := newResyncComponent(def.key, def.label)
	inSF := map[string]bool{}
	for _, inv := range sf {
		sfID := salesforceID18(strings.TrimSpace(derefString(inv.ID)))
		if sfID == "" {
			continue
		}
		inSF[sfID] = true
		label := resyncLabel(inv.Name, sfID)
		st, ok := stored[sfID]
		if !ok {
			b.missingInCSM(sfID, label)
			continue
		}
		b.compare(sfID, label, invoiceDiffFields(ctx, inv, st.Cols))
	}
	for id, row := range stored {
		if !inSF[id] && row.Cols["opportunity_sf_id"] != nil && inScope[salesforceID18(*row.Cols["opportunity_sf_id"])] {
			b.onlyInCSM(id, resyncLabel(row.Cols["name"], id))
		}
	}
	return b.finish(), nil
}

func (r *resyncRun) previewContacts(ctx context.Context, def resyncComponentDef) (domain.SalesforceResyncComponent, error) {
	sf, err := r.sfContacts(ctx)
	if err != nil {
		return domain.SalesforceResyncComponent{}, err
	}
	var ids, emails []string
	for _, c := range sf {
		ids = append(ids, strings.TrimSpace(derefString(c.ID)))
		if e := strings.ToLower(strings.TrimSpace(derefString(c.Email))); e != "" {
			emails = append(emails, e)
		}
	}
	userRows, err := r.s.reads.ContactUsers(ctx, ids, emails)
	if err != nil {
		return domain.SalesforceResyncComponent{}, err
	}
	acRows, err := r.s.reads.AccountContacts(ctx, ids, r.target.SfID)
	if err != nil {
		return domain.SalesforceResyncComponent{}, err
	}
	usersBySf := firstByKey(userRows)
	usersByEmail := map[string]repository.SalesforceStoredRow{}
	for _, row := range userRows {
		if e := derefString(row.Cols["email"]); e != "" {
			if _, dup := usersByEmail[e]; !dup {
				usersByEmail[e] = row
			}
		}
	}
	acs := firstByKey(acRows)

	b := newResyncComponent(def.key, def.label)
	inSF := map[string]bool{}
	for _, c := range sf {
		sfID := strings.TrimSpace(derefString(c.ID))
		if sfID == "" {
			continue
		}
		id18 := salesforceID18(sfID)
		inSF[id18] = true
		accountSfID := contactAccountSfID(c)
		label := resyncLabel(c.Email, sfID)
		if accountSfID == "" {
			// The Contact writer does not write a contact with no account.
			b.inSync()
			continue
		}
		in, err := buildContactUpsert(c, sfID, "", accountSfID)
		if err != nil {
			b.differs(sfID, label, []domain.SalesforceResyncFieldDiff{{Field: "email", CsmValue: nil, SfValue: nil}})
			continue
		}
		user, haveUser := usersBySf[id18]
		if !haveUser {
			user, haveUser = usersByEmail[in.Email]
		}
		ac, haveAC := acs[id18]
		if !haveUser && !haveAC {
			b.missingInCSM(sfID, label)
			continue
		}
		var uc, ac2 map[string]*string
		if haveUser {
			uc = user.Cols
		}
		if haveAC {
			ac2 = ac.Cols
		}
		b.compare(sfID, label, contactDiffFields(in, uc, ac2))
	}
	for id, row := range acs {
		if !inSF[id] && sameSfIDPtr(row.Cols["account_sf_id"], r.target.SfID) {
			b.onlyInCSM(id, resyncLabel(row.Cols["user_name"], id))
		}
	}
	return b.finish(), nil
}

func (r *resyncRun) previewMemberships(ctx context.Context, def resyncComponentDef) (domain.SalesforceResyncComponent, error) {
	data, err := r.sfMemberships(ctx)
	if err != nil {
		return domain.SalesforceResyncComponent{}, err
	}
	projects, err := r.projectSfIDs(ctx)
	if err != nil {
		return domain.SalesforceResyncComponent{}, err
	}
	ids := make([]string, 0, len(data.items))
	for _, pc := range data.items {
		ids = append(ids, strings.TrimSpace(pc.ID))
	}
	rows, err := r.s.reads.Memberships(ctx, ids, projects)
	if err != nil {
		return domain.SalesforceResyncComponent{}, err
	}
	stored := firstByKey(rows)
	storedIDs := make([]string, 0, len(stored))
	for _, row := range stored {
		storedIDs = append(storedIDs, row.Key)
	}
	groups, err := r.s.reads.MembershipGroups(ctx, storedIDs)
	if err != nil {
		return domain.SalesforceResyncComponent{}, err
	}
	groupsByID := map[string][]string{}
	for sf, g := range groups {
		groupsByID[salesforceID18(sf)] = g
	}

	b := newResyncComponent(def.key, def.label)
	inSF := map[string]bool{}
	for _, pc := range data.items {
		sfID := strings.TrimSpace(pc.ID)
		id18 := salesforceID18(sfID)
		inSF[id18] = true
		label := resyncLabel(&pc.Email, sfID)
		contact, ok := membershipContact(data, pc)
		if !ok {
			b.differs(sfID, label, []domain.SalesforceResyncFieldDiff{{Field: "contact", CsmValue: nil, SfValue: nil}})
			continue
		}
		in, _, err := buildMembershipUpsert(pc, contact)
		if err != nil {
			b.differs(sfID, label, []domain.SalesforceResyncFieldDiff{{Field: "state", CsmValue: nil, SfValue: pc.State}})
			continue
		}
		st, ok := stored[id18]
		if !ok {
			b.missingInCSM(sfID, label)
			continue
		}
		b.compare(sfID, label, membershipDiffFields(in, st.Cols, groupsByID[id18]))
	}
	inScope := map[string]bool{}
	for _, p := range projects {
		inScope[salesforceID18(p)] = true
	}
	for id, row := range stored {
		if !inSF[id] && row.Cols["project_sf_id"] != nil && inScope[salesforceID18(*row.Cols["project_sf_id"])] {
			b.onlyInCSM(id, resyncLabel(row.Cols["email"], id))
		}
	}
	return b.finish(), nil
}

// membershipContact is the Contact a membership points at, from the batch
// lookup; ok is false when the membership names none or Salesforce did not
// return it (the ingest then reads it itself, which the resync does not).
func membershipContact(data membershipData, pc salesentity.ProjectContact) (salesentity.Contact, bool) {
	if pc.Contact == nil {
		return salesentity.Contact{}, false
	}
	c, ok := data.contacts[salesforceID18(strings.TrimSpace(derefString(pc.Contact.ID)))]
	return c, ok
}
