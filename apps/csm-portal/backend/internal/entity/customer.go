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
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// CreateCase calls POST /cases on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) CreateCase(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/cases", body)
}

// SearchCases calls POST /cases/search on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) SearchCases(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/cases/search", body)
}

// SearchAnnouncementRegistryCases calls POST /announcements/registry/cases on
// the entity service: every announcement case matching the filters in one
// response (the registry groups the whole set, so paging /cases/search 50 rows
// at a time cost about 100 slow requests). Entity services that predate the
// route answer 404; callers fall back to paging /cases/search.
func (c *CustomerEntityClient) SearchAnnouncementRegistryCases(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/announcements/registry/cases", body)
}

// AggregateCases calls POST /cases/aggregate on the entity service: a
// server-side aggregation of cases by a single field (e.g. account, state),
// capped to the top maxGroups buckets with the remainder folded into
// othersCount. Response is returned as raw JSON; typed response structs are
// deferred.
func (c *CustomerEntityClient) AggregateCases(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/cases/aggregate", body)
}

// SearchFeedback calls POST /cases/feedback/search on the entity service:
// search of case-feedback (satisfaction rating) records across cases,
// filterable by case, accounts, and submission date range. Response is
// returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) SearchFeedback(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/cases/feedback/search", body)
}

// AggregateFeedback calls POST /cases/feedback/aggregate on the entity
// service: date-bucketed rating aggregation of case-feedback records across
// cases. Response is returned as raw JSON; typed response structs are
// deferred.
func (c *CustomerEntityClient) AggregateFeedback(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/cases/feedback/aggregate", body)
}

// GetCase calls GET /cases/{id} on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) GetCase(ctx context.Context, caseID string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/cases/%s", url.PathEscape(caseID)), nil)
}

// GetProductRepoMapping calls GET /products/github-repo?name= on the entity service.
func (c *CustomerEntityClient) GetProductRepoMapping(ctx context.Context, name string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, "/products/github-repo?name="+url.QueryEscape(name), nil)
}

// PatchCase calls PATCH /cases/{id} on the entity service to update case state.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) PatchCase(ctx context.Context, caseID string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, fmt.Sprintf("/cases/%s", url.PathEscape(caseID)), body)
}

// CreateCaseComment calls POST /cases/{id}/comments on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) CreateCaseComment(ctx context.Context, caseID string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/cases/%s/comments", url.PathEscape(caseID)), body)
}

// SearchCaseComments calls POST /cases/{id}/comments/search on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) SearchCaseComments(ctx context.Context, caseID string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/cases/%s/comments/search", url.PathEscape(caseID)), body)
}

// SearchCaseEscalations calls GET /cases/{id}/escalations on the entity service to fetch
// a case's escalation history. Response is returned as raw JSON; typed response structs
// are deferred.
func (c *CustomerEntityClient) SearchCaseEscalations(ctx context.Context, caseID string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/cases/%s/escalations", url.PathEscape(caseID)), nil)
}

// CreateCaseEscalation calls POST /cases/{id}/escalations on the entity service to
// escalate or de-escalate a case. The body is forwarded verbatim. Response is returned
// as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) CreateCaseEscalation(ctx context.Context, caseID string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/cases/%s/escalations", url.PathEscape(caseID)), body)
}

// GetTeamMembers calls GET /teams/{id}/members on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) GetTeamMembers(ctx context.Context, teamID string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/teams/%s/members", url.PathEscape(teamID)), nil)
}

// SearchTeams calls POST /teams/search on the entity service, which lists the
// `team` table. The body is forwarded verbatim and the response is returned as
// raw JSON.
func (c *CustomerEntityClient) SearchTeams(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/teams/search", body)
}

// SearchCaseActivities calls POST /cases/{id}/activities/search on the entity service.
// The path-scoped body is forwarded verbatim and the response is returned as raw JSON;
// typed response structs are deferred.
func (c *CustomerEntityClient) SearchCaseActivities(ctx context.Context, caseID string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/cases/"+url.PathEscape(caseID)+"/activities/search", body)
}

// SearchTasks calls POST /tasks/search on the entity service (standalone task
// search, not scoped to a parent case). Response is returned as raw JSON;
// typed response structs are deferred.
func (c *CustomerEntityClient) SearchTasks(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/tasks/search", body)
}

// GetUserMe calls GET /users/me on the entity service.
// Response is returned as raw JSON.
func (c *CustomerEntityClient) GetUserMe(ctx context.Context) ([]byte, error) {
	return c.do(ctx, http.MethodGet, "/users/me", nil)
}

// PatchUserMe calls PATCH /users/me on the entity service.
// Response is returned as raw JSON.
func (c *CustomerEntityClient) PatchUserMe(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, "/users/me", body)
}

// ListSavedFilterViews calls GET /users/me/saved-filter-views on the entity service.
func (c *CustomerEntityClient) ListSavedFilterViews(ctx context.Context, listKey string) ([]byte, error) {
	q := url.Values{}
	q.Set("listKey", listKey)
	return c.do(ctx, http.MethodGet, "/users/me/saved-filter-views?"+q.Encode(), nil)
}

// SaveSavedFilterView calls PATCH /users/me/saved-filter-views on the entity service.
func (c *CustomerEntityClient) SaveSavedFilterView(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, "/users/me/saved-filter-views", body)
}

// DeleteSavedFilterView calls DELETE /users/me/saved-filter-views on the entity service.
func (c *CustomerEntityClient) DeleteSavedFilterView(ctx context.Context, listKey, name string) ([]byte, error) {
	q := url.Values{}
	q.Set("listKey", listKey)
	q.Set("name", name)
	return c.do(ctx, http.MethodDelete, "/users/me/saved-filter-views?"+q.Encode(), nil)
}

// ReorderSavedFilterView calls POST /users/me/saved-filter-views/reorder on the entity service.
func (c *CustomerEntityClient) ReorderSavedFilterView(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/users/me/saved-filter-views/reorder", body)
}

