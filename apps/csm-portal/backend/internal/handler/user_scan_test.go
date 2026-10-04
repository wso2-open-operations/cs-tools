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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/entity"
	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/middleware"
)

func scanTestRequest(t *testing.T, payload UserScanRequest) *http.Request {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return withViewerWriterUser(httptest.NewRequest(http.MethodPost, "/spl/scan-user", bytes.NewReader(body)))
}

// scanUsersResponse builds a SearchUsers response body with a single user.
func scanUsersResponse(email string, lockedOut bool) []byte {
	return []byte(fmt.Sprintf(`{"users":[{"email":%q,"lockedOut":%t}]}`, email, lockedOut))
}

// scanProjectsResponse builds a SearchProjects response body with a single
// project, keyed so lookupProjectByKey's exact-match filter finds it.
func scanProjectsResponse(id, key, closureState string) []byte {
	return []byte(fmt.Sprintf(`{"projects":[{"id":%q,"key":%q,"closureState":%q}]}`, id, key, closureState))
}

func TestSplScanUser_AuthGates(t *testing.T) {
	h := NewSplUserScanHandler(&mockSalesEntityClient{}, &mockEntityScanClient{}, viewerAccessGuard)

	t.Run("requires authenticated user", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodPost, "/spl/scan-user", bytes.NewReader([]byte(`{}`)))
		w := httptest.NewRecorder()
		h.ScanUser(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("rejects a role that doesn't grant PermViewerAccess", func(t *testing.T) {
		h2 := NewSplUserScanHandler(&mockSalesEntityClient{}, &mockEntityScanClient{}, viewerAccessGuard)
		body, err := json.Marshal(UserScanRequest{Email: "a@b.com"})
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}
		r := httptest.NewRequest(http.MethodPost, "/spl/scan-user", bytes.NewReader(body))
		// Authenticated but holds no role granting PermViewerAccess.
		r = r.WithContext(middleware.WithUserInfo(r.Context(), &middleware.UserInfo{Email: "nobody@example.com", UserID: "u-nobody"}))
		w := httptest.NewRecorder()
		h2.ScanUser(w, r)
		assertStatus(t, w, http.StatusForbidden)
	})

	t.Run("rejects a viewer without write", func(t *testing.T) {
		r := withUser(httptest.NewRequest(http.MethodPost, "/spl/scan-user", bytes.NewReader([]byte(`{"email":"a@b.com","subscriptionKey":"k"}`))))
		w := httptest.NewRecorder()
		h.ScanUser(w, r)
		assertStatus(t, w, http.StatusForbidden)
	})

	t.Run("rejects malformed JSON body", func(t *testing.T) {
		r := withViewerWriterUser(httptest.NewRequest(http.MethodPost, "/spl/scan-user", bytes.NewReader([]byte(`{not json`))))
		w := httptest.NewRecorder()
		h.ScanUser(w, r)
		assertStatus(t, w, http.StatusBadRequest)
	})

	t.Run("rejects an empty or whitespace-only email or subscriptionKey before any upstream lookup", func(t *testing.T) {
		h2 := NewSplUserScanHandler(
			&mockSalesEntityClient{
				getContactByEmailFn: func(context.Context, string) (*entity.Contact, error) {
					t.Fatal("sales entity should not be called when required fields are missing")
					return nil, nil
				},
			},
			&mockEntityScanClient{}, viewerAccessGuard)
		tests := []UserScanRequest{
			{Email: "", SubscriptionKey: "sub-1"},
			{Email: "a@b.com", SubscriptionKey: ""},
			{Email: "   ", SubscriptionKey: "sub-1"},
			{Email: "a@b.com", SubscriptionKey: "  "},
		}
		for _, payload := range tests {
			w := httptest.NewRecorder()
			h2.ScanUser(w, scanTestRequest(t, payload))
			assertStatus(t, w, http.StatusBadRequest)
		}
	})
}

