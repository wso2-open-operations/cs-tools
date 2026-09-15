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
)

func TestSearchInvoices(t *testing.T) {
	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewInvoiceHandler(&mockEntityInvoiceClient{})
		r := httptest.NewRequest(http.MethodPost, "/invoices/search", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1)))
		w := httptest.NewRecorder()
		h.SearchInvoices(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewInvoiceHandler(&mockEntityInvoiceClient{})
		r := httptest.NewRequest(http.MethodPost, "/invoices/search", strings.NewReader(`not-json`))
		w := httptest.NewRecorder()
		h.SearchInvoices(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("allows an empty body", func(t *testing.T) {
		client := &mockEntityInvoiceClient{
			searchInvoicesFn: func(_ context.Context, body []byte) ([]byte, error) {
				if len(body) != 0 {
					t.Errorf("upstream body = %q, want empty", body)
				}
				return []byte(`{"invoices":[],"total":0}`), nil
			},
		}
		h := NewInvoiceHandler(client)
		r := httptest.NewRequest(http.MethodPost, "/invoices/search", nil)
		w := httptest.NewRecorder()
		h.SearchInvoices(w, r)
		assertStatus(t, w, http.StatusOK)
	})

	t.Run("forwards body to upstream and returns 200 with response", func(t *testing.T) {
		const reqPayload = `{"opportunityId":"22222222-2222-2222-2222-222222222222","pagination":{"limit":10}}`
		var capturedBody []byte
		client := &mockEntityInvoiceClient{
			searchInvoicesFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"invoices":[{"id":"33333333-3333-3333-3333-333333333333","name":"INV-1"}],"total":1}`), nil
			},
		}
		h := NewInvoiceHandler(client)
		r := httptest.NewRequest(http.MethodPost, "/invoices/search", strings.NewReader(reqPayload))
		w := httptest.NewRecorder()
		h.SearchInvoices(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if string(capturedBody) != reqPayload {
			t.Errorf("upstream received body %q, want %q", capturedBody, reqPayload)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["total"] != float64(1) {
			t.Errorf("total = %v, want 1", resp["total"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to search invoices.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityInvoiceClient{
					searchInvoicesFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewInvoiceHandler(client)
				r := httptest.NewRequest(http.MethodPost, "/invoices/search", strings.NewReader(`{}`))
				w := httptest.NewRecorder()
				h.SearchInvoices(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestGetInvoice(t *testing.T) {
	t.Run("rejects empty invoice ID", func(t *testing.T) {
		h := NewInvoiceHandler(&mockEntityInvoiceClient{})
		r := httptest.NewRequest(http.MethodGet, "/invoices/", nil)
		w := httptest.NewRecorder()
		h.GetInvoice(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects non-UUID invoice ID", func(t *testing.T) {
		h := NewInvoiceHandler(&mockEntityInvoiceClient{})
		r := httptest.NewRequest(http.MethodGet, "/invoices/inv-42", nil)
		r.SetPathValue("id", "inv-42")
		w := httptest.NewRecorder()
		h.GetInvoice(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("passes ID to upstream and returns 200 with response", func(t *testing.T) {
		const invoiceID = "11111111-1111-1111-1111-111111111111"
		var capturedID string
		client := &mockEntityInvoiceClient{
			getInvoiceFn: func(_ context.Context, id string) ([]byte, error) {
				capturedID = id
				return []byte(`{"id":"` + invoiceID + `","name":"INV-1"}`), nil
			},
		}
		h := NewInvoiceHandler(client)
		r := httptest.NewRequest(http.MethodGet, "/invoices/"+invoiceID, nil)
		r.SetPathValue("id", invoiceID)
		w := httptest.NewRecorder()
		h.GetInvoice(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")
		if capturedID != invoiceID {
			t.Errorf("upstream received id %q, want %q", capturedID, invoiceID)
		}
		resp := decodeJSON[map[string]any](t, w)
		if resp["id"] != invoiceID {
			t.Errorf("response id = %v, want %s", resp["id"], invoiceID)
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		const invoiceID = "11111111-1111-1111-1111-111111111111"
		for _, tc := range upstreamErrors("Failed to retrieve invoice.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityInvoiceClient{
					getInvoiceFn: func(_ context.Context, _ string) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewInvoiceHandler(client)
				r := httptest.NewRequest(http.MethodGet, "/invoices/"+invoiceID, nil)
				r.SetPathValue("id", invoiceID)
				w := httptest.NewRecorder()
				h.GetInvoice(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}