// SearchUsers calls POST /users/search on the entity service.
// Response is returned as raw JSON; field filtering to the portal shape is deferred.
func (c *CustomerEntityClient) SearchUsers(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/users/search", body)
}

// CreateUser calls POST /users on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) CreateUser(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/users", body)
}

// GetProjectContact calls GET /projects/{id}/contacts/{contactId} on the entity service.
// Response is returned as raw JSON.
func (c *CustomerEntityClient) GetProjectContact(ctx context.Context, projectID, contactID string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/projects/%s/contacts/%s",
		url.PathEscape(projectID), url.PathEscape(contactID)), nil)
}

// ResendProjectContactInvitation calls
// POST /projects/{id}/contacts/{email}/resend-invitation on the entity
// service, sending a fresh invitation email to a project contact. No
// request body; entity service returns 204 No Content on success, so the
// []byte result is always empty -- callers care only about the error.
func (c *CustomerEntityClient) ResendProjectContactInvitation(ctx context.Context, projectID, email string) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/projects/%s/contacts/%s/resend-invitation",
		url.PathEscape(projectID), url.PathEscape(email)), nil)
}

// GetUser calls GET /users/{id} on the entity service.
// Response is returned as raw JSON.
func (c *CustomerEntityClient) GetUser(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/users/%s", url.PathEscape(id)), nil)
}

// GetAccount calls GET /accounts/{id} on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) GetAccount(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/accounts/%s", url.PathEscape(id)), nil)
}

// SearchAccounts calls POST /accounts/search on the entity service.
// Response is returned as raw JSON; field filtering to the portal shape is deferred.
func (c *CustomerEntityClient) SearchAccounts(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/accounts/search", body)
}

// SearchAccountContacts calls POST /accounts/{id}/contacts/search on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) SearchAccountContacts(ctx context.Context, accountID string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/accounts/%s/contacts/search", url.PathEscape(accountID)), body)
}

// UpdateAccountTeams calls PATCH /accounts/{id} on the entity service to update an
// account's CRE team and/or SRE team assignment.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) UpdateAccountTeams(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, fmt.Sprintf("/accounts/%s", url.PathEscape(id)), body)
}

// GetProject calls GET /projects/{id} on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) GetProject(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/projects/%s", url.PathEscape(id)), nil)
}

// GetProjectMetadata calls GET /projects/{id}/metadata on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) GetProjectMetadata(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/projects/%s/metadata", url.PathEscape(id)), nil)
}

// SearchProjects calls POST /projects/search on the entity service.
// Response is returned as raw JSON; field filtering to the portal shape is deferred.
func (c *CustomerEntityClient) SearchProjects(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/projects/search", body)
}

// SearchProjectContacts calls POST /projects/{id}/contacts/search on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) SearchProjectContacts(ctx context.Context, projectID string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/projects/%s/contacts/search", url.PathEscape(projectID)), body)
}

// UpdateProject calls PATCH /projects/{id} on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) UpdateProject(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, fmt.Sprintf("/projects/%s", url.PathEscape(id)), body)
}

// SearchProducts calls POST /products/search on the entity service.
// Response is returned as raw JSON; field filtering to the portal shape is deferred.
func (c *CustomerEntityClient) SearchProducts(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/products/search", body)
}

// SearchProductVersions calls POST /products/{id}/versions/search on the entity service.
// Response is returned as raw JSON; field filtering to the portal shape is deferred.
func (c *CustomerEntityClient) SearchProductVersions(ctx context.Context, productID string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/products/%s/versions/search", url.PathEscape(productID)), body)
}

// SearchIncidents calls POST /incidents/search on the entity service.
// Response is returned as raw JSON; field filtering to the portal shape is deferred.
func (c *CustomerEntityClient) SearchIncidents(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/incidents/search", body)
}

// AggregateIncidents calls POST /incidents/aggregate on the entity service: a
// server-side aggregation of incidents by a single field (e.g. state,
// assignmentGroup, businessService), capped to the top maxGroups buckets
// with the remainder folded into othersCount. Response is returned as raw
// JSON; typed response structs are deferred.
func (c *CustomerEntityClient) AggregateIncidents(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/incidents/aggregate", body)
}

// CreateIncident calls POST /incidents on the entity service to create a new incident.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) CreateIncident(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/incidents", body)
}

// GetIncident calls GET /incidents/{id} on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) GetIncident(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/incidents/%s", url.PathEscape(id)), nil)
}

// PatchIncident calls PATCH /incidents/{id} on the entity service to partially update an incident.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) PatchIncident(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, fmt.Sprintf("/incidents/%s", url.PathEscape(id)), body)
}

// SearchIncidentActivities calls POST /incidents/{id}/activities/search on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) SearchIncidentActivities(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/incidents/%s/activities/search", url.PathEscape(id)), body)
}

// ListSpecialistHandoffTeams calls GET /specialist-handoff-teams on the entity service:
// the Special Ops teams the incident handoff dialog offers for serviceID (every team when
// empty). Raw JSON.
func (c *CustomerEntityClient) ListSpecialistHandoffTeams(ctx context.Context, serviceID string) ([]byte, error) {
	path := "/specialist-handoff-teams"
	if serviceID != "" {
		path += "?" + url.Values{"serviceId": {serviceID}}.Encode()
	}
	return c.do(ctx, http.MethodGet, path, nil)
}

// GetIncidentCreateDefaults calls GET /incidents/create-defaults on the entity
// service: the default service and its support group. Raw JSON.
func (c *CustomerEntityClient) GetIncidentCreateDefaults(ctx context.Context) ([]byte, error) {
	return c.do(ctx, http.MethodGet, "/incidents/create-defaults", nil)
}

