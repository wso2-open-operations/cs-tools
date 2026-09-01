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

package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/directory"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/scim"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/updates"
)

// testUser is the authenticated user injected into request contexts. UserID is
// the identity provider's user id carried on the gateway-validated token — it
// is NOT the platform's own user record id (see testPlatformUserID).
var testUser = &middleware.UserInfo{
	Email:  "agent@example.com",
	UserID: "f2d9bf5b-7067-43dc-8578-802c8623af5d",
	Groups: []string{"csm-agents"},
}

// testPlatformUserID is the id GET /users/me resolves for testUser: the
// platform's own user record id, from a different id space than
// testUser.UserID. Any check comparing a platform record's user reference
// against the caller must use this one; the two values are deliberately kept
// distinct here so a regression back to the token claim fails the tests.
const testPlatformUserID = "94a1b01b-1b3c-f050-cb68-98aebd4bcb27"

// withUser returns r with testUser stored in its context.
func withUser(r *http.Request) *http.Request {
	return r.WithContext(middleware.WithUserInfo(r.Context(), testUser))
}

// ----- assertion helpers -----

// assertStatus fails if the recorded status code differs from want.
func assertStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Errorf("status = %d, want %d; body: %s", w.Code, want, w.Body.String())
	}
}

// assertContentType fails if the Content-Type header differs from want.
func assertContentType(t *testing.T, w *httptest.ResponseRecorder, want string) {
	t.Helper()
	if ct := w.Header().Get("Content-Type"); ct != want {
		t.Errorf("Content-Type = %q, want %q", ct, want)
	}
}

// assertErrorMessage decodes {"message":"..."} and checks the message field.
func assertErrorMessage(t *testing.T, w *httptest.ResponseRecorder, want string) {
	t.Helper()
	var body struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v; raw: %s", err, w.Body.String())
	}
	if body.Message != want {
		t.Errorf("message = %q, want %q", body.Message, want)
	}
}

// decodeJSON decodes the recorder body into T and returns it.
func decodeJSON[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.NewDecoder(w.Body).Decode(&v); err != nil {
		t.Fatalf("decode response body: %v; raw: %s", err, w.Body.String())
	}
	return v
}

// ----- mock entity case client -----

type mockEntityCaseClient struct {
	createCaseFn               func(ctx context.Context, body []byte) ([]byte, error)
	patchCaseFn                func(ctx context.Context, caseID string, body []byte) ([]byte, error)
	createCaseCommentFn        func(ctx context.Context, caseID string, body []byte) ([]byte, error)
	searchCommentsFn           func(ctx context.Context, body []byte) ([]byte, error)
	searchCaseActivitiesFn     func(ctx context.Context, caseID string, body []byte) ([]byte, error)
	searchCasesFn              func(ctx context.Context, body []byte) ([]byte, error)
	aggregateCasesFn           func(ctx context.Context, body []byte) ([]byte, error)
	searchFeedbackFn           func(ctx context.Context, body []byte) ([]byte, error)
	aggregateFeedbackFn        func(ctx context.Context, body []byte) ([]byte, error)
	getCaseFn                  func(ctx context.Context, caseID string) ([]byte, error)
	createCaseAttachmentFn     func(ctx context.Context, body []byte) ([]byte, error)
	searchCaseAttachmentsFn    func(ctx context.Context, body []byte) ([]byte, error)
	getCaseAttachmentContentFn func(ctx context.Context, attachmentID string) ([]byte, string, error)
	deleteCaseAttachmentFn     func(ctx context.Context, attachmentID string) ([]byte, error)
	getAttachmentFn            func(ctx context.Context, attachmentID string) ([]byte, error)
	updateAttachmentFn         func(ctx context.Context, attachmentID string, body []byte) ([]byte, error)
	createCallRequestFn        func(ctx context.Context, body []byte) ([]byte, error)
	searchCallRequestsFn       func(ctx context.Context, body []byte) ([]byte, error)
	searchAllCallRequestsFn    func(ctx context.Context, body []byte) ([]byte, error)
	patchCallRequestFn         func(ctx context.Context, callRequestID string, body []byte) ([]byte, error)
	createCaseGithubIssueFn    func(ctx context.Context, caseID string, body []byte) ([]byte, error)
	addCaseTagFn               func(ctx context.Context, caseID string, body []byte) ([]byte, error)
	removeCaseTagFn            func(ctx context.Context, caseID, tagID string) ([]byte, error)
	searchTagsFn               func(ctx context.Context, body []byte) ([]byte, error)
	getUserMeFn                func(ctx context.Context) ([]byte, error)
}

// GetUserMe defaults to the platform user record for testUser: note the id is
// deliberately NOT testUser.UserID, mirroring production where the identity
// provider's user id and the platform's own record id are unrelated values.
func (m *mockEntityCaseClient) GetUserMe(ctx context.Context) ([]byte, error) {
	if m.getUserMeFn != nil {
		return m.getUserMeFn(ctx)
	}
	return []byte(`{"id":"` + testPlatformUserID + `","email":"` + testUser.Email + `"}`), nil
}

