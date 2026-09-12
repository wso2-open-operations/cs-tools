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

const testUpdateRequestCaseID = "11111111-1111-1111-1111-111111111111"

func TestRequestCaseUpdate(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := httptest.NewRequest(http.MethodPost, "/cases/"+testUpdateRequestCaseID+"/request-update", strings.NewReader(`{"stage":"first"}`))
		r.SetPathValue("id", testUpdateRequestCaseID)
		w := httptest.NewRecorder()
		h.RequestCaseUpdate(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
		assertContentType(t, w, "application/json")
	})

	t.Run("rejects empty case ID", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases//request-update", strings.NewReader(`{"stage":"first"}`)))
		w := httptest.NewRecorder()
		h.RequestCaseUpdate(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
	})

	t.Run("rejects malformed UUID", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/not-a-uuid/request-update", strings.NewReader(`{"stage":"first"}`)))
		r.SetPathValue("id", "not-a-uuid")
		w := httptest.NewRecorder()
		h.RequestCaseUpdate(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgInvalidUUID)
	})

	t.Run("rejects body exceeding the size cap", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		body := `{"stage":"custom","customContent":"` + strings.Repeat("x", maxRequestBodyBytes) + `"}`
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+testUpdateRequestCaseID+"/request-update", strings.NewReader(body)))
		r.SetPathValue("id", testUpdateRequestCaseID)
		w := httptest.NewRecorder()
		h.RequestCaseUpdate(w, r)
		assertStatus(t, w, http.StatusRequestEntityTooLarge)
		assertErrorMessage(t, w, ErrMsgTooLarge)
	})

	t.Run("rejects invalid JSON body", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+testUpdateRequestCaseID+"/request-update", strings.NewReader(`not-json`)))
		r.SetPathValue("id", testUpdateRequestCaseID)
		w := httptest.NewRecorder()
		h.RequestCaseUpdate(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
	})

	t.Run("rejects an unrecognized stage", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+testUpdateRequestCaseID+"/request-update", strings.NewReader(`{"stage":"third"}`)))
		r.SetPathValue("id", testUpdateRequestCaseID)
		w := httptest.NewRecorder()
		h.RequestCaseUpdate(w, r)
		assertStatus(t, w, http.StatusBadRequest)
		assertErrorMessage(t, w, ErrMsgBadRequest)
	})

	t.Run("rejects empty customContent when stage is custom", func(t *testing.T) {
		for _, body := range []string{
			`{"stage":"custom"}`,
			`{"stage":"custom","customContent":""}`,
			`{"stage":"custom","customContent":"   "}`,
		} {
			body := body
			t.Run(body, func(t *testing.T) {
				h := NewCaseHandler(&mockEntityCaseClient{})
				r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+testUpdateRequestCaseID+"/request-update", strings.NewReader(body)))
				r.SetPathValue("id", testUpdateRequestCaseID)
				w := httptest.NewRecorder()
				h.RequestCaseUpdate(w, r)
				assertStatus(t, w, http.StatusBadRequest)
				assertErrorMessage(t, w, ErrMsgBadRequest)
			})
		}
	})

	t.Run("rejects a fixed-template stage carrying customContent", func(t *testing.T) {
		for _, stage := range []string{"first", "second", "final"} {
			stage := stage
			t.Run(stage, func(t *testing.T) {
				h := NewCaseHandler(&mockEntityCaseClient{})
				body := `{"stage":"` + stage + `","customContent":"not allowed here"}`
				r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+testUpdateRequestCaseID+"/request-update", strings.NewReader(body)))
				r.SetPathValue("id", testUpdateRequestCaseID)
				w := httptest.NewRecorder()
				h.RequestCaseUpdate(w, r)
				assertStatus(t, w, http.StatusBadRequest)
				assertErrorMessage(t, w, ErrMsgBadRequest)
			})
		}
	})

	t.Run("rejects when case state is not awaiting_info or solution_proposed", func(t *testing.T) {
		for _, state := range []string{"open", "work_in_progress", "waiting_on_wso2", "reopened", "closed"} {
			state := state
			t.Run(state, func(t *testing.T) {
				client := &mockEntityCaseClient{
					getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
						return []byte(`{"state":"` + state + `","type":"case"}`), nil
					},
				}
				h := NewCaseHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+testUpdateRequestCaseID+"/request-update", strings.NewReader(`{"stage":"first"}`)))
				r.SetPathValue("id", testUpdateRequestCaseID)
				w := httptest.NewRecorder()
				h.RequestCaseUpdate(w, r)
				assertStatus(t, w, http.StatusConflict)
				assertErrorMessage(t, w, ErrMsgRequestUpdateNotAllowed)
			})
		}
	})

	t.Run("rejects when the caller is not the case's assigned engineer", func(t *testing.T) {
		client := &mockEntityCaseClient{
			getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
				return []byte(`{"state":"awaiting_info","type":"case","assignedEngineer":{"id":"someone-else"}}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+testUpdateRequestCaseID+"/request-update", strings.NewReader(`{"stage":"first"}`)))
		r.SetPathValue("id", testUpdateRequestCaseID)
		w := httptest.NewRecorder()
		h.RequestCaseUpdate(w, r)
		assertStatus(t, w, http.StatusForbidden)
		assertErrorMessage(t, w, ErrMsgCommentNotOwnCase)
	})

	t.Run("rejects when the case has no assigned engineer", func(t *testing.T) {
		client := &mockEntityCaseClient{
			getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
				return []byte(`{"state":"awaiting_info","type":"case"}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+testUpdateRequestCaseID+"/request-update", strings.NewReader(`{"stage":"first"}`)))
		r.SetPathValue("id", testUpdateRequestCaseID)
		w := httptest.NewRecorder()
		h.RequestCaseUpdate(w, r)
		assertStatus(t, w, http.StatusForbidden)
		assertErrorMessage(t, w, ErrMsgCommentNotOwnCase)
	})

	t.Run("fails closed when the caller's own id cannot be resolved", func(t *testing.T) {
		var commentCreated bool
		client := &mockEntityCaseClient{
			getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
				return []byte(`{"state":"awaiting_info","type":"case","assignedEngineer":{"id":"` + testPlatformUserID + `"}}`), nil
			},
			getUserMeFn: func(context.Context) ([]byte, error) {
				return []byte(`{"id":""}`), nil
			},
			createCaseCommentFn: func(_ context.Context, _ string, _ []byte) ([]byte, error) {
				commentCreated = true
				return []byte(`{"id":"comment-1"}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+testUpdateRequestCaseID+"/request-update", strings.NewReader(`{"stage":"first"}`)))
		r.SetPathValue("id", testUpdateRequestCaseID)
		w := httptest.NewRecorder()
		h.RequestCaseUpdate(w, r)
		assertStatus(t, w, http.StatusInternalServerError)
		assertErrorMessage(t, w, ErrMsgInternal)
		if commentCreated {
			t.Error("comment was created despite the caller's identity being unresolvable")
		}
	})

	t.Run("does not resolve the caller's id when the state gate already rejects", func(t *testing.T) {
		client := &mockEntityCaseClient{
			getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
				return []byte(`{"state":"open","type":"case","assignedEngineer":{"id":"` + testPlatformUserID + `"}}`), nil
			},
			getUserMeFn: func(context.Context) ([]byte, error) {
				t.Error("GetUserMe must not be called once the state gate has rejected the request")
				return nil, nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+testUpdateRequestCaseID+"/request-update", strings.NewReader(`{"stage":"first"}`)))
		r.SetPathValue("id", testUpdateRequestCaseID)
		w := httptest.NewRecorder()
		h.RequestCaseUpdate(w, r)
		assertStatus(t, w, http.StatusConflict)
		assertErrorMessage(t, w, ErrMsgRequestUpdateNotAllowed)
	})

	t.Run("posts the generic template for a non-migration case in each allowed state", func(t *testing.T) {
		for _, state := range []string{"awaiting_info", "solution_proposed"} {
			for _, stage := range []struct {
				name    string
				content string
			}{
				{"first", requestUpdateTemplates[requestUpdateCategoryGeneric][requestUpdateStageFirst]},
				{"second", requestUpdateTemplates[requestUpdateCategoryGeneric][requestUpdateStageSecond]},
				{"final", requestUpdateTemplates[requestUpdateCategoryGeneric][requestUpdateStageFinal]},
			} {
				state, stage := state, stage
				t.Run(state+"/"+stage.name, func(t *testing.T) {
					var posted map[string]string
					client := &mockEntityCaseClient{
						getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
							return []byte(`{"state":"` + state + `","type":"case","assignedEngineer":{"id":"` + testPlatformUserID + `"}}`), nil
						},
						createCaseCommentFn: func(_ context.Context, _ string, body []byte) ([]byte, error) {
							if err := json.Unmarshal(body, &posted); err != nil {
								t.Fatalf("unmarshal posted comment body: %v", err)
							}
							return []byte(`{"id":"comment-1"}`), nil
						},
					}
					h := NewCaseHandler(client)
					r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+testUpdateRequestCaseID+"/request-update", strings.NewReader(`{"stage":"`+stage.name+`"}`)))
					r.SetPathValue("id", testUpdateRequestCaseID)
					w := httptest.NewRecorder()
					h.RequestCaseUpdate(w, r)
					assertStatus(t, w, http.StatusCreated)
					if posted["type"] != "comment" {
						t.Errorf("posted type = %q, want %q", posted["type"], "comment")
					}
					if posted["content"] != stage.content {
						t.Errorf("posted content = %q, want %q", posted["content"], stage.content)
					}
				})
			}
		}
	})

	t.Run("posts the migration template for an engagement case with engagementType migration", func(t *testing.T) {
		for _, stage := range []struct {
			name    string
			content string
		}{
			{"first", requestUpdateTemplates[requestUpdateCategoryMigration][requestUpdateStageFirst]},
			{"second", requestUpdateTemplates[requestUpdateCategoryMigration][requestUpdateStageSecond]},
			{"final", requestUpdateTemplates[requestUpdateCategoryMigration][requestUpdateStageFinal]},
		} {
			stage := stage
			t.Run(stage.name, func(t *testing.T) {
				var posted map[string]string
				client := &mockEntityCaseClient{
					getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
						return []byte(`{"state":"awaiting_info","type":"engagement","engagementType":"Migration","assignedEngineer":{"id":"` + testPlatformUserID + `"}}`), nil
					},
					createCaseCommentFn: func(_ context.Context, _ string, body []byte) ([]byte, error) {
						if err := json.Unmarshal(body, &posted); err != nil {
							t.Fatalf("unmarshal posted comment body: %v", err)
						}
						return []byte(`{"id":"comment-1"}`), nil
					},
				}
				h := NewCaseHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+testUpdateRequestCaseID+"/request-update", strings.NewReader(`{"stage":"`+stage.name+`"}`)))
				r.SetPathValue("id", testUpdateRequestCaseID)
				w := httptest.NewRecorder()
				h.RequestCaseUpdate(w, r)
				assertStatus(t, w, http.StatusCreated)
				if posted["content"] != stage.content {
					t.Errorf("posted content = %q, want %q (migration template, capitalized engagementType from upstream)", posted["content"], stage.content)
				}
			})
		}
	})

	t.Run("treats a non-migration engagement as generic", func(t *testing.T) {
		var posted map[string]string
		client := &mockEntityCaseClient{
			getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
				return []byte(`{"state":"awaiting_info","type":"engagement","engagementType":"onboarding","assignedEngineer":{"id":"` + testPlatformUserID + `"}}`), nil
			},
			createCaseCommentFn: func(_ context.Context, _ string, body []byte) ([]byte, error) {
				_ = json.Unmarshal(body, &posted)
				return []byte(`{"id":"comment-1"}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+testUpdateRequestCaseID+"/request-update", strings.NewReader(`{"stage":"first"}`)))
		r.SetPathValue("id", testUpdateRequestCaseID)
		w := httptest.NewRecorder()
		h.RequestCaseUpdate(w, r)
		assertStatus(t, w, http.StatusCreated)
		want := requestUpdateTemplates[requestUpdateCategoryGeneric][requestUpdateStageFirst]
		if posted["content"] != want {
			t.Errorf("posted content = %q, want the generic template %q", posted["content"], want)
		}
	})

	t.Run("posts the caller-supplied custom message verbatim", func(t *testing.T) {
		var posted map[string]string
		client := &mockEntityCaseClient{
			getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
				return []byte(`{"state":"solution_proposed","type":"case","assignedEngineer":{"id":"` + testPlatformUserID + `"}}`), nil
			},
			createCaseCommentFn: func(_ context.Context, _ string, body []byte) ([]byte, error) {
				_ = json.Unmarshal(body, &posted)
				return []byte(`{"id":"comment-1"}`), nil
			},
		}
		h := NewCaseHandler(client)
		r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+testUpdateRequestCaseID+"/request-update", strings.NewReader(`{"stage":"custom","customContent":"Please confirm you received our fix."}`)))
		r.SetPathValue("id", testUpdateRequestCaseID)
		w := httptest.NewRecorder()
		h.RequestCaseUpdate(w, r)
		assertStatus(t, w, http.StatusCreated)
		if posted["content"] != "Please confirm you received our fix." {
			t.Errorf("posted content = %q, want the custom message verbatim", posted["content"])
		}
	})

	t.Run("maps a GetCase upstream error via mapUpstreamErrorGeneric", func(t *testing.T) {
		for _, tc := range upstreamErrorsGeneric("Failed to request an update.") {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				client := &mockEntityCaseClient{
					getCaseFn: func(_ context.Context, _ string) ([]byte, error) {
						return nil, tc.err
					},
				}
				h := NewCaseHandler(client)
				r := withUser(httptest.NewRequest(http.MethodPost, "/cases/"+testUpdateRequestCaseID+"/request-update", strings.NewReader(`{"stage":"first"}`)))
				r.SetPathValue("id", testUpdateRequestCaseID)
				w := httptest.NewRecorder()
				h.RequestCaseUpdate(w, r)
				assertStatus(t, w, tc.wantCode)
				assertErrorMessage(t, w, tc.wantMsg)
			})
		}
	})
}