// HandOffIncidentToSpecialist calls POST /incidents/{id}/specialist-handoffs on the entity
// service: hands the incident off to its specialist group in one atomic call. Response is
// returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) HandOffIncidentToSpecialist(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/incidents/%s/specialist-handoffs", url.PathEscape(id)), body)
}

// SearchProblems calls POST /problems/search on the entity service.
// Response is returned as raw JSON; field filtering to the portal shape is deferred.
func (c *CustomerEntityClient) SearchProblems(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/problems/search", body)
}

// AggregateProblems calls POST /problems/aggregate on the entity service: a
// server-side aggregation of problems by a single field (e.g. state,
// assignmentGroup), capped to the top maxGroups buckets with the remainder
// folded into othersCount. Response is returned as raw JSON; typed response
// structs are deferred.
func (c *CustomerEntityClient) AggregateProblems(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/problems/aggregate", body)
}

// CreateProblem calls POST /problems on the entity service to create a new problem.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) CreateProblem(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/problems", body)
}

// GetProblem calls GET /problems/{id} on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) GetProblem(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/problems/%s", url.PathEscape(id)), nil)
}

// UpdateProblem calls PATCH /problems/{id} on the entity service to partially update a problem.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) UpdateProblem(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, fmt.Sprintf("/problems/%s", url.PathEscape(id)), body)
}

// SearchIncidentTasks calls POST /incident-tasks/search on the entity service.
// Response is returned as raw JSON; field filtering to the portal shape is deferred.
func (c *CustomerEntityClient) SearchIncidentTasks(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/incident-tasks/search", body)
}

// AggregateIncidentTasks calls POST /incident-tasks/aggregate on the entity
// service: a server-side aggregation of incident tasks by a single field
// (e.g. state, assignmentGroup), capped to the top maxGroups buckets with
// the remainder folded into othersCount. Response is returned as raw JSON;
// typed response structs are deferred.
func (c *CustomerEntityClient) AggregateIncidentTasks(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/incident-tasks/aggregate", body)
}

// GetIncidentTask calls GET /incident-tasks/{id} on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) GetIncidentTask(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/incident-tasks/%s", url.PathEscape(id)), nil)
}

// UpdateIncidentTask calls PATCH /incident-tasks/{id} on the entity service
// to change a task's state and/or close notes.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) UpdateIncidentTask(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, fmt.Sprintf("/incident-tasks/%s", url.PathEscape(id)), body)
}

// PostDeployment calls POST /deployments on the entity service to create a new deployment.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) PostDeployment(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/deployments", body)
}

// PatchDeployment calls PATCH /deployments/{id} on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) PatchDeployment(ctx context.Context, deploymentID string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, fmt.Sprintf("/deployments/%s", url.PathEscape(deploymentID)), body)
}

// SearchDeployments calls POST /deployments/search on the entity service.
// Response is returned as raw JSON; field filtering to the portal shape is deferred.
func (c *CustomerEntityClient) SearchDeployments(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/deployments/search", body)
}

// SearchDeployedProducts calls POST /deployed-products/search on the entity service.
// Response is returned as raw JSON; field filtering to the portal shape is deferred.
func (c *CustomerEntityClient) SearchDeployedProducts(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/deployed-products/search", body)
}

// SearchProjectsByProductVersion calls POST /deployed-products/projects/search on the entity service.
// Response is returned as raw JSON; field filtering to the portal shape is deferred.
func (c *CustomerEntityClient) SearchProjectsByProductVersion(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/deployed-products/projects/search", body)
}

// PostDeployedProduct calls POST /deployed-products on the entity service to create a new deployed product.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) PostDeployedProduct(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/deployed-products", body)
}

// PatchDeployedProduct calls PATCH /deployed-products/{id} on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) PatchDeployedProduct(ctx context.Context, deployedProductID string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, fmt.Sprintf("/deployed-products/%s", url.PathEscape(deployedProductID)), body)
}

// SearchDeployedProductMetrics calls POST /deployed-products/{id}/metrics/search on the entity service.
// Response is returned as raw JSON; field filtering to the portal shape is deferred.
func (c *CustomerEntityClient) SearchDeployedProductMetrics(ctx context.Context, deployedProductID string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/deployed-products/%s/metrics/search", url.PathEscape(deployedProductID)), body)
}

// SearchDeployedProductUsageCounts calls POST /deployed-products/{id}/metrics/usage-counts/search on the entity service.
// Response is returned as raw JSON; field filtering to the portal shape is deferred.
func (c *CustomerEntityClient) SearchDeployedProductUsageCounts(ctx context.Context, deployedProductID string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/deployed-products/%s/metrics/usage-counts/search", url.PathEscape(deployedProductID)), body)
}

// SearchInstances calls POST /instances/search on the entity service.
// Response is returned as raw JSON; field filtering to the portal shape is deferred.
func (c *CustomerEntityClient) SearchInstances(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/instances/search", body)
}

// SearchInstanceMetrics calls POST /instances/metrics/search on the entity service.
// Response is returned as raw JSON; field filtering to the portal shape is deferred.
func (c *CustomerEntityClient) SearchInstanceMetrics(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/instances/metrics/search", body)
}

// SearchInstanceMetricsStats calls POST /instances/metrics/stats/search on the entity service.
// Response is returned as raw JSON; field filtering to the portal shape is deferred.
func (c *CustomerEntityClient) SearchInstanceMetricsStats(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/instances/metrics/stats/search", body)
}

// SearchInstanceUsage calls POST /instances/usages/search on the entity service.
// Response is returned as raw JSON; field filtering to the portal shape is deferred.
func (c *CustomerEntityClient) SearchInstanceUsage(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/instances/usages/search", body)
}

// SearchInstanceUsageStats calls POST /instances/usages/stats/search on the entity service.
// Response is returned as raw JSON; field filtering to the portal shape is deferred.
func (c *CustomerEntityClient) SearchInstanceUsageStats(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/instances/usages/stats/search", body)
}

