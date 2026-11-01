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
	"strings"
	"testing"
)

func TestPatchCase(t *testing.T) {
	const caseID = "11111111-1111-1111-1111-111111111111"

	t.Run("rejects empty case ID", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{}, "", nil)
		r := httptest.NewRequest(http.MethodPatch, "/cases/", strings.NewReader(`{"state":"closed"}`))
		w := httptest.NewRecorder()
		h.PatchCase(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects non-UUID case ID", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{}, "", nil)
		r := httptest.NewRequest(http.MethodPatch, "/cases/case-42", strings.NewReader(`{"state":"closed"}`))
		r.SetPathValue("id", "case-42")
		w := httptest.NewRecorder()
		h.PatchCase(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{}, "", nil)
		r := httptest.NewRequest(http.MethodPatch, "/cases/"+caseID, strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1)))
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.PatchCase(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{}, "", nil)
		r := httptest.NewRequest(http.MethodPatch, "/cases/"+caseID, strings.NewReader(`not-json`))
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.PatchCase(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty body", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{}, "", nil)
		r := httptest.NewRequest(http.MethodPatch, "/cases/"+caseID, nil)
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.PatchCase(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards body verbatim and returns upstream response", func(t *testing.T) {
		var capturedCaseID string
		var capturedBody []byte
		reqBody := `{"state":"closed"}`
		client := &mockEntityCaseClient{
			patchCaseFn: func(_ context.Context, id string, body []byte) ([]byte, error) {
				capturedCaseID = id
				capturedBody = body
				return []byte(`{"message":"Case updated successfully","case":{"id":"` + caseID + `","state":"closed"}}`), nil
			},
		}
		h := NewCaseHandler(client, "", nil)
		r := httptest.NewRequest(http.MethodPatch, "/cases/"+caseID, strings.NewReader(reqBody))
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.PatchCase(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		if capturedCaseID != caseID {
			t.Errorf("caseID = %q, want %q", capturedCaseID, caseID)
		}
		if string(capturedBody) != reqBody {
			t.Errorf("upstream body = %q, want verbatim %q", string(capturedBody), reqBody)
		}

		resp := decodeJSON[map[string]any](t, w)
		if resp["message"] != "Case updated successfully" {
			t.Errorf("message = %v, want %v", resp["message"], "Case updated successfully")
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to update case.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					patchCaseFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewCaseHandler(client, "", nil)
				r := httptest.NewRequest(http.MethodPatch, "/cases/"+caseID, strings.NewReader(`{"state":"closed"}`))
				r.SetPathValue("id", caseID)
				w := httptest.NewRecorder()
				h.PatchCase(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})

	t.Run("a caller-supplied actorEmail is always stripped, even with no mapping configured", func(t *testing.T) {
		var capturedBody []byte
		client := &mockEntityCaseClient{
			patchCaseFn: func(_ context.Context, _ string, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"message":"Case updated successfully"}`), nil
			},
		}
		h := NewCaseHandler(client, "", nil)
		r := httptest.NewRequest(http.MethodPatch, "/cases/"+caseID, strings.NewReader(`{"markFixIssued":true,"actorEmail":"attacker@evil.example"}`))
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.PatchCase(w, r)

		assertStatus(t, w, http.StatusOK)
		if strings.Contains(string(capturedBody), "attacker@evil.example") {
			t.Errorf("upstream body = %q, must not carry the caller-supplied actorEmail", string(capturedBody))
		}
	})

	t.Run("a differently-cased actorEmail key is stripped too", func(t *testing.T) {
		var capturedBody []byte
		client := &mockEntityCaseClient{
			patchCaseFn: func(_ context.Context, _ string, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"message":"Case updated successfully"}`), nil
			},
		}
		h := NewCaseHandler(client, "", nil)
		r := httptest.NewRequest(http.MethodPatch, "/cases/"+caseID, strings.NewReader(`{"markFixIssued":true,"ActorEmail":"attacker@evil.example"}`))
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.PatchCase(w, r)

		assertStatus(t, w, http.StatusOK)
		if strings.Contains(string(capturedBody), "attacker@evil.example") {
			t.Errorf("upstream body = %q, must not carry the caller-supplied ActorEmail (any casing)", string(capturedBody))
		}
	})

	t.Run("a caller-supplied actorEmail is overridden, not just hidden, when the client id is mapped", func(t *testing.T) {
		var capturedBody []byte
		client := &mockEntityCaseClient{
			patchCaseFn: func(_ context.Context, _ string, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"message":"Case updated successfully"}`), nil
			},
		}
		h := NewCaseHandler(client, "", map[string]string{"client-abc": "integration-a@example.com"})
		r := httptest.NewRequest(http.MethodPatch, "/cases/"+caseID, strings.NewReader(`{"markFixIssued":true,"actorEmail":"attacker@evil.example"}`))
		r.SetPathValue("id", caseID)
		r.Header.Set(jwtAssertionHeader, testJWTAssertion(t, "client-abc"))
		w := httptest.NewRecorder()
		h.PatchCase(w, r)

		assertStatus(t, w, http.StatusOK)
		var got patchCaseMarkFixIssuedUpstreamRequest
		if err := json.Unmarshal(capturedBody, &got); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		if got.ActorEmail == nil || *got.ActorEmail != "integration-a@example.com" {
			t.Errorf("actorEmail = %v, want the resolved %q, not the caller's claimed address", got.ActorEmail, "integration-a@example.com")
		}
	})

	t.Run("a caller-supplied actorEmail on a non-markFixIssued update is stripped too", func(t *testing.T) {
		var capturedBody []byte
		client := &mockEntityCaseClient{
			patchCaseFn: func(_ context.Context, _ string, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"message":"Case updated successfully"}`), nil
			},
		}
		h := NewCaseHandler(client, "", map[string]string{"client-abc": "integration-a@example.com"})
		r := httptest.NewRequest(http.MethodPatch, "/cases/"+caseID, strings.NewReader(`{"state":"closed","actorEmail":"attacker@evil.example"}`))
		r.SetPathValue("id", caseID)
		r.Header.Set(jwtAssertionHeader, testJWTAssertion(t, "client-abc"))
		w := httptest.NewRecorder()
		h.PatchCase(w, r)

		assertStatus(t, w, http.StatusOK)
		if strings.Contains(string(capturedBody), "actorEmail") {
			t.Errorf("upstream body = %q, must not carry actorEmail at all for a non-markFixIssued update", string(capturedBody))
		}
	})

	t.Run("markFixIssued: adds actorEmail when the caller's client id is mapped", func(t *testing.T) {
		var capturedBody []byte
		client := &mockEntityCaseClient{
			patchCaseFn: func(_ context.Context, _ string, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"message":"Case updated successfully"}`), nil
			},
		}
		h := NewCaseHandler(client, "", map[string]string{"client-abc": "integration-a@example.com"})
		r := httptest.NewRequest(http.MethodPatch, "/cases/"+caseID, strings.NewReader(`{"markFixIssued":true}`))
		r.SetPathValue("id", caseID)
		r.Header.Set(jwtAssertionHeader, testJWTAssertion(t, "client-abc"))
		w := httptest.NewRecorder()
		h.PatchCase(w, r)

		assertStatus(t, w, http.StatusOK)

		var got patchCaseMarkFixIssuedUpstreamRequest
		if err := json.Unmarshal(capturedBody, &got); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		if got.MarkFixIssued == nil || !*got.MarkFixIssued {
			t.Errorf("markFixIssued = %v, want true", got.MarkFixIssued)
		}
		if got.ActorEmail == nil || *got.ActorEmail != "integration-a@example.com" {
			t.Errorf("actorEmail = %v, want %q", got.ActorEmail, "integration-a@example.com")
		}
	})

	t.Run("markFixIssued: a mapped client's other fields are preserved, not dropped", func(t *testing.T) {
		var capturedBody []byte
		client := &mockEntityCaseClient{
			patchCaseFn: func(_ context.Context, _ string, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"message":"Case updated successfully"}`), nil
			},
		}
		h := NewCaseHandler(client, "", map[string]string{"client-abc": "integration-a@example.com"})
		r := httptest.NewRequest(http.MethodPatch, "/cases/"+caseID, strings.NewReader(`{"markFixIssued":true,"state":"closed"}`))
		r.SetPathValue("id", caseID)
		r.Header.Set(jwtAssertionHeader, testJWTAssertion(t, "client-abc"))
		w := httptest.NewRecorder()
		h.PatchCase(w, r)

		assertStatus(t, w, http.StatusOK)

		var got map[string]any
		if err := json.Unmarshal(capturedBody, &got); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		if got["state"] != "closed" {
			t.Errorf("upstream body = %s, state must be preserved so entity-service's own field-combination rule can still reject it", capturedBody)
		}
		if got["actorEmail"] != "integration-a@example.com" {
			t.Errorf("upstream body = %s, actorEmail = %v, want %q", capturedBody, got["actorEmail"], "integration-a@example.com")
		}
	})

	t.Run("markFixIssued: forwards unchanged when the caller's client id is unmapped", func(t *testing.T) {
		var capturedBody []byte
		reqBody := `{"markFixIssued":true}`
		client := &mockEntityCaseClient{
			patchCaseFn: func(_ context.Context, _ string, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"message":"Case updated successfully"}`), nil
			},
		}
		h := NewCaseHandler(client, "", map[string]string{"client-abc": "integration-a@example.com"})
		r := httptest.NewRequest(http.MethodPatch, "/cases/"+caseID, strings.NewReader(reqBody))
		r.SetPathValue("id", caseID)
		r.Header.Set(jwtAssertionHeader, testJWTAssertion(t, "client-unknown"))
		w := httptest.NewRecorder()
		h.PatchCase(w, r)

		assertStatus(t, w, http.StatusOK)
		if string(capturedBody) != reqBody {
			t.Errorf("upstream body = %q, want verbatim %q", string(capturedBody), reqBody)
		}
	})

	t.Run("markFixIssued: forwards unchanged with no x-jwt-assertion header", func(t *testing.T) {
		var capturedBody []byte
		reqBody := `{"markFixIssued":true}`
		client := &mockEntityCaseClient{
			patchCaseFn: func(_ context.Context, _ string, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"message":"Case updated successfully"}`), nil
			},
		}
		h := NewCaseHandler(client, "", map[string]string{"client-abc": "integration-a@example.com"})
		r := httptest.NewRequest(http.MethodPatch, "/cases/"+caseID, strings.NewReader(reqBody))
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.PatchCase(w, r)

		assertStatus(t, w, http.StatusOK)
		if string(capturedBody) != reqBody {
			t.Errorf("upstream body = %q, want verbatim %q", string(capturedBody), reqBody)
		}
	})

	t.Run("non-markFixIssued update is never enriched, even with a mapped client id", func(t *testing.T) {
		var capturedBody []byte
		reqBody := `{"state":"closed"}`
		client := &mockEntityCaseClient{
			patchCaseFn: func(_ context.Context, _ string, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"message":"Case updated successfully"}`), nil
			},
		}
		h := NewCaseHandler(client, "", map[string]string{"client-abc": "integration-a@example.com"})
		r := httptest.NewRequest(http.MethodPatch, "/cases/"+caseID, strings.NewReader(reqBody))
		r.SetPathValue("id", caseID)
		r.Header.Set(jwtAssertionHeader, testJWTAssertion(t, "client-abc"))
		w := httptest.NewRecorder()
		h.PatchCase(w, r)

		assertStatus(t, w, http.StatusOK)
		if string(capturedBody) != reqBody {
			t.Errorf("upstream body = %q, want verbatim %q", string(capturedBody), reqBody)
		}
	})
}

func TestStripCallerActorEmail(t *testing.T) {
	t.Run("removes actorEmail, leaving every other field intact", func(t *testing.T) {
		got := stripCallerActorEmail([]byte(`{"markFixIssued":true,"actorEmail":"attacker@evil.example"}`))
		var m map[string]any
		if err := json.Unmarshal(got, &m); err != nil {
			t.Fatalf("decode result: %v", err)
		}
		if _, ok := m["actorEmail"]; ok {
			t.Errorf("result = %s, actorEmail must be removed", got)
		}
		if m["markFixIssued"] != true {
			t.Errorf("result = %s, markFixIssued must be preserved", got)
		}
	})

	t.Run("no-op when the body has no actorEmail field", func(t *testing.T) {
		body := []byte(`{"state":"closed"}`)
		if got := stripCallerActorEmail(body); string(got) != string(body) {
			t.Errorf("result = %s, want unchanged %s", got, body)
		}
	})

	t.Run("not a JSON object is returned unchanged rather than failing", func(t *testing.T) {
		body := []byte(`[1,2,3]`)
		if got := stripCallerActorEmail(body); string(got) != string(body) {
			t.Errorf("result = %s, want unchanged %s", got, body)
		}
	})

	t.Run("removes a differently-cased key too, since struct decoding would still match it", func(t *testing.T) {
		for _, key := range []string{"ActorEmail", "ACTOREMAIL", "actoremail"} {
			got := stripCallerActorEmail([]byte(`{"markFixIssued":true,"` + key + `":"attacker@evil.example"}`))
			var m map[string]any
			if err := json.Unmarshal(got, &m); err != nil {
				t.Fatalf("decode result for key %q: %v", key, err)
			}
			for k := range m {
				if strings.EqualFold(k, "actorEmail") {
					t.Errorf("key %q: result = %s, still carries an actorEmail variant", key, got)
				}
			}
		}
	})
}

func TestCreateCaseComment(t *testing.T) {
	const caseID = "11111111-1111-1111-1111-111111111111"

	t.Run("rejects empty case ID", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{}, "", nil)
		r := httptest.NewRequest(http.MethodPost, "/cases//comments", strings.NewReader(`{"type":"comment","content":"hi"}`))
		w := httptest.NewRecorder()
		h.CreateCaseComment(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects non-UUID case ID", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{}, "", nil)
		r := httptest.NewRequest(http.MethodPost, "/cases/case-42/comments", strings.NewReader(`{"type":"comment","content":"hi"}`))
		r.SetPathValue("id", "case-42")
		w := httptest.NewRecorder()
		h.CreateCaseComment(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{}, "", nil)
		r := httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/comments", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1)))
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.CreateCaseComment(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{}, "", nil)
		r := httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/comments", strings.NewReader(`not-json`))
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.CreateCaseComment(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty body", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{}, "", nil)
		r := httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/comments", nil)
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.CreateCaseComment(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects missing content", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{}, "svc@example.com", nil)
		r := httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/comments", strings.NewReader(`{"type":"comment"}`))
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.CreateCaseComment(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgContentRequired)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects blank content", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{}, "svc@example.com", nil)
		r := httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/comments", strings.NewReader(`{"type":"comment","content":"   "}`))
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.CreateCaseComment(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgContentRequired)
		assertContentType(t, w, "application/json")
	})

	t.Run("injects the configured actorEmail and ignores a caller-supplied one", func(t *testing.T) {
		var capturedCaseID string
		var capturedBody []byte
		reqBody := `{"type":"comment","content":"Investigating now.","actorEmail":"attacker@example.com"}`
		client := &mockEntityCaseClient{
			createCaseCommentFn: func(_ context.Context, id string, body []byte) ([]byte, error) {
				capturedCaseID = id
				capturedBody = body
				return []byte(`{"message":"Comment created successfully","comment":{"id":"c-1","createdBy":"svc@example.com"}}`), nil
			},
		}
		h := NewCaseHandler(client, "svc@example.com", nil)
		r := httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/comments", strings.NewReader(reqBody))
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.CreateCaseComment(w, r)

		assertStatus(t, w, http.StatusCreated)
		assertContentType(t, w, "application/json")

		if capturedCaseID != caseID {
			t.Errorf("caseID = %q, want %q", capturedCaseID, caseID)
		}

		var sent struct {
			Type       string `json:"type"`
			Content    string `json:"content"`
			ActorEmail string `json:"actorEmail"`
		}
		if err := json.Unmarshal(capturedBody, &sent); err != nil {
			t.Fatalf("decode captured body: %v; raw: %s", err, capturedBody)
		}
		if sent.Type != "comment" {
			t.Errorf("type = %q, want %q", sent.Type, "comment")
		}
		if sent.Content != "Investigating now." {
			t.Errorf("content = %q, want %q", sent.Content, "Investigating now.")
		}
		if sent.ActorEmail != "svc@example.com" {
			t.Errorf("actorEmail = %q, want the configured service actor email, never the caller-supplied one", sent.ActorEmail)
		}

		resp := decodeJSON[map[string]any](t, w)
		if resp["message"] != "Comment created successfully" {
			t.Errorf("message = %v, want %v", resp["message"], "Comment created successfully")
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to create case comment.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					createCaseCommentFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewCaseHandler(client, "", nil)
				r := httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/comments", strings.NewReader(`{"type":"comment","content":"hi"}`))
				r.SetPathValue("id", caseID)
				w := httptest.NewRecorder()
				h.CreateCaseComment(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestSearchCases(t *testing.T) {
	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{}, "", nil)
		r := httptest.NewRequest(http.MethodPost, "/cases/search", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1)))
		w := httptest.NewRecorder()
		h.SearchCases(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{}, "", nil)
		r := httptest.NewRequest(http.MethodPost, "/cases/search", strings.NewReader(`not-json`))
		w := httptest.NewRecorder()
		h.SearchCases(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("forwards body verbatim and returns upstream response", func(t *testing.T) {
		var capturedBody []byte
		reqBody := `{"filters":{"filters":[{"field":"number","op":"eq","values":["CS0012345"]}]}}`
		client := &mockEntityCaseClient{
			searchCasesFn: func(_ context.Context, body []byte) ([]byte, error) {
				capturedBody = body
				return []byte(`{"cases":[{"id":"11111111-1111-1111-1111-111111111111","number":"CS0012345"}],"total":1,"limit":0,"offset":0}`), nil
			},
		}
		h := NewCaseHandler(client, "", nil)
		r := httptest.NewRequest(http.MethodPost, "/cases/search", strings.NewReader(reqBody))
		w := httptest.NewRecorder()
		h.SearchCases(w, r)

		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		if string(capturedBody) != reqBody {
			t.Errorf("upstream body = %q, want verbatim %q", string(capturedBody), reqBody)
		}

		resp := decodeJSON[map[string]any](t, w)
		cases, _ := resp["cases"].([]any)
		if len(cases) != 1 {
			t.Errorf("cases = %v, want 1 entry", resp["cases"])
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to search cases.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					searchCasesFn: func(_ context.Context, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewCaseHandler(client, "", nil)
				r := httptest.NewRequest(http.MethodPost, "/cases/search", strings.NewReader(`{"filters":{"filters":[]}}`))
				w := httptest.NewRecorder()
				h.SearchCases(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}

func TestAddCaseTag(t *testing.T) {
	const caseID = "11111111-1111-1111-1111-111111111111"

	t.Run("rejects empty case ID", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{}, "svc@example.com", nil)
		r := httptest.NewRequest(http.MethodPost, "/cases//tags", strings.NewReader(`{"label":"urgent"}`))
		w := httptest.NewRecorder()
		h.AddCaseTag(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects non-UUID case ID", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{}, "svc@example.com", nil)
		r := httptest.NewRequest(http.MethodPost, "/cases/case-42/tags", strings.NewReader(`{"label":"urgent"}`))
		r.SetPathValue("id", "case-42")
		w := httptest.NewRecorder()
		h.AddCaseTag(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects body exceeding 1 MiB", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{}, "svc@example.com", nil)
		r := httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/tags", strings.NewReader(strings.Repeat("x", maxRequestBodyBytes+1)))
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.AddCaseTag(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{}, "svc@example.com", nil)
		r := httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/tags", strings.NewReader(`not-json`))
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.AddCaseTag(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty body", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{}, "svc@example.com", nil)
		r := httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/tags", nil)
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.AddCaseTag(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects missing label", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{}, "svc@example.com", nil)
		r := httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/tags", strings.NewReader(`{}`))
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.AddCaseTag(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgLabelRequired)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects blank label", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{}, "svc@example.com", nil)
		r := httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/tags", strings.NewReader(`{"label":"   "}`))
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.AddCaseTag(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgLabelRequired)
		assertContentType(t, w, "application/json")
	})

	t.Run("injects the configured actorEmail and ignores a caller-supplied one", func(t *testing.T) {
		var capturedCaseID string
		var capturedBody []byte
		client := &mockEntityCaseClient{
			addCaseTagFn: func(_ context.Context, id string, body []byte) ([]byte, error) {
				capturedCaseID = id
				capturedBody = body
				return []byte(`{"id":"tag-1","label":"urgent"}`), nil
			},
		}
		h := NewCaseHandler(client, "svc@example.com", nil)
		reqBody := `{"label":"urgent","actorEmail":"attacker@example.com"}`
		r := httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/tags", strings.NewReader(reqBody))
		r.SetPathValue("id", caseID)
		w := httptest.NewRecorder()
		h.AddCaseTag(w, r)

		assertStatus(t, w, http.StatusCreated)
		assertContentType(t, w, "application/json")

		if capturedCaseID != caseID {
			t.Errorf("caseID = %q, want %q", capturedCaseID, caseID)
		}

		var sent struct {
			Label      string `json:"label"`
			ActorEmail string `json:"actorEmail"`
		}
		if err := json.Unmarshal(capturedBody, &sent); err != nil {
			t.Fatalf("decode captured body: %v; raw: %s", err, capturedBody)
		}
		if sent.Label != "urgent" {
			t.Errorf("label = %q, want %q", sent.Label, "urgent")
		}
		if sent.ActorEmail != "svc@example.com" {
			t.Errorf("actorEmail = %q, want the configured service actor email, never the caller-supplied one", sent.ActorEmail)
		}

		resp := decodeJSON[map[string]any](t, w)
		if resp["label"] != "urgent" {
			t.Errorf("label = %v, want %v", resp["label"], "urgent")
		}
	})

	t.Run("upstream errors are mapped correctly", func(t *testing.T) {
		for _, tc := range upstreamErrors("Failed to add case tag.") {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				client := &mockEntityCaseClient{
					addCaseTagFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewCaseHandler(client, "svc@example.com", nil)
				r := httptest.NewRequest(http.MethodPost, "/cases/"+caseID+"/tags", strings.NewReader(`{"label":"urgent"}`))
				r.SetPathValue("id", caseID)
				w := httptest.NewRecorder()
				h.AddCaseTag(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
				assertContentType(t, w, "application/json")
			})
		}
	})
}