func TestGetCaseUpdateRequestTemplates(t *testing.T) {
	t.Run("requires authenticated user", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := httptest.NewRequest(http.MethodGet, "/case-update-request-templates", nil)
		w := httptest.NewRecorder()
		h.GetCaseUpdateRequestTemplates(w, r)
		assertStatus(t, w, http.StatusUnauthorized)
		assertErrorMessage(t, w, ErrMsgUnauthorized)
	})

	t.Run("returns both categories with all three fixed stages and no custom key", func(t *testing.T) {
		h := NewCaseHandler(&mockEntityCaseClient{})
		r := withUser(httptest.NewRequest(http.MethodGet, "/case-update-request-templates", nil))
		w := httptest.NewRecorder()
		h.GetCaseUpdateRequestTemplates(w, r)
		assertStatus(t, w, http.StatusOK)
		assertContentType(t, w, "application/json")

		got := decodeJSON[requestUpdateTemplateResponse](t, w)

		for _, cat := range []struct {
			name string
			m    map[requestUpdateStage]string
		}{
			{"generic", got.Generic},
			{"migration", got.Migration},
		} {
			if len(cat.m) != 3 {
				t.Errorf("%s: got %d stages, want 3: %v", cat.name, len(cat.m), cat.m)
			}
			for _, stage := range []requestUpdateStage{requestUpdateStageFirst, requestUpdateStageSecond, requestUpdateStageFinal} {
				if cat.m[stage] == "" {
					t.Errorf("%s: missing content for stage %q", cat.name, stage)
				}
			}
			if _, ok := cat.m[requestUpdateStageCustom]; ok {
				t.Errorf("%s: unexpectedly contains a %q entry", cat.name, requestUpdateStageCustom)
			}
		}

		if got.Generic[requestUpdateStageFirst] != requestUpdateTemplates[requestUpdateCategoryGeneric][requestUpdateStageFirst] {
			t.Error("generic/first content does not match the source template")
		}
		if got.Migration[requestUpdateStageFinal] != requestUpdateTemplates[requestUpdateCategoryMigration][requestUpdateStageFinal] {
			t.Error("migration/final content does not match the source template")
		}
	})
}