// SearchChangeRequests calls POST /change-requests/search on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) SearchChangeRequests(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/change-requests/search", body)
}

// AggregateChangeRequests calls POST /change-requests/aggregate on the entity
// service: a server-side aggregation of change requests by a single field
// (e.g. state, assignmentGroup), capped to the top maxGroups buckets with
// the remainder folded into othersCount. Response is returned as raw JSON;
// typed response structs are deferred.
func (c *CustomerEntityClient) AggregateChangeRequests(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/change-requests/aggregate", body)
}

// GetChangeRequest calls GET /change-requests/{id} on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) GetChangeRequest(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/change-requests/%s", url.PathEscape(id)), nil)
}

// PatchChangeRequest calls PATCH /change-requests/{id} on the entity service.
// Response is returned as raw JSON.
func (c *CustomerEntityClient) PatchChangeRequest(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, fmt.Sprintf("/change-requests/%s", url.PathEscape(id)), body)
}

// GetChangeRequestLinkOptions calls POST /change-requests/link-options on the
// entity service: the project's deployments, the deployment products that
// follow from the deployments chosen so far, and the project's registered
// customer contacts (the read-only Customer Group).
// Response is returned as raw JSON.
func (c *CustomerEntityClient) GetChangeRequestLinkOptions(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/change-requests/link-options", body)
}

// GetChangeRequestApprovals calls GET /change-requests/{id}/approvals on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) GetChangeRequestApprovals(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/change-requests/%s/approvals", url.PathEscape(id)), nil)
}

// DecideChangeRequestApproval calls POST /change-requests/{id}/approvals/decision on the
// entity service to submit the caller's decision on their own pending approval.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) DecideChangeRequestApproval(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/change-requests/%s/approvals/decision", url.PathEscape(id)), body)
}

// ReplayChangeRequestMirrorFailure calls POST /sn-writeback-failures/{id}/replay
// on the entity service: re-sends one write of a change request the previous
// system is still missing (one entry of the detail's mirrorFailures, by its
// id) and clears it when the re-send succeeds. Response is returned as raw JSON.
func (c *CustomerEntityClient) ReplayChangeRequestMirrorFailure(ctx context.Context, failureID string) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/sn-writeback-failures/%s/replay", url.PathEscape(failureID)), nil)
}

// SearchTimeCards calls POST /time-cards/search on the entity service.
// Response is returned as raw JSON.
func (c *CustomerEntityClient) SearchTimeCards(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/time-cards/search", body)
}

// CreateTimeCard calls POST /time-cards on the entity service.
// Response is returned as raw JSON.
func (c *CustomerEntityClient) CreateTimeCard(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/time-cards", body)
}

// UpdateTimeCard calls PATCH /time-cards/{id} on the entity service. The body may
// carry editable fields, or a state transition ({"state":"approved"} or
// {"state":"rejected","leadComment":"..."}). Response is returned as raw JSON.
func (c *CustomerEntityClient) UpdateTimeCard(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, fmt.Sprintf("/time-cards/%s", url.PathEscape(id)), body)
}

// DeleteTimeCard calls DELETE /time-cards/{id} on the entity service.
func (c *CustomerEntityClient) DeleteTimeCard(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodDelete, fmt.Sprintf("/time-cards/%s", url.PathEscape(id)), nil)
}

// CreateCaseAttachment calls POST /attachments on the entity service.
// Response is returned as raw JSON.
func (c *CustomerEntityClient) CreateCaseAttachment(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/attachments", body)
}

// SearchCaseAttachments calls POST /attachments/search on the entity service.
// Response is returned as raw JSON.
func (c *CustomerEntityClient) SearchCaseAttachments(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/attachments/search", body)
}

// GetCaseAttachmentContent calls GET /attachments/{attachmentId}/content
// and returns the raw binary body with its Content-Type.
func (c *CustomerEntityClient) GetCaseAttachmentContent(ctx context.Context, attachmentID string) ([]byte, string, error) {
	return c.doBinary(ctx, fmt.Sprintf("/attachments/%s/content", url.PathEscape(attachmentID)))
}

// DeleteCaseAttachment calls DELETE /attachments/{attachmentId} on the entity service.
// Response is returned as raw JSON.
func (c *CustomerEntityClient) DeleteCaseAttachment(ctx context.Context, attachmentID string) ([]byte, error) {
	return c.do(ctx, http.MethodDelete, fmt.Sprintf("/attachments/%s", url.PathEscape(attachmentID)), nil)
}

// GetCaseAttachment calls GET /attachments/{attachmentId} on the entity
// service and returns a single attachment's metadata as raw JSON, including
// (once the entity service supports it) its storageKey — the SFTPGo path the
// attachment's file lives at.
//
// Assumption flagged: this single-attachment GET is not one of the entity
// service routes this backend calls elsewhere today (only .../search and
// .../{id}/content exist — see the other CaseAttachment methods above). It
// is required by the SFTPGo-backed share-creation path
// (handler.AttachmentStorageHandler.CreateAttachmentShare) to resolve an
// attachment's storageKey, and assumes a corresponding entity-service change
// that is out of scope for this layer/PR.
func (c *CustomerEntityClient) GetCaseAttachment(ctx context.Context, attachmentID string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/attachments/%s", url.PathEscape(attachmentID)), nil)
}

// ConfirmCaseAttachment calls POST /attachments/{attachmentId}/confirm on the
// entity service, transitioning a 'pending' attachment row (see
// CreateCaseAttachment's status field) to 'complete'. Used by the
// SFTPGo-backed upload flow once the browser's direct-to-SFTPGo upload has
// succeeded; see handler.AttachmentStorageHandler.ConfirmUpload.
func (c *CustomerEntityClient) ConfirmCaseAttachment(ctx context.Context, attachmentID string) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/attachments/%s/confirm", url.PathEscape(attachmentID)), nil)
}