func TestSplScanUser_SalesforceSide(t *testing.T) {
	// Every sub-test only exercises the Salesforce branches; the entity-
	// service side is given a "found, active, open" shape throughout so its
	// results are constant and don't obscure what's under test.
	neutralEntity := &mockEntityScanClient{
		searchUsersFn: func(ctx context.Context, body []byte) ([]byte, error) {
			return scanUsersResponse("a@b.com", false), nil
		},
		searchProjectsFn: func(ctx context.Context, body []byte) ([]byte, error) {
			return scanProjectsResponse("proj-1", "key-1", "Open"), nil
		},
	}

	t.Run("contact not found, subscription not found", func(t *testing.T) {
		sales := &mockSalesEntityClient{}
		h := NewSplUserScanHandler(sales, neutralEntity, viewerAccessGuard)
		r := scanTestRequest(t, UserScanRequest{Email: "nobody@example.com", SubscriptionKey: "key-1"})
		w := httptest.NewRecorder()
		h.ScanUser(w, r)
		assertStatus(t, w, http.StatusOK)
		resp := decodeJSON[[]ScanResponse](t, w)
		sf := resp[0]
		if sf.System != "Salesforce" {
			t.Fatalf("system = %q, want Salesforce", sf.System)
		}
		if sf.SystemResult[0].State {
			t.Error("contact result should not be success")
		}
		if sf.SystemResult[0].Information != infoContactNotFound {
			t.Errorf("contact information = %+v, want ContactNotFound", sf.SystemResult[0].Information)
		}
		if sf.SystemResult[1].Information != infoSubscriptionNotFound {
			t.Errorf("subscription information = %+v, want SubscriptionNotFound", sf.SystemResult[1].Information)
		}
		if sf.SystemResult[2].Information != infoMembershipNotFound {
			t.Errorf("membership information = %+v, want MembershipNotFound", sf.SystemResult[2].Information)
		}
	})

	t.Run("contact found with no memberships", func(t *testing.T) {
		sales := &mockSalesEntityClient{
			getContactByEmailFn: func(ctx context.Context, email string) (*entity.Contact, error) {
				return &entity.Contact{ID: "c1", Account: struct {
					ID             string `json:"id"`
					Name           string `json:"name"`
					Classification string `json:"classification"`
				}{ID: "acct-1"}}, nil
			},
			getSubscriptionByKeyFn: func(ctx context.Context, subscriptionKey string) (*entity.Subscription, error) {
				return &entity.Subscription{ID: "sub-1", CustomerID: "acct-1"}, nil
			},
		}
		h := NewSplUserScanHandler(sales, neutralEntity, viewerAccessGuard)
		r := scanTestRequest(t, UserScanRequest{Email: "a@b.com", SubscriptionKey: "key-1"})
		w := httptest.NewRecorder()
		h.ScanUser(w, r)
		resp := decodeJSON[[]ScanResponse](t, w)
		sf := resp[0]
		if !sf.SystemResult[0].State {
			t.Error("contact result should be success")
		}
		if sf.SystemResult[2].Information != infoMembershipNotFoundInSubscription {
			t.Errorf("membership information = %+v, want MembershipNotFoundInSubscription", sf.SystemResult[2].Information)
		}
	})

	t.Run("valid customer membership succeeds", func(t *testing.T) {
		sales := &mockSalesEntityClient{
			getContactByEmailFn: func(ctx context.Context, email string) (*entity.Contact, error) {
				return &entity.Contact{
					ID: "c1",
					Account: struct {
						ID             string `json:"id"`
						Name           string `json:"name"`
						Classification string `json:"classification"`
					}{ID: "acct-1", Classification: "Enterprise"},
					Memberships: []entity.ContactMembership{{SubscriptionID: "sub-1", Type: membershipTypeCustomer}},
				}, nil
			},
			getSubscriptionByKeyFn: func(ctx context.Context, subscriptionKey string) (*entity.Subscription, error) {
				return &entity.Subscription{ID: "sub-1", CustomerID: "acct-1"}, nil
			},
		}
		h := NewSplUserScanHandler(sales, neutralEntity, viewerAccessGuard)
		r := scanTestRequest(t, UserScanRequest{Email: "a@b.com", SubscriptionKey: "key-1", IsPartner: false})
		w := httptest.NewRecorder()
		h.ScanUser(w, r)
		resp := decodeJSON[[]ScanResponse](t, w)
		sf := resp[0]
		if !sf.SystemResult[2].State {
			t.Errorf("membership result should be success, got %+v", sf.SystemResult[2])
		}
	})

	t.Run("customer membership on a partner account is invalid", func(t *testing.T) {
		sales := &mockSalesEntityClient{
			getContactByEmailFn: func(ctx context.Context, email string) (*entity.Contact, error) {
				return &entity.Contact{
					ID: "c1",
					Account: struct {
						ID             string `json:"id"`
						Name           string `json:"name"`
						Classification string `json:"classification"`
					}{ID: "acct-1", Classification: accountClassificationPartner},
					Memberships: []entity.ContactMembership{{SubscriptionID: "sub-1", Type: membershipTypeCustomer}},
				}, nil
			},
			getSubscriptionByKeyFn: func(ctx context.Context, subscriptionKey string) (*entity.Subscription, error) {
				return &entity.Subscription{ID: "sub-1", CustomerID: "acct-1"}, nil
			},
		}
		h := NewSplUserScanHandler(sales, neutralEntity, viewerAccessGuard)
		// isPartner=true but membership type is CUSTOMER -> invalid on a partner account.
		r := scanTestRequest(t, UserScanRequest{Email: "a@b.com", SubscriptionKey: "key-1", IsPartner: true})
		w := httptest.NewRecorder()
		h.ScanUser(w, r)
		resp := decodeJSON[[]ScanResponse](t, w)
		sf := resp[0]
		if sf.SystemResult[2].State {
			t.Error("membership result should not be success")
		}
		if sf.SystemResult[2].Information != infoInvalidMembership {
			t.Errorf("membership information = %+v, want InvalidMembership", sf.SystemResult[2].Information)
		}
	})

	t.Run("partner membership on a non-partner account is invalid", func(t *testing.T) {
		sales := &mockSalesEntityClient{
			getContactByEmailFn: func(ctx context.Context, email string) (*entity.Contact, error) {
				return &entity.Contact{
					ID: "c1",
					Account: struct {
						ID             string `json:"id"`
						Name           string `json:"name"`
						Classification string `json:"classification"`
					}{ID: "acct-1", Classification: "Enterprise"},
					Memberships: []entity.ContactMembership{{SubscriptionID: "sub-1", Type: membershipTypePartner}},
				}, nil
			},
			getSubscriptionByKeyFn: func(ctx context.Context, subscriptionKey string) (*entity.Subscription, error) {
				return &entity.Subscription{ID: "sub-1", CustomerID: "acct-1"}, nil
			},
		}
		h := NewSplUserScanHandler(sales, neutralEntity, viewerAccessGuard)
		r := scanTestRequest(t, UserScanRequest{Email: "a@b.com", SubscriptionKey: "key-1", IsPartner: false})
		w := httptest.NewRecorder()
		h.ScanUser(w, r)
		resp := decodeJSON[[]ScanResponse](t, w)
		sf := resp[0]
		if sf.SystemResult[2].Information != infoInvalidCustomerMembership {
			t.Errorf("membership information = %+v, want InvalidCustomerMembership", sf.SystemResult[2].Information)
		}
	})

	t.Run("subscription and contact in different accounts", func(t *testing.T) {
		sales := &mockSalesEntityClient{
			getContactByEmailFn: func(ctx context.Context, email string) (*entity.Contact, error) {
				return &entity.Contact{
					ID: "c1",
					Account: struct {
						ID             string `json:"id"`
						Name           string `json:"name"`
						Classification string `json:"classification"`
					}{ID: "acct-1"},
					Memberships: []entity.ContactMembership{{SubscriptionID: "sub-1", Type: membershipTypeCustomer}},
				}, nil
			},
			getSubscriptionByKeyFn: func(ctx context.Context, subscriptionKey string) (*entity.Subscription, error) {
				return &entity.Subscription{ID: "sub-1", CustomerID: "acct-DIFFERENT"}, nil
			},
		}
		h := NewSplUserScanHandler(sales, neutralEntity, viewerAccessGuard)
		r := scanTestRequest(t, UserScanRequest{Email: "a@b.com", SubscriptionKey: "key-1", IsPartner: false})
		w := httptest.NewRecorder()
		h.ScanUser(w, r)
		resp := decodeJSON[[]ScanResponse](t, w)
		sf := resp[0]
		if sf.SystemResult[1].Information != infoSubscriptionNotFoundInAccount {
			t.Errorf("subscription information = %+v, want SubscriptionNotFoundInAccount", sf.SystemResult[1].Information)
		}
	})
}