func (m *mockEntityCaseClient) CreateCase(ctx context.Context, body []byte) ([]byte, error) {
	if m.createCaseFn != nil {
		return m.createCaseFn(ctx, body)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityCaseClient) PatchCase(ctx context.Context, caseID string, body []byte) ([]byte, error) {
	if m.patchCaseFn != nil {
		return m.patchCaseFn(ctx, caseID, body)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityCaseClient) CreateCaseComment(ctx context.Context, caseID string, body []byte) ([]byte, error) {
	if m.createCaseCommentFn != nil {
		return m.createCaseCommentFn(ctx, caseID, body)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityCaseClient) SearchComments(ctx context.Context, body []byte) ([]byte, error) {
	if m.searchCommentsFn != nil {
		return m.searchCommentsFn(ctx, body)
	}
	return []byte(`{"comments":[],"total":0,"limit":20,"offset":0,"hasMore":false}`), nil
}

func (m *mockEntityCaseClient) SearchCaseActivities(ctx context.Context, caseID string, body []byte) ([]byte, error) {
	if m.searchCaseActivitiesFn != nil {
		return m.searchCaseActivitiesFn(ctx, caseID, body)
	}
	return []byte(`{"activities":[],"total":0,"limit":20,"offset":0,"hasMore":false}`), nil
}

func (m *mockEntityCaseClient) SearchCases(ctx context.Context, body []byte) ([]byte, error) {
	if m.searchCasesFn != nil {
		return m.searchCasesFn(ctx, body)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityCaseClient) AggregateCases(ctx context.Context, body []byte) ([]byte, error) {
	if m.aggregateCasesFn != nil {
		return m.aggregateCasesFn(ctx, body)
	}
	return []byte(`{"groups":[],"othersCount":0,"totalRecords":0}`), nil
}

func (m *mockEntityCaseClient) SearchFeedback(ctx context.Context, body []byte) ([]byte, error) {
	if m.searchFeedbackFn != nil {
		return m.searchFeedbackFn(ctx, body)
	}
	return []byte(`{"results":[],"totalRecords":0}`), nil
}

func (m *mockEntityCaseClient) AggregateFeedback(ctx context.Context, body []byte) ([]byte, error) {
	if m.aggregateFeedbackFn != nil {
		return m.aggregateFeedbackFn(ctx, body)
	}
	return []byte(`{"buckets":[],"totalRecords":0}`), nil
}

func (m *mockEntityCaseClient) GetCase(ctx context.Context, caseID string) ([]byte, error) {
	if m.getCaseFn != nil {
		return m.getCaseFn(ctx, caseID)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityCaseClient) CreateCaseAttachment(ctx context.Context, body []byte) ([]byte, error) {
	if m.createCaseAttachmentFn != nil {
		return m.createCaseAttachmentFn(ctx, body)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityCaseClient) SearchCaseAttachments(ctx context.Context, body []byte) ([]byte, error) {
	if m.searchCaseAttachmentsFn != nil {
		return m.searchCaseAttachmentsFn(ctx, body)
	}
	return []byte(`{"attachments":[],"total":0,"limit":20,"offset":0,"hasMore":false}`), nil
}

func (m *mockEntityCaseClient) GetCaseAttachmentContent(ctx context.Context, attachmentID string) ([]byte, string, error) {
	if m.getCaseAttachmentContentFn != nil {
		return m.getCaseAttachmentContentFn(ctx, attachmentID)
	}
	return []byte(`fake-content`), "image/png", nil
}

func (m *mockEntityCaseClient) DeleteCaseAttachment(ctx context.Context, attachmentID string) ([]byte, error) {
	if m.deleteCaseAttachmentFn != nil {
		return m.deleteCaseAttachmentFn(ctx, attachmentID)
	}
	return []byte(`{"message":"Attachment deleted successfully."}`), nil
}

func (m *mockEntityCaseClient) GetAttachment(ctx context.Context, attachmentID string) ([]byte, error) {
	if m.getAttachmentFn != nil {
		return m.getAttachmentFn(ctx, attachmentID)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityCaseClient) UpdateAttachment(ctx context.Context, attachmentID string, body []byte) ([]byte, error) {
	if m.updateAttachmentFn != nil {
		return m.updateAttachmentFn(ctx, attachmentID, body)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityCaseClient) CreateCallRequest(ctx context.Context, body []byte) ([]byte, error) {
	if m.createCallRequestFn != nil {
		return m.createCallRequestFn(ctx, body)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityCaseClient) SearchCallRequests(ctx context.Context, body []byte) ([]byte, error) {
	if m.searchCallRequestsFn != nil {
		return m.searchCallRequestsFn(ctx, body)
	}
	return []byte(`{"callRequests":[],"total":0,"limit":20,"offset":0}`), nil
}

func (m *mockEntityCaseClient) SearchAllCallRequests(ctx context.Context, body []byte) ([]byte, error) {
	if m.searchAllCallRequestsFn != nil {
		return m.searchAllCallRequestsFn(ctx, body)
	}
	return []byte(`{"callRequests":[],"total":0,"limit":20,"offset":0}`), nil
}

func (m *mockEntityCaseClient) PatchCallRequest(ctx context.Context, callRequestID string, body []byte) ([]byte, error) {
	if m.patchCallRequestFn != nil {
		return m.patchCallRequestFn(ctx, callRequestID, body)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityCaseClient) CreateCaseGithubIssue(ctx context.Context, caseID string, body []byte) ([]byte, error) {
	if m.createCaseGithubIssueFn != nil {
		return m.createCaseGithubIssueFn(ctx, caseID, body)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityCaseClient) AddCaseTag(ctx context.Context, caseID string, body []byte) ([]byte, error) {
	if m.addCaseTagFn != nil {
		return m.addCaseTagFn(ctx, caseID, body)
	}
	return []byte(`{"id":"11111111-1111-1111-1111-111111111111","label":"micro-gw","color":null}`), nil
}

func (m *mockEntityCaseClient) RemoveCaseTag(ctx context.Context, caseID, tagID string) ([]byte, error) {
	if m.removeCaseTagFn != nil {
		return m.removeCaseTagFn(ctx, caseID, tagID)
	}
	return nil, nil
}

func (m *mockEntityCaseClient) SearchTags(ctx context.Context, body []byte) ([]byte, error) {
	if m.searchTagsFn != nil {
		return m.searchTagsFn(ctx, body)
	}
	return []byte(`{"tags":[]}`), nil
}

// ----- mock updates client -----

type mockUpdatesClient struct {
	productFn func(ctx context.Context) ([]updates.ProductUpdateLevel, error)
	searchFn  func(ctx context.Context, payload updates.SearchPayload, email string) (map[string]updates.UpdateLevelGroup, error)
}

func (m *mockUpdatesClient) GetProductUpdateLevels(ctx context.Context) ([]updates.ProductUpdateLevel, error) {
	if m.productFn != nil {
		return m.productFn(ctx)
	}
	return []updates.ProductUpdateLevel{}, nil
}

func (m *mockUpdatesClient) SearchUpdatesBetweenUpdateLevels(ctx context.Context, payload updates.SearchPayload, email string) (map[string]updates.UpdateLevelGroup, error) {
	if m.searchFn != nil {
		return m.searchFn(ctx, payload, email)
	}
	return map[string]updates.UpdateLevelGroup{}, nil
}

// ----- mock SCIM client -----

type mockSCIMClient struct {
	searchUserFn         func(ctx context.Context, email string) (*scim.UserInfo, error)
	searchExternalUserFn func(ctx context.Context, email string) (*scim.ExternalUserInfo, error)
	updateUserPhoneFn    func(ctx context.Context, userID, mobile string) (*string, error)
}

func (m *mockSCIMClient) SearchUser(ctx context.Context, email string) (*scim.UserInfo, error) {
	if m.searchUserFn != nil {
		return m.searchUserFn(ctx, email)
	}
	return nil, nil
}

func (m *mockSCIMClient) SearchExternalUser(ctx context.Context, email string) (*scim.ExternalUserInfo, error) {
	if m.searchExternalUserFn != nil {
		return m.searchExternalUserFn(ctx, email)
	}
	return nil, nil
}

func (m *mockSCIMClient) UpdateUserPhone(ctx context.Context, userID, mobile string) (*string, error) {
	if m.updateUserPhoneFn != nil {
		return m.updateUserPhoneFn(ctx, userID, mobile)
	}
	return nil, nil
}

// ----- mock entity user client -----

type mockEntityUserClient struct {
	getUserMeFn   func(ctx context.Context) ([]byte, error)
	patchUserMeFn func(ctx context.Context, body []byte) ([]byte, error)
	searchUsersFn func(ctx context.Context, body []byte) ([]byte, error)
	getUserFn     func(ctx context.Context, id string) ([]byte, error)
}

func (m *mockEntityUserClient) GetUser(ctx context.Context, id string) ([]byte, error) {
	if m.getUserFn != nil {
		return m.getUserFn(ctx, id)
	}
	return []byte(`{"id":"` + id + `","email":"","roles":[],"groups":[],"teams":[]}`), nil
}

// testTeamRegistry is a representative registry in its configured wire form: an
// account-based team with a family and a backing group id, a bare row with
// neither (some real rows legitimately have only two fields), and a team from
// the other discipline. Every name is an invented placeholder.
const testTeamRegistry = "abt-1|ABT One|cre-abt|aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa," +
	"abt-2|ABT Two," +
	"beta|Beta Team|sre-abt|bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

// testRoles is a two-entry allow-list, small enough that "the catalogue is
// exactly what was configured" is a cheap assertion.
const testRoles = "agent,timecard_approver"

// testDirectory builds the startup-resolved catalogue a handler is constructed
// with, from the same parse path main() uses.
func testDirectory(t *testing.T) *directory.Directory {
	t.Helper()
	teams, err := directory.ParseTeamRegistry(testTeamRegistry)
	if err != nil {
		t.Fatalf("ParseTeamRegistry(%q): %v", testTeamRegistry, err)
	}
	roles, err := directory.ParseRoles(testRoles)
	if err != nil {
		t.Fatalf("ParseRoles(%q): %v", testRoles, err)
	}
	dir, err := directory.New(teams, roles)
	if err != nil {
		t.Fatalf("directory.New: %v", err)
	}
	return dir
}

func (m *mockEntityUserClient) GetUserMe(ctx context.Context) ([]byte, error) {
	if m.getUserMeFn != nil {
		return m.getUserMeFn(ctx)
	}
	return []byte(`{"id":"11111111-1111-1111-1111-111111111111","email":"","lastName":"","roles":[]}`), nil
}

func (m *mockEntityUserClient) PatchUserMe(ctx context.Context, body []byte) ([]byte, error) {
	if m.patchUserMeFn != nil {
		return m.patchUserMeFn(ctx, body)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityUserClient) SearchUsers(ctx context.Context, body []byte) ([]byte, error) {
	if m.searchUsersFn != nil {
		return m.searchUsersFn(ctx, body)
	}
	return []byte(`{}`), nil
}

// ----- mock entity account client -----

type mockEntityAccountClient struct {
	getAccountFn            func(ctx context.Context, id string) ([]byte, error)
	searchAccountsFn        func(ctx context.Context, body []byte) ([]byte, error)
	searchAccountContactsFn func(ctx context.Context, accountID string, body []byte) ([]byte, error)
}

func (m *mockEntityAccountClient) GetAccount(ctx context.Context, id string) ([]byte, error) {
	if m.getAccountFn != nil {
		return m.getAccountFn(ctx, id)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityAccountClient) SearchAccounts(ctx context.Context, body []byte) ([]byte, error) {
	if m.searchAccountsFn != nil {
		return m.searchAccountsFn(ctx, body)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityAccountClient) SearchAccountContacts(ctx context.Context, accountID string, body []byte) ([]byte, error) {
	if m.searchAccountContactsFn != nil {
		return m.searchAccountContactsFn(ctx, accountID, body)
	}
	return []byte(`{}`), nil
}

// ----- mock entity project client -----

type mockEntityProjectClient struct {
	getProjectFn            func(ctx context.Context, id string) ([]byte, error)
	searchProjectsFn        func(ctx context.Context, body []byte) ([]byte, error)
	searchProjectContactsFn func(ctx context.Context, projectID string, body []byte) ([]byte, error)
	getProjectContactFn     func(ctx context.Context, projectID, contactID string) ([]byte, error)
	updateProjectFn         func(ctx context.Context, id string, body []byte) ([]byte, error)
}

func (m *mockEntityProjectClient) GetProjectContact(ctx context.Context, projectID, contactID string) ([]byte, error) {
	if m.getProjectContactFn != nil {
		return m.getProjectContactFn(ctx, projectID, contactID)
	}
	return []byte(`{"id":"` + contactID + `","name":"","email":"","registrationState":"","notificationsEnabled":false,"roles":[]}`), nil
}

func (m *mockEntityProjectClient) GetProject(ctx context.Context, id string) ([]byte, error) {
	if m.getProjectFn != nil {
		return m.getProjectFn(ctx, id)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityProjectClient) SearchProjects(ctx context.Context, body []byte) ([]byte, error) {
	if m.searchProjectsFn != nil {
		return m.searchProjectsFn(ctx, body)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityProjectClient) SearchProjectContacts(ctx context.Context, projectID string, body []byte) ([]byte, error) {
	if m.searchProjectContactsFn != nil {
		return m.searchProjectContactsFn(ctx, projectID, body)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityProjectClient) UpdateProject(ctx context.Context, id string, body []byte) ([]byte, error) {
	if m.updateProjectFn != nil {
		return m.updateProjectFn(ctx, id, body)
	}
	return []byte(`{}`), nil
}

// ----- mock entity product client -----

type mockEntityProductClient struct {
	searchProductsFn        func(ctx context.Context, body []byte) ([]byte, error)
	searchProductVersionsFn func(ctx context.Context, productID string, body []byte) ([]byte, error)
}

func (m *mockEntityProductClient) SearchProducts(ctx context.Context, body []byte) ([]byte, error) {
	if m.searchProductsFn != nil {
		return m.searchProductsFn(ctx, body)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityProductClient) SearchProductVersions(ctx context.Context, productID string, body []byte) ([]byte, error) {
	if m.searchProductVersionsFn != nil {
		return m.searchProductVersionsFn(ctx, productID, body)
	}
	return []byte(`{}`), nil
}

// ----- mock entity incident client -----

type mockEntityIncidentClient struct {
	searchIncidentsFn          func(ctx context.Context, body []byte) ([]byte, error)
	aggregateIncidentsFn       func(ctx context.Context, body []byte) ([]byte, error)
	createIncidentFn           func(ctx context.Context, body []byte) ([]byte, error)
	getIncidentFn              func(ctx context.Context, id string) ([]byte, error)
	patchIncidentFn            func(ctx context.Context, id string, body []byte) ([]byte, error)
	createCommentFn            func(ctx context.Context, body []byte) ([]byte, error)
	searchCommentsFn           func(ctx context.Context, body []byte) ([]byte, error)
	searchIncidentActivitiesFn func(ctx context.Context, id string, body []byte) ([]byte, error)
	handOffIncidentFn          func(ctx context.Context, id string, body []byte) ([]byte, error)
}

func (m *mockEntityIncidentClient) SearchIncidents(ctx context.Context, body []byte) ([]byte, error) {
	if m.searchIncidentsFn != nil {
		return m.searchIncidentsFn(ctx, body)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityIncidentClient) AggregateIncidents(ctx context.Context, body []byte) ([]byte, error) {
	if m.aggregateIncidentsFn != nil {
		return m.aggregateIncidentsFn(ctx, body)
	}
	return []byte(`{"groups":[],"othersCount":0,"totalRecords":0}`), nil
}

func (m *mockEntityIncidentClient) CreateIncident(ctx context.Context, body []byte) ([]byte, error) {
	if m.createIncidentFn != nil {
		return m.createIncidentFn(ctx, body)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityIncidentClient) GetIncident(ctx context.Context, id string) ([]byte, error) {
	if m.getIncidentFn != nil {
		return m.getIncidentFn(ctx, id)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityIncidentClient) PatchIncident(ctx context.Context, id string, body []byte) ([]byte, error) {
	if m.patchIncidentFn != nil {
		return m.patchIncidentFn(ctx, id, body)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityIncidentClient) CreateComment(ctx context.Context, body []byte) ([]byte, error) {
	if m.createCommentFn != nil {
		return m.createCommentFn(ctx, body)
	}
	return []byte(`{"message":"Comment created.","comment":{"id":"11111111-1111-1111-1111-111111111111","createdOn":"2026-01-01T00:00:00Z","createdBy":"user@example.com"}}`), nil
}

func (m *mockEntityIncidentClient) SearchComments(ctx context.Context, body []byte) ([]byte, error) {
	if m.searchCommentsFn != nil {
		return m.searchCommentsFn(ctx, body)
	}
	return []byte(`{"comments":[],"total":0,"limit":20,"offset":0}`), nil
}

func (m *mockEntityIncidentClient) SearchIncidentActivities(ctx context.Context, id string, body []byte) ([]byte, error) {
	if m.searchIncidentActivitiesFn != nil {
		return m.searchIncidentActivitiesFn(ctx, id, body)
	}
	return []byte(`{"activity":[],"total":0,"limit":20,"offset":0,"hasMore":false}`), nil
}

func (m *mockEntityIncidentClient) HandOffIncidentToSpecialist(ctx context.Context, id string, body []byte) ([]byte, error) {
	if m.handOffIncidentFn != nil {
		return m.handOffIncidentFn(ctx, id, body)
	}
	return []byte(`{"message":"Incident handed off to specialist group","handoff":{"assignmentGroup":{"id":"11111111-1111-1111-1111-111111111111","name":"Specialist Group"},"previousAssignmentGroup":null,"reasonCode":"no-runbook","reasonDescription":"Runbook is not available","escalationTeam":null,"task":{"id":"22222222-2222-2222-2222-222222222222","number":"TASK0000001","subject":"[Runbook Task] test"},"githubIssue":{"url":"https://github.com/example/repo/issues/1","number":1,"repo":"repo"},"githubIssueError":null,"incident":{}}}`), nil
}

// ----- mock entity problem client -----

type mockEntityProblemClient struct {
	searchProblemsFn    func(ctx context.Context, body []byte) ([]byte, error)
	aggregateProblemsFn func(ctx context.Context, body []byte) ([]byte, error)
	getProblemFn        func(ctx context.Context, id string) ([]byte, error)
	createProblemFn     func(ctx context.Context, body []byte) ([]byte, error)
	updateProblemFn     func(ctx context.Context, id string, body []byte) ([]byte, error)
}

func (m *mockEntityProblemClient) SearchProblems(ctx context.Context, body []byte) ([]byte, error) {
	if m.searchProblemsFn != nil {
		return m.searchProblemsFn(ctx, body)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityProblemClient) AggregateProblems(ctx context.Context, body []byte) ([]byte, error) {
	if m.aggregateProblemsFn != nil {
		return m.aggregateProblemsFn(ctx, body)
	}
	return []byte(`{"groups":[],"othersCount":0,"totalRecords":0}`), nil
}

func (m *mockEntityProblemClient) GetProblem(ctx context.Context, id string) ([]byte, error) {
	if m.getProblemFn != nil {
		return m.getProblemFn(ctx, id)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityProblemClient) CreateProblem(ctx context.Context, body []byte) ([]byte, error) {
	if m.createProblemFn != nil {
		return m.createProblemFn(ctx, body)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityProblemClient) UpdateProblem(ctx context.Context, id string, body []byte) ([]byte, error) {
	if m.updateProblemFn != nil {
		return m.updateProblemFn(ctx, id, body)
	}
	return []byte(`{}`), nil
}

// ----- mock entity alert client -----

type mockEntityAlertClient struct {
	getAlertFn      func(ctx context.Context, id string) ([]byte, error)
	getSmartAlertFn func(ctx context.Context, id string) ([]byte, error)
}

func (m *mockEntityAlertClient) GetAlert(ctx context.Context, id string) ([]byte, error) {
	if m.getAlertFn != nil {
		return m.getAlertFn(ctx, id)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityAlertClient) GetSmartAlert(ctx context.Context, id string) ([]byte, error) {
	if m.getSmartAlertFn != nil {
		return m.getSmartAlertFn(ctx, id)
	}
	return []byte(`{}`), nil
}

// ----- mock entity incident task client -----

type mockEntityIncidentTaskClient struct {
	searchIncidentTasksFn    func(ctx context.Context, body []byte) ([]byte, error)
	aggregateIncidentTasksFn func(ctx context.Context, body []byte) ([]byte, error)
	getIncidentTaskFn        func(ctx context.Context, id string) ([]byte, error)
}

func (m *mockEntityIncidentTaskClient) SearchIncidentTasks(ctx context.Context, body []byte) ([]byte, error) {
	if m.searchIncidentTasksFn != nil {
		return m.searchIncidentTasksFn(ctx, body)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityIncidentTaskClient) AggregateIncidentTasks(ctx context.Context, body []byte) ([]byte, error) {
	if m.aggregateIncidentTasksFn != nil {
		return m.aggregateIncidentTasksFn(ctx, body)
	}
	return []byte(`{"groups":[],"othersCount":0,"totalRecords":0}`), nil
}

func (m *mockEntityIncidentTaskClient) GetIncidentTask(ctx context.Context, id string) ([]byte, error) {
	if m.getIncidentTaskFn != nil {
		return m.getIncidentTaskFn(ctx, id)
	}
	return []byte(`{}`), nil
}

// ----- mock entity change request client -----

type mockEntityChangeRequestClient struct {
	createChangeRequestFn         func(ctx context.Context, body []byte) ([]byte, error)
	searchChangeRequestsFn        func(ctx context.Context, body []byte) ([]byte, error)
	aggregateChangeRequestsFn     func(ctx context.Context, body []byte) ([]byte, error)
	getChangeRequestFn            func(ctx context.Context, id string) ([]byte, error)
	patchChangeRequestFn          func(ctx context.Context, id string, body []byte) ([]byte, error)
	getChangeRequestApprovalsFn   func(ctx context.Context, id string) ([]byte, error)
	createCommentFn               func(ctx context.Context, body []byte) ([]byte, error)
	searchCommentsFn              func(ctx context.Context, body []byte) ([]byte, error)
	decideChangeRequestApprovalFn func(ctx context.Context, id string, body []byte) ([]byte, error)
}

func (m *mockEntityChangeRequestClient) CreateChangeRequest(ctx context.Context, body []byte) ([]byte, error) {
	if m.createChangeRequestFn != nil {
		return m.createChangeRequestFn(ctx, body)
	}
	return []byte(`{"message":"Change request created.","changeRequest":{"id":"11111111-1111-1111-1111-111111111111","number":"CHG0001","createdOn":"2026-01-01T00:00:00Z","createdBy":"user@example.com"}}`), nil
}

func (m *mockEntityChangeRequestClient) SearchChangeRequests(ctx context.Context, body []byte) ([]byte, error) {
	if m.searchChangeRequestsFn != nil {
		return m.searchChangeRequestsFn(ctx, body)
	}
	return []byte(`{"changeRequests":[],"total":0,"limit":20,"offset":0}`), nil
}

func (m *mockEntityChangeRequestClient) AggregateChangeRequests(ctx context.Context, body []byte) ([]byte, error) {
	if m.aggregateChangeRequestsFn != nil {
		return m.aggregateChangeRequestsFn(ctx, body)
	}
	return []byte(`{"groups":[],"othersCount":0,"totalRecords":0}`), nil
}

func (m *mockEntityChangeRequestClient) GetChangeRequest(ctx context.Context, id string) ([]byte, error) {
	if m.getChangeRequestFn != nil {
		return m.getChangeRequestFn(ctx, id)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityChangeRequestClient) PatchChangeRequest(ctx context.Context, id string, body []byte) ([]byte, error) {
	if m.patchChangeRequestFn != nil {
		return m.patchChangeRequestFn(ctx, id, body)
	}
	return []byte(`{"id":"11111111-1111-1111-1111-111111111111","updatedOn":"2026-01-01T00:00:00Z","updatedBy":"user@example.com"}`), nil
}

func (m *mockEntityChangeRequestClient) GetChangeRequestApprovals(ctx context.Context, id string) ([]byte, error) {
	if m.getChangeRequestApprovalsFn != nil {
		return m.getChangeRequestApprovalsFn(ctx, id)
	}
	return []byte(`{"approvals":[]}`), nil
}

func (m *mockEntityChangeRequestClient) CreateComment(ctx context.Context, body []byte) ([]byte, error) {
	if m.createCommentFn != nil {
		return m.createCommentFn(ctx, body)
	}
	return []byte(`{"message":"Comment created.","comment":{"id":"11111111-1111-1111-1111-111111111111","createdOn":"2026-01-01T00:00:00Z","createdBy":"user@example.com"}}`), nil
}

func (m *mockEntityChangeRequestClient) SearchComments(ctx context.Context, body []byte) ([]byte, error) {
	if m.searchCommentsFn != nil {
		return m.searchCommentsFn(ctx, body)
	}
	return []byte(`{"comments":[],"total":0,"limit":20,"offset":0}`), nil
}

func (m *mockEntityChangeRequestClient) DecideChangeRequestApproval(ctx context.Context, id string, body []byte) ([]byte, error) {
	if m.decideChangeRequestApprovalFn != nil {
		return m.decideChangeRequestApprovalFn(ctx, id, body)
	}
	return []byte(`{"id":"11111111-1111-1111-1111-111111111111","state":"approved"}`), nil
}

// ----- mock entity outage client -----

type mockEntityOutageClient struct {
	createOutageFn               func(ctx context.Context, body []byte) ([]byte, error)
	searchOutagesFn              func(ctx context.Context, body []byte) ([]byte, error)
	getOutageFn                  func(ctx context.Context, id string) ([]byte, error)
	patchOutageFn                func(ctx context.Context, id string, body []byte) ([]byte, error)
	addOutageCommunicationFn     func(ctx context.Context, id string, body []byte) ([]byte, error)
	searchOutageCommunicationsFn func(ctx context.Context, id string, body []byte) ([]byte, error)
	getOutageMetadataFn          func(ctx context.Context) ([]byte, error)
}

func (m *mockEntityOutageClient) CreateOutage(ctx context.Context, body []byte) ([]byte, error) {
	if m.createOutageFn != nil {
		return m.createOutageFn(ctx, body)
	}
	return []byte(`{"message":"Outage created successfully","outage":{"id":"11111111-1111-1111-1111-111111111111","number":"OUT0001881"}}`), nil
}

func (m *mockEntityOutageClient) SearchOutages(ctx context.Context, body []byte) ([]byte, error) {
	if m.searchOutagesFn != nil {
		return m.searchOutagesFn(ctx, body)
	}
	return []byte(`{"outages":[],"total":0,"limit":20,"offset":0,"appliedBeginFrom":"","beginFromDefaulted":false}`), nil
}

func (m *mockEntityOutageClient) GetOutage(ctx context.Context, id string) ([]byte, error) {
	if m.getOutageFn != nil {
		return m.getOutageFn(ctx, id)
	}
	return []byte(`{"id":"` + id + `"}`), nil
}

func (m *mockEntityOutageClient) PatchOutage(ctx context.Context, id string, body []byte) ([]byte, error) {
	if m.patchOutageFn != nil {
		return m.patchOutageFn(ctx, id, body)
	}
	return []byte(`{"message":"Outage updated successfully","outage":{"id":"` + id + `"}}`), nil
}

func (m *mockEntityOutageClient) AddOutageCommunication(ctx context.Context, id string, body []byte) ([]byte, error) {
	if m.addOutageCommunicationFn != nil {
		return m.addOutageCommunicationFn(ctx, id, body)
	}
	return []byte(`{"message":"Communication added successfully","communication":{"id":"11111111-1111-1111-1111-111111111111"}}`), nil
}

func (m *mockEntityOutageClient) SearchOutageCommunications(ctx context.Context, id string, body []byte) ([]byte, error) {
	if m.searchOutageCommunicationsFn != nil {
		return m.searchOutageCommunicationsFn(ctx, id, body)
	}
	return []byte(`{"communications":[],"total":0,"limit":20,"offset":0}`), nil
}

func (m *mockEntityOutageClient) GetOutageMetadata(ctx context.Context) ([]byte, error) {
	if m.getOutageMetadataFn != nil {
		return m.getOutageMetadataFn(ctx)
	}
	return []byte(`{"types":[],"statuses":[],"communicationChannels":[],"statusPageClouds":[]}`), nil
}

// ----- mock entity IT service client -----

type mockEntityITServiceClient struct {
	searchITServicesFn func(ctx context.Context, body []byte) ([]byte, error)
}

func (m *mockEntityITServiceClient) SearchITServices(ctx context.Context, body []byte) ([]byte, error) {
	if m.searchITServicesFn != nil {
		return m.searchITServicesFn(ctx, body)
	}
	return []byte(`{"services":[],"total":0,"limit":20,"offset":0}`), nil
}

// ----- mock entity service offering client -----

type mockEntityServiceOfferingClient struct {
	searchServiceOfferingsFn func(ctx context.Context, body []byte) ([]byte, error)
}

func (m *mockEntityServiceOfferingClient) SearchServiceOfferings(ctx context.Context, body []byte) ([]byte, error) {
	if m.searchServiceOfferingsFn != nil {
		return m.searchServiceOfferingsFn(ctx, body)
	}
	return []byte(`{"serviceOfferings":[],"total":0,"limit":20,"offset":0}`), nil
}

// ----- mock entity group client -----

type mockEntityGroupClient struct {
	searchGroupsFn func(ctx context.Context, body []byte) ([]byte, error)
}

func (m *mockEntityGroupClient) SearchGroups(ctx context.Context, body []byte) ([]byte, error) {
	if m.searchGroupsFn != nil {
		return m.searchGroupsFn(ctx, body)
	}
	return []byte(`{"groups":[],"total":0,"limit":20,"offset":0}`), nil
}

// ----- mock entity configuration item client -----

type mockEntityConfigurationItemClient struct {
	searchConfigurationItemsFn func(ctx context.Context, body []byte) ([]byte, error)
}

func (m *mockEntityConfigurationItemClient) SearchConfigurationItems(ctx context.Context, body []byte) ([]byte, error) {
	if m.searchConfigurationItemsFn != nil {
		return m.searchConfigurationItemsFn(ctx, body)
	}
	return []byte(`{"configurationItems":[],"total":0,"limit":20,"offset":0}`), nil
}

// ----- mock entity time-card client -----

type mockEntityTimeCardClient struct {
	searchTimeCardsFn func(ctx context.Context, body []byte) ([]byte, error)
	createTimeCardFn  func(ctx context.Context, body []byte) ([]byte, error)
	updateTimeCardFn  func(ctx context.Context, id string, body []byte) ([]byte, error)
	deleteTimeCardFn  func(ctx context.Context, id string) ([]byte, error)
}

func (m *mockEntityTimeCardClient) SearchTimeCards(ctx context.Context, body []byte) ([]byte, error) {
	if m.searchTimeCardsFn != nil {
		return m.searchTimeCardsFn(ctx, body)
	}
	return []byte(`{"timeCards":[],"total":0,"limit":20,"offset":0}`), nil
}

func (m *mockEntityTimeCardClient) CreateTimeCard(ctx context.Context, body []byte) ([]byte, error) {
	if m.createTimeCardFn != nil {
		return m.createTimeCardFn(ctx, body)
	}
	return []byte(`{"timeCard":{"id":"11111111-1111-1111-1111-111111111111","state":"submitted"}}`), nil
}

func (m *mockEntityTimeCardClient) UpdateTimeCard(ctx context.Context, id string, body []byte) ([]byte, error) {
	if m.updateTimeCardFn != nil {
		return m.updateTimeCardFn(ctx, id, body)
	}
	return []byte(`{"timeCard":{"id":"` + id + `","state":"submitted"}}`), nil
}

func (m *mockEntityTimeCardClient) DeleteTimeCard(ctx context.Context, id string) ([]byte, error) {
	if m.deleteTimeCardFn != nil {
		return m.deleteTimeCardFn(ctx, id)
	}
	return []byte(`{"message":"Time card deleted"}`), nil
}

// ----- mock entity deployment client -----

type mockEntityDeploymentClient struct {
	postDeploymentFn         func(ctx context.Context, body []byte) ([]byte, error)
	searchDeploymentsFn      func(ctx context.Context, body []byte) ([]byte, error)
	searchDeployedProductsFn func(ctx context.Context, body []byte) ([]byte, error)
	patchDeploymentFn        func(ctx context.Context, deploymentID string, body []byte) ([]byte, error)
	postDeployedProductFn    func(ctx context.Context, body []byte) ([]byte, error)
	patchDeployedProductFn   func(ctx context.Context, deployedProductID string, body []byte) ([]byte, error)
}

func (m *mockEntityDeploymentClient) PostDeployment(ctx context.Context, body []byte) ([]byte, error) {
	if m.postDeploymentFn != nil {
		return m.postDeploymentFn(ctx, body)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityDeploymentClient) SearchDeployments(ctx context.Context, body []byte) ([]byte, error) {
	if m.searchDeploymentsFn != nil {
		return m.searchDeploymentsFn(ctx, body)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityDeploymentClient) SearchDeployedProducts(ctx context.Context, body []byte) ([]byte, error) {
	if m.searchDeployedProductsFn != nil {
		return m.searchDeployedProductsFn(ctx, body)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityDeploymentClient) PatchDeployment(ctx context.Context, deploymentID string, body []byte) ([]byte, error) {
	if m.patchDeploymentFn != nil {
		return m.patchDeploymentFn(ctx, deploymentID, body)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityDeploymentClient) PostDeployedProduct(ctx context.Context, body []byte) ([]byte, error) {
	if m.postDeployedProductFn != nil {
		return m.postDeployedProductFn(ctx, body)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityDeploymentClient) PatchDeployedProduct(ctx context.Context, deployedProductID string, body []byte) ([]byte, error) {
	if m.patchDeployedProductFn != nil {
		return m.patchDeployedProductFn(ctx, deployedProductID, body)
	}
	return []byte(`{}`), nil
}

// ----- mock entity conversation client -----

type mockEntityConversationClient struct {
	searchCommentsFn      func(ctx context.Context, body []byte) ([]byte, error)
	searchConversationsFn func(ctx context.Context, body []byte) ([]byte, error)
}

func (m *mockEntityConversationClient) SearchComments(ctx context.Context, body []byte) ([]byte, error) {
	if m.searchCommentsFn != nil {
		return m.searchCommentsFn(ctx, body)
	}
	return []byte(`{"comments":[],"total":0,"limit":20,"offset":0,"hasMore":false}`), nil
}

func (m *mockEntityConversationClient) SearchConversations(ctx context.Context, body []byte) ([]byte, error) {
	if m.searchConversationsFn != nil {
		return m.searchConversationsFn(ctx, body)
	}
	return []byte(`{}`), nil
}

// ----- mock entity task SLA client -----

type mockEntityTaskSlaClient struct {
	searchTaskSlasFn func(ctx context.Context, body []byte) ([]byte, error)
	getTaskSlaFn     func(ctx context.Context, id string) ([]byte, error)
}

func (m *mockEntityTaskSlaClient) SearchTaskSlas(ctx context.Context, body []byte) ([]byte, error) {
	if m.searchTaskSlasFn != nil {
		return m.searchTaskSlasFn(ctx, body)
	}
	return []byte(`{"slas":[],"total":0,"limit":20,"offset":0}`), nil
}

func (m *mockEntityTaskSlaClient) GetTaskSla(ctx context.Context, id string) ([]byte, error) {
	if m.getTaskSlaFn != nil {
		return m.getTaskSlaFn(ctx, id)
	}
	return []byte(`{"id":"11111111-1111-1111-1111-111111111111"}`), nil
}

// ----- mock entity task client -----

type mockEntityTaskClient struct {
	searchCaseTasksFn func(ctx context.Context, caseID string, body []byte) ([]byte, error)
	searchTasksFn     func(ctx context.Context, body []byte) ([]byte, error)
	getTaskFn         func(ctx context.Context, id string) ([]byte, error)
	createCaseTaskFn  func(ctx context.Context, caseID string, body []byte) ([]byte, error)
	updateTaskFn      func(ctx context.Context, id string, body []byte) ([]byte, error)
}

func (m *mockEntityTaskClient) SearchCaseTasks(ctx context.Context, caseID string, body []byte) ([]byte, error) {
	if m.searchCaseTasksFn != nil {
		return m.searchCaseTasksFn(ctx, caseID, body)
	}
	return []byte(`{"tasks":[],"total":0,"limit":20,"offset":0}`), nil
}

func (m *mockEntityTaskClient) SearchTasks(ctx context.Context, body []byte) ([]byte, error) {
	if m.searchTasksFn != nil {
		return m.searchTasksFn(ctx, body)
	}
	return []byte(`{"tasks":[],"total":0,"limit":20,"offset":0}`), nil
}

func (m *mockEntityTaskClient) GetTask(ctx context.Context, id string) ([]byte, error) {
	if m.getTaskFn != nil {
		return m.getTaskFn(ctx, id)
	}
	return []byte(`{"id":"11111111-1111-1111-1111-111111111111"}`), nil
}

func (m *mockEntityTaskClient) CreateCaseTask(ctx context.Context, caseID string, body []byte) ([]byte, error) {
	if m.createCaseTaskFn != nil {
		return m.createCaseTaskFn(ctx, caseID, body)
	}
	return []byte(`{"id":"11111111-1111-1111-1111-111111111111"}`), nil
}

func (m *mockEntityTaskClient) UpdateTask(ctx context.Context, id string, body []byte) ([]byte, error) {
	if m.updateTaskFn != nil {
		return m.updateTaskFn(ctx, id, body)
	}
	return []byte(`{"id":"11111111-1111-1111-1111-111111111111"}`), nil
}