// GetAttachment calls GET /attachments/{attachmentId} on the entity service.
// Response is returned as raw JSON.
func (c *CustomerEntityClient) GetAttachment(ctx context.Context, attachmentID string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/attachments/%s", url.PathEscape(attachmentID)), nil)
}

// UpdateAttachment calls PATCH /attachments/{attachmentId} on the entity service.
// Response is returned as raw JSON.
func (c *CustomerEntityClient) UpdateAttachment(ctx context.Context, attachmentID string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, fmt.Sprintf("/attachments/%s", url.PathEscape(attachmentID)), body)
}

// SearchCatalogs calls POST /catalogs/search on the entity service.
// Response is returned as raw JSON.
func (c *CustomerEntityClient) SearchCatalogs(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/catalogs/search", body)
}

// GetCatalogItemVariables calls GET /catalogs/{catalogId}/items/{catalogItemId}/variables
// on the entity service. Response is returned as raw JSON.
func (c *CustomerEntityClient) GetCatalogItemVariables(ctx context.Context, catalogID, catalogItemID string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/catalogs/%s/items/%s/variables", url.PathEscape(catalogID), url.PathEscape(catalogItemID)), nil)
}

// SearchProductVulnerabilities calls POST /products/vulnerabilities/search on the entity service.
// Response is returned as raw JSON.
func (c *CustomerEntityClient) SearchProductVulnerabilities(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/products/vulnerabilities/search", body)
}

// GetProductVulnerability calls GET /products/vulnerabilities/{id} on the entity service.
// Response is returned as raw JSON.
func (c *CustomerEntityClient) GetProductVulnerability(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/products/vulnerabilities/%s", url.PathEscape(id)), nil)
}

// CreateCallRequest calls POST /call-requests on the entity service.
// Response is returned as raw JSON.
func (c *CustomerEntityClient) CreateCallRequest(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/call-requests", body)
}

// SearchCallRequests calls POST /call-requests/search on the entity service.
// Response is returned as raw JSON.
func (c *CustomerEntityClient) SearchCallRequests(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/call-requests/search", body)
}

// SearchAllCallRequests calls POST /call-requests/search-all on the entity service
// (standalone call request search, not scoped to a parent case). Response is
// returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) SearchAllCallRequests(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/call-requests/search-all", body)
}

// PatchCallRequest calls PATCH /call-requests/{id} on the entity service.
// Response is returned as raw JSON.
func (c *CustomerEntityClient) PatchCallRequest(ctx context.Context, callRequestID string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, fmt.Sprintf("/call-requests/%s", url.PathEscape(callRequestID)), body)
}

// CreateChangeRequest calls POST /change-requests on the entity service.
// Response is returned as raw JSON.
func (c *CustomerEntityClient) CreateChangeRequest(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/change-requests", body)
}

// SearchITServices calls POST /services/search on the entity service.
// Response is returned as raw JSON.
func (c *CustomerEntityClient) SearchITServices(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/services/search", body)
}

// SearchServiceOfferings calls POST /service-offerings/search on the entity service.
// Response is returned as raw JSON.
func (c *CustomerEntityClient) SearchServiceOfferings(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/service-offerings/search", body)
}

// SearchGroups calls POST /groups/search on the entity service.
// Response is returned as raw JSON.
func (c *CustomerEntityClient) SearchGroups(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/groups/search", body)
}

// GetGroup calls GET /groups/{id} on the entity service: one group (name,
// description, email, manager) and its active members, opened from a change
// request approval stage's assignment group. Internal callers only, enforced by
// entity-service. Response is returned as raw JSON.
func (c *CustomerEntityClient) GetGroup(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/groups/%s", url.PathEscape(id)), nil)
}

// GetScheduleCatalogue calls GET /team-schedule/catalogue on the entity service.
// The Team Schedule zones, windows and absence kinds, in one payload: a client
// needs all three to draw a single day. Response is returned as raw JSON.
func (c *CustomerEntityClient) GetScheduleCatalogue(ctx context.Context) ([]byte, error) {
	return c.do(ctx, http.MethodGet, "/team-schedule/catalogue", nil)
}

// SearchScheduleAssignments calls POST /team-schedule/assignments/search on the
// entity service. Response is returned as raw JSON.
func (c *CustomerEntityClient) SearchScheduleAssignments(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/team-schedule/assignments/search", body)
}

// SearchScheduleAbsences calls POST /team-schedule/absences/search on the entity
// service. Response is returned as raw JSON.
func (c *CustomerEntityClient) SearchScheduleAbsences(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/team-schedule/absences/search", body)
}

// GetScheduleOnDuty calls GET /team-schedule/on-duty on the entity service, passing
// the optional `at` instant through unchanged. Response is raw JSON.
func (c *CustomerEntityClient) GetScheduleOnDuty(ctx context.Context, at string) ([]byte, error) {
	path := "/team-schedule/on-duty"
	if at != "" {
		path += "?at=" + url.QueryEscape(at)
	}
	return c.do(ctx, http.MethodGet, path, nil)
}

// SearchConfigurationItems calls POST /configuration-items/search on the entity service.
// Response is returned as raw JSON.
func (c *CustomerEntityClient) SearchConfigurationItems(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/configuration-items/search", body)
}

// CreateCaseGithubIssue calls POST /cases/{id}/github-issues on the entity service.
// Response is returned as raw JSON.
func (c *CustomerEntityClient) CreateCaseGithubIssue(ctx context.Context, caseID string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/cases/%s/github-issues", url.PathEscape(caseID)), body)
}

// SearchComments calls POST /comments/search on the entity service.
// The body must be a JSON-encoded SearchCommentsRequest (referenceId, referenceType, pagination).
func (c *CustomerEntityClient) SearchComments(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/comments/search", body)
}