func TestSplScanUser_EntityServiceSide(t *testing.T) {
	neutralSales := &mockSalesEntityClient{}

	t.Run("project not found", func(t *testing.T) {
		ent := &mockEntityScanClient{
			searchProjectsFn: func(ctx context.Context, body []byte) ([]byte, error) {
				return []byte(`{"projects":[]}`), nil
			},
		}
		h := NewSplUserScanHandler(neutralSales, ent, viewerAccessGuard)
		r := scanTestRequest(t, UserScanRequest{Email: "a@b.com", SubscriptionKey: "key-1"})
		w := httptest.NewRecorder()
		h.ScanUser(w, r)
		resp := decodeJSON[[]ScanResponse](t, w)
		sn := resp[1]
		if sn.System != "Servicenow" {
			t.Fatalf("system = %q, want Servicenow", sn.System)
		}
		if sn.SystemResult[1].Information != infoProjectNotFound {
			t.Errorf("project information = %+v, want ProjectNotFound", sn.SystemResult[1].Information)
		}
		if sn.SystemResult[0].Information != infoUserNotFoundInProject {
			t.Errorf("user information = %+v, want UserNotFoundInProject", sn.SystemResult[0].Information)
		}
	})

	t.Run("project found by fuzzy search but key doesn't match exactly is treated as not found", func(t *testing.T) {
		ent := &mockEntityScanClient{
			searchProjectsFn: func(ctx context.Context, body []byte) ([]byte, error) {
				// SearchProjects' own match is fuzzy (ILIKE); this project's key
				// only contains "key-1" as a substring, so lookupProjectByKey's
				// exact-match filter must reject it.
				return scanProjectsResponse("proj-1", "other-key-1-suffix", "Open"), nil
			},
		}
		h := NewSplUserScanHandler(neutralSales, ent, viewerAccessGuard)
		r := scanTestRequest(t, UserScanRequest{Email: "a@b.com", SubscriptionKey: "key-1"})
		w := httptest.NewRecorder()
		h.ScanUser(w, r)
		resp := decodeJSON[[]ScanResponse](t, w)
		if resp[1].SystemResult[1].Information != infoProjectNotFound {
			t.Errorf("project information = %+v, want ProjectNotFound", resp[1].SystemResult[1].Information)
		}
	})

	t.Run("project not open", func(t *testing.T) {
		ent := &mockEntityScanClient{
			searchProjectsFn: func(ctx context.Context, body []byte) ([]byte, error) {
				return scanProjectsResponse("proj-1", "key-1", "Closed"), nil
			},
			searchUsersFn: func(ctx context.Context, body []byte) ([]byte, error) {
				return scanUsersResponse("a@b.com", false), nil
			},
		}
		h := NewSplUserScanHandler(neutralSales, ent, viewerAccessGuard)
		r := scanTestRequest(t, UserScanRequest{Email: "a@b.com", SubscriptionKey: "key-1"})
		w := httptest.NewRecorder()
		h.ScanUser(w, r)
		resp := decodeJSON[[]ScanResponse](t, w)
		sn := resp[1]
		if sn.SystemResult[1].State {
			t.Error("project result should not be success when closure state isn't Open")
		}
		wantIssue := "The project is not in open state. The project is in Closed state. The project should be in the Open state."
		if sn.SystemResult[1].Information.Issue != wantIssue {
			t.Errorf("issue = %q, want %q", sn.SystemResult[1].Information.Issue, wantIssue)
		}
	})

	t.Run("user not found in an existing open project", func(t *testing.T) {
		ent := &mockEntityScanClient{
			searchProjectsFn: func(ctx context.Context, body []byte) ([]byte, error) {
				return scanProjectsResponse("proj-1", "key-1", "Open"), nil
			},
			searchUsersFn: func(ctx context.Context, body []byte) ([]byte, error) {
				return []byte(`{"users":[]}`), nil
			},
		}
		h := NewSplUserScanHandler(neutralSales, ent, viewerAccessGuard)
		r := scanTestRequest(t, UserScanRequest{Email: "a@b.com", SubscriptionKey: "key-1"})
		w := httptest.NewRecorder()
		h.ScanUser(w, r)
		resp := decodeJSON[[]ScanResponse](t, w)
		sn := resp[1]
		if sn.SystemResult[1].State != true {
			t.Error("project result should be success (Open)")
		}
		if sn.SystemResult[0].Information != infoUserNotFound {
			t.Errorf("user information = %+v, want UserNotFound", sn.SystemResult[0].Information)
		}
	})

	t.Run("active user succeeds", func(t *testing.T) {
		ent := &mockEntityScanClient{
			searchProjectsFn: func(ctx context.Context, body []byte) ([]byte, error) {
				return scanProjectsResponse("proj-1", "key-1", "Open"), nil
			},
			searchUsersFn: func(ctx context.Context, body []byte) ([]byte, error) {
				return scanUsersResponse("a@b.com", false), nil
			},
		}
		h := NewSplUserScanHandler(neutralSales, ent, viewerAccessGuard)
		r := scanTestRequest(t, UserScanRequest{Email: "a@b.com", SubscriptionKey: "key-1"})
		w := httptest.NewRecorder()
		h.ScanUser(w, r)
		resp := decodeJSON[[]ScanResponse](t, w)
		if !resp[1].SystemResult[0].State {
			t.Error("user result should be success")
		}
	})

	t.Run("locked-out user: resend succeeds", func(t *testing.T) {
		var capturedProjectID, capturedEmail string
		ent := &mockEntityScanClient{
			searchProjectsFn: func(ctx context.Context, body []byte) ([]byte, error) {
				return scanProjectsResponse("proj-1", "key-1", "Open"), nil
			},
			searchUsersFn: func(ctx context.Context, body []byte) ([]byte, error) {
				return scanUsersResponse("a@b.com", true), nil
			},
			resendProjectContactInvitationFn: func(ctx context.Context, projectID, email string) ([]byte, error) {
				capturedProjectID, capturedEmail = projectID, email
				return nil, nil
			},
		}
		h := NewSplUserScanHandler(neutralSales, ent, viewerAccessGuard)
		r := scanTestRequest(t, UserScanRequest{Email: "a@b.com", SubscriptionKey: "key-1", ResendInvitation: true})
		w := httptest.NewRecorder()
		h.ScanUser(w, r)
		resp := decodeJSON[[]ScanResponse](t, w)
		userResult := resp[1].SystemResult[0]
		if userResult.State {
			t.Error("locked-out user result should not be success")
		}
		if userResult.Information.Solution != "A fresh invitation email has been sent. Ask the user to check their inbox and accept it." {
			t.Errorf("solution = %q, unexpected", userResult.Information.Solution)
		}
		if capturedProjectID != "proj-1" || capturedEmail != "a@b.com" {
			t.Errorf("ResendProjectContactInvitation called with (%q, %q), want (proj-1, a@b.com)", capturedProjectID, capturedEmail)
		}
	})

	t.Run("locked-out user: resend fails", func(t *testing.T) {
		ent := &mockEntityScanClient{
			searchProjectsFn: func(ctx context.Context, body []byte) ([]byte, error) {
				return scanProjectsResponse("proj-1", "key-1", "Open"), nil
			},
			searchUsersFn: func(ctx context.Context, body []byte) ([]byte, error) {
				return scanUsersResponse("a@b.com", true), nil
			},
			resendProjectContactInvitationFn: func(ctx context.Context, projectID, email string) ([]byte, error) {
				return nil, context.DeadlineExceeded
			},
		}
		h := NewSplUserScanHandler(neutralSales, ent, viewerAccessGuard)
		r := scanTestRequest(t, UserScanRequest{Email: "a@b.com", SubscriptionKey: "key-1", ResendInvitation: true})
		w := httptest.NewRecorder()
		h.ScanUser(w, r)
		assertStatus(t, w, http.StatusOK)
		resp := decodeJSON[[]ScanResponse](t, w)
		userResult := resp[1].SystemResult[0]
		if userResult.Information.Solution != "Could not resend the invitation automatically. Resend it manually from the project's Contacts tab." {
			t.Errorf("solution = %q, unexpected", userResult.Information.Solution)
		}
	})

	t.Run("locked-out user: no resend without the flag", func(t *testing.T) {
		ent := &mockEntityScanClient{
			searchProjectsFn: func(ctx context.Context, body []byte) ([]byte, error) {
				return scanProjectsResponse("proj-1", "key-1", "Open"), nil
			},
			searchUsersFn: func(ctx context.Context, body []byte) ([]byte, error) {
				return scanUsersResponse("a@b.com", true), nil
			},
			resendProjectContactInvitationFn: func(ctx context.Context, projectID, email string) ([]byte, error) {
				t.Fatal("invitation must not be resent without resendInvitation")
				return nil, nil
			},
		}
		h := NewSplUserScanHandler(neutralSales, ent, viewerAccessGuard)
		w := httptest.NewRecorder()
		h.ScanUser(w, scanTestRequest(t, UserScanRequest{Email: "a@b.com", SubscriptionKey: "key-1"}))
		assertStatus(t, w, http.StatusOK)
		userResult := decodeJSON[[]ScanResponse](t, w)[1].SystemResult[0]
		if userResult.State || userResult.Information.Issue != "The user didn't accept the invitation." {
			t.Errorf("locked-out state not reported: %+v", userResult)
		}
		if userResult.Information.Solution == "" {
			t.Error("solution should point at the manual resend")
		}
	})
}

