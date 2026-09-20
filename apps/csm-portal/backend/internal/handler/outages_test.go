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

const testOutageID = "aaaaaaaa-bbbb-cccc-dddd-ffffffffffff"

func TestCreateOutage(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewOutageHandler(&mockEntityOutageClient{})
		r := httptest.NewRequest(http.MethodPost, "/outages", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h.CreateOutage(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewOutageHandler(&mockEntityOutageClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/outages", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1))))
		w := httptest.NewRecorder()
		h.CreateOutage(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewOutageHandler(&mockEntityOutageClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/outages", strings.NewReader(`not-json`)))
		w := httptest.NewRecorder()
		h.CreateOutage(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects unknown type", func(t *testing.T) {
		h := NewOutageHandler(&mockEntityOutageClient{})
		body := `{"type":"catastrophe","begin":"2026-08-18 09:00:00","shortDescription":"test"}`
		r := withUser(httptest.NewRequest(http.MethodPost, "/outages", strings.NewReader(body)))
		w := httptest.NewRecorder()
		h.CreateOutage(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
	})

	t.Run("rejects missing shortDescription", func(t *testing.T) {
		h := NewOutageHandler(&mockEntityOutageClient{})
		body := `{"type":"outage","begin":"2026-08-18 09:00:00"}`
		r := withUser(httptest.NewRequest(http.MethodPost, "/outages", strings.NewReader(body)))
		w := httptest.NewRecorder()
		h.CreateOutage(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
	})

	t.Run("rejects malformed configurationItemId", func(t *testing.T) {
		h := NewOutageHandler(&mockEntityOutageClient{})
		body := `{"type":"outage","begin":"2026-08-18 09:00:00","shortDescription":"test","configurationItemId":"not-a-uuid"}`
		r := withUser(httptest.NewRequest(http.MethodPost, "/outages", strings.NewReader(body)))
		w := httptest.NewRecorder()
		h.CreateOutage(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
	})

	t.Run("forwards body to upstream and returns 201 with response", func(t *testing.T) {
		const reqPayload = `{"type":"degradation","begin":"2026-08-18 09:00:00","shortDescription":"TEST RECORD"}`
		var capturedBody []byte
		client := &mockEntityOutageClient{
			createOutageFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"message":"Outage created successfully","outage":{"id":"` + testOutageID + `","number":"OUT0001881"}}`), nil
			},
		}
		h := NewOutageHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/outages", strings.NewReader(reqPayload)))
		w := httptest.NewRecorder()
		h.CreateOutage(w, r)

		assertStatus(t, w, http.StatusCreated)
		assertContentType(t, w, "application/json")
		if string(capturedBody) != reqPayload {
			t.Errorf("upstream received body %q, want %q", capturedBody, reqPayload)
		}
		resp := decodeJSON[map[string]any](t, w)
		outage, _ := resp["outage"].(map[string]any)
		if outage["number"] != "OUT0001881" {
			t.Errorf("response outage.number = %v, want OUT0001881", outage["number"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to create outage.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityOutageClient{
					createOutageFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewOutageHandler(client)
				body := `{"type":"outage","begin":"2026-08-18 09:00:00","shortDescription":"test"}`
				r := withUser(httptest.NewRequest(http.MethodPost, "/outages", strings.NewReader(body)))
				w := httptest.NewRecorder()
				h.CreateOutage(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestSearchOutages(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewOutageHandler(&mockEntityOutageClient{})
		r := httptest.NewRequest(http.MethodPost, "/outages/search", strings.NewReader(`{}`))
		w := httptest.NewRecorder()
		h.SearchOutages(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewOutageHandler(&mockEntityOutageClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/outages/search", strings.NewReader(`not-json`)))
		w := httptest.NewRecorder()
		h.SearchOutages(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
	})

	t.Run("forwards body and returns 200", func(t *testing.T) {
		var capturedBody []byte
		client := &mockEntityOutageClient{
			searchOutagesFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"outages":[],"total":0,"limit":2,"offset":0,"appliedBeginFrom":"2026-02-18","beginFromDefaulted":true}`), nil
			},
		}
		h := NewOutageHandler(client)
		reqPayload := `{"pagination":{"limit":2}}`
		r := withUser(httptest.NewRequest(http.MethodPost, "/outages/search", strings.NewReader(reqPayload)))
		w := httptest.NewRecorder()
		h.SearchOutages(w, r)
		assertStatus(t, w, http.StatusOK)
		if string(capturedBody) != reqPayload {
			t.Errorf("upstream received body %q, want %q", capturedBody, reqPayload)
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to search outages.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityOutageClient{
					searchOutagesFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewOutageHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/outages/search", strings.NewReader(`{}`)))
				w := httptest.NewRecorder()
				h.SearchOutages(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
			})
		}
	})
}

func TestGetOutage(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewOutageHandler(&mockEntityOutageClient{})
		r := httptest.NewRequest(http.MethodGet, "/outages/"+testOutageID, nil)
		r.SetPathValue("id", testOutageID)
		w := httptest.NewRecorder()
		h.GetOutage(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
	})

	t.Run("rejects malformed UUID", func(t *testing.T) {
		h := NewOutageHandler(&mockEntityOutageClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/outages/not-a-uuid", nil))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.GetOutage(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
	})

	t.Run("returns 200 with the upstream result", func(t *testing.T) {
		client := &mockEntityOutageClient{
			getOutageFn: func(_ context.Context, id string) ([]byte, error) {
				return []byte(`{"id":"` + id + `","number":"OUT0001875"}`), nil
			},
		}
		h := NewOutageHandler(client)
		r := withUser(httptest.NewRequest(http.MethodGet, "/outages/"+testOutageID, nil))
		r.SetPathValue("id", testOutageID)
		w := httptest.NewRecorder()
		h.GetOutage(w, r)
		assertStatus(t, w, http.StatusOK)
		resp := decodeJSON[map[string]any](t, w)
		if resp["number"] != "OUT0001875" {
			t.Errorf("response number = %v, want OUT0001875", resp["number"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to retrieve outage.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityOutageClient{
					getOutageFn: func(_ context.Context, _ string) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewOutageHandler(client)
				r := withUser(httptest.NewRequest(http.MethodGet, "/outages/"+testOutageID, nil))
				r.SetPathValue("id", testOutageID)
				w := httptest.NewRecorder()
				h.GetOutage(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
			})
		}
	})
}

func TestPatchOutage(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewOutageHandler(&mockEntityOutageClient{})
		r := httptest.NewRequest(http.MethodPatch, "/outages/"+testOutageID, strings.NewReader(`{"end":null}`))
		r.SetPathValue("id", testOutageID)
		w := httptest.NewRecorder()
		h.PatchOutage(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
	})

	t.Run("rejects malformed UUID", func(t *testing.T) {
		h := NewOutageHandler(&mockEntityOutageClient{})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/outages/not-a-uuid", strings.NewReader(`{"end":null}`)))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.PatchOutage(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
	})

	t.Run("rejects empty body", func(t *testing.T) {
		h := NewOutageHandler(&mockEntityOutageClient{})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/outages/"+testOutageID, strings.NewReader(`{}`)))
		r.SetPathValue("id", testOutageID)
		w := httptest.NewRecorder()
		h.PatchOutage(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
	})

	t.Run("rejects unknown type", func(t *testing.T) {
		h := NewOutageHandler(&mockEntityOutageClient{})
		r := withUser(httptest.NewRequest(http.MethodPatch, "/outages/"+testOutageID, strings.NewReader(`{"type":"catastrophe"}`)))
		r.SetPathValue("id", testOutageID)
		w := httptest.NewRecorder()
		h.PatchOutage(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
	})

	t.Run("accepts explicit null end (reopen)", func(t *testing.T) {
		var capturedBody []byte
		client := &mockEntityOutageClient{
			patchOutageFn: func(_ context.Context, _ string, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"message":"Outage updated successfully","outage":{"id":"` + testOutageID + `"}}`), nil
			},
		}
		h := NewOutageHandler(client)
		reqPayload := `{"end":null}`
		r := withUser(httptest.NewRequest(http.MethodPatch, "/outages/"+testOutageID, strings.NewReader(reqPayload)))
		r.SetPathValue("id", testOutageID)
		w := httptest.NewRecorder()
		h.PatchOutage(w, r)
		assertStatus(t, w, http.StatusOK)
		if string(capturedBody) != reqPayload {
			t.Errorf("upstream received body %q, want %q", capturedBody, reqPayload)
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to update outage.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityOutageClient{
					patchOutageFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewOutageHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPatch, "/outages/"+testOutageID, strings.NewReader(`{"end":null}`)))
				r.SetPathValue("id", testOutageID)
				w := httptest.NewRecorder()
				h.PatchOutage(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
			})
		}
	})
}

func TestAddOutageCommunication(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewOutageHandler(&mockEntityOutageClient{})
		r := httptest.NewRequest(http.MethodPost, "/outages/"+testOutageID+"/communications", strings.NewReader(`{}`))
		r.SetPathValue("id", testOutageID)
		w := httptest.NewRecorder()
		h.AddOutageCommunication(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
	})

	t.Run("rejects malformed UUID", func(t *testing.T) {
		h := NewOutageHandler(&mockEntityOutageClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/outages/not-a-uuid/communications", strings.NewReader(`{}`)))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.AddOutageCommunication(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
	})

	t.Run("rejects unknown channel", func(t *testing.T) {
		h := NewOutageHandler(&mockEntityOutageClient{})
		body := `{"channel":"twitter","body":"hello"}`
		r := withUser(httptest.NewRequest(http.MethodPost, "/outages/"+testOutageID+"/communications", strings.NewReader(body)))
		r.SetPathValue("id", testOutageID)
		w := httptest.NewRecorder()
		h.AddOutageCommunication(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
	})

	t.Run("rejects empty body text", func(t *testing.T) {
		h := NewOutageHandler(&mockEntityOutageClient{})
		body := `{"channel":"external","body":""}`
		r := withUser(httptest.NewRequest(http.MethodPost, "/outages/"+testOutageID+"/communications", strings.NewReader(body)))
		r.SetPathValue("id", testOutageID)
		w := httptest.NewRecorder()
		h.AddOutageCommunication(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
	})

	t.Run("forwards body and returns 201", func(t *testing.T) {
		var capturedID string
		var capturedBody []byte
		client := &mockEntityOutageClient{
			addOutageCommunicationFn: func(_ context.Context, id string, body []byte) ([]byte, error) {
				capturedID = id
				capturedBody = body
				return []byte(`{"message":"Communication added successfully","communication":{"id":"11111111-1111-1111-1111-111111111111","isPublic":true}}`), nil
			},
		}
		h := NewOutageHandler(client)
		reqPayload := `{"channel":"external","body":"status update"}`
		r := withUser(httptest.NewRequest(http.MethodPost, "/outages/"+testOutageID+"/communications", strings.NewReader(reqPayload)))
		r.SetPathValue("id", testOutageID)
		w := httptest.NewRecorder()
		h.AddOutageCommunication(w, r)
		assertStatus(t, w, http.StatusCreated)
		if capturedID != testOutageID {
			t.Errorf("upstream received id %q, want %q", capturedID, testOutageID)
		}
		if string(capturedBody) != reqPayload {
			t.Errorf("upstream received body %q, want %q", capturedBody, reqPayload)
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to add outage communication.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityOutageClient{
					addOutageCommunicationFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewOutageHandler(client)
				body := `{"channel":"internal","body":"note"}`
				r := withUser(httptest.NewRequest(http.MethodPost, "/outages/"+testOutageID+"/communications", strings.NewReader(body)))
				r.SetPathValue("id", testOutageID)
				w := httptest.NewRecorder()
				h.AddOutageCommunication(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
			})
		}
	})
}

func TestSearchOutageCommunications(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewOutageHandler(&mockEntityOutageClient{})
		r := httptest.NewRequest(http.MethodPost, "/outages/"+testOutageID+"/communications/search", strings.NewReader(`{}`))
		r.SetPathValue("id", testOutageID)
		w := httptest.NewRecorder()
		h.SearchOutageCommunications(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
	})

	t.Run("rejects malformed UUID", func(t *testing.T) {
		h := NewOutageHandler(&mockEntityOutageClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/outages/not-a-uuid/communications/search", strings.NewReader(`{}`)))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.SearchOutageCommunications(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
	})

	t.Run("forwards body and returns 200", func(t *testing.T) {
		var capturedID string
		client := &mockEntityOutageClient{
			searchOutageCommunicationsFn: func(_ context.Context, id string, _ []byte) ([]byte, error) {
				capturedID = id
				return []byte(`{"communications":[],"total":2,"limit":20,"offset":0}`), nil
			},
		}
		h := NewOutageHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/outages/"+testOutageID+"/communications/search", strings.NewReader(`{}`)))
		r.SetPathValue("id", testOutageID)
		w := httptest.NewRecorder()
		h.SearchOutageCommunications(w, r)
		assertStatus(t, w, http.StatusOK)
		if capturedID != testOutageID {
			t.Errorf("upstream received id %q, want %q", capturedID, testOutageID)
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to search outage communications.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityOutageClient{
					searchOutageCommunicationsFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewOutageHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/outages/"+testOutageID+"/communications/search", strings.NewReader(`{}`)))
				r.SetPathValue("id", testOutageID)
				w := httptest.NewRecorder()
				h.SearchOutageCommunications(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
			})
		}
	})
}

func TestGetOutageMetadata(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewOutageHandler(&mockEntityOutageClient{})
		r := httptest.NewRequest(http.MethodGet, "/outages/metadata", nil)
		w := httptest.NewRecorder()
		h.GetOutageMetadata(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
	})

	t.Run("returns 200 with the upstream result", func(t *testing.T) {
		client := &mockEntityOutageClient{
			getOutageMetadataFn: func(_ context.Context) ([]byte, error) {
				return []byte(`{"types":[{"value":"outage","label":"Outage"}],"statuses":[],"communicationChannels":[],"statusPageClouds":["asgardeo"]}`), nil
			},
		}
		h := NewOutageHandler(client)
		r := withUser(httptest.NewRequest(http.MethodGet, "/outages/metadata", nil))
		w := httptest.NewRecorder()
		h.GetOutageMetadata(w, r)
		assertStatus(t, w, http.StatusOK)
		resp := decodeJSON[map[string]any](t, w)
		clouds, _ := resp["statusPageClouds"].([]any)
		if len(clouds) != 1 || clouds[0] != "asgardeo" {
			t.Errorf("response statusPageClouds = %v, want [asgardeo]", resp["statusPageClouds"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to retrieve outage metadata.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityOutageClient{
					getOutageMetadataFn: func(_ context.Context) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewOutageHandler(client)
				r := withUser(httptest.NewRequest(http.MethodGet, "/outages/metadata", nil))
				w := httptest.NewRecorder()
				h.GetOutageMetadata(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
			})
		}
	})
}