// CreateComment calls POST /comments on the entity service — the reference-generic
// comment-create path used for reference types other than case (e.g. change request,
// incident), which have no dedicated create-comment route of their own.
// The body must be a JSON-encoded CreateCommentRequest (referenceId, referenceType, type, content).
func (c *CustomerEntityClient) CreateComment(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/comments", body)
}

// GetComment calls GET /comments/{id} on the entity service -- performs no
// author/role check itself (see entity-service's own comment_service.go,
// GetComment's doc comment). CommentHandler calls this before UpdateComment/
// DeleteComment purely to learn the comment's author, since this backend is
// the one that decides whether this caller may act on it.
func (c *CustomerEntityClient) GetComment(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/comments/%s", url.PathEscape(id)), nil)
}

// UpdateComment calls PATCH /comments/{id} on the entity service — the generic
// edit path for any comment regardless of the aggregate (case, change request,
// incident, ...) it belongs to. Not gated upstream at all: CommentHandler
// decides whether this caller may edit this comment (via GetComment, above)
// before ever calling this.
func (c *CustomerEntityClient) UpdateComment(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, fmt.Sprintf("/comments/%s", url.PathEscape(id)), body)
}

// DeleteComment calls DELETE /comments/{id} on the entity service — a soft
// delete, same "not gated upstream, CommentHandler decides" posture as
// UpdateComment. The entity service returns 204 No Content on success, so the
// returned byte slice is always empty; the caller only needs the error.
func (c *CustomerEntityClient) DeleteComment(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodDelete, fmt.Sprintf("/comments/%s", url.PathEscape(id)), nil)
}

// SearchConversations calls POST /conversations/search on the entity service.
// Response is returned as raw JSON; field filtering to the portal shape is deferred.
func (c *CustomerEntityClient) SearchConversations(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/conversations/search", body)
}

// SearchTaskSlas calls POST /slas/search on the entity service.
// Response is returned as raw JSON.
func (c *CustomerEntityClient) SearchTaskSlas(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/slas/search", body)
}

// GetTaskSla calls GET /slas/{id} on the entity service.
// Response is returned as raw JSON.
func (c *CustomerEntityClient) GetTaskSla(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/slas/%s", url.PathEscape(id)), nil)
}

// SearchCaseTasks calls POST /cases/{caseId}/tasks/search on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) SearchCaseTasks(ctx context.Context, caseID string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/cases/%s/tasks/search", url.PathEscape(caseID)), body)
}

// GetTask calls GET /tasks/{id} on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) GetTask(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/tasks/%s", url.PathEscape(id)), nil)
}

// CreateCaseTask calls POST /cases/{id}/tasks on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) CreateCaseTask(ctx context.Context, caseID string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/cases/%s/tasks", url.PathEscape(caseID)), body)
}

// UpdateTask calls PATCH /tasks/{id} on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) UpdateTask(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, fmt.Sprintf("/tasks/%s", url.PathEscape(id)), body)
}

// AddCaseTag calls POST /cases/{id}/tags on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) AddCaseTag(ctx context.Context, caseID string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/cases/%s/tags", url.PathEscape(caseID)), body)
}

// RemoveCaseTag calls DELETE /cases/{id}/tags/{tagId} on the entity service.
// Response is returned as raw JSON (typically empty for a 204 No Content).
func (c *CustomerEntityClient) RemoveCaseTag(ctx context.Context, caseID, tagID string) ([]byte, error) {
	return c.do(ctx, http.MethodDelete, fmt.Sprintf("/cases/%s/tags/%s", url.PathEscape(caseID), url.PathEscape(tagID)), nil)
}

// SearchTags calls POST /tags/search on the entity service.
// The request body is forwarded verbatim; the response is returned as raw JSON.
func (c *CustomerEntityClient) SearchTags(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/tags/search", body)
}

// CreateKBArticle calls POST /kb-articles on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) CreateKBArticle(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/kb-articles", body)
}

// GetKBArticle calls GET /kb-articles/{id} on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) GetKBArticle(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/kb-articles/%s", url.PathEscape(id)), nil)
}

// SearchKBArticles calls POST /kb-articles/search on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) SearchKBArticles(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/kb-articles/search", body)
}

// PatchKBArticleState calls PATCH /kb-articles/{id}/state on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) PatchKBArticleState(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, fmt.Sprintf("/kb-articles/%s/state", url.PathEscape(id)), body)
}

// SearchKBManagerUsers calls POST /kb-manager-users/search on the entity service.
func (c *CustomerEntityClient) SearchKBManagerUsers(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/kb-manager-users/search", body)
}

// SearchKBManagerGroups calls POST /kb-manager-groups/search on the entity service.
func (c *CustomerEntityClient) SearchKBManagerGroups(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/kb-manager-groups/search", body)
}

// PatchKBArticleContent calls PATCH /kb-articles/{id} on the entity service.
func (c *CustomerEntityClient) PatchKBArticleContent(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, fmt.Sprintf("/kb-articles/%s", url.PathEscape(id)), body)
}

// ListKnowledgeBases calls GET /knowledge-bases on the entity service.
func (c *CustomerEntityClient) ListKnowledgeBases(ctx context.Context) ([]byte, error) {
	return c.do(ctx, http.MethodGet, "/knowledge-bases", nil)
}

// DeleteKBArticle calls DELETE /kb-articles/{id} on the entity service.
func (c *CustomerEntityClient) DeleteKBArticle(ctx context.Context, id string) error {
	_, err := c.do(ctx, http.MethodDelete, fmt.Sprintf("/kb-articles/%s", url.PathEscape(id)), nil)
	return err
}

// ListKBArticleHistory calls GET /kb-articles/{id}/history on the entity service.
func (c *CustomerEntityClient) ListKBArticleHistory(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/kb-articles/%s/history", url.PathEscape(id)), nil)
}

// GetUsersByIDs calls POST /users/by-ids on the entity service.
func (c *CustomerEntityClient) GetUsersByIDs(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/users/by-ids", body)
}