func TestSplScanUser_UpstreamFailuresReturn500WithBespokeMessage(t *testing.T) {
	t.Run("contact lookup failure", func(t *testing.T) {
		sales := &mockSalesEntityClient{
			getContactByEmailFn: func(ctx context.Context, email string) (*entity.Contact, error) {
				return nil, context.DeadlineExceeded
			},
		}
		h := NewSplUserScanHandler(sales, &mockEntityScanClient{}, viewerAccessGuard)
		r := scanTestRequest(t, UserScanRequest{Email: "a@b.com", SubscriptionKey: "sub-1"})
		w := httptest.NewRecorder()
		h.ScanUser(w, r)
		assertStatus(t, w, http.StatusInternalServerError)
		assertErrorMessage(t, w, "Error occurred when retrieving contact information")
	})

	t.Run("entity user search failure", func(t *testing.T) {
		ent := &mockEntityScanClient{
			searchUsersFn: func(ctx context.Context, body []byte) ([]byte, error) {
				return nil, context.DeadlineExceeded
			},
		}
		h := NewSplUserScanHandler(&mockSalesEntityClient{}, ent, viewerAccessGuard)
		r := scanTestRequest(t, UserScanRequest{Email: "a@b.com", SubscriptionKey: "sub-1"})
		w := httptest.NewRecorder()
		h.ScanUser(w, r)
		assertStatus(t, w, http.StatusInternalServerError)
		assertErrorMessage(t, w, "Error occurred when retrieving user information")
	})

	t.Run("entity project search failure", func(t *testing.T) {
		ent := &mockEntityScanClient{
			searchProjectsFn: func(ctx context.Context, body []byte) ([]byte, error) {
				return nil, context.DeadlineExceeded
			},
		}
		h := NewSplUserScanHandler(&mockSalesEntityClient{}, ent, viewerAccessGuard)
		r := scanTestRequest(t, UserScanRequest{Email: "a@b.com", SubscriptionKey: "sub-1"})
		w := httptest.NewRecorder()
		h.ScanUser(w, r)
		assertStatus(t, w, http.StatusInternalServerError)
		assertErrorMessage(t, w, "Error occurred when retrieving project information")
	})
}
