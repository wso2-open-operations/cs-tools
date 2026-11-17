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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/wso2-open-operations/cs-tools/apps/csm-portal/backend/internal/apierror"
)

// A standard change template is one customer's (entity-service decides it): the
// customer filter goes through as sent, the customer comes back as entity-service
// answered, and its refusal of another customer's project keeps its words and its
// errorCode on the change request create and PATCH.

const otherCustomerRefusal = `the standard change template "Rotate the TLS certificate" is Example Corp's and cannot be used for a project of Other Corp: a template is pre-authorised for its own customer's deployments only`

func TestStandardChangeHandler_CustomerFilterAndReadPassThrough(t *testing.T) {
	const answer = `{"templates":[{"id":"` + stdID + `","name":"Rotate","customer":{"id":"acc-1","name":"Example Corp"}}],"total":1,"limit":20,"offset":0}`
	for name, call := range map[string]func(h *StandardChangeHandler) http.HandlerFunc{
		"templates": func(h *StandardChangeHandler) http.HandlerFunc { return h.SearchTemplates },
		"proposals": func(h *StandardChangeHandler) http.HandlerFunc { return h.SearchProposals },
	} {
		t.Run(name, func(t *testing.T) {
			const body = `{"accountId":"00000000-0000-0000-0000-000000000301","pagination":{"limit":20,"offset":0}}`
			fake := &fakeStandardChangeClient{result: []byte(answer)}
			w := httptest.NewRecorder()
			call(NewStandardChangeHandler(fake))(w, withUser(httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(body))))
			assertStatus(t, w, http.StatusOK)
			if string(fake.body) != body {
				t.Fatalf("forwarded %s, want %s", fake.body, body)
			}
			if !strings.Contains(w.Body.String(), `"customer":{"id":"acc-1","name":"Example Corp"}`) {
				t.Fatalf("answer = %s", w.Body.String())
			}
		})
	}

	t.Run("a proposal for another customer keeps its refusal", func(t *testing.T) {
		const refusal = "accountId must be the change request's customer (Example Corp): a standard change belongs to one customer, and the change request cannot be used for another"
		fake := &fakeStandardChangeClient{err: &apierror.Error{StatusCode: http.StatusBadRequest,
			Body: `{"code":400,"message":` + jsonQuote(refusal) + `}`}}
		w := httptest.NewRecorder()
		NewStandardChangeHandler(fake).CreateProposal(w, withUser(httptest.NewRequest(http.MethodPost, "/x",
			strings.NewReader(`{"kind":"new","sourceChangeRequestId":"`+testCRID+`","accountId":"00000000-0000-0000-0000-000000000302"}`))))
		assertStatus(t, w, http.StatusBadRequest)
		if body := assertErrorBodyKeys(t, w); body["message"] != refusal {
			t.Fatalf("body = %v", body)
		}
	})
}

func TestChangeRequest_TemplateOfAnotherCustomerKeepsItsCode(t *testing.T) {
	upstream := &apierror.Error{StatusCode: http.StatusBadRequest,
		Body: `{"code":400,"message":` + jsonQuote(otherCustomerRefusal) + `,"errorCode":"standard_change_template_other_customer"}`}

	t.Run("create", func(t *testing.T) {
		client := &mockEntityChangeRequestClient{
			createChangeRequestFn: func(_ context.Context, _ []byte) ([]byte, error) { return nil, upstream },
		}
		w := httptest.NewRecorder()
		NewChangeRequestHandler(client).CreateChangeRequest(w, withUser(httptest.NewRequest(http.MethodPost, "/change-requests",
			strings.NewReader(`{"type":"standard","standardChangeTemplateId":"`+stdID+`","projectId":"00000000-0000-0000-0000-000000000402"}`))))
		assertStatus(t, w, http.StatusBadRequest)
		if body := assertErrorBodyKeys(t, w); body["message"] != otherCustomerRefusal || body["errorCode"] != "standard_change_template_other_customer" {
			t.Fatalf("body = %v", body)
		}
	})
	t.Run("patch", func(t *testing.T) {
		client := &mockEntityChangeRequestClient{
			patchChangeRequestFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) { return nil, upstream },
		}
		r := withUser(httptest.NewRequest(http.MethodPatch, "/change-requests/"+testCRID, strings.NewReader(`{"projectId":"00000000-0000-0000-0000-000000000402"}`)))
		r.SetPathValue("id", testCRID)
		w := httptest.NewRecorder()
		NewChangeRequestHandler(client).PatchChangeRequest(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		if body := assertErrorBodyKeys(t, w); body["message"] != otherCustomerRefusal || body["errorCode"] != "standard_change_template_other_customer" {
			t.Fatalf("body = %v", body)
		}
	})
}