// CreateKnowledgeBase calls POST /knowledge-bases on the entity service.
func (c *CustomerEntityClient) CreateKnowledgeBase(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/knowledge-bases", body)
}

// UpdateKnowledgeBaseName calls PATCH /knowledge-bases/{id} on the entity service.
func (c *CustomerEntityClient) UpdateKnowledgeBaseName(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, fmt.Sprintf("/knowledge-bases/%s", url.PathEscape(id)), body)
}

// SetKnowledgeBaseActive calls PATCH /knowledge-bases/{id}/active on the entity service.
func (c *CustomerEntityClient) SetKnowledgeBaseActive(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, fmt.Sprintf("/knowledge-bases/%s/active", url.PathEscape(id)), body)
}

// CreateKBManagerUser calls POST /kb-manager-users on the entity service.
func (c *CustomerEntityClient) CreateKBManagerUser(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/kb-manager-users", body)
}

// DeleteKBManagerUser calls DELETE /kb-manager-users on the entity service.
func (c *CustomerEntityClient) DeleteKBManagerUser(ctx context.Context, body []byte) error {
	_, err := c.do(ctx, http.MethodDelete, "/kb-manager-users", body)
	return err
}

// CreateKBManagerGroup calls POST /kb-manager-groups on the entity service.
func (c *CustomerEntityClient) CreateKBManagerGroup(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/kb-manager-groups", body)
}

// DeleteKBManagerGroup calls DELETE /kb-manager-groups on the entity service.
func (c *CustomerEntityClient) DeleteKBManagerGroup(ctx context.Context, body []byte) error {
	_, err := c.do(ctx, http.MethodDelete, "/kb-manager-groups", body)
	return err
}

// GetAlert calls GET /alerts/{id} on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) GetAlert(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/alerts/%s", url.PathEscape(id)), nil)
}

// GetSmartAlert calls GET /smart-alerts/{id} on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) GetSmartAlert(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/smart-alerts/%s", url.PathEscape(id)), nil)
}

// CreateOutage calls POST /outages on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) CreateOutage(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/outages", body)
}

// SearchOutages calls POST /outages/search on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) SearchOutages(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/outages/search", body)
}

// GetOutage calls GET /outages/{id} on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) GetOutage(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/outages/%s", url.PathEscape(id)), nil)
}

// PatchOutage calls PATCH /outages/{id} on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) PatchOutage(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, fmt.Sprintf("/outages/%s", url.PathEscape(id)), body)
}

// AddOutageCommunication calls POST /outages/{id}/communications on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) AddOutageCommunication(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/outages/%s/communications", url.PathEscape(id)), body)
}

// SearchOutageCommunications calls POST /outages/{id}/communications/search on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) SearchOutageCommunications(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/outages/%s/communications/search", url.PathEscape(id)), body)
}

// GetOutageMetadata calls GET /outages/metadata on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) GetOutageMetadata(ctx context.Context) ([]byte, error) {
	return c.do(ctx, http.MethodGet, "/outages/metadata", nil)
}

// CreateAnnouncementRequest calls POST /announcement-requests on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) CreateAnnouncementRequest(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/announcement-requests", body)
}

// GetAnnouncementRequest calls GET /announcement-requests/{id} on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) GetAnnouncementRequest(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/announcement-requests/%s", url.PathEscape(id)), nil)
}

// SearchAnnouncementRequests calls POST /announcement-requests/search on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) SearchAnnouncementRequests(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/announcement-requests/search", body)
}

// UpdateAnnouncementRequest calls PATCH /announcement-requests/{id} on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) UpdateAnnouncementRequest(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, fmt.Sprintf("/announcement-requests/%s", url.PathEscape(id)), body)
}

// RecordAnnouncementRequestDryRun calls POST /announcement-requests/{id}/dry-run on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) RecordAnnouncementRequestDryRun(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/announcement-requests/%s/dry-run", url.PathEscape(id)), body)
}

// SubmitAnnouncementRequest calls POST /announcement-requests/{id}/submit on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) SubmitAnnouncementRequest(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/announcement-requests/%s/submit", url.PathEscape(id)), body)
}

// ApproveAnnouncementRequest calls POST /announcement-requests/{id}/approve on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) ApproveAnnouncementRequest(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/announcement-requests/%s/approve", url.PathEscape(id)), body)
}

// ScheduleAnnouncementRequest calls POST /announcement-requests/{id}/schedule on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) ScheduleAnnouncementRequest(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/announcement-requests/%s/schedule", url.PathEscape(id)), body)
}

// PublishAnnouncementRequest calls POST /announcement-requests/{id}/publish on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) PublishAnnouncementRequest(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/announcement-requests/%s/publish", url.PathEscape(id)), body)
}

// CreateAnnouncementRequestUpdate calls POST /announcement-requests/{id}/updates on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) CreateAnnouncementRequestUpdate(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/announcement-requests/%s/updates", url.PathEscape(id)), body)
}

// ListAnnouncementRequestUpdates calls GET /announcement-requests/{id}/updates on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) ListAnnouncementRequestUpdates(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/announcement-requests/%s/updates", url.PathEscape(id)), nil)
}

// RecordAnnouncementRequestDeliveries calls POST /announcement-requests/{id}/deliveries on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) RecordAnnouncementRequestDeliveries(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, fmt.Sprintf("/announcement-requests/%s/deliveries", url.PathEscape(id)), body)
}

// ListAnnouncementRequestDeliveries calls GET /announcement-requests/{id}/deliveries on the entity service.
// Response is returned as raw JSON; typed response structs are deferred.
func (c *CustomerEntityClient) ListAnnouncementRequestDeliveries(ctx context.Context, id string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, fmt.Sprintf("/announcement-requests/%s/deliveries", url.PathEscape(id)), nil)
}

// CreateScheduleAssignment calls POST /team-schedule/assignments on the entity
// service. Lead edit; the entity service enforces that the caller leads the team.
func (c *CustomerEntityClient) CreateScheduleAssignment(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/team-schedule/assignments", body)
}

