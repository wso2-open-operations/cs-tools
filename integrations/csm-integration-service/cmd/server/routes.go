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

package main

import (
	"net/http"

	"github.com/wso2-open-operations/cs-tools/integrations/csm-integration-service/internal/entity"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-integration-service/internal/handler"
	"github.com/wso2-open-operations/cs-tools/integrations/csm-integration-service/internal/middleware"
)

// Operation scopes. Every route except GET /health requires exactly one of
// these on the caller's token. The names are the contract published in
// openapi.yaml's security scheme: a consumer is granted the scopes for the
// operation groups it needs and nothing else, so adding a route here means
// choosing (or introducing) its scope in both places.
const (
	scopeAccountsRead               = "accounts:read"
	scopeContactsRead               = "contacts:read"
	scopeProjectsRead               = "projects:read"
	scopeProjectsWrite              = "projects:write"
	scopeCasesRead                  = "cases:read"
	scopeCasesWrite                 = "cases:write"
	scopeOpportunitiesRead          = "opportunities:read"
	scopeInvoicesRead               = "invoices:read"
	scopeVulnerabilitiesSync        = "vulnerabilities:sync"
	scopeIncidentsRead              = "incidents:read"
	scopeIncidentsWrite             = "incidents:write"
	scopeServicesRead               = "services:read"
	scopeAlertIncidentMappingsRead  = "alert-incident-mappings:read"
	scopeAlertIncidentMappingsWrite = "alert-incident-mappings:write"
	scopeCloudStatusRead            = "cloud-status:read"
)

// handlers groups the resource handlers the route table is built from.
type handlers struct {
	health                 http.Handler
	account                *handler.AccountHandler
	project                *handler.ProjectHandler
	vulnerability          *handler.VulnerabilityHandler
	cases                  *handler.CaseHandler
	opportunity            *handler.OpportunityHandler
	invoice                *handler.InvoiceHandler
	projectOpportunityLink *handler.ProjectOpportunityLinkHandler
	incident               *handler.IncidentHandler
	itService              *handler.ITServiceHandler
	alertIncidentMapping   *handler.AlertIncidentMappingHandler
	cloudStatus            *handler.CloudStatusHandler
}

// newHandlers builds every resource handler on top of one entity client.
// actorEmail is the service's configured trusted actor for case writes; health
// is the handler mounted at GET /health.
func newHandlers(client *entity.Client, actorEmail string, health http.Handler) handlers {
	return handlers{
		health:                 health,
		account:                handler.NewAccountHandler(client),
		project:                handler.NewProjectHandler(client),
		vulnerability:          handler.NewVulnerabilityHandler(client),
		cases:                  handler.NewCaseHandler(client, actorEmail),
		opportunity:            handler.NewOpportunityHandler(client),
		invoice:                handler.NewInvoiceHandler(client),
		projectOpportunityLink: handler.NewProjectOpportunityLinkHandler(client),
		incident:               handler.NewIncidentHandler(client),
		itService:              handler.NewITServiceHandler(client),
		alertIncidentMapping:   handler.NewAlertIncidentMappingHandler(client),
		cloudStatus:            handler.NewCloudStatusHandler(client),
	}
}

// route is one scope-guarded operation: a Go 1.22 method-prefixed mux pattern,
// the scope it requires, and the handler that serves it.
type route struct {
	pattern string
	scope   string
	handler http.HandlerFunc
}

// routes is the full table of scope-guarded operations. GET /health is not in
// it: the health probe is deliberately unauthenticated and is mounted
// separately by newMux.
func routes(h handlers) []route {
	return []route{
		{"GET /accounts/{id}", scopeAccountsRead, h.account.GetAccount},
		{"POST /accounts/search", scopeAccountsRead, h.account.SearchAccounts},
		{"POST /accounts/{id}/contacts/search", scopeContactsRead, h.account.SearchAccountContacts},
		{"GET /projects/{id}", scopeProjectsRead, h.project.GetProject},
		{"POST /projects/search", scopeProjectsRead, h.project.SearchProjects},
		{"POST /projects/{id}/contacts/search", scopeContactsRead, h.project.SearchProjectContacts},
		{"PATCH /projects/{id}", scopeProjectsWrite, h.project.UpdateProject},
		{"POST /vulnerabilities/sync", scopeVulnerabilitiesSync, h.vulnerability.SyncProductVulnerabilities},
		{"POST /cases/search", scopeCasesRead, h.cases.SearchCases},
		{"PATCH /cases/{id}", scopeCasesWrite, h.cases.PatchCase},
		{"POST /cases/{id}/comments", scopeCasesWrite, h.cases.CreateCaseComment},
		{"POST /cases/{id}/tags", scopeCasesWrite, h.cases.AddCaseTag},
		{"POST /opportunities/search", scopeOpportunitiesRead, h.opportunity.SearchOpportunities},
		{"GET /opportunities/{id}", scopeOpportunitiesRead, h.opportunity.GetOpportunity},
		{"POST /invoices/search", scopeInvoicesRead, h.invoice.SearchInvoices},
		{"GET /invoices/{id}", scopeInvoicesRead, h.invoice.GetInvoice},
		{"POST /project-opportunity-links/search", scopeOpportunitiesRead, h.projectOpportunityLink.SearchProjectOpportunityLinks},
		{"POST /incidents", scopeIncidentsWrite, h.incident.CreateIncident},
		{"PATCH /incidents/{id}", scopeIncidentsWrite, h.incident.PatchIncident},
		{"POST /incidents/search", scopeIncidentsRead, h.incident.SearchIncidents},
		{"POST /services/search", scopeServicesRead, h.itService.SearchITServices},
		{"POST /alert-incident-mappings", scopeAlertIncidentMappingsWrite, h.alertIncidentMapping.CreateAlertIncidentMapping},
		{"POST /alert-incident-mappings/lookup", scopeAlertIncidentMappingsRead, h.alertIncidentMapping.LookupAlertIncidentMappings},
		{"GET /cloud-status/monitors", scopeCloudStatusRead, h.cloudStatus.GetMonitors},
		{"GET /cloud-status/incidents", scopeCloudStatusRead, h.cloudStatus.GetIncidents},
		{"GET /cloud-status/availabilities", scopeCloudStatusRead, h.cloudStatus.GetAvailabilities},
		{"GET /cloud-status/availability-history", scopeCloudStatusRead, h.cloudStatus.GetAvailabilityHistory},
		{"GET /cloud-status/incidents/{id}", scopeCloudStatusRead, h.cloudStatus.GetIncidentDetail},
	}
}

// newMux mounts the unauthenticated health probe and every scope-guarded route.
func newMux(h handlers, guard *middleware.ScopeGuard) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("GET /health", h.health)
	for _, rt := range routes(h) {
		mux.Handle(rt.pattern, guard.Require(rt.scope, rt.handler))
	}
	return mux
}
