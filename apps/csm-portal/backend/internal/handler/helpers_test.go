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

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/scim"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/updates"
)

// testUser is the authenticated user injected into request contexts.
var testUser = &middleware.UserInfo{
	Email:  "agent@example.com",
	UserID: "user-123",
	Groups: []string{"csm-agents"},
}

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
	searchCaseCommentsFn       func(ctx context.Context, caseID string, body []byte) ([]byte, error)
	searchCasesFn              func(ctx context.Context, body []byte) ([]byte, error)
	getCaseFn                  func(ctx context.Context, caseID string) ([]byte, error)
	createCaseAttachmentFn     func(ctx context.Context, caseID string, body []byte) ([]byte, error)
	searchCaseAttachmentsFn    func(ctx context.Context, caseID string, body []byte) ([]byte, error)
	getCaseAttachmentContentFn func(ctx context.Context, caseID, attachmentID string) ([]byte, string, error)
	createCallRequestFn        func(ctx context.Context, body []byte) ([]byte, error)
	searchCallRequestsFn       func(ctx context.Context, body []byte) ([]byte, error)
	patchCallRequestFn         func(ctx context.Context, callRequestID string, body []byte) ([]byte, error)
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

func (m *mockEntityCaseClient) SearchCaseComments(ctx context.Context, caseID string, body []byte) ([]byte, error) {
	if m.searchCaseCommentsFn != nil {
		return m.searchCaseCommentsFn(ctx, caseID, body)
	}
	return []byte(`{"comments":[],"total":0,"limit":20,"offset":0,"hasMore":false}`), nil
}

func (m *mockEntityCaseClient) SearchCases(ctx context.Context, body []byte) ([]byte, error) {
	if m.searchCasesFn != nil {
		return m.searchCasesFn(ctx, body)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityCaseClient) GetCase(ctx context.Context, caseID string) ([]byte, error) {
	if m.getCaseFn != nil {
		return m.getCaseFn(ctx, caseID)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityCaseClient) CreateCaseAttachment(ctx context.Context, caseID string, body []byte) ([]byte, error) {
	if m.createCaseAttachmentFn != nil {
		return m.createCaseAttachmentFn(ctx, caseID, body)
	}
	return []byte(`{}`), nil
}

func (m *mockEntityCaseClient) SearchCaseAttachments(ctx context.Context, caseID string, body []byte) ([]byte, error) {
	if m.searchCaseAttachmentsFn != nil {
		return m.searchCaseAttachmentsFn(ctx, caseID, body)
	}
	return []byte(`{"attachments":[],"total":0,"limit":20,"offset":0,"hasMore":false}`), nil
}

func (m *mockEntityCaseClient) GetCaseAttachmentContent(ctx context.Context, caseID, attachmentID string) ([]byte, string, error) {
	if m.getCaseAttachmentContentFn != nil {
		return m.getCaseAttachmentContentFn(ctx, caseID, attachmentID)
	}
	return []byte(`fake-content`), "image/png", nil
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

func (m *mockEntityCaseClient) PatchCallRequest(ctx context.Context, callRequestID string, body []byte) ([]byte, error) {
	if m.patchCallRequestFn != nil {
		return m.patchCallRequestFn(ctx, callRequestID, body)
	}
	return []byte(`{}`), nil
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
	searchUserFn      func(ctx context.Context, email string) (*scim.UserInfo, error)
	updateUserPhoneFn func(ctx context.Context, userID, mobile string) (*string, error)
}

func (m *mockSCIMClient) SearchUser(ctx context.Context, email string) (*scim.UserInfo, error) {
	if m.searchUserFn != nil {
		return m.searchUserFn(ctx, email)
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
	searchUsersFn func(ctx context.Context, body []byte) ([]byte, error)
}

func (m *mockEntityUserClient) SearchUsers(ctx context.Context, body []byte) ([]byte, error) {
	if m.searchUsersFn != nil {
		return m.searchUsersFn(ctx, body)
	}
	return []byte(`{}`), nil
}

// ----- mock entity account client -----

type mockEntityAccountClient struct {
	getAccountFn     func(ctx context.Context, id string) ([]byte, error)
	searchAccountsFn func(ctx context.Context, body []byte) ([]byte, error)
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

// ----- mock entity project client -----

type mockEntityProjectClient struct {
	getProjectFn     func(ctx context.Context, id string) ([]byte, error)
	searchProjectsFn func(ctx context.Context, body []byte) ([]byte, error)
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

// ----- mock entity change request client -----

type mockEntityChangeRequestClient struct {
	searchChangeRequestsFn func(ctx context.Context, body []byte) ([]byte, error)
	getChangeRequestFn     func(ctx context.Context, id string) ([]byte, error)
}

func (m *mockEntityChangeRequestClient) SearchChangeRequests(ctx context.Context, body []byte) ([]byte, error) {
	if m.searchChangeRequestsFn != nil {
		return m.searchChangeRequestsFn(ctx, body)
	}
	return []byte(`{"changeRequests":[],"total":0,"limit":20,"offset":0}`), nil
}

func (m *mockEntityChangeRequestClient) GetChangeRequest(ctx context.Context, id string) ([]byte, error) {
	if m.getChangeRequestFn != nil {
		return m.getChangeRequestFn(ctx, id)
	}
	return []byte(`{}`), nil
}

// ----- mock entity deployment client -----

type mockEntityDeploymentClient struct {
	postDeploymentFn          func(ctx context.Context, body []byte) ([]byte, error)
	searchDeploymentsFn       func(ctx context.Context, body []byte) ([]byte, error)
	searchDeployedProductsFn  func(ctx context.Context, body []byte) ([]byte, error)
	patchDeploymentFn         func(ctx context.Context, deploymentID string, body []byte) ([]byte, error)
	postDeployedProductFn     func(ctx context.Context, body []byte) ([]byte, error)
	patchDeployedProductFn    func(ctx context.Context, deployedProductID string, body []byte) ([]byte, error)
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