// UpdateScheduleAssignment calls PATCH /team-schedule/assignments/{id}.
func (c *CustomerEntityClient) UpdateScheduleAssignment(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, "/team-schedule/assignments/"+url.PathEscape(id), body)
}

// DeleteScheduleAssignment calls DELETE /team-schedule/assignments/{id}, passing
// the optional note through unchanged.
func (c *CustomerEntityClient) DeleteScheduleAssignment(ctx context.Context, id, note string) ([]byte, error) {
	path := "/team-schedule/assignments/" + url.PathEscape(id)
	if note != "" {
		path += "?note=" + url.QueryEscape(note)
	}
	return c.do(ctx, http.MethodDelete, path, nil)
}

// GetScheduleActivity calls GET /team-schedule/activity for one team and window.
func (c *CustomerEntityClient) GetScheduleActivity(ctx context.Context, teamKey, from, to string) ([]byte, error) {
	q := url.Values{}
	q.Set("teamKey", teamKey)
	q.Set("from", from)
	q.Set("to", to)
	return c.do(ctx, http.MethodGet, "/team-schedule/activity?"+q.Encode(), nil)
}

// GetPagingChain calls GET /team-schedule/paging-chain: who is on each tier of
// a family's Case Paging chain, and what the caller may change.
func (c *CustomerEntityClient) GetPagingChain(ctx context.Context, family string) ([]byte, error) {
	path := "/team-schedule/paging-chain"
	if family != "" {
		path += "?family=" + url.QueryEscape(family)
	}
	return c.do(ctx, http.MethodGet, path, nil)
}

// UpdatePagingMember calls PATCH /team-schedule/paging-chain/members/{id}.
func (c *CustomerEntityClient) UpdatePagingMember(ctx context.Context, id string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPatch, "/team-schedule/paging-chain/members/"+url.PathEscape(id), body)
}

// PutPagingContact calls PUT /team-schedule/paging-contacts/{userId}: set the
// paging-only phone number entity-service keeps for a person.
func (c *CustomerEntityClient) PutPagingContact(ctx context.Context, userID string, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPut, "/team-schedule/paging-contacts/"+url.PathEscape(userID), body)
}

// DeletePagingContact calls DELETE /team-schedule/paging-contacts/{userId}.
func (c *CustomerEntityClient) DeletePagingContact(ctx context.Context, userID string) ([]byte, error) {
	return c.do(ctx, http.MethodDelete, "/team-schedule/paging-contacts/"+url.PathEscape(userID), nil)
}

// TestPagingContact calls POST /team-schedule/paging-contacts/{userId}/test:
// a test call to the person's paging number.
func (c *CustomerEntityClient) TestPagingContact(ctx context.Context, userID string) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/team-schedule/paging-contacts/"+url.PathEscape(userID)+"/test", nil)
}

// GetPagingReadiness calls GET /team-schedule/paging-readiness: whether each
// Case Paging chain has someone to page on every day of the next `days`.
func (c *CustomerEntityClient) GetPagingReadiness(ctx context.Context, days int) ([]byte, error) {
	q := url.Values{}
	q.Set("days", strconv.Itoa(days))
	return c.do(ctx, http.MethodGet, "/team-schedule/paging-readiness?"+q.Encode(), nil)
}

// GetMyLeadTeams calls GET /team-schedule/my-lead-teams on the entity service.
func (c *CustomerEntityClient) GetMyLeadTeams(ctx context.Context) ([]byte, error) {
	return c.do(ctx, http.MethodGet, "/team-schedule/my-lead-teams", nil)
}

// ApplyScheduleRange calls POST /team-schedule/assignments/apply on the entity
// service -- how the roster's picker edits a span of days in one call.
func (c *CustomerEntityClient) ApplyScheduleRange(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/team-schedule/assignments/apply", body)
}

// ApplyScheduleAbsence calls POST /team-schedule/absences/apply on the entity
// service -- the picker marking somebody away, or bringing them back.
func (c *CustomerEntityClient) ApplyScheduleAbsence(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/team-schedule/absences/apply", body)
}

// DeleteScheduleAbsence calls DELETE /team-schedule/absences/{id} on the
// entity service -- one absence removed whole, open-ended ones included.
func (c *CustomerEntityClient) DeleteScheduleAbsence(ctx context.Context, id, note string) ([]byte, error) {
	path := "/team-schedule/absences/" + url.PathEscape(id)
	if note != "" {
		path += "?note=" + url.QueryEscape(note)
	}
	return c.do(ctx, http.MethodDelete, path, nil)
}

// CreateScheduleAbsenceKind calls POST /team-schedule/absence-kinds on the
// entity service -- a lead adding a shared leave or allocation tag.
func (c *CustomerEntityClient) CreateScheduleAbsenceKind(ctx context.Context, body []byte) ([]byte, error) {
	return c.do(ctx, http.MethodPost, "/team-schedule/absence-kinds", body)
}

// DeleteScheduleAbsenceKind calls DELETE /team-schedule/absence-kinds/{code}
// on the entity service -- a lead deleting a tag a lead added.
func (c *CustomerEntityClient) DeleteScheduleAbsenceKind(ctx context.Context, code string) ([]byte, error) {
	return c.do(ctx, http.MethodDelete, "/team-schedule/absence-kinds/"+url.PathEscape(code), nil)
}

// GetScheduleEditMarkers calls GET /team-schedule/edit-markers on the entity
// service -- which roster cells a person has changed, in a window.
func (c *CustomerEntityClient) GetScheduleEditMarkers(ctx context.Context, from, to string) ([]byte, error) {
	return c.do(ctx, http.MethodGet,
		fmt.Sprintf("/team-schedule/edit-markers?from=%s&to=%s",
			url.QueryEscape(from), url.QueryEscape(to)), nil)
}
